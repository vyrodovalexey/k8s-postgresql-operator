package config

const (
	defaultMetricsAddr          = "0"
	defaultMetricsCertPath      = ""
	defaultMetricsCertName      = "tls.crt"
	defaultMetricsCertKey       = "tls.key"
	defaultWebhookCertPath      = ""
	defaultWebhookCertName      = "tls.crt"
	defaultWebhookCertKey       = "tls.key"
	defaultEnableLeaderElection = false
	defaultProbeAddr            = ":8081"
	defaultSecureMetrics        = true
	defaultEnableHTTP2          = false
	defaultVaultAddr            = "http://0.0.0.0:8200"
	defaultVaultRole            = "role"
	defaultVaultMountPoint      = "secret"
	defaultVaultSecretPath      = "pdb"
	defaultVaultK8sTokenPath    = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

// Config Структура для хранения конфигурации
type Config struct {
	MetricsAddr          string `env:"METRICS_ADDRESS"`
	MetricsCertPath      string `env:"METRICS_CERT_PATH"`
	MetricsCertName      string `env:"METRICS_CERT_NAME"`
	MetricsCertKey       string `env:"METRICS_CERT_KEY"`
	WebhookCertPath      string `env:"WEBHOOK_CERT_PATH"`
	WebhookCertName      string `env:"WEBHOOK_CERT_NAME"`
	WebhookCertKey       string `env:"WEBHOOK_CERT_KEY"`
	EnableLeaderElection bool   `env:"ENABLE_LEADER_ELECTION"`
	ProbeAddr            string `env:"PROBE_ADDR"`
	SecureMetrics        bool   `env:"SECURE_METRICS"`
	EnableHTTP2          bool   `env:"ENABLE_HTTP2"`
	VaultAddr            string `env:"VAULT_ADDR"`
	VaultRole            string `env:"VAULT_ROLE"`
	VaultMountPoint      string `env:"VAULT_MOUNT_POINT"`
	VaultSecretPath      string `env:"VAULT_SECRET_PATH"`
	VaultK8sTokenPath    string `env:"VAULT_K8S_TOKEN_PATH"`
}

// New Функция для создания нового экземпляра конфигурации
func New() *Config {
	return &Config{
		defaultMetricsAddr,
		defaultMetricsCertPath,
		defaultMetricsCertName,
		defaultMetricsCertKey,
		defaultWebhookCertPath,
		defaultWebhookCertName,
		defaultWebhookCertKey,
		defaultEnableLeaderElection,
		defaultProbeAddr,
		defaultSecureMetrics,
		defaultEnableHTTP2,
		defaultVaultAddr,
		defaultVaultRole,
		defaultVaultMountPoint,
		defaultVaultSecretPath,
		defaultVaultK8sTokenPath,
	}
}
