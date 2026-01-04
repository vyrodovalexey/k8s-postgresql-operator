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
	k8sclient "github.com/vyrodovalexey/k8s-postgresql-operator/internal/k8s"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
)

// SchemaValidator validates Schema resources
type SchemaValidator struct {
	Client                      client.Client
	Decoder                     admission.Decoder
	Log                         *zap.SugaredLogger
	VaultClient                 *vault.Client
	PostgresqlConnectionRetries int
	PostgresqlConnectionTimeout time.Duration
	VaultAvailabilityRetries    int
	VaultAvailabilityRetryDelay time.Duration
}

// Handle handles the admission request for Schema resources
func (v *SchemaValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var schema instancev1alpha1.Schema

	if err := v.Decoder.Decode(req, &schema); err != nil {
		v.Log.Errorw("Failed to decode Schema object", "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if postgresqlID, schema, and owner are specified
	if schema.Spec.PostgresqlID == "" {
		return admission.Allowed("No postgresqlID specified")
	}
	if schema.Spec.Schema == "" {
		return admission.Allowed("No schema name specified")
	}
	if schema.Spec.Owner == "" {
		return admission.Allowed("No owner specified")
	}

	postgresqlID := schema.Spec.PostgresqlID
	schemaName := schema.Spec.Schema

	v.Log.Infow("Validating Schema resource",
		"name", schema.Name, "namespace", schema.Namespace, "postgresqlID", postgresqlID, "schema", schemaName)

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

	// Check for duplicate postgresqlID + schema combination across the entire cluster
	duplicateResult, err := k8sclient.CheckDuplicateSchema(
		ctx, v.Client, schema.Name, schema.Namespace, postgresqlID, schemaName,
		req.Operation == admissionv1.Update)
	if err != nil {
		v.Log.Errorw("Failed to check for duplicate Schema", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if duplicateResult.Found {
		v.Log.Infow("Validation denied",
			"reason", duplicateResult.Message, "postgresqlID", postgresqlID, "schema", schemaName,
			"existing-namespace", duplicateResult.Existing.GetNamespace(), "existing-name", duplicateResult.Existing.GetName())
		return admission.Denied(duplicateResult.Message)
	}

	v.Log.Infow("Validation passed",
		"name", schema.Name, "namespace", schema.Namespace, "postgresqlID", postgresqlID, "schema", schemaName)
	return admission.Allowed("No duplicate postgresqlID and schema combination found in cluster")
}
