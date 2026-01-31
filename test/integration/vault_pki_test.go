//go:build integration

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

package integration

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vyrodovalexey/k8s-postgresql-operator/test/testutils"
)

// VaultPKITestConfig holds configuration for Vault PKI tests
type VaultPKITestConfig struct {
	VaultAddr   string
	VaultToken  string
	PKIPath     string
	PKIRole     string
	ServiceName string
	Namespace   string
}

// getVaultPKITestConfig returns the Vault PKI test configuration from environment
func getVaultPKITestConfig() *VaultPKITestConfig {
	return &VaultPKITestConfig{
		VaultAddr:   getEnvOrDefault("TEST_VAULT_ADDR", "http://localhost:8200"),
		VaultToken:  getEnvOrDefault("TEST_VAULT_TOKEN", "myroot"),
		PKIPath:     getEnvOrDefault("TEST_VAULT_PKI_PATH", "pki"),
		PKIRole:     getEnvOrDefault("TEST_VAULT_PKI_ROLE", "webhook-cert"),
		ServiceName: "test-webhook",
		Namespace:   "default",
	}
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// skipIfNoVaultPKI skips the test if Vault PKI is not available
func skipIfNoVaultPKI(t *testing.T) {
	t.Helper()
	skipIfNoVault(t)

	cfg := getVaultPKITestConfig()

	// Check if PKI secrets engine is enabled
	mounts, err := vaultClient.Sys().ListMounts()
	if err != nil {
		t.Skipf("Failed to list Vault mounts: %v", err)
	}

	pkiMountPath := cfg.PKIPath + "/"
	if _, ok := mounts[pkiMountPath]; !ok {
		t.Skipf("Vault PKI secrets engine not enabled at '%s'. Run setup-vault-pki.sh first.", cfg.PKIPath)
	}

	// Check if the PKI role exists
	rolePath := fmt.Sprintf("%s/roles/%s", cfg.PKIPath, cfg.PKIRole)
	secret, err := vaultClient.Logical().Read(rolePath)
	if err != nil || secret == nil {
		t.Skipf("Vault PKI role '%s' not found. Run setup-vault-pki.sh first.", cfg.PKIRole)
	}
}

// TestIntegration_VaultPKI_IssueCertificate tests issuing a certificate via Vault PKI
func TestIntegration_VaultPKI_IssueCertificate(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serviceName := testutils.GenerateTestName("webhook")
	namespace := "default"
	commonName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	// Issue certificate via Vault PKI
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
	require.NoError(t, err, "Failed to issue certificate from Vault PKI")
	require.NotNil(t, secret, "Vault PKI returned nil secret")
	require.NotNil(t, secret.Data, "Vault PKI returned nil data")

	// Verify certificate is present
	certPEM, ok := secret.Data["certificate"].(string)
	require.True(t, ok, "Certificate not found in response")
	assert.NotEmpty(t, certPEM, "Certificate is empty")

	// Verify private key is present
	keyPEM, ok := secret.Data["private_key"].(string)
	require.True(t, ok, "Private key not found in response")
	assert.NotEmpty(t, keyPEM, "Private key is empty")

	// Verify issuing CA is present
	caPEM, ok := secret.Data["issuing_ca"].(string)
	require.True(t, ok, "Issuing CA not found in response")
	assert.NotEmpty(t, caPEM, "Issuing CA is empty")

	// Verify serial number is present
	serialNumber, ok := secret.Data["serial_number"].(string)
	require.True(t, ok, "Serial number not found in response")
	assert.NotEmpty(t, serialNumber, "Serial number is empty")

	// Parse and verify the certificate
	block, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, block, "Failed to decode certificate PEM")

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err, "Failed to parse certificate")

	// Verify common name
	assert.Equal(t, commonName, cert.Subject.CommonName, "Common name mismatch")

	// Verify certificate is not expired
	assert.True(t, time.Now().Before(cert.NotAfter), "Certificate is already expired")
	assert.True(t, time.Now().After(cert.NotBefore), "Certificate is not yet valid")

	t.Logf("Successfully issued certificate with serial: %s, expires: %v", serialNumber, cert.NotAfter)
}

