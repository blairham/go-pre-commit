package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
)

// Hook script hash markers used by Python pre-commit to identify its hooks.
// These are embedded in the hook template and used for ownership detection.
// PRIOR_HASHES are for backwards compatibility with older pre-commit versions.
var (
	// CURRENT_HASH is the current hash marker used by Python pre-commit
	CURRENT_HASH = []byte("138fd403232d2ddd5efb44317e38bf03")

	// PRIOR_HASHES are hash markers from previous Python pre-commit versions
	PRIOR_HASHES = [][]byte{
		[]byte("4d9958c90bc262f47553e2c073f14cfe"),
		[]byte("d8ee923c46731b42cd95cc869add4062"),
		[]byte("49fd668cb42069aa1b6048464be5d395"),
		[]byte("79f09a650522a87b0da915d0d983b2de"),
		[]byte("e358c9dae00eac5d06b38dfdb1e33a8c"),
	}
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
// Python's uninstall always returns 0, even on errors.
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
		// Python's uninstall doesn't fail on parse errors for uninstall
		// but we should still return 0 for parity
		return 0
	}

	// Find git repository - if not in a git repo, silently return 0
	// (matching Python's behavior of not failing)
	repo, err := git.NewRepository("")
	if err != nil {
		return 0
	}

	// Determine which hook types to uninstall
	hookTypes := c.getHookTypes(&opts)

	// Uninstall each hook type
	// Python ignores errors and continues, always returning 0
	for _, hookType := range hookTypes {
		_ = c.uninstallHook(repo, hookType)
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
		// Restore legacy hook - use os.Rename which is like Python's os.replace
		if err := os.Rename(legacyPath, hookPath); err != nil {
			return fmt.Errorf("failed to restore legacy hook: %w", err)
		}
		// Python outputs the full hook_path, not a relative path
		fmt.Printf("Restored previous hooks to %s\n", hookPath)
	}

	return nil
}

// isOurHook checks if the hook file was installed by pre-commit.
// This matches Python's is_our_script() behavior by checking for
// hash markers embedded in the hook template.
func (c *UninstallCommand) isOurHook(hookPath string) (bool, error) {
	// Check if file exists (handles symlinks)
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		return false, nil
	}

	content, err := os.ReadFile(hookPath)
	if err != nil {
		return false, err
	}

	// Check for current hash first
	if bytes.Contains(content, CURRENT_HASH) {
		return true, nil
	}

	// Check for any prior hashes (backwards compatibility)
	for _, hash := range PRIOR_HASHES {
		if bytes.Contains(content, hash) {
			return true, nil
		}
	}

	return false, nil
}

// UninstallCommandFactory creates a new uninstall command instance
func UninstallCommandFactory() (cli.Command, error) {
	return &UninstallCommand{}, nil
}
