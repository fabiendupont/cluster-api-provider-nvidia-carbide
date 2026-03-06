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

package e2e_live

import (
	"context"
	"fmt"
	"testing"
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

func TestE2ELive(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting cluster-api-provider-nvidia-carbide live e2e test suite\n")
	RunSpecs(t, "Live E2E Suite")
}

var _ = Describe("Live NVIDIA Carbide Cluster E2E", Label("live"), func() {
	var (
		k8sClient     client.Client
		ctx           context.Context
		testNamespace string
		clusterName   string
		token         string
	)

	BeforeEach(func() {
		ctx = context.Background()

		kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)

		config, err := kubeconfig.ClientConfig()
		Expect(err).NotTo(HaveOccurred())

		err = infrastructurev1beta1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		err = clusterv1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		k8sClient, err = client.New(config, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		testNamespace = "default"
		clusterName = fmt.Sprintf("e2e-live-%d", time.Now().Unix())

		token = getKeycloakToken()
	})

	Context("Cluster and machine lifecycle against live Carbide API", func() {
		It("should create cluster infrastructure and a machine, then clean up", func() {
			By("Setting up infrastructure via Carbide API")
			siteID, tenantID := setupSiteViaAPI(token, "test-org", clusterName)

			By("Creating credentials secret")
			secret := createCredentialsSecret(ctx, k8sClient, fmt.Sprintf("%s-creds", clusterName), testNamespace, token)

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
						APIGroup: "infrastructure.cluster.x-k8s.io",
						Kind:     "NvidiaCarbideCluster",
						Name:     clusterName,
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
							"test":    "e2e-live",
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
			waitForClusterReady(ctx, k8sClient, nvidiaCarbideCluster)

			By("Verifying VPC was created")
			Expect(nvidiaCarbideCluster.Status.VPCID).NotTo(BeEmpty())

			By("Verifying subnets were created")
			Expect(nvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs).To(HaveLen(2))
			Expect(nvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs).To(HaveKey("control-plane"))
			Expect(nvidiaCarbideCluster.Status.NetworkStatus.SubnetIDs).To(HaveKey("worker"))

			By("Creating a machine")
			machineName := fmt.Sprintf("%s-machine-0", clusterName)

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
						APIGroup: "infrastructure.cluster.x-k8s.io",
						Kind:     "NvidiaCarbideMachine",
						Name:     machineName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, machine)).To(Succeed())

			bootstrapSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-bootstrap", machineName),
					Namespace: testNamespace,
				},
				Data: map[string][]byte{
					"value": []byte("#!/bin/bash\necho 'bootstrap'"),
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
					InstanceType: infrastructurev1beta1.InstanceTypeSpec{},
					Network: infrastructurev1beta1.NetworkSpec{
						SubnetName: "control-plane",
					},
					Labels: map[string]string{
						"test":    "e2e-live",
						"cluster": clusterName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, nvidiaCarbideMachine)).To(Succeed())

			By("Waiting for NvidiaCarbideMachine to be ready")
			waitForMachineReady(ctx, k8sClient, nvidiaCarbideMachine)

			By("Verifying machine status fields are populated")
			Expect(nvidiaCarbideMachine.Status.InstanceID).NotTo(BeEmpty())
			Expect(nvidiaCarbideMachine.Status.InstanceState).NotTo(BeEmpty())

			By("Verifying machine has a provider ID")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(nvidiaCarbideMachine), nvidiaCarbideMachine)).To(Succeed())
			Expect(nvidiaCarbideMachine.Spec.ProviderID).NotTo(BeNil())
			Expect(*nvidiaCarbideMachine.Spec.ProviderID).To(HavePrefix("nvidia-carbide://"))

			By("Deleting the machine")
			Expect(k8sClient.Delete(ctx, machine)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nvidiaCarbideMachine)).To(Succeed())

			By("Waiting for NvidiaCarbideMachine to be deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(nvidiaCarbideMachine), nvidiaCarbideMachine)
				return err != nil
			}, clusterDeletionTimeout, pollInterval).Should(BeTrue())

			By("Deleting the cluster")
			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			Expect(k8sClient.Delete(ctx, nvidiaCarbideCluster)).To(Succeed())

			By("Waiting for NvidiaCarbideCluster to be deleted")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(nvidiaCarbideCluster), nvidiaCarbideCluster)
				return err != nil
			}, clusterDeletionTimeout, pollInterval).Should(BeTrue())

			By("Cleaning up secrets")
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			Expect(k8sClient.Delete(ctx, bootstrapSecret)).To(Succeed())

			By("Cleaning up site via Carbide API")
			cleanupSiteViaAPI(token, "test-org", siteID)
		})
	})
})
