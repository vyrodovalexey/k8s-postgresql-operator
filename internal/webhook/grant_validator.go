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
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"time"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// GrantValidator validates Grant resources
type GrantValidator struct {
	Client                      client.Client
	Decoder                     admission.Decoder
	Log                         *zap.SugaredLogger
	VaultClient                 *vault.Client
	PostgresqlConnectionRetries int
	PostgresqlConnectionTimeout time.Duration
	VaultAvailabilityRetries    int
	VaultAvailabilityRetryDelay time.Duration
}

// Handle handles the admission request for Grant resources
func (v *GrantValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var grant instancev1alpha1.Grant

	if err := v.Decoder.Decode(req, &grant); err != nil {
		v.Log.Errorw("Failed to decode Grant object", "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if postgresqlID, role, and database are specified
	if grant.Spec.PostgresqlID == "" {
		return admission.Allowed("No postgresqlID specified")
	}
	if grant.Spec.Role == "" {
		return admission.Allowed("No role specified")
	}
	if grant.Spec.Database == "" {
		return admission.Allowed("No database specified")
	}

	postgresqlID := grant.Spec.PostgresqlID
	role := grant.Spec.Role
	databaseName := grant.Spec.Database

	v.Log.Infow("Validating Grant resource", "name", grant.Name, "namespace", grant.Namespace, "postgresqlID", postgresqlID, "role", role, "database", databaseName)

	// Check Vault availability if Vault client is configured
	if v.VaultClient != nil {
		if err := checkVaultAvailability(ctx, v.VaultClient, v.Log, v.VaultAvailabilityRetries, v.VaultAvailabilityRetryDelay); err != nil {
			msg := fmt.Sprintf("Vault is not available: %v", err)
			v.Log.Infow("Validation denied", "reason", msg)
			return admission.Denied(msg)
		}
	}

	// First, verify that the postgresqlID exists in a PostgreSQL CRD object
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := v.Client.List(ctx, postgresqlList); err != nil {
		v.Log.Errorw("Failed to list PostgreSQL resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	var foundPostgresql *instancev1alpha1.Postgresql
	for i := range postgresqlList.Items {
		if postgresqlList.Items[i].Spec.ExternalInstance != nil &&
			postgresqlList.Items[i].Spec.ExternalInstance.PostgresqlID == postgresqlID {
			foundPostgresql = &postgresqlList.Items[i]
			break
		}
	}

	if foundPostgresql == nil {
		msg := fmt.Sprintf("PostgreSQL instance with postgresqlID %s does not exist in the cluster", postgresqlID)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID)
		return admission.Denied(msg)
	}

	// Test PostgreSQL connection
	if err := testPostgreSQLConnection(ctx, foundPostgresql, v.VaultClient, v.Log, v.PostgresqlConnectionRetries, v.PostgresqlConnectionTimeout); err != nil {
		msg := fmt.Sprintf("Cannot connect to PostgreSQL instance with postgresqlID %s: %v", postgresqlID, err)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "error", err)
		return admission.Denied(msg)
	}

	// Verify that the database exists in any namespace for the same postgresqlID
	databaseList := &instancev1alpha1.DatabaseList{}
	if err := v.Client.List(ctx, databaseList); err != nil {
		v.Log.Errorw("Failed to list Database resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	databaseExists := false
	for _, database := range databaseList.Items {
		if database.Spec.PostgresqlID == postgresqlID &&
			database.Spec.Database == databaseName {
			databaseExists = true
			break
		}
	}

	if !databaseExists {
		msg := fmt.Sprintf("Database %s with postgresqlID %s does not exist in any namespace", databaseName, postgresqlID)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "database", databaseName)
		return admission.Denied(msg)
	}

	// List all Grant resources across all namespaces in the cluster
	grantList := &instancev1alpha1.GrantList{}
	if err := v.Client.List(ctx, grantList); err != nil {
		v.Log.Errorw("Failed to list Grant resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check for duplicate postgresqlID + role + database combination across the entire cluster
	for _, existingGrant := range grantList.Items {
		// Skip the current resource if this is an update
		if req.Operation == admissionv1.Update &&
			existingGrant.Name == grant.Name &&
			existingGrant.Namespace == grant.Namespace {
			continue
		}

		if existingGrant.Spec.PostgresqlID == postgresqlID &&
			existingGrant.Spec.Role == role &&
			existingGrant.Spec.Database == databaseName {
			msg := fmt.Sprintf("Grant with postgresqlID %s, role %s, and database %s already exists in namespace %s (instance: %s)",
				postgresqlID, role, databaseName, existingGrant.Namespace, existingGrant.Name)
			v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "role", role, "database", databaseName,
				"existing-namespace", existingGrant.Namespace, "existing-name", existingGrant.Name)
			return admission.Denied(msg)
		}
	}

	v.Log.Infow("Validation passed", "name", grant.Name, "namespace", grant.Namespace, "postgresqlID", postgresqlID, "role", role, "database", databaseName)
	return admission.Allowed("No duplicate postgresqlID, role, and database combination found in cluster")
}
