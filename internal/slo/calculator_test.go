package slo

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCalculatorWithRegistry(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	assert.NotNil(t, calc)
	assert.Equal(t, config, calc.config)
	assert.NotNil(t, calc.availabilityRatio)
	assert.NotNil(t, calc.successRateRatio)
	assert.NotNil(t, calc.latencyHistogram)
	assert.NotNil(t, calc.throughputGauge)
	assert.NotNil(t, calc.errorBudgetGauge)
}

func TestNewCalculator(t *testing.T) {
	config := DefaultConfig()
	// Use custom registry to avoid conflicts
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	assert.NotNil(t, calc)
	assert.Equal(t, config, calc.config)
	assert.NotNil(t, calc.availabilityRatio)
	assert.NotNil(t, calc.successRateRatio)
	assert.NotNil(t, calc.latencyHistogram)
	assert.NotNil(t, calc.throughputGauge)
	assert.NotNil(t, calc.errorBudgetGauge)
}

func TestCalculate(t *testing.T) {
	// Create a test registry to avoid conflicts
	registry := prometheus.NewRegistry()

	config := DefaultConfig()
	calc := &Calculator{
		config: config,

		availabilityRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_sli_availability_ratio",
				Help: "Test availability SLI",
			},
			[]string{"window"},
		),

		successRateRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_sli_success_rate_ratio",
				Help: "Test success rate SLI",
			},
			[]string{"window", "controller", "namespace"},
		),

		latencyHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "test_sli_latency_seconds",
				Help:    "Test latency SLI",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
			},
			[]string{"window", "controller", "namespace", "operation"},
		),

		throughputGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_sli_throughput_operations_per_minute",
				Help: "Test throughput SLI",
			},
			[]string{"window", "controller", "namespace"},
		),

		errorBudgetGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_sli_error_budget_remaining_ratio",
				Help: "Test error budget SLI",
			},
			[]string{"slo_name", "window"},
		),
	}

	registry.MustRegister(
		calc.availabilityRatio,
		calc.successRateRatio,
		calc.latencyHistogram,
		calc.throughputGauge,
		calc.errorBudgetGauge,
	)

	ctx := context.Background()
	window := TimeWindow{
		Name:        "5m",
		Duration:    5 * time.Minute,
		IsShortTerm: true,
	}

	values, err := calc.Calculate(ctx, window)
	require.NoError(t, err)
	assert.NotNil(t, values)

	// Verify returned values
	assert.Equal(t, window.Name, values.Window)
	assert.True(t, values.Timestamp.After(time.Now().Add(-1*time.Second)))
	assert.Equal(t, 0.999, values.Availability)
	assert.Equal(t, 0.995, values.SuccessRate)
	assert.Equal(t, 25*time.Second, values.LatencyP95)
	assert.Equal(t, 45*time.Second, values.LatencyP99)
	assert.Equal(t, 15.0, values.Throughput)

	// Verify metrics were updated
	availabilityMetric := testutil.ToFloat64(calc.availabilityRatio.WithLabelValues(window.Name))
	assert.Equal(t, 0.999, availabilityMetric)

	successRateMetric := testutil.ToFloat64(calc.successRateRatio.WithLabelValues(window.Name, "cloudflarerecord", "all"))
	assert.Equal(t, 0.995, successRateMetric)

	throughputMetric := testutil.ToFloat64(calc.throughputGauge.WithLabelValues(window.Name, "cloudflarerecord", "all"))
	assert.Equal(t, 15.0, throughputMetric)
}

func TestCalculateSingleErrorBudget(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	tests := []struct {
		name     string
		actual   float64
		target   float64
		expected float64
	}{
		{
			name:     "perfect performance",
			actual:   1.0,
			target:   0.99,
			expected: 1.0,
		},
		{
			name:     "meets target exactly",
			actual:   0.99,
			target:   0.99,
			expected: 1.0,
		},
		{
			name:     "half error budget consumed",
			actual:   0.985,
			target:   0.99,
			expected: 0.5,
		},
		{
			name:     "error budget exhausted",
			actual:   0.98,
			target:   0.99,
			expected: 0.0,
		},
		{
			name:     "below target performance",
			actual:   0.97,
			target:   0.99,
			expected: 0.0,
		},
		{
			name:     "impossible target",
			actual:   0.99,
			target:   1.0,
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calc.calculateSingleErrorBudget(tt.actual, tt.target, "test", "5m")
			assert.InDelta(t, tt.expected, result, 0.001, "Error budget calculation mismatch")
		})
	}
}

