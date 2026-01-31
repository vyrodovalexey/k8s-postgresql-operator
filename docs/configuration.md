# Configuration Guide

This guide provides comprehensive documentation for all configuration options available in the k8s-postgresql-operator.

## Configuration Precedence

Configuration values are applied in the following order (highest to lowest precedence):

1. **Environment Variables** - Highest precedence
2. **Command-line Flags** - Medium precedence  
3. **Default Values** - Lowest precedence

Environment variables will override command-line flags, and both will override default values.

## Environment Variables

### Vault Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `VAULT_ADDR` | Vault server address | `http://0.0.0.0:8200` | `https://vault.example.com:8200` |
| `VAULT_ROLE` | Vault role name for Kubernetes authentication | `role` | `k8s-postgresql-operator` |
| `VAULT_MOUNT_POINT` | KV v2 secrets engine mount point | `secret` | `kv` |
| `VAULT_SECRET_PATH` | Prefix path for storing secrets in Vault | `pdb` | `postgresql-credentials` |
| `VAULT_AVAILABILITY_RETRIES` | Number of retries for Vault availability check | `3` | `5` |
| `VAULT_AVAILABILITY_RETRY_DELAY_SECS` | Delay in seconds between Vault availability retries | `10` | `15` |
| `VAULT_AUTH_TIMEOUT_SECS` | Timeout in seconds for Vault authentication | `30` | `60` |

### Vault PKI Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `VAULT_PKI_ENABLED` | Enable Vault PKI for webhook certificates | `false` | `true` |
| `VAULT_PKI_MOUNT_PATH` | Vault PKI secrets engine mount path | `pki` | `pki-int` |
| `VAULT_PKI_ROLE` | Vault PKI role name for certificate issuance | `webhook-cert` | `k8s-webhook` |
| `VAULT_PKI_TTL` | TTL for Vault PKI issued certificates | `720h` | `168h` |
| `VAULT_PKI_RENEWAL_BUFFER` | Time before expiry to trigger renewal | `24h` | `48h` |

### Webhook Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `WEBHOOK_CERT_PATH` | Directory containing webhook certificates (empty for auto-generation) | `` | `/etc/certs` |
| `WEBHOOK_CERT_NAME` | Webhook certificate file name | `tls.crt` | `webhook.crt` |
| `WEBHOOK_CERT_KEY` | Webhook key file name | `tls.key` | `webhook.key` |
| `WEBHOOK_SERVER_PORT` | Webhook server port | `8443` | `9443` |
| `WEBHOOK_SERVER_ADDR` | Webhook server address | `0.0.0.0` | `127.0.0.1` |
| `WEBHOOK_K8S_SERVICE_NAME` | Kubernetes service name for webhook | `k8s-postgresql-operator-controller-service` | `custom-webhook-service` |

### Webhook Names Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `K8S_WEBHOOK_NAME_POSTGRESQL` | ValidatingWebhookConfiguration name for PostgreSQL | `k8s-postgresql-operator-validating-webhook-postgresql` |
| `K8S_WEBHOOK_NAME_USER` | ValidatingWebhookConfiguration name for User | `k8s-postgresql-operator-validating-webhook-user` |
| `K8S_WEBHOOK_NAME_DATABASE` | ValidatingWebhookConfiguration name for Database | `k8s-postgresql-operator-validating-webhook-database` |
| `K8S_WEBHOOK_NAME_GRANT` | ValidatingWebhookConfiguration name for Grant | `k8s-postgresql-operator-validating-webhook-grant` |
| `K8S_WEBHOOK_NAME_ROLEGROUP` | ValidatingWebhookConfiguration name for RoleGroup | `k8s-postgresql-operator-validating-webhook-rolegroup` |
| `K8S_WEBHOOK_NAME_SCHEMA` | ValidatingWebhookConfiguration name for Schema | `k8s-postgresql-operator-validating-webhook-schema` |

### Operator Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `ENABLE_LEADER_ELECTION` | Enable leader election for controller manager | `false` | `true` |
| `PROBE_ADDR` | Health probe bind address | `:8081` | `:9081` |
| `EXCLUDE_USER_LIST` | Comma-separated list of users to exclude from management | `postgres` | `postgres,admin,root` |
| `LOG_LEVEL` | Log level: debug, info, warn, error | `info` | `debug` |

### PostgreSQL Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `POSTGRESQL_CONNECTION_RETRIES` | Number of retries for PostgreSQL connection test | `3` | `5` |
| `POSTGRESQL_CONNECTION_TIMEOUT_SECS` | Timeout in seconds between PostgreSQL connection retries | `10` | `30` |

### Rate Limiting Configuration

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `POSTGRESQL_RATE_LIMIT_PER_SECOND` | Rate limit for PostgreSQL connections per second | `10` | `20` |
| `POSTGRESQL_RATE_LIMIT_BURST` | Burst limit for PostgreSQL connections | `20` | `50` |
| `VAULT_RATE_LIMIT_PER_SECOND` | Rate limit for Vault requests per second | `10` | `15` |
| `VAULT_RATE_LIMIT_BURST` | Burst limit for Vault requests | `20` | `30` |

