/*
Copyright 2023.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MigritorSpec defines the desired state of Migritor
type MigritorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of Migritor. Edit migritor_types.go to remove/update
	Foo                     string `json:"foo,omitempty"`
	SourcePodName           string `json:"source_pod_name,omitempty"`
	SourcePodNameSpace      string `json:"source_pod_name_space,omitempty"`
	SourcePodContainer      string `json:"source_pod_container,omitempty"`
	SourceNodeId            string `json:"sourceNodeId,omitempty"`
	DestinationNodeId       string `json:"destinationNodeId,omitempty"`
	DestinationPodName      string `json:"destinationPodName,omitempty"`
	DestinationPodNameSpace string `json:"destinationPodName_space,omitempty"`
	Destination             string `json:"destination,omitempty"`
	RegistryUsername        string `json:"registryUsername,omitempty"`
	RegistryPassword        string `json:"registryPassword,omitempty"`
}

// MigritorStatus defines the observed state of Migritor
type MigritorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Migritor is the Schema for the migritors API
type Migritor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MigritorSpec   `json:"spec,omitempty"`
	Status MigritorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MigritorList contains a list of Migritor
type MigritorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Migritor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Migritor{}, &MigritorList{})
}
