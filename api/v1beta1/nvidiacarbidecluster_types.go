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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NvidiaCarbideClusterSpec defines the desired state of NvidiaCarbideCluster
type NvidiaCarbideClusterSpec struct {
	// SiteRef references the NVIDIA Carbide Site where the cluster will be provisioned
	// +required
	SiteRef SiteReference `json:"siteRef"`

	// TenantID is the NVIDIA Carbide tenant ID for multi-tenancy
	// +required
	TenantID string `json:"tenantID"`

	// VPC configuration for the cluster network
	// +required
	VPC VPCSpec `json:"vpc"`

	// Subnets for control-plane and worker nodes
	// +kubebuilder:validation:MinItems=1
	// +required
	Subnets []SubnetSpec `json:"subnets"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane
	// +optional
	ControlPlaneEndpoint *clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty"`

	// Authentication contains credentials for accessing the NVIDIA Carbide API
	// +required
	Authentication AuthenticationSpec `json:"authentication"`
}

// SiteReference references an NVIDIA Carbide Site
type SiteReference struct {
	// Name references a Site CRD in the same namespace
	// +optional
	Name string `json:"name,omitempty"`

	// ID directly specifies the Site UUID
	// +optional
	ID string `json:"id,omitempty"`
}

// VPCSpec defines the VPC configuration
type VPCSpec struct {
	// Name of the VPC
	// +kubebuilder:validation:MaxLength=63
	// +required
	Name string `json:"name"`

	// NetworkVirtualizationType specifies the network virtualization type
	// Valid values: ETHERNET_VIRTUALIZER, FNN
	// +kubebuilder:validation:Enum=ETHERNET_VIRTUALIZER;FNN
	// +required
	NetworkVirtualizationType string `json:"networkVirtualizationType"`

	// Labels to apply to the VPC
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// NetworkSecurityGroup configuration
	// +optional
	NetworkSecurityGroup *NSGSpec `json:"networkSecurityGroup,omitempty"`

	// NVLinkLogicalPartitionID attaches an NVLink logical partition at the VPC level
	// +optional
	NVLinkLogicalPartitionID string `json:"nvLinkLogicalPartitionId,omitempty"`

	// Vni specifies an explicit VNI for the VPC
	// +optional
	Vni *int32 `json:"vni,omitempty"`

	// Description for the VPC
	// +optional
	Description string `json:"description,omitempty"`
}

// NSGSpec defines Network Security Group configuration
type NSGSpec struct {
	// Name of the Network Security Group
	// +kubebuilder:validation:MaxLength=63
	// +required
	Name string `json:"name"`

	// Rules for the Network Security Group
	// +optional
	Rules []NSGRule `json:"rules,omitempty"`
}

// NSGRule defines a single security rule
type NSGRule struct {
	// Name of the rule
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`
	// +required
	Name string `json:"name"`

	// Direction of traffic (ingress or egress)
	// +kubebuilder:validation:Enum=ingress;egress
	// +required
	Direction string `json:"direction"`

	// Protocol (tcp, udp, icmp, or all)
	// +kubebuilder:validation:Enum=tcp;udp;icmp;all
	// +required
	Protocol string `json:"protocol"`

	// PortRange specifies the port range (e.g., "80", "1000-2000")
	// +kubebuilder:validation:Pattern=`^[0-9]{1,5}(-[0-9]{1,5})?$`
	// +optional
	PortRange string `json:"portRange,omitempty"`

	// SourceCIDR specifies the source IP range
	// +optional
	SourceCIDR string `json:"sourceCIDR,omitempty"`

	// Action to take (allow or deny)
	// +kubebuilder:validation:Enum=allow;deny
	// +required
	Action string `json:"action"`
}

// SubnetSpec defines a subnet configuration
type SubnetSpec struct {
	// Name of the subnet
	// +kubebuilder:validation:MaxLength=63
	// +required
	Name string `json:"name"`

	// CIDR block for the subnet
	// +required
	CIDR string `json:"cidr"`

	// Role of the subnet (control-plane or worker)
	// +kubebuilder:validation:Enum=control-plane;worker
	// +optional
	Role string `json:"role,omitempty"`

	// Labels to apply to the subnet
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
}

// AuthenticationSpec contains credentials for NVIDIA Carbide API
type AuthenticationSpec struct {
	// SecretRef references a Secret containing NVIDIA Carbide credentials
	// The secret must contain: endpoint, orgName, token
	// +required
	SecretRef corev1.SecretReference `json:"secretRef"`
}

// NvidiaCarbideClusterStatus defines the observed state of NvidiaCarbideCluster.
type NvidiaCarbideClusterStatus struct {
	// Ready indicates if the cluster infrastructure is ready
	// +optional
	Ready bool `json:"ready"`

	// VPCID is the NVIDIA Carbide VPC ID
	// +optional
	VPCID string `json:"vpcID,omitempty"`

	// NetworkStatus contains the network infrastructure status
	// +optional
	NetworkStatus NetworkStatus `json:"networkStatus,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the cluster and will contain a succinct value suitable for
	// machine interpretation.
	// +optional
	FailureReason *capierrors.ClusterStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the cluster and will contain a more descriptive value suitable
	// for human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions represent the current state of the NvidiaCarbideCluster
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// NetworkStatus contains network infrastructure status
type NetworkStatus struct {
	// SubnetIDs maps subnet names to their IDs
	// +optional
	SubnetIDs map[string]string `json:"subnetIDs,omitempty"`

	// NSGID is the Network Security Group ID
	// +optional
	NSGID string `json:"nsgID,omitempty"`

	// IPBlockID is the NVIDIA Carbide IP Block ID used for subnet allocation
	// +optional
	IPBlockID string `json:"ipBlockID,omitempty"`

	// AllocationID is the NVIDIA Carbide Allocation ID
	// +optional
	AllocationID string `json:"allocationID,omitempty"`

	// ChildIPBlockID is the tenant-owned child IP block derived from the allocation
	// +optional
	ChildIPBlockID string `json:"childIPBlockID,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NvidiaCarbideCluster is the Schema for the nvidiacarbideclusters API
type NvidiaCarbideCluster struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of NvidiaCarbideCluster
	// +required
	Spec NvidiaCarbideClusterSpec `json:"spec"`

	// status defines the observed state of NvidiaCarbideCluster
	// +optional
	Status NvidiaCarbideClusterStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// NvidiaCarbideClusterList contains a list of NvidiaCarbideCluster
type NvidiaCarbideClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []NvidiaCarbideCluster `json:"items"`
}

// GetConditions returns the conditions from the status
func (c *NvidiaCarbideCluster) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions sets the conditions in the status
func (c *NvidiaCarbideCluster) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&NvidiaCarbideCluster{}, &NvidiaCarbideClusterList{})
}
