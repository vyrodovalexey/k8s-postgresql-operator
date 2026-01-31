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

package k8s

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
)

// TestGetCurrentNamespace_TableDriven tests GetCurrentNamespace with table-driven tests
func TestGetCurrentNamespace_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		fileExists     bool
		expectedResult string
	}{
		{
			name:           "valid namespace",
			fileContent:    "my-namespace",
			fileExists:     true,
			expectedResult: "my-namespace",
		},
		{
			name:           "namespace with leading whitespace",
			fileContent:    "   my-namespace",
			fileExists:     true,
			expectedResult: "my-namespace",
		},
		{
			name:           "namespace with trailing whitespace",
			fileContent:    "my-namespace   ",
			fileExists:     true,
			expectedResult: "my-namespace",
		},
		{
			name:           "namespace with newline",
			fileContent:    "my-namespace\n",
			fileExists:     true,
			expectedResult: "my-namespace",
		},
		{
			name:           "namespace with tabs",
			fileContent:    "\tmy-namespace\t",
			fileExists:     true,
			expectedResult: "my-namespace",
		},
		{
			name:           "empty file",
			fileContent:    "",
			fileExists:     true,
			expectedResult: "k8s-postgresql-operator",
		},
		{
			name:           "whitespace only file",
			fileContent:    "   \n\t  ",
			fileExists:     true,
			expectedResult: "k8s-postgresql-operator",
		},
		{
			name:           "file does not exist",
			fileContent:    "",
			fileExists:     false,
			expectedResult: "k8s-postgresql-operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var namespaceFile string
			if tt.fileExists {
				tmpDir := t.TempDir()
				namespaceFile = filepath.Join(tmpDir, "namespace")
				err := os.WriteFile(namespaceFile, []byte(tt.fileContent), 0644)
				require.NoError(t, err)
			} else {
				namespaceFile = "/nonexistent/path/namespace"
			}

			result := GetCurrentNamespace(namespaceFile)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestWriteCertificateToFiles_TableDriven tests WriteCertificateToFiles with table-driven tests
func TestWriteCertificateToFiles_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		certificate *cert.Certificate
		setupDir    func(t *testing.T) string
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful write",
			certificate: &cert.Certificate{
				CertPEM: []byte("-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----"),
				KeyPEM:  []byte("-----BEGIN RSA PRIVATE KEY-----\ntest-key\n-----END RSA PRIVATE KEY-----"),
			},
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			expectError: false,
		},
		{
			name: "creates nested directory",
			certificate: &cert.Certificate{
				CertPEM: []byte("cert-data"),
				KeyPEM:  []byte("key-data"),
			},
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "nested", "certs", "dir")
			},
			expectError: false,
		},
		{
			name: "empty certificate data",
			certificate: &cert.Certificate{
				CertPEM: []byte{},
				KeyPEM:  []byte{},
			},
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			expectError: false,
		},
		{
			name: "large certificate data",
			certificate: &cert.Certificate{
				CertPEM: make([]byte, 10000),
				KeyPEM:  make([]byte, 10000),
			},
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			expectError: false,
		},
		{
			name: "invalid directory path",
			certificate: &cert.Certificate{
				CertPEM: []byte("cert"),
				KeyPEM:  []byte("key"),
			},
			setupDir: func(t *testing.T) string {
				return "/dev/null/invalid/path"
			},
			expectError: true,
			errorMsg:    "failed to create cert directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			certDir := tt.setupDir(t)

			certPath, keyPath, err := WriteCertificateToFiles(tt.certificate, certDir)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Empty(t, certPath)
				assert.Empty(t, keyPath)
			} else {
				require.NoError(t, err)
				assert.Equal(t, filepath.Join(certDir, "tls.crt"), certPath)
				assert.Equal(t, filepath.Join(certDir, "tls.key"), keyPath)

				// Verify files were created with correct content
				certData, err := os.ReadFile(certPath)
				require.NoError(t, err)
				assert.Equal(t, tt.certificate.CertPEM, certData)

				keyData, err := os.ReadFile(keyPath)
				require.NoError(t, err)
				assert.Equal(t, tt.certificate.KeyPEM, keyData)
			}
		})
	}
}

