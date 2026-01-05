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

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FindPostgresqlByID finds a PostgreSQL instance by its PostgresqlID across all namespaces
func FindPostgresqlByID(
	ctx context.Context, k8sClient client.Client, postgresqlID string) (*instancev1alpha1.Postgresql, error) {
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := k8sClient.List(ctx, postgresqlList); err != nil {
		return nil, fmt.Errorf("failed to list PostgreSQL instances: %w", err)
	}

	for i := range postgresqlList.Items {
		if postgresqlList.Items[i].Spec.ExternalInstance != nil &&
			postgresqlList.Items[i].Spec.ExternalInstance.PostgresqlID == postgresqlID {
			return &postgresqlList.Items[i], nil
		}
	}

	return nil, fmt.Errorf("PostgreSQL instance with ID %s not found", postgresqlID)
}

// FindDatabaseByPostgresqlIDAndName finds a Database instance by PostgresqlID and database name across all namespaces
func FindDatabaseByPostgresqlIDAndName(
	ctx context.Context, k8sClient client.Client, postgresqlID, databaseName string) (*instancev1alpha1.Database, error) {
	databaseList := &instancev1alpha1.DatabaseList{}
	if err := k8sClient.List(ctx, databaseList); err != nil {
		return nil, fmt.Errorf("failed to list Database instances: %w", err)
	}

	for i := range databaseList.Items {
		if databaseList.Items[i].Spec.PostgresqlID == postgresqlID &&
			databaseList.Items[i].Spec.Database == databaseName {
			return &databaseList.Items[i], nil
		}
	}

	return nil, fmt.Errorf("database %s with postgresqlID %s not found", databaseName, postgresqlID)
}

// DatabaseExistsByPostgresqlIDAndName checks if a Database exists by PostgresqlID and database name
// across all namespaces
func DatabaseExistsByPostgresqlIDAndName(
	ctx context.Context, k8sClient client.Client, postgresqlID, databaseName string) (bool, error) {
	databaseList := &instancev1alpha1.DatabaseList{}
	if err := k8sClient.List(ctx, databaseList); err != nil {
		return false, fmt.Errorf("failed to list Database instances: %w", err)
	}

	for _, database := range databaseList.Items {
		if database.Spec.PostgresqlID == postgresqlID &&
			database.Spec.Database == databaseName {
			return true, nil
		}
	}

	return false, nil
}
