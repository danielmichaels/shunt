package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielmichaels/shunt/internal/broker"
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/nats-io/nats.go/jetstream"
)

type KVPushCmd struct {
	Path string `arg:"" help:"File or directory to push" type:"existingpath"`
}

func (p *KVPushCmd) Run(kv *KVCmd, globals *Globals) error {
	nc, bucket, err := kv.connectToNATS()
	if err != nil {
		return err
	}
	defer nc.Close()

	log := logger.NewNopLogger()
	loader := rule.NewRulesLoader(log, nil)

	info, err := os.Stat(p.Path)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", p.Path, err)
	}

	var files []string
	if info.IsDir() {
		entries, err := filepath.Glob(filepath.Join(p.Path, "*.yaml"))
		if err != nil {
			return fmt.Errorf("failed to glob directory: %w", err)
		}
		ymlEntries, err := filepath.Glob(filepath.Join(p.Path, "*.yml"))
		if err != nil {
			return fmt.Errorf("failed to glob directory: %w", err)
		}
		files = append(entries, ymlEntries...)
	} else {
		files = []string{p.Path}
	}

	if len(files) == 0 {
		return fmt.Errorf("no YAML files found in %s", p.Path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
	defer cancel()

	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("failed to create JetStream context: %w", err)
	}

	resolver := broker.NewStreamResolver(js, log)
	if err := resolver.Discover(ctx); err != nil {
		return fmt.Errorf("failed to discover streams: %w", err)
	}

	if globals.Debug {
		streams := resolver.GetStreams()
		fmt.Fprintf(os.Stderr, "discovered %d streams:\n", len(streams))
		for _, s := range streams {
			fmt.Fprintf(os.Stderr, "  %s subjects=%v\n", s.Name, s.Subjects)
		}
	}

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

		if streamErrs := resolver.ValidateRulesHaveStreams(rules); len(streamErrs) > 0 {
			msgs := make([]string, len(streamErrs))
			for i, e := range streamErrs {
				msgs[i] = e.Error()
			}
			return fmt.Errorf("stream validation failed for %s:\n  %s", f, strings.Join(msgs, "\n  "))
		}

		key := deriveKVKey(f, kv.Bucket)
		if _, err := bucket.Put(ctx, key, data); err != nil {
			return fmt.Errorf("failed to put key '%s': %w", key, err)
		}

		fmt.Fprintf(os.Stderr, "pushed %s → %s (%d rules)\n", f, key, len(rules))
		pushed++
	}

	fmt.Fprintf(os.Stderr, "\n%d files pushed successfully\n", pushed)
	return nil
}
