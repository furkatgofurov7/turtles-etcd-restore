/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	etcdv1 "github.com/furkatgofurov7/turtles-etcd-restore/api/v1alpha1"
	bootstrapv1 "github.com/rancher-sandbox/cluster-api-provider-rke2/bootstrap/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// EtcdSnapshotReconciler reconciles a EtcdSnapshot object
type EtcdSnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *EtcdSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1.ETCDSnapshotRestore{}).
		Complete(r)
}

// scope holds the different objects that are read and used during the reconcile.
type scope struct {
	// cluster is the Cluster object the Machine belongs to.
	// It is set at the beginning of the reconcile function.
	cluster *clusterv1.Cluster

	// machine is the Machine object. It is set at the beginning
	// of the reconcile function.
	machines []clusterv1.Machine

	// initMachine is the machine where the etcd snapshot should be restored.
	initMachine *clusterv1.Machine

	// etcdmachinebackup is the EtcdMachineBackup object.
	etcdmachinebackup *etcdv1.EtcdMachineBackup
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets;events;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="management.cattle.io",resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=rke2configs;rke2configs/status;rke2configs/finalizers,verbs=get;list;watch;create;update;patch;delete

func (r *EtcdSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	etcdSnapshotRestore := &etcdv1.ETCDSnapshotRestore{ObjectMeta: metav1.ObjectMeta{
		Name:      req.Name,
		Namespace: req.Namespace,
	}}
	if err := r.Client.Get(ctx, req.NamespacedName, etcdSnapshotRestore); apierrors.IsNotFound(err) {
		// Object not found, return. Created objects are automatically garbage collected.
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, fmt.Sprintf("Unable to get EtcdSnapshot resource: %s", req.String()))
		return ctrl.Result{}, err
	}

	patchBase := client.MergeFromWithOptions(etcdSnapshotRestore.DeepCopy(), client.MergeFromWithOptimisticLock{})

	// Handle deleted etcdSnapshot
	if !etcdSnapshotRestore.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, etcdSnapshotRestore)
	}

	var errs []error

	result, err := r.reconcileNormal(ctx, etcdSnapshotRestore)
	if err != nil {
		errs = append(errs, fmt.Errorf("error reconciling etcd snapshot restore: %w", err))
	}

	etcdSnapshotRestoreStatus := etcdSnapshotRestore.Status.DeepCopy()

	if err := r.Client.Patch(ctx, etcdSnapshotRestore, patchBase); err != nil {
		errs = append(errs, fmt.Errorf("failed to patch etcd snapshot restore: %w", err))
	}

	etcdSnapshotRestore.Status = *etcdSnapshotRestoreStatus

	if err := r.Client.Status().Patch(ctx, etcdSnapshotRestore, patchBase); err != nil {
		errs = append(errs, fmt.Errorf("failed to patch etcd snapshot restore: %w", err))
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errorutils.NewAggregate(errs)
	}

	return result, nil
}

