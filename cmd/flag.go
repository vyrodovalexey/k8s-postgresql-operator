package main

import (
	"flag"
	"github.com/caarlos0/env/v6"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"log"
)

func ConfigParser(cfg *config.Config) {

	flag.StringVar(&cfg.MetricsAddr, "metrics-bind-address", cfg.MetricsAddr, "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&cfg.ProbeAddr, "health-probe-bind-address", cfg.ProbeAddr, "The address the probe endpoint binds to.")
	flag.BoolVar(&cfg.EnableLeaderElection, "leader-elect", cfg.EnableLeaderElection,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&cfg.SecureMetrics, "metrics-secure", cfg.SecureMetrics,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.StringVar(&cfg.WebhookCertPath, "webhook-cert-path", cfg.WebhookCertPath, "The directory that contains the webhook certificate.")
	flag.StringVar(&cfg.WebhookCertName, "webhook-cert-name", cfg.WebhookCertName, "The name of the webhook certificate file.")
	flag.StringVar(&cfg.WebhookCertKey, "webhook-cert-key", cfg.WebhookCertKey, "The name of the webhook key file.")
	flag.StringVar(&cfg.MetricsCertPath, "metrics-cert-path", cfg.MetricsCertPath,
		"The directory that contains the metrics server certificate.")
	flag.StringVar(&cfg.MetricsCertName, "metrics-cert-name", cfg.MetricsCertName, "The name of the metrics server certificate file.")
	flag.StringVar(&cfg.MetricsCertKey, "metrics-cert-key", cfg.MetricsCertKey, "The name of the metrics server key file.")
	flag.BoolVar(&cfg.EnableHTTP2, "enable-http2", cfg.EnableHTTP2,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&cfg.VaultAddr, "vault-addr", cfg.VaultAddr, "Vault addr, example http://0.0.0.0:8200")
	flag.StringVar(&cfg.VaultRole, "vault-role", cfg.VaultRole, "Vault role name")
	flag.StringVar(&cfg.VaultMountPoint, "vault-mount-point", cfg.VaultMountPoint, "KV V2 Name")
	flag.StringVar(&cfg.VaultSecretPath, "vault-secret-path", cfg.VaultSecretPath, "prefix path")
	flag.StringVar(&cfg.VaultK8sTokenPath, "vault-k8s-token-path", cfg.VaultK8sTokenPath, "path to k8s SA token mounted in container")

	flag.Parse() // Парсим флаги командной строки

	// Парсим переменные окружения и сохраняем их в конфигурацию и перезаписывая существующие
	err := env.Parse(cfg)

	if err != nil {

		log.Printf("can't parse ENV: %v", err)
	}

}
