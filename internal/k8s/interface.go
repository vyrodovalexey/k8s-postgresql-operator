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

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretsInterface abstracts Kubernetes Secrets operations for testing
type SecretsInterface interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Secret, error)
	Create(ctx context.Context, secret *corev1.Secret, opts metav1.CreateOptions) (*corev1.Secret, error)
	Update(ctx context.Context, secret *corev1.Secret, opts metav1.UpdateOptions) (*corev1.Secret, error)
}

// ValidatingWebhookConfigurationsInterface abstracts Kubernetes ValidatingWebhookConfigurations operations for testing
type ValidatingWebhookConfigurationsInterface interface {
	Get(
		ctx context.Context, name string, opts metav1.GetOptions,
	) (*admissionregistrationv1.ValidatingWebhookConfiguration, error)
	Update(
		ctx context.Context, config *admissionregistrationv1.ValidatingWebhookConfiguration, opts metav1.UpdateOptions,
	) (*admissionregistrationv1.ValidatingWebhookConfiguration, error)
}

// KubernetesClient abstracts Kubernetes client operations for testing
type KubernetesClient interface {
	Secrets(namespace string) SecretsInterface
	ValidatingWebhookConfigurations() ValidatingWebhookConfigurationsInterface
}
