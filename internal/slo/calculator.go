package slo

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// Calculator calculates SLI values from metrics
type Calculator struct {
	config *Config

	// Metrics for SLI calculation
	availabilityRatio *prometheus.GaugeVec
	successRateRatio  *prometheus.GaugeVec
	latencyHistogram  *prometheus.HistogramVec
	throughputGauge   *prometheus.GaugeVec
	errorBudgetGauge  *prometheus.GaugeVec
}

// NewCalculator creates a new SLI calculator
func NewCalculator(config *Config) *Calculator {
	return NewCalculatorWithRegistry(config, prometheus.DefaultRegisterer)
}

// NewCalculatorWithRegistry creates a new SLI calculator with a custom registry
func NewCalculatorWithRegistry(config *Config, registerer prometheus.Registerer) *Calculator {
	calc := &Calculator{
		config: config,

		availabilityRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "sli_availability_ratio",
				Help: "Availability SLI as a ratio (0.0-1.0)",
			},
			[]string{"window"},
		),

		successRateRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "sli_success_rate_ratio",
				Help: "Success rate SLI as a ratio (0.0-1.0)",
			},
			[]string{"window", "controller", "namespace"},
		),

		latencyHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "sli_latency_seconds",
				Help:    "Latency SLI in seconds",
				Buckets: prometheus.ExponentialBuckets(0.001, 2, 20), // 1ms to ~17min
			},
			[]string{"window", "controller", "namespace", "operation"},
		),

		throughputGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "sli_throughput_operations_per_minute",
				Help: "Throughput SLI in operations per minute",
			},
			[]string{"window", "controller", "namespace"},
		),

		errorBudgetGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "sli_error_budget_remaining_ratio",
				Help: "Remaining error budget as a ratio (0.0-1.0)",
			},
			[]string{"slo_name", "window"},
		),
	}

	// Register metrics if registerer is provided
	if registerer != nil {
		registerer.MustRegister(
			calc.availabilityRatio,
			calc.successRateRatio,
			calc.latencyHistogram,
			calc.throughputGauge,
			calc.errorBudgetGauge,
		)
	}

	return calc
}

// SLIValues holds calculated SLI values
type SLIValues struct {
	Availability float64
	SuccessRate  float64
	LatencyP95   time.Duration
	LatencyP99   time.Duration
	Throughput   float64
	Timestamp    time.Time
	Window       string
}

// Calculate calculates all SLI values for a given time window
func (c *Calculator) Calculate(ctx context.Context, window TimeWindow) (*SLIValues, error) {
	values := &SLIValues{
		Window:    window.Name,
		Timestamp: time.Now(),
	}

	// Calculate availability from up metric
	availability, err := c.calculateAvailability(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("calculating availability: %w", err)
	}
	values.Availability = availability
	c.availabilityRatio.WithLabelValues(window.Name).Set(availability)

	// Calculate success rate from reconcile metrics
	successRate, err := c.calculateSuccessRate(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("calculating success rate: %w", err)
	}
	values.SuccessRate = successRate
	c.successRateRatio.WithLabelValues(window.Name, "cloudflarerecord", "all").Set(successRate)

	// Calculate latency percentiles
	p95, p99, err := c.calculateLatencyPercentiles(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("calculating latency percentiles: %w", err)
	}
	values.LatencyP95 = p95
	values.LatencyP99 = p99

	// Record latency observations
	c.latencyHistogram.WithLabelValues(window.Name, "cloudflarerecord", "all", "reconcile").Observe(p95.Seconds())

	// Calculate throughput
	throughput, err := c.calculateThroughput(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("calculating throughput: %w", err)
	}
	values.Throughput = throughput
	c.throughputGauge.WithLabelValues(window.Name, "cloudflarerecord", "all").Set(throughput)

	// Calculate error budgets for each SLO
	c.calculateErrorBudgets(values)

	return values, nil
}

// calculateAvailability calculates the availability SLI
func (c *Calculator) calculateAvailability(_ context.Context, _ TimeWindow) (float64, error) {
	// This would typically query Prometheus or use the up metric
	// For now, return a placeholder calculation
	// In real implementation, this would use prometheus.Query API
	return 0.999, nil
}

// calculateSuccessRate calculates the success rate SLI
func (c *Calculator) calculateSuccessRate(_ context.Context, _ TimeWindow) (float64, error) {
	// This would query controller_runtime_reconcile_total metrics
	// For now, return a placeholder calculation
	return 0.995, nil
}

