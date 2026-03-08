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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// ---------- mock VaultPKIClient ----------

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
		return m.issueCertFunc(
			ctx, mountPath, roleName, commonName,
			altNames, ipSANs, ttl,
		)
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

// ---------- test cert generator ----------

func generateTestCert(
	t *testing.T, notAfter time.Time,
) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serial, err := rand.Int(
		rand.Reader,
		new(big.Int).Lsh(big.NewInt(1), 128),
	)
	require.NoError(t, err)

	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "test.default.svc",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	der, err := x509.CreateCertificate(
		rand.Reader, &tmpl, &tmpl, &key.PublicKey, key,
	)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(
		&pem.Block{Type: "CERTIFICATE", Bytes: der},
	)
	keyPEM = pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)
	return certPEM, keyPEM
}

// ---------- helper to build a default manager ----------

func newTestManager(
	t *testing.T,
	client VaultPKIClient,
	certDir string,
) *VaultPKIManager {
	t.Helper()
	log := zap.NewNop().Sugar()
	return NewVaultPKIManager(
		client,
		"pki", "webhook-cert", "720h",
		24*time.Hour,
		"my-svc", "default", certDir,
		log,
	)
}

// ============================================================
// TestNewVaultPKIManager
// ============================================================

func TestNewVaultPKIManager(t *testing.T) {
	// Arrange
	client := &mockVaultPKIClient{}
	log := zap.NewNop().Sugar()
	certDir := "/tmp/certs"

	// Act
	mgr := NewVaultPKIManager(
		client,
		"pki-mount", "my-role", "48h",
		12*time.Hour,
		"webhook-svc", "prod", certDir,
		log,
	)

	// Assert
	require.NotNil(t, mgr)
	assert.Equal(t, "pki-mount", mgr.mountPath)
	assert.Equal(t, "my-role", mgr.roleName)
	assert.Equal(t, "48h", mgr.ttl)
	assert.Equal(t, 12*time.Hour, mgr.renewalBuffer)
	assert.Equal(t, "webhook-svc", mgr.serviceName)
	assert.Equal(t, "prod", mgr.namespace)
	assert.Equal(t, certDir, mgr.certDir)
	assert.Equal(t, filepath.Join(certDir, "tls.crt"), mgr.certPath)
	assert.Equal(t, filepath.Join(certDir, "tls.key"), mgr.keyPath)
	assert.Equal(t, filepath.Join(certDir, "ca.crt"), mgr.caPath)
	assert.NotNil(t, mgr.stopCh)
	assert.NotNil(t, mgr.log)
}

// ============================================================
// TestVaultPKIManager_CertPath / KeyPath
// ============================================================

func TestVaultPKIManager_CertPath(t *testing.T) {
	mgr := newTestManager(t, &mockVaultPKIClient{}, "/certs")
	assert.Equal(t, "/certs/tls.crt", mgr.CertPath())
}

func TestVaultPKIManager_KeyPath(t *testing.T) {
	mgr := newTestManager(t, &mockVaultPKIClient{}, "/certs")
	assert.Equal(t, "/certs/tls.key", mgr.KeyPath())
}

// ============================================================
// TestBuildAltNames
// ============================================================

func TestBuildAltNames(t *testing.T) {
	tests := []struct {
		name      string
		svcName   string
		namespace string
		want      []string
	}{
		{
			name:      "standard names",
			svcName:   "my-svc",
			namespace: "default",
			want: []string{
				"my-svc",
				"my-svc.default",
				"my-svc.default.svc",
				"my-svc.default.svc.cluster.local",
			},
		},
		{
			name:      "custom namespace",
			svcName:   "webhook",
			namespace: "prod",
			want: []string{
				"webhook",
				"webhook.prod",
				"webhook.prod.svc",
				"webhook.prod.svc.cluster.local",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAltNames(tt.svcName, tt.namespace)
			assert.Equal(t, tt.want, got)
			assert.Len(t, got, 4)
		})
	}
}

// ============================================================
// TestWritePKICertFiles
// ============================================================

func TestWritePKICertFiles_Success(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	caPath := filepath.Join(dir, "ca.crt")

	pkiCert := &vault.PKICertificate{
		Certificate: "-----BEGIN CERTIFICATE-----\nMIIB...\n",
		PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIE...\n",
		CAChain:     []string{"-----BEGIN CERTIFICATE-----\nCA1"},
	}

	// Act
	err := writePKICertFiles(pkiCert, certPath, keyPath, caPath)

	// Assert
	require.NoError(t, err)

	certData, err := os.ReadFile(certPath)
	require.NoError(t, err)
	assert.Equal(t, pkiCert.Certificate, string(certData))

	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	assert.Equal(t, pkiCert.PrivateKey, string(keyData))

	caData, err := os.ReadFile(caPath)
	require.NoError(t, err)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nCA1", string(caData))
}

