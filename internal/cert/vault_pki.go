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
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/kubernetes"
	"go.uber.org/zap"
)

// VaultPKIConfig contains configuration for the Vault PKI provider
type VaultPKIConfig struct {
	// VaultAddr is the Vault server address
	VaultAddr string
	// VaultRole is the Vault role for Kubernetes authentication
	VaultRole string
	// TokenPath is the path to the Kubernetes service account token
	TokenPath string
	// PKIMountPath is the Vault PKI secrets engine mount path
	PKIMountPath string
	// PKIRole is the Vault PKI role name for certificate issuance
	PKIRole string
	// TTL is the TTL for issued certificates
	TTL time.Duration
	// RenewalBuffer is the time before expiry to trigger renewal
	RenewalBuffer time.Duration
	// Logger is the zap logger
	Logger *zap.SugaredLogger
	// RetryConfig is the retry configuration for Vault operations
	RetryConfig RetryConfig
}

// VaultPKIProvider implements CertificateProvider using Vault PKI secrets engine
type VaultPKIProvider struct {
	client        *api.Client
	pkiMountPath  string
	pkiRole       string
	ttl           time.Duration
	renewalBuffer time.Duration
	serviceName   string
	namespace     string
	logger        *zap.SugaredLogger
	retryConfig   RetryConfig
	currentCert   *Certificate
	mu            sync.RWMutex
}

// NewVaultPKIProvider creates a new VaultPKIProvider
// The provided context is used for Vault authentication
func NewVaultPKIProvider(ctx context.Context, cfg VaultPKIConfig) (*VaultPKIProvider, error) {
	if cfg.VaultAddr == "" {
		return nil, fmt.Errorf("vault address is required")
	}
	if cfg.VaultRole == "" {
		return nil, fmt.Errorf("vault role is required")
	}
	if cfg.PKIMountPath == "" {
		return nil, fmt.Errorf("PKI mount path is required")
	}
	if cfg.PKIRole == "" {
		return nil, fmt.Errorf("PKI role is required")
	}

	// Create Vault client
	config := api.DefaultConfig()
	config.Address = cfg.VaultAddr

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Authenticate with Vault using Kubernetes auth
	k8sAuth, err := auth.NewKubernetesAuth(cfg.VaultRole,
		auth.WithServiceAccountTokenPath(cfg.TokenPath),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes auth method: %w", err)
	}

	// Use provided context for Vault authentication instead of context.Background()
	authInfo, err := client.Auth().Login(ctx, k8sAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate with Vault: %w", err)
	}

	if authInfo == nil {
		return nil, fmt.Errorf("authentication returned empty auth info")
	}

	ttl := cfg.TTL
	if ttl == 0 {
		ttl = DefaultCertificateTTL
	}

	renewalBuffer := cfg.RenewalBuffer
	if renewalBuffer == 0 {
		renewalBuffer = DefaultRenewalBuffer
	}

	retryConfig := cfg.RetryConfig
	if retryConfig.MaxRetries == 0 {
		retryConfig = DefaultRetryConfig()
	}

	if cfg.Logger != nil {
		cfg.Logger.Infow("Vault PKI provider initialized",
			"vault_addr", cfg.VaultAddr,
			"pki_mount_path", cfg.PKIMountPath,
			"pki_role", cfg.PKIRole,
			"ttl", ttl)
	}

	return &VaultPKIProvider{
		client:        client,
		pkiMountPath:  cfg.PKIMountPath,
		pkiRole:       cfg.PKIRole,
		ttl:           ttl,
		renewalBuffer: renewalBuffer,
		logger:        cfg.Logger,
		retryConfig:   retryConfig,
	}, nil
}

// IssueCertificate issues a new certificate from Vault PKI
func (p *VaultPKIProvider) IssueCertificate(ctx context.Context, serviceName, namespace string) (*Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.serviceName = serviceName
	p.namespace = namespace

	if p.logger != nil {
		p.logger.Infow("Issuing certificate from Vault PKI",
			"service", serviceName,
			"namespace", namespace,
			"pki_role", p.pkiRole)
	}

	cert, err := p.issueCertificateWithRetry(ctx, serviceName, namespace)
	if err != nil {
		return nil, err
	}

	p.currentCert = cert
	return cert, nil
}

// issueCertificateWithRetry issues a certificate with retry logic
func (p *VaultPKIProvider) issueCertificateWithRetry(
	ctx context.Context, serviceName, namespace string) (*Certificate, error) {
	var lastErr error

	for attempt := 0; attempt <= p.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := p.calculateBackoff(attempt)
			if p.logger != nil {
				p.logger.Infow("Retrying certificate issuance",
					"attempt", attempt,
					"backoff", backoff,
					"last_error", lastErr)
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue with retry
			}
		}

		cert, err := p.doIssueCertificate(ctx, serviceName, namespace)
		if err == nil {
			return cert, nil
		}

		lastErr = err
		if !isRetryableError(err) {
			return nil, fmt.Errorf("non-retryable error issuing certificate: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to issue certificate after %d attempts: %w",
		p.retryConfig.MaxRetries+1, lastErr)
}

