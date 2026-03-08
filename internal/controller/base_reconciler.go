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

package controller

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// BaseReconcilerConfig contains common configuration for all reconcilers
type BaseReconcilerConfig struct {
	client.Client
	Scheme                      *runtime.Scheme
	VaultClient                 *vault.Client
	Log                         *zap.SugaredLogger
	Recorder                    record.EventRecorder
	PostgresqlConnectionRetries int
	PostgresqlConnectionTimeout time.Duration
	VaultAvailabilityRetries    int
	VaultAvailabilityRetryDelay time.Duration
}

// NewBaseReconcilerConfig creates a new BaseReconcilerConfig from manager, vault client, logger, and config
func NewBaseReconcilerConfig(
	mgr ctrl.Manager, vaultClient *vault.Client, lg *zap.SugaredLogger, cfg *config.Config,
) BaseReconcilerConfig {
	return BaseReconcilerConfig{
		Client:                      mgr.GetClient(),
		Scheme:                      mgr.GetScheme(),
		VaultClient:                 vaultClient,
		Log:                         lg,
		Recorder:                    mgr.GetEventRecorderFor("postgresql-operator"),
		PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
		PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
		VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
		VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
	}
}

type controllerSetup struct {
	name  string
	setup func() error
}

// SetupControllers sets up all controllers with the manager
func SetupControllers(mgr ctrl.Manager, cfg *config.Config, vaultClient *vault.Client, lg *zap.SugaredLogger) error {
	// Create base configuration for all controllers
	baseCfg := NewBaseReconcilerConfig(mgr, vaultClient, lg, cfg)

	// Setup all controllers
	controllerSetups := []controllerSetup{
		{
			name: "Postgresql",
			setup: func() error {
				return (&PostgresqlReconciler{BaseReconcilerConfig: baseCfg}).SetupWithManager(mgr)
			},
		},
		{
			name: "User",
			setup: func() error {
				return (&UserReconciler{BaseReconcilerConfig: baseCfg}).SetupWithManager(mgr)
			},
		},
		{
			name: "Database",
			setup: func() error {
				return (&DatabaseReconciler{BaseReconcilerConfig: baseCfg}).SetupWithManager(mgr)
			},
		},
		{
			name: "Grant",
			setup: func() error {
				return (&GrantReconciler{BaseReconcilerConfig: baseCfg}).SetupWithManager(mgr)
			},
		},
		{
			name: "RoleGroup",
			setup: func() error {
				return (&RoleGroupReconciler{BaseReconcilerConfig: baseCfg}).SetupWithManager(mgr)
			},
		},
		{
			name: "Schema",
			setup: func() error {
				return (&SchemaReconciler{BaseReconcilerConfig: baseCfg}).SetupWithManager(mgr)
			},
		},
	}

	for _, ctrlSetup := range controllerSetups {
		if err := ctrlSetup.setup(); err != nil {
			lg.Errorw("unable to create controller", "error", err, "controller", ctrlSetup.name)
			return fmt.Errorf("unable to create controller %s: %w", ctrlSetup.name, err)
		}
	}

	return nil
}

// connectionInfo holds PostgreSQL connection details resolved from a CRD
type connectionInfo struct {
	ExternalInstance *instancev1alpha1.ExternalPostgresqlInstance
	Port             int32
	SSLMode          string
	Username         string
	Password         string
}

// resolvePostgresqlConnection finds a PostgreSQL instance by ID,
// validates it is connected, and retrieves admin credentials.
// Returns nil connectionInfo and an error string if validation fails.
func (r *BaseReconcilerConfig) resolvePostgresqlConnection(
	ctx context.Context, postgresqlID string,
) (info *connectionInfo, reason string, message string) {
	postgresql, err := k8sclient.FindPostgresqlByID(
		ctx, r.Client, postgresqlID,
	)
	if err != nil {
		r.Log.Errorw("Failed to find PostgreSQL instance",
			"error", err, "postgresqlID", postgresqlID)
		return nil, "PostgresqlNotFound",
			fmt.Sprintf("PostgreSQL instance with ID %s not found: %v",
				postgresqlID, err)
	}

	if !postgresql.Status.Connected {
		r.Log.Infow("PostgreSQL instance is not connected, waiting",
			"postgresqlID", postgresqlID)
		return nil, "PostgresqlNotConnected",
			fmt.Sprintf("PostgreSQL instance with ID %s is not connected",
				postgresqlID)
	}

	externalInstance := postgresql.Spec.ExternalInstance
	if externalInstance == nil {
		r.Log.Errorw(
			"PostgreSQL instance has no external instance configuration",
			"postgresqlID", postgresqlID)
		return nil, "InvalidConfiguration",
			"PostgreSQL instance has no external instance configuration"
	}

	port := externalInstance.Port
	if port == 0 {
		port = pg.DefaultPort
	}

	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = pg.DefaultSSLMode
	}

	return &connectionInfo{
		ExternalInstance: externalInstance,
		Port:             port,
		SSLMode:          sslMode,
	}, "", ""
}

// resolveAdminCredentials retrieves admin credentials from Vault
// and populates them in the connectionInfo.
func (r *BaseReconcilerConfig) resolveAdminCredentials(
	ctx context.Context, info *connectionInfo,
) error {
	if r.VaultClient == nil {
		r.Log.Infow("Vault client not available")
		return nil
	}

	username, password, err := getVaultCredentialsWithRetry(
		ctx, r.VaultClient, info.ExternalInstance.PostgresqlID,
		r.Log, r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay,
	)
	if err != nil {
		r.Log.Errorw("Failed to get credentials from Vault",
			"error", err, "postgresqlID", info.ExternalInstance.PostgresqlID)
		return err
	}

	info.Username = username
	info.Password = password
	r.Log.Infow("Credentials retrieved from Vault",
		"postgresqlID", info.ExternalInstance.PostgresqlID)
	return nil
}
