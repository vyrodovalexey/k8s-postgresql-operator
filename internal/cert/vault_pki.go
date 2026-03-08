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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/metrics"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/telemetry"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// VaultPKIClient is the interface for Vault PKI operations (for testability)
type VaultPKIClient interface {
	IssueCertificate(
		ctx context.Context, mountPath, roleName, commonName string,
		altNames []string, ipSANs []string, ttl string,
	) (*vault.PKICertificate, error)
	GetCACertificate(
		ctx context.Context, mountPath string,
	) (string, error)
}

// VaultPKIManager manages webhook TLS certificates issued by Vault PKI
type VaultPKIManager struct {
	vaultClient   VaultPKIClient
	mountPath     string
	roleName      string
	ttl           string
	renewalBuffer time.Duration
	serviceName   string
	namespace     string
	certDir       string
	certPath      string
	keyPath       string
	caPath        string
	log           *zap.SugaredLogger
	stopCh        chan struct{}
	mu            sync.RWMutex
}

// NewVaultPKIManager creates a new VaultPKIManager
func NewVaultPKIManager(
	vaultClient VaultPKIClient,
	mountPath, roleName, ttl string,
	renewalBuffer time.Duration,
	serviceName, namespace, certDir string,
	log *zap.SugaredLogger,
) *VaultPKIManager {
	return &VaultPKIManager{
		vaultClient:   vaultClient,
		mountPath:     mountPath,
		roleName:      roleName,
		ttl:           ttl,
		renewalBuffer: renewalBuffer,
		serviceName:   serviceName,
		namespace:     namespace,
		certDir:       certDir,
		certPath:      filepath.Join(certDir, "tls.crt"),
		keyPath:       filepath.Join(certDir, "tls.key"),
		caPath:        filepath.Join(certDir, "ca.crt"),
		log:           log,
		stopCh:        make(chan struct{}),
	}
}

// IssueCertificateAndWriteToDisk issues a cert from Vault PKI
// and writes it to disk
func (m *VaultPKIManager) IssueCertificateAndWriteToDisk(
	ctx context.Context,
) error {
	ctx, span := otel.Tracer("cert").Start(ctx,
		"cert.IssueCertificateAndWriteToDisk",
		trace.WithAttributes(
			attribute.String(telemetry.AttrOperation,
				"issue_certificate_and_write_to_disk"),
			attribute.String("cert.service_name", m.serviceName),
		))
	defer span.End()

	m.mu.Lock()
	defer m.mu.Unlock()

	commonName := fmt.Sprintf(
		"%s.%s.svc", m.serviceName, m.namespace,
	)
	altNames := buildAltNames(m.serviceName, m.namespace)
	ipSANs := []string{"127.0.0.1"}

	pkiCert, err := m.vaultClient.IssueCertificate(
		ctx, m.mountPath, m.roleName, commonName,
		altNames, ipSANs, m.ttl,
	)
	if err != nil {
		issueErr := fmt.Errorf(
			"failed to issue certificate from Vault PKI: %w", err,
		)
		telemetry.RecordError(span, issueErr)
		return issueErr
	}

	if err := os.MkdirAll(m.certDir, 0o750); err != nil {
		mkdirErr := fmt.Errorf(
			"failed to create cert directory %s: %w",
			m.certDir, err,
		)
		telemetry.RecordError(span, mkdirErr)
		return mkdirErr
	}

	if err := writePKICertFiles(
		pkiCert, m.certPath, m.keyPath, m.caPath,
	); err != nil {
		telemetry.RecordError(span, err)
		return err
	}

	// Update certificate expiry metric
	block, _ := pem.Decode([]byte(pkiCert.Certificate))
	if block != nil {
		if parsedCert, parseErr := x509.ParseCertificate(
			block.Bytes,
		); parseErr == nil {
			secondsUntilExpiry := time.Until(
				parsedCert.NotAfter,
			).Seconds()
			metrics.SetCertificateExpiry(
				"vault_pki", secondsUntilExpiry,
			)
		}
	}

	m.log.Infow("Vault PKI certificate issued and written to disk",
		"serial-number", pkiCert.SerialNumber,
		"expiration", pkiCert.Expiration,
		"cert-path", m.certPath,
		"key-path", m.keyPath,
		"ca-path", m.caPath,
	)

	return nil
}

