package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultNATSURL                 = "nats://localhost:4222"
	DefaultReconnectWait           = 50 * time.Millisecond
	DefaultFetchTimeout            = 5 * time.Second
	DefaultAckWaitTimeout          = 30 * time.Second
	DefaultPublishAckTimeout       = 5 * time.Second
	DefaultRetryBaseDelay          = 50 * time.Millisecond
	DefaultHTTPReadTimeout         = 30 * time.Second
	DefaultHTTPWriteTimeout        = 30 * time.Second
	DefaultHTTPIdleTimeout         = 120 * time.Second
	DefaultHTTPShutdownGracePeriod = 30 * time.Second
	DefaultHTTPClientTimeout       = 30 * time.Second
	DefaultHTTPIdleConnTimeout     = 90 * time.Second
	DefaultMaxHeaderBytes          = 1 << 20
)

const (
	DefaultWorkerCount          = 2
	DefaultFetchBatchSize       = 1
	DefaultMaxDeliver           = 3
	DefaultMaxAckPending        = 1000
	DefaultPublishMaxRetries    = 3
	DefaultMaxIdleConns         = 100
	DefaultMaxIdleConnsPerHost  = 10
	DefaultForEachMaxIterations = 100
	DefaultInboundQueueSize     = 1000
)

const (
	MaxWorkerCount       = 1000
	MaxFetchBatchSize    = 10000
	MaxAckPending        = 100000
	MaxForEachIterations = 10000
	MaxInboundQueueSize  = 100000
)

const (
	DefaultConsumerPrefix    = "shunt"
	DefaultPublishMode       = "jetstream"
	DefaultDeliverPolicy     = "new"
	DefaultReplayPolicy      = "instant"
	DefaultLogLevel          = "info"
	DefaultLogEncoding       = "json"
	DefaultLogOutput         = "stdout"
	DefaultMetricsAddress    = ":2112"
	DefaultMetricsPath       = "/metrics"
	DefaultHTTPServerAddress = ":8080"
	DefaultPublicKeyHeader   = "Nats-Public-Key"
	DefaultSignatureHeader   = "Nats-Signature"
)

type Config struct {
	NATS        NATSConfig        `json:"nats" yaml:"nats"`
	HTTP        HTTPConfig        `json:"http,omitempty" yaml:"http,omitempty"`
	Logging     LogConfig         `json:"logging" yaml:"logging"`
	Metrics     MetricsConfig     `json:"metrics" yaml:"metrics"`
	KV          KVConfig          `json:"kv" yaml:"kv"`
	Rules       RulesConfig       `json:"rules" yaml:"rules"`
	Security    SecurityConfig    `json:"security" yaml:"security"`
	ForEach     ForEachConfig     `json:"forEach" yaml:"forEach"`
	Gateway     GatewayConfig     `json:"gateway" yaml:"gateway"`
	AuthManager AuthManagerConfig `json:"authManager" yaml:"authManager"`
}

type GatewayConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

type AuthManagerConfig struct {
	Enabled   bool                  `json:"enabled" yaml:"enabled"`
	Storage   AuthManagerStorage    `json:"storage" yaml:"storage"`
	Providers []AuthManagerProvider `json:"providers" yaml:"providers"`
}

type AuthManagerStorage struct {
	Bucket    string `json:"bucket" yaml:"bucket"`
	KeyPrefix string `json:"keyPrefix" yaml:"keyPrefix"`
}

type AuthManagerProvider struct {
	ID            string            `json:"id" yaml:"id"`
	Type          string            `json:"type" yaml:"type"`
	KVKey         string            `json:"kvKey" yaml:"kvKey"`
	RefreshBefore string            `json:"refreshBefore" yaml:"refreshBefore"`
	RefreshEvery  string            `json:"refreshEvery" yaml:"refreshEvery"`
	TokenURL      string            `json:"tokenUrl" yaml:"tokenUrl"`
	ClientID      string            `json:"clientId" yaml:"clientId"`
	ClientSecret  string            `json:"clientSecret" yaml:"clientSecret"`
	Scopes        []string          `json:"scopes" yaml:"scopes"`
	AuthURL       string            `json:"authUrl" yaml:"authUrl"`
	Method        string            `json:"method" yaml:"method"`
	Headers       map[string]string `json:"headers" yaml:"headers"`
	Body          string            `json:"body" yaml:"body"`
	TokenPath     string            `json:"tokenPath" yaml:"tokenPath"`
}

