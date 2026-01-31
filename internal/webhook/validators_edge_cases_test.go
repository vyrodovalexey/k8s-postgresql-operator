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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// TestDatabaseValidator_Handle_UpdateOperation tests update operation
func TestDatabaseValidator_Handle_UpdateOperation(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &DatabaseValidator{
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
	databaseName := "testdb"

	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID: postgresqlID,
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

	// Same database in the list (update scenario)
	databaseList := &instancev1alpha1.DatabaseList{
		Items: []instancev1alpha1.Database{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-db",
					Namespace: "default",
				},
				Spec: instancev1alpha1.DatabaseSpec{
					PostgresqlID: postgresqlID,
					Database:     databaseName,
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Database")).Return(database, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(databaseList, nil)

	response := validator.Handle(context.Background(), req)
	// Should be allowed because it's the same resource being updated
	assert.True(t, response.Allowed)
}

// TestDatabaseValidator_Handle_DatabaseListError tests database list error
func TestDatabaseValidator_Handle_DatabaseListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &DatabaseValidator{
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

	database := &instancev1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-db",
			Namespace: "default",
		},
		Spec: instancev1alpha1.DatabaseSpec{
			PostgresqlID: "pg-1",
			Database:     "testdb",
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

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Database")).Return(database, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}

// TestGrantValidator_Handle_UpdateOperation tests update operation
func TestGrantValidator_Handle_UpdateOperation(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
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
	role := "testrole"
	databaseName := "testdb"

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

	// Same grant in the list (update scenario)
	grantList := &instancev1alpha1.GrantList{
		Items: []instancev1alpha1.Grant{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-grant",
					Namespace: "default",
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
			Operation: admissionv1.Update,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(databaseList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(grantList, nil)

	response := validator.Handle(context.Background(), req)
	// Should be allowed because it's the same resource being updated
	assert.True(t, response.Allowed)
}

// TestGrantValidator_Handle_GrantListError tests grant list error
func TestGrantValidator_Handle_GrantListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &GrantValidator{
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
	role := "testrole"
	databaseName := "testdb"

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

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.DatabaseList"), mock.Anything).Return(databaseList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.GrantList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}

// TestRoleGroupValidator_Handle_UpdateOperation tests update operation
func TestRoleGroupValidator_Handle_UpdateOperation(t *testing.T) {
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
	groupRole := "testgroup"

	roleGroup := &instancev1alpha1.RoleGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rg",
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
					Name:      "test-rg",
					Namespace: "default",
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
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(roleGroupList, nil)

	response := validator.Handle(context.Background(), req)
	// Should be allowed because it's the same resource being updated
	assert.True(t, response.Allowed)
}

// TestRoleGroupValidator_Handle_RoleGroupListError tests rolegroup list error
func TestRoleGroupValidator_Handle_RoleGroupListError(t *testing.T) {
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
			Name:      "test-rg",
			Namespace: "default",
		},
		Spec: instancev1alpha1.RoleGroupSpec{
			PostgresqlID: "pg-1",
			GroupRole:    "testgroup",
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

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(roleGroup, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.RoleGroupList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}

// TestSchemaValidator_Handle_UpdateOperation tests update operation
func TestSchemaValidator_Handle_UpdateOperation(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
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
	schemaName := "testschema"

	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: postgresqlID,
			Schema:       schemaName,
			Owner:        "owner1",
			Database:     "testdb",
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

	// Same schema in the list (update scenario)
	schemaList := &instancev1alpha1.SchemaList{
		Items: []instancev1alpha1.Schema{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-schema",
					Namespace: "default",
				},
				Spec: instancev1alpha1.SchemaSpec{
					PostgresqlID: postgresqlID,
					Schema:       schemaName,
					Owner:        "owner1",
					Database:     "testdb",
				},
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Update,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(schemaList, nil)

	response := validator.Handle(context.Background(), req)
	// Should be allowed because it's the same resource being updated
	assert.True(t, response.Allowed)
}

// TestSchemaValidator_Handle_SchemaListError tests schema list error
func TestSchemaValidator_Handle_SchemaListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
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

	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: "pg-1",
			Schema:       "testschema",
			Owner:        "owner1",
			Database:     "testdb",
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

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}

// TestPostgresqlValidator_Handle_PostgresqlListError tests postgresql list error
func TestPostgresqlValidator_Handle_PostgresqlListError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &PostgresqlValidator{
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

	postgresql := &instancev1alpha1.Postgresql{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pg",
			Namespace: "default",
		},
		Spec: instancev1alpha1.PostgresqlSpec{
			ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{
				PostgresqlID: "pg-1",
			},
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Postgresql")).Return(postgresql, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(500), response.Result.Code)
}

// TestUserValidator_Handle_UserListError tests user list error
func TestUserValidator_Handle_UserListError_Comprehensive(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &UserValidator{
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
		ExcludeUserList: []string{},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-user",
			Namespace: "default",
		},
		Spec: instancev1alpha1.UserSpec{
			PostgresqlID: "pg-1",
			Username:     "testuser",
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
