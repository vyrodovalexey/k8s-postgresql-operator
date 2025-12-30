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

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/cert"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/config"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/logging"
	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	instancev1alpha1 "github.com/vyrodovalexey/k8s-postgresql-operator/api/v1alpha1"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/controller"
	"github.com/vyrodovalexey/k8s-postgresql-operator/internal/vault"
	// +kubebuilder:scaffold:imports
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(instancev1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

// postgresqlValidator validates PostgreSQL resources
type postgresqlValidator struct {
	Client  client.Client
	decoder admission.Decoder
	log     *zap.SugaredLogger
}

// Handle handles the admission request for PostgreSQL resources
func (v *postgresqlValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var postgresql instancev1alpha1.Postgresql

	if err := v.decoder.Decode(req, &postgresql); err != nil {
		v.log.Errorw("Failed to decode PostgreSQL object", "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if postgresqlID is specified
	if postgresql.Spec.ExternalInstance == nil || postgresql.Spec.ExternalInstance.PostgresqlID == "" {
		// No postgresqlID specified, allow the request
		return admission.Allowed("No postgresqlID specified")
	}

	postgresqlID := postgresql.Spec.ExternalInstance.PostgresqlID

	v.log.Infow("Validating PostgreSQL resource", "name", postgresql.Name, "namespace", postgresql.Namespace, "postgresqlID", postgresqlID)

	// List all PostgreSQL resources across all namespaces in the cluster
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := v.Client.List(ctx, postgresqlList); err != nil {
		v.log.Errorw("Failed to list PostgreSQL resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check for duplicate postgresqlID across the entire cluster
	for _, existingPostgresql := range postgresqlList.Items {
		// Skip the current resource if this is an update
		if req.Operation == admissionv1.Update &&
			existingPostgresql.Name == postgresql.Name &&
			existingPostgresql.Namespace == postgresql.Namespace {
			continue
		}

		if existingPostgresql.Spec.ExternalInstance != nil &&
			existingPostgresql.Spec.ExternalInstance.PostgresqlID == postgresqlID {
			msg := fmt.Sprintf("PostgreSQL instance with postgresqlID %s already exists in namespace %s (instance: %s)",
				postgresqlID, existingPostgresql.Namespace, existingPostgresql.Name)
			v.log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID,
				"existing-namespace", existingPostgresql.Namespace, "existing-name", existingPostgresql.Name)
			return admission.Denied(msg)
		}
	}

	v.log.Infow("Validation passed", "name", postgresql.Name, "namespace", postgresql.Namespace, "postgresqlID", postgresqlID)
	return admission.Allowed("No duplicate postgresqlID found in cluster")
}

// userValidator validates User resources
type userValidator struct {
	Client          client.Client
	decoder         admission.Decoder
	log             *zap.SugaredLogger
	excludeUserList []string
}

// Handle handles the admission request for User resources
func (v *userValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var user instancev1alpha1.User

	if err := v.decoder.Decode(req, &user); err != nil {
		v.log.Errorw("Failed to decode User object", "error", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Check if postgresqlID and username are specified
	if user.Spec.PostgresqlID == "" {
		return admission.Allowed("No postgresqlID specified")
	}
	if user.Spec.Username == "" {
		return admission.Allowed("No username specified")
	}

	postgresqlID := user.Spec.PostgresqlID
	username := user.Spec.Username

	v.log.Infow("Validating User resource", "name", user.Name, "namespace", user.Namespace, "postgresqlID", postgresqlID, "username", username)

	// Check if username is in the Excludelist
	for _, excludelistedUser := range v.excludeUserList {
		if excludelistedUser == username {
			msg := fmt.Sprintf("Username %s is in the excludelist and cannot be created", username)
			v.log.Infow("Validation denied", "reason", msg, "username", username)
			return admission.Denied(msg)
		}
	}

	// First, verify that the postgresqlID exists in a PostgreSQL CRD object
	postgresqlList := &instancev1alpha1.PostgresqlList{}
	if err := v.Client.List(ctx, postgresqlList); err != nil {
		v.log.Errorw("Failed to list PostgreSQL resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	postgresqlExists := false
	for _, postgresql := range postgresqlList.Items {
		if postgresql.Spec.ExternalInstance != nil &&
			postgresql.Spec.ExternalInstance.PostgresqlID == postgresqlID {
			postgresqlExists = true
			break
		}
	}

	if !postgresqlExists {
		msg := fmt.Sprintf("PostgreSQL instance with postgresqlID %s does not exist in the cluster", postgresqlID)
		v.log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID)
		return admission.Denied(msg)
	}

	// List all User resources across all namespaces in the cluster
	userList := &instancev1alpha1.UserList{}
	if err := v.Client.List(ctx, userList); err != nil {
		v.log.Errorw("Failed to list User resources", "error", err)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	// Check for duplicate postgresqlID + username combination across the entire cluster
	for _, existingUser := range userList.Items {
		// Skip the current resource if this is an update
		if req.Operation == admissionv1.Update &&
			existingUser.Name == user.Name &&
			existingUser.Namespace == user.Namespace {
			continue
		}

		if existingUser.Spec.PostgresqlID == postgresqlID &&
			existingUser.Spec.Username == username {
			msg := fmt.Sprintf("User with postgresqlID %s and username %s already exists in namespace %s (instance: %s)",
				postgresqlID, username, existingUser.Namespace, existingUser.Name)
			v.log.Infow("Validation denied", "reason", msg, "postgresqlID", postgresqlID, "username", username,
				"existing-namespace", existingUser.Namespace, "existing-name", existingUser.Name)
			return admission.Denied(msg)
		}
	}

	v.log.Infow("Validation passed", "name", user.Name, "namespace", user.Namespace, "postgresqlID", postgresqlID, "username", username)
	return admission.Allowed("No duplicate postgresqlID and username combination found in cluster")
}

// getCurrentNamespace discovers the current namespace from the service account
// Returns the namespace or a default value if not found
func getCurrentNamespace(namespaceFile string) string {
	// Try to read from the service account namespace file (standard in Kubernetes pods)
	if data, err := os.ReadFile(namespaceFile); err == nil {
		if ns := strings.TrimSpace(string(data)); ns != "" {
			return ns
		}
	}

	// Default fallback
	return "k8s-postgresql-operator"
}

// updateWebhookConfigurationWithCABundle updates the ValidatingWebhookConfiguration with the CA bundle from the secret
func updateWebhookConfigurationWithCABundle(secretName, namespace string, config *rest.Config, webHookName string, lg *zap.SugaredLogger) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	ctx := context.Background()

	// Get the secret containing the certificate
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	caBundle, ok := secret.Data["tls.crt"]
	if !ok {
		return fmt.Errorf("tls.crt not found in secret %s", secretName)
	}

	// Get the ValidatingWebhookConfiguration
	webhookConfigName := webHookName
	webhookConfig, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, webhookConfigName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ValidatingWebhookConfiguration: %w", err)
	}

	// Update the CA bundle in the webhook configuration
	updated := false
	for i := range webhookConfig.Webhooks {
		if webhookConfig.Webhooks[i].Name == "postgresql-operator.vyrodovalexey.github.com" {
			webhookConfig.Webhooks[i].ClientConfig.CABundle = []byte(caBundle)
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("webhook 'postgresql-operator.vyrodovalexey.github.com' not found in ValidatingWebhookConfiguration")
	}

	// Update the ValidatingWebhookConfiguration
	_, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(ctx, webhookConfig, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ValidatingWebhookConfiguration: %w", err)
	}

	return nil
}

// nolint:gocyclo
func main() {
	// Создаем новый экземпляр конфигурации
	cfg := config.New()
	// Парсим настройки конфигурации
	ConfigParser(cfg)

	var tlsOpts []func(*tls.Config)

	lg := logging.NewLogging(zapcore.InfoLevel)

	// Set the logger for controller-runtime
	// Convert SugaredLogger to Logger and wrap it for controller-runtime
	zapLogger := lg.Desugar()
	ctrl.SetLogger(zapr.NewLogger(zapLogger))

	lg.Infow("Server starting with: ")

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		lg.Infow("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts = append(tlsOpts, disableHTTP2)

	// Create watcher for webhook certificates
	var webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	// Generate or use provided webhook certificates
	var webhookCertPath, webhookKeyPath string
	if len(cfg.WebhookCertPath) > 0 {
		// Use provided certificates
		webhookCertPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertName)
		webhookKeyPath = filepath.Join(cfg.WebhookCertPath, cfg.WebhookCertKey)
		lg.Infow("Using provided webhook certificates",
			"webhook-cert-path", cfg.WebhookCertPath, "webhook-cert-name", cfg.WebhookCertName, "webhook-cert-key", cfg.WebhookCertKey)
	} else {
		// Generate self-signed certificates and store in Kubernetes Secret
		tempCertDir, err := os.MkdirTemp("", "webhook-certs-")
		if err != nil {
			lg.Error(err, "Failed to create temporary directory for webhook certificates")
			os.Exit(1)
		}
		lg.Infow("Generating self-signed webhook certificates and storing in Secret", "cert-dir", tempCertDir)

		// Get service name from environment or use default

		// Discover current namespace
		namespace := getCurrentNamespace(cfg.K8sNamespacePath)

		secretName := fmt.Sprintf("%s-webhook-cert", cfg.WebhookK8sServiceName)

		// Get Kubernetes config
		k8sConfig := ctrl.GetConfigOrDie()
		_, webhookCertPath, webhookKeyPath, err = cert.GenerateSelfSignedCertAndStoreInSecret(cfg.WebhookK8sServiceName, namespace, secretName, tempCertDir, k8sConfig)
		if err != nil {
			lg.Error(err, "Failed to generate self-signed webhook certificates")
			os.Exit(1)
		}
		lg.Infow("Webhook certificates generated and stored in Secret", "secret-name", secretName, "cert-path", webhookCertPath, "key-path", webhookKeyPath)

		// Update ValidatingWebhookPostgresql  with CA bundle from secret
		if err := updateWebhookConfigurationWithCABundle(secretName, namespace, k8sConfig, cfg.K8sWebhookNamePostgresql, lg); err != nil {
			lg.Error(err, "Failed to update ValidatingWebhookConfiguration with CA bundle", "secret-name", secretName)
			// Don't exit, as the webhook might still work if the CA bundle is set manually
		} else {
			lg.Infow("Successfully updated ValidatingWebhookConfiguration with CA bundle", "secret-name", secretName)
		}
		// Update ValidatingWebhookUser with CA bundle from secret
		if err := updateWebhookConfigurationWithCABundle(secretName, namespace, k8sConfig, cfg.K8sWebhookNameUser, lg); err != nil {
			lg.Error(err, "Failed to update ValidatingWebhookConfiguration with CA bundle", "secret-name", secretName)
			// Don't exit, as the webhook might still work if the CA bundle is set manually
		} else {
			lg.Infow("Successfully updated ValidatingWebhookConfiguration with CA bundle", "secret-name", secretName)
		}
	}

	// Initialize webhook certificate watcher
	var err error
	webhookCertWatcher, err = certwatcher.New(webhookCertPath, webhookKeyPath)
	if err != nil {
		lg.Error(err, "Failed to initialize webhook certificate watcher")
		os.Exit(1)
	}

	webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
		config.GetCertificate = webhookCertWatcher.GetCertificate
	})

	webhookServer := webhook.NewServer(webhook.Options{
		Host:    cfg.WebhookServerAddr,
		Port:    cfg.WebhookServerPort,
		TLSOpts: webhookTLSOpts,
	})
	lg.Infow("Webhook server configured", "host", cfg.WebhookServerAddr, "port", cfg.WebhookServerPort, "cert-path", webhookCertPath, "key-path", webhookKeyPath)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"}, // Disable metrics server
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: cfg.ProbeAddr,
		LeaderElection:         cfg.EnableLeaderElection,
		LeaderElectionID:       "9e7b1538.postgresql-operator.vyrodovalexey.github.com",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		lg.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Initialize Vault client if environment variables are set
	var vaultClient *vault.Client
	if cfg.VaultAddr != "" {
		vc, err := vault.NewClient(cfg.VaultAddr, cfg.VaultRole, cfg.K8sTokenPath, cfg.VaultMountPoint, cfg.VaultSecretPath)
		if err != nil {
			lg.Error(err, "unable to create Vault client")
			os.Exit(1)
		} else {
			vaultClient = vc
			lg.Infow("Vault client initialized successfully")
		}
	} else {
		lg.Infow("VAULT_ADDR not set, Vault integration disabled")
	}

	if err := (&controller.PostgresqlReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		VaultClient: vaultClient,
		Log:         lg,
	}).SetupWithManager(mgr); err != nil {
		lg.Error(err, "unable to create controller Postgresql")
		os.Exit(1)
	}

	if err := (&controller.UserReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		VaultClient: vaultClient,
		Log:         lg,
	}).SetupWithManager(mgr); err != nil {
		lg.Error(err, "unable to create controller User")
		os.Exit(1)
	}

	if err := (&controller.DatabaseReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		VaultClient: vaultClient,
		Log:         lg,
	}).SetupWithManager(mgr); err != nil {
		lg.Error(err, "unable to create controller Database")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	if webhookCertWatcher != nil {
		lg.Infow("Adding webhook certificate watcher to postgresql operator")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			lg.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		lg.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		lg.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	// Register /postgresql_validate webhook handler
	postgresqlValidateHandler := &postgresqlValidator{
		Client:  mgr.GetClient(),
		decoder: admission.NewDecoder(scheme),
		log:     lg,
	}
	webhookServer.Register("/postgresqlvalidate", &webhook.Admission{Handler: postgresqlValidateHandler})
	lg.Infow("Registered /postgresqlvalidate webhook handler")

	// Register /user_validate webhook handler
	// Parse exclude user list from comma-separated string
	excludeUserList := []string{}
	if cfg.ExcludeUserList != "" {
		excludeUserList = strings.Split(cfg.ExcludeUserList, ",")
		// Trim whitespace from each username
		for i, user := range excludeUserList {
			excludeUserList[i] = strings.TrimSpace(user)
		}
		lg.Infow("Exclude user list configured", "users", excludeUserList)
	}
	userValidateHandler := &userValidator{
		Client:          mgr.GetClient(),
		decoder:         admission.NewDecoder(scheme),
		log:             lg,
		excludeUserList: excludeUserList,
	}
	webhookServer.Register("/uservalidate", &webhook.Admission{Handler: userValidateHandler})
	lg.Infow("Registered /uservalidate webhook handler")

	if err := mgr.Add(webhookServer); err != nil {
		lg.Error(err, "unable to set up webhook server")
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		lg.Error(err, "problem starting postgresql operator")
		os.Exit(1)
	}

}
