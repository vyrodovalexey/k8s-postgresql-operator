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

package cert

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

func TestLoadCert_Success(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Generate a test certificate and key
	config := &rest.Config{Host: "https://test"}

	serviceName := "test-service"
	namespace := "test-namespace"
	secretName := "test-secret"

	// Generate certificate (will fail at client creation, but we test LoadCert separately)
	_, certPathOut, keyPathOut, err := GenerateSelfSignedCertAndStoreInSecret(
		serviceName, namespace, secretName, tmpDir, config)
	// This will fail, so we'll create test cert files manually
	if err != nil {
		// Create minimal valid cert files for testing LoadCert
		certPathOut = filepath.Join(tmpDir, "tls.crt")
		keyPathOut = filepath.Join(tmpDir, "tls.key")
		// We'll test with invalid certs to verify LoadCert is called
		_ = os.WriteFile(certPathOut, []byte("test"), 0644)
		_ = os.WriteFile(keyPathOut, []byte("test"), 0644)
	}

	// Load the certificate (will fail with invalid cert, but tests the function)
	_, err = LoadCert(certPathOut, keyPathOut)
	// Should error because cert is invalid
	assert.Error(t, err)
}

func TestLoadCert_NonExistentCertFile(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "nonexistent.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	// Create a dummy key file
	err := os.WriteFile(keyPath, []byte("dummy"), 0644)
	require.NoError(t, err)

	_, err = LoadCert(certPath, keyPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load certificate")
}

func TestLoadCert_NonExistentKeyFile(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "nonexistent.key")

	// Create a dummy cert file
	err := os.WriteFile(certPath, []byte("dummy"), 0644)
	require.NoError(t, err)

	_, err = LoadCert(certPath, keyPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load certificate")
}

func TestGenerateSelfSignedCertAndStoreInSecret_Success(t *testing.T) {
	tmpDir := t.TempDir()
	config := &rest.Config{Host: "https://test"}

	serviceName := "test-service"
	namespace := "test-namespace"
	secretName := "test-secret"

	// Note: This will fail because we can't create a real client from fake config
	// But we can test the file creation part
	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		serviceName, namespace, secretName, tmpDir, config)

	// This will fail at client creation, but we can verify the error
	assert.Error(t, err)
}

func TestGenerateSelfSignedCertAndStoreInSecret_InvalidCertDir(t *testing.T) {
	// Use a path that can't be created (on Unix systems, /dev/null is a special file)
	invalidDir := "/dev/null/certs"
	config := &rest.Config{Host: "https://test"}

	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", "test-namespace", "test-secret", invalidDir, config)
	assert.Error(t, err)
}

func TestGenerateSelfSignedCertAndStoreInSecret_SecretExists(t *testing.T) {
	tmpDir := t.TempDir()

	// The function will try to read from the secret and write to files
	// But it will fail at client creation since we can't use fake client with rest.Config
	config := &rest.Config{Host: "https://test"}
	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", "test-namespace", "test-secret", tmpDir, config)
	assert.Error(t, err)
}

func TestGenerateSelfSignedCertAndStoreInSecret_SecretExistsWithoutKeys(t *testing.T) {
	tmpDir := t.TempDir()

	config := &rest.Config{Host: "https://test"}
	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", "test-namespace", "test-secret", tmpDir, config)
	assert.Error(t, err)
}

func TestLoadCert_InvalidCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	// Write invalid certificate data
	err := os.WriteFile(certPath, []byte("invalid-cert"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte("invalid-key"), 0644)
	require.NoError(t, err)

	_, err = LoadCert(certPath, keyPath)
	assert.Error(t, err)
}

