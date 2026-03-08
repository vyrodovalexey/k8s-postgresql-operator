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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	certpkg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

const (
	testNamespace     = "test-namespace"
	testWebhookConfig = "test-webhook-config"
)

// --- GetCurrentNamespace tests ---

func TestGetCurrentNamespace(t *testing.T) {
	tests := []struct {
		name          string
		setupFile     func(t *testing.T) string
		wantNamespace string
	}{
		{
			name: "valid file with namespace",
			setupFile: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				f := filepath.Join(tmpDir, "namespace")
				err := os.WriteFile(f, []byte("test-namespace"), 0644)
				require.NoError(t, err)
				return f
			},
			wantNamespace: "test-namespace",
		},
		{
			name: "file with whitespace",
			setupFile: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				f := filepath.Join(tmpDir, "namespace")
				err := os.WriteFile(f, []byte("  test-namespace  \n"), 0644)
				require.NoError(t, err)
				return f
			},
			wantNamespace: "test-namespace",
		},
		{
			name: "empty file",
			setupFile: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				f := filepath.Join(tmpDir, "namespace")
				err := os.WriteFile(f, []byte(""), 0644)
				require.NoError(t, err)
				return f
			},
			wantNamespace: "k8s-postgresql-operator",
		},
		{
			name: "non-existent file",
			setupFile: func(_ *testing.T) string {
				return "/non/existent/path/namespace"
			},
			wantNamespace: "k8s-postgresql-operator",
		},
		{
			name: "whitespace-only file",
			setupFile: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				f := filepath.Join(tmpDir, "namespace")
				err := os.WriteFile(f, []byte("   \n\t  "), 0644)
				require.NoError(t, err)
				return f
			},
			wantNamespace: "k8s-postgresql-operator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespaceFile := tt.setupFile(t)
			result := GetCurrentNamespace(namespaceFile)
			assert.Equal(t, tt.wantNamespace, result)
		})
	}
}

// --- UpdateWebhookConfigurationWithCABundle tests ---

// newFakeAPIServer creates a fake Kubernetes API server for testing
// It handles the specific endpoints needed by UpdateWebhookConfigurationWithCABundle
func newFakeAPIServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, handler := range handlers {
		mux.HandleFunc(path, handler)
	}
	return httptest.NewServer(mux)
}

func TestUpdateWebhookConfigurationWithCABundle_Success(t *testing.T) {
	// Arrange
	secretName := "test-secret"
	namespace := testNamespace
	webhookConfigName := testWebhookConfig
	caBundle := []byte("test-ca-bundle-data")

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": caBundle,
		},
	}

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookConfigName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "postgresql-operator.vyrodovalexey.github.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(secret)
			_, _ = w.Write(data)
		},
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/" + webhookConfigName: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(webhookConfig)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				// Return the webhook config as-is with updated CA bundle
				updatedConfig := webhookConfig.DeepCopy()
				updatedConfig.Webhooks[0].ClientConfig.CABundle = caBundle
				data, _ := json.Marshal(updatedConfig)
				_, _ = w.Write(data)
			}
		},
	})
	defer server.Close()

	restConfig := &rest.Config{
		Host:          server.URL,
		ContentConfig: rest.ContentConfig{ContentType: "application/json"},
	}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookConfigurationWithCABundle(context.Background(), secretName, namespace, restConfig, webhookConfigName, logger)

	// Assert
	assert.NoError(t, err)
}

func TestUpdateWebhookConfigurationWithCABundle_SecretNotFound(t *testing.T) {
	// Arrange
	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + testNamespace + "/secrets/nonexistent-secret": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","reason":"NotFound","code":404}`))
		},
	})
	defer server.Close()

	restConfig := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookConfigurationWithCABundle(context.Background(), "nonexistent-secret", testNamespace, restConfig, testWebhookConfig, logger)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get secret")
}

func TestUpdateWebhookConfigurationWithCABundle_MissingTLSCrt(t *testing.T) {
	// Arrange
	secretName := "test-secret"
	namespace := testNamespace

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"other-key": []byte("value"),
		},
	}

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(secret)
			_, _ = w.Write(data)
		},
	})
	defer server.Close()

	restConfig := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookConfigurationWithCABundle(context.Background(), secretName, namespace, restConfig, testWebhookConfig, logger)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tls.crt not found in secret")
}

