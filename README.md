# k8s-postgresql-operator

A Kubernetes operator for managing PostgreSQL databases, users, grants, role groups, and schemas across multiple PostgreSQL instances. The operator integrates with HashiCorp Vault for secure credential storage and provides comprehensive validation through Kubernetes webhooks.

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
- **Webhook Validation**: Kubernetes validating webhooks for all CRD types
- **Vault PKI Integration**: Vault PKI for webhook certificates (optional)
- **Advanced Retry Logic**: Exponential backoff with configurable parameters for PostgreSQL and Vault operations
- **Rate Limiting**: Configurable rate limiting for PostgreSQL and Vault API calls to prevent overwhelming external services
- **Event Recording**: Comprehensive Kubernetes event recording for all operations and failures
- **Prometheus Metrics**: Built-in metrics for monitoring operator performance, including rate limiting and retry metrics
- **Leader Election**: Support for high availability deployments
- **Configurable Logging**: Structured JSON logging with configurable log levels (debug, info, warn, error)
- **Certificate Management**: Multiple certificate providers (self-signed, Vault PKI, or provided certificates)

## Architecture

The operator consists of:

- **Controllers**: Reconcile desired state for each CRD type
- **Webhooks**: Validate CRD specifications before creation/update
- **Vault Client**: Manages credential storage and retrieval
- **PostgreSQL Client**: Handles database operations with retry logic
- **Metrics Collector**: Exposes Prometheus metrics

## Prerequisites

- Kubernetes cluster (v1.24+)
- HashiCorp Vault with:
  - Kubernetes authentication method configured
  - Vault integrated with Kubernetes using the Vault client's JWT as the reviewer JWT
  - KV v2 secrets engine enabled
  - See: https://developer.hashicorp.com/vault/docs/auth/kubernetes
- PostgreSQL instances accessible from the Kubernetes cluster
- Helm 3.0+ (for Helm chart installation)

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.24+)
- HashiCorp Vault with:
  - Kubernetes authentication method configured
  - Vault integrated with Kubernetes using the Vault client's JWT as the reviewer JWT
  - KV v2 secrets engine enabled
  - See: https://developer.hashicorp.com/vault/docs/auth/kubernetes
- PostgreSQL instances accessible from the Kubernetes cluster
- Helm 3.0+ (for Helm chart installation)

### Installation with Helm

```bash
# Basic installation
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set image.repository=my-registry/k8s-postgresql-operator \
  --set image.tag=v0.1.0

# Installation with Vault PKI enabled
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set vaultPKI.enabled=true \
  --set vaultPKI.mountPath=pki \
  --set vaultPKI.role=webhook-cert
```

### Sample Resources Usage

Create PostgreSQL resources in this order:

1. **PostgreSQL Instance**:
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

2. **Database**:
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
```

3. **User**:
```yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: User
metadata:
  name: my-user
spec:
  postgresqlID: "my-postgresql-instance"
  username: "appuser"
  updatePassword: false
```

4. **Grant Permissions**:
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

For detailed getting started guide, see [docs/getting-started.md](docs/getting-started.md).

For Vault integration setup, see [docs/vault-integration.md](docs/vault-integration.md).

## Installation

### Prerequisites for Building

Before building the operator, ensure you have:

- Go 1.25.6+ installed
- Docker or Podman (for container builds)
- kubectl configured to access your Kubernetes cluster
- Make installed

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
  --set image.repository=my-registry/k8s-postgresql-operator \
  --set image.tag=v0.1.0

# Install with Vault PKI enabled
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set vaultPKI.enabled=true \
  --set vaultPKI.mountPath=pki \
  --set vaultPKI.role=webhook-cert
```

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
| `VAULT_AUTH_TIMEOUT_SECS` | Timeout in seconds for Vault authentication | `30` |

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

#### Vault PKI Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `VAULT_PKI_ENABLED` | Enable Vault PKI for webhook certificates | `false` |
| `VAULT_PKI_MOUNT_PATH` | Vault PKI secrets engine mount path | `pki` |
| `VAULT_PKI_ROLE` | Vault PKI role name for certificate issuance | `webhook-cert` |
| `VAULT_PKI_TTL` | TTL for Vault PKI issued certificates | `720h` |
| `VAULT_PKI_RENEWAL_BUFFER` | Time before expiry to trigger renewal | `24h` |

#### Operator Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ENABLE_LEADER_ELECTION` | Enable leader election for controller manager | `false` |
| `PROBE_ADDR` | Health probe bind address | `:8081` |
| `EXCLUDE_USER_LIST` | Comma-separated list of users to exclude from management | `postgres` |
| `POSTGRESQL_CONNECTION_RETRIES` | Number of retries for PostgreSQL connection test | `3` |
| `POSTGRESQL_CONNECTION_TIMEOUT_SECS` | Timeout in seconds between PostgreSQL connection retries | `10` |
| `LOG_LEVEL` | Log level: debug, info, warn, error | `info` |

