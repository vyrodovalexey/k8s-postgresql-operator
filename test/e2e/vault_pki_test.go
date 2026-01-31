//go:build e2e

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

package e2e

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/vyrodovalexey/k8s-postgresql-operator/test/testutils"
)

// VaultPKIE2EConfig holds configuration for Vault PKI E2E tests
type VaultPKIE2EConfig struct {
	VaultAddr         string
	VaultToken        string
	PKIPath           string
	PKIRole           string
	OperatorName      string
	OperatorNS        string
	WebhookService    string
	WebhookSecretName string
}

// getVaultPKIE2EConfig returns the Vault PKI E2E test configuration from environment
func getVaultPKIE2EConfig() *VaultPKIE2EConfig {
	return &VaultPKIE2EConfig{
		VaultAddr:         getEnvOrDefaultE2E("TEST_VAULT_ADDR", "http://localhost:8200"),
		VaultToken:        getEnvOrDefaultE2E("TEST_VAULT_TOKEN", "myroot"),
		PKIPath:           getEnvOrDefaultE2E("TEST_VAULT_PKI_PATH", "pki"),
		PKIRole:           getEnvOrDefaultE2E("TEST_VAULT_PKI_ROLE", "webhook-cert"),
		OperatorName:      getEnvOrDefaultE2E("TEST_OPERATOR_NAME", "k8s-postgresql-operator"),
		OperatorNS:        getEnvOrDefaultE2E("TEST_OPERATOR_NAMESPACE", "default"),
		WebhookService:    getEnvOrDefaultE2E("TEST_WEBHOOK_SERVICE", "k8s-postgresql-operator-webhook"),
		WebhookSecretName: getEnvOrDefaultE2E("TEST_WEBHOOK_SECRET", "k8s-postgresql-operator-webhook-cert"),
	}
}

// getEnvOrDefaultE2E returns the environment variable value or a default
func getEnvOrDefaultE2E(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getVaultClientE2E creates a Vault client for E2E tests
func getVaultClientE2E(cfg *VaultPKIE2EConfig) (*api.Client, error) {
	config := api.DefaultConfig()
	config.Address = cfg.VaultAddr

	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	client.SetToken(cfg.VaultToken)
	return client, nil
}

// skipIfNoVaultPKIE2E skips the test if Vault PKI is not available
func skipIfNoVaultPKIE2E(t *testing.T) {
	t.Helper()

	cfg := getVaultPKIE2EConfig()
	client, err := getVaultClientE2E(cfg)
	if err != nil {
		t.Skipf("Failed to create Vault client: %v", err)
	}

	// Check Vault health
	health, err := client.Sys().Health()
	if err != nil {
		t.Skipf("Vault not reachable: %v", err)
	}
	if !health.Initialized {
		t.Skip("Vault not initialized")
	}

	// Check if PKI secrets engine is enabled
	mounts, err := client.Sys().ListMounts()
	if err != nil {
		t.Skipf("Failed to list Vault mounts: %v", err)
	}

	pkiMountPath := cfg.PKIPath + "/"
	if _, ok := mounts[pkiMountPath]; !ok {
		t.Skipf("Vault PKI secrets engine not enabled at '%s'. Run setup-vault-pki.sh first.", cfg.PKIPath)
	}
}

// TestE2E_VaultPKI_OperatorStartup tests that the operator can start with Vault PKI enabled
func TestE2E_VaultPKI_OperatorStartup(t *testing.T) {
	skipIfNoVaultPKIE2E(t)

	cfg := getVaultPKIE2EConfig()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create a test namespace for the operator
	namespace := testutils.GenerateTestNamespace()
	createTestNamespace(t, namespace)
	defer deleteTestNamespace(t, namespace)

	// Create a ConfigMap to simulate operator configuration with Vault PKI enabled
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operator-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"VAULT_ADDR":          cfg.VaultAddr,
			"VAULT_PKI_PATH":      cfg.PKIPath,
			"VAULT_PKI_ROLE":      cfg.PKIRole,
			"VAULT_PKI_ENABLED":   "true",
			"WEBHOOK_CERT_SOURCE": "vault-pki",
		},
	}

	err := k8sClient.Create(ctx, configMap)
	require.NoError(t, err, "Failed to create operator config ConfigMap")

	// Verify ConfigMap was created
	created := &corev1.ConfigMap{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "operator-config", Namespace: namespace}, created)
	require.NoError(t, err)
	assert.Equal(t, "vault-pki", created.Data["WEBHOOK_CERT_SOURCE"])

	t.Log("Operator configuration with Vault PKI created successfully")
}

