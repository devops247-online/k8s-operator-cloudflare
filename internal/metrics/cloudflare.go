package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Cloudflare API request metrics
	cloudflareAPIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudflare_api_requests_total",
			Help: "Total number of requests made to Cloudflare API",
		},
		[]string{"method", "endpoint", "status", "zone_id"},
	)

	// Cloudflare API response time
	cloudflareAPIResponseTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cloudflare_api_response_time_seconds",
			Help:    "Response time of Cloudflare API requests in seconds",
			Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"method", "endpoint", "status", "zone_id"},
	)

	// Cloudflare API rate limit metrics
	cloudflareAPIRateLimitRemaining = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_api_rate_limit_remaining",
			Help: "Remaining API rate limit for Cloudflare requests",
		},
		[]string{"endpoint"},
	)

	// Cloudflare API rate limit total
	cloudflareAPIRateLimitTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_api_rate_limit_total",
			Help: "Total API rate limit for Cloudflare requests",
		},
		[]string{"endpoint"},
	)

	// Cloudflare API errors
	cloudflareAPIErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudflare_api_errors_total",
			Help: "Total number of Cloudflare API errors",
		},
		[]string{"method", "endpoint", "error_type", "zone_id"},
	)

	// Cloudflare DNS records managed
	cloudflareDNSRecordsManagedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_dns_records_managed_total",
			Help: "Total number of DNS records currently managed by the operator",
		},
		[]string{"zone_id", "zone_name", "record_type"},
	)

	// Cloudflare zones managed
	cloudflareZonesManagedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_zones_managed_total",
			Help: "Total number of Cloudflare zones managed by the operator",
		},
		[]string{"zone_name", "plan_type"},
	)

	// Cloudflare DNS record operations
	cloudflareDNSRecordOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cloudflare_dns_record_operations_total",
			Help: "Total number of DNS record operations (create, update, delete)",
		},
		[]string{"operation", "record_type", "zone_id", "result"},
	)

	// Cloudflare API request duration by operation
	cloudflareOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "cloudflare_operation_duration_seconds",
			Help:    "Time spent on specific Cloudflare operations",
			Buckets: []float64{0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 30.0},
		},
		[]string{"operation", "zone_id", "result"},
	)

	// Cloudflare zone status
	cloudflareZoneStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cloudflare_zone_status",
			Help: "Status of Cloudflare zones (1 = active, 0 = inactive)",
		},
		[]string{"zone_id", "zone_name", "status"},
	)
)

func init() {
	// Register Cloudflare metrics with controller-runtime metrics registry
	metrics.Registry.MustRegister(
		cloudflareAPIRequestsTotal,
		cloudflareAPIResponseTime,
		cloudflareAPIRateLimitRemaining,
		cloudflareAPIRateLimitTotal,
		cloudflareAPIErrorsTotal,
		cloudflareDNSRecordsManagedTotal,
		cloudflareZonesManagedTotal,
		cloudflareDNSRecordOperationsTotal,
		cloudflareOperationDuration,
		cloudflareZoneStatus,
	)
}

// CloudflareMetrics provides methods to update Cloudflare-specific metrics
type CloudflareMetrics struct{}

// NewCloudflareMetrics creates a new CloudflareMetrics instance
func NewCloudflareMetrics() *CloudflareMetrics {
	return &CloudflareMetrics{}
}

// RecordAPIRequest records a Cloudflare API request
func (cm *CloudflareMetrics) RecordAPIRequest(method, endpoint, status, zoneID string, duration time.Duration) {
	cloudflareAPIRequestsTotal.WithLabelValues(method, endpoint, status, zoneID).Inc()
	cloudflareAPIResponseTime.WithLabelValues(method, endpoint, status, zoneID).Observe(duration.Seconds())
}

// RecordAPIError records a Cloudflare API error
func (cm *CloudflareMetrics) RecordAPIError(method, endpoint, errorType, zoneID string) {
	cloudflareAPIErrorsTotal.WithLabelValues(method, endpoint, errorType, zoneID).Inc()
}

