# Monitoring and Alerting

This document describes the comprehensive monitoring and alerting solution for the Cloudflare DNS Operator.

## Overview

The operator provides extensive observability through:
- **Prometheus Metrics**: Business, API, and performance metrics
- **Grafana Dashboards**: 5 comprehensive dashboards for different use cases
- **Prometheus Alerting Rules**: Critical, warning, and informational alerts
- **ServiceMonitor**: Integration with Prometheus Operator

## Grafana Dashboards

### 1. Operator Overview Dashboard
**File**: `dashboards/operator-overview.json`  
**Purpose**: High-level view of operator health and performance

**Key Panels**:
- Operator Status (UP/DOWN)
- Active DNS Records count
- Reconcile Success Rate gauge
- API Rate Limit remaining
- Resource usage overview
- Error distribution

### 2. DNS Management Dashboard  
**File**: `dashboards/dns-management.json`  
**Purpose**: Detailed DNS operations monitoring

**Key Panels**:
- DNS Records by Type (A, AAAA, CNAME, etc.)
- Records by Status (active, pending, error)
- DNS Operations Rate (create, update, delete)
- DNS Propagation Time percentiles
- Zone Health Status
- Operations Success Rate

### 3. Performance & Resource Dashboard
**File**: `dashboards/performance-resource.json`  
**Purpose**: System performance and resource utilization

**Key Panels**:
- Memory Usage (heap, stack, system)
- CPU Usage trends
- Goroutines count
- Work Queue Depth
- Reconcile Duration percentiles
- API Response Time percentiles
- Garbage Collection statistics
- Resource Utilization vs Limits

### 4. Troubleshooting Dashboard
**File**: `dashboards/troubleshooting.json`  
**Purpose**: Error analysis and debugging

**Key Panels**:
- Error Rate Trends
- Top Error Types
- Failed Reconciles Details (table)
- API Errors by Status Code
- Rate Limit Status

### 5. SLA/SLO Tracking Dashboard
**File**: `dashboards/sla-slo-tracking.json`  
**Purpose**: Service reliability and SLA compliance

**Key Panels**:
- Service Availability (24h)
- Reconcile Success Rate (24h)
- Average API Response Time
- Error Budget Remaining (99% SLO)
- Availability Trends (1h, 24h, 7d, 30d)
- Success Rate Trends
- SLO Compliance Status

## Prometheus Alerting Rules

### Critical Alerts

#### CloudflareDNSOperatorDown
- **Condition**: Operator pod is not running
- **Duration**: 1 minute
- **Action**: Immediate investigation required

#### CloudflareDNSOperatorHighErrorRate
- **Condition**: Error rate > 5% for 2 minutes
- **Action**: Check logs and troubleshooting dashboard

### Warning Alerts

#### CloudflareAPIRateLimitHigh
- **Condition**: Rate limit usage > 90% for 5 minutes
- **Action**: Review API usage patterns

#### CloudflareDNSOperatorHighLatency
- **Condition**: P95 reconcile latency > 30 seconds for 5 minutes  
- **Action**: Check performance dashboard

#### CloudflareDNSOperatorResourceExhaustion
- **Condition**: Memory > 90% or CPU > 80% for 5 minutes
- **Action**: Consider scaling or resource adjustments

#### CloudflareDNSOperatorWorkQueueHigh
- **Condition**: Work queue depth > 20 for 10 minutes
- **Action**: Check controller performance

#### CloudflareDNSRecordStuckInPending
- **Condition**: DNS records in pending state > 30 minutes
- **Action**: Check DNS propagation issues

### Informational Alerts

#### CloudflareDNSOperatorLeaderElectionChange
- **Condition**: Leader election change detected
- **Action**: Monitor for stability

#### CloudflareDNSOperatorConfigReload
- **Condition**: Operator restart detected
- **Action**: Verify configuration changes

## Configuration

### Basic Setup

```yaml
monitoring:
  # Enable Grafana dashboards
  grafana:
    dashboards:
      enabled: true
      namespace: monitoring

  # Enable Prometheus alerting rules
  prometheusRule:
    enabled: true

  # Enable ServiceMonitor for Prometheus Operator
  serviceMonitor:
    enabled: true
```

### Advanced Configuration

```yaml
monitoring:
  grafana:
    dashboards:
      enabled: true
      namespace: monitoring
      labels:
        grafana_dashboard: "1"
        team: "platform"
      annotations:
        grafana_folder: "Cloudflare DNS Operator"

      # Individual dashboard controls
      operatorOverview:
        enabled: true
      dnsManagement:
        enabled: true
      performanceResource:
        enabled: true
      troubleshooting:
        enabled: true
      slaTracking:
        enabled: true

  prometheusRule:
    enabled: true
    namespace: monitoring
    labels:
      prometheus: "main"

    # Customize alert thresholds
    groups:
      critical:
        rules:
          highErrorRate:
            threshold: 0.05  # 5% error rate
            duration: "2m"
      warning:
        rules:
          resourceExhaustion:
            memoryThreshold: 0.9  # 90% memory
            cpuThreshold: 0.8     # 80% CPU

    # Add custom rules
    customRules:
      - alert: CustomDNSAlert
        expr: custom_dns_metric > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: Custom DNS alert fired

  # AlertManager integration
  alertmanager:
    enabled: true
    notifications:
      slack:
        enabled: true
        webhook: "https://hooks.slack.com/..."
        channel: "#alerts"
```

