# Performance Tuning and Resource Optimization

This document describes the performance tuning and resource optimization features available in the Cloudflare DNS Operator.

## Horizontal Pod Autoscaler (HPA)

The operator supports Kubernetes HPA for automatic scaling based on CPU, memory, and custom metrics.

### Basic Configuration

```yaml
horizontalPodAutoscaler:
  enabled: true
  minReplicas: 1
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
  targetMemoryUtilizationPercentage: 80
```

### Custom Metrics

```yaml
horizontalPodAutoscaler:
  customMetrics:
    enabled: true
    metrics:
      queueDepth:
        enabled: true
        targetValue: 10
      reconcileRate:
        enabled: true
        targetValue: 5
      errorRate:
        enabled: true
        targetValue: 2
```

### Scaling Behavior (Kubernetes 1.23+)

```yaml
horizontalPodAutoscaler:
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      selectPolicy: Max
      policies:
        - type: Percent
          value: 100
          periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
      selectPolicy: Min
      policies:
        - type: Percent
          value: 10
          periodSeconds: 60
```

## Performance Configuration

### Controller Performance

```yaml
performance:
  controller:
    maxConcurrentReconciles: 5
    reconcileTimeout: 300
    requeueInterval: 300
    requeueIntervalOnError: 60

    rateLimiter:
      baseDelay: 5
      maxDelay: 300
      failureThreshold: 5
      bucketSize: 100
      fillRate: 10

    cache:
      syncTimeout: 120
      resyncPeriod: 600
      metricsEnabled: true

    batch:
      enabled: false
      size: 10
      timeout: 1000
      maxWait: 5000
```

### Resource Optimization

```yaml
performance:
  resources:
    cpu:
      profilingEnabled: false
      requestMultiplier: 1.0
      limitMultiplier: 1.5

    memory:
      profilingEnabled: false
      requestMultiplier: 1.0
      limitMultiplier: 2.0
      gcTuning:
        enabled: false
        gogc: 100
        memlimit: 90
```

### Monitoring

```yaml
performance:
  monitoring:
    metricsEnabled: true
    metricsInterval: 30
    customMetrics:
      enabled: true
      exportInterval: 15

    alerts:
      cpuThreshold: 85
      memoryThreshold: 90
      errorRateThreshold: 5
      queueDepthThreshold: 20
      responseTimeThreshold: 2000
```

## Environment Variables

The following environment variables are automatically configured based on the Helm values:

| Variable | Description | Default |
|----------|-------------|---------|
| `MAX_CONCURRENT_RECONCILES` | Maximum concurrent reconcile operations | 5 |
| `RECONCILE_TIMEOUT` | Timeout for reconcile operations | 300s |
| `REQUEUE_INTERVAL` | Requeue interval for successful reconciles | 300s |
| `REQUEUE_INTERVAL_ON_ERROR` | Requeue interval for failed reconciles | 60s |
| `RATE_LIMITER_BASE_DELAY` | Base delay for rate limiter | 5ms |
| `RATE_LIMITER_MAX_DELAY` | Maximum delay for rate limiter | 300s |
| `PERFORMANCE_METRICS_ENABLED` | Enable performance metrics collection | false |
| `CUSTOM_METRICS_ENABLED` | Enable custom metrics for HPA | false |

## Metrics

The operator exposes the following performance metrics:

- `cloudflare_operator_queue_depth` - Number of pending reconcile requests
- `cloudflare_operator_reconcile_rate_total` - Total number of reconciles processed
- `cloudflare_operator_error_rate_total` - Total number of errors encountered
- `cloudflare_operator_api_response_time_seconds` - Cloudflare API response time
- `cloudflare_operator_reconcile_duration_seconds` - Time taken to complete reconciliation
- `cloudflare_operator_resources_per_second` - Resources processed per second
- `cloudflare_operator_cache_hit_ratio` - Cache hit ratio for operations
- `cloudflare_operator_memory_usage_bytes` - Memory usage of the operator
- `cloudflare_operator_cpu_usage_seconds_total` - CPU usage of the operator
- `cloudflare_operator_active_workers` - Number of active reconcile workers

## Production Configuration Example

```yaml
# Production HPA configuration
horizontalPodAutoscaler:
  enabled: true
  minReplicas: 2
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

  customMetrics:
    enabled: true
    metrics:
      queueDepth:
        enabled: true
        targetValue: 15
      errorRate:
        enabled: true
        targetValue: 3

# Production performance tuning
performance:
  controller:
    maxConcurrentReconciles: 10
    reconcileTimeout: 600
    requeueInterval: 600
    requeueIntervalOnError: 120

    rateLimiter:
      baseDelay: 10
      maxDelay: 600
      failureThreshold: 10
      bucketSize: 200
      fillRate: 20

    cache:
      syncTimeout: 240
      resyncPeriod: 1200
      metricsEnabled: true

  monitoring:
    metricsEnabled: true
    metricsInterval: 15
    customMetrics:
      enabled: true
      exportInterval: 10

# Production resources
resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 128Mi
```

## Troubleshooting

### High Queue Depth

If you see consistently high queue depth metrics:

1. Increase `maxConcurrentReconciles`
2. Optimize `reconcileTimeout`
3. Review rate limiter settings
4. Consider enabling batch processing

### High Error Rates

If error rates are high:

1. Increase `requeueIntervalOnError` to reduce retry frequency
2. Review Cloudflare API rate limits
3. Check network connectivity and DNS resolution
4. Monitor API response times

### Resource Usage

Monitor CPU and memory usage:

1. Adjust resource requests/limits based on actual usage
2. Enable profiling in development environments
3. Use Go garbage collection tuning for memory optimization
4. Consider enabling resource multipliers for dynamic scaling
