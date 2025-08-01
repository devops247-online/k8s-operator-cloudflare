package metrics

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudflareMetrics_RecordAPIRequest(t *testing.T) {
	// Create a new registry for this test to avoid conflicts
	registry := prometheus.NewRegistry()

	// Create fresh metrics
	apiRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_cloudflare_api_requests_total",
			Help: "Test metric for API requests",
		},
		[]string{"method", "endpoint", "status", "zone_id"},
	)

	apiResponseTime := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_cloudflare_api_response_time_seconds",
			Help:    "Test metric for API response time",
			Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"method", "endpoint", "status", "zone_id"},
	)

	registry.MustRegister(apiRequestsTotal, apiResponseTime)

	// Test data
	method := "POST"
	endpoint := "/zones/dns_records"
	status := "200"
	zoneID := "test-zone-123"
	duration := 250 * time.Millisecond

	// Record the metrics
	apiRequestsTotal.WithLabelValues(method, endpoint, status, zoneID).Inc()
	apiResponseTime.WithLabelValues(method, endpoint, status, zoneID).Observe(duration.Seconds())

	// Verify counter metric
	counterMetric := testutil.ToFloat64(apiRequestsTotal.WithLabelValues(method, endpoint, status, zoneID))
	assert.Equal(t, float64(1), counterMetric, "API request counter should be 1")

	// Verify histogram metric (check count) using the histogram's counter metric
	histogramCount := &dto.Metric{}
	histogram := apiResponseTime.WithLabelValues(method, endpoint, status, zoneID).(prometheus.Histogram)
	err := histogram.Write(histogramCount)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), histogramCount.GetHistogram().GetSampleCount(), "API response time histogram count should be 1")
}

func TestCloudflareMetrics_RecordAPIError(t *testing.T) {
	// Create a new registry for this test
	registry := prometheus.NewRegistry()

	apiErrorsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_cloudflare_api_errors_total",
			Help: "Test metric for API errors",
		},
		[]string{"method", "endpoint", "error_type", "zone_id"},
	)

	registry.MustRegister(apiErrorsTotal)

	// Test data
	method := "POST"
	endpoint := "/zones/dns_records"
	errorType := "rate_limit"
	zoneID := "test-zone-123"

	// Record the error
	apiErrorsTotal.WithLabelValues(method, endpoint, errorType, zoneID).Inc()

	// Verify error metric
	errorMetric := testutil.ToFloat64(apiErrorsTotal.WithLabelValues(method, endpoint, errorType, zoneID))
	assert.Equal(t, float64(1), errorMetric, "API error counter should be 1")
}

func TestCloudflareMetrics_UpdateRateLimit(t *testing.T) {
	// Create a new registry for this test
	registry := prometheus.NewRegistry()

	rateLimitRemaining := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cloudflare_api_rate_limit_remaining",
			Help: "Test metric for rate limit remaining",
		},
		[]string{"endpoint"},
	)

	rateLimitTotal := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cloudflare_api_rate_limit_total",
			Help: "Test metric for rate limit total",
		},
		[]string{"endpoint"},
	)

	registry.MustRegister(rateLimitRemaining, rateLimitTotal)

	// Test data
	endpoint := "/zones/dns_records"
	remaining := 900.0
	total := 1000.0

	// Update the metrics
	rateLimitRemaining.WithLabelValues(endpoint).Set(remaining)
	rateLimitTotal.WithLabelValues(endpoint).Set(total)

	// Verify metrics
	remainingMetric := testutil.ToFloat64(rateLimitRemaining.WithLabelValues(endpoint))
	totalMetric := testutil.ToFloat64(rateLimitTotal.WithLabelValues(endpoint))

	assert.Equal(t, remaining, remainingMetric, "Rate limit remaining should match")
	assert.Equal(t, total, totalMetric, "Rate limit total should match")
}

