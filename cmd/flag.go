package main

import (
	"flag"
	"log"

	"github.com/caarlos0/env/v6"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
)

func ConfigParser(cfg *config.Config) {

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
	flag.Parse() // Парсим флаги командной строки

	// Парсим переменные окружения и сохраняем их в конфигурацию и перезаписывая существующие
	err := env.Parse(cfg)

	if err != nil {

		log.Printf("can't parse ENV: %v", err)
	}
}
