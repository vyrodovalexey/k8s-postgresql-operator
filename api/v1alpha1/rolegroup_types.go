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

// RoleGroupSpec defines the desired state of RoleGroup
type RoleGroupSpec struct {
	// GroupRole is the name of the PostgreSQL role that will be the group
	// +required
	GroupRole string `json:"groupRole"`

	// MemberRoles is the list of PostgreSQL role names that will be members of the group role
	// +required
	MemberRoles []string `json:"memberRoles"`

	// PostgresqlID is the ID of the PostgreSQL instance where this role group should be created
	// +required
	PostgresqlID string `json:"postgresqlID"`
}

// RoleGroupStatus defines the observed state of RoleGroup
type RoleGroupStatus struct {
	// Created indicates whether the role group has been successfully created in PostgreSQL
	// +optional
	Created bool `json:"created,omitempty"`

	// LastSyncAttempt is the timestamp of the last sync attempt
	// +optional
	LastSyncAttempt *metav1.Time `json:"lastSyncAttempt,omitempty"`

	// Conditions represent the latest available observations of the RoleGroup state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RoleGroup is the Schema for the rolegroups API
type RoleGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleGroupSpec   `json:"spec,omitempty"`
	Status RoleGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RoleGroupList contains a list of RoleGroup
type RoleGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RoleGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RoleGroup{}, &RoleGroupList{})
}
