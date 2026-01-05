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

package controller

import (
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// getBaseReconcilerConfig creates a base reconciler config for testing
// Note: VaultClient is set to nil since we can't easily inject mocks into the actual *vault.Client type
// Tests that need Vault functionality should mock at a higher level or use integration tests
func getBaseReconcilerConfig(mockClient client.Client, mockVaultClient *MockVaultClient) BaseReconcilerConfig {
	logger := zap.NewNop().Sugar()
	return BaseReconcilerConfig{
		Client:                      mockClient,
		VaultClient:                 nil, // Set to nil for unit tests - Vault operations are tested separately
		Log:                         logger,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}
}

// createTestPostgresql creates a test PostgreSQL instance
func createTestPostgresql(name, namespace, postgresqlID, address string, port int32, connected bool) *instancev1alpha1.Postgresql {
	return &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: postgresqlID,
				Address:      address,
				Port:         port,
				SSLMode:      "require",
			},
		},
		Status: instancev1alpha1.PostgresqlStatus{
			Connected: connected,
		},
	}
}

// createTestUser creates a test User instance
func createTestUser(name, namespace, postgresqlID, username string, updatePassword bool) *instancev1alpha1.User {
	return &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   postgresqlID,
			Username:       username,
			UpdatePassword: updatePassword,
		},
	}
}

// createTestDatabase creates a test Database instance
func createTestDatabase(name, namespace, postgresqlID, databaseName, owner string, deleteFromCRD bool) *instancev1alpha1.Database {
	return &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID:  postgresqlID,
			Database:      databaseName,
			Owner:         owner,
			DeleteFromCRD: deleteFromCRD,
		},
	}
}