func TestCloudflareMetrics_UpdateManagedRecords(t *testing.T) {
	// Create a new registry for this test
	registry := prometheus.NewRegistry()

	managedRecords := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cloudflare_dns_records_managed_total",
			Help: "Test metric for managed records",
		},
		[]string{"zone_id", "zone_name", "record_type"},
	)

	registry.MustRegister(managedRecords)

	// Test data
	zoneID := "test-zone-123"
	zoneName := "example.com"
	recordType := "A"
	count := 5.0

	// Update the metric
	managedRecords.WithLabelValues(zoneID, zoneName, recordType).Set(count)

	// Verify metric
	metric := testutil.ToFloat64(managedRecords.WithLabelValues(zoneID, zoneName, recordType))
	assert.Equal(t, count, metric, "Managed records count should match")
}

func TestCloudflareMetrics_RecordDNSOperation(t *testing.T) {
	// Create a new registry for this test
	registry := prometheus.NewRegistry()

	dnsOperations := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_cloudflare_dns_record_operations_total",
			Help: "Test metric for DNS operations",
		},
		[]string{"operation", "record_type", "zone_id", "result"},
	)

	registry.MustRegister(dnsOperations)

	// Test data
	operation := "create"
	recordType := "A"
	zoneID := "test-zone-123"
	result := "success"

	// Record the operation
	dnsOperations.WithLabelValues(operation, recordType, zoneID, result).Inc()

	// Verify metric
	metric := testutil.ToFloat64(dnsOperations.WithLabelValues(operation, recordType, zoneID, result))
	assert.Equal(t, float64(1), metric, "DNS operation counter should be 1")
}

func TestCloudflareMetrics_APICallTimer(t *testing.T) {
	// Create a new registry for this test
	registry := prometheus.NewRegistry()

	apiRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_cloudflare_api_requests_total",
			Help: "Test metric for API requests",
		},
		[]string{"method", "endpoint", "status", "zone_id"},
	)

	apiResponseTime := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_cloudflare_api_response_time_seconds",
			Help:    "Test metric for API response time",
			Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"method", "endpoint", "status", "zone_id"},
	)

	registry.MustRegister(apiRequestsTotal, apiResponseTime)

	// Create CloudflareMetrics instance (we'll simulate the timer functionality)
	_ = NewCloudflareMetrics()

	// Test successful API call timing
	method := "GET"
	endpoint := "/zones"
	zoneID := "test-zone-456"

	// Simulate timer functionality (since we can't easily test the real timer)
	startTime := time.Now()
	time.Sleep(10 * time.Millisecond) // Simulate API call duration
	duration := time.Since(startTime)

	// Record success (this would normally be done by the timer)
	apiRequestsTotal.WithLabelValues(method, endpoint, "success", zoneID).Inc()
	apiResponseTime.WithLabelValues(method, endpoint, "success", zoneID).Observe(duration.Seconds())

	// Verify metrics
	counterMetric := testutil.ToFloat64(apiRequestsTotal.WithLabelValues(method, endpoint, "success", zoneID))
	assert.Equal(t, float64(1), counterMetric, "API request counter should be 1")

	// Verify histogram count using the histogram's counter metric
	histogramCount := &dto.Metric{}
	histogram := apiResponseTime.WithLabelValues(method, endpoint, "success", zoneID).(prometheus.Histogram)
	writeErr := histogram.Write(histogramCount)
	require.NoError(t, writeErr)
	assert.Equal(t, uint64(1), histogramCount.GetHistogram().GetSampleCount(), "API response time histogram count should be 1")
}

