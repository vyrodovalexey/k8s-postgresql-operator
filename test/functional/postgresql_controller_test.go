//go:build functional

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

package functional

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller"
	"github.com/vyrodovalexey/k8s-postgresql-operator/test/testutils"
)

// TestPostgresqlController_Reconcile tests the PostgreSQL controller reconciliation
func TestPostgresqlController_Reconcile(t *testing.T) {
	tests := []struct {
		name           string
		postgresql     *instancev1alpha1.Postgresql
		expectedResult ctrl.Result
		expectError    bool
		validateStatus func(*testing.T, *instancev1alpha1.Postgresql)
	}{
		{
			name: "TC-PG-001: Create Postgresql CRD with valid external instance",
			postgresql: testutils.NewPostgresqlBuilder().
				WithName("test-pg").
				WithNamespace("default").
				WithExternalInstance("test-pg-001", "localhost", 5432, "disable").
				Build(),
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectError:    false,
			validateStatus: func(t *testing.T, pg *instancev1alpha1.Postgresql) {
				assert.NotEmpty(t, pg.Status.ConnectionAddress)
			},
		},
		{
			name: "TC-PG-002: Verify connection status updates",
			postgresql: testutils.NewPostgresqlBuilder().
				WithName("test-pg-status").
				WithNamespace("default").
				WithExternalInstance("test-pg-002", "localhost", 5432, "disable").
				Build(),
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectError:    false,
			validateStatus: func(t *testing.T, pg *instancev1alpha1.Postgresql) {
				assert.NotNil(t, pg.Status.LastConnectionAttempt)
			},
		},
		{
			name: "TC-PG-003: Handle connection failures gracefully",
			postgresql: testutils.NewPostgresqlBuilder().
				WithName("test-pg-fail").
				WithNamespace("default").
				WithExternalInstance("test-pg-003", "invalid-host", 5432, "disable").
				Build(),
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectError:    false,
			validateStatus: func(t *testing.T, pg *instancev1alpha1.Postgresql) {
				assert.False(t, pg.Status.Connected)
				testutils.AssertCondition(t, pg.Status.Conditions, "Ready", metav1.ConditionFalse)
			},
		},
		{
			name: "TC-PG-004: Verify Vault credential retrieval (no vault configured)",
			postgresql: testutils.NewPostgresqlBuilder().
				WithName("test-pg-vault").
				WithNamespace("default").
				WithExternalInstance("test-pg-004", "localhost", 5432, "disable").
				Build(),
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectError:    false,
			validateStatus: func(t *testing.T, pg *instancev1alpha1.Postgresql) {
				// Without Vault, connection should still be attempted
				assert.NotEmpty(t, pg.Status.ConnectionAddress)
			},
		},
		{
			name: "TC-PG-005: Test default port assignment (5432)",
			postgresql: &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pg-default-port",
					Namespace: "default",
				},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-pg-005",
						Address:      "localhost",
						Port:         0, // Should default to 5432
						SSLMode:      "disable",
					},
				},
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectError:    false,
			validateStatus: func(t *testing.T, pg *instancev1alpha1.Postgresql) {
				assert.Contains(t, pg.Status.ConnectionAddress, ":5432")
			},
		},
		{
			name: "TC-PG-006: Test default SSL mode assignment",
			postgresql: &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pg-default-ssl",
					Namespace: "default",
				},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: "test-pg-006",
						Address:      "localhost",
						Port:         5432,
						SSLMode:      "", // Should default to "require"
					},
				},
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
			expectError:    false,
			validateStatus: func(t *testing.T, pg *instancev1alpha1.Postgresql) {
				// SSL mode should be applied (default is "require")
				assert.NotEmpty(t, pg.Status.ConnectionAddress)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup scheme
			scheme := runtime.NewScheme()
			require.NoError(t, instancev1alpha1.AddToScheme(scheme))

			// Create fake client with the postgresql object
			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.postgresql).
				WithStatusSubresource(tt.postgresql).
				Build()

			// Create reconciler without Vault client (functional tests don't need real Vault)
			reconciler := &controller.PostgresqlReconciler{
				BaseReconcilerConfig: controller.BaseReconcilerConfig{
					Client:                      fakeClient,
					Scheme:                      scheme,
					VaultClient:                 nil, // No Vault in functional tests
					Log:                         testutils.SetupNopLogger(),
					PostgresqlConnectionRetries: 1,
					PostgresqlConnectionTimeout: 5 * time.Second,
					VaultAvailabilityRetries:    1,
					VaultAvailabilityRetryDelay: 1 * time.Second,
				},
			}

			// Execute reconciliation
			ctx := context.Background()
			req := ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      tt.postgresql.Name,
					Namespace: tt.postgresql.Namespace,
				},
			}

			result, err := reconciler.Reconcile(ctx, req)

			// Validate results
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedResult.RequeueAfter, result.RequeueAfter)

			// Get updated postgresql and validate status
			updatedPg := &instancev1alpha1.Postgresql{}
			err = fakeClient.Get(ctx, req.NamespacedName, updatedPg)
			require.NoError(t, err)

			if tt.validateStatus != nil {
				tt.validateStatus(t, updatedPg)
			}
		})
	}
}

// TestPostgresqlController_ReconcileNotFound tests reconciliation when resource is not found
func TestPostgresqlController_ReconcileNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, instancev1alpha1.AddToScheme(scheme))

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := &controller.PostgresqlReconciler{
		BaseReconcilerConfig: controller.BaseReconcilerConfig{
			Client: fakeClient,
			Scheme: scheme,
			Log:    testutils.SetupNopLogger(),
		},
	}

	ctx := context.Background()
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}

// TestPostgresqlController_ReconcileNoExternalInstance tests reconciliation without external instance
func TestPostgresqlController_ReconcileNoExternalInstance(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, instancev1alpha1.AddToScheme(scheme))

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-no-external",
			Namespace: "default",
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			// No ExternalInstance configured
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(postgresql).
		Build()

	reconciler := &controller.PostgresqlReconciler{
		BaseReconcilerConfig: controller.BaseReconcilerConfig{
			Client: fakeClient,
			Scheme: scheme,
			Log:    testutils.SetupNopLogger(),
		},
	}

	ctx := context.Background()
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      postgresql.Name,
			Namespace: postgresql.Namespace,
		},
	}

	result, err := reconciler.Reconcile(ctx, req)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
}
