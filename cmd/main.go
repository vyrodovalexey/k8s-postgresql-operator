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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/logging"
	"go.uber.org/zap/zapcore"

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
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
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

// nolint:gocyclo
func main() {
	// Создаем новый экземпляр конфигурации
	cfg := config.New()
	// Парсим настройки конфигурации
	ConfigParser(cfg)

	var tlsOpts []func(*tls.Config)

	lg := logging.NewLogging(zapcore.InfoLevel)

	// Set the logger for controller-runtime
	// Convert SugaredLogger to Logger and wrap it for controller-runtime
	zapLogger := lg.Desugar()
	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	// Configure klog to use JSON format by redirecting to zap logger
	// This ensures leader election logs are also in JSON format
	klog.SetLogger(zapr.NewLogger(zapLogger))

	lg.Infow("Server starting with: ")

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		lg.Infow("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts = append(tlsOpts, disableHTTP2)

	// Create watcher for webhook certificates
	var webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	// Generate or use provided webhook certificates
	var webhookCertPath, webhookKeyPath string
	if cfg.WebhookCertPath != "" {
		// Use provided certificates
		webhookCertPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertName)
		webhookKeyPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertKey)
		lg.Infow("Using provided webhook certificates",
			"webhook-cert-path", cfg.WebhookCertPath,
			"webhook-cert-name", cfg.WebhookCertName,
			"webhook-cert-key", cfg.WebhookCertKey)
	} else {
		// Generate self-signed certificates and store in Kubernetes Secret
		tempCertDir, err := os.MkdirTemp("", "webhook-certs-")
		if err != nil {
			lg.Error(err, "Failed to create temporary directory for webhook certificates")
			os.Exit(1)
		}
		lg.Infow("Generating self-signed webhook certificates and storing in Secret", "cert-dir", tempCertDir)

		// Get service name from environment or use default

		// Discover current namespace
		namespace := k8s.GetCurrentNamespace(cfg.K8sNamespacePath)

		secretName := fmt.Sprintf("%s-webhook-cert", cfg.WebhookK8sServiceName)

		// Get Kubernetes config
		k8sConfig := ctrl.GetConfigOrDie()
		_, webhookCertPath, webhookKeyPath, err = cert.GenerateSelfSignedCertAndStoreInSecret(
			cfg.WebhookK8sServiceName, namespace, secretName, tempCertDir, k8sConfig)
		if err != nil {
			lg.Error(err, "Failed to generate self-signed webhook certificates")
			os.Exit(1)
		}
		lg.Infow("Webhook certificates generated and stored in Secret",
			"secret-name", secretName,
			"cert-path", webhookCertPath,
			"key-path", webhookKeyPath)

		// Update all ValidatingWebhookConfigurations with CA bundle from secret
		webhookNames := []string{
			cfg.K8sWebhookNamePostgresql,
			cfg.K8sWebhookNameUser,
			cfg.K8sWebhookNameDatabase,
			cfg.K8sWebhookNameGrant,
			cfg.K8sWebhookNameRoleGroup,
			cfg.K8sWebhookNameSchema,
		}

		for _, webhookName := range webhookNames {
			if err := k8s.UpdateWebhookConfigurationWithCABundle(
				secretName, namespace, k8sConfig, webhookName, lg); err != nil {
				lg.Error(err, "Failed to update ValidatingWebhookConfiguration with CA bundle",
					"secret-name", secretName,
					"webhook-name", webhookName)
				// Don't exit, as the webhook might still work if the CA bundle is set manually
			} else {
				lg.Infow("Successfully updated ValidatingWebhookConfiguration with CA bundle",
					"secret-name", secretName,
					"webhook-name", webhookName)
			}
		}
	}

	// Initialize webhook certificate watcher
	var err error
	webhookCertWatcher, err = certwatcher.New(webhookCertPath, webhookKeyPath)
	if err != nil {
		lg.Error(err, "Failed to initialize webhook certificate watcher")
		os.Exit(1)
	}

	webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
		config.GetCertificate = webhookCertWatcher.GetCertificate
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

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: ":8080"}, // Enable metrics server on port 8080
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.ProbeAddr,
		LeaderElection:         cfg.EnableLeaderElection,
		LeaderElectionID:       "9e7b1538.postgresql-operator.vyrodovalexey.github.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		lg.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize Vault client if environment variables are set
	var vaultClient *vault.Client
	if cfg.VaultAddr != "" {
		vc, err := vault.NewClient(cfg.VaultAddr, cfg.VaultRole, cfg.K8sTokenPath, cfg.VaultMountPoint, cfg.VaultSecretPath)
		if err != nil {
			lg.Error(err, "unable to create Vault client")
			os.Exit(1)
		} else {
			vaultClient = vc
			lg.Infow("Vault client initialized successfully")
		}
	} else {
		lg.Infow("VAULT_ADDR not set, Vault integration disabled")
	}

	// Setup all controllers
	type controllerSetup struct {
		name  string
		setup func() error
	}

	controllerSetups := []controllerSetup{
		{
			name: "Postgresql",
			setup: func() error {
				return (&controller.PostgresqlReconciler{
					Client:      mgr.GetClient(),
					Scheme:      mgr.GetScheme(),
					VaultClient: vaultClient,
					Log:         lg,
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "User",
			setup: func() error {
				return (&controller.UserReconciler{
					Client:      mgr.GetClient(),
					Scheme:      mgr.GetScheme(),
					VaultClient: vaultClient,
					Log:         lg,
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "Database",
			setup: func() error {
				return (&controller.DatabaseReconciler{
					Client:      mgr.GetClient(),
					Scheme:      mgr.GetScheme(),
					VaultClient: vaultClient,
					Log:         lg,
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "Grant",
			setup: func() error {
				return (&controller.GrantReconciler{
					Client:      mgr.GetClient(),
					Scheme:      mgr.GetScheme(),
					VaultClient: vaultClient,
					Log:         lg,
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "RoleGroup",
			setup: func() error {
				return (&controller.RoleGroupReconciler{
					Client:      mgr.GetClient(),
					Scheme:      mgr.GetScheme(),
					VaultClient: vaultClient,
					Log:         lg,
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "Schema",
			setup: func() error {
				return (&controller.SchemaReconciler{
					Client:      mgr.GetClient(),
					Scheme:      mgr.GetScheme(),
					VaultClient: vaultClient,
					Log:         lg,
				}).SetupWithManager(mgr)
			},
		},
	}

	for _, ctrlSetup := range controllerSetups {
		if err := ctrlSetup.setup(); err != nil {
			lg.Error(err, "unable to create controller", "controller", ctrlSetup.name)
			os.Exit(1)
		}
	}

	// +kubebuilder:scaffold:builder

	if webhookCertWatcher != nil {
		lg.Infow("Adding webhook certificate watcher to postgresql operator")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			lg.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		lg.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		lg.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	// Register webhook handlers
	webhookDecoder := admission.NewDecoder(scheme)

	// Parse exclude user list from comma-separated string (used for UserValidator)
	excludeUserList := []string{}
	if cfg.ExcludeUserList != "" {
		excludeUserList = strings.Split(cfg.ExcludeUserList, ",")
		// Trim whitespace from each username
		for i, user := range excludeUserList {
			excludeUserList[i] = strings.TrimSpace(user)
		}
		lg.Infow("Exclude user list configured", "users", excludeUserList)
	}

	// Define webhook registration configs
	type webhookConfig struct {
		path    string
		handler admission.Handler
		name    string
	}

	webhookConfigs := []webhookConfig{
		{
			path: "/uservalidate",
			handler: &webhookpkg.UserValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				ExcludeUserList:             excludeUserList,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "uservalidate",
		},
		{
			path: "/postgresqlvalidate",
			handler: &webhookpkg.PostgresqlValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "postgresqlvalidate",
		},
		{
			path: "/databasevalidate",
			handler: &webhookpkg.DatabaseValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "databasevalidate",
		},
		{
			path: "/grantvalidate",
			handler: &webhookpkg.GrantValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "grantvalidate",
		},
		{
			path: "/rolegroupvalidate",
			handler: &webhookpkg.RoleGroupValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "rolegroupvalidate",
		},
		{
			path: "/schemavalidate",
			handler: &webhookpkg.SchemaValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "schemavalidate",
		},
	}

	// Register all webhook handlers
	for _, cfg := range webhookConfigs {
		webhookServer.Register(cfg.path, &webhook.Admission{Handler: cfg.handler})
		lg.Infow("Registered webhook handler", "path", cfg.path, "name", cfg.name)
	}

	if err := mgr.Add(webhookServer); err != nil {
		lg.Error(err, "unable to set up webhook server")
		os.Exit(1)
	}

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector(mgr.GetClient(), lg)

	// Collect metrics on startup
	startupCtx := context.Background()
	if err := metricsCollector.CollectMetrics(startupCtx); err != nil {
		lg.Error(err, "failed to collect initial metrics")
		// Don't exit, metrics collection failure shouldn't prevent operator from starting
	} else {
		lg.Infow("Initial metrics collection completed")
	}

	// Set up periodic metrics collection (every 30 seconds)
	// Use a context that will be cancelled when the manager stops
	metricsCtx, metricsCancel := context.WithCancel(context.Background())
	go func() {
		defer metricsCancel()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := metricsCollector.CollectMetrics(metricsCtx); err != nil {
					lg.Error(err, "failed to collect metrics")
				}
			case <-metricsCtx.Done():
				return
			}
		}
	}()

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		lg.Error(err, "problem starting postgresql operator")
		os.Exit(1)
	}
}