func TestWritePKICertFiles_CertWriteError(t *testing.T) {
	// Arrange — certPath points to a directory
	dir := t.TempDir()
	certPath := filepath.Join(dir, "bad")
	require.NoError(t, os.MkdirAll(certPath, 0o750))
	keyPath := filepath.Join(dir, "tls.key")
	caPath := filepath.Join(dir, "ca.crt")

	pkiCert := &vault.PKICertificate{
		Certificate: "cert-data",
		PrivateKey:  "key-data",
	}

	// Act
	err := writePKICertFiles(pkiCert, certPath, keyPath, caPath)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write certificate")
}

func TestWritePKICertFiles_KeyWriteError(t *testing.T) {
	// Arrange — keyPath points to a directory
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "bad")
	require.NoError(t, os.MkdirAll(keyPath, 0o750))
	caPath := filepath.Join(dir, "ca.crt")

	pkiCert := &vault.PKICertificate{
		Certificate: "cert-data",
		PrivateKey:  "key-data",
	}

	// Act
	err := writePKICertFiles(pkiCert, certPath, keyPath, caPath)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write private key")
}

func TestWritePKICertFiles_CAWriteError(t *testing.T) {
	// Arrange — caPath points to a directory
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	caPath := filepath.Join(dir, "bad")
	require.NoError(t, os.MkdirAll(caPath, 0o750))

	pkiCert := &vault.PKICertificate{
		Certificate: "cert-data",
		PrivateKey:  "key-data",
		CAChain:     []string{"ca-data"},
	}

	// Act
	err := writePKICertFiles(pkiCert, certPath, keyPath, caPath)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write CA chain")
}

// ============================================================
// TestVaultPKIManager_IssueCertificateAndWriteToDisk
// ============================================================

func TestVaultPKIManager_IssueCertAndWriteToDisk_Success(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return &vault.PKICertificate{
				Certificate:  "CERT",
				PrivateKey:   "KEY",
				CAChain:      []string{"CA"},
				SerialNumber: "aa:bb",
				Expiration:   9999999999,
			}, nil
		},
	}
	mgr := newTestManager(t, client, dir)

	// Act
	err := mgr.IssueCertificateAndWriteToDisk(context.Background())

	// Assert
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "tls.crt"))
	require.NoError(t, err)
	assert.Equal(t, "CERT", string(data))

	data, err = os.ReadFile(filepath.Join(dir, "tls.key"))
	require.NoError(t, err)
	assert.Equal(t, "KEY", string(data))

	data, err = os.ReadFile(filepath.Join(dir, "ca.crt"))
	require.NoError(t, err)
	assert.Equal(t, "CA", string(data))
}

func TestVaultPKIManager_IssueCertAndWriteToDisk_IssueErr(
	t *testing.T,
) {
	// Arrange
	dir := t.TempDir()
	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return nil, fmt.Errorf("vault unavailable")
		},
	}
	mgr := newTestManager(t, client, dir)

	// Act
	err := mgr.IssueCertificateAndWriteToDisk(context.Background())

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vault unavailable")
}

func TestVaultPKIManager_IssueCertAndWriteToDisk_BadCertDir(
	t *testing.T,
) {
	// Arrange — certDir is /dev/null/x which cannot be created
	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			return &vault.PKICertificate{
				Certificate: "C",
				PrivateKey:  "K",
			}, nil
		},
	}
	mgr := newTestManager(t, client, "/dev/null/impossible")

	// Act
	err := mgr.IssueCertificateAndWriteToDisk(context.Background())

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create cert directory")
}

// ============================================================
// TestVaultPKIManager_GetCACertificatePEM
// ============================================================