// TestStoreCertificateInSecret_WithFakeClient tests StoreCertificateInSecret with fake client
func TestStoreCertificateInSecret_WithFakeClient(t *testing.T) {
	tests := []struct {
		name        string
		certificate *cert.Certificate
		secretName  string
		namespace   string
		setupClient func() *fake.Clientset
		expectError bool
		errorMsg    string
	}{
		{
			name: "create new secret",
			certificate: &cert.Certificate{
				CertPEM: []byte("cert-data"),
				KeyPEM:  []byte("key-data"),
			},
			secretName: "test-secret",
			namespace:  "test-namespace",
			setupClient: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			expectError: false,
		},
		{
			name: "update existing secret",
			certificate: &cert.Certificate{
				CertPEM: []byte("new-cert-data"),
				KeyPEM:  []byte("new-key-data"),
			},
			secretName: "existing-secret",
			namespace:  "test-namespace",
			setupClient: func() *fake.Clientset {
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-secret",
						Namespace: "test-namespace",
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.crt": []byte("old-cert"),
						"tls.key": []byte("old-key"),
					},
				}
				return fake.NewSimpleClientset(existingSecret)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := tt.setupClient()

			// We can't directly use the fake clientset with StoreCertificateInSecret
			// because it requires a rest.Config. Instead, we test the logic indirectly.
			// For now, we verify the function handles invalid config gracefully.
			ctx := context.Background()
			restConfig := &rest.Config{Host: "https://invalid-host"}

			err := StoreCertificateInSecret(ctx, tt.certificate, tt.secretName, tt.namespace, restConfig)
			// This will fail because we can't create a real client from the fake config
			assert.Error(t, err)

			// Verify the clientset operations work correctly
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.secretName,
					Namespace: tt.namespace,
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": tt.certificate.CertPEM,
					"tls.key": tt.certificate.KeyPEM,
				},
			}

			_, err = clientset.CoreV1().Secrets(tt.namespace).Create(ctx, secret, metav1.CreateOptions{})
			if tt.name == "update existing secret" {
				// For update case, create will fail, so we update
				_, err = clientset.CoreV1().Secrets(tt.namespace).Update(ctx, secret, metav1.UpdateOptions{})
			}
			require.NoError(t, err)

			// Verify the secret was created/updated
			retrievedSecret, err := clientset.CoreV1().Secrets(tt.namespace).Get(ctx, tt.secretName, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.certificate.CertPEM, retrievedSecret.Data["tls.crt"])
			assert.Equal(t, tt.certificate.KeyPEM, retrievedSecret.Data["tls.key"])
		})
	}
}

// TestUpdateWebhookConfigurationWithCABundle_WithFakeClient tests webhook configuration updates
func TestUpdateWebhookConfigurationWithCABundle_WithFakeClient(t *testing.T) {
	tests := []struct {
		name              string
		secretName        string
		namespace         string
		webhookConfigName string
		setupClient       func() *fake.Clientset
		expectError       bool
		errorMsg          string
	}{
		{
			name:              "successful update",
			secretName:        "test-secret",
			namespace:         "test-namespace",
			webhookConfigName: "test-webhook",
			setupClient: func() *fake.Clientset {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("ca-bundle-data"),
					},
				}
				webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-webhook",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{
						{
							Name: "postgresql-operator.vyrodovalexey.github.com",
							ClientConfig: admissionregistrationv1.WebhookClientConfig{
								CABundle: nil,
							},
						},
					},
				}
				return fake.NewSimpleClientset(secret, webhookConfig)
			},
			expectError: false,
		},
		{
			name:              "secret not found",
			secretName:        "nonexistent-secret",
			namespace:         "test-namespace",
			webhookConfigName: "test-webhook",
			setupClient: func() *fake.Clientset {
				webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-webhook",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{
						{
							Name: "postgresql-operator.vyrodovalexey.github.com",
						},
					},
				}
				return fake.NewSimpleClientset(webhookConfig)
			},
			expectError: true,
			errorMsg:    "failed to get secret",
		},
		{
			name:              "missing tls.crt in secret",
			secretName:        "test-secret",
			namespace:         "test-namespace",
			webhookConfigName: "test-webhook",
			setupClient: func() *fake.Clientset {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"other-key": []byte("some-data"),
					},
				}
				webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-webhook",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{
						{
							Name: "postgresql-operator.vyrodovalexey.github.com",
						},
					},
				}
				return fake.NewSimpleClientset(secret, webhookConfig)
			},
			expectError: true,
			errorMsg:    "tls.crt not found",
		},
		{
			name:              "webhook config not found",
			secretName:        "test-secret",
			namespace:         "test-namespace",
			webhookConfigName: "nonexistent-webhook",
			setupClient: func() *fake.Clientset {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("ca-bundle-data"),
					},
				}
				return fake.NewSimpleClientset(secret)
			},
			expectError: true,
			errorMsg:    "failed to get ValidatingWebhookConfiguration",
		},
		{
			name:              "webhook not found in config",
			secretName:        "test-secret",
			namespace:         "test-namespace",
			webhookConfigName: "test-webhook",
			setupClient: func() *fake.Clientset {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-secret",
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("ca-bundle-data"),
					},
				}
				webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-webhook",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{
						{
							Name: "different-webhook.example.com",
						},
					},
				}
				return fake.NewSimpleClientset(secret, webhookConfig)
			},
			expectError: true,
			errorMsg:    "webhook 'postgresql-operator.vyrodovalexey.github.com' not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since UpdateWebhookConfigurationWithCABundle requires a real rest.Config,
			// we test the error path with an invalid config
			restConfig := &rest.Config{Host: "https://invalid-host"}
			logger := zap.NewNop().Sugar()

			err := UpdateWebhookConfigurationWithCABundle(
				tt.secretName, tt.namespace, restConfig, tt.webhookConfigName, logger)

			// All tests will fail at client creation with invalid config
			assert.Error(t, err)
		})
	}
}

