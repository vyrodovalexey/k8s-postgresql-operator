package config

import "time"

const (
	defaultWebhookCertPath          = ""
	defaultWebhookCertName          = "tls.crt"
	defaultWebhookCertKey           = "tls.key"
	defaultWebhookServerPort        = 8443
	defaultWebhookServerAddr        = "0.0.0.0"
	defaultWebhookK8sServiceName    = "k8s-postgresql-operator-controller-service"
	defaultK8sWebhookNamePostgresql = "k8s-postgresql-operator-validating-webhook-postgresql"
	defaultK8sWebhookNameUser       = "k8s-postgresql-operator-validating-webhook-user"
	defaultK8sWebhookNameDatabase   = "k8s-postgresql-operator-validating-webhook-database"
	defaultK8sWebhookNameGrant      = "k8s-postgresql-operator-validating-webhook-grant"
	defaultK8sWebhookNameRoleGroup  = "k8s-postgresql-operator-validating-webhook-rolegroup"
	defaultK8sWebhookNameSchema     = "k8s-postgresql-operator-validating-webhook-schema"
	defaultEnableLeaderElection     = false
	defaultProbeAddr                = ":8081"
	defaultVaultAddr                = "http://0.0.0.0:8200"
	defaultVaultRole                = "role"
	defaultVaultMountPoint          = "secret"
	defaultVaultSecretPath          = "pdb"
	//nolint:gosec // default k8s token path
	defaultK8sTokenPath                    = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultK8SNamespacePath                = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultExcludeUserList                 = "postgres"
	defaultPostgresqlConnectionRetries     = 3
	defaultPostgresqlConnectionTimeoutSecs = 10
	defaultVaultAvailabilityRetries        = 3
	defaultVaultAvailabilityRetryDelaySecs = 10
	defaultVaultAuthTimeoutSecs            = 30

	// Exponential backoff defaults for retry logic
	defaultRetryInitialIntervalMs   = 500
	defaultRetryMaxIntervalSecs     = 30
	defaultRetryMaxElapsedTimeSecs  = 300 // 5 minutes
	defaultRetryMultiplier          = 2.0
	defaultRetryRandomizationFactor = 0.5

	// Vault PKI defaults
	defaultVaultPKIEnabled       = false
	defaultVaultPKIMountPath     = "pki"
	defaultVaultPKIRole          = "webhook-cert"
	defaultVaultPKITTL           = "720h" // 30 days
	defaultVaultPKIRenewalBuffer = "24h"  // Renew 24h before expiry

	// Rate limiting defaults for external service calls
	defaultPostgresqlRateLimitPerSecond = 10.0 // 10 requests per second
	defaultPostgresqlRateLimitBurst     = 20   // Allow burst of 20 requests
	defaultVaultRateLimitPerSecond      = 10.0 // 10 requests per second
	defaultVaultRateLimitBurst          = 20   // Allow burst of 20 requests

	// Log level defaults
	defaultLogLevel = "info"
)

