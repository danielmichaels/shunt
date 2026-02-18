package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
)

type ruleListEntry struct {
	Key       string `json:"key" yaml:"key"`
	Revision  uint64 `json:"revision" yaml:"revision"`
	Operation string `json:"operation" yaml:"operation"`
	Size      int    `json:"size" yaml:"size"`
	Created   string `json:"created" yaml:"created"`
}

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all rules in the NATS KV bucket",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		format, err := validateFormat(format)
		if err != nil {
			return err
		}

		nc, kv, err := connectToNATS(cmd)
		if err != nil {
			return err
		}
		defer nc.Close()

		ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
		defer cancel()

		keys, err := kv.Keys(ctx)
		if err != nil {
			return fmt.Errorf("failed to list keys: %w", err)
		}

		if len(keys) == 0 {
			fmt.Fprintln(os.Stderr, "No rules found in bucket")
			return nil
		}

		var entries []ruleListEntry
		for _, key := range keys {
			entry, err := kv.Get(ctx, key)
			if err != nil {
				continue
			}
			op := "PUT"
			if entry.Operation() == jetstream.KeyValueDelete {
				op = "DEL"
			}
			entries = append(entries, ruleListEntry{
				Key:       entry.Key(),
				Revision:  entry.Revision(),
				Operation: op,
				Size:      len(entry.Value()),
				Created:   entry.Created().Format(time.DateTime),
			})
		}

		headers := []string{"NAME", "REV", "OP", "SIZE", "CREATED"}
		return formatOutput(format, entries, headers, func() [][]string {
			rows := make([][]string, len(entries))
			for i, e := range entries {
				rows[i] = []string{
					e.Key,
					fmt.Sprintf("%d", e.Revision),
					e.Operation,
					fmt.Sprintf("%d B", e.Size),
					e.Created,
				}
			}
			return rows
		})
	},
}

func init() {
	listCmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml)")
}
