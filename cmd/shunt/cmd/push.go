package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push <file|dir>",
	Short: "Validate and push rules into a NATS KV bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		nc, err := connectNATS(cmd)
		if err != nil {
			return err
		}
		defer nc.Close()

		kv, err := openKVBucket(cmd, nc)
		if err != nil {
			return err
		}

		log := logger.NewNopLogger()
		loader := rule.NewRulesLoader(log, nil)

		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("cannot access %s: %w", path, err)
		}

		var files []string
		if info.IsDir() {
			entries, err := filepath.Glob(filepath.Join(path, "*.yaml"))
			if err != nil {
				return fmt.Errorf("failed to glob directory: %w", err)
			}
			ymlEntries, err := filepath.Glob(filepath.Join(path, "*.yml"))
			if err != nil {
				return fmt.Errorf("failed to glob directory: %w", err)
			}
			files = append(entries, ymlEntries...)
		} else {
			files = []string{path}
		}

		if len(files) == 0 {
			return fmt.Errorf("no YAML files found in %s", path)
		}

		ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
		defer cancel()

		pushed := 0
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", f, err)
			}

			rules, err := rule.ParseYAML(data)
			if err != nil {
				return fmt.Errorf("invalid YAML in %s: %w", f, err)
			}

			for i := range rules {
				if err := loader.ValidateRule(&rules[i]); err != nil {
					return fmt.Errorf("validation failed for rule %d in %s: %w", i, f, err)
				}
			}

			key := sanitizeKVKey(filepath.Base(f))
			if _, err := kv.Put(ctx, key, data); err != nil {
				return fmt.Errorf("failed to put key '%s': %w", key, err)
			}

			fmt.Printf("  pushed %s → %s (%d rules)\n", filepath.Base(f), key, len(rules))
			pushed++
		}

		fmt.Printf("\n%d files pushed successfully\n", pushed)
		return nil
	},
}
