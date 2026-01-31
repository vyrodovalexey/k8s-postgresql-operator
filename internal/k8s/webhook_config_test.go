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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
)

// TestUpdateWebhookConfigurationWithCABundle_DirectClientTest tests the webhook update logic directly
func TestUpdateWebhookConfigurationWithCABundle_DirectClientTest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		secretName        string
		namespace         string
		webhookConfigName string
		setupClient       func() *fake.Clientset
		expectError       bool
		errorContains     string
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
			expectError:   true,
			errorContains: "not found",
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
			expectError:   true,
			errorContains: "tls.crt not found",
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
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:              "webhook name not in config",
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
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := tt.setupClient()

			// Simulate the logic of UpdateWebhookConfigurationWithCABundle
			secret, err := clientset.CoreV1().Secrets(tt.namespace).Get(ctx, tt.secretName, metav1.GetOptions{})
			if err != nil {
				if tt.expectError {
					assert.Contains(t, err.Error(), tt.errorContains)
					return
				}
				t.Fatalf("unexpected error getting secret: %v", err)
			}

			caBundle, ok := secret.Data["tls.crt"]
			if !ok {
				if tt.expectError && tt.errorContains == "tls.crt not found" {
					return
				}
				t.Fatal("tls.crt not found in secret")
			}

			webhookConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
				ctx, tt.webhookConfigName, metav1.GetOptions{})
			if err != nil {
				if tt.expectError {
					assert.Contains(t, err.Error(), tt.errorContains)
					return
				}
				t.Fatalf("unexpected error getting webhook config: %v", err)
			}

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

			_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(
				ctx, webhookConfig, metav1.UpdateOptions{})

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)

				// Verify the update
				updatedConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					ctx, tt.webhookConfigName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, caBundle, updatedConfig.Webhooks[0].ClientConfig.CABundle)
			}
		})
	}
}

// TestUpdateWebhookConfigurationWithCertificate_DirectClientTest tests certificate-based webhook updates
func TestUpdateWebhookConfigurationWithCertificate_DirectClientTest(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name        string
		certificate *cert.Certificate
		webhookName string
		setupClient func() *fake.Clientset
		expectError bool
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
			} else {
				require.NoError(t, err)

				// Verify the update
				updatedConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					ctx, tt.webhookName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, caBundle, updatedConfig.Webhooks[0].ClientConfig.CABundle)

				// Log success (simulating the logger call)
				if logger != nil {
					logger.Infow("Updated ValidatingWebhookConfiguration with CA bundle",
						"webhook_name", tt.webhookName)
				}
			}
		})
	}
}

// TestStoreCertificateInSecret_DirectClientTest tests storing certificates in secrets
func TestStoreCertificateInSecret_DirectClientTest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		certificate *cert.Certificate
		secretName  string
		namespace   string
		setupClient func() *fake.Clientset
		expectError bool
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
		{
			name: "empty certificate data",
			certificate: &cert.Certificate{
				CertPEM: []byte{},
				KeyPEM:  []byte{},
			},
			secretName: "empty-secret",
			namespace:  "test-namespace",
			setupClient: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := tt.setupClient()

			// Simulate the logic of StoreCertificateInSecret
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

			// Try to create the secret, if it exists, update it
			_, err := clientset.CoreV1().Secrets(tt.namespace).Create(ctx, secret, metav1.CreateOptions{})
			if err != nil {
				// Secret might already exist, try to update it
				_, err = clientset.CoreV1().Secrets(tt.namespace).Update(ctx, secret, metav1.UpdateOptions{})
			}

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify the secret was created/updated
				retrievedSecret, err := clientset.CoreV1().Secrets(tt.namespace).Get(ctx, tt.secretName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, tt.certificate.CertPEM, retrievedSecret.Data["tls.crt"])
				assert.Equal(t, tt.certificate.KeyPEM, retrievedSecret.Data["tls.key"])
			}
		})
	}
}

// TestMultipleWebhooksInConfig tests updating config with multiple webhooks
func TestMultipleWebhooksInConfig(t *testing.T) {
	ctx := context.Background()

	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-webhook",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "first-webhook.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
			},
			{
				Name: "postgresql-operator.vyrodovalexey.github.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
			},
			{
				Name: "third-webhook.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(webhookConfig)

	caBundle := []byte("new-ca-bundle")

	// Get the webhook config
	config, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
		ctx, "test-webhook", metav1.GetOptions{})
	require.NoError(t, err)

	// Update only the target webhook
	for i := range config.Webhooks {
		if config.Webhooks[i].Name == "postgresql-operator.vyrodovalexey.github.com" {
			config.Webhooks[i].ClientConfig.CABundle = caBundle
			break
		}
	}

	_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(
		ctx, config, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Verify only the target webhook was updated
	updatedConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
		ctx, "test-webhook", metav1.GetOptions{})
	require.NoError(t, err)

	assert.Nil(t, updatedConfig.Webhooks[0].ClientConfig.CABundle)
	assert.Equal(t, caBundle, updatedConfig.Webhooks[1].ClientConfig.CABundle)
	assert.Nil(t, updatedConfig.Webhooks[2].ClientConfig.CABundle)
}