func TestVaultPKIManager_GetCACertificatePEM_Success(t *testing.T) {
	// Arrange
	client := &mockVaultPKIClient{
		getCACertFunc: func(
			_ context.Context, _ string,
		) (string, error) {
			return "-----BEGIN CERTIFICATE-----\nCA\n-----END", nil
		},
	}
	mgr := newTestManager(t, client, t.TempDir())

	// Act
	pem, err := mgr.GetCACertificatePEM(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Equal(
		t,
		[]byte("-----BEGIN CERTIFICATE-----\nCA\n-----END"),
		pem,
	)
}

func TestVaultPKIManager_GetCACertificatePEM_Error(t *testing.T) {
	// Arrange
	client := &mockVaultPKIClient{
		getCACertFunc: func(
			_ context.Context, _ string,
		) (string, error) {
			return "", fmt.Errorf("pki error")
		},
	}
	mgr := newTestManager(t, client, t.TempDir())

	// Act
	pem, err := mgr.GetCACertificatePEM(context.Background())

	// Assert
	require.Error(t, err)
	assert.Nil(t, pem)
	assert.Contains(t, err.Error(), "pki error")
}

// ============================================================
// TestVaultPKIManager_CalculateRenewalTime
// ============================================================

func TestVaultPKIManager_CalculateRenewalTime_Success(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	notAfter := time.Now().Add(48 * time.Hour)
	certPEM, _ := generateTestCert(t, notAfter)
	certPath := filepath.Join(dir, "tls.crt")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o644))

	mgr := newTestManager(t, &mockVaultPKIClient{}, dir)

	// Act
	renewAt, err := mgr.calculateRenewalTime()

	// Assert
	require.NoError(t, err)
	expected := notAfter.Add(-24 * time.Hour)
	diff := renewAt.Sub(expected)
	assert.InDelta(t, 0, diff.Seconds(), 2)
}

func TestVaultPKIManager_CalculateRenewalTime_NoCertFile(
	t *testing.T,
) {
	// Arrange — no cert file on disk
	dir := t.TempDir()
	mgr := newTestManager(t, &mockVaultPKIClient{}, dir)

	// Act
	_, err := mgr.calculateRenewalTime()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read certificate file")
}

func TestVaultPKIManager_CalculateRenewalTime_InvalidPEM(
	t *testing.T,
) {
	// Arrange — write garbage PEM
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	require.NoError(
		t,
		os.WriteFile(certPath, []byte("not-pem"), 0o644),
	)
	mgr := newTestManager(t, &mockVaultPKIClient{}, dir)

	// Act
	_, err := mgr.calculateRenewalTime()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode PEM block")
}

func TestVaultPKIManager_CalculateRenewalTime_InvalidCert(
	t *testing.T,
) {
	// Arrange — valid PEM wrapper but garbage DER inside
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	badPEM := pem.EncodeToMemory(
		&pem.Block{Type: "CERTIFICATE", Bytes: []byte("bad")},
	)
	require.NoError(t, os.WriteFile(certPath, badPEM, 0o644))
	mgr := newTestManager(t, &mockVaultPKIClient{}, dir)

	// Act
	_, err := mgr.calculateRenewalTime()

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse certificate")
}

// ============================================================
// TestVaultPKIManager_Stop
// ============================================================

func TestVaultPKIManager_Stop(t *testing.T) {
	mgr := newTestManager(t, &mockVaultPKIClient{}, t.TempDir())

	// Act
	mgr.Stop()

	// Assert — channel should be closed (receive returns immediately)
	select {
	case <-mgr.stopCh:
		// ok — channel closed
	default:
		t.Fatal("stopCh was not closed")
	}
}

// ============================================================
// TestVaultPKIManager_WaitOrStop
// ============================================================

func TestVaultPKIManager_WaitOrStop_TimerExpires(t *testing.T) {
	mgr := newTestManager(t, &mockVaultPKIClient{}, t.TempDir())
	ctx := context.Background()

	// Act — very short timer
	stopped := mgr.waitOrStop(ctx, time.Millisecond)

	// Assert
	assert.False(t, stopped)
}

func TestVaultPKIManager_WaitOrStop_StopCalled(t *testing.T) {
	mgr := newTestManager(t, &mockVaultPKIClient{}, t.TempDir())
	ctx := context.Background()

	// Close stop channel before calling waitOrStop
	close(mgr.stopCh)

	// Act
	stopped := mgr.waitOrStop(ctx, time.Hour)

	// Assert
	assert.True(t, stopped)
}

