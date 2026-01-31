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
	"time"
)

// CertificateProviderType represents the type of certificate provider
type CertificateProviderType string

const (
	// ProviderTypeSelfSigned indicates a self-signed certificate provider
	ProviderTypeSelfSigned CertificateProviderType = "self-signed"
	// ProviderTypeProvided indicates an externally provided certificate provider
	ProviderTypeProvided CertificateProviderType = "provided"
	// ProviderTypeVaultPKI indicates a Vault PKI certificate provider
	ProviderTypeVaultPKI CertificateProviderType = "vault-pki"
)

// Certificate represents a TLS certificate with its metadata
type Certificate struct {
	// CertPEM is the PEM-encoded certificate
	CertPEM []byte
	// KeyPEM is the PEM-encoded private key
	KeyPEM []byte
	// CACertPEM is the PEM-encoded CA certificate (optional, may be same as CertPEM for self-signed)
	CACertPEM []byte
	// ExpiresAt is the expiration time of the certificate
	ExpiresAt time.Time
	// IssuedAt is the time when the certificate was issued
	IssuedAt time.Time
	// SerialNumber is the serial number of the certificate
	SerialNumber string
}

// IsExpired returns true if the certificate has expired
func (c *Certificate) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// TimeUntilExpiry returns the duration until the certificate expires
func (c *Certificate) TimeUntilExpiry() time.Duration {
	return time.Until(c.ExpiresAt)
}

// IsValid returns true if the certificate is not nil and has not expired
func (c *Certificate) IsValid() bool {
	return c != nil && !c.IsExpired() && len(c.CertPEM) > 0 && len(c.KeyPEM) > 0
}

// DefaultRenewalBuffer is the default time before expiry to trigger renewal
const DefaultRenewalBuffer = 24 * time.Hour

// DefaultCertificateTTL is the default TTL for certificates
const DefaultCertificateTTL = 720 * time.Hour // 30 days

// RetryConfig holds configuration for retry logic
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialBackoff is the initial backoff duration
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	// BackoffFactor is the multiplier for exponential backoff
	BackoffFactor float64
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     5,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}
}