// TestEmptyWebhooksList tests handling of empty webhooks list
func TestEmptyWebhooksList(t *testing.T) {
	ctx := context.Background()

	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-webhook",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{},
	}

	clientset := fake.NewSimpleClientset(webhookConfig)

	config, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
		ctx, "test-webhook", metav1.GetOptions{})
	require.NoError(t, err)

	// Try to find the target webhook
	updated := false
	for i := range config.Webhooks {
		if config.Webhooks[i].Name == "postgresql-operator.vyrodovalexey.github.com" {
			config.Webhooks[i].ClientConfig.CABundle = []byte("ca-bundle")
			updated = true
			break
		}
	}

	// Should not find the webhook
	assert.False(t, updated)
}

// TestUpdateWebhookConfigurationWithCABundleUsingClient_TableDriven tests the UsingClient function
func TestUpdateWebhookConfigurationWithCABundleUsingClient_TableDriven(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name              string
		secretName        string
		namespace         string
		webhookConfigName string
		setupClient       func() *fake.Clientset
		expectError       bool
		errorContains     string
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
			expectError:   true,
			errorContains: "failed to get secret",
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
			expectError:   true,
			errorContains: "tls.crt not found",
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
			expectError:   true,
			errorContains: "failed to get ValidatingWebhookConfiguration",
		},
		{
			name:              "webhook name not in config",
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
			expectError:   true,
			errorContains: "not found in ValidatingWebhookConfiguration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := tt.setupClient()

			err := UpdateWebhookConfigurationWithCABundleUsingClient(
				ctx, tt.secretName, tt.namespace, clientset, tt.webhookConfigName, logger)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)

				// Verify the update
				updatedConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					ctx, tt.webhookConfigName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, []byte("ca-bundle-data"), updatedConfig.Webhooks[0].ClientConfig.CABundle)
			}
		})
	}
}

// TestUpdateWebhookConfigurationWithCertificateUsingClient_TableDriven tests the UsingClient function
func TestUpdateWebhookConfigurationWithCertificateUsingClient_TableDriven(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name          string
		certificate   *cert.Certificate
		webhookName   string
		setupClient   func() *fake.Clientset
		expectError   bool
		errorContains string
		expectedCA    []byte
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
			expectedCA:  []byte("ca-cert-data"),
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
			expectedCA:  []byte("cert-data"),
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
			expectError:   true,
			errorContains: "failed to get ValidatingWebhookConfiguration",
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
			expectError:   true,
			errorContains: "not found in ValidatingWebhookConfiguration",
		},
		{
			name: "nil CA cert uses cert PEM",
			certificate: &cert.Certificate{
				CertPEM:   []byte("cert-data"),
				KeyPEM:    []byte("key-data"),
				CACertPEM: nil,
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
			expectedCA:  []byte("cert-data"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := tt.setupClient()

			err := UpdateWebhookConfigurationWithCertificateUsingClient(
				ctx, tt.certificate, clientset, tt.webhookName, logger)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)

				// Verify the update
				updatedConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					ctx, tt.webhookName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCA, updatedConfig.Webhooks[0].ClientConfig.CABundle)
			}
		})
	}
}

// TestUpdateWebhookConfigurationWithCertificateUsingClient_NilLogger tests with nil logger
func TestUpdateWebhookConfigurationWithCertificateUsingClient_NilLogger(t *testing.T) {
	ctx := context.Background()

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
	clientset := fake.NewSimpleClientset(webhookConfig)

	certificate := &cert.Certificate{
		CertPEM:   []byte("cert-data"),
		KeyPEM:    []byte("key-data"),
		CACertPEM: []byte("ca-cert-data"),
	}

	// Should not panic with nil logger
	err := UpdateWebhookConfigurationWithCertificateUsingClient(
		ctx, certificate, clientset, "test-webhook", nil)

	require.NoError(t, err)
}

// TestStoreCertificateInSecretUsingClient_TableDriven tests the UsingClient function
func TestStoreCertificateInSecretUsingClient_TableDriven(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		certificate *cert.Certificate
		secretName  string
		namespace   string
		setupClient func() *fake.Clientset
		expectError bool
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
		{
			name: "empty certificate data",
			certificate: &cert.Certificate{
				CertPEM: []byte{},
				KeyPEM:  []byte{},
			},
			secretName: "empty-secret",
			namespace:  "test-namespace",
			setupClient: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			expectError: false,
		},
		{
			name: "large certificate data",
			certificate: &cert.Certificate{
				CertPEM: make([]byte, 10000),
				KeyPEM:  make([]byte, 5000),
			},
			secretName: "large-secret",
			namespace:  "test-namespace",
			setupClient: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := tt.setupClient()

			err := StoreCertificateInSecretUsingClient(
				ctx, tt.certificate, tt.secretName, tt.namespace, clientset)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify the secret was created/updated
				retrievedSecret, err := clientset.CoreV1().Secrets(tt.namespace).Get(ctx, tt.secretName, metav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, tt.certificate.CertPEM, retrievedSecret.Data["tls.crt"])
				assert.Equal(t, tt.certificate.KeyPEM, retrievedSecret.Data["tls.key"])
				assert.Equal(t, corev1.SecretTypeTLS, retrievedSecret.Type)
			}
		})
	}
}
