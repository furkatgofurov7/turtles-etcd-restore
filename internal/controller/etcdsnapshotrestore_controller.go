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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	etcdv1alpha1 "github.com/furkatgofurov7/turtles-etcd-restore/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// EtcdSnapshotReconciler reconciles a EtcdSnapshot object
type EtcdSnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// scope holds the different objects that are read and used during the reconcile.
type scope struct {
	// cluster is the Cluster object the Machine belongs to.
	// It is set at the beginning of the reconcile function.
	cluster *clusterv1.Cluster

	// machine is the Machine object. It is set at the beginning
	// of the reconcile function.
	machine *clusterv1.Machine

	// secret is the Secret object.
	secret *corev1.Secret

	// etcdmachinebackup is the EtcdMachineBackup object.
	etcdmachinebackup *etcdv1alpha1.EtcdMachineBackup
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

	etcdSnapshot := &etcdv1alpha1.ETCDSnapshotRestore{ObjectMeta: metav1.ObjectMeta{
		Name:      req.Name,
		Namespace: req.Namespace,
	}}
	if err := r.Client.Get(ctx, req.NamespacedName, etcdSnapshot); apierrors.IsNotFound(err) {
		// Object not found, return. Created objects are automatically garbage collected.
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, fmt.Sprintf("Unable to get EtcdSnapshot resource: %s", req.String()))
		return ctrl.Result{}, err
	}

	// Handle deleted etcdSnapshot
	if !etcdSnapshot.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, etcdSnapshot)
	}

	return r.reconcileNormal(ctx, etcdSnapshot)
}

func (r *EtcdSnapshotReconciler) reconcileNormal(ctx context.Context, etcdSnapshot *etcdv1alpha1.ETCDSnapshotRestore) (_ ctrl.Result, err error) {
	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotReconciler) reconcileDelete(ctx context.Context, etcdSnapshot *etcdv1alpha1.ETCDSnapshotRestore) (ctrl.Result, error) {

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EtcdSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1alpha1.ETCDSnapshotRestore{}).
		Complete(r)
}

// getClusters returns the list of clusters from the given list of objects.
func getClusters(objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	res := make([]*unstructured.Unstructured, 0)
	for _, obj := range objs {
		if obj.GroupVersionKind() == clusterv1.GroupVersion.WithKind("Cluster") {
			res = append(res, obj)
		}
	}
	return res
}

// getEtcdMachineBackup returns the name of the EtcdMachineBackup object for the given machine.
func getEtcdMachineBackup(ctx context.Context, cl client.Client, cluster *clusterv1.Cluster, nodeName string) (string, error) {
	etcdMachineBackupList := &etcdv1alpha1.EtcdMachineBackupList{}
	if err := cl.List(ctx, etcdMachineBackupList, client.InNamespace(cluster.Namespace)); err != nil {
		return "", fmt.Errorf("failed to list etcd machine backups: %w", err)
	}

	for _, etcdMachineBackup := range etcdMachineBackupList.Items {
		if etcdMachineBackup.Spec.ClusterName == cluster.Name {
			if etcdMachineBackup.Spec.MachineName == nodeName {
				return etcdMachineBackup.Name, nil
			}
		}
	}
	return "", nil
}

// getAllMachinesInCluster returns all the machines in the cluster.
func getAllMachinesInCluster(ctx context.Context, c client.Client, namespace, clusterName string) (*clusterv1.MachineList, error) {
	if clusterName == "" {
		return nil, nil
	}

	machineList := &clusterv1.MachineList{}
	labels := map[string]string{clusterv1.ClusterNameLabel: clusterName}

	if err := c.List(ctx, machineList, client.InNamespace(namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return machineList, nil
}

// getSecret returns the secret object for the given namespace and clusterName.
// func (ctx context.Context, c client.Client, namespace, clusterName string) getSecret() (*corev1.Secret, error) {
// 	secret := corev1.Secret{}
// 	if err, machines := getAllMachinesInCluster(ctx, с, clusterName); err != nil {

// 		return nil, fmt.Errorf("failed to list machines: %w", err)
// 	}
// 	if err := c.List(ctx, clusterv1.MachineList, client.InNamespace(cluster.Namespace)); err != nil {
// 		return "", fmt.Errorf("failed to list etcd machine backups: %w", err)
// 	}
// 	for _, machine := range machines.Items {

// 	}
// 	return secret
// }
