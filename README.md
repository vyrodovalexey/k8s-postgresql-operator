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
- **Retry Logic**: Configurable retry mechanisms for PostgreSQL and Vault operations
- **Prometheus Metrics**: Built-in metrics for monitoring operator performance
- **Leader Election**: Support for high availability deployments

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

## Installation

### Prerequisites for Building

Before building the operator, ensure you have:

- Go 1.21+ installed
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
- `--webhook-cert-path`: Webhook certificate directory
- `--exclude-user-list`: Users to exclude from management

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

## Monitoring

The operator exposes Prometheus metrics at the `/metrics` endpoint:

- `postgresql_operator_object_count`: Count of objects per kind and postgresqlID
- `postgresql_operator_object_info`: Information about objects (name, namespace, databasename, username) per kind and postgresqlID

## Development

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
```

#### View Available Make Targets

See all available Make targets with descriptions:

```bash
make help
```

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

## License

Licensed under the Apache License, Version 2.0.