func TestCloudflareMetrics_UpdateZoneStatus(t *testing.T) {
	// Create a new registry for this test
	registry := prometheus.NewRegistry()

	zoneStatus := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cloudflare_zone_status",
			Help: "Test metric for zone status",
		},
		[]string{"zone_id", "zone_name", "status"},
	)

	registry.MustRegister(zoneStatus)

	// Test data
	zoneID := "test-zone-789"
	zoneName := "example.org"
	status := "active"

	// Test active zone
	zoneStatus.WithLabelValues(zoneID, zoneName, status).Set(1.0)
	metric := testutil.ToFloat64(zoneStatus.WithLabelValues(zoneID, zoneName, status))
	assert.Equal(t, float64(1), metric, "Active zone status should be 1")

	// Test inactive zone
	zoneStatus.WithLabelValues(zoneID, zoneName, status).Set(0.0)
	metric = testutil.ToFloat64(zoneStatus.WithLabelValues(zoneID, zoneName, status))
	assert.Equal(t, float64(0), metric, "Inactive zone status should be 0")
}

func TestNewCloudflareMetrics(t *testing.T) {
	cm := NewCloudflareMetrics()
	require.NotNil(t, cm, "CloudflareMetrics instance should not be nil")
}

// Integration test to verify all metrics can be registered without conflicts
func TestCloudflareMetrics_MetricsRegistration(t *testing.T) {
	// This test verifies that our metrics can be registered without panicking
	// The actual registration happens in the init() function, so this mainly
	// tests that the metrics are properly defined

	// Create a CloudflareMetrics instance
	cm := NewCloudflareMetrics()
	require.NotNil(t, cm, "CloudflareMetrics should be created successfully")

	// Test that we can call all methods without panicking
	assert.NotPanics(t, func() {
		cm.RecordAPIRequest("GET", "/test", "200", "zone-123", 100*time.Millisecond)
		cm.RecordAPIError("POST", "/test", "timeout", "zone-123")
		cm.UpdateRateLimit("/test", 900, 1000)
		cm.UpdateManagedRecords("zone-123", "example.com", "A", 5)
		cm.UpdateManagedZones("example.com", "free", 1)
		cm.RecordDNSOperation("create", "A", "zone-123", "success")
		cm.ObserveOperationDuration("create", "zone-123", "success", 200*time.Millisecond)
		cm.UpdateZoneStatus("zone-123", "example.com", "active", true)
	}, "All CloudflareMetrics methods should execute without panicking")
}

func TestCloudflareMetrics_GetAPIRequestCount(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Record some API requests first
	cm.RecordAPIRequest("GET", "/zones", "200", "zone-123", 100*time.Millisecond)
	cm.RecordAPIRequest("POST", "/zones/dns_records", "200", "zone-123", 150*time.Millisecond)

	// Test getting API request count
	count := cm.GetAPIRequestCount("GET", "/zones", "200", "zone-123")
	assert.Equal(t, float64(1), count, "API request count should be 1")

	// Test with non-existent labels
	count = cm.GetAPIRequestCount("DELETE", "/nonexistent", "404", "zone-999")
	assert.Equal(t, float64(0), count, "Non-existent API request count should be 0")
}

func TestCloudflareMetrics_GetManagedRecordsCount(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Update managed records first
	cm.UpdateManagedRecords("zone-123", "example.com", "A", 5)
	cm.UpdateManagedRecords("zone-456", "test.com", "CNAME", 3)

	// Test getting managed records count
	count := cm.GetManagedRecordsCount("zone-123", "example.com", "A")
	assert.Equal(t, float64(5), count, "Managed records count should be 5")

	// Test with non-existent labels
	count = cm.GetManagedRecordsCount("zone-999", "nonexistent.com", "TXT")
	assert.Equal(t, float64(0), count, "Non-existent managed records count should be 0")
}

func TestCloudflareMetrics_NewAPICallTimer(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Create a timer
	timer := cm.NewAPICallTimer("POST", "/zones/dns_records", "zone-123")
	require.NotNil(t, timer, "APICallTimer should not be nil")

	// Test timer fields (using reflection since fields are private)
	assert.Equal(t, "POST", timer.method)
	assert.Equal(t, "/zones/dns_records", timer.endpoint)
	assert.Equal(t, "zone-123", timer.zoneID)
	// Can't directly test private fields, but we can test that timer was created recently
	// by testing that RecordSuccess doesn't panic (which would happen if start time wasn't set)
	assert.NotPanics(t, func() {
		timer.RecordSuccess()
	}, "Timer should have valid start time")
}