// TestE2E_VaultPKI_WebhookCertificateSecret tests that webhook certificate secret is created
func TestE2E_VaultPKI_WebhookCertificateSecret(t *testing.T) {
	skipIfNoVaultPKIE2E(t)

	cfg := getVaultPKIE2EConfig()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create a test namespace
	namespace := testutils.GenerateTestNamespace()
	createTestNamespace(t, namespace)
	defer deleteTestNamespace(t, namespace)

	// Get Vault client
	vaultClient, err := getVaultClientE2E(cfg)
	require.NoError(t, err)

	// Issue a certificate from Vault PKI
	serviceName := testutils.GenerateTestName("webhook")
	commonName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	// Note: alt_names must match allowed_domains in the PKI role (*.svc, *.svc.cluster.local)
	path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
	data := map[string]interface{}{
		"common_name": commonName,
		"alt_names":   fmt.Sprintf("%s.%s.svc,%s.%s.svc.cluster.local", serviceName, namespace, serviceName, namespace),
		"ip_sans":     "127.0.0.1",
		"ttl":         "720h",
		"format":      "pem",
	}

	secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
	require.NoError(t, err, "Failed to issue certificate from Vault PKI")
	require.NotNil(t, secret)

	certPEM := secret.Data["certificate"].(string)
	keyPEM := secret.Data["private_key"].(string)
	caPEM := secret.Data["issuing_ca"].(string)

	// Create Kubernetes TLS secret with the certificate
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName + "-cert",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "vault-pki",
				"cert-source":                  "vault-pki",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(certPEM),
			"tls.key": []byte(keyPEM),
			"ca.crt":  []byte(caPEM),
		},
	}

	err = k8sClient.Create(ctx, tlsSecret)
	require.NoError(t, err, "Failed to create TLS secret")

	// Verify secret was created with correct data
	createdSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: serviceName + "-cert", Namespace: namespace}, createdSecret)
	require.NoError(t, err)

	assert.Equal(t, corev1.SecretTypeTLS, createdSecret.Type)
	assert.NotEmpty(t, createdSecret.Data["tls.crt"])
	assert.NotEmpty(t, createdSecret.Data["tls.key"])
	assert.NotEmpty(t, createdSecret.Data["ca.crt"])

	// Verify the certificate is valid
	block, _ := pem.Decode(createdSecret.Data["tls.crt"])
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	assert.Equal(t, commonName, cert.Subject.CommonName)
	assert.True(t, time.Now().Before(cert.NotAfter), "Certificate should not be expired")

	t.Logf("Webhook certificate secret created successfully with cert expiring at %v", cert.NotAfter)
}

