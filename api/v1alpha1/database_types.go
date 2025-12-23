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

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	// Database is the name of the PostgreSQL database to create
	// +required
	Database string `json:"database"`

	// Owner is the PostgreSQL user that will own the database
	// +required
	Owner string `json:"owner"`

	// PostgresqlID is the ID of the PostgreSQL instance where this database should be created
	// +required
	PostgresqlID string `json:"postgresqlID"`
}

// DatabaseStatus defines the observed state of Database
type DatabaseStatus struct {
	// Created indicates whether the database has been successfully created in PostgreSQL
	// +optional
	Created bool `json:"created,omitempty"`

	// LastSyncAttempt is the timestamp of the last sync attempt
	// +optional
	LastSyncAttempt *metav1.Time `json:"lastSyncAttempt,omitempty"`

	// Conditions represent the latest available observations of the Database state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Database is the Schema for the databases API
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
