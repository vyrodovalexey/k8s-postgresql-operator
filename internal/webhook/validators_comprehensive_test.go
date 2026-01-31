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

// TestUserValidator_Handle_TableDriven tests UserValidator.Handle with table-driven tests
func TestUserValidator_Handle_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		user            *instancev1alpha1.User
		operation       admissionv1.Operation
		excludeUserList []string
		setupMocks      func(*MockWebhookClient, *MockDecoder, admission.Request, *instancev1alpha1.User)
		expectAllowed   bool
		expectCode      int32
		expectMessage   string
	}{
		{
			name: "allowed - no postgresqlID",
			user: &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec:       instancev1alpha1.UserSpec{PostgresqlID: "", Username: "testuser"},
			},
			operation:       admissionv1.Create,
			excludeUserList: []string{},
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, user *instancev1alpha1.User) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
			},
			expectAllowed: true,
			expectMessage: "No postgresqlID specified",
		},
		{
			name: "allowed - no username",
			user: &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec:       instancev1alpha1.UserSpec{PostgresqlID: "pg-1", Username: ""},
			},
			operation:       admissionv1.Create,
			excludeUserList: []string{},
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, user *instancev1alpha1.User) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
			},
			expectAllowed: true,
			expectMessage: "No username specified",
		},
		{
			name: "denied - excluded user",
			user: &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec:       instancev1alpha1.UserSpec{PostgresqlID: "pg-1", Username: "postgres"},
			},
			operation:       admissionv1.Create,
			excludeUserList: []string{"postgres", "admin"},
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, user *instancev1alpha1.User) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
			},
			expectAllowed: false,
			expectMessage: "excludelist",
		},
		{
			name: "denied - postgresql not found",
			user: &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec:       instancev1alpha1.UserSpec{PostgresqlID: "pg-1", Username: "testuser"},
			},
			operation:       admissionv1.Create,
			excludeUserList: []string{},
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, user *instancev1alpha1.User) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
				mc.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).
					Return(&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)
			},
			expectAllowed: false,
			expectMessage: "does not exist",
		},
		{
			name:            "error - decode error",
			user:            nil,
			operation:       admissionv1.Create,
			excludeUserList: []string{},
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, user *instancev1alpha1.User) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(nil, assert.AnError)
			},
			expectAllowed: false,
			expectCode:    400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				ExcludeUserList: tt.excludeUserList,
			}

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: tt.operation,
				},
			}

			tt.setupMocks(mockClient, mockDecoder, req, tt.user)

			response := validator.Handle(context.Background(), req)

			assert.Equal(t, tt.expectAllowed, response.Allowed)
			if tt.expectCode != 0 {
				assert.Equal(t, tt.expectCode, response.Result.Code)
			}
			if tt.expectMessage != "" {
				assert.Contains(t, response.Result.Message, tt.expectMessage)
			}
		})
	}
}

// TestDatabaseValidator_Handle_TableDriven tests DatabaseValidator.Handle with table-driven tests
func TestDatabaseValidator_Handle_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		database      *instancev1alpha1.Database
		operation     admissionv1.Operation
		setupMocks    func(*MockWebhookClient, *MockDecoder, admission.Request, *instancev1alpha1.Database)
		expectAllowed bool
		expectCode    int32
		expectMessage string
	}{
		{
			name: "allowed - no postgresqlID",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "test-db", Namespace: "default"},
				Spec:       instancev1alpha1.DatabaseSpec{PostgresqlID: "", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, db *instancev1alpha1.Database) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Database")).Return(db, nil)
			},
			expectAllowed: true,
			expectMessage: "No postgresqlID specified",
		},
		{
			name: "allowed - no database name",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "test-db", Namespace: "default"},
				Spec:       instancev1alpha1.DatabaseSpec{PostgresqlID: "pg-1", Database: ""},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, db *instancev1alpha1.Database) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Database")).Return(db, nil)
			},
			expectAllowed: true,
			expectMessage: "No database name specified",
		},
		{
			name: "denied - postgresql not found",
			database: &instancev1alpha1.Database{
				ObjectMeta: metav1.ObjectMeta{Name: "test-db", Namespace: "default"},
				Spec:       instancev1alpha1.DatabaseSpec{PostgresqlID: "pg-1", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, db *instancev1alpha1.Database) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Database")).Return(db, nil)
				mc.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).
					Return(&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)
			},
			expectAllowed: false,
			expectMessage: "does not exist",
		},
		{
			name:      "error - decode error",
			database:  nil,
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, db *instancev1alpha1.Database) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Database")).Return(nil, assert.AnError)
			},
			expectAllowed: false,
			expectCode:    400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: tt.operation,
				},
			}

			tt.setupMocks(mockClient, mockDecoder, req, tt.database)

			response := validator.Handle(context.Background(), req)

			assert.Equal(t, tt.expectAllowed, response.Allowed)
			if tt.expectCode != 0 {
				assert.Equal(t, tt.expectCode, response.Result.Code)
			}
			if tt.expectMessage != "" {
				assert.Contains(t, response.Result.Message, tt.expectMessage)
			}
		})
	}
}

