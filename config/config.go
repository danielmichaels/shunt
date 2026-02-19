// file: config/config.go

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Default timeout and configuration constants
const (
	// DefaultNATSURL is the default NATS server URL
	DefaultNATSURL = "nats://localhost:4222"

	// DefaultReconnectWait is the default delay between NATS reconnection attempts
	DefaultReconnectWait = 50 * time.Millisecond

	// DefaultFetchTimeout is the default timeout for JetStream fetch operations
	DefaultFetchTimeout = 5 * time.Second

	// DefaultAckWaitTimeout is the default timeout for message acknowledgement
	DefaultAckWaitTimeout = 30 * time.Second

	// DefaultPublishAckTimeout is the default timeout for publish acknowledgement
	DefaultPublishAckTimeout = 5 * time.Second

	// DefaultRetryBaseDelay is the default base delay for publish retries
	DefaultRetryBaseDelay = 50 * time.Millisecond

	// DefaultHTTPReadTimeout is the default read timeout for HTTP server
	DefaultHTTPReadTimeout = 30 * time.Second

	// DefaultHTTPWriteTimeout is the default write timeout for HTTP server
	DefaultHTTPWriteTimeout = 30 * time.Second

	// DefaultHTTPIdleTimeout is the default idle timeout for HTTP server
	DefaultHTTPIdleTimeout = 120 * time.Second

	// DefaultHTTPShutdownGracePeriod is the default graceful shutdown period for HTTP server
	DefaultHTTPShutdownGracePeriod = 30 * time.Second

	// DefaultHTTPClientTimeout is the default timeout for HTTP client requests
	DefaultHTTPClientTimeout = 30 * time.Second

	// DefaultHTTPIdleConnTimeout is the default idle connection timeout for HTTP client
	DefaultHTTPIdleConnTimeout = 90 * time.Second

	// DefaultMaxHeaderBytes is the default maximum header size (1MB)
	DefaultMaxHeaderBytes = 1 << 20
)

// Default numeric configuration values
const (
	// DefaultWorkerCount is the default number of consumer workers
	DefaultWorkerCount = 2

	// DefaultFetchBatchSize is the default number of messages to fetch per batch
	DefaultFetchBatchSize = 1

	// DefaultMaxDeliver is the default maximum delivery attempts per message
	DefaultMaxDeliver = 3

	// DefaultMaxAckPending is the default maximum pending acknowledgements
	DefaultMaxAckPending = 1000

	// DefaultPublishMaxRetries is the default maximum publish retry attempts
	DefaultPublishMaxRetries = 3

	// DefaultMaxIdleConns is the default maximum idle HTTP connections
	DefaultMaxIdleConns = 100

	// DefaultMaxIdleConnsPerHost is the default maximum idle connections per host
	DefaultMaxIdleConnsPerHost = 10

	// DefaultForEachMaxIterations is the default maximum forEach iterations
	DefaultForEachMaxIterations = 100

	// DefaultInboundQueueSize is the default HTTP inbound queue size
	DefaultInboundQueueSize = 1000
)

// Validation limits
const (
	// MaxWorkerCount is the maximum allowed worker count
	MaxWorkerCount = 1000

	// MaxFetchBatchSize is the maximum allowed fetch batch size
	MaxFetchBatchSize = 10000

	// MaxAckPending is the maximum allowed pending acknowledgements
	MaxAckPending = 100000

	// MaxForEachIterations is the maximum allowed forEach iterations
	MaxForEachIterations = 10000

	// MaxInboundQueueSize is the maximum allowed inbound queue size
	MaxInboundQueueSize = 100000
)

