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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1beta1 "github.com/fabiendupont/cluster-api-provider-nvidia-carbide/api/v1beta1"
)

const (
	keycloakRealm        = "carbide-dev"
	keycloakClientID     = "carbide-api"
	keycloakClientSecret = "carbide-local-secret"
	keycloakUsername      = "admin@example.com"
	keycloakPassword     = "adminpassword"
)

// getKeycloakToken acquires a JWT from Keycloak using the resource owner password grant.
func getKeycloakToken() string {
	keycloakURL := os.Getenv("NVIDIA_CARBIDE_KEYCLOAK_URL")
	Expect(keycloakURL).NotTo(BeEmpty(), "NVIDIA_CARBIDE_KEYCLOAK_URL must be set")

	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", keycloakURL, keycloakRealm)

	data := url.Values{
		"grant_type":    {"password"},
		"client_id":     {keycloakClientID},
		"client_secret": {keycloakClientSecret},
		"username":      {keycloakUsername},
		"password":      {keycloakPassword},
	}

	resp, err := http.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	Expect(err).NotTo(HaveOccurred(), "Failed to request Keycloak token")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.StatusCode).To(Equal(http.StatusOK),
		"Keycloak token request failed with status %d: %s", resp.StatusCode, string(body))

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	Expect(json.Unmarshal(body, &tokenResp)).To(Succeed())
	Expect(tokenResp.AccessToken).NotTo(BeEmpty(), "Received empty access token from Keycloak")

	_, _ = fmt.Fprintf(GinkgoWriter, "Successfully acquired Keycloak token\n")
	return tokenResp.AccessToken
}

// createCredentialsSecret creates a Kubernetes secret with NVIDIA Carbide API credentials.
func createCredentialsSecret(ctx context.Context, k8sClient client.Client, name, namespace, token string) *corev1.Secret {
	// Use the in-cluster endpoint if available (for controllers running inside the cluster),
	// otherwise fall back to the external endpoint.
	endpoint := os.Getenv("NVIDIA_CARBIDE_API_ENDPOINT_INTERNAL")
	if endpoint == "" {
		endpoint = os.Getenv("NVIDIA_CARBIDE_API_ENDPOINT")
	}
	Expect(endpoint).NotTo(BeEmpty(), "NVIDIA_CARBIDE_API_ENDPOINT or NVIDIA_CARBIDE_API_ENDPOINT_INTERNAL must be set")

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"endpoint": []byte(endpoint),
			"orgName":  []byte("test-org"),
			"token":    []byte(token),
		},
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	_, _ = fmt.Fprintf(GinkgoWriter, "Created credentials secret %s/%s\n", namespace, name)
	return secret
}

// waitForClusterReady polls the NvidiaCarbideCluster status until Ready is true.
func waitForClusterReady(ctx context.Context, k8sClient client.Client, cluster *infrastructurev1beta1.NvidiaCarbideCluster) {
	Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Error getting cluster: %v\n", err)
			return false
		}
		_, _ = fmt.Fprintf(GinkgoWriter, "Cluster ready=%v, vpcID=%s\n", cluster.Status.Ready, cluster.Status.VPCID)
		return cluster.Status.Ready
	}, clusterCreationTimeout, pollInterval).Should(BeTrue(), "NvidiaCarbideCluster did not become ready")
}

// waitForMachineReady polls the NvidiaCarbideMachine status until Ready is true.
func waitForMachineReady(ctx context.Context, k8sClient client.Client, machine *infrastructurev1beta1.NvidiaCarbideMachine) {
	Eventually(func() bool {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(machine), machine)
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Error getting machine: %v\n", err)
			return false
		}
		_, _ = fmt.Fprintf(GinkgoWriter, "Machine ready=%v, instanceID=%s, state=%s\n",
			machine.Status.Ready, machine.Status.InstanceID, machine.Status.InstanceState)
		return machine.Status.Ready
	}, clusterCreationTimeout, pollInterval).Should(BeTrue(), "NvidiaCarbideMachine did not become ready")
}

// carbideAPIRequest makes an authenticated request to the Carbide REST API.
func carbideAPIRequest(method, path, token string, body interface{}) (map[string]interface{}, int) {
	endpoint := os.Getenv("NVIDIA_CARBIDE_API_ENDPOINT")
	Expect(endpoint).NotTo(BeEmpty())

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		Expect(err).NotTo(HaveOccurred())
		reqBody = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequest(method, endpoint+path, reqBody)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	var result map[string]interface{}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &result)
	}

	_, _ = fmt.Fprintf(GinkgoWriter, "%s %s -> %d\n", method, path, resp.StatusCode)
	return result, resp.StatusCode
}