// TestGrantValidator_Handle_TableDriven tests GrantValidator.Handle with table-driven tests
func TestGrantValidator_Handle_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		grant         *instancev1alpha1.Grant
		operation     admissionv1.Operation
		setupMocks    func(*MockWebhookClient, *MockDecoder, admission.Request, *instancev1alpha1.Grant)
		expectAllowed bool
		expectCode    int32
		expectMessage string
	}{
		{
			name: "allowed - no postgresqlID",
			grant: &instancev1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{Name: "test-grant", Namespace: "default"},
				Spec:       instancev1alpha1.GrantSpec{PostgresqlID: "", Role: "testrole", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, grant *instancev1alpha1.Grant) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
			},
			expectAllowed: true,
			expectMessage: "No postgresqlID specified",
		},
		{
			name: "allowed - no role",
			grant: &instancev1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{Name: "test-grant", Namespace: "default"},
				Spec:       instancev1alpha1.GrantSpec{PostgresqlID: "pg-1", Role: "", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, grant *instancev1alpha1.Grant) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
			},
			expectAllowed: true,
			expectMessage: "No role specified",
		},
		{
			name: "allowed - no database",
			grant: &instancev1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{Name: "test-grant", Namespace: "default"},
				Spec:       instancev1alpha1.GrantSpec{PostgresqlID: "pg-1", Role: "testrole", Database: ""},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, grant *instancev1alpha1.Grant) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
			},
			expectAllowed: true,
			expectMessage: "No database specified",
		},
		{
			name: "denied - postgresql not found",
			grant: &instancev1alpha1.Grant{
				ObjectMeta: metav1.ObjectMeta{Name: "test-grant", Namespace: "default"},
				Spec:       instancev1alpha1.GrantSpec{PostgresqlID: "pg-1", Role: "testrole", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, grant *instancev1alpha1.Grant) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(grant, nil)
				mc.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).
					Return(&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)
			},
			expectAllowed: false,
			expectMessage: "does not exist",
		},
		{
			name:      "error - decode error",
			grant:     nil,
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, grant *instancev1alpha1.Grant) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Grant")).Return(nil, assert.AnError)
			},
			expectAllowed: false,
			expectCode:    400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: tt.operation,
				},
			}

			tt.setupMocks(mockClient, mockDecoder, req, tt.grant)

			response := validator.Handle(context.Background(), req)

			assert.Equal(t, tt.expectAllowed, response.Allowed)
			if tt.expectCode != 0 {
				assert.Equal(t, tt.expectCode, response.Result.Code)
			}
			if tt.expectMessage != "" {
				assert.Contains(t, response.Result.Message, tt.expectMessage)
			}
		})
	}
}

// TestRoleGroupValidator_Handle_TableDriven tests RoleGroupValidator.Handle with table-driven tests
func TestRoleGroupValidator_Handle_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		roleGroup     *instancev1alpha1.RoleGroup
		operation     admissionv1.Operation
		setupMocks    func(*MockWebhookClient, *MockDecoder, admission.Request, *instancev1alpha1.RoleGroup)
		expectAllowed bool
		expectCode    int32
		expectMessage string
	}{
		{
			name: "allowed - no postgresqlID",
			roleGroup: &instancev1alpha1.RoleGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rg", Namespace: "default"},
				Spec:       instancev1alpha1.RoleGroupSpec{PostgresqlID: "", GroupRole: "testgroup"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, rg *instancev1alpha1.RoleGroup) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(rg, nil)
			},
			expectAllowed: true,
			expectMessage: "No postgresqlID specified",
		},
		{
			name: "allowed - no groupRole",
			roleGroup: &instancev1alpha1.RoleGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rg", Namespace: "default"},
				Spec:       instancev1alpha1.RoleGroupSpec{PostgresqlID: "pg-1", GroupRole: ""},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, rg *instancev1alpha1.RoleGroup) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(rg, nil)
			},
			expectAllowed: true,
			expectMessage: "No groupRole specified",
		},
		{
			name: "denied - postgresql not found",
			roleGroup: &instancev1alpha1.RoleGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-rg", Namespace: "default"},
				Spec:       instancev1alpha1.RoleGroupSpec{PostgresqlID: "pg-1", GroupRole: "testgroup"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, rg *instancev1alpha1.RoleGroup) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(rg, nil)
				mc.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).
					Return(&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)
			},
			expectAllowed: false,
			expectMessage: "does not exist",
		},
		{
			name:      "error - decode error",
			roleGroup: nil,
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, rg *instancev1alpha1.RoleGroup) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.RoleGroup")).Return(nil, assert.AnError)
			},
			expectAllowed: false,
			expectCode:    400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
					Operation: tt.operation,
				},
			}

			tt.setupMocks(mockClient, mockDecoder, req, tt.roleGroup)

			response := validator.Handle(context.Background(), req)

			assert.Equal(t, tt.expectAllowed, response.Allowed)
			if tt.expectCode != 0 {
				assert.Equal(t, tt.expectCode, response.Result.Code)
			}
			if tt.expectMessage != "" {
				assert.Contains(t, response.Result.Message, tt.expectMessage)
			}
		})
	}
}

