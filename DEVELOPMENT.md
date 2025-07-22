# Development Guide

This guide provides comprehensive instructions for developing, testing, and contributing to the Cloudflare DNS Operator.

## Table of Contents

- [Quick Start](#quick-start)
- [Development Environment Setup](#development-environment-setup)
- [Local Testing Workflow](#local-testing-workflow)
- [Code Quality Tools](#code-quality-tools)
- [Advanced Testing](#advanced-testing)
- [Debugging](#debugging)
- [Release Process](#release-process)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Essential Commands for Daily Development

```bash
# Before starting work
make test lint fmt vet      # Run all quality checks

# After making changes
make generate manifests     # Regenerate code if API changed
make test                   # Run tests
make lint-fix              # Fix any linting issues

# Before committing
pre-commit run --all-files  # Run pre-commit hooks
```

## Development Environment Setup

### 1. Prerequisites Installation

**Required tools:**
```bash
# Go (install via https://golang.org/dl/)
go version  # Should be 1.24.0+

# Docker (install via https://docs.docker.com/get-docker/)
docker version

# kubectl (install via https://kubernetes.io/docs/tasks/tools/)
kubectl version --client

# Optional but recommended: Kind for local K8s
# Install via: https://kind.sigs.k8s.io/docs/user/quick-start/
kind version
```

**Project dependencies (auto-installed via Makefile):**
```bash
make setup-envtest          # Install Kubernetes test environment
make controller-gen         # Install Kubebuilder code generator
make kustomize             # Install Kustomize for YAML management
make golangci-lint         # Install Go linter
```

### 2. Pre-commit Setup

```bash
# Install pre-commit system-wide
pip install pre-commit

# Install hooks for this repository
pre-commit install

# Test hooks work
pre-commit run --all-files
```

### 3. IDE Configuration

**VS Code recommended settings (.vscode/settings.json):**
```json
{
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package",
  "editor.formatOnSave": true,
  "go.formatTool": "goimports"
}
```

## Local Testing Workflow

### Unit Testing

**Run all tests:**
```bash
make test                          # Full test suite with coverage
```

**Run specific package tests:**
```bash
go test -v ./api/v1               # Test API types
go test -v ./internal/controller  # Test controller logic
```

**Test with coverage details:**
```bash
make test
go tool cover -func=cover.out    # Show function-level coverage
go tool cover -html=cover.out     # Generate HTML report
```

**Watch mode for active development:**
```bash
# Install entr for file watching
# macOS: brew install entr
# Linux: apt-get install entr

find . -name "*.go" | entr -r make test
```

### Integration Testing

**Setup local Kubernetes cluster:**
```bash
# Create Kind cluster
make setup-test-e2e

# Verify cluster is ready
kubectl cluster-info --context kind-k8s-operator-cloudflare-test-e2e
```

**Run E2E tests:**
```bash
make test-e2e                     # Full E2E test suite

# Or with specific timeout
make test-e2e TIMEOUT=45m
```

**Cleanup:**
```bash
make cleanup-test-e2e             # Remove Kind cluster
```

### Local Controller Development

**Run controller locally against remote cluster:**
```bash
# Install CRDs to cluster
make install

# Run controller locally (uses your kubeconfig)
make run

# In another terminal, apply test resources
kubectl apply -f config/samples/
```

**Deploy controller to cluster:**
```bash
# Build and deploy
make docker-build docker-push IMG=your-registry/k8s-operator-cloudflare:dev
make deploy IMG=your-registry/k8s-operator-cloudflare:dev

# Check deployment
kubectl get pods -n k8s-operator-cloudflare-system
kubectl logs -f deployment/k8s-operator-cloudflare-controller-manager -n k8s-operator-cloudflare-system
```

## Code Quality Tools

### Linting

**Run linter:**
```bash
make lint                         # Run all enabled linters
make lint-fix                     # Auto-fix issues where possible
make lint-config                  # Validate linter configuration
```

**Enabled linters (.golangci.yml):**
- `errcheck` - Check error handling
- `govet` - Go static analysis
- `staticcheck` - Advanced static analysis
- `ineffassign` - Detect ineffective assignments
- `misspell` - Fix spelling mistakes
- `unused` - Find unused code
- `gocyclo` - Detect complex functions
- `dupl` - Find duplicate code

**Custom linter runs:**
```bash
# Run specific linter
golangci-lint run --enable-only=errcheck

# Run with different config
golangci-lint run --config=.golangci-strict.yml
```

### Code Formatting

```bash
make fmt                          # Format all Go code
make vet                          # Run go vet static analysis

# Check formatting without applying
gofmt -l .
```

### Code Generation

**When to regenerate:**
- After modifying API types in `api/v1/`
- After changing controller RBAC requirements
- After updating kubebuilder annotations

```bash
make generate                     # Generate DeepCopy methods
make manifests                    # Generate CRDs, RBAC, webhooks

# Check what changed
git diff
```

## Advanced Testing

### Testing Different DNS Record Types

```bash
# Apply different sample types
kubectl apply -f config/samples/dns_v1_cloudflarerecord.yaml         # A record
kubectl apply -f config/samples/dns_v1_cloudflarerecord_cname.yaml    # CNAME record
kubectl apply -f config/samples/dns_v1_cloudflarerecord_mx.yaml       # MX record

# Watch reconciliation
kubectl get cloudflarerecords -w
kubectl describe cloudflarerecord sample-cloudflarerecord
```

### Performance Testing

```bash
# Benchmark tests
go test -bench=. -benchmem ./internal/controller

# Load testing with multiple resources
for i in {1..10}; do
  sed "s/sample-cloudflarerecord/sample-record-$i/g" config/samples/dns_v1_cloudflarerecord.yaml | kubectl apply -f -
done

# Monitor resource usage
kubectl top pods -n k8s-operator-cloudflare-system
```

### Memory and CPU Profiling

```bash
# Run controller with profiling
go run -race ./cmd/main.go -enable-pprof=true

# In another terminal, collect profiles
go tool pprof http://localhost:8080/debug/pprof/heap
go tool pprof http://localhost:8080/debug/pprof/profile
```

## Debugging

### Debugging Controller Logic

**Add debug logging:**
```go
import "sigs.k8s.io/controller-runtime/pkg/log"

func (r *CloudflareRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)
    log.Info("Reconciling CloudflareRecord", "request", req)
    // ... your code
}
```

**Local debugging with Delve:**
```bash
# Install Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug tests
dlv test ./internal/controller -- -test.run TestControllers

# Debug controller
dlv exec ./bin/manager
```

### Debugging Tests

```bash
# Run tests with verbose output
go test -v ./internal/controller -run TestSpecificTest

# Debug failed test
dlv test ./internal/controller -- -test.run TestFailingTest
```

### Cluster Debugging

```bash
# Check CRD installation
kubectl get crd cloudflarerecords.dns.cloudflare.io

# Check controller logs
kubectl logs -f deployment/k8s-operator-cloudflare-controller-manager -n k8s-operator-cloudflare-system

# Check RBAC
kubectl auth can-i get cloudflarerecords --as=system:serviceaccount:k8s-operator-cloudflare-system:k8s-operator-cloudflare-controller-manager

# Debug resource status
kubectl describe cloudflarerecord sample-cloudflarerecord
kubectl get events --field-selector involvedObject.name=sample-cloudflarerecord
```

## Release Process

### Pre-release Checklist

1. **Code quality:**
   ```bash
   make test lint fmt vet
   make test-e2e
   ```

2. **Documentation:**
   - Update README.md
   - Update API documentation
   - Update CHANGELOG.md

3. **Version bumping:**
   ```bash
   # Update version in relevant files
   # Tag release
   git tag -a v1.0.0 -m "Release v1.0.0"
   ```

### Building Release Assets

```bash
# Build multi-arch images
make docker-buildx IMG=devops247-online/k8s-operator-cloudflare:v1.0.0

# Generate install manifests
make build-installer IMG=devops247-online/k8s-operator-cloudflare:v1.0.0

# Verify install.yaml
kubectl apply --dry-run=client -f dist/install.yaml
```

## Troubleshooting

### Common Issues

**Tests failing:**
```bash
# Clean test cache
go clean -testcache

# Reset test environment
make cleanup-test-e2e
make setup-test-e2e
```

**Linter issues:**
```bash
# Update linter
make golangci-lint
./bin/golangci-lint --version

# Run with debug
./bin/golangci-lint run --verbose
```

**Generation issues:**
```bash
# Clean generated files
rm -rf api/v1/zz_generated.deepcopy.go
make generate

# Verify controller-gen version
./bin/controller-gen --version
```

**Docker build issues:**
```bash
# Clean Docker cache
docker system prune -a

# Build with debug
make docker-build DOCKER_BUILDKIT=0
```

### Getting Help

1. **Check CI/CD pipeline** - often shows what's failing
2. **Run `make help`** - see all available commands
3. **Check logs** - `kubectl logs` for runtime issues
4. **Use debugging tools** - delve, pprof for deep debugging
5. **Consult [Kubebuilder docs](https://book.kubebuilder.io/)** for operator patterns

## Useful Development Scripts

### Quick Test Script
```bash
#!/bin/bash
# save as scripts/quick-test.sh
set -e
echo "Running quick development checks..."
make fmt
make vet
make lint
make test
echo "✅ All checks passed!"
```

### Coverage Report Script
```bash
#!/bin/bash
# save as scripts/coverage-report.sh
set -e
make test
go tool cover -html=cover.out -o coverage.html
echo "Coverage report generated: coverage.html"
if command -v open &> /dev/null; then
  open coverage.html
fi
```

Make scripts executable:
```bash
chmod +x scripts/*.sh
```
