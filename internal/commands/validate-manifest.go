package commands

import (
	"fmt"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/mitchellh/cli"
)

// ValidateManifestCommand handles the validate-manifest command functionality
type ValidateManifestCommand struct{}

// Help returns the help text for the validate-manifest command
func (c *ValidateManifestCommand) Help() string {
	return `usage: pre-commit validate-manifest [filenames...]

Validate .pre-commit-hooks.yaml files

positional arguments:
  filenames    Manifest files to validate
`
}

// Synopsis returns a short description of the validate-manifest command
func (c *ValidateManifestCommand) Synopsis() string {
	return "Validate .pre-commit-hooks.yaml files"
}

// Run executes the validate-manifest command
// Matches Python: loops through files, loads each, prints errors, returns 0/1
func (c *ValidateManifestCommand) Run(args []string) int {
	filenames := args

	ret := 0
	for _, filename := range filenames {
		_, err := config.LoadHooksConfig(filename)
		if err != nil {
			fmt.Println(err)
			ret = 1
		}
		// Silent on success (matches Python behavior)
	}

	return ret
}

// ValidateManifestCommandFactory creates a new validate-manifest command instance
func ValidateManifestCommandFactory() (cli.Command, error) {
	return &ValidateManifestCommand{}, nil
}
