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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	controllerhelpers "github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller/helpers"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/metrics"
	pg "github.com/vyrodovalexey/k8s-postgresql-operator/internal/postgresql"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
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
	ctx, span := otel.Tracer("controller").Start(ctx,
		"PostgresqlReconciler.Reconcile",
		trace.WithAttributes(
			attribute.String("controller", "postgresql"),
			attribute.String("k8s.resource.name", req.Name),
			attribute.String("k8s.resource.namespace",
				req.Namespace),
		))
	defer span.End()

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
	var usedDefaultCreds bool

	if r.VaultClient != nil {
		username, password, usedDefaultCreds = r.resolveInstanceCredentials(
			ctx, externalInstance.PostgresqlID)
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

	// If connected with default creds, store them as instance_admin
	if connected && usedDefaultCreds && r.VaultClient != nil {
		r.storeInstanceAdminCredentials(ctx, postgresql, externalInstance.PostgresqlID, username, password)
	}

	// Check for instance admin password rotation
	if connected && r.VaultClient != nil && !usedDefaultCreds {
		r.handleInstanceAdminPasswordRotation(ctx, postgresql, externalInstance, port, sslMode)
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

// resolveInstanceCredentials resolves credentials for a PostgreSQL instance.
// Returns (login, password, usedDefaultCreds).
// First tries instance_admin, then falls back to default credentials.
func (r *PostgresqlReconciler) resolveInstanceCredentials(
	ctx context.Context, postgresqlID string,
) (login, password string, usedDefaultCreds bool) {
	// Try instance-specific credentials first
	login, password, err := getVaultCredentialsWithRetry(
		ctx, r.VaultClient, postgresqlID,
		r.Log, r.VaultAvailabilityRetries,
		r.VaultAvailabilityRetryDelay)
	if err == nil {
		r.Log.Infow("Instance admin credentials retrieved from Vault",
			"postgresqlID", postgresqlID)
		metrics.IncVaultCredentialResolution("instance_admin")
		return login, password, false
	}

	// If secret not found, try default credentials
	if vault.IsSecretNotFound(err) {
		r.Log.Infow("Instance admin credentials not found, trying default credentials",
			"postgresqlID", postgresqlID)

		defaultLogin, defaultPassword, defaultErr := getDefaultVaultCredentialsWithRetry(
			ctx, r.VaultClient,
			r.Log, r.VaultAvailabilityRetries,
			r.VaultAvailabilityRetryDelay)
		if defaultErr != nil {
			r.Log.Errorw("Failed to get default credentials from Vault",
				"error", defaultErr, "postgresqlID", postgresqlID)
			metrics.IncVaultCredentialResolution("none")
			return "", "", false
		}

		r.Log.Infow("Using default credentials for instance",
			"postgresqlID", postgresqlID)
		metrics.IncVaultCredentialResolution("default")
		return defaultLogin, defaultPassword, true
	}

	// Vault error (not a 404) - don't fallback
	r.Log.Errorw("Failed to get credentials from Vault",
		"error", err, "postgresqlID", postgresqlID)
	metrics.IncVaultCredentialResolution("none")
	return "", "", false
}

// storeInstanceAdminCredentials stores credentials as instance_admin in Vault
func (r *PostgresqlReconciler) storeInstanceAdminCredentials(
	ctx context.Context,
	postgresql *instancev1alpha1.Postgresql,
	postgresqlID, login, password string,
) {
	err := storeVaultCredentialsWithRetry(
		ctx, r.VaultClient, postgresqlID, login, password,
		r.Log, r.VaultAvailabilityRetries,
		r.VaultAvailabilityRetryDelay)
	if err != nil {
		r.Log.Errorw("Failed to store instance admin credentials in Vault",
			"error", err, "postgresqlID", postgresqlID)
		r.Recorder.Event(postgresql, corev1.EventTypeWarning,
			"CredentialStoreFailed",
			fmt.Sprintf("Failed to store instance admin credentials for %s: %v",
				postgresqlID, err))
		return
	}
	r.Log.Infow("Instance admin credentials stored in Vault",
		"postgresqlID", postgresqlID)
	r.Recorder.Event(postgresql, corev1.EventTypeNormal,
		"InstanceRegistered",
		fmt.Sprintf("Instance admin credentials stored for %s", postgresqlID))
}

// handleInstanceAdminPasswordRotation checks for and handles password rotation
func (r *PostgresqlReconciler) handleInstanceAdminPasswordRotation(
	ctx context.Context,
	postgresql *instancev1alpha1.Postgresql,
	externalInstance *instancev1alpha1.ExternalPostgresqlInstance,
	port int32, sslMode string,
) {
	newPassword, err := getInstanceAdminNewPasswordWithRetry(
		ctx, r.VaultClient, externalInstance.PostgresqlID,
		r.Log, r.VaultAvailabilityRetries,
		r.VaultAvailabilityRetryDelay)
	if err != nil {
		r.Log.Warnw("Failed to check for password rotation",
			"error", err, "postgresqlID", externalInstance.PostgresqlID)
		return
	}

	if newPassword == "" {
		// No rotation needed
		return
	}

	r.Log.Infow("Password rotation detected, changing instance admin password",
		"postgresqlID", externalInstance.PostgresqlID)

	// Get current credentials to connect
	login, currentPassword, err := getVaultCredentialsWithRetry(
		ctx, r.VaultClient, externalInstance.PostgresqlID,
		r.Log, r.VaultAvailabilityRetries,
		r.VaultAvailabilityRetryDelay)
	if err != nil {
		r.Log.Errorw("Failed to get current credentials for password rotation",
			"error", err, "postgresqlID", externalInstance.PostgresqlID)
		return
	}

	// Change password in PostgreSQL using the existing CreateOrUpdateUser function
	// which handles ALTER USER ... WITH PASSWORD
	pgErr := pg.ExecuteOperationWithRetry(ctx, func() error {
		return pg.CreateOrUpdateUser(
			ctx, externalInstance.Address, port,
			login, currentPassword, sslMode,
			login, newPassword,
		)
	}, r.Log, r.PostgresqlConnectionRetries,
		r.PostgresqlConnectionTimeout, "rotateInstanceAdminPassword")

	if pgErr != nil {
		r.Log.Errorw("Failed to change instance admin password in PostgreSQL",
			"error", pgErr, "postgresqlID", externalInstance.PostgresqlID)
		r.Recorder.Event(postgresql, corev1.EventTypeWarning,
			"PasswordRotationFailed",
			fmt.Sprintf("Failed to change password in PostgreSQL for %s: %v",
				externalInstance.PostgresqlID, pgErr))
		metrics.IncReconcileErrors("postgresql")
		metrics.IncVaultPasswordRotation("pg_failed")
		return
	}

	// PostgreSQL password changed successfully, now update Vault
	rotateErr := rotateInstanceAdminPasswordWithRetry(
		ctx, r.VaultClient, externalInstance.PostgresqlID, newPassword,
		r.Log, r.VaultAvailabilityRetries,
		r.VaultAvailabilityRetryDelay)
	if rotateErr != nil {
		r.Log.Errorw("Failed to rotate password in Vault after PostgreSQL change",
			"error", rotateErr, "postgresqlID", externalInstance.PostgresqlID)
		r.Recorder.Event(postgresql, corev1.EventTypeWarning,
			"VaultRotationFailed",
			fmt.Sprintf("Password changed in PostgreSQL but Vault update failed for %s: %v",
				externalInstance.PostgresqlID, rotateErr))
		metrics.IncVaultPasswordRotation("vault_failed")
		return
	}

	r.Log.Infow("Instance admin password rotation completed successfully",
		"postgresqlID", externalInstance.PostgresqlID)
	r.Recorder.Event(postgresql, corev1.EventTypeNormal,
		"PasswordRotated",
		fmt.Sprintf("Instance admin password rotated for %s", externalInstance.PostgresqlID))
	metrics.IncVaultPasswordRotation("success")
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
