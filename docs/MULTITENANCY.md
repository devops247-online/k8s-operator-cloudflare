# Multi-tenancy Configuration Guide

This document describes how to configure and use multi-tenancy features in the Cloudflare DNS Operator.

## Overview

Multi-tenancy allows a single Cloudflare DNS Operator instance to serve multiple isolated tenants, providing:
- **Security isolation** between tenants
- **Resource quotas** per tenant
- **Flexible deployment scopes**
- **Tenant-specific RBAC** controls

## Deployment Modes

### 1. Cluster Scope (Default)
The operator watches all namespaces in the cluster.

```yaml
multitenancy:
  enabled: false  # Uses traditional cluster-wide mode
  scope: cluster
```

**Use case**: Single tenant environment or trusted multi-tenant setup.

### 2. Namespace Scope
The operator only watches a single namespace (typically its own).

```yaml
multitenancy:
  enabled: true
  scope: namespace
  validation:
    enforceNamespaceSecrets: true
  resourceQuotas:
    enabled: true
    limits:
      maxRecords: 50
```

**Use case**:
- Dedicated operator instance per tenant
- Maximum isolation
- SaaS platforms with tenant-per-namespace

### 3. Multi-namespace Scope
The operator watches specific pre-defined namespaces.

```yaml
multitenancy:
  enabled: true
  scope: multi-namespace
  watchNamespaces:
    - tenant-a
    - tenant-b
    - tenant-c
  validation:
    enforceNamespaceSecrets: true
    validateZoneOwnership: true
    allowedZones:
      tenant-a: ["customer-a.com", "*.dev.customer-a.com"]
      tenant-b: ["customer-b.org", "api.customer-b.org"]
      tenant-c: ["customer-c.net"]
```

**Use case**:
- Managed hosting providers
- Platform teams serving multiple development teams
- Controlled multi-tenancy

## Configuration Options

### Basic Multi-tenancy
```yaml
multitenancy:
  enabled: true
  scope: multi-namespace
  watchNamespaces:
    - production
    - staging
    - development
```

### Resource Quotas
Limit resources per tenant namespace:

```yaml
multitenancy:
  resourceQuotas:
    enabled: true
    limits:
      maxRecords: 100        # Max CloudflareRecord resources
      cpu: "1000m"          # CPU limit for workloads
      memory: "512Mi"       # Memory limit for workloads
```

### Validation Controls
```yaml
multitenancy:
  validation:
    # Enforce secrets must be in same namespace as CloudflareRecord
    enforceNamespaceSecrets: true

    # Validate zone ownership (requires webhook)
    validateZoneOwnership: true

    # Define allowed zones per namespace
    allowedZones:
      production: ["company.com", "api.company.com"]
      staging: ["*.staging.company.com"]
      development: ["*.dev.company.com", "*.test.company.com"]
```

### Network Isolation
```yaml
multitenancy:
  networkPolicies:
    enabled: true  # Creates NetworkPolicies for tenant isolation
```

## Security Features

### 1. Namespace Secret Enforcement
When `enforceNamespaceSecrets: true`, CloudflareRecord resources can only reference secrets from the same namespace:

```yaml
# ✅ Valid - secret in same namespace
apiVersion: dns.cloudflare.io/v1
kind: CloudflareRecord
metadata:
  name: my-record
  namespace: tenant-a
spec:
  cloudflareCredentialsSecretRef:
    name: tenant-a-cloudflare-token
    namespace: tenant-a  # Same namespace
```

```yaml
# ❌ Invalid - cross-namespace secret access blocked
apiVersion: dns.cloudflare.io/v1
kind: CloudflareRecord
metadata:
  name: my-record
  namespace: tenant-a
spec:
  cloudflareCredentialsSecretRef:
    name: tenant-b-cloudflare-token
    namespace: tenant-b  # Different namespace - blocked!
```

### 2. Zone Ownership Validation
Prevents tenants from managing DNS records for zones they don't own:

```yaml
# Configuration
multitenancy:
  validation:
    allowedZones:
      tenant-a: ["customer-a.com", "*.customer-a.com"]

# ✅ Valid - zone matches allowed pattern
apiVersion: dns.cloudflare.io/v1
kind: CloudflareRecord
metadata:
  namespace: tenant-a
spec:
  zone: "customer-a.com"  # Allowed
  name: "api.customer-a.com"

# ❌ Invalid - zone not in allowed list
apiVersion: dns.cloudflare.io/v1
kind: CloudflareRecord
metadata:
  namespace: tenant-a
spec:
  zone: "customer-b.com"  # Not allowed for tenant-a
```

## RBAC Model

