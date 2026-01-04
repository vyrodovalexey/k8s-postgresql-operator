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

// UserValidator validates User resources
type UserValidator struct {
	Client                      client.Client
	Decoder                     admission.Decoder
	Log                         *zap.SugaredLogger
	ExcludeUserList             []string
	VaultClient                 *vault.Client
	PostgresqlConnectionRetries int
	PostgresqlConnectionTimeout time.Duration
	VaultAvailabilityRetries    int
	VaultAvailabilityRetryDelay time.Duration
}

// Handle handles the admission request for User resources
func (v *UserValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var user instancev1alpha1.User

	if err := v.Decoder.Decode(req, &user); err != nil {
		v.Log.Errorw("Failed to decode User object", "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if postgresqlID and username are specified
	if user.Spec.PostgresqlID == "" {
		return admission.Allowed("No postgresqlID specified")
	}
	if user.Spec.Username == "" {
		return admission.Allowed("No username specified")
	}

	postgresqlID := user.Spec.PostgresqlID
	username := user.Spec.Username

	v.Log.Infow("Validating User resource",
		"name", user.Name, "namespace", user.Namespace, "postgresqlID", postgresqlID, "username", username)

	// Check Vault availability if Vault client is configured
	if v.VaultClient != nil {
		if err := checkVaultAvailability(
			ctx, v.VaultClient, v.Log, v.VaultAvailabilityRetries, v.VaultAvailabilityRetryDelay); err != nil {
			msg := fmt.Sprintf("Vault is not available: %v", err)
			v.Log.Infow("Validation denied", "reason", msg)
			return admission.Denied(msg)
		}
	}

	// Check if username is in the Excludelist
	for _, excludelistedUser := range v.ExcludeUserList {
		if excludelistedUser == username {
			msg := fmt.Sprintf("Username %s is in the excludelist and cannot be created", username)
			v.Log.Infow("Validation denied", "reason", msg, "username", username)
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

	// Check for duplicate postgresqlID + username combination across the entire cluster
	duplicateResult, err := k8sclient.CheckDuplicateUser(
		ctx, v.Client, user.Name, user.Namespace, postgresqlID, username,
		req.Operation == admissionv1.Update)
	if err != nil {
		v.Log.Errorw("Failed to check for duplicate User", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if duplicateResult.Found {
		v.Log.Infow("Validation denied",
			"reason", duplicateResult.Message, "postgresqlID", postgresqlID, "username", username,
			"existing-namespace", duplicateResult.Existing.GetNamespace(), "existing-name", duplicateResult.Existing.GetName())
		return admission.Denied(duplicateResult.Message)
	}

	v.Log.Infow("Validation passed",
		"name", user.Name, "namespace", user.Namespace, "postgresqlID", postgresqlID, "username", username)
	return admission.Allowed("No duplicate postgresqlID and username combination found in cluster")
}
