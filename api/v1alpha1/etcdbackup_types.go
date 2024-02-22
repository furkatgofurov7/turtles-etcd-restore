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
)

// S3EtcdBackupConfig defines the configuration of S3EtcdBackup
type S3EtcdBackupConfig struct {
	AccessKey  string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	BucketName string `json:"bucketName,omitempty" yaml:"bucketName,omitempty"`
	CustomCA   string `json:"customCa,omitempty" yaml:"customCa,omitempty"`
	Endpoint   string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Folder     string `json:"folder,omitempty" yaml:"folder,omitempty"`
	Region     string `json:"region,omitempty" yaml:"region,omitempty"`
	SecretKey  string `json:"secretKey,omitempty" yaml:"secretKey,omitempty"`
}

// EtcdBackupConfig defines the configuration of EtcdBackup
type EtcdBackupConfig struct {
	Enabled            *bool               `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	IntervalHours      int64               `json:"intervalHours,omitempty" yaml:"intervalHours,omitempty"`
	Retention          int64               `json:"retention,omitempty" yaml:"retention,omitempty"`
	S3EtcdBackupConfig *S3EtcdBackupConfig `json:"s3EtcdBackupConfig,omitempty" yaml:"s3EtcdBackupConfig,omitempty"`
	SafeTimestamp      bool                `json:"safeTimestamp,omitempty" yaml:"safeTimestamp,omitempty"`
	Timeout            int64               `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// EtcdBackupCondition defines the condition of EtcdBackup
type EtcdBackupCondition struct {
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	LastUpdateTime     string `json:"lastUpdateTime,omitempty" yaml:"lastUpdateTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}

// EtcdBackupSpec defines the desired state of EtcdBackup
type EtcdBackupSpec struct {
	EtcdBackupConfig *EtcdBackupConfig `json:"etcdBackupConfig,omitempty" yaml:"etcdBackupConfig,omitempty"`
	ClusterID        string            `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Filename         string            `json:"filename,omitempty" yaml:"filename,omitempty"`
	Manual           bool              `json:"manual,omitempty" yaml:"manual,omitempty"`
}

// EtcdBackupStatus defines the observed state of EtcdBackup
type EtcdBackupStatus struct {
	ClusterObject     string                `json:"clusterObject,omitempty" yaml:"clusterObject,omitempty"`
	Conditions        []EtcdBackupCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	KubernetesVersion string                `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// EtcdBackup is the Schema for the etcdbackups API
type EtcdBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// backup spec
	Spec EtcdBackupSpec `json:"spec,omitempty"`
	// backup status
	Status EtcdBackupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EtcdBackupList contains a list of EtcdBackup
type EtcdBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdBackup{}, &EtcdBackupList{})
}
