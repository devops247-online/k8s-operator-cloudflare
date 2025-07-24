# Cloudflare DNS Operator Helm Chart

This Helm chart deploys the Cloudflare DNS Operator on a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- Cloudflare API Token with DNS edit permissions
- kubectl configured to communicate with your cluster

## Installation

### Basic Installation

Add the repository:
```bash
helm repo add cloudflare-dns-operator https://devops247-online.github.io/k8s-operator-cloudflare
helm repo update
```

Install the chart:
```bash
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  --namespace cloudflare-operator-system \
  --create-namespace
```

### Production Installation

For production environments with high availability and monitoring:
```bash
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  --namespace cloudflare-operator-system \
  --create-namespace \
  --set replicaCount=3 \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi \
  --set resources.limits.cpu=500m \
  --set resources.limits.memory=512Mi \
  --set podSecurityContext.runAsNonRoot=true \
  --set podSecurityContext.runAsUser=65534 \
  --set securityContext.allowPrivilegeEscalation=false \
  --set securityContext.readOnlyRootFilesystem=true \
  --set monitoring.enabled=true \
  --set monitoring.prometheusRule.enabled=true \
  -f production-values.yaml
```

### Development Installation

For development environments with minimal resources:
```bash
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  --namespace cloudflare-operator-system \
  --create-namespace \
  --set replicaCount=1 \
  --set resources.requests.cpu=10m \
  --set resources.requests.memory=64Mi \
  --set resources.limits.cpu=100m \
  --set resources.limits.memory=128Mi \
  --set env[0].name=LOG_LEVEL \
  --set env[0].value=debug
```

### Custom Registry Installation

When using a private container registry:
```bash
# Create image pull secret
kubectl create secret docker-registry regcred \
  --docker-server=your-registry.example.com \
  --docker-username=your-username \
  --docker-password=your-password \
  --docker-email=your-email@example.com \
  -n cloudflare-operator-system

# Install with custom registry
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  --namespace cloudflare-operator-system \
  --create-namespace \
  --set global.imageRegistry=your-registry.example.com \
  --set image.repository=your-org/cloudflare-dns-operator \
  --set imagePullSecrets[0].name=regcred
```

### CRD Management Scenarios

#### Install with CRDs
```bash
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  --namespace cloudflare-operator-system \
  --create-namespace \
  --set crds.install=true
```

#### Install without CRDs (when CRDs are managed separately)
```bash
# First install CRDs manually
kubectl apply -f https://raw.githubusercontent.com/devops247-online/k8s-operator-cloudflare/main/config/crd/bases/cloudflare.io_dnsrecords.yaml

# Then install the operator without CRDs
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  --namespace cloudflare-operator-system \
  --create-namespace \
  --set crds.install=false
```

#### Keep CRDs on uninstall
```bash
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  --namespace cloudflare-operator-system \
  --create-namespace \
  --set crds.install=true \
  --set crds.keep=true
```

## Configuration

### Complete Values Reference

