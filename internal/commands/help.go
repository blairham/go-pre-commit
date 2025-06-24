package commands

import (
	"errors"
	"fmt"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

// HelpCommand handles the help command functionality
type HelpCommand struct {
	UI cli.Ui // User interface for command output
}

// HelpOptions holds command-line options for the help command
type HelpOptions struct {
	Help bool `short:"h" long:"help" description:"Show this help message"`
}

// Help returns the help text for the help command
func (c *HelpCommand) Help() string {
	helpText := `
Show help for a specific command.

Usage: pre-commit help [COMMAND]

If COMMAND is specified, shows detailed help for that command.
If no command is specified, shows general help.

Available commands:
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

Note: (Go extension) indicates commands added in the Go implementation that are not
available in the original Python version.

`
	return helpText
}

// Synopsis returns a short description of the help command
func (c *HelpCommand) Synopsis() string {
	return "Show help for a specific command"
}

// Run executes the help command
func (c *HelpCommand) Run(args []string) int {
	var opts HelpOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[COMMAND]"

	remaining, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return 1
	}

	if len(remaining) == 0 {
		// Show general help
		fmt.Print(c.Help())
		return 0
	}

	command := remaining[0]

	// Map of command descriptions
	commandHelp := map[string]string{
		"install":           "Install git hooks for pre-commit. Run this once per repository to set up the hooks.",
		"run":               "Run the configured hooks against staged files (or all files with --all-files).",
		"uninstall":         "Remove pre-commit hooks from the repository.",
		"clean":             "Clean out cached repositories and hook environments.",
		"gc":                "Garbage collect unused cached repositories (more conservative than clean).",
		"autoupdate":        "Automatically update hook repository versions in your config file.",
		"install-hooks":     "Pre-install hook environments without installing git hooks (useful for CI).",
		"try-repo":          "Test hooks from a repository without installing them in your project.",
		"init-templatedir":  "Set up hooks in a git template directory for automatic installation in new repos.",
		"migrate-config":    "Update old-format configuration files to the new format.",
		"validate-config":   "Check that your .pre-commit-config.yaml file is valid.",
		"validate-manifest": "Check that .pre-commit-hooks.yaml files are valid.",
		"sample-config":     "Generate an example .pre-commit-config.yaml file.",
		"help":              "Show help information for commands.",
	}

	if help, exists := commandHelp[command]; exists {
		fmt.Printf("Command: %s\n\n", command)
		fmt.Printf("Description: %s\n\n", help)
		fmt.Printf("For detailed usage information, run:\n")
		fmt.Printf("  pre-commit %s --help\n", command)
	} else {
		fmt.Printf("Unknown command: %s\n\n", command)
		fmt.Println("Available commands:")
		for cmd := range commandHelp {
			fmt.Printf("  %s\n", cmd)
		}
		return 1
	}

	return 0
}

// HelpCommandFactory creates a new help command instance
func HelpCommandFactory() (cli.Command, error) {
	return &HelpCommand{}, nil
}
