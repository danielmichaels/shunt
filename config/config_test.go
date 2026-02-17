package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestSetViperDefaults(t *testing.T) {
	v := viper.New()
	setViperDefaults(v)

	checks := []struct {
		key  string
		want any
	}{
		{"nats.connection.maxReconnects", -1},
		{"nats.connection.reconnectWait", DefaultReconnectWait},
		{"nats.consumers.consumerPrefix", DefaultConsumerPrefix},
		{"nats.consumers.workerCount", DefaultWorkerCount},
		{"nats.consumers.fetchBatchSize", DefaultFetchBatchSize},
		{"nats.consumers.fetchTimeout", DefaultFetchTimeout},
		{"nats.consumers.maxAckPending", DefaultMaxAckPending},
		{"nats.consumers.ackWaitTimeout", DefaultAckWaitTimeout},
		{"nats.consumers.maxDeliver", DefaultMaxDeliver},
		{"nats.consumers.deliverPolicy", DefaultDeliverPolicy},
		{"nats.consumers.replayPolicy", DefaultReplayPolicy},
		{"nats.publish.mode", DefaultPublishMode},
		{"nats.publish.ackTimeout", DefaultPublishAckTimeout},
		{"nats.publish.maxRetries", DefaultPublishMaxRetries},
		{"nats.publish.retryBaseDelay", DefaultRetryBaseDelay},
		{"nats.tls.enable", false},
		{"http.server.address", DefaultHTTPServerAddress},
		{"http.server.readTimeout", DefaultHTTPReadTimeout},
		{"http.server.writeTimeout", DefaultHTTPWriteTimeout},
		{"http.server.idleTimeout", DefaultHTTPIdleTimeout},
		{"http.server.maxHeaderBytes", DefaultMaxHeaderBytes},
		{"http.server.shutdownGracePeriod", DefaultHTTPShutdownGracePeriod},
		{"http.server.inboundWorkerCount", 10},
		{"http.server.inboundQueueSize", DefaultInboundQueueSize},
		{"http.client.timeout", DefaultHTTPClientTimeout},
		{"http.client.maxIdleConns", DefaultMaxIdleConns},
		{"http.client.maxIdleConnsPerHost", DefaultMaxIdleConnsPerHost},
		{"http.client.idleConnTimeout", DefaultHTTPIdleConnTimeout},
		{"logging.level", DefaultLogLevel},
		{"logging.encoding", DefaultLogEncoding},
		{"logging.outputPath", DefaultLogOutput},
		{"metrics.enabled", true},
		{"metrics.address", DefaultMetricsAddress},
		{"metrics.path", DefaultMetricsPath},
		{"metrics.updateInterval", "15s"},
		{"rules.kvBucket", "rules"},
		{"security.verification.publicKeyHeader", DefaultPublicKeyHeader},
		{"security.verification.signatureHeader", DefaultSignatureHeader},
		{"forEach.maxIterations", DefaultForEachMaxIterations},
		{"gateway.enabled", false},
		{"authManager.enabled", false},
	}

	for _, c := range checks {
		got := v.Get(c.key)
		if got != c.want {
			t.Errorf("key %q = %v (%T), want %v (%T)", c.key, got, got, c.want, c.want)
		}
	}

	urls := v.GetStringSlice("nats.urls")
	if len(urls) != 1 || urls[0] != DefaultNATSURL {
		t.Errorf("nats.urls = %v, want [%s]", urls, DefaultNATSURL)
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

// validConfig creates a minimal valid config by loading defaults through viper,
// matching how Load() works in production.
func validConfig() *Config {
	v := viper.New()
	setViperDefaults(v)
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic("failed to unmarshal defaults: " + err.Error())
	}
	applyConditionalDefaults(&cfg)
	return &cfg
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

func TestLoadWithEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(yamlPath, []byte(`
nats:
  tls:
    enable: true
logging:
  level: warn
`), 0o600)

	t.Setenv("SHUNT_NATS_TLS_ENABLE", "false")
	t.Setenv("SHUNT_LOGGING_LEVEL", "debug")

	cfg, err := Load(yamlPath, nil)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.NATS.TLS.Enable {
		t.Error("expected SHUNT_NATS_TLS_ENABLE=false to override yaml tls.enable=true")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected SHUNT_LOGGING_LEVEL=debug to override yaml level=warn, got %s", cfg.Logging.Level)
	}
}

func TestLoadWithViperOverrides(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(yamlPath, []byte(`
logging:
  level: warn
`), 0o600)

	v := viper.New()
	v.Set("logging.level", "debug")

	cfg, err := Load(yamlPath, v)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected viper.Set to override file value, got %s", cfg.Logging.Level)
	}
}

func TestLoadDefaultsWithoutConfigFile(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(yamlPath, []byte("{}"), 0o600)

	cfg, err := Load(yamlPath, nil)
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
