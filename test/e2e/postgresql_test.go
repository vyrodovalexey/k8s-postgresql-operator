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

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/test/testutils"
)

// TestE2E_PostgresqlCRD_Create tests creating a Postgresql CRD
func TestE2E_PostgresqlCRD_Create(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	name := testutils.GenerateTestName("pg")
	namespace := "default"

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: testutils.GenerateTestName("pg-id"),
				Address:      testConfig.PostgresHost,
				Port:         testConfig.PostgresPort,
				SSLMode:      testConfig.PostgresSSLMode,
			},
		},
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, postgresql)
	})

	// Create Postgresql CRD
	err := k8sClient.Create(ctx, postgresql)
	require.NoError(t, err)

	// Verify CRD was created
	created := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, created)
	require.NoError(t, err)
	assert.Equal(t, name, created.Name)
	assert.NotNil(t, created.Spec.ExternalInstance)
}

// TestE2E_PostgresqlCRD_Update tests updating a Postgresql CRD
func TestE2E_PostgresqlCRD_Update(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	name := testutils.GenerateTestName("pg")
	namespace := "default"

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: testutils.GenerateTestName("pg-id"),
				Address:      testConfig.PostgresHost,
				Port:         testConfig.PostgresPort,
				SSLMode:      "disable",
			},
		},
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, postgresql)
	})

	// Create Postgresql CRD
	err := k8sClient.Create(ctx, postgresql)
	require.NoError(t, err)

	// Update the CRD
	updated := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, updated)
	require.NoError(t, err)

	updated.Spec.ExternalInstance.SSLMode = "require"
	err = k8sClient.Update(ctx, updated)
	require.NoError(t, err)

	// Verify update
	final := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, final)
	require.NoError(t, err)
	assert.Equal(t, "require", final.Spec.ExternalInstance.SSLMode)
}

// TestE2E_PostgresqlCRD_Delete tests deleting a Postgresql CRD
func TestE2E_PostgresqlCRD_Delete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	name := testutils.GenerateTestName("pg")
	namespace := "default"

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: testutils.GenerateTestName("pg-id"),
				Address:      testConfig.PostgresHost,
				Port:         testConfig.PostgresPort,
				SSLMode:      testConfig.PostgresSSLMode,
			},
		},
	}

	// Create Postgresql CRD
	err := k8sClient.Create(ctx, postgresql)
	require.NoError(t, err)

	// Delete the CRD
	err = k8sClient.Delete(ctx, postgresql)
	require.NoError(t, err)

	// Verify deletion
	deleted := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deleted)
	assert.True(t, errors.IsNotFound(err))
}

// TestE2E_UserCRD_Create tests creating a User CRD
func TestE2E_UserCRD_Create(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// First create a Postgresql CRD
	pgName := testutils.GenerateTestName("pg")
	pgID := testutils.GenerateTestName("pg-id")
	namespace := "default"

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: pgID,
				Address:      testConfig.PostgresHost,
				Port:         testConfig.PostgresPort,
				SSLMode:      testConfig.PostgresSSLMode,
			},
		},
	}

	userName := testutils.GenerateTestName("user")
	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID:   pgID,
			Username:       testutils.SanitizeName(testutils.GenerateTestUsername()),
			UpdatePassword: false,
		},
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, user)
		_ = k8sClient.Delete(cleanupCtx, postgresql)
	})

	// Create Postgresql CRD
	err := k8sClient.Create(ctx, postgresql)
	require.NoError(t, err)

	// Create User CRD
	err = k8sClient.Create(ctx, user)
	require.NoError(t, err)

	// Verify User CRD was created
	created := &instancev1alpha1.User{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: userName, Namespace: namespace}, created)
	require.NoError(t, err)
	assert.Equal(t, userName, created.Name)
	assert.Equal(t, pgID, created.Spec.PostgresqlID)
}

// TestE2E_DatabaseCRD_Create tests creating a Database CRD
func TestE2E_DatabaseCRD_Create(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// First create a Postgresql CRD
	pgName := testutils.GenerateTestName("pg")
	pgID := testutils.GenerateTestName("pg-id")
	namespace := "default"

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: pgID,
				Address:      testConfig.PostgresHost,
				Port:         testConfig.PostgresPort,
				SSLMode:      testConfig.PostgresSSLMode,
			},
		},
	}

	dbName := testutils.GenerateTestName("db")
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID:  pgID,
			Database:      testutils.SanitizeName(testutils.GenerateTestDatabaseName()),
			Owner:         testConfig.PostgresUser,
			DeleteFromCRD: false,
		},
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, database)
		_ = k8sClient.Delete(cleanupCtx, postgresql)
	})

	// Create Postgresql CRD
	err := k8sClient.Create(ctx, postgresql)
	require.NoError(t, err)

	// Create Database CRD
	err = k8sClient.Create(ctx, database)
	require.NoError(t, err)

	// Verify Database CRD was created
	created := &instancev1alpha1.Database{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: dbName, Namespace: namespace}, created)
	require.NoError(t, err)
	assert.Equal(t, dbName, created.Name)
	assert.Equal(t, pgID, created.Spec.PostgresqlID)
}

