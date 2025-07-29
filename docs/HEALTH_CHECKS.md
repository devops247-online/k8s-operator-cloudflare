# Health Checks and Probes

This document describes the comprehensive health check system implemented for the Cloudflare DNS Operator.

## Overview

The operator implements advanced health checking with custom logic for production reliability, including:

- **Liveness probes** to detect unhealthy pods
- **Readiness probes** to control traffic routing  
- **Startup probes** for slow-starting containers
- **Custom health checks** for operator-specific logic
- **Profiling endpoints** for debugging (development only)
- **Graceful shutdown** handling

## Health Check Endpoints

### `/healthz` - Liveness Check
Basic liveness check that verifies:
- Manager is running
- Kubernetes API connectivity

### `/readyz` - Readiness Check  
Comprehensive readiness check that validates:
- Kubernetes API accessibility
- CRD availability (CloudflareRecord)
- Cloudflare API connectivity (if configured)
- Leader election status

### `/metrics` - Prometheus Metrics
Standard Prometheus metrics endpoint for monitoring.

### `/debug/pprof/*` - Profiling (Development Only)
Performance profiling endpoints enabled only when:
- `ENABLE_PPROF=true` environment variable is set, OR
- `GO_ENV=development` environment variable is set

## Configuration

### Helm Values

```yaml
# Health check configuration
healthChecks:
  enabled: true

  # Liveness probe configuration
  livenessProbe:
    httpGet:
      path: /healthz
      port: 8081
    initialDelaySeconds: 15
    periodSeconds: 20
    timeoutSeconds: 5
    failureThreshold: 3
    successThreshold: 1

  # Readiness probe configuration  
  readinessProbe:
    httpGet:
      path: /readyz
      port: 8081
    initialDelaySeconds: 5
    periodSeconds: 10
    timeoutSeconds: 5
    failureThreshold: 3
    successThreshold: 1

  # Startup probe configuration (for slow-starting containers)
  startupProbe:
    httpGet:
      path: /healthz  
      port: 8081
    initialDelaySeconds: 10
    periodSeconds: 10
    timeoutSeconds: 5
    failureThreshold: 30
    successThreshold: 1

# Profiling configuration
profiling:
  enabled: false  # Only enable in development
  port: 6060
```

## Custom Health Logic

The health checker implements custom validation for:

1. **Kubernetes API Connectivity**
   - Verifies connection to Kubernetes API server
   - Checks cluster version accessibility

2. **CRD Availability**
   - Validates CloudflareRecord CRD is installed and accessible
   - Ensures operator can manage custom resources

3. **Cloudflare API Connectivity**
   - Tests API token validity (if configured)
   - Verifies token is active and has required permissions

4. **Leader Election Status**
   - Checks leader election configuration (if enabled)
   - Validates lease resource accessibility

## Graceful Shutdown

The operator implements graceful shutdown with:

- **SIGTERM handling** for clean shutdowns
- **Configurable timeout** (default 30 seconds)
- **Callback system** for custom cleanup logic
- **In-flight request** completion
- **Resource cleanup** on shutdown

### Environment Variables

- `ENABLE_PPROF`: Enable pprof profiling endpoints (default: false)
- `GO_ENV`: Set to "development" to enable pprof automatically

## Testing

Health checks are tested with:

1. **Unit Tests**: `./internal/health/checker_test.go`
2. **E2E Tests**: `./test/e2e/health_checks_test.go`
3. **CI Configuration**: `./charts/cloudflare-dns-operator/ci/health-checks-test-values.yaml`

### Running Tests

```bash
# Unit tests
go test ./internal/health/... -v

# E2E tests  
make test-e2e

# Helm chart validation
helm lint charts/cloudflare-dns-operator/
helm template cloudflare-dns-operator charts/cloudflare-dns-operator/ -f charts/cloudflare-dns-operator/ci/health-checks-test-values.yaml
```

## Production Deployment

For production deployments:

1. **Enable health checks**: Set `healthChecks.enabled: true`
2. **Disable profiling**: Ensure `profiling.enabled: false`
3. **Configure timeouts**: Adjust probe timings based on your environment
4. **Monitor endpoints**: Set up monitoring for `/healthz`, `/readyz`, and `/metrics`

## Security Considerations

- Profiling endpoints are **disabled by default** for security
- Health check endpoints do not expose sensitive information
- All endpoints respect Pod Security Standards
- Network policies restrict access appropriately