#### Kubernetes Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `K8S_TOKEN_PATH` | Path to Kubernetes service account token | `/var/run/secrets/kubernetes.io/serviceaccount/token` |
| `K8S_NAMESPACE_PATH` | Path to Kubernetes namespace file | `/var/run/secrets/kubernetes.io/serviceaccount/namespace` |

#### Rate Limiting Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `POSTGRESQL_RATE_LIMIT_PER_SECOND` | Rate limit for PostgreSQL connections per second | `10` |
| `POSTGRESQL_RATE_LIMIT_BURST` | Burst limit for PostgreSQL connections | `20` |
| `VAULT_RATE_LIMIT_PER_SECOND` | Rate limit for Vault requests per second | `10` |
| `VAULT_RATE_LIMIT_BURST` | Burst limit for Vault requests | `20` |

#### Retry Configuration (Exponential Backoff)

| Variable | Description | Default |
|----------|-------------|---------|
| `RETRY_INITIAL_INTERVAL_MS` | Initial interval in milliseconds for exponential backoff | `500` |
| `RETRY_MAX_INTERVAL_SECS` | Maximum interval in seconds for exponential backoff | `30` |
| `RETRY_MAX_ELAPSED_TIME_SECS` | Maximum elapsed time in seconds for all retries | `300` |
| `RETRY_MULTIPLIER` | Multiplier for exponential backoff interval growth | `2.0` |
| `RETRY_RANDOMIZATION_FACTOR` | Randomization factor (jitter) for exponential backoff | `0.5` |

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
- `--webhook-cert-path`: Webhook certificate directory
- `--vault-pki-enabled`: Enable Vault PKI for webhook certificates
- `--vault-pki-mount-path`: Vault PKI secrets engine mount path
- `--vault-pki-role`: Vault PKI role name for certificate issuance
- `--vault-pki-ttl`: TTL for Vault PKI issued certificates
- `--vault-pki-renewal-buffer`: Time before expiry to trigger renewal
- `--exclude-user-list`: Users to exclude from management
- `--log-level`: Log level (debug, info, warn, error)
- `--postgresql-rate-limit-per-second`: Rate limit for PostgreSQL connections per second
- `--postgresql-rate-limit-burst`: Burst limit for PostgreSQL connections
- `--vault-rate-limit-per-second`: Rate limit for Vault requests per second
- `--vault-rate-limit-burst`: Burst limit for Vault requests
- `--retry-initial-interval-ms`: Initial interval for exponential backoff
- `--retry-max-interval-secs`: Maximum interval for exponential backoff
- `--retry-max-elapsed-time-secs`: Maximum elapsed time for all retries
- `--retry-multiplier`: Multiplier for exponential backoff interval growth
- `--retry-randomization-factor`: Randomization factor for exponential backoff
- `--vault-auth-timeout-secs`: Timeout for Vault authentication

## Vault PKI Setup

The operator supports using HashiCorp Vault PKI for webhook certificate management as an alternative to self-signed certificates.

### Certificate Provider Selection

The operator selects certificate providers in the following order:

1. **Provided Certificates**: If `WEBHOOK_CERT_PATH` is set, use certificates from the specified directory
2. **Vault PKI**: If `VAULT_PKI_ENABLED` is true, use Vault PKI to issue and manage certificates
3. **Self-Signed**: Default behavior - generate self-signed certificates

### Prerequisites

- Vault server with PKI secrets engine enabled
- Kubernetes authentication configured in Vault
- Vault role with appropriate permissions for certificate issuance

### Setup Steps

#### 1. Enable PKI Secrets Engine

```bash
vault secrets enable pki
vault secrets tune -max-lease-ttl=8760h pki
```

#### 2. Generate Root CA

```bash
vault write pki/root/generate/internal \
    common_name="K8s PostgreSQL Operator CA" \
    ttl=8760h
```

#### 3. Create PKI Role

```bash
vault write pki/roles/webhook-cert \
    allowed_domains="k8s-postgresql-operator-controller-service,k8s-postgresql-operator-controller-service.default,k8s-postgresql-operator-controller-service.default.svc,k8s-postgresql-operator-controller-service.default.svc.cluster.local" \
    allow_subdomains=false \
    max_ttl=720h \
    generate_lease=true
```

#### 4. Create Policy

```bash
vault policy write k8s-postgresql-operator-pki - <<EOF
path "pki/issue/webhook-cert" {
  capabilities = ["create", "update"]
}
EOF
```

#### 5. Update Kubernetes Auth Role

```bash
vault write auth/kubernetes/role/my-vault-role \
    bound_service_account_names=k8s-postgresql-operator-controller \
    bound_service_account_namespaces=default \
    policies=k8s-postgresql-operator-pki \
    ttl=1h
```

### Using the Setup Script

For convenience, you can use the provided setup script:

