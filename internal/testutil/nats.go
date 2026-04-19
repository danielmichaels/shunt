package testutil

import (
	"os"
	"testing"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

func RunJetStreamServer(t *testing.T) *server.Server {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	return natsserver.RunServer(&opts)
}

func ShutdownServer(t *testing.T, s *server.Server) {
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

func SetupNATS(t *testing.T) (*nats.Conn, jetstream.JetStream, func()) {
	t.Helper()
	s := RunJetStreamServer(t)

	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	cleanup := func() {
		nc.Close()
		ShutdownServer(t, s)
	}
	return nc, js, cleanup
}
