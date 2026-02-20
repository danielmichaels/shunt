package cmd

import (
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/tester"
)

type LintCmd struct {
	Rules string `short:"r" required:"" help:"Path to rules directory" type:"existingdir"`
}

func (l *LintCmd) Run(globals *Globals) error {
	log := logger.NewNopLogger()
	testRunner := tester.New(log, false, 0)
	return testRunner.Lint(l.Rules)
}
