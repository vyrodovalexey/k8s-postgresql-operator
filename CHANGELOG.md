# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2025-01-30

### Added

#### Core Features
- **Rate Limiting**: Configurable rate limiting for PostgreSQL and Vault API calls to prevent overwhelming external services
  - Separate rate limits for PostgreSQL and Vault operations
  - Configurable requests per second and burst limits
  - Token bucket algorithm implementation with metrics integration
  - Environment variables: `POSTGRESQL_RATE_LIMIT_PER_SECOND`, `POSTGRESQL_RATE_LIMIT_BURST`, `VAULT_RATE_LIMIT_PER_SECOND`, `VAULT_RATE_LIMIT_BURST`

- **Advanced Retry Logic**: Exponential backoff with configurable parameters for all external operations
  - Configurable initial interval, maximum interval, and maximum elapsed time
  - Jitter/randomization factor to prevent thundering herd problems
  - Multiplier for exponential growth of retry intervals
  - Environment variables: `RETRY_INITIAL_INTERVAL_MS`, `RETRY_MAX_INTERVAL_SECS`, `RETRY_MAX_ELAPSED_TIME_SECS`, `RETRY_MULTIPLIER`, `RETRY_RANDOMIZATION_FACTOR`

- **Event Recording**: Comprehensive Kubernetes event recording for all operations and failures
  - Standardized event reasons and messages for all resource types
  - Success events: Created, Updated, Deleted, Connected, Synced, Applied
  - Failure events: CreateFailed, UpdateFailed, DeleteFailed, ConnectionFailed, SyncFailed, ApplyFailed, VaultError, InvalidConfiguration
  - Event recording for PostgreSQL and Vault connectivity issues

- **Enhanced Logging**: Structured JSON logging with configurable log levels
  - Support for debug, info, warn, error log levels
  - Consistent structured logging across all components
  - Environment variable: `LOG_LEVEL`

- **Vault Authentication Timeout**: Configurable timeout for Vault authentication operations
  - Prevents hanging authentication requests
  - Environment variable: `VAULT_AUTH_TIMEOUT_SECS` (default: 30 seconds)

#### Certificate Management
- **Multiple Certificate Providers**: Support for different certificate sources for webhook TLS
  - Provided certificates: Use existing certificates from specified directory
  - Vault PKI: Issue and manage certificates using Vault PKI secrets engine
  - Self-signed: Generate self-signed certificates (default behavior)
  - Automatic provider selection based on configuration

- **Vault PKI Integration**: Complete integration with HashiCorp Vault PKI for webhook certificate management
  - Automatic certificate issuance from Vault PKI secrets engine
  - Configurable certificate TTL and renewal buffer
  - Automatic certificate renewal before expiry
  - CA bundle management for webhook configurations
  - Environment variables: `VAULT_PKI_ENABLED`, `VAULT_PKI_MOUNT_PATH`, `VAULT_PKI_ROLE`, `VAULT_PKI_TTL`, `VAULT_PKI_RENEWAL_BUFFER`

#### Monitoring and Metrics
- **Enhanced Prometheus Metrics**: Additional metrics for monitoring operator performance
  - Rate limiting metrics: `postgresql_operator_rate_limit_wait_total`, `postgresql_operator_rate_limit_allow_total`
  - Retry metrics: `postgresql_operator_retry_attempts_total`, `postgresql_operator_retry_duration_seconds`
  - Vault authentication metrics: `postgresql_operator_vault_auth_duration_seconds`
  - Certificate management metrics

- **ServiceMonitor Support**: Kubernetes ServiceMonitor resource for Prometheus Operator integration
  - Configurable scrape interval and timeout
  - Support for additional labels and namespace configuration
  - Helm chart values: `metrics.serviceMonitor.enabled`, `metrics.serviceMonitor.interval`, `metrics.serviceMonitor.scrapeTimeout`

#### High Availability
- **Pod Disruption Budget**: Support for Pod Disruption Budget to ensure availability during cluster maintenance
  - Configurable minimum available or maximum unavailable pods
  - Helm chart values: `podDisruptionBudget.enabled`, `podDisruptionBudget.minAvailable`, `podDisruptionBudget.maxUnavailable`

#### Configuration Enhancements
- **Comprehensive Configuration Documentation**: Complete documentation of all configuration options
  - Environment variables and command-line flags
  - Configuration precedence (ENV > flags > defaults)
  - Examples for different deployment scenarios
  - Validation and troubleshooting guidance

- **Helm Chart Enhancements**: Updated Helm chart with all new configuration options
  - Rate limiting configuration
  - Retry configuration with exponential backoff
  - Vault PKI configuration
  - Pod Disruption Budget configuration
  - ServiceMonitor configuration
  - Enhanced values schema validation

