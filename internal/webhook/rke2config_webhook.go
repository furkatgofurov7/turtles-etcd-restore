package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	bootstrapv1 "github.com/rancher-sandbox/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const (
	rke2ConfigNameLabel = "cluster-api.cattle.io/rke2config-name"
	planSecretNameLabel = "cluster-api.cattle.io/plan-secret-name"

	serviceAccountSecretLabel = "cluster-api.cattle.io/service-account.name"

	secretTypeMachinePlan = "cluster-api.cattle.io/machine-plan"

	defaultFileOwner = "root:root"
)

// +kubebuilder:webhook:path=/mutate-bootstrap-cluster-x-k8s-io-v1beta1-rke2config,mutating=true,failurePolicy=fail,sideEffects=None,groups=bootstrap.cluster.x-k8s.io,resources=rke2configs,verbs=create;update,versions=v1beta1,name=mrke2config.kb.io,admissionReviewVersions=v1

type RKE2ConfigWebhook struct {
	client.Client
}

var _ webhook.CustomDefaulter = &RKE2ConfigWebhook{}

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *RKE2ConfigWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&bootstrapv1.RKE2Config{}).
		WithDefaulter(r).
		Complete()
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *RKE2ConfigWebhook) Default(ctx context.Context, obj runtime.Object) error {
	logger := log.FromContext(ctx)

	logger.Info("Configuring system agent on for RKE2Config")

	rke2Config, ok := obj.(*bootstrapv1.RKE2Config)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a RKE2Config but got a %T", obj))
	}

	planSecretName := strings.Join([]string{rke2Config.Name, "rke2config", "plan"}, "-")

	if err := r.createSecretPlanResources(ctx, planSecretName, rke2Config); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to create secret plan resources: %s", err))
	}

	serviceAccountToken, err := r.ensureServiceAccountSecretPopulated(ctx, planSecretName)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to ensure service account secret is populated: %s", err))
	}

	logger.Info("Service account secret is populated")

	serverUrlSetting := &unstructured.Unstructured{}
	serverUrlSetting.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Kind:    "Setting",
		Version: "v3",
	})

	if err := r.Get(context.Background(), client.ObjectKey{
		Name: "server-url",
	}, serverUrlSetting); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get server url setting: %s", err))
	}
	serverUrl, ok := serverUrlSetting.Object["value"].(string)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get server url setting: %s", err))
	}

	if serverUrl == "" {
		return apierrors.NewBadRequest("server url setting is empty")
	}

	caSetting := &unstructured.Unstructured{}
	caSetting.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "management.cattle.io",
		Kind:    "Setting",
		Version: "v3",
	})
	if err := r.Get(context.Background(), client.ObjectKey{
		Name: "cacerts",
	}, caSetting); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get ca setting: %s", err))
	}

	pem, ok := caSetting.Object["value"].(string)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get ca setting: %s", err))
	}

	if err := r.createConnectInfoJson(ctx, rke2Config, planSecretName, serverUrl, pem, serviceAccountToken); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to create connect info json: %s", err))
	}

	if err := r.createSystemAgentInstallScript(ctx, serverUrl, rke2Config); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to create system agent install script: %s", err))
	}

	if err := r.createConfigYAML(rke2Config); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to create config.yaml: %s", err))
	}

	r.addPostInstallCommands(rke2Config)

	return nil
}

func (r *RKE2ConfigWebhook) createSecretPlanResources(ctx context.Context, planSecretName string, rke2Config *bootstrapv1.RKE2Config) error {
	logger := log.FromContext(ctx)

	logger.Info("Creating secret plan resources")

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: rke2Config.Namespace,
			Labels: map[string]string{
				rke2ConfigNameLabel: rke2Config.Name,
				planSecretNameLabel: planSecretName,
			},
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: rke2Config.Namespace,
			Labels: map[string]string{
				rke2ConfigNameLabel: rke2Config.Name,
			},
		},
		Type: secretTypeMachinePlan,
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: rke2Config.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"watch", "get", "update", "list"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{planSecretName},
			},
		},
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: rke2Config.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     planSecretName,
		},
	}

	saSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-token", planSecretName),
			Namespace: rke2Config.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": planSecretName,
			},
			Labels: map[string]string{
				serviceAccountSecretLabel: planSecretName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	for _, obj := range []client.Object{sa, secret, role, roleBinding, saSecret} {
		if err := r.Create(ctx, obj); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return fmt.Errorf("failed to create %s: %w", obj.GetObjectKind().GroupVersionKind().String(), err)
			}
		}
	}

	return nil
}

func (r *RKE2ConfigWebhook) ensureServiceAccountSecretPopulated(ctx context.Context, planSecretName string) ([]byte, error) {
	logger := log.FromContext(ctx)

	logger.Info("Ensuring service account secret is populated")

	serviceAccountToken := []byte{}

	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, func() error {
		secretList := &corev1.SecretList{}

		if err := r.List(ctx, secretList, client.MatchingLabels{serviceAccountSecretLabel: planSecretName}); err != nil {
			err = fmt.Errorf("failed to list secrets: %w", err)
			logger.Error(err, "failed to list secrets")
			return err
		}

		if len(secretList.Items) == 0 || len(secretList.Items) > 1 {
			err := fmt.Errorf("secret for %s doesn't exist, or more than one secret exists", planSecretName)
			logger.Error(err, "secret for %s doesn't exist, or more than one secret exists", "secret", planSecretName)
			return err
		}

		saSecret := secretList.Items[0]

		if len(saSecret.Data[corev1.ServiceAccountTokenKey]) == 0 {
			err := fmt.Errorf("secret %s not yet populated", planSecretName)
			logger.Error(err, "Secret %s not yet populated", "secret", planSecretName)
			return err
		}

		serviceAccountToken = saSecret.Data[corev1.ServiceAccountTokenKey]

		return nil
	}); err != nil {
		return nil, err
	}
	return serviceAccountToken, nil
}

