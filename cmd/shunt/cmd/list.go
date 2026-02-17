package cmd

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all rules in the NATS KV bucket",
	RunE: func(cmd *cobra.Command, args []string) error {
		nc, err := connectNATS(cmd)
		if err != nil {
			return err
		}
		defer nc.Close()

		kv, err := openKVBucket(cmd, nc)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
		defer cancel()

		keys, err := kv.Keys(ctx)
		if err != nil {
			return fmt.Errorf("failed to list keys: %w", err)
		}

		if len(keys) == 0 {
			fmt.Println("No rules found in bucket")
			return nil
		}

		bucket, _ := cmd.Flags().GetString("bucket")
		fmt.Printf("Rules in bucket '%s':\n\n", bucket)

		for _, key := range keys {
			entry, err := kv.Get(ctx, key)
			if err != nil {
				fmt.Printf("  %-30s  (error: %v)\n", key, err)
				continue
			}

			op := "PUT"
			if entry.Operation() == jetstream.KeyValueDelete {
				op = "DEL"
			}

			fmt.Printf("  %-30s  rev=%d  op=%s  size=%d bytes\n",
				key, entry.Revision(), op, len(entry.Value()))
		}

		fmt.Printf("\nTotal: %d keys\n", len(keys))
		return nil
	},
}