#### Testing and Documentation
- **Enhanced Testing Framework**: Improved testing infrastructure and coverage
  - Comprehensive unit tests for all new components
  - Integration tests for rate limiting and retry logic
  - E2E tests with Vault PKI integration
  - Test environment setup scripts for local development

- **Architecture Documentation**: Detailed architecture documentation
  - Component overview and interactions
  - Data flow diagrams
  - Design decisions and rationale
  - Security and scalability considerations

- **Configuration Guide**: Comprehensive configuration documentation
  - All environment variables and command-line flags
  - Configuration examples for different scenarios
  - Troubleshooting and best practices

### Changed

#### Improved Error Handling
- **Enhanced Error Messages**: More descriptive error messages with context
- **Standardized Error Types**: Consistent error handling across all components
- **Better Failure Recovery**: Improved recovery from transient failures

#### Performance Optimizations
- **Reduced External API Calls**: Rate limiting and caching reduce unnecessary API calls
- **Optimized Reconciliation**: More efficient reconciliation loops with better error handling
- **Memory Usage**: Improved memory efficiency and garbage collection

#### Security Enhancements
- **Certificate Validation**: Enhanced certificate validation for all TLS connections
- **Secure Defaults**: More secure default configurations
- **Audit Logging**: Better audit trail through comprehensive event recording

### Fixed

#### Stability Improvements
- **Connection Handling**: More robust connection handling for PostgreSQL and Vault
- **Resource Cleanup**: Better cleanup of resources on operator shutdown
- **Memory Leaks**: Fixed potential memory leaks in long-running operations

#### Bug Fixes
- **Webhook Certificate Renewal**: Fixed issues with webhook certificate renewal
- **Concurrent Access**: Fixed race conditions in concurrent operations
- **Error Propagation**: Fixed error propagation in nested operations

### Security

#### Vault Integration Security
- **Token Management**: Improved Vault token lifecycle management
- **Credential Encryption**: Enhanced credential encryption and storage
- **Access Control**: Better access control and permission management

#### Certificate Security
- **TLS Configuration**: Enhanced TLS configuration for all external connections
- **Certificate Rotation**: Automatic certificate rotation with Vault PKI
- **CA Validation**: Proper CA certificate validation

### Migration Notes

#### From 0.0.x to 0.1.x

**No Breaking Changes**: This release maintains full backward compatibility with 0.0.x versions.

**New Environment Variables**: The following new environment variables are available but optional:
- `LOG_LEVEL` - Configure logging level (default: info)
- `VAULT_AUTH_TIMEOUT_SECS` - Vault authentication timeout (default: 30)
- `POSTGRESQL_RATE_LIMIT_PER_SECOND` - PostgreSQL rate limit (default: 10)
- `POSTGRESQL_RATE_LIMIT_BURST` - PostgreSQL burst limit (default: 20)
- `VAULT_RATE_LIMIT_PER_SECOND` - Vault rate limit (default: 10)
- `VAULT_RATE_LIMIT_BURST` - Vault burst limit (default: 20)
- `RETRY_*` - Exponential backoff configuration (uses sensible defaults)
- `VAULT_PKI_*` - Vault PKI configuration (disabled by default)

**Helm Chart Updates**: Update your Helm values to take advantage of new features:
```yaml
# Enable new features
operator:
  logLevel: info
  enableLeaderElection: true
  postgresqlRateLimitPerSecond: 10
  vaultRateLimitPerSecond: 10

# Enable Vault PKI (optional)
vaultPKI:
  enabled: true
  ttl: "720h"
  renewalBuffer: "24h"

# Enable monitoring (optional)
metrics:
  enabled: true
  serviceMonitor:
    enabled: true

# Enable Pod Disruption Budget (optional)
podDisruptionBudget:
  enabled: true
  minAvailable: 1
```

**Monitoring**: If you use Prometheus, new metrics are available for monitoring:
- Rate limiting behavior
- Retry attempts and success rates
- Vault authentication performance
- Certificate management events

**Logging**: Log format has been enhanced with structured JSON logging. Update your log parsing if needed.

### Deprecations

None in this release.

### Removed

None in this release.

---

## [0.0.1] - 2024-XX-XX

### Added
- Initial release of k8s-postgresql-operator
- Basic PostgreSQL instance management
- Database, User, Grant, RoleGroup, and Schema CRDs
- Vault integration for credential storage
- Kubernetes validating webhooks
- Basic Prometheus metrics
- Self-signed certificate generation for webhooks
- Leader election support
- Comprehensive test suite

### Security
- Secure credential storage in HashiCorp Vault
- TLS encryption for all external communications
- RBAC integration with Kubernetes

---

[Unreleased]: https://github.com/vyrodovalexey/k8s-postgresql-operator/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/vyrodovalexey/k8s-postgresql-operator/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/vyrodovalexey/k8s-postgresql-operator/releases/tag/v0.0.1