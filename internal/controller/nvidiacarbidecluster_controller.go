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

package controller

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1 "github.com/fabiendupont/cluster-api-provider-nvidia-carbide/api/v1beta1"
	carbidemetrics "github.com/fabiendupont/cluster-api-provider-nvidia-carbide/internal/metrics"
	"github.com/fabiendupont/cluster-api-provider-nvidia-carbide/pkg/scope"
	bmm "github.com/nvidia/bare-metal-manager-rest/sdk/standard"
)

const (
	// NvidiaCarbideClusterFinalizer allows cleanup of NVIDIA Carbide resources before deletion
	NvidiaCarbideClusterFinalizer = "nvidiacarbidecluster.infrastructure.cluster.x-k8s.io"
)

// Condition types
const (
	VPCReadyCondition     clusterv1.ConditionType = "VPCReady"
	SubnetsReadyCondition clusterv1.ConditionType = "SubnetsReady"
	NSGReadyCondition     clusterv1.ConditionType = "NSGReady"
)

// NvidiaCarbideClusterReconciler reconciles a NvidiaCarbideCluster object
type NvidiaCarbideClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// NvidiaCarbideClient can be set for testing to inject a mock client
	NvidiaCarbideClient scope.NvidiaCarbideClientInterface
	// OrgName can be set for testing
	OrgName string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=nvidiacarbideclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=nvidiacarbideclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=nvidiacarbideclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles NvidiaCarbideCluster reconciliation
func (r *NvidiaCarbideClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the NvidiaCarbideCluster instance
	nvidiaCarbideCluster := &infrastructurev1.NvidiaCarbideCluster{}
	if err := r.Get(ctx, req.NamespacedName, nvidiaCarbideCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the owner Cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, nvidiaCarbideCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("Waiting for Cluster Controller to set OwnerRef on NvidiaCarbideCluster")
		return ctrl.Result{}, nil
	}

	// Check if cluster is paused
	if annotations.IsPaused(cluster, nvidiaCarbideCluster) {
		logger.Info("NvidiaCarbideCluster or Cluster is marked as paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Initialize patch helper
	patchHelper, err := patch.NewHelper(nvidiaCarbideCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always attempt to patch the object and status after each reconciliation
	defer func() {
		if err := patchHelper.Patch(ctx, nvidiaCarbideCluster); err != nil {
			logger.Error(err, "failed to patch NvidiaCarbideCluster")
		}
	}()

	// Create cluster scope
	clusterScope, err := scope.NewClusterScope(ctx, scope.ClusterScopeParams{
		Client:               r.Client,
		Cluster:              cluster,
		NvidiaCarbideCluster: nvidiaCarbideCluster,
		NvidiaCarbideClient:  r.NvidiaCarbideClient, // Will be nil in production, set for tests
		OrgName:              r.OrgName,             // Will be empty in production (fetched from secret), set for tests
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create cluster scope: %w", err)
	}

	// Handle deletion
	if !nvidiaCarbideCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	// Handle normal reconciliation
	return r.reconcileNormal(ctx, clusterScope)
}

func (r *NvidiaCarbideClusterReconciler) reconcileNormal(
	ctx context.Context, clusterScope *scope.ClusterScope,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling NvidiaCarbideCluster")

	// Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(clusterScope.NvidiaCarbideCluster, NvidiaCarbideClusterFinalizer) {
		controllerutil.AddFinalizer(clusterScope.NvidiaCarbideCluster, NvidiaCarbideClusterFinalizer)
		return ctrl.Result{Requeue: true}, nil
	}

	// Get Site ID
	siteID, err := clusterScope.SiteID(ctx)
	if err != nil {
		conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
			Type:    string(VPCReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "SiteNotFound",
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	// Ensure IP block and allocation exist before VPC creation
	// (the tenant must have an allocation with the site to create VPCs)
	if _, err := r.ensureIPBlockAndAllocation(ctx, clusterScope, siteID); err != nil {
		conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
			Type:    string(SubnetsReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "AllocationFailed",
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}

	// Reconcile VPC
	if err := r.reconcileVPC(ctx, clusterScope, siteID); err != nil {
		conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
			Type:    string(VPCReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "VPCReconcileFailed",
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}
	conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
		Type:   string(VPCReadyCondition),
		Status: metav1.ConditionTrue,
		Reason: "VPCReady",
	})

	// Reconcile Subnets
	if err := r.reconcileSubnets(ctx, clusterScope, siteID); err != nil {
		conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
			Type:    string(SubnetsReadyCondition),
			Status:  metav1.ConditionFalse,
			Reason:  "SubnetReconcileFailed",
			Message: err.Error(),
		})
		return ctrl.Result{}, err
	}
	conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
		Type:   string(SubnetsReadyCondition),
		Status: metav1.ConditionTrue,
		Reason: "SubnetsReady",
	})

	// Reconcile Network Security Group (if specified)
	if clusterScope.NvidiaCarbideCluster.Spec.VPC.NetworkSecurityGroup != nil {
		if err := r.reconcileNSG(ctx, clusterScope, siteID); err != nil {
			conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
				Type:    string(NSGReadyCondition),
				Status:  metav1.ConditionFalse,
				Reason:  "NSGReconcileFailed",
				Message: err.Error(),
			})
			return ctrl.Result{}, err
		}
		conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
			Type:   string(NSGReadyCondition),
			Status: metav1.ConditionTrue,
			Reason: "NSGReady",
		})
	}

	// Mark cluster as ready
	clusterScope.SetReady(true)
	conditions.Set(clusterScope.NvidiaCarbideCluster, metav1.Condition{
		Type:   string(clusterv1.ReadyCondition),
		Status: metav1.ConditionTrue,
		Reason: "NvidiaCarbideClusterReady",
	})

	r.recordEvent(clusterScope.NvidiaCarbideCluster, corev1.EventTypeNormal, "ClusterInfrastructureReady",
		"Cluster infrastructure is ready")
	logger.Info("Successfully reconciled NvidiaCarbideCluster")
	return ctrl.Result{}, nil
}

