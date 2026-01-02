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

	v.Log.Infow("Validating PostgreSQL resource", "name", postgresql.Name, "namespace", postgresql.Namespace, "postgresqlID", postgresqlID)

	// Check Vault availability if Vault client is configured
	if v.VaultClient != nil {
		if err := checkVaultAvailability(ctx, v.VaultClient, v.Log, v.VaultAvailabilityRetries, v.VaultAvailabilityRetryDelay); err != nil {
			msg := fmt.Sprintf("Vault is not available: %v", err)
			v.Log.Infow("Validation denied", "reason", msg)
			return admission.Denied(msg)
		}
	}

	// List all PostgreSQL resources across all namespaces in the cluster
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := v.Client.List(ctx, postgresqlList); err != nil {
		v.Log.Errorw("Failed to list PostgreSQL resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check for duplicate postgresqlID across the entire cluster
	for _, existingPostgresql := range postgresqlList.Items {
		// Skip the current resource if this is an update
		if req.Operation == admissionv1.Update &&
			existingPostgresql.Name == postgresql.Name &&
			existingPostgresql.Namespace == postgresql.Namespace {
			continue
		}

		if existingPostgresql.Spec.ExternalInstance != nil &&
			existingPostgresql.Spec.ExternalInstance.PostgresqlID == postgresqlID {
			msg := fmt.Sprintf("PostgreSQL instance with postgresqlID %s already exists in namespace %s (instance: %s)",
				postgresqlID, existingPostgresql.Namespace, existingPostgresql.Name)
			v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID,
				"existing-namespace", existingPostgresql.Namespace, "existing-name", existingPostgresql.Name)
			return admission.Denied(msg)
		}
	}

	// Test PostgreSQL connection
	if err := testPostgreSQLConnection(ctx, &postgresql, v.VaultClient, v.Log, v.PostgresqlConnectionRetries, v.PostgresqlConnectionTimeout); err != nil {
		msg := fmt.Sprintf("Cannot connect to PostgreSQL instance with postgresqlID %s: %v", postgresqlID, err)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "error", err)
		return admission.Denied(msg)
	}

	v.Log.Infow("Validation passed", "name", postgresql.Name, "namespace", postgresql.Namespace, "postgresqlID", postgresqlID)
	return admission.Allowed("No duplicate postgresqlID found in cluster and connection test passed")
}
