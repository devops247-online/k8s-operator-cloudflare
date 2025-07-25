# Security Guide

This document covers security features and best practices for the Cloudflare DNS Operator.

## Table of Contents

- [Secret Management](#secret-management)
  - [Native Kubernetes Secrets](#native-kubernetes-secrets)
  - [External Secrets Operator](#external-secrets-operator)
  - [HashiCorp Vault](#hashicorp-vault)
  - [Secret Rotation](#secret-rotation)
- [Security Hardening](#security-hardening)
  - [Image Security](#image-security)
  - [Admission Controllers](#admission-controllers)
  - [Runtime Security](#runtime-security)
  - [Compliance](#compliance)
- [Best Practices](#best-practices)

## Secret Management

The operator supports multiple secret management providers to secure Cloudflare API credentials.

### Native Kubernetes Secrets

Default method using Kubernetes native secrets:

```yaml
# values.yaml
secrets:
  provider: kubernetes

cloudflare:
  apiTokenSecretName: "cloudflare-credentials"  # pragma: allowlist secret
  apiTokenSecretKey: "api-token"  # pragma: allowlist secret
```

Create the secret:  # pragma: allowlist secret
```bash
kubectl create secret generic cloudflare-credentials \  # pragma: allowlist secret
  --from-literal=api-token=your-api-token \
  --from-literal=email=your-email \
  --namespace cloudflare-operator-system
```

### External Secrets Operator

Integration with External Secrets Operator for centralized secret management:

```yaml
# values.yaml
secrets:
  provider: external-secrets-operator

  externalSecretsOperator:
    enabled: true
    secretStore:
      name: cloudflare-secret-store
      kind: SecretStore  # or ClusterSecretStore
      backend: vault
    refreshInterval: 1h
    externalSecret:
      name: cloudflare-credentials
      dataFrom:
        - extract:
            key: cloudflare/credentials
```

Supported backends:
- HashiCorp Vault
- AWS Secrets Manager
- Azure Key Vault
- Google Secret Manager
- 1Password
- Kubernetes

### HashiCorp Vault

Direct Vault integration:

```yaml
# values.yaml
secrets:
  provider: vault

  vault:
    enabled: true
    address: https://vault.example.com
    authMethod: kubernetes
    path: secret/data/cloudflare
    kubernetes:
      role: cloudflare-operator
      serviceAccount: cloudflare-dns-operator
    rotation:
      enabled: true
      schedule: "0 0 * * 0"  # Weekly
```

#### Vault Setup

1. Enable Kubernetes auth:
```bash
vault auth enable kubernetes
```

2. Configure Kubernetes auth:
```bash
vault write auth/kubernetes/config \
  kubernetes_host="https://$KUBERNETES_PORT_443_TCP_ADDR:443" \
  kubernetes_ca_cert=@/var/run/secrets/kubernetes.io/serviceaccount/ca.crt \
  token_reviewer_jwt=@/var/run/secrets/kubernetes.io/serviceaccount/token
```

3. Create policy:
```hcl
path "secret/data/cloudflare/*" {
  capabilities = ["read"]
}
```

4. Create role:
```bash
vault write auth/kubernetes/role/cloudflare-operator \
  bound_service_account_names=cloudflare-dns-operator \
  bound_service_account_namespaces=cloudflare-operator-system \
  policies=cloudflare-reader \
  ttl=24h
```

### Secret Rotation

Automated secret rotation configuration:

```yaml
secrets:
  vault:
    rotation:
      enabled: true
      schedule: "0 0 * * 0"  # Weekly rotation
```

For External Secrets Operator:
```yaml
secrets:
  externalSecretsOperator:
    refreshInterval: 1h  # Check for updates every hour
```

## Security Hardening

### Image Security

#### Vulnerability Scanning

The operator uses Trivy for automated vulnerability scanning in CI/CD:

```yaml
securityHardening:
  imageScanning:
    enabled: true
    scanner: trivy
    failOnCritical: true
    severityLevels:
      - CRITICAL
      - HIGH
    ignoreCVEs: []
```

#### Distroless Images

The operator uses distroless base images for minimal attack surface:
- No shell
- No package manager
- Minimal OS footprint
- Non-root user (65532)

#### Image Signing

Support for cosign image verification:

```yaml
securityHardening:
  imageSigning:
    enabled: true
    cosignPublicKey: |
      -----BEGIN PUBLIC KEY-----
      MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE...
      -----END PUBLIC KEY-----
    policyMode: enforce  # enforce, warn, disabled
```

### Admission Controllers

#### OPA (Open Policy Agent)

```yaml
securityHardening:
  admissionController:
    enabled: true
    engine: opa
    policies:
      requireSecurityContext: true
      disallowPrivileged: true
      requireNonRoot: true
      requireReadOnlyRootFS: true
```

#### Kyverno

```yaml
securityHardening:
  admissionController:
    enabled: true
    engine: kyverno
    policies:
      # Same policies as OPA
```

### Runtime Security

#### Falco Integration

```yaml
securityHardening:
  runtimeSecurity:
    enabled: true
    provider: falco
    alerting:
      enabled: true
      webhook: https://alerts.example.com/falco
```

#### Security Monitoring

```yaml
securityHardening:
  monitoring:
    enabled: true
    metrics:
      enabled: true
      port: 9091
    webhook:
      enabled: true
      url: https://siem.example.com/events
```

### Compliance

#### Benchmarks

The operator supports compliance checking against:
- CIS Kubernetes Benchmark
- NSA Kubernetes Hardening Guide

```yaml
securityHardening:
  compliance:
    enabled: true
    benchmarks:
      - cis-kubernetes
      - nsa-kubernetes
    reporting:
      enabled: true
      schedule: "0 0 * * 0"  # Weekly reports
```

## Best Practices

### 1. Least Privilege

- Use namespace-scoped deployments when possible
- Limit RBAC permissions to minimum required
- Use resource quotas

### 2. Network Security

- Enable NetworkPolicies
- Use TLS for all communications
- Restrict egress to Cloudflare API only

### 3. Secret Management

- Never store secrets in code or ConfigMaps
- Use external secret management systems
- Enable secret rotation
- Audit secret access

### 4. Container Security

- Run as non-root user
- Use read-only root filesystem
- Drop all capabilities
- Use security contexts

### 5. Supply Chain Security

- Verify image signatures
- Scan images for vulnerabilities
- Use distroless or minimal base images
- Generate and verify SBOM

### 6. Monitoring and Auditing

- Enable audit logging
- Monitor security events
- Set up alerts for anomalies
- Regular compliance scans

### 7. Regular Updates

- Keep operator updated
- Update base images
- Patch vulnerabilities promptly
- Review security policies

## Security Contacts

Report security vulnerabilities to: security@cloudflare-operator.io

## Additional Resources

- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [NSA Kubernetes Hardening Guide](https://www.nsa.gov/Press-Room/News-Highlights/Article/Article/2716980/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
