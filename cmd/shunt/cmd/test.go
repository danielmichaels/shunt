package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/tester"
)

type TestCmd struct {
	Rules    string `short:"r" required:"" help:"Path to rules directory" type:"existingdir"`
	Output   string `short:"o" default:"pretty" help:"Output format" enum:"pretty,json"`
	Verbose  bool   `short:"V" help:"Show detailed output for failures"`
	Parallel int    `short:"p" default:"4" help:"Parallel test workers (0 = sequential)"`
}

func (t *TestCmd) Run(globals *Globals) error {
	if t.Output == "pretty" {
		fmt.Printf("▶ RUNNING TESTS in %s\n\n", t.Rules)
	}

	log := logger.NewNopLogger()
	testRunner := tester.New(log, t.Verbose, t.Parallel)
	summary, err := testRunner.RunBatchTest(t.Rules)

	if t.Output == "json" {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encodeErr := encoder.Encode(summary); encodeErr != nil {
			return encodeErr
		}
	} else {
		printSummaryPretty(summary)
	}

	if err != nil {
		return err
	}

	if summary.Failed > 0 {
		return fmt.Errorf("tests failed")
	}
	return nil
}

func printSummaryPretty(summary tester.TestSummary) {
	fmt.Println("--- SUMMARY ---")
	fmt.Printf("Total Tests: %d, Passed: %d, Failed: %d\n",
		summary.Total, summary.Passed, summary.Failed)
	fmt.Printf("Duration: %dms\n", summary.DurationMs)

	if summary.Failed > 0 {
		fmt.Println("\n--- FAILURES ---")
		for _, result := range summary.Results {
			if !result.Passed {
				fmt.Printf("✖ %s\n", result.File)
				fmt.Printf("  Error: %s\n", result.Error)
				if result.Details != "" {
					fmt.Printf("  Details: %s\n", strings.ReplaceAll(result.Details, "\n", "\n  "))
				}
			}
		}
	}
}
