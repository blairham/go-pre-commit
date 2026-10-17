// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"

	mcli "github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/v4/internal/config"
)

// BuildInfo holds build-time metadata injected via ldflags.
type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

// versionString returns the display version combining the Python-compatible
// version with the build metadata.
func versionString(b BuildInfo) string {
	v := "pre-commit " + config.Version
	if b.Version != "" && b.Version != "dev" {
		v += " (build " + b.Version
		if b.Commit != "" && b.Commit != "none" {
			short := b.Commit
			if len(short) > 8 {
				short = short[:8]
			}
			v += ", " + short
		}
		if b.Date != "" && b.Date != "unknown" {
			v += ", " + b.Date
		}
		v += ")"
	}
	return v
}

// Run creates the CLI application and executes the command specified by args.
//
// It plays the part of upstream's error_handler around the whole command: a
// panic is reported as an unexpected error (exit 3), and SIGINT cancels the
// command's context and, once the command has unwound, is reported as
// "Interrupted (^C)" (exit 130). A second SIGINT is not caught, and kills
// the process as it would have before.
func Run(args []string, build BuildInfo) (code int) {
	interrupted.Store(false)
	errorReported.Store(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	defer signal.Stop(sigs)
	go func() {
		select {
		case <-sigs:
			interrupted.Store(true)
			signal.Stop(sigs)
			cancel()
		case <-ctx.Done():
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			code = reportPanic(r, debug.Stack())
		}
	}()

	// Upstream: "argparse doesn't really provide a way to use a `default`
	// subparser" — a bare `pre-commit` is `pre-commit run`.
	if len(args) == 0 {
		args = []string{"run"}
	}
	// The top-level parser's own rejections: an option it does not know, and
	// a subcommand that is not one of its choices.
	if args[0] == "-V" {
		// argparse's short form; mitchellh/cli knows only the long one.
		args = append([]string{"--version"}, args[1:]...)
	}
	switch first := args[0]; {
	case first == "-h" || first == "--help" || first == "-V" || first == "--version" || first == "help":
	case len(first) > 1 && first[0] == '-':
		return argparseError(topLevelSpec, "unrecognized arguments: "+first)
	case !slices.Contains(commandNames, first):
		return argparseError(topLevelSpec, fmt.Sprintf("argument command: invalid choice: %s (choose from %s)",
			config.PyRepr(first), pyReprList(commandNames)))
	}
	ui := &mcli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	meta := &Meta{UI: ui, Ctx: ctx}

	c := &mcli.CLI{
		Name:    "pre-commit",
		Version: versionString(build),
		Args:    args,
		// Upstream (argparse) prints --version and --help to stdout and only
		// errors to stderr. mitchellh/cli defaults both to stderr, which left
		// `v=$(pre-commit --version)` empty.
		HelpWriter:  os.Stdout,
		ErrorWriter: os.Stderr,
		Commands: map[string]mcli.CommandFactory{
			"run":               func() (mcli.Command, error) { return &RunCommand{Meta: meta}, nil },
			"install":           func() (mcli.Command, error) { return &InstallCommand{Meta: meta}, nil },
			"uninstall":         func() (mcli.Command, error) { return &UninstallCommand{Meta: meta}, nil },
			"install-hooks":     func() (mcli.Command, error) { return &InstallHooksCommand{Meta: meta}, nil },
			"autoupdate":        func() (mcli.Command, error) { return &AutoupdateCommand{Meta: meta}, nil },
			"clean":             func() (mcli.Command, error) { return &CleanCommand{Meta: meta}, nil },
			"gc":                func() (mcli.Command, error) { return &GCCommand{Meta: meta}, nil },
			"init-templatedir":  func() (mcli.Command, error) { return &InitTemplateDirCommand{Meta: meta}, nil },
			"sample-config":     func() (mcli.Command, error) { return &SampleConfigCommand{Meta: meta}, nil },
			"try-repo":          func() (mcli.Command, error) { return &TryRepoCommand{Meta: meta}, nil },
			"validate-config":   func() (mcli.Command, error) { return &ValidateConfigCommand{Meta: meta}, nil },
			"validate-manifest": func() (mcli.Command, error) { return &ValidateManifestCommand{Meta: meta}, nil },
			"migrate-config":    func() (mcli.Command, error) { return &MigrateConfigCommand{Meta: meta}, nil },
			"hook-impl":         func() (mcli.Command, error) { return &HookImplCommand{Meta: meta}, nil },
			"hazmat cd": func() (mcli.Command, error) {
				return &HazmatCdCommand{Meta: meta}, nil
			},
			"hazmat ignore-exit-code": func() (mcli.Command, error) {
				return &HazmatIgnoreExitCodeCommand{Meta: meta}, nil
			},
			"hazmat n1": func() (mcli.Command, error) {
				return &HazmatN1Command{Meta: meta}, nil
			},
		},
		HiddenCommands: []string{
			"hook-impl",
			"hazmat cd",
			"hazmat ignore-exit-code",
			"hazmat n1",
		},
	}

	exitCode, err := c.Run()
	if interrupted.Load() && !errorReported.Load() {
		return reportError(keyboardInterrupt{})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return exitCode
}
