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
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1 "github.com/fabiendupont/cluster-api-provider-nvidia-carbide/api/v1beta1"
	bmm "github.com/nvidia/bare-metal-manager-rest/sdk/standard"
)

// NvidiaCarbideClientInterface defines the methods we need from the NVIDIA Carbide REST client
type NvidiaCarbideClientInterface interface {
	// VPC
	CreateVpc(ctx context.Context, org string, req bmm.VpcCreateRequest) (*bmm.VPC, *http.Response, error)
	GetVpc(ctx context.Context, org string, vpcId string) (*bmm.VPC, *http.Response, error)
	DeleteVpc(ctx context.Context, org string, vpcId string) (*http.Response, error)

	// Subnet
	CreateSubnet(ctx context.Context, org string, req bmm.SubnetCreateRequest) (*bmm.Subnet, *http.Response, error)
	GetSubnet(ctx context.Context, org string, subnetId string) (*bmm.Subnet, *http.Response, error)
	DeleteSubnet(ctx context.Context, org string, subnetId string) (*http.Response, error)

	// IPBlock
	CreateIpblock(ctx context.Context, org string, req bmm.IpBlockCreateRequest) (*bmm.IpBlock, *http.Response, error)
	GetIpblock(ctx context.Context, org string, ipBlockId string) (*bmm.IpBlock, *http.Response, error)
	DeleteIpblock(ctx context.Context, org string, ipBlockId string) (*http.Response, error)

	// NetworkSecurityGroup
	CreateNetworkSecurityGroup(
		ctx context.Context, org string, req bmm.NetworkSecurityGroupCreateRequest,
	) (*bmm.NetworkSecurityGroup, *http.Response, error)
	GetNetworkSecurityGroup(
		ctx context.Context, org string, nsgId string,
	) (*bmm.NetworkSecurityGroup, *http.Response, error)
	DeleteNetworkSecurityGroup(ctx context.Context, org string, nsgId string) (*http.Response, error)

	// Allocation
	CreateAllocation(ctx context.Context, org string, req bmm.AllocationCreateRequest) (*bmm.Allocation, *http.Response, error)
	GetAllocation(ctx context.Context, org string, allocationId string) (*bmm.Allocation, *http.Response, error)
	GetAllAllocation(ctx context.Context, org string) ([]bmm.Allocation, *http.Response, error)
	DeleteAllocation(ctx context.Context, org string, allocationId string) (*http.Response, error)

	// Instance List (for duplicate prevention)
	GetAllInstance(ctx context.Context, org string) ([]bmm.Instance, *http.Response, error)

	// Instance
	CreateInstance(ctx context.Context, org string, req bmm.InstanceCreateRequest) (*bmm.Instance, *http.Response, error)
	GetInstance(ctx context.Context, org string, instanceId string) (*bmm.Instance, *http.Response, error)
	DeleteInstance(ctx context.Context, org string, instanceId string) (*http.Response, error)
}

// carbideClient wraps the SDK APIClient and injects auth context
type carbideClient struct {
	client *bmm.APIClient
	token  string
}

func (c *carbideClient) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, bmm.ContextAccessToken, c.token)
}

// VPC methods
func (c *carbideClient) CreateVpc(
	ctx context.Context, org string, req bmm.VpcCreateRequest,
) (*bmm.VPC, *http.Response, error) {
	return c.client.VPCAPI.CreateVpc(c.authCtx(ctx), org).VpcCreateRequest(req).Execute()
}
func (c *carbideClient) GetVpc(ctx context.Context, org, vpcId string) (*bmm.VPC, *http.Response, error) {
	return c.client.VPCAPI.GetVpc(c.authCtx(ctx), org, vpcId).Execute()
}
func (c *carbideClient) DeleteVpc(ctx context.Context, org, vpcId string) (*http.Response, error) {
	return c.client.VPCAPI.DeleteVpc(c.authCtx(ctx), org, vpcId).Execute()
}

