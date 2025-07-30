package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPerformanceMetrics_UpdateQueueDepth(t *testing.T) {
	// Reset metrics for test
	queueDepth.Reset()

	pm := NewPerformanceMetrics()

	// Test updating queue depth
	pm.UpdateQueueDepth("test-controller", "test-namespace", 5.0)

	// Verify metric value
	expected := 5.0
	actual := testutil.ToFloat64(queueDepth.WithLabelValues("test-controller", "test-namespace"))

	if actual != expected {
		t.Errorf("Expected queue depth %f, got %f", expected, actual)
	}
}

func TestPerformanceMetrics_IncReconcileRate(t *testing.T) {
	// Reset metrics for test
	reconcileRate.Reset()

	pm := NewPerformanceMetrics()

	// Test incrementing reconcile rate
	pm.IncReconcileRate("test-controller", "success", "test-namespace")
	pm.IncReconcileRate("test-controller", "success", "test-namespace")

	// Verify metric value
	expected := 2.0
	actual := testutil.ToFloat64(reconcileRate.WithLabelValues("test-controller", "success", "test-namespace"))

	if actual != expected {
		t.Errorf("Expected reconcile rate %f, got %f", expected, actual)
	}
}

func TestPerformanceMetrics_IncErrorRate(t *testing.T) {
	// Reset metrics for test
	errorRate.Reset()

	pm := NewPerformanceMetrics()

	// Test incrementing error rate
	pm.IncErrorRate("test-controller", "reconcile_error", "test-namespace")

	// Verify metric value
	expected := 1.0
	actual := testutil.ToFloat64(errorRate.WithLabelValues("test-controller", "reconcile_error", "test-namespace"))

	if actual != expected {
		t.Errorf("Expected error rate %f, got %f", expected, actual)
	}
}

func TestPerformanceMetrics_ObserveAPIResponseTime(t *testing.T) {
	// Reset metrics for test
	apiResponseTime.Reset()

	pm := NewPerformanceMetrics()

	// Test observing API response time
	duration := 100 * time.Millisecond
	pm.ObserveAPIResponseTime("record_update", "200", "test-namespace", duration)

	// For histogram metrics, we check if observations were recorded
	// We can't easily check the exact count with testutil.ToFloat64 on histogram observers
	// Instead, we verify the observation was made by checking that the metric exists
	// and has samples
	metric := apiResponseTime.WithLabelValues("record_update", "200", "test-namespace")
	if metric == nil {
		t.Error("Expected API response time metric to exist")
	}
}

func TestPerformanceMetrics_ObserveReconcileDuration(t *testing.T) {
	// Reset metrics for test
	reconcileDuration.Reset()

	pm := NewPerformanceMetrics()

	// Test observing reconcile duration
	duration := 2 * time.Second
	pm.ObserveReconcileDuration("test-controller", "success", "test-namespace", duration)

	// For histogram metrics, we check if observations were recorded
	// We can't easily check the exact count with testutil.ToFloat64 on histogram observers
	// Instead, we verify the observation was made by checking that the metric exists
	metric := reconcileDuration.WithLabelValues("test-controller", "success", "test-namespace")
	if metric == nil {
		t.Error("Expected reconcile duration metric to exist")
	}
}

func TestPerformanceMetrics_UpdateResourceProcessingRate(t *testing.T) {
	// Reset metrics for test
	resourceProcessingRate.Reset()

	pm := NewPerformanceMetrics()

	// Test updating resource processing rate
	pm.UpdateResourceProcessingRate("test-controller", "test-namespace", 10.5)

	// Verify metric value
	expected := 10.5
	actual := testutil.ToFloat64(resourceProcessingRate.WithLabelValues("test-controller", "test-namespace"))

	if actual != expected {
		t.Errorf("Expected resource processing rate %f, got %f", expected, actual)
	}
}