func TestVaultPKIManager_WaitOrStop_ContextCanceled(t *testing.T) {
	mgr := newTestManager(t, &mockVaultPKIClient{}, t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Act
	stopped := mgr.waitOrStop(ctx, time.Hour)

	// Assert
	assert.True(t, stopped)
}

// ============================================================
// TestVaultPKIManager_RenewalLoop_StopsOnCancel
// ============================================================

func TestVaultPKIManager_RenewalLoop_StopsOnCancel(t *testing.T) {
	// Arrange — write a valid cert so calculateRenewalTime works
	dir := t.TempDir()
	notAfter := time.Now().Add(48 * time.Hour)
	certPEM, _ := generateTestCert(t, notAfter)
	certPath := filepath.Join(dir, "tls.crt")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o644))

	client := &mockVaultPKIClient{}
	mgr := newTestManager(t, client, dir)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		mgr.renewalLoop(ctx)
		close(done)
	}()

	// Act — cancel context to stop the loop
	cancel()

	// Assert — loop should exit promptly
	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("renewalLoop did not exit after context cancel")
	}
}

func TestVaultPKIManager_RenewalLoop_ErrorPath(t *testing.T) {
	// Arrange — no cert file on disk → calculateRenewalTime fails
	// → loop retries in 1m → we cancel before that
	dir := t.TempDir()
	mgr := newTestManager(t, &mockVaultPKIClient{}, dir)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		mgr.renewalLoop(ctx)
		close(done)
	}()

	// Give the loop time to hit the error path
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("renewalLoop did not exit after context cancel")
	}
}

func TestVaultPKIManager_RenewalLoop_RenewsExpiredCert(
	t *testing.T,
) {
	// Arrange — generate cert first, then compute notAfter
	// relative to current time to avoid timing issues from
	// key generation overhead.
	dir := t.TempDir()

	// Generate cert with a far-future expiry first
	farFuture := time.Now().Add(10 * time.Hour)
	certPEM, _ := generateTestCert(t, farFuture)

	// Now create the actual cert with a notAfter that is
	// 3 seconds from NOW (after key generation is done).
	notAfter := time.Now().Add(3 * time.Second)
	certPEM, _ = generateTestCert(t, notAfter)
	certPath := filepath.Join(dir, "tls.crt")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o644))

	renewed := make(chan struct{}, 1)
	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			select {
			case renewed <- struct{}{}:
			default:
			}
			return &vault.PKICertificate{
				Certificate: string(certPEM),
				PrivateKey:  "KEY",
				CAChain:     []string{"CA"},
			}, nil
		},
	}

	log := zap.NewNop().Sugar()
	mgr := NewVaultPKIManager(
		client,
		"pki", "webhook-cert", "720h",
		time.Second, // buffer → renewAt ≈ now+2s
		"my-svc", "default", dir,
		log,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.renewalLoop(ctx)
		close(done)
	}()

	// Wait for renewal to happen (should be within ~2s)
	select {
	case <-renewed:
		// ok — renewal was triggered
	case <-time.After(10 * time.Second):
		t.Fatal("renewal was not triggered")
	}

	cancel()
	<-done
}

func TestVaultPKIManager_RenewalLoop_RenewFails(t *testing.T) {
	// Arrange — generate cert, then set notAfter 3s from now.
	dir := t.TempDir()
	notAfter := time.Now().Add(3 * time.Second)
	certPEM, _ := generateTestCert(t, notAfter)
	certPath := filepath.Join(dir, "tls.crt")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o644))

	called := make(chan struct{}, 1)
	client := &mockVaultPKIClient{
		issueCertFunc: func(
			_ context.Context,
			_, _, _ string,
			_ []string, _ []string, _ string,
		) (*vault.PKICertificate, error) {
			select {
			case called <- struct{}{}:
			default:
			}
			return nil, fmt.Errorf("vault down")
		},
	}

	log := zap.NewNop().Sugar()
	mgr := NewVaultPKIManager(
		client,
		"pki", "webhook-cert", "720h",
		time.Second, // buffer → renewAt ≈ now+2s
		"my-svc", "default", dir,
		log,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		mgr.renewalLoop(ctx)
		close(done)
	}()

	select {
	case <-called:
		// ok — renewal was attempted
	case <-time.After(10 * time.Second):
		t.Fatal("renewal was not attempted")
	}

	cancel()
	<-done
}

func TestVaultPKIManager_StartRenewal(t *testing.T) {
	// Arrange — just verify StartRenewal launches a goroutine
	dir := t.TempDir()
	mgr := newTestManager(t, &mockVaultPKIClient{}, dir)

	ctx, cancel := context.WithCancel(context.Background())

	// Act
	mgr.StartRenewal(ctx)

	// Give the goroutine time to start
	time.Sleep(20 * time.Millisecond)

	// Cancel to stop the renewal loop
	cancel()

	// Give it time to exit
	time.Sleep(50 * time.Millisecond)
}