// Subnet methods
func (c *carbideClient) CreateSubnet(
	ctx context.Context, org string, req bmm.SubnetCreateRequest,
) (*bmm.Subnet, *http.Response, error) {
	return c.client.SubnetAPI.CreateSubnet(c.authCtx(ctx), org).SubnetCreateRequest(req).Execute()
}
func (c *carbideClient) GetSubnet(ctx context.Context, org, subnetId string) (*bmm.Subnet, *http.Response, error) {
	return c.client.SubnetAPI.GetSubnet(c.authCtx(ctx), org, subnetId).Execute()
}
func (c *carbideClient) DeleteSubnet(ctx context.Context, org, subnetId string) (*http.Response, error) {
	return c.client.SubnetAPI.DeleteSubnet(c.authCtx(ctx), org, subnetId).Execute()
}

// IPBlock methods
func (c *carbideClient) CreateIpblock(
	ctx context.Context, org string, req bmm.IpBlockCreateRequest,
) (*bmm.IpBlock, *http.Response, error) {
	return c.client.IPBlockAPI.CreateIpblock(c.authCtx(ctx), org).IpBlockCreateRequest(req).Execute()
}
func (c *carbideClient) GetIpblock(ctx context.Context, org, ipBlockId string) (*bmm.IpBlock, *http.Response, error) {
	return c.client.IPBlockAPI.GetIpblock(c.authCtx(ctx), org, ipBlockId).Execute()
}
func (c *carbideClient) DeleteIpblock(ctx context.Context, org, ipBlockId string) (*http.Response, error) {
	return c.client.IPBlockAPI.DeleteIpblock(c.authCtx(ctx), org, ipBlockId).Execute()
}

// NetworkSecurityGroup methods
func (c *carbideClient) CreateNetworkSecurityGroup(
	ctx context.Context, org string, req bmm.NetworkSecurityGroupCreateRequest,
) (*bmm.NetworkSecurityGroup, *http.Response, error) {
	return c.client.NetworkSecurityGroupAPI.CreateNetworkSecurityGroup(
		c.authCtx(ctx), org,
	).NetworkSecurityGroupCreateRequest(req).Execute()
}
func (c *carbideClient) GetNetworkSecurityGroup(
	ctx context.Context, org, nsgId string,
) (*bmm.NetworkSecurityGroup, *http.Response, error) {
	return c.client.NetworkSecurityGroupAPI.GetNetworkSecurityGroup(c.authCtx(ctx), org, nsgId).Execute()
}
func (c *carbideClient) DeleteNetworkSecurityGroup(ctx context.Context, org, nsgId string) (*http.Response, error) {
	return c.client.NetworkSecurityGroupAPI.DeleteNetworkSecurityGroup(c.authCtx(ctx), org, nsgId).Execute()
}

// Allocation methods

func (c *carbideClient) CreateAllocation(
	ctx context.Context, org string, req bmm.AllocationCreateRequest,
) (*bmm.Allocation, *http.Response, error) {
	return c.client.AllocationAPI.CreateAllocation(c.authCtx(ctx), org).AllocationCreateRequest(req).Execute()
}
func (c *carbideClient) GetAllocation(
	ctx context.Context, org, allocationId string,
) (*bmm.Allocation, *http.Response, error) {
	return c.client.AllocationAPI.GetAllocation(c.authCtx(ctx), org, allocationId).Execute()
}
func (c *carbideClient) GetAllAllocation(ctx context.Context, org string) ([]bmm.Allocation, *http.Response, error) {
	return c.client.AllocationAPI.GetAllAllocation(c.authCtx(ctx), org).Execute()
}
func (c *carbideClient) DeleteAllocation(ctx context.Context, org, allocationId string) (*http.Response, error) {
	return c.client.AllocationAPI.DeleteAllocation(c.authCtx(ctx), org, allocationId).Execute()
}

// Instance methods