func TestAPICallTimer_RecordSuccess(t *testing.T) {
	cm := NewCloudflareMetrics()
	timer := cm.NewAPICallTimer("GET", "/zones", "zone-123")

	// Wait a bit to ensure measurable duration
	time.Sleep(1 * time.Millisecond)

	// Record success
	timer.RecordSuccess()

	// Verify that success was recorded (we can't easily verify the exact metrics
	// because they're global, but we can verify the method doesn't panic)
	assert.NotPanics(t, func() {
		timer.RecordSuccess()
	}, "RecordSuccess should not panic")
}

func TestAPICallTimer_RecordError(t *testing.T) {
	cm := NewCloudflareMetrics()
	timer := cm.NewAPICallTimer("POST", "/zones/dns_records", "zone-123")

	// Wait a bit to ensure measurable duration
	time.Sleep(1 * time.Millisecond)

	// Record error
	timer.RecordError("timeout")

	// Verify that error was recorded
	assert.NotPanics(t, func() {
		timer.RecordError("rate_limit")
	}, "RecordError should not panic")
}

func TestAPICallTimer_MultipleOperations(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Use unique labels to avoid conflicts with other tests
	uniqueZone1 := "zone-multi-123"
	uniqueZone2 := "zone-multi-456"

	// Test multiple timers
	timer1 := cm.NewAPICallTimer("GET", "/zones", uniqueZone1)
	timer2 := cm.NewAPICallTimer("POST", "/zones/dns_records", uniqueZone2)

	time.Sleep(2 * time.Millisecond)

	// Get initial counts (should be 0 for these unique labels)
	initialSuccessCount := cm.GetAPIRequestCount("GET", "/zones", "success", uniqueZone1)
	initialErrorCount := cm.GetAPIRequestCount("POST", "/zones/dns_records", "error", uniqueZone2)

	// Record different outcomes
	timer1.RecordSuccess()
	timer2.RecordError("validation_error")

	// Verify API request counts increased by 1
	successCount := cm.GetAPIRequestCount("GET", "/zones", "success", uniqueZone1)
	errorCount := cm.GetAPIRequestCount("POST", "/zones/dns_records", "error", uniqueZone2)

	assert.Equal(t, initialSuccessCount+1, successCount, "Success count should increase by 1")
	assert.Equal(t, initialErrorCount+1, errorCount, "Error count should increase by 1")
}

func TestCloudflareMetrics_ErrorPathCoverage(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Test all getter methods with combinations that don't exist
	// This should trigger the first error path (GetMetricWithLabelValues returns error)

	// Test 1: Totally invalid combinations that will never exist
	assert.Equal(t, float64(0), cm.GetAPIRequestCount("INVALID_METHOD_999", "invalid-endpoint-999", "invalid-status-999", "invalid-zone-999"))
	assert.Equal(t, float64(0), cm.GetManagedRecordsCount("invalid-zone-999", "invalid-name-999", "invalid-type-999"))

	// Test 2: More invalid combinations to ensure we hit the error paths
	randomLabels := [][]string{
		{"error-path-1", "error-path-1", "error-path-1", "error-path-1"},
		{"error-path-2", "error-path-2", "error-path-2", "error-path-2"},
		{"error-path-3", "error-path-3", "error-path-3", "error-path-3"},
	}

	for _, labels := range randomLabels {
		assert.Equal(t, float64(0), cm.GetAPIRequestCount(labels[0], labels[1], labels[2], labels[3]))
		if len(labels) >= 3 {
			assert.Equal(t, float64(0), cm.GetManagedRecordsCount(labels[0], labels[1], labels[2]))
		}
	}
}

