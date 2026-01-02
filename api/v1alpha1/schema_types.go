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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SchemaSpec defines the desired state of Schema
type SchemaSpec struct {
	// Schema is the name of the PostgreSQL schema to create
	// +required
	Schema string `json:"schema"`

	// Owner is the PostgreSQL user that will own the schema
	// +required
	Owner string `json:"owner"`

	// PostgresqlID is the ID of the PostgreSQL instance where this schema should be created
	// +required
	PostgresqlID string `json:"postgresqlID"`
}

// SchemaStatus defines the observed state of Schema
type SchemaStatus struct {
	// Created indicates whether the schema has been successfully created in PostgreSQL
	// +optional
	Created bool `json:"created,omitempty"`

	// LastSyncAttempt is the timestamp of the last sync attempt
	// +optional
	LastSyncAttempt *metav1.Time `json:"lastSyncAttempt,omitempty"`

	// Conditions represent the latest available observations of the Schema state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Schema is the Schema for the schemas API
type Schema struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SchemaSpec   `json:"spec,omitempty"`
	Status SchemaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SchemaList contains a list of Schema
type SchemaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Schema `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Schema{}, &SchemaList{})
}
