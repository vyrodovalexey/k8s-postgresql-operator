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
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/metrics"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
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

// nolint:gocyclo // main function orchestrates multiple setup steps which requires complex control flow
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

	// Define webhooks
	webhookNames := cfg.SetupWebhooksList()
	// Setup webhook certificates
	webhookCertPath, webhookKeyPath := setupWebhookCertificates(cfg, webhookNames, lg)

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
	if err := setupControllers(mgr, cfg, vaultClient, lg); err != nil {
		lg.Error(err, "unable to setup controllers")
		os.Exit(1)
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

	// Register webhooks
	if err := registerWebhooks(mgr, webhookServer, webhookDecoder, cfg, vaultClient, excludeUserList, lg); err != nil {
		lg.Error(err, "unable to register webhooks")
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
