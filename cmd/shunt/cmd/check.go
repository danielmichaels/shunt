package cmd

import (
	"fmt"
	"strings"

	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/tester"
)

type CheckCmd struct {
	Rule    string   `required:"" help:"Path to rule file" type:"existingfile"`
	Message string   `required:"" help:"Path to message file" type:"existingfile"`
	Subject string   `help:"Override NATS subject from rule trigger"`
	KVMock  string   `name:"kv-mock" help:"Path to mock KV data file"`
	Header  []string `name:"header" sep:"none" help:"HTTP/NATS header to inject (Key: Value), repeatable"`
}

func (c *CheckCmd) Run(_ *Globals) error {
	headers, err := parseHeaders(c.Header)
	if err != nil {
		return err
	}
	log := logger.NewNopLogger()
	testRunner := tester.New(log, false, 0)
	return testRunner.QuickCheck(tester.QuickCheckOptions{
		RulePath:    c.Rule,
		MessagePath: c.Message,
		Subject:     c.Subject,
		KVMockPath:  c.KVMock,
		Headers:     headers,
	})
}

func parseHeaders(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(raw))
	for _, h := range raw {
		rawKey, rawValue, ok := strings.Cut(h, ":")
		if !ok {
			return nil, fmt.Errorf("invalid --header %q: expected \"Key: Value\"", h)
		}
		key := strings.TrimSpace(rawKey)
		if key == "" {
			return nil, fmt.Errorf("invalid --header %q: expected \"Key: Value\"", h)
		}
		out[key] = strings.TrimSpace(rawValue)
	}
	return out, nil
}
