package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Host          string              `mapstructure:"host"`
	Port          int                 `mapstructure:"port"`
	GinMode       string              `mapstructure:"gin_mode"`
	LogLevel      string              `mapstructure:"log_level"`
	UseMock       bool                `mapstructure:"use_mock"`
	App           AppConfig           `mapstructure:"app"`
	Bootstrap     BootstrapConfig     `mapstructure:"bootstrap"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Redis         RedisConfig         `mapstructure:"redis"`
	Runtime       RuntimeConfig       `mapstructure:"runtime"`
	Volcengine    VolcengineConfig    `mapstructure:"volcengine"`
	ComfyUIBridge ComfyUIBridgeConfig `mapstructure:"comfyui_bridge"`
	Minimax       MinimaxConfig       `mapstructure:"minimax"`
	KimiCoding    KimiCodingConfig    `mapstructure:"kimi_coding"`
	Security      SecurityConfig      `mapstructure:"security"`
	OAuth         OAuthConfig         `mapstructure:"oauth"`
	Monitoring    MonitoringConfig    `mapstructure:"monitoring"`
	Tasks         TasksConfig         `mapstructure:"tasks"`
}

type AppConfig struct {
	FrontendBaseURL string `mapstructure:"frontend_base_url"`
}

type RuntimeConfig struct {
	WorkerEnabled      bool          `mapstructure:"worker_enabled"`
	WorkerConcurrency  int           `mapstructure:"worker_concurrency"`
	QueueName          string        `mapstructure:"queue_name"`
	ExecutionTimeout   time.Duration `mapstructure:"execution_timeout"`
	RetryBackoff       time.Duration `mapstructure:"retry_backoff"`
	PollInitialBackoff time.Duration `mapstructure:"poll_initial_backoff"`
	PollBackoff        time.Duration `mapstructure:"poll_backoff"`
	PollTimeout        time.Duration `mapstructure:"poll_timeout"`
	MaxAttempts        int           `mapstructure:"max_attempts"`
}

type BootstrapConfig struct {
	SyncEnabled bool `mapstructure:"sync_enabled"`

	Identity   IdentityBootstrapConfig   `mapstructure:"identity"`
	Commercial CommercialBootstrapConfig `mapstructure:"commercial"`
	Runtime    RuntimeBootstrapConfig    `mapstructure:"runtime"`
	Storage    StorageBootstrapConfig    `mapstructure:"storage"`
}

type IdentityBootstrapConfig struct {
	DevSeedEnabled      bool                `mapstructure:"dev_seed_enabled"`
	ForceRotatePassword bool                `mapstructure:"force_rotate_password"`
	ForceAdminState     bool                `mapstructure:"force_admin_state"`
	DevAdmins           []BootstrapDevAdmin `mapstructure:"dev_admins"`
}

type BootstrapDevAdmin struct {
	Email            string `mapstructure:"email"`
	Password         string `mapstructure:"password"`
	PasswordEnv      string `mapstructure:"password_env"`
	OrganizationName string `mapstructure:"organization_name"`
	Role             string `mapstructure:"role"`
}

type CommercialBootstrapConfig struct {
	Products           []BootstrapProduct          `mapstructure:"products"`
	VisibleBaselines   []string                    `mapstructure:"visible_baselines"`
	CommercialEntities []BootstrapCommercialEntity `mapstructure:"commercial_entities"`
	BillingProfiles    []BootstrapBillingProfile   `mapstructure:"billing_profiles"`
	BillableItems      []BootstrapBillableItem     `mapstructure:"billable_items"`
}

type RuntimeBootstrapConfig struct {
	ProviderDefinitions []BootstrapRuntimeProviderDefinition `mapstructure:"provider_definitions"`
	ProductEndpoints    []BootstrapRuntimeProductEndpoint    `mapstructure:"product_endpoints"`
	ProviderBindings    []BootstrapRuntimeProviderBinding    `mapstructure:"provider_bindings"`
}

type StorageBootstrapConfig struct {
	Bindings []BootstrapStorageBinding `mapstructure:"bindings"`
}

type BootstrapProduct struct {
	Code      string `mapstructure:"code"`
	Name      string `mapstructure:"name"`
	Status    string `mapstructure:"status"`
	OwnerTeam string `mapstructure:"owner_team"`
	Metadata  string `mapstructure:"metadata"`
}

type BootstrapCommercialEntity struct {
	Code        string `mapstructure:"code"`
	Name        string `mapstructure:"name"`
	EntityType  string `mapstructure:"entity_type"`
	CountryCode string `mapstructure:"country_code"`
	Currency    string `mapstructure:"currency"`
	Status      string `mapstructure:"status"`
	Metadata    string `mapstructure:"metadata"`
}

type BootstrapBillingProfile struct {
	Code                 string `mapstructure:"code"`
	ProductCode          string `mapstructure:"product_code"`
	CommercialEntityCode string `mapstructure:"commercial_entity_code"`
	RegionScope          string `mapstructure:"region_scope"`
	Currency             string `mapstructure:"currency"`
	PricingStrategy      string `mapstructure:"pricing_strategy"`
	TaxStrategy          string `mapstructure:"tax_strategy"`
	Status               string `mapstructure:"status"`
	Metadata             string `mapstructure:"metadata"`
}

type BootstrapBillableItem struct {
	ProductCode     string `mapstructure:"product_code"`
	Code            string `mapstructure:"code"`
	Name            string `mapstructure:"name"`
	MeterUnit       string `mapstructure:"meter_unit"`
	BillingScope    string `mapstructure:"billing_scope"`
	SettlementMode  string `mapstructure:"settlement_mode"`
	PricingBehavior string `mapstructure:"pricing_behavior"`
	Status          string `mapstructure:"status"`
	Metadata        string `mapstructure:"metadata"`
}

type BootstrapRuntimeProviderDefinition struct {
	Code          string `mapstructure:"code"`
	Name          string `mapstructure:"name"`
	ProviderType  string `mapstructure:"provider_type"`
	Mode          string `mapstructure:"mode"`
	CredentialRef string `mapstructure:"credential_ref"`
	Capabilities  string `mapstructure:"capabilities"`
	Status        string `mapstructure:"status"`
	Metadata      string `mapstructure:"metadata"`
}

type BootstrapRuntimeProductEndpoint struct {
	ProductCode  string `mapstructure:"product_code"`
	CallbackKind string `mapstructure:"callback_kind"`
	BaseURL      string `mapstructure:"base_url"`
	Secret       string `mapstructure:"secret"`
	Status       string `mapstructure:"status"`
	Metadata     string `mapstructure:"metadata"`
}

type BootstrapStorageBinding struct {
	ProductCode  string `mapstructure:"product_code"`
	Category     string `mapstructure:"category"`
	ProviderCode string `mapstructure:"provider_code"`
	LocalBaseDir string `mapstructure:"local_base_dir"`
	Priority     int    `mapstructure:"priority"`
	Enabled      bool   `mapstructure:"enabled"`
	Metadata     string `mapstructure:"metadata"`
}

type BootstrapRuntimeProviderBinding struct {
	ProductCode   string `mapstructure:"product_code"`
	TaskType      string `mapstructure:"task_type"`
	ProviderCode  string `mapstructure:"provider_code"`
	Model         string `mapstructure:"model"`
	CredentialRef string `mapstructure:"credential_ref"`
	Priority      int    `mapstructure:"priority"`
	Enabled       bool   `mapstructure:"enabled"`
	Metadata      string `mapstructure:"metadata"`
}

type VolcengineConfig struct {
	BaseURL      string `mapstructure:"base_url"`
	APIKey       string `mapstructure:"api_key"`
	ImageModel   string `mapstructure:"image_model"`
	ImageSize    string `mapstructure:"image_size"`
	OutputFormat string `mapstructure:"output_format"`
	Watermark    bool   `mapstructure:"watermark"`
}

type ComfyUIBridgeConfig struct {
	Enabled             bool          `mapstructure:"enabled"`
	BaseURL             string        `mapstructure:"base_url"`
	APIKey              string        `mapstructure:"api_key"`
	RequestTimeout      time.Duration `mapstructure:"request_timeout"`
	CallbackBaseURL     string        `mapstructure:"callback_base_url"`
	DefaultWorkflowID   string        `mapstructure:"default_workflow_id"`
	DefaultOutputFormat string        `mapstructure:"default_output_format"`
}

type MinimaxConfig struct {
	BaseURL        string        `mapstructure:"base_url"`
	APIKey         string        `mapstructure:"api_key"`
	Model          string        `mapstructure:"model"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	MaxTokens      int           `mapstructure:"max_tokens"`
	Temperature    float64       `mapstructure:"temperature"`
}

