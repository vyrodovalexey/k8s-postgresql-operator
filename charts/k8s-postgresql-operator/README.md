# k8s-postgresql-operator

A Helm chart for k8s-postgresql-operator

## Introduction

This chart deploys the k8s-postgresql-operator on a Kubernetes cluster using the Helm package manager.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.0+
- Vault server (for credential storage)

## Installing the Chart

To install the chart with the release name `my-release`:

```bash
helm install my-release ./charts/k8s-postgresql-operator
```

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```bash
helm delete my-release
```

## Configuration

The following table lists the configurable parameters and their default values:

### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `k8s-postgresql-operator` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `replicaCount` | Number of replicas | `1` |

### Service Account

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `serviceAccount.name` | Service account name | `""` |

### Vault Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `vault.addr` | Vault server address | `http://0.0.0.0:8200` |
| `vault.role` | Vault role for Kubernetes auth | `role` |
| `vault.mountPoint` | Vault KV mount point | `secret` |
| `vault.secretPath` | Vault secret path prefix | `pdb` |

### Vault PKI Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `vaultPKI.enabled` | Enable Vault PKI for webhook certificates | `false` |
| `vaultPKI.mountPath` | Vault PKI secrets engine mount path | `pki` |
| `vaultPKI.role` | Vault PKI role name for certificate issuance | `webhook-cert` |
| `vaultPKI.ttl` | TTL for issued certificates | `720h` |
| `vaultPKI.renewalBuffer` | Time before expiry to trigger certificate renewal | `24h` |

### Operator Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.enableLeaderElection` | Enable leader election | `true` |
| `operator.probeAddr` | Health probe bind address | `:8081` |
| `operator.excludeUserList` | Comma-separated list of excluded users | `postgres` |
| `operator.postgresqlConnectionRetries` | PostgreSQL connection retries | `3` |
| `operator.postgresqlConnectionTimeoutSecs` | PostgreSQL connection timeout (seconds) | `10` |
| `operator.vaultAvailabilityRetries` | Vault availability retries | `3` |
| `operator.vaultAvailabilityRetryDelaySecs` | Vault availability retry delay (seconds) | `10` |
| `operator.vaultAuthTimeoutSecs` | Vault authentication timeout (seconds) | `30` |
| `operator.logLevel` | Log level: debug, info, warn, error | `info` |
| `operator.postgresqlRateLimitPerSecond` | Rate limit for PostgreSQL connections per second | `10` |
| `operator.postgresqlRateLimitBurst` | Burst limit for PostgreSQL connections | `20` |
| `operator.vaultRateLimitPerSecond` | Rate limit for Vault requests per second | `10` |
| `operator.vaultRateLimitBurst` | Burst limit for Vault requests | `20` |

### Retry Configuration (Exponential Backoff)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `operator.retry.initialIntervalMs` | Initial interval in milliseconds for exponential backoff | `500` |
| `operator.retry.maxIntervalSecs` | Maximum interval in seconds for exponential backoff | `30` |
| `operator.retry.maxElapsedTimeSecs` | Maximum elapsed time in seconds for all retries | `300` |
| `operator.retry.multiplier` | Multiplier for exponential backoff interval growth | `2.0` |
| `operator.retry.randomizationFactor` | Randomization factor (jitter) for exponential backoff | `0.5` |

### Webhook Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `webhook.certPath` | Webhook certificate path (empty for auto-generation) | `""` |
| `webhook.certName` | Webhook certificate file name | `tls.crt` |
| `webhook.certKey` | Webhook key file name | `tls.key` |
| `webhook.serverPort` | Webhook server port | `8443` |
| `webhook.k8sServiceName` | Kubernetes service name for webhook | `k8s-postgresql-operator-controller-service` |
| `webhooks.enabled` | Enable validating webhooks | `true` |
| `webhooks.failurePolicy` | Webhook failure policy | `Fail` |
| `webhooks.sideEffects` | Webhook side effects | `None` |

