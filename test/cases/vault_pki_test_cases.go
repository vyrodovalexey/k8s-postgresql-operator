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

// Package cases contains test case definitions for the k8s-postgresql-operator
package cases

import "time"

// VaultPKITestCase represents a test case for Vault PKI functionality
type VaultPKITestCase struct {
	// Name is the test case name
	Name string
	// Description describes what the test case validates
	Description string
	// TestType indicates whether this is an integration or e2e test
	TestType string
	// Prerequisites lists what must be set up before running the test
	Prerequisites []string
	// Steps describes the test steps
	Steps []string
	// ExpectedResult describes the expected outcome
	ExpectedResult string
	// Timeout is the maximum time the test should take
	Timeout time.Duration
}

// VaultPKIIntegrationTestCases contains all integration test cases for Vault PKI
var VaultPKIIntegrationTestCases = []VaultPKITestCase{
	{
		Name:        "TestIntegration_VaultPKI_IssueCertificate",
		Description: "Verify that a certificate can be issued via Vault PKI secrets engine",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault server running at TEST_VAULT_ADDR",
			"PKI secrets engine enabled at TEST_VAULT_PKI_PATH",
			"PKI role configured at TEST_VAULT_PKI_ROLE",
		},
		Steps: []string{
			"1. Connect to Vault using token authentication",
			"2. Build certificate request with common name and SANs",
			"3. Issue certificate via PKI issue endpoint",
			"4. Verify response contains certificate, private key, and CA",
			"5. Parse and validate the certificate",
		},
		ExpectedResult: "Certificate is issued successfully with valid PEM data",
		Timeout:        30 * time.Second,
	},
	{
		Name:        "TestIntegration_VaultPKI_CertificateContainsSANs",
		Description: "Verify that issued certificates contain the requested Subject Alternative Names",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Issue certificate with multiple DNS SANs and IP SANs",
			"2. Parse the certificate",
			"3. Verify all requested DNS names are present",
			"4. Verify all requested IP addresses are present",
		},
		ExpectedResult: "Certificate contains all requested SANs",
		Timeout:        30 * time.Second,
	},
	{
		Name:        "TestIntegration_VaultPKI_CertificateHasCorrectTTL",
		Description: "Verify that certificate TTL matches the requested value",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Issue certificates with different TTL values (1h, 24h, 720h)",
			"2. Parse each certificate",
			"3. Calculate actual TTL from NotBefore and NotAfter",
			"4. Verify TTL is within expected range",
		},
		ExpectedResult: "Certificate TTL matches requested value within tolerance",
		Timeout:        30 * time.Second,
	},
	{
		Name:        "TestIntegration_VaultPKI_MultipleCertificateIssuance",
		Description: "Verify that multiple certificates have unique serial numbers",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Issue multiple certificates concurrently",
			"2. Collect serial numbers from all certificates",
			"3. Verify no duplicate serial numbers exist",
		},
		ExpectedResult: "Each certificate has a unique serial number",
		Timeout:        60 * time.Second,
	},
	{
		Name:        "TestIntegration_VaultPKI_CACertificateRetrieval",
		Description: "Verify that the CA certificate can be retrieved and matches the issuer",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault PKI secrets engine configured with root CA",
		},
		Steps: []string{
			"1. Retrieve CA certificate from PKI endpoint",
			"2. Parse the CA certificate",
			"3. Verify it is marked as a CA certificate",
			"4. Verify basic constraints are valid",
		},
		ExpectedResult: "CA certificate is accessible and valid",
		Timeout:        30 * time.Second,
	},
	{
		Name:        "TestIntegration_VaultPKI_InvalidRole",
		Description: "Verify proper error handling when using an invalid PKI role",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Attempt to issue certificate with non-existent role",
			"2. Verify an error is returned",
			"3. Verify no certificate is issued",
		},
		ExpectedResult: "Error is returned for invalid role",
		Timeout:        30 * time.Second,
	},
	{
		Name:        "TestIntegration_VaultPKI_RetryOnFailure",
		Description: "Verify retry logic works for transient failures",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Attempt certificate issuance with retry logic",
			"2. Verify successful issuance after retries",
			"3. Verify exponential backoff is applied",
		},
		ExpectedResult: "Certificate is issued successfully with retry mechanism",
		Timeout:        60 * time.Second,
	},
	{
		Name:        "TestIntegration_VaultPKI_CertificateChain",
		Description: "Verify the certificate chain is valid and can be verified",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault PKI secrets engine configured with root CA",
		},
		Steps: []string{
			"1. Issue a leaf certificate",
			"2. Retrieve the issuing CA certificate",
			"3. Build certificate pool with CA",
			"4. Verify leaf certificate against CA",
		},
		ExpectedResult: "Certificate chain verification succeeds",
		Timeout:        30 * time.Second,
	},
	{
		Name:        "TestIntegration_VaultPKI_KeyUsage",
		Description: "Verify certificates have correct key usage extensions",
		TestType:    "integration",
		Prerequisites: []string{
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Issue a certificate",
			"2. Parse the certificate",
			"3. Verify KeyUsage includes DigitalSignature and KeyEncipherment",
			"4. Verify ExtKeyUsage includes ServerAuth",
		},
		ExpectedResult: "Certificate has correct key usage settings",
		Timeout:        30 * time.Second,
	},
}

