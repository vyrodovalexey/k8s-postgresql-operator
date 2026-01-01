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
)

func TestSchemaValidator_Handle_NoPostgresqlID(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
		Client:  mockClient,
		Decoder: mockDecoder,
		Log:     logger,
	}

	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "No postgresqlID specified")
}

func TestSchemaValidator_Handle_NoSchemaName(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
		Client:  mockClient,
		Decoder: mockDecoder,
		Log:     logger,
	}

	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: "pg-1",
			Schema:       "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "No schema name specified")
}

func TestSchemaValidator_Handle_NoOwner(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
		Client:  mockClient,
		Decoder: mockDecoder,
		Log:     logger,
	}

	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: "pg-1",
			Schema:       "schema1",
			Owner:        "",
		},
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "No owner specified")
}

func TestSchemaValidator_Handle_DuplicateSchema(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
		Client:  mockClient,
		Decoder: mockDecoder,
		Log:     logger,
	}

	postgresqlID := "pg-1"
	schemaName := "schema1"

	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: postgresqlID,
			Schema:       schemaName,
			Owner:        "owner1",
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

	schemaList := &instancev1alpha1.SchemaList{
		Items: []instancev1alpha1.Schema{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-schema",
					Namespace: "other-namespace",
				},
				Spec: instancev1alpha1.SchemaSpec{
					PostgresqlID: postgresqlID,
					Schema:       schemaName,
					Owner:        "owner1",
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
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(schemaList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, postgresqlID)
	assert.Contains(t, response.Result.Message, schemaName)
}

func TestSchemaValidator_Handle_NoDuplicate(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
		Client:  mockClient,
		Decoder: mockDecoder,
		Log:     logger,
	}

	postgresqlID := "pg-1"
	schemaName := "schema1"

	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: postgresqlID,
			Schema:       schemaName,
			Owner:        "owner1",
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

	schemaList := &instancev1alpha1.SchemaList{
		Items: []instancev1alpha1.Schema{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-schema",
					Namespace: "other-namespace",
				},
				Spec: instancev1alpha1.SchemaSpec{
					PostgresqlID: postgresqlID,
					Schema:       "different-schema", // Different schema name
					Owner:        "owner1",
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
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.SchemaList"), mock.Anything).Return(schemaList, nil)

	response := validator.Handle(context.Background(), req)
	assert.True(t, response.Allowed)
}

func TestSchemaValidator_Handle_PostgresqlNotFound(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
		Client:  mockClient,
		Decoder: mockDecoder,
		Log:     logger,
	}

	schema := &instancev1alpha1.Schema{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-schema",
			Namespace: "default",
		},
		Spec: instancev1alpha1.SchemaSpec{
			PostgresqlID: "pg-1",
			Schema:       "schema1",
			Owner:        "owner1",
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

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(schema, nil)
	mockClient.On("List", context.Background(), mock.AnythingOfType("*v1alpha1.PostgresqlList"), mock.Anything).Return(postgresqlList, nil)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "does not exist")
}

func TestSchemaValidator_Handle_DecodeError(t *testing.T) {
	mockClient := new(MockWebhookClient)
	mockDecoder := new(MockDecoder)
	logger := zap.NewNop().Sugar()

	validator := &SchemaValidator{
		Client:  mockClient,
		Decoder: mockDecoder,
		Log:     logger,
	}

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
		},
	}

	mockDecoder.On("Decode", req, mock.AnythingOfType("*v1alpha1.Schema")).Return(nil, assert.AnError)

	response := validator.Handle(context.Background(), req)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(400), response.Result.Code)
}
