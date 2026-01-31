# Testing Guide

This guide covers all testing approaches for the k8s-postgresql-operator, from unit tests to comprehensive end-to-end testing.

## Testing Overview

The operator includes multiple testing levels:

- **Unit Tests**: Test individual components and functions
- **Functional Tests**: Test business logic with mocked dependencies
- **Integration Tests**: Test with real PostgreSQL and Vault
- **E2E Tests**: Test complete workflows in Kubernetes

## Prerequisites

### For Local Testing

- Go 1.25.6+
- Docker and Docker Compose
- kubectl and kind (for E2E tests)
- Make
- Helm 3.0+ (for E2E tests)

### For Integration Tests

- PostgreSQL instance (provided via docker-compose)
- HashiCorp Vault instance (provided via docker-compose)

## Unit Tests

Unit tests focus on individual components without external dependencies.

### Running Unit Tests

```bash
# Run all unit tests
make test

# Run with coverage
make test-coverage

# Run specific package
go test ./internal/controller/...

# Run with verbose output
go test -v ./internal/...
```

### Test Structure

Unit tests are located alongside source code:

```
internal/
├── controller/
│   ├── postgresql_controller.go
│   ├── postgresql_controller_test.go
│   ├── user_controller.go
│   └── user_controller_test.go
├── vault/
│   ├── client.go
│   ├── client_test.go
│   ├── pki.go
│   └── pki_test.go
└── postgresql/
    ├── client.go
    ├── client_test.go
    ├── operations.go
    └── operations_test.go
```

### Writing Unit Tests

Example unit test structure:

```go
func TestPostgreSQLController_Reconcile(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(*testing.T) *PostgreSQLController
        request ctrl.Request
        want    ctrl.Result
        wantErr bool
    }{
        {
            name: "successful reconciliation",
            setup: func(t *testing.T) *PostgreSQLController {
                // Setup test controller with mocks
            },
            request: ctrl.Request{
                NamespacedName: types.NamespacedName{
                    Name:      "test-postgresql",
                    Namespace: "default",
                },
            },
            want:    ctrl.Result{},
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            controller := tt.setup(t)
            got, err := controller.Reconcile(context.TODO(), tt.request)
            
            if (err != nil) != tt.wantErr {
                t.Errorf("Reconcile() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Reconcile() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Functional Tests

Functional tests verify business logic with controlled dependencies.

### Running Functional Tests

```bash
# Run functional tests
make test-functional

# Run specific functional test
go test -tags=functional ./test/functional/...
```

### Functional Test Structure

```
test/
└── functional/
    ├── controller_test.go
    ├── vault_test.go
    └── postgresql_test.go
```

## Integration Tests

Integration tests use real PostgreSQL and Vault instances to test complete workflows.

### Setup Test Environment

```bash
# Start PostgreSQL and Vault containers
make test-env-up

# Setup Vault configuration
make test-env-setup-vault

# Verify services are running
docker-compose -f test/docker-compose/docker-compose.yml ps
```

### Running Integration Tests

```bash
# Run all integration tests
make test-integration

# Run specific integration test
go test -tags=integration -v ./test/integration/... -run TestVaultIntegration

# Run with timeout
go test -tags=integration -timeout 10m ./test/integration/...
```

### Integration Test Environment

The docker-compose setup provides:

- **PostgreSQL 16**: Available at `localhost:5432`
  - Database: `postgres`
  - User: `postgres`
  - Password: `postgres`

- **Vault 1.13.3**: Available at `localhost:8200`
  - Root token: `myroot`
  - Dev mode enabled

### Environment Variables for Testing

Set these environment variables for local testing:

```bash
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=myroot
export TEST_POSTGRES_HOST=localhost
export TEST_POSTGRES_PORT=5432
export TEST_POSTGRES_USER=postgres
export TEST_POSTGRES_PASSWORD=postgres
export TEST_POSTGRES_SSLMODE=disable

# Optional: Configure test-specific settings
export LOG_LEVEL=debug
export POSTGRESQL_RATE_LIMIT_PER_SECOND=50
export VAULT_RATE_LIMIT_PER_SECOND=50
export RETRY_INITIAL_INTERVAL_MS=100
export RETRY_MAX_INTERVAL_SECS=5
```

### Quick Start: Complete Local Test Environment

```bash
# 1. Start Docker Compose environment
cd test/docker-compose && docker-compose up -d && cd ../..