// TestIntegration_VaultPKI_CertificateContainsSANs tests that issued certificates contain correct SANs
func TestIntegration_VaultPKI_CertificateContainsSANs(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serviceName := testutils.GenerateTestName("webhook")
	namespace := "test-ns"
	commonName := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	// Note: SANs must match allowed_domains in the PKI role (*.svc, *.svc.cluster.local)
	expectedDNSNames := []string{
		fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
	}

	// Issue certificate with SANs
	path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
	data := map[string]interface{}{
		"common_name": commonName,
		"alt_names":   strings.Join(expectedDNSNames, ","),
		"ip_sans":     "127.0.0.1,10.0.0.1",
		"ttl":         "1h",
		"format":      "pem",
	}

	secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
	require.NoError(t, err, "Failed to issue certificate")
	require.NotNil(t, secret)

	certPEM := secret.Data["certificate"].(string)
	block, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Verify DNS SANs
	for _, expectedDNS := range expectedDNSNames {
		found := false
		for _, dns := range cert.DNSNames {
			if dns == expectedDNS {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected DNS SAN '%s' not found in certificate. Got: %v", expectedDNS, cert.DNSNames)
	}

	// Verify IP SANs
	assert.NotEmpty(t, cert.IPAddresses, "Certificate should have IP SANs")
	foundLocalhost := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "127.0.0.1" {
			foundLocalhost = true
			break
		}
	}
	assert.True(t, foundLocalhost, "Expected IP SAN 127.0.0.1 not found")

	t.Logf("Certificate contains DNS SANs: %v, IP SANs: %v", cert.DNSNames, cert.IPAddresses)
}

// TestIntegration_VaultPKI_CertificateHasCorrectTTL tests that certificate TTL matches requested
func TestIntegration_VaultPKI_CertificateHasCorrectTTL(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testCases := []struct {
		name         string
		requestedTTL string
		expectedMin  time.Duration
		expectedMax  time.Duration
	}{
		{
			name:         "1 hour TTL",
			requestedTTL: "1h",
			expectedMin:  55 * time.Minute,
			expectedMax:  65 * time.Minute,
		},
		{
			name:         "24 hour TTL",
			requestedTTL: "24h",
			expectedMin:  23 * time.Hour,
			expectedMax:  25 * time.Hour,
		},
		{
			name:         "30 day TTL",
			requestedTTL: "720h",
			expectedMin:  719 * time.Hour,
			expectedMax:  721 * time.Hour,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serviceName := testutils.GenerateTestName("webhook")
			commonName := fmt.Sprintf("%s.default.svc.cluster.local", serviceName)

			path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
			data := map[string]interface{}{
				"common_name": commonName,
				"ttl":         tc.requestedTTL,
				"format":      "pem",
			}

			secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
			require.NoError(t, err)
			require.NotNil(t, secret)

			certPEM := secret.Data["certificate"].(string)
			block, _ := pem.Decode([]byte(certPEM))
			require.NotNil(t, block)

			cert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			// Calculate actual TTL
			actualTTL := cert.NotAfter.Sub(cert.NotBefore)

			assert.GreaterOrEqual(t, actualTTL, tc.expectedMin,
				"Certificate TTL %v is less than expected minimum %v", actualTTL, tc.expectedMin)
			assert.LessOrEqual(t, actualTTL, tc.expectedMax,
				"Certificate TTL %v is greater than expected maximum %v", actualTTL, tc.expectedMax)

			t.Logf("Requested TTL: %s, Actual TTL: %v", tc.requestedTTL, actualTTL)
		})
	}
}

