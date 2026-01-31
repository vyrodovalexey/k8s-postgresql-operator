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
	"strings"
	"time"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/constants"
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

// nolint:gocyclo,funlen // main function orchestrates multiple setup steps which requires complex control flow
func main() {
	// Create a new configuration instance
	cfg := config.New()
	// Parse configuration settings - fatal on error
	if err := ConfigParser(cfg); err != nil {
		// Use standard log since logger is not yet initialized
		fmt.Fprintf(os.Stderr, "Fatal: failed to parse configuration: %v\n", err)
		os.Exit(1)
	}

	// Create root context that will be cancelled on shutdown
	// This context is used throughout the application for graceful shutdown
	// Created early so it can be used in all initialization steps
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tlsOpts := make([]func(*tls.Config), 0, 1)

	// Create logger with configurable log level
	lg := logging.NewLoggingFromString(cfg.LogLevel)

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

	// Discover current namespace
	namespace := k8sclient.GetCurrentNamespace(cfg.K8sNamespacePath)

	// Create certificate provider factory
	providerFactory, err := cert.NewProviderFactory(cert.ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: ctrl.GetConfigOrDie(),
		Logger:     lg,
		Namespace:  namespace,
	})
	if err != nil {
		lg.Errorw("Failed to create certificate provider factory", "error", err)
		os.Exit(1) //nolint:gocritic // exitAfterDefer: context cancellation not critical on fatal error
	}

	// Log the provider type that will be used
	lg.Infow("Certificate provider type determined",
		"provider_type", providerFactory.DetermineProviderType())

	// Create the certificate provider with context for proper cancellation handling
	certProvider, err := providerFactory.CreateProviderWithContext(ctx)
	if err != nil {
		lg.Errorw("Failed to create certificate provider", "error", err)
		os.Exit(1)
	}
	lg.Infow("Certificate provider initialized", "type", certProvider.Type())

	// Setup webhook certificates using the provider
	webhookCertPath, webhookKeyPath, err := k8sclient.SetupWebhookCertificatesWithProvider(
		ctx, cfg, webhookNames, certProvider, lg)
	if err != nil {
		lg.Errorw("Failed to setup webhook certificates", "error", err)
		os.Exit(1)
	}

	// Initialize webhook certificate watcher
	webhookCertWatcher, err = certwatcher.New(webhookCertPath, webhookKeyPath)
	if err != nil {
		lg.Errorw("Failed to initialize webhook certificate watcher", "error", err)
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
		vaultOpts := vault.ClientOptions{
			AuthTimeout: cfg.VaultAuthTimeout(),
		}
		lg.Infow("Initializing Vault client",
			"address", cfg.VaultAddr,
			"authTimeout", cfg.VaultAuthTimeout())
		vc, err := vault.NewClientWithOptions(
			ctx, cfg.VaultAddr, cfg.VaultRole, cfg.K8sTokenPath, cfg.VaultMountPoint, cfg.VaultSecretPath, vaultOpts)
		if err != nil {
			lg.Errorw("unable to create Vault client", "error", err)
			os.Exit(1)
		}
		vaultClient = vc
		lg.Infow("Vault client initialized successfully")
	} else {
		lg.Infow("VAULT_ADDR not set, Vault integration disabled")
	}

	// Setup all controllers
	if err := controller.SetupControllers(mgr, cfg, vaultClient, lg); err != nil {
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
	if err := webhookpkg.RegisterWebhooks(
		mgr, webhookServer, webhookDecoder, cfg, vaultClient, excludeUserList, lg,
	); err != nil {
		lg.Error(err, "unable to register webhooks")
		os.Exit(1)
	}

	// Initialize metrics collector
	metricsCollector := metrics.NewCollector(mgr.GetClient(), lg)

	// Collect metrics on startup using the root context
	if err := metricsCollector.CollectMetrics(ctx); err != nil {
		lg.Errorw("failed to collect initial metrics", "error", err)
		// Don't exit, metrics collection failure shouldn't prevent operator from starting
	} else {
		lg.Infow("Initial metrics collection completed")
	}

	// Set up periodic metrics collection
	// Use a child context derived from the root context for graceful shutdown
	metricsCtx, metricsCancel := context.WithCancel(ctx)
	go func() {
		defer metricsCancel()
		ticker := time.NewTicker(constants.DefaultMetricsCollectionInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := metricsCollector.CollectMetrics(metricsCtx); err != nil {
					lg.Errorw("failed to collect metrics", "error", err)
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
