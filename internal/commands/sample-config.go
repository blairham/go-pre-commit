package commands

import (
	"fmt"

	"github.com/mitchellh/cli"
)

// SampleConfigCommand handles the sample-config command functionality
type SampleConfigCommand struct{}

// SAMPLE_CONFIG matches Python's pre-commit exactly
const SAMPLE_CONFIG = `# See https://pre-commit.com for more information
# See https://pre-commit.com/hooks.html for more hooks
repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v3.2.0
    hooks:
    -   id: trailing-whitespace
    -   id: end-of-file-fixer
    -   id: check-yaml
    -   id: check-added-large-files
`

// Help returns the help text for the sample-config command
func (c *SampleConfigCommand) Help() string {
	return `Usage: pre-commit sample-config

  Produce a sample .pre-commit-config.yaml file.

  This command prints a sample configuration to stdout.
  Redirect to a file to create your config:

    pre-commit sample-config > .pre-commit-config.yaml
`
}

// Synopsis returns a short description of the sample-config command
func (c *SampleConfigCommand) Synopsis() string {
	return "Produce a sample .pre-commit-config.yaml file"
}

// Run executes the sample-config command
func (c *SampleConfigCommand) Run(args []string) int {
	// Match Python exactly: print to stdout and return 0
	fmt.Print(SAMPLE_CONFIG)
	return 0
}

// SampleConfigCommandFactory creates a new sample-config command instance
func SampleConfigCommandFactory() (cli.Command, error) {
	return &SampleConfigCommand{}, nil
}
