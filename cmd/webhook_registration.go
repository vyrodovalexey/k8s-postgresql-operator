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

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
	webhookpkg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/webhook"
)

// registerWebhooks registers all webhook handlers with the webhook server
func registerWebhooks(mgr ctrl.Manager, webhookServer webhook.Server, webhookDecoder admission.Decoder,
	cfg *config.Config, vaultClient *vault.Client, excludeUserList []string, lg *zap.SugaredLogger) error {
	// Define webhook registration configs
	type webhookConfig struct {
		path    string
		handler admission.Handler
		name    string
	}

	webhookConfigs := []webhookConfig{
		{
			path: "/uservalidate",
			handler: &webhookpkg.UserValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				ExcludeUserList:             excludeUserList,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "uservalidate",
		},
		{
			path: "/postgresqlvalidate",
			handler: &webhookpkg.PostgresqlValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "postgresqlvalidate",
		},
		{
			path: "/databasevalidate",
			handler: &webhookpkg.DatabaseValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "databasevalidate",
		},
		{
			path: "/grantvalidate",
			handler: &webhookpkg.GrantValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "grantvalidate",
		},
		{
			path: "/rolegroupvalidate",
			handler: &webhookpkg.RoleGroupValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "rolegroupvalidate",
		},
		{
			path: "/schemavalidate",
			handler: &webhookpkg.SchemaValidator{
				Client:                      mgr.GetClient(),
				Decoder:                     webhookDecoder,
				Log:                         lg,
				VaultClient:                 vaultClient,
				PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
				PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
				VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
				VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
			},
			name: "schemavalidate",
		},
	}

	// Register all webhook handlers
	for _, cfg := range webhookConfigs {
		webhookServer.Register(cfg.path, &webhook.Admission{Handler: cfg.handler})
		lg.Infow("Registered webhook handler", "path", cfg.path, "name", cfg.name)
	}

	if err := mgr.Add(webhookServer); err != nil {
		lg.Error(err, "unable to set up webhook server")
		return fmt.Errorf("unable to set up webhook server: %w", err)
	}

	return nil
}