| Parameter | Description | Default |
|-----------|-------------|---------|
| **Global Settings** |
| `global.imageRegistry` | Global Docker image registry | `""` |
| `global.imagePullSecrets` | Global Docker registry secret names | `[]` |
| **Replica Settings** |
| `replicaCount` | Number of operator replicas | `1` |
| **Image Configuration** |
| `image.registry` | Container image registry | `ghcr.io` |
| `image.repository` | Container image repository | `devops247-online/cloudflare-dns-operator` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.tag` | Overrides the image tag | `""` |
| `imagePullSecrets` | Docker registry secret names | `[]` |
| **Name Overrides** |
| `nameOverride` | String to partially override fullname | `""` |
| `fullnameOverride` | String to fully override fullname | `""` |
| **Service Account** |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.automount` | Automount service account token | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `serviceAccount.name` | Service account name override | `""` |
| **RBAC** |
| `rbac.create` | Create RBAC resources | `true` |
| **Pod Configuration** |
| `podAnnotations` | Pod annotations | `{}` |
| `podLabels` | Additional pod labels | `{}` |
| `podSecurityContext` | Pod security context | `{}` |
| `securityContext` | Container security context | `{}` |
| **Resources** |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `64Mi` |
| **Probes** |
| `livenessProbe.httpGet.path` | Liveness probe path | `/healthz` |
| `livenessProbe.httpGet.port` | Liveness probe port | `8081` |
| `livenessProbe.initialDelaySeconds` | Initial delay | `15` |
| `livenessProbe.periodSeconds` | Check period | `20` |
| `readinessProbe.httpGet.path` | Readiness probe path | `/readyz` |
| `readinessProbe.httpGet.port` | Readiness probe port | `8081` |
| `readinessProbe.initialDelaySeconds` | Initial delay | `5` |
| `readinessProbe.periodSeconds` | Check period | `10` |
| **Service** |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `service.metricsPort` | Metrics port | `8080` |
| **Monitoring** |
| `monitoring.enabled` | Enable monitoring | `false` |
| `monitoring.prometheusRule.enabled` | Create PrometheusRule | `false` |
| `monitoring.prometheusRule.labels` | PrometheusRule labels | `{}` |
| `monitoring.prometheusRule.rules` | Prometheus rules | `[]` |
| **Node Assignment** |
| `nodeSelector` | Node selector labels | `{}` |
| `tolerations` | Pod tolerations | `[]` |
| `affinity` | Pod affinity rules | `{}` |
| **Environment Variables** |
| `env` | Additional environment variables | `[]` |
| `envFrom` | Environment variables from sources | `[]` |
| **CRD Management** |
| `crds.install` | Install CRDs with chart | `true` |
| `crds.keep` | Keep CRDs on chart uninstall | `true` |
| **Cloudflare Configuration** |
| `cloudflare.email` | Cloudflare account email | `""` |
| `cloudflare.apiToken` | Cloudflare API Token | `""` |
| `cloudflare.apiTokenSecretName` | Existing secret name | `""` |
| `cloudflare.apiTokenSecretKey` | Key in secret | `api-token` |

### Example values.yaml for Production

```yaml
# production-values.yaml
replicaCount: 3

image:
  pullPolicy: Always

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 65534
  fsGroup: 65534

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: 65534
  capabilities:
    drop:
    - ALL

monitoring:
  enabled: true
  prometheusRule:
    enabled: true
    labels:
      prometheus: kube-prometheus
    rules:
    - alert: CloudflareOperatorDown
      expr: up{job="cloudflare-dns-operator"} == 0
      for: 5m
      labels:
        severity: critical
      annotations:
        summary: "Cloudflare DNS Operator is down"
        description: "Cloudflare DNS Operator has been down for more than 5 minutes."

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - cloudflare-dns-operator
        topologyKey: kubernetes.io/hostname
```

## Upgrading

### Upgrade Process

1. **Check the release notes** for any breaking changes:
   ```bash
   helm repo update
   helm search repo cloudflare-dns-operator/cloudflare-dns-operator --versions
   ```

2. **Backup your custom values**:
   ```bash
   helm get values cloudflare-dns-operator -n cloudflare-operator-system > values-backup.yaml
   ```

3. **Perform the upgrade**:
   ```bash
   helm upgrade cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
     -n cloudflare-operator-system \
     -f values-backup.yaml \
     --atomic \
     --timeout 5m
   ```

4. **Verify the upgrade**:
   ```bash
   kubectl get pods -n cloudflare-operator-system
   helm list -n cloudflare-operator-system
   ```

### Rolling Back

If issues occur during upgrade:
```bash
helm rollback cloudflare-dns-operator -n cloudflare-operator-system
```

### CRD Upgrades

When CRDs change between versions:
```bash
# If managing CRDs separately
kubectl apply -f https://raw.githubusercontent.com/devops247-online/k8s-operator-cloudflare/v{NEW_VERSION}/config/crd/bases/cloudflare.io_dnsrecords.yaml

# Then upgrade the operator
helm upgrade cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  -n cloudflare-operator-system \
  --set crds.install=false
```

## Security Best Practices

### API Token Management

1. **Use Kubernetes Secrets** for API tokens:
   ```bash
   kubectl create secret generic cloudflare-api-token \
     --from-literal=api-token=your-cloudflare-api-token \
     -n cloudflare-operator-system
   ```

2. **Reference the secret** in your values:
   ```yaml
   cloudflare:
     apiTokenSecretName: cloudflare-api-token
     apiTokenSecretKey: api-token
   ```

### Pod Security

