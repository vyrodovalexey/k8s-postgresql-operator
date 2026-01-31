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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
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
	secretName, namespace string, restConfig *rest.Config, webHookName string, lg *zap.SugaredLogger) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return UpdateWebhookConfigurationWithCABundleUsingClient(
		context.Background(), secretName, namespace, clientset, webHookName, lg)
}

// UpdateWebhookConfigurationWithCABundleUsingClient updates the ValidatingWebhookConfiguration
// with the CA bundle from the secret using a provided Kubernetes clientset.
// This function is useful for testing with fake clients.
func UpdateWebhookConfigurationWithCABundleUsingClient(
	ctx context.Context,
	secretName, namespace string,
	clientset kubernetes.Interface,
	webHookName string,
	lg *zap.SugaredLogger,
) error {
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

// SetupWebhookCertificates sets up webhook certificates either from provided paths or by generating them.
//
// Deprecated: Use SetupWebhookCertificatesWithProvider instead for new implementations.
func SetupWebhookCertificates(
	cfg *config.Config, webhookNames []string, lg *zap.SugaredLogger,
) (webhookCertPath, webhookKeyPath string) {
	// Generate or use provided webhook certificates
	if cfg.WebhookCertPath != "" {
		// Use provided certificates
		webhookCertPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertName)
		webhookKeyPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertKey)
		lg.Infow("Using provided webhook certificates",
			"webhook-cert-path", cfg.WebhookCertPath,
			"webhook-cert-name", cfg.WebhookCertName,
			"webhook-cert-key", cfg.WebhookCertKey)
		return webhookCertPath, webhookKeyPath
	}

	// Generate self-signed certificates and store in Kubernetes Secret
	tempCertDir, err := os.MkdirTemp("", "webhook-certs-")
	if err != nil {
		lg.Error(err, "Failed to create temporary directory for webhook certificates")
		os.Exit(1)
	}
	lg.Infow("Generating self-signed webhook certificates and storing in Secret", "cert-dir", tempCertDir)

	// Discover current namespace
	namespace := GetCurrentNamespace(cfg.K8sNamespacePath)

	secretName := fmt.Sprintf("%s-webhook-cert", cfg.WebhookK8sServiceName)

	// Get Kubernetes config
	restConfig := ctrl.GetConfigOrDie()
	_, webhookCertPath, webhookKeyPath, err = cert.GenerateSelfSignedCertAndStoreInSecret(
		cfg.WebhookK8sServiceName, namespace, secretName, tempCertDir, restConfig)
	if err != nil {
		lg.Error(err, "Failed to generate self-signed webhook certificates")
		os.Exit(1)
	}
	lg.Infow("Webhook certificates generated and stored in Secret",
		"secret-name", secretName,
		"cert-path", webhookCertPath,
		"key-path", webhookKeyPath)

	// Update all ValidatingWebhookConfigurations with CA bundle from secret
	for _, webhookName := range webhookNames {
		if err := UpdateWebhookConfigurationWithCABundle(
			secretName, namespace, restConfig, webhookName, lg); err != nil {
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

	return webhookCertPath, webhookKeyPath
}

// SetupWebhookCertificatesWithProvider sets up webhook certificates using the certificate provider pattern
// This is the preferred method for setting up webhook certificates
func SetupWebhookCertificatesWithProvider(
	ctx context.Context,
	cfg *config.Config,
	webhookNames []string,
	provider cert.CertificateProvider,
	lg *zap.SugaredLogger,
) (webhookCertPath, webhookKeyPath string, err error) {
	// Discover current namespace
	namespace := GetCurrentNamespace(cfg.K8sNamespacePath)

	lg.Infow("Setting up webhook certificates",
		"provider_type", provider.Type(),
		"namespace", namespace)

	// Handle provided certificates differently - they already exist on disk
	if provider.Type() == cert.ProviderTypeProvided {
		providedProvider, ok := provider.(*cert.ProvidedProvider)
		if !ok {
			return "", "", fmt.Errorf("invalid provider type assertion for provided provider")
		}
		webhookCertPath = providedProvider.GetCertPath()
		webhookKeyPath = providedProvider.GetKeyPath()

		lg.Infow("Using provided webhook certificates",
			"cert_path", webhookCertPath,
			"key_path", webhookKeyPath)

		return webhookCertPath, webhookKeyPath, nil
	}

	// Issue certificate using the provider
	certificate, err := provider.IssueCertificate(ctx, cfg.WebhookK8sServiceName, namespace)
	if err != nil {
		return "", "", fmt.Errorf("failed to issue certificate: %w", err)
	}

	// Get Kubernetes config
	restConfig := ctrl.GetConfigOrDie()

	// Store certificate in Kubernetes secret
	secretName := fmt.Sprintf("%s-webhook-cert", cfg.WebhookK8sServiceName)
	if err := StoreCertificateInSecret(ctx, certificate, secretName, namespace, restConfig); err != nil {
		return "", "", fmt.Errorf("failed to store certificate in secret: %w", err)
	}

	// Write certificate to files
	certDir, err := os.MkdirTemp("", "webhook-certs-")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temporary directory for certificates: %w", err)
	}

	webhookCertPath, webhookKeyPath, err = WriteCertificateToFiles(certificate, certDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to write certificate to files: %w", err)
	}

	lg.Infow("Webhook certificates set up successfully",
		"provider_type", provider.Type(),
		"secret_name", secretName,
		"cert_path", webhookCertPath,
		"key_path", webhookKeyPath,
		"expires_at", certificate.ExpiresAt)

	// Update all ValidatingWebhookConfigurations with CA bundle from secret
	for _, webhookName := range webhookNames {
		if err := UpdateWebhookConfigurationWithCABundle(
			secretName, namespace, restConfig, webhookName, lg); err != nil {
			lg.Errorw("Failed to update ValidatingWebhookConfiguration with CA bundle",
				"error", err,
				"secret_name", secretName,
				"webhook_name", webhookName)
			// Don't return error, as the webhook might still work if the CA bundle is set manually
			continue
		}
		lg.Infow("Successfully updated ValidatingWebhookConfiguration with CA bundle",
			"secret_name", secretName,
			"webhook_name", webhookName)
	}

	return webhookCertPath, webhookKeyPath, nil
}