func (r *NvidiaCarbideClusterReconciler) reconcileVPC(
	ctx context.Context, clusterScope *scope.ClusterScope, siteID string,
) error {
	logger := log.FromContext(ctx)

	// Check if VPC already exists
	if clusterScope.VPCID() != "" {
		// Verify VPC still exists in NVIDIA Carbide
		vpc, _, err := clusterScope.NvidiaCarbideClient.GetVpc(ctx, clusterScope.OrgName, clusterScope.VPCID())
		if err != nil {
			logger.Error(err, "VPC not found in NVIDIA Carbide, will recreate", "vpcID", clusterScope.VPCID())
			clusterScope.SetVPCID("")
		} else if vpc != nil {
			logger.V(1).Info("VPC already exists", "vpcID", clusterScope.VPCID())
			return nil
		} else {
			logger.Info("VPC not found, will recreate", "vpcID", clusterScope.VPCID())
			clusterScope.SetVPCID("")
		}
	}

	// Create VPC
	vpcSpec := clusterScope.NvidiaCarbideCluster.Spec.VPC

	vpcReq := bmm.VpcCreateRequest{
		Name:   vpcSpec.Name,
		SiteId: siteID,
	}
	if vpcSpec.NetworkVirtualizationType != "" {
		netVirtType := vpcSpec.NetworkVirtualizationType
		vpcReq.NetworkVirtualizationType = *bmm.NewNullableString(&netVirtType)
	}
	if len(vpcSpec.Labels) > 0 {
		vpcReq.Labels = vpcSpec.Labels
	}

	logger.Info("Creating VPC", "name", vpcSpec.Name, "siteID", siteID)
	vpc, httpResp, err := clusterScope.NvidiaCarbideClient.CreateVpc(ctx, clusterScope.OrgName, vpcReq)
	if err != nil {
		return fmt.Errorf("failed to create VPC: %w", err)
	}

	if httpResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create VPC, status %d", httpResp.StatusCode)
	}

	if vpc == nil || vpc.Id == nil {
		return fmt.Errorf("VPC ID missing in response")
	}

	clusterScope.SetVPCID(*vpc.Id)
	logger.Info("Successfully created VPC", "vpcID", *vpc.Id)
	r.recordEvent(clusterScope.NvidiaCarbideCluster, corev1.EventTypeNormal, "VPCCreated",
		"Successfully created VPC %s", *vpc.Id)
	carbidemetrics.VPCCount.WithLabelValues(siteID).Inc()

	return nil
}

