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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// createTestCertificateFiles creates test certificate and key files for testing
func createTestCertificateFiles(t *testing.T, dir string, expiresIn time.Duration) (certPath, keyPath string) {
	t.Helper()

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "test.example.com",
			Organization: []string{"Test Org"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(expiresIn),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"test.example.com"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Encode private key to PEM
	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyDER})

	// Write files
	certPath = filepath.Join(dir, "tls.crt")
	keyPath = filepath.Join(dir, "tls.key")

	err = os.WriteFile(certPath, certPEM, 0644)
	require.NoError(t, err)

	err = os.WriteFile(keyPath, keyPEM, 0600)
	require.NoError(t, err)

	return certPath, keyPath
}

// TestNewProvidedProvider_ValidConfig tests creating a provider with valid config
func TestNewProvidedProvider_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, zap.NewNop().Sugar())
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, certPath, provider.certPath)
	assert.Equal(t, keyPath, provider.keyPath)
}

// TestNewProvidedProvider_EmptyCertPath tests error when cert path is empty
func TestNewProvidedProvider_EmptyCertPath(t *testing.T) {
	provider, err := NewProvidedProvider("", "/some/key/path", nil)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "certificate path is required")
}

// TestNewProvidedProvider_EmptyKeyPath tests error when key path is empty
func TestNewProvidedProvider_EmptyKeyPath(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	err := os.WriteFile(certPath, []byte("cert"), 0644)
	require.NoError(t, err)

	provider, err := NewProvidedProvider(certPath, "", nil)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "key path is required")
}

// TestNewProvidedProvider_CertFileNotExist tests error when cert file doesn't exist
func TestNewProvidedProvider_CertFileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "tls.key")
	err := os.WriteFile(keyPath, []byte("key"), 0600)
	require.NoError(t, err)

	provider, err := NewProvidedProvider("/nonexistent/cert.pem", keyPath, nil)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "certificate file does not exist")
}

// TestNewProvidedProvider_KeyFileNotExist tests error when key file doesn't exist
func TestNewProvidedProvider_KeyFileNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	err := os.WriteFile(certPath, []byte("cert"), 0644)
	require.NoError(t, err)

	provider, err := NewProvidedProvider(certPath, "/nonexistent/key.pem", nil)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "key file does not exist")
}

// TestProvidedProvider_IssueCertificate tests loading certificate via IssueCertificate
func TestProvidedProvider_IssueCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, zap.NewNop().Sugar())
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.IssueCertificate(ctx, "ignored-service", "ignored-namespace")
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.NotEmpty(t, cert.CertPEM)
	assert.NotEmpty(t, cert.KeyPEM)
	assert.NotEmpty(t, cert.SerialNumber)
	assert.True(t, cert.IsValid())
}

// TestProvidedProvider_RenewCertificate tests that renewal returns error
func TestProvidedProvider_RenewCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.RenewCertificate(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "renewal not supported for provided certificates")
}

// TestProvidedProvider_GetCertificate tests GetCertificate method
func TestProvidedProvider_GetCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, zap.NewNop().Sugar())
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.GetCertificate(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.True(t, cert.IsValid())
}

// TestProvidedProvider_GetCertificate_Cached tests that GetCertificate returns cached cert
func TestProvidedProvider_GetCertificate_Cached(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// First call loads the certificate
	cert1, err := provider.GetCertificate(ctx)
	require.NoError(t, err)

	// Second call should return cached certificate
	cert2, err := provider.GetCertificate(ctx)
	require.NoError(t, err)

	assert.Equal(t, cert1.SerialNumber, cert2.SerialNumber)
}

// TestProvidedProvider_Type tests the Type method
func TestProvidedProvider_Type(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeProvided, provider.Type())
}

// TestProvidedProvider_NeedsRenewal tests the NeedsRenewal method
func TestProvidedProvider_NeedsRenewal(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, zap.NewNop().Sugar())
	require.NoError(t, err)

	tests := []struct {
		name     string
		cert     *Certificate
		expected bool
	}{
		{
			name:     "nil certificate",
			cert:     nil,
			expected: true,
		},
		{
			name: "fresh certificate",
			cert: &Certificate{
				ExpiresAt: time.Now().Add(48 * time.Hour),
			},
			expected: false,
		},
		{
			name: "expiring soon",
			cert: &Certificate{
				ExpiresAt: time.Now().Add(12 * time.Hour),
			},
			expected: true,
		},
		{
			name: "expired",
			cert: &Certificate{
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, provider.NeedsRenewal(tt.cert))
		})
	}
}

// TestProvidedProvider_GetCertPath tests the GetCertPath method
func TestProvidedProvider_GetCertPath(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	assert.Equal(t, certPath, provider.GetCertPath())
}

// TestProvidedProvider_GetKeyPath tests the GetKeyPath method
func TestProvidedProvider_GetKeyPath(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	assert.Equal(t, keyPath, provider.GetKeyPath())
}

// TestProvidedProvider_LoadCertificate_InvalidCert tests loading invalid certificate
func TestProvidedProvider_LoadCertificate_InvalidCert(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")

	// Write invalid certificate data
	err := os.WriteFile(certPath, []byte("invalid cert"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte("invalid key"), 0600)
	require.NoError(t, err)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.GetCertificate(ctx)
	assert.Error(t, err)
	assert.Nil(t, cert)
}

// TestProvidedProvider_LoadCertificate_CertReadError tests error when cert file can't be read
func TestProvidedProvider_LoadCertificate_CertReadError(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	// Remove the cert file after provider creation
	err = os.Remove(certPath)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.GetCertificate(ctx)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "failed to read certificate file")
}

// TestProvidedProvider_LoadCertificate_KeyReadError tests error when key file can't be read
func TestProvidedProvider_LoadCertificate_KeyReadError(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	// Remove the key file after provider creation
	err = os.Remove(keyPath)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.GetCertificate(ctx)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "failed to read key file")
}

// TestProvidedProvider_NeedsRenewal_WithLogger tests NeedsRenewal logs warning
func TestProvidedProvider_NeedsRenewal_WithLogger(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	// Use a real logger to verify no panics
	logger := zap.NewNop().Sugar()
	provider, err := NewProvidedProvider(certPath, keyPath, logger)
	require.NoError(t, err)

	// Certificate expiring soon should trigger warning log
	cert := &Certificate{
		ExpiresAt: time.Now().Add(12 * time.Hour),
	}

	needsRenewal := provider.NeedsRenewal(cert)
	assert.True(t, needsRenewal)
}

// TestProvidedProvider_NeedsRenewal_WithoutLogger tests NeedsRenewal without logger
func TestProvidedProvider_NeedsRenewal_WithoutLogger(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	// Certificate expiring soon - should not panic without logger
	cert := &Certificate{
		ExpiresAt: time.Now().Add(12 * time.Hour),
	}

	needsRenewal := provider.NeedsRenewal(cert)
	assert.True(t, needsRenewal)
}

// TestProvidedProvider_ConcurrentAccess tests concurrent access to provider
func TestProvidedProvider_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	provider, err := NewProvidedProvider(certPath, keyPath, nil)
	require.NoError(t, err)

	ctx := context.Background()
	const numGoroutines = 10

	results := make(chan *Certificate, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			cert, err := provider.GetCertificate(ctx)
			if err != nil {
				errors <- err
				return
			}
			results <- cert
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case cert := <-results:
			assert.NotNil(t, cert)
			assert.True(t, cert.IsValid())
		case err := <-errors:
			t.Errorf("unexpected error: %v", err)
		}
	}
}
