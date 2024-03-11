package controller

import (
	"context"
	"fmt"
	"time"

	backupv1 "github.com/furkatgofurov7/turtles-etcd-restore/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	capiutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type EtcdSnapshotSyncReconciler struct {
	Client client.Client

	controller controller.Controller
	Tracker    *remote.ClusterCacheTracker
}

func (r *EtcdSnapshotSyncReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// TODO: Setup predicates for the controller.
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller: %w", err)
	}

	r.controller = c

	return nil
}

func (r *EtcdSnapshotSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI cluster")

	cluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	if cluster.Spec.Paused {
		log.Info("Cluster is paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Only reconcile RKE2 clusters
	if cluster.Spec.ControlPlaneRef.Kind != "RKE2ControlPlane" { // TODO: Move to predicate
		log.Info("Cluster is not an RKE2 cluster, skipping reconciliation")
		return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
	}

	if !cluster.Status.ControlPlaneReady {
		log.Info("Control plane is not ready, skipping reconciliation")
		return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
	}

	log.Info("Setting up watch on ETCDSnapshotFile")

	etcdnapshotFile := &unstructured.Unstructured{}
	etcdnapshotFile.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k3s.cattle.io",
		Kind:    "ETCDSnapshotFile",
		Version: "v1",
	})

	if err := r.Tracker.Watch(ctx, remote.WatchInput{
		Name:         "cluster-watchConfigMapWithRestore",
		Cluster:      capiutil.ObjectKey(cluster),
		Watcher:      r.controller,
		Kind:         etcdnapshotFile,
		EventHandler: handler.EnqueueRequestsFromMapFunc(r.etcdSnapshotFile(ctx, cluster)),
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to start watch for nodes: %w", err)
	}

	remoteClient, err := r.Tracker.GetClient(ctx, capiutil.ObjectKey(cluster))
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Listing etcd snapshot files")

	etcdnapshotFileList := &unstructured.UnstructuredList{}
	etcdnapshotFileList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k3s.cattle.io",
		Kind:    "ETCDSnapshotFile",
		Version: "v1",
	})

	if err := remoteClient.List(ctx, etcdnapshotFileList); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to list etcd snapshot files: %w", err)
	}

	for _, snapshotFile := range etcdnapshotFileList.Items {
		log.Info("Found etcd snapshot file", "name", snapshotFile.GetName())

		location, nodeName, snapshotName, readyToUse, err := extractFieldsFromETCDSnapshotFile(snapshotFile)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to extract fields from etcd snapshot file: %w", err)
		}

		if !readyToUse {
			log.Info("Snapshot is not ready to use, skipping")
			continue
		}

		machineName, err := findMachineForBackup(ctx, r.Client, cluster, nodeName)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to find machine for backup: %w", err)
		}

		if machineName == "" {
			log.Info("Machine not found for backup, skipping. Will try again later.")
			continue
		}

		etcdMachineBackup := &backupv1.EtcdMachineBackup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshotName,
				Namespace: cluster.Namespace,
			},
			Spec: backupv1.EtcdMachineBackupSpec{
				ClusterName: cluster.Name,
				Location:    location,
				MachineName: machineName,
			},
		}

		if err := r.Client.Create(ctx, etcdMachineBackup); err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.Info("EtcdMachineBackup already exists, skipping")
				continue
			}

			return ctrl.Result{}, fmt.Errorf("failed to create EtcdMachineBackup: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotSyncReconciler) etcdSnapshotFile(ctx context.Context, cluster *clusterv1.Cluster) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(_ context.Context, o client.Object) []ctrl.Request {
		log.Info("Cluster name", "name", cluster.GetName())

		gvk := schema.GroupVersionKind{
			Group:   "k3s.cattle.io",
			Kind:    "ETCDSnapshotFile",
			Version: "v1",
		}

		if o.GetObjectKind().GroupVersionKind() != gvk {
			panic(fmt.Sprintf("Expected a %s but got a %s", gvk, o.GetObjectKind().GroupVersionKind()))
		}

		return []reconcile.Request{{NamespacedName: capiutil.ObjectKey(cluster)}}
	}
}

func extractFieldsFromETCDSnapshotFile(snapshotFile unstructured.Unstructured) (string, string, string, bool, error) {
	metadata, ok := snapshotFile.Object["metadata"].(map[string]interface{})
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get metadata from etcd snapshot file, expected a map[string]interface{}, got %T", snapshotFile.Object["metadata"])
	}

	snapshotName, ok := metadata["name"].(string)
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get snapshotName from etcd snapshot file spec, expected a string, got %T", metadata["name"])
	}

	spec, ok := snapshotFile.Object["spec"].(map[string]interface{})
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get spec from etcd snapshot file expected a map[string]interface{}, got %T", snapshotFile.Object["spec"])
	}

	location, ok := spec["location"].(string)
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get location from etcd snapshot file spec, expected a string, got %T", spec["location"])
	}

	nodeName, ok := spec["nodeName"].(string)
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get nodeName from etcd snapshot file spec, expected a string, got %T", spec["nodeName"])
	}

	status, ok := snapshotFile.Object["status"].(map[string]interface{})
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get status from etcd snapshot file expected a map[string]interface{}, got %T", snapshotFile.Object["status"])
	}

	readyToUse, ok := status["readyToUse"].(bool)
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get readyToUse from etcd snapshot file status, expected a bool, got %T", status["readyToUse"])
	}

	return location, nodeName, snapshotName, readyToUse, nil
}

func findMachineForBackup(ctx context.Context, cl client.Client, cluster *clusterv1.Cluster, nodeName string) (string, error) {
	machineList := &clusterv1.MachineList{}
	if err := cl.List(ctx, machineList, client.InNamespace(cluster.Namespace)); err != nil {
		return "", fmt.Errorf("failed to list machines: %w", err)
	}

	for _, machine := range machineList.Items {
		if machine.Spec.ClusterName == cluster.Name {
			if machine.Status.NodeRef.Name == nodeName {
				return machine.Name, nil
			}
		}
	}

	return "", nil
}