// TestIntegration_VaultPKI_MultipleCertificateIssuance tests that each certificate has a unique serial
func TestIntegration_VaultPKI_MultipleCertificateIssuance(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	numCerts := 5
	serialNumbers := make(map[string]bool)
	var mu sync.Mutex

	// Issue multiple certificates concurrently
	var wg sync.WaitGroup
	errors := make(chan error, numCerts)

	for i := 0; i < numCerts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			serviceName := testutils.GenerateTestName(fmt.Sprintf("webhook-%d", index))
			commonName := fmt.Sprintf("%s.default.svc.cluster.local", serviceName)

			path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
			data := map[string]interface{}{
				"common_name": commonName,
				"ttl":         "1h",
				"format":      "pem",
			}

			secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
			if err != nil {
				errors <- fmt.Errorf("failed to issue certificate %d: %w", index, err)
				return
			}

			serialNumber, ok := secret.Data["serial_number"].(string)
			if !ok {
				errors <- fmt.Errorf("serial number not found for certificate %d", index)
				return
			}

			mu.Lock()
			if serialNumbers[serialNumber] {
				errors <- fmt.Errorf("duplicate serial number found: %s", serialNumber)
			}
			serialNumbers[serialNumber] = true
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify we got the expected number of unique serial numbers
	assert.Equal(t, numCerts, len(serialNumbers), "Expected %d unique serial numbers, got %d", numCerts, len(serialNumbers))

	t.Logf("Successfully issued %d certificates with unique serial numbers", len(serialNumbers))
}

// TestIntegration_VaultPKI_CACertificateRetrieval tests that CA certificate can be retrieved
func TestIntegration_VaultPKI_CACertificateRetrieval(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get CA certificate
	caPath := fmt.Sprintf("%s/ca/pem", cfg.PKIPath)
	secret, err := vaultClient.Logical().ReadWithContext(ctx, caPath)

	// Note: The CA endpoint returns raw PEM, not a secret
	// We need to use the raw API
	caPEM, err := vaultClient.Logical().ReadRawWithContext(ctx, fmt.Sprintf("/v1/%s/ca/pem", cfg.PKIPath))
	if err != nil {
		// Try alternative method
		secret, err = vaultClient.Logical().ReadWithContext(ctx, fmt.Sprintf("%s/cert/ca", cfg.PKIPath))
		require.NoError(t, err, "Failed to read CA certificate")
		require.NotNil(t, secret)

		caPEMStr, ok := secret.Data["certificate"].(string)
		require.True(t, ok, "CA certificate not found")

		// Parse CA certificate
		block, _ := pem.Decode([]byte(caPEMStr))
		require.NotNil(t, block, "Failed to decode CA certificate PEM")

		caCert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err, "Failed to parse CA certificate")

		// Verify it's a CA certificate
		assert.True(t, caCert.IsCA, "Certificate should be a CA")
		assert.True(t, caCert.BasicConstraintsValid, "CA should have valid basic constraints")

		t.Logf("CA Certificate Subject: %s, Expires: %v", caCert.Subject.CommonName, caCert.NotAfter)
		return
	}

	// If raw read succeeded
	require.NotNil(t, caPEM)
	defer caPEM.Body.Close()

	t.Log("Successfully retrieved CA certificate")
}

// TestIntegration_VaultPKI_InvalidRole tests error handling for invalid PKI role
func TestIntegration_VaultPKI_InvalidRole(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serviceName := testutils.GenerateTestName("webhook")
	commonName := fmt.Sprintf("%s.default.svc.cluster.local", serviceName)

	// Try to issue certificate with non-existent role
	invalidRole := "non-existent-role-" + testutils.GenerateTestName("role")
	path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, invalidRole)
	data := map[string]interface{}{
		"common_name": commonName,
		"ttl":         "1h",
		"format":      "pem",
	}

	secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)

	// Should fail with an error
	assert.Error(t, err, "Expected error when using invalid role")
	assert.Nil(t, secret, "Expected nil secret when using invalid role")

	t.Logf("Correctly received error for invalid role: %v", err)
}

