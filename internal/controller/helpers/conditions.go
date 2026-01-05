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

package helpers

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
)

// UpdateCondition updates or adds a condition to a conditions slice using meta.SetStatusCondition
// This is a generic helper that works with any slice of conditions
func UpdateCondition(
	conditions *[]metav1.Condition, generation int64, conditionType string,
	status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	}
	meta.SetStatusCondition(conditions, condition)
}

// FindCondition finds a condition by type in the conditions slice using meta.FindStatusCondition
func FindCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	return meta.FindStatusCondition(conditions, conditionType)
}

// Type-specific condition helpers for each CRD type

// UpdateUserCondition updates or adds a condition to the User status
func UpdateUserCondition(
	user *instancev1alpha1.User, conditionType string, status metav1.ConditionStatus, reason, message string) {
	UpdateCondition(&user.Status.Conditions, user.Generation, conditionType, status, reason, message)
}

// UpdateDatabaseCondition updates or adds a condition to the Database status
func UpdateDatabaseCondition(
	database *instancev1alpha1.Database, conditionType string,
	status metav1.ConditionStatus, reason, message string) {
	UpdateCondition(&database.Status.Conditions, database.Generation, conditionType, status, reason, message)
}

// UpdateGrantCondition updates or adds a condition to the Grant status
func UpdateGrantCondition(
	grant *instancev1alpha1.Grant, conditionType string,
	status metav1.ConditionStatus, reason, message string) {
	UpdateCondition(&grant.Status.Conditions, grant.Generation, conditionType, status, reason, message)
}

// UpdateRoleGroupCondition updates or adds a condition to the RoleGroup status
func UpdateRoleGroupCondition(
	roleGroup *instancev1alpha1.RoleGroup, conditionType string,
	status metav1.ConditionStatus, reason, message string) {
	UpdateCondition(&roleGroup.Status.Conditions, roleGroup.Generation, conditionType, status, reason, message)
}

// UpdateSchemaCondition updates or adds a condition to the Schema status
func UpdateSchemaCondition(
	schema *instancev1alpha1.Schema, conditionType string,
	status metav1.ConditionStatus, reason, message string) {
	UpdateCondition(&schema.Status.Conditions, schema.Generation, conditionType, status, reason, message)
}

// UpdatePostgresqlCondition updates or adds a condition to the Postgresql status
func UpdatePostgresqlCondition(
	postgresql *instancev1alpha1.Postgresql, conditionType string,
	status metav1.ConditionStatus, reason, message string) {
	UpdateCondition(&postgresql.Status.Conditions, postgresql.Generation, conditionType, status, reason, message)
}
