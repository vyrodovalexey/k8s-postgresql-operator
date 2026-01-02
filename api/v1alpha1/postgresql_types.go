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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PostgresqlSpec defines the desired state of Postgresql.
type PostgresqlSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// ExternalInstance defines connection details for an external PostgreSQL instance
	// If specified, the operator will manage this external instance instead of creating a new one
	// +optional
	ExternalInstance *ExternalPostgresqlInstance `json:"externalInstance,omitempty"`
}

// ExternalPostgresqlInstance defines connection details for an external PostgreSQL instance
type ExternalPostgresqlInstance struct {
	PostgresqlID string `json:"postgresqlID"`
	// Address is the SDN (Service Discovery Name) or network address of the PostgreSQL instance
	// This can be a hostname, IP address, or Kubernetes service name
	// Example: "postgres.example.com", "192.168.1.100", "postgres-service.namespace.svc.cluster.local"
	// +required

	Address string `json:"address"`

	// Port is the port number on which PostgreSQL is listening
	// Defaults to 5432 if not specified
	// +optional
	// +kubebuilder:default=5432
	Port int32 `json:"port,omitempty"`

	// Database is the name of the database to connect to
	// +optional
	// Database string `json:"database,omitempty"`

	// Username string `json:"username"`
	// Password string `json:"password"`
	// CredentialsSecretRef is a reference to a Kubernetes secret containing PostgreSQL credentials
	// The secret should contain keys: username and password
	// +optional
	//CredentialsSecretRef *corev1.SecretReference `json:"credentialsSecretRef,omitempty"`

	// SSLMode specifies the SSL connection mode
	// Valid values: disable, require, verify-ca, verify-full
	// +optional
	// +kubebuilder:default="require"
	SSLMode string `json:"sslMode,omitempty"`
}

// PostgresqlStatus defines the observed state of Postgresql.
type PostgresqlStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Connected indicates whether the operator has successfully connected to the PostgreSQL instance
	// +optional
	Connected bool `json:"connected,omitempty"`

	// ConnectionAddress is the actual address being used to connect
	// +optional
	ConnectionAddress string `json:"connectionAddress,omitempty"`

	// LastConnectionAttempt is the timestamp of the last connection attempt
	// +optional
	LastConnectionAttempt *metav1.Time `json:"lastConnectionAttempt,omitempty"`

	// Conditions represent the latest available observations of the PostgreSQL instance state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Postgresql is the Schema for the postgresqls API.
type Postgresql struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PostgresqlSpec   `json:"spec,omitempty"`
	Status PostgresqlStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PostgresqlList contains a list of Postgresql.
type PostgresqlList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Postgresql `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Postgresql{}, &PostgresqlList{})
}
