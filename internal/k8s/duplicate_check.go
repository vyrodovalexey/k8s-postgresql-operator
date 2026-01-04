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

package k8s

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// DuplicateCheckResult contains information about a duplicate check
type DuplicateCheckResult struct {
	Found    bool
	Existing metav1.Object
	Resource string
	Fields   map[string]string
	Message  string
}

// CheckDuplicatePostgresqlID checks for duplicate postgresqlID in PostgreSQL resources
func CheckDuplicatePostgresqlID(
	ctx context.Context, k8sClient client.Client, currentName, currentNamespace, postgresqlID string,
	isUpdate bool) (*DuplicateCheckResult, error) {
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := k8sClient.List(ctx, postgresqlList); err != nil {
		return nil, fmt.Errorf("failed to list PostgreSQL instances: %w", err)
	}

	for i := range postgresqlList.Items {
		existing := &postgresqlList.Items[i]

		// Skip the current resource if this is an update
		if isUpdate && existing.Name == currentName && existing.Namespace == currentNamespace {
			continue
		}

		if existing.Spec.ExternalInstance != nil &&
			existing.Spec.ExternalInstance.PostgresqlID == postgresqlID {
			return &DuplicateCheckResult{
				Found:    true,
				Existing: existing,
				Resource: "PostgreSQL",
				Fields: map[string]string{
					"postgresqlID": postgresqlID,
				},
				Message: fmt.Sprintf("PostgreSQL instance with postgresqlID %s already exists in namespace %s (instance: %s)",
					postgresqlID, existing.Namespace, existing.Name),
			}, nil
		}
	}

	return &DuplicateCheckResult{Found: false}, nil
}

// CheckDuplicateUser checks for duplicate postgresqlID + username combination in User resources
func CheckDuplicateUser(
	ctx context.Context, k8sClient client.Client, currentName, currentNamespace, postgresqlID, username string,
	isUpdate bool) (*DuplicateCheckResult, error) {
	userList := &instancev1alpha1.UserList{}
	if err := k8sClient.List(ctx, userList); err != nil {
		return nil, fmt.Errorf("failed to list User instances: %w", err)
	}

	for i := range userList.Items {
		existing := &userList.Items[i]

		// Skip the current resource if this is an update
		if isUpdate && existing.Name == currentName && existing.Namespace == currentNamespace {
			continue
		}

		if existing.Spec.PostgresqlID == postgresqlID &&
			existing.Spec.Username == username {
			return &DuplicateCheckResult{
				Found:    true,
				Existing: existing,
				Resource: "User",
				Fields: map[string]string{
					"postgresqlID": postgresqlID,
					"username":     username,
				},
				Message: fmt.Sprintf("User with postgresqlID %s and username %s already exists in namespace %s (instance: %s)",
					postgresqlID, username, existing.Namespace, existing.Name),
			}, nil
		}
	}

	return &DuplicateCheckResult{Found: false}, nil
}

// CheckDuplicateDatabase checks for duplicate postgresqlID + database name combination in Database resources
func CheckDuplicateDatabase(
	ctx context.Context, k8sClient client.Client, currentName, currentNamespace, postgresqlID, databaseName string,
	isUpdate bool) (*DuplicateCheckResult, error) {
	databaseList := &instancev1alpha1.DatabaseList{}
	if err := k8sClient.List(ctx, databaseList); err != nil {
		return nil, fmt.Errorf("failed to list Database instances: %w", err)
	}

	for i := range databaseList.Items {
		existing := &databaseList.Items[i]

		// Skip the current resource if this is an update
		if isUpdate && existing.Name == currentName && existing.Namespace == currentNamespace {
			continue
		}

		if existing.Spec.PostgresqlID == postgresqlID &&
			existing.Spec.Database == databaseName {
			return &DuplicateCheckResult{
				Found:    true,
				Existing: existing,
				Resource: "Database",
				Fields: map[string]string{
					"postgresqlID": postgresqlID,
					"database":     databaseName,
				},
				Message: fmt.Sprintf(
					"Database with postgresqlID %s and database name %s already exists in namespace %s (instance: %s)",
					postgresqlID, databaseName, existing.Namespace, existing.Name),
			}, nil
		}
	}

	return &DuplicateCheckResult{Found: false}, nil
}