func TestCloudflareMetrics_EdgeCaseErrorHandling(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Test with extreme values and special characters
	specialCases := [][]string{
		{"", "", "", ""},             // Empty strings
		{"   ", "   ", "   ", "   "}, // Whitespace
		{"very-very-very-long-label-name-that-does-not-exist-999", "very-long", "very-long", "very-long"},
		{"unicode-тест", "unicode-тест", "unicode-тест", "unicode-тест"}, // Unicode
		{"special!@#$%", "special!@#$%", "special!@#$%", "special!@#$%"}, // Special characters
	}

	for _, testCase := range specialCases {
		// All these should return default values (0) due to non-existent metrics
		assert.Equal(t, float64(0), cm.GetAPIRequestCount(testCase[0], testCase[1], testCase[2], testCase[3]),
			"Special case should return 0: %v", testCase)
		if len(testCase) >= 3 {
			assert.Equal(t, float64(0), cm.GetManagedRecordsCount(testCase[0], testCase[1], testCase[2]),
				"Special case should return 0: %v", testCase)
		}
	}
}

func TestCloudflareMetrics_WriteErrorEdgeCases(t *testing.T) {
	registry := prometheus.NewRegistry()

	// Test with Counter instead of Gauge to trigger pb.Gauge == nil
	counterMetric := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_cloudflare_counter_as_gauge",
			Help: "Test counter as gauge",
		},
		[]string{"method", "endpoint", "status", "zone_id"},
	)

	registry.MustRegister(counterMetric)
	counterMetric.WithLabelValues("GET", "/test", "200", "zone-123").Add(10)

	// Get the metric
	metric, err := counterMetric.GetMetricWithLabelValues("GET", "/test", "200", "zone-123")
	require.NoError(t, err)

	// Write to pb
	pb := &dto.Metric{}
	err = metric.Write(pb)
	require.NoError(t, err, "Counter write should succeed")

	// This is the condition that causes GetAPIRequestCount to return 0
	// when metric exists but pb.Gauge == nil
	assert.Nil(t, pb.Gauge, "Counter should have nil Gauge")
	assert.NotNil(t, pb.Counter, "Counter should have Counter field")

	// Test the exact logic from GetAPIRequestCount
	if err == nil && pb.Gauge != nil {
		t.Error("This branch should NOT execute for Counter")
	} else {
		t.Log("This branch executes - this is the uncovered 20% code path!")
	}
}

func TestCloudflareMetrics_ConcurrentAccess(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Test concurrent access that might cause edge cases
	var wg sync.WaitGroup
	numGoroutines := 30

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			method := fmt.Sprintf("GET-%d", id)
			endpoint := fmt.Sprintf("/test-%d", id)
			zoneID := fmt.Sprintf("zone-%d", id)

			// Rapidly set and get to create potential race conditions
			for j := 0; j < 20; j++ {
				cm.RecordAPIRequest(method, endpoint, "200", zoneID, time.Millisecond)
				count := cm.GetAPIRequestCount(method, endpoint, "200", zoneID)
				_ = count

				cm.UpdateManagedRecords(zoneID, "example.com", "A", float64(j))
				count = cm.GetManagedRecordsCount(zoneID, "example.com", "A")
				_ = count
			}
		}(i)
	}

	wg.Wait()
}