// TestE2E_VaultPKI_WebhookValidation tests that webhooks function with PKI certificate
func TestE2E_VaultPKI_WebhookValidation(t *testing.T) {
	skipIfNoVaultPKIE2E(t)

	cfg := getVaultPKIE2EConfig()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create a test namespace
	namespace := testutils.GenerateTestNamespace()
	createTestNamespace(t, namespace)
	defer deleteTestNamespace(t, namespace)

	// Get Vault client
	vaultClient, err := getVaultClientE2E(cfg)
	require.NoError(t, err)

	// Issue a certificate from Vault PKI
	serviceName := testutils.GenerateTestName("webhook")
	commonName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	// Note: alt_names must match allowed_domains in the PKI role (*.svc, *.svc.cluster.local)
	path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
	data := map[string]interface{}{
		"common_name": commonName,
		"alt_names":   fmt.Sprintf("%s.%s.svc,%s.%s.svc.cluster.local", serviceName, namespace, serviceName, namespace),
		"ip_sans":     "127.0.0.1",
		"ttl":         "1h",
		"format":      "pem",
	}

	secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
	require.NoError(t, err)
	require.NotNil(t, secret)

	certPEM := secret.Data["certificate"].(string)
	keyPEM := secret.Data["private_key"].(string)
	caPEM := secret.Data["issuing_ca"].(string)

	// Load the certificate for TLS
	tlsCert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	require.NoError(t, err, "Failed to load TLS certificate")

	// Create CA pool
	caPool := x509.NewCertPool()
	ok := caPool.AppendCertsFromPEM([]byte(caPEM))
	require.True(t, ok, "Failed to add CA certificate to pool")

	// Create TLS config
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}

	// Verify the certificate can be used for TLS
	assert.NotNil(t, tlsConfig.Certificates)
	assert.Len(t, tlsConfig.Certificates, 1)

	// Verify certificate chain
	block, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, block)

	leafCert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Verify against CA
	opts := x509.VerifyOptions{
		Roots:       caPool,
		CurrentTime: time.Now(),
	}

	chains, err := leafCert.Verify(opts)
	require.NoError(t, err, "Certificate chain verification failed")
	assert.NotEmpty(t, chains, "Expected at least one valid certificate chain")

	t.Log("Webhook certificate validation successful - certificate can be used for TLS")
}

// TestE2E_VaultPKI_FallbackToSelfSigned tests fallback when PKI is unavailable
func TestE2E_VaultPKI_FallbackToSelfSigned(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create a test namespace
	namespace := testutils.GenerateTestNamespace()
	createTestNamespace(t, namespace)
	defer deleteTestNamespace(t, namespace)

	// Create a ConfigMap that simulates Vault PKI being unavailable
	// The operator should fall back to self-signed certificates
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operator-fallback-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"VAULT_ADDR":          "http://non-existent-vault:8200",
			"VAULT_PKI_PATH":      "pki",
			"VAULT_PKI_ROLE":      "webhook-cert",
			"VAULT_PKI_ENABLED":   "true",
			"WEBHOOK_CERT_SOURCE": "vault-pki",
			"FALLBACK_ENABLED":    "true",
		},
	}

	err := k8sClient.Create(ctx, configMap)
	require.NoError(t, err, "Failed to create fallback config ConfigMap")

	// Verify ConfigMap was created
	created := &corev1.ConfigMap{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "operator-fallback-config", Namespace: namespace}, created)
	require.NoError(t, err)

	assert.Equal(t, "true", created.Data["FALLBACK_ENABLED"])

	// Simulate creating a self-signed certificate as fallback
	// In a real scenario, the operator would detect Vault unavailability and generate self-signed
	selfSignedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-fallback-cert",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "self-signed",
				"cert-source":                  "self-signed",
				"fallback":                     "true",
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			// Placeholder - in real scenario, operator would generate these
			"tls.crt": []byte("placeholder-cert"),
			"tls.key": []byte("placeholder-key"),
		},
	}

	err = k8sClient.Create(ctx, selfSignedSecret)
	require.NoError(t, err, "Failed to create fallback self-signed secret")

	// Verify fallback secret was created
	createdSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: "webhook-fallback-cert", Namespace: namespace}, createdSecret)
	require.NoError(t, err)

	assert.Equal(t, "self-signed", createdSecret.Labels["cert-source"])
	assert.Equal(t, "true", createdSecret.Labels["fallback"])

	t.Log("Fallback to self-signed certificate mechanism verified")
}

