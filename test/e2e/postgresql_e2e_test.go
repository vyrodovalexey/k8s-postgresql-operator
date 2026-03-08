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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func createTestNamespace(t *testing.T) string {
	t.Helper()

	nsName := fmt.Sprintf("e2e-test-%d", time.Now().UnixNano())
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}

	testCtx, testCancel := context.WithTimeout(ctx, 10*time.Second)
	defer testCancel()

	err := k8sClient.Create(testCtx, ns)
	require.NoError(t, err, "Creating test namespace should succeed")

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		// Delete all resources in namespace first
		_ = k8sClient.DeleteAllOf(cleanupCtx, &instancev1alpha1.Postgresql{}, client.InNamespace(nsName))
		_ = k8sClient.DeleteAllOf(cleanupCtx, &instancev1alpha1.Database{}, client.InNamespace(nsName))
		_ = k8sClient.DeleteAllOf(cleanupCtx, &instancev1alpha1.User{}, client.InNamespace(nsName))
		_ = k8sClient.DeleteAllOf(cleanupCtx, &instancev1alpha1.Grant{}, client.InNamespace(nsName))
		_ = k8sClient.DeleteAllOf(cleanupCtx, &instancev1alpha1.RoleGroup{}, client.InNamespace(nsName))
		_ = k8sClient.DeleteAllOf(cleanupCtx, &instancev1alpha1.Schema{}, client.InNamespace(nsName))

		// Delete namespace
		_ = k8sClient.Delete(cleanupCtx, ns)
	})

	return nsName
}

func TestE2E_PostgresqlCRDCreate(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-create",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: fmt.Sprintf("e2e-pg-create-%d", time.Now().UnixNano()),
				Address:      "postgres.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	// Create the Postgresql CRD
	err := k8sClient.Create(testCtx, postgresql)
	require.NoError(t, err, "Creating Postgresql CRD should succeed")

	// Verify it was created by fetching it
	fetched := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      postgresql.Name,
		Namespace: nsName,
	}, fetched)
	require.NoError(t, err, "Fetching created Postgresql CRD should succeed")

	assert.Equal(t, postgresql.Spec.ExternalInstance.PostgresqlID, fetched.Spec.ExternalInstance.PostgresqlID)
	assert.Equal(t, postgresql.Spec.ExternalInstance.Address, fetched.Spec.ExternalInstance.Address)
	assert.Equal(t, int32(5432), fetched.Spec.ExternalInstance.Port)
	assert.Equal(t, "disable", fetched.Spec.ExternalInstance.SSLMode)
}

func TestE2E_PostgresqlCRDUpdate(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-update",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: fmt.Sprintf("e2e-pg-update-%d", time.Now().UnixNano()),
				Address:      "postgres-old.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	// Create
	err := k8sClient.Create(testCtx, postgresql)
	require.NoError(t, err, "Creating Postgresql CRD should succeed")

	// Update the address
	fetched := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      postgresql.Name,
		Namespace: nsName,
	}, fetched)
	require.NoError(t, err)

	fetched.Spec.ExternalInstance.Address = "postgres-new.example.com"
	fetched.Spec.ExternalInstance.Port = 5433

	err = k8sClient.Update(testCtx, fetched)
	require.NoError(t, err, "Updating Postgresql CRD should succeed")

	// Verify the update
	updated := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      postgresql.Name,
		Namespace: nsName,
	}, updated)
	require.NoError(t, err)

	assert.Equal(t, "postgres-new.example.com", updated.Spec.ExternalInstance.Address)
	assert.Equal(t, int32(5433), updated.Spec.ExternalInstance.Port)
}

func TestE2E_PostgresqlCRDDelete(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-delete",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: fmt.Sprintf("e2e-pg-delete-%d", time.Now().UnixNano()),
				Address:      "postgres.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	// Create
	err := k8sClient.Create(testCtx, postgresql)
	require.NoError(t, err, "Creating Postgresql CRD should succeed")

	// Delete
	err = k8sClient.Delete(testCtx, postgresql)
	require.NoError(t, err, "Deleting Postgresql CRD should succeed")

	// Verify it's deleted
	deleted := &instancev1alpha1.Postgresql{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      postgresql.Name,
		Namespace: nsName,
	}, deleted)
	assert.True(t, errors.IsNotFound(err), "Postgresql CRD should be deleted")
}
