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
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SelfSignedProviderOptions contains options for creating a SelfSignedProvider
type SelfSignedProviderOptions struct {
	// ServiceName is the Kubernetes service name for the webhook
	ServiceName string
	// Namespace is the Kubernetes namespace
	Namespace string
	// SecretName is the name of the Kubernetes secret to store the certificate
	SecretName string
	// CertDir is the directory to write certificate files
	CertDir string
	// RestConfig is the Kubernetes REST config
	RestConfig *rest.Config
	// Logger is the zap logger
	Logger *zap.SugaredLogger
	// RenewalBuffer is the time before expiry to trigger renewal
	RenewalBuffer time.Duration
	// CertificateTTL is the TTL for the certificate
	CertificateTTL time.Duration
}

// SelfSignedProvider implements CertificateProvider for self-signed certificates
type SelfSignedProvider struct {
	serviceName    string
	namespace      string
	secretName     string
	certDir        string
	restConfig     *rest.Config
	logger         *zap.SugaredLogger
	renewalBuffer  time.Duration
	certificateTTL time.Duration
	currentCert    *Certificate
	mu             sync.RWMutex
}

// NewSelfSignedProvider creates a new SelfSignedProvider
func NewSelfSignedProvider(opts SelfSignedProviderOptions) (*SelfSignedProvider, error) {
	if opts.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if opts.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if opts.RestConfig == nil {
		return nil, fmt.Errorf("rest config is required")
	}

	renewalBuffer := opts.RenewalBuffer
	if renewalBuffer == 0 {
		renewalBuffer = DefaultRenewalBuffer
	}

	certificateTTL := opts.CertificateTTL
	if certificateTTL == 0 {
		certificateTTL = 365 * 24 * time.Hour // 1 year default
	}

	secretName := opts.SecretName
	if secretName == "" {
		secretName = fmt.Sprintf("%s-webhook-cert", opts.ServiceName)
	}

	return &SelfSignedProvider{
		serviceName:    opts.ServiceName,
		namespace:      opts.Namespace,
		secretName:     secretName,
		certDir:        opts.CertDir,
		restConfig:     opts.RestConfig,
		logger:         opts.Logger,
		renewalBuffer:  renewalBuffer,
		certificateTTL: certificateTTL,
	}, nil
}

// IssueCertificate generates a new self-signed certificate
func (p *SelfSignedProvider) IssueCertificate(
	ctx context.Context, serviceName, namespace string) (*Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.logger != nil {
		p.logger.Infow("Issuing self-signed certificate",
			"service", serviceName,
			"namespace", namespace)
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			Organization: []string{"postgresql-operator.vyrodovalexey.github.com"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(p.certificateTTL),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames: []string{
			serviceName,
			fmt.Sprintf("%s.%s", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
		},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Encode private key to PEM
	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyDER})

	cert := &Certificate{
		CertPEM:      certPEM,
		KeyPEM:       keyPEM,
		CACertPEM:    certPEM, // Self-signed, so CA is the same as cert
		ExpiresAt:    template.NotAfter,
		IssuedAt:     template.NotBefore,
		SerialNumber: serialNumber.String(),
	}

	p.currentCert = cert

	if p.logger != nil {
		p.logger.Infow("Self-signed certificate issued successfully",
			"serial", cert.SerialNumber,
			"expires_at", cert.ExpiresAt)
	}

	return cert, nil
}

// RenewCertificate renews the certificate by issuing a new one
func (p *SelfSignedProvider) RenewCertificate(ctx context.Context, _ *Certificate) (*Certificate, error) {
	if p.logger != nil {
		p.logger.Infow("Renewing self-signed certificate",
			"service", p.serviceName,
			"namespace", p.namespace)
	}
	return p.IssueCertificate(ctx, p.serviceName, p.namespace)
}

// GetCertificate retrieves the current certificate, loading from secret if available
func (p *SelfSignedProvider) GetCertificate(ctx context.Context) (*Certificate, error) {
	p.mu.RLock()
	if p.currentCert != nil && p.currentCert.IsValid() {
		cert := p.currentCert
		p.mu.RUnlock()
		return cert, nil
	}
	p.mu.RUnlock()

	// Try to load from Kubernetes secret
	cert, err := p.loadCertificateFromSecret(ctx)
	if err == nil && cert != nil {
		return cert, nil
	}

	// No valid certificate found, issue a new one
	return p.IssueCertificate(ctx, p.serviceName, p.namespace)
}

// loadCertificateFromSecret attempts to load a certificate from the Kubernetes secret
func (p *SelfSignedProvider) loadCertificateFromSecret(ctx context.Context) (*Certificate, error) {
	clientset, err := kubernetes.NewForConfig(p.restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	secret, err := clientset.CoreV1().Secrets(p.namespace).Get(ctx, p.secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	certData, certOk := secret.Data["tls.crt"]
	keyData, keyOk := secret.Data["tls.key"]
	if !certOk || !keyOk {
		return nil, fmt.Errorf("certificate or key not found in secret")
	}

	// Parse certificate to get expiration
	cert, parseErr := parseCertificatePEM(certData)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", parseErr)
	}

	p.mu.Lock()
	p.currentCert = cert
	p.currentCert.CertPEM = certData
	p.currentCert.KeyPEM = keyData
	p.currentCert.CACertPEM = certData
	p.mu.Unlock()

	if p.logger != nil {
		p.logger.Infow("Loaded certificate from secret",
			"secret", p.secretName,
			"expires_at", cert.ExpiresAt)
	}

	return p.currentCert, nil
}

// Type returns the provider type
func (p *SelfSignedProvider) Type() CertificateProviderType {
	return ProviderTypeSelfSigned
}

// NeedsRenewal checks if the certificate needs to be renewed
func (p *SelfSignedProvider) NeedsRenewal(cert *Certificate) bool {
	if cert == nil {
		return true
	}
	return time.Until(cert.ExpiresAt) < p.renewalBuffer
}

// StoreInSecret stores the certificate in a Kubernetes secret
func (p *SelfSignedProvider) StoreInSecret(ctx context.Context, cert *Certificate) error {
	clientset, err := kubernetes.NewForConfig(p.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.secretName,
			Namespace: p.namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": cert.CertPEM,
			"tls.key": cert.KeyPEM,
		},
	}

	// Try to create the secret, if it exists, update it
	_, err = clientset.CoreV1().Secrets(p.namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		// Secret might already exist, try to update it
		_, err = clientset.CoreV1().Secrets(p.namespace).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create/update secret: %w", err)
		}
	}

	if p.logger != nil {
		p.logger.Infow("Certificate stored in secret",
			"secret", p.secretName,
			"namespace", p.namespace)
	}

	return nil
}

// GetSecretName returns the secret name
func (p *SelfSignedProvider) GetSecretName() string {
	return p.secretName
}

// parseCertificatePEM parses a PEM-encoded certificate and extracts metadata
func parseCertificatePEM(certPEM []byte) (*Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &Certificate{
		ExpiresAt:    x509Cert.NotAfter,
		IssuedAt:     x509Cert.NotBefore,
		SerialNumber: x509Cert.SerialNumber.String(),
	}, nil
}
