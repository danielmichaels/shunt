package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type ruleListEntry struct {
	Key       string `json:"key" yaml:"key"`
	Revision  uint64 `json:"revision" yaml:"revision"`
	Operation string `json:"operation" yaml:"operation"`
	Size      int    `json:"size" yaml:"size"`
	Created   string `json:"created" yaml:"created"`
}

type KVListCmd struct {
	Format string `short:"f" default:"table" help:"Output format (table, json, yaml)"`
}

func (l *KVListCmd) Run(kv *KVCmd) error {
	format, err := validateFormat(l.Format)
	if err != nil {
		return err
	}

	nc, bucket, err := kv.connectToNATS()
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
	defer cancel()

	keys, err := bucket.Keys(ctx)
	if errors.Is(err, jetstream.ErrNoKeysFound) {
		fmt.Fprintln(os.Stderr, "No rules found in bucket")
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	var entries []ruleListEntry
	for _, key := range keys {
		entry, err := bucket.Get(ctx, key)
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
}