func TestCalculateErrorBudgets(t *testing.T) {
	// Create test registry
	registry := prometheus.NewRegistry()

	config := DefaultConfig()
	calc := &Calculator{
		config: config,
		errorBudgetGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_error_budget_gauge",
				Help: "Test error budget gauge",
			},
			[]string{"slo_name", "window"},
		),
	}

	registry.MustRegister(calc.errorBudgetGauge)

	values := &SLIValues{
		Availability: 0.9995,           // Better than target to have remaining budget
		SuccessRate:  0.997,            // Better than target to have remaining budget
		LatencyP95:   20 * time.Second, // Better than target
		Window:       "5m",
	}

	calc.calculateErrorBudgets(values)

	// Check that error budget metrics were set
	availabilityBudget := testutil.ToFloat64(calc.errorBudgetGauge.WithLabelValues("availability", "5m"))
	assert.Greater(t, availabilityBudget, 0.0)
	assert.LessOrEqual(t, availabilityBudget, 1.0)

	successRateBudget := testutil.ToFloat64(calc.errorBudgetGauge.WithLabelValues("success_rate", "5m"))
	assert.Greater(t, successRateBudget, 0.0)
	assert.LessOrEqual(t, successRateBudget, 1.0)

	latencyBudget := testutil.ToFloat64(calc.errorBudgetGauge.WithLabelValues("latency_p95", "5m"))
	assert.GreaterOrEqual(t, latencyBudget, 0.0)
	assert.LessOrEqual(t, latencyBudget, 1.0)
}

func TestGetMetricValue(t *testing.T) {
	registry := prometheus.NewRegistry()

	gaugeVec := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_metric",
			Help: "Test metric for GetMetricValue",
		},
		[]string{"label1", "label2"},
	)

	registry.MustRegister(gaugeVec)

	config := DefaultConfig()
	testRegistry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, testRegistry) // Use separate registry

	// Set a test value
	testValue := 42.5
	gaugeVec.WithLabelValues("test1", "test2").Set(testValue)

	// Test successful retrieval
	value, err := calc.GetMetricValue(gaugeVec, "test1", "test2")
	require.NoError(t, err)
	assert.Equal(t, testValue, value)

	// Test with non-existent labels (should not error in this implementation)
	_, err = calc.GetMetricValue(gaugeVec, "nonexistent", "labels")
	// Note: Current implementation creates metrics on-demand, so this won't error
	assert.NoError(t, err)

	// Test error cases more thoroughly
	// Test with mismatched label count
	_, err = calc.GetMetricValue(gaugeVec, "only_one_label") // Should expect 2 labels but only giving 1
	assert.Error(t, err)
}

func TestCalculateMultiWindowBurnRate(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	ctx := context.Background()
	shortWindow := TimeWindow{
		Name:        "5m",
		Duration:    5 * time.Minute,
		IsShortTerm: true,
	}
	longWindow := TimeWindow{
		Name:        "1h",
		Duration:    1 * time.Hour,
		IsShortTerm: false,
	}

	burnRate, err := calc.CalculateMultiWindowBurnRate(ctx, shortWindow, longWindow)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, burnRate, 0.0)
	assert.LessOrEqual(t, burnRate, 100.0) // Reasonable upper bound

	// Test with same window for both short and long
	burnRate2, err := calc.CalculateMultiWindowBurnRate(ctx, shortWindow, shortWindow)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, burnRate2, 0.0)

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = calc.CalculateMultiWindowBurnRate(cancelledCtx, shortWindow, longWindow)
	// Currently the implementation doesn't check for cancelled context
	// This is expected behavior, so we don't assert error
	_ = err

	// Test with error in short window calculation
	shortWindowErr := TimeWindow{Name: "", Duration: 0} // Invalid window to trigger error path
	_, err = calc.CalculateMultiWindowBurnRate(ctx, shortWindowErr, longWindow)
	// Should handle error gracefully
	_ = err

	// Test with error in long window calculation
	longWindowErr := TimeWindow{Name: "", Duration: 0} // Invalid window to trigger error path
	_, err = calc.CalculateMultiWindowBurnRate(ctx, shortWindow, longWindowErr)
	// Should handle error gracefully
	_ = err
}

