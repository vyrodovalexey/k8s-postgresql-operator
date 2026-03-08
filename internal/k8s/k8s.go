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

package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	certpkg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
)

// GetCurrentNamespace discovers the current namespace from the service account
// Returns the namespace or a default value if not found
func GetCurrentNamespace(namespaceFile string) string {
	// Try to read from the service account namespace file (standard in Kubernetes pods)
	// nolint:gosec // reading from service account namespace file is safe in Kubernetes context
	if data, err := os.ReadFile(namespaceFile); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}

	// Default fallback
	return "k8s-postgresql-operator"
}

// UpdateWebhookConfigurationWithCABundle updates the ValidatingWebhookConfiguration with the CA bundle from the secret
func UpdateWebhookConfigurationWithCABundle(
	ctx context.Context, secretName, namespace string, restConfig *rest.Config,
	webHookName string, lg *zap.SugaredLogger,
) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get the secret containing the certificate
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	caBundle, ok := secret.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("tls.crt not found in secret %s", secretName)
	}

	// Get the ValidatingWebhookConfiguration
	webhookConfigName := webHookName
	webhookConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
		ctx, webhookConfigName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ValidatingWebhookConfiguration: %w", err)
	}

	// Update the CA bundle in the webhook configuration
	updated := false
	for i := range webhookConfig.Webhooks {
		if webhookConfig.Webhooks[i].Name == "postgresql-operator.vyrodovalexey.github.com" {
			webhookConfig.Webhooks[i].ClientConfig.CABundle = caBundle
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf(
			"webhook 'postgresql-operator.vyrodovalexey.github.com' not found in ValidatingWebhookConfiguration")
	}

	// Update the ValidatingWebhookConfiguration
	_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(
		ctx, webhookConfig, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ValidatingWebhookConfiguration: %w", err)
	}

	return nil
}

// SetupWebhookCertificates sets up webhook certificates either from provided paths or by generating them
func SetupWebhookCertificates(
	ctx context.Context, cfg *config.Config, webhookNames []string, lg *zap.SugaredLogger,
) (webhookCertPath, webhookKeyPath string) {
	certPath, keyPath, err := setupWebhookCertificatesInternal(
		ctx, cfg, webhookNames, lg, ctrl.GetConfigOrDie,
	)
	if err != nil {
		lg.Error(err, "Failed to setup webhook certificates")
		os.Exit(1)
	}
	return certPath, keyPath
}

// setupWebhookCertificatesInternal contains the testable logic for SetupWebhookCertificates.
// It accepts a getConfig function to allow injecting a test config instead of ctrl.GetConfigOrDie.
func setupWebhookCertificatesInternal(
	ctx context.Context, cfg *config.Config, webhookNames []string, lg *zap.SugaredLogger,
	getConfig func() *rest.Config,
) (webhookCertPath, webhookKeyPath string, err error) {
	// Generate or use provided webhook certificates
	if cfg.WebhookCertPath != "" {
		// Use provided certificates
		webhookCertPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertName)
		webhookKeyPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertKey)
		lg.Infow("Using provided webhook certificates",
			"webhook-cert-path", cfg.WebhookCertPath,
			"webhook-cert-name", cfg.WebhookCertName,
			"webhook-cert-key", cfg.WebhookCertKey)
		return webhookCertPath, webhookKeyPath, nil
	}

	// Generate self-signed certificates and store in Kubernetes Secret
	tempCertDir, err := os.MkdirTemp("", "webhook-certs-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temporary directory for webhook certificates: %w", err)
	}
	lg.Infow("Generating self-signed webhook certificates and storing in Secret", "cert-dir", tempCertDir)

	// Discover current namespace
	namespace := GetCurrentNamespace(cfg.K8sNamespacePath)

	secretName := fmt.Sprintf("%s-webhook-cert", cfg.WebhookK8sServiceName)

	// Get Kubernetes config
	restConfig := getConfig()
	//nolint:contextcheck // GenerateSelfSignedCertAndStoreInSecret does not accept context yet
	_, webhookCertPath, webhookKeyPath, err = certpkg.GenerateSelfSignedCertAndStoreInSecret(
		cfg.WebhookK8sServiceName, namespace, secretName, tempCertDir, restConfig)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate self-signed webhook certificates: %w", err)
	}
	lg.Infow("Webhook certificates generated and stored in Secret",
		"secret-name", secretName,
		"cert-path", webhookCertPath,
		"key-path", webhookKeyPath)

	// Update all ValidatingWebhookConfigurations with CA bundle from secret
	for _, webhookName := range webhookNames {
		if err := UpdateWebhookConfigurationWithCABundle(
			ctx, secretName, namespace, restConfig, webhookName, lg); err != nil {
			lg.Error(err, "Failed to update ValidatingWebhookConfiguration with CA bundle",
				"secret-name", secretName,
				"webhook-name", webhookName)
			// Don't exit, as the webhook might still work if the CA bundle is set manually
			continue
		}
		lg.Infow("Successfully updated ValidatingWebhookConfiguration with CA bundle",
			"secret-name", secretName,
			"webhook-name", webhookName)
	}

	return webhookCertPath, webhookKeyPath, nil
}