### Retry Configuration (Exponential Backoff)

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `RETRY_INITIAL_INTERVAL_MS` | Initial interval in milliseconds for exponential backoff | `500` | `1000` |
| `RETRY_MAX_INTERVAL_SECS` | Maximum interval in seconds for exponential backoff | `30` | `60` |
| `RETRY_MAX_ELAPSED_TIME_SECS` | Maximum elapsed time in seconds for all retries | `300` | `600` |
| `RETRY_MULTIPLIER` | Multiplier for exponential backoff interval growth | `2.0` | `1.5` |
| `RETRY_RANDOMIZATION_FACTOR` | Randomization factor (jitter) for exponential backoff | `0.5` | `0.3` |

### Kubernetes Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `K8S_TOKEN_PATH` | Path to Kubernetes service account token | `/var/run/secrets/kubernetes.io/serviceaccount/token` |
| `K8S_NAMESPACE_PATH` | Path to Kubernetes namespace file | `/var/run/secrets/kubernetes.io/serviceaccount/namespace` |

## Command-Line Flags

All environment variables can also be set via command-line flags. The flag names follow the pattern of converting the environment variable to lowercase and replacing underscores with hyphens.

### Common Flags

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--health-probe-bind-address` | `PROBE_ADDR` | Health probe bind address |
| `--leader-elect` | `ENABLE_LEADER_ELECTION` | Enable leader election |
| `--vault-addr` | `VAULT_ADDR` | Vault server address |
| `--vault-role` | `VAULT_ROLE` | Vault role name |
| `--webhook-cert-path` | `WEBHOOK_CERT_PATH` | Webhook certificate directory |
| `--log-level` | `LOG_LEVEL` | Log level |

### Vault PKI Flags

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--vault-pki-enabled` | `VAULT_PKI_ENABLED` | Enable Vault PKI for webhook certificates |
| `--vault-pki-mount-path` | `VAULT_PKI_MOUNT_PATH` | Vault PKI secrets engine mount path |
| `--vault-pki-role` | `VAULT_PKI_ROLE` | Vault PKI role name for certificate issuance |
| `--vault-pki-ttl` | `VAULT_PKI_TTL` | TTL for Vault PKI issued certificates |
| `--vault-pki-renewal-buffer` | `VAULT_PKI_RENEWAL_BUFFER` | Time before expiry to trigger renewal |

### Rate Limiting Flags

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--postgresql-rate-limit-per-second` | `POSTGRESQL_RATE_LIMIT_PER_SECOND` | Rate limit for PostgreSQL connections per second |
| `--postgresql-rate-limit-burst` | `POSTGRESQL_RATE_LIMIT_BURST` | Burst limit for PostgreSQL connections |
| `--vault-rate-limit-per-second` | `VAULT_RATE_LIMIT_PER_SECOND` | Rate limit for Vault requests per second |
| `--vault-rate-limit-burst` | `VAULT_RATE_LIMIT_BURST` | Burst limit for Vault requests |

### Retry Configuration Flags

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--retry-initial-interval-ms` | `RETRY_INITIAL_INTERVAL_MS` | Initial interval in milliseconds for exponential backoff |
| `--retry-max-interval-secs` | `RETRY_MAX_INTERVAL_SECS` | Maximum interval in seconds for exponential backoff |
| `--retry-max-elapsed-time-secs` | `RETRY_MAX_ELAPSED_TIME_SECS` | Maximum elapsed time in seconds for all retries |
| `--retry-multiplier` | `RETRY_MULTIPLIER` | Multiplier for exponential backoff interval growth |
| `--retry-randomization-factor` | `RETRY_RANDOMIZATION_FACTOR` | Randomization factor (jitter) for exponential backoff |

## Configuration Examples

### Development Environment

```bash
# Environment variables for development
export VAULT_ADDR=http://localhost:8200
export VAULT_ROLE=k8s-postgresql-operator
export LOG_LEVEL=debug
export ENABLE_LEADER_ELECTION=false
export POSTGRESQL_RATE_LIMIT_PER_SECOND=50
export VAULT_RATE_LIMIT_PER_SECOND=50
export RETRY_INITIAL_INTERVAL_MS=100
```

### Production Environment

```bash
# Environment variables for production
export VAULT_ADDR=https://vault.production.com:8200
export VAULT_ROLE=k8s-postgresql-operator
export LOG_LEVEL=info
export ENABLE_LEADER_ELECTION=true
export VAULT_PKI_ENABLED=true
export VAULT_PKI_TTL=168h
export VAULT_PKI_RENEWAL_BUFFER=24h
export POSTGRESQL_RATE_LIMIT_PER_SECOND=10
export VAULT_RATE_LIMIT_PER_SECOND=10
export RETRY_MAX_ELAPSED_TIME_SECS=600
```