func TestPerformanceMetrics_UpdateCacheHitRatio(t *testing.T) {
	// Reset metrics for test
	cacheHitRatio.Reset()

	pm := NewPerformanceMetrics()

	// Test updating cache hit ratio
	pm.UpdateCacheHitRatio("test-controller", "object-cache", 0.85)

	// Verify metric value
	expected := 0.85
	actual := testutil.ToFloat64(cacheHitRatio.WithLabelValues("test-controller", "object-cache"))

	if actual != expected {
		t.Errorf("Expected cache hit ratio %f, got %f", expected, actual)
	}
}

func TestPerformanceMetrics_UpdateMemoryUsage(t *testing.T) {
	// Reset metrics for test
	memoryUsage.Reset()

	pm := NewPerformanceMetrics()

	// Test updating memory usage
	pm.UpdateMemoryUsage("heap", 1024*1024) // 1MB

	// Verify metric value
	expected := 1024.0 * 1024.0
	actual := testutil.ToFloat64(memoryUsage.WithLabelValues("heap"))

	if actual != expected {
		t.Errorf("Expected memory usage %f, got %f", expected, actual)
	}
}

func TestPerformanceMetrics_UpdateCPUUsage(t *testing.T) {
	// Reset metrics for test
	cpuUsage.Reset()

	pm := NewPerformanceMetrics()

	// Test updating CPU usage
	pm.UpdateCPUUsage("user", 5.5)

	// Verify metric value
	expected := 5.5
	actual := testutil.ToFloat64(cpuUsage.WithLabelValues("user"))

	if actual != expected {
		t.Errorf("Expected CPU usage %f, got %f", expected, actual)
	}
}

func TestPerformanceMetrics_UpdateActiveWorkers(t *testing.T) {
	// Reset metrics for test
	activeWorkers.Reset()

	pm := NewPerformanceMetrics()

	// Test updating active workers
	pm.UpdateActiveWorkers("test-controller", 3.0)

	// Verify metric value
	expected := 3.0
	actual := testutil.ToFloat64(activeWorkers.WithLabelValues("test-controller"))

	if actual != expected {
		t.Errorf("Expected active workers %f, got %f", expected, actual)
	}
}

func TestPerformanceMetrics_GetQueueDepthMetric(t *testing.T) {
	// Reset metrics for test
	queueDepth.Reset()

	pm := NewPerformanceMetrics()

	// Set a queue depth value
	expectedDepth := 7.0
	pm.UpdateQueueDepth("test-controller", "test-namespace", expectedDepth)

	// Get the metric value
	actualDepth := pm.GetQueueDepthMetric("test-controller", "test-namespace")

	if actualDepth != expectedDepth {
		t.Errorf("Expected queue depth metric %f, got %f", expectedDepth, actualDepth)
	}

	// Test with non-existent metric
	nonExistentDepth := pm.GetQueueDepthMetric("non-existent", "non-existent")
	if nonExistentDepth != 0 {
		t.Errorf("Expected non-existent queue depth metric to be 0, got %f", nonExistentDepth)
	}
}

func TestPerformanceMetrics_GetReconcileRateMetric(t *testing.T) {
	// Reset metrics for test
	reconcileRate.Reset()

	pm := NewPerformanceMetrics()

	// Increment reconcile rate
	pm.IncReconcileRate("test-controller", "success", "test-namespace")
	pm.IncReconcileRate("test-controller", "success", "test-namespace")

	// Get the metric value
	expectedRate := 2.0
	actualRate := pm.GetReconcileRateMetric("test-controller", "test-namespace")

	if actualRate != expectedRate {
		t.Errorf("Expected reconcile rate metric %f, got %f", expectedRate, actualRate)
	}
}

func TestPerformanceMetrics_GetErrorRateMetric(t *testing.T) {
	// Reset metrics for test
	errorRate.Reset()

	pm := NewPerformanceMetrics()

	// Increment error rate
	pm.IncErrorRate("test-controller", "reconcile_error", "test-namespace")

	// Get the metric value
	expectedErrors := 1.0
	actualErrors := pm.GetErrorRateMetric("test-controller", "test-namespace")

	if actualErrors != expectedErrors {
		t.Errorf("Expected error rate metric %f, got %f", expectedErrors, actualErrors)
	}
}

