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
	runtime "k8s.io/apimachinery/pkg/runtime"
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

// ETCDSnapshotCreate defines the desired state of ETCDSnapshotCreate
type ETCDSnapshotCreate struct {
	// Changing the Generation is the only thing required to initiate a snapshot creation.
	Generation int `json:"generation,omitempty"`
}

// ETCDSnapshotRestore defines the desired state of ETCDSnapshotRestore
type ETCDSnapshotRestore struct {
	// Name refers to the name of the associated etcdsnapshot object
	Name string `json:"name,omitempty"`
	// Changing the Generation is the only thing required to initiate a snapshot restore.
	Generation int `json:"generation,omitempty"`
	// Set to either none (or empty string), all, or kubernetesVersion
	RestoreRKEConfig string `json:"restoreRKEConfig,omitempty"`
}

// ETCDSnapshot is the Schema for the ETCDSnapshots API
type ETCDSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ETCDSnapshotSpec   `json:"spec,omitempty"`
	SnapshotFile      ETCDSnapshotFile   `json:"snapshotFile,omitempty"`
	Status            ETCDSnapshotStatus `json:"status"`
}

// ETCDSnapshotSpec defines the desired state of ETCDSnapshot
type ETCDSnapshotSpec struct {
	ClusterName string `json:"clusterName,omitempty"`
}

// ETCDSnapshotFile defines the desired state of ETCDSnapshotFile
type ETCDSnapshotFile struct {
	// Name is the name of the snapshot file
	Name string `json:"name,omitempty"`
	// NodeName is the name of the node where the snapshot was taken
	NodeName string `json:"nodeName,omitempty"`
	// Location is the location of the snapshot file
	Location string `json:"location,omitempty"`
	// Metadata is the metadata of the snapshot file
	Metadata string `json:"metadata,omitempty"`
	// CreatedAt is the time when the snapshot was created
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
	// Size is the size of the snapshot file
	Size int64 `json:"size,omitempty"`
	// Status is the status of the snapshot file
	Status string `json:"status,omitempty"`
	// Message is the message of the snapshot file
	Message string `json:"message,omitempty"`
}

// ETCDSnapshotStatus defines the observed state of ETCDSnapshot
type ETCDSnapshotStatus struct {
	// Missing is true if the snapshot is missing
	Missing bool `json:"missing"`
}

// ETCDSnapshotList contains a list of ETCDSnapshot
type ETCDSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ETCDSnapshot `json:"items"`
}

func (e *ETCDSnapshot) DeepCopyObject() runtime.Object {
	return e.DeepCopy()
}

func (e *ETCDSnapshotList) DeepCopyObject() runtime.Object {
	return e.DeepCopy()
}

func init() {
	SchemeBuilder.Register(&ETCDSnapshot{}, &ETCDSnapshotList{})
}