func TestUpdateWebhookConfigurationWithCABundle_WebhookConfigNotFound(t *testing.T) {
	// Arrange
	secretName := "test-secret"
	namespace := testNamespace
	webhookConfigName := "nonexistent-webhook"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("ca-bundle"),
		},
	}

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(secret)
			_, _ = w.Write(data)
		},
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/" + webhookConfigName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","reason":"NotFound","code":404}`))
		},
	})
	defer server.Close()

	restConfig := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookConfigurationWithCABundle(context.Background(), secretName, namespace, restConfig, webhookConfigName, logger)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get ValidatingWebhookConfiguration")
}

func TestUpdateWebhookConfigurationWithCABundle_WebhookNotFoundInConfig(t *testing.T) {
	// Arrange: webhook config exists but doesn't contain the expected webhook name
	secretName := "test-secret"
	namespace := testNamespace
	webhookConfigName := testWebhookConfig

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("ca-bundle"),
		},
	}

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookConfigName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "some-other-webhook.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(secret)
			_, _ = w.Write(data)
		},
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/" + webhookConfigName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(webhookConfig)
			_, _ = w.Write(data)
		},
	})
	defer server.Close()

	restConfig := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookConfigurationWithCABundle(context.Background(), secretName, namespace, restConfig, webhookConfigName, logger)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook 'postgresql-operator.vyrodovalexey.github.com' not found")
}

func TestUpdateWebhookConfigurationWithCABundle_EmptyWebhooksList(t *testing.T) {
	// Arrange: webhook config exists but has no webhooks
	secretName := "test-secret"
	namespace := testNamespace
	webhookConfigName := testWebhookConfig

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("ca-bundle"),
		},
	}

	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookConfigName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{},
	}

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(secret)
			_, _ = w.Write(data)
		},
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/" + webhookConfigName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(webhookConfig)
			_, _ = w.Write(data)
		},
	})
	defer server.Close()

	restConfig := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookConfigurationWithCABundle(context.Background(), secretName, namespace, restConfig, webhookConfigName, logger)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook 'postgresql-operator.vyrodovalexey.github.com' not found")
}

func TestUpdateWebhookConfigurationWithCABundle_UpdateFails(t *testing.T) {
	// Arrange: everything works except the update call
	secretName := "test-secret"
	namespace := testNamespace
	webhookConfigName := testWebhookConfig

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("ca-bundle"),
		},
	}

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: webhookConfigName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "postgresql-operator.vyrodovalexey.github.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(secret)
			_, _ = w.Write(data)
		},
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/" + webhookConfigName: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(webhookConfig)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","message":"internal error","reason":"InternalError","code":500}`))
			}
		},
	})
	defer server.Close()

	restConfig := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookConfigurationWithCABundle(context.Background(), secretName, namespace, restConfig, webhookConfigName, logger)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update ValidatingWebhookConfiguration")
}

func TestUpdateWebhookConfigurationWithCABundle_InvalidRestConfig(t *testing.T) {
	// Arrange: use an invalid rest config that will fail client creation
	restConfig := &rest.Config{
		Host: "https://invalid-host:99999",
		TLSClientConfig: rest.TLSClientConfig{
			// Use an invalid cert file to force client creation failure
			CertFile: "/nonexistent/cert",
			KeyFile:  "/nonexistent/key",
		},
	}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookConfigurationWithCABundle(context.Background(), "secret", "ns", restConfig, "webhook", logger)

	// Assert
	assert.Error(t, err)
}

// --- SetupWebhookCertificates tests ---

func TestSetupWebhookCertificates_WithProvidedCertPath(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		WebhookCertPath: "/etc/certs",
		WebhookCertName: "tls.crt",
		WebhookCertKey:  "tls.key",
	}
	logger := zap.NewNop().Sugar()

	// Act
	certPath, keyPath := SetupWebhookCertificates(context.Background(), cfg, []string{"webhook1"}, logger)

	// Assert
	assert.Equal(t, "/etc/certs/tls.crt", certPath)
	assert.Equal(t, "/etc/certs/tls.key", keyPath)
}

func TestSetupWebhookCertificates_WithCustomCertNames(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		WebhookCertPath: "/custom/path",
		WebhookCertName: "server.crt",
		WebhookCertKey:  "server.key",
	}
	logger := zap.NewNop().Sugar()

	// Act
	certPath, keyPath := SetupWebhookCertificates(context.Background(), cfg, []string{"wh1", "wh2"}, logger)

	// Assert
	assert.Equal(t, "/custom/path/server.crt", certPath)
	assert.Equal(t, "/custom/path/server.key", keyPath)
}