### Resources

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `64Mi` |

### Pod Disruption Budget

| Parameter | Description | Default |
|-----------|-------------|---------|
| `podDisruptionBudget.enabled` | Enable Pod Disruption Budget | `false` |
| `podDisruptionBudget.minAvailable` | Minimum available pods | `nil` |
| `podDisruptionBudget.maxUnavailable` | Maximum unavailable pods | `nil` |

### Metrics

| Parameter | Description | Default |
|-----------|-------------|---------|
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.serviceMonitor.enabled` | Enable ServiceMonitor (requires Prometheus Operator) | `false` |
| `metrics.serviceMonitor.namespace` | ServiceMonitor namespace | `""` |
| `metrics.serviceMonitor.interval` | Scrape interval | `30s` |
| `metrics.serviceMonitor.scrapeTimeout` | Scrape timeout | `10s` |
| `metrics.serviceMonitor.labels` | Additional labels for ServiceMonitor | `{}` |

## Example Installation

### Basic Installation

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set image.repository=my-registry/k8s-postgresql-operator \
  --set image.tag=v0.1.0
```

### Installation with Custom Values File

Create a `values.yaml` file:

```yaml
# values.yaml
image:
  repository: my-registry/k8s-postgresql-operator
  tag: v0.1.0
  pullPolicy: IfNotPresent

vault:
  addr: "https://vault.production.com:8200"
  role: "k8s-postgresql-operator"
  mountPoint: "secret"
  secretPath: "pdb"

operator:
  enableLeaderElection: true
  logLevel: "info"
  postgresqlRateLimitPerSecond: 10
  vaultRateLimitPerSecond: 10

resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

Install with the values file:

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator -f values.yaml
```

### Installation with Vault PKI

When using Vault PKI for webhook certificates, the operator will automatically obtain TLS certificates from Vault instead of generating self-signed certificates:

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set vaultPKI.enabled=true \
  --set vaultPKI.mountPath=pki \
  --set vaultPKI.role=webhook-cert \
  --set vaultPKI.ttl=720h \
  --set vaultPKI.renewalBuffer=24h
```

### Installation with Prometheus Monitoring

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.labels.release=prometheus
```

### Installation with Pod Disruption Budget

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set replicaCount=2 \
  --set podDisruptionBudget.enabled=true \
  --set podDisruptionBudget.minAvailable=1
```

## Custom Resource Definitions (CRDs)

The chart includes the following CRDs:
- `postgresqls.postgresql-operator.vyrodovalexey.github.com`
- `users.postgresql-operator.vyrodovalexey.github.com`
- `databases.postgresql-operator.vyrodovalexey.github.com`
- `grants.postgresql-operator.vyrodovalexey.github.com`
- `rolegroups.postgresql-operator.vyrodovalexey.github.com`
- `schemas.postgresql-operator.vyrodovalexey.github.com`

These CRDs are automatically installed when the chart is deployed.

## Validating Webhooks

The chart creates ValidatingWebhookConfigurations for all CRD types:
- PostgreSQL
- User
- Database
- Grant
- RoleGroup
- Schema

Webhooks are enabled by default and can be disabled by setting `webhooks.enabled=false`.

## Vault PKI Setup

The operator supports Vault PKI for automatic webhook certificate management. This provides better security than self-signed certificates.

### Prerequisites

- Vault server with PKI secrets engine enabled
- Kubernetes authentication configured in Vault
- Vault role with appropriate permissions

### Quick Setup

Use the provided setup script for automated configuration:

```bash
# From the project root
./test/scripts/setup-vault-pki.sh
```

### Manual Setup

1. **Enable PKI secrets engine**:
```bash
vault secrets enable pki
vault secrets tune -max-lease-ttl=8760h pki
```

2. **Generate root CA**:
```bash
vault write pki/root/generate/internal \
    common_name="K8s PostgreSQL Operator CA" \
    ttl=8760h