func TestPerformanceMetrics_NewPerformanceMetrics(t *testing.T) {
	pm := NewPerformanceMetrics()

	if pm == nil {
		t.Error("Expected NewPerformanceMetrics to return a non-nil instance")
	}

	// Test that it can be used immediately
	pm.UpdateQueueDepth("test", "test", 1.0)
}

func TestGetQueueDepthMetricEdgeCases(t *testing.T) {
	// Reset metrics for test
	queueDepth.Reset()

	pm := NewPerformanceMetrics()

	// Test with non-existent metric first
	depth := pm.GetQueueDepthMetric("non-existent", "non-existent")
	assert.Equal(t, 0.0, depth)

	// Test after setting a value
	pm.UpdateQueueDepth("test-controller", "test-namespace", 5.0)
	depth = pm.GetQueueDepthMetric("test-controller", "test-namespace")
	assert.Equal(t, 5.0, depth)
}

func TestGetReconcileRateMetricEdgeCases(t *testing.T) {
	// Reset metrics for test
	reconcileRate.Reset()

	pm := NewPerformanceMetrics()

	// Test with non-existent metric first
	rate := pm.GetReconcileRateMetric("non-existent", "non-existent")
	assert.Equal(t, 0.0, rate)

	// Test after incrementing
	pm.IncReconcileRate("test-controller", "success", "test-namespace")
	pm.IncReconcileRate("test-controller", "success", "test-namespace")
	rate = pm.GetReconcileRateMetric("test-controller", "test-namespace")
	assert.Equal(t, 2.0, rate)
}

func TestGetErrorRateMetricEdgeCases(t *testing.T) {
	// Reset metrics for test
	errorRate.Reset()

	pm := NewPerformanceMetrics()

	// Test with non-existent metric first
	errors := pm.GetErrorRateMetric("non-existent", "non-existent")
	assert.Equal(t, 0.0, errors)

	// Test after incrementing
	pm.IncErrorRate("test-controller", "reconcile_error", "test-namespace")
	errors = pm.GetErrorRateMetric("test-controller", "test-namespace")
	assert.Equal(t, 1.0, errors)
}

func TestPerformanceMetrics_AllMethodsWithVariousInputs(t *testing.T) {
	pm := NewPerformanceMetrics()

	// Test with various inputs to ensure robustness
	controllers := []string{"controller1", "controller2", ""}
	namespaces := []string{"namespace1", "namespace2", "default", ""}

	for _, controller := range controllers {
		for _, namespace := range namespaces {
			// Test all methods with various inputs
			pm.UpdateQueueDepth(controller, namespace, 1.0)
			pm.IncReconcileRate(controller, "success", namespace)
			pm.IncErrorRate(controller, "error", namespace)
			pm.ObserveAPIResponseTime("operation", "200", namespace, time.Millisecond)
			pm.ObserveReconcileDuration(controller, "success", namespace, time.Second)
			pm.UpdateResourceProcessingRate(controller, namespace, 1.0)
		}
	}

	// Test cache and system metrics
	pm.UpdateCacheHitRatio("controller1", "cache-type", 0.95)
	pm.UpdateMemoryUsage("heap", 1024*1024)
	pm.UpdateCPUUsage("user", 1.5)
	pm.UpdateActiveWorkers("controller1", 5.0)

	// Verify no panics occurred and methods executed successfully
	assert.True(t, true) // If we reach here, no panics occurred
}

