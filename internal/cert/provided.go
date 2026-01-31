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
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ProvidedProvider implements CertificateProvider for externally provided certificates
// This provider loads certificates from file paths and does not support renewal
type ProvidedProvider struct {
	certPath      string
	keyPath       string
	logger        *zap.SugaredLogger
	renewalBuffer time.Duration
	currentCert   *Certificate
	mu            sync.RWMutex
}

// NewProvidedProvider creates a new ProvidedProvider
func NewProvidedProvider(certPath, keyPath string, logger *zap.SugaredLogger) (*ProvidedProvider, error) {
	if certPath == "" {
		return nil, fmt.Errorf("certificate path is required")
	}
	if keyPath == "" {
		return nil, fmt.Errorf("key path is required")
	}

	// Verify files exist
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("certificate file does not exist: %s", certPath)
	}
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("key file does not exist: %s", keyPath)
	}

	return &ProvidedProvider{
		certPath:      certPath,
		keyPath:       keyPath,
		logger:        logger,
		renewalBuffer: DefaultRenewalBuffer,
	}, nil
}

// IssueCertificate loads the certificate from the provided paths
// For ProvidedProvider, this is equivalent to GetCertificate
func (p *ProvidedProvider) IssueCertificate(ctx context.Context, _, _ string) (*Certificate, error) {
	return p.loadCertificate()
}

// RenewCertificate returns an error as renewal is not supported for provided certificates
// External certificate management is expected to handle renewal
func (p *ProvidedProvider) RenewCertificate(_ context.Context, _ *Certificate) (*Certificate, error) {
	return nil, fmt.Errorf("renewal not supported for provided certificates: certificates are managed externally")
}

// GetCertificate retrieves the current certificate from the provided paths
func (p *ProvidedProvider) GetCertificate(_ context.Context) (*Certificate, error) {
	p.mu.RLock()
	if p.currentCert != nil && p.currentCert.IsValid() {
		cert := p.currentCert
		p.mu.RUnlock()
		return cert, nil
	}
	p.mu.RUnlock()

	return p.loadCertificate()
}

// Type returns the provider type
func (p *ProvidedProvider) Type() CertificateProviderType {
	return ProviderTypeProvided
}

// NeedsRenewal checks if the certificate needs to be renewed
// For provided certificates, this only indicates expiration status
// Actual renewal must be done externally
func (p *ProvidedProvider) NeedsRenewal(cert *Certificate) bool {
	if cert == nil {
		return true
	}
	needsRenewal := time.Until(cert.ExpiresAt) < p.renewalBuffer
	if needsRenewal && p.logger != nil {
		p.logger.Warnw("Provided certificate needs renewal - please update externally",
			"expires_at", cert.ExpiresAt,
			"time_until_expiry", time.Until(cert.ExpiresAt))
	}
	return needsRenewal
}

// loadCertificate loads the certificate from files
func (p *ProvidedProvider) loadCertificate() (*Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Read certificate file
	//nolint:gosec // certificate path is provided by configuration
	certPEM, err := os.ReadFile(p.certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	// Read key file
	//nolint:gosec // key path is provided by configuration
	keyPEM, err := os.ReadFile(p.keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Parse certificate to get metadata
	certMeta, err := parseCertificatePEM(certPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	cert := &Certificate{
		CertPEM:      certPEM,
		KeyPEM:       keyPEM,
		CACertPEM:    certPEM, // Assume CA is included or same as cert
		ExpiresAt:    certMeta.ExpiresAt,
		IssuedAt:     certMeta.IssuedAt,
		SerialNumber: certMeta.SerialNumber,
	}

	p.currentCert = cert

	if p.logger != nil {
		p.logger.Infow("Loaded provided certificate",
			"cert_path", p.certPath,
			"expires_at", cert.ExpiresAt,
			"serial", cert.SerialNumber)
	}

	return cert, nil
}

// GetCertPath returns the certificate file path
func (p *ProvidedProvider) GetCertPath() string {
	return p.certPath
}

// GetKeyPath returns the key file path
func (p *ProvidedProvider) GetKeyPath() string {
	return p.keyPath
}