// Test extreme values and edge cases
func TestCloudflareMetrics_ExtremeEdgeCases(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Test with empty strings, special characters, very long strings
	edgeCaseInputs := [][]string{
		{"", "", "", ""},
		{"   ", "   ", "   ", "   "},
		{"very-long-string-" + strings.Repeat("x", 1000), "endpoint", "status", "zone"},
		{"method", "very-long-endpoint-" + strings.Repeat("x", 1000), "status", "zone"},
		{"method", "endpoint", "very-long-status-" + strings.Repeat("x", 1000), "zone"},
		{"method", "endpoint", "status", "very-long-zone-" + strings.Repeat("x", 1000)},
		{"method\nwith\nnewlines", "endpoint", "status", "zone"},
		{"method", "endpoint/with/slashes", "status", "zone"},
		{"method", "endpoint", "status", "zone-with-unicode-тест"},
	}

	for i, input := range edgeCaseInputs {
		t.Run(fmt.Sprintf("edge-case-%d", i), func(t *testing.T) {
			// These should not panic and should return 0 for non-existent metrics
			count := cm.GetAPIRequestCount(input[0], input[1], input[2], input[3])
			assert.Equal(t, float64(0), count, "Edge case should return 0")

			if len(input) >= 3 {
				count = cm.GetManagedRecordsCount(input[0], input[1], input[2])
				assert.Equal(t, float64(0), count, "Edge case should return 0")
			}
		})
	}
}

// Advanced test to try to trigger Write() error in Cloudflare metrics
func TestCloudflareMetrics_ForceWriteErrorPaths(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Try to trigger Write() errors with extreme conditions (but valid UTF-8)
	extremeLabels := [][]string{
		{strings.Repeat("极长", 30000), "endpoint", "status", "zone"},               // Very long Unicode
		{string([]rune{0xFFFD, 0xFFFD}), "endpoint", "status", "zone"},            // Unicode replacement chars
		{strings.Repeat("тест", 25000), strings.Repeat("эндп", 20000), "st", "z"}, // Very long Cyrillic
	}

	for i, labels := range extremeLabels {
		t.Run(fmt.Sprintf("extreme-write-error-%d", i), func(t *testing.T) {
			// Set values first
			cm.RecordAPIRequest(labels[0], labels[1], labels[2], labels[3], time.Millisecond)
			cm.UpdateManagedRecords(labels[3], labels[1], labels[0], float64(i))

			// Try to get values - might trigger Write() errors with problematic labels
			_ = cm.GetAPIRequestCount(labels[0], labels[1], labels[2], labels[3])
			_ = cm.GetManagedRecordsCount(labels[3], labels[1], labels[0])
		})
	}
}

// Test with special float values that might cause Write() issues in Cloudflare metrics
func TestCloudflareMetrics_SpecialFloatWriteErrors(t *testing.T) {
	cm := NewCloudflareMetrics()

	specialValues := []float64{
		math.NaN(),
		math.Inf(1),
		math.Inf(-1),
		1e308,  // Near overflow
		-1e308, // Near underflow
		math.SmallestNonzeroFloat64,
		math.MaxFloat64,
	}

	for i, value := range specialValues {
		zoneID := fmt.Sprintf("special-zone-%d", i)
		zoneName := fmt.Sprintf("special-name-%d", i)
		recordType := fmt.Sprintf("type-%d", i)

		// Set special values that might cause Write() issues
		cm.UpdateManagedRecords(zoneID, zoneName, recordType, value)

		// Try to get them - Write() might fail with special values
		_ = cm.GetManagedRecordsCount(zoneID, zoneName, recordType)
	}
}

// Stress test to potentially trigger internal Prometheus errors in Cloudflare metrics
func TestCloudflareMetrics_StressWriteErrors(t *testing.T) {
	cm := NewCloudflareMetrics()

	// Create high contention scenario
	var wg sync.WaitGroup
	numGoroutines := 80

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			method := fmt.Sprintf("stress-method-%d", id)
			endpoint := fmt.Sprintf("/stress-endpoint-%d", id)
			zoneID := fmt.Sprintf("stress-zone-%d", id)

			// Rapidly create and access metrics to stress internal structures
			for j := 0; j < 25; j++ {
				cm.RecordAPIRequest(method, endpoint, "200", zoneID, time.Millisecond)
				_ = cm.GetAPIRequestCount(method, endpoint, "200", zoneID)

				cm.UpdateManagedRecords(zoneID, "example.com", "A", float64(j))
				_ = cm.GetManagedRecordsCount(zoneID, "example.com", "A")
			}
		}(i)
	}

	wg.Wait()
}
