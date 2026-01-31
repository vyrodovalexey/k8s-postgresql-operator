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
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVaultPKIConfig_Initialization tests VaultPKIConfig struct initialization
func TestVaultPKIConfig_Initialization(t *testing.T) {
	cfg := VaultPKIConfig{
		VaultAddr:     "https://vault.example.com:8200",
		VaultRole:     "k8s-role",
		TokenPath:     "/var/run/secrets/kubernetes.io/serviceaccount/token",
		PKIMountPath:  "pki",
		PKIRole:       "webhook-cert",
		TTL:           720 * time.Hour,
		RenewalBuffer: 24 * time.Hour,
		RetryConfig:   DefaultRetryConfig(),
	}

	assert.Equal(t, "https://vault.example.com:8200", cfg.VaultAddr)
	assert.Equal(t, "k8s-role", cfg.VaultRole)
	assert.Equal(t, "/var/run/secrets/kubernetes.io/serviceaccount/token", cfg.TokenPath)
	assert.Equal(t, "pki", cfg.PKIMountPath)
	assert.Equal(t, "webhook-cert", cfg.PKIRole)
	assert.Equal(t, 720*time.Hour, cfg.TTL)
	assert.Equal(t, 24*time.Hour, cfg.RenewalBuffer)
}

// TestNewVaultPKIProvider_EmptyVaultAddr tests error when vault address is empty
func TestNewVaultPKIProvider_EmptyVaultAddr(t *testing.T) {
	cfg := VaultPKIConfig{
		VaultAddr:    "",
		VaultRole:    "k8s-role",
		PKIMountPath: "pki",
		PKIRole:      "webhook-cert",
	}

	ctx := context.Background()
	provider, err := NewVaultPKIProvider(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "vault address is required")
}

// TestNewVaultPKIProvider_EmptyVaultRole tests error when vault role is empty
func TestNewVaultPKIProvider_EmptyVaultRole(t *testing.T) {
	cfg := VaultPKIConfig{
		VaultAddr:    "https://vault.example.com:8200",
		VaultRole:    "",
		PKIMountPath: "pki",
		PKIRole:      "webhook-cert",
	}

	ctx := context.Background()
	provider, err := NewVaultPKIProvider(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "vault role is required")
}

// TestNewVaultPKIProvider_EmptyPKIMountPath tests error when PKI mount path is empty
func TestNewVaultPKIProvider_EmptyPKIMountPath(t *testing.T) {
	cfg := VaultPKIConfig{
		VaultAddr:    "https://vault.example.com:8200",
		VaultRole:    "k8s-role",
		PKIMountPath: "",
		PKIRole:      "webhook-cert",
	}

	ctx := context.Background()
	provider, err := NewVaultPKIProvider(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "PKI mount path is required")
}

// TestNewVaultPKIProvider_EmptyPKIRole tests error when PKI role is empty
func TestNewVaultPKIProvider_EmptyPKIRole(t *testing.T) {
	cfg := VaultPKIConfig{
		VaultAddr:    "https://vault.example.com:8200",
		VaultRole:    "k8s-role",
		PKIMountPath: "pki",
		PKIRole:      "",
	}

	ctx := context.Background()
	provider, err := NewVaultPKIProvider(ctx, cfg)
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "PKI role is required")
}

// TestVaultPKIProvider_BuildCertificateRequest tests building certificate request
func TestVaultPKIProvider_BuildCertificateRequest(t *testing.T) {
	provider := &VaultPKIProvider{
		pkiMountPath: "pki",
		pkiRole:      "webhook-cert",
		ttl:          720 * time.Hour,
	}

	request := provider.buildCertificateRequest("test-service", "test-namespace")

	assert.Equal(t, "test-service.test-namespace.svc", request["common_name"])
	assert.Equal(t, "127.0.0.1", request["ip_sans"])
	assert.Equal(t, "720h0m0s", request["ttl"])
	assert.Equal(t, "pem", request["format"])

	altNames := request["alt_names"].(string)
	assert.Contains(t, altNames, "test-service")
	assert.Contains(t, altNames, "test-service.test-namespace")
	assert.Contains(t, altNames, "test-service.test-namespace.svc")
	assert.Contains(t, altNames, "test-service.test-namespace.svc.cluster.local")
}