// buildAltNames constructs the list of DNS SANs for the certificate
func buildAltNames(serviceName, namespace string) []string {
	return []string{
		serviceName,
		fmt.Sprintf("%s.%s", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		fmt.Sprintf(
			"%s.%s.svc.cluster.local", serviceName, namespace,
		),
	}
}

// writePKICertFiles writes certificate, key, and CA chain to disk.
// The key is written BEFORE the cert to avoid a race condition with
// controller-runtime's certwatcher: it watches the cert file and
// reloads the cert+key pair on change. If the cert is written first,
// the watcher may read the new cert with the old key, causing a
// "private key does not match public key" error.
func writePKICertFiles(
	pkiCert *vault.PKICertificate,
	certPath, keyPath, caPath string,
) error {
	// 1. Write private key first (certwatcher does NOT watch this)
	if err := os.WriteFile(
		keyPath, []byte(pkiCert.PrivateKey), 0o600,
	); err != nil {
		return fmt.Errorf(
			"failed to write private key to %s: %w", keyPath, err,
		)
	}

	// 2. Write CA chain
	caChainPEM := strings.Join(pkiCert.CAChain, "\n")
	//nolint:gosec // microservices approach - CA cert needs to be readable
	if err := os.WriteFile(
		caPath, []byte(caChainPEM), 0o644,
	); err != nil {
		return fmt.Errorf(
			"failed to write CA chain to %s: %w", caPath, err,
		)
	}

	// 3. Write certificate LAST (certwatcher triggers reload on this)
	//nolint:gosec // microservices approach - cert files need to be readable
	if err := os.WriteFile(
		certPath, []byte(pkiCert.Certificate), 0o644,
	); err != nil {
		return fmt.Errorf(
			"failed to write certificate to %s: %w", certPath, err,
		)
	}

	return nil
}

// GetCACertificatePEM returns the CA certificate PEM for webhook
// configuration
func (m *VaultPKIManager) GetCACertificatePEM(
	ctx context.Context,
) ([]byte, error) {
	ctx, span := otel.Tracer("cert").Start(ctx,
		"cert.GetCACertificatePEM",
		trace.WithAttributes(
			attribute.String(telemetry.AttrOperation,
				"get_ca_certificate_pem"),
		))
	defer span.End()

	caCert, err := m.vaultClient.GetCACertificate(ctx, m.mountPath)
	if err != nil {
		telemetry.RecordError(span, err)
		return nil, fmt.Errorf(
			"failed to get CA certificate from Vault PKI: %w", err,
		)
	}

	return []byte(caCert), nil
}

// StartRenewal starts a goroutine that renews the certificate
// before expiry
func (m *VaultPKIManager) StartRenewal(ctx context.Context) {
	go m.renewalLoop(ctx)
}

// renewalLoop is the main renewal loop that runs in a goroutine
func (m *VaultPKIManager) renewalLoop(ctx context.Context) {
	for {
		_, span := otel.Tracer("cert").Start(ctx,
			"cert.renewalLoop.iteration",
			trace.WithAttributes(
				attribute.String(telemetry.AttrOperation,
					"certificate_renewal_check"),
			))

		renewAt, err := m.calculateRenewalTime()
		if err != nil {
			telemetry.RecordError(span, err)
			span.End()
			m.log.Errorw(
				"Failed to calculate renewal time, "+
					"retrying in 1m",
				"error", err,
			)
			if m.waitOrStop(ctx, time.Minute) {
				return
			}
			continue
		}

		sleepDuration := time.Until(renewAt)
		if sleepDuration <= 0 {
			sleepDuration = time.Minute
		}

		span.End()

		m.log.Infow("Vault PKI certificate renewal scheduled",
			"renew-at", renewAt.Format(time.RFC3339),
			"sleep-duration", sleepDuration.String(),
		)

		if m.waitOrStop(ctx, sleepDuration) {
			return
		}

		m.log.Infow("Renewing Vault PKI certificate")
		if err := m.IssueCertificateAndWriteToDisk(ctx); err != nil {
			metrics.IncCertificateRenewal("error")
			m.log.Errorw(
				"Failed to renew Vault PKI certificate",
				"error", err,
			)
		} else {
			metrics.IncCertificateRenewal("success")
			m.log.Infow(
				"Vault PKI certificate renewed successfully",
			)
		}
	}
}

// waitOrStop waits for the given duration or until stop/ctx cancel.
// Returns true if the loop should exit.
func (m *VaultPKIManager) waitOrStop(
	ctx context.Context, d time.Duration,
) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return false
	case <-m.stopCh:
		m.log.Infow("Vault PKI renewal stopped")
		return true
	case <-ctx.Done():
		m.log.Infow("Vault PKI renewal context canceled")
		return true
	}
}

// calculateRenewalTime parses the on-disk certificate to determine
// when to renew
func (m *VaultPKIManager) calculateRenewalTime() (time.Time, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	certPEM, err := os.ReadFile(m.certPath)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"failed to read certificate file %s: %w", m.certPath, err,
		)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, fmt.Errorf(
			"failed to decode PEM block from %s", m.certPath,
		)
	}

	parsedCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"failed to parse certificate from %s: %w",
			m.certPath, err,
		)
	}

	renewAt := parsedCert.NotAfter.Add(-m.renewalBuffer)

	// Update certificate expiry metric
	secondsUntilExpiry := time.Until(parsedCert.NotAfter).Seconds()
	metrics.SetCertificateExpiry("vault_pki", secondsUntilExpiry)

	return renewAt, nil
}

// Stop stops the renewal goroutine
func (m *VaultPKIManager) Stop() {
	close(m.stopCh)
}

// CertPath returns the path to the certificate file
func (m *VaultPKIManager) CertPath() string {
	return m.certPath
}

// KeyPath returns the path to the key file
func (m *VaultPKIManager) KeyPath() string {
	return m.keyPath
}