type RulesConfig struct {
	KVBucket string `json:"kvBucket" yaml:"kvBucket"`
}

type ForEachConfig struct {
	MaxIterations int `json:"maxIterations" yaml:"maxIterations"`
}

type HTTPConfig struct {
	Server HTTPServerConfig `json:"server" yaml:"server"`
	Client HTTPClientConfig `json:"client" yaml:"client"`
}

type HTTPServerConfig struct {
	Address             string        `json:"address" yaml:"address"`
	ReadTimeout         time.Duration `json:"readTimeout" yaml:"readTimeout"`
	WriteTimeout        time.Duration `json:"writeTimeout" yaml:"writeTimeout"`
	IdleTimeout         time.Duration `json:"idleTimeout" yaml:"idleTimeout"`
	MaxHeaderBytes      int           `json:"maxHeaderBytes" yaml:"maxHeaderBytes"`
	ShutdownGracePeriod time.Duration `json:"shutdownGracePeriod" yaml:"shutdownGracePeriod"`
	InboundWorkerCount  int           `json:"inboundWorkerCount" yaml:"inboundWorkerCount"`
	InboundQueueSize    int           `json:"inboundQueueSize" yaml:"inboundQueueSize"`
}

type HTTPClientConfig struct {
	Timeout             time.Duration `json:"timeout" yaml:"timeout"`
	MaxIdleConns        int           `json:"maxIdleConns" yaml:"maxIdleConns"`
	MaxIdleConnsPerHost int           `json:"maxIdleConnsPerHost" yaml:"maxIdleConnsPerHost"`
	IdleConnTimeout     time.Duration `json:"idleConnTimeout" yaml:"idleConnTimeout"`
	TLS                 HTTPClientTLS `json:"tls,omitempty" yaml:"tls,omitempty"`
}

type HTTPClientTLS struct {
	InsecureSkipVerify bool `json:"insecureSkipVerify" yaml:"insecureSkipVerify"`
}

type NATSConfig struct {
	URLs      []string `json:"urls" yaml:"urls"`
	Username  string   `json:"username" yaml:"username"`
	Password  string   `json:"password" yaml:"password"`
	Token     string   `json:"token" yaml:"token"`
	NKey      string   `json:"nkey" yaml:"nkey"`
	CredsFile string   `json:"credsFile" yaml:"credsFile"`

	TLS struct {
		Enable   bool   `json:"enable" yaml:"enable"`
		CertFile string `json:"certFile" yaml:"certFile"`
		KeyFile  string `json:"keyFile" yaml:"keyFile"`
		CAFile   string `json:"caFile" yaml:"caFile"`
		Insecure bool   `json:"insecure" yaml:"insecure"`
	} `json:"tls" yaml:"tls"`

	Consumers  ConsumerConfig   `json:"consumers" yaml:"consumers"`
	Connection ConnectionConfig `json:"connection" yaml:"connection"`
	Publish    PublishConfig    `json:"publish" yaml:"publish"`
}

type ConsumerConfig struct {
	ConsumerPrefix string        `json:"consumerPrefix" yaml:"consumerPrefix"`
	WorkerCount    int           `json:"workerCount" yaml:"workerCount"`
	FetchBatchSize int           `json:"fetchBatchSize" yaml:"fetchBatchSize"`
	FetchTimeout   time.Duration `json:"fetchTimeout" yaml:"fetchTimeout"`
	MaxAckPending  int           `json:"maxAckPending" yaml:"maxAckPending"`
	AckWaitTimeout time.Duration `json:"ackWaitTimeout" yaml:"ackWaitTimeout"`
	MaxDeliver     int           `json:"maxDeliver" yaml:"maxDeliver"`
	DeliverPolicy  string        `json:"deliverPolicy" yaml:"deliverPolicy"`
	ReplayPolicy   string        `json:"replayPolicy" yaml:"replayPolicy"`
}

