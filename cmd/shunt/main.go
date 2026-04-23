package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/alecthomas/kong"
	"github.com/danielmichaels/shunt/cmd/shunt/cmd"
)

var version = "dev"

func resolveVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var revision, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if revision == "" {
		return version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	v := "dev-" + revision
	if modified == "true" {
		v += "+dirty"
	}
	return v
}

type CLI struct {
	cmd.Globals

	Serve    cmd.ServeCmd    `cmd:"" help:"Start the shunt message routing server"`
	Dev      cmd.DevCmd      `cmd:"" help:"Start shunt with an embedded NATS server (no external deps required)"`
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
	v := resolveVersion()
	ctx := kong.Parse(&cli,
		kong.Name("shunt"),
		kong.Description("A rule-based message router for NATS JetStream"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.DefaultEnvars("SHUNT"),
		kong.Vars{"version": v},
	)
	cli.Globals.Version = v
	return ctx.Run(&cli.Globals)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
