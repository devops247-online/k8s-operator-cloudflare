package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Business metrics for DNS records by type
	dnsRecordsByType = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_dns_records_by_type_total",
			Help: "Total number of DNS records managed by type",
		},
		[]string{"record_type", "zone_name", "zone_id"},
	)

	// DNS records by status
	dnsRecordsByStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_dns_records_by_status_total",
			Help: "Total number of DNS records by status (active, pending, error)",
		},
		[]string{"status", "zone_name", "zone_id"},
	)

	// Zone health metrics
	zoneHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_zone_health_status",
			Help: "Health status of Cloudflare zones (1 = healthy, 0 = unhealthy)",
		},
		[]string{"zone_id", "zone_name", "health_check"},
	)

	// Operator reconciliation health
	reconciliationHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_reconciliation_health",
			Help: "Health status of reconciliation process (1 = healthy, 0 = unhealthy)",
		},
		[]string{"controller", "namespace", "resource_type"},
	)

	// DNS propagation metrics
	dnsPropagationTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cloudflare_dns_propagation_time_seconds",
			Help:    "Time taken for DNS changes to propagate",
			Buckets: []float64{1, 5, 10, 30, 60, 180, 300, 600},
		},
		[]string{"record_type", "zone_id", "operation"},
	)

	// CRD resource status
	crdResourceStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_crd_resources_total",
			Help: "Total number of CloudflareRecord CRD resources by status",
		},
		[]string{"namespace", "status", "record_type"},
	)

	// Operator uptime
	operatorUptime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_operator_uptime_seconds",
			Help: "Uptime of the Cloudflare operator in seconds",
		},
		[]string{"version", "namespace"},
	)

	// Configuration validity
	configurationValidity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_configuration_validity",
			Help: "Validity of operator configuration (1 = valid, 0 = invalid)",
		},
		[]string{"config_type", "namespace"},
	)

	// SLI metrics for SLO tracking
	reconcileSuccessRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_reconcile_success_rate",
			Help: "Success rate of reconciliation operations (percentage)",
		},
		[]string{"controller", "namespace", "time_window"},
	)

	// Average response time SLI
	averageResponseTime = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_average_response_time_seconds",
			Help: "Average response time for operations (SLI metric)",
		},
		[]string{"operation_type", "time_window"},
	)

	// Error budget remaining
	errorBudgetRemaining = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_error_budget_remaining_percent",
			Help: "Remaining error budget percentage for SLO tracking",
		},
		[]string{"slo_name", "time_window"},
	)
)

func init() {
	// Register business metrics with controller-runtime metrics registry
	metrics.Registry.MustRegister(
		dnsRecordsByType,
		dnsRecordsByStatus,
		zoneHealthStatus,
		reconciliationHealth,
		dnsPropagationTime,
		crdResourceStatus,
		operatorUptime,
		configurationValidity,
		reconcileSuccessRate,
		averageResponseTime,
		errorBudgetRemaining,
	)
}

// BusinessMetrics provides methods to update business-related metrics
type BusinessMetrics struct{}

// NewBusinessMetrics creates a new BusinessMetrics instance
func NewBusinessMetrics() *BusinessMetrics {
	return &BusinessMetrics{}
}

// UpdateDNSRecordsByType updates the count of DNS records by type
func (bm *BusinessMetrics) UpdateDNSRecordsByType(recordType, zoneName, zoneID string, count float64) {
	dnsRecordsByType.WithLabelValues(recordType, zoneName, zoneID).Set(count)
}

// UpdateDNSRecordsByStatus updates the count of DNS records by status
func (bm *BusinessMetrics) UpdateDNSRecordsByStatus(status, zoneName, zoneID string, count float64) {
	dnsRecordsByStatus.WithLabelValues(status, zoneName, zoneID).Set(count)
}

// UpdateZoneHealth updates the health status of a zone
func (bm *BusinessMetrics) UpdateZoneHealth(zoneID, zoneName, healthCheck string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	zoneHealthStatus.WithLabelValues(zoneID, zoneName, healthCheck).Set(value)
}

