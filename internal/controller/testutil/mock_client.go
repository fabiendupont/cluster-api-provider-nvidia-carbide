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

package testutil

import (
	"context"
	"net/http"

	bmm "github.com/nvidia/bare-metal-manager-rest/sdk/standard"
)

// MockCarbideClient is a mock implementation of NvidiaCarbideClientInterface for testing
type MockCarbideClient struct {
	// VPC methods
	CreateVPCFunc func(
		ctx context.Context, org string, req bmm.VpcCreateRequest,
	) (*bmm.VPC, *http.Response, error)
	GetVPCFunc func(
		ctx context.Context, org string, vpcId string,
	) (*bmm.VPC, *http.Response, error)
	DeleteVPCFunc func(
		ctx context.Context, org string, vpcId string,
	) (*http.Response, error)

	// Subnet methods
	CreateSubnetFunc func(
		ctx context.Context, org string, req bmm.SubnetCreateRequest,
	) (*bmm.Subnet, *http.Response, error)
	GetSubnetFunc func(
		ctx context.Context, org string, subnetId string,
	) (*bmm.Subnet, *http.Response, error)
	DeleteSubnetFunc func(
		ctx context.Context, org string, subnetId string,
	) (*http.Response, error)

	// Instance methods
	CreateInstanceFunc func(
		ctx context.Context, org string, req bmm.InstanceCreateRequest,
	) (*bmm.Instance, *http.Response, error)
	GetInstanceFunc func(
		ctx context.Context, org string, instanceId string,
	) (*bmm.Instance, *http.Response, error)
	DeleteInstanceFunc func(
		ctx context.Context, org string, instanceId string,
	) (*http.Response, error)

	// Network Security Group methods
	CreateNetworkSecurityGroupFunc func(
		ctx context.Context, org string, req bmm.NetworkSecurityGroupCreateRequest,
	) (*bmm.NetworkSecurityGroup, *http.Response, error)
	GetNetworkSecurityGroupFunc func(
		ctx context.Context, org string, nsgId string,
	) (*bmm.NetworkSecurityGroup, *http.Response, error)
	DeleteNetworkSecurityGroupFunc func(
		ctx context.Context, org string, nsgId string,
	) (*http.Response, error)

	// Allocation methods
	CreateAllocationFunc func(
		ctx context.Context, org string, req bmm.AllocationCreateRequest,
	) (*bmm.Allocation, *http.Response, error)
	GetAllocationFunc func(
		ctx context.Context, org string, allocationId string,
	) (*bmm.Allocation, *http.Response, error)
	GetAllAllocationFunc func(
		ctx context.Context, org string,
	) ([]bmm.Allocation, *http.Response, error)
	DeleteAllocationFunc func(
		ctx context.Context, org string, allocationId string,
	) (*http.Response, error)

	// IP Block methods
	CreateIpblockFunc func(
		ctx context.Context, org string, req bmm.IpBlockCreateRequest,
	) (*bmm.IpBlock, *http.Response, error)
	GetIpblockFunc func(
		ctx context.Context, org string, ipBlockId string,
	) (*bmm.IpBlock, *http.Response, error)
	DeleteIpblockFunc func(
		ctx context.Context, org string, ipBlockId string,
	) (*http.Response, error)

	// Site methods
	GetAllSiteFunc func(
		ctx context.Context, org string,
	) ([]bmm.Site, *http.Response, error)

	// Instance list
	GetAllInstanceFunc func(
		ctx context.Context, org string,
	) ([]bmm.Instance, *http.Response, error)

	// Site details
	GetSiteFunc func(
		ctx context.Context, org string, siteId string,
	) (*bmm.Site, *http.Response, error)

	// Tenant
	GetCurrentTenantFunc func(
		ctx context.Context, org string,
	) (*bmm.Tenant, *http.Response, error)

	// Instance update and history
	UpdateInstanceFunc func(
		ctx context.Context, org string, instanceId string, req bmm.InstanceUpdateRequest,
	) (*bmm.Instance, *http.Response, error)

	GetInstanceStatusHistoryFunc func(
		ctx context.Context, org string, instanceId string,
	) ([]bmm.StatusDetail, *http.Response, error)

	// Batch
	BatchCreateInstanceFunc func(
		ctx context.Context, org string, req bmm.BatchInstanceCreateRequest,
	) ([]bmm.Instance, *http.Response, error)
}