func (r *EtcdSnapshotReconciler) reconcileNormal(ctx context.Context, etcdSnapshotRestore *etcdv1.ETCDSnapshotRestore) (_ ctrl.Result, err error) {
	log := log.FromContext(ctx)

	if etcdSnapshotRestore.Status.Phase == "" {
		etcdSnapshotRestore.Status.Phase = etcdv1.ETCDSnapshotPhaseStarted
	}

	scope, err := newScope(ctx, r.Client, etcdSnapshotRestore)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, machine := range scope.machines {
		if machine.Status.Phase != string(clusterv1.MachinePhaseRunning) {
			log.Info("Machine is not running yet, requeuing", "machine", machine.Name)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
	}

	// Pause CAPI cluster.
	patchBase := client.MergeFromWithOptions(scope.cluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	scope.cluster.Spec.Paused = true

	if err := r.Client.Patch(ctx, scope.cluster, patchBase); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to pause cluster: %w", err)
	}

	switch etcdSnapshotRestore.Status.Phase {
	case etcdv1.ETCDSnapshotPhaseStarted:
		// Stop RKE2 on all the machines.
		return r.stopRKE2OnAllMachines(ctx, scope, etcdSnapshotRestore)
	case etcdv1.ETCDSnapshotPhaseShutdown:
		// Restore the etcd snapshot on the init machine.
		return r.restoreSnaphotOnInitMachine(ctx, scope, etcdSnapshotRestore)
	case etcdv1.ETCDSnapshotPhaseRestore:
		// Start RKE2 on all the machines.
		return r.startRKE2OnAllMachines(ctx, scope, etcdSnapshotRestore)
	case etcdv1.ETCDSnapshotPhaseFinished:
		// Unpause CAPI cluster.
		patchBase = client.MergeFromWithOptions(scope.cluster.DeepCopy(), client.MergeFromWithOptimisticLock{})
		scope.cluster.Spec.Paused = false

		if err := r.Client.Patch(ctx, scope.cluster, patchBase); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to pause cluster: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotReconciler) reconcileDelete(_ context.Context, _ *etcdv1.ETCDSnapshotRestore) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func newScope(ctx context.Context, c client.Client, etcdSnapshotRestore *etcdv1.ETCDSnapshotRestore) (*scope, error) {
	log := log.FromContext(ctx)

	if etcdSnapshotRestore.Spec.ClusterName == "" || etcdSnapshotRestore.Spec.EtcdMachineBackupName == "" {
		return nil, fmt.Errorf("clusterName and etcdMachineBackupName must be set")
	}

	// Get the cluster object.
	cluster := &clusterv1.Cluster{}

	if err := c.Get(ctx, client.ObjectKey{Namespace: etcdSnapshotRestore.Namespace, Name: etcdSnapshotRestore.Spec.ClusterName}, cluster); err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Get all the machines in the cluster.
	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{clusterv1.ClusterNameLabel: cluster.Name}),
	}

	machineList := &clusterv1.MachineList{}
	if err := c.List(ctx, machineList, listOptions...); err != nil {
		return nil, err
	}

	// Get etcd machine backup object.
	etcdMachineBackup := &etcdv1.EtcdMachineBackup{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: etcdSnapshotRestore.Namespace, Name: etcdSnapshotRestore.Spec.EtcdMachineBackupName}, etcdMachineBackup); err != nil {
		return nil, fmt.Errorf("failed to get etcd machine backup: %w", err)
	}

	initMachine := &clusterv1.Machine{}
	for _, machine := range machineList.Items {
		if machine.Name == etcdMachineBackup.Spec.MachineName {
			log.Info("Found the machine where the etcd snapshot should be restored", "machine", machine.Name)
			initMachine = &machine
			break
		}
	}

	return &scope{
		cluster:           cluster,
		machines:          machineList.Items,
		etcdmachinebackup: etcdMachineBackup,
		initMachine:       initMachine,
	}, nil
}

