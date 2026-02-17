package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:   "pull [key]",
	Short: "Download rules from a NATS KV bucket to local YAML files",
	Long: `Pull downloads rules from the KV bucket. If a key is specified, only that rule is
downloaded. Otherwise all rules in the bucket are downloaded.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outDir, _ := cmd.Flags().GetString("output")

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

		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		if len(args) == 1 {
			key := args[0]
			entry, err := kv.Get(ctx, key)
			if err != nil {
				return fmt.Errorf("failed to get key '%s': %w", key, err)
			}
			filename := key + ".yaml"
			outPath := filepath.Join(outDir, filename)
			if err := os.WriteFile(outPath, entry.Value(), 0o644); err != nil {
				return fmt.Errorf("failed to write %s: %w", outPath, err)
			}
			fmt.Printf("  pulled %s → %s\n", key, outPath)
			return nil
		}

		keys, err := kv.Keys(ctx)
		if err != nil {
			return fmt.Errorf("failed to list keys: %w", err)
		}

		pulled := 0
		for _, key := range keys {
			entry, err := kv.Get(ctx, key)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  warning: failed to get key '%s': %v\n", key, err)
				continue
			}
			filename := key + ".yaml"
			outPath := filepath.Join(outDir, filename)
			if err := os.WriteFile(outPath, entry.Value(), 0o644); err != nil {
				return fmt.Errorf("failed to write %s: %w", outPath, err)
			}
			fmt.Printf("  pulled %s → %s\n", key, outPath)
			pulled++
		}

		fmt.Printf("\n%d rules pulled to %s\n", pulled, outDir)
		return nil
	},
}

func init() {
	pullCmd.Flags().StringP("output", "o", ".", "Output directory for downloaded rule files")
}
