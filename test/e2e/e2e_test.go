/*
Copyright 2025.

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

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive // Using ginkgo DSL
	. "github.com/onsi/gomega"    //nolint:revive // Using gomega DSL

	"github.com/devops247-online/k8s-operator-cloudflare/test/utils"
)

// namespace where the project is deployed in
const namespace = "k8s-operator-cloudflare-system"

// serviceAccountName created for the project
const serviceAccountName = "k8s-operator-cloudflare-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "k8s-operator-cloudflare-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "k8s-operator-cloudflare-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("ensuring manager namespace exists")
		cmd := exec.Command("kubectl", "get", "ns", namespace)
		_, err := utils.Run(cmd)
		if err != nil {
			// Namespace doesn't exist, create it
			By("creating manager namespace")
			cmd = exec.Command("kubectl", "create", "ns", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")
		}

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("ensuring ClusterRoleBinding for metrics access exists")
			cmd := exec.Command("kubectl", "get", "clusterrolebinding", metricsRoleBindingName)
			_, err := utils.Run(cmd)
			if err != nil {
				// ClusterRoleBinding doesn't exist, create it
				By("creating a ClusterRoleBinding for the service account to allow access to metrics")
				cmd = exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
					"--clusterrole=k8s-operator-cloudflare-metrics-reader",
					fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
				)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")
			}

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				// Get the controller pod name if not already set
				if controllerPodName == "" {
					cmd := exec.Command("kubectl", "get",
						"pods", "-l", "control-plane=controller-manager",
						"-o", "go-template={{ range .items }}"+
							"{{ if not .metadata.deletionTimestamp }}"+
							"{{ .metadata.name }}"+
							"{{ \"\\n\" }}{{ end }}{{ end }}",
						"-n", namespace,
					)
					podOutput, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
					podNames := utils.GetNonEmptyLines(podOutput)
					g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
					controllerPodName = podNames[0]
				}

				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("cleaning up any existing curl-metrics pod")
			cleanupCmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace, "--ignore-not-found")
			_, _ = utils.Run(cleanupCmd)

			By("creating the curl-metrics pod to access the metrics endpoint")
			// Create a temporary YAML file for the pod with proper security context
			podYAML := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: curl-metrics
  namespace: %s
spec:
  restartPolicy: Never
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: curl
    image: curlimages/curl:latest
    command: ["curl", "-k", "-f", "-v", "-H", "Authorization: Bearer %s", "https://%s.%s.svc.cluster.local:8443/metrics"]
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop: ["ALL"]
      readOnlyRootFilesystem: true
`, namespace, token, metricsServiceName, namespace)

			// Write the YAML to a temporary file
			tmpFile := "/tmp/curl-metrics-pod.yaml"
			err = os.WriteFile(tmpFile, []byte(podYAML), 0644)
			Expect(err).NotTo(HaveOccurred(), "Failed to write pod YAML")
			defer func() { _ = os.Remove(tmpFile) }()

			// Apply the pod manifest
			cmd = exec.Command("kubectl", "apply", "-f", tmpFile)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				// Log current status for debugging
				fmt.Printf("curl-metrics pod status: %s\n", output)

				// Accept both Succeeded and Failed - we'll check logs regardless
				g.Expect(output).To(Or(Equal("Succeeded"), Equal("Failed")), "curl pod should complete (either success or failure)")
			}
			Eventually(verifyCurlUp, 3*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			var metricsOutput string
			Eventually(func(g Gomega) {
				metricsOutput = getMetricsOutput()
				// Check for a basic metric that should always be present
				g.Expect(metricsOutput).To(ContainSubstring("controller_runtime_webhook_panics_total"))
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		// TODO: Customize the e2e test suite with scenarios specific to your project.
		// Consider applying sample/CR(s) and check their status and/or verifying
		// the reconciliation by using the metrics, i.e.:
		// metricsOutput := getMetricsOutput()
		// Expect(metricsOutput).To(ContainSubstring(
		//    fmt.Sprintf(`controller_runtime_reconcile_total{controller="%s",result="success"} 1`,
		//    strings.ToLower(<Kind>),
		// ))
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")

	// Always log the output for debugging
	fmt.Printf("curl-metrics pod logs:\n%s\n", metricsOutput)

	// Check if the pod succeeded and got metrics
	if !strings.Contains(metricsOutput, "< HTTP/1.1 200 OK") {
		// Get more debugging info
		By("curl pod did not get HTTP 200, gathering debug info")

		// Check service endpoint
		endpointCmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace, "-o", "yaml")
		endpointDetails, _ := utils.Run(endpointCmd)
		fmt.Printf("metrics service endpoints:\n%s\n", endpointDetails)

		// Check service details
		serviceCmd := exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace, "-o", "yaml")
		serviceDetails, _ := utils.Run(serviceCmd)
		fmt.Printf("metrics service details:\n%s\n", serviceDetails)

		Fail("curl pod did not receive HTTP 200 OK response from metrics endpoint")
	}

	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
