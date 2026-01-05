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

package webhook

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// BaseValidatorConfig contains common configuration for all validators
type BaseValidatorConfig struct {
	Client                      client.Client
	Decoder                     admission.Decoder
	Log                         *zap.SugaredLogger
	VaultClient                 *vault.Client
	PostgresqlConnectionRetries int
	PostgresqlConnectionTimeout time.Duration
	VaultAvailabilityRetries    int
	VaultAvailabilityRetryDelay time.Duration
}

// NewBaseValidatorConfig creates a new BaseValidatorConfig from manager, decoder, vault client, logger, and config
func NewBaseValidatorConfig(
	mgr ctrl.Manager,
	webhookDecoder admission.Decoder,
	vaultClient *vault.Client,
	lg *zap.SugaredLogger,
	cfg *config.Config,
) BaseValidatorConfig {
	return BaseValidatorConfig{
		Client:                      mgr.GetClient(),
		Decoder:                     webhookDecoder,
		Log:                         lg,
		VaultClient:                 vaultClient,
		PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
		PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
		VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
		VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
	}
}

type webhookConfig struct {
	path    string
	handler admission.Handler
	name    string
}

// RegisterWebhooks registers all webhook handlers with the webhook server
func RegisterWebhooks(
	mgr ctrl.Manager,
	webhookServer webhook.Server,
	webhookDecoder admission.Decoder,
	cfg *config.Config,
	vaultClient *vault.Client,
	excludeUserList []string,
	lg *zap.SugaredLogger,
) error {
	// Create base configuration for all validators
	baseCfg := NewBaseValidatorConfig(mgr, webhookDecoder, vaultClient, lg, cfg)

	// Define webhook registration configs
	webhookConfigs := []webhookConfig{
		{
			path: "/uservalidate",
			handler: &UserValidator{
				BaseValidatorConfig: baseCfg,
				ExcludeUserList:     excludeUserList,
			},
			name: "uservalidate",
		},
		{
			path: "/postgresqlvalidate",
			handler: &PostgresqlValidator{
				BaseValidatorConfig: baseCfg,
			},
			name: "postgresqlvalidate",
		},
		{
			path: "/databasevalidate",
			handler: &DatabaseValidator{
				BaseValidatorConfig: baseCfg,
			},
			name: "databasevalidate",
		},
		{
			path: "/grantvalidate",
			handler: &GrantValidator{
				BaseValidatorConfig: baseCfg,
			},
			name: "grantvalidate",
		},
		{
			path: "/rolegroupvalidate",
			handler: &RoleGroupValidator{
				BaseValidatorConfig: baseCfg,
			},
			name: "rolegroupvalidate",
		},
		{
			path: "/schemavalidate",
			handler: &SchemaValidator{
				BaseValidatorConfig: baseCfg,
			},
			name: "schemavalidate",
		},
	}

	// Register all webhook handlers
	for _, wc := range webhookConfigs {
		webhookServer.Register(wc.path, &webhook.Admission{Handler: wc.handler})
		lg.Infow("Registered webhook handler", "path", wc.path, "name", wc.name)
	}

	if err := mgr.Add(webhookServer); err != nil {
		lg.Error(err, "unable to set up webhook server")
		return fmt.Errorf("unable to set up webhook server: %w", err)
	}

	return nil
}
