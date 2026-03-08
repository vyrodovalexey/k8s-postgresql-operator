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

package webhook

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"time"
)

func TestRoleGroupValidator_Handle_NoPostgresqlID(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "postgresqlID is required")
}

func TestRoleGroupValidator_Handle_NoGroupRole(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: "pg-1",
			GroupRole:    "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "groupRole is required")
}

func TestRoleGroupValidator_Handle_DuplicateRoleGroup(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	postgresqlID := "pg-1"
	groupRole := "group1"

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: postgresqlID,
			GroupRole:    groupRole,
		},
	}

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
					},
				},
			},
		},
	}

	roleGroupList := &instancev1alpha1.RoleGroupList{
		Items: []instancev1alpha1.RoleGroup{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-rolegroup",
					Namespace: "other-namespace",
				},
				Spec: instancev1alpha1.RoleGroupSpec{
					PostgresqlID: postgresqlID,
					GroupRole:    groupRole,
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(roleGroupList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, postgresqlID)
	assert.Contains(t, response.Result.Message, groupRole)
}

func TestRoleGroupValidator_Handle_NoDuplicate(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	postgresqlID := "pg-1"
	groupRole := "group1"

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: postgresqlID,
			GroupRole:    groupRole,
		},
	}

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
					},
				},
			},
		},
	}

	roleGroupList := &instancev1alpha1.RoleGroupList{
		Items: []instancev1alpha1.RoleGroup{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-rolegroup",
					Namespace: "other-namespace",
				},
				Spec: instancev1alpha1.RoleGroupSpec{
					PostgresqlID: postgresqlID,
					GroupRole:    "different-group", // Different group role
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(roleGroupList, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
}

func TestRoleGroupValidator_Handle_PostgresqlNotFound(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: "pg-1",
			GroupRole:    "group1",
		},
	}

	postgresqlList := &instancev1alpha1.PostgresqlList{
		Items: []instancev1alpha1.Postgresql{}, // Empty list
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "does not exist")
}

func TestRoleGroupValidator_Handle_DecodeError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(400), response.Result.Code)
}

func TestRoleGroupValidator_Handle_VaultUnavailable(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	vaultClient := newUnhealthyVaultClient(t)

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 vaultClient,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Second,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: "pg-1",
			GroupRole:    "group1",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "Vault is not available")
}

func TestRoleGroupValidator_Handle_PostgresqlConnectionFailed(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	vaultClient := newHealthyVaultClient(t)

	postgresqlID := "pg-1"

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 vaultClient,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Second,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: postgresqlID,
			GroupRole:    "group1",
		},
	}

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
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "Cannot connect to PostgreSQL")
}

func TestRoleGroupValidator_Handle_RoleGroupListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	postgresqlID := "pg-1"
	groupRole := "group1"

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 1,
			PostgresqlConnectionTimeout: 1 * time.Second,
			VaultAvailabilityRetries:    1,
			VaultAvailabilityRetryDelay: 1 * time.Millisecond,
		},
	}

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: postgresqlID,
			GroupRole:    groupRole,
		},
	}

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
					},
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}

func TestRoleGroupValidator_Handle_UpdateSameResource(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	postgresqlID := "pg-1"
	groupRole := "group1"

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: postgresqlID,
			GroupRole:    groupRole,
		},
	}

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
					},
				},
			},
		},
	}

	// Same rolegroup in the list (update scenario)
	roleGroupList := &instancev1alpha1.RoleGroupList{
		Items: []instancev1alpha1.RoleGroup{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-rolegroup", // Same name
					Namespace: "default",        // Same namespace
				},
				Spec: instancev1alpha1.RoleGroupSpec{
					PostgresqlID: postgresqlID,
					GroupRole:    groupRole,
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(roleGroupList, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
}

func TestRoleGroupValidator_Handle_PostgresqlListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &RoleGroupValidator{
		BaseValidatorConfig: BaseValidatorConfig{
			Client:                      mockClient,
			Decoder:                     mockDecoder,
			Log:                         logger,
			VaultClient:                 nil,
			PostgresqlConnectionRetries: 3,
			PostgresqlConnectionTimeout: 10 * time.Second,
			VaultAvailabilityRetries:    3,
			VaultAvailabilityRetryDelay: 10 * time.Second,
		},
	}

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolegroup",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: "pg-1",
			GroupRole:    "group1",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)
	mockClient.On("List", mock.Anything, mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(403), response.Result.Code)
}