// TestSchemaValidator_Handle_TableDriven tests SchemaValidator.Handle with table-driven tests
func TestSchemaValidator_Handle_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		schema        *instancev1alpha1.Schema
		operation     admissionv1.Operation
		setupMocks    func(*MockWebhookClient, *MockDecoder, admission.Request, *instancev1alpha1.Schema)
		expectAllowed bool
		expectCode    int32
		expectMessage string
	}{
		{
			name: "denied - no postgresqlID",
			schema: &instancev1alpha1.Schema{
				ObjectMeta: metav1.ObjectMeta{Name: "test-schema", Namespace: "default"},
				Spec:       instancev1alpha1.SchemaSpec{PostgresqlID: "", Schema: "testschema", Owner: "owner", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, schema *instancev1alpha1.Schema) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)
			},
			expectAllowed: false,
			expectMessage: "postgresqlID is required",
		},
		{
			name: "denied - no schema name",
			schema: &instancev1alpha1.Schema{
				ObjectMeta: metav1.ObjectMeta{Name: "test-schema", Namespace: "default"},
				Spec:       instancev1alpha1.SchemaSpec{PostgresqlID: "pg-1", Schema: "", Owner: "owner", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, schema *instancev1alpha1.Schema) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)
			},
			expectAllowed: false,
			expectMessage: "schema name is required",
		},
		{
			name: "denied - no owner",
			schema: &instancev1alpha1.Schema{
				ObjectMeta: metav1.ObjectMeta{Name: "test-schema", Namespace: "default"},
				Spec:       instancev1alpha1.SchemaSpec{PostgresqlID: "pg-1", Schema: "testschema", Owner: "", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, schema *instancev1alpha1.Schema) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)
			},
			expectAllowed: false,
			expectMessage: "owner is required",
		},
		{
			name: "denied - no database",
			schema: &instancev1alpha1.Schema{
				ObjectMeta: metav1.ObjectMeta{Name: "test-schema", Namespace: "default"},
				Spec:       instancev1alpha1.SchemaSpec{PostgresqlID: "pg-1", Schema: "testschema", Owner: "owner", Database: ""},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, schema *instancev1alpha1.Schema) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)
			},
			expectAllowed: false,
			expectMessage: "database is required",
		},
		{
			name: "denied - postgresql not found",
			schema: &instancev1alpha1.Schema{
				ObjectMeta: metav1.ObjectMeta{Name: "test-schema", Namespace: "default"},
				Spec:       instancev1alpha1.SchemaSpec{PostgresqlID: "pg-1", Schema: "testschema", Owner: "owner", Database: "testdb"},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, schema *instancev1alpha1.Schema) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)
				mc.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).
					Return(&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)
			},
			expectAllowed: false,
			expectMessage: "does not exist",
		},
		{
			name:      "error - decode error",
			schema:    nil,
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, schema *instancev1alpha1.Schema) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(nil, assert.AnError)
			},
			expectAllowed: false,
			expectCode:    400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: tt.operation,
				},
			}

			tt.setupMocks(mockClient, mockDecoder, req, tt.schema)

			response := validator.Handle(context.Background(), req)

			assert.Equal(t, tt.expectAllowed, response.Allowed)
			if tt.expectCode != 0 {
				assert.Equal(t, tt.expectCode, response.Result.Code)
			}
			if tt.expectMessage != "" {
				assert.Contains(t, response.Result.Message, tt.expectMessage)
			}
		})
	}
}

