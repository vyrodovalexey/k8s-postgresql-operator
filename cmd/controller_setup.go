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

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// setupControllers sets up all controllers with the manager
func setupControllers(mgr ctrl.Manager, cfg *config.Config, vaultClient *vault.Client, lg *zap.SugaredLogger) error {
	// Setup all controllers
	type controllerSetup struct {
		name  string
		setup func() error
	}

	controllerSetups := []controllerSetup{
		{
			name: "Postgresql",
			setup: func() error {
				return (&controller.PostgresqlReconciler{
					Client:                      mgr.GetClient(),
					Scheme:                      mgr.GetScheme(),
					VaultClient:                 vaultClient,
					Log:                         lg,
					PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
					PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
					VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
					VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "User",
			setup: func() error {
				return (&controller.UserReconciler{
					Client:                      mgr.GetClient(),
					Scheme:                      mgr.GetScheme(),
					VaultClient:                 vaultClient,
					Log:                         lg,
					PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
					PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
					VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
					VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "Database",
			setup: func() error {
				return (&controller.DatabaseReconciler{
					Client:                      mgr.GetClient(),
					Scheme:                      mgr.GetScheme(),
					VaultClient:                 vaultClient,
					Log:                         lg,
					PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
					PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
					VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
					VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "Grant",
			setup: func() error {
				return (&controller.GrantReconciler{
					Client:                      mgr.GetClient(),
					Scheme:                      mgr.GetScheme(),
					VaultClient:                 vaultClient,
					Log:                         lg,
					PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
					PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
					VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
					VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "RoleGroup",
			setup: func() error {
				return (&controller.RoleGroupReconciler{
					Client:                      mgr.GetClient(),
					Scheme:                      mgr.GetScheme(),
					VaultClient:                 vaultClient,
					Log:                         lg,
					PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
					PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
					VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
					VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
				}).SetupWithManager(mgr)
			},
		},
		{
			name: "Schema",
			setup: func() error {
				return (&controller.SchemaReconciler{
					Client:                      mgr.GetClient(),
					Scheme:                      mgr.GetScheme(),
					VaultClient:                 vaultClient,
					Log:                         lg,
					PostgresqlConnectionRetries: cfg.PostgresqlConnectionRetries,
					PostgresqlConnectionTimeout: cfg.PostgresqlConnectionTimeout(),
					VaultAvailabilityRetries:    cfg.VaultAvailabilityRetries,
					VaultAvailabilityRetryDelay: cfg.VaultAvailabilityRetryDelay(),
				}).SetupWithManager(mgr)
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