// parseCIDR parses a CIDR string and returns the network address and prefix length
func parseCIDR(cidr string) (network string, prefixLength int, err error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid CIDR %s: %w", cidr, err)
	}

	// Get prefix length
	ones, _ := ipNet.Mask.Size()

	// Return the network address (not the host part)
	networkAddr := ipNet.IP.String()

	return networkAddr, ones, nil
}

// ensureIPBlockAndAllocation ensures an IP block and allocation exist for subnet allocation.
// The allocation creates a child IP block owned by the tenant, which must be used for subnets.
// Returns the child IP block ID.
func (r *NvidiaCarbideClusterReconciler) ensureIPBlockAndAllocation(
	ctx context.Context, clusterScope *scope.ClusterScope, siteID string,
) (string, error) {
	logger := log.FromContext(ctx)

	// If we already have a child IP block ID, verify it exists
	if clusterScope.ChildIPBlockID() != "" {
		ipBlock, _, err := clusterScope.NvidiaCarbideClient.GetIpblock(ctx, clusterScope.OrgName, clusterScope.ChildIPBlockID())
		if err == nil && ipBlock != nil {
			logger.V(1).Info("Child IP block already exists", "childIPBlockID", clusterScope.ChildIPBlockID())
			return clusterScope.ChildIPBlockID(), nil
		}
		logger.Info("Existing child IP block not found, will recreate", "oldChildIPBlockID", clusterScope.ChildIPBlockID())
		clusterScope.SetChildIPBlockID("")
		clusterScope.SetAllocationID("")
	}

	// Step 1: Create parent IP block if needed
	parentIPBlockID := clusterScope.IPBlockID()
	if parentIPBlockID == "" {
		ipBlockName := fmt.Sprintf("%s-ipblock", clusterScope.NvidiaCarbideCluster.Name)
		ipBlockReq := bmm.IpBlockCreateRequest{
			Name:            ipBlockName,
			Prefix:          "10.0.0.0",
			PrefixLength:    16,
			ProtocolVersion: "IPv4",
			RoutingType:     "DatacenterOnly",
			SiteId:          siteID,
		}

		logger.Info("Creating IP block", "name", ipBlockName, "prefix", "10.0.0.0/16", "siteID", siteID)
		ipBlock, httpResp, err := clusterScope.NvidiaCarbideClient.CreateIpblock(ctx, clusterScope.OrgName, ipBlockReq)
		if err != nil {
			return "", fmt.Errorf("failed to create IP block: %w", err)
		}
		if httpResp.StatusCode != http.StatusCreated {
			return "", fmt.Errorf("failed to create IP block, status %d", httpResp.StatusCode)
		}
		if ipBlock == nil || ipBlock.Id == nil {
			return "", fmt.Errorf("IP block ID missing in response")
		}

		parentIPBlockID = *ipBlock.Id
		clusterScope.SetIPBlockID(parentIPBlockID)
		logger.Info("Successfully created IP block", "ipBlockID", parentIPBlockID)
	}

	// Step 2: Create allocation to link tenant to IP block (creates child IP block)
	if clusterScope.AllocationID() == "" {
		allocName := fmt.Sprintf("%s-allocation", clusterScope.NvidiaCarbideCluster.Name)
		resourceType := "IPBlock"
		allocReq := bmm.AllocationCreateRequest{
			Name:     allocName,
			TenantId: clusterScope.TenantID(),
			SiteId:   siteID,
			AllocationConstraints: []bmm.AllocationConstraintCreateRequest{
				{
					ResourceType:    &resourceType,
					ResourceTypeId:  parentIPBlockID,
					ConstraintType:  "OnDemand",
					ConstraintValue: 24,
				},
			},
		}

		logger.Info("Creating allocation", "name", allocName, "tenantID", clusterScope.TenantID(), "siteID", siteID)
		alloc, httpResp, err := clusterScope.NvidiaCarbideClient.CreateAllocation(ctx, clusterScope.OrgName, allocReq)

		// The SDK may fail to deserialize the response (e.g., unrecognized status enum)
		// even when the allocation was created successfully. Check the HTTP status first.
		if httpResp != nil && httpResp.StatusCode == http.StatusCreated {
			// Allocation created — extract IDs if available
			if alloc != nil && alloc.Id != nil {
				clusterScope.SetAllocationID(*alloc.Id)
				logger.Info("Successfully created allocation", "allocationID", *alloc.Id)
				r.extractChildIPBlockID(clusterScope, alloc)
			} else if err != nil {
				// SDK deserialization failed but allocation was created.
				// We cannot recover without knowing the allocation ID.
				logger.Info("Allocation created (201) but SDK deserialization failed, will retry to get IDs", "error", err)
				return "", fmt.Errorf("allocation created but response parsing failed, will retry: %w", err)
			}
		} else if httpResp != nil && httpResp.StatusCode == http.StatusConflict {
			// 409 Conflict — allocation already exists from a previous attempt.
			// Query existing allocations to find the matching one.
			logger.Info("Allocation already exists (409 Conflict), querying existing allocations")
			if foundAlloc, err := r.findExistingAllocation(ctx, clusterScope, allocName); err != nil {
				return "", fmt.Errorf("failed to find existing allocation: %w", err)
			} else if foundAlloc != nil && foundAlloc.Id != nil {
				clusterScope.SetAllocationID(*foundAlloc.Id)
				r.extractChildIPBlockID(clusterScope, foundAlloc)
				logger.Info("Found existing allocation", "allocationID", *foundAlloc.Id)
			} else {
				return "", fmt.Errorf("allocation conflict but could not find existing allocation")
			}
		} else if err != nil {
			return "", fmt.Errorf("failed to create allocation: %w", err)
		}
	}

	// If we have an allocation ID but no child IP block ID, query the allocation
	if clusterScope.AllocationID() != "" && clusterScope.ChildIPBlockID() == "" {
		alloc, _, err := clusterScope.NvidiaCarbideClient.GetAllocation(ctx, clusterScope.OrgName, clusterScope.AllocationID())
		if err != nil {
			return "", fmt.Errorf("failed to get allocation %s: %w", clusterScope.AllocationID(), err)
		}
		if alloc != nil {
			r.extractChildIPBlockID(clusterScope, alloc)
		}
	}

	childIPBlockID := clusterScope.ChildIPBlockID()
	if childIPBlockID == "" {
		return "", fmt.Errorf("child IP block ID not found after allocation creation")
	}

	return childIPBlockID, nil
}

