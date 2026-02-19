package natsutil

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"

	"github.com/nats-io/nats.go"
)

const DefaultURL = "nats://localhost:4222"

// ResolveEnv returns the first non-empty value from SHUNT_-prefixed then
// NATS_-prefixed environment variables. This ensures all CLI commands and
// the server share identical env var precedence.
func ResolveEnv(shuntVar, natsVar string) string {
	if v := os.Getenv(shuntVar); v != "" {
		return v
	}
	return os.Getenv(natsVar)
}

type StructuredLogger interface {
	Info(msg string, args ...any)
}

type AuthTLSConfig struct {
	CredsFile string
	NKey      string
	Token     string
	Username  string
	Password  string

	TLSEnable   bool
	TLSCertFile string
	TLSKeyFile  string
	TLSCAFile   string
	TLSInsecure bool
}

func BuildAuthTLSOptions(cfg AuthTLSConfig, log StructuredLogger) ([]nats.Option, error) {
	if log == nil {
		log = slog.Default()
	}

	var opts []nats.Option

	if cfg.CredsFile != "" {
		log.Info("using NATS creds file authentication", "credsFile", cfg.CredsFile)
		opts = append(opts, nats.UserCredentials(cfg.CredsFile))
	} else if cfg.NKey != "" {
		log.Info("using NATS NKey authentication", "seedFile", cfg.NKey)
		nkeyOpt, err := nats.NkeyOptionFromSeed(cfg.NKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load NKey seed file: %w", err)
		}
		opts = append(opts, nkeyOpt)
	} else if cfg.Token != "" {
		log.Info("using NATS token authentication")
		opts = append(opts, nats.Token(cfg.Token))
	} else if cfg.Username != "" {
		log.Info("using NATS username/password authentication", "username", cfg.Username)
		opts = append(opts, nats.UserInfo(cfg.Username, cfg.Password))
	}

	if cfg.TLSEnable {
		log.Info("enabling TLS for NATS connection", "insecure", cfg.TLSInsecure)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.TLSInsecure,
		}

		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
			if err != nil {
				return nil, fmt.Errorf("failed to load TLS client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{cert}
			log.Info("loaded TLS client certificate", "certFile", cfg.TLSCertFile)
		}

		if cfg.TLSCAFile != "" {
			opts = append(opts, nats.RootCAs(cfg.TLSCAFile))
			log.Info("loaded TLS CA certificate", "caFile", cfg.TLSCAFile)
		}

		opts = append(opts, nats.Secure(tlsConfig))
	}

	return opts, nil
}