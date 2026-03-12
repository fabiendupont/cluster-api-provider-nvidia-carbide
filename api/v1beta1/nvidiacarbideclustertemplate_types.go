/*
Copyright 2026 Fabien Dupont.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NvidiaCarbideClusterTemplateSpec defines the desired state of NvidiaCarbideClusterTemplate
type NvidiaCarbideClusterTemplateSpec struct {
	// Template contains the NvidiaCarbideCluster template specification
	// +required
	Template NvidiaCarbideClusterTemplateResource `json:"template"`
}

// NvidiaCarbideClusterTemplateResource describes the data needed to create a NvidiaCarbideCluster from a template
type NvidiaCarbideClusterTemplateResource struct {
	// Standard object's metadata
	// +optional
	ObjectMeta metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification of the desired behavior of the cluster
	// +required
	Spec NvidiaCarbideClusterSpec `json:"spec"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=nvidiacarbideclustertemplates,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// NvidiaCarbideClusterTemplate is the Schema for the nvidiacarbideclustertemplates API
type NvidiaCarbideClusterTemplate struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of NvidiaCarbideClusterTemplate
	// +required
	Spec NvidiaCarbideClusterTemplateSpec `json:"spec"`
}

// +kubebuilder:object:root=true

// NvidiaCarbideClusterTemplateList contains a list of NvidiaCarbideClusterTemplate
type NvidiaCarbideClusterTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []NvidiaCarbideClusterTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NvidiaCarbideClusterTemplate{}, &NvidiaCarbideClusterTemplateList{})
}
