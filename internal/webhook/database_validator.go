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

// DatabaseValidator validates Database resources
type DatabaseValidator struct {
	BaseValidatorConfig
}

// Handle handles the admission request for Database resources
func (v *DatabaseValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var database instancev1alpha1.Database

	if err := v.Decoder.Decode(req, &database); err != nil {
		v.Log.Errorw("Failed to decode Database object", "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if postgresqlID and database name are specified
	if database.Spec.PostgresqlID == "" {
		return admission.Allowed("No postgresqlID specified")
	}
	if database.Spec.Database == "" {
		return admission.Allowed("No database name specified")
	}

	postgresqlID := database.Spec.PostgresqlID
	databaseName := database.Spec.Database

	v.Log.Infow("Validating Database resource",
		"name", database.Name, "namespace", database.Namespace, "postgresqlID", postgresqlID, "database", databaseName)

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

	// Check for duplicate postgresqlID + database name combination across the entire cluster
	duplicateResult, err := k8sclient.CheckDuplicateDatabase(
		ctx, v.Client, database.Name, database.Namespace, postgresqlID, databaseName,
		req.Operation == admissionv1.Update)
	if err != nil {
		v.Log.Errorw("Failed to check for duplicate Database", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if duplicateResult.Found {
		v.Log.Infow("Validation denied",
			"reason", duplicateResult.Message, "postgresqlID", postgresqlID, "database", databaseName,
			"existing-namespace", duplicateResult.Existing.GetNamespace(), "existing-name", duplicateResult.Existing.GetName())
		return admission.Denied(duplicateResult.Message)
	}

	v.Log.Infow("Validation passed",
		"name", database.Name, "namespace", database.Namespace, "postgresqlID", postgresqlID, "database", databaseName)
	return admission.Allowed("No duplicate postgresqlID and database name combination found in cluster")
}