func (r *RKE2ConfigWebhook) createConnectInfoJson(ctx context.Context, rke2Config *bootstrapv1.RKE2Config, planSecretName, serverUrl, pem string, serviceAccountToken []byte) error {
	connectInfoJsonPath := "/etc/rancher/agent/connect-info-config.json"

	for _, file := range rke2Config.Spec.Files { // TODO: find a better way to check if the file already exists
		if file.Path == connectInfoJsonPath {
			return nil
		}
	}

	kubeConfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"agent": {
				Server:                   serverUrl,
				CertificateAuthorityData: []byte(pem),
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"agent": {
				Token: string(serviceAccountToken),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"agent": {
				Cluster:  "agent",
				AuthInfo: "agent",
			},
		},
		CurrentContext: "agent",
	})

	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to write kubeconfig: %s", err))
	}

	connectInfoConfig := struct {
		Namespace  string `json:"namespace"`
		SecretName string `json:"secretName"`
		KubeConfig string `json:"kubeConfig"`
	}{
		Namespace:  rke2Config.Namespace,
		SecretName: planSecretName,
		KubeConfig: string(kubeConfig),
	}

	connectInfoConfigJson, err := json.MarshalIndent(connectInfoConfig, "", " ")
	if err != nil {
		return err
	}

	connectInfoConfigSecretName := fmt.Sprintf("%s-system-agent-connect-info-config", rke2Config.Name)
	connectInfoConfigKey := "connect-info-config.json"

	if err := r.Create(ctx, &corev1.Secret{ // TODO: set owner references
		ObjectMeta: metav1.ObjectMeta{
			Name:      connectInfoConfigSecretName,
			Namespace: rke2Config.Namespace,
		},
		Data: map[string][]byte{
			connectInfoConfigKey: connectInfoConfigJson,
		},
	},
	); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
		Path:        connectInfoJsonPath,
		Owner:       defaultFileOwner,
		Permissions: "0600",
		ContentFrom: &bootstrapv1.FileSource{
			Secret: bootstrapv1.SecretFileSource{
				Name: connectInfoConfigSecretName,
				Key:  connectInfoConfigKey,
			},
		},
	})

	return nil
}

func (r *RKE2ConfigWebhook) createSystemAgentInstallScript(ctx context.Context, serverUrl string, rke2Config *bootstrapv1.RKE2Config) error {
	systemAgentInstallScriptPath := "/opt/system-agent-install.sh"

	for _, file := range rke2Config.Spec.Files { // TODO: find a better way to check if the file already exists
		if file.Path == systemAgentInstallScriptPath {
			return nil
		}
	}

	installScriptSecretName := fmt.Sprintf("%s-system-agent-install-script", rke2Config.Name)
	installScriptKey := "install.sh"

	serverUrlBash := fmt.Sprintf("CATTLE_SERVER=%s\n", serverUrl)

	if err := r.Create(ctx, &corev1.Secret{ // TODO: set owner references
		ObjectMeta: metav1.ObjectMeta{
			Name:      installScriptSecretName,
			Namespace: rke2Config.Namespace,
		},
		Data: map[string][]byte{
			installScriptKey: []byte(fmt.Sprintf("%s%s", serverUrlBash, installsh)),
		},
	},
	); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
		Path:        systemAgentInstallScriptPath,
		Owner:       defaultFileOwner,
		Permissions: "0600",
		ContentFrom: &bootstrapv1.FileSource{
			Secret: bootstrapv1.SecretFileSource{
				Name: installScriptSecretName,
				Key:  installScriptKey,
			},
		},
	})

	return nil
}

func (r *RKE2ConfigWebhook) createConfigYAML(rke2Config *bootstrapv1.RKE2Config) error {
	configYAMLPath := "/etc/rancher/agent/config.yaml"

	for _, file := range rke2Config.Spec.Files { // TODO: find a better way to check if the file already exists
		if file.Path == configYAMLPath {
			return nil
		}
	}

	rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
		Path:        configYAMLPath,
		Owner:       defaultFileOwner,
		Permissions: "0600",
		Content: `workDirectory: /var/lib/rancher/agent/work
localPlanDirectory: /var/lib/rancher/agent/plans
remoteEnabled: true
connectionInfoFile: /etc/rancher/agent/connect-info-config.json
preserveWorkDirectory: true`,
	})

	return nil
}

func (r *RKE2ConfigWebhook) addPostInstallCommands(rke2Config *bootstrapv1.RKE2Config) {
	postInstallCommand := "sudo sh /opt/system-agent-install.sh"

	for _, cmd := range rke2Config.Spec.PostRKE2Commands {
		if cmd == postInstallCommand {
			return
		}
	}
	rke2Config.Spec.PostRKE2Commands = append(rke2Config.Spec.PostRKE2Commands, postInstallCommand)
}