// TestE2E_GrantCRD_Create tests creating a Grant CRD
func TestE2E_GrantCRD_Create(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	pgName := testutils.GenerateTestName("pg")
	pgID := testutils.GenerateTestName("pg-id")
	namespace := "default"

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: pgID,
				Address:      testConfig.PostgresHost,
				Port:         testConfig.PostgresPort,
				SSLMode:      testConfig.PostgresSSLMode,
			},
		},
	}

	grantName := testutils.GenerateTestName("grant")
	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      grantName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: pgID,
			Role:         testConfig.PostgresUser,
			Database:     "postgres",
			Grants: []instancev1alpha1.GrantItem{
				{
					Type:       instancev1alpha1.GrantTypeDatabase,
					Privileges: []string{"CONNECT"},
				},
			},
		},
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, grant)
		_ = k8sClient.Delete(cleanupCtx, postgresql)
	})

	// Create Postgresql CRD
	err := k8sClient.Create(ctx, postgresql)
	require.NoError(t, err)

	// Create Grant CRD
	err = k8sClient.Create(ctx, grant)
	require.NoError(t, err)

	// Verify Grant CRD was created
	created := &instancev1alpha1.Grant{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: grantName, Namespace: namespace}, created)
	require.NoError(t, err)
	assert.Equal(t, grantName, created.Name)
	assert.Equal(t, pgID, created.Spec.PostgresqlID)
}

// TestE2E_RoleGroupCRD_Create tests creating a RoleGroup CRD
func TestE2E_RoleGroupCRD_Create(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	pgName := testutils.GenerateTestName("pg")
	pgID := testutils.GenerateTestName("pg-id")
	namespace := "default"

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: pgID,
				Address:      testConfig.PostgresHost,
				Port:         testConfig.PostgresPort,
				SSLMode:      testConfig.PostgresSSLMode,
			},
		},
	}

	rgName := testutils.GenerateTestName("rg")
	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rgName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: pgID,
			GroupRole:    testutils.SanitizeName(testutils.GenerateTestRoleName()),
			MemberRoles:  []string{},
		},
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, roleGroup)
		_ = k8sClient.Delete(cleanupCtx, postgresql)
	})

	// Create Postgresql CRD
	err := k8sClient.Create(ctx, postgresql)
	require.NoError(t, err)

	// Create RoleGroup CRD
	err = k8sClient.Create(ctx, roleGroup)
	require.NoError(t, err)

	// Verify RoleGroup CRD was created
	created := &instancev1alpha1.RoleGroup{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: rgName, Namespace: namespace}, created)
	require.NoError(t, err)
	assert.Equal(t, rgName, created.Name)
	assert.Equal(t, pgID, created.Spec.PostgresqlID)
}

// TestE2E_SchemaCRD_Create tests creating a Schema CRD
func TestE2E_SchemaCRD_Create(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	pgName := testutils.GenerateTestName("pg")
	pgID := testutils.GenerateTestName("pg-id")
	namespace := "default"

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pgName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: pgID,
				Address:      testConfig.PostgresHost,
				Port:         testConfig.PostgresPort,
				SSLMode:      testConfig.PostgresSSLMode,
			},
		},
	}

	schemaName := testutils.GenerateTestName("schema")
	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      schemaName,
			Namespace: namespace,
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: pgID,
			Schema:       testutils.SanitizeName(testutils.GenerateTestSchemaName()),
			Owner:        testConfig.PostgresUser,
		},
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, schema)
		_ = k8sClient.Delete(cleanupCtx, postgresql)
	})

	// Create Postgresql CRD
	err := k8sClient.Create(ctx, postgresql)
	require.NoError(t, err)

	// Create Schema CRD
	err = k8sClient.Create(ctx, schema)
	require.NoError(t, err)

	// Verify Schema CRD was created
	created := &instancev1alpha1.Schema{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: schemaName, Namespace: namespace}, created)
	require.NoError(t, err)
	assert.Equal(t, schemaName, created.Name)
	assert.Equal(t, pgID, created.Spec.PostgresqlID)
}
