package cmd

import (
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/tester"
)

type ScaffoldCmd struct {
	Path        string `arg:"" help:"Path to rule file" type:"existingfile"`
	NoOverwrite bool   `name:"no-overwrite" help:"Don't overwrite existing test directory"`
}

func (s *ScaffoldCmd) Run(globals *Globals) error {
	log := logger.NewNopLogger()
	testRunner := tester.New(log, false, 0)
	return testRunner.Scaffold(s.Path, s.NoOverwrite)
}