func TestSetupWebhookCertificates_WithEmptyWebhookNames(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		WebhookCertPath: "/certs",
		WebhookCertName: "tls.crt",
		WebhookCertKey:  "tls.key",
	}
	logger := zap.NewNop().Sugar()

	// Act
	certPath, keyPath := SetupWebhookCertificates(context.Background(), cfg, []string{}, logger)

	// Assert
	assert.Equal(t, "/certs/tls.crt", certPath)
	assert.Equal(t, "/certs/tls.key", keyPath)
}

func TestSetupWebhookCertificates_WithNilWebhookNames(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		WebhookCertPath: "/my/certs/dir",
		WebhookCertName: "cert.pem",
		WebhookCertKey:  "key.pem",
	}
	logger := zap.NewNop().Sugar()

	// Act
	certPath, keyPath := SetupWebhookCertificates(context.Background(), cfg, nil, logger)

	// Assert
	assert.Equal(t, "/my/certs/dir/cert.pem", certPath)
	assert.Equal(t, "/my/certs/dir/key.pem", keyPath)
}

// TestSetupWebhookCertificates_GenerateCerts_ExitsOnNoKubeconfig tests the code path
// where WebhookCertPath is empty, triggering self-signed cert generation.
// This path calls ctrl.GetConfigOrDie() which calls os.Exit(1) when no kubeconfig is available.
// We use a subprocess test pattern to verify this behavior.
// --- UpdateWebhookCABundleFromPEM tests ---

func TestUpdateWebhookCABundleFromPEM_Success(t *testing.T) {
	// Arrange
	webhookName := "test-webhook-pem"
	caBundle := []byte("test-ca-pem-data")

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	whCfg := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{Name: webhookName},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "postgresql-operator.vyrodovalexey.github.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	whPath := "/apis/admissionregistration.k8s.io/v1/" +
		"validatingwebhookconfigurations/" + webhookName
	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		whPath: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				updated := whCfg.DeepCopy()
				updated.Webhooks[0].ClientConfig.CABundle = caBundle
				data, _ := json.Marshal(updated)
				_, _ = w.Write(data)
			}
		},
	})
	defer server.Close()

	restCfg := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookCABundleFromPEM(
		context.Background(), caBundle, restCfg, webhookName, logger,
	)

	// Assert
	assert.NoError(t, err)
}

func TestUpdateWebhookCABundleFromPEM_WebhookNotFound(t *testing.T) {
	// Arrange — webhook config exists but no matching webhook name
	webhookName := "test-webhook-pem-nf"

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	whCfg := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{Name: webhookName},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "other-webhook.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	whPath := "/apis/admissionregistration.k8s.io/v1/" +
		"validatingwebhookconfigurations/" + webhookName
	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		whPath: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(whCfg)
			_, _ = w.Write(data)
		},
	})
	defer server.Close()

	restCfg := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookCABundleFromPEM(
		context.Background(), []byte("ca"), restCfg, webhookName, logger,
	)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found in webhook configuration")
}

func TestUpdateWebhookCABundleFromPEM_GetError(t *testing.T) {
	// Arrange — GET returns 404
	webhookName := "nonexistent-webhook-pem"

	whPath := "/apis/admissionregistration.k8s.io/v1/" +
		"validatingwebhookconfigurations/" + webhookName
	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		whPath: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			resp := `{"kind":"Status","apiVersion":"v1",` +
				`"status":"Failure","message":"not found",` +
				`"reason":"NotFound","code":404}`
			_, _ = w.Write([]byte(resp))
		},
	})
	defer server.Close()

	restCfg := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookCABundleFromPEM(
		context.Background(), []byte("ca"), restCfg, webhookName, logger,
	)

	// Assert
	require.Error(t, err)
	assert.Contains(
		t, err.Error(),
		"failed to get ValidatingWebhookConfiguration",
	)
}

func TestUpdateWebhookCABundleFromPEM_UpdateError(t *testing.T) {
	// Arrange — GET succeeds, PUT returns 500
	webhookName := "test-webhook-pem-ue"

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	whCfg := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{Name: webhookName},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "postgresql-operator.vyrodovalexey.github.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	whPath := "/apis/admissionregistration.k8s.io/v1/" +
		"validatingwebhookconfigurations/" + webhookName
	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		whPath: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusInternalServerError)
				resp := `{"kind":"Status","apiVersion":"v1",` +
					`"status":"Failure","message":"err",` +
					`"reason":"InternalError","code":500}`
				_, _ = w.Write([]byte(resp))
			}
		},
	})
	defer server.Close()

	restCfg := &rest.Config{Host: server.URL}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookCABundleFromPEM(
		context.Background(), []byte("ca"), restCfg, webhookName, logger,
	)

	// Assert
	require.Error(t, err)
	assert.Contains(
		t, err.Error(),
		"failed to update ValidatingWebhookConfiguration",
	)
}

