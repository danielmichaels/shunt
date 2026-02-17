package cmd

import "github.com/spf13/cobra"

func AddCommands(root *cobra.Command) {
	root.AddCommand(serveCmd)
	root.AddCommand(newCmd)
	root.AddCommand(lintCmd)
	root.AddCommand(testCmd)
	root.AddCommand(scaffoldCmd)
	root.AddCommand(checkCmd)
	root.AddCommand(kvCmd)
}
