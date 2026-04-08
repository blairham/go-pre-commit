// Package cli provides the command-line interface for pre-commit.
package cli

import (
	mcli "github.com/mitchellh/cli"
)

// Meta contains shared state for all commands.
type Meta struct {
	UI mcli.Ui
}

// GlobalFlags are flags available to all commands.
type GlobalFlags struct {
	Color  string `long:"color" default:"auto" description:"Whether to use color in output. Options: auto, always, never."`
	Config string `long:"config" short:"c" default:".pre-commit-config.yaml" description:"Path to alternate config file."`
}
