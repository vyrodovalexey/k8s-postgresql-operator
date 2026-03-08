//go:build integration

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

package integration_test

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vyrodovalexey/k8s-postgresql-operator/test/helpers"
)

const (
	defaultPKIMountPath = "pki-test"
	defaultPKIRole      = "test-role"
	defaultPKIDomain    = "example.com"
)

func setupPKI(t *testing.T, vaultClient *helpers.VaultTestClient) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mountPath := helpers.GetEnvOrDefault("TEST_VAULT_PKI_MOUNT_PATH", defaultPKIMountPath)
	role := helpers.GetEnvOrDefault("TEST_VAULT_PKI_ROLE", defaultPKIRole)

	err := vaultClient.SetupPKI(ctx, mountPath, role, defaultPKIDomain)
	require.NoError(t, err, "Setting up PKI should succeed")
}

func TestIntegration_VaultPKI_IssueCertificate(t *testing.T) {
	vaultClient := newVaultClient(t)
	setupPKI(t, vaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mountPath := helpers.GetEnvOrDefault("TEST_VAULT_PKI_MOUNT_PATH", defaultPKIMountPath)
	role := helpers.GetEnvOrDefault("TEST_VAULT_PKI_ROLE", defaultPKIRole)

	cert, key, ca, err := vaultClient.IssuePKICert(ctx, mountPath, role, "test.example.com")
	require.NoError(t, err, "Issuing PKI certificate should succeed")

	assert.NotEmpty(t, cert, "Certificate should not be empty")
	assert.NotEmpty(t, key, "Private key should not be empty")
	assert.NotEmpty(t, ca, "CA certificate should not be empty")

	// Verify the certificate is valid PEM
	block, _ := pem.Decode([]byte(cert))
	require.NotNil(t, block, "Certificate should be valid PEM")
	assert.Equal(t, "CERTIFICATE", block.Type, "PEM block should be a CERTIFICATE")

	// Verify the private key is valid PEM
	keyBlock, _ := pem.Decode([]byte(key))
	require.NotNil(t, keyBlock, "Private key should be valid PEM")
}

func TestIntegration_VaultPKI_CertificateFields(t *testing.T) {
	vaultClient := newVaultClient(t)
	setupPKI(t, vaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mountPath := helpers.GetEnvOrDefault("TEST_VAULT_PKI_MOUNT_PATH", defaultPKIMountPath)
	role := helpers.GetEnvOrDefault("TEST_VAULT_PKI_ROLE", defaultPKIRole)
	commonName := "myservice.example.com"

	cert, _, _, err := vaultClient.IssuePKICert(ctx, mountPath, role, commonName)
	require.NoError(t, err, "Issuing PKI certificate should succeed")

	// Parse the certificate
	block, _ := pem.Decode([]byte(cert))
	require.NotNil(t, block, "Certificate should be valid PEM")

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err, "Parsing certificate should succeed")

	// Verify certificate fields
	assert.Equal(t, commonName, parsedCert.Subject.CommonName,
		"Certificate CN should match the requested common name")

	// Verify the certificate is not expired
	assert.True(t, parsedCert.NotAfter.After(time.Now()),
		"Certificate should not be expired")
	assert.True(t, parsedCert.NotBefore.Before(time.Now()),
		"Certificate NotBefore should be in the past")

	// Verify TTL is approximately 24h (what we requested)
	expectedExpiry := time.Now().Add(24 * time.Hour)
	assert.WithinDuration(t, expectedExpiry, parsedCert.NotAfter, 5*time.Minute,
		"Certificate expiry should be approximately 24h from now")
}

func TestIntegration_VaultPKI_ReadCA(t *testing.T) {
	vaultClient := newVaultClient(t)
	setupPKI(t, vaultClient)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mountPath := helpers.GetEnvOrDefault("TEST_VAULT_PKI_MOUNT_PATH", defaultPKIMountPath)

	ca, err := vaultClient.ReadPKICA(ctx, mountPath)
	require.NoError(t, err, "Reading PKI CA should succeed")
	assert.NotEmpty(t, ca, "CA certificate should not be empty")

	// Verify the CA is valid PEM
	block, _ := pem.Decode([]byte(ca))
	require.NotNil(t, block, "CA should be valid PEM")
	assert.Equal(t, "CERTIFICATE", block.Type, "PEM block should be a CERTIFICATE")

	// Parse and verify it's a CA certificate
	parsedCA, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err, "Parsing CA certificate should succeed")
	assert.True(t, parsedCA.IsCA, "Certificate should be a CA")
	assert.Equal(t, "K8s PostgreSQL Operator CA", parsedCA.Subject.CommonName,
		"CA CN should match what we configured")
}
