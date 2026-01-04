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
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// GetCurrentNamespace discovers the current namespace from the service account
// Returns the namespace or a default value if not found
func GetCurrentNamespace(namespaceFile string) string {
	// Try to read from the service account namespace file (standard in Kubernetes pods)
	// nolint:gosec // reading from service account namespace file is safe in Kubernetes context
	if data, err := os.ReadFile(namespaceFile); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}

	// Default fallback
	return "k8s-postgresql-operator"
}

// UpdateWebhookConfigurationWithCABundle updates the ValidatingWebhookConfiguration with the CA bundle from the secret
func UpdateWebhookConfigurationWithCABundle(
	secretName, namespace string, config *rest.Config, webHookName string, lg *zap.SugaredLogger) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	ctx := context.Background()

	// Get the secret containing the certificate
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	caBundle, ok := secret.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("tls.crt not found in secret %s", secretName)
	}

	// Get the ValidatingWebhookConfiguration
	webhookConfigName := webHookName
	webhookConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
		ctx, webhookConfigName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ValidatingWebhookConfiguration: %w", err)
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
		return fmt.Errorf(
			"webhook 'postgresql-operator.vyrodovalexey.github.com' not found in ValidatingWebhookConfiguration")
	}

	// Update the ValidatingWebhookConfiguration
	_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(
		ctx, webhookConfig, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ValidatingWebhookConfiguration: %w", err)
	}

	return nil
}
