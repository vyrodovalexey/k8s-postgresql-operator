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

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// DatabaseValidator validates Database resources
type DatabaseValidator struct {
	Client  client.Client
	Decoder admission.Decoder
	Log     *zap.SugaredLogger
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

	v.Log.Infow("Validating Database resource", "name", database.Name, "namespace", database.Namespace, "postgresqlID", postgresqlID, "database", databaseName)

	// First, verify that the postgresqlID exists in a PostgreSQL CRD object
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := v.Client.List(ctx, postgresqlList); err != nil {
		v.Log.Errorw("Failed to list PostgreSQL resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	postgresqlExists := false
	for _, postgresql := range postgresqlList.Items {
		if postgresql.Spec.ExternalInstance != nil &&
			postgresql.Spec.ExternalInstance.PostgresqlID == postgresqlID {
			postgresqlExists = true
			break
		}
	}

	if !postgresqlExists {
		msg := fmt.Sprintf("PostgreSQL instance with postgresqlID %s does not exist in the cluster", postgresqlID)
		v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID)
		return admission.Denied(msg)
	}

	// List all Database resources across all namespaces in the cluster
	databaseList := &instancev1alpha1.DatabaseList{}
	if err := v.Client.List(ctx, databaseList); err != nil {
		v.Log.Errorw("Failed to list Database resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check for duplicate postgresqlID + database name combination across the entire cluster
	for _, existingDatabase := range databaseList.Items {
		// Skip the current resource if this is an update
		if req.Operation == admissionv1.Update &&
			existingDatabase.Name == database.Name &&
			existingDatabase.Namespace == database.Namespace {
			continue
		}

		if existingDatabase.Spec.PostgresqlID == postgresqlID &&
			existingDatabase.Spec.Database == databaseName {
			msg := fmt.Sprintf("Database with postgresqlID %s and database name %s already exists in namespace %s (instance: %s)",
				postgresqlID, databaseName, existingDatabase.Namespace, existingDatabase.Name)
			v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "database", databaseName,
				"existing-namespace", existingDatabase.Namespace, "existing-name", existingDatabase.Name)
			return admission.Denied(msg)
		}
	}

	v.Log.Infow("Validation passed", "name", database.Name, "namespace", database.Namespace, "postgresqlID", postgresqlID, "database", databaseName)
	return admission.Allowed("No duplicate postgresqlID and database name combination found in cluster")
}