### Cluster Scope RBAC
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cloudflare-dns-operator-multitenancy
rules:
  - apiGroups: ["dns.cloudflare.io"]
    resources: ["cloudflarerecords", "cloudflarerecords/status"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["secrets", "namespaces"]
    verbs: ["get", "list", "watch"]
```

### Namespace Scope RBAC
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cloudflare-dns-operator
  namespace: tenant-a
rules:
  - apiGroups: ["dns.cloudflare.io"]
    resources: ["cloudflarerecords", "cloudflarerecords/status"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch"]
```

## Resource Quotas

ResourceQuotas are automatically created for tenant namespaces:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: cloudflare-dns-operator-tenant-quota
  namespace: tenant-a
spec:
  hard:
    count/cloudflarerecords.dns.cloudflare.io: "100"
    limits.cpu: "1000m"
    limits.memory: "512Mi"
```

## Deployment Examples

### Example 1: SaaS Platform
```yaml
# values.yaml for SaaS provider
multitenancy:
  enabled: true
  scope: namespace
  validation:
    enforceNamespaceSecrets: true
    validateZoneOwnership: true
  resourceQuotas:
    enabled: true
    limits:
      maxRecords: 50
      cpu: "500m"
      memory: "256Mi"
  networkPolicies:
    enabled: true

# Deploy one operator per customer namespace
helm install customer-a-dns ./charts/cloudflare-dns-operator \
  --namespace customer-a-system \
  --create-namespace \
  -f saas-values.yaml
```

### Example 2: Corporate Multi-team
```yaml
# values.yaml for corporate deployment
multitenancy:
  enabled: true
  scope: multi-namespace
  watchNamespaces:
    - development
    - staging  
    - production
  validation:
    enforceNamespaceSecrets: true
    allowedZones:
      development: ["*.dev.company.com"]
      staging: ["*.staging.company.com"]
      production: ["company.com", "api.company.com"]
  resourceQuotas:
    enabled: true
    limits:
      maxRecords: 200

helm install dns-operator ./charts/cloudflare-dns-operator \
  --namespace dns-operator-system \
  --create-namespace \
  -f corporate-values.yaml
```

### Example 3: Hosting Provider
```yaml
# values.yaml for hosting provider
multitenancy:
  enabled: true
  scope: multi-namespace
  watchNamespaces:
    - customer-1
    - customer-2
    - customer-3
  validation:
    enforceNamespaceSecrets: true
    validateZoneOwnership: true
    allowedZones:
      customer-1: ["customer1.com", "*.customer1.com"]
      customer-2: ["customer2.org", "mail.customer2.org"]
      customer-3: ["customer3.net"]
  resourceQuotas:
    enabled: true
    limits:
      maxRecords: 25  # Per customer limit
```

## Monitoring and Troubleshooting

### Environment Variables
The operator sets these environment variables based on configuration:

- `MULTITENANCY_ENABLED`: "true" when multi-tenancy is enabled
- `MULTITENANCY_SCOPE`: "cluster", "namespace", or "multi-namespace"
- `WATCH_NAMESPACE`: Single namespace to watch (namespace scope)
- `WATCH_NAMESPACES`: Comma-separated list (multi-namespace scope)
- `ENFORCE_NAMESPACE_SECRETS`: "true" to enforce secret locality
- `VALIDATE_ZONE_OWNERSHIP`: "true" to validate zone ownership
- `ALLOWED_ZONES_CONFIG`: JSON configuration of allowed zones

### Common Issues

1. **Cross-namespace secret access denied**
   - Check `enforceNamespaceSecrets` setting
   - Ensure secrets are in the same namespace as CloudflareRecord

2. **Zone ownership validation failed**
   - Verify `allowedZones` configuration
   - Check zone patterns match your DNS records

3. **ResourceQuota exceeded**
   - Check current usage: `kubectl describe quota -n <namespace>`
   - Adjust quota limits or clean up unused records

4. **RBAC permission denied**
   - Verify correct Role/ClusterRole is created
   - Check RoleBinding/ClusterRoleBinding subjects

### Metrics and Logging
Enable debug logging to troubleshoot multi-tenancy issues:

```yaml
operator:
  logLevel: debug
```

Look for log messages containing:
- `multitenancy` - general multi-tenancy operations
- `zone-validation` - zone ownership checks
- `namespace-validation` - namespace secret enforcement
- `quota-check` - resource quota validation

## Migration Guide

### From Single-tenant to Multi-tenant

1. **Backup existing configuration**
   ```bash
   kubectl get cloudflarerecords -A -o yaml > backup.yaml
   ```

2. **Update values.yaml**
   ```yaml
   multitenancy:
     enabled: true
     scope: cluster  # Start with cluster scope for compatibility
     validation:
       enforceNamespaceSecrets: false  # Initially permissive
   ```

3. **Deploy updated operator**
   ```bash
   helm upgrade dns-operator ./charts/cloudflare-dns-operator \
     -f updated-values.yaml
   ```

4. **Gradually enable restrictions**
   - Enable namespace secret enforcement
   - Add zone ownership validation
   - Add resource quotas

### Best Practices

1. **Start with permissive settings** and gradually tighten security
2. **Test in staging** before production deployment
3. **Monitor resource usage** with quotas enabled
4. **Use namespace labels** for tenant identification
5. **Document zone ownership** clearly for teams
6. **Regular audit** of RBAC permissions and quotas

## Troubleshooting Commands

```bash
# Check operator configuration
kubectl get deployment dns-operator -o yaml | grep -A 20 env:

# List all CloudflareRecords across namespaces
kubectl get cloudflarerecords -A

# Check ResourceQuota usage
kubectl describe quota -n tenant-a

# View operator logs
kubectl logs -n dns-operator-system deployment/dns-operator

# Validate RBAC permissions
kubectl auth can-i list cloudflarerecords --as=system:serviceaccount:dns-operator-system:dns-operator -n tenant-a
```