# 2. Wait for services to be ready
sleep 5

# 3. Setup Vault with KV secrets, PKI, and K8s auth
./test/scripts/setup-vault-local-k8s.sh

# 4. Verify setup
curl -s -H "X-Vault-Token: myroot" http://localhost:8200/v1/sys/health | jq .

# 5. Run integration tests
make test-integration
```

### Test Credentials Stored in Vault

After running the setup scripts, the following test credentials are available:

| Path | Description |
|------|-------------|
| `secret/pdb/test-pg-001/admin` | Admin credentials (admin_username, admin_password) |
| `secret/pdb/test-pg-001/testuser` | Test user credentials (password) |
| `secret/pdb/postgresql/test-uuid-001/appuser` | App user credentials (password) |
| `secret/pdb/postgresql/test-uuid-001/readonly` | Readonly user credentials (password) |

### Vault PKI Integration Tests

Test Vault PKI certificate management:

```bash
# Setup Vault PKI
make test-env-setup-vault-pki

# Run PKI-specific tests
make test-vault-pki

# Or run specific PKI test
go test -tags=integration -v ./test/integration/... -run TestIntegration_VaultPKI
```

### Cleanup Test Environment

```bash
# Stop and remove containers
make test-env-down

# Remove volumes (complete cleanup)
docker-compose -f test/docker-compose/docker-compose.yml down -v
```

## End-to-End (E2E) Tests

E2E tests validate complete operator functionality in a real Kubernetes environment.

### Local E2E Testing

The project includes a comprehensive E2E test script that:

1. Creates a kind Kubernetes cluster
2. Deploys PostgreSQL and Vault
3. Installs the operator via Helm
4. Tests complete CRD lifecycle
5. Cleans up resources

### Running E2E Tests

```bash
# Run complete E2E test suite
make test-e2e-local

# Run with verbose output
make test-e2e-local-verbose

# Cleanup only (if test was interrupted)
make test-e2e-local-cleanup
```

### E2E Test Script Features

The `test/scripts/e2e-local-test.sh` script provides:

- **Automatic Setup**: Creates kind cluster, deploys dependencies
- **Operator Testing**: Installs operator and tests all CRDs
- **Vault Integration**: Tests credential storage and PKI certificates
- **Cleanup**: Removes all test resources
- **Verbose Mode**: Detailed logging for debugging

### E2E Test Workflow

1. **Environment Setup**:
   ```bash
   # Create kind cluster
   kind create cluster --name k8s-postgresql-operator-test
   
   # Deploy PostgreSQL
   kubectl apply -f test/fixtures/postgresql-deployment.yaml
   
   # Deploy Vault
   kubectl apply -f test/fixtures/vault-deployment.yaml
   ```

2. **Operator Installation**:
   ```bash
   # Build and load operator image
   make docker-build IMG=k8s-postgresql-operator:test
   kind load docker-image k8s-postgresql-operator:test
   
   # Install via Helm
   helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator
   ```

3. **CRD Testing**:
   ```bash
   # Test PostgreSQL instance
   kubectl apply -f test/cases/postgresql-instance.yaml
   
   # Test Database creation
   kubectl apply -f test/cases/database.yaml
   
   # Test User creation
   kubectl apply -f test/cases/user.yaml
   
   # Test Grant creation
   kubectl apply -f test/cases/grant.yaml
   ```

4. **Validation**:
   ```bash
   # Verify resources are created
   kubectl get postgresql,database,user,grant
   
   # Check operator logs
   kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator
   
   # Verify Vault integration
   kubectl exec vault-pod -- vault kv get secret/pdb/test/testuser
   ```

### E2E Test Cases

Test cases are located in `test/cases/`:

```
test/cases/
├── postgresql-instance.yaml
├── database.yaml
├── user.yaml
├── grant.yaml
├── schema.yaml
└── rolegroup.yaml
```

### Running Against Real Cluster

To run E2E tests against an existing cluster:

```bash
# Set environment variable
export USE_REAL_CLUSTER=true

# Run E2E tests
make test-e2e-real

