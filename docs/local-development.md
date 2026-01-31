# Local Development Guide

This guide covers setting up a complete local development environment for the k8s-postgresql-operator.

## Prerequisites

Before starting local development, ensure you have:

- **Go 1.25.6+** installed
- **Docker** or Podman for container builds
- **kubectl** configured to access your Kubernetes cluster
- **Make** installed
- **kind** for local Kubernetes clusters (optional)
- **Helm 3.0+** for chart testing
- **Git** for version control

## Development Environment Setup

### 1. Clone the Repository

```bash
git clone <repository-url>
cd k8s-postgresql-operator

# Install Go dependencies
go mod download
```

### 2. Start Local Services

The project includes a Docker Compose setup for local development with PostgreSQL and Vault:

```bash
# Start PostgreSQL and Vault containers
make test-env-up

# Verify services are running
docker-compose -f test/docker-compose/docker-compose.yml ps
```

This starts:
- **PostgreSQL 16** on `localhost:5432`
  - Database: `postgres`
  - User: `postgres`
  - Password: `postgres`
- **Vault 1.13.3** on `localhost:8200`
  - Root token: `myroot`
  - Dev mode enabled

### 3. Configure Vault for Local Development

#### Basic Vault Setup

```bash
# Setup Vault with KV secrets engine and basic policies
make test-env-setup-vault
```

#### Vault with Kubernetes Integration

For testing with local Kubernetes:

```bash
# Setup Vault for local Kubernetes development
make test-env-setup-vault-local-k8s
```

This configures:
- KV v2 secrets engine at `secret/`
- Kubernetes authentication method
- Required policies for the operator
- Test credentials

#### Vault PKI Setup (Optional)

For testing webhook certificate management:

```bash
# Setup Vault PKI for webhook certificates
make test-env-setup-vault-pki
```

### 4. Environment Variables

Set these environment variables for local development:

```bash
# Vault configuration
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=myroot
export VAULT_ROLE=k8s-postgresql-operator

# PostgreSQL configuration
export TEST_POSTGRES_HOST=localhost
export TEST_POSTGRES_PORT=5432
export TEST_POSTGRES_USER=postgres
export TEST_POSTGRES_PASSWORD=postgres
export TEST_POSTGRES_SSLMODE=disable

# Development settings
export LOG_LEVEL=debug
export ENABLE_LEADER_ELECTION=false
export POSTGRESQL_RATE_LIMIT_PER_SECOND=50
export VAULT_RATE_LIMIT_PER_SECOND=50
export RETRY_INITIAL_INTERVAL_MS=100
```

## Running the Operator

### 1. Run Locally Against Kubernetes Cluster

```bash
# Run the operator locally (requires kubectl access)
make run
```

This will:
- Generate CRD manifests and DeepCopy code
- Format and vet the code
- Run the operator binary locally

### 2. Deploy to Local Kubernetes

#### Using kind

```bash
# Create a kind cluster
kind create cluster --name k8s-postgresql-operator-dev

# Build and load the operator image
make docker-build IMG=k8s-postgresql-operator:dev
kind load docker-image k8s-postgresql-operator:dev --name k8s-postgresql-operator-dev

# Deploy the operator
make deploy IMG=k8s-postgresql-operator:dev
```

#### Using Helm

```bash
# Install via Helm chart
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://host.docker.internal:8200 \
  --set vault.role=k8s-postgresql-operator \
  --set image.repository=k8s-postgresql-operator \
  --set image.tag=dev \
  --set image.pullPolicy=Never \
  --set operator.logLevel=debug
```

## Development Workflow

### 1. Code Generation

Generate required code and manifests:

```bash
# Generate CRD manifests
make manifests

# Generate DeepCopy code
make generate

# Generate both
make manifests generate
```

### 2. Code Quality

Maintain code quality with these commands:

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

### 3. Building

Build the operator binary and container images:

```bash
# Build binary
make build

# Build Docker image
make docker-build

# Build for multiple platforms
make docker-buildx
```

### 4. Testing

Run tests at different levels:

```bash
# Unit tests
make test

# Unit tests with coverage
make test-coverage

# Integration tests (requires test environment)
make test-integration

# Vault PKI integration tests
make test-vault-pki

# End-to-end tests
make test-e2e-local
```

## Testing Your Changes

### 1. Unit Testing

```bash
# Run all unit tests
make test

# Run specific package tests
go test ./internal/controller/...

# Run with verbose output
go test -v ./internal/...

# Run specific test
go test -v -run TestPostgreSQLController_Reconcile ./internal/controller/
```

### 2. Integration Testing

```bash
# Ensure test environment is running
make test-env-up

# Run integration tests
make test-integration

# Run specific integration test
go test -tags=integration -v ./test/integration/... -run TestVaultIntegration
```