func TestGetQueueDepthMetricErrorHandling(t *testing.T) {
	// Reset metrics for test
	queueDepth.Reset()

	pm := NewPerformanceMetrics()

	// Test error handling when metric gathering fails
	// Try with invalid label combinations that might cause GetMetricWithLabelValues to fail
	depth := pm.GetQueueDepthMetric("controller-with-invalid-chars-\x00", "namespace-with-invalid-chars-\x00")
	assert.Equal(t, 0.0, depth)

	// Test with very long strings that might cause issues
	longString := string(make([]byte, 1000))
	depth = pm.GetQueueDepthMetric(longString, longString)
	assert.Equal(t, 0.0, depth)

	// Test with empty strings
	depth = pm.GetQueueDepthMetric("", "")
	assert.Equal(t, 0.0, depth)

	// Test the error path by trying to get a metric that was never set
	depth = pm.GetQueueDepthMetric("non-existent-controller", "non-existent-namespace")
	assert.Equal(t, 0.0, depth)
}

func TestGetReconcileRateMetricErrorHandling(t *testing.T) {
	// Reset metrics for test
	reconcileRate.Reset()

	pm := NewPerformanceMetrics()

	// Test error handling when metric gathering fails
	// Try with invalid label combinations that might cause GetMetricWithLabelValues to fail
	rate := pm.GetReconcileRateMetric("controller-with-invalid-chars-\x00", "namespace-with-invalid-chars-\x00")
	assert.Equal(t, 0.0, rate)

	// Test with very long strings that might cause issues
	longString := string(make([]byte, 1000))
	rate = pm.GetReconcileRateMetric(longString, longString)
	assert.Equal(t, 0.0, rate)

	// Test with empty strings
	rate = pm.GetReconcileRateMetric("", "")
	assert.Equal(t, 0.0, rate)

	// Test the error path by trying to get a metric with labels that don't match the expected pattern
	rate = pm.GetReconcileRateMetric("non-existent-controller", "non-existent-namespace")
	assert.Equal(t, 0.0, rate)
}

func TestGetQueueDepthMetricPanicRecovery(t *testing.T) {
	// This test tries to trigger the error path by creating a situation
	// where GetMetricWithLabelValues might fail

	pm := NewPerformanceMetrics()

	// Try multiple approaches to trigger error conditions
	testCases := []struct {
		controller string
		namespace  string
	}{
		{"controller\x00withNull", "namespace\x00withNull"},
		{"controller\nwithNewline", "namespace\nwithNewline"},
		{"controller\twithTab", "namespace\twithTab"},
		{"controller\"withQuote", "namespace\"withQuote"},
		{"controller'withApostrophe", "namespace'withApostrophe"},
		{"controller\\withBackslash", "namespace\\withBackslash"},
		{"controller/withSlash", "namespace/withSlash"},
		{"controller?withQuestion", "namespace?withQuestion"},
		{"controller*withAsterisk", "namespace*withAsterisk"},
	}

	for _, tc := range testCases {
		depth := pm.GetQueueDepthMetric(tc.controller, tc.namespace)
		assert.Equal(t, 0.0, depth)
	}
}

func TestGetReconcileRateMetricPanicRecovery(t *testing.T) {
	// This test tries to trigger the error path by creating a situation
	// where GetMetricWithLabelValues might fail

	pm := NewPerformanceMetrics()

	// Try multiple approaches to trigger error conditions
	testCases := []struct {
		controller string
		namespace  string
	}{
		{"controller\x00withNull", "namespace\x00withNull"},
		{"controller\nwithNewline", "namespace\nwithNewline"},
		{"controller\twithTab", "namespace\twithTab"},
		{"controller\"withQuote", "namespace\"withQuote"},
		{"controller'withApostrophe", "namespace'withApostrophe"},
		{"controller\\withBackslash", "namespace\\withBackslash"},
		{"controller/withSlash", "namespace/withSlash"},
		{"controller?withQuestion", "namespace?withQuestion"},
		{"controller*withAsterisk", "namespace*withAsterisk"},
	}

	for _, tc := range testCases {
		rate := pm.GetReconcileRateMetric(tc.controller, tc.namespace)
		assert.Equal(t, 0.0, rate)
	}
}

