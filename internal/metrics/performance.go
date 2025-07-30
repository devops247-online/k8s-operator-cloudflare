package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Queue depth metric - number of pending reconcile requests
	queueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_operator_queue_depth",
			Help: "Number of pending reconcile requests in the controller queue",
		},
		[]string{"controller", "namespace"},
	)

	// Reconcile rate metric - reconciles per second
	reconcileRate = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudflare_operator_reconcile_rate_total",
			Help: "Total number of reconciles processed",
		},
		[]string{"controller", "result", "namespace"}, // result: success, error, requeue
	)

	// Error rate metric - errors per minute
	errorRate = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudflare_operator_error_rate_total",
			Help: "Total number of errors encountered during reconciliation",
		},
		[]string{"controller", "error_type", "namespace"},
	)

	// API response time metric
	apiResponseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cloudflare_operator_api_response_time_seconds",
			Help:    "Response time of Cloudflare API calls in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation", "status_code", "namespace"},
	)

	// Reconcile duration metric
	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cloudflare_operator_reconcile_duration_seconds",
			Help:    "Time taken to complete reconciliation in seconds",
			Buckets: []float64{0.1, 0.5, 1.0, 2.5, 5.0, 10.0, 15.0, 30.0},
		},
		[]string{"controller", "result", "namespace"},
	)

	// Resource processing rate
	resourceProcessingRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_operator_resources_per_second",
			Help: "Number of resources processed per second",
		},
		[]string{"controller", "namespace"},
	)

	// Cache hit ratio
	cacheHitRatio = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_operator_cache_hit_ratio",
			Help: "Cache hit ratio for controller operations",
		},
		[]string{"controller", "cache_type"},
	)

	// Memory usage tracking
	memoryUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_operator_memory_usage_bytes",
			Help: "Memory usage of the operator in bytes",
		},
		[]string{"type"}, // type: heap, stack, gc
	)

	// CPU usage tracking
	cpuUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_operator_cpu_usage_seconds_total",
			Help: "CPU usage of the operator in seconds",
		},
		[]string{"type"}, // type: user, system
	)

	// Active workers metric
	activeWorkers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_operator_active_workers",
			Help: "Number of active reconcile workers",
		},
		[]string{"controller"},
	)
)

func init() {
	// Register metrics with controller-runtime metrics registry
	metrics.Registry.MustRegister(
		queueDepth,
		reconcileRate,
		errorRate,
		apiResponseTime,
		reconcileDuration,
		resourceProcessingRate,
		cacheHitRatio,
		memoryUsage,
		cpuUsage,
		activeWorkers,
	)
}

// PerformanceMetrics provides methods to update performance metrics
type PerformanceMetrics struct{}

// NewPerformanceMetrics creates a new PerformanceMetrics instance
func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{}
}

// UpdateQueueDepth updates the queue depth metric
func (pm *PerformanceMetrics) UpdateQueueDepth(controller, namespace string, depth float64) {
	queueDepth.WithLabelValues(controller, namespace).Set(depth)
}

// IncReconcileRate increments the reconcile rate metric
func (pm *PerformanceMetrics) IncReconcileRate(controller, result, namespace string) {
	reconcileRate.WithLabelValues(controller, result, namespace).Inc()
}

// IncErrorRate increments the error rate metric
func (pm *PerformanceMetrics) IncErrorRate(controller, errorType, namespace string) {
	errorRate.WithLabelValues(controller, errorType, namespace).Inc()
}

// ObserveAPIResponseTime records API response time
func (pm *PerformanceMetrics) ObserveAPIResponseTime(operation, statusCode, namespace string, duration time.Duration) {
	apiResponseTime.WithLabelValues(operation, statusCode, namespace).Observe(duration.Seconds())
}

// ObserveReconcileDuration records reconcile duration
func (pm *PerformanceMetrics) ObserveReconcileDuration(controller, result, namespace string, duration time.Duration) {
	reconcileDuration.WithLabelValues(controller, result, namespace).Observe(duration.Seconds())
}

// UpdateResourceProcessingRate updates the resource processing rate
func (pm *PerformanceMetrics) UpdateResourceProcessingRate(controller, namespace string, rate float64) {
	resourceProcessingRate.WithLabelValues(controller, namespace).Set(rate)
}

// UpdateCacheHitRatio updates the cache hit ratio
func (pm *PerformanceMetrics) UpdateCacheHitRatio(controller, cacheType string, ratio float64) {
	cacheHitRatio.WithLabelValues(controller, cacheType).Set(ratio)
}

// UpdateMemoryUsage updates memory usage metrics
func (pm *PerformanceMetrics) UpdateMemoryUsage(memType string, bytes float64) {
	memoryUsage.WithLabelValues(memType).Set(bytes)
}

// UpdateCPUUsage updates CPU usage metrics
func (pm *PerformanceMetrics) UpdateCPUUsage(cpuType string, seconds float64) {
	cpuUsage.WithLabelValues(cpuType).Set(seconds)
}

// UpdateActiveWorkers updates the active workers metric
func (pm *PerformanceMetrics) UpdateActiveWorkers(controller string, count float64) {
	activeWorkers.WithLabelValues(controller).Set(count)
}

// GetQueueDepthMetric returns the current queue depth for HPA
func (pm *PerformanceMetrics) GetQueueDepthMetric(controller, namespace string) float64 {
	metric := &dto.Metric{}
	if m, err := queueDepth.GetMetricWithLabelValues(controller, namespace); err == nil {
		if err := m.Write(metric); err == nil && metric.Gauge != nil {
			return *metric.Gauge.Value
		}
	}
	return 0
}

// GetReconcileRateMetric returns the current reconcile rate for HPA
func (pm *PerformanceMetrics) GetReconcileRateMetric(controller, namespace string) float64 {
	// Calculate rate based on success reconciles in the last minute
	metric := &dto.Metric{}
	if m, err := reconcileRate.GetMetricWithLabelValues(controller, "success", namespace); err == nil {
		if err := m.Write(metric); err == nil && metric.Counter != nil {
			return *metric.Counter.Value
		}
	}
	return 0
}

// GetErrorRateMetric returns the current error rate for HPA
func (pm *PerformanceMetrics) GetErrorRateMetric(controller, namespace string) float64 {
	// Sum all error types for the controller and namespace
	totalErrors := 0.0

	// This is a simplified version - in practice, you'd iterate through all error types
	metric := &dto.Metric{}
	if m, err := errorRate.GetMetricWithLabelValues(controller, "reconcile_error", namespace); err == nil {
		if err := m.Write(metric); err == nil && metric.Counter != nil {
			totalErrors += *metric.Counter.Value
		}
	}

	return totalErrors
}
