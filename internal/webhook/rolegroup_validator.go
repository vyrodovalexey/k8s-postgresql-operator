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

// RoleGroupValidator validates RoleGroup resources
type RoleGroupValidator struct {
	Client  client.Client
	Decoder admission.Decoder
	Log     *zap.SugaredLogger
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

	v.Log.Infow("Validating RoleGroup resource", "name", roleGroup.Name, "namespace", roleGroup.Namespace, "postgresqlID", postgresqlID, "groupRole", groupRole)

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

	// List all RoleGroup resources across all namespaces in the cluster
	roleGroupList := &instancev1alpha1.RoleGroupList{}
	if err := v.Client.List(ctx, roleGroupList); err != nil {
		v.Log.Errorw("Failed to list RoleGroup resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check for duplicate postgresqlID + groupRole combination across the entire cluster
	for _, existingRoleGroup := range roleGroupList.Items {
		// Skip the current resource if this is an update
		if req.Operation == admissionv1.Update &&
			existingRoleGroup.Name == roleGroup.Name &&
			existingRoleGroup.Namespace == roleGroup.Namespace {
			continue
		}

		if existingRoleGroup.Spec.PostgresqlID == postgresqlID &&
			existingRoleGroup.Spec.GroupRole == groupRole {
			msg := fmt.Sprintf("RoleGroup with postgresqlID %s and groupRole %s already exists in namespace %s (instance: %s)",
				postgresqlID, groupRole, existingRoleGroup.Namespace, existingRoleGroup.Name)
			v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "groupRole", groupRole,
				"existing-namespace", existingRoleGroup.Namespace, "existing-name", existingRoleGroup.Name)
			return admission.Denied(msg)
		}
	}

	v.Log.Infow("Validation passed", "name", roleGroup.Name, "namespace", roleGroup.Namespace, "postgresqlID", postgresqlID, "groupRole", groupRole)
	return admission.Allowed("No duplicate postgresqlID and groupRole combination found in cluster")
}
