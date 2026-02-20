package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielmichaels/shunt/internal/cli"
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/danielmichaels/shunt/internal/tester"
)

type NewCmd struct {
	Template    string `short:"t" help:"Named template (e.g., nats-basic)"`
	Output      string `short:"o" help:"Output file path"`
	Interactive bool   `short:"i" help:"Start interactive rule builder"`
	List        bool   `help:"List available templates"`
	Show        string `help:"Show template content"`
	WithTests   bool   `name:"with-tests" help:"Scaffold tests after creation"`
}

func (n *NewCmd) Run(globals *Globals) error {
	renderer := cli.NewRenderer()
	prompter := cli.NewPrompter()

	if n.List {
		return listTemplates(renderer)
	}

	if n.Show != "" {
		return showTemplateContent(renderer, n.Show)
	}

	var ruleBytes []byte
	var err error

	if n.Interactive || n.Template == "" {
		builder := cli.NewRuleBuilder(prompter)
		ruleBytes, err = builder.BuildRule()
		if err != nil {
			return fmt.Errorf("interactive build failed: %w", err)
		}
	} else {
		content, err := renderer.GetTemplateContent(n.Template)
		if err != nil {
			return err
		}
		ruleBytes = []byte(content)
	}

	output := n.Output
	if output == "" {
		output, err = prompter.AskWithDefault("Enter filename for the new rule:", "new-rule.yaml")
		if err != nil {
			return err
		}
	}
	output = normalizeOutputPath(output)

	if err := writeFileWithConfirm(output, ruleBytes); err != nil {
		if err.Error() == "cancelled" {
			return nil
		}
		return err
	}
	fmt.Printf("%s✓ Success! Rule file '%s' created.%s\n", cli.ColorGreen, output, cli.ColorReset)

	if err := validateRuleFile(output); err != nil {
		fmt.Printf("%sWarning: The generated rule has a validation issue: %v%s\n", cli.ColorYellow, err, cli.ColorReset)
	}

	scaffoldTests := n.WithTests
	if !scaffoldTests {
		scaffoldTests, _ = prompter.Confirm("Generate test scaffold for this rule?")
	}

	if scaffoldTests {
		testRunner := tester.New(logger.NewNopLogger(), false, 0)
		if err := testRunner.Scaffold(output, false); err != nil {
			return fmt.Errorf("failed to scaffold tests: %w", err)
		}
	}

	return nil
}

func listTemplates(r *cli.Renderer) error {
	templates, err := r.ListTemplates()
	if err != nil {
		return err
	}
	fmt.Println("Available templates:")
	for _, t := range templates {
		fmt.Printf("  - %s\n", t)
	}
	return nil
}

func showTemplateContent(r *cli.Renderer, templateName string) error {
	content, err := r.GetTemplateContent(templateName)
	if err != nil {
		return err
	}
	fmt.Printf("--- Template: %s ---\n", templateName)
	fmt.Println(content)
	return nil
}

func normalizeOutputPath(path string) string {
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		path += ".yaml"
	}
	if filepath.Dir(path) == "." {
		if _, err := os.Stat("rules"); !os.IsNotExist(err) {
			path = filepath.Join("rules", path)
		}
	}
	return path
}

func writeFileWithConfirm(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("File '%s' already exists. Overwrite? (y/N): ", path)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Cancelled.")
			return fmt.Errorf("cancelled")
		}
	}
	return os.WriteFile(path, data, 0644)
}

func validateRuleFile(path string) error {
	loader := rule.NewRulesLoader(logger.NewNopLogger(), nil)
	_, err := loader.LoadFromFile(path)
	return err
}