// Default string configuration values
const (
	// DefaultConsumerPrefix is the default prefix for consumer names
	DefaultConsumerPrefix = "shunt"

	// DefaultPublishMode is the default NATS publish mode
	DefaultPublishMode = "jetstream"

	// DefaultDeliverPolicy is the default consumer deliver policy
	DefaultDeliverPolicy = "new"

	// DefaultReplayPolicy is the default consumer replay policy
	DefaultReplayPolicy = "instant"

	// DefaultLogLevel is the default logging level
	DefaultLogLevel = "info"

	// DefaultLogEncoding is the default log encoding format
	DefaultLogEncoding = "json"

	// DefaultLogOutput is the default log output destination
	DefaultLogOutput = "stdout"

	// DefaultMetricsAddress is the default metrics server address
	DefaultMetricsAddress = ":2112"

	// DefaultMetricsPath is the default metrics endpoint path
	DefaultMetricsPath = "/metrics"

	// DefaultHTTPServerAddress is the default HTTP server address
	DefaultHTTPServerAddress = ":8080"

	// DefaultPublicKeyHeader is the default header for NKey public key
	DefaultPublicKeyHeader = "Nats-Public-Key"

	// DefaultSignatureHeader is the default header for NKey signature
	DefaultSignatureHeader = "Nats-Signature"
)

// Config represents the unified configuration for shunt
type Config struct {
	NATS        NATSConfig        `json:"nats" yaml:"nats" mapstructure:"nats"`
	HTTP        HTTPConfig        `json:"http,omitempty" yaml:"http,omitempty" mapstructure:"http"`
	Logging     LogConfig         `json:"logging" yaml:"logging" mapstructure:"logging"`
	Metrics     MetricsConfig     `json:"metrics" yaml:"metrics" mapstructure:"metrics"`
	KV          KVConfig          `json:"kv" yaml:"kv" mapstructure:"kv"`
	Rules       RulesConfig       `json:"rules" yaml:"rules" mapstructure:"rules"`
	Security    SecurityConfig    `json:"security" yaml:"security" mapstructure:"security"`
	ForEach     ForEachConfig     `json:"forEach" yaml:"forEach" mapstructure:"forEach"`
	Gateway     GatewayConfig     `json:"gateway" yaml:"gateway" mapstructure:"gateway"`
	AuthManager AuthManagerConfig `json:"authManager" yaml:"authManager" mapstructure:"authManager"`
}

// GatewayConfig controls the optional HTTP gateway subsystem.
// When enabled, the gateway uses the HTTP config section for server/client settings.
type GatewayConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
}

