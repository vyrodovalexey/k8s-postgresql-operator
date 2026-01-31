# Getting Started with k8s-postgresql-operator

This guide will help you get the k8s-postgresql-operator up and running quickly.

## Prerequisites

Before you begin, ensure you have:

- **Kubernetes cluster** (v1.24+) with kubectl access
- **HashiCorp Vault** server accessible from your cluster
- **PostgreSQL instances** accessible from your cluster
- **Helm 3.0+** for installation
- **Go 1.25.6+** (if building from source)

## Quick Installation

### 1. Install with Helm

The fastest way to get started is using the Helm chart:

```bash
# Add your image registry (replace with your actual registry)
export IMG=my-registry/k8s-postgresql-operator:latest

# Install the operator with basic configuration
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set image.repository=my-registry/k8s-postgresql-operator \
  --set image.tag=latest

# Install with advanced features enabled
helm install k8s-postgresql-operator ./charts/k8s-postgresql-operator \
  --set vault.addr=http://vault.example.com:8200 \
  --set vault.role=my-vault-role \
  --set image.repository=my-registry/k8s-postgresql-operator \
  --set image.tag=latest \
  --set operator.logLevel=debug \
  --set operator.enableLeaderElection=true \
  --set operator.postgresqlRateLimitPerSecond=15 \
  --set operator.vaultRateLimitPerSecond=15 \
  --set vaultPKI.enabled=true \
  --set metrics.enabled=true
```

### 2. Verify Installation

Check that the operator is running:

```bash
kubectl get pods -l app.kubernetes.io/name=k8s-postgresql-operator
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator
```

## Sample Resource Application Sequence

Follow this sequence to create your first PostgreSQL resources:

### 1. Define a PostgreSQL Instance

Create a PostgreSQL instance resource that points to your existing PostgreSQL server:

```yaml
# postgresql-instance.yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Postgresql
metadata:
  name: my-postgresql
  namespace: default
spec:
  externalInstance:
    postgresqlID: "my-postgresql-instance"
    address: "postgres.example.com"
    port: 5432
    sslMode: "require"
```

Apply the resource:
```bash
kubectl apply -f postgresql-instance.yaml
```

### 2. Create a Database

```yaml
# database.yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Database
metadata:
  name: my-app-db
  namespace: default
spec:
  postgresqlID: "my-postgresql-instance"
  database: "myapp"
  owner: "app_owner"
  schema: "public"
  deleteFromCRD: false
```

Apply the resource:
```bash
kubectl apply -f database.yaml
```

### 3. Create a User

```yaml
# user.yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: User
metadata:
  name: app-user
  namespace: default
spec:
  postgresqlID: "my-postgresql-instance"
  username: "app_user"
  updatePassword: false
```

Apply the resource:
```bash
kubectl apply -f user.yaml
```

### 4. Create a Schema

```yaml
# schema.yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Schema
metadata:
  name: app-schema
  namespace: default
spec:
  postgresqlID: "my-postgresql-instance"
  database: "myapp"
  schema: "app_schema"
  owner: "app_user"
```

Apply the resource:
```bash
kubectl apply -f schema.yaml
```

### 5. Grant Permissions

```yaml
# grant.yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: Grant
metadata:
  name: app-permissions
  namespace: default
spec:
  postgresqlID: "my-postgresql-instance"
  database: "myapp"
  role: "app_user"
  grants:
    - type: "table"
      privileges: ["SELECT", "INSERT", "UPDATE", "DELETE"]
      schema: "app_schema"
      table: "*"
    - type: "schema"
      privileges: ["USAGE", "CREATE"]
      schema: "app_schema"
```

Apply the resource:
```bash
kubectl apply -f grant.yaml
```

### 6. Create Role Groups (Optional)

```yaml
# rolegroup.yaml
apiVersion: postgresql-operator.vyrodovalexey.github.com/v1alpha1
kind: RoleGroup
metadata:
  name: app-roles
  namespace: default
spec:
  postgresqlID: "my-postgresql-instance"
  roleGroup: "app_group"
  roles:
    - "app_user"
    - "readonly_user"
```

Apply the resource:
```bash
kubectl apply -f rolegroup.yaml
```

## Verify Your Setup

### Check Resource Status

```bash
# Check all PostgreSQL resources
kubectl get postgresql,database,user,grant,schema,rolegroup

# Check specific resource details
kubectl describe postgresql my-postgresql
kubectl describe database my-app-db
kubectl describe user app-user
```

### Check Vault Integration

The operator stores user passwords in Vault. Check that credentials are stored:

```bash
# Using Vault CLI (if you have access)
vault kv get secret/pdb/my-postgresql-instance/app_user
```

### Check Operator Logs

```bash
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator -f
```

## Configuration Options

The operator supports extensive configuration options:

### Basic Configuration
- **Log Level**: Set `operator.logLevel` to `debug`, `info`, `warn`, or `error`
- **Leader Election**: Enable with `operator.enableLeaderElection=true` for HA deployments
- **Rate Limiting**: Configure `operator.postgresqlRateLimitPerSecond` and `operator.vaultRateLimitPerSecond`

### Advanced Configuration
- **Retry Settings**: Configure exponential backoff with `operator.retry.*` parameters
- **Vault PKI**: Enable automatic certificate management with `vaultPKI.enabled=true`
- **Metrics**: Enable Prometheus metrics with `metrics.enabled=true`

For complete configuration options, see [configuration.md](configuration.md).

## Next Steps

- **Vault Integration**: Set up Vault PKI for webhook certificates - see [vault-integration.md](vault-integration.md)
- **Testing**: Run tests to validate your setup - see [testing.md](testing.md)
- **Configuration**: Review all configuration options - see [configuration.md](configuration.md)
- **Architecture**: Understand the operator architecture - see [architecture.md](architecture.md)
- **Monitoring**: Enable Prometheus metrics for monitoring
- **Production**: Configure resource limits, leader election, and high availability

## Common Issues

### Vault Connection Issues

If you see Vault connection errors:

1. Verify Vault address is accessible from the cluster
2. Check Vault authentication configuration
3. Ensure the Vault role has proper permissions

### PostgreSQL Connection Issues

If PostgreSQL connection fails:

1. Verify PostgreSQL address and port
2. Check SSL configuration
3. Ensure network connectivity from cluster to PostgreSQL

### Webhook Certificate Issues

If webhook validation fails:

1. Check webhook certificate generation
2. Consider using Vault PKI for certificate management
3. Verify webhook service is accessible

For more troubleshooting, check the operator logs and see the main [README.md](../README.md) for detailed configuration options.