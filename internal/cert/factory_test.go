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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
)

// TestNewProviderFactory_ValidConfig tests creating a factory with valid config
func TestNewProviderFactory_ValidConfig(t *testing.T) {
	cfg := config.New()

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
		CertDir:    "/tmp/certs",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)
	assert.NotNil(t, factory)
	assert.Equal(t, cfg, factory.config)
	assert.Equal(t, "test-namespace", factory.namespace)
	assert.Equal(t, "/tmp/certs", factory.certDir)
}

// TestNewProviderFactory_NilConfig tests error when config is nil
func TestNewProviderFactory_NilConfig(t *testing.T) {
	opts := ProviderFactoryOptions{
		Config: nil,
	}

	factory, err := NewProviderFactory(opts)
	assert.Error(t, err)
	assert.Nil(t, factory)
	assert.Contains(t, err.Error(), "config is required")
}

// TestNewProviderFactory_DefaultCertDir tests default cert directory creation
func TestNewProviderFactory_DefaultCertDir(t *testing.T) {
	cfg := config.New()

	opts := ProviderFactoryOptions{
		Config:  cfg,
		CertDir: "", // Empty, should create temp dir
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)
	assert.NotNil(t, factory)
	assert.NotEmpty(t, factory.certDir)
	assert.Contains(t, factory.certDir, "webhook-certs-")

	// Cleanup
	os.RemoveAll(factory.certDir)
}

// TestProviderFactory_CreateProvider_SelfSigned tests creating self-signed provider (default)
func TestProviderFactory_CreateProvider_SelfSigned(t *testing.T) {
	cfg := config.New()
	cfg.WebhookCertPath = ""    // No provided certs
	cfg.VaultPKIEnabled = false // Vault PKI disabled

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, ProviderTypeSelfSigned, provider.Type())
}

// TestProviderFactory_CreateProvider_Provided tests creating provided provider
func TestProviderFactory_CreateProvider_Provided(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	cfg := config.New()
	cfg.WebhookCertPath = tmpDir
	cfg.WebhookCertName = filepath.Base(certPath)
	cfg.WebhookCertKey = filepath.Base(keyPath)

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, ProviderTypeProvided, provider.Type())
}

// TestProviderFactory_CreateProvider_ProvidedPriority tests that provided takes priority over vault-pki
func TestProviderFactory_CreateProvider_ProvidedPriority(t *testing.T) {
	tmpDir := t.TempDir()
	certPath, keyPath := createTestCertificateFiles(t, tmpDir, 24*time.Hour)

	cfg := config.New()
	cfg.WebhookCertPath = tmpDir
	cfg.WebhookCertName = filepath.Base(certPath)
	cfg.WebhookCertKey = filepath.Base(keyPath)
	cfg.VaultPKIEnabled = true // Also enabled, but provided should take priority

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, ProviderTypeProvided, provider.Type())
}

// TestProviderFactory_CreateProvider_ProvidedInvalidPath tests error with invalid provided cert path
func TestProviderFactory_CreateProvider_ProvidedInvalidPath(t *testing.T) {
	cfg := config.New()
	cfg.WebhookCertPath = "/nonexistent/path"
	cfg.WebhookCertName = "tls.crt"
	cfg.WebhookCertKey = "tls.key"

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "failed to create provided certificate provider")
}

// TestProviderFactory_CreateProvider_VaultPKI tests creating vault PKI provider
func TestProviderFactory_CreateProvider_VaultPKI(t *testing.T) {
	cfg := config.New()
	cfg.WebhookCertPath = ""   // No provided certs
	cfg.VaultPKIEnabled = true // Vault PKI enabled
	cfg.VaultAddr = "https://vault.example.com:8200"
	cfg.VaultRole = "k8s-role"
	cfg.VaultPKIMountPath = "pki"
	cfg.VaultPKIRole = "webhook-cert"

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	// This will fail because we can't actually connect to Vault
	// but we can verify the factory attempts to create the right provider
	provider, err := factory.CreateProvider()
	assert.Error(t, err) // Expected to fail without real Vault
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "failed to create Vault PKI certificate provider")
}

// TestProviderFactory_CreateProvider_VaultPKIInvalidConfig tests vault PKI with invalid config
func TestProviderFactory_CreateProvider_VaultPKIInvalidConfig(t *testing.T) {
	cfg := config.New()
	cfg.WebhookCertPath = ""
	cfg.VaultPKIEnabled = true
	cfg.VaultAddr = "" // Empty vault address

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	assert.Error(t, err)
	assert.Nil(t, provider)
}

