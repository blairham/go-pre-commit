package commands

import (
	"errors"
	"fmt"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/git"
)

// UninstallCommand handles the uninstall command functionality
type UninstallCommand struct{}

// UninstallOptions holds command-line options for the uninstall command
type UninstallOptions struct {
	Help bool `short:"h" long:"help" description:"Show this help message"`
}

// Help returns the help text for the uninstall command
func (c *UninstallCommand) Help() string {
	var opts UninstallOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	formatter := &HelpFormatter{
		Command:     "uninstall",
		Description: "Uninstall pre-commit hooks from the git repository.",
		Examples: []Example{
			{Command: "pre-commit uninstall", Description: "Remove all pre-commit hooks"},
		},
		Notes: []string{
			"This removes all pre-commit hooks that were installed with 'pre-commit install'.",
			"It does not affect your .pre-commit-config.yaml file.",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the uninstall command
func (c *UninstallCommand) Synopsis() string {
	return "Uninstall pre-commit hooks from git repository"
}

// Run executes the uninstall command
func (c *UninstallCommand) Run(args []string) int {
	var opts UninstallOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	_, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return 1
	}

	// Find git repository
	repo, err := git.NewRepository("")
	if err != nil {
		fmt.Printf("Error: not in a git repository: %v\n", err)
		return 1
	}

	// Uninstall pre-commit hook
	if err := repo.UninstallHook("pre-commit"); err != nil {
		fmt.Printf("Error: failed to uninstall pre-commit hook: %v\n", err)
		return 1
	}

	fmt.Println("pre-commit uninstalled")
	return 0
}

// UninstallCommandFactory creates a new uninstall command instance
func UninstallCommandFactory() (cli.Command, error) {
	return &UninstallCommand{}, nil
}
