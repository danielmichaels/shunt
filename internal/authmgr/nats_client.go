// file: internal/authmgr/nats_client.go

package authmgr

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danielmichaels/shunt/internal/natsutil"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"log/slog"
)

// Timeout and retry constants for NATS client operations
const (
	// natsKVOperationTimeout is the maximum time for KV store operations
	natsKVOperationTimeout = 10 * time.Second

	// natsReconnectWait is the delay between NATS reconnection attempts
	natsReconnectWait = 50 * time.Millisecond
)

// NATSClient provides minimal NATS KV write functionality
// No subscriptions, no consumers, no streams - just connect and write to KV bucket
type NATSClient struct {
	conn   *nats.Conn
	js     jetstream.JetStream
	kv     jetstream.KeyValue
	logger *slog.Logger
	config *NATSConfig
	shared bool
}

// NewNATSClient creates a NATS client and opens KV bucket
func NewNATSClient(cfg *NATSConfig, storageConfig *StorageConfig, log *slog.Logger) (*NATSClient, error) {
	log.Info("connecting to NATS", "urls", cfg.URLs)

	// Build connection options (same pattern as broker package)
	opts, err := buildNATSOptions(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to build NATS options: %w", err)
	}

	// Connect to NATS
	urlString := strings.Join(cfg.URLs, ",")
	nc, err := nats.Connect(urlString, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	connectedURL := nc.ConnectedUrl()
	log.Info("NATS connection established", "connectedURL", connectedURL)

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), natsKVOperationTimeout)
	defer cancel()

	kv, err := js.KeyValue(ctx, storageConfig.Bucket)
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketNotFound) && storageConfig.AutoProvision {
			kv, err = js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: storageConfig.Bucket})
			if err != nil {
				nc.Close()
				return nil, fmt.Errorf("failed to auto-create KV bucket '%s': %w", storageConfig.Bucket, err)
			}
			log.Info("auto-provisioned KV bucket", "bucket", storageConfig.Bucket)
		} else if errors.Is(err, jetstream.ErrBucketNotFound) {
			nc.Close()
			return nil, fmt.Errorf("KV bucket '%s' not found. Create it with: nats kv add %s",
				storageConfig.Bucket, storageConfig.Bucket)
		} else {
			nc.Close()
			return nil, fmt.Errorf("failed to open KV bucket '%s': %w", storageConfig.Bucket, err)
		}
	}

	log.Info("KV bucket opened successfully", "bucket", storageConfig.Bucket)

	return &NATSClient{
		conn:   nc,
		js:     js,
		kv:     kv,
		logger: log,
		config: cfg,
	}, nil
}

// NewNATSClientFromConn creates a NATSClient that reuses an existing NATS connection.
// Used when the auth-manager runs as a subsystem of shunt.
func NewNATSClientFromConn(nc *nats.Conn, storageConfig *StorageConfig, log *slog.Logger) (*NATSClient, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), natsKVOperationTimeout)
	defer cancel()

	kv, err := js.KeyValue(ctx, storageConfig.Bucket)
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketNotFound) && storageConfig.AutoProvision {
			kv, err = js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: storageConfig.Bucket})
			if err != nil {
				return nil, fmt.Errorf("failed to auto-create KV bucket '%s': %w", storageConfig.Bucket, err)
			}
			log.Info("auto-provisioned KV bucket", "bucket", storageConfig.Bucket)
		} else if errors.Is(err, jetstream.ErrBucketNotFound) {
			return nil, fmt.Errorf("KV bucket '%s' not found. Create it with: nats kv add %s",
				storageConfig.Bucket, storageConfig.Bucket)
		} else {
			return nil, fmt.Errorf("failed to open KV bucket '%s': %w", storageConfig.Bucket, err)
		}
	}

	log.Info("KV bucket opened successfully (shared connection)", "bucket", storageConfig.Bucket)

	return &NATSClient{
		conn:   nc,
		js:     js,
		kv:     kv,
		logger: log,
		shared: true,
	}, nil
}

// StoreToken writes a token to the KV bucket
func (c *NATSClient) StoreToken(ctx context.Context, key, token string) error {
	c.logger.Debug("storing token in KV", "key", key)

	_, err := c.kv.Put(ctx, key, []byte(token))
	if err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}

	c.logger.Debug("token stored successfully", "key", key)
	return nil
}

// Close gracefully closes the NATS connection.
// When using a shared connection, it skips draining since the owner manages the connection.
func (c *NATSClient) Close() error {
	if c.shared {
		c.logger.Info("auth-manager NATS client closed (shared connection, skipping drain)")
		return nil
	}

	c.logger.Info("closing NATS connection")

	if err := c.conn.Drain(); err != nil {
		return fmt.Errorf("failed to drain connection: %w", err)
	}

	c.logger.Info("NATS connection closed")
	return nil
}

// buildNATSOptions creates NATS connection options with auth and TLS
func buildNATSOptions(cfg *NATSConfig, log *slog.Logger) ([]nats.Option, error) {
	var opts []nats.Option

	opts = append(opts,
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Warn("NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Error("NATS connection closed", "error", nc.LastError())
		}),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(natsReconnectWait),
	)

	authTLSOpts, err := natsutil.BuildAuthTLSOptions(natsutil.AuthTLSConfig{
		CredsFile:   cfg.CredsFile,
		NKey:        cfg.NKey,
		Token:       cfg.Token,
		Username:    cfg.Username,
		Password:    cfg.Password,
		TLSEnable:   cfg.TLS.Enable,
		TLSCertFile: cfg.TLS.CertFile,
		TLSKeyFile:  cfg.TLS.KeyFile,
		TLSCAFile:   cfg.TLS.CAFile,
		TLSInsecure: cfg.TLS.Insecure,
	}, log)
	if err != nil {
		return nil, err
	}
	opts = append(opts, authTLSOpts...)

	return opts, nil
}
