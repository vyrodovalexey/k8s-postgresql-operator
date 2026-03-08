# k8s-postgresql-operator

A Kubernetes operator for managing PostgreSQL databases, users, grants, role groups, and schemas across multiple PostgreSQL instances. The operator integrates with HashiCorp Vault for secure credential storage and provides comprehensive validation through Kubernetes webhooks.

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Custom Resource Definitions](#custom-resource-definitions)
- [Local Development](#local-development)
- [Testing](#testing)
- [Test Environment Setup](#test-environment-setup)
- [Deployment to Local Kubernetes](#deployment-to-local-kubernetes)
- [Monitoring](#monitoring)
- [Makefile Reference](#makefile-reference)

## Overview

The k8s-postgresql-operator is a Kubernetes operator built using the [Kubebuilder](https://book.kubebuilder.io/) framework. It manages PostgreSQL resources declaratively through Custom Resource Definitions (CRDs), allowing you to define databases, users, permissions, and other PostgreSQL objects as Kubernetes resources.

## Features

- **Multi-Instance Support**: Manage multiple PostgreSQL instances from a single operator
- **Vault Integration**: Secure credential storage using HashiCorp Vault with Kubernetes authentication
- **CRD Management**: Declarative management of:
  - PostgreSQL instances (connection validation)
  - Databases (with template support and optional deletion)
  - Users (with password management)
  - Grants (permissions and privileges)
  - Role Groups (role membership management)
  - Schemas (schema creation and ownership)
- **Vault PKI Certificates**: Automatic webhook TLS certificate issuance and renewal via Vault PKI (default when Vault is configured)
- **Webhook Validation**: Kubernetes validating webhooks for all CRD types — empty required fields are denied at admission time
- **Retry Logic**: Exponential backoff with jitter for both PostgreSQL and Vault operations; retries are context-aware and cancel immediately on context cancellation
- **Event Recording**: Kubernetes events emitted on reconciliation success and failure for each CRD type
- **Prometheus Metrics**: Built-in metrics exposed on port 8080 for monitoring operator performance
- **OTLP Telemetry**: OpenTelemetry trace export support
- **Leader Election**: Support for high availability deployments

## Architecture

The operator consists of the following components:

- **Controllers**: Reconcile desired state for each CRD type
- **Webhooks**: Validate CRD specifications before creation/update (port 8443)
- **Vault Client**: Manages credential storage and retrieval using KV v2 and PKI engines
- **PostgreSQL Client**: Handles database operations with retry logic
- **Metrics Collector**: Exposes Prometheus metrics on port 8080

```
┌─────────────────────────────────────────────────────────┐
│                  Kubernetes Cluster                      │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │            k8s-postgresql-operator               │   │
│  │                                                  │   │
│  │  Controllers   Webhooks   Metrics   Health       │   │
│  │  (reconcile)   (:8443)    (:8080)   (:8081)      │   │
│  └────────┬──────────┬────────────────────────────┘   │
│           │          │                                  │
└───────────┼──────────┼──────────────────────────────────┘
            │          │
     ┌──────▼──┐  ┌────▼──────┐
     │PostgreSQL│  │HashiCorp  │
     │Instances │  │Vault      │
     └──────────┘  └───────────┘
```

## Prerequisites

### Runtime Prerequisites

- Kubernetes cluster (v1.24+)
- HashiCorp Vault with:
  - Kubernetes authentication method configured
  - KV v2 secrets engine enabled
  - See: https://developer.hashicorp.com/vault/docs/auth/kubernetes
- PostgreSQL instances accessible from the Kubernetes cluster
- Helm 3.0+ (for Helm chart installation)

### Build Prerequisites

- Go 1.25.7+
- golangci-lint 2.8.0 (installed automatically via `make lint`)
- Docker (for container builds)
- kubectl configured to access your Kubernetes cluster
- Make

### Local Development and Testing Prerequisites

- Go 1.25.7+
- Docker with Docker Compose v2
- kubectl
- Helm 3.0+
- Docker Desktop with Kubernetes enabled (for local K8s deployment)
- HashiCorp Vault CLI (for Vault setup scripts)

## Installation

### Building the Operator

#### Build Binary

Build the operator binary locally:

```bash
make build
```

This will:
- Generate CRD manifests
- Generate DeepCopy code
- Format and vet the code
- Build the binary to `bin/manager`

#### Build Docker Image

Build a Docker image for the operator:

```bash
# Build with default image name (controller:latest)
make docker-build

# Build with custom image name
make docker-build IMG=my-registry/k8s-postgresql-operator:v0.1.0
```

The Dockerfile accepts build arguments for version injection:

```bash
docker build \
  --build-arg VERSION=v0.1.0 \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -t my-registry/k8s-postgresql-operator:v0.1.0 .
```

#### Build for Multiple Platforms

Build Docker image for multiple architectures:

```bash
make docker-buildx IMG=my-registry/k8s-postgresql-operator:v0.1.0
```

This builds for: `linux/arm64`, `linux/amd64`, `linux/s390x`, `linux/ppc64le`

#### Push Docker Image

Push the Docker image to a registry:

```bash
make docker-push IMG=my-registry/k8s-postgresql-operator:v0.1.0
```

### Installing the Operator

#### Install CRDs Only

Install only the Custom Resource Definitions:

```bash
make install
```

This installs CRDs into the cluster specified in `~/.kube/config`.

#### Deploy Full Operator

Deploy the complete operator (CRDs + RBAC + Manager):

```bash
# Set the image name if using a custom registry
export IMG=my-registry/k8s-postgresql-operator:v0.1.0

# Deploy the operator
make deploy
```

#### Generate Installer YAML

Generate a consolidated YAML file with all resources:

```bash
make build-installer IMG=my-registry/k8s-postgresql-operator:v0.1.0
```

This creates `dist/install.yaml` which can be applied directly:

```bash
kubectl apply -f dist/install.yaml
```

#### Uninstall the Operator

Remove the operator from the cluster:

```bash
# Undeploy controller and RBAC
make undeploy

# Uninstall CRDs
make uninstall

# Or uninstall CRDs ignoring not found errors
make uninstall ignore-not-found=true
```

### Using Helm Chart

```bash
# Install the operator
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set vaultPKI.enabled=true \
  --set image.repository=my-registry/k8s-postgresql-operator \
  --set image.tag=v0.1.0
```

Key Helm values:

| Value | Description | Default |
|-------|-------------|---------|
| `image.repository` | Operator image repository | `k8s-postgresql-operator` |
| `image.tag` | Operator image tag | `latest` |
| `vault.addr` | Vault server address | `http://0.0.0.0:8200` |
| `vault.role` | Vault Kubernetes auth role | `role` |
| `vault.mountPoint` | Vault KV v2 mount point | `secret` |
| `vault.secretPath` | Vault secret path prefix | `pdb` |
| `metrics.enabled` | Enable Prometheus metrics | `true` |
| `metrics.port` | Metrics port | `8080` |
| `metrics.serviceMonitor.enabled` | Create ServiceMonitor resource | `false` |
| `vaultPKI.enabled` | Enable Vault PKI for webhook certificates | `true` |
| `vaultPKI.mountPath` | Vault PKI mount path | `pki` |
| `vaultPKI.role` | Vault PKI role name | `webhook-cert` |
| `vaultPKI.ttl` | Certificate TTL | `720h` |
| `vaultPKI.renewalBuffer` | Renewal buffer before expiry | `24h` |

### Using Kustomize Directly

```bash
# Apply CRDs
kubectl apply -k config/crd

# Apply RBAC and Manager
kubectl apply -k config/default
```

## Configuration

The operator can be configured using environment variables or command-line flags. Environment variables take precedence over command-line flags.

### Environment Variables

#### Vault Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `VAULT_ADDR` | Vault server address | `http://0.0.0.0:8200` |
| `VAULT_ROLE` | Vault role name for Kubernetes authentication | `role` |
| `VAULT_MOUNT_POINT` | KV v2 secrets engine mount point | `secret` |
| `VAULT_SECRET_PATH` | Prefix path for storing secrets in Vault | `pdb` |
| `VAULT_AVAILABILITY_RETRIES` | Number of retries for Vault availability check | `3` |
| `VAULT_AVAILABILITY_RETRY_DELAY_SECS` | Delay in seconds between Vault availability retries | `10` |

#### Vault PKI Configuration

When Vault is configured, the operator uses Vault PKI by default to issue webhook TLS certificates. This provides automatic certificate rotation and eliminates the need for self-signed certificates.

| Variable | Description | Default |
|----------|-------------|---------|
| `VAULT_PKI_ENABLED` | Enable Vault PKI for webhook certificates | `true` |
| `VAULT_PKI_MOUNT_PATH` | Vault PKI secrets engine mount path | `pki` |
| `VAULT_PKI_ROLE` | Vault PKI role name for issuing certificates | `webhook-cert` |
| `VAULT_PKI_TTL` | TTL for certificates issued by Vault PKI | `720h` |
| `VAULT_PKI_RENEWAL_BUFFER` | Time before expiry to renew certificates | `24h` |

**Certificate Management Priority:**
1. **External certificates** (`WEBHOOK_CERT_PATH` set) — Use pre-existing certificates
2. **Vault PKI** (default when `VAULT_ADDR` is set and `VAULT_PKI_ENABLED=true`) — Issue from Vault PKI
3. **Self-signed** (fallback) — Auto-generate self-signed certificates

#### Webhook Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `WEBHOOK_CERT_PATH` | Directory containing webhook certificates (empty for auto-generation) | `` |
| `WEBHOOK_CERT_NAME` | Webhook certificate file name | `tls.crt` |
| `WEBHOOK_CERT_KEY` | Webhook key file name | `tls.key` |
| `WEBHOOK_SERVER_PORT` | Webhook server port | `8443` |
| `WEBHOOK_SERVER_ADDR` | Webhook server address | `0.0.0.0` |
| `WEBHOOK_K8S_SERVICE_NAME` | Kubernetes service name for webhook | `k8s-postgresql-operator-controller-service` |
| `K8S_WEBHOOK_NAME_POSTGRESQL` | ValidatingWebhookConfiguration name for PostgreSQL | `k8s-postgresql-operator-validating-webhook-postgresql` |
| `K8S_WEBHOOK_NAME_USER` | ValidatingWebhookConfiguration name for User | `k8s-postgresql-operator-validating-webhook-user` |
| `K8S_WEBHOOK_NAME_DATABASE` | ValidatingWebhookConfiguration name for Database | `k8s-postgresql-operator-validating-webhook-database` |
| `K8S_WEBHOOK_NAME_GRANT` | ValidatingWebhookConfiguration name for Grant | `k8s-postgresql-operator-validating-webhook-grant` |
| `K8S_WEBHOOK_NAME_ROLEGROUP` | ValidatingWebhookConfiguration name for RoleGroup | `k8s-postgresql-operator-validating-webhook-rolegroup` |
| `K8S_WEBHOOK_NAME_SCHEMA` | ValidatingWebhookConfiguration name for Schema | `k8s-postgresql-operator-validating-webhook-schema` |

#### Operator Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ENABLE_LEADER_ELECTION` | Enable leader election for controller manager | `false` |
| `PROBE_ADDR` | Health probe bind address | `:8081` |
| `EXCLUDE_USER_LIST` | Comma-separated list of users to exclude from management | `postgres` |
| `POSTGRESQL_CONNECTION_RETRIES` | Number of retries for PostgreSQL connection test | `3` |
| `POSTGRESQL_CONNECTION_TIMEOUT_SECS` | Timeout in seconds between PostgreSQL connection retries | `10` |
| `METRICS_COLLECTION_INTERVAL_SECS` | Interval in seconds for periodic metrics collection | `30` |

#### Kubernetes Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `K8S_TOKEN_PATH` | Path to Kubernetes service account token | `/var/run/secrets/kubernetes.io/serviceaccount/token` |
| `K8S_NAMESPACE_PATH` | Path to Kubernetes namespace file | `/var/run/secrets/kubernetes.io/serviceaccount/namespace` |

### Command-Line Flags

All environment variables can also be set via command-line flags. Use `--help` to see all available flags:

```bash
./manager --help
```

Common flags:
- `--health-probe-bind-address`: Health probe bind address
- `--leader-elect`: Enable leader election
- `--vault-addr`: Vault server address
- `--vault-role`: Vault role name
- `--vault-pki-enabled`: Enable Vault PKI for webhook certificates
- `--vault-pki-mount-path`: Vault PKI mount path
- `--vault-pki-role`: Vault PKI role name
- `--vault-pki-ttl`: Certificate TTL
- `--vault-pki-renewal-buffer`: Renewal buffer before expiry
- `--webhook-cert-path`: Webhook certificate directory
- `--exclude-user-list`: Users to exclude from management
- `--metrics-collection-interval-secs`: Interval in seconds for periodic metrics collection (default: 30)

## Usage

### Creating a PostgreSQL Instance

```yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Postgresql
metadata:
  name: my-postgresql
spec:
  externalInstance:
    postgresqlID: "my-postgresql-instance"
    address: "postgres.example.com"
    port: 5432
    sslMode: "require"
```

### Creating a Database

```yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Database
metadata:
  name: my-database
spec:
  postgresqlID: "my-postgresql-instance"
  database: "mydb"
  owner: "dbowner"
  schema: "public"
  deleteFromCRD: false
  dbTemplate: ""  # Optional: create from template database
```

### Creating a User

```yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: User
metadata:
  name: my-user
spec:
  postgresqlID: "my-postgresql-instance"
  username: "appuser"
  updatePassword: false  # Set to true to force password regeneration
```

### Creating Grants

```yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Grant
metadata:
  name: my-grant
spec:
  postgresqlID: "my-postgresql-instance"
  database: "mydb"
  role: "appuser"
  grants:
    - type: "table"
      privileges: ["SELECT", "INSERT", "UPDATE"]
      schema: "public"
      table: "users"
```

See `config/samples/` for more examples.

## Custom Resource Definitions

The operator manages the following CRDs:

- **Postgresql**: Defines external PostgreSQL instances
- **Database**: Creates and manages databases
- **User**: Creates and manages database users
- **Grant**: Manages permissions and privileges
- **RoleGroup**: Manages role group membership
- **Schema**: Creates and manages database schemas

## Local Development

### Development Workflow

#### Run Locally

Run the operator locally for development (requires access to Kubernetes cluster):

```bash
make run
```

This will:
- Generate manifests and code
- Format and vet code
- Run the operator locally

#### Generate Code

Generate CRD manifests and DeepCopy code:

```bash
# Generate CRD manifests
make manifests

# Generate DeepCopy code
make generate

# Or both
make manifests generate
```

#### Code Quality

Format and check code:

```bash
# Format code
make fmt

# Run go vet
make vet

# Run linter
make lint

# Run linter with auto-fix
make lint-fix

# Verify linter configuration
make lint-config

# Run vulnerability check
make govulncheck

# Lint Helm chart
make helm-lint

# Validate Helm templates
make helm-template
```

## Testing

The project has four test levels, each with a dedicated build tag and Makefile target. All internal packages maintain 90%+ unit test coverage.

### Unit Tests

Unit tests cover internal packages and do not require external services. They run with race detection and produce a coverage report.

```bash
make test-unit
```

Coverage output: `coverage/unit.out`

### Functional Tests

Functional tests verify individual components (Vault client, PostgreSQL client, configuration) against real services. They require the test environment to be running.

```bash
# Start the test environment first
make setup-test-env
make setup-vault

# Set required environment variables
export TEST_VAULT_ADDR=http://localhost:8200
export TEST_VAULT_TOKEN=myroot
export TEST_POSTGRES_HOST=localhost
export TEST_POSTGRES_USER=postgres
export TEST_POSTGRES_PASSWORD=postgres

# Run functional tests
make test-functional
```

Coverage output: `coverage/functional.out`

Functional test suites:
- `test/functional/vault_test.go` — Vault KV v2 read/write/delete operations (4 tests)
- `test/functional/postgresql_test.go` — PostgreSQL connection, database, user, schema, role group, grant operations (7 tests)
- `test/functional/config_test.go` — Operator configuration defaults and structure (2 tests)

### Integration Tests

Integration tests verify interactions between multiple components (Vault + PostgreSQL together). They require both services to be running.

```bash
# Start the test environment first
make setup-test-env
make setup-vault

# Set required environment variables
export TEST_VAULT_ADDR=http://localhost:8200
export TEST_VAULT_TOKEN=myroot
export TEST_POSTGRES_HOST=localhost
export TEST_POSTGRES_USER=postgres
export TEST_POSTGRES_PASSWORD=postgres

# Run integration tests
make test-integration
```

Coverage output: `coverage/integration.out`

Integration test suites:
- `test/integration/vault_postgresql_test.go` — End-to-end credential flow: store in Vault, retrieve, use to connect to PostgreSQL (3 tests)
- `test/integration/vault_pki_test.go` — Vault PKI certificate issuance for webhook TLS (3 tests)
- `test/integration/postgresql_operations_test.go` — PostgreSQL CRUD operations with verification (3 tests)

### End-to-End Tests

E2E tests use `envtest` to run a real Kubernetes API server and test the operator controllers and webhooks. These tests compile but are not yet run in CI.

```bash
make test-e2e
```

Coverage output: `coverage/e2e.out`

E2E test suites:
- `test/e2e/postgresql_e2e_test.go` — PostgreSQL CRD reconciliation
- `test/e2e/database_e2e_test.go` — Database CRD reconciliation
- `test/e2e/user_e2e_test.go` — User CRD reconciliation
- `test/e2e/webhook_e2e_test.go` — Webhook validation

### Run All Tests

```bash
make test-all
```

This runs unit, functional, integration, and e2e tests in sequence.

### Test Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `TEST_VAULT_ADDR` | Vault address for tests | `http://localhost:8200` |
| `TEST_VAULT_TOKEN` | Vault root token for tests | `myroot` |
| `TEST_POSTGRES_HOST` | PostgreSQL host for tests | `localhost` |
| `TEST_POSTGRES_PORT` | PostgreSQL port for tests | `5432` |
| `TEST_POSTGRES_USER` | PostgreSQL admin user for tests | `postgres` |
| `TEST_POSTGRES_PASSWORD` | PostgreSQL admin password for tests | `postgres` |
| `TEST_POSTGRES_SSLMODE` | PostgreSQL SSL mode for tests | `disable` |

## Test Environment Setup

The test environment uses Docker Compose to run Vault, PostgreSQL, VictoriaMetrics, Grafana Tempo, and Grafana locally. Kubernetes-level components (monitoring agents, operator) are deployed to Docker Desktop's built-in Kubernetes cluster.

### Step 1: Start Docker Compose Services

Start Vault, PostgreSQL, VictoriaMetrics, Tempo, and Grafana:

```bash
make setup-test-env
```

This runs `docker compose -f test/docker-compose/docker-compose.yml up -d` and waits for services to be ready.

Services started:

| Service | Port | Description |
|---------|------|-------------|
| Vault | 8200 | HashiCorp Vault (dev mode, token: `myroot`) |
| PostgreSQL | 5432 | PostgreSQL 16 (user: `postgres`, password: `postgres`) |
| VictoriaMetrics | 8428 | Metrics storage backend |
| Grafana Tempo | 3200, 4317 | Distributed tracing backend (OTLP gRPC on 4317) |
| Grafana | 3000 | Dashboards (anonymous admin access) |

### Step 2: Configure Vault KV Secrets

Configure Vault KV v2 with test PostgreSQL credentials:

```bash
make setup-vault
```

This runs `test/scripts/setup-vault.sh` which:
- Enables the KV v2 secrets engine at the `secret` mount
- Stores admin credentials at `secret/pdb/<instance-id>/admin`
- Stores test user credentials at `secret/pdb/<instance-id>/<username>`
- Creates the `k8s-postgresql-operator-kv` Vault policy

Environment variables accepted by the script:

| Variable | Default | Description |
|----------|---------|-------------|
| `VAULT_ADDR` | `http://127.0.0.1:8200` | Vault address |
| `VAULT_TOKEN` | `myroot` | Vault root token |
| `VAULT_MOUNT_POINT` | `secret` | KV v2 mount point |
| `VAULT_SECRET_PATH` | `pdb` | Secret path prefix |
| `PG_INSTANCE_ID` | `test-pg-001` | PostgreSQL instance ID |
| `ADMIN_USERNAME` | `postgres` | Admin username to store |
| `ADMIN_PASSWORD` | `postgres` | Admin password to store |

### Step 3: Configure Vault PKI (for Webhook TLS)

Configure Vault PKI for issuing webhook TLS certificates:

```bash
make setup-vault-pki
```

This runs `test/scripts/setup-vault-pki.sh` which:
- Enables the PKI secrets engine at the `pki` mount
- Generates an internal root CA
- Creates the `webhook-cert` PKI role for issuing certificates
- Creates the `k8s-postgresql-operator-pki` Vault policy

### Step 4: Configure Vault Kubernetes Auth

Configure Vault Kubernetes authentication for the Docker Desktop cluster:

```bash
make setup-vault-k8s-auth
```

This runs `test/scripts/setup-vault-k8s-auth.sh` which:
- Creates the operator namespace (`k8s-postgresql-operator-system`)
- Creates a `vault-auth` service account with token review permissions
- Configures Vault Kubernetes auth using the cluster's CA and API endpoint
- Creates the `k8s-postgresql-operator` Vault role bound to the operator's service account
- Attaches both `k8s-postgresql-operator-kv` and `k8s-postgresql-operator-pki` policies

> **Note**: This step requires Docker Desktop Kubernetes to be running. The script automatically resolves the Kubernetes API endpoint to `kubernetes.docker.internal` so that Vault (running in Docker Compose) can reach the cluster.

### Teardown

Remove all test environment resources:

```bash
make teardown-test-env
```

This runs `test/scripts/teardown.sh` which:
- Uninstalls Helm releases (operator, vmagent, otel-collector)
- Deletes Kubernetes namespaces
- Stops and removes Docker Compose services and volumes

## Deployment to Local Kubernetes

Deploy the operator to Docker Desktop's Kubernetes cluster for local testing.

### Step 1: Deploy Monitoring Stack

Deploy VictoriaMetrics Agent and OpenTelemetry Collector to Kubernetes:

```bash
make deploy-monitoring
```

This runs `test/scripts/deploy-monitoring.sh` which deploys:
- `vmagent` (from `test/monitoring/vmagent/`) to the `monitoring` namespace — scrapes Prometheus metrics and forwards to VictoriaMetrics
- `otel-collector` (from `test/monitoring/otel-collector/`) to the `monitoring` namespace — receives OTLP traces and forwards to Tempo

### Step 2: Deploy the Operator

Build the Docker image and deploy via Helm:

```bash
make deploy-operator-local
```

This runs `make docker-build` followed by `test/scripts/deploy-operator.sh` which:
- Builds the Docker image as `k8s-postgresql-operator:local`
- Deploys via `helm upgrade --install` to the `k8s-postgresql-operator-system` namespace
- Sets `imagePullPolicy: Never` so Kubernetes uses the locally built image
- Waits for the deployment to be ready and checks pod logs

Environment variables accepted by the deploy script:

| Variable | Default | Description |
|----------|---------|-------------|
| `OPERATOR_NAMESPACE` | `k8s-postgresql-operator-system` | Target namespace |
| `OPERATOR_IMAGE` | `k8s-postgresql-operator` | Docker image name |
| `OPERATOR_TAG` | `local` | Docker image tag |
| `VAULT_ADDR` | `http://host.docker.internal:8200` | Vault address (reachable from K8s pods) |
| `VAULT_ROLE` | `k8s-postgresql-operator` | Vault Kubernetes auth role |
| `SKIP_BUILD` | `false` | Skip Docker image build step |
| `VAULT_PKI_ENABLED` | `true` | Enable Vault PKI |
| `VAULT_PKI_MOUNT_PATH` | `pki` | PKI mount path |
| `VAULT_PKI_ROLE` | `webhook-cert` | PKI role name |
| `VAULT_PKI_TTL` | `720h` | Certificate TTL |
| `VAULT_PKI_RENEWAL_BUFFER` | `24h` | Renewal buffer |

### Step 3: Publish Grafana Dashboards

Upload the operator dashboards to Grafana:

```bash
make publish-dashboards
```

This runs `test/scripts/publish-dashboards.sh` which uploads the three dashboards from `monitoring/grafana/dashboards/` to the Grafana API.

### Verify the Deployment

```bash
# Check operator pod status
kubectl get pods -n k8s-postgresql-operator-system

# View operator logs
kubectl logs -n k8s-postgresql-operator-system -l app.kubernetes.io/name=k8s-postgresql-operator -f

# Check monitoring pods
kubectl get pods -n monitoring
```

## Monitoring

### Prometheus Metrics

The operator exposes Prometheus metrics at `:8080/metrics`. The following custom metrics are available:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `postgresql_operator_postgresql_count` | Gauge | — | Total number of registered PostgreSQL instances |
| `postgresql_operator_object_count` | Gauge | `kind`, `postgresql_id` | Count of managed objects per kind and PostgreSQL instance |
| `postgresql_operator_object_info` | Gauge | `kind`, `postgresql_id`, `name`, `namespace`, `databasename`, `username` | Detailed information about managed objects (value is always 1) |
| `postgresql_operator_reconcile_duration_seconds` | Histogram | `controller` | Duration of each reconciliation loop per controller |
| `postgresql_operator_reconcile_errors_total` | Counter | `controller` | Total number of reconciliation errors per controller |
| `postgresql_operator_reconcile_total` | Counter | `controller` | Total number of reconciliations per controller |

Metrics are collected periodically (every `METRICS_COLLECTION_INTERVAL_SECS` seconds, default 30) and also updated inline during each reconciliation.

Standard controller-runtime metrics (work queue depth, REST client latency, etc.) are also exposed.

### Grafana Dashboards

Three pre-built Grafana dashboards are included in `monitoring/grafana/dashboards/`:

| Dashboard | File | Description |
|-----------|------|-------------|
| Operator Overview | `operator-overview.json` | Reconcile rates, error rates, queue depth, memory/CPU usage |
| PostgreSQL Instances | `postgresql-instances.json` | Per-instance object counts and status |
| Webhooks | `webhooks.json` | Webhook request rates, latency, and error rates |

Access Grafana at http://localhost:3000 after starting the test environment. Anonymous admin access is enabled in the test configuration.

### Metrics Collection Architecture

In the local test environment, metrics flow as follows:

```
Operator (:8080/metrics)
        |
        v
   vmagent (K8s)          -- scrapes Prometheus metrics
        |
        v
VictoriaMetrics (:8428)   -- stores metrics (Docker Compose)
        |
        v
   Grafana (:3000)         -- visualizes metrics (Docker Compose)

Operator (OTLP traces)
        |
        v
otel-collector (K8s)      -- receives and forwards traces
        |
        v
   Tempo (:4317)           -- stores traces (Docker Compose)
        |
        v
   Grafana (:3000)         -- visualizes traces (Docker Compose)
```

## Makefile Reference

Run `make help` to see all available targets with descriptions.

### Development Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the operator binary to `bin/manager` |
| `make run` | Run the operator locally against the current kubeconfig |
| `make manifests` | Generate CRD manifests and webhook configurations |
| `make generate` | Generate DeepCopy and other code |
| `make fmt` | Run `go fmt` |
| `make vet` | Run `go vet` |
| `make lint` | Run golangci-lint |
| `make lint-fix` | Run golangci-lint with auto-fix |
| `make lint-config` | Verify golangci-lint configuration |
| `make govulncheck` | Run vulnerability scanner |
| `make helm-lint` | Lint the Helm chart |
| `make helm-template` | Validate Helm templates render correctly |

### Build Targets

| Target | Description |
|--------|-------------|
| `make docker-build` | Build Docker image (`IMG` variable sets the tag) |
| `make docker-push` | Push Docker image to registry |
| `make docker-buildx` | Build and push multi-platform Docker image |
| `make build-installer` | Generate `dist/install.yaml` with all resources |

### Deployment Targets

| Target | Description |
|--------|-------------|
| `make install` | Install CRDs into the cluster |
| `make uninstall` | Remove CRDs from the cluster |
| `make deploy` | Deploy operator using Kustomize |
| `make undeploy` | Remove operator deployment |

### Testing Targets

| Target | Description |
|--------|-------------|
| `make test-unit` | Run unit tests with race detection and coverage |
| `make test-functional` | Run functional tests (requires test environment) |
| `make test-integration` | Run integration tests (requires test environment) |
| `make test-e2e` | Run end-to-end tests using envtest |
| `make test-all` | Run all test suites in sequence |

### Test Environment Targets

| Target | Description |
|--------|-------------|
| `make setup-test-env` | Start Docker Compose services (Vault, PostgreSQL, VictoriaMetrics, Tempo, Grafana) |
| `make setup-vault` | Configure Vault KV v2 with test credentials |
| `make setup-vault-pki` | Configure Vault PKI for webhook TLS certificates |
| `make setup-vault-k8s-auth` | Configure Vault Kubernetes auth for Docker Desktop |
| `make deploy-monitoring` | Deploy vmagent and otel-collector to local Kubernetes |
| `make deploy-operator-local` | Build and deploy operator to local Kubernetes via Helm |
| `make publish-dashboards` | Upload Grafana dashboards to the local Grafana instance |
| `make teardown-test-env` | Remove all test environment resources |

## License

Licensed under the Apache License, Version 2.0.