// extractChildIPBlockID extracts the child IP block ID from an allocation's constraints.
func (r *NvidiaCarbideClusterReconciler) extractChildIPBlockID(
	clusterScope *scope.ClusterScope, alloc *bmm.Allocation,
) {
	for _, ac := range alloc.AllocationConstraints {
		if ac.ResourceType != nil && *ac.ResourceType == "IPBlock" {
			if derivedID := ac.DerivedResourceId.Get(); derivedID != nil {
				clusterScope.SetChildIPBlockID(*derivedID)
				break
			}
		}
	}
}

// findExistingAllocation queries all allocations and returns the one matching the given name.
func (r *NvidiaCarbideClusterReconciler) findExistingAllocation(
	ctx context.Context, clusterScope *scope.ClusterScope, name string,
) (*bmm.Allocation, error) {
	// The GetAllAllocation SDK method doesn't support name filtering directly,
	// so we fetch all and filter client-side.
	allocations, _, err := clusterScope.NvidiaCarbideClient.GetAllAllocation(ctx, clusterScope.OrgName)
	if err != nil {
		return nil, err
	}
	for i := range allocations {
		if allocations[i].Name != nil && *allocations[i].Name == name {
			return &allocations[i], nil
		}
	}
	return nil, nil
}

func (r *NvidiaCarbideClusterReconciler) reconcileSubnets(
	ctx context.Context, clusterScope *scope.ClusterScope, siteID string,
) error {
	logger := log.FromContext(ctx)

	vpcID := clusterScope.VPCID()
	if vpcID == "" {
		return fmt.Errorf("VPC ID is empty")
	}

	// Ensure IP block and allocation exist (creates child IP block for tenant)
	childIPBlockID, err := r.ensureIPBlockAndAllocation(ctx, clusterScope, siteID)
	if err != nil {
		return fmt.Errorf("failed to ensure IP block and allocation: %w", err)
	}

	subnetIDs := clusterScope.SubnetIDs()

	// Reconcile each subnet
	for _, subnetSpec := range clusterScope.NvidiaCarbideCluster.Spec.Subnets {
		// Check if subnet already exists
		if existingID, exists := subnetIDs[subnetSpec.Name]; exists {
			// Verify subnet still exists in NVIDIA Carbide
			subnet, _, err := clusterScope.NvidiaCarbideClient.GetSubnet(ctx, clusterScope.OrgName, existingID)
			if err != nil || subnet == nil {
				logger.Error(err, "Subnet not found in NVIDIA Carbide, will recreate",
					"subnetName", subnetSpec.Name, "subnetID", existingID)
				delete(subnetIDs, subnetSpec.Name)
			} else {
				logger.V(1).Info("Subnet already exists", "subnetName", subnetSpec.Name, "subnetID", existingID)
				continue
			}
		}

		// Parse CIDR to get prefix length
		_, prefixLength, err := parseCIDR(subnetSpec.CIDR)
		if err != nil {
			return fmt.Errorf("failed to parse CIDR for subnet %s: %w", subnetSpec.Name, err)
		}

		// Create subnet using child IP block (tenant-owned, from allocation)
		subnetReq := bmm.SubnetCreateRequest{
			Name:         subnetSpec.Name,
			VpcId:        vpcID,
			Ipv4BlockId:  &childIPBlockID,
			PrefixLength: int32(prefixLength),
		}

		logger.Info("Creating subnet",
			"name", subnetSpec.Name, "cidr", subnetSpec.CIDR,
			"prefixLength", prefixLength, "vpcID", vpcID,
			"childIPBlockID", childIPBlockID)
		subnet, httpResp, err := clusterScope.NvidiaCarbideClient.CreateSubnet(ctx, clusterScope.OrgName, subnetReq)
		if err != nil {
			return fmt.Errorf("failed to create subnet %s: %w", subnetSpec.Name, err)
		}

		if httpResp.StatusCode != http.StatusCreated {
			return fmt.Errorf("failed to create subnet %s, status %d", subnetSpec.Name, httpResp.StatusCode)
		}

		if subnet == nil || subnet.Id == nil {
			return fmt.Errorf("subnet ID missing in response for %s", subnetSpec.Name)
		}

		clusterScope.SetSubnetID(subnetSpec.Name, *subnet.Id)
		logger.Info("Successfully created subnet", "subnetName", subnetSpec.Name, "subnetID", *subnet.Id)
		r.recordEvent(clusterScope.NvidiaCarbideCluster, corev1.EventTypeNormal, "SubnetCreated",
			"Successfully created subnet %s (%s)", subnetSpec.Name, *subnet.Id)
	}

	return nil
}