func (c *carbideClient) CreateInstance(
	ctx context.Context, org string, req bmm.InstanceCreateRequest,
) (*bmm.Instance, *http.Response, error) {
	return c.client.InstanceAPI.CreateInstance(c.authCtx(ctx), org).InstanceCreateRequest(req).Execute()
}
func (c *carbideClient) GetInstance(
	ctx context.Context, org, instanceId string,
) (*bmm.Instance, *http.Response, error) {
	return c.client.InstanceAPI.GetInstance(c.authCtx(ctx), org, instanceId).Execute()
}
func (c *carbideClient) DeleteInstance(ctx context.Context, org, instanceId string) (*http.Response, error) {
	return c.client.InstanceAPI.DeleteInstance(c.authCtx(ctx), org, instanceId).Execute()
}
func (c *carbideClient) GetAllInstance(ctx context.Context, org string) ([]bmm.Instance, *http.Response, error) {
	return c.client.InstanceAPI.GetAllInstance(c.authCtx(ctx), org).Execute()
}

// ClusterScopeParams defines parameters for creating a cluster scope
type ClusterScopeParams struct {
	Client               client.Client
	Cluster              *clusterv1.Cluster
	NvidiaCarbideCluster *infrastructurev1.NvidiaCarbideCluster
	NvidiaCarbideClient  NvidiaCarbideClientInterface // Optional: skip creating new client
	OrgName              string                       // Optional: org name
}

// ClusterScope defines the scope for cluster operations
type ClusterScope struct {
	client.Client

	Cluster              *clusterv1.Cluster
	NvidiaCarbideCluster *infrastructurev1.NvidiaCarbideCluster
	NvidiaCarbideClient  NvidiaCarbideClientInterface
	OrgName              string // Organization name for API calls
}

// NewClusterScope creates a new cluster scope
func NewClusterScope(ctx context.Context, params ClusterScopeParams) (*ClusterScope, error) {
	if params.Client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if params.Cluster == nil {
		return nil, fmt.Errorf("cluster is required")
	}
	if params.NvidiaCarbideCluster == nil {
		return nil, fmt.Errorf("nvidia carbide cluster is required")
	}

	var nvidiaCarbideClient NvidiaCarbideClientInterface
	var orgName string

	// Use provided client if available (for testing), otherwise create a new one
	if params.NvidiaCarbideClient != nil {
		nvidiaCarbideClient = params.NvidiaCarbideClient
		orgName = params.OrgName
	} else {
		// Fetch credentials from secret
		secret := &corev1.Secret{}
		secretKey := types.NamespacedName{
			Name:      params.NvidiaCarbideCluster.Spec.Authentication.SecretRef.Name,
			Namespace: params.NvidiaCarbideCluster.Spec.Authentication.SecretRef.Namespace,
		}
		if secretKey.Namespace == "" {
			secretKey.Namespace = params.NvidiaCarbideCluster.Namespace
		}

		if err := params.Client.Get(ctx, secretKey, secret); err != nil {
			return nil, fmt.Errorf("failed to get credentials secret: %w", err)
		}

		// Validate secret contains required fields
		endpoint, ok := secret.Data["endpoint"]
		if !ok {
			return nil, fmt.Errorf("secret %s is missing 'endpoint' field", secretKey.Name)
		}
		orgNameBytes, ok := secret.Data["orgName"]
		if !ok {
			return nil, fmt.Errorf("secret %s is missing 'orgName' field", secretKey.Name)
		}
		token, ok := secret.Data["token"]
		if !ok {
			return nil, fmt.Errorf("secret %s is missing 'token' field", secretKey.Name)
		}

		orgName = string(orgNameBytes)

		endpointStr := string(endpoint)
		if !strings.HasPrefix(endpointStr, "https://") {
			return nil, fmt.Errorf("endpoint must use https:// scheme, got: %s", endpointStr)
		}

		// Create NVIDIA Carbide API client with authentication
		sdkCfg := bmm.NewConfiguration()
		sdkCfg.Servers = bmm.ServerConfigurations{
			{URL: string(endpoint)},
		}
		nvidiaCarbideClient = &carbideClient{
			client: bmm.NewAPIClient(sdkCfg),
			token:  string(token),
		}
	}

	return &ClusterScope{
		Client:               params.Client,
		Cluster:              params.Cluster,
		NvidiaCarbideCluster: params.NvidiaCarbideCluster,
		NvidiaCarbideClient:  nvidiaCarbideClient,
		OrgName:              orgName,
	}, nil
}