1. **Enable Pod Security Standards**:
   ```yaml
   podSecurityContext:
     runAsNonRoot: true
     runAsUser: 65534
     fsGroup: 65534
     seccompProfile:
       type: RuntimeDefault

   securityContext:
     allowPrivilegeEscalation: false
     readOnlyRootFilesystem: true
     runAsNonRoot: true
     runAsUser: 65534
     capabilities:
       drop:
       - ALL
   ```

2. **Use Network Policies** to restrict traffic:
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: cloudflare-dns-operator
     namespace: cloudflare-operator-system
   spec:
     podSelector:
       matchLabels:
         app.kubernetes.io/name: cloudflare-dns-operator
     policyTypes:
     - Ingress
     - Egress
     ingress:
     - from:
       - podSelector:
           matchLabels:
             app.kubernetes.io/name: prometheus
       ports:
       - protocol: TCP
         port: 8080
     egress:
     - to:
       - namespaceSelector: {}
       ports:
       - protocol: TCP
         port: 443  # Cloudflare API
     - to:
       - namespaceSelector: {}
         podSelector:
           matchLabels:
             k8s-app: kube-dns
       ports:
       - protocol: UDP
         port: 53
   ```

### RBAC Configuration

The operator requires minimal permissions. Review and adjust RBAC if needed:
```bash
kubectl describe clusterrole cloudflare-dns-operator -n cloudflare-operator-system
```

## Troubleshooting

### Common Issues

#### 1. Operator Pod Not Starting

**Symptoms**: Pod in `CrashLoopBackOff` or `Error` state

**Check logs**:
```bash
kubectl logs -n cloudflare-operator-system deployment/cloudflare-dns-operator
```

**Common causes**:
- Missing or invalid Cloudflare API token
- Insufficient permissions
- Resource constraints

**Solution**:
```bash
# Check events
kubectl describe pod -n cloudflare-operator-system -l app.kubernetes.io/name=cloudflare-dns-operator

# Verify secret exists
kubectl get secret -n cloudflare-operator-system

# Check resource usage
kubectl top pod -n cloudflare-operator-system
```

#### 2. DNS Records Not Being Created

**Symptoms**: DNSRecord resources created but DNS entries not appearing in Cloudflare

**Check operator logs**:
```bash
kubectl logs -n cloudflare-operator-system deployment/cloudflare-dns-operator -f
```

**Check DNSRecord status**:
```bash
kubectl describe dnsrecord <record-name> -n <namespace>
```

**Common causes**:
- Invalid API token permissions
- Zone ID mismatch
- Rate limiting from Cloudflare

#### 3. Webhook Certificate Issues

**Symptoms**: Admission webhook errors

**Solution**:
```bash
# Delete webhook certificates to force regeneration
kubectl delete secret webhook-server-cert -n cloudflare-operator-system

# Restart operator
kubectl rollout restart deployment/cloudflare-dns-operator -n cloudflare-operator-system
```

#### 4. High Memory Usage

**Symptoms**: OOMKilled errors

**Solution**:
```bash
# Increase memory limits
helm upgrade cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  -n cloudflare-operator-system \
  --set resources.limits.memory=512Mi \
  --set resources.requests.memory=256Mi
```

### Debug Mode

Enable debug logging:
```bash
helm upgrade cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  -n cloudflare-operator-system \
  --set env[0].name=LOG_LEVEL \
  --set env[0].value=debug
```

### Health Checks

Check operator health:
```bash
# Port-forward to access health endpoints
kubectl port-forward -n cloudflare-operator-system deployment/cloudflare-dns-operator 8081:8081

# In another terminal
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

### Metrics and Monitoring

Access Prometheus metrics:
```bash
kubectl port-forward -n cloudflare-operator-system deployment/cloudflare-dns-operator 8080:8080
curl http://localhost:8080/metrics
```

## Uninstalling the Chart

To uninstall/delete the deployment:

```bash
helm uninstall cloudflare-dns-operator -n cloudflare-operator-system
```

To also remove CRDs (if `crds.keep` was false):
```bash
kubectl delete crd dnsrecords.cloudflare.io
```

To completely remove the namespace:
```bash
kubectl delete namespace cloudflare-operator-system
```

## Support

For issues and feature requests, please open an issue at:
https://github.com/devops247-online/k8s-operator-cloudflare/issues
