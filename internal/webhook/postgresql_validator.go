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

// PostgresqlValidator validates PostgreSQL resources
type PostgresqlValidator struct {
	Client                      client.Client
	Decoder                     admission.Decoder
	Log                         *zap.SugaredLogger
	VaultClient                 *vault.Client
	PostgresqlConnectionRetries int
	PostgresqlConnectionTimeout time.Duration
	VaultAvailabilityRetries    int
	VaultAvailabilityRetryDelay time.Duration
}

// Handle handles the admission request for PostgreSQL resources
func (v *PostgresqlValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var postgresql instancev1alpha1.Postgresql

	if err := v.Decoder.Decode(req, &postgresql); err != nil {
		v.Log.Errorw("Failed to decode PostgreSQL object", "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if postgresqlID is specified
	if postgresql.Spec.ExternalInstance == nil || postgresql.Spec.ExternalInstance.PostgresqlID == "" {
		// No postgresqlID specified, allow the request
		return admission.Allowed("No postgresqlID specified")
	}

	postgresqlID := postgresql.Spec.ExternalInstance.PostgresqlID

	v.Log.Infow("Validating PostgreSQL resource",
		"name", postgresql.Name, "namespace", postgresql.Namespace, "postgresqlID", postgresqlID)

	// Check Vault availability if Vault client is configured
	if v.VaultClient != nil {
		if err := checkVaultAvailability(
			ctx, v.VaultClient, v.Log, v.VaultAvailabilityRetries, v.VaultAvailabilityRetryDelay); err != nil {
			msg := fmt.Sprintf("Vault is not available: %v", err)
			v.Log.Infow("Validation denied", "reason", msg)
			return admission.Denied(msg)
		}
	}

	// Check for duplicate postgresqlID across the entire cluster
	duplicateResult, err := k8sclient.CheckDuplicatePostgresqlID(
		ctx, v.Client, postgresql.Name, postgresql.Namespace, postgresqlID,
		req.Operation == admissionv1.Update)
	if err != nil {
		v.Log.Errorw("Failed to check for duplicate PostgreSQL", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if duplicateResult.Found {
		v.Log.Infow("Validation denied", "reason", duplicateResult.Message, "postgresqlID", postgresqlID,
			"existing-namespace", duplicateResult.Existing.GetNamespace(), "existing-name", duplicateResult.Existing.GetName())
		return admission.Denied(duplicateResult.Message)
	}

	// Test PostgreSQL connection
	if err := testPostgreSQLConnection(
		ctx, &postgresql, v.VaultClient, v.Log,
		v.PostgresqlConnectionRetries, v.PostgresqlConnectionTimeout); err != nil {
		msg := fmt.Sprintf("Cannot connect to PostgreSQL instance with postgresqlID %s: %v", postgresqlID, err)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "error", err)
		return admission.Denied(msg)
	}

	v.Log.Infow("Validation passed",
		"name", postgresql.Name, "namespace", postgresql.Namespace, "postgresqlID", postgresqlID)
	return admission.Allowed("No duplicate postgresqlID found in cluster and connection test passed")
}
