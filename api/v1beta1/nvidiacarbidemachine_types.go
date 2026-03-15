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
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capierrors "sigs.k8s.io/cluster-api/errors"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NvidiaCarbideMachineSpec defines the desired state of NvidiaCarbideMachine
type NvidiaCarbideMachineSpec struct {
	// ProviderID is the unique identifier for the machine instance
	// Format: nvidia-carbide://org/tenant/site/instance-id
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// InstanceType specifies the machine instance configuration
	// +required
	InstanceType InstanceTypeSpec `json:"instanceType"`

	// OperatingSystem configuration for the machine
	// +optional
	OperatingSystem *OSSpec `json:"operatingSystem,omitempty"`

	// Network configuration for the machine
	// +required
	Network NetworkSpec `json:"network"`

	// SSHKeyGroups contains SSH key group IDs for accessing the machine
	// +optional
	SSHKeyGroups []string `json:"sshKeyGroups,omitempty"`

	// InfiniBandInterfaces specifies InfiniBand partition attachments
	// +optional
	InfiniBandInterfaces []InfiniBandInterfaceSpec `json:"infiniBandInterfaces,omitempty"`

	// NVLinkInterfaces specifies NVLink logical partition attachments
	// +optional
	NVLinkInterfaces []NVLinkInterfaceSpec `json:"nvlinkInterfaces,omitempty"`

	// Labels to apply to the NVIDIA Carbide instance
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Description for the NVIDIA Carbide instance
	// +optional
	Description string `json:"description,omitempty"`

	// AlwaysBootWithCustomIpxe when true, the iPXE script will always run on reboot.
	// Requires the OS to be of iPXE type.
	// +optional
	AlwaysBootWithCustomIpxe bool `json:"alwaysBootWithCustomIpxe,omitempty"`

	// PhoneHomeEnabled enables the Phone Home service on the instance
	// +kubebuilder:default:=true
	// +optional
	PhoneHomeEnabled *bool `json:"phoneHomeEnabled,omitempty"`
}

// InfiniBandInterfaceSpec defines an InfiniBand partition attachment
type InfiniBandInterfaceSpec struct {
	// PartitionID is the InfiniBand partition to attach to
	// +required
	PartitionID string `json:"partitionID"`

	// Device is the InfiniBand device name
	// +optional
	Device string `json:"device,omitempty"`

	// DeviceInstance is the index of the device
	// +optional
	DeviceInstance *int32 `json:"deviceInstance,omitempty"`

	// IsPhysical specifies whether to attach over physical interface
	// +optional
	IsPhysical bool `json:"isPhysical,omitempty"`
}

// NVLinkInterfaceSpec defines an NVLink logical partition attachment
type NVLinkInterfaceSpec struct {
	// LogicalPartitionID is the NVLink logical partition to attach to
	// +required
	LogicalPartitionID string `json:"logicalPartitionID"`

	// DeviceInstance is the index of the GPU device
	// +optional
	DeviceInstance *int32 `json:"deviceInstance,omitempty"`
}

// InstanceTypeSpec specifies the instance type or specific machine allocation
type InstanceTypeSpec struct {
	// ID specifies the NVIDIA Carbide instance type UUID
	// Mutually exclusive with MachineID
	// +optional
	ID string `json:"id,omitempty"`

	// MachineID specifies a specific machine UUID for targeted provisioning
	// Mutually exclusive with ID
	// +optional
	MachineID string `json:"machineID,omitempty"`

	// AllowUnhealthyMachine allows provisioning on an unhealthy machine
	// +optional
	AllowUnhealthyMachine bool `json:"allowUnhealthyMachine,omitempty"`
}

// OSSpec defines operating system configuration
type OSSpec struct {
	// ID specifies the NVIDIA Carbide operating system UUID
	// +optional
	ID string `json:"id,omitempty"`

	// Type specifies the OS type (e.g., "ubuntu", "rhel")
	// +optional
	Type string `json:"type,omitempty"`

	// Version specifies the OS version
	// +optional
	Version string `json:"version,omitempty"`
}

// NetworkSpec defines network configuration for the machine
type NetworkSpec struct {
	// SubnetName specifies the subnet to attach the machine to
	// +required
	SubnetName string `json:"subnetName"`

	// AdditionalInterfaces for multi-NIC configurations
	// +optional
	AdditionalInterfaces []NetworkInterface `json:"additionalInterfaces,omitempty"`
}

// NetworkInterface defines an additional network interface
type NetworkInterface struct {
	// SubnetName specifies the subnet for this interface
	// +required
	SubnetName string `json:"subnetName"`

	// IsPhysical indicates if this is a physical interface
	// +optional
	IsPhysical bool `json:"isPhysical,omitempty"`
}

// NvidiaCarbideMachineStatus defines the observed state of NvidiaCarbideMachine.
type NvidiaCarbideMachineStatus struct {
	// Ready indicates if the machine is ready and available
	// +optional
	Ready bool `json:"ready"`

	// InstanceID is the NVIDIA Carbide instance ID
	// +optional
	InstanceID string `json:"instanceID,omitempty"`

	// MachineID is the physical machine ID
	// +optional
	MachineID string `json:"machineID,omitempty"`

	// InstanceState represents the current state of the instance
	// Possible values: Pending, Provisioning, Ready, Error, Terminating
	// +optional
	InstanceState string `json:"instanceState,omitempty"`

	// ProviderID is the unique identifier for the machine instance set by the provider
	// Format: nvidia-carbide://org/tenant/site/instance-id
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Addresses contains the IP addresses assigned to the machine
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the machine and will contain a succinct value suitable for
	// machine interpretation.
	// +optional
	FailureReason *capierrors.MachineStatusError `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the machine and will contain a more descriptive value suitable
	// for human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions represent the current state of the NvidiaCarbideMachine
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// GetConditions returns the conditions from the status
func (m *NvidiaCarbideMachine) GetConditions() []metav1.Condition {
	return m.Status.Conditions
}

// SetConditions sets the conditions in the status
func (m *NvidiaCarbideMachine) SetConditions(conditions []metav1.Condition) {
	m.Status.Conditions = conditions
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NvidiaCarbideMachine is the Schema for the nvidiacarbidemachines API
type NvidiaCarbideMachine struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of NvidiaCarbideMachine
	// +required
	Spec NvidiaCarbideMachineSpec `json:"spec"`

	// status defines the observed state of NvidiaCarbideMachine
	// +optional
	Status NvidiaCarbideMachineStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// NvidiaCarbideMachineList contains a list of NvidiaCarbideMachine
type NvidiaCarbideMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []NvidiaCarbideMachine `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NvidiaCarbideMachine{}, &NvidiaCarbideMachineList{})
}
