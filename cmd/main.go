/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/logging"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/go-logr/zapr"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller"
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/metrics"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
	webhookpkg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(instancev1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// setupWebhookServer creates and configures the webhook server
func setupWebhookServer(
	cfg *config.Config, tlsOpts []func(*tls.Config),
	lg *zap.SugaredLogger, vaultClient *vault.Client,
) (webhook.Server, *certwatcher.CertWatcher) {
	webhookTLSOpts := tlsOpts
	webhookNames := cfg.SetupWebhooksList()

	webhookCertPath, webhookKeyPath := resolveWebhookCerts(
		cfg, vaultClient, webhookNames, lg,
	)

	webhookCertWatcher, err := certwatcher.New(
		webhookCertPath, webhookKeyPath,
	)
	if err != nil {
		lg.Errorw("Failed to initialize webhook certificate watcher",
			"error", err)
		os.Exit(1)
	}

	webhookTLSOpts = append(webhookTLSOpts, func(c *tls.Config) {
		c.GetCertificate = webhookCertWatcher.GetCertificate
	})

	webhookServer := webhook.NewServer(webhook.Options{
		Host:    cfg.WebhookServerAddr,
		Port:    cfg.WebhookServerPort,
		TLSOpts: webhookTLSOpts,
	})
	lg.Infow("Webhook server configured",
		"host", cfg.WebhookServerAddr,
		"port", cfg.WebhookServerPort,
		"cert-path", webhookCertPath,
		"key-path", webhookKeyPath)

	return webhookServer, webhookCertWatcher
}

// resolveWebhookCerts determines the certificate paths based on config
func resolveWebhookCerts(
	cfg *config.Config, vaultClient *vault.Client,
	webhookNames []string, lg *zap.SugaredLogger,
) (certPath, keyPath string) {
	if cfg.VaultPKIEnabled && vaultClient != nil &&
		cfg.WebhookCertPath == "" {
		return setupVaultPKICerts(cfg, vaultClient, webhookNames, lg)
	}
	return k8sclient.SetupWebhookCertificates(context.Background(), cfg, webhookNames, lg)
}

// setupVaultPKICerts creates a VaultPKIManager and issues certificates
func setupVaultPKICerts(
	cfg *config.Config, vaultClient *vault.Client,
	webhookNames []string, lg *zap.SugaredLogger,
) (certPath, keyPath string) {
	renewalBuffer, err := time.ParseDuration(cfg.VaultPKIRenewalBuffer)
	if err != nil {
		lg.Errorw("Failed to parse Vault PKI renewal buffer",
			"value", cfg.VaultPKIRenewalBuffer, "error", err)
		os.Exit(1)
	}

	namespace := k8sclient.GetCurrentNamespace(cfg.K8sNamespacePath)

	tempCertDir, err := os.MkdirTemp("", "vault-pki-certs-")
	if err != nil {
		lg.Errorw("Failed to create temp dir for Vault PKI certs",
			"error", err)
		os.Exit(1)
	}

	pkiManager := cert.NewVaultPKIManager(
		vaultClient, cfg.VaultPKIMountPath, cfg.VaultPKIRole,
		cfg.VaultPKITTL, renewalBuffer,
		cfg.WebhookK8sServiceName, namespace, tempCertDir, lg,
	)

	lg.Infow("Using Vault PKI for webhook certificates",
		"mount-path", cfg.VaultPKIMountPath,
		"role", cfg.VaultPKIRole,
		"ttl", cfg.VaultPKITTL,
		"renewal-buffer", cfg.VaultPKIRenewalBuffer,
	)

	certPath, keyPath = k8sclient.SetupWebhookCertificatesWithVaultPKI(
		context.Background(), pkiManager, cfg, webhookNames, lg,
	)

	pkiManager.StartRenewal(context.Background())

	return certPath, keyPath
}

// initVaultClient initializes the Vault client if configured
func initVaultClient(
	cfg *config.Config, lg *zap.SugaredLogger,
) *vault.Client {
	if cfg.VaultAddr == "" {
		lg.Infow("VAULT_ADDR not set, Vault integration disabled")
		return nil
	}

	vc, err := vault.NewClient(
		context.Background(), cfg.VaultAddr, cfg.VaultRole, cfg.K8sTokenPath,
		cfg.VaultMountPoint, cfg.VaultSecretPath,
	)
	if err != nil {
		lg.Errorw("unable to create Vault client", "error", err)
		os.Exit(1)
	}
	lg.Infow("Vault client initialized successfully")
	return vc
}

// parseExcludeUserList parses the exclude user list from config
func parseExcludeUserList(
	cfg *config.Config, lg *zap.SugaredLogger,
) []string {
	excludeUserList := []string{}
	if cfg.ExcludeUserList != "" {
		excludeUserList = strings.Split(cfg.ExcludeUserList, ",")
		for i, user := range excludeUserList {
			excludeUserList[i] = strings.TrimSpace(user)
		}
		lg.Infow("Exclude user list configured", "users", excludeUserList)
	}
	return excludeUserList
}

// startPeriodicMetrics starts periodic metrics collection
func startPeriodicMetrics(
	ctx context.Context, collector *metrics.Collector, lg *zap.SugaredLogger, interval time.Duration,
) {
	if err := collector.CollectMetrics(ctx); err != nil {
		lg.Errorw("Failed to collect initial metrics", "error", err)
	} else {
		lg.Infow("Initial metrics collection completed")
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := collector.CollectMetrics(ctx); err != nil {
					lg.Errorw("Failed to collect metrics", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// nolint:gocyclo // main function orchestrates multiple setup steps which requires complex control flow
func main() {
	cfg := config.New()
	ConfigParser(cfg)

	lg := logging.NewLogging(zapcore.InfoLevel)

	// Set the logger for controller-runtime and klog
	zapLogger := lg.Desugar()
	ctrl.SetLogger(zapr.NewLogger(zapLogger))
	klog.SetLogger(zapr.NewLogger(zapLogger))

	lg.Infow("Server starting with: ")

	// Disable http/2 due to vulnerabilities (CVE mitigations)
	tlsOpts := []func(*tls.Config){
		func(c *tls.Config) {
			lg.Infow("disabling http/2")
			c.NextProtos = []string{"http/1.1"}
		},
	}

	// Initialize Vault client before webhook server setup
	// so Vault PKI can be used for webhook certificates
	vaultClient := initVaultClient(cfg, lg)

	webhookServer, webhookCertWatcher := setupWebhookServer(
		cfg, tlsOpts, lg, vaultClient,
	)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: ":8080"},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.ProbeAddr,
		LeaderElection:         cfg.EnableLeaderElection,
		LeaderElectionID: "9e7b1538.postgresql-operator" +
			".vyrodovalexey.github.com",
	})
	if err != nil {
		lg.Errorw("unable to start manager", "error", err)
		os.Exit(1)
	}

	if err := controller.SetupControllers(
		mgr, cfg, vaultClient, lg,
	); err != nil {
		lg.Errorw("unable to setup controllers", "error", err)
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if webhookCertWatcher != nil {
		lg.Infow("Adding webhook certificate watcher to postgresql operator")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			lg.Errorw("unable to add webhook certificate watcher to manager", "error", err)
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		lg.Errorw("unable to set up health check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		lg.Errorw("unable to set up ready check", "error", err)
		os.Exit(1)
	}

	webhookDecoder := admission.NewDecoder(scheme)
	excludeUserList := parseExcludeUserList(cfg, lg)

	if err := webhookpkg.RegisterWebhooks(
		mgr, webhookServer, webhookDecoder,
		cfg, vaultClient, excludeUserList, lg,
	); err != nil {
		lg.Errorw("unable to register webhooks", "error", err)
		os.Exit(1)
	}

	metricsCollector := metrics.NewCollector(mgr.GetClient(), lg)
	ctx := ctrl.SetupSignalHandler()
	startPeriodicMetrics(ctx, metricsCollector, lg, cfg.MetricsCollectionInterval())

	if err := mgr.Start(ctx); err != nil {
		lg.Errorw("Problem starting postgresql operator", "error", err)
		os.Exit(1)
	}
}