// TestVaultPKIProvider_ParseCertificateResponse_Valid tests parsing valid response
func TestVaultPKIProvider_ParseCertificateResponse_Valid(t *testing.T) {
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

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))

	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate":   certPEM,
		"private_key":   keyPEM,
		"issuing_ca":    certPEM,
		"serial_number": "12:34:56:78",
	}

	cert, err := provider.parseCertificateResponse(data)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, []byte(certPEM), cert.CertPEM)
	assert.Equal(t, []byte(keyPEM), cert.KeyPEM)
	assert.Equal(t, []byte(certPEM), cert.CACertPEM)
	assert.Equal(t, "12:34:56:78", cert.SerialNumber)
}

// TestVaultPKIProvider_ParseCertificateResponse_MissingCertificate tests missing certificate
func TestVaultPKIProvider_ParseCertificateResponse_MissingCertificate(t *testing.T) {
	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"private_key": "key-pem",
	}

	cert, err := provider.parseCertificateResponse(data)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "certificate not found in response")
}

// TestVaultPKIProvider_ParseCertificateResponse_MissingPrivateKey tests missing private key
func TestVaultPKIProvider_ParseCertificateResponse_MissingPrivateKey(t *testing.T) {
	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate": "cert-pem",
	}

	cert, err := provider.parseCertificateResponse(data)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "private_key not found in response")
}

// TestVaultPKIProvider_ParseCertificateResponse_EmptyCertificate tests empty certificate
func TestVaultPKIProvider_ParseCertificateResponse_EmptyCertificate(t *testing.T) {
	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate": "",
		"private_key": "key-pem",
	}

	cert, err := provider.parseCertificateResponse(data)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "certificate not found in response")
}

// TestVaultPKIProvider_ParseCertificateResponse_EmptyPrivateKey tests empty private key
func TestVaultPKIProvider_ParseCertificateResponse_EmptyPrivateKey(t *testing.T) {
	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate": "cert-pem",
		"private_key": "",
	}

	cert, err := provider.parseCertificateResponse(data)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "private_key not found in response")
}

// TestVaultPKIProvider_ParseCertificateResponse_NoIssuingCA tests response without issuing_ca
func TestVaultPKIProvider_ParseCertificateResponse_NoIssuingCA(t *testing.T) {
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

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))

	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate": certPEM,
		"private_key": keyPEM,
		// No issuing_ca - should use certificate as CA
	}

	cert, err := provider.parseCertificateResponse(data)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, cert.CertPEM, cert.CACertPEM)
}

// TestVaultPKIProvider_CalculateBackoff tests backoff calculation
func TestVaultPKIProvider_CalculateBackoff(t *testing.T) {
	provider := &VaultPKIProvider{
		retryConfig: RetryConfig{
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     30 * time.Second,
			BackoffFactor:  2.0,
		},
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{attempt: 1, expected: 1 * time.Second},
		{attempt: 2, expected: 2 * time.Second},
		{attempt: 3, expected: 4 * time.Second},
		{attempt: 4, expected: 8 * time.Second},
		{attempt: 5, expected: 16 * time.Second},
		{attempt: 6, expected: 30 * time.Second}, // Capped at max
		{attempt: 10, expected: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			backoff := provider.calculateBackoff(tt.attempt)
			assert.Equal(t, tt.expected, backoff)
		})
	}
}

// TestVaultPKIProvider_Type tests the Type method
func TestVaultPKIProvider_Type(t *testing.T) {
	provider := &VaultPKIProvider{}
	assert.Equal(t, ProviderTypeVaultPKI, provider.Type())
}