type KimiCodingConfig struct {
	BaseURL        string        `mapstructure:"base_url"`
	APIKey         string        `mapstructure:"api_key"`
	Model          string        `mapstructure:"model"`
	RequestTimeout time.Duration `mapstructure:"request_timeout"`
	MaxTokens      int           `mapstructure:"max_tokens"`
	Temperature    float64       `mapstructure:"temperature"`
}

type TasksConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	ExpireInterval time.Duration `mapstructure:"expire_interval"`
	CycleInterval  time.Duration `mapstructure:"cycle_interval"`
}

type DatabaseConfig struct {
	Driver              string        `mapstructure:"driver"`
	Host                string        `mapstructure:"host"`
	Port                int           `mapstructure:"port"`
	User                string        `mapstructure:"user"`
	Password            string        `mapstructure:"password"`
	DBName              string        `mapstructure:"dbname"`
	SSLMode             string        `mapstructure:"sslmode"`
	MaxOpenConns        int           `mapstructure:"max_open_conns"`
	MaxIdleConns        int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime     time.Duration `mapstructure:"conn_max_lifetime"`
	SQLitePath          string        `mapstructure:"sqlite_path"`
	AutoMigrateEnabled  bool          `mapstructure:"auto_migrate_enabled"`
	AllowStartupMigrate bool          `mapstructure:"allow_startup_migrate_in_non_dev"`
}

type RedisConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type SecurityConfig struct {
	JWTSecret             string        `mapstructure:"jwt_secret"`
	JWTExpiration         time.Duration `mapstructure:"jwt_expiration"`
	MaxBodyBytes          int64         `mapstructure:"max_body_bytes"`
	RateLimitPerSecond    int           `mapstructure:"rate_limit_per_second"`
	RateLimitBurst        int           `mapstructure:"rate_limit_burst"`
	KongSharedSecret      string        `mapstructure:"kong_shared_secret"`
	InternalServiceSecret string        `mapstructure:"internal_service_secret"`
	EncryptionKey         string        `mapstructure:"encryption_key"`
}

type OAuthConfig struct {
	Google OAuthProviderConfig `mapstructure:"google"`
}

type OAuthProviderConfig struct {
	ClientID          string `mapstructure:"client_id"`
	ClientSecret      string `mapstructure:"client_secret"`
	RedirectURL       string `mapstructure:"redirect_url"`
	FrontendReturnURL string `mapstructure:"frontend_return_url"`
}

type MonitoringConfig struct {
	Metrics MetricsConfig `mapstructure:"metrics"`
	Tracing TracingConfig `mapstructure:"tracing"`
}

type MetricsConfig struct {
	Enabled          bool      `mapstructure:"enabled"`
	Port             int       `mapstructure:"port"`
	Path             string    `mapstructure:"path"`
	Namespace        string    `mapstructure:"namespace"`
	Subsystem        string    `mapstructure:"subsystem"`
	PushInterval     string    `mapstructure:"push_interval"`
	HistogramBuckets []float64 `mapstructure:"histogram_buckets"`
}

type TracingConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	ServiceName    string  `mapstructure:"service_name"`
	ServiceVersion string  `mapstructure:"service_version"`
	Environment    string  `mapstructure:"environment"`
	JaegerEndpoint string  `mapstructure:"jaeger_endpoint"`
	SampleRate     float64 `mapstructure:"sample_rate"`
	LogSpans       bool    `mapstructure:"log_spans"`
}

