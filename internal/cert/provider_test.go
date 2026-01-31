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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockCertificateProvider is a mock implementation of CertificateProvider for testing
type MockCertificateProvider struct {
	issueCertificateFunc func(ctx context.Context, serviceName, namespace string) (*Certificate, error)
	renewCertificateFunc func(ctx context.Context, cert *Certificate) (*Certificate, error)
	getCertificateFunc   func(ctx context.Context) (*Certificate, error)
	typeFunc             func() CertificateProviderType
	needsRenewalFunc     func(cert *Certificate) bool
}

func (m *MockCertificateProvider) IssueCertificate(ctx context.Context, serviceName, namespace string) (*Certificate, error) {
	if m.issueCertificateFunc != nil {
		return m.issueCertificateFunc(ctx, serviceName, namespace)
	}
	return nil, nil
}

func (m *MockCertificateProvider) RenewCertificate(ctx context.Context, cert *Certificate) (*Certificate, error) {
	if m.renewCertificateFunc != nil {
		return m.renewCertificateFunc(ctx, cert)
	}
	return nil, nil
}

func (m *MockCertificateProvider) GetCertificate(ctx context.Context) (*Certificate, error) {
	if m.getCertificateFunc != nil {
		return m.getCertificateFunc(ctx)
	}
	return nil, nil
}

func (m *MockCertificateProvider) Type() CertificateProviderType {
	if m.typeFunc != nil {
		return m.typeFunc()
	}
	return ProviderTypeSelfSigned
}

func (m *MockCertificateProvider) NeedsRenewal(cert *Certificate) bool {
	if m.needsRenewalFunc != nil {
		return m.needsRenewalFunc(cert)
	}
	return false
}

// Ensure MockCertificateProvider implements CertificateProvider
var _ CertificateProvider = (*MockCertificateProvider)(nil)

// TestCertificateProvider_Interface tests that the interface is properly defined
func TestCertificateProvider_Interface(t *testing.T) {
	// Create a mock provider
	mockProvider := &MockCertificateProvider{
		issueCertificateFunc: func(ctx context.Context, serviceName, namespace string) (*Certificate, error) {
			return &Certificate{
				CertPEM:      []byte("cert"),
				KeyPEM:       []byte("key"),
				ExpiresAt:    time.Now().Add(24 * time.Hour),
				SerialNumber: "123",
			}, nil
		},
		renewCertificateFunc: func(ctx context.Context, cert *Certificate) (*Certificate, error) {
			return &Certificate{
				CertPEM:      []byte("renewed-cert"),
				KeyPEM:       []byte("renewed-key"),
				ExpiresAt:    time.Now().Add(48 * time.Hour),
				SerialNumber: "456",
			}, nil
		},
		getCertificateFunc: func(ctx context.Context) (*Certificate, error) {
			return &Certificate{
				CertPEM:      []byte("existing-cert"),
				KeyPEM:       []byte("existing-key"),
				ExpiresAt:    time.Now().Add(12 * time.Hour),
				SerialNumber: "789",
			}, nil
		},
		typeFunc: func() CertificateProviderType {
			return ProviderTypeSelfSigned
		},
		needsRenewalFunc: func(cert *Certificate) bool {
			return cert != nil && time.Until(cert.ExpiresAt) < 24*time.Hour
		},
	}

	// Test IssueCertificate
	cert, err := mockProvider.IssueCertificate(context.Background(), "test-service", "test-namespace")
	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.Equal(t, []byte("cert"), cert.CertPEM)
	assert.Equal(t, "123", cert.SerialNumber)

	// Test RenewCertificate
	renewedCert, err := mockProvider.RenewCertificate(context.Background(), cert)
	assert.NoError(t, err)
	assert.NotNil(t, renewedCert)
	assert.Equal(t, []byte("renewed-cert"), renewedCert.CertPEM)
	assert.Equal(t, "456", renewedCert.SerialNumber)

	// Test GetCertificate
	existingCert, err := mockProvider.GetCertificate(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, existingCert)
	assert.Equal(t, []byte("existing-cert"), existingCert.CertPEM)

	// Test Type
	assert.Equal(t, ProviderTypeSelfSigned, mockProvider.Type())

	// Test NeedsRenewal
	assert.True(t, mockProvider.NeedsRenewal(existingCert))
}

// TestCertificateProvider_NilMethods tests mock provider with nil methods
func TestCertificateProvider_NilMethods(t *testing.T) {
	mockProvider := &MockCertificateProvider{}

	cert, err := mockProvider.IssueCertificate(context.Background(), "test", "test")
	assert.NoError(t, err)
	assert.Nil(t, cert)

	renewedCert, err := mockProvider.RenewCertificate(context.Background(), nil)
	assert.NoError(t, err)
	assert.Nil(t, renewedCert)

	existingCert, err := mockProvider.GetCertificate(context.Background())
	assert.NoError(t, err)
	assert.Nil(t, existingCert)

	assert.Equal(t, ProviderTypeSelfSigned, mockProvider.Type())
	assert.False(t, mockProvider.NeedsRenewal(nil))
}

// TestCertificateProvider_AllTypes tests that all provider types can be used
func TestCertificateProvider_AllTypes(t *testing.T) {
	types := []CertificateProviderType{
		ProviderTypeSelfSigned,
		ProviderTypeProvided,
		ProviderTypeVaultPKI,
	}

	for _, providerType := range types {
		t.Run(string(providerType), func(t *testing.T) {
			mockProvider := &MockCertificateProvider{
				typeFunc: func() CertificateProviderType {
					return providerType
				},
			}
			assert.Equal(t, providerType, mockProvider.Type())
		})
	}
}
