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
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrastructurev1 "github.com/fabiendupont/cluster-api-provider-nvidia-carbide/api/v1beta1"
	"github.com/fabiendupont/cluster-api-provider-nvidia-carbide/internal/controller/testutil"
	"github.com/fabiendupont/cluster-api-provider-nvidia-carbide/pkg/scope"
	bmm "github.com/nvidia/bare-metal-manager-rest/sdk/standard"
)

func newTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = infrastructurev1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	return scheme
}

var _ = Describe("NvidiaCarbideCluster Controller", func() {
	const (
		clusterName      = "test-cluster"
		clusterNamespace = "default"
		orgName          = "test-org"
		siteID           = "550e8400-e29b-41d4-a716-446655440000"
		tenantID         = "660e8400-e29b-41d4-a716-446655440001"
	)

	var (
		ctx                  context.Context
		scheme               *runtime.Scheme
		cluster              *clusterv1.Cluster
		nvidiaCarbideCluster *infrastructurev1.NvidiaCarbideCluster
		credsSecret          *corev1.Secret
		namespacedName       types.NamespacedName
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = newTestScheme()
		namespacedName = types.NamespacedName{
			Name:      clusterName,
			Namespace: clusterNamespace,
		}

		cluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: clusterNamespace,
				UID:       "cluster-uid",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: clusterv1.ContractVersionedObjectReference{
					APIGroup: "infrastructure.cluster.x-k8s.io",
					Kind:     "NvidiaCarbideCluster",
					Name:     clusterName,
				},
			},
		}

		nvidiaCarbideCluster = &infrastructurev1.NvidiaCarbideCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: clusterNamespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "cluster.x-k8s.io/v1beta2",
						Kind:       "Cluster",
						Name:       clusterName,
						UID:        "cluster-uid",
					},
				},
			},
			Spec: infrastructurev1.NvidiaCarbideClusterSpec{
				SiteRef: infrastructurev1.SiteReference{
					ID: siteID,
				},
				TenantID: tenantID,
				VPC: infrastructurev1.VPCSpec{
					Name:                      "test-vpc",
					NetworkVirtualizationType: "ETHERNET_VIRTUALIZER",
				},
				Subnets: []infrastructurev1.SubnetSpec{
					{
						Name: "control-plane",
						CIDR: "10.0.1.0/24",
						Role: "control-plane",
					},
				},
				Authentication: infrastructurev1.AuthenticationSpec{
					SecretRef: corev1.SecretReference{
						Name:      "nvidia-carbide-creds",
						Namespace: clusterNamespace,
					},
				},
			},
		}

		credsSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nvidia-carbide-creds",
				Namespace: clusterNamespace,
			},
			Data: map[string][]byte{
				"endpoint": []byte("https://api.carbide.test"),
				"orgName":  []byte(orgName),
				"token":    []byte("test-token"),
			},
		}
	})

	Context("When reconciling a new NvidiaCarbideCluster", func() {
		It("should add finalizer on first reconcile", func() {
			vpcID := uuid.New().String()
			ipBlockID := uuid.New().String()
			allocationID := uuid.New().String()
			childIPBlockID := uuid.New().String()
			subnetID := uuid.New().String()

			mockClient := &testutil.MockCarbideClient{
				CreateVPCFunc: func(ctx context.Context, org string, req bmm.VpcCreateRequest) (*bmm.VPC, *http.Response, error) {
					return &bmm.VPC{Id: &vpcID}, testutil.MockHTTPResponse(201), nil
				},
				GetVPCFunc: func(ctx context.Context, org, id string) (*bmm.VPC, *http.Response, error) {
					return nil, nil, fmt.Errorf("not found")
				},
				CreateIpblockFunc: func(ctx context.Context, org string, req bmm.IpBlockCreateRequest) (*bmm.IpBlock, *http.Response, error) {
					return &bmm.IpBlock{Id: &ipBlockID}, testutil.MockHTTPResponse(201), nil
				},
				GetIpblockFunc: func(ctx context.Context, org, id string) (*bmm.IpBlock, *http.Response, error) {
					return nil, nil, fmt.Errorf("not found")
				},
				CreateAllocationFunc: func(ctx context.Context, org string, req bmm.AllocationCreateRequest) (*bmm.Allocation, *http.Response, error) {
					resourceType := "IPBlock"
					return &bmm.Allocation{
						Id:   &allocationID,
						Name: testutil.Ptr("test-cluster-allocation"),
						AllocationConstraints: []bmm.AllocationConstraint{
							{
								ResourceType:      &resourceType,
								DerivedResourceId: *bmm.NewNullableString(&childIPBlockID),
							},
						},
					}, testutil.MockHTTPResponse(201), nil
				},
				CreateSubnetFunc: func(ctx context.Context, org string, req bmm.SubnetCreateRequest) (*bmm.Subnet, *http.Response, error) {
					return &bmm.Subnet{Id: &subnetID}, testutil.MockHTTPResponse(201), nil
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster, nvidiaCarbideCluster, credsSecret).
				WithStatusSubresource(&infrastructurev1.NvidiaCarbideCluster{}).
				Build()

			reconciler := &NvidiaCarbideClusterReconciler{
				Client:              k8sClient,
				Scheme:              scheme,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
			}

			// First reconcile — should add finalizer
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())

			// Verify finalizer was added
			updatedCluster := &infrastructurev1.NvidiaCarbideCluster{}
			Expect(k8sClient.Get(ctx, namespacedName, updatedCluster)).To(Succeed())
			Expect(updatedCluster.Finalizers).To(ContainElement(NvidiaCarbideClusterFinalizer))
		})

		It("should create VPC, IP block, allocation, and subnets on reconcile", func() {
			vpcID := uuid.New().String()
			ipBlockID := uuid.New().String()
			allocationID := uuid.New().String()
			childIPBlockID := uuid.New().String()
			subnetID := uuid.New().String()

			createVPCCalled := false
			createSubnetCalled := false

			mockClient := &testutil.MockCarbideClient{
				CreateVPCFunc: func(ctx context.Context, org string, req bmm.VpcCreateRequest) (*bmm.VPC, *http.Response, error) {
					createVPCCalled = true
					Expect(org).To(Equal(orgName))
					Expect(req.Name).To(Equal("test-vpc"))
					Expect(req.SiteId).To(Equal(siteID))
					return &bmm.VPC{Id: &vpcID, Name: testutil.Ptr("test-vpc")}, testutil.MockHTTPResponse(201), nil
				},
				GetVPCFunc: func(ctx context.Context, org, id string) (*bmm.VPC, *http.Response, error) {
					return nil, nil, fmt.Errorf("not found")
				},
				CreateIpblockFunc: func(ctx context.Context, org string, req bmm.IpBlockCreateRequest) (*bmm.IpBlock, *http.Response, error) {
					Expect(req.Prefix).To(Equal("10.0.0.0"))
					Expect(req.PrefixLength).To(Equal(int32(16)))
					return &bmm.IpBlock{Id: &ipBlockID}, testutil.MockHTTPResponse(201), nil
				},
				GetIpblockFunc: func(ctx context.Context, org, id string) (*bmm.IpBlock, *http.Response, error) {
					return nil, nil, fmt.Errorf("not found")
				},
				CreateAllocationFunc: func(ctx context.Context, org string, req bmm.AllocationCreateRequest) (*bmm.Allocation, *http.Response, error) {
					Expect(req.TenantId).To(Equal(tenantID))
					resourceType := "IPBlock"
					return &bmm.Allocation{
						Id: &allocationID,
						AllocationConstraints: []bmm.AllocationConstraint{
							{
								ResourceType:      &resourceType,
								DerivedResourceId: *bmm.NewNullableString(&childIPBlockID),
							},
						},
					}, testutil.MockHTTPResponse(201), nil
				},
				CreateSubnetFunc: func(ctx context.Context, org string, req bmm.SubnetCreateRequest) (*bmm.Subnet, *http.Response, error) {
					createSubnetCalled = true
					Expect(req.Name).To(Equal("control-plane"))
					Expect(req.VpcId).To(Equal(vpcID))
					Expect(*req.Ipv4BlockId).To(Equal(childIPBlockID))
					return &bmm.Subnet{Id: &subnetID}, testutil.MockHTTPResponse(201), nil
				},
			}

			// Pre-add finalizer to skip the first reconcile
			nvidiaCarbideCluster.Finalizers = []string{NvidiaCarbideClusterFinalizer}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster, nvidiaCarbideCluster, credsSecret).
				WithStatusSubresource(&infrastructurev1.NvidiaCarbideCluster{}).
				Build()

			reconciler := &NvidiaCarbideClusterReconciler{
				Client:              k8sClient,
				Scheme:              scheme,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(createVPCCalled).To(BeTrue())
			Expect(createSubnetCalled).To(BeTrue())

			// Verify status was updated
			updatedCluster := &infrastructurev1.NvidiaCarbideCluster{}
			Expect(k8sClient.Get(ctx, namespacedName, updatedCluster)).To(Succeed())
			Expect(updatedCluster.Status.Ready).To(BeTrue())
			Expect(updatedCluster.Status.VPCID).To(Equal(vpcID))
			Expect(updatedCluster.Status.NetworkStatus.IPBlockID).To(Equal(ipBlockID))
			Expect(updatedCluster.Status.NetworkStatus.AllocationID).To(Equal(allocationID))
			Expect(updatedCluster.Status.NetworkStatus.ChildIPBlockID).To(Equal(childIPBlockID))
			Expect(updatedCluster.Status.NetworkStatus.SubnetIDs).To(HaveKeyWithValue("control-plane", subnetID))
		})
	})

	Context("When VPC creation fails", func() {
		It("should return error on 500 response", func() {
			mockClient := &testutil.MockCarbideClient{
				GetIpblockFunc: func(ctx context.Context, org, id string) (*bmm.IpBlock, *http.Response, error) {
					return nil, nil, fmt.Errorf("not found")
				},
				CreateIpblockFunc: func(ctx context.Context, org string, req bmm.IpBlockCreateRequest) (*bmm.IpBlock, *http.Response, error) {
					id := uuid.New().String()
					return &bmm.IpBlock{Id: &id}, testutil.MockHTTPResponse(201), nil
				},
				CreateAllocationFunc: func(ctx context.Context, org string, req bmm.AllocationCreateRequest) (*bmm.Allocation, *http.Response, error) {
					allocID := uuid.New().String()
					childID := uuid.New().String()
					resourceType := "IPBlock"
					return &bmm.Allocation{
						Id: &allocID,
						AllocationConstraints: []bmm.AllocationConstraint{
							{ResourceType: &resourceType, DerivedResourceId: *bmm.NewNullableString(&childID)},
						},
					}, testutil.MockHTTPResponse(201), nil
				},
				CreateVPCFunc: func(ctx context.Context, org string, req bmm.VpcCreateRequest) (*bmm.VPC, *http.Response, error) {
					return nil, testutil.MockHTTPResponse(500), fmt.Errorf("internal server error")
				},
			}

			nvidiaCarbideCluster.Finalizers = []string{NvidiaCarbideClusterFinalizer}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster, nvidiaCarbideCluster, credsSecret).
				WithStatusSubresource(&infrastructurev1.NvidiaCarbideCluster{}).
				Build()

			reconciler := &NvidiaCarbideClusterReconciler{
				Client:              k8sClient,
				Scheme:              scheme,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create VPC"))
		})
	})

	Context("When allocation returns 409 Conflict", func() {
		It("should recover by querying existing allocations", func() {
			allocationID := uuid.New().String()
			childIPBlockID := uuid.New().String()
			ipBlockID := uuid.New().String()
			vpcID := uuid.New().String()
			subnetID := uuid.New().String()

			mockClient := &testutil.MockCarbideClient{
				GetIpblockFunc: func(ctx context.Context, org, id string) (*bmm.IpBlock, *http.Response, error) {
					return nil, nil, fmt.Errorf("not found")
				},
				CreateIpblockFunc: func(ctx context.Context, org string, req bmm.IpBlockCreateRequest) (*bmm.IpBlock, *http.Response, error) {
					return &bmm.IpBlock{Id: &ipBlockID}, testutil.MockHTTPResponse(201), nil
				},
				CreateAllocationFunc: func(ctx context.Context, org string, req bmm.AllocationCreateRequest) (*bmm.Allocation, *http.Response, error) {
					return nil, testutil.MockHTTPResponse(409), fmt.Errorf("conflict")
				},
				GetAllAllocationFunc: func(ctx context.Context, org string) ([]bmm.Allocation, *http.Response, error) {
					resourceType := "IPBlock"
					return []bmm.Allocation{
						{
							Id:   &allocationID,
							Name: testutil.Ptr("test-cluster-allocation"),
							AllocationConstraints: []bmm.AllocationConstraint{
								{
									ResourceType:      &resourceType,
									DerivedResourceId: *bmm.NewNullableString(&childIPBlockID),
								},
							},
						},
					}, testutil.MockHTTPResponse(200), nil
				},
				CreateVPCFunc: func(ctx context.Context, org string, req bmm.VpcCreateRequest) (*bmm.VPC, *http.Response, error) {
					return &bmm.VPC{Id: &vpcID}, testutil.MockHTTPResponse(201), nil
				},
				GetVPCFunc: func(ctx context.Context, org, id string) (*bmm.VPC, *http.Response, error) {
					return nil, nil, fmt.Errorf("not found")
				},
				CreateSubnetFunc: func(ctx context.Context, org string, req bmm.SubnetCreateRequest) (*bmm.Subnet, *http.Response, error) {
					return &bmm.Subnet{Id: &subnetID}, testutil.MockHTTPResponse(201), nil
				},
			}

			nvidiaCarbideCluster.Finalizers = []string{NvidiaCarbideClusterFinalizer}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster, nvidiaCarbideCluster, credsSecret).
				WithStatusSubresource(&infrastructurev1.NvidiaCarbideCluster{}).
				Build()

			reconciler := &NvidiaCarbideClusterReconciler{
				Client:              k8sClient,
				Scheme:              scheme,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			updatedCluster := &infrastructurev1.NvidiaCarbideCluster{}
			Expect(k8sClient.Get(ctx, namespacedName, updatedCluster)).To(Succeed())
			Expect(updatedCluster.Status.Ready).To(BeTrue())
			Expect(updatedCluster.Status.NetworkStatus.AllocationID).To(Equal(allocationID))
			Expect(updatedCluster.Status.NetworkStatus.ChildIPBlockID).To(Equal(childIPBlockID))
		})
	})

	Context("When deleting a NvidiaCarbideCluster", func() {
		It("should clean up all resources in correct order", func() {
			vpcID := uuid.New().String()
			subnetID := uuid.New().String()
			nsgID := uuid.New().String()
			allocationID := uuid.New().String()
			childIPBlockID := uuid.New().String()
			parentIPBlockID := uuid.New().String()

			deleteOrder := []string{}

			mockClient := &testutil.MockCarbideClient{
				DeleteNetworkSecurityGroupFunc: func(ctx context.Context, org, id string) (*http.Response, error) {
					Expect(id).To(Equal(nsgID))
					deleteOrder = append(deleteOrder, "nsg")
					return testutil.MockHTTPResponse(200), nil
				},
				DeleteSubnetFunc: func(ctx context.Context, org, id string) (*http.Response, error) {
					Expect(id).To(Equal(subnetID))
					deleteOrder = append(deleteOrder, "subnet")
					return testutil.MockHTTPResponse(200), nil
				},
				DeleteAllocationFunc: func(ctx context.Context, org, id string) (*http.Response, error) {
					Expect(id).To(Equal(allocationID))
					deleteOrder = append(deleteOrder, "allocation")
					return testutil.MockHTTPResponse(200), nil
				},
				DeleteIpblockFunc: func(ctx context.Context, org, id string) (*http.Response, error) {
					if id == childIPBlockID {
						deleteOrder = append(deleteOrder, "child-ipblock")
					} else if id == parentIPBlockID {
						deleteOrder = append(deleteOrder, "parent-ipblock")
					}
					return testutil.MockHTTPResponse(200), nil
				},
				DeleteVPCFunc: func(ctx context.Context, org, id string) (*http.Response, error) {
					Expect(id).To(Equal(vpcID))
					deleteOrder = append(deleteOrder, "vpc")
					return testutil.MockHTTPResponse(200), nil
				},
			}

			// Test reconcileDelete directly via the scope to avoid fake client
			// issues with DeletionTimestamp objects
			clusterScope := &scope.ClusterScope{
				Client:              nil,
				Cluster:             cluster,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
				NvidiaCarbideCluster: &infrastructurev1.NvidiaCarbideCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:       clusterName,
						Namespace:  clusterNamespace,
						Finalizers: []string{NvidiaCarbideClusterFinalizer},
					},
					Status: infrastructurev1.NvidiaCarbideClusterStatus{
						VPCID: vpcID,
						NetworkStatus: infrastructurev1.NetworkStatus{
							SubnetIDs:      map[string]string{"control-plane": subnetID},
							NSGID:          nsgID,
							AllocationID:   allocationID,
							ChildIPBlockID: childIPBlockID,
							IPBlockID:      parentIPBlockID,
						},
					},
				},
			}

			reconciler := &NvidiaCarbideClusterReconciler{
				Scheme:              scheme,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
			}

			result, err := reconciler.reconcileDelete(ctx, clusterScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			// Verify deletion order: NSG → Subnets → Allocation → Child IP Block → Parent IP Block → VPC
			Expect(deleteOrder).To(Equal([]string{
				"nsg", "subnet", "allocation", "child-ipblock", "parent-ipblock", "vpc",
			}))

			// Verify finalizer was removed
			Expect(clusterScope.NvidiaCarbideCluster.Finalizers).NotTo(ContainElement(NvidiaCarbideClusterFinalizer))
		})

		It("should handle 404 gracefully during deletion", func() {
			vpcID := uuid.New().String()

			mockClient := &testutil.MockCarbideClient{
				DeleteVPCFunc: func(ctx context.Context, org, id string) (*http.Response, error) {
					return testutil.MockHTTPResponse(404), fmt.Errorf("not found")
				},
			}

			clusterScope := &scope.ClusterScope{
				Client:              nil,
				Cluster:             cluster,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
				NvidiaCarbideCluster: &infrastructurev1.NvidiaCarbideCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:       clusterName,
						Namespace:  clusterNamespace,
						Finalizers: []string{NvidiaCarbideClusterFinalizer},
					},
					Status: infrastructurev1.NvidiaCarbideClusterStatus{
						VPCID: vpcID,
					},
				},
			}

			reconciler := &NvidiaCarbideClusterReconciler{
				Scheme:              scheme,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
			}

			result, err := reconciler.reconcileDelete(ctx, clusterScope)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	Context("When cluster is paused", func() {
		It("should skip reconciliation", func() {
			paused := true
			cluster.Spec.Paused = &paused

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster, nvidiaCarbideCluster, credsSecret).
				WithStatusSubresource(&infrastructurev1.NvidiaCarbideCluster{}).
				Build()

			reconciler := &NvidiaCarbideClusterReconciler{
				Client: k8sClient,
				Scheme: scheme,
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	Context("When NvidiaCarbideCluster does not exist", func() {
		It("should return without error", func() {
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster).
				Build()

			reconciler := &NvidiaCarbideClusterReconciler{
				Client: k8sClient,
				Scheme: scheme,
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
		})
	})

	Context("When VPC already exists in status", func() {
		It("should skip VPC creation", func() {
			vpcID := uuid.New().String()
			childIPBlockID := uuid.New().String()
			subnetID := uuid.New().String()

			createVPCCalled := false
			mockClient := &testutil.MockCarbideClient{
				CreateVPCFunc: func(ctx context.Context, org string, req bmm.VpcCreateRequest) (*bmm.VPC, *http.Response, error) {
					createVPCCalled = true
					return nil, nil, fmt.Errorf("should not be called")
				},
				GetVPCFunc: func(ctx context.Context, org, id string) (*bmm.VPC, *http.Response, error) {
					return &bmm.VPC{Id: &vpcID}, testutil.MockHTTPResponse(200), nil
				},
				GetIpblockFunc: func(ctx context.Context, org, id string) (*bmm.IpBlock, *http.Response, error) {
					return &bmm.IpBlock{Id: &childIPBlockID}, testutil.MockHTTPResponse(200), nil
				},
				GetSubnetFunc: func(ctx context.Context, org, id string) (*bmm.Subnet, *http.Response, error) {
					return &bmm.Subnet{Id: &subnetID}, testutil.MockHTTPResponse(200), nil
				},
			}

			nvidiaCarbideCluster.Finalizers = []string{NvidiaCarbideClusterFinalizer}
			nvidiaCarbideCluster.Status = infrastructurev1.NvidiaCarbideClusterStatus{
				VPCID: vpcID,
				NetworkStatus: infrastructurev1.NetworkStatus{
					ChildIPBlockID: childIPBlockID,
					SubnetIDs:      map[string]string{"control-plane": subnetID},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(cluster, nvidiaCarbideCluster, credsSecret).
				WithStatusSubresource(&infrastructurev1.NvidiaCarbideCluster{}).
				Build()

			reconciler := &NvidiaCarbideClusterReconciler{
				Client:              k8sClient,
				Scheme:              scheme,
				NvidiaCarbideClient: mockClient,
				OrgName:             orgName,
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: namespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())
			Expect(createVPCCalled).To(BeFalse())
		})
	})
})
