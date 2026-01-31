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
	"path/filepath"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
)

// ProviderFactory creates certificate providers based on configuration
type ProviderFactory struct {
	config     *config.Config
	restConfig *rest.Config
	logger     *zap.SugaredLogger
	namespace  string
	certDir    string
}

// ProviderFactoryOptions contains options for creating a ProviderFactory
type ProviderFactoryOptions struct {
	// Config is the application configuration
	Config *config.Config
	// RestConfig is the Kubernetes REST config
	RestConfig *rest.Config
	// Logger is the zap logger
	Logger *zap.SugaredLogger
	// Namespace is the Kubernetes namespace
	Namespace string
	// CertDir is the directory to store certificate files
	CertDir string
}

// NewProviderFactory creates a new ProviderFactory
func NewProviderFactory(opts ProviderFactoryOptions) (*ProviderFactory, error) {
	if opts.Config == nil {
		return nil, fmt.Errorf("config is required")
	}

	certDir := opts.CertDir
	if certDir == "" {
		// Create a temporary directory for certificates
		var err error
		certDir, err = os.MkdirTemp("", "webhook-certs-")
		if err != nil {
			return nil, fmt.Errorf("failed to create temporary directory for certificates: %w", err)
		}
	}

	return &ProviderFactory{
		config:     opts.Config,
		restConfig: opts.RestConfig,
		logger:     opts.Logger,
		namespace:  opts.Namespace,
		certDir:    certDir,
	}, nil
}

// CreateProvider creates the appropriate certificate provider based on configuration
// Priority order:
// 1. If WEBHOOK_CERT_PATH is set -> ProvidedProvider
// 2. If VAULT_PKI_ENABLED is true -> VaultPKIProvider
// 3. Default -> SelfSignedProvider
//
// Deprecated: Use CreateProviderWithContext instead
func (f *ProviderFactory) CreateProvider() (CertificateProvider, error) {
	return f.CreateProviderWithContext(context.Background())
}

// CreateProviderWithContext creates the appropriate certificate provider based on configuration
// The provided context is used for Vault authentication when using VaultPKIProvider
// Priority order:
// 1. If WEBHOOK_CERT_PATH is set -> ProvidedProvider
// 2. If VAULT_PKI_ENABLED is true -> VaultPKIProvider
// 3. Default -> SelfSignedProvider
func (f *ProviderFactory) CreateProviderWithContext(ctx context.Context) (CertificateProvider, error) {
	// Priority 1: Provided certificates
	if f.config.WebhookCertPath != "" {
		return f.createProvidedProvider()
	}

	// Priority 2: Vault PKI
	if f.config.VaultPKIEnabled {
		return f.createVaultPKIProvider(ctx)
	}

	// Priority 3: Self-signed (default)
	return f.createSelfSignedProvider()
}

// createProvidedProvider creates a ProvidedProvider
func (f *ProviderFactory) createProvidedProvider() (CertificateProvider, error) {
	certPath := filepath.Join(f.config.WebhookCertPath, f.config.WebhookCertName)
	keyPath := filepath.Join(f.config.WebhookCertPath, f.config.WebhookCertKey)

	if f.logger != nil {
		f.logger.Infow("Creating provided certificate provider",
			"cert_path", certPath,
			"key_path", keyPath)
	}

	provider, err := NewProvidedProvider(certPath, keyPath, f.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create provided certificate provider: %w", err)
	}

	return provider, nil
}

// createVaultPKIProvider creates a VaultPKIProvider
// The provided context is used for Vault authentication
func (f *ProviderFactory) createVaultPKIProvider(ctx context.Context) (CertificateProvider, error) {
	if f.logger != nil {
		f.logger.Infow("Creating Vault PKI certificate provider",
			"vault_addr", f.config.VaultAddr,
			"pki_mount_path", f.config.VaultPKIMountPath,
			"pki_role", f.config.VaultPKIRole)
	}

	cfg := VaultPKIConfig{
		VaultAddr:     f.config.VaultAddr,
		VaultRole:     f.config.VaultRole,
		TokenPath:     f.config.K8sTokenPath,
		PKIMountPath:  f.config.VaultPKIMountPath,
		PKIRole:       f.config.VaultPKIRole,
		TTL:           f.config.VaultPKITTLDuration(),
		RenewalBuffer: f.config.VaultPKIRenewalBufferDuration(),
		Logger:        f.logger,
		RetryConfig:   DefaultRetryConfig(),
	}

	provider, err := NewVaultPKIProvider(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault PKI certificate provider: %w", err)
	}

	return provider, nil
}

// createSelfSignedProvider creates a SelfSignedProvider
func (f *ProviderFactory) createSelfSignedProvider() (CertificateProvider, error) {
	if f.logger != nil {
		f.logger.Infow("Creating self-signed certificate provider",
			"service_name", f.config.WebhookK8sServiceName,
			"namespace", f.namespace,
			"cert_dir", f.certDir)
	}

	opts := SelfSignedProviderOptions{
		ServiceName: f.config.WebhookK8sServiceName,
		Namespace:   f.namespace,
		CertDir:     f.certDir,
		RestConfig:  f.restConfig,
		Logger:      f.logger,
	}

	provider, err := NewSelfSignedProvider(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create self-signed certificate provider: %w", err)
	}

	return provider, nil
}

// GetCertDir returns the certificate directory
func (f *ProviderFactory) GetCertDir() string {
	return f.certDir
}

// DetermineProviderType returns the provider type that would be created based on configuration
// This is useful for logging and debugging without actually creating the provider
func (f *ProviderFactory) DetermineProviderType() CertificateProviderType {
	if f.config.WebhookCertPath != "" {
		return ProviderTypeProvided
	}
	if f.config.VaultPKIEnabled {
		return ProviderTypeVaultPKI
	}
	return ProviderTypeSelfSigned
}