// UpdateReconciliationHealth updates the health status of reconciliation
func (bm *BusinessMetrics) UpdateReconciliationHealth(controller, namespace, resourceType string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	reconciliationHealth.WithLabelValues(controller, namespace, resourceType).Set(value)
}

// ObserveDNSPropagationTime records DNS propagation time
func (bm *BusinessMetrics) ObserveDNSPropagationTime(recordType, zoneID, operation string, seconds float64) {
	dnsPropagationTime.WithLabelValues(recordType, zoneID, operation).Observe(seconds)
}

// UpdateCRDResourceStatus updates the count of CRD resources by status
func (bm *BusinessMetrics) UpdateCRDResourceStatus(namespace, status, recordType string, count float64) {
	crdResourceStatus.WithLabelValues(namespace, status, recordType).Set(count)
}

// UpdateOperatorUptime updates the operator uptime
func (bm *BusinessMetrics) UpdateOperatorUptime(version, namespace string, seconds float64) {
	operatorUptime.WithLabelValues(version, namespace).Set(seconds)
}

// UpdateConfigurationValidity updates configuration validity status
func (bm *BusinessMetrics) UpdateConfigurationValidity(configType, namespace string, valid bool) {
	value := 0.0
	if valid {
		value = 1.0
	}
	configurationValidity.WithLabelValues(configType, namespace).Set(value)
}

// UpdateReconcileSuccessRate updates the reconciliation success rate SLI
func (bm *BusinessMetrics) UpdateReconcileSuccessRate(controller, namespace, timeWindow string, rate float64) {
	reconcileSuccessRate.WithLabelValues(controller, namespace, timeWindow).Set(rate)
}

// UpdateAverageResponseTime updates the average response time SLI
func (bm *BusinessMetrics) UpdateAverageResponseTime(operationType, timeWindow string, seconds float64) {
	averageResponseTime.WithLabelValues(operationType, timeWindow).Set(seconds)
}

// UpdateErrorBudgetRemaining updates the remaining error budget percentage
func (bm *BusinessMetrics) UpdateErrorBudgetRemaining(sloName, timeWindow string, percent float64) {
	errorBudgetRemaining.WithLabelValues(sloName, timeWindow).Set(percent)
}

// GetDNSRecordsByType returns the current count of DNS records by type
func (bm *BusinessMetrics) GetDNSRecordsByType(recordType, zoneName, zoneID string) float64 {
	if metric, err := dnsRecordsByType.GetMetricWithLabelValues(recordType, zoneName, zoneID); err == nil {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err == nil && pb.Gauge != nil {
			return *pb.Gauge.Value
		}
	}
	return 0
}

// GetCRDResourceStatus returns the current count of CRD resources by status
func (bm *BusinessMetrics) GetCRDResourceStatus(namespace, status, recordType string) float64 {
	if metric, err := crdResourceStatus.GetMetricWithLabelValues(namespace, status, recordType); err == nil {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err == nil && pb.Gauge != nil {
			return *pb.Gauge.Value
		}
	}
	return 0
}

// GetReconcileSuccessRate returns the current reconciliation success rate
func (bm *BusinessMetrics) GetReconcileSuccessRate(controller, namespace, timeWindow string) float64 {
	if metric, err := reconcileSuccessRate.GetMetricWithLabelValues(controller, namespace, timeWindow); err == nil {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err == nil && pb.Gauge != nil {
			return *pb.Gauge.Value
		}
	}
	return 0
}

// IsZoneHealthy checks if a zone is healthy based on metrics
func (bm *BusinessMetrics) IsZoneHealthy(zoneID, zoneName, healthCheck string) bool {
	if metric, err := zoneHealthStatus.GetMetricWithLabelValues(zoneID, zoneName, healthCheck); err == nil {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err == nil && pb.Gauge != nil {
			return *pb.Gauge.Value == 1.0
		}
	}
	return false
}

// IsReconciliationHealthy checks if reconciliation is healthy
func (bm *BusinessMetrics) IsReconciliationHealthy(controller, namespace, resourceType string) bool {
	if metric, err := reconciliationHealth.GetMetricWithLabelValues(controller, namespace, resourceType); err == nil {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err == nil && pb.Gauge != nil {
			return *pb.Gauge.Value == 1.0
		}
	}
	return false
}
