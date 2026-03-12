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

package scope

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1 "github.com/fabiendupont/cluster-api-provider-nvidia-carbide/api/v1beta1"
	"github.com/fabiendupont/cluster-api-provider-nvidia-carbide/pkg/providerid"
)

// MachineScopeParams defines parameters for creating a machine scope
type MachineScopeParams struct {
	Client               client.Client
	Cluster              *clusterv1.Cluster
	Machine              *clusterv1.Machine
	NvidiaCarbideCluster *infrastructurev1.NvidiaCarbideCluster
	NvidiaCarbideMachine *infrastructurev1.NvidiaCarbideMachine
	NvidiaCarbideClient  NvidiaCarbideClientInterface
	OrgName              string // Organization name for API calls
}

// MachineScope defines the scope for machine operations
type MachineScope struct {
	client.Client

	Cluster              *clusterv1.Cluster
	Machine              *clusterv1.Machine
	NvidiaCarbideCluster *infrastructurev1.NvidiaCarbideCluster
	NvidiaCarbideMachine *infrastructurev1.NvidiaCarbideMachine
	NvidiaCarbideClient  NvidiaCarbideClientInterface
	OrgName              string // Organization name for API calls
}

// NewMachineScope creates a new machine scope
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if params.Cluster == nil {
		return nil, fmt.Errorf("cluster is required")
	}
	if params.Machine == nil {
		return nil, fmt.Errorf("machine is required")
	}
	if params.NvidiaCarbideCluster == nil {
		return nil, fmt.Errorf("nvidia carbide cluster is required")
	}
	if params.NvidiaCarbideMachine == nil {
		return nil, fmt.Errorf("nvidia carbide machine is required")
	}
	if params.NvidiaCarbideClient == nil {
		return nil, fmt.Errorf("nvidia carbide client is required")
	}
	if params.OrgName == "" {
		return nil, fmt.Errorf("org name is required")
	}

	return &MachineScope{
		Client:               params.Client,
		Cluster:              params.Cluster,
		Machine:              params.Machine,
		NvidiaCarbideCluster: params.NvidiaCarbideCluster,
		NvidiaCarbideMachine: params.NvidiaCarbideMachine,
		NvidiaCarbideClient:  params.NvidiaCarbideClient,
		OrgName:              params.OrgName,
	}, nil
}

// Name returns the machine name
func (s *MachineScope) Name() string {
	return s.Machine.Name
}

// Namespace returns the machine namespace
func (s *MachineScope) Namespace() string {
	return s.Machine.Namespace
}

// IsControlPlane returns whether the machine is a control plane node
func (s *MachineScope) IsControlPlane() bool {
	return s.Machine.Labels[clusterv1.MachineControlPlaneLabel] != ""
}

// Role returns the machine role (control-plane or worker)
func (s *MachineScope) Role() string {
	if s.IsControlPlane() {
		return "control-plane"
	}
	return "worker"
}

// ProviderID returns the provider ID, checking status first then falling back to spec
func (s *MachineScope) ProviderID() *providerid.ProviderID {
	// Check status first (set by provider after creation)
	if s.NvidiaCarbideMachine.Status.ProviderID != nil && *s.NvidiaCarbideMachine.Status.ProviderID != "" {
		pid, err := providerid.ParseProviderID(*s.NvidiaCarbideMachine.Status.ProviderID)
		if err == nil {
			return pid
		}
	}

	// Fall back to spec (for pre-existing machines)
	if s.NvidiaCarbideMachine.Spec.ProviderID == nil || *s.NvidiaCarbideMachine.Spec.ProviderID == "" {
		return nil
	}

	pid, err := providerid.ParseProviderID(*s.NvidiaCarbideMachine.Spec.ProviderID)
	if err != nil {
		return nil
	}

	return pid
}

// SetProviderID sets the provider ID in both status and spec
// Format: nvidia-carbide://<org>/<tenant>/<site>/<instance-id>
func (s *MachineScope) SetProviderID(tenantName, siteName, instanceIDStr string) error {
	instanceUUID, err := uuid.Parse(instanceIDStr)
	if err != nil {
		return fmt.Errorf("invalid instance UUID %s: %w", instanceIDStr, err)
	}

	pid := providerid.NewProviderID(s.OrgName, tenantName, siteName, instanceUUID)
	providerIDStr := pid.String()
	s.NvidiaCarbideMachine.Status.ProviderID = &providerIDStr
	s.NvidiaCarbideMachine.Spec.ProviderID = &providerIDStr
	s.Machine.Spec.ProviderID = providerIDStr
	return nil
}