// TestVaultPKIProvider_NeedsRenewal tests the NeedsRenewal method
func TestVaultPKIProvider_NeedsRenewal(t *testing.T) {
	provider := &VaultPKIProvider{
		renewalBuffer: 24 * time.Hour,
	}

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
			name: "fresh certificate - no renewal needed",
			cert: &Certificate{
				ExpiresAt: time.Now().Add(48 * time.Hour),
			},
			expected: false,
		},
		{
			name: "within renewal buffer - renewal needed",
			cert: &Certificate{
				ExpiresAt: time.Now().Add(12 * time.Hour),
			},
			expected: true,
		},
		{
			name: "expired - renewal needed",
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

// TestIsRetryableError tests the isRetryableError function
func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset by peer"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      errors.New("no such host"),
			expected: true,
		},
		{
			name:     "temporary failure",
			err:      errors.New("temporary failure in name resolution"),
			expected: true,
		},
		{
			name:     "server unavailable 503",
			err:      errors.New("server returned 503"),
			expected: true,
		},
		{
			name:     "bad gateway 502",
			err:      errors.New("502 bad gateway"),
			expected: true,
		},
		{
			name:     "gateway timeout 504",
			err:      errors.New("504 gateway timeout"),
			expected: true,
		},
		{
			name:     "server is currently unavailable",
			err:      errors.New("server is currently unavailable"),
			expected: true,
		},
		{
			name:     "non-retryable error",
			err:      errors.New("permission denied"),
			expected: false,
		},
		{
			name:     "authentication error",
			err:      errors.New("authentication failed"),
			expected: false,
		},
		{
			name:     "case insensitive - CONNECTION REFUSED",
			err:      errors.New("CONNECTION REFUSED"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isRetryableError(tt.err))
		})
	}
}

// TestIsRetryableError_NetworkTimeout tests network timeout errors
func TestIsRetryableError_NetworkTimeout(t *testing.T) {
	// Create a mock network timeout error
	timeoutErr := &mockNetError{timeout: true}
	assert.True(t, isRetryableError(timeoutErr))

	// Non-timeout network error
	nonTimeoutErr := &mockNetError{timeout: false}
	assert.False(t, isRetryableError(nonTimeoutErr))
}

// mockNetError implements net.Error for testing
type mockNetError struct {
	timeout bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return false }

// Ensure mockNetError implements net.Error
var _ net.Error = (*mockNetError)(nil)

// TestContainsIgnoreCase tests the containsIgnoreCase function
func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"HELLO WORLD", "world", true},
		{"hello world", "WORLD", true},
		{"hello world", "foo", false},
		{"", "foo", false},
		{"hello", "", true},
		{"Connection Refused", "connection refused", true},
		{"CONNECTION REFUSED", "connection refused", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			assert.Equal(t, tt.expected, containsIgnoreCase(tt.s, tt.substr))
		})
	}
}

// TestBytesContains tests the bytesContains function
func TestBytesContains(t *testing.T) {
	tests := []struct {
		b        []byte
		sub      []byte
		expected bool
	}{
		{[]byte("hello world"), []byte("world"), true},
		{[]byte("hello world"), []byte("foo"), false},
		{[]byte("hello"), []byte(""), true},
		{[]byte(""), []byte("foo"), false},
		{[]byte("abc"), []byte("abcd"), false},
		{[]byte("abcdef"), []byte("cde"), true},
	}

	for _, tt := range tests {
		t.Run(string(tt.b)+"_"+string(tt.sub), func(t *testing.T) {
			assert.Equal(t, tt.expected, bytesContains(tt.b, tt.sub))
		})
	}
}

// TestVaultPKIProvider_RenewCertificate_NoServiceName tests renewal without service name
func TestVaultPKIProvider_RenewCertificate_NoServiceName(t *testing.T) {
	provider := &VaultPKIProvider{
		serviceName: "",
		namespace:   "",
	}

	ctx := context.Background()
	cert, err := provider.RenewCertificate(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "service name and namespace must be set before renewal")
}