func (r *NvidiaCarbideClusterReconciler) reconcileNSG(
	ctx context.Context, clusterScope *scope.ClusterScope, siteID string,
) error {
	logger := log.FromContext(ctx)

	nsgSpec := clusterScope.NvidiaCarbideCluster.Spec.VPC.NetworkSecurityGroup

	// Check if NSG already exists
	if clusterScope.NSGID() != "" {
		// Verify NSG still exists in NVIDIA Carbide
		nsg, _, err := clusterScope.NvidiaCarbideClient.GetNetworkSecurityGroup(
			ctx, clusterScope.OrgName, clusterScope.NSGID())
		if err != nil || nsg == nil {
			logger.Error(err, "NSG not found in NVIDIA Carbide, will recreate", "nsgID", clusterScope.NSGID())
			clusterScope.SetNSGID("")
		} else {
			logger.V(1).Info("NSG already exists", "nsgID", clusterScope.NSGID())
			return nil
		}
	}

	// Convert NSG rules from CRD types to API types
	rules := make([]bmm.NetworkSecurityGroupRule, 0, len(nsgSpec.Rules))
	for _, rule := range nsgSpec.Rules {
		// API requires both source and destination prefixes
		// Use "0.0.0.0/0" as default (any) if not specified
		sourcePrefix := rule.SourceCIDR
		if sourcePrefix == "" {
			sourcePrefix = "0.0.0.0/0"
		}
		destPrefix := "0.0.0.0/0" // Default to any destination

		ruleName := rule.Name
		nsgRule := bmm.NetworkSecurityGroupRule{
			Name:              *bmm.NewNullableString(&ruleName),
			Direction:         strings.ToLower(rule.Direction),
			Protocol:          strings.ToLower(rule.Protocol),
			Action:            strings.ToLower(rule.Action),
			SourcePrefix:      sourcePrefix,
			DestinationPrefix: destPrefix,
		}

		// Map port range to destination port range
		if rule.PortRange != "" {
			portRange := rule.PortRange
			nsgRule.DestinationPortRange = *bmm.NewNullableString(&portRange)
		}

		rules = append(rules, nsgRule)
	}

	// Create NSG
	nsgReq := bmm.NetworkSecurityGroupCreateRequest{
		Name:   nsgSpec.Name,
		SiteId: siteID,
	}
	if len(rules) > 0 {
		nsgReq.Rules = rules
	}

	logger.Info("Creating NSG", "name", nsgSpec.Name, "siteID", siteID)
	nsg, httpResp, err := clusterScope.NvidiaCarbideClient.CreateNetworkSecurityGroup(ctx, clusterScope.OrgName, nsgReq)
	if err != nil {
		return fmt.Errorf("failed to create NSG: %w", err)
	}

	if httpResp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create NSG, status %d", httpResp.StatusCode)
	}

	if nsg == nil || nsg.Id == nil {
		return fmt.Errorf("NSG ID missing in response")
	}

	clusterScope.SetNSGID(*nsg.Id)
	logger.Info("Successfully created NSG", "nsgID", *nsg.Id)
	r.recordEvent(clusterScope.NvidiaCarbideCluster, corev1.EventTypeNormal, "NSGCreated",
		"Successfully created NSG %s", *nsg.Id)

	return nil
}