// TestIntegration_VaultPKI_InvalidCommonName tests error handling for invalid common name
func TestIntegration_VaultPKI_InvalidCommonName(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to issue certificate with invalid common name (not matching allowed_domains)
	// The role is configured to allow only *.svc and *.svc.cluster.local domains
	invalidCommonName := "invalid.example.com"

	path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
	data := map[string]interface{}{
		"common_name": invalidCommonName,
		"ttl":         "1h",
		"format":      "pem",
	}

	secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)

	// Should fail because the common name doesn't match allowed domains
	assert.Error(t, err, "Expected error when using invalid common name")
	assert.Nil(t, secret, "Expected nil secret when using invalid common name")

	t.Logf("Correctly received error for invalid common name: %v", err)
}

// TestIntegration_VaultPKI_RetryOnFailure tests retry logic for transient failures
func TestIntegration_VaultPKI_RetryOnFailure(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()

	// Test that we can successfully issue a certificate after simulating retries
	// This test verifies the retry pattern works by issuing multiple certificates
	// in sequence with short delays

	maxRetries := 3
	var lastErr error
	var success bool

	for attempt := 0; attempt < maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		serviceName := testutils.GenerateTestName("webhook")
		commonName := fmt.Sprintf("%s.default.svc.cluster.local", serviceName)

		path := fmt.Sprintf("%s/issue/%s", cfg.PKIPath, cfg.PKIRole)
		data := map[string]interface{}{
			"common_name": commonName,
			"ttl":         "1h",
			"format":      "pem",
		}

		secret, err := vaultClient.Logical().WriteWithContext(ctx, path, data)
		cancel()

		if err != nil {
			lastErr = err
			t.Logf("Attempt %d failed: %v", attempt+1, err)
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond) // Exponential backoff simulation
			continue
		}

		if secret != nil && secret.Data["certificate"] != nil {
			success = true
			t.Logf("Successfully issued certificate on attempt %d", attempt+1)
			break
		}
	}

	assert.True(t, success, "Failed to issue certificate after %d attempts: %v", maxRetries, lastErr)
}

// TestIntegration_VaultPKI_CertificateChain tests that the certificate chain is valid
func TestIntegration_VaultPKI_CertificateChain(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serviceName := testutils.GenerateTestName("webhook")
	commonName := fmt.Sprintf("%s.default.svc.cluster.local", serviceName)

	// Issue certificate
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
	caPEM := secret.Data["issuing_ca"].(string)

	// Parse leaf certificate
	certBlock, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, certBlock)
	leafCert, err := x509.ParseCertificate(certBlock.Bytes)
	require.NoError(t, err)

	// Parse CA certificate
	caBlock, _ := pem.Decode([]byte(caPEM))
	require.NotNil(t, caBlock)
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	require.NoError(t, err)

	// Create certificate pool with CA
	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	// Verify the certificate chain
	opts := x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: time.Now(),
	}

	chains, err := leafCert.Verify(opts)
	require.NoError(t, err, "Certificate chain verification failed")
	assert.NotEmpty(t, chains, "Expected at least one valid certificate chain")

	t.Logf("Certificate chain verified successfully. Leaf: %s, CA: %s",
		leafCert.Subject.CommonName, caCert.Subject.CommonName)
}

// TestIntegration_VaultPKI_KeyUsage tests that certificates have correct key usage
func TestIntegration_VaultPKI_KeyUsage(t *testing.T) {
	skipIfNoVaultPKI(t)

	cfg := getVaultPKITestConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serviceName := testutils.GenerateTestName("webhook")
	commonName := fmt.Sprintf("%s.default.svc.cluster.local", serviceName)

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
	block, _ := pem.Decode([]byte(certPEM))
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Verify key usage includes digital signature and key encipherment
	assert.True(t, cert.KeyUsage&x509.KeyUsageDigitalSignature != 0,
		"Certificate should have DigitalSignature key usage")
	assert.True(t, cert.KeyUsage&x509.KeyUsageKeyEncipherment != 0,
		"Certificate should have KeyEncipherment key usage")

	// Verify extended key usage includes server auth
	hasServerAuth := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			hasServerAuth = true
			break
		}
	}
	assert.True(t, hasServerAuth, "Certificate should have ServerAuth extended key usage")

	t.Logf("Certificate key usage: %v, Extended key usage: %v", cert.KeyUsage, cert.ExtKeyUsage)
}
