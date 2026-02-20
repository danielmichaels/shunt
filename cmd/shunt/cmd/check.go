package cmd

import (
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/tester"
)

type CheckCmd struct {
	Rule    string `required:"" help:"Path to rule file" type:"existingfile"`
	Message string `required:"" help:"Path to message file" type:"existingfile"`
	Subject string `help:"Override NATS subject from rule trigger"`
	KVMock  string `name:"kv-mock" help:"Path to mock KV data file"`
}

func (c *CheckCmd) Run(_ *Globals) error {
	log := logger.NewNopLogger()
	testRunner := tester.New(log, false, 0)
	return testRunner.QuickCheck(c.Rule, c.Message, c.Subject, c.KVMock)
}