//nolint:unparam // ctrl.Result is part of the reconciler interface contract
func (r *NvidiaCarbideClusterReconciler) reconcileDelete(
	ctx context.Context, clusterScope *scope.ClusterScope,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deleting NvidiaCarbideCluster")

	// Delete NSG if it exists
	if clusterScope.NSGID() != "" {
		logger.Info("Deleting NSG", "nsgID", clusterScope.NSGID())
		if err := r.deleteResource(ctx, clusterScope, "NSG", clusterScope.NSGID(),
			clusterScope.NvidiaCarbideClient.DeleteNetworkSecurityGroup); err != nil {
			return ctrl.Result{}, err
		}
		clusterScope.SetNSGID("")
	}

	// Delete Subnets
	for subnetName, subnetID := range clusterScope.SubnetIDs() {
		logger.Info("Deleting subnet", "subnetName", subnetName, "subnetID", subnetID)
		if err := r.deleteResource(ctx, clusterScope, "subnet", subnetID,
			clusterScope.NvidiaCarbideClient.DeleteSubnet); err != nil {
			return ctrl.Result{}, err
		}
		delete(clusterScope.SubnetIDs(), subnetName)
	}

	// Delete Allocation if it exists
	if clusterScope.AllocationID() != "" {
		logger.Info("Deleting allocation", "allocationID", clusterScope.AllocationID())
		if err := r.deleteResource(ctx, clusterScope, "allocation", clusterScope.AllocationID(),
			clusterScope.NvidiaCarbideClient.DeleteAllocation); err != nil {
			return ctrl.Result{}, err
		}
		clusterScope.SetAllocationID("")
	}

	// Delete child IP block if it exists
	if clusterScope.ChildIPBlockID() != "" {
		logger.Info("Deleting child IP block", "childIPBlockID", clusterScope.ChildIPBlockID())
		if err := r.deleteResource(ctx, clusterScope, "child IP block", clusterScope.ChildIPBlockID(),
			clusterScope.NvidiaCarbideClient.DeleteIpblock); err != nil {
			return ctrl.Result{}, err
		}
		clusterScope.SetChildIPBlockID("")
	}

	// Delete parent IP block if it exists
	if clusterScope.IPBlockID() != "" {
		logger.Info("Deleting parent IP block", "ipBlockID", clusterScope.IPBlockID())
		if err := r.deleteResource(ctx, clusterScope, "parent IP block", clusterScope.IPBlockID(),
			clusterScope.NvidiaCarbideClient.DeleteIpblock); err != nil {
			return ctrl.Result{}, err
		}
		clusterScope.SetIPBlockID("")
	}

	// Delete VPC
	if clusterScope.VPCID() != "" {
		logger.Info("Deleting VPC", "vpcID", clusterScope.VPCID())
		if err := r.deleteResource(ctx, clusterScope, "VPC", clusterScope.VPCID(),
			clusterScope.NvidiaCarbideClient.DeleteVpc); err != nil {
			return ctrl.Result{}, err
		}
		clusterScope.SetVPCID("")
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(clusterScope.NvidiaCarbideCluster, NvidiaCarbideClusterFinalizer)

	logger.Info("Successfully deleted NvidiaCarbideCluster")
	return ctrl.Result{}, nil
}

// deleteResource calls a delete API method and handles 404 (already deleted) gracefully.
func (r *NvidiaCarbideClusterReconciler) deleteResource(
	ctx context.Context, clusterScope *scope.ClusterScope,
	resourceType, resourceID string,
	deleteFn func(ctx context.Context, org string, id string) (*http.Response, error),
) error {
	logger := log.FromContext(ctx)

	httpResp, err := deleteFn(ctx, clusterScope.OrgName, resourceID)
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			logger.Info("Resource already deleted", "type", resourceType, "id", resourceID)
			return nil
		}
		return fmt.Errorf("failed to delete %s %s: %w", resourceType, resourceID, err)
	}
	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNoContent &&
		httpResp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("failed to delete %s %s, status %d", resourceType, resourceID, httpResp.StatusCode)
	}
	return nil
}