// AuthManagerConfig controls the optional authentication manager subsystem
type AuthManagerConfig struct {
	Enabled   bool                    `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Storage   AuthManagerStorage      `json:"storage" yaml:"storage" mapstructure:"storage"`
	Providers []AuthManagerProvider   `json:"providers" yaml:"providers" mapstructure:"providers"`
}

// AuthManagerStorage defines where to store tokens
type AuthManagerStorage struct {
	Bucket    string `json:"bucket" yaml:"bucket" mapstructure:"bucket"`
	KeyPrefix string `json:"keyPrefix" yaml:"keyPrefix" mapstructure:"keyPrefix"`
}

// AuthManagerProvider defines an authentication provider
type AuthManagerProvider struct {
	ID            string            `json:"id" yaml:"id" mapstructure:"id"`
	Type          string            `json:"type" yaml:"type" mapstructure:"type"`
	KVKey         string            `json:"kvKey" yaml:"kvKey" mapstructure:"kvKey"`
	RefreshBefore string            `json:"refreshBefore" yaml:"refreshBefore" mapstructure:"refreshBefore"`
	RefreshEvery  string            `json:"refreshEvery" yaml:"refreshEvery" mapstructure:"refreshEvery"`
	TokenURL      string            `json:"tokenUrl" yaml:"tokenUrl" mapstructure:"tokenUrl"`
	ClientID      string            `json:"clientId" yaml:"clientId" mapstructure:"clientId"`
	ClientSecret  string            `json:"clientSecret" yaml:"clientSecret" mapstructure:"clientSecret"`
	Scopes        []string          `json:"scopes" yaml:"scopes" mapstructure:"scopes"`
	AuthURL       string            `json:"authUrl" yaml:"authUrl" mapstructure:"authUrl"`
	Method        string            `json:"method" yaml:"method" mapstructure:"method"`
	Headers       map[string]string `json:"headers" yaml:"headers" mapstructure:"headers"`
	Body          string            `json:"body" yaml:"body" mapstructure:"body"`
	TokenPath     string            `json:"tokenPath" yaml:"tokenPath" mapstructure:"tokenPath"`
}

type RulesConfig struct {
	KVBucket string `json:"kvBucket" yaml:"kvBucket" mapstructure:"kvBucket"`
}

// ForEachConfig contains configuration for array iteration operations
type ForEachConfig struct {
	MaxIterations int `json:"maxIterations" yaml:"maxIterations" mapstructure:"maxIterations"`
}

// HTTPConfig contains HTTP server and client configuration for http-gateway
type HTTPConfig struct {
	Server HTTPServerConfig `json:"server" yaml:"server" mapstructure:"server"`
	Client HTTPClientConfig `json:"client" yaml:"client" mapstructure:"client"`
}

// HTTPServerConfig configures the inbound HTTP server
type HTTPServerConfig struct {
	Address             string        `json:"address" yaml:"address" mapstructure:"address"`
	ReadTimeout         time.Duration `json:"readTimeout" yaml:"readTimeout" mapstructure:"readTimeout"`
	WriteTimeout        time.Duration `json:"writeTimeout" yaml:"writeTimeout" mapstructure:"writeTimeout"`
	IdleTimeout         time.Duration `json:"idleTimeout" yaml:"idleTimeout" mapstructure:"idleTimeout"`
	MaxHeaderBytes      int           `json:"maxHeaderBytes" yaml:"maxHeaderBytes" mapstructure:"maxHeaderBytes"`
	ShutdownGracePeriod time.Duration `json:"shutdownGracePeriod" yaml:"shutdownGracePeriod" mapstructure:"shutdownGracePeriod"`
	InboundWorkerCount  int           `json:"inboundWorkerCount" yaml:"inboundWorkerCount" mapstructure:"inboundWorkerCount"`
	InboundQueueSize    int           `json:"inboundQueueSize" yaml:"inboundQueueSize" mapstructure:"inboundQueueSize"`
}

// HTTPClientConfig configures the outbound HTTP client
type HTTPClientConfig struct {
	Timeout             time.Duration `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	MaxIdleConns        int           `json:"maxIdleConns" yaml:"maxIdleConns" mapstructure:"maxIdleConns"`
	MaxIdleConnsPerHost int           `json:"maxIdleConnsPerHost" yaml:"maxIdleConnsPerHost" mapstructure:"maxIdleConnsPerHost"`
	IdleConnTimeout     time.Duration `json:"idleConnTimeout" yaml:"idleConnTimeout" mapstructure:"idleConnTimeout"`
	TLS                 HTTPClientTLS `json:"tls,omitempty" yaml:"tls,omitempty" mapstructure:"tls"`
}

// HTTPClientTLS configures TLS settings for the outbound HTTP client
type HTTPClientTLS struct {
	InsecureSkipVerify bool `json:"insecureSkipVerify" yaml:"insecureSkipVerify" mapstructure:"insecureSkipVerify"`
}

// NATSConfig contains NATS connection and JetStream configuration
type NATSConfig struct {
	URLs      []string `json:"urls" yaml:"urls" mapstructure:"urls"`
	Username  string   `json:"username" yaml:"username" mapstructure:"username"`
	Password  string   `json:"password" yaml:"password" mapstructure:"password"`
	Token     string   `json:"token" yaml:"token" mapstructure:"token"`
	NKey      string   `json:"nkey" yaml:"nkey" mapstructure:"nkey"`
	CredsFile string   `json:"credsFile" yaml:"credsFile" mapstructure:"credsFile"`

	TLS struct {
		Enable   bool   `json:"enable" yaml:"enable" mapstructure:"enable"`
		CertFile string `json:"certFile" yaml:"certFile" mapstructure:"certFile"`
		KeyFile  string `json:"keyFile" yaml:"keyFile" mapstructure:"keyFile"`
		CAFile   string `json:"caFile" yaml:"caFile" mapstructure:"caFile"`
		Insecure bool   `json:"insecure" yaml:"insecure" mapstructure:"insecure"`
	} `json:"tls" yaml:"tls" mapstructure:"tls"`

	Consumers  ConsumerConfig   `json:"consumers" yaml:"consumers" mapstructure:"consumers"`
	Connection ConnectionConfig `json:"connection" yaml:"connection" mapstructure:"connection"`
	Publish    PublishConfig    `json:"publish" yaml:"publish" mapstructure:"publish"`
}

