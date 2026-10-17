// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/fatal"
)

// ValidateConfigCommand implements the "validate-config" command.
type ValidateConfigCommand struct {
	Meta *Meta
}

func (c *ValidateConfigCommand) Run(args []string) int {
	var opts ColorFlags
	remaining, code, ok := parseArgs("validate-config", &opts, args, args)
	if !ok {
		return code
	}

	return validateFiles(remaining, func(filename string) error {
		_, err := config.LoadConfig(filename)
		return err
	})
}

func (c *ValidateConfigCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit validate-config [options] [filenames...]

  Validate .pre-commit-config.yaml files.

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
	var opts ColorFlags
	remaining, code, ok := parseArgs("validate-manifest", &opts, args, args)
	if !ok {
		return code
	}

	return validateFiles(remaining, func(filename string) error {
		_, err := config.LoadManifest(filename)
		return err
	})
}

// validateFiles ports upstream's validate_config / validate_manifest: load
// each named file, print the InvalidConfigError (or InvalidManifestError) of
// any that fails to stdout, and print nothing on success. With no filenames
// upstream validates nothing (argparse nargs='*'), so neither do we. Any
// other error is uncaught upstream, and goes to the error handler.
func validateFiles(filenames []string, load func(string) error) int {
	out, ret, err := validateOutput(filenames, load)
	fmt.Print(out)
	if err != nil {
		return reportError(err)
	}
	return ret
}

// validateOutput returns what validateFiles prints and its exit code, or
// the first error that is not a validation failure.
func validateOutput(filenames []string, load func(string) error) (string, int, error) {
	var out strings.Builder
	ret := 0
	for _, filename := range filenames {
		err := load(filename)
		var fe *fatal.Error
		switch {
		case err == nil:
		case errors.As(err, &fe):
			out.WriteString(fe.Msg + "\n")
			ret = 1
		default:
			return out.String(), ret, err
		}
	}
	return out.String(), ret, nil
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