func TestUpdateWebhookCABundleFromPEM_InvalidRestConfig(t *testing.T) {
	// Arrange: use an invalid rest config that will fail client creation
	restConfig := &rest.Config{
		Host: "https://invalid-host:99999",
		TLSClientConfig: rest.TLSClientConfig{
			CertFile: "/nonexistent/cert",
			KeyFile:  "/nonexistent/key",
		},
	}
	logger := zap.NewNop().Sugar()

	// Act
	err := UpdateWebhookCABundleFromPEM(
		context.Background(), []byte("ca"), restConfig, "webhook", logger,
	)

	// Assert
	assert.Error(t, err)
}

func TestSetupWebhookCertificatesWithVaultPKI_IssueCertError_Exits(t *testing.T) {
	if os.Getenv("TEST_VAULT_PKI_ISSUE_EXIT") == "1" {
		// Create a VaultPKIManager that will fail on IssueCertificateAndWriteToDisk
		// by using a nil vault client (will panic/fail)
		tmpDir := t.TempDir()
		mgr := certpkg.NewVaultPKIManager(
			nil, // nil client will cause failure
			"pki", "role", "720h",
			24*time.Hour,
			"test-service", "test-ns", tmpDir,
			zap.NewNop().Sugar(),
		)
		cfg := &config.Config{}
		logger := zap.NewNop().Sugar()
		SetupWebhookCertificatesWithVaultPKI(context.Background(), mgr, cfg, []string{"wh1"}, logger)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestSetupWebhookCertificatesWithVaultPKI_IssueCertError_Exits")
	cmd.Env = append(os.Environ(),
		"TEST_VAULT_PKI_ISSUE_EXIT=1",
		"KUBECONFIG=/nonexistent/kubeconfig",
	)
	err := cmd.Run()

	assert.Error(t, err, "expected subprocess to exit with error due to os.Exit(1)")
	if exitErr, ok := err.(*exec.ExitError); ok {
		assert.NotEqual(t, 0, exitErr.ExitCode(), "expected non-zero exit code")
	}
}

func TestSetupWebhookCertificates_GenerateCerts_ExitsOnNoKubeconfig(t *testing.T) {
	if os.Getenv("TEST_SETUP_WEBHOOK_EXIT") == "1" {
		cfg := &config.Config{
			WebhookCertPath:       "", // Empty to trigger generation
			WebhookCertName:       "tls.crt",
			WebhookCertKey:        "tls.key",
			WebhookK8sServiceName: "test-service",
			K8sNamespacePath:      "/nonexistent",
		}
		logger := zap.NewNop().Sugar()
		SetupWebhookCertificates(context.Background(), cfg, []string{"webhook1"}, logger)
		return
	}

	// Run this test in a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestSetupWebhookCertificates_GenerateCerts_ExitsOnNoKubeconfig")
	cmd.Env = append(os.Environ(),
		"TEST_SETUP_WEBHOOK_EXIT=1",
		"KUBECONFIG=/nonexistent/kubeconfig",
	)
	err := cmd.Run()

	// The subprocess should exit with a non-zero exit code
	assert.Error(t, err, "expected subprocess to exit with error due to os.Exit(1)")
	if exitErr, ok := err.(*exec.ExitError); ok {
		assert.NotEqual(t, 0, exitErr.ExitCode(), "expected non-zero exit code")
	}
}

// --- mockVaultPKIClient for testing VaultPKI integration ---

type mockVaultPKIClient struct {
	issueCertFunc func(
		ctx context.Context,
		mountPath, roleName, commonName string,
		altNames []string, ipSANs []string, ttl string,
	) (*vault.PKICertificate, error)
	getCACertFunc func(
		ctx context.Context, mountPath string,
	) (string, error)
}

func (m *mockVaultPKIClient) IssueCertificate(
	ctx context.Context,
	mountPath, roleName, commonName string,
	altNames []string, ipSANs []string, ttl string,
) (*vault.PKICertificate, error) {
	if m.issueCertFunc != nil {
		return m.issueCertFunc(ctx, mountPath, roleName, commonName, altNames, ipSANs, ttl)
	}
	return nil, fmt.Errorf("issueCertFunc not set")
}

func (m *mockVaultPKIClient) GetCACertificate(
	ctx context.Context, mountPath string,
) (string, error) {
	if m.getCACertFunc != nil {
		return m.getCACertFunc(ctx, mountPath)
	}
	return "", fmt.Errorf("getCACertFunc not set")
}

// --- setupWebhookCertificatesInternal tests ---

func TestSetupWebhookCertificatesInternal_WithProvidedCertPath(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		WebhookCertPath: "/etc/certs",
		WebhookCertName: "tls.crt",
		WebhookCertKey:  "tls.key",
	}
	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		t.Fatal("getConfig should not be called when cert path is provided")
		return nil
	}

	// Act
	certPath, keyPath, err := setupWebhookCertificatesInternal(
		context.Background(), cfg, []string{"webhook1"}, logger, getConfig,
	)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/etc/certs/tls.crt", certPath)
	assert.Equal(t, "/etc/certs/tls.key", keyPath)
}

