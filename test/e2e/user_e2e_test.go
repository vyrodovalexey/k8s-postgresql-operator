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

func TestE2E_UserCRDCreate(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	suffix := time.Now().UnixNano()

	// Create Postgresql CRD first
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-for-user",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: fmt.Sprintf("e2e-user-pg-%d", suffix),
				Address:      "postgres.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	err := k8sClient.Create(testCtx, postgresql)
	require.NoError(t, err, "Creating parent Postgresql CRD should succeed")

	// Create User CRD
	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user-create",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.UserSpec{
			Username:       fmt.Sprintf("e2e_test_user_%d", suffix),
			UpdatePassword: false,
			PostgresqlID:   postgresql.Spec.ExternalInstance.PostgresqlID,
		},
	}

	err = k8sClient.Create(testCtx, user)
	require.NoError(t, err, "Creating User CRD should succeed")

	// Verify it was created
	fetched := &instancev1alpha1.User{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      user.Name,
		Namespace: nsName,
	}, fetched)
	require.NoError(t, err, "Fetching created User CRD should succeed")

	assert.Equal(t, user.Spec.Username, fetched.Spec.Username)
	assert.Equal(t, user.Spec.PostgresqlID, fetched.Spec.PostgresqlID)
	assert.False(t, fetched.Spec.UpdatePassword, "UpdatePassword should be false")
}

func TestE2E_UserCRDUpdatePassword(t *testing.T) {
	nsName := createTestNamespace(t)

	testCtx, testCancel := context.WithTimeout(ctx, 30*time.Second)
	defer testCancel()

	suffix := time.Now().UnixNano()

	// Create Postgresql CRD first
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg-for-user-update",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: fmt.Sprintf("e2e-user-update-pg-%d", suffix),
				Address:      "postgres.example.com",
				Port:         5432,
				SSLMode:      "disable",
			},
		},
	}

	err := k8sClient.Create(testCtx, postgresql)
	require.NoError(t, err, "Creating parent Postgresql CRD should succeed")

	// Create User CRD with updatePassword=false
	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user-update-pass",
			Namespace: nsName,
		},
		Spec: instancev1alpha1.UserSpec{
			Username:       fmt.Sprintf("e2e_update_user_%d", suffix),
			UpdatePassword: false,
			PostgresqlID:   postgresql.Spec.ExternalInstance.PostgresqlID,
		},
	}

	err = k8sClient.Create(testCtx, user)
	require.NoError(t, err, "Creating User CRD should succeed")

	// Update to set updatePassword=true
	fetched := &instancev1alpha1.User{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      user.Name,
		Namespace: nsName,
	}, fetched)
	require.NoError(t, err)

	fetched.Spec.UpdatePassword = true
	err = k8sClient.Update(testCtx, fetched)
	require.NoError(t, err, "Updating User CRD with updatePassword=true should succeed")

	// Verify the update
	updated := &instancev1alpha1.User{}
	err = k8sClient.Get(testCtx, types.NamespacedName{
		Name:      user.Name,
		Namespace: nsName,
	}, updated)
	require.NoError(t, err)

	assert.True(t, updated.Spec.UpdatePassword, "UpdatePassword should be true after update")
}
