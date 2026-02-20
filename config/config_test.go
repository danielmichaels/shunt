package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	cfg := newDefaults()

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"nats.connection.maxReconnects", cfg.NATS.Connection.MaxReconnects, -1},
		{"nats.connection.reconnectWait", cfg.NATS.Connection.ReconnectWait, DefaultReconnectWait},
		{"nats.consumers.consumerPrefix", cfg.NATS.Consumers.ConsumerPrefix, DefaultConsumerPrefix},
		{"nats.consumers.workerCount", cfg.NATS.Consumers.WorkerCount, DefaultWorkerCount},
		{"nats.consumers.fetchBatchSize", cfg.NATS.Consumers.FetchBatchSize, DefaultFetchBatchSize},
		{"nats.consumers.fetchTimeout", cfg.NATS.Consumers.FetchTimeout, DefaultFetchTimeout},
		{"nats.consumers.maxAckPending", cfg.NATS.Consumers.MaxAckPending, DefaultMaxAckPending},
		{"nats.consumers.ackWaitTimeout", cfg.NATS.Consumers.AckWaitTimeout, DefaultAckWaitTimeout},
		{"nats.consumers.maxDeliver", cfg.NATS.Consumers.MaxDeliver, DefaultMaxDeliver},
		{"nats.consumers.deliverPolicy", cfg.NATS.Consumers.DeliverPolicy, DefaultDeliverPolicy},
		{"nats.consumers.replayPolicy", cfg.NATS.Consumers.ReplayPolicy, DefaultReplayPolicy},
		{"nats.publish.mode", cfg.NATS.Publish.Mode, DefaultPublishMode},
		{"nats.publish.ackTimeout", cfg.NATS.Publish.AckTimeout, DefaultPublishAckTimeout},
		{"nats.publish.maxRetries", cfg.NATS.Publish.MaxRetries, DefaultPublishMaxRetries},
		{"nats.publish.retryBaseDelay", cfg.NATS.Publish.RetryBaseDelay, DefaultRetryBaseDelay},
		{"nats.tls.enable", cfg.NATS.TLS.Enable, false},
		{"http.server.address", cfg.HTTP.Server.Address, DefaultHTTPServerAddress},
		{"http.server.readTimeout", cfg.HTTP.Server.ReadTimeout, DefaultHTTPReadTimeout},
		{"http.server.writeTimeout", cfg.HTTP.Server.WriteTimeout, DefaultHTTPWriteTimeout},
		{"http.server.idleTimeout", cfg.HTTP.Server.IdleTimeout, DefaultHTTPIdleTimeout},
		{"http.server.maxHeaderBytes", cfg.HTTP.Server.MaxHeaderBytes, DefaultMaxHeaderBytes},
		{"http.server.shutdownGracePeriod", cfg.HTTP.Server.ShutdownGracePeriod, DefaultHTTPShutdownGracePeriod},
		{"http.server.inboundWorkerCount", cfg.HTTP.Server.InboundWorkerCount, 10},
		{"http.server.inboundQueueSize", cfg.HTTP.Server.InboundQueueSize, DefaultInboundQueueSize},
		{"http.client.timeout", cfg.HTTP.Client.Timeout, DefaultHTTPClientTimeout},
		{"http.client.maxIdleConns", cfg.HTTP.Client.MaxIdleConns, DefaultMaxIdleConns},
		{"http.client.maxIdleConnsPerHost", cfg.HTTP.Client.MaxIdleConnsPerHost, DefaultMaxIdleConnsPerHost},
		{"http.client.idleConnTimeout", cfg.HTTP.Client.IdleConnTimeout, DefaultHTTPIdleConnTimeout},
		{"logging.level", cfg.Logging.Level, DefaultLogLevel},
		{"logging.encoding", cfg.Logging.Encoding, DefaultLogEncoding},
		{"logging.outputPath", cfg.Logging.OutputPath, DefaultLogOutput},
		{"metrics.enabled", cfg.Metrics.Enabled, true},
		{"metrics.address", cfg.Metrics.Address, DefaultMetricsAddress},
		{"metrics.path", cfg.Metrics.Path, DefaultMetricsPath},
		{"metrics.updateInterval", cfg.Metrics.UpdateInterval, "15s"},
		{"rules.kvBucket", cfg.Rules.KVBucket, "rules"},
		{"security.verification.publicKeyHeader", cfg.Security.Verification.PublicKeyHeader, DefaultPublicKeyHeader},
		{"security.verification.signatureHeader", cfg.Security.Verification.SignatureHeader, DefaultSignatureHeader},
		{"forEach.maxIterations", cfg.ForEach.MaxIterations, DefaultForEachMaxIterations},
		{"gateway.enabled", cfg.Gateway.Enabled, false},
		{"authManager.enabled", cfg.AuthManager.Enabled, false},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("key %q = %v (%T), want %v (%T)", c.name, c.got, c.got, c.want, c.want)
		}
	}

	if len(cfg.NATS.URLs) != 1 || cfg.NATS.URLs[0] != DefaultNATSURL {
		t.Errorf("nats.urls = %v, want [%s]", cfg.NATS.URLs, DefaultNATSURL)
	}
}

