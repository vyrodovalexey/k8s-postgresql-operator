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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCertificateProviderType_Constants tests that provider type constants have correct values
func TestCertificateProviderType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		got      CertificateProviderType
		expected string
	}{
		{
			name:     "ProviderTypeSelfSigned",
			got:      ProviderTypeSelfSigned,
			expected: "self-signed",
		},
		{
			name:     "ProviderTypeProvided",
			got:      ProviderTypeProvided,
			expected: "provided",
		},
		{
			name:     "ProviderTypeVaultPKI",
			got:      ProviderTypeVaultPKI,
			expected: "vault-pki",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.got))
		})
	}
}

// TestCertificate_Initialization tests Certificate struct initialization
func TestCertificate_Initialization(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	cert := &Certificate{
		CertPEM:      []byte("cert-pem-data"),
		KeyPEM:       []byte("key-pem-data"),
		CACertPEM:    []byte("ca-cert-pem-data"),
		ExpiresAt:    expiresAt,
		IssuedAt:     now,
		SerialNumber: "12345",
	}

	assert.Equal(t, []byte("cert-pem-data"), cert.CertPEM)
	assert.Equal(t, []byte("key-pem-data"), cert.KeyPEM)
	assert.Equal(t, []byte("ca-cert-pem-data"), cert.CACertPEM)
	assert.Equal(t, expiresAt, cert.ExpiresAt)
	assert.Equal(t, now, cert.IssuedAt)
	assert.Equal(t, "12345", cert.SerialNumber)
}

// TestCertificate_IsExpired tests the IsExpired method
func TestCertificate_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired - future date",
			expiresAt: time.Now().Add(24 * time.Hour),
			expected:  false,
		},
		{
			name:      "expired - past date",
			expiresAt: time.Now().Add(-24 * time.Hour),
			expected:  true,
		},
		{
			name:      "expired - just now",
			expiresAt: time.Now().Add(-1 * time.Second),
			expected:  true,
		},
		{
			name:      "not expired - 1 second from now",
			expiresAt: time.Now().Add(1 * time.Second),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := &Certificate{
				ExpiresAt: tt.expiresAt,
			}
			assert.Equal(t, tt.expected, cert.IsExpired())
		})
	}
}

// TestCertificate_TimeUntilExpiry tests the TimeUntilExpiry method
func TestCertificate_TimeUntilExpiry(t *testing.T) {
	tests := []struct {
		name           string
		expiresAt      time.Time
		expectedRange  [2]time.Duration // min and max expected duration
		shouldPositive bool
	}{
		{
			name:           "future expiry - 1 hour",
			expiresAt:      time.Now().Add(1 * time.Hour),
			expectedRange:  [2]time.Duration{59 * time.Minute, 61 * time.Minute},
			shouldPositive: true,
		},
		{
			name:           "past expiry - 1 hour ago",
			expiresAt:      time.Now().Add(-1 * time.Hour),
			expectedRange:  [2]time.Duration{-61 * time.Minute, -59 * time.Minute},
			shouldPositive: false,
		},
		{
			name:           "future expiry - 24 hours",
			expiresAt:      time.Now().Add(24 * time.Hour),
			expectedRange:  [2]time.Duration{23*time.Hour + 59*time.Minute, 24*time.Hour + 1*time.Minute},
			shouldPositive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := &Certificate{
				ExpiresAt: tt.expiresAt,
			}
			duration := cert.TimeUntilExpiry()

			if tt.shouldPositive {
				assert.True(t, duration > 0, "expected positive duration")
			} else {
				assert.True(t, duration < 0, "expected negative duration")
			}

			assert.True(t, duration >= tt.expectedRange[0] && duration <= tt.expectedRange[1],
				"duration %v not in expected range [%v, %v]", duration, tt.expectedRange[0], tt.expectedRange[1])
		})
	}
}

// TestCertificate_IsValid tests the IsValid method
func TestCertificate_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		cert     *Certificate
		expected bool
	}{
		{
			name:     "nil certificate",
			cert:     nil,
			expected: false,
		},
		{
			name: "valid certificate",
			cert: &Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
			expected: true,
		},
		{
			name: "expired certificate",
			cert: &Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				ExpiresAt: time.Now().Add(-24 * time.Hour),
			},
			expected: false,
		},
		{
			name: "empty CertPEM",
			cert: &Certificate{
				CertPEM:   []byte{},
				KeyPEM:    []byte("key-data"),
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
			expected: false,
		},
		{
			name: "empty KeyPEM",
			cert: &Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte{},
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
			expected: false,
		},
		{
			name: "both CertPEM and KeyPEM empty",
			cert: &Certificate{
				CertPEM:   []byte{},
				KeyPEM:    []byte{},
				ExpiresAt: time.Now().Add(24 * time.Hour),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cert.IsValid())
		})
	}
}

// TestDefaultConstants tests default constant values
func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, 24*time.Hour, DefaultRenewalBuffer)
	assert.Equal(t, 720*time.Hour, DefaultCertificateTTL)
}

// TestRetryConfig_Initialization tests RetryConfig struct initialization
func TestRetryConfig_Initialization(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}

	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.InitialBackoff)
	assert.Equal(t, 30*time.Second, cfg.MaxBackoff)
	assert.Equal(t, 2.0, cfg.BackoffFactor)
}

// TestDefaultRetryConfig tests the DefaultRetryConfig function
func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()

	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 1*time.Second, cfg.InitialBackoff)
	assert.Equal(t, 30*time.Second, cfg.MaxBackoff)
	assert.Equal(t, 2.0, cfg.BackoffFactor)
}

// TestCertificate_ZeroValues tests Certificate with zero values
func TestCertificate_ZeroValues(t *testing.T) {
	cert := &Certificate{}

	// Zero time is in the past, so should be expired
	assert.True(t, cert.IsExpired())
	assert.False(t, cert.IsValid())
	assert.True(t, cert.TimeUntilExpiry() < 0)
}

// TestCertificateProviderType_String tests that CertificateProviderType can be used as string
func TestCertificateProviderType_String(t *testing.T) {
	tests := []struct {
		providerType CertificateProviderType
		expected     string
	}{
		{ProviderTypeSelfSigned, "self-signed"},
		{ProviderTypeProvided, "provided"},
		{ProviderTypeVaultPKI, "vault-pki"},
	}

	for _, tt := range tests {
		t.Run(string(tt.providerType), func(t *testing.T) {
			// Test direct string conversion
			assert.Equal(t, tt.expected, string(tt.providerType))

			// Test comparison
			assert.Equal(t, CertificateProviderType(tt.expected), tt.providerType)
		})
	}
}
