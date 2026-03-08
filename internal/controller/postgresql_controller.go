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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	controllerhelpers "github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller/helpers"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/metrics"
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
	startTime := time.Now()
	metrics.IncReconcileTotal("postgresql")
	defer func() {
		metrics.ObserveReconcileDuration("postgresql", time.Since(startTime))
	}()

	// Fetch the Postgresql instance
	postgresql := &instancev1alpha1.Postgresql{}
	if err := r.Get(ctx, req.NamespacedName, postgresql); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Errorw("Failed to get Postgresql", "error", err)
		metrics.IncReconcileErrors("postgresql")
		return ctrl.Result{}, err
	}

	// Handle external PostgreSQL instance if specified
	if postgresql.Spec.ExternalInstance != nil {
		return r.reconcileExternalInstance(ctx, postgresql)
	}

	// For now, if no external instance is specified, we'll just return
	r.Log.Infow("No external instance specified, managed instance creation not yet implemented")
	return ctrl.Result{}, nil
}

// reconcileExternalInstance handles reconciliation for external PostgreSQL instances
// nolint:unparam // error return is kept for Reconcile signature compatibility and future extensibility
func (r *PostgresqlReconciler) reconcileExternalInstance(
	ctx context.Context,
	postgresql *instancev1alpha1.Postgresql,
) (ctrl.Result, error) {
	externalInstance := postgresql.Spec.ExternalInstance

	port := externalInstance.Port
	if port == 0 {
		port = pg.DefaultPort
	}

	connectionAddress := fmt.Sprintf("%s:%d", externalInstance.Address, port)
	var username, password string

	if r.VaultClient != nil {
		vaultUsername, vaultPassword, err := getVaultCredentialsWithRetry(
			ctx, r.VaultClient, externalInstance.PostgresqlID,
			r.Log, r.VaultAvailabilityRetries,
			r.VaultAvailabilityRetryDelay)
		if err != nil {
			r.Log.Errorw("Failed to get credentials from Vault",
				"error", err, "postgresqlID", externalInstance.PostgresqlID)
		} else {
			username = vaultUsername
			password = vaultPassword
			r.Log.Infow("Credentials retrieved from Vault",
				"postgresqlID", externalInstance.PostgresqlID)
		}
	}

	sslMode := externalInstance.SSLMode
	if sslMode == "" {
		sslMode = pg.DefaultSSLMode
	}

	database := pg.DefaultDB

	connected, connErr := pg.TestConnection(
		ctx, externalInstance.Address, port, database,
		username, password, sslMode,
		r.Log, r.PostgresqlConnectionRetries,
		r.PostgresqlConnectionTimeout)
	if connErr != nil {
		r.Log.Errorw("Failed to test PostgreSQL connection",
			"error", connErr, "address", connectionAddress)
		metrics.IncReconcileErrors("postgresql")
		r.Recorder.Event(postgresql, corev1.EventTypeWarning,
			"ConnectionFailed",
			fmt.Sprintf("Failed to connect to PostgreSQL at %s: %v",
				connectionAddress, connErr))
	}

	statusNeedsUpdate := r.updateConnectionStatus(
		postgresql, connectionAddress, connected, connErr,
	)

	if statusNeedsUpdate {
		r.logAndPersistStatus(
			ctx, postgresql, externalInstance,
			port, database, username, connected, connectionAddress,
			connErr,
		)
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// updateConnectionStatus checks and updates the postgresql status fields.
// Returns true if the status was modified.
func (r *PostgresqlReconciler) updateConnectionStatus(
	postgresql *instancev1alpha1.Postgresql,
	connectionAddress string,
	connected bool,
	connErr error,
) bool {
	statusNeedsUpdate := false

	if postgresql.Status.ConnectionAddress != connectionAddress {
		postgresql.Status.ConnectionAddress = connectionAddress
		statusNeedsUpdate = true
	}

	if postgresql.Status.Connected != connected {
		postgresql.Status.Connected = connected
		statusNeedsUpdate = true
	}

	conditionStatus, conditionReason, conditionMessage := buildConnectionCondition(
		connected, connErr,
		postgresql.Spec.ExternalInstance.PostgresqlID,
		connectionAddress,
	)

	readyCondition := findCondition(
		postgresql.Status.Conditions, "Ready",
	)
	if readyCondition == nil ||
		readyCondition.Status != conditionStatus ||
		readyCondition.Message != conditionMessage {
		updateCondition(postgresql, "Ready",
			conditionStatus, conditionReason, conditionMessage)
		statusNeedsUpdate = true
	}

	return statusNeedsUpdate
}

// buildConnectionCondition builds the condition fields for connection status
func buildConnectionCondition(
	connected bool, connErr error,
	postgresqlID, connectionAddress string,
) (status metav1.ConditionStatus, reason string, message string) {
	if connected {
		return metav1.ConditionTrue, "Connected",
			fmt.Sprintf("Successfully connected to PostgreSQL at %s",
				connectionAddress)
	}

	msg := fmt.Sprintf(
		"Failed to connect to PostgreSQL with id %s at %s",
		postgresqlID, connectionAddress)
	if connErr != nil {
		msg = fmt.Sprintf(
			"Failed to connect to PostgreSQL with id %s at %s: %v",
			postgresqlID, connectionAddress, connErr)
	}
	return metav1.ConditionFalse, "ConnectionFailed", msg
}

// logAndPersistStatus persists the updated status and logs the result
func (r *PostgresqlReconciler) logAndPersistStatus(
	ctx context.Context,
	postgresql *instancev1alpha1.Postgresql,
	externalInstance *instancev1alpha1.ExternalPostgresqlInstance,
	port int32, database, username string,
	connected bool, connectionAddress string,
	connErr error,
) {
	now := metav1.Now()
	postgresql.Status.LastConnectionAttempt = &now

	r.Log.Infow("Testing PostgreSQL connection",
		"uuid", externalInstance.PostgresqlID,
		"address", externalInstance.Address,
		"port", port,
		"database", database,
		"username", username,
		"connected", connected)

	if err := r.Status().Update(ctx, postgresql); err != nil {
		r.Log.Errorw("Failed to update Postgresql status", "error", err)
		return
	}

	if connected {
		r.Log.Infow("Successfully connected to PostgreSQL instance",
			"address", connectionAddress)
		r.Recorder.Event(postgresql, corev1.EventTypeNormal,
			"Connected",
			fmt.Sprintf("Successfully connected to PostgreSQL at %s",
				connectionAddress))
	} else {
		r.Log.Infow("Failed to connect to PostgreSQL instance",
			"address", connectionAddress, "error", connErr)
		r.Recorder.Event(postgresql, corev1.EventTypeWarning,
			"Disconnected",
			fmt.Sprintf("Failed to connect to PostgreSQL at %s",
				connectionAddress))
	}
}

// updateCondition updates or adds a condition to the Postgresql status using meta.SetStatusCondition
// nolint:unparam // conditionType parameter is kept for API consistency and future extensibility
func updateCondition(
	postgresql *instancev1alpha1.Postgresql, conditionType string,
	status metav1.ConditionStatus, reason, message string,
) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: postgresql.Generation,
	}
	meta.SetStatusCondition(&postgresql.Status.Conditions, condition)
}

// findCondition finds a condition by type in the conditions slice using meta.FindStatusCondition
func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	return meta.FindStatusCondition(conditions, conditionType)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PostgresqlReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&instancev1alpha1.Postgresql{}).
		Named("postgresql").
		WithEventFilter(controllerhelpers.IgnoreStatusUpdatesPredicate()).
		Complete(r)
}
