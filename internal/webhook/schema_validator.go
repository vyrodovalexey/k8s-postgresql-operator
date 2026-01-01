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

// SchemaValidator validates Schema resources
type SchemaValidator struct {
	Client  client.Client
	Decoder admission.Decoder
	Log     *zap.SugaredLogger
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

	v.Log.Infow("Validating Schema resource", "name", schema.Name, "namespace", schema.Namespace, "postgresqlID", postgresqlID, "schema", schemaName)

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

	// List all Schema resources across all namespaces in the cluster
	schemaList := &instancev1alpha1.SchemaList{}
	if err := v.Client.List(ctx, schemaList); err != nil {
		v.Log.Errorw("Failed to list Schema resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check for duplicate postgresqlID + schema combination across the entire cluster
	for _, existingSchema := range schemaList.Items {
		// Skip the current resource if this is an update
		if req.Operation == admissionv1.Update &&
			existingSchema.Name == schema.Name &&
			existingSchema.Namespace == schema.Namespace {
			continue
		}

		if existingSchema.Spec.PostgresqlID == postgresqlID &&
			existingSchema.Spec.Schema == schemaName {
			msg := fmt.Sprintf("Schema with postgresqlID %s and schema %s already exists in namespace %s (instance: %s)",
				postgresqlID, schemaName, existingSchema.Namespace, existingSchema.Name)
			v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "schema", schemaName,
				"existing-namespace", existingSchema.Namespace, "existing-name", existingSchema.Name)
			return admission.Denied(msg)
		}
	}

	v.Log.Infow("Validation passed", "name", schema.Name, "namespace", schema.Namespace, "postgresqlID", postgresqlID, "schema", schemaName)
	return admission.Allowed("No duplicate postgresqlID and schema combination found in cluster")
}