func Load(configFile string) (*Config, error) {
	if configFile == "" {
		configFile = "config.local"
	}

	v := viper.New()
	v.SetConfigName(strings.TrimSuffix(configFile, ".yaml"))
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/platform-service/")
	v.SetEnvPrefix("PLATFORM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := validateSecurityConfig(cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// insecureDefaultSecrets 列出不可用于非 debug 模式的已知默认密钥值。
var insecureDefaultSecrets = []string{
	"platform-dev-secret",
	"platform-kong-shared-secret",
	"platform-internal-secret",
	"platform-encryption-key-change-me",
}

func validateSecurityConfig(cfg Config) error {
	if strings.EqualFold(cfg.GinMode, "debug") {
		return nil
	}
	secrets := []struct {
		name  string
		value string
	}{
		{"security.jwt_secret", cfg.Security.JWTSecret},
		{"security.kong_shared_secret", cfg.Security.KongSharedSecret},
		{"security.internal_service_secret", cfg.Security.InternalServiceSecret},
		{"security.encryption_key", cfg.Security.EncryptionKey},
	}
	for _, s := range secrets {
		for _, insecure := range insecureDefaultSecrets {
			if s.value == insecure {
				return fmt.Errorf("SECURITY: %s is using insecure default value %q in non-debug mode; override it via config file or PLATFORM_%s env var",
					s.name, insecure, strings.ToUpper(strings.ReplaceAll(s.name, ".", "_")))
			}
		}
		if s.value == "" {
			return fmt.Errorf("SECURITY: %s must not be empty in non-debug mode", s.name)
		}
	}
	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("host", "0.0.0.0")
	v.SetDefault("port", 8095)
	v.SetDefault("gin_mode", "debug")
	v.SetDefault("log_level", "info")
	v.SetDefault("use_mock", false)
	v.SetDefault("app.frontend_base_url", "http://localhost:3000")
	v.SetDefault("bootstrap.sync_enabled", false)
	v.SetDefault("bootstrap.identity.dev_seed_enabled", false)
	v.SetDefault("bootstrap.identity.force_rotate_password", false)
	v.SetDefault("bootstrap.identity.force_admin_state", false)
	v.SetDefault("runtime.worker_enabled", true)
	v.SetDefault("runtime.worker_concurrency", 8)
	v.SetDefault("runtime.queue_name", "runtime:default")
	v.SetDefault("runtime.execution_timeout", "5m")
	v.SetDefault("runtime.retry_backoff", "15s")
	v.SetDefault("runtime.poll_initial_backoff", "2s")
	v.SetDefault("runtime.poll_backoff", "5s")
	v.SetDefault("runtime.poll_timeout", "5m")
	v.SetDefault("runtime.max_attempts", 3)
	v.SetDefault("volcengine.base_url", "https://ark.cn-beijing.volces.com/api/v3")
	v.SetDefault("volcengine.api_key", "")
	v.SetDefault("volcengine.image_model", "doubao-seedream-5-0-260128")
	v.SetDefault("volcengine.image_size", "2K")
	v.SetDefault("volcengine.output_format", "png")
	v.SetDefault("volcengine.watermark", false)
	v.SetDefault("comfyui_bridge.enabled", false)
	v.SetDefault("comfyui_bridge.base_url", "")
	v.SetDefault("comfyui_bridge.api_key", "")
	v.SetDefault("comfyui_bridge.request_timeout", "60s")
	v.SetDefault("comfyui_bridge.callback_base_url", "")
	v.SetDefault("comfyui_bridge.default_workflow_id", "")
	v.SetDefault("comfyui_bridge.default_output_format", "png")
	v.SetDefault("minimax.base_url", "https://api.minimaxi.com/v1")
	v.SetDefault("minimax.api_key", "")
	v.SetDefault("minimax.model", "MiniMax-M2.7")
	v.SetDefault("minimax.request_timeout", "60s")
	v.SetDefault("minimax.max_tokens", 2048)
	v.SetDefault("minimax.temperature", 0.2)
	v.SetDefault("kimi_coding.base_url", "https://api.kimi.com/coding")
	v.SetDefault("kimi_coding.api_key", "")
	v.SetDefault("kimi_coding.model", "kimi-k2.6")
	v.SetDefault("kimi_coding.request_timeout", "90s")
	v.SetDefault("kimi_coding.max_tokens", 4096)
	v.SetDefault("kimi_coding.temperature", 0.2)
	v.SetDefault("tasks.enabled", true)
	v.SetDefault("tasks.expire_interval", "1h")
	v.SetDefault("tasks.cycle_interval", "1h")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.host", "database")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "platform")
	v.SetDefault("database.password", "platformpassword")
	v.SetDefault("database.dbname", "platform")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.sqlite_path", filepath.Join("data", "platform.db"))
	v.SetDefault("database.auto_migrate_enabled", false)
	v.SetDefault("database.allow_startup_migrate_in_non_dev", false)
	v.SetDefault("redis.enabled", false)
	v.SetDefault("redis.host", "redis")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)
	v.SetDefault("redis.min_idle_conns", 5)
	v.SetDefault("redis.max_retries", 3)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("security.jwt_secret", "platform-dev-secret")
	v.SetDefault("security.jwt_expiration", "24h")
	v.SetDefault("security.max_body_bytes", 16*1024*1024)
	v.SetDefault("security.rate_limit_per_second", 100)
	v.SetDefault("security.rate_limit_burst", 200)
	v.SetDefault("security.kong_shared_secret", "platform-kong-shared-secret")
	v.SetDefault("security.internal_service_secret", "platform-internal-secret")
	v.SetDefault("security.encryption_key", "platform-encryption-key-change-me")
	v.SetDefault("oauth.google.client_id", "")
	v.SetDefault("oauth.google.client_secret", "")
	v.SetDefault("oauth.google.redirect_url", "")
	v.SetDefault("oauth.google.frontend_return_url", "")
	v.SetDefault("monitoring.metrics.enabled", true)
	v.SetDefault("monitoring.metrics.port", 9091)
	v.SetDefault("monitoring.metrics.path", "/metrics")
	v.SetDefault("monitoring.metrics.namespace", "platform")
	v.SetDefault("monitoring.metrics.subsystem", "service")
	v.SetDefault("monitoring.metrics.push_interval", "30s")
	v.SetDefault("monitoring.metrics.histogram_buckets", []float64{0.1, 0.5, 1, 2, 5, 10})
	v.SetDefault("monitoring.tracing.enabled", false)
	v.SetDefault("monitoring.tracing.service_name", "platform-service")
	v.SetDefault("monitoring.tracing.service_version", "1.0.0")
	v.SetDefault("monitoring.tracing.environment", "development")
	v.SetDefault("monitoring.tracing.jaeger_endpoint", "http://localhost:14268/api/traces")
	v.SetDefault("monitoring.tracing.sample_rate", 1.0)
	v.SetDefault("monitoring.tracing.log_spans", false)
}