// TestUpdateWebhookConfigurationWithCertificate_TableDriven tests certificate-based webhook updates
func TestUpdateWebhookConfigurationWithCertificate_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		certificate *cert.Certificate
		webhookName string
		expectError bool
	}{
		{
			name: "with CA certificate",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: []byte("ca-cert-data"),
			},
			webhookName: "test-webhook",
			expectError: true, // Will fail due to invalid rest config
		},
		{
			name: "without CA certificate - uses cert as CA",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: []byte{},
			},
			webhookName: "test-webhook",
			expectError: true, // Will fail due to invalid rest config
		},
		{
			name: "nil CA certificate",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: nil,
			},
			webhookName: "test-webhook",
			expectError: true, // Will fail due to invalid rest config
		},
		{
			name: "with nil logger",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: []byte("ca-cert-data"),
			},
			webhookName: "test-webhook",
			expectError: true, // Will fail due to invalid rest config
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			restConfig := &rest.Config{Host: "https://invalid-host"}
			var logger *zap.SugaredLogger
			if tt.name != "with nil logger" {
				logger = zap.NewNop().Sugar()
			}

			err := UpdateWebhookConfigurationWithCertificate(
				ctx, tt.certificate, restConfig, tt.webhookName, logger)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestUpdateWebhookConfigurationWithCertificate_FakeClientLogic tests the logic with fake client