func TestSetupWebhookCertificatesInternal_GenerateCertsSuccess(t *testing.T) {
	// Arrange: set up a fake K8s API server that handles secret creation and webhook config.
	// Force JSON content type so the K8s client doesn't use protobuf.
	secretName := "test-service-webhook-cert"
	namespace := "k8s-postgresql-operator" // default namespace when file doesn't exist

	// Track the stored secret JSON so GET after POST returns the created secret
	var storedSecretJSON []byte
	secretCreated := false

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	webhookConfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "webhook1"},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name: "postgresql-operator.vyrodovalexey.github.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					CABundle: nil,
				},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	mux := http.NewServeMux()
	// Handle secret GET by name — returns stored secret after creation
	mux.HandleFunc("/api/v1/namespaces/"+namespace+"/secrets/"+secretName, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			if secretCreated && storedSecretJSON != nil {
				_, _ = w.Write(storedSecretJSON)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
			_, _ = w.Write([]byte(resp))
		}
	})
	// Handle secret creation via POST — decode JSON body and store it
	mux.HandleFunc("/api/v1/namespaces/"+namespace+"/secrets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			var s corev1.Secret
			if err := json.NewDecoder(r.Body).Decode(&s); err == nil {
				secretCreated = true
				storedSecretJSON, _ = json.Marshal(&s)
				_, _ = w.Write(storedSecretJSON)
			} else {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","status":"Failure","code":400}`))
			}
		}
	})
	// Handle webhook config GET and PUT
	mux.HandleFunc(
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/webhook1",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(webhookConfig)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				data, _ := json.Marshal(webhookConfig)
				_, _ = w.Write(data)
			}
		},
	)
	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := &config.Config{
		WebhookCertPath:       "", // Empty to trigger generation
		WebhookCertName:       "tls.crt",
		WebhookCertKey:        "tls.key",
		WebhookK8sServiceName: "test-service",
		K8sNamespacePath:      "/nonexistent/namespace",
	}
	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{
			Host: server.URL,
			// Force JSON so the fake server can decode the request body
			ContentConfig: rest.ContentConfig{ContentType: "application/json"},
		}
	}

	// Act
	certPath, keyPath, err := setupWebhookCertificatesInternal(
		context.Background(), cfg, []string{"webhook1"}, logger, getConfig,
	)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)
	// Verify the cert and key files exist
	_, statErr := os.Stat(certPath)
	assert.NoError(t, statErr, "cert file should exist")
	_, statErr = os.Stat(keyPath)
	assert.NoError(t, statErr, "key file should exist")
}