```bash
./test/scripts/setup-vault-pki.sh
```

This script automates the PKI setup process and configures Vault with the necessary policies and roles.

### Troubleshooting

- **Certificate Renewal**: Certificates are automatically renewed when they approach expiry (based on `VAULT_PKI_RENEWAL_BUFFER`)
- **Vault Connectivity**: Ensure the operator can reach Vault and has valid authentication
- **PKI Role Permissions**: Verify the Vault role has access to the PKI secrets engine and the specified role

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

## Testing

The operator includes comprehensive testing at multiple levels:

### Unit Tests
```bash
make test
```

### Integration Tests
Requires PostgreSQL and Vault running:
```bash
# Start test environment
make test-env-up

# Run integration tests
make test-integration
```

### E2E Tests
For comprehensive end-to-end testing with a local Kubernetes cluster:
```bash
# Run complete e2e test suite (includes setup, testing, and cleanup)
make test-e2e-local

# Run with verbose output
make test-e2e-local-verbose

# Cleanup only
make test-e2e-local-cleanup
```

The E2E test script (`test/scripts/e2e-local-test.sh`) provides:
- Automatic kind cluster setup
- Vault and PostgreSQL deployment
- Operator installation via Helm
- Complete CRD lifecycle testing
- Automatic cleanup

### Test Environment Setup

The project includes docker-compose configuration for local testing:

```bash
# Start PostgreSQL and Vault
make test-env-up

# Setup Vault for Kubernetes integration
make test-env-setup-vault-local-k8s

# Start full environment with K8s integration
make test-env-up-k8s

# Stop test environment
make test-env-down
```

For more detailed testing information, see [docs/testing.md](docs/testing.md).

## Configuration

For comprehensive configuration documentation, see [docs/configuration.md](docs/configuration.md).

## Architecture

For detailed architecture documentation, see [docs/architecture.md](docs/architecture.md).

## Custom Resource Definitions

The operator manages the following CRDs:

- **Postgresql**: Defines external PostgreSQL instances
- **Database**: Creates and manages databases
- **User**: Creates and manages database users
- **Grant**: Manages permissions and privileges
- **RoleGroup**: Manages role group membership
- **Schema**: Creates and manages database schemas

## Monitoring

The operator exposes Prometheus metrics at the `/metrics` endpoint:

- `postgresql_operator_object_count`: Count of objects per kind and postgresqlID
- `postgresql_operator_object_info`: Information about objects (name, namespace, databasename, username) per kind and postgresqlID

## Development Setup

### Prerequisites for Development

- Go 1.25.6+ installed
- Docker or Podman (for container builds)
- kubectl configured to access your Kubernetes cluster
- Make installed
- HashiCorp Vault (for local testing)
- PostgreSQL instance (for local testing)

### Local Development Environment

#### 1. Clone and Setup

```bash
git clone <repository-url>
cd k8s-postgresql-operator

# Install dependencies
go mod download
```

#### 2. Start Local Test Environment

```bash
# Start PostgreSQL and Vault containers
make test-env-up

# Setup Vault for local development
make test-env-setup-vault-local-k8s
```

#### 3. Run Operator Locally

```bash
# Run the operator locally (requires access to Kubernetes cluster)
make run
```

This will:
- Generate manifests and code
- Format and vet code
- Run the operator locally

### Development Workflow

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
```

#### View Available Make Targets

See all available Make targets with descriptions:

```bash
make help
```

For detailed local development setup, see [docs/local-development.md](docs/local-development.md).

### Common Make Targets

| Target | Description |
|--------|-------------|
| `make build` | Build the operator binary |
| `make run` | Run the operator locally |
| `make test` | Run unit tests |
| `make lint` | Run linter |
| `make docker-build` | Build Docker image |
| `make docker-push` | Push Docker image |
| `make install` | Install CRDs only |
| `make deploy` | Deploy full operator |
| `make undeploy` | Remove operator deployment |
| `make uninstall` | Remove CRDs |
| `make manifests` | Generate CRD manifests |
| `make generate` | Generate DeepCopy code |
| `make build-installer` | Generate consolidated installer YAML |
| `make test-env-setup-vault-pki` | Setup Vault PKI for testing |
| `make test-vault-pki` | Run Vault PKI integration tests |
| `make test-env-up-full` | Start full test environment with PKI |
| `make test-env-setup-vault-local-k8s` | Setup Docker Vault for local K8s |
| `make test-env-up-k8s` | Start test environment with K8s integration |
| `make test-e2e-local` | Run comprehensive e2e test |
| `make helm-lint` | Lint Helm chart |
| `make helm-template` | Generate Helm templates |
| `make helm-package` | Package Helm chart |

## Contributing

We welcome contributions to the k8s-postgresql-operator! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on:

- Code style and standards
- Testing requirements (90%+ coverage)
- Pull request process
- Commit message format

## License

Licensed under the Apache License, Version 2.0.
