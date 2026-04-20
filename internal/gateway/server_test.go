package gateway

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/danielmichaels/shunt/internal/testutil"
)

func nopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func defaultServerCfg() *ServerConfig {
	return &ServerConfig{
		Address:             ":0",
		InboundWorkerCount:  2,
		InboundQueueSize:    10,
		ShutdownGracePeriod: time.Second,
	}
}

func defaultPublishCfg() *config.PublishConfig {
	return &config.PublishConfig{
		Mode:           "jetstream",
		AckTimeout:     5 * time.Second,
		MaxRetries:     3,
		RetryBaseDelay: 10 * time.Millisecond,
	}
}

// TestInboundServer_PerRuleModeOverride confirms that when global mode is
// "jetstream" but a rule sets action.Mode="core", the message is delivered
// via core NATS (visible to a plain subscriber, no stream required).
func TestInboundServer_PerRuleModeOverride(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	sub, err := nc.SubscribeSync("test.override")
	require.NoError(t, err)
	defer sub.Unsubscribe()

	cfg := defaultPublishCfg() // global: jetstream
	srv := NewInboundServer(nopLogger(), nil, nil, js, nc, defaultServerCfg(), cfg)

	action := &rule.NATSAction{
		Subject: "test.override",
		Payload: "hello",
		Mode:    "core", // per-rule override
	}

	err = srv.publisher.PublishAction(context.Background(), action, "test-rule", "")
	require.NoError(t, err)

	msg, err := sub.NextMsg(time.Second)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(msg.Data))
}

// TestInboundServer_RetryOnNoStream confirms that publishing to a subject
// with no JetStream stream retries and ultimately fails (not silently dropped).
func TestInboundServer_RetryOnNoStream(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	cfg := defaultPublishCfg()
	cfg.MaxRetries = 2
	srv := NewInboundServer(nopLogger(), nil, nil, js, nc, defaultServerCfg(), cfg)

	action := &rule.NATSAction{Subject: "no.stream.subject", Payload: "data"}

	err := srv.publisher.PublishAction(context.Background(), action, "test-rule", "")
	assert.Error(t, err, "publish to subject with no stream must fail, not be silently dropped")
}
