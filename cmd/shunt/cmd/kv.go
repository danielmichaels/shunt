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
	"github.com/spf13/cobra"
)

const kvOperationTimeout = 10 * time.Second

var kvCmd = &cobra.Command{
	Use:   "kv",
	Short: "Manage rules in a NATS KV bucket",
	Long: `The kv command group provides subcommands for managing rules stored in a NATS KV bucket.
Use push, pull, list, and delete to interact with the rule store.`,
}

func init() {
	kvCmd.PersistentFlags().String("nats-url", natsutil.DefaultURL, "NATS server URL (or set SHUNT_NATS_URL / NATS_URL)")
	kvCmd.PersistentFlags().String("creds", "", "Path to NATS credentials file (or set SHUNT_NATS_CREDS / NATS_CREDS)")
	kvCmd.PersistentFlags().String("nkey", "", "NATS NKey seed (or set SHUNT_NATS_NKEY / NATS_NKEY)")
	kvCmd.PersistentFlags().String("bucket", "rules", "KV bucket name")

	kvCmd.AddCommand(pushCmd)
	kvCmd.AddCommand(pullCmd)
	kvCmd.AddCommand(listCmd)
	kvCmd.AddCommand(deleteCmd)
}

func connectNATS(cmd *cobra.Command) (*nats.Conn, error) {
	natsURL, _ := cmd.Flags().GetString("nats-url")
	creds, _ := cmd.Flags().GetString("creds")
	nkey, _ := cmd.Flags().GetString("nkey")

	if !cmd.Flags().Changed("nats-url") {
		if v := natsutil.ResolveEnv("SHUNT_NATS_URL", "NATS_URL"); v != "" {
			natsURL = v
		}
	}
	if !cmd.Flags().Changed("creds") {
		if v := natsutil.ResolveEnv("SHUNT_NATS_CREDS", "NATS_CREDS"); v != "" {
			creds = v
		}
	}
	if !cmd.Flags().Changed("nkey") {
		if v := natsutil.ResolveEnv("SHUNT_NATS_NKEY", "NATS_NKEY"); v != "" {
			nkey = v
		}
	}

	opts, err := natsutil.BuildAuthTLSOptions(natsutil.AuthTLSConfig{
		CredsFile: creds,
		NKey:      nkey,
	}, nil)
	if err != nil {
		return nil, err
	}

	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS at %s: %w", natsURL, err)
	}

	return nc, nil
}

func openKVBucket(cmd *cobra.Command, nc *nats.Conn) (jetstream.KeyValue, error) {
	bucket, _ := cmd.Flags().GetString("bucket")

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
	defer cancel()

	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		if errors.Is(err, jetstream.ErrBucketNotFound) {
			return nil, fmt.Errorf("KV bucket '%s' not found. Create it with: nats kv add %s", bucket, bucket)
		}
		return nil, fmt.Errorf("failed to open KV bucket '%s': %w", bucket, err)
	}

	return kv, nil
}

func connectToNATS(cmd *cobra.Command) (*nats.Conn, jetstream.KeyValue, error) {
	s := newSpinner("Connecting to NATS...")
	spinStart(s)

	nc, err := connectNATS(cmd)
	if err != nil {
		spinStop(s)
		return nil, nil, err
	}

	if s != nil {
		s.Suffix = " Getting KV bucket..."
	}
	kv, err := openKVBucket(cmd, nc)
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