// ConsumerConfig contains JetStream consumer configuration
type ConsumerConfig struct {
	ConsumerPrefix string        `json:"consumerPrefix" yaml:"consumerPrefix" mapstructure:"consumerPrefix"`
	WorkerCount    int           `json:"workerCount" yaml:"workerCount" mapstructure:"workerCount"`
	FetchBatchSize int           `json:"fetchBatchSize" yaml:"fetchBatchSize" mapstructure:"fetchBatchSize"`
	FetchTimeout   time.Duration `json:"fetchTimeout" yaml:"fetchTimeout" mapstructure:"fetchTimeout"`
	MaxAckPending  int           `json:"maxAckPending" yaml:"maxAckPending" mapstructure:"maxAckPending"`
	AckWaitTimeout time.Duration `json:"ackWaitTimeout" yaml:"ackWaitTimeout" mapstructure:"ackWaitTimeout"`
	MaxDeliver     int           `json:"maxDeliver" yaml:"maxDeliver" mapstructure:"maxDeliver"`
	DeliverPolicy  string        `json:"deliverPolicy" yaml:"deliverPolicy" mapstructure:"deliverPolicy"`
	ReplayPolicy   string        `json:"replayPolicy" yaml:"replayPolicy" mapstructure:"replayPolicy"`
}

// ConnectionConfig contains NATS connection settings
type ConnectionConfig struct {
	MaxReconnects int           `json:"maxReconnects" yaml:"maxReconnects" mapstructure:"maxReconnects"`
	ReconnectWait time.Duration `json:"reconnectWait" yaml:"reconnectWait" mapstructure:"reconnectWait"`
}

// PublishConfig contains NATS publish configuration
type PublishConfig struct {
	Mode           string        `json:"mode" yaml:"mode" mapstructure:"mode"`
	AckTimeout     time.Duration `json:"ackTimeout" yaml:"ackTimeout" mapstructure:"ackTimeout"`
	MaxRetries     int           `json:"maxRetries" yaml:"maxRetries" mapstructure:"maxRetries"`
	RetryBaseDelay time.Duration `json:"retryBaseDelay" yaml:"retryBaseDelay" mapstructure:"retryBaseDelay"`
}

// LogConfig contains logging configuration
type LogConfig struct {
	Level      string `json:"level" yaml:"level" mapstructure:"level"`
	Encoding   string `json:"encoding" yaml:"encoding" mapstructure:"encoding"`
	OutputPath string `json:"outputPath" yaml:"outputPath" mapstructure:"outputPath"`
}

// MetricsConfig contains metrics server configuration
type MetricsConfig struct {
	Enabled        bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	Address        string `json:"address" yaml:"address" mapstructure:"address"`
	Path           string `json:"path" yaml:"path" mapstructure:"path"`
	UpdateInterval string `json:"updateInterval" yaml:"updateInterval" mapstructure:"updateInterval"`
}

