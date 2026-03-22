package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/danielmichaels/shunt/cmd/shunt/cmd"
)

var version = "dev"

type CLI struct {
	cmd.Globals

	Serve    cmd.ServeCmd    `cmd:"" help:"Start the shunt message routing server"`
	New      cmd.NewCmd      `cmd:"" help:"Create a new rule from a template or interactively"`
	Lint     cmd.LintCmd     `cmd:"" help:"Validate rule file syntax and structure"`
	Test     cmd.TestCmd     `cmd:"" help:"Run all test suites for rules"`
	Check    cmd.CheckCmd    `cmd:"" help:"Quick check of a single rule against a message"`
	Scaffold cmd.ScaffoldCmd `cmd:"" help:"Generate a test directory for a rule file"`
	KV       cmd.KVCmd       `cmd:"" help:"Manage rules in a NATS KV bucket"`

	Version kong.VersionFlag `help:"Print version information" short:"v"`
}

func run() error {
	cli := CLI{}
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "--help")
	}
	ctx := kong.Parse(&cli,
		kong.Name("shunt"),
		kong.Description("A rule-based message router for NATS JetStream"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.DefaultEnvars("SHUNT"),
		kong.Vars{"version": version},
	)
	cli.Globals.Version = version
	return ctx.Run(&cli.Globals)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