type ConnectionConfig struct {
	MaxReconnects int           `json:"maxReconnects" yaml:"maxReconnects"`
	ReconnectWait time.Duration `json:"reconnectWait" yaml:"reconnectWait"`
}

type PublishConfig struct {
	Mode           string        `json:"mode" yaml:"mode"`
	AckTimeout     time.Duration `json:"ackTimeout" yaml:"ackTimeout"`
	MaxRetries     int           `json:"maxRetries" yaml:"maxRetries"`
	RetryBaseDelay time.Duration `json:"retryBaseDelay" yaml:"retryBaseDelay"`
}

type LogConfig struct {
	Level      string `json:"level" yaml:"level"`
	Encoding   string `json:"encoding" yaml:"encoding"`
	OutputPath string `json:"outputPath" yaml:"outputPath"`
}

type MetricsConfig struct {
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	Address        string `json:"address" yaml:"address"`
	Path           string `json:"path" yaml:"path"`
	UpdateInterval string `json:"updateInterval" yaml:"updateInterval"`
}

type KVConfig struct {
	Enabled       bool     `json:"enabled" yaml:"enabled"`
	AutoProvision bool     `json:"autoProvision" yaml:"autoProvision"`
	Buckets       []string `json:"buckets" yaml:"buckets"`
	LocalCache    struct {
		Enabled bool `json:"enabled" yaml:"enabled"`
	} `json:"localCache" yaml:"localCache"`
}

type SecurityConfig struct {
	Verification VerificationConfig `json:"verification" yaml:"verification"`
}

type VerificationConfig struct {
	Enabled         bool   `json:"enabled" yaml:"enabled"`
	PublicKeyHeader string `json:"publicKeyHeader" yaml:"publicKeyHeader"`
	SignatureHeader string `json:"signatureHeader" yaml:"signatureHeader"`
}

type ServeOverrides struct {
	NATSURLs       []string
	LogLevel       string
	MetricsEnabled *bool
	MetricsAddr    string
	MetricsPath    string
	GatewayEnabled *bool
	KVEnabled      *bool
	WorkerCount    *int
}

