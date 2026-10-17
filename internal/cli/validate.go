// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/v4/internal/config"
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
	var opts GlobalFlags
	remaining, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return validateFiles(remaining, func(filename string) error {
		_, err := config.LoadManifest(filename)
		return err
	})
}

// validateFiles ports upstream's validate_config / validate_manifest: load
// each named file, print the error of any that fails to stdout, and print
// nothing on success. With no filenames upstream validates nothing (argparse
// nargs='*'), so neither do we.
func validateFiles(filenames []string, load func(string) error) int {
	out, ret := validateOutput(filenames, load)
	fmt.Print(out)
	return ret
}

// validateOutput returns what validateFiles prints, and its exit code.
func validateOutput(filenames []string, load func(string) error) (string, int) {
	var out strings.Builder
	ret := 0
	for _, filename := range filenames {
		if err := load(filename); err != nil {
			out.WriteString(cfgvError(filename, err) + "\n")
			ret = 1
		}
	}
	return out.String(), ret
}

// cfgvError renders a load failure the way cfgv's ValidationError.__str__
// does for load_from_filename: a leading blank line, a "==> File" context for
// a file that exists, and the message after "=====> ". A path that is not a
// regular file fails before the File context is entered.
//
// The message itself is ours, not cfgv's: upstream's nested "==> At ..."
// contexts and its schema wording are not ported here.
func cfgvError(filename string, err error) string {
	if fi, statErr := os.Stat(filename); statErr != nil || !fi.Mode().IsRegular() {
		return fmt.Sprintf("\n=====> %s is not a file", filename)
	}
	if inner := errors.Unwrap(err); inner != nil {
		err = inner
	}
	return fmt.Sprintf("\n==> File %s\n=====> %v", filename, err)
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
