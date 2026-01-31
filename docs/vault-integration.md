# Vault Integration Guide

This guide covers comprehensive HashiCorp Vault integration with the k8s-postgresql-operator, including credential storage and PKI certificate management.

## Overview

The operator integrates with Vault in two main ways:

1. **Credential Storage**: User passwords are stored in Vault KV v2 secrets engine
2. **PKI Integration**: Webhook TLS certificates can be issued and managed by Vault PKI

## Vault Setup for Local Development

### Prerequisites

- HashiCorp Vault server (v1.13+)
- Kubernetes cluster with kubectl access
- Vault CLI tool

### 1. Basic Vault Configuration

#### Enable Required Secrets Engines

```bash
# Enable KV v2 secrets engine for credential storage
vault secrets enable -path=secret kv-v2

# Enable PKI secrets engine for certificate management (optional)
vault secrets enable pki
vault secrets tune -max-lease-ttl=8760h pki
```

#### Configure Kubernetes Authentication

```bash
# Enable Kubernetes auth method
vault auth enable kubernetes

# Configure Kubernetes auth (replace with your cluster details)
vault write auth/kubernetes/config \
    token_reviewer_jwt="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
    kubernetes_host="https://kubernetes.default.svc:443" \
    kubernetes_ca_cert="$(cat /var/run/secrets/kubernetes.io/serviceaccount/ca.crt)"
```

### 2. Automated Setup Scripts

The project includes scripts to automate Vault setup:

#### For Docker Compose Environment

```bash
# Start Vault and PostgreSQL
make test-env-up

# Setup basic Vault configuration
make test-env-setup-vault
```

#### For Local Kubernetes Integration

```bash
# Setup Vault for local Kubernetes development
make test-env-setup-vault-local-k8s

# Or start complete environment with K8s integration
make test-env-up-k8s
```

## Kubernetes Authentication Configuration

### 1. Create Vault Policy

Create a policy that allows the operator to manage credentials:

```bash
vault policy write k8s-postgresql-operator - <<EOF
# Allow reading and writing secrets for PostgreSQL credentials
path "secret/data/pdb/*" {
  capabilities = ["create", "read", "update", "delete"]
}

path "secret/metadata/pdb/*" {
  capabilities = ["list", "delete"]
}

# Allow reading Vault configuration
path "auth/token/lookup-self" {
  capabilities = ["read"]
}
EOF
```

### 2. Create Kubernetes Auth Role

```bash
vault write auth/kubernetes/role/k8s-postgresql-operator \
    bound_service_account_names=k8s-postgresql-operator-controller \
    bound_service_account_namespaces=default \
    policies=k8s-postgresql-operator \
    ttl=1h
```

### 3. Configure Operator

Set the Vault role in your operator configuration:

```yaml
# In Helm values.yaml
vault:
  addr: "http://vault.example.com:8200"
  role: "k8s-postgresql-operator"
  mountPoint: "secret"
  secretPath: "pdb"

# Optional: Configure Vault authentication timeout and retry settings
operator:
  vaultAuthTimeoutSecs: 30
  vaultAvailabilityRetries: 3
  vaultAvailabilityRetryDelaySecs: 10
  vaultRateLimitPerSecond: 10
  vaultRateLimitBurst: 20
```

## PKI Setup for Webhook Certificates

### 1. Enable and Configure PKI

```bash
# Enable PKI secrets engine
vault secrets enable pki
vault secrets tune -max-lease-ttl=8760h pki

# Generate root CA
vault write pki/root/generate/internal \
    common_name="K8s PostgreSQL Operator CA" \
    ttl=8760h
```

### 2. Create PKI Role for Webhook Certificates

```bash
vault write pki/roles/webhook-cert \
    allowed_domains="k8s-postgresql-operator-controller-service,k8s-postgresql-operator-controller-service.default,k8s-postgresql-operator-controller-service.default.svc,k8s-postgresql-operator-controller-service.default.svc.cluster.local" \
    allow_subdomains=false \
    max_ttl=720h \
    generate_lease=true
```

### 3. Create PKI Policy

```bash
vault policy write k8s-postgresql-operator-pki - <<EOF
# Allow issuing certificates
path "pki/issue/webhook-cert" {
  capabilities = ["create", "update"]
}

# Allow reading CA information
path "pki/ca/pem" {
  capabilities = ["read"]
}

path "pki/ca_chain" {
  capabilities = ["read"]
}
EOF
```

### 4. Update Kubernetes Auth Role

Add PKI policy to the existing role:

```bash
vault write auth/kubernetes/role/k8s-postgresql-operator \
    bound_service_account_names=k8s-postgresql-operator-controller \
    bound_service_account_namespaces=default \
    policies=k8s-postgresql-operator,k8s-postgresql-operator-pki \
    ttl=1h
```

### 5. Enable PKI in Operator

```yaml
# In Helm values.yaml
vaultPKI:
  enabled: true
  mountPath: "pki"
  role: "webhook-cert"
  ttl: "720h"
  renewalBuffer: "24h"
```

