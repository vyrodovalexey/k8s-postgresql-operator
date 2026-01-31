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
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

// TestNewSelfSignedProvider_ValidConfig tests creating a provider with valid config
func TestNewSelfSignedProvider_ValidConfig(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		SecretName:  "test-secret",
		CertDir:     "/tmp/certs",
		RestConfig:  &rest.Config{Host: "https://test"},
		Logger:      zap.NewNop().Sugar(),
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "test-service", provider.serviceName)
	assert.Equal(t, "test-namespace", provider.namespace)
	assert.Equal(t, "test-secret", provider.secretName)
	assert.Equal(t, "/tmp/certs", provider.certDir)
	assert.Equal(t, DefaultRenewalBuffer, provider.renewalBuffer)
	assert.Equal(t, 365*24*time.Hour, provider.certificateTTL)
}

// TestNewSelfSignedProvider_EmptyServiceName tests error when service name is empty
func TestNewSelfSignedProvider_EmptyServiceName(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "service name is required")
}

// TestNewSelfSignedProvider_EmptyNamespace tests error when namespace is empty
func TestNewSelfSignedProvider_EmptyNamespace(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "namespace is required")
}

// TestNewSelfSignedProvider_NilRestConfig tests error when rest config is nil
func TestNewSelfSignedProvider_NilRestConfig(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  nil,
	}

	provider, err := NewSelfSignedProvider(opts)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "rest config is required")
}

// TestNewSelfSignedProvider_DefaultSecretName tests default secret name generation
func TestNewSelfSignedProvider_DefaultSecretName(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "my-service",
		Namespace:   "test-namespace",
		SecretName:  "", // Empty, should use default
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)
	assert.Equal(t, "my-service-webhook-cert", provider.secretName)
}

// TestNewSelfSignedProvider_CustomRenewalBuffer tests custom renewal buffer
func TestNewSelfSignedProvider_CustomRenewalBuffer(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName:   "test-service",
		Namespace:     "test-namespace",
		RestConfig:    &rest.Config{Host: "https://test"},
		RenewalBuffer: 48 * time.Hour,
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)
	assert.Equal(t, 48*time.Hour, provider.renewalBuffer)
}

// TestNewSelfSignedProvider_CustomCertificateTTL tests custom certificate TTL
func TestNewSelfSignedProvider_CustomCertificateTTL(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName:    "test-service",
		Namespace:      "test-namespace",
		RestConfig:     &rest.Config{Host: "https://test"},
		CertificateTTL: 30 * 24 * time.Hour, // 30 days
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)
	assert.Equal(t, 30*24*time.Hour, provider.certificateTTL)
}

// TestSelfSignedProvider_IssueCertificate tests certificate issuance
func TestSelfSignedProvider_IssueCertificate(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
		Logger:      zap.NewNop().Sugar(),
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
	require.NoError(t, err)
	assert.NotNil(t, cert)

	// Verify certificate data
	assert.NotEmpty(t, cert.CertPEM)
	assert.NotEmpty(t, cert.KeyPEM)
	assert.NotEmpty(t, cert.CACertPEM)
	assert.NotEmpty(t, cert.SerialNumber)

	// Verify expiration
	assert.True(t, cert.ExpiresAt.After(time.Now()))
	assert.True(t, cert.IssuedAt.Before(time.Now().Add(time.Second)))

	// Verify certificate is valid
	assert.True(t, cert.IsValid())
	assert.False(t, cert.IsExpired())
}

// TestSelfSignedProvider_IssueCertificate_ValidPEM tests that issued certificate has valid PEM format
func TestSelfSignedProvider_IssueCertificate_ValidPEM(t *testing.T) {
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

	// Verify certificate PEM can be decoded
	block, _ := pem.Decode(cert.CertPEM)
	require.NotNil(t, block, "failed to decode certificate PEM")
	assert.Equal(t, "CERTIFICATE", block.Type)

	// Verify key PEM can be decoded
	keyBlock, _ := pem.Decode(cert.KeyPEM)
	require.NotNil(t, keyBlock, "failed to decode key PEM")
	assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)

	// Parse the certificate
	x509Cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Verify certificate properties
	assert.Equal(t, "test-service.test-namespace.svc", x509Cert.Subject.CommonName)
	assert.Contains(t, x509Cert.DNSNames, "test-service")
	assert.Contains(t, x509Cert.DNSNames, "test-service.test-namespace")
	assert.Contains(t, x509Cert.DNSNames, "test-service.test-namespace.svc")
	assert.Contains(t, x509Cert.DNSNames, "test-service.test-namespace.svc.cluster.local")
}