// TestProviderFactory_GetCertDir tests the GetCertDir method
func TestProviderFactory_GetCertDir(t *testing.T) {
	cfg := config.New()

	opts := ProviderFactoryOptions{
		Config:  cfg,
		CertDir: "/custom/cert/dir",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	assert.Equal(t, "/custom/cert/dir", factory.GetCertDir())
}

// TestProviderFactory_DetermineProviderType tests the DetermineProviderType method
func TestProviderFactory_DetermineProviderType(t *testing.T) {
	tests := []struct {
		name            string
		webhookCertPath string
		vaultPKIEnabled bool
		expected        CertificateProviderType
	}{
		{
			name:            "provided - cert path set",
			webhookCertPath: "/some/path",
			vaultPKIEnabled: false,
			expected:        ProviderTypeProvided,
		},
		{
			name:            "provided takes priority over vault-pki",
			webhookCertPath: "/some/path",
			vaultPKIEnabled: true,
			expected:        ProviderTypeProvided,
		},
		{
			name:            "vault-pki - enabled",
			webhookCertPath: "",
			vaultPKIEnabled: true,
			expected:        ProviderTypeVaultPKI,
		},
		{
			name:            "self-signed - default",
			webhookCertPath: "",
			vaultPKIEnabled: false,
			expected:        ProviderTypeSelfSigned,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New()
			cfg.WebhookCertPath = tt.webhookCertPath
			cfg.VaultPKIEnabled = tt.vaultPKIEnabled

			opts := ProviderFactoryOptions{
				Config: cfg,
			}

			factory, err := NewProviderFactory(opts)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, factory.DetermineProviderType())
		})
	}
}

// TestProviderFactory_CreateProvider_SelfSignedWithoutRestConfig tests self-signed without rest config
func TestProviderFactory_CreateProvider_SelfSignedWithoutRestConfig(t *testing.T) {
	cfg := config.New()
	cfg.WebhookCertPath = ""
	cfg.VaultPKIEnabled = false

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: nil, // No rest config
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "failed to create self-signed certificate provider")
}

// TestProviderFactory_CreateProvider_SelfSignedWithoutNamespace tests self-signed without namespace
func TestProviderFactory_CreateProvider_SelfSignedWithoutNamespace(t *testing.T) {
	cfg := config.New()
	cfg.WebhookCertPath = ""
	cfg.VaultPKIEnabled = false

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "", // Empty namespace
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	assert.Error(t, err)
	assert.Nil(t, provider)
}

// TestProviderFactory_CreateProvider_WithLogger tests provider creation with logger
func TestProviderFactory_CreateProvider_WithLogger(t *testing.T) {
	cfg := config.New()
	cfg.WebhookCertPath = ""
	cfg.VaultPKIEnabled = false

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     zap.NewNop().Sugar(),
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

// TestProviderFactory_CreateProvider_WithoutLogger tests provider creation without logger
func TestProviderFactory_CreateProvider_WithoutLogger(t *testing.T) {
	cfg := config.New()
	cfg.WebhookCertPath = ""
	cfg.VaultPKIEnabled = false

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: &rest.Config{Host: "https://test"},
		Logger:     nil, // No logger
		Namespace:  "test-namespace",
	}

	factory, err := NewProviderFactory(opts)
	require.NoError(t, err)

	provider, err := factory.CreateProvider()
	require.NoError(t, err)
	assert.NotNil(t, provider)
}

// TestProviderFactoryOptions_Initialization tests ProviderFactoryOptions struct
func TestProviderFactoryOptions_Initialization(t *testing.T) {
	cfg := config.New()
	restConfig := &rest.Config{Host: "https://test"}
	logger := zap.NewNop().Sugar()

	opts := ProviderFactoryOptions{
		Config:     cfg,
		RestConfig: restConfig,
		Logger:     logger,
		Namespace:  "my-namespace",
		CertDir:    "/my/cert/dir",
	}

	assert.Equal(t, cfg, opts.Config)
	assert.Equal(t, restConfig, opts.RestConfig)
	assert.Equal(t, logger, opts.Logger)
	assert.Equal(t, "my-namespace", opts.Namespace)
	assert.Equal(t, "/my/cert/dir", opts.CertDir)
}
