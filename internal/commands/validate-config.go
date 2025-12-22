package commands

import (
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// ValidateConfigCommand handles the validate-config command functionality
type ValidateConfigCommand struct{}

// Help returns the help text for the validate-config command
func (c *ValidateConfigCommand) Help() string {
	return `Usage: pre-commit validate-config [filenames...]

Validate .pre-commit-config.yaml files

Arguments:
  filenames    One or more config files to validate

Example:
  pre-commit validate-config .pre-commit-config.yaml
  pre-commit validate-config file1.yaml file2.yaml
`
}

// Synopsis returns a short description of the validate-config command
func (c *ValidateConfigCommand) Synopsis() string {
	return "Validate .pre-commit-config.yaml files"
}

// Run executes the validate-config command
// This matches Python's validate_config behavior:
// - Takes filenames as positional arguments
// - Validates all files, continuing even if some fail
// - Silent on success
// - Returns 0 if all valid, 1 if any invalid
func (c *ValidateConfigCommand) Run(args []string) int {
	// Filenames are positional arguments (matching Python)
	filenames := args

	ret := 0
	for _, filename := range filenames {
		// Load and validate configuration
		cfg, err := config.LoadConfig(filename)
		if err != nil {
			// Print error and continue (matching Python behavior)
			fmt.Println(err)
			ret = 1
			continue
		}

		// Validate configuration structure
		if err := cfg.Validate(); err != nil {
			fmt.Println(err)
			ret = 1
		}
		// Silent on success (matching Python)
	}

	return ret
}

// ValidateConfigCommandFactory creates a new validate-config command instance
func ValidateConfigCommandFactory() (cli.Command, error) {
	return &ValidateConfigCommand{}, nil
}
