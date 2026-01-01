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

// GrantType represents the type of grant
// +kubebuilder:validation:Enum=database;schema;table;sequence;function;all_tables;all_sequences
type GrantType string

const (
	GrantTypeDatabase     GrantType = "database"
	GrantTypeSchema       GrantType = "schema"
	GrantTypeTable        GrantType = "table"
	GrantTypeSequence     GrantType = "sequence"
	GrantTypeFunction     GrantType = "function"
	GrantTypeAllTables    GrantType = "all_tables"
	GrantTypeAllSequences GrantType = "all_sequences"
)

// GrantItem defines a single grant
type GrantItem struct {
	// Type is the type of grant: database, schema, table, sequence, function, all_tables, all_sequences
	// +required
	Type GrantType `json:"type"`

	// Schema is the schema name (required for schema, table, sequence, function, all_tables, all_sequences types)
	// +optional
	Schema string `json:"schema,omitempty"`

	// Table is the table name (required for table type)
	// +optional
	Table string `json:"table,omitempty"`

	// Sequence is the sequence name (required for sequence type)
	// +optional
	Sequence string `json:"sequence,omitempty"`

	// Function is the function name (required for function type)
	// +optional
	Function string `json:"function,omitempty"`

	// Privileges is the list of privileges to grant
	// +required
	Privileges []string `json:"privileges"`
}

// DefaultPrivilegeItem defines default privileges for future objects
type DefaultPrivilegeItem struct {
	// ObjectType is the type of object: tables, sequences, functions, types
	// +required
	ObjectType string `json:"objectType"`

	// Schema is the schema name
	// +required
	Schema string `json:"schema"`

	// Privileges is the list of privileges to grant
	// +required
	Privileges []string `json:"privileges"`
}

// GrantSpec defines the desired state of Grant
type GrantSpec struct {
	// Role is the PostgreSQL role/user to grant permissions to
	// +required
	Role string `json:"role"`

	// Database is the PostgreSQL database name
	// +required
	Database string `json:"database"`

	// PostgresqlID is the ID of the PostgreSQL instance where grants should be applied
	// +required
	PostgresqlID string `json:"postgresqlID"`

	// Grants is the list of grants to apply
	// +optional
	Grants []GrantItem `json:"grants,omitempty"`

	// DefaultPrivileges is the list of default privileges for future objects
	// +optional
	DefaultPrivileges []DefaultPrivilegeItem `json:"defaultPrivileges,omitempty"`
}

// GrantStatus defines the observed state of Grant
type GrantStatus struct {
	// Applied indicates whether the grants have been successfully applied in PostgreSQL
	// +optional
	Applied bool `json:"applied,omitempty"`

	// LastSyncAttempt is the timestamp of the last sync attempt
	// +optional
	LastSyncAttempt *metav1.Time `json:"lastSyncAttempt,omitempty"`

	// Conditions represent the latest available observations of the Grant state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Grant is the Schema for the grants API
type Grant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GrantSpec   `json:"spec,omitempty"`
	Status GrantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GrantList contains a list of Grant
type GrantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Grant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Grant{}, &GrantList{})
}
