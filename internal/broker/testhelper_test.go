package broker

import (
	"context"
	"os"
	"testing"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/logger"
)

func runJetStreamServer(t *testing.T) *server.Server {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	return natsserver.RunServer(&opts)
}

func shutdownServer(t *testing.T, s *server.Server) {
	t.Helper()
	var sd string
	if cfg := s.JetStreamConfig(); cfg != nil {
		sd = cfg.StoreDir
	}
	s.Shutdown()
	if sd != "" {
		if err := os.RemoveAll(sd); err != nil {
			t.Fatalf("unable to remove storage %q: %v", sd, err)
		}
	}
	s.WaitForShutdown()
}

func setupNATS(t *testing.T) (*nats.Conn, jetstream.JetStream, func()) {
	t.Helper()
	s := runJetStreamServer(t)

	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	cleanup := func() {
		nc.Close()
		shutdownServer(t, s)
	}
	return nc, js, cleanup
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
