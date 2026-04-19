package authmgr

import (
	"context"
	"os"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"

	"github.com/danielmichaels/shunt/internal/testutil"
)

func setupTestNATSClient(t *testing.T, keyPrefix string) (*NATSClient, func()) {
	t.Helper()
	nc, js, cleanup := testutil.SetupNATS(t)

	kv, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{Bucket: "test-tokens"})
	require.NoError(t, err)

	client := &NATSClient{
		conn:      nc,
		js:        js,
		kv:        kv,
		logger:    slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		keyPrefix: keyPrefix,
	}
	return client, cleanup
}

func TestStoreToken_NoPrefix(t *testing.T) {
	client, cleanup := setupTestNATSClient(t, "")
	defer cleanup()

	err := client.StoreToken(context.Background(), "github", "mytoken")
	require.NoError(t, err)

	entry, err := client.kv.Get(context.Background(), "github")
	require.NoError(t, err)
	assert.Equal(t, "mytoken", string(entry.Value()))
}

func TestStoreToken_WithPrefix(t *testing.T) {
	client, cleanup := setupTestNATSClient(t, "auth.")
	defer cleanup()

	err := client.StoreToken(context.Background(), "github", "mytoken")
	require.NoError(t, err)

	entry, err := client.kv.Get(context.Background(), "auth.github")
	require.NoError(t, err)
	assert.Equal(t, "mytoken", string(entry.Value()))

	_, err = client.kv.Get(context.Background(), "github")
	assert.Error(t, err, "bare key must not exist when prefix is set")
}

func TestKeyWithPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		key    string
		want   string
	}{
		{"", "github", "github"},
		{"auth.", "github", "auth.github"},
		{"ns:", "slack", "ns:slack"},
	}
	for _, tt := range tests {
		c := &NATSClient{keyPrefix: tt.prefix}
		assert.Equal(t, tt.want, c.keyWithPrefix(tt.key), "prefix=%q key=%q", tt.prefix, tt.key)
	}
}
