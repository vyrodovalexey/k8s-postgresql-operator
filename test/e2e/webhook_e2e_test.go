//go:build e2e

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

package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// TestE2E_WebhookDuplicatePostgresqlID tests that two Postgresql CRDs with the same
// postgresqlID can be created in envtest (without webhooks running).
// When webhooks are configured, this would be rejected.
// This test validates the CRD structure and that the postgresqlID field is properly stored.
func TestE2E_WebhookDuplicatePostgresqlID(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	suffix := time.Now().UnixNano()
	sharedPostgresqlID := fmt.Sprintf("e2e-duplicate-pg-%d", suffix)

	// Create first Postgresql CRD
	pg1 := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-dup-1",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: sharedPostgresqlID,
				Address:      "postgres1.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	err := k8sClient.Create(testCtx, pg1)
	require.NoError(t, err, "Creating first Postgresql CRD should succeed")

	// Create second Postgresql CRD with the same postgresqlID
	// In envtest without webhooks, this will succeed (no validation webhook running)
	pg2 := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-dup-2",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: sharedPostgresqlID,
				Address:      "postgres2.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	// Without webhooks, both CRDs can be created
	// This test validates the CRD structure is correct
	err = k8sClient.Create(testCtx, pg2)
	require.NoError(t, err, "Creating second Postgresql CRD should succeed in envtest (no webhooks)")

	// Verify both exist with the same postgresqlID
	fetched1 := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      pg1.Name,
		Namespace: nsName,
	}, fetched1)
	require.NoError(t, err)

	fetched2 := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      pg2.Name,
		Namespace: nsName,
	}, fetched2)
	require.NoError(t, err)

	assert.Equal(t, sharedPostgresqlID, fetched1.Spec.ExternalInstance.PostgresqlID)
	assert.Equal(t, sharedPostgresqlID, fetched2.Spec.ExternalInstance.PostgresqlID)
	assert.NotEqual(t, fetched1.Spec.ExternalInstance.Address, fetched2.Spec.ExternalInstance.Address,
		"The two CRDs should have different addresses despite same postgresqlID")
}

// TestE2E_CRDFieldValidation tests that CRD field validation works correctly.
func TestE2E_CRDFieldValidation(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	suffix := time.Now().UnixNano()

	t.Run("GrantCRDWithAllGrantTypes", func(t *testing.T) {
		// Create a Grant CRD with various grant types to validate the enum
		grant := &instancev1alpha1.Grant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-grant-types",
				Namespace: nsName,
			},
			Spec: instancev1alpha1.GrantSpec{
				Role:         "testrole",
				Database:     "testdb",
				PostgresqlID: fmt.Sprintf("e2e-grant-pg-%d", suffix),
				Grants: []instancev1alpha1.GrantItem{
					{
						Type:       instancev1alpha1.GrantTypeDatabase,
						Privileges: []string{"CONNECT"},
					},
					{
						Type:       instancev1alpha1.GrantTypeSchema,
						Schema:     "public",
						Privileges: []string{"USAGE"},
					},
					{
						Type:       instancev1alpha1.GrantTypeTable,
						Schema:     "public",
						Table:      "users",
						Privileges: []string{"SELECT", "INSERT"},
					},
					{
						Type:       instancev1alpha1.GrantTypeSequence,
						Schema:     "public",
						Sequence:   "users_id_seq",
						Privileges: []string{"USAGE"},
					},
					{
						Type:       instancev1alpha1.GrantTypeAllTables,
						Schema:     "public",
						Privileges: []string{"SELECT"},
					},
					{
						Type:       instancev1alpha1.GrantTypeAllSequences,
						Schema:     "public",
						Privileges: []string{"USAGE"},
					},
				},
				DefaultPrivileges: []instancev1alpha1.DefaultPrivilegeItem{
					{
						ObjectType: "tables",
						Schema:     "public",
						Privileges: []string{"SELECT"},
					},
					{
						ObjectType: "sequences",
						Schema:     "public",
						Privileges: []string{"USAGE"},
					},
				},
			},
		}

		err := k8sClient.Create(testCtx, grant)
		require.NoError(t, err, "Creating Grant CRD with all grant types should succeed")

		// Verify it was created correctly
		fetched := &instancev1alpha1.Grant{}
		err = k8sClient.Get(testCtx, types.NamespacedName{
			Name:      grant.Name,
			Namespace: nsName,
		}, fetched)
		require.NoError(t, err)

		assert.Len(t, fetched.Spec.Grants, 6, "Should have 6 grant items")
		assert.Len(t, fetched.Spec.DefaultPrivileges, 2, "Should have 2 default privilege items")
	})

	t.Run("RoleGroupCRD", func(t *testing.T) {
		roleGroup := &instancev1alpha1.RoleGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-rolegroup",
				Namespace: nsName,
			},
			Spec: instancev1alpha1.RoleGroupSpec{
				GroupRole:    "app_readers",
				MemberRoles:  []string{"user1", "user2", "user3"},
				PostgresqlID: fmt.Sprintf("e2e-rg-pg-%d", suffix),
			},
		}

		err := k8sClient.Create(testCtx, roleGroup)
		require.NoError(t, err, "Creating RoleGroup CRD should succeed")

		fetched := &instancev1alpha1.RoleGroup{}
		err = k8sClient.Get(testCtx, types.NamespacedName{
			Name:      roleGroup.Name,
			Namespace: nsName,
		}, fetched)
		require.NoError(t, err)

		assert.Equal(t, "app_readers", fetched.Spec.GroupRole)
		assert.Len(t, fetched.Spec.MemberRoles, 3)
	})

	t.Run("SchemaCRD", func(t *testing.T) {
		schema := &instancev1alpha1.Schema{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-schema",
				Namespace: nsName,
			},
			Spec: instancev1alpha1.SchemaSpec{
				Schema:       "app_schema",
				Owner:        "app_owner",
				PostgresqlID: fmt.Sprintf("e2e-schema-pg-%d", suffix),
			},
		}

		err := k8sClient.Create(testCtx, schema)
		require.NoError(t, err, "Creating Schema CRD should succeed")

		fetched := &instancev1alpha1.Schema{}
		err = k8sClient.Get(testCtx, types.NamespacedName{
			Name:      schema.Name,
			Namespace: nsName,
		}, fetched)
		require.NoError(t, err)

		assert.Equal(t, "app_schema", fetched.Spec.Schema)
		assert.Equal(t, "app_owner", fetched.Spec.Owner)
	})
}