// StoreCertificateInSecret stores a certificate in a Kubernetes secret
func StoreCertificateInSecret(
	ctx context.Context,
	certificate *cert.Certificate,
	secretName, namespace string,
	restConfig *rest.Config,
) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return StoreCertificateInSecretUsingClient(ctx, certificate, secretName, namespace, clientset)
}

// StoreCertificateInSecretUsingClient stores a certificate in a Kubernetes secret
// using a provided Kubernetes clientset.
// This function is useful for testing with fake clients.
func StoreCertificateInSecretUsingClient(
	ctx context.Context,
	certificate *cert.Certificate,
	secretName, namespace string,
	clientset kubernetes.Interface,
) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certificate.CertPEM,
			"tls.key": certificate.KeyPEM,
		},
	}

	// Try to create the secret, if it exists, update it
	_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		// Secret might already exist, try to update it
		_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update secret: %w", err)
		}
	}

	return nil
}

// WriteCertificateToFiles writes a certificate to files in the specified directory
func WriteCertificateToFiles(certificate *cert.Certificate, certDir string) (certPath, keyPath string, err error) {
	// Create cert directory if it doesn't exist
	if err := os.MkdirAll(certDir, 0o750); err != nil {
		return "", "", fmt.Errorf("failed to create cert directory: %w", err)
	}

	certPath = filepath.Join(certDir, "tls.crt")
	keyPath = filepath.Join(certDir, "tls.key")

	// Write certificate to file
	//nolint:gosec // microservices approach - certificate files need to be readable
	certFile, err := os.Create(certPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFile.Close()

	if _, err := certFile.Write(certificate.CertPEM); err != nil {
		return "", "", fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key to file
	//nolint:gosec // microservices approach - key files need to be readable by the service
	keyFile, err := os.Create(keyPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	if _, err := keyFile.Write(certificate.KeyPEM); err != nil {
		return "", "", fmt.Errorf("failed to write private key: %w", err)
	}

	return certPath, keyPath, nil
}

// UpdateWebhookConfigurationWithCertificate updates the ValidatingWebhookConfiguration
// with the CA bundle from a certificate
func UpdateWebhookConfigurationWithCertificate(
	ctx context.Context,
	certificate *cert.Certificate,
	restConfig *rest.Config,
	webhookName string,
	lg *zap.SugaredLogger,
) error {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return UpdateWebhookConfigurationWithCertificateUsingClient(
		ctx, certificate, clientset, webhookName, lg)
}

// UpdateWebhookConfigurationWithCertificateUsingClient updates the ValidatingWebhookConfiguration
// with the CA bundle from a certificate using a provided Kubernetes clientset.
// This function is useful for testing with fake clients.
func UpdateWebhookConfigurationWithCertificateUsingClient(
	ctx context.Context,
	certificate *cert.Certificate,
	clientset kubernetes.Interface,
	webhookName string,
	lg *zap.SugaredLogger,
) error {
	// Get the ValidatingWebhookConfiguration
	webhookConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
		ctx, webhookName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ValidatingWebhookConfiguration: %w", err)
	}

	// Use CA certificate if available, otherwise use the certificate itself
	caBundle := certificate.CACertPEM
	if len(caBundle) == 0 {
		caBundle = certificate.CertPEM
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

	if lg != nil {
		lg.Infow("Updated ValidatingWebhookConfiguration with CA bundle",
			"webhook_name", webhookName)
	}

	return nil
}