func (r *EtcdSnapshotReconciler) stopRKE2OnAllMachines(ctx context.Context, scope *scope, etcdSnapshotRestore *etcdv1.ETCDSnapshotRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	allMachinesReady := true
	for _, machine := range scope.machines {
		log.Info("Stopping RKE2 on machine", "machine", machine.Name)

		// Get instruction to stop RKE2 on the machine.
		killAllInstruction, err := instructionsAsJson([]OneTimeInstruction{killAllInstruction()})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get kill all instruction: %w", err)
		}

		// Get the plan secret for the machine.
		planSecret, err := getPlanSecretForMachine(ctx, r.Client, &machine)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get plan secret for machine: %w", err)
		}

		if planSecret.Data == nil {
			planSecret.Data = map[string][]byte{}
		}

		// If the plan secret is not filled with the proper plan, fill it.
		if err := updatePlanSecretWithPlan(ctx, r.Client, scope.initMachine, planSecret, killAllInstruction); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to patch plan secret: %w", err)
		}

		planSecret, err = getPlanSecretForMachine(ctx, r.Client, &machine)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get plan secret for machine: %w", err)
		}

		// Check if appliedPlan is equal to the kill all instruction, if not requeue and wait until it is applied.
		// TODO: Handle plan failure.
		if !isPlanApplied(killAllInstruction, planSecret.Data["applied-checksum"]) {
			log.Info("Plan not applied yet", "machine", machine.Name)
			allMachinesReady = false
			continue
		}

		// Decompress the plan output.
		outputMap, err := decompressPlanOutput(planSecret)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to decompress plan output: %w", err)
		}

		log.Info(fmt.Sprintf("Decompressed plan output: %s", outputMap), "machine", machine.Name)
	}

	if allMachinesReady {
		log.Info("All machines are ready to proceed to the next phase, setting phase to shutdown")
		etcdSnapshotRestore.Status.Phase = etcdv1.ETCDSnapshotPhaseShutdown
		return ctrl.Result{}, nil
	}

	log.Info("Not all machines are ready yet, requeuing")

	// Requeue after 30 seconds if not ready.
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *EtcdSnapshotReconciler) restoreSnaphotOnInitMachine(ctx context.Context, scope *scope, etcdSnapshotRestore *etcdv1.ETCDSnapshotRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if scope.etcdmachinebackup.Spec.Location == "" {
		return ctrl.Result{}, fmt.Errorf("etcdMachineBackup location must be set")
	}

	if scope.initMachine == nil {
		return ctrl.Result{}, fmt.Errorf("initMachine must be set")
	}

	// Get the plan secret for the machine.
	planSecret, err := getPlanSecretForMachine(ctx, r.Client, scope.initMachine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get plan secret for machine: %w", err)
	}

	// Get the etcd restore instructions
	instructions, err := instructionsAsJson([]OneTimeInstruction{
		removeServerUrlFromConfig(),
		manifestRemovalInstruction(),
		etcdRestoreInstruction(scope.etcdmachinebackup),
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get etcd restore instructions: %w", err)
	}

	if planSecret.Data == nil {
		planSecret.Data = map[string][]byte{}
	}

	log.Info("Filling plan secret with etcd restore instructions", "machine", scope.initMachine.Name)

	// If the plan secret is not filled with the proper plan, fill it.
	if err := updatePlanSecretWithPlan(ctx, r.Client, scope.initMachine, planSecret, instructions); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch plan secret: %w", err)
	}

	// Check if appliedPlan is equal to the etcd restore, if not requeue and wait until it is applied.
	// TODO: Handle plan failure.
	if !isPlanApplied(instructions, planSecret.Data["applied-checksum"]) {
		log.Info("Plan not applied yet", "machine", scope.initMachine.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Decompress the plan output.
	outputMap, err := decompressPlanOutput(planSecret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to decompress plan output: %w", err)
	}

	log.Info(fmt.Sprintf("Decompressed plan output: %s", outputMap), "machine", scope.initMachine.Name)

	etcdSnapshotRestore.Status.Phase = etcdv1.ETCDSnapshotPhaseRestore

	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotReconciler) startRKE2OnAllMachines(ctx context.Context, scope *scope, etcdSnapshotRestore *etcdv1.ETCDSnapshotRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Start from the init machine.
	initMachineIP := getInternalMachineIP(scope.initMachine)
	if initMachineIP == "" {
		return ctrl.Result{}, fmt.Errorf("failed to get internal machine IP, field is empty")
	}

	planSecret, err := getPlanSecretForMachine(ctx, r.Client, scope.initMachine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get plan secret for machine: %w", err)
	}

	startRKE2InstructionInit, err := instructionsAsJson([]OneTimeInstruction{startRKE2Instruction()})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get start RKE2 instruction: %w", err)
	}

	// If the plan secret is not filled with the proper plan, fill it.
	if err := updatePlanSecretWithPlan(ctx, r.Client, scope.initMachine, planSecret, startRKE2InstructionInit); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch plan secret: %w", err)
	}

	// Check if appliedPlan is equal to the rke2 start instruction, if not requeue and wait until it is applied.
	// TODO: Handle plan failure.
	if !isPlanApplied(startRKE2InstructionInit, planSecret.Data["applied-checksum"]) {
		log.Info("Starting RKE2 on init machine", "machine", scope.initMachine.Name)
		log.Info("Plan not applied yet", "machine", scope.initMachine.Name)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Decompress the plan output.
	outputMap, err := decompressPlanOutput(planSecret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to decompress plan output: %w", err)
	}

	log.Info(fmt.Sprintf("Decompressed plan output for init machine: %s", outputMap), "machine", scope.initMachine.Name)

	// Start RKE2 on all the machines, except the init machine.
	allMachinesReady := true

	for _, machine := range scope.machines {
		if machine.Name == scope.initMachine.Name {
			continue
		}

		log.Info("Starting RKE2 on machine", "machine", machine.Name)

		startRKE2Instructions, err := instructionsAsJson([]OneTimeInstruction{
			removeServerUrlFromConfig(),
			addServerUrlToConfig(initMachineIP),
			removeEtcdDataInstruction(),
			manifestRemovalInstruction(),
			startRKE2Instruction(),
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get start RKE2 instruction: %w", err)
		}

		planSecret, err := getPlanSecretForMachine(ctx, r.Client, &machine)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get plan secret for machine: %w", err)
		}

		if planSecret.Data == nil {
			planSecret.Data = map[string][]byte{}
		}

		// If the plan secret is not filled with the proper plan, fill it.
		if err := updatePlanSecretWithPlan(ctx, r.Client, &machine, planSecret, startRKE2Instructions); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to patch plan secret: %w", err)
		}

		planSecret, err = getPlanSecretForMachine(ctx, r.Client, &machine)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get plan secret for machine: %w", err)
		}

		// Check if appliedPlan is equal to the kill all instruction, if not requeue and wait until it is applied.
		// TODO: Handle plan failure.
		if !isPlanApplied(startRKE2Instructions, planSecret.Data["applied-checksum"]) {
			log.Info("Plan not applied yet", "machine", machine.Name)
			allMachinesReady = false
			continue
		}

		// Decompress the plan output.
		outputMap, err := decompressPlanOutput(planSecret)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to decompress plan output: %w", err)
		}

		log.Info(fmt.Sprintf("Decompressed plan output: %s", outputMap), "machine", machine.Name)
	}

	if allMachinesReady {
		log.Info("All machines are ready and started RKE2")
		etcdSnapshotRestore.Status.Phase = etcdv1.ETCDSnapshotPhaseFinished
		return ctrl.Result{}, nil
	}

	log.Info("Not all machines are ready yet, requeuing")
	// Requeue after 30 seconds if not ready.
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func getPlanSecretForMachine(ctx context.Context, c client.Client, machine *clusterv1.Machine) (*corev1.Secret, error) {
	rke2Config := &bootstrapv1.RKE2Config{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: machine.Namespace, Name: machine.Spec.Bootstrap.ConfigRef.Name}, rke2Config); err != nil {
		return nil, fmt.Errorf("failed to get RKE2Config: %w", err)
	}

	planSecretName := strings.Join([]string{rke2Config.Name, "rke2config", "plan"}, "-")
	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: machine.Namespace, Name: planSecretName}, secret); err != nil {
		return nil, fmt.Errorf("failed to get plan secret: %w", err)
	}

	return secret, nil
}

func updatePlanSecretWithPlan(ctx context.Context, c client.Client, machine *clusterv1.Machine, secret *corev1.Secret, data []byte) error {
	log := log.FromContext(ctx)

	if !bytes.Equal(secret.Data["plan"], data) {
		log.Info("Plan secret not filled with proper plan", "machine", machine.Name)

		patchBase := client.MergeFromWithOptions(secret.DeepCopy(), client.MergeFromWithOptimisticLock{})

		secret.Data["plan"] = []byte(data)
		if err := c.Patch(ctx, secret, patchBase); err != nil {
			return fmt.Errorf("failed to patch plan secret: %w", err)
		}

		log.Info("Patched plan secret with plan", "machine", machine.Name)
	}
	return nil
}

func decompressPlanOutput(secret *corev1.Secret) (map[string][]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(secret.Data["applied-output"]))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decompressedPlanOutput, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	outputMap := map[string][]byte{}
	if err := json.Unmarshal(decompressedPlanOutput, &outputMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal output: %w", err)
	}

	return outputMap, nil
}

func getInternalMachineIP(machine *clusterv1.Machine) string {
	for _, address := range machine.Status.Addresses {
		if address.Type == clusterv1.MachineInternalIP {
			return address.Address
		}
	}
	return ""
}