// recordEvent records an event on the given object if a Recorder is set.
func (r *NvidiaCarbideClusterReconciler) recordEvent(obj runtime.Object, eventType, reason, messageFmt string, args ...interface{}) {
	if r.Recorder != nil {
		r.Recorder.Eventf(obj, eventType, reason, messageFmt, args...)
	}
}

// classifyError classifies an HTTP response and returns an appropriate requeue result.
func classifyError(httpResp *http.Response, err error) (ctrl.Result, error) {
	if httpResp == nil {
		carbidemetrics.APIErrors.WithLabelValues("unknown", "network_error").Inc()
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	if httpResp.StatusCode >= 400 {
		carbidemetrics.APIErrors.WithLabelValues("unknown", strconv.Itoa(httpResp.StatusCode)).Inc()
	}
	switch httpResp.StatusCode {
	case 400, 422:
		return ctrl.Result{}, err
	case 401, 403:
		return ctrl.Result{}, err
	case 404:
		return ctrl.Result{}, nil
	case 409:
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	case 429:
		retryAfter := parseRetryAfter(httpResp)
		return ctrl.Result{RequeueAfter: retryAfter}, nil
	default:
		if httpResp.StatusCode >= 500 {
			return ctrl.Result{RequeueAfter: 30 * time.Second}, err
		}
		return ctrl.Result{}, err
	}
}

// parseRetryAfter extracts the Retry-After header value from an HTTP response.
func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 30 * time.Second
	}
	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 30 * time.Second
	}
	if seconds, err := strconv.Atoi(retryAfter); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return 30 * time.Second
}

// setClusterFailure sets the FailureReason and FailureMessage on the cluster status.
func setClusterFailure(cluster *infrastructurev1.NvidiaCarbideCluster, reason capierrors.ClusterStatusError, message string) {
	cluster.Status.FailureReason = &reason
	cluster.Status.FailureMessage = &message
}

// SetupWithManager sets up the controller with the Manager.
func (r *NvidiaCarbideClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1.NvidiaCarbideCluster{}).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(
				util.ClusterToInfrastructureMapFunc(
					ctx,
					infrastructurev1.GroupVersion.WithKind("NvidiaCarbideCluster"),
					mgr.GetClient(),
					&infrastructurev1.NvidiaCarbideCluster{},
				),
			),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(
			mgr.GetScheme(), ctrl.Log.WithName("nvidiacarbidecluster"), "")).
		Named("nvidiacarbidecluster").
		Complete(r)
}