// doIssueCertificate performs the actual certificate issuance
func (p *VaultPKIProvider) doIssueCertificate(
	ctx context.Context, serviceName, namespace string) (*Certificate, error) {
	// Build the request
	path := fmt.Sprintf("%s/issue/%s", p.pkiMountPath, p.pkiRole)
	data := p.buildCertificateRequest(serviceName, namespace)

	// Issue the certificate
	secret, err := p.client.Logical().WriteWithContext(ctx, path, data)
	if err != nil {
		return nil, fmt.Errorf("failed to issue certificate from Vault PKI: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("empty response from Vault PKI")
	}

	// Parse the response
	cert, err := p.parseCertificateResponse(secret.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate response: %w", err)
	}

	if p.logger != nil {
		p.logger.Infow("Certificate issued successfully from Vault PKI",
			"serial", cert.SerialNumber,
			"expires_at", cert.ExpiresAt)
	}

	return cert, nil
}

// buildCertificateRequest builds the request data for certificate issuance
func (p *VaultPKIProvider) buildCertificateRequest(serviceName, namespace string) map[string]interface{} {
	commonName := fmt.Sprintf("%s.%s.svc", serviceName, namespace)
	altNames := fmt.Sprintf("%s,%s.%s,%s.%s.svc,%s.%s.svc.cluster.local",
		serviceName,
		serviceName, namespace,
		serviceName, namespace,
		serviceName, namespace)

	return map[string]interface{}{
		"common_name": commonName,
		"alt_names":   altNames,
		"ip_sans":     "127.0.0.1",
		"ttl":         p.ttl.String(),
		"format":      "pem",
	}
}

// parseCertificateResponse parses the Vault PKI response into a Certificate
func (p *VaultPKIProvider) parseCertificateResponse(data map[string]interface{}) (*Certificate, error) {
	certPEM, ok := data["certificate"].(string)
	if !ok || certPEM == "" {
		return nil, fmt.Errorf("certificate not found in response")
	}

	keyPEM, ok := data["private_key"].(string)
	if !ok || keyPEM == "" {
		return nil, fmt.Errorf("private_key not found in response")
	}

	caCertPEM, ok := data["issuing_ca"].(string)
	if !ok {
		// Use certificate as CA if issuing_ca is not present
		caCertPEM = certPEM
	}

	serialNumber, _ := data["serial_number"].(string)

	// Parse expiration from the certificate
	certMeta, err := parseCertificatePEM([]byte(certPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate metadata: %w", err)
	}

	return &Certificate{
		CertPEM:      []byte(certPEM),
		KeyPEM:       []byte(keyPEM),
		CACertPEM:    []byte(caCertPEM),
		ExpiresAt:    certMeta.ExpiresAt,
		IssuedAt:     certMeta.IssuedAt,
		SerialNumber: serialNumber,
	}, nil
}

// calculateBackoff calculates the backoff duration for a given attempt
func (p *VaultPKIProvider) calculateBackoff(attempt int) time.Duration {
	backoff := p.retryConfig.InitialBackoff
	for i := 1; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * p.retryConfig.BackoffFactor)
		if backoff > p.retryConfig.MaxBackoff {
			backoff = p.retryConfig.MaxBackoff
			break
		}
	}
	return backoff
}

// RenewCertificate renews the certificate by issuing a new one
func (p *VaultPKIProvider) RenewCertificate(ctx context.Context, _ *Certificate) (*Certificate, error) {
	p.mu.RLock()
	serviceName := p.serviceName
	namespace := p.namespace
	p.mu.RUnlock()

	if serviceName == "" || namespace == "" {
		return nil, fmt.Errorf("service name and namespace must be set before renewal")
	}

	if p.logger != nil {
		p.logger.Infow("Renewing certificate from Vault PKI",
			"service", serviceName,
			"namespace", namespace)
	}

	return p.IssueCertificate(ctx, serviceName, namespace)
}

// GetCertificate retrieves the current certificate
func (p *VaultPKIProvider) GetCertificate(ctx context.Context) (*Certificate, error) {
	p.mu.RLock()
	if p.currentCert != nil && p.currentCert.IsValid() && !p.NeedsRenewal(p.currentCert) {
		cert := p.currentCert
		p.mu.RUnlock()
		return cert, nil
	}
	serviceName := p.serviceName
	namespace := p.namespace
	p.mu.RUnlock()

	if serviceName == "" || namespace == "" {
		return nil, fmt.Errorf("no certificate available and service name/namespace not set")
	}

	// Issue a new certificate
	return p.IssueCertificate(ctx, serviceName, namespace)
}

// Type returns the provider type
func (p *VaultPKIProvider) Type() CertificateProviderType {
	return ProviderTypeVaultPKI
}

// NeedsRenewal checks if the certificate needs to be renewed
func (p *VaultPKIProvider) NeedsRenewal(cert *Certificate) bool {
	if cert == nil {
		return true
	}
	return time.Until(cert.ExpiresAt) < p.renewalBuffer
}

// isRetryableError determines if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network timeout errors
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for specific Vault errors that are retryable
	errStr := err.Error()
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"i/o timeout",
		"no such host",
		"temporary failure",
		"server is currently unavailable",
		"503",
		"502",
		"504",
	}

	for _, pattern := range retryablePatterns {
		if containsIgnoreCase(errStr, pattern) {
			return true
		}
	}

	return false
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))

	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			sLower[i] = s[i] + 32
		} else {
			sLower[i] = s[i]
		}
	}

	for i := 0; i < len(substr); i++ {
		if substr[i] >= 'A' && substr[i] <= 'Z' {
			substrLower[i] = substr[i] + 32
		} else {
			substrLower[i] = substr[i]
		}
	}

	return bytesContains(sLower, substrLower)
}

// bytesContains checks if b contains sub
func bytesContains(b, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	if len(b) < len(sub) {
		return false
	}

	for i := 0; i <= len(b)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if b[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