// SetupWebhookCertificatesWithVaultPKI sets up webhook certificates
// using Vault PKI
func SetupWebhookCertificatesWithVaultPKI(
	ctx context.Context,
	pkiManager *certpkg.VaultPKIManager,
	cfg *config.Config,
	webhookNames []string,
	lg *zap.SugaredLogger,
) (webhookCertPath, webhookKeyPath string) {
	certPath, keyPath, err := setupWebhookCertificatesWithVaultPKIInternal(
		ctx, pkiManager, webhookNames, lg, ctrl.GetConfigOrDie,
	)
	if err != nil {
		lg.Error(err, "Failed to setup webhook certificates with Vault PKI")
		os.Exit(1)
	}
	return certPath, keyPath
}

// setupWebhookCertificatesWithVaultPKIInternal contains the testable logic
// for SetupWebhookCertificatesWithVaultPKI.
func setupWebhookCertificatesWithVaultPKIInternal(
	ctx context.Context,
	pkiManager *certpkg.VaultPKIManager,
	webhookNames []string,
	lg *zap.SugaredLogger,
	getConfig func() *rest.Config,
) (webhookCertPath, webhookKeyPath string, err error) {
	if err := pkiManager.IssueCertificateAndWriteToDisk(ctx); err != nil {
		return "", "", fmt.Errorf(
			"failed to issue Vault PKI certificate: %w", err,
		)
	}

	caBundle, err := pkiManager.GetCACertificatePEM(ctx)
	if err != nil {
		return "", "", fmt.Errorf(
			"failed to get Vault PKI CA certificate: %w", err,
		)
	}

	restConfig := getConfig()
	for _, webhookName := range webhookNames {
		if err := UpdateWebhookCABundleFromPEM(
			ctx, caBundle, restConfig, webhookName, lg,
		); err != nil {
			lg.Errorw(
				"Failed to update webhook CA bundle from Vault PKI",
				"webhook-name", webhookName,
				"error", err,
			)
			continue
		}
		lg.Infow(
			"Updated webhook CA bundle from Vault PKI",
			"webhook-name", webhookName,
		)
	}

	return pkiManager.CertPath(), pkiManager.KeyPath(), nil
}

// UpdateWebhookCABundleFromPEM updates a ValidatingWebhookConfiguration
// with a CA bundle provided as raw PEM bytes
func UpdateWebhookCABundleFromPEM(
	ctx context.Context,
	caBundle []byte,
	restConfig *rest.Config,
	webHookName string,
	lg *zap.SugaredLogger,
) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	webhookConfig, err := clientset.AdmissionregistrationV1().
		ValidatingWebhookConfigurations().
		Get(ctx, webHookName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf(
			"failed to get ValidatingWebhookConfiguration: %w", err,
		)
	}

	updated := false
	webhookDNS := "postgresql-operator.vyrodovalexey.github.com"
	for i := range webhookConfig.Webhooks {
		if webhookConfig.Webhooks[i].Name == webhookDNS {
			webhookConfig.Webhooks[i].ClientConfig.CABundle = caBundle
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf(
			"webhook '%s' not found in webhook configuration",
			webhookDNS,
		)
	}

	_, err = clientset.AdmissionregistrationV1().
		ValidatingWebhookConfigurations().
		Update(ctx, webhookConfig, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf(
			"failed to update ValidatingWebhookConfiguration: %w", err,
		)
	}

	lg.Infow("Successfully updated webhook CA bundle from PEM",
		"webhook-name", webHookName,
	)

	return nil
}
