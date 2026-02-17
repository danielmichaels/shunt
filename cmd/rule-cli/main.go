// file: cmd/rule-cli/main.go
package main

import (
	"os"

	"github.com/danielmichaels/rule-engine/cmd/rule-cli/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rule-cli",
	Short: "CLI for creating, testing, and managing rules for rule-router",
	Long: `rule-cli is a command-line tool for building, validating, testing, and managing
rules. It supports offline rule evaluation and NATS KV bucket management for
pushing, pulling, listing, and deleting rules in a live cluster.`,
	// If a subcommand is not provided, default to showing help.
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	// Add all subcommands from the cmd package
	cmd.AddCommands(rootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra prints the error, so we just need to exit
		os.Exit(1)
	}
}