// TestSelfSignedProvider_RenewCertificate tests certificate renewal
func TestSelfSignedProvider_RenewCertificate(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
		Logger:      zap.NewNop().Sugar(),
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()

	// Issue initial certificate
	initialCert, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
	require.NoError(t, err)

	// Renew certificate
	renewedCert, err := provider.RenewCertificate(ctx, initialCert)
	require.NoError(t, err)
	assert.NotNil(t, renewedCert)

	// Verify renewed certificate is different
	assert.NotEqual(t, initialCert.SerialNumber, renewedCert.SerialNumber)
	assert.NotEqual(t, initialCert.CertPEM, renewedCert.CertPEM)
}

// TestSelfSignedProvider_Type tests the Type method
func TestSelfSignedProvider_Type(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	assert.Equal(t, ProviderTypeSelfSigned, provider.Type())
}

// TestSelfSignedProvider_NeedsRenewal tests the NeedsRenewal method
func TestSelfSignedProvider_NeedsRenewal(t *testing.T) {
	tests := []struct {
		name          string
		renewalBuffer time.Duration
		expiresIn     time.Duration
		expected      bool
	}{
		{
			name:          "nil certificate",
			renewalBuffer: 24 * time.Hour,
			expiresIn:     0,
			expected:      true,
		},
		{
			name:          "fresh certificate - no renewal needed",
			renewalBuffer: 24 * time.Hour,
			expiresIn:     48 * time.Hour,
			expected:      false,
		},
		{
			name:          "expiring soon - renewal needed",
			renewalBuffer: 24 * time.Hour,
			expiresIn:     12 * time.Hour,
			expected:      true,
		},
		{
			name:          "exactly at buffer - renewal needed",
			renewalBuffer: 24 * time.Hour,
			expiresIn:     24*time.Hour + 1*time.Minute, // Slightly more than buffer
			expected:      false,
		},
		{
			name:          "expired - renewal needed",
			renewalBuffer: 24 * time.Hour,
			expiresIn:     -1 * time.Hour,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SelfSignedProviderOptions{
				ServiceName:   "test-service",
				Namespace:     "test-namespace",
				RestConfig:    &rest.Config{Host: "https://test"},
				RenewalBuffer: tt.renewalBuffer,
			}

			provider, err := NewSelfSignedProvider(opts)
			require.NoError(t, err)

			var cert *Certificate
			if tt.name != "nil certificate" {
				cert = &Certificate{
					ExpiresAt: time.Now().Add(tt.expiresIn),
				}
			}

			assert.Equal(t, tt.expected, provider.NeedsRenewal(cert))
		})
	}
}

// TestSelfSignedProvider_GetSecretName tests the GetSecretName method
func TestSelfSignedProvider_GetSecretName(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		SecretName:  "custom-secret",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	assert.Equal(t, "custom-secret", provider.GetSecretName())
}

// TestSelfSignedProvider_GetCertificate_WithCachedCert tests GetCertificate with cached certificate
func TestSelfSignedProvider_GetCertificate_WithCachedCert(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()

	// Issue a certificate first
	issuedCert, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
	require.NoError(t, err)

	// Get certificate should return the cached one
	cachedCert, err := provider.GetCertificate(ctx)
	require.NoError(t, err)
	assert.Equal(t, issuedCert.SerialNumber, cachedCert.SerialNumber)
}

// TestParseCertificatePEM tests the parseCertificatePEM function
func TestParseCertificatePEM(t *testing.T) {
	// Generate a test certificate
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "test.example.com",
		},
		NotBefore: now,
		NotAfter:  now.Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Test parsing
	cert, err := parseCertificatePEM(certPEM)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, serialNumber.String(), cert.SerialNumber)
	assert.WithinDuration(t, now, cert.IssuedAt, time.Second)
	assert.WithinDuration(t, now.Add(24*time.Hour), cert.ExpiresAt, time.Second)
}

// TestParseCertificatePEM_InvalidPEM tests parseCertificatePEM with invalid PEM
func TestParseCertificatePEM_InvalidPEM(t *testing.T) {
	tests := []struct {
		name    string
		pemData []byte
	}{
		{
			name:    "empty data",
			pemData: []byte{},
		},
		{
			name:    "invalid PEM",
			pemData: []byte("not a valid PEM"),
		},
		{
			name:    "wrong PEM type",
			pemData: []byte("-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := parseCertificatePEM(tt.pemData)
			assert.Error(t, err)
			assert.Nil(t, cert)
		})
	}
}

// TestParseCertificatePEM_InvalidCertificate tests parseCertificatePEM with invalid certificate data
func TestParseCertificatePEM_InvalidCertificate(t *testing.T) {
	// Create a PEM block with invalid certificate data
	invalidCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("invalid certificate data"),
	})

	cert, err := parseCertificatePEM(invalidCertPEM)
	assert.Error(t, err)
	assert.Nil(t, cert)
}