func newDefaults() *Config {
	return &Config{
		NATS: NATSConfig{
			URLs: []string{DefaultNATSURL},
			Consumers: ConsumerConfig{
				ConsumerPrefix: DefaultConsumerPrefix,
				WorkerCount:    DefaultWorkerCount,
				FetchBatchSize: DefaultFetchBatchSize,
				FetchTimeout:   DefaultFetchTimeout,
				MaxAckPending:  DefaultMaxAckPending,
				AckWaitTimeout: DefaultAckWaitTimeout,
				MaxDeliver:     DefaultMaxDeliver,
				DeliverPolicy:  DefaultDeliverPolicy,
				ReplayPolicy:   DefaultReplayPolicy,
			},
			Connection: ConnectionConfig{
				MaxReconnects: -1,
				ReconnectWait: DefaultReconnectWait,
			},
			Publish: PublishConfig{
				Mode:           DefaultPublishMode,
				AckTimeout:     DefaultPublishAckTimeout,
				MaxRetries:     DefaultPublishMaxRetries,
				RetryBaseDelay: DefaultRetryBaseDelay,
			},
		},
		HTTP: HTTPConfig{
			Server: HTTPServerConfig{
				Address:             DefaultHTTPServerAddress,
				ReadTimeout:         DefaultHTTPReadTimeout,
				WriteTimeout:        DefaultHTTPWriteTimeout,
				IdleTimeout:         DefaultHTTPIdleTimeout,
				MaxHeaderBytes:      DefaultMaxHeaderBytes,
				ShutdownGracePeriod: DefaultHTTPShutdownGracePeriod,
				InboundWorkerCount:  10,
				InboundQueueSize:    DefaultInboundQueueSize,
			},
			Client: HTTPClientConfig{
				Timeout:             DefaultHTTPClientTimeout,
				MaxIdleConns:        DefaultMaxIdleConns,
				MaxIdleConnsPerHost: DefaultMaxIdleConnsPerHost,
				IdleConnTimeout:     DefaultHTTPIdleConnTimeout,
			},
		},
		Logging: LogConfig{
			Level:      DefaultLogLevel,
			Encoding:   DefaultLogEncoding,
			OutputPath: DefaultLogOutput,
		},
		Metrics: MetricsConfig{
			Enabled:        true,
			Address:        DefaultMetricsAddress,
			Path:           DefaultMetricsPath,
			UpdateInterval: "15s",
		},
		KV: KVConfig{
			AutoProvision: true,
		},
		Rules: RulesConfig{
			KVBucket: "rules",
		},
		Security: SecurityConfig{
			Verification: VerificationConfig{
				PublicKeyHeader: DefaultPublicKeyHeader,
				SignatureHeader: DefaultSignatureHeader,
			},
		},
		ForEach: ForEachConfig{
			MaxIterations: DefaultForEachMaxIterations,
		},
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg := newDefaults()
			applyConditionalDefaults(cfg)
			if err := validateConfig(cfg); err != nil {
				return nil, fmt.Errorf("invalid configuration: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := newDefaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	applyConditionalDefaults(cfg)
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return cfg, nil
}

func (c *Config) ApplyOverrides(o ServeOverrides) {
	if len(o.NATSURLs) > 0 {
		c.NATS.URLs = o.NATSURLs
	}
	if o.LogLevel != "" {
		c.Logging.Level = o.LogLevel
	}
	if o.MetricsEnabled != nil {
		c.Metrics.Enabled = *o.MetricsEnabled
	}
	if o.MetricsAddr != "" {
		c.Metrics.Address = o.MetricsAddr
	}
	if o.MetricsPath != "" {
		c.Metrics.Path = o.MetricsPath
	}
	if o.GatewayEnabled != nil {
		c.Gateway.Enabled = *o.GatewayEnabled
	}
	if o.KVEnabled != nil {
		c.KV.Enabled = *o.KVEnabled
	}
	if o.WorkerCount != nil {
		c.NATS.Consumers.WorkerCount = *o.WorkerCount
	}
}

func applyConditionalDefaults(cfg *Config) {
	if cfg.KV.Enabled {
		cfg.KV.LocalCache.Enabled = true
	}
	if cfg.AuthManager.Enabled && cfg.AuthManager.Storage.Bucket == "" {
		cfg.AuthManager.Storage.Bucket = "tokens"
	}
}

func validateConfig(cfg *Config) error {
	if len(cfg.NATS.URLs) == 0 {
		return fmt.Errorf("at least one NATS URL must be specified")
	}

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

	if cfg.NATS.Publish.Mode != "jetstream" && cfg.NATS.Publish.Mode != "core" {
		return fmt.Errorf("publish mode must be 'jetstream' or 'core', got: %s", cfg.NATS.Publish.Mode)
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", cfg.Logging.Level)
	}

	if cfg.Metrics.Enabled {
		if cfg.Metrics.UpdateInterval != "" {
			if _, err := time.ParseDuration(cfg.Metrics.UpdateInterval); err != nil {
				return fmt.Errorf("invalid metrics update interval '%s': %w", cfg.Metrics.UpdateInterval, err)
			}
		}
	}

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

	if cfg.ForEach.MaxIterations < 0 {
		return fmt.Errorf("forEach maxIterations cannot be negative (use 0 for unlimited)")
	}
	if cfg.ForEach.MaxIterations > MaxForEachIterations {
		return fmt.Errorf("forEach maxIterations too high (%d), maximum is %d", cfg.ForEach.MaxIterations, MaxForEachIterations)
	}

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