```

3. **Create PKI role for webhook certificates**:
```bash
vault write pki/roles/webhook-cert \
    allowed_domains="k8s-postgresql-operator-controller-service,k8s-postgresql-operator-controller-service.default,k8s-postgresql-operator-controller-service.default.svc,k8s-postgresql-operator-controller-service.default.svc.cluster.local" \
    allow_subdomains=false \
    max_ttl=720h \
    generate_lease=true
```

4. **Create PKI policy**:
```bash
vault policy write k8s-postgresql-operator-pki - <<EOF
path "pki/issue/webhook-cert" {
  capabilities = ["create", "update"]
}
path "pki/ca/pem" {
  capabilities = ["read"]
}
path "pki/ca_chain" {
  capabilities = ["read"]
}
EOF
```

5. **Update Kubernetes auth role**:
```bash
vault write auth/kubernetes/role/k8s-postgresql-operator \
    bound_service_account_names=k8s-postgresql-operator-controller \
    bound_service_account_namespaces=default \
    policies=k8s-postgresql-operator,k8s-postgresql-operator-pki \
    ttl=1h
```

### Certificate Provider Selection

The operator selects certificate providers in this order:

1. **Provided Certificates**: If `webhook.certPath` is set
2. **Vault PKI**: If `vaultPKI.enabled=true`
3. **Self-Signed**: Default behavior

### Vault PKI Configuration Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `vaultPKI.enabled` | Enable Vault PKI for webhook certificates | `false` |
| `vaultPKI.mountPath` | Vault PKI secrets engine mount path | `pki` |
| `vaultPKI.role` | Vault PKI role name for certificate issuance | `webhook-cert` |
| `vaultPKI.ttl` | TTL for issued certificates | `720h` |
| `vaultPKI.renewalBuffer` | Time before expiry to trigger renewal | `24h` |

## Credential Storage Patterns

The operator stores PostgreSQL user credentials in Vault using this path structure:

```
{vault.mountPoint}/data/{vault.secretPath}/{postgresql_id}/{username}
```

**Example path:**
```
secret/data/pdb/test-pg-001/app_user
```

Where:
- `vault.mountPoint`: KV v2 mount point (default: `secret`)
- `vault.secretPath`: Secret path prefix (default: `pdb`)
- `postgresql_id`: PostgreSQL instance ID from CRD (e.g., `test-pg-001`)
- `username`: PostgreSQL username (e.g., `app_user`)

### Credential Format

Stored credentials include:
```json
{
  "password": "generated-password",
  "username": "app_user",
  "postgresql_id": "test-pg-001",
  "created_at": "2024-01-30T10:00:00Z",
  "updated_at": "2024-01-30T10:00:00Z"
}
```

## Advanced Configuration Examples

### High Availability Setup

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set replicaCount=2 \
  --set operator.enableLeaderElection=true \
  --set podDisruptionBudget.enabled=true \
  --set podDisruptionBudget.minAvailable=1 \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi \
  --set resources.limits.cpu=500m \
  --set resources.limits.memory=256Mi
```

### Production Setup with Vault PKI and Monitoring

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=https://vault.production.com:8200 \
  --set vault.role=k8s-postgresql-operator \
  --set vaultPKI.enabled=true \
  --set vaultPKI.ttl=168h \
  --set vaultPKI.renewalBuffer=24h \
  --set metrics.enabled=true \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.labels.release=prometheus \
  --set operator.enableLeaderElection=true \
  --set replicaCount=2 \
  --set podDisruptionBudget.enabled=true \
  --set podDisruptionBudget.minAvailable=1
```

### Development Setup with Local Vault

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.default.svc.cluster.local:8200 \
  --set vault.role=k8s-postgresql-operator \
  --set operator.enableLeaderElection=false \
  --set replicaCount=1 \
  --set image.pullPolicy=Always
```

## Testing the Installation

### Verify Operator Status

```bash
# Check operator pod status
kubectl get pods -l app.kubernetes.io/name=k8s-postgresql-operator

# Check operator logs
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator

# Verify CRDs are installed
kubectl get crd | grep postgresql-operator
```

