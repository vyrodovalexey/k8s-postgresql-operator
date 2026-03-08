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

func TestE2E_DatabaseCRDCreate(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	suffix := time.Now().UnixNano()

	// Create Postgresql CRD first (parent resource)
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-for-db",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: fmt.Sprintf("e2e-db-pg-%d", suffix),
				Address:      "postgres.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	err := k8sClient.Create(testCtx, postgresql)
	require.NoError(t, err, "Creating parent Postgresql CRD should succeed")

	// Create Database CRD
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-database-create",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.DatabaseSpec{
			Database:     fmt.Sprintf("e2e_test_db_%d", suffix),
			Owner:        "testowner",
			Schema:       "public",
			PostgresqlID: postgresql.Spec.ExternalInstance.PostgresqlID,
		},
	}

	err = k8sClient.Create(testCtx, database)
	require.NoError(t, err, "Creating Database CRD should succeed")

	// Verify it was created
	fetched := &instancev1alpha1.Database{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      database.Name,
		Namespace: nsName,
	}, fetched)
	require.NoError(t, err, "Fetching created Database CRD should succeed")

	assert.Equal(t, database.Spec.Database, fetched.Spec.Database)
	assert.Equal(t, database.Spec.Owner, fetched.Spec.Owner)
	assert.Equal(t, database.Spec.PostgresqlID, fetched.Spec.PostgresqlID)
	assert.Equal(t, "public", fetched.Spec.Schema)
	assert.False(t, fetched.Spec.DeleteFromCRD, "DeleteFromCRD should default to false")
}

func TestE2E_DatabaseCRDWithFinalizer(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	suffix := time.Now().UnixNano()

	// Create Postgresql CRD first
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-for-db-finalizer",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: fmt.Sprintf("e2e-db-finalizer-pg-%d", suffix),
				Address:      "postgres.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	err := k8sClient.Create(testCtx, postgresql)
	require.NoError(t, err, "Creating parent Postgresql CRD should succeed")

	// Create Database CRD with deleteFromCRD=true
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-database-finalizer",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.DatabaseSpec{
			Database:      fmt.Sprintf("e2e_finalizer_db_%d", suffix),
			Owner:         "testowner",
			Schema:        "app_schema",
			PostgresqlID:  postgresql.Spec.ExternalInstance.PostgresqlID,
			DeleteFromCRD: true,
		},
	}

	err = k8sClient.Create(testCtx, database)
	require.NoError(t, err, "Creating Database CRD with deleteFromCRD should succeed")

	// Verify it was created with deleteFromCRD=true
	fetched := &instancev1alpha1.Database{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      database.Name,
		Namespace: nsName,
	}, fetched)
	require.NoError(t, err, "Fetching created Database CRD should succeed")

	assert.True(t, fetched.Spec.DeleteFromCRD, "DeleteFromCRD should be true")
	assert.Equal(t, "app_schema", fetched.Spec.Schema)
}

func TestE2E_DatabaseCRDWithTemplate(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	suffix := time.Now().UnixNano()

	// Create Postgresql CRD first
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-for-db-template",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: fmt.Sprintf("e2e-db-template-pg-%d", suffix),
				Address:      "postgres.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	err := k8sClient.Create(testCtx, postgresql)
	require.NoError(t, err, "Creating parent Postgresql CRD should succeed")

	// Create Database CRD with template
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-database-template",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.DatabaseSpec{
			Database:     fmt.Sprintf("e2e_template_db_%d", suffix),
			Owner:        "testowner",
			PostgresqlID: postgresql.Spec.ExternalInstance.PostgresqlID,
			DBTemplate:   "template1",
		},
	}

	err = k8sClient.Create(testCtx, database)
	require.NoError(t, err, "Creating Database CRD with template should succeed")

	// Verify it was created with template
	fetched := &instancev1alpha1.Database{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      database.Name,
		Namespace: nsName,
	}, fetched)
	require.NoError(t, err, "Fetching created Database CRD should succeed")

	assert.Equal(t, "template1", fetched.Spec.DBTemplate)
}