// Config is a structure for storing configuration
type Config struct {
	WebhookCertPath                 string `env:"WEBHOOK_CERT_PATH"`
	WebhookCertName                 string `env:"WEBHOOK_CERT_NAME"`
	WebhookCertKey                  string `env:"WEBHOOK_CERT_KEY"`
	WebhookServerPort               int    `env:"WEBHOOK_SERVER_PORT"`
	WebhookServerAddr               string `env:"WEBHOOK_SERVER_ADDR"`
	WebhookK8sServiceName           string `env:"WEBHOOK_K8S_SERVICE_NAME"`
	K8sWebhookNamePostgresql        string `env:"K8S_WEBHOOK_NAME_POSTGRESQL"`
	K8sWebhookNameUser              string `env:"K8S_WEBHOOK_NAME_USER"`
	K8sWebhookNameDatabase          string `env:"K8S_WEBHOOK_NAME_DATABASE"`
	K8sWebhookNameGrant             string `env:"K8S_WEBHOOK_NAME_GRANT"`
	K8sWebhookNameRoleGroup         string `env:"K8S_WEBHOOK_NAME_ROLEGROUP"`
	K8sWebhookNameSchema            string `env:"K8S_WEBHOOK_NAME_SCHEMA"`
	EnableLeaderElection            bool   `env:"ENABLE_LEADER_ELECTION"`
	ProbeAddr                       string `env:"PROBE_ADDR"`
	VaultAddr                       string `env:"VAULT_ADDR"`
	VaultRole                       string `env:"VAULT_ROLE"`
	VaultMountPoint                 string `env:"VAULT_MOUNT_POINT"`
	VaultSecretPath                 string `env:"VAULT_SECRET_PATH"`
	K8sTokenPath                    string `env:"K8S_TOKEN_PATH"`
	K8sNamespacePath                string `env:"K8S_NAMESPACE_PATH"`
	ExcludeUserList                 string `env:"EXCLUDE_USER_LIST"`
	PostgresqlConnectionRetries     int    `env:"POSTGRESQL_CONNECTION_RETRIES"`
	PostgresqlConnectionTimeoutSecs int    `env:"POSTGRESQL_CONNECTION_TIMEOUT_SECS"`
	VaultAvailabilityRetries        int    `env:"VAULT_AVAILABILITY_RETRIES"`
	VaultAvailabilityRetryDelaySecs int    `env:"VAULT_AVAILABILITY_RETRY_DELAY_SECS"`
	VaultAuthTimeoutSecs            int    `env:"VAULT_AUTH_TIMEOUT_SECS"`

	// Exponential backoff configuration for retry logic
	RetryInitialIntervalMs   int     `env:"RETRY_INITIAL_INTERVAL_MS"`
	RetryMaxIntervalSecs     int     `env:"RETRY_MAX_INTERVAL_SECS"`
	RetryMaxElapsedTimeSecs  int     `env:"RETRY_MAX_ELAPSED_TIME_SECS"`
	RetryMultiplier          float64 `env:"RETRY_MULTIPLIER"`
	RetryRandomizationFactor float64 `env:"RETRY_RANDOMIZATION_FACTOR"`

	// Vault PKI Configuration for webhook certificates
	VaultPKIEnabled       bool   `env:"VAULT_PKI_ENABLED"`
	VaultPKIMountPath     string `env:"VAULT_PKI_MOUNT_PATH"`
	VaultPKIRole          string `env:"VAULT_PKI_ROLE"`
	VaultPKITTL           string `env:"VAULT_PKI_TTL"`
	VaultPKIRenewalBuffer string `env:"VAULT_PKI_RENEWAL_BUFFER"`

	// Rate limiting configuration for external service calls
	PostgresqlRateLimitPerSecond float64 `env:"POSTGRESQL_RATE_LIMIT_PER_SECOND"`
	PostgresqlRateLimitBurst     int     `env:"POSTGRESQL_RATE_LIMIT_BURST"`
	VaultRateLimitPerSecond      float64 `env:"VAULT_RATE_LIMIT_PER_SECOND"`
	VaultRateLimitBurst          int     `env:"VAULT_RATE_LIMIT_BURST"`

	// Log level configuration
	LogLevel string `env:"LOG_LEVEL"`
}