// KVConfig contains Key-Value store configuration
type KVConfig struct {
	Enabled       bool     `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	AutoProvision bool     `json:"autoProvision" yaml:"autoProvision" mapstructure:"autoProvision"`
	Buckets       []string `json:"buckets" yaml:"buckets" mapstructure:"buckets"`
	LocalCache    struct {
		Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	} `json:"localCache" yaml:"localCache" mapstructure:"localCache"`
}

// SecurityConfig contains security-related configuration
type SecurityConfig struct {
	Verification VerificationConfig `json:"verification" yaml:"verification" mapstructure:"verification"`
}

// VerificationConfig contains signature verification settings
type VerificationConfig struct {
	Enabled         bool   `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	PublicKeyHeader string `json:"publicKeyHeader" yaml:"publicKeyHeader" mapstructure:"publicKeyHeader"`
	SignatureHeader string `json:"signatureHeader" yaml:"signatureHeader" mapstructure:"signatureHeader"`
}

// Load reads configuration using Viper, supporting file, env vars, and flags.
// If v is nil, a fresh viper instance is created. Pass an existing viper with
// bound pflags to enable CLI flag overrides.
func Load(path string, v *viper.Viper) (*Config, error) {
	if v == nil {
		v = viper.New()
	}

	v.SetConfigFile(path)
	ext := filepath.Ext(path)
	v.SetConfigType(strings.TrimPrefix(ext, "."))

	v.SetEnvPrefix("SHUNT")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.BindEnv("nats.urls", "SHUNT_NATS_URL")
	v.BindEnv("nats.credsFile", "SHUNT_NATS_CREDS")
	v.BindEnv("nats.nkey", "SHUNT_NATS_NKEY")

	setViperDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	applyConditionalDefaults(&config)

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}


// setViperDefaults registers all config keys with viper so that AutomaticEnv
// can resolve environment variables during Unmarshal. Without this, viper's
// key registry is empty for keys absent from the YAML file.
func setViperDefaults(v *viper.Viper) {
	// NATS
	v.SetDefault("nats.urls", []string{DefaultNATSURL})
	v.SetDefault("nats.username", "")
	v.SetDefault("nats.password", "")
	v.SetDefault("nats.token", "")
	v.SetDefault("nats.nkey", "")
	v.SetDefault("nats.credsFile", "")
	v.SetDefault("nats.tls.enable", false)
	v.SetDefault("nats.tls.certFile", "")
	v.SetDefault("nats.tls.keyFile", "")
	v.SetDefault("nats.tls.caFile", "")
	v.SetDefault("nats.tls.insecure", false)
	v.SetDefault("nats.connection.maxReconnects", -1)
	v.SetDefault("nats.connection.reconnectWait", DefaultReconnectWait)

	// Consumers
	v.SetDefault("nats.consumers.consumerPrefix", DefaultConsumerPrefix)
	v.SetDefault("nats.consumers.workerCount", DefaultWorkerCount)
	v.SetDefault("nats.consumers.fetchBatchSize", DefaultFetchBatchSize)
	v.SetDefault("nats.consumers.fetchTimeout", DefaultFetchTimeout)
	v.SetDefault("nats.consumers.maxAckPending", DefaultMaxAckPending)
	v.SetDefault("nats.consumers.ackWaitTimeout", DefaultAckWaitTimeout)
	v.SetDefault("nats.consumers.maxDeliver", DefaultMaxDeliver)
	v.SetDefault("nats.consumers.deliverPolicy", DefaultDeliverPolicy)
	v.SetDefault("nats.consumers.replayPolicy", DefaultReplayPolicy)

	// Publish
	v.SetDefault("nats.publish.mode", DefaultPublishMode)
	v.SetDefault("nats.publish.ackTimeout", DefaultPublishAckTimeout)
	v.SetDefault("nats.publish.maxRetries", DefaultPublishMaxRetries)
	v.SetDefault("nats.publish.retryBaseDelay", DefaultRetryBaseDelay)

	// HTTP Server
	v.SetDefault("http.server.address", DefaultHTTPServerAddress)
	v.SetDefault("http.server.readTimeout", DefaultHTTPReadTimeout)
	v.SetDefault("http.server.writeTimeout", DefaultHTTPWriteTimeout)
	v.SetDefault("http.server.idleTimeout", DefaultHTTPIdleTimeout)
	v.SetDefault("http.server.maxHeaderBytes", DefaultMaxHeaderBytes)
	v.SetDefault("http.server.shutdownGracePeriod", DefaultHTTPShutdownGracePeriod)
	v.SetDefault("http.server.inboundWorkerCount", 10)
	v.SetDefault("http.server.inboundQueueSize", DefaultInboundQueueSize)

	// HTTP Client
	v.SetDefault("http.client.timeout", DefaultHTTPClientTimeout)
	v.SetDefault("http.client.maxIdleConns", DefaultMaxIdleConns)
	v.SetDefault("http.client.maxIdleConnsPerHost", DefaultMaxIdleConnsPerHost)
	v.SetDefault("http.client.idleConnTimeout", DefaultHTTPIdleConnTimeout)
	v.SetDefault("http.client.tls.insecureSkipVerify", false)

	// Logging
	v.SetDefault("logging.level", DefaultLogLevel)
	v.SetDefault("logging.encoding", DefaultLogEncoding)
	v.SetDefault("logging.outputPath", DefaultLogOutput)

	// Metrics
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.address", DefaultMetricsAddress)
	v.SetDefault("metrics.path", DefaultMetricsPath)
	v.SetDefault("metrics.updateInterval", "15s")

	// KV
	v.SetDefault("kv.enabled", false)
	v.SetDefault("kv.autoProvision", true)
	v.SetDefault("kv.localCache.enabled", false)

	// Rules
	v.SetDefault("rules.kvBucket", "rules")

	// Security
	v.SetDefault("security.verification.enabled", false)
	v.SetDefault("security.verification.publicKeyHeader", DefaultPublicKeyHeader)
	v.SetDefault("security.verification.signatureHeader", DefaultSignatureHeader)

	// ForEach
	v.SetDefault("forEach.maxIterations", DefaultForEachMaxIterations)

	// Gateway
	v.SetDefault("gateway.enabled", false)

	// AuthManager
	v.SetDefault("authManager.enabled", false)
	v.SetDefault("authManager.storage.bucket", "")
	v.SetDefault("authManager.storage.keyPrefix", "")
}