### High-Throughput Environment

```bash
# Environment variables for high-throughput scenarios
export POSTGRESQL_RATE_LIMIT_PER_SECOND=100
export POSTGRESQL_RATE_LIMIT_BURST=200
export VAULT_RATE_LIMIT_PER_SECOND=50
export VAULT_RATE_LIMIT_BURST=100
export RETRY_INITIAL_INTERVAL_MS=100
export RETRY_MAX_INTERVAL_SECS=10
export POSTGRESQL_CONNECTION_RETRIES=5
export VAULT_AVAILABILITY_RETRIES=5
```

## Helm Chart Configuration

When using the Helm chart, these configuration options can be set via values:

```yaml
# values.yaml
vault:
  addr: "https://vault.example.com:8200"
  role: "k8s-postgresql-operator"
  mountPoint: "secret"
  secretPath: "pdb"

vaultPKI:
  enabled: true
  mountPath: "pki"
  role: "webhook-cert"
  ttl: "720h"
  renewalBuffer: "24h"

operator:
  enableLeaderElection: true
  logLevel: "info"
  postgresqlRateLimitPerSecond: 10
  postgresqlRateLimitBurst: 20
  vaultRateLimitPerSecond: 10
  vaultRateLimitBurst: 20
  vaultAuthTimeoutSecs: 30
  retry:
    initialIntervalMs: 500
    maxIntervalSecs: 30
    maxElapsedTimeSecs: 300
    multiplier: 2.0
    randomizationFactor: 0.5
```

## Configuration Validation

The operator validates configuration at startup and will log warnings or errors for invalid values:

### Log Level Validation
- Valid values: `debug`, `info`, `warn`, `error`
- Invalid values default to `info`

### Rate Limiting Validation
- Rate per second must be positive
- Burst must be positive and >= rate per second
- Invalid values use defaults

### Retry Configuration Validation
- Initial interval must be positive
- Max interval must be >= initial interval
- Max elapsed time must be positive
- Multiplier must be > 1.0
- Randomization factor must be between 0.0 and 1.0

### Vault PKI Validation
- TTL and renewal buffer must be valid duration strings
- Renewal buffer should be less than TTL
- Invalid durations use defaults

## Monitoring Configuration

Monitor configuration effectiveness through metrics and logs:

### Metrics to Monitor
- `postgresql_operator_rate_limit_wait_total` - Rate limiting effectiveness
- `postgresql_operator_retry_attempts_total` - Retry behavior
- `postgresql_operator_vault_auth_duration_seconds` - Vault authentication performance

### Log Messages to Watch
- Configuration validation warnings
- Rate limiting events
- Retry attempts and backoff behavior
- Certificate renewal events

## Troubleshooting Configuration

### Common Issues

#### Rate Limiting Too Aggressive
**Symptoms**: Slow operation processing, high retry counts
**Solution**: Increase rate limits or burst values

#### Insufficient Retries
**Symptoms**: Operations failing due to temporary network issues
**Solution**: Increase retry counts or max elapsed time

#### Vault Authentication Timeouts
**Symptoms**: Vault authentication failures in logs
**Solution**: Increase `VAULT_AUTH_TIMEOUT_SECS`

#### Certificate Renewal Issues
**Symptoms**: Webhook failures, certificate expiration warnings
**Solution**: Adjust `VAULT_PKI_RENEWAL_BUFFER` or `VAULT_PKI_TTL`

### Configuration Testing

Test configuration changes in a development environment:

```bash
# Test with debug logging
export LOG_LEVEL=debug
./manager

# Test rate limiting
export POSTGRESQL_RATE_LIMIT_PER_SECOND=1
export POSTGRESQL_RATE_LIMIT_BURST=1
# Observe rate limiting in action

# Test retry behavior
export RETRY_INITIAL_INTERVAL_MS=100
export RETRY_MAX_INTERVAL_SECS=5
# Simulate failures to observe retry behavior
```

## Best Practices

### Production Configuration
1. **Enable leader election** for high availability
2. **Use appropriate log levels** (info or warn for production)
3. **Configure rate limiting** based on your infrastructure capacity
4. **Set reasonable retry limits** to balance reliability and performance
5. **Enable Vault PKI** for better certificate management
6. **Monitor metrics** to tune configuration over time

### Security Configuration
1. **Use HTTPS for Vault** in production
2. **Limit excluded user list** to essential system users only
3. **Configure appropriate certificate TTL** values
4. **Use strong authentication** for Vault integration

### Performance Configuration
1. **Tune rate limits** based on your PostgreSQL and Vault capacity
2. **Adjust retry parameters** for your network conditions
3. **Monitor and adjust** based on observed performance metrics
4. **Consider burst limits** for handling traffic spikes