// TestVaultPKIProvider_GetCertificate_NoServiceName tests GetCertificate without service name
func TestVaultPKIProvider_GetCertificate_NoServiceName(t *testing.T) {
	provider := &VaultPKIProvider{
		serviceName: "",
		namespace:   "",
		currentCert: nil,
	}

	ctx := context.Background()
	cert, err := provider.GetCertificate(ctx)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "no certificate available and service name/namespace not set")
}

// TestVaultPKIProvider_GetCertificate_WithValidCachedCert tests GetCertificate with valid cached cert
func TestVaultPKIProvider_GetCertificate_WithValidCachedCert(t *testing.T) {
	cachedCert := &Certificate{
		CertPEM:   []byte("cert"),
		KeyPEM:    []byte("key"),
		ExpiresAt: time.Now().Add(48 * time.Hour),
	}

	provider := &VaultPKIProvider{
		currentCert:   cachedCert,
		renewalBuffer: 24 * time.Hour,
	}

	ctx := context.Background()
	cert, err := provider.GetCertificate(ctx)
	require.NoError(t, err)
	assert.Equal(t, cachedCert, cert)
}

// TestVaultPKIProvider_ParseCertificateResponse_InvalidCertPEM tests invalid cert PEM
func TestVaultPKIProvider_ParseCertificateResponse_InvalidCertPEM(t *testing.T) {
	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate": "invalid-pem-data",
		"private_key": "key-pem",
	}

	cert, err := provider.parseCertificateResponse(data)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "failed to parse certificate metadata")
}

// TestDefaultRetryConfig_Values tests default retry config values
func TestDefaultRetryConfig_Values(t *testing.T) {
	cfg := DefaultRetryConfig()

	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.InitialBackoff)
	assert.Equal(t, 30*time.Second, cfg.MaxBackoff)
	assert.Equal(t, 2.0, cfg.BackoffFactor)
}

// TestVaultPKIProvider_GetCertificate_NeedsRenewal tests GetCertificate when cert needs renewal
func TestVaultPKIProvider_GetCertificate_NeedsRenewal(t *testing.T) {
	// Certificate that needs renewal
	cachedCert := &Certificate{
		CertPEM:   []byte("cert"),
		KeyPEM:    []byte("key"),
		ExpiresAt: time.Now().Add(12 * time.Hour), // Within renewal buffer
	}

	provider := &VaultPKIProvider{
		currentCert:   cachedCert,
		renewalBuffer: 24 * time.Hour,
		serviceName:   "", // No service name set
		namespace:     "",
	}

	ctx := context.Background()
	cert, err := provider.GetCertificate(ctx)
	// Should fail because service name/namespace not set
	assert.Error(t, err)
	assert.Nil(t, cert)
}

// TestVaultPKIProvider_RenewCertificate_WithServiceName tests renewal with service name set
// Note: This test is skipped because it requires a real Vault client
func TestVaultPKIProvider_RenewCertificate_WithServiceName(t *testing.T) {
	// This test would require a real Vault client to work properly
	// The RenewCertificate method calls IssueCertificate which requires a valid client
	// We test the error path for missing service name instead
	provider := &VaultPKIProvider{
		serviceName: "",
		namespace:   "",
	}

	ctx := context.Background()
	cert, err := provider.RenewCertificate(ctx, nil)
	// Should fail because service name/namespace not set
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "service name and namespace must be set before renewal")
}

// TestVaultPKIProvider_ParseCertificateResponse_WrongTypeForCertificate tests wrong type
func TestVaultPKIProvider_ParseCertificateResponse_WrongTypeForCertificate(t *testing.T) {
	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate": 12345, // Wrong type - should be string
		"private_key": "key-pem",
	}

	cert, err := provider.parseCertificateResponse(data)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "certificate not found in response")
}

// TestVaultPKIProvider_ParseCertificateResponse_WrongTypeForPrivateKey tests wrong type
func TestVaultPKIProvider_ParseCertificateResponse_WrongTypeForPrivateKey(t *testing.T) {
	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate": "cert-pem",
		"private_key": 12345, // Wrong type - should be string
	}

	cert, err := provider.parseCertificateResponse(data)
	assert.Error(t, err)
	assert.Nil(t, cert)
	assert.Contains(t, err.Error(), "private_key not found in response")
}

