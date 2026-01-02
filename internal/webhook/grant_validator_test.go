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

func TestGrantValidator_Handle_NoPostgresqlID(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "No postgresqlID specified")
}

func TestGrantValidator_Handle_NoRole(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: "pg-1",
			Role:         "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "No role specified")
}

func TestGrantValidator_Handle_NoDatabase(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: "pg-1",
			Role:         "role1",
			Database:     "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "No database specified")
}

func TestGrantValidator_Handle_DuplicateGrant(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	postgresqlID := "pg-1"
	role := "role1"
	databaseName := "db1"

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: postgresqlID,
			Role:         role,
			Database:     databaseName,
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

	grantList := &instancev1alpha1.GrantList{
		Items: []instancev1alpha1.Grant{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-grant",
					Namespace: "other-namespace",
				},
				Spec: instancev1alpha1.GrantSpec{
					PostgresqlID: postgresqlID,
					Role:         role,
					Database:     databaseName,
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: postgresqlID,
					Database:     databaseName,
				},
			},
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(databaseList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(grantList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, postgresqlID)
	assert.Contains(t, response.Result.Message, role)
	assert.Contains(t, response.Result.Message, databaseName)
}

func TestGrantValidator_Handle_NoDuplicate(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	postgresqlID := "pg-1"
	role := "role1"
	databaseName := "db1"

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: postgresqlID,
			Role:         role,
			Database:     databaseName,
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

	grantList := &instancev1alpha1.GrantList{
		Items: []instancev1alpha1.Grant{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-grant",
					Namespace: "other-namespace",
				},
				Spec: instancev1alpha1.GrantSpec{
					PostgresqlID: postgresqlID,
					Role:         "different-role", // Different role
					Database:     databaseName,
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "db1",
					Namespace: "default",
				},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: postgresqlID,
					Database:     databaseName,
				},
			},
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(databaseList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(grantList, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
}

func TestGrantValidator_Handle_PostgresqlNotFound(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: "pg-1",
			Role:         "role1",
			Database:     "db1",
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

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "does not exist")
}

func TestGrantValidator_Handle_DatabaseNotFound(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	postgresqlID := "pg-1"
	role := "role1"
	databaseName := "db1"

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: postgresqlID,
			Role:         role,
			Database:     databaseName,
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

	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{}, // Empty list - database doesn't exist
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(databaseList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "does not exist")
	assert.Contains(t, response.Result.Message, databaseName)
	assert.Contains(t, response.Result.Message, postgresqlID)
}

func TestGrantValidator_Handle_DatabaseListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	postgresqlID := "pg-1"
	role := "role1"
	databaseName := "db1"

	grant := &instancev1alpha1.Grant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-grant",
			Namespace: "default",
		},
		Spec: instancev1alpha1.GrantSpec{
			PostgresqlID: postgresqlID,
			Role:         role,
			Database:     databaseName,
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

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}

func TestGrantValidator_Handle_DecodeError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
		Client:                      mockClient,
		Decoder:                     mockDecoder,
		Log:                         logger,
		VaultClient:                 nil,
		PostgresqlConnectionRetries: 3,
		PostgresqlConnectionTimeout: 10 * time.Second,
		VaultAvailabilityRetries:    3,
		VaultAvailabilityRetryDelay: 10 * time.Second,
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(400), response.Result.Code)
}
