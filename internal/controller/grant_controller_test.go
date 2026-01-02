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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestGrantReconciler_Reconcile_NotFound(t *testing.T) {
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	reconciler := &GrantReconciler{
		Client: mockClient,
		Log:    logger,
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-grant",
			Namespace: "default",
		},
	}

	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.NewNotFound(schema.GroupResource{}, "grant"))

	result, err := reconciler.Reconcile(context.Background(), req)

	assert.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)
	mockClient.AssertExpectations(t)
}

func TestGrantReconciler_FindPostgresqlByID_Success(t *testing.T) {
	mockClient := new(MockControllerClient)
	logger := zap.NewNop().Sugar()

	reconciler := &GrantReconciler{
		Client: mockClient,
		Log:    logger,
	}

	postgresqlID := "test-id-123"
	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pg1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
						PostgresqlID: postgresqlID,
						Address:      "localhost",
						Port:         5432,
					},
				},
				Status: instancev1alpha1.PostgresqlStatus{
					Connected: true,
				},
			},
		},
	}

	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		list := args.Get(1).(*instancev1alpha1.PostgresqlList)
		*list = *postgresqlList
	})

	result, err := reconciler.findPostgresqlByID(context.Background(), postgresqlID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, postgresqlID, result.Spec.ExternalInstance.PostgresqlID)
	mockClient.AssertExpectations(t)
}

func TestUpdateGrantCondition(t *testing.T) {
	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test",
			Generation: 1,
		},
		Status: instancev1alpha1.GrantStatus{
			Conditions: []metav1.Condition{},
		},
	}

	// Test adding new condition
	updateGrantCondition(grant, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, grant.Status.Conditions, 1)
	assert.Equal(t, "Ready", grant.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, grant.Status.Conditions[0].Status)
	assert.Equal(t, "TestReason", grant.Status.Conditions[0].Reason)
	assert.Equal(t, "Test message", grant.Status.Conditions[0].Message)

	// Test updating existing condition
	updateGrantCondition(grant, "Ready", metav1.ConditionFalse, "NewReason", "New message")
	assert.Len(t, grant.Status.Conditions, 1)
	assert.Equal(t, "Ready", grant.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionFalse, grant.Status.Conditions[0].Status)
	assert.Equal(t, "NewReason", grant.Status.Conditions[0].Reason)
	assert.Equal(t, "New message", grant.Status.Conditions[0].Message)
}