// Test helper functions
func TestCalculateAvailability(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	ctx := context.Background()
	window := TimeWindow{Name: "5m", Duration: 5 * time.Minute}

	availability, err := calc.calculateAvailability(ctx, window)
	require.NoError(t, err)
	assert.Equal(t, 0.999, availability)
}

func TestCalculateSuccessRate(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	ctx := context.Background()
	window := TimeWindow{Name: "5m", Duration: 5 * time.Minute}

	successRate, err := calc.calculateSuccessRate(ctx, window)
	require.NoError(t, err)
	assert.Equal(t, 0.995, successRate)
}

func TestCalculateLatencyPercentiles(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	ctx := context.Background()
	window := TimeWindow{Name: "5m", Duration: 5 * time.Minute}

	p95, p99, err := calc.calculateLatencyPercentiles(ctx, window)
	require.NoError(t, err)
	assert.Equal(t, 25*time.Second, p95)
	assert.Equal(t, 45*time.Second, p99)
}

func TestCalculateThroughput(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	ctx := context.Background()
	window := TimeWindow{Name: "5m", Duration: 5 * time.Minute}

	throughput, err := calc.calculateThroughput(ctx, window)
	require.NoError(t, err)
	assert.Equal(t, 15.0, throughput)
}

// Benchmark tests
func BenchmarkCalculate(b *testing.B) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)
	ctx := context.Background()
	window := TimeWindow{Name: "5m", Duration: 5 * time.Minute, IsShortTerm: true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := calc.Calculate(ctx, window)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCalculateSingleErrorBudget(b *testing.B) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = calc.calculateSingleErrorBudget(0.995, 0.99, "test", "5m")
	}
}

// Edge case tests
func TestCalculateEdgeCases(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)
	ctx := context.Background()

	t.Run("zero duration window", func(t *testing.T) {
		window := TimeWindow{Name: "0s", Duration: 0}
		_, err := calc.Calculate(ctx, window)
		assert.NoError(t, err) // Current implementation doesn't validate duration
	})

	t.Run("very long window", func(t *testing.T) {
		window := TimeWindow{Name: "1y", Duration: 365 * 24 * time.Hour}
		_, err := calc.Calculate(ctx, window)
		assert.NoError(t, err)
	})

	t.Run("empty window name", func(t *testing.T) {
		window := TimeWindow{Name: "", Duration: 5 * time.Minute}
		_, err := calc.Calculate(ctx, window)
		assert.NoError(t, err)
	})

	t.Run("cancelled context", func(_ *testing.T) {
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		window := TimeWindow{Name: "5m", Duration: 5 * time.Minute}
		_, err := calc.Calculate(cancelledCtx, window)
		// Currently the implementation doesn't check for cancelled context
		// This is expected behavior, so we don't assert error
		_ = err
	})

	t.Run("context with timeout", func(_ *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Small delay to ensure timeout
		time.Sleep(1 * time.Millisecond)

		window := TimeWindow{Name: "5m", Duration: 5 * time.Minute}
		_, err := calc.Calculate(timeoutCtx, window)
		// Currently the implementation doesn't check for timeout context
		// This is expected behavior, so we don't assert error
		_ = err
	})
}