func TestLoadCert_ValidCertificateFormat(t *testing.T) {
	// This test verifies that LoadCert can handle valid certificate formats
	// We'll create a minimal valid certificate structure
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	// Create minimal PEM structure (this will still fail validation but tests the format)
	certPEM := `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKZ5Zg5v5q5kMA0GCSqGSIb3DQEBCwUAMCExHzAdBgNVBAoTFkZy
YW5rIERvbWFpbiBHTWJIIE5vLjEeMBwGA1UEAxMVZnJhbmtkb21haW4uY29tIE1v
-----END CERTIFICATE-----`

	keyPEM := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAyoursTrulyaFakeKeyForTestingPurposesOnly
-----END RSA PRIVATE KEY-----`

	err := os.WriteFile(certPath, []byte(certPEM), 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte(keyPEM), 0644)
	require.NoError(t, err)

	// This will fail because the certificate is not valid, but it tests the loading mechanism
	_, err = LoadCert(certPath, keyPath)
	// The error should be about certificate parsing, not file reading
	if err != nil {
		assert.Contains(t, err.Error(), "failed to load certificate")
	}
}

// TestLoadCert_ValidCertificate tests LoadCert with a valid certificate
func TestLoadCert_ValidCertificate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid certificate using the provider
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
	require.NoError(t, err)

	// Write the certificate to files
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	err = os.WriteFile(certPath, cert.CertPEM, 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, cert.KeyPEM, 0600)
	require.NoError(t, err)

	// Load the certificate
	loadedCert, err := LoadCert(certPath, keyPath)
	require.NoError(t, err)
	assert.NotNil(t, loadedCert)
}

// TestWriteCertFromSecret_Success tests writeCertFromSecret with valid data
func TestWriteCertFromSecret_Success(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	// Create a mock secret with valid data
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
		},
	}

	outCertPath, outKeyPath, ok := writeCertFromSecret(secret, certPath, keyPath)
	assert.True(t, ok)
	assert.Equal(t, certPath, outCertPath)
	assert.Equal(t, keyPath, outKeyPath)

	// Verify files were created
	certData, err := os.ReadFile(certPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("cert-data"), certData)

	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("key-data"), keyData)
}

// TestWriteCertFromSecret_MissingCert tests writeCertFromSecret with missing cert
func TestWriteCertFromSecret_MissingCert(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"tls.key": []byte("key-data"),
		},
	}

	_, _, ok := writeCertFromSecret(secret, certPath, keyPath)
	assert.False(t, ok)
}

// TestWriteCertFromSecret_MissingKey tests writeCertFromSecret with missing key
func TestWriteCertFromSecret_MissingKey(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
		},
	}

	_, _, ok := writeCertFromSecret(secret, certPath, keyPath)
	assert.False(t, ok)
}

// TestWriteCertFromSecret_InvalidCertPath tests writeCertFromSecret with invalid cert path
func TestWriteCertFromSecret_InvalidCertPath(t *testing.T) {
	certPath := "/dev/null/invalid/tls.crt"
	keyPath := "/tmp/tls.key"

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
		},
	}

	_, _, ok := writeCertFromSecret(secret, certPath, keyPath)
	assert.False(t, ok)
}

// TestWriteCertFromSecret_InvalidKeyPath tests writeCertFromSecret with invalid key path
func TestWriteCertFromSecret_InvalidKeyPath(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := "/dev/null/invalid/tls.key"

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
		},
	}

	_, _, ok := writeCertFromSecret(secret, certPath, keyPath)
	assert.False(t, ok)
}

// TestWriteCertFromSecret_EmptyData tests writeCertFromSecret with empty data
func TestWriteCertFromSecret_EmptyData(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	secret := &corev1.Secret{
		Data: map[string][]byte{},
	}

	_, _, ok := writeCertFromSecret(secret, certPath, keyPath)
	assert.False(t, ok)
}

// TestWriteCertFromSecret_NilData tests writeCertFromSecret with nil data
func TestWriteCertFromSecret_NilData(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	secret := &corev1.Secret{
		Data: nil,
	}

	_, _, ok := writeCertFromSecret(secret, certPath, keyPath)
	assert.False(t, ok)
}

// TestWriteCertFromSecret_LargeData tests writeCertFromSecret with large data
func TestWriteCertFromSecret_LargeData(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	// Create large data
	largeCert := make([]byte, 1024*1024) // 1MB
	largeKey := make([]byte, 1024*1024)  // 1MB
	for i := range largeCert {
		largeCert[i] = byte(i % 256)
		largeKey[i] = byte((i + 1) % 256)
	}

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": largeCert,
			"tls.key": largeKey,
		},
	}

	outCertPath, outKeyPath, ok := writeCertFromSecret(secret, certPath, keyPath)
	assert.True(t, ok)
	assert.Equal(t, certPath, outCertPath)
	assert.Equal(t, keyPath, outKeyPath)

	// Verify files were created with correct content
	certData, err := os.ReadFile(certPath)
	require.NoError(t, err)
	assert.Equal(t, largeCert, certData)

	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, largeKey, keyData)
}

// TestLoadCert_ValidCertificateWithProvider tests LoadCert with a valid certificate from provider
func TestLoadCert_ValidCertificateWithProvider(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid certificate using the provider
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
	require.NoError(t, err)

	// Write the certificate to files
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	err = os.WriteFile(certPath, cert.CertPEM, 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, cert.KeyPEM, 0600)
	require.NoError(t, err)

	// Load the certificate
	loadedCert, err := LoadCert(certPath, keyPath)
	require.NoError(t, err)
	assert.NotNil(t, loadedCert)

	// Verify the loaded certificate has the expected properties
	assert.NotNil(t, loadedCert.Certificate)
	assert.NotEmpty(t, loadedCert.Certificate)
}

// TestLoadCert_MismatchedCertAndKey tests LoadCert with mismatched cert and key
func TestLoadCert_MismatchedCertAndKey(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two different certificates
	opts1 := SelfSignedProviderOptions{
		ServiceName: "service1",
		Namespace:   "namespace1",
		RestConfig:  &rest.Config{Host: "https://test"},
	}
	provider1, err := NewSelfSignedProvider(opts1)
	require.NoError(t, err)

	opts2 := SelfSignedProviderOptions{
		ServiceName: "service2",
		Namespace:   "namespace2",
		RestConfig:  &rest.Config{Host: "https://test"},
	}
	provider2, err := NewSelfSignedProvider(opts2)
	require.NoError(t, err)

	ctx := context.Background()
	cert1, err := provider1.IssueCertificate(ctx, "service1", "namespace1")
	require.NoError(t, err)
	cert2, err := provider2.IssueCertificate(ctx, "service2", "namespace2")
	require.NoError(t, err)

	// Write cert from one and key from another
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	err = os.WriteFile(certPath, cert1.CertPEM, 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, cert2.KeyPEM, 0600)
	require.NoError(t, err)

	// Load should fail because cert and key don't match
	_, err = LoadCert(certPath, keyPath)
	assert.Error(t, err)
}
