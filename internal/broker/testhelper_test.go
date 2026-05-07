package broker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/danielmichaels/shunt/internal/testutil"
)

func setupNATS(t *testing.T) (*nats.Conn, jetstream.JetStream, func()) {
	t.Helper()
	return testutil.SetupNATS(t)
}

// newMinimalBroker builds a NATSBroker with only the fields needed for KV watch tests.
// subscriptionMgr is intentionally nil — AddAndStartSubscription will return an error
// which the KV manager logs and skips; this is fine for HTTP-only rule tests.
func newMinimalBroker(t *testing.T, js jetstream.JetStream) *NATSBroker {
	t.Helper()
	log := logger.NewNopLogger()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cfg, _ := config.Load("")
	return &NATSBroker{
		config:         cfg,
		logger:         log,
		jetStream:      js,
		kvStores:       make(map[string]jetstream.KeyValue),
		consumers:      make(map[string]consumerRef),
		streamResolver: NewStreamResolver(js, log),
		ctx:            ctx,
		cancel:         cancel,
	}
}

func newFullBroker(t *testing.T, nc *nats.Conn, js jetstream.JetStream, processor *rule.Processor) *NATSBroker {
	t.Helper()
	b := newMinimalBroker(t, js)
	b.natsConn = nc
	b.InitializeSubscriptionManager(processor)
	t.Cleanup(func() { _ = b.subscriptionMgr.Stop() })
	return b
}

// noopSubscriber satisfies subscriptionController without touching JetStream.
// Use this in tests that exercise rule-loading and outbound-registration logic
// independently of the broker's NATS subscription mechanics.
type noopSubscriber struct{}

func (noopSubscriber) AddAndStartSubscription(string) error { return nil }
func (noopSubscriber) RemoveSubscription(string)            {}

func consumerExists(t *testing.T, js jetstream.JetStream, stream, consumer string) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s, err := js.Stream(ctx, stream)
	if err != nil {
		t.Logf("consumerExists: stream lookup error (treating as absent): %v", err)
		return false
	}
	_, err = s.Consumer(ctx, consumer)
	if err == nil {
		return true
	}
	if !errors.Is(err, jetstream.ErrConsumerNotFound) {
		t.Logf("consumerExists: consumer lookup error (treating as absent): %v", err)
	}
	return false
}
