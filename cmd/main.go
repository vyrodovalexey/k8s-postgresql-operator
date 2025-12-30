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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/logging"
	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller"
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

// getCurrentNamespace discovers the current namespace from the service account
// Returns the namespace or a default value if not found
func getCurrentNamespace() string {
	// Try to read from the service account namespace file (standard in Kubernetes pods)
	namespaceFile := "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	if data, err := os.ReadFile(namespaceFile); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}

	// Fallback to environment variable
	if ns := os.Getenv("NAMESPACE"); ns != "" {
		return ns
	}

	// Default fallback
	return "k8s-postgresql-operator"
}

// updateWebhookConfigurationWithCABundle updates the ValidatingWebhookConfiguration with the CA bundle from the secret
func updateWebhookConfigurationWithCABundle(secretName, namespace string, config *rest.Config, lg *zap.SugaredLogger) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	ctx := context.Background()

	// Get the secret containing the certificate
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	certData, ok := secret.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("tls.crt not found in secret %s", secretName)
	}

	// Encode certificate as base64 for CA bundle
	caBundle := base64.StdEncoding.EncodeToString(certData)

	// Get the ValidatingWebhookConfiguration
	webhookConfigName := "k8s-postgresql-operator-validating-webhook-configuration"
	webhookConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, webhookConfigName, metav1.GetOptions{})
	if err != nil {
		// Try alternative name
		webhookConfigName = "validating-webhook-configuration"
		webhookConfig, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, webhookConfigName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get ValidatingWebhookConfiguration: %w", err)
		}
	}

	// Update the CA bundle in the webhook configuration
	updated := false
	for i := range webhookConfig.Webhooks {
		if webhookConfig.Webhooks[i].Name == "postgresql-operator.vyrodovalexey.github.com" {
			webhookConfig.Webhooks[i].ClientConfig.CABundle = []byte(caBundle)
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("webhook 'postgresql-operator.vyrodovalexey.github.com' not found in ValidatingWebhookConfiguration")
	}

	// Update the ValidatingWebhookConfiguration
	_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(ctx, webhookConfig, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ValidatingWebhookConfiguration: %w", err)
	}

	return nil
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

	if !cfg.EnableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watcher for webhook certificates
	var webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	// Generate or use provided webhook certificates
	var webhookCertPath, webhookKeyPath string
	if len(cfg.WebhookCertPath) > 0 {
		// Use provided certificates
		webhookCertPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertName)
		webhookKeyPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertKey)
		lg.Infow("Using provided webhook certificates",
			"webhook-cert-path", cfg.WebhookCertPath, "webhook-cert-name", cfg.WebhookCertName, "webhook-cert-key", cfg.WebhookCertKey)
	} else {
		// Generate self-signed certificates and store in Kubernetes Secret
		tempCertDir, err := os.MkdirTemp("", "webhook-certs-")
		if err != nil {
			lg.Error(err, "Failed to create temporary directory for webhook certificates")
			os.Exit(1)
		}
		lg.Infow("Generating self-signed webhook certificates and storing in Secret", "cert-dir", tempCertDir)

		// Get service name from environment or use default
		serviceName := os.Getenv("SERVICE_NAME")
		if serviceName == "" {
			serviceName = "k8s-postgresql-operator"
		}
		// Discover current namespace
		namespace := getCurrentNamespace()

		secretName := fmt.Sprintf("%s-webhook-cert", serviceName)

		// Get Kubernetes config
		k8sConfig := ctrl.GetConfigOrDie()
		_, webhookCertPath, webhookKeyPath, err = cert.GenerateSelfSignedCertAndStoreInSecret(serviceName, namespace, secretName, tempCertDir, k8sConfig)
		if err != nil {
			lg.Error(err, "Failed to generate self-signed webhook certificates")
			os.Exit(1)
		}
		lg.Infow("Webhook certificates generated and stored in Secret", "secret-name", secretName, "cert-path", webhookCertPath, "key-path", webhookKeyPath)

		// Update ValidatingWebhookConfiguration with CA bundle from secret
		if err := updateWebhookConfigurationWithCABundle(secretName, namespace, k8sConfig, lg); err != nil {
			lg.Error(err, "Failed to update ValidatingWebhookConfiguration with CA bundle", "secret-name", secretName)
			// Don't exit, as the webhook might still work if the CA bundle is set manually
		} else {
			lg.Infow("Successfully updated ValidatingWebhookConfiguration with CA bundle", "secret-name", secretName)
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
		Host:    "0.0.0.0",
		Port:    8443,
		TLSOpts: webhookTLSOpts,
	})
	lg.Infow("Webhook server configured", "host", "0.0.0.0", "port", 8443, "cert-path", webhookCertPath, "key-path", webhookKeyPath)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"}, // Disable metrics server
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
		vc, err := vault.NewClient(cfg.VaultAddr, cfg.VaultRole, cfg.VaultK8sTokenPath, cfg.VaultMountPoint, cfg.VaultSecretPath)
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

	if err := (&controller.PostgresqlReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		VaultClient: vaultClient,
		Log:         lg,
	}).SetupWithManager(mgr); err != nil {
		lg.Error(err, "unable to create controller Postgresql")
		os.Exit(1)
	}

	if err := (&controller.UserReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		VaultClient: vaultClient,
		Log:         lg,
	}).SetupWithManager(mgr); err != nil {
		lg.Error(err, "unable to create controller User")
		os.Exit(1)
	}

	if err := (&controller.DatabaseReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		VaultClient: vaultClient,
		Log:         lg,
	}).SetupWithManager(mgr); err != nil {
		lg.Error(err, "unable to create controller Database")
		os.Exit(1)
	}

	// Register webhook
	if err := (&instancev1alpha1.Postgresql{}).SetupWebhookWithManager(mgr); err != nil {
		lg.Error(err, "unable to create webhook", "webhook", "Postgresql")
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

	lg.Infow("starting postgresql operator", "webhook-port", 8443)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		lg.Error(err, "problem starting postgresql operator")
		os.Exit(1)
	}
}