// getExistingSiteID finds the local-dev-site created by setup-local.sh.
// VPC creation triggers a Temporal workflow that requires a site-agent — only
// the pre-provisioned local-dev-site has one connected.
func getExistingSiteID(token, orgName string) string {
	apiBase := fmt.Sprintf("/v2/org/%s/carbide", orgName)

	result, status := carbideAPIRequest("GET", apiBase+"/site", token, nil)
	Expect(status).To(Equal(http.StatusOK), "Failed to list sites: %v", result)

	// The response is an array — parse from raw response
	endpoint := os.Getenv("NVIDIA_CARBIDE_API_ENDPOINT")
	req, err := http.NewRequest("GET", endpoint+apiBase+"/site", nil)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())

	var sites []map[string]interface{}
	Expect(json.Unmarshal(body, &sites)).To(Succeed())
	Expect(sites).NotTo(BeEmpty(), "No sites found — was setup-local.sh run?")

	siteID := sites[0]["id"].(string)
	siteName := sites[0]["name"].(string)
	_, _ = fmt.Fprintf(GinkgoWriter, "Using existing site %s (id=%s)\n", siteName, siteID)
	return siteID
}
// ensureSiteRegistered ensures the site is in Registered state.
// The site-agent registration workflow may not have completed yet.
func ensureSiteRegistered(siteID string) {
	cmd := exec.Command("kubectl", "exec", "-n", "postgres", "statefulset/postgres", "--",
		"psql", "-U", "forge", "-d", "forge", "-c",
		fmt.Sprintf("UPDATE site SET status = 'Registered' WHERE id = '%s' AND status != 'Registered'", siteID))
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "Failed to ensure site is registered: %s", string(output))
	_, _ = fmt.Fprintf(GinkgoWriter, "Ensured site %s is Registered\n", siteID)
}

// ensureInstanceTypeReady ensures the instance type is in Ready state.
func ensureInstanceTypeReady(instanceTypeID string) {
	cmd := exec.Command("kubectl", "exec", "-n", "postgres", "statefulset/postgres", "--",
		"psql", "-U", "forge", "-d", "forge", "-c",
		fmt.Sprintf("UPDATE instance_type SET status = 'Ready' WHERE id = '%s' AND status != 'Ready'", instanceTypeID))
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "Failed to ensure instance type is ready: %s", string(output))
	_, _ = fmt.Fprintf(GinkgoWriter, "Ensured instance type %s is Ready\n", instanceTypeID)
}

// setupSiteViaAPI finds the existing site and creates the tenant resources needed
// for the cluster controller: Tenant -> IP Block -> Instance Type -> Allocation.
// Returns siteID, tenantID, and instanceTypeID.
func setupSiteViaAPI(token, orgName, prefix string) (siteID, tenantID, instanceTypeID string) {
	apiBase := fmt.Sprintf("/v2/org/%s/carbide", orgName)

	// Use the existing site (has a connected site-agent for Temporal workflows)
	siteID = getExistingSiteID(token, orgName)

	// Ensure site is in Registered state (the site-agent registration may not have completed yet)
	ensureSiteRegistered(siteID)

	// Get or create Tenant (idempotent)
	carbideAPIRequest("POST", apiBase+"/tenant", token, map[string]interface{}{"org": orgName})
	currentTenant, tStatus := carbideAPIRequest("GET", apiBase+"/tenant/current", token, nil)
	Expect(tStatus).To(Equal(http.StatusOK), "Failed to get current tenant: %v", currentTenant)
	tenantID = currentTenant["id"].(string)
	_, _ = fmt.Fprintf(GinkgoWriter, "Tenant ID: %s\n", tenantID)

	// Create IP Block
	ipBlockResult, status := carbideAPIRequest("POST", apiBase+"/ipblock", token, map[string]interface{}{
		"name": prefix + "-ipblock", "siteId": siteID,
		"prefix": "10.0.0.0", "prefixLength": 16, "protocolVersion": "IPv4", "routingType": "Public",
	})
	Expect(status).To(Equal(http.StatusCreated), "Failed to create IP block: %v", ipBlockResult)
	ipBlockID := ipBlockResult["id"].(string)

	// Create Instance Type
	itResult, itStatus := carbideAPIRequest("POST", apiBase+"/instance/type", token, map[string]interface{}{
		"name":   prefix + "-instance-type",
		"siteId": siteID,
	})
	Expect(itStatus).To(Equal(http.StatusCreated), "Failed to create instance type: %v", itResult)
	instanceTypeID = itResult["id"].(string)
	_, _ = fmt.Fprintf(GinkgoWriter, "Instance Type ID: %s\n", instanceTypeID)

	// Ensure instance type is Ready
	ensureInstanceTypeReady(instanceTypeID)

	// Create Allocation with both IPBlock and InstanceType constraints
	allocResult, status := carbideAPIRequest("POST", apiBase+"/allocation", token, map[string]interface{}{
		"name":     prefix + "-allocation",
		"tenantId": tenantID,
		"siteId":   siteID,
		"allocationConstraints": []map[string]interface{}{
			{"resourceType": "IPBlock", "resourceTypeId": ipBlockID, "constraintType": "OnDemand", "constraintValue": 24},
			{"resourceType": "InstanceType", "resourceTypeId": instanceTypeID, "constraintType": "OnDemand", "constraintValue": 1},
		},
	})
	Expect(status).To(Equal(http.StatusCreated), "Failed to create allocation: %v", allocResult)

	return siteID, tenantID, instanceTypeID
}
