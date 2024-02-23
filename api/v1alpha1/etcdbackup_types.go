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

// EtcdMachineBackupSpec defines the desired state of EtcdMachineBackup
type EtcdMachineBackupSpec struct {
	ClusterName string `json:"clusterName"`
	MachineName string `json:"machineName"`
	Location    string `json:"location"`
	Manual      bool   `json:"manual"`
}

// EtcdMachineBackupStatus defines the observed state of EtcdMachineBackup
type EtcdMachineBackupStatus struct {
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EtcdMachineBackup is the Schema for the EtcdMachineBackups API
type EtcdMachineBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// backup spec
	Spec EtcdMachineBackupSpec `json:"spec,omitempty"`
	// backup status
	Status EtcdMachineBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EtcdMachineBackupList contains a list of EtcdMachineBackup
type EtcdMachineBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdMachineBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdMachineBackup{}, &EtcdMachineBackupList{})
}
