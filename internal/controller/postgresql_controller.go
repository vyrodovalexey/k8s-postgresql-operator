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

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
)

// PostgresqlReconciler reconciles a Postgresql object
type PostgresqlReconciler struct {
	BaseReconcilerConfig
}

// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls/status,verbs=get;update;patch
//nolint:lll // kubebuilder directive cannot be split
// +kubebuilder:rbac:groups=postgresql-operator.vyrodovalexey.github.com,resources=postgresqls/finalizers,verbs=update
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;patch;update
//nolint:lll // kubebuilder directive cannot be split

func (r *PostgresqlReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Fetch the Postgresql instance
	postgresql := &instancev1alpha1.Postgresql{}
	if err := r.Get(ctx, req.NamespacedName, postgresql); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "Failed to get Postgresql")
		return ctrl.Result{}, err
	}

	// Handle external PostgreSQL instance if specified
	if postgresql.Spec.ExternalInstance != nil {
		return r.reconcileExternalInstance(ctx, postgresql)
	}

	// For now, if no external instance is specified, we'll just return
	r.Log.Info("No external instance specified, managed instance creation not yet implemented")
	return ctrl.Result{}, nil
}

// reconcileExternalInstance handles reconciliation for external PostgreSQL instances
func (r *PostgresqlReconciler) reconcileExternalInstance(
	ctx context.Context, postgresql *instancev1alpha1.Postgresql) (ctrl.Result, error) {
	externalInstance := postgresql.Spec.ExternalInstance

	// Set default port if not specified
	port := externalInstance.Port
	if port == 0 {
		port = 5432
	}

	// Build connection address
	connectionAddress := fmt.Sprintf("%s:%d", externalInstance.Address, port)
	var username, password string

	// If credentials are not provided in spec, try to get them from Vault
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := getVaultCredentialsWithRetry(
			ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
			r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay)
		if err != nil {
			r.Log.Error(err, "Failed to get credentials from Vault, ", "postgresqlID: ", externalInstance.PostgresqlID)
		} else {
			username = vaultUsername
			password = vaultPassword
			r.Log.Info("Credentials retrieved from Vault, ", "postgresqlID: ", externalInstance.PostgresqlID)
		}
	}

	// Set default SSL mode if not specified
	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = pg.DefaultSSLMode
	}

	// Set default database if not specified
	database := "postgres"

	// Test PostgreSQL connection
	connected, err := pg.TestConnection(ctx, externalInstance.Address, port, database, username, password, sslMode,
		r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout)
	if err != nil {
		r.Log.Error(err, "Failed to test PostgreSQL connection, ", "address: ", connectionAddress)
	}

	// Check if status needs to be updated
	statusNeedsUpdate := false
	desiredConnectionAddress := connectionAddress

	// Only update status if something actually changed
	if postgresql.Status.ConnectionAddress != desiredConnectionAddress {
		postgresql.Status.ConnectionAddress = desiredConnectionAddress
		statusNeedsUpdate = true
	}

	if postgresql.Status.Connected != connected {
		postgresql.Status.Connected = connected
		statusNeedsUpdate = true
	}

	// Update condition based on connection status
	var conditionStatus metav1.ConditionStatus
	var conditionReason, conditionMessage string
	if connected {
		conditionStatus = metav1.ConditionTrue
		conditionReason = "Connected"
		conditionMessage = fmt.Sprintf("Successfully connected to PostgreSQL at %s", connectionAddress)
	} else {
		conditionStatus = metav1.ConditionFalse
		conditionReason = "ConnectionFailed"
		if err != nil {
			conditionMessage = fmt.Sprintf("Failed to connect to PostgreSQL with id %s at %s: %v",
				externalInstance.PostgresqlID, connectionAddress, err)
		} else {
			conditionMessage = fmt.Sprintf("Failed to connect to PostgreSQL with id %s at %s",
				externalInstance.PostgresqlID, connectionAddress)
		}
	}

	readyCondition := findCondition(postgresql.Status.Conditions, "Ready")
	if readyCondition == nil || readyCondition.Status != conditionStatus || readyCondition.Message != conditionMessage {
		updateCondition(postgresql, "Ready", conditionStatus, conditionReason, conditionMessage)
		statusNeedsUpdate = true
	}

	// Only update status if something changed
	if statusNeedsUpdate {
		now := metav1.Now()
		postgresql.Status.LastConnectionAttempt = &now

		r.Log.Info("Testing PostgreSQL connection: ",
			" uuid: ", externalInstance.PostgresqlID,
			" address: ", externalInstance.Address,
			" port: ", port,
			" database: ", database,
			" username: ", username,
			" connected: ", connected)

		// Update the status
		if err := r.Status().Update(ctx, postgresql); err != nil {
			r.Log.Error(err, "Failed to update Postgresql status")
			return ctrl.Result{}, err
		}

		if connected {
			r.Log.Info("Successfully connected to PostgreSQL instance, ", "address: ", connectionAddress)
		} else {
			r.Log.Info("Failed to connect to PostgreSQL instance, ", "address: ", connectionAddress, "error: ", err)
		}
	}

	// Requeue for periodic connectivity check (default: 30 seconds)
	// This ensures we periodically check the connection even if nothing changed
	requeueInterval := 30 * time.Second
	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

// updateCondition updates or adds a condition to the Postgresql status
// nolint:unparam // conditionType parameter is kept for API consistency and future extensibility
func updateCondition(
	postgresql *instancev1alpha1.Postgresql, conditionType string, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
		ObservedGeneration: postgresql.Generation,
	}

	// Find and update existing condition or add new one
	found := false
	for i, c := range postgresql.Status.Conditions {
		if c.Type == conditionType {
			postgresql.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		postgresql.Status.Conditions = append(postgresql.Status.Conditions, condition)
	}
}

// findCondition finds a condition by type in the conditions slice
func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a predicate that ignores status-only updates
	ignoreStatusUpdates := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Reconcile if spec changed
			oldObj := e.ObjectOld.(*instancev1alpha1.Postgresql)
			newObj := e.ObjectNew.(*instancev1alpha1.Postgresql)

			// Reconcile if generation changed (spec changed)
			if oldObj.Generation != newObj.Generation {
				return true
			}

			// Reconcile if deletion timestamp changed
			if oldObj.DeletionTimestamp != newObj.DeletionTimestamp {
				return true
			}

			// Ignore status-only updates
			return false
		},
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return true },
		GenericFunc: func(e event.GenericEvent) bool { return true },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Postgresql{}).
		Named("postgresql").
		WithEventFilter(ignoreStatusUpdates).
		Complete(r)
}
