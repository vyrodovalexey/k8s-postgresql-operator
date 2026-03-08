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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// generateTestCertAndKey generates a valid self-signed certificate and key PEM for testing.
func generateTestCertAndKey(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "test-service.test-namespace.svc",
			Organization: []string{"test-org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"test-service", "test-service.test-namespace.svc"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return certPEM, keyPEM
}

// writeTestCertFiles writes valid cert and key PEM files to the given directory and returns their paths.
func writeTestCertFiles(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	certPEM, keyPEM := generateTestCertAndKey(t)
	certPath = filepath.Join(dir, "tls.crt")
	keyPath = filepath.Join(dir, "tls.key")

	require.NoError(t, os.WriteFile(certPath, certPEM, 0644))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0644))

	return certPath, keyPath
}

// newFakeK8sServer creates a fake K8s API server for testing.
// It handles GET and POST/PUT for secrets in the given namespace.
func newFakeK8sServer(t *testing.T, namespace string, existingSecret *corev1.Secret, failCreate, failUpdate bool) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	secretPath := "/api/v1/namespaces/" + namespace + "/secrets/"

	// Handle GET secret
	mux.HandleFunc("GET "+secretPath, func(w http.ResponseWriter, r *http.Request) {
		secretName := r.URL.Path[len(secretPath):]
		if existingSecret != nil && existingSecret.Name == secretName {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(existingSecret)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"not found","reason":"NotFound","code":404}`))
	})

	// Handle POST (create) secret
	mux.HandleFunc("POST "+secretPath, func(w http.ResponseWriter, r *http.Request) {
		if failCreate {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"already exists","reason":"AlreadyExists","code":409}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		var secret corev1.Secret
		_ = json.NewDecoder(r.Body).Decode(&secret)
		data, _ := json.Marshal(&secret)
		_, _ = w.Write(data)
	})

	// Handle PUT (update) secret
	mux.HandleFunc("PUT "+secretPath, func(w http.ResponseWriter, r *http.Request) {
		if failUpdate {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"internal error","reason":"InternalError","code":500}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		var secret corev1.Secret
		_ = json.NewDecoder(r.Body).Decode(&secret)
		data, _ := json.Marshal(&secret)
		_, _ = w.Write(data)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// ============================================================
// Tests for LoadCert
// ============================================================

func TestLoadCert(t *testing.T) {
	tests := []struct {
		name        string
		setupFiles  func(t *testing.T, dir string) (certPath, keyPath string)
		expectError bool
		errContains string
	}{
		{
			name: "success with valid certificate",
			setupFiles: func(t *testing.T, dir string) (string, string) {
				return writeTestCertFiles(t, dir)
			},
			expectError: false,
		},
		{
			name: "error when cert file does not exist",
			setupFiles: func(t *testing.T, dir string) (string, string) {
				keyPath := filepath.Join(dir, "tls.key")
				require.NoError(t, os.WriteFile(keyPath, []byte("dummy"), 0644))
				return filepath.Join(dir, "nonexistent.crt"), keyPath
			},
			expectError: true,
			errContains: "failed to load certificate",
		},
		{
			name: "error when key file does not exist",
			setupFiles: func(t *testing.T, dir string) (string, string) {
				certPath := filepath.Join(dir, "tls.crt")
				require.NoError(t, os.WriteFile(certPath, []byte("dummy"), 0644))
				return certPath, filepath.Join(dir, "nonexistent.key")
			},
			expectError: true,
			errContains: "failed to load certificate",
		},
		{
			name: "error with invalid certificate content",
			setupFiles: func(t *testing.T, dir string) (string, string) {
				certPath := filepath.Join(dir, "tls.crt")
				keyPath := filepath.Join(dir, "tls.key")
				require.NoError(t, os.WriteFile(certPath, []byte("invalid-cert"), 0644))
				require.NoError(t, os.WriteFile(keyPath, []byte("invalid-key"), 0644))
				return certPath, keyPath
			},
			expectError: true,
			errContains: "failed to load certificate",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			certPath, keyPath := tc.setupFiles(t, tmpDir)

			cert, err := LoadCert(certPath, keyPath)

			if tc.expectError {
				assert.Error(t, err)
				assert.Nil(t, cert)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cert)
			}
		})
	}
}

// ============================================================
// Tests for writeCertFromSecret
// ============================================================

func TestWriteCertFromSecret(t *testing.T) {
	validCertPEM, validKeyPEM := generateTestCertAndKey(t)

	tests := []struct {
		name     string
		secret   *corev1.Secret
		setupDir func(t *testing.T) (certPath, keyPath string)
		expectOk bool
	}{
		{
			name: "success with valid secret data",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.crt": validCertPEM,
					"tls.key": validKeyPEM,
				},
			},
			setupDir: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				return filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
			},
			expectOk: true,
		},
		{
			name: "failure when tls.crt key is missing",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.key": validKeyPEM,
				},
			},
			setupDir: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				return filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
			},
			expectOk: false,
		},
		{
			name: "failure when tls.key key is missing",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.crt": validCertPEM,
				},
			},
			setupDir: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				return filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
			},
			expectOk: false,
		},
		{
			name: "failure when both keys are missing",
			secret: &corev1.Secret{
				Data: map[string][]byte{},
			},
			setupDir: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				return filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
			},
			expectOk: false,
		},
		{
			name: "failure when cert path is invalid",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.crt": validCertPEM,
					"tls.key": validKeyPEM,
				},
			},
			setupDir: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				return filepath.Join(dir, "nonexistent-dir", "tls.crt"), filepath.Join(dir, "tls.key")
			},
			expectOk: false,
		},
		{
			name: "failure when key path is invalid",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"tls.crt": validCertPEM,
					"tls.key": validKeyPEM,
				},
			},
			setupDir: func(t *testing.T) (string, string) {
				dir := t.TempDir()
				return filepath.Join(dir, "tls.crt"), filepath.Join(dir, "nonexistent-dir", "tls.key")
			},
			expectOk: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			certPath, keyPath := tc.setupDir(t)

			outCertPath, outKeyPath, ok := writeCertFromSecret(tc.secret, certPath, keyPath)

			if tc.expectOk {
				assert.True(t, ok)
				assert.Equal(t, certPath, outCertPath)
				assert.Equal(t, keyPath, outKeyPath)

				// Verify files were written correctly
				certData, err := os.ReadFile(outCertPath)
				require.NoError(t, err)
				assert.Equal(t, tc.secret.Data["tls.crt"], certData)

				keyData, err := os.ReadFile(outKeyPath)
				require.NoError(t, err)
				assert.Equal(t, tc.secret.Data["tls.key"], keyData)
			} else {
				assert.False(t, ok)
				assert.Empty(t, outCertPath)
				assert.Empty(t, outKeyPath)
			}
		})
	}
}

func TestWriteCertFromSecret_WriteFailure(t *testing.T) {
	// This test uses RLIMIT_FSIZE to make file writes fail after os.Create succeeds.
	// os.Create creates an empty file (0 bytes) which is allowed, but Write fails
	// because the file size limit is 0.
	validCertPEM, validKeyPEM := generateTestCertAndKey(t)

	// Ignore SIGXFSZ so we get an error instead of being killed
	signal.Ignore(syscall.SIGXFSZ)
	defer signal.Reset(syscall.SIGXFSZ)

	// Save and set RLIMIT_FSIZE to 0
	var oldRlim syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_FSIZE, &oldRlim)
	require.NoError(t, err)

	newRlim := syscall.Rlimit{Cur: 0, Max: oldRlim.Max}
	err = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &newRlim)
	require.NoError(t, err)
	defer func() {
		_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &oldRlim)
	}()

	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")

	secret := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": validCertPEM,
			"tls.key": validKeyPEM,
		},
	}

	// Act: os.Create will succeed but Write will fail due to RLIMIT_FSIZE=0
	outCertPath, outKeyPath, ok := writeCertFromSecret(secret, certPath, keyPath)

	// Restore limit before assertions
	_ = syscall.Setrlimit(syscall.RLIMIT_FSIZE, &oldRlim)

	// Assert
	assert.False(t, ok)
	assert.Empty(t, outCertPath)
	assert.Empty(t, outKeyPath)
}

// ============================================================
// Tests for GenerateSelfSignedCertAndStoreInSecret
// ============================================================

func TestGenerateSelfSignedCertAndStoreInSecret_InvalidCertDir(t *testing.T) {
	// Use a path that can't be created
	invalidDir := "/dev/null/certs"
	config := &rest.Config{Host: "https://test"}

	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", "test-namespace", "test-secret", invalidDir, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cert directory")
}

func TestGenerateSelfSignedCertAndStoreInSecret_SecretExistsWithValidData(t *testing.T) {
	// Arrange: generate valid cert/key data
	certPEM, keyPEM := generateTestCertAndKey(t)

	namespace := "test-namespace"
	secretName := "test-secret"

	existingSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
		},
	}

	server := newFakeK8sServer(t, namespace, existingSecret, false, false)

	config := &rest.Config{Host: server.URL}
	tmpDir := t.TempDir()

	// Act
	secretNameOut, certPath, keyPath, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, tmpDir, config)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, secretName, secretNameOut)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)

	// Verify files exist and contain the expected data
	certData, err := os.ReadFile(certPath)
	require.NoError(t, err)
	assert.Equal(t, certPEM, certData)

	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, keyPEM, keyData)
}

func TestGenerateSelfSignedCertAndStoreInSecret_SecretExistsWithMissingKeys(t *testing.T) {
	// Arrange: secret exists but without tls.crt/tls.key
	namespace := "test-namespace"
	secretName := "test-secret"

	existingSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"other-key": []byte("some-data"),
		},
	}

	// Secret exists but has no tls data, so it should generate new cert and create secret
	server := newFakeK8sServer(t, namespace, existingSecret, false, false)

	config := &rest.Config{Host: server.URL}
	tmpDir := t.TempDir()

	// Act
	secretNameOut, certPath, keyPath, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, tmpDir, config)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, secretName, secretNameOut)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)

	// Verify files exist
	_, err = os.Stat(certPath)
	assert.NoError(t, err)
	_, err = os.Stat(keyPath)
	assert.NoError(t, err)
}

func TestGenerateSelfSignedCertAndStoreInSecret_SecretDoesNotExist_CreateSuccess(t *testing.T) {
	// Arrange: no existing secret, create succeeds
	namespace := "test-namespace"
	secretName := "test-secret"

	server := newFakeK8sServer(t, namespace, nil, false, false)

	config := &rest.Config{Host: server.URL}
	tmpDir := t.TempDir()

	// Act
	secretNameOut, certPath, keyPath, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, tmpDir, config)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, secretName, secretNameOut)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)

	// Verify cert files are valid
	_, loadErr := LoadCert(certPath, keyPath)
	assert.NoError(t, loadErr)
}

func TestGenerateSelfSignedCertAndStoreInSecret_CreateFails_UpdateSucceeds(t *testing.T) {
	// Arrange: create fails (conflict), update succeeds
	namespace := "test-namespace"
	secretName := "test-secret"

	server := newFakeK8sServer(t, namespace, nil, true, false)

	config := &rest.Config{Host: server.URL}
	tmpDir := t.TempDir()

	// Act
	secretNameOut, certPath, keyPath, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, tmpDir, config)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, secretName, secretNameOut)
	assert.NotEmpty(t, certPath)
	assert.NotEmpty(t, keyPath)
}

func TestGenerateSelfSignedCertAndStoreInSecret_CreateAndUpdateBothFail(t *testing.T) {
	// Arrange: both create and update fail
	namespace := "test-namespace"
	secretName := "test-secret"

	server := newFakeK8sServer(t, namespace, nil, true, true)

	config := &rest.Config{Host: server.URL}
	tmpDir := t.TempDir()

	// Act
	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, tmpDir, config)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create/update secret")
}

func TestGenerateSelfSignedCertAndStoreInSecret_NewForConfigFailure(t *testing.T) {
	// Arrange: rest.Config with an invalid ExecProvider causes kubernetes.NewForConfig to fail
	tmpDir := t.TempDir()

	config := &rest.Config{
		Host: "https://test",
		ExecProvider: &clientcmdapi.ExecConfig{
			Command: "",
			// Empty APIVersion causes NewForConfig to fail with "exec plugin: invalid apiVersion"
		},
	}

	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", "test-namespace", "test-secret", tmpDir, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create Kubernetes client")
}

func TestGenerateSelfSignedCertAndStoreInSecret_CertFileWriteFailure(t *testing.T) {
	// Arrange: secret doesn't exist, create succeeds, but cert file write fails
	namespace := "test-namespace"
	secretName := "test-secret"

	server := newFakeK8sServer(t, namespace, nil, false, false)

	config := &rest.Config{Host: server.URL}

	// Create a tmpDir and then make the cert file path point to a read-only location
	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "certs")
	require.NoError(t, os.MkdirAll(certDir, 0750))

	// After the function creates the directory, make it read-only so file creation fails
	// We need to make the certDir read-only AFTER MkdirAll succeeds but BEFORE file creation
	// Since we can't intercept, let's create a file where the cert file should be and make it a directory
	certFilePath := filepath.Join(certDir, "tls.crt")
	require.NoError(t, os.MkdirAll(certFilePath, 0750)) // Make tls.crt a directory so os.Create fails

	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, certDir, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cert file")
}

func TestGenerateSelfSignedCertAndStoreInSecret_KeyFileWriteFailure(t *testing.T) {
	// Arrange: secret doesn't exist, create succeeds, cert file writes ok, but key file write fails
	namespace := "test-namespace"
	secretName := "test-secret"

	server := newFakeK8sServer(t, namespace, nil, false, false)

	config := &rest.Config{Host: server.URL}

	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "certs")
	require.NoError(t, os.MkdirAll(certDir, 0750))

	// Make tls.key a directory so os.Create fails
	keyFilePath := filepath.Join(certDir, "tls.key")
	require.NoError(t, os.MkdirAll(keyFilePath, 0750))

	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, certDir, config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create key file")
}

func TestGenerateSelfSignedCertAndStoreInSecret_ExistingSecretWithInvalidCertPath(t *testing.T) {
	// Arrange: secret exists with valid data but cert file path is invalid
	// This tests the writeCertFromSecret failure path within GenerateSelfSignedCertAndStoreInSecret
	certPEM, keyPEM := generateTestCertAndKey(t)

	namespace := "test-namespace"
	secretName := "test-secret"

	existingSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
		},
	}

	server := newFakeK8sServer(t, namespace, existingSecret, false, false)

	config := &rest.Config{Host: server.URL}

	// Use a certDir where the cert file path will be a directory (causing writeCertFromSecret to fail)
	tmpDir := t.TempDir()
	certDir := filepath.Join(tmpDir, "certs")
	require.NoError(t, os.MkdirAll(certDir, 0750))

	// Make tls.crt a directory so writeCertFromSecret fails, then it falls through to generate new cert
	certFilePath := filepath.Join(certDir, "tls.crt")
	require.NoError(t, os.MkdirAll(certFilePath, 0750))

	// The function should fall through to generate a new cert, but then fail at cert file creation
	_, _, _, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, certDir, config)
	// It will fail at the cert file creation step since tls.crt is a directory
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cert file")
}

func TestGenerateSelfSignedCertAndStoreInSecret_VerifyCertContent(t *testing.T) {
	// Arrange: full success path - verify the generated certificate content
	namespace := "test-namespace"
	secretName := "test-secret"

	server := newFakeK8sServer(t, namespace, nil, false, false)

	config := &rest.Config{Host: server.URL}
	tmpDir := t.TempDir()

	// Act
	secretNameOut, certPath, keyPath, err := GenerateSelfSignedCertAndStoreInSecret(
		"test-service", namespace, secretName, tmpDir, config)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, secretName, secretNameOut)

	// Verify the certificate is valid and has expected properties
	certData, err := os.ReadFile(certPath)
	require.NoError(t, err)

	block, _ := pem.Decode(certData)
	require.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	assert.Equal(t, "test-service.test-namespace.svc", parsedCert.Subject.CommonName)
	assert.Contains(t, parsedCert.DNSNames, "test-service")
	assert.Contains(t, parsedCert.DNSNames, "test-service.test-namespace")
	assert.Contains(t, parsedCert.DNSNames, "test-service.test-namespace.svc")
	assert.Contains(t, parsedCert.DNSNames, "test-service.test-namespace.svc.cluster.local")

	// Verify the key is valid
	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	keyBlock, _ := pem.Decode(keyData)
	require.NotNil(t, keyBlock)
	assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)

	_, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	assert.NoError(t, err)

	// Verify the cert and key can be loaded together
	cert, err := LoadCert(certPath, keyPath)
	assert.NoError(t, err)
	assert.NotNil(t, cert)
}