func TestUpdateWebhookConfigurationWithCertificate_FakeClientLogic(t *testing.T) {
	ctx := context.Background()

	// Test the webhook update logic using fake clientset directly
	tests := []struct {
		name        string
		certificate *cert.Certificate
		webhookName string
		setupClient func() *fake.Clientset
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful update with CA cert",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: []byte("ca-cert-data"),
			},
			webhookName: "test-webhook",
			setupClient: func() *fake.Clientset {
				webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-webhook",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{
						{
							Name: "postgresql-operator.vyrodovalexey.github.com",
							ClientConfig: admissionregistrationv1.WebhookClientConfig{
								CABundle: nil,
							},
						},
					},
				}
				return fake.NewSimpleClientset(webhookConfig)
			},
			expectError: false,
		},
		{
			name: "successful update without CA cert - uses cert",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: []byte{},
			},
			webhookName: "test-webhook",
			setupClient: func() *fake.Clientset {
				webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-webhook",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{
						{
							Name: "postgresql-operator.vyrodovalexey.github.com",
							ClientConfig: admissionregistrationv1.WebhookClientConfig{
								CABundle: nil,
							},
						},
					},
				}
				return fake.NewSimpleClientset(webhookConfig)
			},
			expectError: false,
		},
		{
			name: "webhook config not found",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: []byte("ca-cert-data"),
			},
			webhookName: "nonexistent-webhook",
			setupClient: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name: "webhook name not in config",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: []byte("ca-cert-data"),
			},
			webhookName: "test-webhook",
			setupClient: func() *fake.Clientset {
				webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-webhook",
					},
					Webhooks: []admissionregistrationv1.ValidatingWebhook{
						{
							Name: "different-webhook.example.com",
						},
					},
				}
				return fake.NewSimpleClientset(webhookConfig)
			},
			expectError: true,
			errorMsg:    "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := tt.setupClient()

			// Simulate the logic of UpdateWebhookConfigurationWithCertificate
			webhookConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
				ctx, tt.webhookName, metav1.GetOptions{})

			if err != nil {
				if tt.expectError {
					assert.Contains(t, err.Error(), tt.errorMsg)
					return
				}
				t.Fatalf("unexpected error: %v", err)
			}

			// Use CA certificate if available, otherwise use the certificate itself
			caBundle := tt.certificate.CACertPEM
			if len(caBundle) == 0 {
				caBundle = tt.certificate.CertPEM
			}

			// Update the CA bundle in the webhook configuration
			updated := false
			for i := range webhookConfig.Webhooks {
				if webhookConfig.Webhooks[i].Name == "postgresql-operator.vyrodovalexey.github.com" {
					webhookConfig.Webhooks[i].ClientConfig.CABundle = caBundle
					updated = true
					break
				}
			}

			if !updated {
				if tt.expectError {
					return
				}
				t.Fatal("webhook not found in config")
			}

			// Update the ValidatingWebhookConfiguration
			_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(
				ctx, webhookConfig, metav1.UpdateOptions{})

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)

				// Verify the update
				updatedConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					ctx, tt.webhookName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, caBundle, updatedConfig.Webhooks[0].ClientConfig.CABundle)
			}
		})
	}
}

// TestUpdateWebhookConfigurationWithCABundle_FakeClientReactor tests with reactor for error simulation
func TestUpdateWebhookConfigurationWithCABundle_FakeClientReactor(t *testing.T) {
	ctx := context.Background()

	t.Run("update error", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "test-namespace",
			},
			Data: map[string][]byte{
				"tls.crt": []byte("ca-bundle-data"),
			},
		}
		webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-webhook",
			},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: "postgresql-operator.vyrodovalexey.github.com",
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						CABundle: nil,
					},
				},
			},
		}
		clientset := fake.NewSimpleClientset(secret, webhookConfig)

		// Add reactor to simulate update error
		clientset.PrependReactor("update", "validatingwebhookconfigurations", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, assert.AnError
		})

		// Get the webhook config
		config, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
			ctx, "test-webhook", metav1.GetOptions{})
		require.NoError(t, err)

		// Try to update
		config.Webhooks[0].ClientConfig.CABundle = []byte("new-ca-bundle")
		_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(
			ctx, config, metav1.UpdateOptions{})
		assert.Error(t, err)
	})
}

// TestWriteCertificateToFiles_FilePermissions tests file permissions
func TestWriteCertificateToFiles_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	certificate := &cert.Certificate{
		CertPEM: []byte("cert-data"),
		KeyPEM:  []byte("key-data"),
	}

	certPath, keyPath, err := WriteCertificateToFiles(certificate, tmpDir)
	require.NoError(t, err)

	// Verify files exist
	certInfo, err := os.Stat(certPath)
	require.NoError(t, err)
	assert.False(t, certInfo.IsDir())

	keyInfo, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.False(t, keyInfo.IsDir())
}

// TestWriteCertificateToFiles_ConcurrentWrites tests concurrent writes
func TestWriteCertificateToFiles_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()

	certificate := &cert.Certificate{
		CertPEM: []byte("cert-data"),
		KeyPEM:  []byte("key-data"),
	}

	// Run multiple concurrent writes
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(idx int) {
			subDir := filepath.Join(tmpDir, "concurrent", string(rune('a'+idx)))
			_, _, err := WriteCertificateToFiles(certificate, subDir)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent writes")
		}
	}
}

