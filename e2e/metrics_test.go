package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2" //nolint:revive // Using ginkgo DSL
	. "github.com/onsi/gomega"    //nolint:revive // Using gomega DSL
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Metrics", func() {
	Context("when operator is running", func() {
		It("should expose metrics endpoint", func() {
			ctx := context.Background()

			// Wait for operator deployment to be ready
			err := testSuite.WaitForOperatorReady(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Get operator pod
			pod, err := testSuite.getOperatorPod(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Port forward to metrics port
			stopCh := make(chan struct{})
			readyCh := make(chan struct{})

			go func() {
				defer close(stopCh)
				testSuite.portForward(pod.Name, 8081, 8081, stopCh, readyCh)
			}()

			// Wait for port forward to be ready
			select {
			case <-readyCh:
				GinkgoLogr.Info("Port forward ready")
			case <-time.After(30 * time.Second):
				close(stopCh)
				Fail("Port forward not ready within timeout")
			}

			defer close(stopCh)

			// Test metrics endpoint
			metricsURL := "http://localhost:8081/metrics"

			// Wait for metrics endpoint to be available
			Eventually(func() bool {
				resp, err := http.Get(metricsURL)
				if err != nil {
					return false
				}
				defer func() {
					if err := resp.Body.Close(); err != nil {
						testSuite.T().Logf("Failed to close response body: %v", err)
					}
				}()
				return resp.StatusCode == http.StatusOK
			}, 60*time.Second, 5*time.Second).Should(BeTrue())

			// Get metrics content
			resp, err := http.Get(metricsURL)
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				if err := resp.Body.Close(); err != nil {
					testSuite.T().Logf("Failed to close response body: %v", err)
				}
			}()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			metricsContent := string(body)
			GinkgoLogr.Info("Metrics content", "length", len(metricsContent))

			// Verify standard controller-runtime metrics
			testSuite.assertMetricExists(metricsContent, "controller_runtime_reconcile_total")
			testSuite.assertMetricExists(metricsContent, "controller_runtime_reconcile_time_seconds")
			testSuite.assertMetricExists(metricsContent, "workqueue_adds_total")
			testSuite.assertMetricExists(metricsContent, "workqueue_depth")

			// Verify performance metrics
			testSuite.assertMetricExists(metricsContent, "cloudflare_operator_memory_usage_bytes")
			testSuite.assertMetricExists(metricsContent, "cloudflare_operator_cpu_usage_seconds")
			testSuite.assertMetricExists(metricsContent, "cloudflare_operator_goroutines_total")

			// Verify Cloudflare-specific metrics
			testSuite.assertMetricExists(metricsContent, "cloudflare_api_requests_total")
			testSuite.assertMetricExists(metricsContent, "cloudflare_api_response_time_seconds")
			testSuite.assertMetricExists(metricsContent, "cloudflare_api_rate_limit_remaining")

			// Verify business metrics
			testSuite.assertMetricExists(metricsContent, "cloudflare_dns_records_by_type_total")
			testSuite.assertMetricExists(metricsContent, "cloudflare_crd_resources_total")
			testSuite.assertMetricExists(metricsContent, "cloudflare_reconcile_success_rate")
		})
	})
})

// Helper functions

func (suite *E2ETestSuite) getOperatorPod(ctx context.Context) (*corev1.Pod, error) {
	// Get deployment
	deployment := &appsv1.Deployment{}
	err := suite.k8sClient.Get(ctx, types.NamespacedName{
		Name:      "cloudflare-dns-operator",
		Namespace: suite.operatorNamespace,
	}, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Get pods for this deployment
	podList := &corev1.PodList{}
	err = suite.k8sClient.List(ctx, podList,
		client.InNamespace(suite.operatorNamespace),
		client.MatchingLabels(deployment.Spec.Selector.MatchLabels),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("no pods found for deployment")
	}

	// Return first running pod
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return &pod, nil
		}
	}

	return nil, fmt.Errorf("no running pods found")
}

func (suite *E2ETestSuite) assertMetricExists(metricsContent, metricName string) {
	Expect(metricsContent).To(ContainSubstring(metricName), "Metric %s should exist in metrics output", metricName)
}

func (suite *E2ETestSuite) WaitForOperatorReady(_ context.Context) error {
	return wait.PollUntilContextTimeout(
		context.Background(), 5*time.Second, 300*time.Second, true,
		func(ctx context.Context) (bool, error) {
			deployment := &appsv1.Deployment{}
			err := suite.k8sClient.Get(ctx, types.NamespacedName{
				Name:      "cloudflare-dns-operator",
				Namespace: suite.operatorNamespace,
			}, deployment)
			if err != nil {
				return false, nil
			}

			return deployment.Status.ReadyReplicas > 0, nil
		})
}
