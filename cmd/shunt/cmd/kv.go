package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danielmichaels/shunt/internal/natsutil"

	"github.com/briandowns/spinner"
	"github.com/mattn/go-isatty"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const kvOperationTimeout = 10 * time.Second

type KVCmd struct {
	NatsURL string `name:"nats-url" default:"nats://localhost:4222" env:"SHUNT_NATS_URL,NATS_URL" help:"NATS server URL"`
	Creds   string `env:"SHUNT_NATS_CREDS,NATS_CREDS" help:"Path to NATS credentials file"`
	NKey    string `name:"nkey" env:"SHUNT_NATS_NKEY,NATS_NKEY" help:"NATS NKey seed"`
	Bucket  string `default:"rules" help:"KV bucket name"`

	Push   KVPushCmd   `cmd:"" help:"Push rules to KV bucket" aliases:"put"`
	Pull   KVPullCmd   `cmd:"" help:"Pull rules from KV bucket" aliases:"get"`
	List   KVListCmd   `cmd:"" help:"List rules in KV bucket" aliases:"ls"`
	Delete KVDeleteCmd `cmd:"" help:"Delete a rule from KV bucket" aliases:"rm"`
}

func (k *KVCmd) connectNATS() (*nats.Conn, error) {
	opts, err := natsutil.BuildAuthTLSOptions(natsutil.AuthTLSConfig{
		CredsFile: k.Creds,
		NKey:      k.NKey,
	}, nil)
	if err != nil {
		return nil, err
	}

	nc, err := nats.Connect(k.NatsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS at %s: %w", k.NatsURL, err)
	}

	return nc, nil
}

func (k *KVCmd) openKVBucket(nc *nats.Conn) (jetstream.KeyValue, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
	defer cancel()

	kv, err := js.KeyValue(ctx, k.Bucket)
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketNotFound) {
			return nil, fmt.Errorf("KV bucket '%s' not found. Create it with: nats kv add %s", k.Bucket, k.Bucket)
		}
		return nil, fmt.Errorf("failed to open KV bucket '%s': %w", k.Bucket, err)
	}

	return kv, nil
}

func (k *KVCmd) connectToNATS() (*nats.Conn, jetstream.KeyValue, error) {
	s := newSpinner("Connecting to NATS...")
	spinStart(s)

	nc, err := k.connectNATS()
	if err != nil {
		spinStop(s)
		return nil, nil, err
	}

	if s != nil {
		s.Suffix = " Getting KV bucket..."
	}
	kv, err := k.openKVBucket(nc)
	if err != nil {
		spinStop(s)
		nc.Close()
		return nil, nil, err
	}

	spinStop(s)
	return nc, kv, nil
}

func newSpinner(suffix string) *spinner.Spinner {
	if !isatty.IsTerminal(os.Stderr.Fd()) && !isatty.IsCygwinTerminal(os.Stderr.Fd()) {
		return nil
	}
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
	s.Suffix = " " + suffix
	return s
}

func spinStart(s *spinner.Spinner) {
	if s != nil {
		s.Start()
	}
}

func spinStop(s *spinner.Spinner) {
	if s != nil {
		s.Stop()
	}
}

func sanitizeKVKey(filename string) string {
	key := strings.TrimSuffix(filename, ".yaml")
	key = strings.TrimSuffix(key, ".yml")
	key = strings.ReplaceAll(key, "/", ".")
	key = strings.ReplaceAll(key, "\\", ".")
	return key
}

func deriveKVKey(filePath, bucket string) string {
	key := sanitizeKVKey(filePath)
	return strings.TrimPrefix(key, bucket+".")
}
