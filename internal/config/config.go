package config

const (
	defaultWebhookCertPath          = ""
	defaultWebhookCertName          = "tls.crt"
	defaultWebhookCertKey           = "tls.key"
	defaultWebhookServerPort        = 8443
	defaultWebhookServerAddr        = "0.0.0.0"
	defaultWebhookK8sServiceName    = "k8s-postgresql-operator-controller-service"
	defaultK8sWebhookNamePostgresql = "k8s-postgresql-operator-validating-webhook-postgresql"
	defaultK8sWebhookNameUser       = "k8s-postgresql-operator-validating-webhook-user"
	defaultK8sWebhookNameDatabase   = "k8s-postgresql-operator-validating-webhook-database"
	defaultK8sWebhookNameGrant      = "k8s-postgresql-operator-validating-webhook-grant"
	defaultK8sWebhookNameRoleGroup  = "k8s-postgresql-operator-validating-webhook-rolegroup"
	defaultK8sWebhookNameSchema     = "k8s-postgresql-operator-validating-webhook-schema"
	defaultEnableLeaderElection     = false
	defaultProbeAddr                = ":8081"
	defaultVaultAddr                = "http://0.0.0.0:8200"
	defaultVaultRole                = "role"
	defaultVaultMountPoint          = "secret"
	defaultVaultSecretPath          = "pdb"
	defaultK8sTokenPath             = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	defaultK8SNamespacePath         = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	defaultExcludeUserList          = "postgres"
)

// Config Структура для хранения конфигурации
type Config struct {
	WebhookCertPath          string `env:"WEBHOOK_CERT_PATH"`
	WebhookCertName          string `env:"WEBHOOK_CERT_NAME"`
	WebhookCertKey           string `env:"WEBHOOK_CERT_KEY"`
	WebhookServerPort        int    `env:"WEBHOOK_SERVER_PORT"`
	WebhookServerAddr        string `env:"WEBHOOK_SERVER_ADDR"`
	WebhookK8sServiceName    string `env:"WEBHOOK_K8S_SERVICE_NAME"`
	K8sWebhookNamePostgresql string `env:"K8S_WEBHOOK_NAME_POSTGRESQL"`
	K8sWebhookNameUser       string `env:"K8S_WEBHOOK_NAME_USER"`
	K8sWebhookNameDatabase   string `env:"K8S_WEBHOOK_NAME_DATABASE"`
	K8sWebhookNameGrant      string `env:"K8S_WEBHOOK_NAME_GRANT"`
	K8sWebhookNameRoleGroup  string `env:"K8S_WEBHOOK_NAME_ROLEGROUP"`
	K8sWebhookNameSchema     string `env:"K8S_WEBHOOK_NAME_SCHEMA"`
	EnableLeaderElection     bool   `env:"ENABLE_LEADER_ELECTION"`
	ProbeAddr                string `env:"PROBE_ADDR"`
	VaultAddr                string `env:"VAULT_ADDR"`
	VaultRole                string `env:"VAULT_ROLE"`
	VaultMountPoint          string `env:"VAULT_MOUNT_POINT"`
	VaultSecretPath          string `env:"VAULT_SECRET_PATH"`
	K8sTokenPath             string `env:"K8S_TOKEN_PATH"`
	K8sNamespacePath         string `env:"K8S_NAMESPACE_PATH"`
	ExcludeUserList          string `env:"EXCLUDE_USER_LIST"`
}

// New Функция для создания нового экземпляра конфигурации
func New() *Config {
	return &Config{
		defaultWebhookCertPath,
		defaultWebhookCertName,
		defaultWebhookCertKey,
		defaultWebhookServerPort,
		defaultWebhookServerAddr,
		defaultWebhookK8sServiceName,
		defaultK8sWebhookNamePostgresql,
		defaultK8sWebhookNameUser,
		defaultK8sWebhookNameDatabase,
		defaultK8sWebhookNameGrant,
		defaultK8sWebhookNameRoleGroup,
		defaultK8sWebhookNameSchema,
		defaultEnableLeaderElection,
		defaultProbeAddr,
		defaultVaultAddr,
		defaultVaultRole,
		defaultVaultMountPoint,
		defaultVaultSecretPath,
		defaultK8sTokenPath,
		defaultK8SNamespacePath,
		defaultExcludeUserList,
	}
}