### 3. End-to-End Testing

```bash
# Run complete E2E test suite
make test-e2e-local

# Run with verbose output
make test-e2e-local-verbose

# Cleanup after interrupted test
make test-e2e-local-cleanup
```

### 4. Manual Testing

Create test resources to verify functionality:

```bash
# Apply test PostgreSQL instance
kubectl apply -f - <<EOF
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Postgresql
metadata:
  name: test-postgresql
spec:
  externalInstance:
    postgresqlID: "test-instance"
    address: "localhost"
    port: 5432
    sslMode: "disable"
EOF

# Check resource status
kubectl describe postgresql test-postgresql

# Check operator logs
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator -f
```

## Debugging

### 1. Operator Debugging

```bash
# Run with debug logging
export LOG_LEVEL=debug
make run

# Use delve debugger
dlv exec ./bin/manager
```

### 2. Test Environment Debugging

```bash
# Check service status
docker-compose -f test/docker-compose/docker-compose.yml ps

# View service logs
docker-compose -f test/docker-compose/docker-compose.yml logs vault
docker-compose -f test/docker-compose/docker-compose.yml logs postgres

# Test Vault connectivity
curl -H "X-Vault-Token: myroot" http://localhost:8200/v1/sys/health

# Test PostgreSQL connectivity
psql -h localhost -U postgres -d postgres -c "SELECT version();"
```

### 3. Kubernetes Debugging

```bash
# Check operator pod status
kubectl get pods -l app.kubernetes.io/name=k8s-postgresql-operator

# View operator logs
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator

# Check CRD status
kubectl get crd | grep postgresql-operator

# Describe specific resources
kubectl describe postgresql,database,user,grant
```

## Configuration Scripts

The project includes several scripts to automate common development tasks:

### Vault Configuration Scripts

- `test/scripts/setup-vault.sh` - Basic Vault setup
- `test/scripts/setup-vault-k8s.sh` - Kubernetes auth setup
- `test/scripts/setup-vault-pki.sh` - PKI secrets engine setup
- `test/scripts/setup-vault-local-k8s.sh` - Complete local K8s integration

### Test Scripts

- `test/scripts/e2e-local-test.sh` - Complete E2E test automation
- `test/scripts/cleanup-test-env.sh` - Environment cleanup

## Common Development Tasks

### 1. Adding a New CRD

```bash
# Generate new CRD using kubebuilder
kubebuilder create api --group postgresql-operator --version v1alpha1 --kind NewResource

# Generate manifests
make manifests

# Generate code
make generate

# Update RBAC permissions in config/rbac/
```

### 2. Updating Dependencies

```bash
# Update Go dependencies
go get -u ./...
go mod tidy

# Update Helm chart dependencies
helm dependency update ./charts/k8s-postgresql-operator
```

### 3. Testing Configuration Changes

```bash
# Test with different configuration
export POSTGRESQL_RATE_LIMIT_PER_SECOND=1
export VAULT_PKI_ENABLED=true
make run

# Observe behavior changes in logs
```

## Cleanup

### Stop Local Services

```bash
# Stop Docker Compose services
make test-env-down

# Remove volumes (complete cleanup)
docker-compose -f test/docker-compose/docker-compose.yml down -v
```

### Clean Development Environment

```bash
# Clean build artifacts
make clean

# Remove kind cluster
kind delete cluster --name k8s-postgresql-operator-dev

# Clean Docker images
docker rmi k8s-postgresql-operator:dev
```

## IDE Setup

### VS Code

Recommended extensions:
- Go extension
- Kubernetes extension
- YAML extension
- Docker extension

### GoLand/IntelliJ

Configure:
- Go SDK path
- GOPATH and GOROOT
- Code style settings
- Run configurations for tests

## Troubleshooting

### Common Issues

#### Port Conflicts

```bash
# Check for port conflicts
lsof -i :8200  # Vault
lsof -i :5432  # PostgreSQL
lsof -i :8443  # Webhook server

# Kill conflicting processes
kill -9 <PID>
```

#### Docker Issues

```bash
# Clean Docker resources
docker system prune

# Restart Docker Compose
make test-env-down
make test-env-up
```

#### Go Module Issues

```bash
# Clean module cache
go clean -modcache

# Re-download dependencies
go mod download
```

#### Kubernetes Issues

```bash
# Reset kubectl context
kubectl config current-context

# Check cluster connectivity
kubectl cluster-info

# Restart kind cluster
kind delete cluster --name k8s-postgresql-operator-dev
kind create cluster --name k8s-postgresql-operator-dev
```

For more troubleshooting information, see:
- [Testing Guide](testing.md)
- [Vault Integration Guide](vault-integration.md)
- [Main README](../README.md)