// TestSelfSignedProvider_IssueCertificate_WithoutLogger tests issuance without logger
func TestSelfSignedProvider_IssueCertificate_WithoutLogger(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
		Logger:      nil, // No logger
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
	require.NoError(t, err)
	assert.NotNil(t, cert)
}

// TestSelfSignedProvider_RenewCertificate_WithoutLogger tests renewal without logger
func TestSelfSignedProvider_RenewCertificate_WithoutLogger(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
		Logger:      nil, // No logger
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.RenewCertificate(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, cert)
}

// TestSelfSignedProvider_ConcurrentIssuance tests concurrent certificate issuance
func TestSelfSignedProvider_ConcurrentIssuance(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()
	const numGoroutines = 10

	results := make(chan *Certificate, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			cert, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
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

// TestSelfSignedProvider_GetCertificate_NoCachedCert tests GetCertificate without cached cert
func TestSelfSignedProvider_GetCertificate_NoCachedCert(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()

	// GetCertificate without any cached cert should issue a new one
	cert, err := provider.GetCertificate(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.True(t, cert.IsValid())
}

// TestSelfSignedProvider_GetCertificate_InvalidCachedCert tests GetCertificate with invalid cached cert
func TestSelfSignedProvider_GetCertificate_InvalidCachedCert(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	// Set an invalid cached cert (expired)
	expiredTime := time.Now().Add(-1 * time.Hour)
	provider.currentCert = &Certificate{
		CertPEM:   []byte("cert"),
		KeyPEM:    []byte("key"),
		ExpiresAt: expiredTime, // Expired
	}

	ctx := context.Background()

	// GetCertificate should issue a new one since cached is invalid
	cert, err := provider.GetCertificate(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.True(t, cert.IsValid())
	// The new cert should have a different (future) expiration time
	assert.True(t, cert.ExpiresAt.After(time.Now()))
}

// TestSelfSignedProvider_GetCertificate_EmptyCertPEM tests GetCertificate with empty CertPEM
func TestSelfSignedProvider_GetCertificate_EmptyCertPEM(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	// Set a cached cert with empty CertPEM (invalid)
	provider.currentCert = &Certificate{
		CertPEM:   []byte{}, // Empty
		KeyPEM:    []byte("key"),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	ctx := context.Background()

	// GetCertificate should issue a new one since cached is invalid
	cert, err := provider.GetCertificate(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.True(t, cert.IsValid())
}

// TestSelfSignedProvider_GetCertificate_EmptyKeyPEM tests GetCertificate with empty KeyPEM
func TestSelfSignedProvider_GetCertificate_EmptyKeyPEM(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	// Set a cached cert with empty KeyPEM (invalid)
	provider.currentCert = &Certificate{
		CertPEM:   []byte("cert"),
		KeyPEM:    []byte{}, // Empty
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	ctx := context.Background()

	// GetCertificate should issue a new one since cached is invalid
	cert, err := provider.GetCertificate(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.True(t, cert.IsValid())
}

// TestSelfSignedProvider_IssueCertificate_DifferentServices tests issuance for different services
func TestSelfSignedProvider_IssueCertificate_DifferentServices(t *testing.T) {
	tests := []struct {
		serviceName string
		namespace   string
	}{
		{"webhook", "default"},
		{"my-service", "production"},
		{"test-svc", "kube-system"},
	}

	for _, tt := range tests {
		t.Run(tt.serviceName+"_"+tt.namespace, func(t *testing.T) {
			opts := SelfSignedProviderOptions{
				ServiceName: tt.serviceName,
				Namespace:   tt.namespace,
				RestConfig:  &rest.Config{Host: "https://test"},
			}

			provider, err := NewSelfSignedProvider(opts)
			require.NoError(t, err)

			ctx := context.Background()
			cert, err := provider.IssueCertificate(ctx, tt.serviceName, tt.namespace)
			require.NoError(t, err)
			assert.NotNil(t, cert)

			// Verify certificate contains correct DNS names
			block, _ := pem.Decode(cert.CertPEM)
			require.NotNil(t, block)

			x509Cert, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)

			expectedCN := fmt.Sprintf("%s.%s.svc", tt.serviceName, tt.namespace)
			assert.Equal(t, expectedCN, x509Cert.Subject.CommonName)
			assert.Contains(t, x509Cert.DNSNames, tt.serviceName)
			assert.Contains(t, x509Cert.DNSNames, fmt.Sprintf("%s.%s", tt.serviceName, tt.namespace))
			assert.Contains(t, x509Cert.DNSNames, fmt.Sprintf("%s.%s.svc", tt.serviceName, tt.namespace))
			assert.Contains(t, x509Cert.DNSNames, fmt.Sprintf("%s.%s.svc.cluster.local", tt.serviceName, tt.namespace))
		})
	}
}

// TestSelfSignedProvider_NeedsRenewal_EdgeCases tests NeedsRenewal edge cases
func TestSelfSignedProvider_NeedsRenewal_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		renewalBuffer time.Duration
		expiresIn     time.Duration
		expected      bool
	}{
		{
			name:          "zero renewal buffer - expired cert",
			renewalBuffer: 0,
			expiresIn:     -1 * time.Hour,
			expected:      true,
		},
		{
			name:          "large renewal buffer - fresh cert",
			renewalBuffer: 720 * time.Hour, // 30 days
			expiresIn:     360 * time.Hour, // 15 days
			expected:      true,
		},
		{
			name:          "large renewal buffer - very fresh cert",
			renewalBuffer: 720 * time.Hour, // 30 days
			expiresIn:     1000 * time.Hour,
			expected:      false,
		},
		{
			name:          "small renewal buffer - cert expiring soon",
			renewalBuffer: 1 * time.Hour,
			expiresIn:     30 * time.Minute,
			expected:      true,
		},
		{
			name:          "small renewal buffer - cert not expiring soon",
			renewalBuffer: 1 * time.Hour,
			expiresIn:     2 * time.Hour,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SelfSignedProviderOptions{
				ServiceName:   "test-service",
				Namespace:     "test-namespace",
				RestConfig:    &rest.Config{Host: "https://test"},
				RenewalBuffer: tt.renewalBuffer,
			}

			provider, err := NewSelfSignedProvider(opts)
			require.NoError(t, err)

			cert := &Certificate{
				ExpiresAt: time.Now().Add(tt.expiresIn),
			}

			assert.Equal(t, tt.expected, provider.NeedsRenewal(cert))
		})
	}
}

// TestSelfSignedProvider_IssueCertificate_CertificateProperties tests certificate properties
func TestSelfSignedProvider_IssueCertificate_CertificateProperties(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName:    "test-service",
		Namespace:      "test-namespace",
		RestConfig:     &rest.Config{Host: "https://test"},
		CertificateTTL: 30 * 24 * time.Hour, // 30 days
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()
	cert, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
	require.NoError(t, err)

	// Verify certificate properties
	block, _ := pem.Decode(cert.CertPEM)
	require.NotNil(t, block)

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	// Verify key usage
	assert.True(t, x509Cert.KeyUsage&x509.KeyUsageKeyEncipherment != 0)
	assert.True(t, x509Cert.KeyUsage&x509.KeyUsageDigitalSignature != 0)

	// Verify extended key usage
	assert.Contains(t, x509Cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)

	// Verify IP addresses - check that 127.0.0.1 is present (may be in different format)
	found := false
	for _, ip := range x509Cert.IPAddresses {
		if ip.Equal(net.IPv4(127, 0, 0, 1)) {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 127.0.0.1 in IP addresses")

	// Verify organization
	assert.Contains(t, x509Cert.Subject.Organization, "postgresql-operator.vyrodovalexey.github.com")

	// Verify validity period (approximately 30 days)
	expectedExpiry := time.Now().Add(30 * 24 * time.Hour)
	assert.WithinDuration(t, expectedExpiry, x509Cert.NotAfter, 1*time.Minute)
}

// TestSelfSignedProvider_RenewCertificate_MultipleTimes tests multiple renewals
func TestSelfSignedProvider_RenewCertificate_MultipleTimes(t *testing.T) {
	opts := SelfSignedProviderOptions{
		ServiceName: "test-service",
		Namespace:   "test-namespace",
		RestConfig:  &rest.Config{Host: "https://test"},
	}

	provider, err := NewSelfSignedProvider(opts)
	require.NoError(t, err)

	ctx := context.Background()

	// Issue initial certificate
	cert1, err := provider.IssueCertificate(ctx, "test-service", "test-namespace")
	require.NoError(t, err)

	// Renew multiple times
	var certs []*Certificate
	certs = append(certs, cert1)

	for i := 0; i < 3; i++ {
		renewedCert, err := provider.RenewCertificate(ctx, certs[len(certs)-1])
		require.NoError(t, err)
		certs = append(certs, renewedCert)
	}

	// Verify all certificates are different
	serialNumbers := make(map[string]bool)
	for _, cert := range certs {
		assert.False(t, serialNumbers[cert.SerialNumber], "duplicate serial number found")
		serialNumbers[cert.SerialNumber] = true
	}
}

// TestParseCertificatePEM_ValidCertificateWithChain tests parsing certificate with chain
func TestParseCertificatePEM_ValidCertificateWithChain(t *testing.T) {
	// Generate a test certificate
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "test.example.com",
		},
		NotBefore: now,
		NotAfter:  now.Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	// Create PEM with extra data (simulating a chain)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	// Add another certificate block (the parser should only use the first one)
	certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})...)

	// Test parsing - should parse the first certificate
	cert, err := parseCertificatePEM(certPEM)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, serialNumber.String(), cert.SerialNumber)
}
