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

// UserValidator validates User resources
type UserValidator struct {
	Client          client.Client
	Decoder         admission.Decoder
	Log             *zap.SugaredLogger
	ExcludeUserList []string
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

	v.Log.Infow("Validating User resource", "name", user.Name, "namespace", user.Namespace, "postgresqlID", postgresqlID, "username", username)

	// Check if username is in the Excludelist
	for _, excludelistedUser := range v.ExcludeUserList {
		if excludelistedUser == username {
			msg := fmt.Sprintf("Username %s is in the excludelist and cannot be created", username)
			v.Log.Infow("Validation denied", "reason", msg, "username", username)
			return admission.Denied(msg)
		}
	}

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

	// List all User resources across all namespaces in the cluster
	userList := &instancev1alpha1.UserList{}
	if err := v.Client.List(ctx, userList); err != nil {
		v.Log.Errorw("Failed to list User resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check for duplicate postgresqlID + username combination across the entire cluster
	for _, existingUser := range userList.Items {
		// Skip the current resource if this is an update
		if req.Operation == admissionv1.Update &&
			existingUser.Name == user.Name &&
			existingUser.Namespace == user.Namespace {
			continue
		}

		if existingUser.Spec.PostgresqlID == postgresqlID &&
			existingUser.Spec.Username == username {
			msg := fmt.Sprintf("User with postgresqlID %s and username %s already exists in namespace %s (instance: %s)",
				postgresqlID, username, existingUser.Namespace, existingUser.Name)
			v.Log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "username", username,
				"existing-namespace", existingUser.Namespace, "existing-name", existingUser.Name)
			return admission.Denied(msg)
		}
	}

	v.Log.Infow("Validation passed", "name", user.Name, "namespace", user.Namespace, "postgresqlID", postgresqlID, "username", username)
	return admission.Allowed("No duplicate postgresqlID and username combination found in cluster")
}