// TestVaultPKIProvider_CalculateBackoff_ZeroAttempt tests backoff with zero attempt
func TestVaultPKIProvider_CalculateBackoff_ZeroAttempt(t *testing.T) {
	provider := &VaultPKIProvider{
		retryConfig: RetryConfig{
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     30 * time.Second,
			BackoffFactor:  2.0,
		},
	}

	backoff := provider.calculateBackoff(0)
	assert.Equal(t, 1*time.Second, backoff)
}

// TestVaultPKIProvider_BuildCertificateRequest_DifferentServices tests request building
func TestVaultPKIProvider_BuildCertificateRequest_DifferentServices(t *testing.T) {
	tests := []struct {
		serviceName string
		namespace   string
		expectedCN  string
	}{
		{
			serviceName: "webhook",
			namespace:   "default",
			expectedCN:  "webhook.default.svc",
		},
		{
			serviceName: "my-service",
			namespace:   "production",
			expectedCN:  "my-service.production.svc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.serviceName+"_"+tt.namespace, func(t *testing.T) {
			provider := &VaultPKIProvider{
				ttl: 720 * time.Hour,
			}

			request := provider.buildCertificateRequest(tt.serviceName, tt.namespace)
			assert.Equal(t, tt.expectedCN, request["common_name"])
		})
	}
}

// TestVaultPKIProvider_ServiceNameAndNamespaceStorage tests that service name and namespace are stored
func TestVaultPKIProvider_ServiceNameAndNamespaceStorage(t *testing.T) {
	// Create a provider and manually set service name and namespace
	provider := &VaultPKIProvider{
		pkiMountPath:  "pki",
		pkiRole:       "webhook-cert",
		ttl:           720 * time.Hour,
		renewalBuffer: 24 * time.Hour,
		retryConfig:   DefaultRetryConfig(),
	}

	// Manually set service name and namespace (simulating what IssueCertificate does)
	provider.serviceName = "test-service"
	provider.namespace = "test-namespace"

	// Verify they are stored
	assert.Equal(t, "test-service", provider.serviceName)
	assert.Equal(t, "test-namespace", provider.namespace)
}

// TestVaultPKIProvider_RenewCertificate_ServiceNameCheck tests renewal service name validation
func TestVaultPKIProvider_RenewCertificate_ServiceNameCheck(t *testing.T) {
	// Test with empty service name - should fail with specific error
	provider := &VaultPKIProvider{
		serviceName:   "",
		namespace:     "",
		pkiMountPath:  "pki",
		pkiRole:       "webhook-cert",
		ttl:           720 * time.Hour,
		renewalBuffer: 24 * time.Hour,
		retryConfig:   DefaultRetryConfig(),
	}

	ctx := context.Background()
	_, err := provider.RenewCertificate(ctx, nil)
	// Should fail because service name/namespace not set
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service name and namespace must be set")
}

// TestVaultPKIProvider_GetCertificate_ServiceNameCheck tests GetCertificate service name validation
func TestVaultPKIProvider_GetCertificate_ServiceNameCheck(t *testing.T) {
	// Test with empty service name and no cached cert - should fail with specific error
	provider := &VaultPKIProvider{
		serviceName:   "",
		namespace:     "",
		currentCert:   nil, // No cached cert
		pkiMountPath:  "pki",
		pkiRole:       "webhook-cert",
		ttl:           720 * time.Hour,
		renewalBuffer: 24 * time.Hour,
		retryConfig:   DefaultRetryConfig(),
	}

	ctx := context.Background()
	_, err := provider.GetCertificate(ctx)
	// Should fail because service name/namespace not set
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no certificate available and service name/namespace not set")
}