// TestPostgresqlValidator_Handle_TableDriven tests PostgresqlValidator.Handle with table-driven tests
func TestPostgresqlValidator_Handle_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		postgresql    *instancev1alpha1.Postgresql
		operation     admissionv1.Operation
		setupMocks    func(*MockWebhookClient, *MockDecoder, admission.Request, *instancev1alpha1.Postgresql)
		expectAllowed bool
		expectCode    int32
		expectMessage string
	}{
		{
			name: "allowed - no external instance",
			postgresql: &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pg", Namespace: "default"},
				Spec:       instancev1alpha1.PostgresqlSpec{ExternalInstance: nil},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, pg *instancev1alpha1.Postgresql) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Postgresql")).Return(pg, nil)
			},
			expectAllowed: true,
			expectMessage: "No postgresqlID specified",
		},
		{
			name: "allowed - empty postgresqlID",
			postgresql: &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pg", Namespace: "default"},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{PostgresqlID: ""},
				},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, pg *instancev1alpha1.Postgresql) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Postgresql")).Return(pg, nil)
			},
			expectAllowed: true,
			expectMessage: "No postgresqlID specified",
		},
		{
			name: "denied - duplicate postgresqlID",
			postgresql: &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pg", Namespace: "default"},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{PostgresqlID: "pg-1"},
				},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, pg *instancev1alpha1.Postgresql) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Postgresql")).Return(pg, nil)
				existingList := &instancev1alpha1.PostgresqlList{
					Items: []instancev1alpha1.Postgresql{
						{
							ObjectMeta: metav1.ObjectMeta{Name: "existing-pg", Namespace: "other-namespace"},
							Spec: instancev1alpha1.PostgresqlSpec{
								ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{PostgresqlID: "pg-1"},
							},
						},
					},
				}
				mc.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).
					Return(existingList, nil)
			},
			expectAllowed: false,
			expectMessage: "pg-1",
		},
		{
			name:       "error - decode error",
			postgresql: nil,
			operation:  admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, pg *instancev1alpha1.Postgresql) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Postgresql")).Return(nil, assert.AnError)
			},
			expectAllowed: false,
			expectCode:    400,
		},
		{
			name: "error - list error",
			postgresql: &instancev1alpha1.Postgresql{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pg", Namespace: "default"},
				Spec: instancev1alpha1.PostgresqlSpec{
					ExternalInstance: &instancev1alpha1.ExternalPostgresqlInstance{PostgresqlID: "pg-1"},
				},
			},
			operation: admissionv1.Create,
			setupMocks: func(mc *MockWebhookClient, md *MockDecoder, req admission.Request, pg *instancev1alpha1.Postgresql) {
				md.On("Decode", req, mock.AnythingOfType("*v1alpha1.Postgresql")).Return(pg, nil)
				mc.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).
					Return(nil, assert.AnError)
			},
			expectAllowed: false,
			expectCode:    500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: tt.operation,
				},
			}

			tt.setupMocks(mockClient, mockDecoder, req, tt.postgresql)

			response := validator.Handle(context.Background(), req)

			assert.Equal(t, tt.expectAllowed, response.Allowed)
			if tt.expectCode != 0 {
				assert.Equal(t, tt.expectCode, response.Result.Code)
			}
			if tt.expectMessage != "" {
				assert.Contains(t, response.Result.Message, tt.expectMessage)
			}
		})
	}
}

// TestUserValidator_Handle_MultipleExcludedUsers tests multiple excluded users
func TestUserValidator_Handle_MultipleExcludedUsers(t *testing.T) {
	excludeList := []string{"postgres", "admin", "root", "superuser"}

	for _, excludedUser := range excludeList {
		t.Run("excluded_"+excludedUser, func(t *testing.T) {
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
				ExcludeUserList: excludeList,
			}

			user := &instancev1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
				Spec:       instancev1alpha1.UserSpec{PostgresqlID: "pg-1", Username: excludedUser},
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
		})
	}
}

// TestUserValidator_Handle_NotExcludedUser tests user not in exclude list
func TestUserValidator_Handle_NotExcludedUser(t *testing.T) {
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
		ExcludeUserList: []string{"postgres", "admin"},
	}

	user := &instancev1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
		Spec:       instancev1alpha1.UserSpec{PostgresqlID: "pg-1", Username: "normaluser"},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.User")).Return(user, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).
		Return(&instancev1alpha1.PostgresqlList{Items: []instancev1alpha1.Postgresql{}}, nil)

	response := validator.Handle(context.Background(), req)

	// Should be denied because PostgreSQL not found, not because of exclude list
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "does not exist")
	assert.NotContains(t, response.Result.Message, "excludelist")
}
