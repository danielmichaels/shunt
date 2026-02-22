package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

type KVDeleteCmd struct {
	Key   string `arg:"" help:"Key to delete"`
	Force bool   `help:"Skip confirmation prompt"`
}

func (d *KVDeleteCmd) BeforeApply(ctx *kong.Context) error {
	if d.Key == "--help" || d.Key == "-h" {
		_ = ctx.PrintUsage(false)
		ctx.Kong.Exit(0)
	}
	return nil
}

func (d *KVDeleteCmd) Run(kv *KVCmd) error {
	if !d.Force {
		fmt.Fprintf(os.Stderr, "Delete rule '%s' from KV bucket? [y/N] ", d.Key)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stderr, "Cancelled")
			return nil
		}
	}

	nc, bucket, err := kv.connectToNATS()
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
	defer cancel()

	if err := bucket.Delete(ctx, d.Key); err != nil {
		return fmt.Errorf("failed to delete key '%s': %w", d.Key, err)
	}

	fmt.Fprintf(os.Stderr, "Deleted '%s'\n", d.Key)
	return nil
}
