package authmgr

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/danielmichaels/shunt/config"
	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)\}`)

type Config struct {
	NATS      NATSConfig       `yaml:"nats"`
	Storage   StorageConfig    `yaml:"storage"`
	Logging   config.LogConfig `yaml:"logging"`
	Metrics   MetricsConfig    `yaml:"metrics"`
	Providers []ProviderConfig `yaml:"providers"`
}

type NATSConfig struct {
	URLs      []string `yaml:"urls"`
	Username  string   `yaml:"username"`
	Password  string   `yaml:"password"`
	Token     string   `yaml:"token"`
	NKey      string   `yaml:"nkey"`
	CredsFile string   `yaml:"credsFile"`

	TLS struct {
		Enable   bool   `yaml:"enable"`
		CertFile string `yaml:"certFile"`
		KeyFile  string `yaml:"keyFile"`
		CAFile   string `yaml:"caFile"`
		Insecure bool   `yaml:"insecure"`
	} `yaml:"tls"`
}

type StorageConfig struct {
	Bucket        string `yaml:"bucket"`
	KeyPrefix     string `yaml:"keyPrefix"`
	AutoProvision bool   `yaml:"-"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Address string `yaml:"address"`
}

type ProviderConfig struct {
	ID            string `yaml:"id"`
	Type          string `yaml:"type"`
	KVKey         string `yaml:"kvKey"`
	RefreshBefore string `yaml:"refreshBefore"`
	RefreshEvery  string `yaml:"refreshEvery"`

	TokenURL     string   `yaml:"tokenUrl"`
	ClientID     string   `yaml:"clientId"`
	ClientSecret string   `yaml:"clientSecret"`
	Scopes       []string `yaml:"scopes"`

	AuthURL   string            `yaml:"authUrl"`
	Method    string            `yaml:"method"`
	Headers   map[string]string `yaml:"headers"`
	Body      string            `yaml:"body"`
	TokenPath string            `yaml:"tokenPath"`
}

func Load(configPath string) (*Config, error) {
	var cfg Config
	setDefaults(&cfg)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	if err := expandEnvVars(&cfg); err != nil {
		return nil, fmt.Errorf("failed to expand environment variables: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

var unsetEnvVars []string

func expandEnvVars(cfg interface{}) error {
	unsetEnvVars = nil

	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("config must be a non-nil pointer")
	}
	if err := expandRecursive(v); err != nil {
		return err
	}

	if len(unsetEnvVars) > 0 {
		return fmt.Errorf("missing environment variables: %v (set them or remove ${} references from config)", unsetEnvVars)
	}
	return nil
}

func expandRecursive(v reflect.Value) error {
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if err := expandRecursive(v.Field(i)); err != nil {
				return err
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := expandRecursive(v.Index(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			val := v.MapIndex(key)
			if val.Kind() == reflect.Interface {
				val = val.Elem()
			}
			if val.Kind() == reflect.String {
				expanded := expandString(val.String())
				v.SetMapIndex(key, reflect.ValueOf(expanded))
			} else {
				if err := expandRecursive(val); err != nil {
					return err
				}
			}
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString(expandString(v.String()))
		}
	}
	return nil
}

func expandString(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[2 : len(match)-1]
		value := os.Getenv(varName)
		if value == "" {
			found := false
			for _, v := range unsetEnvVars {
				if v == varName {
					found = true
					break
				}
			}
			if !found {
				unsetEnvVars = append(unsetEnvVars, varName)
			}
		}
		return value
	})
}

func setDefaults(cfg *Config) {
	if len(cfg.NATS.URLs) == 0 {
		cfg.NATS.URLs = []string{"nats://localhost:4222"}
	}
	if cfg.Storage.Bucket == "" {
		cfg.Storage.Bucket = "tokens"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Encoding == "" {
		cfg.Logging.Encoding = "json"
	}
	if cfg.Logging.OutputPath == "" {
		cfg.Logging.OutputPath = "stdout"
	}
	if cfg.Metrics.Address == "" {
		cfg.Metrics.Address = ":2113"
	}
}

func validate(cfg *Config) error {
	if len(cfg.NATS.URLs) == 0 {
		return fmt.Errorf("at least one NATS URL required")
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
		return fmt.Errorf("only one NATS auth method allowed")
	}

	if len(cfg.Providers) == 0 {
		return fmt.Errorf("at least one provider required")
	}

	seenIDs := make(map[string]bool)
	for i, p := range cfg.Providers {
		if p.ID == "" {
			return fmt.Errorf("provider %d: id is required", i)
		}
		if seenIDs[p.ID] {
			return fmt.Errorf("provider %d: duplicate id '%s'", i, p.ID)
		}
		seenIDs[p.ID] = true

		if p.Type != "oauth2" && p.Type != "custom-http" {
			return fmt.Errorf("provider %s: invalid type '%s' (must be 'oauth2' or 'custom-http')", p.ID, p.Type)
		}

		if cfg.Providers[i].KVKey == "" {
			cfg.Providers[i].KVKey = p.ID
		}

		if p.Type == "oauth2" {
			if p.TokenURL == "" {
				return fmt.Errorf("provider %s: tokenUrl required for oauth2", p.ID)
			}
			if p.ClientID == "" {
				return fmt.Errorf("provider %s: clientId required for oauth2", p.ID)
			}
			if p.ClientSecret == "" {
				return fmt.Errorf("provider %s: clientSecret required for oauth2", p.ID)
			}
			if p.RefreshBefore == "" {
				return fmt.Errorf("provider %s: refreshBefore required for oauth2", p.ID)
			}
			if _, err := time.ParseDuration(p.RefreshBefore); err != nil {
				return fmt.Errorf("provider %s: invalid refreshBefore duration: %w", p.ID, err)
			}
		} else if p.Type == "custom-http" {
			if p.AuthURL == "" {
				return fmt.Errorf("provider %s: authUrl required for custom-http", p.ID)
			}
			if cfg.Providers[i].Method == "" {
				cfg.Providers[i].Method = "POST"
			}
			if p.TokenPath == "" {
				return fmt.Errorf("provider %s: tokenPath required for custom-http", p.ID)
			}
			if p.RefreshEvery == "" {
				return fmt.Errorf("provider %s: refreshEvery required for custom-http", p.ID)
			}
			if _, err := time.ParseDuration(p.RefreshEvery); err != nil {
				return fmt.Errorf("provider %s: invalid refreshEvery duration: %w", p.ID, err)
			}
		}
	}

	if cfg.Storage.Bucket == "" {
		return fmt.Errorf("storage bucket name cannot be empty")
	}

	if cfg.NATS.TLS.Enable {
		if cfg.NATS.TLS.CertFile != "" && cfg.NATS.TLS.KeyFile == "" {
			return fmt.Errorf("NATS TLS key file required when cert file provided")
		}
		if cfg.NATS.TLS.KeyFile != "" && cfg.NATS.TLS.CertFile == "" {
			return fmt.Errorf("NATS TLS cert file required when key file provided")
		}
	}

	if cfg.NATS.CredsFile != "" {
		if _, err := os.Stat(cfg.NATS.CredsFile); os.IsNotExist(err) {
			return fmt.Errorf("NATS creds file does not exist: %s", cfg.NATS.CredsFile)
		}
	}

	return nil
}