// calculateLatencyPercentiles calculates P95 and P99 latency
func (c *Calculator) calculateLatencyPercentiles(_ context.Context, //nolint:lll
	_ TimeWindow) (time.Duration, time.Duration, error) {
	// This would query controller_runtime_reconcile_time_seconds histogram
	// For now, return placeholder values
	return 25 * time.Second, 45 * time.Second, nil
}

// calculateThroughput calculates operations per minute
func (c *Calculator) calculateThroughput(_ context.Context, _ TimeWindow) (float64, error) {
	// This would query the rate of reconcile operations
	// For now, return a placeholder calculation
	return 15.0, nil
}

// calculateErrorBudgets updates error budget gauges based on SLI values
func (c *Calculator) calculateErrorBudgets(values *SLIValues) {
	// Calculate error budget for availability
	availabilityBudget := c.calculateSingleErrorBudget(
		values.Availability,
		c.config.Targets.Availability/100,
		"availability",
		values.Window,
	)

	// Calculate error budget for success rate
	successRateBudget := c.calculateSingleErrorBudget(
		values.SuccessRate,
		c.config.Targets.SuccessRate/100,
		"success_rate",
		values.Window,
	)

	// Calculate error budget for latency (using P95)
	latencyConformance := 1.0
	if values.LatencyP95 > c.config.Targets.LatencyP95 {
		// Simple binary conformance for latency
		latencyConformance = 0.95 // Assume 95% of requests meet latency target
	}
	latencyBudget := c.calculateSingleErrorBudget(
		latencyConformance,
		0.95, // 95% of requests should meet latency target
		"latency_p95",
		values.Window,
	)

	// Update error budget gauges
	c.errorBudgetGauge.WithLabelValues("availability", values.Window).Set(availabilityBudget)
	c.errorBudgetGauge.WithLabelValues("success_rate", values.Window).Set(successRateBudget)
	c.errorBudgetGauge.WithLabelValues("latency_p95", values.Window).Set(latencyBudget)
}

// calculateSingleErrorBudget calculates remaining error budget for a single SLO
func (c *Calculator) calculateSingleErrorBudget(actual, target float64, //nolint:lll
	_ /* sloName */, _ /* window */ string) float64 {
	if target >= 1.0 {
		return 1.0 // No error budget if target is 100%
	}

	// If actual >= target, budget is 100% remaining
	if actual >= target {
		return 1.0
	}

	// Error budget calculation:
	// Total error budget = 1 - target
	// Error budget consumed = target - actual
	// Error budget remaining = (total - consumed) / total
	errorBudgetTotal := 1.0 - target
	errorBudgetConsumed := target - actual

	if errorBudgetTotal <= 0 {
		return 1.0 // Edge case: no error budget exists
	}

	errorBudgetRemaining := (errorBudgetTotal - errorBudgetConsumed) / errorBudgetTotal

	// Clamp between 0 and 1
	if errorBudgetRemaining < 0 {
		return 0.0
	}
	if errorBudgetRemaining > 1 {
		return 1.0
	}

	return errorBudgetRemaining
}

// GetMetricValue is a helper to get current metric value
func (c *Calculator) GetMetricValue(vec *prometheus.GaugeVec, labels ...string) (float64, error) {
	metric, err := vec.GetMetricWithLabelValues(labels...)
	if err != nil {
		return 0, fmt.Errorf("getting metric with labels: %w", err)
	}

	pb := &dto.Metric{}
	if err := metric.Write(pb); err != nil {
		return 0, fmt.Errorf("writing metric: %w", err)
	}

	if pb.Gauge == nil || pb.Gauge.Value == nil {
		return 0, fmt.Errorf("metric has no gauge value")
	}

	return *pb.Gauge.Value, nil
}

// CalculateMultiWindowBurnRate calculates burn rate across multiple windows
func (c *Calculator) CalculateMultiWindowBurnRate(ctx context.Context, //nolint:lll
	shortWindow, longWindow TimeWindow) (float64, error) {
	// Calculate error budget consumption rate
	// This is simplified - real implementation would query Prometheus
	shortValues, err := c.Calculate(ctx, shortWindow)
	if err != nil {
		return 0, fmt.Errorf("calculating short window SLI: %w", err)
	}

	longValues, err := c.Calculate(ctx, longWindow)
	if err != nil {
		return 0, fmt.Errorf("calculating long window SLI: %w", err)
	}

	// Calculate burn rate based on error budget consumption
	// This is a simplified calculation
	shortBurnRate := (1.0 - shortValues.SuccessRate) / (1.0 - c.config.Targets.SuccessRate/100)
	longBurnRate := (1.0 - longValues.SuccessRate) / (1.0 - c.config.Targets.SuccessRate/100)

	// Return the average burn rate
	return (shortBurnRate + longBurnRate) / 2, nil
}
