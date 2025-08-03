# Structured Logging and Distributed Tracing

This document describes the structured logging and distributed tracing capabilities of the Cloudflare DNS Operator.

## Table of Contents

- [Overview](#overview)
- [Structured Logging](#structured-logging)
  - [Configuration](#logging-configuration)  
  - [Examples](#logging-examples)
  - [Environment Overrides](#logging-environment-overrides)
- [Distributed Tracing](#distributed-tracing)
  - [Configuration](#tracing-configuration)
  - [Examples](#tracing-examples)
  - [Environment Overrides](#tracing-environment-overrides)
- [Integration](#integration)
  - [Monitoring Stack Integration](#monitoring-stack-integration)
  - [Log Correlation](#log-correlation)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Overview

The Cloudflare DNS Operator implements comprehensive observability through:

- **Structured Logging**: JSON-formatted logs with contextual information using Uber's Zap logger
- **Distributed Tracing**: OpenTelemetry-based tracing for request correlation and performance monitoring

These features provide deep insights into operator behavior, making debugging, monitoring, and performance analysis more effective.

## Structured Logging

### Architecture

The operator uses a structured logging approach with the following components:

- **Logger**: Uber's Zap logger configured for high performance and structured output
- **Middleware**: Request context enrichment for correlation
- **Configuration**: Environment-based configuration with hot-reload support

### Features

- JSON-formatted output for machine parsing
- Contextual information (request IDs, trace IDs, resource names)
- Log sampling to reduce volume in high-throughput scenarios
- Development mode with human-readable console output
- Multiple output destinations (stdout, stderr, files)

### Logging Configuration

Configure logging through the `logging` section in your values.yaml:

```yaml
logging:
  # Enable structured logging (default: true)
  enabled: true

  # Log level: debug, info, warn, error (default: info)
  level: info

  # Log format: json, console (default: json)
  format: json

  # Development mode (default: false)
  development: false

  # Output destinations (default: [stdout])
  outputs:
    - stdout

  # Log sampling configuration
  sampling:
    enabled: false
    initial: 100      # Always emit first N logs
    thereafter: 100   # Then emit every Nth log
```

### Environment Variables

The following environment variables control logging behavior:

- `LOG_LEVEL`: Log level (debug, info, warn, error)
- `LOG_FORMAT`: Output format (json, console)
- `LOG_DEVELOPMENT`: Enable development mode (true, false)
- `LOG_SAMPLING_ENABLED`: Enable log sampling (true, false)
- `LOG_SAMPLING_INITIAL`: Initial logs to always emit
- `LOG_SAMPLING_THEREAFTER`: Emit every Nth log after initial

### Logging Examples

#### Basic Configuration

```yaml
# Production configuration
logging:
  enabled: true
  level: info
  format: json
  development: false
  sampling:
    enabled: true
    initial: 50
    thereafter: 200
```

#### Development Configuration

```yaml
# Development configuration
logging:
  enabled: true
  level: debug
  format: console
  development: true
  sampling:
    enabled: false
```

#### Sample Log Output

```json
{
  "level": "info",
  "ts": "2025-01-01T12:00:00.000Z",
  "logger": "controller.cloudflarerecord",
  "msg": "Reconciling CloudflareRecord",
  "request.id": "req-20250101120000.123456",
  "operation": "reconcile",
  "resource": "example-dns-record",
  "namespace": "default",
  "trace.id": "4bf92f3a567894a0", # pragma: allowlist secret
  "span.id": "12345678", # pragma: allowlist secret
  "dns.record.type": "A",
  "dns.record.name": "example.com",
  "cloudflare.zone.id": "1234567890abcdef" # pragma: allowlist secret
}
```

### Logging Environment Overrides

Configure different logging levels per environment:

```yaml
logging:
  # Global defaults
  enabled: true
  level: info
  format: json

  # Environment-specific overrides
  environments:
    development:
      level: debug
      format: console
      development: true
      sampling:
        enabled: false

    staging:
      level: info
      format: json
      sampling:
        enabled: true
        initial: 100
        thereafter: 100

    production:
      level: info
      format: json
      sampling:
        enabled: true
        initial: 50
        thereafter: 200
```

## Distributed Tracing

### Architecture

The operator implements distributed tracing using:

- **OpenTelemetry**: Industry-standard tracing framework
- **OTLP Protocol**: For sending traces to compatible backends
- **Context Propagation**: Automatic trace correlation across components
- **Sampling**: Configurable trace sampling strategies

### Features

- Request-level tracing across all operator components
- Automatic parent-child span relationships
- Custom attributes for CloudFlare-specific context
- Multiple exporter options (OTLP, Console, None)
- Flexible sampling strategies

### Tracing Configuration

Configure tracing through the `tracing` section in your values.yaml:

```yaml
tracing:
  # Enable distributed tracing (default: false)
  enabled: false

  # Service name for tracing (default: cloudflare-dns-operator)
  serviceName: cloudflare-dns-operator

  # Service version (uses chart appVersion if empty)
  serviceVersion: "1.0.0"

  # Deployment environment
  environment: production

  # Trace exporter configuration
  exporter:
    # Exporter type: otlp, console, none (default: otlp)
    type: otlp

    # OTLP endpoint for traces
    endpoint: http://jaeger-collector:14268/api/traces

    # Request timeout (default: 30s)
    timeout: 30s

    # Use insecure connection (default: false)
    insecure: false

    # Custom headers for exporter
    headers:
      authorization: "Bearer <token>"

    # Compression: gzip, none (default: gzip)
    compression: gzip

  # Trace sampling configuration
  sampling:
    # Sampling type: always, never, trace_id_ratio, parent_based
    type: parent_based

    # Sampling ratio (0.0 to 1.0) for ratio-based sampling
    ratio: 0.1

  # Custom resource attributes
  resourceAttributes:
    deployment.environment: production
    service.namespace: cloudflare-dns-operator
```

### Environment Variables

The following environment variables control tracing behavior:

- `TRACING_ENABLED`: Enable tracing (true, false)
- `TRACING_SERVICE_NAME`: Service name for traces
- `TRACING_SERVICE_VERSION`: Service version
- `TRACING_ENVIRONMENT`: Deployment environment
- `TRACING_EXPORTER_TYPE`: Exporter type (otlp, console, none)
- `TRACING_EXPORTER_ENDPOINT`: OTLP endpoint URL
- `TRACING_EXPORTER_TIMEOUT`: Request timeout
- `TRACING_EXPORTER_INSECURE`: Use insecure connection
- `TRACING_EXPORTER_COMPRESSION`: Compression type
- `TRACING_SAMPLING_TYPE`: Sampling strategy
- `TRACING_SAMPLING_RATIO`: Sampling ratio

### Tracing Examples

#### Jaeger Integration

```yaml
tracing:
  enabled: true
  serviceName: cloudflare-dns-operator
  environment: production

  exporter:
    type: otlp
    endpoint: http://jaeger-collector.monitoring:14268/api/traces
    timeout: 30s
    insecure: false
    compression: gzip

  sampling:
    type: parent_based
    ratio: 0.1

  resourceAttributes:
    deployment.environment: production
    cluster.name: production-k8s
    service.namespace: cloudflare-dns-operator
```

#### Development Configuration

```yaml
tracing:
  enabled: true
  serviceName: cloudflare-dns-operator
  environment: development

  exporter:
    type: console  # Output traces to console

  sampling:
    type: always   # Sample all traces
    ratio: 1.0
```

#### Zipkin Integration

```yaml
tracing:
  enabled: true
  serviceName: cloudflare-dns-operator

  exporter:
    type: otlp
    endpoint: http://zipkin.monitoring:9411/api/v2/spans
    headers:
      content-type: "application/json"
```

### Tracing Environment Overrides

Configure different tracing behavior per environment:

```yaml
tracing:
  # Global defaults
  enabled: false
  serviceName: cloudflare-dns-operator

  # Environment-specific overrides
  environments:
    development:
      enabled: true
      exporter:
        type: console
      sampling:
        type: always
        ratio: 1.0

    staging:
      enabled: true
      exporter:
        type: otlp
        endpoint: http://jaeger-collector.monitoring:14268/api/traces
      sampling:
        type: trace_id_ratio
        ratio: 0.5

    production:
      enabled: false  # Disabled by default in production
      exporter:
        type: otlp
        endpoint: http://jaeger-collector.monitoring:14268/api/traces
      sampling:
        type: parent_based
        ratio: 0.1
```

## Integration

### Monitoring Stack Integration

#### Prometheus + Grafana + Jaeger Stack

```yaml
# Enable tracing with Jaeger
tracing:
  enabled: true
  serviceName: cloudflare-dns-operator

  exporter:
    type: otlp
    endpoint: http://jaeger-collector:14268/api/traces

  sampling:
    type: parent_based
    ratio: 0.1

# Configure structured logging
logging:
  enabled: true
  level: info
  format: json

  sampling:
    enabled: true
    initial: 50
    thereafter: 200
```

#### ELK Stack Integration

```yaml
# Configure JSON logging for Elasticsearch
logging:
  enabled: true
  level: info
  format: json
  development: false

  # Disable sampling for complete log capture
  sampling:
    enabled: false
```

### Log Correlation

Logs and traces are automatically correlated through:

1. **Request IDs**: Unique identifiers for each reconciliation request
2. **Trace IDs**: OpenTelemetry trace identifiers in log entries  
3. **Span IDs**: Individual span identifiers for precise correlation

Example correlated log entry:

```json
{
  "level": "info",
  "ts": "2025-01-01T12:00:00.000Z",
  "msg": "DNS record created successfully",
  "request.id": "req-20250101120000.123456",
  "trace.id": "4bf92f3a567894a0", # pragma: allowlist secret
  "span.id": "12345678", # pragma: allowlist secret
  "dns.record.name": "example.com",
  "dns.record.type": "A",
  "duration_ms": 1250
}
```

### Querying Correlated Data

#### Jaeger Query by Trace ID

```bash
# Find trace by ID from logs
curl "http://jaeger:16686/api/traces/4bf92f3a567894a0"
```

#### Elasticsearch Query by Request ID

```json
{
  "query": {
    "match": {
      "request.id": "req-20250101120000.123456"
    }
  }
}
```

## Troubleshooting

### Common Issues

#### Logs Not Appearing

1. **Check log level**: Ensure log level is appropriate (debug, info, warn, error)
2. **Verify output configuration**: Confirm outputs are correctly configured
3. **Check sampling**: Disable sampling if logs are being filtered out

```yaml
logging:
  level: debug
  sampling:
    enabled: false
```

#### Traces Not Appearing

1. **Verify tracing is enabled**: Check `tracing.enabled: true`
2. **Check exporter endpoint**: Ensure endpoint is reachable
3. **Verify sampling**: Increase sampling ratio for testing

```yaml
tracing:
  enabled: true
  sampling:
    type: always
    ratio: 1.0
```

#### High Log Volume

Enable sampling to reduce log volume:

```yaml
logging:
  sampling:
    enabled: true
    initial: 10
    thereafter: 100
```

#### Performance Impact

1. **Adjust sampling ratios**: Lower ratios reduce overhead
2. **Use asynchronous logging**: Enabled by default in structured logging
3. **Monitor resource usage**: Use provided Grafana dashboards

### Debug Commands

#### Check Environment Variables

```bash
kubectl exec -it deployment/cloudflare-dns-operator -- env | grep -E "(LOG_|TRACING_)"
```

#### View Live Logs

```bash
kubectl logs -f deployment/cloudflare-dns-operator | jq .
```

#### Test Trace Connectivity

```bash
kubectl exec -it deployment/cloudflare-dns-operator -- \
  curl -v http://jaeger-collector:14268/api/traces
```

## Best Practices

### Logging Best Practices

1. **Use structured logging in production**: Always enable JSON format
2. **Configure appropriate log levels**: Use `info` for production, `debug` for development
3. **Enable sampling in high-traffic environments**: Prevents log volume explosion
4. **Include contextual information**: Leverage middleware for automatic context enrichment
5. **Monitor log volume**: Use Grafana dashboards to track log rates

### Tracing Best Practices

1. **Enable tracing selectively**: Use in development and staging, consider carefully for production
2. **Configure appropriate sampling**: Balance observability needs with performance impact
3. **Use parent-based sampling**: Ensures complete traces while controlling volume
4. **Set resource attributes**: Include deployment and cluster information
5. **Monitor trace volume**: Track trace rates and adjust sampling accordingly

### Security Considerations

1. **Avoid logging sensitive data**: Never log API tokens, secrets, or PII
2. **Use secure trace endpoints**: Always use TLS for trace export in production
3. **Implement access controls**: Restrict access to logging and tracing infrastructure
4. **Regular security audits**: Review log and trace content for sensitive information

### Performance Considerations

1. **Asynchronous logging**: Enabled by default to minimize performance impact
2. **Sampling strategies**: Use to balance observability with performance
3. **Resource limits**: Monitor CPU and memory usage when enabling tracing
4. **Batch configuration**: Optimize trace export batch sizes for your environment

### Monitoring Recommendations

1. **Set up alerts**: Monitor for logging errors and trace export failures
2. **Dashboard creation**: Use provided Grafana dashboards as starting points
3. **Regular reviews**: Periodically review log and trace configurations
4. **Capacity planning**: Monitor storage requirements for logs and traces
