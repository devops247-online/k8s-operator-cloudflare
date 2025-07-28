# Cloudflare DNS Operator

[![CI Pipeline](https://github.com/devops247-online/k8s-operator-cloudflare/actions/workflows/ci.yml/badge.svg)](https://github.com/devops247-online/k8s-operator-cloudflare/actions/workflows/ci.yml)
[![GoSecScan](https://img.shields.io/github/actions/workflow/status/devops247-online/k8s-operator-cloudflare/ci.yml?label=gosec&logo=security&logoColor=white)](https://github.com/devops247-online/k8s-operator-cloudflare/actions/workflows/ci.yml)
[![CIS Benchmark](https://img.shields.io/badge/CIS%20Kubernetes%20Benchmark-compliant-success?logo=kubernetes&logoColor=white)](https://github.com/devops247-online/k8s-operator-cloudflare/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/devops247-online/k8s-operator-cloudflare/branch/main/graph/badge.svg)](https://codecov.io/gh/devops247-online/k8s-operator-cloudflare)
[![Go Report Card](https://goreportcard.com/badge/github.com/devops247-online/k8s-operator-cloudflare)](https://goreportcard.com/report/github.com/devops247-online/k8s-operator-cloudflare)

A Kubernetes operator for managing DNS records in Cloudflare using the official Cloudflare Go SDK.

## Description

This operator allows you to manage DNS records in Cloudflare directly from Kubernetes using Custom Resource Definitions (CRDs). It provides a declarative way to create, update, and delete DNS records while following Kubernetes best practices for operator development.

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/k8s-operator-cloudflare:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/k8s-operator-cloudflare:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/k8s-operator-cloudflare:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/k8s-operator-cloudflare/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Development

### Prerequisites for Development
- Go 1.24.0+
- Docker 17.03+
- kubectl v1.11.3+
- Access to a Kubernetes cluster (local or remote)
- golangci-lint (automatically installed via Makefile)

### Local Development Workflow

#### 1. Clone and Setup
```bash
git clone https://github.com/devops247-online/k8s-operator-cloudflare.git
cd k8s-operator-cloudflare

# Quick setup with provided script
./scripts/setup-dev.sh

# Or manually install development dependencies
make setup-envtest
make controller-gen
make kustomize
```

#### 2. Code Quality and Testing

**Run all tests:**
```bash
make test                    # Run unit tests with coverage
make test-e2e               # Run end-to-end tests (requires Kind cluster)
```

**Linting and code quality:**
```bash
make lint                   # Run golangci-lint
make lint-fix               # Run golangci-lint and auto-fix issues
make lint-config            # Verify linter configuration
make fmt                    # Format Go code
make vet                    # Run go vet
```

**Code generation:**
```bash
make generate               # Generate DeepCopy methods
make manifests              # Generate CRDs and RBAC
```

#### 3. Building and Testing

**Build the project:**
```bash
make build                  # Build manager binary to bin/
make docker-build           # Build Docker image
make build-installer        # Generate consolidated install.yaml
```

**Local development testing:**
```bash
# Run controller locally (requires kubeconfig)
make run

# Or install CRDs and run in-cluster
make install               # Install CRDs to cluster
make deploy               # Deploy controller to cluster
```

#### 4. Testing Different Scenarios

**Unit tests with specific coverage:**
```bash
# Test specific packages
go test -v ./api/v1 -coverprofile=api_cover.out
go test -v ./internal/controller -coverprofile=controller_cover.out

# View coverage report
go tool cover -html=cover.out -o coverage.html
```

**E2E testing workflow:**
```bash
make setup-test-e2e        # Create Kind cluster if needed
make test-e2e              # Run E2E tests
make cleanup-test-e2e      # Clean up Kind cluster
```

#### 5. Pre-commit Workflow

The project uses pre-commit hooks. Install and run them:
```bash
pip install pre-commit
pre-commit install
pre-commit run --all-files
```

**Hooks include:**
- Trailing whitespace removal
- End-of-file fixing
- YAML validation
- Secret detection
- Go linting

### Development Commands Reference

| Command | Description | When to Use |
|---------|-------------|-------------|
| `make test` | Run all unit tests with coverage | Before committing code |
| `make lint` | Run golangci-lint code analysis | Before committing code |
| `make lint-fix` | Auto-fix linting issues | To quickly resolve lint problems |
| `make fmt` | Format Go code | Before committing |
| `make vet` | Run go vet static analysis | Before committing |
| `make generate` | Generate DeepCopy methods | After modifying API types |
| `make manifests` | Generate CRDs and RBAC | After modifying API types or controller |
| `make build` | Build manager binary | To test local builds |
| `make run` | Run controller locally | For local development/debugging |
| `make docker-build` | Build container image | Before testing deployment |
| `make test-e2e` | Run end-to-end tests | Before releases |
| `make install` | Install CRDs to cluster | When testing CRD changes |
| `make deploy` | Deploy controller to cluster | When testing full deployment |

### Development Scripts

We provide convenient scripts for common development tasks:

```bash
./scripts/setup-dev.sh         # Initial development environment setup
./scripts/quick-test.sh        # Fast pre-commit checks
./scripts/coverage-report.sh   # Generate and view coverage report
./scripts/local-test.sh        # Comprehensive local testing (including E2E)
```

### Testing Your Changes

1. **Quick pre-commit check:**
   ```bash
   ./scripts/quick-test.sh       # Recommended before each commit
   ```

2. **Test API changes:**
   ```bash
   make generate manifests
   make test
   ```

3. **Full test suite with coverage:**
   ```bash
   ./scripts/coverage-report.sh  # Generates HTML coverage report
   ```

4. **Comprehensive testing:**
   ```bash
   ./scripts/local-test.sh       # Runs everything including E2E tests
   ```

### Code Coverage

Current test coverage:
- **Controller logic**: 100%
- **API types**: 100% (non-generated code)
- **Overall**: ~15% (includes auto-generated code)

**Note on API Coverage**: The API package shows low coverage (1.1%) because it primarily consists of auto-generated DeepCopy methods (`zz_generated.deepcopy.go`). These methods are:
- Generated automatically by controller-gen
- Tested implicitly through controller tests
- Industry standard practice to exclude from coverage metrics
- Regenerated on each `make generate` run

Generate coverage reports:
```bash
make test
go tool cover -html=cover.out -o coverage.html
# Open coverage.html in browser
```

## Security & Compliance

### CIS Kubernetes Benchmark

This operator is designed to comply with the [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes) security standards. Our CI/CD pipeline automatically validates compliance with CIS benchmarks on every commit.

#### Security Features

- **Pod Security Standards**: Enforces `restricted` security profile by default
- **Security Contexts**: All containers run as non-root with read-only root filesystem
- **Resource Limits**: Mandatory resource constraints to prevent resource exhaustion
- **Network Policies**: Supports network isolation and traffic control
- **RBAC**: Least privilege access with minimal required permissions

#### CIS Compliance Checks

The CI/CD pipeline includes automated CIS Kubernetes Benchmark validation:

```bash
# Run CIS benchmark checks locally

# For Kubernetes manifests
docker run --rm -v "$PWD:/src" aquasec/trivy:latest \
  k8s /src --compliance k8s-cis-1.23

# For Docker images
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  aquasec/trivy:latest image --compliance docker-cis <image-name>
```

#### Security Best Practices

1. **Secret Management**:
   - Use Kubernetes secrets for API tokens
   - Support for External Secrets Operator
   - Integration with HashiCorp Vault

2. **Image Security**:
   - Distroless base images
   - Regular vulnerability scanning with Trivy
   - Image signing support with cosign

3. **Runtime Security**:
   - Admission controller policies (OPA/Kyverno)
   - Runtime monitoring with Falco
   - Security event logging and alerting

For detailed security configuration, see [docs/SECURITY.md](docs/SECURITY.md).

## Contributing

We welcome contributions! Please follow these guidelines:

1. **Fork the repository** and create a feature branch
2. **Run all tests** and linting before submitting: `make test lint`
3. **Follow Go conventions** and add tests for new functionality
4. **Update documentation** if adding new features
5. **Ensure CI passes** - all checks must pass before merging

### Pull Request Process

1. Ensure your code passes all checks:
   ```bash
   make test lint fmt vet
   make test-e2e  # If applicable
   ```
2. Update documentation and examples if needed
3. Add tests for new functionality
4. Submit PR with clear description of changes

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
