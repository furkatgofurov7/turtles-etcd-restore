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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	etcdv1alpha1 "github.com/furkatgofurov7/turtles-etcd-restore/api/v1alpha1"
)

// EtcdSnapshotReconciler reconciles a EtcdSnapshot object
type EtcdSnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