func TestApplyConditionalDefaults(t *testing.T) {
	t.Run("KV local cache enabled when KV enabled", func(t *testing.T) {
		cfg := &Config{KV: KVConfig{Enabled: true}}
		applyConditionalDefaults(cfg)
		if !cfg.KV.LocalCache.Enabled {
			t.Error("KV.LocalCache.Enabled should be true when KV is enabled")
		}
	})

	t.Run("KV local cache unchanged when KV disabled", func(t *testing.T) {
		cfg := &Config{KV: KVConfig{Enabled: false}}
		applyConditionalDefaults(cfg)
		if cfg.KV.LocalCache.Enabled {
			t.Error("KV.LocalCache.Enabled should remain false when KV is disabled")
		}
	})

	t.Run("AuthManager storage bucket defaulted when enabled", func(t *testing.T) {
		cfg := &Config{AuthManager: AuthManagerConfig{Enabled: true}}
		applyConditionalDefaults(cfg)
		if cfg.AuthManager.Storage.Bucket != "tokens" {
			t.Errorf("AuthManager.Storage.Bucket = %s, want tokens", cfg.AuthManager.Storage.Bucket)
		}
	})

	t.Run("AuthManager storage bucket preserved when set", func(t *testing.T) {
		cfg := &Config{AuthManager: AuthManagerConfig{
			Enabled: true,
			Storage: AuthManagerStorage{Bucket: "custom"},
		}}
		applyConditionalDefaults(cfg)
		if cfg.AuthManager.Storage.Bucket != "custom" {
			t.Errorf("AuthManager.Storage.Bucket = %s, want custom", cfg.AuthManager.Storage.Bucket)
		}
	})
}

