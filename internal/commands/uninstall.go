package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
)

// UninstallCommand handles the uninstall command functionality
type UninstallCommand struct{}

// UninstallOptions holds command-line options for the uninstall command
type UninstallOptions struct {
	Color     string   `long:"color" description:"Whether to use color in output" default:"auto" choice:"auto" choice:"always" choice:"never"`
	Config    string   `short:"c" long:"config" description:"Path to alternate config file" default:".pre-commit-config.yaml"`
	HookTypes []string `short:"t" long:"hook-type" description:"Hook type to uninstall"`
	Help      bool     `short:"h" long:"help" description:"Show this help message"`
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

	// Determine which hook types to uninstall
	hookTypes := c.getHookTypes(&opts)

	// Uninstall each hook type
	for _, hookType := range hookTypes {
		if err := c.uninstallHook(repo, hookType); err != nil {
			fmt.Printf("Error: failed to uninstall %s hook: %v\n", hookType, err)
			return 1
		}
	}

	return 0
}

// getHookTypes determines which hook types to uninstall
// This matches Python's _hook_types() behavior
func (c *UninstallCommand) getHookTypes(opts *UninstallOptions) []string {
	// If hook types are provided via CLI, use them
	if len(opts.HookTypes) > 0 {
		return opts.HookTypes
	}

	// Try to load config to get default_install_hook_types
	cfg, err := config.LoadConfig(opts.Config)
	if err != nil {
		// If config is invalid, fallback to pre-commit (matching Python)
		return []string{"pre-commit"}
	}

	// Return default_install_hook_types from config
	// If not specified in config, Python defaults to ['pre-commit']
	if len(cfg.DefaultInstallHookTypes) == 0 {
		return []string{"pre-commit"}
	}
	return cfg.DefaultInstallHookTypes
}

// uninstallHook uninstalls a specific hook type
// This matches Python's _uninstall_hook_script() behavior
func (c *UninstallCommand) uninstallHook(repo *git.Repository, hookType string) error {
	hookPath := filepath.Join(repo.Root, ".git", "hooks", hookType)
	legacyPath := hookPath + ".legacy"

	// Check if hook exists
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		// Hook doesn't exist - return silently (matching Python behavior)
		return nil
	}

	// Check if this is our hook (contains pre-commit identifier)
	isOurs, err := c.isOurHook(hookPath)
	if err != nil {
		return fmt.Errorf("failed to check hook ownership: %w", err)
	}

	if !isOurs {
		// Not our hook, don't remove it - return silently (matching Python)
		return nil
	}

	// Remove the hook
	if err := os.Remove(hookPath); err != nil {
		return fmt.Errorf("failed to remove hook: %w", err)
	}

	fmt.Printf("%s uninstalled\n", hookType)

	// Check if there's a legacy hook to restore
	if _, err := os.Stat(legacyPath); err == nil {
		// Restore legacy hook
		if err := os.Rename(legacyPath, hookPath); err != nil {
			return fmt.Errorf("failed to restore legacy hook: %w", err)
		}
		// Use relative path for output (matching Python)
		relPath := filepath.Join(".git", "hooks", hookType)
		fmt.Printf("Restored previous hooks to %s\n", relPath)
	}

	return nil
}

// isOurHook checks if the hook file was installed by pre-commit
func (c *UninstallCommand) isOurHook(hookPath string) (bool, error) {
	content, err := os.ReadFile(hookPath)
	if err != nil {
		return false, err
	}
	// Check for pre-commit identifier in the hook script
	return strings.Contains(string(content), "pre-commit"), nil
}

// UninstallCommandFactory creates a new uninstall command instance
func UninstallCommandFactory() (cli.Command, error) {
	return &UninstallCommand{}, nil
}
