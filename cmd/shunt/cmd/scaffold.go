// file: cmd/shunt/cmd/scaffold.go
package cmd

import (
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/tester"
	"github.com/spf13/cobra"
)

var scaffoldCmd = &cobra.Command{
	Use:   "scaffold <path-to-rule.yaml>",
	Short: "Generate a test directory for a given rule file",
	Long: `The scaffold command inspects a rule file and generates a corresponding '_test'
directory with boilerplate test files. It intelligently detects forEach operations
and creates array-based examples to accelerate test development.`,
	Args: cobra.ExactArgs(1), // Requires exactly one argument: the rule file path
	RunE: func(cmd *cobra.Command, args []string) error {
		rulePath := args[0]
		noOverwrite, _ := cmd.Flags().GetBool("no-overwrite")

		log := logger.NewNopLogger()
		testRunner := tester.New(log, false, 0)

		return testRunner.Scaffold(rulePath, noOverwrite)
	},
}

func init() {
	scaffoldCmd.Flags().Bool("no-overwrite", false, "Prevent scaffold from overwriting an existing test directory")
}