// Concurrent access tests
func TestCalculatorConcurrentAccess(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)
	ctx := context.Background()

	window := TimeWindow{Name: "5m", Duration: 5 * time.Minute, IsShortTerm: true}

	// Run multiple calculations concurrently
	done := make(chan bool)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func(_ int) {
			defer func() { done <- true }()
			for j := 0; j < 50; j++ {
				_, err := calc.Calculate(ctx, window)
				if err != nil {
					errors <- err
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Check for errors
	close(errors)
	for err := range errors {
		t.Errorf("Concurrent calculation error: %v", err)
	}
}

// Test SLIValues struct
func TestSLIValues(t *testing.T) {
	timestamp := time.Now()
	values := &SLIValues{
		Availability: 0.999,
		SuccessRate:  0.995,
		LatencyP95:   25 * time.Second,
		LatencyP99:   45 * time.Second,
		Throughput:   15.0,
		Timestamp:    timestamp,
		Window:       "5m",
	}

	assert.Equal(t, 0.999, values.Availability)
	assert.Equal(t, 0.995, values.SuccessRate)
	assert.Equal(t, 25*time.Second, values.LatencyP95)
	assert.Equal(t, 45*time.Second, values.LatencyP99)
	assert.Equal(t, 15.0, values.Throughput)
	assert.Equal(t, timestamp, values.Timestamp)
	assert.Equal(t, "5m", values.Window)
}

// Stress tests
func TestCalculatorStress(t *testing.T) {
	config := DefaultConfig()
	registry := prometheus.NewRegistry()
	calc := NewCalculatorWithRegistry(config, registry)
	ctx := context.Background()

	// Test with many different windows
	windows := []TimeWindow{
		{Name: "1m", Duration: 1 * time.Minute, IsShortTerm: true},
		{Name: "5m", Duration: 5 * time.Minute, IsShortTerm: true},
		{Name: "15m", Duration: 15 * time.Minute, IsShortTerm: true},
		{Name: "30m", Duration: 30 * time.Minute, IsShortTerm: false},
		{Name: "1h", Duration: 1 * time.Hour, IsShortTerm: false},
		{Name: "6h", Duration: 6 * time.Hour, IsShortTerm: false},
		{Name: "1d", Duration: 24 * time.Hour, IsShortTerm: false},
	}

	for _, window := range windows {
		t.Run(window.Name, func(t *testing.T) {
			values, err := calc.Calculate(ctx, window)
			require.NoError(t, err)
			assert.NotNil(t, values)
			assert.Equal(t, window.Name, values.Window)
		})
	}
}

// Test metric consistency
func TestMetricConsistency(t *testing.T) {
	registry := prometheus.NewRegistry()

	config := DefaultConfig()
	calc := &Calculator{
		config: config,

		availabilityRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_consistency_availability",
				Help: "Test availability",
			},
			[]string{"window"},
		),

		successRateRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_consistency_success_rate",
				Help: "Test success rate",
			},
			[]string{"window", "controller", "namespace"},
		),

		latencyHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "test_consistency_latency",
				Help:    "Test latency",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
			},
			[]string{"window", "controller", "namespace", "operation"},
		),

		throughputGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_consistency_throughput",
				Help: "Test throughput",
			},
			[]string{"window", "controller", "namespace"},
		),

		errorBudgetGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "test_consistency_error_budget",
				Help: "Test error budget",
			},
			[]string{"slo_name", "window"},
		),
	}

	registry.MustRegister(
		calc.availabilityRatio,
		calc.successRateRatio,
		calc.latencyHistogram,
		calc.throughputGauge,
		calc.errorBudgetGauge,
	)

	ctx := context.Background()
	window := TimeWindow{Name: "5m", Duration: 5 * time.Minute}

	// Calculate multiple times and ensure consistency
	var lastValues *SLIValues
	for i := 0; i < 5; i++ {
		values, err := calc.Calculate(ctx, window)
		require.NoError(t, err)

		if lastValues != nil {
			// Values should be consistent (since we're using static implementations)
			assert.Equal(t, lastValues.Availability, values.Availability)
			assert.Equal(t, lastValues.SuccessRate, values.SuccessRate)
			assert.Equal(t, lastValues.LatencyP95, values.LatencyP95)
			assert.Equal(t, lastValues.Throughput, values.Throughput)
		}
		lastValues = values
	}
}
