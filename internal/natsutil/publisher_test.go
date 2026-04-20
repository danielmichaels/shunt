package natsutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
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

func defaultPublishCfg() *config.PublishConfig {
	return &config.PublishConfig{
		Mode:           "jetstream",
		AckTimeout:     5 * time.Second,
		MaxRetries:     3,
		RetryBaseDelay: 10 * time.Millisecond,
	}
}

func setupStream(t *testing.T, js jetstream.JetStream, subject string) {
	t.Helper()
	_, err := js.CreateStream(context.Background(), jetstream.StreamConfig{
		Name:     "TEST",
		Subjects: []string{subject},
	})
	require.NoError(t, err)
}

func TestPublishAction_JetStream_Success(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	setupStream(t, js, "test.subject")

	p := NewPublisher(nc, js, defaultPublishCfg(), nil, nopLogger())
	action := &rule.NATSAction{Subject: "test.subject", Payload: "hello"}

	err := p.PublishAction(context.Background(), action, "rule1", "")
	assert.NoError(t, err)
}

func TestPublishAction_Core_Success(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	sub, err := nc.SubscribeSync("test.core")
	require.NoError(t, err)
	defer sub.Unsubscribe()

	cfg := defaultPublishCfg()
	cfg.Mode = "core"
	p := NewPublisher(nc, js, cfg, nil, nopLogger())
	action := &rule.NATSAction{Subject: "test.core", Payload: "ping"}

	err = p.PublishAction(context.Background(), action, "rule1", "")
	require.NoError(t, err)

	msg, err := sub.NextMsg(time.Second)
	require.NoError(t, err)
	assert.Equal(t, "ping", string(msg.Data))
}

func TestPublishAction_PerRuleModeOverride(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	sub, err := nc.SubscribeSync("test.override")
	require.NoError(t, err)
	defer sub.Unsubscribe()

	// Global mode jetstream, but action overrides to core
	p := NewPublisher(nc, js, defaultPublishCfg(), nil, nopLogger())
	action := &rule.NATSAction{Subject: "test.override", Payload: "override", Mode: "core"}

	err = p.PublishAction(context.Background(), action, "rule1", "")
	require.NoError(t, err)

	msg, err := sub.NextMsg(time.Second)
	require.NoError(t, err)
	assert.Equal(t, "override", string(msg.Data))
}

func TestPublishAction_ContextCancelled(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	p := NewPublisher(nc, js, defaultPublishCfg(), nil, nopLogger())
	action := &rule.NATSAction{Subject: "test.cancel", Payload: "data"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.PublishAction(ctx, action, "rule1", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

func TestPublishAction_NoStream_Fails(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	cfg := defaultPublishCfg()
	cfg.MaxRetries = 1
	p := NewPublisher(nc, js, cfg, nil, nopLogger())
	action := &rule.NATSAction{Subject: "no.stream.here", Payload: "data"}

	err := p.PublishAction(context.Background(), action, "rule1", "")
	assert.Error(t, err)
}

func TestPublishAction_WithHeaders(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	sub, err := nc.SubscribeSync("test.headers")
	require.NoError(t, err)
	defer sub.Unsubscribe()

	cfg := defaultPublishCfg()
	cfg.Mode = "core"
	p := NewPublisher(nc, js, cfg, nil, nopLogger())
	action := &rule.NATSAction{
		Subject: "test.headers",
		Payload: "body",
		Headers: map[string]string{"X-Foo": "bar"},
	}

	err = p.PublishAction(context.Background(), action, "rule1", "trace-123")
	require.NoError(t, err)

	msg, err := sub.NextMsg(time.Second)
	require.NoError(t, err)
	assert.Equal(t, "bar", msg.Header.Get("X-Foo"))
	assert.Equal(t, "trace-123", msg.Header.Get("Nats-Trace-Id"))
}

func TestPublishAction_Passthrough(t *testing.T) {
	nc, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	sub, err := nc.SubscribeSync("test.passthrough")
	require.NoError(t, err)
	defer sub.Unsubscribe()

	cfg := defaultPublishCfg()
	cfg.Mode = "core"
	p := NewPublisher(nc, js, cfg, nil, nopLogger())
	raw := []byte(`{"raw":true}`)
	action := &rule.NATSAction{
		Subject:     "test.passthrough",
		Passthrough: true,
		RawPayload:  raw,
	}

	err = p.PublishAction(context.Background(), action, "rule1", "")
	require.NoError(t, err)

	msg, err := sub.NextMsg(time.Second)
	require.NoError(t, err)
	assert.Equal(t, raw, msg.Data)
}
