package broker

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/logger"
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
		consumers:      make(map[string]string),
		streamResolver: NewStreamResolver(js, log),
		ctx:            ctx,
		cancel:         cancel,
	}
}