// applyConditionalDefaults handles defaults that depend on other config values
// and cannot be expressed as static viper defaults.
func applyConditionalDefaults(cfg *Config) {
	if cfg.KV.Enabled {
		cfg.KV.LocalCache.Enabled = true
	}
	if cfg.AuthManager.Enabled && cfg.AuthManager.Storage.Bucket == "" {
		cfg.AuthManager.Storage.Bucket = "tokens"
	}
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	// NATS validation
	if len(cfg.NATS.URLs) == 0 {
		return fmt.Errorf("at least one NATS URL must be specified")
	}

	// Authentication validation
	authCount := 0
	if cfg.NATS.Username != "" {
		authCount++
	}
	if cfg.NATS.Token != "" {
		authCount++
	}
	if cfg.NATS.NKey != "" {
		authCount++
	}
	if cfg.NATS.CredsFile != "" {
		authCount++
	}
	if authCount > 1 {
		return fmt.Errorf("only one NATS authentication method should be specified")
	}

	// TLS validation
	if cfg.NATS.TLS.Enable {
		if cfg.NATS.TLS.CertFile != "" && cfg.NATS.TLS.KeyFile == "" {
			return fmt.Errorf("NATS TLS key file is required when a cert file is provided")
		}
		if cfg.NATS.TLS.KeyFile != "" && cfg.NATS.TLS.CertFile == "" {
			return fmt.Errorf("NATS TLS cert file is required when a key file is provided")
		}
		if cfg.NATS.TLS.CertFile != "" {
			if _, err := os.Stat(cfg.NATS.TLS.CertFile); errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("NATS TLS cert file does not exist: %s", cfg.NATS.TLS.CertFile)
			}
		}
		if cfg.NATS.TLS.KeyFile != "" {
			if _, err := os.Stat(cfg.NATS.TLS.KeyFile); errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("NATS TLS key file does not exist: %s", cfg.NATS.TLS.KeyFile)
			}
		}
		if cfg.NATS.TLS.CAFile != "" {
			if _, err := os.Stat(cfg.NATS.TLS.CAFile); errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("NATS TLS CA file does not exist: %s", cfg.NATS.TLS.CAFile)
			}
		}
	}

	if cfg.NATS.CredsFile != "" {
		if _, err := os.Stat(cfg.NATS.CredsFile); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("NATS creds file does not exist: %s", cfg.NATS.CredsFile)
		}
	}

	if cfg.NATS.NKey != "" {
		if _, err := os.Stat(cfg.NATS.NKey); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("NATS NKey seed file does not exist: %s", cfg.NATS.NKey)
		}
	}

	// Consumer validation with bounds checking
	if cfg.NATS.Consumers.WorkerCount < 1 {
		return fmt.Errorf("worker count must be at least 1")
	}
	if cfg.NATS.Consumers.WorkerCount > MaxWorkerCount {
		return fmt.Errorf("worker count too high (%d), maximum is %d", cfg.NATS.Consumers.WorkerCount, MaxWorkerCount)
	}
	if cfg.NATS.Consumers.FetchBatchSize < 1 {
		return fmt.Errorf("fetch batch size must be at least 1")
	}
	if cfg.NATS.Consumers.FetchBatchSize > MaxFetchBatchSize {
		return fmt.Errorf("fetch batch size too high (%d), maximum is %d", cfg.NATS.Consumers.FetchBatchSize, MaxFetchBatchSize)
	}
	if cfg.NATS.Consumers.FetchTimeout <= 0 {
		return fmt.Errorf("fetch timeout must be positive")
	}
	if cfg.NATS.Consumers.MaxAckPending < 1 {
		return fmt.Errorf("max ack pending must be at least 1")
	}
	if cfg.NATS.Consumers.MaxAckPending > MaxAckPending {
		return fmt.Errorf("max ack pending too high (%d), maximum is %d", cfg.NATS.Consumers.MaxAckPending, MaxAckPending)
	}
	if cfg.NATS.Consumers.MaxDeliver < 1 {
		return fmt.Errorf("max deliver must be at least 1")
	}

	validDeliverPolicies := map[string]bool{
		"all": true, "new": true, "last": true, "by_start_time": true, "by_start_sequence": true,
	}
	if !validDeliverPolicies[cfg.NATS.Consumers.DeliverPolicy] {
		return fmt.Errorf("invalid deliver policy: %s", cfg.NATS.Consumers.DeliverPolicy)
	}

	validReplayPolicies := map[string]bool{"instant": true, "original": true}
	if !validReplayPolicies[cfg.NATS.Consumers.ReplayPolicy] {
		return fmt.Errorf("invalid replay policy: %s", cfg.NATS.Consumers.ReplayPolicy)
	}

	// Publish validation
	if cfg.NATS.Publish.Mode != "jetstream" && cfg.NATS.Publish.Mode != "core" {
		return fmt.Errorf("publish mode must be 'jetstream' or 'core', got: %s", cfg.NATS.Publish.Mode)
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", cfg.Logging.Level)
	}

	// Metrics validation
	if cfg.Metrics.Enabled {
		if cfg.Metrics.UpdateInterval != "" {
			if _, err := time.ParseDuration(cfg.Metrics.UpdateInterval); err != nil {
				return fmt.Errorf("invalid metrics update interval '%s': %w", cfg.Metrics.UpdateInterval, err)
			}
		}
	}

	// HTTP-specific validation with bounds checking
	if cfg.HTTP.Server.Address != "" {
		if cfg.HTTP.Server.ReadTimeout < 0 {
			return fmt.Errorf("HTTP server read timeout cannot be negative")
		}
		if cfg.HTTP.Server.WriteTimeout < 0 {
			return fmt.Errorf("HTTP server write timeout cannot be negative")
		}
		if cfg.HTTP.Server.InboundWorkerCount < 1 {
			return fmt.Errorf("inbound worker count must be at least 1")
		}
		if cfg.HTTP.Server.InboundWorkerCount > MaxWorkerCount {
			return fmt.Errorf("inbound worker count too high (%d), maximum is %d", cfg.HTTP.Server.InboundWorkerCount, MaxWorkerCount)
		}
		if cfg.HTTP.Server.InboundQueueSize < 1 {
			return fmt.Errorf("inbound queue size must be at least 1")
		}
		if cfg.HTTP.Server.InboundQueueSize > MaxInboundQueueSize {
			return fmt.Errorf("inbound queue size too high (%d), maximum is %d", cfg.HTTP.Server.InboundQueueSize, MaxInboundQueueSize)
		}
		if cfg.HTTP.Client.Timeout < 0 {
			return fmt.Errorf("HTTP client timeout cannot be negative")
		}
		if cfg.HTTP.Client.MaxIdleConns < 0 {
			return fmt.Errorf("HTTP client max idle connections cannot be negative")
		}
		if cfg.HTTP.Client.MaxIdleConnsPerHost < 0 {
			return fmt.Errorf("HTTP client max idle connections per host cannot be negative")
		}
	}

	// ForEach validation
	if cfg.ForEach.MaxIterations < 0 {
		return fmt.Errorf("forEach maxIterations cannot be negative (use 0 for unlimited)")
	}
	if cfg.ForEach.MaxIterations > MaxForEachIterations {
		return fmt.Errorf("forEach maxIterations too high (%d), maximum is %d", cfg.ForEach.MaxIterations, MaxForEachIterations)
	}

	// AuthManager validation
	if cfg.AuthManager.Enabled {
		if len(cfg.AuthManager.Providers) == 0 {
			return fmt.Errorf("authManager requires at least one provider when enabled")
		}
		if cfg.AuthManager.Storage.Bucket == "" {
			return fmt.Errorf("authManager storage bucket cannot be empty")
		}
		seenIDs := make(map[string]bool)
		for i, p := range cfg.AuthManager.Providers {
			if p.ID == "" {
				return fmt.Errorf("authManager provider %d: id is required", i)
			}
			if seenIDs[p.ID] {
				return fmt.Errorf("authManager provider %d: duplicate id '%s'", i, p.ID)
			}
			seenIDs[p.ID] = true
			if p.Type != "oauth2" && p.Type != "custom-http" {
				return fmt.Errorf("authManager provider %s: type must be 'oauth2' or 'custom-http'", p.ID)
			}
			if p.Type == "oauth2" {
				if p.TokenURL == "" {
					return fmt.Errorf("authManager provider %s: tokenUrl required for oauth2", p.ID)
				}
				if p.ClientID == "" {
					return fmt.Errorf("authManager provider %s: clientId required for oauth2", p.ID)
				}
				if p.ClientSecret == "" {
					return fmt.Errorf("authManager provider %s: clientSecret required for oauth2", p.ID)
				}
				if p.RefreshBefore == "" {
					return fmt.Errorf("authManager provider %s: refreshBefore required for oauth2", p.ID)
				}
				if _, err := time.ParseDuration(p.RefreshBefore); err != nil {
					return fmt.Errorf("authManager provider %s: invalid refreshBefore: %w", p.ID, err)
				}
			} else if p.Type == "custom-http" {
				if p.AuthURL == "" {
					return fmt.Errorf("authManager provider %s: authUrl required for custom-http", p.ID)
				}
				if p.TokenPath == "" {
					return fmt.Errorf("authManager provider %s: tokenPath required for custom-http", p.ID)
				}
				if p.RefreshEvery == "" {
					return fmt.Errorf("authManager provider %s: refreshEvery required for custom-http", p.ID)
				}
				if _, err := time.ParseDuration(p.RefreshEvery); err != nil {
					return fmt.Errorf("authManager provider %s: invalid refreshEvery: %w", p.ID, err)
				}
			}
		}
	}

	return nil
}