func validConfig() *Config {
	cfg := newDefaults()
	applyConditionalDefaults(cfg)
	return cfg
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr string
	}{
		{
			name:    "valid default config",
			modify:  func(cfg *Config) {},
			wantErr: "",
		},
		{
			name: "empty NATS URLs",
			modify: func(cfg *Config) {
				cfg.NATS.URLs = []string{}
			},
			wantErr: "at least one NATS URL must be specified",
		},
		{
			name: "multiple auth methods - username and token",
			modify: func(cfg *Config) {
				cfg.NATS.Username = "user"
				cfg.NATS.Token = "token"
			},
			wantErr: "only one NATS authentication method",
		},
		{
			name: "multiple auth methods - token and nkey",
			modify: func(cfg *Config) {
				cfg.NATS.Token = "token"
				cfg.NATS.NKey = "nkey"
			},
			wantErr: "only one NATS authentication method",
		},
		{
			name: "TLS cert without key",
			modify: func(cfg *Config) {
				cfg.NATS.TLS.Enable = true
				cfg.NATS.TLS.CertFile = "/path/to/cert.pem"
			},
			wantErr: "NATS TLS key file is required",
		},
		{
			name: "TLS key without cert",
			modify: func(cfg *Config) {
				cfg.NATS.TLS.Enable = true
				cfg.NATS.TLS.KeyFile = "/path/to/key.pem"
			},
			wantErr: "NATS TLS cert file is required",
		},
		{
			name: "worker count too low",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.WorkerCount = 0
			},
			wantErr: "worker count must be at least 1",
		},
		{
			name: "worker count too high",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.WorkerCount = 1001
			},
			wantErr: "worker count too high",
		},
		{
			name: "fetch batch size too low",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.FetchBatchSize = 0
			},
			wantErr: "fetch batch size must be at least 1",
		},
		{
			name: "fetch batch size too high",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.FetchBatchSize = 10001
			},
			wantErr: "fetch batch size too high",
		},
		{
			name: "negative fetch timeout",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.FetchTimeout = -1 * time.Second
			},
			wantErr: "fetch timeout must be positive",
		},
		{
			name: "max ack pending too low",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.MaxAckPending = 0
			},
			wantErr: "max ack pending must be at least 1",
		},
		{
			name: "max ack pending too high",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.MaxAckPending = 100001
			},
			wantErr: "max ack pending too high",
		},
		{
			name: "max deliver too low",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.MaxDeliver = 0
			},
			wantErr: "max deliver must be at least 1",
		},
		{
			name: "invalid deliver policy",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.DeliverPolicy = "invalid"
			},
			wantErr: "invalid deliver policy",
		},
		{
			name: "invalid replay policy",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.ReplayPolicy = "invalid"
			},
			wantErr: "invalid replay policy",
		},
		{
			name: "invalid publish mode",
			modify: func(cfg *Config) {
				cfg.NATS.Publish.Mode = "invalid"
			},
			wantErr: "publish mode must be",
		},
		{
			name: "invalid log level",
			modify: func(cfg *Config) {
				cfg.Logging.Level = "invalid"
			},
			wantErr: "invalid log level",
		},
		{
			name: "invalid metrics update interval",
			modify: func(cfg *Config) {
				cfg.Metrics.Enabled = true
				cfg.Metrics.Address = ":2112"
				cfg.Metrics.UpdateInterval = "invalid"
			},
			wantErr: "invalid metrics update interval",
		},
		{
			name: "negative HTTP read timeout",
			modify: func(cfg *Config) {
				cfg.HTTP.Server.ReadTimeout = -1 * time.Second
			},
			wantErr: "HTTP server read timeout cannot be negative",
		},
		{
			name: "negative HTTP write timeout",
			modify: func(cfg *Config) {
				cfg.HTTP.Server.WriteTimeout = -1 * time.Second
			},
			wantErr: "HTTP server write timeout cannot be negative",
		},
		{
			name: "inbound worker count too low",
			modify: func(cfg *Config) {
				cfg.HTTP.Server.InboundWorkerCount = 0
			},
			wantErr: "inbound worker count must be at least 1",
		},
		{
			name: "inbound worker count too high",
			modify: func(cfg *Config) {
				cfg.HTTP.Server.InboundWorkerCount = 1001
			},
			wantErr: "inbound worker count too high",
		},
		{
			name: "inbound queue size too low",
			modify: func(cfg *Config) {
				cfg.HTTP.Server.InboundQueueSize = 0
			},
			wantErr: "inbound queue size must be at least 1",
		},
		{
			name: "inbound queue size too high",
			modify: func(cfg *Config) {
				cfg.HTTP.Server.InboundQueueSize = 100001
			},
			wantErr: "inbound queue size too high",
		},
		{
			name: "negative HTTP client timeout",
			modify: func(cfg *Config) {
				cfg.HTTP.Client.Timeout = -1 * time.Second
			},
			wantErr: "HTTP client timeout cannot be negative",
		},
		{
			name: "negative max idle conns",
			modify: func(cfg *Config) {
				cfg.HTTP.Client.MaxIdleConns = -1
			},
			wantErr: "HTTP client max idle connections cannot be negative",
		},
		{
			name: "negative max idle conns per host",
			modify: func(cfg *Config) {
				cfg.HTTP.Client.MaxIdleConnsPerHost = -1
			},
			wantErr: "HTTP client max idle connections per host cannot be negative",
		},
		{
			name: "negative forEach maxIterations",
			modify: func(cfg *Config) {
				cfg.ForEach.MaxIterations = -1
			},
			wantErr: "forEach maxIterations cannot be negative",
		},
		{
			name: "forEach maxIterations too high",
			modify: func(cfg *Config) {
				cfg.ForEach.MaxIterations = 10001
			},
			wantErr: "forEach maxIterations too high",
		},
		{
			name: "valid deliver policies",
			modify: func(cfg *Config) {
				for _, policy := range []string{"all", "new", "last", "by_start_time", "by_start_sequence"} {
					cfg.NATS.Consumers.DeliverPolicy = policy
				}
			},
			wantErr: "",
		},
		{
			name: "valid replay policies",
			modify: func(cfg *Config) {
				cfg.NATS.Consumers.ReplayPolicy = "original"
			},
			wantErr: "",
		},
		{
			name: "publish mode core is valid",
			modify: func(cfg *Config) {
				cfg.NATS.Publish.Mode = "core"
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(cfg)

			err := validateConfig(cfg)

			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateConfig() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("validateConfig() expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("validateConfig() error = %q, want error containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestValidateConfigTLSFileExistence(t *testing.T) {
	t.Run("TLS cert file does not exist", func(t *testing.T) {
		cfg := validConfig()
		cfg.NATS.TLS.Enable = true
		cfg.NATS.TLS.CertFile = "/nonexistent/cert.pem"
		cfg.NATS.TLS.KeyFile = "/nonexistent/key.pem"

		err := validateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "NATS TLS cert file does not exist") {
			t.Errorf("expected cert file not exist error, got: %v", err)
		}
	})

	t.Run("TLS key file does not exist", func(t *testing.T) {
		certFile := filepath.Join(t.TempDir(), "cert.pem")
		os.WriteFile(certFile, []byte("cert"), 0o600)

		cfg := validConfig()
		cfg.NATS.TLS.Enable = true
		cfg.NATS.TLS.CertFile = certFile
		cfg.NATS.TLS.KeyFile = "/nonexistent/key.pem"

		err := validateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "NATS TLS key file does not exist") {
			t.Errorf("expected key file not exist error, got: %v", err)
		}
	})

	t.Run("TLS CA file does not exist", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "cert.pem")
		keyFile := filepath.Join(dir, "key.pem")
		os.WriteFile(certFile, []byte("cert"), 0o600)
		os.WriteFile(keyFile, []byte("key"), 0o600)

		cfg := validConfig()
		cfg.NATS.TLS.Enable = true
		cfg.NATS.TLS.CertFile = certFile
		cfg.NATS.TLS.KeyFile = keyFile
		cfg.NATS.TLS.CAFile = "/nonexistent/ca.pem"

		err := validateConfig(cfg)
		if err == nil || !strings.Contains(err.Error(), "NATS TLS CA file does not exist") {
			t.Errorf("expected CA file not exist error, got: %v", err)
		}
	})

	t.Run("TLS all files exist passes validation", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "cert.pem")
		keyFile := filepath.Join(dir, "key.pem")
		caFile := filepath.Join(dir, "ca.pem")
		os.WriteFile(certFile, []byte("cert"), 0o600)
		os.WriteFile(keyFile, []byte("key"), 0o600)
		os.WriteFile(caFile, []byte("ca"), 0o600)

		cfg := validConfig()
		cfg.NATS.TLS.Enable = true
		cfg.NATS.TLS.CertFile = certFile
		cfg.NATS.TLS.KeyFile = keyFile
		cfg.NATS.TLS.CAFile = caFile

		err := validateConfig(cfg)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestLoadDefaultsWithoutConfigFile(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(yamlPath, []byte("{}"), 0o600)

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.NATS.URLs) != 1 || cfg.NATS.URLs[0] != DefaultNATSURL {
		t.Errorf("NATS URLs = %v, want [%s]", cfg.NATS.URLs, DefaultNATSURL)
	}
	if cfg.Logging.Level != DefaultLogLevel {
		t.Errorf("Logging.Level = %s, want %s", cfg.Logging.Level, DefaultLogLevel)
	}
	if cfg.NATS.Consumers.WorkerCount != DefaultWorkerCount {
		t.Errorf("WorkerCount = %d, want %d", cfg.NATS.Consumers.WorkerCount, DefaultWorkerCount)
	}
	if !cfg.Metrics.Enabled {
		t.Error("Metrics.Enabled should default to true")
	}
}

func TestLoadMissingConfigFileUsesDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(cfg.NATS.URLs) != 1 || cfg.NATS.URLs[0] != DefaultNATSURL {
		t.Errorf("NATS URLs = %v, want [%s]", cfg.NATS.URLs, DefaultNATSURL)
	}
	if cfg.Logging.Level != DefaultLogLevel {
		t.Errorf("Logging.Level = %s, want %s", cfg.Logging.Level, DefaultLogLevel)
	}
}

func TestLoadYAMLOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(yamlPath, []byte(`
logging:
  level: debug
  encoding: console
nats:
  consumers:
    workerCount: 8
    fetchBatchSize: 64
`), 0o600)

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %s, want debug", cfg.Logging.Level)
	}
	if cfg.Logging.Encoding != "console" {
		t.Errorf("Logging.Encoding = %s, want console", cfg.Logging.Encoding)
	}
	if cfg.NATS.Consumers.WorkerCount != 8 {
		t.Errorf("WorkerCount = %d, want 8", cfg.NATS.Consumers.WorkerCount)
	}
	if cfg.NATS.Consumers.FetchBatchSize != 64 {
		t.Errorf("FetchBatchSize = %d, want 64", cfg.NATS.Consumers.FetchBatchSize)
	}

	// Unset fields within a partially-specified section must keep defaults
	if cfg.Logging.OutputPath != DefaultLogOutput {
		t.Errorf("Logging.OutputPath = %s, want %s (default preserved)", cfg.Logging.OutputPath, DefaultLogOutput)
	}
	if cfg.NATS.Consumers.ConsumerPrefix != DefaultConsumerPrefix {
		t.Errorf("ConsumerPrefix = %s, want %s (default preserved)", cfg.NATS.Consumers.ConsumerPrefix, DefaultConsumerPrefix)
	}
	if cfg.NATS.Consumers.FetchTimeout != DefaultFetchTimeout {
		t.Errorf("FetchTimeout = %v, want %v (default preserved)", cfg.NATS.Consumers.FetchTimeout, DefaultFetchTimeout)
	}
	if cfg.NATS.Consumers.MaxDeliver != DefaultMaxDeliver {
		t.Errorf("MaxDeliver = %d, want %d (default preserved)", cfg.NATS.Consumers.MaxDeliver, DefaultMaxDeliver)
	}
	if cfg.NATS.Consumers.DeliverPolicy != DefaultDeliverPolicy {
		t.Errorf("DeliverPolicy = %s, want %s (default preserved)", cfg.NATS.Consumers.DeliverPolicy, DefaultDeliverPolicy)
	}

	// Entirely unmentioned sections must keep defaults
	if cfg.Metrics.Address != DefaultMetricsAddress {
		t.Errorf("Metrics.Address = %s, want %s", cfg.Metrics.Address, DefaultMetricsAddress)
	}
	if cfg.NATS.Publish.Mode != DefaultPublishMode {
		t.Errorf("Publish.Mode = %s, want %s (default preserved)", cfg.NATS.Publish.Mode, DefaultPublishMode)
	}
}

