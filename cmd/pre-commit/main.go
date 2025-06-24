// Package main provides the pre-commit command-line tool.
// This is a Go implementation of pre-commit hooks for Git repositories.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/internal/commands"
)

// Version information set by GoReleaser
var (
	version = "dev"
	commit  = "none"    //nolint:unused // Set by GoReleaser
	date    = "unknown" //nolint:unused // Set by GoReleaser
	builtBy = "unknown" //nolint:unused // Set by GoReleaser
)

func main() {
	c := cli.NewCLI("pre-commit", version)
	c.Args = os.Args[1:]
	c.HelpFunc = customHelpFunc
	c.Commands = map[string]cli.CommandFactory{
		"autoupdate":        commands.AutoupdateCommandFactory,
		"clean":             commands.CleanCommandFactory,
		"doctor":            commands.DoctorCommandFactory,
		"gc":                commands.GcCommandFactory,
		"install":           commands.InstallCommandFactory,
		"install-hooks":     commands.InstallHooksCommandFactory,
		"migrate-config":    commands.MigrateConfigCommandFactory,
		"run":               commands.RunCommandFactory,
		"sample-config":     commands.SampleConfigCommandFactory,
		"try-repo":          commands.TryRepoCommandFactory,
		"uninstall":         commands.UninstallCommandFactory,
		"validate-config":   commands.ValidateConfigCommandFactory,
		"validate-manifest": commands.ValidateManifestCommandFactory,
		"help":              commands.HelpCommandFactory,
		"hook-impl":         commands.HookImplCommandFactory,
		"init-templatedir":  commands.InitTemplatedirCommandFactory,
	}

	exitStatus, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	os.Exit(exitStatus)
}

// customHelpFunc provides Python pre-commit style help output
func customHelpFunc(cmdFactories map[string]cli.CommandFactory) string {
	// Build the command list in alphabetical order (like Python version)
	var commandNames []string
	for name := range cmdFactories {
		// Skip internal commands from main help
		if name != "hook-impl" && name != "help" {
			commandNames = append(commandNames, name)
		}
	}

	// Sort commands alphabetically
	sort.Strings(commandNames)

	// Build the usage line with all commands
	usageLine := "usage: pre-commit [-h] [--version]\n"
	usageLine += "                  {"
	usageLine += strings.Join(commandNames, ",")
	usageLine += "}\n                  ...\n"

	helpText := usageLine + `
A framework for managing and maintaining multi-language pre-commit hooks.

positional arguments:
  {` + strings.Join(commandNames, ",") + `}
    autoupdate          Auto-update pre-commit config to the latest repos' versions
    clean               Clean cached repositories and environments
    doctor              Check and repair environment health (Go extension)
    gc                  Clean unused cached repos
    init-templatedir    Install hook script in a directory intended for use with git init templateDir (Go extension)
    install             Install the pre-commit script
    install-hooks       Install hook environments for all environments in the config file
    migrate-config      Migrate list configuration to new map configuration
    run                 Run hooks
    sample-config       Produce a sample .pre-commit-config.yaml file
    try-repo            Try the hooks in a repository, useful for developing new hooks
    uninstall           Uninstall the pre-commit script
    validate-config     Validate .pre-commit-config.yaml files
    validate-manifest   Validate .pre-commit-hooks.yaml files

optional arguments:
  -h, --help            show this help message and exit
  --version             show program's version number and exit
`

	return helpText
}