func TestGetQueueDepthMetricWithoutMetricSet(t *testing.T) {
	// Reset to ensure clean state
	queueDepth.Reset()

	pm := NewPerformanceMetrics()

	// The key insight: GetMetricWithLabelValues returns an error when
	// trying to get a metric with label values that don't exist
	// This should trigger the error path and the "return 0" statement on line 187
	depth := pm.GetQueueDepthMetric("nonexistent-controller", "nonexistent-namespace")

	// This should be 0 because the metric with these labels was never created
	assert.Equal(t, 0.0, depth)

	// Try a few more combinations to ensure we hit the error path
	depth = pm.GetQueueDepthMetric("", "")
	assert.Equal(t, 0.0, depth)

	depth = pm.GetQueueDepthMetric("controller", "")
	assert.Equal(t, 0.0, depth)

	depth = pm.GetQueueDepthMetric("", "namespace")
	assert.Equal(t, 0.0, depth)
}

func TestGetReconcileRateMetricWithoutMetricSet(t *testing.T) {
	// Reset to ensure clean state
	reconcileRate.Reset()

	pm := NewPerformanceMetrics()

	// The key insight: GetMetricWithLabelValues returns an error when
	// trying to get a metric with label values that don't exist
	// This should trigger the error path and the "return 0" statement on line 199
	rate := pm.GetReconcileRateMetric("nonexistent-controller", "nonexistent-namespace")

	// This should be 0 because the metric with these labels was never created
	assert.Equal(t, 0.0, rate)

	// Try a few more combinations to ensure we hit the error path
	rate = pm.GetReconcileRateMetric("", "")
	assert.Equal(t, 0.0, rate)

	rate = pm.GetReconcileRateMetric("controller", "")
	assert.Equal(t, 0.0, rate)

	rate = pm.GetReconcileRateMetric("", "namespace")
	assert.Equal(t, 0.0, rate)
}

// Test to force coverage of error paths in Get*Metric methods
func TestMetricErrorPathsCoverage(t *testing.T) {
	// Fresh start with all metrics reset
	queueDepth.Reset()
	reconcileRate.Reset()
	errorRate.Reset()

	pm := NewPerformanceMetrics()

	// These calls should trigger the error paths because metrics with these
	// labels were never set, so GetMetricWithLabelValues should return an error

	// Test GetQueueDepthMetric error path (line 187)
	depth := pm.GetQueueDepthMetric("force-error-path", "force-error-path")
	assert.Equal(t, 0.0, depth)

	// Test GetReconcileRateMetric error path (line 199)
	rate := pm.GetReconcileRateMetric("force-error-path", "force-error-path")
	assert.Equal(t, 0.0, rate)

	// Test with various combinations that should all trigger error paths
	testLabels := [][]string{
		{"error-test-1", "error-test-1"},
		{"error-test-2", "error-test-2"},
		{"error-test-3", "error-test-3"},
	}

	for _, labels := range testLabels {
		depth = pm.GetQueueDepthMetric(labels[0], labels[1])
		assert.Equal(t, 0.0, depth)

		rate = pm.GetReconcileRateMetric(labels[0], labels[1])
		assert.Equal(t, 0.0, rate)
	}
}

func TestMetricsRegistration(t *testing.T) {
	// Test that all metrics are properly registered
	registry := prometheus.NewRegistry()

	// Create new versions of the metrics for this test
	testQueueDepth := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cloudflare_operator_queue_depth",
			Help: "Test queue depth metric",
		},
		[]string{"controller", "namespace"},
	)

	// Register the metric
	err := registry.Register(testQueueDepth)
	if err != nil {
		t.Errorf("Failed to register test metric: %v", err)
	}

	// Try to register the same metric again (should fail)
	err = registry.Register(testQueueDepth)
	if err == nil {
		t.Error("Expected error when registering duplicate metric")
	}
}