func TestLoadBoolTrueDefaultsPreserved(t *testing.T) {
	dir := t.TempDir()

	t.Run("unmentioned bool-true fields get defaults", func(t *testing.T) {
		yamlPath := filepath.Join(dir, "no-metrics.yaml")
		os.WriteFile(yamlPath, []byte(`
logging:
  level: debug
`), 0o600)

		cfg, err := Load(yamlPath)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if !cfg.Metrics.Enabled {
			t.Error("Metrics.Enabled should default to true when not in YAML")
		}
		if !cfg.KV.AutoProvision {
			t.Error("KV.AutoProvision should default to true when not in YAML")
		}
	})

	t.Run("explicit false is honored", func(t *testing.T) {
		yamlPath := filepath.Join(dir, "disabled.yaml")
		os.WriteFile(yamlPath, []byte(`
metrics:
  enabled: false
kv:
  autoProvision: false
`), 0o600)

		cfg, err := Load(yamlPath)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.Metrics.Enabled {
			t.Error("Metrics.Enabled should be false when explicitly set")
		}
		if cfg.KV.AutoProvision {
			t.Error("KV.AutoProvision should be false when explicitly set")
		}
	})

	t.Run("explicit true is honored", func(t *testing.T) {
		yamlPath := filepath.Join(dir, "enabled.yaml")
		os.WriteFile(yamlPath, []byte(`
metrics:
  enabled: true
kv:
  autoProvision: true
`), 0o600)

		cfg, err := Load(yamlPath)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if !cfg.Metrics.Enabled {
			t.Error("Metrics.Enabled should be true when explicitly set")
		}
		if !cfg.KV.AutoProvision {
			t.Error("KV.AutoProvision should be true when explicitly set")
		}
	})
}

func TestApplyOverrides(t *testing.T) {
	cfg := newDefaults()

	enabled := true
	wc := 16
	cfg.ApplyOverrides(ServeOverrides{
		NATSURLs:       []string{"nats://custom:4222"},
		LogLevel:       "debug",
		MetricsEnabled: &enabled,
		MetricsAddr:    ":9090",
		WorkerCount:    &wc,
	})

	if cfg.NATS.URLs[0] != "nats://custom:4222" {
		t.Errorf("NATS.URLs = %v, want [nats://custom:4222]", cfg.NATS.URLs)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %s, want debug", cfg.Logging.Level)
	}
	if !cfg.Metrics.Enabled {
		t.Error("Metrics.Enabled should be true")
	}
	if cfg.Metrics.Address != ":9090" {
		t.Errorf("Metrics.Address = %s, want :9090", cfg.Metrics.Address)
	}
	if cfg.NATS.Consumers.WorkerCount != 16 {
		t.Errorf("WorkerCount = %d, want 16", cfg.NATS.Consumers.WorkerCount)
	}
}

func TestApplyOverridesNilPointerSkips(t *testing.T) {
	cfg := newDefaults()
	original := cfg.Metrics.Enabled

	cfg.ApplyOverrides(ServeOverrides{})

	if cfg.Metrics.Enabled != original {
		t.Errorf("Metrics.Enabled changed from %v to %v with nil override", original, cfg.Metrics.Enabled)
	}
}
