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
)

// CertificateProvider defines the interface for certificate providers
// Implementations include self-signed, provided (external), and Vault PKI providers
type CertificateProvider interface {
	// IssueCertificate issues a new certificate for the given service
	// serviceName is the Kubernetes service name
	// namespace is the Kubernetes namespace
	IssueCertificate(ctx context.Context, serviceName, namespace string) (*Certificate, error)

	// RenewCertificate renews an existing certificate
	// For some providers (like Provided), this may return an error as renewal is not supported
	RenewCertificate(ctx context.Context, cert *Certificate) (*Certificate, error)

	// GetCertificate retrieves the current certificate
	// This may load from cache, file, or issue a new certificate depending on the provider
	GetCertificate(ctx context.Context) (*Certificate, error)

	// Type returns the provider type
	Type() CertificateProviderType

	// NeedsRenewal checks if the certificate needs to be renewed
	// This typically checks if the certificate is within the renewal buffer period
	NeedsRenewal(cert *Certificate) bool
}
