package main

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v6"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
)

// ConfigParser parses configuration from command line flags and environment variables.
// It returns an error if parsing or validation fails.
func ConfigParser(cfg *config.Config) error {
	flag.StringVar(&cfg.ProbeAddr, "health-probe-bind-address", cfg.ProbeAddr, "The address the probe endpoint binds to.")
	flag.BoolVar(&cfg.EnableLeaderElection, "leader-elect", cfg.EnableLeaderElection,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&cfg.WebhookCertPath, "webhook-cert-path", cfg.WebhookCertPath,
		"The directory that contains the webhook certificate.")
	flag.StringVar(&cfg.WebhookCertName, "webhook-cert-name", cfg.WebhookCertName,
		"The name of the webhook certificate file.")
	flag.StringVar(&cfg.WebhookCertKey, "webhook-cert-key", cfg.WebhookCertKey,
		"The name of the webhook key file.")
	flag.IntVar(&cfg.WebhookServerPort, "webhook-server-port", cfg.WebhookServerPort,
		"Webhook server port ")
	flag.StringVar(&cfg.WebhookServerAddr, "webhook-server-addr", cfg.WebhookServerAddr,
		"Webhook server address")
	flag.StringVar(&cfg.WebhookK8sServiceName, "webhook-k8s-service-name",
		cfg.WebhookK8sServiceName, "Webhook k8s service name, using to create certificate")
	flag.StringVar(&cfg.K8sWebhookNamePostgresql, "k8s-webhook-name-postgresql",
		cfg.K8sWebhookNamePostgresql,
		"Name of webhook in k8s for postgresql kind, using to add cabundle")
	flag.StringVar(&cfg.K8sWebhookNameUser, "k8s-webhook-name-user",
		cfg.K8sWebhookNameUser, "Name of webhook in k8s for user kind, using to add cabundle")
	flag.StringVar(&cfg.K8sWebhookNameDatabase, "k8s-webhook-name-database",
		cfg.K8sWebhookNameDatabase, "Name of webhook in k8s for database kind, using to add cabundle")
	flag.StringVar(&cfg.K8sWebhookNameGrant, "k8s-webhook-name-grant",
		cfg.K8sWebhookNameGrant, "Name of webhook in k8s for grant kind, using to add cabundle")
	flag.StringVar(&cfg.K8sWebhookNameRoleGroup, "k8s-webhook-name-rolegroup",
		cfg.K8sWebhookNameRoleGroup, "Name of webhook in k8s for rolegroup kind, using to add cabundle")
	flag.StringVar(&cfg.K8sWebhookNameSchema, "k8s-webhook-name-schema",
		cfg.K8sWebhookNameSchema, "Name of webhook in k8s for schema kind, using to add cabundle")
	flag.StringVar(&cfg.VaultAddr, "vault-addr", cfg.VaultAddr, "Vault addr, example http://0.0.0.0:8200")
	flag.StringVar(&cfg.VaultRole, "vault-role", cfg.VaultRole, "Vault role name")
	flag.StringVar(&cfg.VaultMountPoint, "vault-mount-point", cfg.VaultMountPoint, "KV V2 Name")
	flag.StringVar(&cfg.VaultSecretPath, "vault-secret-path", cfg.VaultSecretPath, "prefix path")
	flag.StringVar(&cfg.K8sTokenPath, "k8s-token-path", cfg.K8sTokenPath, "path to k8s SA token mounted in container")
	flag.StringVar(&cfg.K8sNamespacePath, "k8s-namespace-path", cfg.K8sNamespacePath,
		"path to k8s namespace file mounted in container")
	flag.StringVar(&cfg.ExcludeUserList, "exclude-user-list", cfg.ExcludeUserList,
		"block creation or changing password for postgresql users list, "+
			"use comma for delimit users, default 'postgres'")
	flag.IntVar(&cfg.PostgresqlConnectionRetries, "postgresql-connection-retries",
		cfg.PostgresqlConnectionRetries, "Number of retries for PostgreSQL connection test (default: 3)")
	flag.IntVar(&cfg.PostgresqlConnectionTimeoutSecs, "postgresql-connection-timeout-secs",
		cfg.PostgresqlConnectionTimeoutSecs, "Timeout in seconds between PostgreSQL connection retries (default: 10)")
	flag.IntVar(&cfg.VaultAvailabilityRetries, "vault-availability-retries",
		cfg.VaultAvailabilityRetries, "Number of retries for Vault availability check (default: 3)")
	flag.IntVar(&cfg.VaultAvailabilityRetryDelaySecs, "vault-availability-retry-delay-secs",
		cfg.VaultAvailabilityRetryDelaySecs, "Delay in seconds between Vault availability retries (default: 10)")
	flag.IntVar(&cfg.VaultAuthTimeoutSecs, "vault-auth-timeout-secs",
		cfg.VaultAuthTimeoutSecs, "Timeout in seconds for Vault authentication (default: 30)")

	// Exponential backoff configuration flags
	flag.IntVar(&cfg.RetryInitialIntervalMs, "retry-initial-interval-ms",
		cfg.RetryInitialIntervalMs, "Initial interval in milliseconds for exponential backoff (default: 500)")
	flag.IntVar(&cfg.RetryMaxIntervalSecs, "retry-max-interval-secs",
		cfg.RetryMaxIntervalSecs, "Maximum interval in seconds for exponential backoff (default: 30)")
	flag.IntVar(&cfg.RetryMaxElapsedTimeSecs, "retry-max-elapsed-time-secs",
		cfg.RetryMaxElapsedTimeSecs, "Maximum elapsed time in seconds for all retries (default: 300)")
	flag.Float64Var(&cfg.RetryMultiplier, "retry-multiplier",
		cfg.RetryMultiplier, "Multiplier for exponential backoff interval growth (default: 2.0)")
	flag.Float64Var(&cfg.RetryRandomizationFactor, "retry-randomization-factor",
		cfg.RetryRandomizationFactor, "Randomization factor (jitter) for exponential backoff (default: 0.5)")

	// Vault PKI flags for webhook certificates
	flag.BoolVar(&cfg.VaultPKIEnabled, "vault-pki-enabled", cfg.VaultPKIEnabled,
		"Enable Vault PKI for webhook certificates")
	flag.StringVar(&cfg.VaultPKIMountPath, "vault-pki-mount-path", cfg.VaultPKIMountPath,
		"Vault PKI secrets engine mount path (default: pki)")
	flag.StringVar(&cfg.VaultPKIRole, "vault-pki-role", cfg.VaultPKIRole,
		"Vault PKI role name for certificate issuance (default: webhook-cert)")
	flag.StringVar(&cfg.VaultPKITTL, "vault-pki-ttl", cfg.VaultPKITTL,
		"TTL for Vault PKI issued certificates (default: 720h)")
	flag.StringVar(&cfg.VaultPKIRenewalBuffer, "vault-pki-renewal-buffer", cfg.VaultPKIRenewalBuffer,
		"Time before expiry to trigger certificate renewal (default: 24h)")

	// Rate limiting flags
	flag.Float64Var(&cfg.PostgresqlRateLimitPerSecond, "postgresql-rate-limit-per-second",
		cfg.PostgresqlRateLimitPerSecond, "Rate limit for PostgreSQL connections per second (default: 10)")
	flag.IntVar(&cfg.PostgresqlRateLimitBurst, "postgresql-rate-limit-burst",
		cfg.PostgresqlRateLimitBurst, "Burst limit for PostgreSQL connections (default: 20)")
	flag.Float64Var(&cfg.VaultRateLimitPerSecond, "vault-rate-limit-per-second",
		cfg.VaultRateLimitPerSecond, "Rate limit for Vault requests per second (default: 10)")
	flag.IntVar(&cfg.VaultRateLimitBurst, "vault-rate-limit-burst",
		cfg.VaultRateLimitBurst, "Burst limit for Vault requests (default: 20)")

	// Log level flag
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel,
		"Log level: debug, info, warn, error (default: info)")

	flag.Parse() // Parse command line flags

	// Parse environment variables and save them to configuration, overwriting existing values
	if err := env.Parse(cfg); err != nil {
		return fmt.Errorf("failed to parse environment variables: %w", err)
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}