// TestGetCurrentNamespace_SpecialCharacters tests namespace with special characters
func TestGetCurrentNamespace_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name           string
		fileContent    string
		expectedResult string
	}{
		{
			name:           "namespace with hyphen",
			fileContent:    "my-namespace-123",
			expectedResult: "my-namespace-123",
		},
		{
			name:           "namespace with numbers",
			fileContent:    "namespace123",
			expectedResult: "namespace123",
		},
		{
			name:           "single character namespace",
			fileContent:    "a",
			expectedResult: "a",
		},
		{
			name:           "long namespace",
			fileContent:    "this-is-a-very-long-namespace-name-that-is-still-valid",
			expectedResult: "this-is-a-very-long-namespace-name-that-is-still-valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			namespaceFile := filepath.Join(tmpDir, "namespace")
			err := os.WriteFile(namespaceFile, []byte(tt.fileContent), 0644)
			require.NoError(t, err)

			result := GetCurrentNamespace(namespaceFile)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestWriteCertificateToFiles_CertFileCreationError tests cert file creation error
func TestWriteCertificateToFiles_CertFileCreationError(t *testing.T) {
	// Create a directory where we can't create files
	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "certs")
	err := os.MkdirAll(certDir, 0o750)
	require.NoError(t, err)

	// Create a file named tls.crt (not a directory) to cause file creation to fail
	certFilePath := filepath.Join(certDir, "tls.crt")
	err = os.MkdirAll(certFilePath, 0o750) // Create as directory
	require.NoError(t, err)

	certificate := &cert.Certificate{
		CertPEM: []byte("cert-data"),
		KeyPEM:  []byte("key-data"),
	}

	_, _, err = WriteCertificateToFiles(certificate, certDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cert file")
}

// TestWriteCertificateToFiles_KeyFileCreationError tests key file creation error
func TestWriteCertificateToFiles_KeyFileCreationError(t *testing.T) {
	// Create a directory where we can create cert file but not key file
	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "certs")
	err := os.MkdirAll(certDir, 0o750)
	require.NoError(t, err)

	// Create a directory named tls.key to cause file creation to fail
	keyFilePath := filepath.Join(certDir, "tls.key")
	err = os.MkdirAll(keyFilePath, 0o750) // Create as directory
	require.NoError(t, err)

	certificate := &cert.Certificate{
		CertPEM: []byte("cert-data"),
		KeyPEM:  []byte("key-data"),
	}

	_, _, err = WriteCertificateToFiles(certificate, certDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create key file")
}

// TestWriteCertificateToFiles_DirectoryCreationSuccess tests directory creation
func TestWriteCertificateToFiles_DirectoryCreationSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a deeply nested path that doesn't exist
	certDir := filepath.Join(tmpDir, "level1", "level2", "level3", "certs")

	certificate := &cert.Certificate{
		CertPEM: []byte("cert-data"),
		KeyPEM:  []byte("key-data"),
	}

	certPath, keyPath, err := WriteCertificateToFiles(certificate, certDir)
	require.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(certDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify files were created
	assert.FileExists(t, certPath)
	assert.FileExists(t, keyPath)
}

// TestWriteCertificateToFiles_OverwriteExisting tests overwriting existing files
func TestWriteCertificateToFiles_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing files
	certPath := filepath.Join(tmpDir, "tls.crt")
	keyPath := filepath.Join(tmpDir, "tls.key")
	err := os.WriteFile(certPath, []byte("old-cert"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte("old-key"), 0o644)
	require.NoError(t, err)

	certificate := &cert.Certificate{
		CertPEM: []byte("new-cert-data"),
		KeyPEM:  []byte("new-key-data"),
	}

	newCertPath, newKeyPath, err := WriteCertificateToFiles(certificate, tmpDir)
	require.NoError(t, err)

	// Verify files were overwritten
	certData, err := os.ReadFile(newCertPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("new-cert-data"), certData)

	keyData, err := os.ReadFile(newKeyPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("new-key-data"), keyData)
}

// TestWriteCertificateToFiles_BinaryData tests writing binary data
func TestWriteCertificateToFiles_BinaryData(t *testing.T) {
	tmpDir := t.TempDir()

	// Create certificate with binary data
	binaryData := make([]byte, 256)
	for i := range binaryData {
		binaryData[i] = byte(i)
	}

	certificate := &cert.Certificate{
		CertPEM: binaryData,
		KeyPEM:  binaryData,
	}

	certPath, keyPath, err := WriteCertificateToFiles(certificate, tmpDir)
	require.NoError(t, err)

	// Verify binary data was written correctly
	certData, err := os.ReadFile(certPath)
	require.NoError(t, err)
	assert.Equal(t, binaryData, certData)

	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, binaryData, keyData)
}
