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

const (
	testUsername = "user1"
)

func TestUserValidator_Handle_NoPostgresqlID(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "No postgresqlID specified")
}

func TestUserValidator_Handle_NoUsername(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "pg-1",
			Username:     "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "No username specified")
}

func TestUserValidator_Handle_ExcludedUser(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{"postgres", "admin"},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "pg-1",
			Username:     "postgres",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "excludelist")
}

func TestUserValidator_Handle_PostgresqlNotFound(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "pg-1",
			Username:     "user1",
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

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "does not exist")
}

func TestUserValidator_Handle_DuplicateUser(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	postgresqlID := "pg-1"
	username := testUsername

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: postgresqlID,
			Username:     username,
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

	userList := &instancev1alpha1.UserList{
		Items: []instancev1alpha1.User{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-user",
					Namespace: "other-namespace",
				},
				Spec: instancev1alpha1.UserSpec{
					PostgresqlID: postgresqlID,
					Username:     username,
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(userList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, postgresqlID)
	assert.Contains(t, response.Result.Message, username)
}

func TestUserValidator_Handle_NoDuplicate(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	postgresqlID := "pg-1"
	username := testUsername

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: postgresqlID,
			Username:     username,
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

	userList := &instancev1alpha1.UserList{
		Items: []instancev1alpha1.User{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-user",
					Namespace: "other-namespace",
				},
				Spec: instancev1alpha1.UserSpec{
					PostgresqlID: postgresqlID,
					Username:     "different-user", // Different username
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(userList, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
}

func TestUserValidator_Handle_UpdateSameResource(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	postgresqlID := "pg-1"
	username := testUsername

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: postgresqlID,
			Username:     username,
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

	// Same user in the list (update scenario)
	userList := &instancev1alpha1.UserList{
		Items: []instancev1alpha1.User{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-user", // Same name
					Namespace: "default",   // Same namespace
				},
				Spec: instancev1alpha1.UserSpec{
					PostgresqlID: postgresqlID,
					Username:     username,
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(userList, nil)

	response := validator.Handle(context.Background(), req)
	// Should be allowed because it's the same resource being updated
	assert.True(t, response.Allowed)
}

func TestUserValidator_Handle_DecodeError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(400), response.Result.Code)
}

func TestUserValidator_Handle_PostgresqlListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "pg-1",
			Username:     "user1",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}

func TestUserValidator_Handle_UserListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
		ExcludeUserList:             []string{},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "pg-1",
			Username:     "user1",
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
						PostgresqlID: "pg-1",
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

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.UserList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}
