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

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
)

// GrantValidator validates Grant resources
type GrantValidator struct {
	BaseValidatorConfig
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

	v.Log.Infow("Validating Grant resource",
		"name", grant.Name, "namespace", grant.Namespace, "postgresqlID", postgresqlID,
		"role", role, "database", databaseName)

	// Check Vault availability if Vault client is configured
	if v.VaultClient != nil {
		if err := checkVaultAvailability(
			ctx, v.VaultClient, v.Log, v.VaultAvailabilityRetries, v.VaultAvailabilityRetryDelay); err != nil {
			msg := fmt.Sprintf("Vault is not available: %v", err)
			v.Log.Infow("Validation denied", "reason", msg)
			return admission.Denied(msg)
		}
	}

	// First, verify that the postgresqlID exists in a PostgreSQL CRD object
	foundPostgresql, err := k8sclient.FindPostgresqlByID(ctx, v.Client, postgresqlID)
	if err != nil {
		msg := fmt.Sprintf("PostgreSQL instance with postgresqlID %s does not exist in the cluster", postgresqlID)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID)
		return admission.Denied(msg)
	}

	// Test PostgreSQL connection
	if err := testPostgreSQLConnection(
		ctx, foundPostgresql, v.VaultClient, v.Log,
		v.PostgresqlConnectionRetries, v.PostgresqlConnectionTimeout); err != nil {
		msg := fmt.Sprintf("Cannot connect to PostgreSQL instance with postgresqlID %s: %v", postgresqlID, err)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "error", err)
		return admission.Denied(msg)
	}

	// Verify that the database exists in any namespace for the same postgresqlID
	databaseExists, err := k8sclient.DatabaseExistsByPostgresqlIDAndName(ctx, v.Client, postgresqlID, databaseName)
	if err != nil {
		v.Log.Errorw("Failed to check Database existence", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !databaseExists {
		msg := fmt.Sprintf("Database %s with postgresqlID %s does not exist in any namespace", databaseName, postgresqlID)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "database", databaseName)
		return admission.Denied(msg)
	}

	// Check for duplicate postgresqlID + role + database combination across the entire cluster
	duplicateResult, err := k8sclient.CheckDuplicateGrant(
		ctx, v.Client, grant.Name, grant.Namespace, postgresqlID, role, databaseName,
		req.Operation == admissionv1.Update)
	if err != nil {
		v.Log.Errorw("Failed to check for duplicate Grant", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if duplicateResult.Found {
		v.Log.Infow("Validation denied",
			"reason", duplicateResult.Message, "postgresqlID", postgresqlID, "role", role, "database", databaseName,
			"existing-namespace", duplicateResult.Existing.GetNamespace(), "existing-name", duplicateResult.Existing.GetName())
		return admission.Denied(duplicateResult.Message)
	}

	v.Log.Infow("Validation passed",
		"name", grant.Name, "namespace", grant.Namespace, "postgresqlID", postgresqlID,
		"role", role, "database", databaseName)
	return admission.Allowed("No duplicate postgresqlID, role, and database combination found in cluster")
}
