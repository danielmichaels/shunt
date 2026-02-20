package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type KVPullCmd struct {
	Key    string `arg:"" optional:"" help:"Key to pull (omit for all)"`
	Output string `short:"o" default:"." help:"Output directory for downloaded rule files"`
	Format string `short:"f" default:"" help:"Output format to stdout instead of files (yaml or json)"`
}

func (p *KVPullCmd) Run(kv *KVCmd) error {
	format := p.Format
	if format != "" {
		format = strings.ToLower(format)
		if format != "yaml" && format != "json" {
			return fmt.Errorf("unsupported format %q (use yaml or json)", format)
		}
	}

	nc, bucket, err := kv.connectToNATS()
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), kvOperationTimeout)
	defer cancel()

	if format == "" {
		if err := os.MkdirAll(p.Output, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	if p.Key != "" {
		entry, err := bucket.Get(ctx, p.Key)
		if err != nil {
			return fmt.Errorf("failed to get key '%s': %w", p.Key, err)
		}
		if format != "" {
			return printRule(entry.Value(), format)
		}
		filename := p.Key + ".yaml"
		outPath := filepath.Join(p.Output, filename)
		if err := os.WriteFile(outPath, entry.Value(), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outPath, err)
		}
		fmt.Fprintf(os.Stderr, "  pulled %s → %s\n", p.Key, outPath)
		return nil
	}

	keys, err := bucket.Keys(ctx)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	pulled := 0
	for _, key := range keys {
		entry, err := bucket.Get(ctx, key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to get key '%s': %v\n", key, err)
			continue
		}
		if format != "" {
			if pulled > 0 {
				fmt.Println()
			}
			if err := printRule(entry.Value(), format); err != nil {
				return err
			}
			pulled++
			continue
		}
		filename := key + ".yaml"
		outPath := filepath.Join(p.Output, filename)
		if err := os.WriteFile(outPath, entry.Value(), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", outPath, err)
		}
		fmt.Fprintf(os.Stderr, "  pulled %s → %s\n", key, outPath)
		pulled++
	}

	if format == "" {
		fmt.Fprintf(os.Stderr, "\n%d rules pulled to %s\n", pulled, p.Output)
	}
	return nil
}

func printRule(data []byte, format string) error {
	if format == FormatJSON {
		var raw any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
		raw = inlineJSONStrings(raw)
		return printJSON(os.Stdout, raw)
	}
	fmt.Println(string(data))
	return nil
}

var bareTemplateVarRe = regexp.MustCompile(`:\s*(\{[\w.@()]+\})\s*([,}\n])`)

func inlineJSONStrings(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			val[k] = inlineJSONStrings(child)
		}
		return val
	case []any:
		for i, child := range val {
			val[i] = inlineJSONStrings(child)
		}
		return val
	case string:
		s := strings.TrimSpace(val)
		if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
			(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
			var parsed any
			if err := json.Unmarshal([]byte(s), &parsed); err == nil {
				return parsed
			}
			quoted := bareTemplateVarRe.ReplaceAllString(s, `: "$1"$2`)
			if err := json.Unmarshal([]byte(quoted), &parsed); err == nil {
				return parsed
			}
		}
		return val
	default:
		return val
	}
}