// InstanceID returns the instance ID from status
func (s *MachineScope) InstanceID() string {
	return s.NvidiaCarbideMachine.Status.InstanceID
}

// SetInstanceID sets the instance ID in status
func (s *MachineScope) SetInstanceID(instanceID string) {
	s.NvidiaCarbideMachine.Status.InstanceID = instanceID
}

// MachineID returns the physical machine ID from status
func (s *MachineScope) MachineID() string {
	return s.NvidiaCarbideMachine.Status.MachineID
}

// SetMachineID sets the physical machine ID in status
func (s *MachineScope) SetMachineID(machineID string) {
	s.NvidiaCarbideMachine.Status.MachineID = machineID
}

// InstanceState returns the instance state from status
func (s *MachineScope) InstanceState() string {
	return s.NvidiaCarbideMachine.Status.InstanceState
}

// SetInstanceState sets the instance state in status
func (s *MachineScope) SetInstanceState(state string) {
	s.NvidiaCarbideMachine.Status.InstanceState = state
}

// SetReady sets the ready status
func (s *MachineScope) SetReady(ready bool) {
	s.NvidiaCarbideMachine.Status.Ready = ready
}

// IsReady returns whether the machine is ready
func (s *MachineScope) IsReady() bool {
	return s.NvidiaCarbideMachine.Status.Ready
}

// SetAddresses sets the machine addresses
func (s *MachineScope) SetAddresses(addresses []clusterv1.MachineAddress) {
	s.NvidiaCarbideMachine.Status.Addresses = addresses
	s.Machine.Status.Addresses = addresses
}

// GetBootstrapData returns the bootstrap data for the machine
func (s *MachineScope) GetBootstrapData(ctx context.Context) (string, error) {
	if s.Machine.Spec.Bootstrap.DataSecretName == nil {
		return "", fmt.Errorf("bootstrap data secret name is not set")
	}

	secret := &client.ObjectKey{
		Namespace: s.Machine.Namespace,
		Name:      *s.Machine.Spec.Bootstrap.DataSecretName,
	}

	bootstrapSecret := &corev1.Secret{}
	if err := s.Get(ctx, *secret, bootstrapSecret); err != nil {
		return "", fmt.Errorf("failed to get bootstrap secret: %w", err)
	}

	data, ok := bootstrapSecret.Data["value"]
	if !ok {
		return "", fmt.Errorf("bootstrap secret missing 'value' key")
	}

	return string(data), nil
}

// GetSubnetID returns the subnet ID for the machine's network
func (s *MachineScope) GetSubnetID() (string, error) {
	subnetName := s.NvidiaCarbideMachine.Spec.Network.SubnetName

	// Look up subnet ID from cluster status
	subnetIDs := s.NvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs
	subnetID, ok := subnetIDs[subnetName]
	if !ok {
		return "", fmt.Errorf("subnet %s not found in cluster status", subnetName)
	}

	return subnetID, nil
}

// VPCID returns the VPC ID from the cluster
func (s *MachineScope) VPCID() string {
	return s.NvidiaCarbideCluster.Status.VPCID
}

// TenantID returns the tenant ID from the cluster
func (s *MachineScope) TenantID() string {
	return s.NvidiaCarbideCluster.Spec.TenantID
}

// PatchObject persists the machine status
func (s *MachineScope) PatchObject(ctx context.Context) error {
	// Update NvidiaCarbideMachine status
	if err := s.Client.Status().Update(ctx, s.NvidiaCarbideMachine); err != nil {
		return fmt.Errorf("failed to update nvidia carbide machine status: %w", err)
	}

	// Update Machine status
	if err := s.Client.Status().Update(ctx, s.Machine); err != nil {
		return fmt.Errorf("failed to update machine status: %w", err)
	}

	return nil
}

// Close closes the scope
func (s *MachineScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}
