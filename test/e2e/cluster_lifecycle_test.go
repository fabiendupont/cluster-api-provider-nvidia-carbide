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

//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1beta1 "github.com/fabiendupont/cluster-api-provider-nvidia-carbide/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	clusterCreationTimeout = 30 * time.Minute
	clusterDeletionTimeout = 20 * time.Minute
	pollInterval           = 30 * time.Second
)

var _ = Describe("NVIDIA Carbide Cluster Lifecycle E2E", func() {
	var (
		k8sClient     client.Client
		ctx           context.Context
		testNamespace string
		clusterName   string
		siteID        string
		tenantID      string
	)

	BeforeEach(func() {
		ctx = context.Background()

		// Load kubeconfig
		kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)

		config, err := kubeconfig.ClientConfig()
		Expect(err).NotTo(HaveOccurred())

		// Add schemes
		err = infrastructurev1beta1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = clusterv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		// Create client
		k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		// Get environment variables
		siteID = os.Getenv("E2E_SITE_ID")
		tenantID = os.Getenv("E2E_TENANT_ID")

		if siteID == "" || tenantID == "" {
			Skip("E2E_SITE_ID and E2E_TENANT_ID environment variables must be set")
		}

		// Generate unique names
		timestamp := time.Now().Unix()
		testNamespace = "default"
		clusterName = fmt.Sprintf("e2e-cluster-%d", timestamp)
	})

	Context("Full cluster creation and deletion", func() {
		It("should create a complete HA cluster with 3 control plane and 3 worker nodes", func() {
			By("Creating credentials secret")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-creds", clusterName),
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					"endpoint": []byte(os.Getenv("NVIDIA_CARBIDE_API_ENDPOINT")),
					"orgName":  []byte(os.Getenv("NVIDIA_CARBIDE_ORG_NAME")),
					"token":    []byte(os.Getenv("NVIDIA_CARBIDE_API_TOKEN")),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("Creating CAPI Cluster")
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: testNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: clusterv1.ClusterNetwork{
						Pods: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.244.0.0/16"},
						},
						Services: clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.96.0.0/12"},
						},
					},
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup:  "infrastructure.cluster.x-k8s.io",
						Kind:      "NvidiaCarbideCluster",
						Name:      clusterName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("Creating NvidiaCarbideCluster")
			nvidiaCarbideCluster := &infrastructurev1beta1.NvidiaCarbideCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: testNamespace,
				},
				Spec: infrastructurev1beta1.NvidiaCarbideClusterSpec{
					SiteRef: infrastructurev1beta1.SiteReference{
						ID: siteID,
					},
					TenantID: tenantID,
					VPC: infrastructurev1beta1.VPCSpec{
						Name:                      fmt.Sprintf("%s-vpc", clusterName),
						NetworkVirtualizationType: "ETHERNET_VIRTUALIZER",
						Labels: map[string]string{
							"test":    "e2e",
							"cluster": clusterName,
						},
					},
					Subnets: []infrastructurev1beta1.SubnetSpec{
						{
							Name: "control-plane",
							CIDR: "10.100.1.0/24",
							Role: "control-plane",
						},
						{
							Name: "worker",
							CIDR: "10.100.2.0/24",
							Role: "worker",
						},
					},
					Authentication: infrastructurev1beta1.AuthenticationSpec{
						SecretRef: corev1.SecretReference{
							Name:      secret.Name,
							Namespace: testNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, nvidiaCarbideCluster)).To(Succeed())

			By("Waiting for NvidiaCarbideCluster to be ready")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(nvidiaCarbideCluster), nvidiaCarbideCluster)
				if err != nil {
					return false
				}
				return nvidiaCarbideCluster.Status.Ready
			}, clusterCreationTimeout, pollInterval).Should(BeTrue())

			By("Verifying VPC was created")
			Expect(nvidiaCarbideCluster.Status.VPCID).NotTo(BeEmpty())

			By("Verifying subnets were created")
			Expect(nvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs).To(HaveLen(2))
			Expect(nvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs).To(HaveKey("control-plane"))
			Expect(nvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs).To(HaveKey("worker"))

			By("Creating control plane machines")
			for i := 0; i < 3; i++ {
				machineName := fmt.Sprintf("%s-cp-%d", clusterName, i)

				machine := &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
						Namespace: testNamespace,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:         clusterName,
							clusterv1.MachineControlPlaneLabel: "",
						},
					},
					Spec: clusterv1.MachineSpec{
						ClusterName: clusterName,
						Version:     "v1.28.0",
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To(fmt.Sprintf("%s-bootstrap", machineName)),
						},
						InfrastructureRef: clusterv1.ContractVersionedObjectReference{
							APIGroup:  "infrastructure.cluster.x-k8s.io",
							Kind:      "NvidiaCarbideMachine",
							Name:      machineName,
						},
					},
				}
				Expect(k8sClient.Create(ctx, machine)).To(Succeed())

				// Create bootstrap secret
				bootstrapSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-bootstrap", machineName),
						Namespace: testNamespace,
					},
					Data: map[string][]byte{
						"value": []byte("#!/bin/bash\nkubeadm init..."),
					},
				}
				Expect(k8sClient.Create(ctx, bootstrapSecret)).To(Succeed())

				nvidiaCarbideMachine := &infrastructurev1beta1.NvidiaCarbideMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
						Namespace: testNamespace,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel:         clusterName,
							clusterv1.MachineControlPlaneLabel: "",
						},
					},
					Spec: infrastructurev1beta1.NvidiaCarbideMachineSpec{
						InstanceType: infrastructurev1beta1.InstanceTypeSpec{
							ID: os.Getenv("E2E_INSTANCE_TYPE_ID"),
						},
						Network: infrastructurev1beta1.NetworkSpec{
							SubnetName: "control-plane",
						},
						SSHKeyGroups: []string{os.Getenv("E2E_SSH_KEY_GROUP_ID")},
						Labels: map[string]string{
							"role":    "control-plane",
							"cluster": clusterName,
						},
					},
				}
				Expect(k8sClient.Create(ctx, nvidiaCarbideMachine)).To(Succeed())
			}

			By("Creating worker machines")
			for i := 0; i < 3; i++ {
				machineName := fmt.Sprintf("%s-worker-%d", clusterName, i)

				machine := &clusterv1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
						Namespace: testNamespace,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel: clusterName,
						},
					},
					Spec: clusterv1.MachineSpec{
						ClusterName: clusterName,
						Version:     "v1.28.0",
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To(fmt.Sprintf("%s-bootstrap", machineName)),
						},
						InfrastructureRef: clusterv1.ContractVersionedObjectReference{
							APIGroup:  "infrastructure.cluster.x-k8s.io",
							Kind:      "NvidiaCarbideMachine",
							Name:      machineName,
						},
					},
				}
				Expect(k8sClient.Create(ctx, machine)).To(Succeed())

				// Create bootstrap secret
				bootstrapSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-bootstrap", machineName),
						Namespace: testNamespace,
					},
					Data: map[string][]byte{
						"value": []byte("#!/bin/bash\nkubeadm join..."),
					},
				}
				Expect(k8sClient.Create(ctx, bootstrapSecret)).To(Succeed())

				nvidiaCarbideMachine := &infrastructurev1beta1.NvidiaCarbideMachine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      machineName,
						Namespace: testNamespace,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel: clusterName,
						},
					},
					Spec: infrastructurev1beta1.NvidiaCarbideMachineSpec{
						InstanceType: infrastructurev1beta1.InstanceTypeSpec{
							ID: os.Getenv("E2E_INSTANCE_TYPE_ID"),
						},
						Network: infrastructurev1beta1.NetworkSpec{
							SubnetName: "worker",
						},
						SSHKeyGroups: []string{os.Getenv("E2E_SSH_KEY_GROUP_ID")},
						Labels: map[string]string{
							"role":    "worker",
							"cluster": clusterName,
						},
					},
				}
				Expect(k8sClient.Create(ctx, nvidiaCarbideMachine)).To(Succeed())
			}

			By("Waiting for all machines to be provisioned")
			Eventually(func() int {
				machineList := &infrastructurev1beta1.NvidiaCarbideMachineList{}
				err := k8sClient.List(ctx, machineList, client.InNamespace(testNamespace),
					client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName})
				if err != nil {
					return 0
				}

				provisionedCount := 0
				for _, machine := range machineList.Items {
					if machine.Status.InstanceID != "" {
						provisionedCount++
					}
				}
				return provisionedCount
			}, clusterCreationTimeout, pollInterval).Should(Equal(6))

			By("Waiting for all machines to be ready")
			Eventually(func() int {
				machineList := &infrastructurev1beta1.NvidiaCarbideMachineList{}
				err := k8sClient.List(ctx, machineList, client.InNamespace(testNamespace),
					client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName})
				if err != nil {
					return 0
				}

				readyCount := 0
				for _, machine := range machineList.Items {
					if machine.Status.Ready {
						readyCount++
					}
				}
				return readyCount
			}, clusterCreationTimeout, pollInterval).Should(Equal(6))

			By("Deleting the cluster")
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())

			By("Waiting for all machines to be deleted")
			Eventually(func() int {
				machineList := &infrastructurev1beta1.NvidiaCarbideMachineList{}
				err := k8sClient.List(ctx, machineList, client.InNamespace(testNamespace),
					client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName})
				if err != nil {
					return -1
				}
				return len(machineList.Items)
			}, clusterDeletionTimeout, pollInterval).Should(Equal(0))

			By("Waiting for NvidiaCarbideCluster to be deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(nvidiaCarbideCluster), nvidiaCarbideCluster)
				return err != nil
			}, clusterDeletionTimeout, pollInterval).Should(BeTrue())

			By("Cleaning up credentials secret")
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
		})
	})
})