// New creates a new configuration instance with default values
func New() *Config {
	return &Config{
		WebhookCertPath:                 defaultWebhookCertPath,
		WebhookCertName:                 defaultWebhookCertName,
		WebhookCertKey:                  defaultWebhookCertKey,
		WebhookServerPort:               defaultWebhookServerPort,
		WebhookServerAddr:               defaultWebhookServerAddr,
		WebhookK8sServiceName:           defaultWebhookK8sServiceName,
		K8sWebhookNamePostgresql:        defaultK8sWebhookNamePostgresql,
		K8sWebhookNameUser:              defaultK8sWebhookNameUser,
		K8sWebhookNameDatabase:          defaultK8sWebhookNameDatabase,
		K8sWebhookNameGrant:             defaultK8sWebhookNameGrant,
		K8sWebhookNameRoleGroup:         defaultK8sWebhookNameRoleGroup,
		K8sWebhookNameSchema:            defaultK8sWebhookNameSchema,
		EnableLeaderElection:            defaultEnableLeaderElection,
		ProbeAddr:                       defaultProbeAddr,
		VaultAddr:                       defaultVaultAddr,
		VaultRole:                       defaultVaultRole,
		VaultMountPoint:                 defaultVaultMountPoint,
		VaultSecretPath:                 defaultVaultSecretPath,
		K8sTokenPath:                    defaultK8sTokenPath,
		K8sNamespacePath:                defaultK8SNamespacePath,
		ExcludeUserList:                 defaultExcludeUserList,
		PostgresqlConnectionRetries:     defaultPostgresqlConnectionRetries,
		PostgresqlConnectionTimeoutSecs: defaultPostgresqlConnectionTimeoutSecs,
		VaultAvailabilityRetries:        defaultVaultAvailabilityRetries,
		VaultAvailabilityRetryDelaySecs: defaultVaultAvailabilityRetryDelaySecs,
		VaultAuthTimeoutSecs:            defaultVaultAuthTimeoutSecs,
		RetryInitialIntervalMs:          defaultRetryInitialIntervalMs,
		RetryMaxIntervalSecs:            defaultRetryMaxIntervalSecs,
		RetryMaxElapsedTimeSecs:         defaultRetryMaxElapsedTimeSecs,
		RetryMultiplier:                 defaultRetryMultiplier,
		RetryRandomizationFactor:        defaultRetryRandomizationFactor,
		VaultPKIEnabled:                 defaultVaultPKIEnabled,
		VaultPKIMountPath:               defaultVaultPKIMountPath,
		VaultPKIRole:                    defaultVaultPKIRole,
		VaultPKITTL:                     defaultVaultPKITTL,
		VaultPKIRenewalBuffer:           defaultVaultPKIRenewalBuffer,
		PostgresqlRateLimitPerSecond:    defaultPostgresqlRateLimitPerSecond,
		PostgresqlRateLimitBurst:        defaultPostgresqlRateLimitBurst,
		VaultRateLimitPerSecond:         defaultVaultRateLimitPerSecond,
		VaultRateLimitBurst:             defaultVaultRateLimitBurst,
		LogLevel:                        defaultLogLevel,
	}
}

// PostgresqlConnectionTimeout returns the timeout duration for PostgreSQL connection retries
func (c *Config) PostgresqlConnectionTimeout() time.Duration {
	return time.Duration(c.PostgresqlConnectionTimeoutSecs) * time.Second
}

// VaultAvailabilityRetryDelay returns the delay duration for Vault availability retries
func (c *Config) VaultAvailabilityRetryDelay() time.Duration {
	return time.Duration(c.VaultAvailabilityRetryDelaySecs) * time.Second
}

// VaultAuthTimeout returns the timeout duration for Vault authentication
func (c *Config) VaultAuthTimeout() time.Duration {
	return time.Duration(c.VaultAuthTimeoutSecs) * time.Second
}

// RetryInitialInterval returns the initial interval for exponential backoff
func (c *Config) RetryInitialInterval() time.Duration {
	return time.Duration(c.RetryInitialIntervalMs) * time.Millisecond
}

// RetryMaxInterval returns the maximum interval for exponential backoff
func (c *Config) RetryMaxInterval() time.Duration {
	return time.Duration(c.RetryMaxIntervalSecs) * time.Second
}

// RetryMaxElapsedTime returns the maximum elapsed time for exponential backoff
func (c *Config) RetryMaxElapsedTime() time.Duration {
	return time.Duration(c.RetryMaxElapsedTimeSecs) * time.Second
}

func (c *Config) SetupWebhooksList() []string {
	return []string{
		c.K8sWebhookNamePostgresql,
		c.K8sWebhookNameUser,
		c.K8sWebhookNameDatabase,
		c.K8sWebhookNameGrant,
		c.K8sWebhookNameRoleGroup,
		c.K8sWebhookNameSchema,
	}
}

// VaultPKITTLDuration returns the TTL duration for Vault PKI certificates
// Returns the default TTL if parsing fails
func (c *Config) VaultPKITTLDuration() time.Duration {
	duration, err := time.ParseDuration(c.VaultPKITTL)
	if err != nil {
		// Return default TTL (30 days) if parsing fails
		return 720 * time.Hour
	}
	return duration
}

