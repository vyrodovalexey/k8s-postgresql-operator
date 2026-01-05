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

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Image repository | `k8s-postgresql-operator` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `replicaCount` | Number of replicas | `1` |
| `serviceAccount.create` | Create service account | `true` |
| `vault.addr` | Vault server address | `http://0.0.0.0:8200` |
| `vault.role` | Vault role for Kubernetes auth | `role` |
| `vault.mountPoint` | Vault KV mount point | `secret` |
| `vault.secretPath` | Vault secret path prefix | `pdb` |
| `operator.enableLeaderElection` | Enable leader election | `true` |
| `operator.excludeUserList` | Comma-separated list of excluded users | `postgres` |
| `operator.postgresqlConnectionRetries` | PostgreSQL connection retries | `3` |
| `operator.postgresqlConnectionTimeoutSecs` | PostgreSQL connection timeout (seconds) | `10` |
| `operator.vaultAvailabilityRetries` | Vault availability retries | `3` |
| `operator.vaultAvailabilityRetryDelaySecs` | Vault availability retry delay (seconds) | `10` |
| `webhook.certPath` | Webhook certificate path (empty for auto-generation) | `""` |
| `webhook.certName` | Webhook certificate file name | `tls.crt` |
| `webhook.certKey` | Webhook key file name | `tls.key` |
| `webhook.serverPort` | Webhook server port | `8443` |
| `webhook.k8sServiceName` | Kubernetes service name for webhook | `k8s-postgresql-operator-controller-service` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `128Mi` |
| `resources.requests.cpu` | CPU request | `10m` |
| `resources.requests.memory` | Memory request | `64Mi` |

## Example Installation

```bash
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set image.repository=my-registry/k8s-postgresql-operator \
  --set image.tag=v0.1.0
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

