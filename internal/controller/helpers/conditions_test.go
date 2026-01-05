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

package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

func TestUpdateCondition(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []metav1.Condition
		generation    int64
		conditionType string
		status        metav1.ConditionStatus
		reason        string
		message       string
		expectedLen   int
	}{
		{
			name:          "Add new condition to empty slice",
			conditions:    []metav1.Condition{},
			generation:    1,
			conditionType: "Ready",
			status:        metav1.ConditionTrue,
			reason:        "TestReason",
			message:       "Test message",
			expectedLen:   1,
		},
		{
			name: "Update existing condition",
			conditions: []metav1.Condition{
				{
					Type:   "Ready",
					Status: metav1.ConditionFalse,
					Reason: "OldReason",
				},
			},
			generation:    2,
			conditionType: "Ready",
			status:        metav1.ConditionTrue,
			reason:        "NewReason",
			message:       "New message",
			expectedLen:   1,
		},
		{
			name: "Add condition when other conditions exist",
			conditions: []metav1.Condition{
				{
					Type:   "Other",
					Status: metav1.ConditionTrue,
				},
			},
			generation:    1,
			conditionType: "Ready",
			status:        metav1.ConditionTrue,
			reason:        "TestReason",
			message:       "Test message",
			expectedLen:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UpdateCondition(
				tt.conditions, tt.generation, tt.conditionType, tt.status, tt.reason, tt.message)
			assert.Len(t, result, tt.expectedLen)

			found := false
			for _, c := range result {
				if c.Type == tt.conditionType {
					found = true
					assert.Equal(t, tt.status, c.Status)
					assert.Equal(t, tt.reason, c.Reason)
					assert.Equal(t, tt.message, c.Message)
					assert.Equal(t, tt.generation, c.ObservedGeneration)
					assert.NotZero(t, c.LastTransitionTime)
				}
			}
			assert.True(t, found, "Condition should be found in result")
		})
	}
}

func TestFindCondition(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []metav1.Condition
		conditionType string
		expected      bool
	}{
		{
			name: "Find existing condition",
			conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
				{Type: "Other", Status: metav1.ConditionFalse},
			},
			conditionType: "Ready",
			expected:      true,
		},
		{
			name: "Condition not found",
			conditions: []metav1.Condition{
				{Type: "Other", Status: metav1.ConditionTrue},
			},
			conditionType: "Ready",
			expected:      false,
		},
		{
			name:          "Empty conditions",
			conditions:    []metav1.Condition{},
			conditionType: "Ready",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindCondition(tt.conditions, tt.conditionType)
			if tt.expected {
				assert.NotNil(t, result)
				assert.Equal(t, tt.conditionType, result.Type)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestUpdateUserCondition(t *testing.T) {
	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: instancev1alpha1.UserStatus{
			Conditions: []metav1.Condition{},
		},
	}

	UpdateUserCondition(user, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, user.Status.Conditions, 1)
	assert.Equal(t, "Ready", user.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, user.Status.Conditions[0].Status)
}

func TestUpdateDatabaseCondition(t *testing.T) {
	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: instancev1alpha1.DatabaseStatus{
			Conditions: []metav1.Condition{},
		},
	}

	UpdateDatabaseCondition(database, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, database.Status.Conditions, 1)
	assert.Equal(t, "Ready", database.Status.Conditions[0].Type)
}

func TestUpdateGrantCondition(t *testing.T) {
	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: instancev1alpha1.GrantStatus{
			Conditions: []metav1.Condition{},
		},
	}

	UpdateGrantCondition(grant, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, grant.Status.Conditions, 1)
	assert.Equal(t, "Ready", grant.Status.Conditions[0].Type)
}

func TestUpdateRoleGroupCondition(t *testing.T) {
	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: instancev1alpha1.RoleGroupStatus{
			Conditions: []metav1.Condition{},
		},
	}

	UpdateRoleGroupCondition(roleGroup, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, roleGroup.Status.Conditions, 1)
	assert.Equal(t, "Ready", roleGroup.Status.Conditions[0].Type)
}

func TestUpdateSchemaCondition(t *testing.T) {
	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: instancev1alpha1.SchemaStatus{
			Conditions: []metav1.Condition{},
		},
	}

	UpdateSchemaCondition(schema, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, schema.Status.Conditions, 1)
	assert.Equal(t, "Ready", schema.Status.Conditions[0].Type)
}

func TestUpdatePostgresqlCondition(t *testing.T) {
	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Status: instancev1alpha1.PostgresqlStatus{
			Conditions: []metav1.Condition{},
		},
	}

	UpdatePostgresqlCondition(postgresql, "Ready", metav1.ConditionTrue, "TestReason", "Test message")
	assert.Len(t, postgresql.Status.Conditions, 1)
	assert.Equal(t, "Ready", postgresql.Status.Conditions[0].Type)
}