### Test Basic Functionality

```bash
# Create a test PostgreSQL instance
kubectl apply -f - <<EOF
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Postgresql
metadata:
  name: test-postgresql
spec:
  externalInstance:
    postgresqlID: "test-instance"
    address: "postgres.example.com"
    port: 5432
    sslMode: "require"
EOF

# Check the resource status
kubectl describe postgresql test-postgresql
```

## Troubleshooting

### Common Issues

#### Operator Pod Not Starting

```bash
# Check pod events
kubectl describe pod -l app.kubernetes.io/name=k8s-postgresql-operator

# Check service account permissions
kubectl auth can-i create postgresqls --as=system:serviceaccount:default:k8s-postgresql-operator-controller
```

#### Vault Connection Issues

```bash
# Test Vault connectivity from cluster
kubectl run vault-test --rm -it --image=curlimages/curl -- \
  curl -s http://vault.example.com:8200/v1/sys/health

# Check Vault authentication
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator | grep vault
```

#### Webhook Certificate Issues

```bash
# Check webhook configuration
kubectl get validatingwebhookconfiguration | grep k8s-postgresql-operator

# Check certificate status (if using Vault PKI)
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator | grep certificate
```

## Upgrading

### From 0.0.x to 0.1.x

No breaking changes. New features:
- Vault PKI support for webhook certificates
- Pod Disruption Budget support
- ServiceMonitor support for Prometheus Operator
- Enhanced credential storage patterns
- Exponential backoff retry logic with configurable parameters
- Rate limiting for PostgreSQL and Vault API calls
- Comprehensive Kubernetes event recording
- Configurable log levels with structured JSON logging
- Enhanced metrics for monitoring rate limiting and retries
- Improved certificate management with multiple providers
- Vault authentication timeout configuration
- Improved testing and documentation

### Upgrade Process

```bash
# Update the chart
helm upgrade k8s-postgresql-operator ./charts/k8s-postgresql-operator

# Verify upgrade
kubectl rollout status deployment/k8s-postgresql-operator-controller
```

## Vault Integration Setup

### Prerequisites for Vault Integration

Before installing the operator, ensure your Vault instance is properly configured:

1. **Enable KV v2 secrets engine**:
   ```bash
   vault secrets enable -path=secret kv-v2
   ```

2. **Enable Kubernetes authentication**:
   ```bash
   vault auth enable kubernetes
   vault write auth/kubernetes/config \
       token_reviewer_jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
       kubernetes_host="https://kubernetes.default.svc:443" \
       kubernetes_ca_cert="$(cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt)"
   ```

3. **Create operator policy**:
   ```bash
   vault policy write k8s-postgresql-operator - <<EOF
   path "secret/data/pdb/*" {
     capabilities = ["create", "read", "update", "delete"]
   }
   path "secret/metadata/pdb/*" {
     capabilities = ["list", "delete"]
   }
   EOF
   ```

4. **Create Kubernetes auth role**:
   ```bash
   vault write auth/kubernetes/role/k8s-postgresql-operator \
       bound_service_account_names=k8s-postgresql-operator-controller \
       bound_service_account_namespaces=default \
       policies=k8s-postgresql-operator \
       ttl=1h
   ```

### Path Structure for Credentials

The operator stores PostgreSQL credentials using this path pattern:

```
{vault.mountPoint}/data/{vault.secretPath}/{instance_type}/{postgresql_id}/{username}
```

**Example**: `secret/data/pdb/postgresql/my-postgresql-instance/app_user`

## Documentation

For more detailed information, see:

- [Getting Started Guide](../../docs/getting-started.md)
- [Vault Integration Guide](../../docs/vault-integration.md)
- [Local Development Guide](../../docs/local-development.md)
- [Testing Guide](../../docs/testing.md)
- [Configuration Guide](../../docs/configuration.md)
- [Main README](../../README.md)

