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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ETCDSnapshotPhase is a string representation of the phase of the etcd snapshot
type ETCDSnapshotPhase string

const (
	// ETCDSnapshotPhaseStarted is the phase when the snapshot creation has started
	ETCDSnapshotPhaseStarted ETCDSnapshotPhase = "Started"
	// ETCDSnapshotPhaseShutdown is the phase when the etcd cluster is being shutdown
	ETCDSnapshotPhaseShutdown ETCDSnapshotPhase = "Shutdown"
	// ETCDSnapshotPhaseRestore is the phase when the snapshot is being restored
	ETCDSnapshotPhaseRestore ETCDSnapshotPhase = "Restore"
	// ETCDSnapshotPhasePostRestorePodCleanup is the phase when the pods are being cleaned up after the restore
	ETCDSnapshotPhasePostRestorePodCleanup ETCDSnapshotPhase = "PostRestorePodCleanup"
	// ETCDSnapshotPhaseInitialRestartCluster is the phase when the cluster is being restarted after the restore
	ETCDSnapshotPhaseInitialRestartCluster ETCDSnapshotPhase = "InitialRestartCluster"
	// ETCDSnapshotPhasePostRestoreNodeCleanup is the phase when the nodes are being cleaned up after the restore
	ETCDSnapshotPhasePostRestoreNodeCleanup ETCDSnapshotPhase = "PostRestoreNodeCleanup"
	// ETCDSnapshotPhaseRestartCluster is the phase when the cluster is being restarted after the restore
	ETCDSnapshotPhaseRestartCluster ETCDSnapshotPhase = "RestartCluster"
	// ETCDSnapshotPhaseFinished is the phase when the snapshot creation has finished
	ETCDSnapshotPhaseFinished ETCDSnapshotPhase = "Finished"
	// ETCDSnapshotPhaseFailed is the phase when the snapshot creation has failed
	ETCDSnapshotPhaseFailed ETCDSnapshotPhase = "Failed"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ETCDSnapshotRestore is the Schema for the ETCDSnapshots API
type ETCDSnapshotRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ETCDSnapshotSpec   `json:"spec,omitempty"`
	Status            ETCDSnapshotStatus `json:"status,omitempty"`
}

// ETCDSnapshotSpec defines the desired state of ETCDSnapshot
type ETCDSnapshotSpec struct {
	ClusterName           string `json:"clusterName,omitempty"`
	EtcdMachineBackupName string `json:"etcdMachineBackupName,omitempty"`
}

// ETCDSnapshotStatus defines the observed state of ETCDSnapshot
type ETCDSnapshotStatus struct {
	Phase      ETCDSnapshotPhase    `json:"phase,omitempty"`
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true

// ETCDSnapshotList contains a list of ETCDSnapshot
type ETCDSnapshotRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ETCDSnapshotRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ETCDSnapshotRestore{}, &ETCDSnapshotRestoreList{})
}