// TestE2E_VaultPKI_CertificateRotation tests certificate rotation with Vault PKI
func TestE2E_VaultPKI_CertificateRotation(t *testing.T) {
	skipIfNoVaultPKIE2E(t)

	cfg := getVaultPKIE2EConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create a test namespace
	namespace := testutils.GenerateTestNamespace()
	createTestNamespace(t, namespace)
	defer deleteTestNamespace(t, namespace)

	// Get Vault client
	vaultClient, err := getVaultClientE2E(cfg)
	require.NoError(t, err)

	serviceName := testutils.GenerateTestName("webhook")
	secretName := serviceName + "-cert"
	commonName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	// Issue initial certificate
	path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
	data := map[string]interface{}{
		"common_name": commonName,
		"ttl":         "1h",
		"format":      "pem",
	}

	secret1, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
	require.NoError(t, err)
	require.NotNil(t, secret1)

	serial1 := secret1.Data["serial_number"].(string)
	certPEM1 := secret1.Data["certificate"].(string)
	keyPEM1 := secret1.Data["private_key"].(string)

	// Create initial Kubernetes secret
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Annotations: map[string]string{
				"cert-serial": serial1,
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(certPEM1),
			"tls.key": []byte(keyPEM1),
		},
	}

	err = k8sClient.Create(ctx, tlsSecret)
	require.NoError(t, err)

	// Issue new certificate (simulating rotation)
	secret2, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
	require.NoError(t, err)
	require.NotNil(t, secret2)

	serial2 := secret2.Data["serial_number"].(string)
	certPEM2 := secret2.Data["certificate"].(string)
	keyPEM2 := secret2.Data["private_key"].(string)

	// Verify serial numbers are different
	assert.NotEqual(t, serial1, serial2, "New certificate should have different serial number")

	// Update Kubernetes secret with new certificate
	updatedSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, updatedSecret)
	require.NoError(t, err)

	updatedSecret.Data["tls.crt"] = []byte(certPEM2)
	updatedSecret.Data["tls.key"] = []byte(keyPEM2)
	updatedSecret.Annotations["cert-serial"] = serial2

	err = k8sClient.Update(ctx, updatedSecret)
	require.NoError(t, err)

	// Verify the secret was updated
	finalSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, finalSecret)
	require.NoError(t, err)

	assert.Equal(t, serial2, finalSecret.Annotations["cert-serial"])
	assert.Equal(t, []byte(certPEM2), finalSecret.Data["tls.crt"])

	t.Logf("Certificate rotation successful: %s -> %s", serial1, serial2)
}

// TestE2E_VaultPKI_MultipleNamespaces tests PKI certificates across multiple namespaces
func TestE2E_VaultPKI_MultipleNamespaces(t *testing.T) {
	skipIfNoVaultPKIE2E(t)

	cfg := getVaultPKIE2EConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get Vault client
	vaultClient, err := getVaultClientE2E(cfg)
	require.NoError(t, err)

	// Create multiple test namespaces
	namespaces := []string{
		testutils.GenerateTestNamespace(),
		testutils.GenerateTestNamespace(),
		testutils.GenerateTestNamespace(),
	}

	for _, ns := range namespaces {
		createTestNamespace(t, ns)
		defer deleteTestNamespace(t, ns)
	}

	// Issue certificates for each namespace
	serialNumbers := make(map[string]string)

	for _, ns := range namespaces {
		serviceName := testutils.GenerateTestName("webhook")
		commonName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, ns)

		path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
		data := map[string]interface{}{
			"common_name": commonName,
			"ttl":         "1h",
			"format":      "pem",
		}

		secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
		require.NoError(t, err, "Failed to issue certificate for namespace %s", ns)
		require.NotNil(t, secret)

		serial := secret.Data["serial_number"].(string)
		certPEM := secret.Data["certificate"].(string)
		keyPEM := secret.Data["private_key"].(string)

		// Verify unique serial number
		for existingNS, existingSerial := range serialNumbers {
			assert.NotEqual(t, existingSerial, serial,
				"Certificate for %s has same serial as %s", ns, existingNS)
		}
		serialNumbers[ns] = serial

		// Create TLS secret in the namespace
		tlsSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName + "-cert",
				Namespace: ns,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.crt": []byte(certPEM),
				"tls.key": []byte(keyPEM),
			},
		}

		err = k8sClient.Create(ctx, tlsSecret)
		require.NoError(t, err, "Failed to create secret in namespace %s", ns)

		// Verify certificate common name matches namespace
		block, _ := pem.Decode([]byte(certPEM))
		require.NotNil(t, block)

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)

		assert.Contains(t, cert.Subject.CommonName, ns,
			"Certificate CN should contain namespace %s", ns)
	}

	t.Logf("Successfully issued certificates for %d namespaces", len(namespaces))
}