// SiteID returns the Site ID from the site reference
func (s *ClusterScope) SiteID(ctx context.Context) (string, error) {
	// If ID is directly specified, use it
	if s.NvidiaCarbideCluster.Spec.SiteRef.ID != "" {
		return s.NvidiaCarbideCluster.Spec.SiteRef.ID, nil
	}

	// TODO: Fetch Site CRD and extract UUID
	// This requires importing the Site CRD type from carbide-rest/site-manager
	// For now, return an error if name-based reference is used
	if s.NvidiaCarbideCluster.Spec.SiteRef.Name != "" {
		return "", fmt.Errorf("site name reference not yet implemented, please use direct ID")
	}

	return "", fmt.Errorf("site reference is empty")
}

// Name returns the cluster name
func (s *ClusterScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// TenantID returns the tenant ID
func (s *ClusterScope) TenantID() string {
	return s.NvidiaCarbideCluster.Spec.TenantID
}

// VPCID returns the VPC ID from status
func (s *ClusterScope) VPCID() string {
	return s.NvidiaCarbideCluster.Status.VPCID
}

// SetVPCID sets the VPC ID in status
func (s *ClusterScope) SetVPCID(vpcID string) {
	s.NvidiaCarbideCluster.Status.VPCID = vpcID
}

// SetReady sets the ready status
func (s *ClusterScope) SetReady(ready bool) {
	s.NvidiaCarbideCluster.Status.Ready = ready
}

// IsReady returns whether the cluster is ready
func (s *ClusterScope) IsReady() bool {
	return s.NvidiaCarbideCluster.Status.Ready
}

// SubnetIDs returns the subnet IDs from status
func (s *ClusterScope) SubnetIDs() map[string]string {
	if s.NvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs == nil {
		s.NvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs = make(map[string]string)
	}
	return s.NvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs
}

// SetSubnetID sets a subnet ID in status
func (s *ClusterScope) SetSubnetID(name, id string) {
	if s.NvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs == nil {
		s.NvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs = make(map[string]string)
	}
	s.NvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs[name] = id
}

// NSGID returns the network security group ID from status
func (s *ClusterScope) NSGID() string {
	return s.NvidiaCarbideCluster.Status.NetworkStatus.NSGID
}

// SetNSGID sets the network security group ID in status
func (s *ClusterScope) SetNSGID(nsgID string) {
	s.NvidiaCarbideCluster.Status.NetworkStatus.NSGID = nsgID
}

// IPBlockID returns the IP block ID from status
func (s *ClusterScope) IPBlockID() string {
	return s.NvidiaCarbideCluster.Status.NetworkStatus.IPBlockID
}

// SetIPBlockID sets the IP block ID in status
func (s *ClusterScope) SetIPBlockID(ipBlockID string) {
	s.NvidiaCarbideCluster.Status.NetworkStatus.IPBlockID = ipBlockID
}


func (s *ClusterScope) AllocationID() string {
	return s.NvidiaCarbideCluster.Status.NetworkStatus.AllocationID
}

func (s *ClusterScope) SetAllocationID(allocationID string) {
	s.NvidiaCarbideCluster.Status.NetworkStatus.AllocationID = allocationID
}

func (s *ClusterScope) ChildIPBlockID() string {
	return s.NvidiaCarbideCluster.Status.NetworkStatus.ChildIPBlockID
}

func (s *ClusterScope) SetChildIPBlockID(childIPBlockID string) {
	s.NvidiaCarbideCluster.Status.NetworkStatus.ChildIPBlockID = childIPBlockID
}

// PatchObject persists the cluster status
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	return s.Client.Status().Update(ctx, s.NvidiaCarbideCluster)
}

// Close closes the scope
func (s *ClusterScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}