// VPC methods
func (m *MockCarbideClient) CreateVpc(
	ctx context.Context, org string, req bmm.VpcCreateRequest,
) (*bmm.VPC, *http.Response, error) {
	if m.CreateVPCFunc != nil {
		return m.CreateVPCFunc(ctx, org, req)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetVpc(
	ctx context.Context, org string, vpcId string,
) (*bmm.VPC, *http.Response, error) {
	if m.GetVPCFunc != nil {
		return m.GetVPCFunc(ctx, org, vpcId)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) DeleteVpc(
	ctx context.Context, org string, vpcId string,
) (*http.Response, error) {
	if m.DeleteVPCFunc != nil {
		return m.DeleteVPCFunc(ctx, org, vpcId)
	}
	return nil, nil
}

// Subnet methods
func (m *MockCarbideClient) CreateSubnet(
	ctx context.Context, org string, req bmm.SubnetCreateRequest,
) (*bmm.Subnet, *http.Response, error) {
	if m.CreateSubnetFunc != nil {
		return m.CreateSubnetFunc(ctx, org, req)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetSubnet(
	ctx context.Context, org string, subnetId string,
) (*bmm.Subnet, *http.Response, error) {
	if m.GetSubnetFunc != nil {
		return m.GetSubnetFunc(ctx, org, subnetId)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) DeleteSubnet(
	ctx context.Context, org string, subnetId string,
) (*http.Response, error) {
	if m.DeleteSubnetFunc != nil {
		return m.DeleteSubnetFunc(ctx, org, subnetId)
	}
	return nil, nil
}

// Instance methods
func (m *MockCarbideClient) CreateInstance(
	ctx context.Context, org string, req bmm.InstanceCreateRequest,
) (*bmm.Instance, *http.Response, error) {
	if m.CreateInstanceFunc != nil {
		return m.CreateInstanceFunc(ctx, org, req)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetInstance(
	ctx context.Context, org string, instanceId string,
) (*bmm.Instance, *http.Response, error) {
	if m.GetInstanceFunc != nil {
		return m.GetInstanceFunc(ctx, org, instanceId)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) DeleteInstance(
	ctx context.Context, org string, instanceId string,
) (*http.Response, error) {
	if m.DeleteInstanceFunc != nil {
		return m.DeleteInstanceFunc(ctx, org, instanceId)
	}
	return nil, nil
}

// NetworkSecurityGroup methods
func (m *MockCarbideClient) CreateNetworkSecurityGroup(
	ctx context.Context, org string, req bmm.NetworkSecurityGroupCreateRequest,
) (*bmm.NetworkSecurityGroup, *http.Response, error) {
	if m.CreateNetworkSecurityGroupFunc != nil {
		return m.CreateNetworkSecurityGroupFunc(ctx, org, req)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetNetworkSecurityGroup(
	ctx context.Context, org string, nsgId string,
) (*bmm.NetworkSecurityGroup, *http.Response, error) {
	if m.GetNetworkSecurityGroupFunc != nil {
		return m.GetNetworkSecurityGroupFunc(ctx, org, nsgId)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) DeleteNetworkSecurityGroup(
	ctx context.Context, org string, nsgId string,
) (*http.Response, error) {
	if m.DeleteNetworkSecurityGroupFunc != nil {
		return m.DeleteNetworkSecurityGroupFunc(ctx, org, nsgId)
	}
	return nil, nil
}

// Allocation methods
func (m *MockCarbideClient) CreateAllocation(
	ctx context.Context, org string, req bmm.AllocationCreateRequest,
) (*bmm.Allocation, *http.Response, error) {
	if m.CreateAllocationFunc != nil {
		return m.CreateAllocationFunc(ctx, org, req)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetAllocation(
	ctx context.Context, org string, allocationId string,
) (*bmm.Allocation, *http.Response, error) {
	if m.GetAllocationFunc != nil {
		return m.GetAllocationFunc(ctx, org, allocationId)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetAllAllocation(
	ctx context.Context, org string,
) ([]bmm.Allocation, *http.Response, error) {
	if m.GetAllAllocationFunc != nil {
		return m.GetAllAllocationFunc(ctx, org)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) DeleteAllocation(
	ctx context.Context, org string, allocationId string,
) (*http.Response, error) {
	if m.DeleteAllocationFunc != nil {
		return m.DeleteAllocationFunc(ctx, org, allocationId)
	}
	return nil, nil
}

// IPBlock methods
func (m *MockCarbideClient) CreateIpblock(
	ctx context.Context, org string, req bmm.IpBlockCreateRequest,
) (*bmm.IpBlock, *http.Response, error) {
	if m.CreateIpblockFunc != nil {
		return m.CreateIpblockFunc(ctx, org, req)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetIpblock(
	ctx context.Context, org string, ipBlockId string,
) (*bmm.IpBlock, *http.Response, error) {
	if m.GetIpblockFunc != nil {
		return m.GetIpblockFunc(ctx, org, ipBlockId)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) DeleteIpblock(
	ctx context.Context, org string, ipBlockId string,
) (*http.Response, error) {
	if m.DeleteIpblockFunc != nil {
		return m.DeleteIpblockFunc(ctx, org, ipBlockId)
	}
	return nil, nil
}

// Site methods
func (m *MockCarbideClient) GetAllSite(
	ctx context.Context, org string,
) ([]bmm.Site, *http.Response, error) {
	if m.GetAllSiteFunc != nil {
		return m.GetAllSiteFunc(ctx, org)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetAllInstance(
	ctx context.Context, org string,
) ([]bmm.Instance, *http.Response, error) {
	if m.GetAllInstanceFunc != nil {
		return m.GetAllInstanceFunc(ctx, org)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetSite(
	ctx context.Context, org string, siteId string,
) (*bmm.Site, *http.Response, error) {
	if m.GetSiteFunc != nil {
		return m.GetSiteFunc(ctx, org, siteId)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetCurrentTenant(
	ctx context.Context, org string,
) (*bmm.Tenant, *http.Response, error) {
	if m.GetCurrentTenantFunc != nil {
		return m.GetCurrentTenantFunc(ctx, org)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) UpdateInstance(
	ctx context.Context, org string, instanceId string, req bmm.InstanceUpdateRequest,
) (*bmm.Instance, *http.Response, error) {
	if m.UpdateInstanceFunc != nil {
		return m.UpdateInstanceFunc(ctx, org, instanceId, req)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) GetInstanceStatusHistory(
	ctx context.Context, org string, instanceId string,
) ([]bmm.StatusDetail, *http.Response, error) {
	if m.GetInstanceStatusHistoryFunc != nil {
		return m.GetInstanceStatusHistoryFunc(ctx, org, instanceId)
	}
	return nil, nil, nil
}

func (m *MockCarbideClient) BatchCreateInstance(
	ctx context.Context, org string, req bmm.BatchInstanceCreateRequest,
) ([]bmm.Instance, *http.Response, error) {
	if m.BatchCreateInstanceFunc != nil {
		return m.BatchCreateInstanceFunc(ctx, org, req)
	}
	return nil, nil, nil
}

// Helper functions to create common response objects

func MockHTTPResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Header:     make(http.Header),
	}
}

func Ptr[T any](v T) *T {
	return &v
}