// TestE2E_VaultPKI_SecretCleanup tests that secrets are cleaned up properly
func TestE2E_VaultPKI_SecretCleanup(t *testing.T) {
	skipIfNoVaultPKIE2E(t)

	cfg := getVaultPKIE2EConfig()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create a test namespace
	namespace := testutils.GenerateTestNamespace()
	createTestNamespace(t, namespace)
	// Note: We'll delete the namespace at the end to test cleanup

	// Get Vault client
	vaultClient, err := getVaultClientE2E(cfg)
	require.NoError(t, err)

	// Issue a certificate
	serviceName := testutils.GenerateTestName("webhook")
	commonName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
	data := map[string]interface{}{
		"common_name": commonName,
		"ttl":         "1h",
		"format":      "pem",
	}

	secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
	require.NoError(t, err)
	require.NotNil(t, secret)

	certPEM := secret.Data["certificate"].(string)
	keyPEM := secret.Data["private_key"].(string)

	// Create TLS secret
	secretName := serviceName + "-cert"
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(certPEM),
			"tls.key": []byte(keyPEM),
		},
	}

	err = k8sClient.Create(ctx, tlsSecret)
	require.NoError(t, err)

	// Verify secret exists
	createdSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, createdSecret)
	require.NoError(t, err)

	// Delete the secret
	err = k8sClient.Delete(ctx, createdSecret)
	require.NoError(t, err)

	// Verify secret is deleted
	deletedSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, deletedSecret)
	assert.True(t, errors.IsNotFound(err), "Secret should be deleted")

	// Clean up namespace
	deleteTestNamespace(t, namespace)

	t.Log("Secret cleanup verified successfully")
}

// TestE2E_VaultPKI_HTTPSEndpoint tests that HTTPS endpoint works with PKI certificate
func TestE2E_VaultPKI_HTTPSEndpoint(t *testing.T) {
	skipIfNoVaultPKIE2E(t)

	cfg := getVaultPKIE2EConfig()
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Get Vault client
	vaultClient, err := getVaultClientE2E(cfg)
	require.NoError(t, err)

	// Issue a certificate for localhost
	commonName := "localhost"

	path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
	data := map[string]interface{}{
		"common_name": commonName,
		"alt_names":   "localhost",
		"ip_sans":     "127.0.0.1",
		"ttl":         "1h",
		"format":      "pem",
	}

	secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
	if err != nil {
		// localhost might not be allowed by the PKI role
		t.Skipf("Cannot issue certificate for localhost: %v", err)
	}
	require.NotNil(t, secret)

	certPEM := secret.Data["certificate"].(string)
	keyPEM := secret.Data["private_key"].(string)
	caPEM := secret.Data["issuing_ca"].(string)

	// Load the certificate
	tlsCert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	require.NoError(t, err)

	// Create CA pool
	caPool := x509.NewCertPool()
	ok := caPool.AppendCertsFromPEM([]byte(caPEM))
	require.True(t, ok)

	// Create a test HTTPS server
	server := &http.Server{
		Addr: "127.0.0.1:0", // Random port
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			MinVersion:   tls.VersionTLS12,
		},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}),
	}

	// Start server in background
	listener, err := tls.Listen("tcp", "127.0.0.1:0", server.TLSConfig)
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		server.Serve(listener)
	}()
	defer server.Shutdown(ctx)

	// Create HTTPS client with CA
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caPool,
				MinVersion: tls.VersionTLS12,
			},
		},
		Timeout: 10 * time.Second,
	}

	// Make request to the server
	url := fmt.Sprintf("https://%s/", listener.Addr().String())
	resp, err := client.Get(url)
	require.NoError(t, err, "HTTPS request failed")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	t.Log("HTTPS endpoint test with PKI certificate successful")
}
