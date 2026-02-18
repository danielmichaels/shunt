package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
)

var validFormats = []string{FormatTable, FormatJSON, FormatYAML}

func validateFormat(format string) (string, error) {
	f := strings.ToLower(format)
	for _, v := range validFormats {
		if f == v {
			return f, nil
		}
	}
	return "", fmt.Errorf("unsupported format %q (use table, json, or yaml)", format)
}

func printTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

func printJSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func printYAML(w io.Writer, data any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(data)
}

func formatOutput(format string, data any, headers []string, toRows func() [][]string) error {
	switch format {
	case FormatTable:
		printTable(os.Stdout, headers, toRows())
	case FormatJSON:
		if err := printJSON(os.Stdout, data); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
	case FormatYAML:
		if err := printYAML(os.Stdout, data); err != nil {
			return fmt.Errorf("failed to encode YAML: %w", err)
		}
	}
	return nil
}
