package commands

import (
	"errors"
	"fmt"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// ValidateConfigCommand handles the validate-config command functionality
type ValidateConfigCommand struct{}

// ValidateConfigOptions holds command-line options for the validate-config command
type ValidateConfigOptions struct {
	Help bool `short:"h" long:"help" description:"Show this help message"`
}

// Help returns the help text for the validate-config command
func (c *ValidateConfigCommand) Help() string {
	var opts ValidateConfigOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	formatter := &HelpFormatter{
		Command:     "validate-config",
		Description: "Validate the .pre-commit-config.yaml configuration file.",
		Examples: []Example{
			{Command: "pre-commit validate-config", Description: "Validate the configuration file"},
		},
		Notes: []string{
			"Checks the syntax and structure of your .pre-commit-config.yaml file.",
			"Returns exit code 0 if valid, non-zero if there are errors.",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the validate-config command
func (c *ValidateConfigCommand) Synopsis() string {
	return "Validate configuration file"
}

// Run executes the validate-config command
func (c *ValidateConfigCommand) Run(args []string) int {
	var opts ValidateConfigOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	_, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return 1
	}

	// Load configuration
	cfg, err := config.LoadConfig(".pre-commit-config.yaml")
	if err != nil {
		fmt.Printf("Error: failed to load configuration: %v\n", err)
		return 1
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Printf("Error: configuration is invalid: %v\n", err)
		return 1
	}

	fmt.Println("Configuration is valid")
	return 0
}

// ValidateConfigCommandFactory creates a new validate-config command instance
func ValidateConfigCommandFactory() (cli.Command, error) {
	return &ValidateConfigCommand{}, nil
}