## How to Use Dashboards

### Dashboard Access

Once deployed, dashboards are automatically discovered by Grafana through the sidecar mechanism:

1. **Automatic Import**: Dashboards appear in the "Cloudflare DNS Operator" folder
2. **Manual Import**: JSON files can be imported directly via Grafana UI
3. **API Access**: Use Grafana API for programmatic dashboard management

### Dashboard Navigation

#### Operator Overview Dashboard
- **Use Case**: First point of contact for monitoring operator health
- **Key Metrics**: Start here to identify any high-level issues
- **Navigation**: Links to detailed dashboards for deeper investigation

#### DNS Management Dashboard  
- **Use Case**: Monitor DNS record operations and health
- **Filtering**: Use zone variables to focus on specific domains
- **Alerts**: Shows which zones have issues requiring attention

#### Performance & Resource Dashboard
- **Use Case**: Capacity planning and performance optimization
- **Monitoring**: Track resource usage trends over time
- **Scaling**: Identify when to scale operator resources

#### Troubleshooting Dashboard
- **Use Case**: Active incident investigation and debugging
- **Error Analysis**: Start with error rate trends, drill down to specific error types  
- **Log Correlation**: Use timestamps to correlate with log entries

#### SLA/SLO Tracking Dashboard
- **Use Case**: Service reliability reporting and compliance tracking
- **Reporting**: Generate availability reports for stakeholders
- **SLO Management**: Track error budget consumption

### Dashboard Variables

All dashboards support the following variables:
- **Datasource**: Prometheus datasource selection
- **Namespace**: Filter by Kubernetes namespace
- **Zone**: Filter by Cloudflare zone (DNS dashboards)
- **Pod**: Filter by operator pod (Performance dashboard)

### Common Workflows

#### Daily Health Check
1. Start with **Operator Overview** dashboard
2. Check service availability and error rates
3. Review **SLA/SLO Tracking** for trends
4. Investigate any anomalies in **Troubleshooting** dashboard

#### Incident Response
1. **Troubleshooting** dashboard for error analysis
2. **Performance & Resource** dashboard for resource constraints
3. **DNS Management** dashboard for DNS-specific issues
4. Use time range selector to focus on incident timeframe

#### Capacity Planning
1. **Performance & Resource** dashboard for usage trends
2. **Operator Overview** for workload patterns
3. **SLA/SLO Tracking** for performance degradation trends
4. Plan scaling based on resource utilization patterns

#### Performance Optimization
1. **Performance & Resource** dashboard for bottlenecks
2. **DNS Management** dashboard for operation latency
3. **Troubleshooting** dashboard for error patterns
4. Correlate metrics with application changes

## Metric Labels

Key metric labels for filtering and grouping:
- `namespace`: Kubernetes namespace
- `zone_name`: Cloudflare zone name
- `zone_id`: Cloudflare zone ID
- `record_type`: DNS record type (A, AAAA, CNAME, etc.)
- `status`: Record status (active, pending, error)
- `result`: Operation result (success, error)
- `controller`: Controller name
- `error_type`: Error classification

## Troubleshooting Guide

### High Error Rate
1. Check Troubleshooting dashboard
2. Review operator logs
3. Verify Cloudflare API credentials
4. Check network connectivity

### High Latency
1. Check Performance dashboard
2. Review resource utilization
3. Check work queue depth
4. Verify Cloudflare API response times

### Missing Metrics
1. Verify ServiceMonitor is enabled
2. Check Prometheus scraping configuration
3. Ensure metrics port (8081) is accessible
4. Review operator pod status

### Dashboard Not Loading
1. Verify Grafana sidecar is enabled
2. Check ConfigMap labels match sidecar configuration
3. Ensure correct namespace deployment
4. Verify Prometheus datasource configuration

## Best Practices

1. **Set up alerting early**: Enable critical alerts from day one
2. **Monitor SLAs**: Use SLA/SLO dashboard for reliability tracking
3. **Regular reviews**: Weekly review of troubleshooting dashboard
4. **Capacity planning**: Monitor resource trends for scaling decisions
5. **Alert tuning**: Adjust thresholds based on your environment
6. **Dashboard customization**: Clone and customize dashboards for specific needs

## Integration Examples

### With Prometheus Operator
```yaml
# ServiceMonitor automatically discovered
monitoring:
  serviceMonitor:
    enabled: true
    labels:
      prometheus: kube-prometheus-stack
```

### With Grafana Operator
```yaml
# Dashboards automatically imported
monitoring:
  grafana:
    dashboards:
      enabled: true
      labels:
        grafana_dashboard: "1"
```

### With AlertManager
```yaml
# Alerts routed to multiple channels
monitoring:
  alertmanager:
    enabled: true
    notifications:
      slack:
        enabled: true
        webhook: "${SLACK_WEBHOOK}"
      pagerduty:
        enabled: true
        serviceKey: "${PAGERDUTY_KEY}"
```
