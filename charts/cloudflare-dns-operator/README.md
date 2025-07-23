# Cloudflare DNS Operator Helm Chart

This Helm chart deploys the Cloudflare DNS Operator on a Kubernetes cluster.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- Cloudflare API Token with DNS edit permissions

## Installation

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

## Configuration

The following table lists the configurable parameters of the Cloudflare DNS Operator chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `1` |
| `image.repository` | Operator image repository | `cloudflare-dns-operator` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.tag` | Overrides the image tag | `""` |
| `imagePullSecrets` | Image pull secrets | `[]` |
| `nameOverride` | String to partially override fullname | `""` |
| `fullnameOverride` | String to fully override fullname | `""` |
| `serviceAccount.create` | Specifies whether a service account should be created | `true` |
| `serviceAccount.annotations` | Annotations to add to the service account | `{}` |
| `serviceAccount.name` | The name of the service account to use | `""` |
| `rbac.create` | Create RBAC resources | `true` |
| `podAnnotations` | Pod annotations | `{}` |
| `podSecurityContext` | Pod security context | `{}` |
| `securityContext` | Container security context | `{}` |
| `resources` | CPU/Memory resource requests/limits | `{}` |
| `nodeSelector` | Node labels for pod assignment | `{}` |
| `tolerations` | Tolerations for pod assignment | `[]` |
| `affinity` | Affinity for pod assignment | `{}` |

### Cloudflare Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `cloudflare.email` | Cloudflare account email | `""` |
| `cloudflare.apiToken` | Cloudflare API Token | `""` |
| `cloudflare.apiTokenSecretName` | Name of existing secret containing API token | `""` |
| `cloudflare.apiTokenSecretKey` | Key in the secret containing API token | `api-token` |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example:

```bash
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator \
  --set cloudflare.email=your-email@example.com \
  --set cloudflare.apiToken=your-api-token
```

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart:

```bash
helm install cloudflare-dns-operator cloudflare-dns-operator/cloudflare-dns-operator -f values.yaml
```

## Uninstalling the Chart

To uninstall/delete the deployment:

```bash
helm uninstall cloudflare-dns-operator -n cloudflare-operator-system
```

The command removes all the Kubernetes components associated with the chart and deletes the release.