// VaultPKIRenewalBufferDuration returns the renewal buffer duration for Vault PKI certificates
// Returns the default buffer if parsing fails
func (c *Config) VaultPKIRenewalBufferDuration() time.Duration {
	duration, err := time.ParseDuration(c.VaultPKIRenewalBuffer)
	if err != nil {
		// Return default buffer (24 hours) if parsing fails
		return 24 * time.Hour
	}
	return duration
}

// Validate validates the configuration and returns an error if any required fields are invalid
func (c *Config) Validate() error {
	var validationErrors []string

	// Validate port ranges
	if c.WebhookServerPort < 1 || c.WebhookServerPort > 65535 {
		validationErrors = append(validationErrors,
			"webhook server port must be between 1 and 65535")
	}

	// Validate retry configuration
	if c.PostgresqlConnectionRetries < 0 {
		validationErrors = append(validationErrors,
			"postgresql connection retries must be non-negative")
	}

	if c.PostgresqlConnectionTimeoutSecs < 0 {
		validationErrors = append(validationErrors,
			"postgresql connection timeout must be non-negative")
	}

	if c.VaultAvailabilityRetries < 0 {
		validationErrors = append(validationErrors,
			"vault availability retries must be non-negative")
	}

	if c.VaultAvailabilityRetryDelaySecs < 0 {
		validationErrors = append(validationErrors,
			"vault availability retry delay must be non-negative")
	}

	if c.VaultAuthTimeoutSecs < 0 {
		validationErrors = append(validationErrors,
			"vault auth timeout must be non-negative")
	}

	// Validate rate limiting configuration
	if c.PostgresqlRateLimitPerSecond <= 0 {
		validationErrors = append(validationErrors,
			"postgresql rate limit per second must be positive")
	}

	if c.PostgresqlRateLimitBurst <= 0 {
		validationErrors = append(validationErrors,
			"postgresql rate limit burst must be positive")
	}

	if c.VaultRateLimitPerSecond <= 0 {
		validationErrors = append(validationErrors,
			"vault rate limit per second must be positive")
	}

	if c.VaultRateLimitBurst <= 0 {
		validationErrors = append(validationErrors,
			"vault rate limit burst must be positive")
	}

	// Validate exponential backoff configuration
	if c.RetryInitialIntervalMs < 0 {
		validationErrors = append(validationErrors,
			"retry initial interval must be non-negative")
	}

	if c.RetryMaxIntervalSecs < 0 {
		validationErrors = append(validationErrors,
			"retry max interval must be non-negative")
	}

	if c.RetryMaxElapsedTimeSecs < 0 {
		validationErrors = append(validationErrors,
			"retry max elapsed time must be non-negative")
	}

	if c.RetryMultiplier < 1.0 {
		validationErrors = append(validationErrors,
			"retry multiplier must be at least 1.0")
	}

	if c.RetryRandomizationFactor < 0 || c.RetryRandomizationFactor > 1 {
		validationErrors = append(validationErrors,
			"retry randomization factor must be between 0 and 1")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "warning": true, "error": true,
	}
	if c.LogLevel != "" && !validLogLevels[c.LogLevel] {
		validationErrors = append(validationErrors,
			"log level must be one of: debug, info, warn, error")
	}

	// Validate Vault PKI TTL and renewal buffer if PKI is enabled
	if c.VaultPKIEnabled {
		if _, err := time.ParseDuration(c.VaultPKITTL); err != nil {
			validationErrors = append(validationErrors,
				"vault PKI TTL must be a valid duration (e.g., 720h)")
		}
		if _, err := time.ParseDuration(c.VaultPKIRenewalBuffer); err != nil {
			validationErrors = append(validationErrors,
				"vault PKI renewal buffer must be a valid duration (e.g., 24h)")
		}
	}

	if len(validationErrors) > 0 {
		return &ConfigValidationError{Errors: validationErrors}
	}

	return nil
}

// ConfigValidationError represents configuration validation errors
type ConfigValidationError struct {
	Errors []string
}

// Error implements the error interface
func (e *ConfigValidationError) Error() string {
	if len(e.Errors) == 1 {
		return "configuration validation error: " + e.Errors[0]
	}
	result := "configuration validation errors:"
	for _, err := range e.Errors {
		result += "\n  - " + err
	}
	return result
}
