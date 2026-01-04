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

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
)

// setupWebhookCertificates sets up webhook certificates either from provided paths or by generating them
func setupWebhookCertificates(
	cfg *config.Config, webhookNames []string, lg *zap.SugaredLogger) (webhookCertPath, webhookKeyPath string) {
	// Generate or use provided webhook certificates
	if cfg.WebhookCertPath != "" {
		// Use provided certificates
		webhookCertPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertName)
		webhookKeyPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertKey)
		lg.Infow("Using provided webhook certificates",
			"webhook-cert-path", cfg.WebhookCertPath,
			"webhook-cert-name", cfg.WebhookCertName,
			"webhook-cert-key", cfg.WebhookCertKey)
	} else {
		// Generate self-signed certificates and store in Kubernetes Secret
		tempCertDir, err := os.MkdirTemp("", "webhook-certs-")
		if err != nil {
			lg.Error(err, "Failed to create temporary directory for webhook certificates")
			os.Exit(1)
		}
		lg.Infow("Generating self-signed webhook certificates and storing in Secret", "cert-dir", tempCertDir)

		// Discover current namespace
		namespace := k8s.GetCurrentNamespace(cfg.K8sNamespacePath)

		secretName := fmt.Sprintf("%s-webhook-cert", cfg.WebhookK8sServiceName)

		// Get Kubernetes config
		k8sConfig := ctrl.GetConfigOrDie()
		_, webhookCertPath, webhookKeyPath, err = cert.GenerateSelfSignedCertAndStoreInSecret(
			cfg.WebhookK8sServiceName, namespace, secretName, tempCertDir, k8sConfig)
		if err != nil {
			lg.Error(err, "Failed to generate self-signed webhook certificates")
			os.Exit(1)
		}
		lg.Infow("Webhook certificates generated and stored in Secret",
			"secret-name", secretName,
			"cert-path", webhookCertPath,
			"key-path", webhookKeyPath)

		// Update all ValidatingWebhookConfigurations with CA bundle from secret

		for _, webhookName := range webhookNames {
			if err := k8s.UpdateWebhookConfigurationWithCABundle(
				secretName, namespace, k8sConfig, webhookName, lg); err != nil {
				lg.Error(err, "Failed to update ValidatingWebhookConfiguration with CA bundle",
					"secret-name", secretName,
					"webhook-name", webhookName)
				// Don't exit, as the webhook might still work if the CA bundle is set manually
			} else {
				lg.Infow("Successfully updated ValidatingWebhookConfiguration with CA bundle",
					"secret-name", secretName,
					"webhook-name", webhookName)
			}
		}
	}

	return webhookCertPath, webhookKeyPath
}
