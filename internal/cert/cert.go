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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// GenerateSelfSignedCertAndStoreInSecret generates a self-signed certificate and stores it in a Kubernetes Secret
// Returns the secret name and paths to the certificate and key files
func GenerateSelfSignedCertAndStoreInSecret(
	serviceName, namespace, secretName, certDir string, config *rest.Config) (
	secretNameOut string, certPath, keyPath string, err error) {
	// Create cert directory if it doesn't exist
	if err := os.MkdirAll(certDir, 0o750); err != nil {
		return "", "", "", fmt.Errorf("failed to create cert directory: %w", err)
	}

	certPath = filepath.Join(certDir, "tls.crt")
	keyPath = filepath.Join(certDir, "tls.key")

	// Create Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	ctx := context.Background()

	// Check if secret already exists
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err == nil && secret != nil {
		// Secret exists, check if it has the required keys
		if certData, ok := secret.Data["tls.crt"]; ok {
			if keyData, ok := secret.Data["tls.key"]; ok {
				// Write certificate to file
				certFile, err := os.Create(certPath) //nolint:gosec  //microservices approach
				if err != nil {
					return "", "", "", fmt.Errorf("failed to create cert file: %w", err)
				}
				defer certFile.Close()
				if _, err := certFile.Write(certData); err != nil {
					return "", "", "", fmt.Errorf("failed to write certificate: %w", err)
				}

				// Write private key to file
				keyFile, err := os.Create(keyPath) //nolint:gosec // microservices approach
				if err != nil {
					return "", "", "", fmt.Errorf("failed to create key file: %w", err)
				}
				defer keyFile.Close()
				if _, err := keyFile.Write(keyData); err != nil {
					return "", "", "", fmt.Errorf("failed to write private key: %w", err)
				}

				return secretName, certPath, keyPath, nil
			}
		}
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			Organization: []string{"postgresql-operator.vyrodovalexey.github.com"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
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
		return "", "", "", fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode certificate to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Encode private key to PEM
	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyDER})

	// Create or update the Secret
	secret = &corev1.Secret{
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

	// Try to create the secret, if it exists, update it
	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		// Secret might already exist, try to update it
		_, err = clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return "", "", "", fmt.Errorf("failed to create/update secret: %w", err)
		}
	}

	// Write certificate to file
	certFile, err := os.Create(certPath) //nolint:gosec // microservices approach
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certFile.Close()

	if _, err := certFile.Write(certPEM); err != nil {
		return "", "", "", fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key to file
	keyFile, err := os.Create(keyPath) //nolint:gosec // microservices approach
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyFile.Close()

	if _, err := keyFile.Write(keyPEM); err != nil {
		return "", "", "", fmt.Errorf("failed to write private key: %w", err)
	}

	return secretName, certPath, keyPath, nil
}

// LoadCert loads a certificate and key from files
func LoadCert(certPath, keyPath string) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}
	return &cert, nil
}