// CheckDuplicateGrant checks for duplicate postgresqlID + role + database combination in Grant resources
func CheckDuplicateGrant(
	ctx context.Context, k8sClient client.Client, currentName, currentNamespace, postgresqlID, role, databaseName string,
	isUpdate bool) (*DuplicateCheckResult, error) {
	grantList := &instancev1alpha1.GrantList{}
	if err := k8sClient.List(ctx, grantList); err != nil {
		return nil, fmt.Errorf("failed to list Grant instances: %w", err)
	}

	for i := range grantList.Items {
		existing := &grantList.Items[i]

		// Skip the current resource if this is an update
		if isUpdate && existing.Name == currentName && existing.Namespace == currentNamespace {
			continue
		}

		if existing.Spec.PostgresqlID == postgresqlID &&
			existing.Spec.Role == role &&
			existing.Spec.Database == databaseName {
			return &DuplicateCheckResult{
				Found:    true,
				Existing: existing,
				Resource: "Grant",
				Fields: map[string]string{
					"postgresqlID": postgresqlID,
					"role":         role,
					"database":     databaseName,
				},
				Message: fmt.Sprintf(
					"Grant with postgresqlID %s, role %s, and database %s already exists in namespace %s (instance: %s)",
					postgresqlID, role, databaseName, existing.Namespace, existing.Name),
			}, nil
		}
	}

	return &DuplicateCheckResult{Found: false}, nil
}

// CheckDuplicateRoleGroup checks for duplicate postgresqlID + groupRole combination in RoleGroup resources
func CheckDuplicateRoleGroup(
	ctx context.Context, k8sClient client.Client, currentName, currentNamespace, postgresqlID, groupRole string,
	isUpdate bool) (*DuplicateCheckResult, error) {
	roleGroupList := &instancev1alpha1.RoleGroupList{}
	if err := k8sClient.List(ctx, roleGroupList); err != nil {
		return nil, fmt.Errorf("failed to list RoleGroup instances: %w", err)
	}

	for i := range roleGroupList.Items {
		existing := &roleGroupList.Items[i]

		// Skip the current resource if this is an update
		if isUpdate && existing.Name == currentName && existing.Namespace == currentNamespace {
			continue
		}

		if existing.Spec.PostgresqlID == postgresqlID &&
			existing.Spec.GroupRole == groupRole {
			return &DuplicateCheckResult{
				Found:    true,
				Existing: existing,
				Resource: "RoleGroup",
				Fields: map[string]string{
					"postgresqlID": postgresqlID,
					"groupRole":    groupRole,
				},
				Message: fmt.Sprintf(
					"RoleGroup with postgresqlID %s and groupRole %s already exists in namespace %s (instance: %s)",
					postgresqlID, groupRole, existing.Namespace, existing.Name),
			}, nil
		}
	}

	return &DuplicateCheckResult{Found: false}, nil
}

// CheckDuplicateSchema checks for duplicate postgresqlID + schema combination in Schema resources
func CheckDuplicateSchema(
	ctx context.Context, k8sClient client.Client, currentName, currentNamespace, postgresqlID, schemaName string,
	isUpdate bool) (*DuplicateCheckResult, error) {
	schemaList := &instancev1alpha1.SchemaList{}
	if err := k8sClient.List(ctx, schemaList); err != nil {
		return nil, fmt.Errorf("failed to list Schema instances: %w", err)
	}

	for i := range schemaList.Items {
		existing := &schemaList.Items[i]

		// Skip the current resource if this is an update
		if isUpdate && existing.Name == currentName && existing.Namespace == currentNamespace {
			continue
		}

		if existing.Spec.PostgresqlID == postgresqlID &&
			existing.Spec.Schema == schemaName {
			return &DuplicateCheckResult{
				Found:    true,
				Existing: existing,
				Resource: "Schema",
				Fields: map[string]string{
					"postgresqlID": postgresqlID,
					"schema":       schemaName,
				},
				Message: fmt.Sprintf("Schema with postgresqlID %s and schema %s already exists in namespace %s (instance: %s)",
					postgresqlID, schemaName, existing.Namespace, existing.Name),
			}, nil
		}
	}

	return &DuplicateCheckResult{Found: false}, nil
}
