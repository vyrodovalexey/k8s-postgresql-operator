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

// RoleGroupValidator validates RoleGroup resources
type RoleGroupValidator struct {
	Client                      client.Client
	Decoder                     admission.Decoder
	Log                         *zap.SugaredLogger
	VaultClient                 *vault.Client
	PostgresqlConnectionRetries int
	PostgresqlConnectionTimeout time.Duration
	VaultAvailabilityRetries    int
	VaultAvailabilityRetryDelay time.Duration
}

// Handle handles the admission request for RoleGroup resources
func (v *RoleGroupValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var roleGroup instancev1alpha1.RoleGroup

	if err := v.Decoder.Decode(req, &roleGroup); err != nil {
		v.Log.Errorw("Failed to decode RoleGroup object", "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if postgresqlID and groupRole are specified
	if roleGroup.Spec.PostgresqlID == "" {
		return admission.Allowed("No postgresqlID specified")
	}
	if roleGroup.Spec.GroupRole == "" {
		return admission.Allowed("No groupRole specified")
	}

	postgresqlID := roleGroup.Spec.PostgresqlID
	groupRole := roleGroup.Spec.GroupRole

	v.Log.Infow("Validating RoleGroup resource",
		"name", roleGroup.Name, "namespace", roleGroup.Namespace, "postgresqlID", postgresqlID, "groupRole", groupRole)

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

	// Check for duplicate postgresqlID + groupRole combination across the entire cluster
	duplicateResult, err := k8sclient.CheckDuplicateRoleGroup(
		ctx, v.Client, roleGroup.Name, roleGroup.Namespace, postgresqlID, groupRole,
		req.Operation == admissionv1.Update)
	if err != nil {
		v.Log.Errorw("Failed to check for duplicate RoleGroup", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if duplicateResult.Found {
		v.Log.Infow("Validation denied",
			"reason", duplicateResult.Message, "postgresqlID", postgresqlID, "groupRole", groupRole,
			"existing-namespace", duplicateResult.Existing.GetNamespace(), "existing-name", duplicateResult.Existing.GetName())
		return admission.Denied(duplicateResult.Message)
	}

	v.Log.Infow("Validation passed",
		"name", roleGroup.Name, "namespace", roleGroup.Namespace, "postgresqlID", postgresqlID, "groupRole", groupRole)
	return admission.Allowed("No duplicate postgresqlID and groupRole combination found in cluster")
}