### 6. Automated PKI Setup

Use the provided script for automated PKI setup:

```bash
# Setup Vault PKI for testing
make test-env-setup-vault-pki

# Or use the script directly
./test/scripts/setup-vault-pki.sh
```

## Credential Storage Patterns

### Storage Path Structure

The operator stores PostgreSQL user credentials using this path pattern:

```
{mount_point}/data/{secret_path}/{postgresql_id}/{username}
```

**Example:**
```
secret/data/pdb/test-pg-001/app_user
```

Where:
- `mount_point`: Vault KV mount point (default: `secret`)
- `secret_path`: Secret path prefix (default: `pdb`)
- `postgresql_id`: PostgreSQL instance ID from the CRD (e.g., `test-pg-001`)
- `username`: PostgreSQL username (e.g., `app_user`)

**Note**: The path structure has been simplified from the previous version. The `instance_type` and `uuid` components have been consolidated into the `postgresql_id` for better clarity and consistency.

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

### Accessing Credentials

Applications can retrieve credentials using Vault:

```bash
# Using Vault CLI
vault kv get secret/pdb/test-pg-001/app_user

# Using Vault API
curl -H "X-Vault-Token: $VAULT_TOKEN" \
     $VAULT_ADDR/v1/secret/data/pdb/test-pg-001/app_user
```

## Production Configuration

### 1. Vault High Availability

For production, configure Vault with:

- **Raft storage backend** for high availability
- **TLS encryption** for all communications
- **Proper authentication** and authorization
- **Regular backups** of Vault data

### 2. Network Security

- Use **private networks** for Vault communication
- Configure **network policies** to restrict access
- Enable **audit logging** for security monitoring

### 3. Certificate Management

When using Vault PKI:

- Set appropriate **certificate TTL** values
- Configure **automatic renewal** before expiry
- Monitor **certificate expiration** alerts
- Use **intermediate CAs** for better security

### 4. Monitoring and Alerting

Monitor Vault integration:

```bash
# Check Vault connectivity
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator | grep vault

# Monitor certificate expiration
kubectl logs -l app.kubernetes.io/name=k8s-postgresql-operator | grep certificate

# Check rate limiting metrics (if Prometheus is enabled)
kubectl port-forward svc/k8s-postgresql-operator-metrics 8080:8080
curl http://localhost:8080/metrics | grep vault_rate_limit

# Monitor retry metrics
curl http://localhost:8080/metrics | grep retry
```

## Troubleshooting

### Common Issues

#### Vault Authentication Failures

```bash
# Check service account token
kubectl get serviceaccount k8s-postgresql-operator-controller -o yaml

# Verify Vault role configuration
vault read auth/kubernetes/role/k8s-postgresql-operator

# Test authentication manually
vault write auth/kubernetes/login \
    role=k8s-postgresql-operator \
    jwt="$(kubectl get secret -o jsonpath='{.data.token}' | base64 -d)"
```

#### PKI Certificate Issues

```bash
# Check PKI role configuration
vault read pki/roles/webhook-cert

# Test certificate issuance
vault write pki/issue/webhook-cert \
    common_name=k8s-postgresql-operator-controller-service.default.svc.cluster.local
```

#### Credential Storage Issues

```bash
# Check KV engine status
vault secrets list

# Verify policy permissions
vault policy read k8s-postgresql-operator

# Test credential storage
vault kv put secret/pdb/test-pg-001/testuser password=testpass
vault kv get secret/pdb/test-pg-001/testuser

# List all credentials for a PostgreSQL instance
vault kv list secret/pdb/test-pg-001/

# Check if path exists
vault kv get secret/pdb/test-pg-001/app_user
```

#### Path Structure Issues

If you're having issues with credential storage paths:

```bash
# Verify the correct path structure
# Format: {mount_point}/data/{secret_path}/{postgresql_id}/{username}
# Example: secret/data/pdb/test-pg-001/app_user

# Check operator configuration
kubectl get deployment k8s-postgresql-operator-controller -o yaml | grep -A 10 env

# Verify PostgreSQL ID matches between CRDs
kubectl get postgresql -o yaml | grep postgresqlID
kubectl get user -o yaml | grep postgresqlID
```

### Debug Mode

Enable debug logging in the operator:

```yaml
# In deployment
env:
- name: LOG_LEVEL
  value: "debug"
```

## Security Best Practices

1. **Least Privilege**: Grant minimal required permissions
2. **Token Rotation**: Use short-lived tokens with automatic renewal
3. **Audit Logging**: Enable comprehensive audit logs
4. **Network Isolation**: Use network policies to restrict access
5. **Regular Updates**: Keep Vault and operator versions current
6. **Backup Strategy**: Implement regular Vault backup procedures
7. **Certificate Monitoring**: Monitor certificate expiration and renewal

## Integration Testing

Test your Vault integration:

```bash
# Run Vault PKI integration tests
make test-vault-pki

# Run full integration tests
make test-integration

# Run e2e tests with Vault
make test-e2e-local
```

For more information, see the [testing documentation](testing.md).