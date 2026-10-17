// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package cli provides the command-line interface for pre-commit.
package cli

import (
	"context"

	mcli "github.com/mitchellh/cli"
)

// Meta contains shared state for all commands.
type Meta struct {
	UI mcli.Ui
	// Ctx is canceled when the user interrupts (SIGINT), so a command
	// stops starting work and unwinds -- restoring a stash on the way, as
	// upstream's context managers do for KeyboardInterrupt.
	Ctx context.Context
}

// ctx is m.Ctx, or a context that is never canceled when there is none (a
// command constructed directly, as tests do).
func (m *Meta) ctx() context.Context {
	if m == nil || m.Ctx == nil {
		return context.Background()
	}
	return m.Ctx
}

// GlobalFlags are flags available to all commands.
// ColorFlags is the one option every subcommand has: upstream's commands
// without -c (clean, gc, sample-config, validate-*) take only --color.
type ColorFlags struct {
	Color string `long:"color" default:"auto" description:"Whether to use color in output. Options: auto, always, never."`
}

type GlobalFlags struct {
	Color  string `long:"color"  default:"auto"                    description:"Whether to use color in output. Options: auto, always, never."`
	Config string `long:"config" default:".pre-commit-config.yaml" description:"Path to alternate config file."                                short:"c"`
}
