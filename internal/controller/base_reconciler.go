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
	"fmt"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// BaseReconcilerConfig contains common configuration for all reconcilers
type BaseReconcilerConfig struct {
	client.Client
	Scheme                      *runtime.Scheme
	VaultClient                 *vault.Client
	Log                         *zap.SugaredLogger
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
			lg.Error(err, "unable to create controller", "controller", ctrlSetup.name)
			return fmt.Errorf("unable to create controller %s: %w", ctrlSetup.name, err)
		}
	}

	return nil
}