# Or run Go tests directly
USE_REAL_CLUSTER=true go test -tags=e2e -v ./test/e2e/...
```

## Test Environment Configuration

### Docker Compose Services

The test environment includes:

```yaml
# test/docker-compose/docker-compose.yml
services:
  vault:
    image: hashicorp/vault:1.13.3
    environment:
      VAULT_DEV_ROOT_TOKEN_ID: myroot
      VAULT_DEV_LISTEN_ADDRESS: 0.0.0.0:8200
    ports:
      - 8200:8200

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: postgres
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - 5432:5432
```

### Vault Setup Scripts

Automated Vault configuration:

- `test/scripts/setup-vault.sh`: Basic Vault setup
- `test/scripts/setup-vault-k8s.sh`: Kubernetes auth setup
- `test/scripts/setup-vault-pki.sh`: PKI secrets engine setup
- `test/scripts/setup-vault-local-k8s.sh`: Local K8s integration

### Test Fixtures

Reusable test resources:

```
test/fixtures/
├── postgresql-deployment.yaml
├── vault-deployment.yaml
├── vault-config.yaml
└── test-data.sql
```

## Continuous Integration

### GitHub Actions

The project includes CI workflows:

```yaml
# .github/workflows/test.yml
name: Tests
on: [push, pull_request]
jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: '1.25.6'
      - run: make test

  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: make test-env-up
      - run: make test-integration
      - run: make test-env-down

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: make test-e2e-local
```

### Test Coverage

Monitor test coverage:

```bash
# Generate coverage report
make test-coverage

# View coverage in browser
go tool cover -html=coverage/unit.out

# Check coverage percentage
go tool cover -func=coverage/unit.out | grep total
```

## Debugging Tests

### Verbose Output

```bash
# Run tests with verbose output
go test -v ./...

# Run specific test with debug info
go test -v -run TestSpecificTest ./internal/controller/
```

### Test Debugging

```bash
# Run single test with debugging
go test -v -run TestPostgreSQLController_Reconcile ./internal/controller/ -count=1

# Use delve debugger
dlv test ./internal/controller/ -- -test.run TestSpecificTest
```

### Environment Debugging

```bash
# Check test environment status
make test-env-up
docker-compose -f test/docker-compose/docker-compose.yml logs

# Test Vault connectivity
curl -H "X-Vault-Token: myroot" http://localhost:8200/v1/sys/health

# Test PostgreSQL connectivity
psql -h localhost -U postgres -d postgres -c "SELECT version();"
```

## Best Practices

### Test Organization

1. **Separate Concerns**: Unit, integration, and E2E tests in separate packages
2. **Use Build Tags**: Tag integration and E2E tests appropriately
3. **Mock External Dependencies**: Use mocks for unit tests
4. **Test Data**: Use fixtures for consistent test data

### Test Writing

1. **Table-Driven Tests**: Use table-driven approach for multiple scenarios
2. **Setup and Teardown**: Proper test environment setup and cleanup
3. **Error Testing**: Test both success and failure scenarios
4. **Timeouts**: Use appropriate timeouts for async operations

### CI/CD Integration

1. **Fast Feedback**: Run unit tests first, then integration tests
2. **Parallel Execution**: Run independent tests in parallel
3. **Resource Cleanup**: Ensure proper cleanup in CI environment
4. **Test Reports**: Generate and store test reports and coverage

## Troubleshooting

### Common Issues

#### Test Environment Issues

```bash
# Port conflicts
lsof -i :8200  # Check if Vault port is in use
lsof -i :5432  # Check if PostgreSQL port is in use

# Docker issues
docker system prune  # Clean up Docker resources
docker-compose -f test/docker-compose/docker-compose.yml down -v
```

#### Vault Connection Issues

```bash
# Check Vault status
curl http://localhost:8200/v1/sys/health

# Verify Vault token
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=myroot
vault status
```

#### PostgreSQL Connection Issues

```bash
# Test PostgreSQL connection
pg_isready -h localhost -p 5432

# Connect to PostgreSQL
psql -h localhost -U postgres -d postgres
```

#### Kind Cluster Issues

```bash
# List kind clusters
kind get clusters

# Delete problematic cluster
kind delete cluster --name k8s-postgresql-operator-test

# Check Docker resources
docker ps -a | grep kind
```

For more troubleshooting information, see the main [README.md](../README.md) and [vault-integration.md](vault-integration.md) documentation.