// VaultPKIE2ETestCases contains all E2E test cases for Vault PKI
var VaultPKIE2ETestCases = []VaultPKITestCase{
	{
		Name:        "TestE2E_VaultPKI_OperatorStartup",
		Description: "Verify operator can start with Vault PKI configuration enabled",
		TestType:    "e2e",
		Prerequisites: []string{
			"Kubernetes cluster available",
			"Vault server accessible from cluster",
			"PKI secrets engine configured",
		},
		Steps: []string{
			"1. Create test namespace",
			"2. Create ConfigMap with Vault PKI configuration",
			"3. Verify ConfigMap is created with correct values",
		},
		ExpectedResult: "Operator configuration with Vault PKI is valid",
		Timeout:        60 * time.Second,
	},
	{
		Name:        "TestE2E_VaultPKI_WebhookCertificateSecret",
		Description: "Verify webhook certificate secret is created from Vault PKI",
		TestType:    "e2e",
		Prerequisites: []string{
			"Kubernetes cluster available",
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Create test namespace",
			"2. Issue certificate from Vault PKI",
			"3. Create Kubernetes TLS secret with certificate",
			"4. Verify secret contains valid certificate data",
			"5. Verify certificate is not expired",
		},
		ExpectedResult: "TLS secret is created with valid Vault PKI certificate",
		Timeout:        60 * time.Second,
	},
	{
		Name:        "TestE2E_VaultPKI_WebhookValidation",
		Description: "Verify webhooks can function with PKI-issued certificates",
		TestType:    "e2e",
		Prerequisites: []string{
			"Kubernetes cluster available",
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Issue certificate from Vault PKI",
			"2. Load certificate for TLS",
			"3. Create CA pool with issuing CA",
			"4. Verify certificate chain",
			"5. Verify certificate can be used for TLS",
		},
		ExpectedResult: "Certificate is valid for webhook TLS",
		Timeout:        60 * time.Second,
	},
	{
		Name:        "TestE2E_VaultPKI_FallbackToSelfSigned",
		Description: "Verify fallback to self-signed certificates when Vault PKI is unavailable",
		TestType:    "e2e",
		Prerequisites: []string{
			"Kubernetes cluster available",
		},
		Steps: []string{
			"1. Create test namespace",
			"2. Create ConfigMap with invalid Vault address",
			"3. Create fallback self-signed certificate secret",
			"4. Verify fallback mechanism is triggered",
		},
		ExpectedResult: "Fallback to self-signed certificates works",
		Timeout:        60 * time.Second,
	},
	{
		Name:        "TestE2E_VaultPKI_CertificateRotation",
		Description: "Verify certificate rotation works with Vault PKI",
		TestType:    "e2e",
		Prerequisites: []string{
			"Kubernetes cluster available",
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Issue initial certificate",
			"2. Create Kubernetes secret with certificate",
			"3. Issue new certificate (rotation)",
			"4. Update Kubernetes secret",
			"5. Verify serial numbers are different",
		},
		ExpectedResult: "Certificate rotation succeeds with unique serial numbers",
		Timeout:        120 * time.Second,
	},
	{
		Name:        "TestE2E_VaultPKI_MultipleNamespaces",
		Description: "Verify PKI certificates work across multiple namespaces",
		TestType:    "e2e",
		Prerequisites: []string{
			"Kubernetes cluster available",
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Create multiple test namespaces",
			"2. Issue certificates for each namespace",
			"3. Create TLS secrets in each namespace",
			"4. Verify all certificates are unique",
			"5. Verify certificate CNs match namespaces",
		},
		ExpectedResult: "Certificates work correctly across namespaces",
		Timeout:        120 * time.Second,
	},
	{
		Name:        "TestE2E_VaultPKI_SecretCleanup",
		Description: "Verify secrets are cleaned up properly",
		TestType:    "e2e",
		Prerequisites: []string{
			"Kubernetes cluster available",
			"Vault PKI secrets engine configured",
		},
		Steps: []string{
			"1. Create test namespace",
			"2. Issue certificate and create secret",
			"3. Delete the secret",
			"4. Verify secret is deleted",
			"5. Clean up namespace",
		},
		ExpectedResult: "Secret cleanup works correctly",
		Timeout:        60 * time.Second,
	},
	{
		Name:        "TestE2E_VaultPKI_HTTPSEndpoint",
		Description: "Verify HTTPS endpoint works with PKI certificate",
		TestType:    "e2e",
		Prerequisites: []string{
			"Vault PKI secrets engine configured",
			"PKI role allows localhost certificates",
		},
		Steps: []string{
			"1. Issue certificate for localhost",
			"2. Start HTTPS server with certificate",
			"3. Create HTTPS client with CA",
			"4. Make request to server",
			"5. Verify successful TLS handshake",
		},
		ExpectedResult: "HTTPS communication works with PKI certificate",
		Timeout:        60 * time.Second,
	},
}

// GetAllVaultPKITestCases returns all Vault PKI test cases
func GetAllVaultPKITestCases() []VaultPKITestCase {
	all := make([]VaultPKITestCase, 0, len(VaultPKIIntegrationTestCases)+len(VaultPKIE2ETestCases))
	all = append(all, VaultPKIIntegrationTestCases...)
	all = append(all, VaultPKIE2ETestCases...)
	return all
}

// GetIntegrationTestCases returns only integration test cases
func GetIntegrationTestCases() []VaultPKITestCase {
	return VaultPKIIntegrationTestCases
}

// GetE2ETestCases returns only E2E test cases
func GetE2ETestCases() []VaultPKITestCase {
	return VaultPKIE2ETestCases
}