func TestSetupWebhookCertificatesInternal_GenerateCertsFailure(t *testing.T) {
	// Arrange: fake server that returns errors for secret operations
	namespace := "k8s-postgresql-operator"
	secretName := "test-service-webhook-cert"

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
				resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
				_, _ = w.Write([]byte(resp))
			}
		},
		"/api/v1/namespaces/" + namespace + "/secrets": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"error","code":500}`
			_, _ = w.Write([]byte(resp))
		},
	})
	defer server.Close()

	cfg := &config.Config{
		WebhookCertPath:       "",
		WebhookCertName:       "tls.crt",
		WebhookCertKey:        "tls.key",
		WebhookK8sServiceName: "test-service",
		K8sNamespacePath:      "/nonexistent/namespace",
	}
	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{
			Host:          server.URL,
			ContentConfig: rest.ContentConfig{ContentType: "application/json"},
		}
	}

	// Act
	_, _, err := setupWebhookCertificatesInternal(
		context.Background(), cfg, []string{"webhook1"}, logger, getConfig,
	)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate self-signed webhook certificates")
}

func TestSetupWebhookCertificatesInternal_WebhookUpdateError(t *testing.T) {
	// Arrange: cert generation succeeds but webhook update fails
	namespace := "k8s-postgresql-operator"
	secretName := "test-service-webhook-cert"

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
				resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
				_, _ = w.Write([]byte(resp))
			}
		},
		"/api/v1/namespaces/" + namespace + "/secrets": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost {
				secret := &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
					ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
				}
				data, _ := json.Marshal(secret)
				_, _ = w.Write(data)
			}
		},
		// Webhook config returns error
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/webhook1": func(
			w http.ResponseWriter, _ *http.Request,
		) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
			_, _ = w.Write([]byte(resp))
		},
	})
	defer server.Close()

	cfg := &config.Config{
		WebhookCertPath:       "",
		WebhookCertName:       "tls.crt",
		WebhookCertKey:        "tls.key",
		WebhookK8sServiceName: "test-service",
		K8sNamespacePath:      "/nonexistent/namespace",
	}
	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{Host: server.URL}
	}

	// Act — should succeed even though webhook update fails (it continues)
	certPath, keyPath, err := setupWebhookCertificatesInternal(
		context.Background(), cfg, []string{"webhook1"}, logger, getConfig,
	)

	// Assert — no error because webhook update failure is non-fatal
	require.NoError(t, err)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)
}

func TestSetupWebhookCertificatesInternal_MultipleWebhooks(t *testing.T) {
	// Arrange: cert generation succeeds, multiple webhooks to update
	namespace := "k8s-postgresql-operator"
	secretName := "test-service-webhook-cert"

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	makeWebhookConfig := func(name string) *admissionregistrationv1.ValidatingWebhookConfiguration {
		return &admissionregistrationv1.ValidatingWebhookConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admissionregistration.k8s.io/v1",
				Kind:       "ValidatingWebhookConfiguration",
			},
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:                    "postgresql-operator.vyrodovalexey.github.com",
					ClientConfig:            admissionregistrationv1.WebhookClientConfig{},
					SideEffects:             &sidecarPolicy,
					AdmissionReviewVersions: []string{"v1"},
				},
			},
		}
	}

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
				resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
				_, _ = w.Write([]byte(resp))
			}
		},
		"/api/v1/namespaces/" + namespace + "/secrets": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost {
				secret := &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
					ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
				}
				data, _ := json.Marshal(secret)
				_, _ = w.Write(data)
			}
		},
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/wh1": func(
			w http.ResponseWriter, r *http.Request,
		) {
			w.Header().Set("Content-Type", "application/json")
			whCfg := makeWebhookConfig("wh1")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			}
		},
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/wh2": func(
			w http.ResponseWriter, r *http.Request,
		) {
			w.Header().Set("Content-Type", "application/json")
			whCfg := makeWebhookConfig("wh2")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			}
		},
	})
	defer server.Close()

	cfg := &config.Config{
		WebhookCertPath:       "",
		WebhookCertName:       "tls.crt",
		WebhookCertKey:        "tls.key",
		WebhookK8sServiceName: "test-service",
		K8sNamespacePath:      "/nonexistent/namespace",
	}
	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{Host: server.URL}
	}

	// Act
	certPath, keyPath, err := setupWebhookCertificatesInternal(
		context.Background(), cfg, []string{"wh1", "wh2"}, logger, getConfig,
	)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)
}

func TestSetupWebhookCertificatesInternal_EmptyWebhookNames(t *testing.T) {
	// Arrange: cert generation succeeds, no webhooks to update
	namespace := "k8s-postgresql-operator"
	secretName := "test-service-webhook-cert"

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
				resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
				_, _ = w.Write([]byte(resp))
			}
		},
		"/api/v1/namespaces/" + namespace + "/secrets": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost {
				secret := &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
					ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
				}
				data, _ := json.Marshal(secret)
				_, _ = w.Write(data)
			}
		},
	})
	defer server.Close()

	cfg := &config.Config{
		WebhookCertPath:       "",
		WebhookCertName:       "tls.crt",
		WebhookCertKey:        "tls.key",
		WebhookK8sServiceName: "test-service",
		K8sNamespacePath:      "/nonexistent/namespace",
	}
	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{Host: server.URL}
	}

	// Act
	certPath, keyPath, err := setupWebhookCertificatesInternal(
		context.Background(), cfg, []string{}, logger, getConfig,
	)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)
}

func TestSetupWebhookCertificatesInternal_WithNamespaceFile(t *testing.T) {
	// Arrange: create a namespace file so GetCurrentNamespace returns a custom namespace
	tmpDir := t.TempDir()
	nsFile := filepath.Join(tmpDir, "namespace")
	require.NoError(t, os.WriteFile(nsFile, []byte("custom-ns"), 0o644))

	namespace := "custom-ns"
	secretName := "test-service-webhook-cert"

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/" + namespace + "/secrets/" + secretName: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
				resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
				_, _ = w.Write([]byte(resp))
			}
		},
		"/api/v1/namespaces/" + namespace + "/secrets": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPost {
				secret := &corev1.Secret{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
					ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
				}
				data, _ := json.Marshal(secret)
				_, _ = w.Write(data)
			}
		},
	})
	defer server.Close()

	cfg := &config.Config{
		WebhookCertPath:       "",
		WebhookCertName:       "tls.crt",
		WebhookCertKey:        "tls.key",
		WebhookK8sServiceName: "test-service",
		K8sNamespacePath:      nsFile,
	}
	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{Host: server.URL}
	}

	// Act
	certPath, keyPath, err := setupWebhookCertificatesInternal(
		context.Background(), cfg, []string{}, logger, getConfig,
	)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)
}

// --- setupWebhookCertificatesWithVaultPKIInternal tests ---

func TestSetupWebhookCertificatesWithVaultPKIInternal_Success(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()
	webhookName := "test-vault-webhook"

	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return &vault.PKICertificate{
				Certificate:  "CERT-DATA",
				PrivateKey:   "KEY-DATA",
				CAChain:      []string{"CA-CHAIN"},
				SerialNumber: "aa:bb:cc",
				Expiration:   9999999999,
			}, nil
		},
		getCACertFunc: func(_ context.Context, _ string) (string, error) {
			return "CA-BUNDLE-PEM", nil
		},
	}

	mgr := certpkg.NewVaultPKIManager(
		client, "pki", "role", "720h",
		24*time.Hour,
		"test-service", "test-ns", tmpDir,
		zap.NewNop().Sugar(),
	)

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	whCfg := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{Name: webhookName},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name:                    "postgresql-operator.vyrodovalexey.github.com",
				ClientConfig:            admissionregistrationv1.WebhookClientConfig{},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	whPath := "/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/" + webhookName
	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		whPath: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			}
		},
	})
	defer server.Close()

	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{Host: server.URL}
	}

	// Act
	certPath, keyPath, err := setupWebhookCertificatesWithVaultPKIInternal(
		context.Background(), mgr, []string{webhookName}, logger, getConfig,
	)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "tls.crt"), certPath)
	assert.Equal(t, filepath.Join(tmpDir, "tls.key"), keyPath)
}

func TestSetupWebhookCertificatesWithVaultPKIInternal_IssueCertError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()

	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return nil, fmt.Errorf("vault unavailable")
		},
	}

	mgr := certpkg.NewVaultPKIManager(
		client, "pki", "role", "720h",
		24*time.Hour,
		"test-service", "test-ns", tmpDir,
		zap.NewNop().Sugar(),
	)

	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		t.Fatal("getConfig should not be called when IssueCertificate fails")
		return nil
	}

	// Act
	_, _, err := setupWebhookCertificatesWithVaultPKIInternal(
		context.Background(), mgr, []string{"wh1"}, logger, getConfig,
	)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to issue Vault PKI certificate")
}

func TestSetupWebhookCertificatesWithVaultPKIInternal_GetCAError(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()

	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return &vault.PKICertificate{
				Certificate: "CERT",
				PrivateKey:  "KEY",
				CAChain:     []string{"CA"},
			}, nil
		},
		getCACertFunc: func(_ context.Context, _ string) (string, error) {
			return "", fmt.Errorf("CA retrieval failed")
		},
	}

	mgr := certpkg.NewVaultPKIManager(
		client, "pki", "role", "720h",
		24*time.Hour,
		"test-service", "test-ns", tmpDir,
		zap.NewNop().Sugar(),
	)

	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		t.Fatal("getConfig should not be called when GetCACertificatePEM fails")
		return nil
	}

	// Act
	_, _, err := setupWebhookCertificatesWithVaultPKIInternal(
		context.Background(), mgr, []string{"wh1"}, logger, getConfig,
	)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get Vault PKI CA certificate")
}

func TestSetupWebhookCertificatesWithVaultPKIInternal_WebhookUpdateError(t *testing.T) {
	// Arrange: cert issuance and CA retrieval succeed, but webhook update fails
	tmpDir := t.TempDir()

	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return &vault.PKICertificate{
				Certificate: "CERT",
				PrivateKey:  "KEY",
				CAChain:     []string{"CA"},
			}, nil
		},
		getCACertFunc: func(_ context.Context, _ string) (string, error) {
			return "CA-PEM", nil
		},
	}

	mgr := certpkg.NewVaultPKIManager(
		client, "pki", "role", "720h",
		24*time.Hour,
		"test-service", "test-ns", tmpDir,
		zap.NewNop().Sugar(),
	)

	// Webhook config endpoint returns 404
	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/wh1": func(
			w http.ResponseWriter, _ *http.Request,
		) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
			_, _ = w.Write([]byte(resp))
		},
	})
	defer server.Close()

	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{Host: server.URL}
	}

	// Act — should succeed even though webhook update fails (it continues)
	certPath, keyPath, err := setupWebhookCertificatesWithVaultPKIInternal(
		context.Background(), mgr, []string{"wh1"}, logger, getConfig,
	)

	// Assert — no error because webhook update failure is non-fatal
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "tls.crt"), certPath)
	assert.Equal(t, filepath.Join(tmpDir, "tls.key"), keyPath)
}

func TestSetupWebhookCertificatesWithVaultPKIInternal_MultipleWebhooks(t *testing.T) {
	// Arrange: one webhook succeeds, one fails
	tmpDir := t.TempDir()

	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return &vault.PKICertificate{
				Certificate: "CERT",
				PrivateKey:  "KEY",
				CAChain:     []string{"CA"},
			}, nil
		},
		getCACertFunc: func(_ context.Context, _ string) (string, error) {
			return "CA-PEM", nil
		},
	}

	mgr := certpkg.NewVaultPKIManager(
		client, "pki", "role", "720h",
		24*time.Hour,
		"test-service", "test-ns", tmpDir,
		zap.NewNop().Sugar(),
	)

	sidecarPolicy := admissionregistrationv1.SideEffectClassNone
	whCfg := &admissionregistrationv1.ValidatingWebhookConfiguration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
		},
		ObjectMeta: metav1.ObjectMeta{Name: "wh-ok"},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name:                    "postgresql-operator.vyrodovalexey.github.com",
				ClientConfig:            admissionregistrationv1.WebhookClientConfig{},
				SideEffects:             &sidecarPolicy,
				AdmissionReviewVersions: []string{"v1"},
			},
		},
	}

	server := newFakeAPIServer(t, map[string]http.HandlerFunc{
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/wh-ok": func(
			w http.ResponseWriter, r *http.Request,
		) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodGet {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			} else if r.Method == http.MethodPut {
				data, _ := json.Marshal(whCfg)
				_, _ = w.Write(data)
			}
		},
		"/apis/admissionregistration.k8s.io/v1/validatingwebhookconfigurations/wh-fail": func(
			w http.ResponseWriter, _ *http.Request,
		) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			resp := `{"kind":"Status","apiVersion":"v1","status":"Failure","message":"not found","code":404}`
			_, _ = w.Write([]byte(resp))
		},
	})
	defer server.Close()

	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{Host: server.URL}
	}

	// Act
	certPath, keyPath, err := setupWebhookCertificatesWithVaultPKIInternal(
		context.Background(), mgr, []string{"wh-ok", "wh-fail"}, logger, getConfig,
	)

	// Assert — succeeds despite one webhook failing
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "tls.crt"), certPath)
	assert.Equal(t, filepath.Join(tmpDir, "tls.key"), keyPath)
}

func TestSetupWebhookCertificatesWithVaultPKIInternal_EmptyWebhookNames(t *testing.T) {
	// Arrange
	tmpDir := t.TempDir()

	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return &vault.PKICertificate{
				Certificate: "CERT",
				PrivateKey:  "KEY",
				CAChain:     []string{"CA"},
			}, nil
		},
		getCACertFunc: func(_ context.Context, _ string) (string, error) {
			return "CA-PEM", nil
		},
	}

	mgr := certpkg.NewVaultPKIManager(
		client, "pki", "role", "720h",
		24*time.Hour,
		"test-service", "test-ns", tmpDir,
		zap.NewNop().Sugar(),
	)

	logger := zap.NewNop().Sugar()
	getConfig := func() *rest.Config {
		return &rest.Config{Host: "http://localhost:1"}
	}

	// Act
	certPath, keyPath, err := setupWebhookCertificatesWithVaultPKIInternal(
		context.Background(), mgr, []string{}, logger, getConfig,
	)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, "tls.crt"), certPath)
	assert.Equal(t, filepath.Join(tmpDir, "tls.key"), keyPath)
}