// UpdateRateLimit updates the API rate limit metrics
func (cm *CloudflareMetrics) UpdateRateLimit(endpoint string, remaining, total float64) {
	cloudflareAPIRateLimitRemaining.WithLabelValues(endpoint).Set(remaining)
	cloudflareAPIRateLimitTotal.WithLabelValues(endpoint).Set(total)
}

// UpdateManagedRecords updates the count of managed DNS records
func (cm *CloudflareMetrics) UpdateManagedRecords(zoneID, zoneName, recordType string, count float64) {
	cloudflareDNSRecordsManagedTotal.WithLabelValues(zoneID, zoneName, recordType).Set(count)
}

// UpdateManagedZones updates the count of managed zones
func (cm *CloudflareMetrics) UpdateManagedZones(zoneName, planType string, count float64) {
	cloudflareZonesManagedTotal.WithLabelValues(zoneName, planType).Set(count)
}

// RecordDNSOperation records a DNS record operation (create, update, delete)
func (cm *CloudflareMetrics) RecordDNSOperation(operation, recordType, zoneID, result string) {
	cloudflareDNSRecordOperationsTotal.WithLabelValues(operation, recordType, zoneID, result).Inc()
}

// ObserveOperationDuration records the duration of a Cloudflare operation
func (cm *CloudflareMetrics) ObserveOperationDuration(operation, zoneID, result string, duration time.Duration) {
	cloudflareOperationDuration.WithLabelValues(operation, zoneID, result).Observe(duration.Seconds())
}

// UpdateZoneStatus updates the status of a Cloudflare zone
func (cm *CloudflareMetrics) UpdateZoneStatus(zoneID, zoneName, status string, active bool) {
	value := 0.0
	if active {
		value = 1.0
	}
	cloudflareZoneStatus.WithLabelValues(zoneID, zoneName, status).Set(value)
}

// GetAPIRequestCount returns the current API request count for a specific endpoint
func (cm *CloudflareMetrics) GetAPIRequestCount(method, endpoint, status, zoneID string) float64 {
	if metric, err := cloudflareAPIRequestsTotal.GetMetricWithLabelValues(method, endpoint, status, zoneID); err == nil {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err == nil && pb.Counter != nil {
			return *pb.Counter.Value
		}
	}
	return 0
}

// GetManagedRecordsCount returns the current count of managed records
func (cm *CloudflareMetrics) GetManagedRecordsCount(zoneID, zoneName, recordType string) float64 {
	if metric, err := cloudflareDNSRecordsManagedTotal.GetMetricWithLabelValues(zoneID, zoneName, recordType); err == nil {
		pb := &dto.Metric{}
		if err := metric.Write(pb); err == nil && pb.Gauge != nil {
			return *pb.Gauge.Value
		}
	}
	return 0
}

// APICallTimer provides a convenient way to time API calls
type APICallTimer struct {
	cm       *CloudflareMetrics
	method   string
	endpoint string
	zoneID   string
	start    time.Time
}

// NewAPICallTimer creates a new API call timer
func (cm *CloudflareMetrics) NewAPICallTimer(method, endpoint, zoneID string) *APICallTimer {
	return &APICallTimer{
		cm:       cm,
		method:   method,
		endpoint: endpoint,
		zoneID:   zoneID,
		start:    time.Now(),
	}
}

// RecordSuccess records a successful API call
func (t *APICallTimer) RecordSuccess() {
	duration := time.Since(t.start)
	t.cm.RecordAPIRequest(t.method, t.endpoint, "success", t.zoneID, duration)
}

// RecordError records a failed API call
func (t *APICallTimer) RecordError(errorType string) {
	duration := time.Since(t.start)
	t.cm.RecordAPIRequest(t.method, t.endpoint, "error", t.zoneID, duration)
	t.cm.RecordAPIError(t.method, t.endpoint, errorType, t.zoneID)
}
