package main

import (
	"os"

	"github.com/danielmichaels/shunt/cmd/shunt/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "shunt",
	Short: "A rule-based message router for NATS JetStream",
	Long: `Shunt is a high-performance, rule-based message router for NATS JetStream
with an integrated HTTP gateway and automated token management.

Rules are stored in NATS KV and hot-reloaded via KV Watch — no restarts required.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func init() {
	cmd.AddCommands(rootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
