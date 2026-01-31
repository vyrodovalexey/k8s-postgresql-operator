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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/constants"
	controllerhelpers "github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller/helpers"
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
		r.Log.Errorw("Failed to get Postgresql",
			"name", req.Name, "namespace", req.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	// Handle external PostgreSQL instance if specified
	if postgresql.Spec.ExternalInstance != nil {
		return r.reconcileExternalInstance(ctx, postgresql)
	}

	// For now, if no external instance is specified, we'll just return
	r.Log.Infow("No external instance specified, managed instance creation not yet implemented",
		"name", postgresql.Name, "namespace", postgresql.Namespace)
	return ctrl.Result{}, nil
}

// reconcileExternalInstance handles reconciliation for external PostgreSQL instances
func (r *PostgresqlReconciler) reconcileExternalInstance(
	ctx context.Context, postgresql *instancev1alpha1.Postgresql) (ctrl.Result, error) {
	externalInstance := postgresql.Spec.ExternalInstance

	// Get connection defaults
	port, sslMode := controllerhelpers.GetConnectionDefaults(externalInstance.Port, externalInstance.SSLMode)

	// Build connection address
	connectionAddress := fmt.Sprintf("%s:%d", externalInstance.Address, port)
	var username, password string

	// If credentials are not provided in spec, try to get them from Vault
	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := getVaultCredentialsWithRetryAndRateLimit(
			ctx, r.VaultClient, externalInstance.PostgresqlID, r.Log,
			r.VaultAvailabilityRetries, r.VaultAvailabilityRetryDelay, r.VaultRateLimiter)
		if err != nil {
			r.Log.Errorw("Failed to get credentials from Vault",
				"name", postgresql.Name, "namespace", postgresql.Namespace,
				"postgresqlID", externalInstance.PostgresqlID, "error", err)
		} else {
			username = vaultUsername
			password = vaultPassword
			r.Log.Debugw("Credentials retrieved from Vault",
				"name", postgresql.Name, "namespace", postgresql.Namespace, "postgresqlID", externalInstance.PostgresqlID)
		}
	}

	// Set default database if not specified
	database := constants.DefaultDatabaseName

	// Apply rate limiting before PostgreSQL operation
	if r.PostgresqlRateLimiter != nil {
		if err := r.PostgresqlRateLimiter.Wait(ctx); err != nil {
			return ctrl.Result{}, fmt.Errorf("rate limit wait failed: %w", err)
		}
	}

	// Test PostgreSQL connection
	connected, err := pg.TestConnection(ctx, externalInstance.Address, port, database, username, password, sslMode,
		r.Log, r.PostgresqlConnectionRetries, r.PostgresqlConnectionTimeout)
	if err != nil {
		r.Log.Errorw("Failed to test PostgreSQL connection",
			"name", postgresql.Name, "namespace", postgresql.Namespace,
			"address", connectionAddress, "operation", "testConnection", "error", err)
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
		conditionReason = constants.ConditionReasonConnected
		conditionMessage = fmt.Sprintf("Successfully connected to PostgreSQL at %s", connectionAddress)
	} else {
		conditionStatus = metav1.ConditionFalse
		conditionReason = constants.ConditionReasonConnectionFailed
		if err != nil {
			conditionMessage = fmt.Sprintf("Failed to connect to PostgreSQL with id %s at %s: %v",
				externalInstance.PostgresqlID, connectionAddress, err)
		} else {
			conditionMessage = fmt.Sprintf("Failed to connect to PostgreSQL with id %s at %s",
				externalInstance.PostgresqlID, connectionAddress)
		}
	}

	readyCondition := controllerhelpers.FindCondition(
		postgresql.Status.Conditions, constants.ConditionTypeReady)
	if readyCondition == nil || readyCondition.Status != conditionStatus ||
		readyCondition.Message != conditionMessage {
		controllerhelpers.UpdatePostgresqlCondition(postgresql, constants.ConditionTypeReady,
			conditionStatus, conditionReason, conditionMessage)
		statusNeedsUpdate = true
	}

	// Only update status if something changed
	if !statusNeedsUpdate {
		// Requeue for periodic connectivity check
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
	}

	now := metav1.Now()
	postgresql.Status.LastConnectionAttempt = &now

	r.Log.Debugw("Testing PostgreSQL connection",
		"uuid", externalInstance.PostgresqlID,
		"address", externalInstance.Address,
		"port", port,
		"database", database,
		"connected", connected)

	// Update the status
	if err := r.Status().Update(ctx, postgresql); err != nil {
		r.Log.Errorw("Failed to update Postgresql status",
			"name", postgresql.Name, "namespace", postgresql.Namespace, "error", err)
		return ctrl.Result{}, err
	}

	r.logConnectionResult(postgresql, connectionAddress, connected, err)

	// Requeue for periodic connectivity check
	// This ensures we periodically check the connection even if nothing changed
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueDelay}, nil
}

// logConnectionResult logs the connection result and records appropriate events
func (r *PostgresqlReconciler) logConnectionResult(
	postgresql *instancev1alpha1.Postgresql, connectionAddress string, connected bool, err error) {
	if connected {
		r.EventRecorder.RecordConnected(postgresql, connectionAddress)
		r.Log.Infow("Successfully connected to PostgreSQL instance",
			"name", postgresql.Name, "namespace", postgresql.Namespace, "address", connectionAddress)
		return
	}

	errMsg := "connection failed"
	if err != nil {
		errMsg = err.Error()
	}
	r.EventRecorder.RecordConnectionFailed(postgresql, connectionAddress, errMsg)
	r.Log.Warnw("Failed to connect to PostgreSQL instance",
		"name", postgresql.Name, "namespace", postgresql.Namespace, "address", connectionAddress, "error", err)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Postgresql{}).
		Named("postgresql").
		WithEventFilter(controllerhelpers.IgnoreStatusUpdatesPredicate()).
		Complete(r)
}
