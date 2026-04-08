package cli

import (
	"fmt"
	"os"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/internal/config"
)

// ValidateConfigCommand implements the "validate-config" command.
type ValidateConfigCommand struct {
	Meta *Meta
}

func (c *ValidateConfigCommand) Run(args []string) int {
	var opts GlobalFlags
	remaining, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	filenames := remaining
	if len(filenames) == 0 {
		filenames = []string{opts.Config}
	}

	allValid := true
	for _, filename := range filenames {
		cfg, err := config.LoadConfig(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", filename, err)
			allValid = false
			continue
		}
		if err := cfg.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", filename, err)
			allValid = false
		}
	}

	if !allValid {
		return 1
	}

	fmt.Println("Config file(s) are valid.")
	return 0
}

func (c *ValidateConfigCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit validate-config [options] [filenames...]

  Validate .pre-commit-config.yaml files. If no filenames are given,
  validates the default config.

Options:

  -c, --config=FILE   Path to alternate config file.
      --color=MODE    Whether to use color (auto, always, never).
`)
}

func (c *ValidateConfigCommand) Synopsis() string {
	return "Validate .pre-commit-config.yaml files"
}

// ValidateManifestCommand implements the "validate-manifest" command.
type ValidateManifestCommand struct {
	Meta *Meta
}

func (c *ValidateManifestCommand) Run(args []string) int {
	var opts GlobalFlags
	remaining, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	filenames := remaining
	if len(filenames) == 0 {
		filenames = []string{config.ManifestFile}
	}

	allValid := true
	for _, filename := range filenames {
		_, err := config.LoadManifest(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", filename, err)
			allValid = false
		}
	}

	if !allValid {
		return 1
	}

	fmt.Println("Manifest file(s) are valid.")
	return 0
}

func (c *ValidateManifestCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit validate-manifest [options] [filenames...]

  Validate .pre-commit-hooks.yaml manifest files.

Options:

  -c, --config=FILE   Path to alternate config file.
      --color=MODE    Whether to use color (auto, always, never).
`)
}

func (c *ValidateManifestCommand) Synopsis() string {
	return "Validate .pre-commit-hooks.yaml files"
}