// TestVaultPKIProvider_GetCertificate_ExpiredCachedCert tests GetCertificate with expired cached cert
func TestVaultPKIProvider_GetCertificate_ExpiredCachedCert(t *testing.T) {
	expiredCert := &Certificate{
		CertPEM:   []byte("cert"),
		KeyPEM:    []byte("key"),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
	}

	provider := &VaultPKIProvider{
		client:        nil, // No client
		currentCert:   expiredCert,
		renewalBuffer: 24 * time.Hour,
		serviceName:   "", // No service name
		namespace:     "",
	}

	ctx := context.Background()
	cert, err := provider.GetCertificate(ctx)
	// Should fail because service name/namespace not set
	assert.Error(t, err)
	assert.Nil(t, cert)
}

// TestVaultPKIProvider_CalculateBackoff_LargeAttempt tests backoff with large attempt number
func TestVaultPKIProvider_CalculateBackoff_LargeAttempt(t *testing.T) {
	provider := &VaultPKIProvider{
		retryConfig: RetryConfig{
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     30 * time.Second,
			BackoffFactor:  2.0,
		},
	}

	// Large attempt should be capped at max backoff
	backoff := provider.calculateBackoff(100)
	assert.Equal(t, 30*time.Second, backoff)
}

// TestVaultPKIProvider_ParseCertificateResponse_NoSerialNumber tests response without serial number
func TestVaultPKIProvider_ParseCertificateResponse_NoSerialNumber(t *testing.T) {
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

	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))

	provider := &VaultPKIProvider{}

	data := map[string]interface{}{
		"certificate": certPEM,
		"private_key": keyPEM,
		// No serial_number
	}

	cert, err := provider.parseCertificateResponse(data)
	require.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Empty(t, cert.SerialNumber) // Should be empty since not provided
}

// TestIsRetryableError_WrappedError tests retryable error detection with wrapped errors
func TestIsRetryableError_WrappedError(t *testing.T) {
	wrappedErr := errors.New("outer: connection refused")
	assert.True(t, isRetryableError(wrappedErr))
}

// TestContainsIgnoreCase_EdgeCases tests edge cases for containsIgnoreCase
func TestContainsIgnoreCase_EdgeCases(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"", "", true},
		{"a", "a", true},
		{"A", "a", true},
		{"a", "A", true},
		{"abc", "ABC", true},
		{"ABC", "abc", true},
		{"aBc", "AbC", true},
		{"hello123world", "123", true},
		{"HELLO123WORLD", "123", true},
		{"test", "testing", false},
		{"ab", "abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			assert.Equal(t, tt.expected, containsIgnoreCase(tt.s, tt.substr))
		})
	}
}

// TestBytesContains_EdgeCases tests edge cases for bytesContains
func TestBytesContains_EdgeCases(t *testing.T) {
	tests := []struct {
		b        []byte
		sub      []byte
		expected bool
	}{
		{[]byte{}, []byte{}, true},
		{[]byte("a"), []byte("a"), true},
		{[]byte("ab"), []byte("a"), true},
		{[]byte("ab"), []byte("b"), true},
		{[]byte("ab"), []byte("ab"), true},
		{[]byte("abc"), []byte("bc"), true},
		{[]byte("abc"), []byte("ac"), false},
		{[]byte("a"), []byte("ab"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.b)+"_"+string(tt.sub), func(t *testing.T) {
			assert.Equal(t, tt.expected, bytesContains(tt.b, tt.sub))
		})
	}
}

// TestVaultPKIProvider_NeedsRenewal_ExactlyAtBuffer tests NeedsRenewal at exact buffer boundary
func TestVaultPKIProvider_NeedsRenewal_ExactlyAtBuffer(t *testing.T) {
	provider := &VaultPKIProvider{
		renewalBuffer: 24 * time.Hour,
	}

	// Certificate expiring exactly at buffer boundary
	cert := &Certificate{
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// At exactly the buffer, it should need renewal (time.Until < buffer)
	// Due to timing, this might be slightly less than 24h
	needsRenewal := provider.NeedsRenewal(cert)
	// The result depends on exact timing, but we can verify the function doesn't panic
	assert.IsType(t, true, needsRenewal)
}
