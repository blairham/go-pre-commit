package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

// CleanCommand handles the clean command functionality
type CleanCommand struct{}

// CleanOptions holds command-line options for the clean command
type CleanOptions struct {
	Help  bool   `long:"help"  description:"show this help message and exit" short:"h"`
	Color string `long:"color" description:"Whether to use color in output. Defaults to BTICK_auto_BTICK." choice:"auto" choice:"always" choice:"never"`
}

// Help returns the help text for the clean command
func (c *CleanCommand) Help() string {
	var opts CleanOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--color {auto,always,never}]"

	formatter := &HelpFormatter{
		Command:     "clean",
		Description: "",
		Examples:    []Example{},
		Notes:       []string{},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the clean command
func (c *CleanCommand) Synopsis() string {
	return "Clean cached repositories and environments"
}

// Run executes the clean command
func (c *CleanCommand) Run(args []string) int {
	var opts CleanOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--color {auto,always,never}]"

	_, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return 1
	}

	if opts.Help {
		fmt.Print(c.Help())
		return 0
	}

	// Get cache directory (matches Python pre-commit behavior)
	cacheDir := getCacheDirectory()
	legacyDir := filepath.Join(os.Getenv("HOME"), ".pre-commit") // Legacy directory

	// Clean main cache directory (always print message like Python does)
	if _, err := os.Stat(cacheDir); err == nil {
		if err := os.RemoveAll(cacheDir); err != nil {
			fmt.Printf("Error: failed to clean cache directory: %v\n", err)
			return 1
		}
	}
	fmt.Printf("Cleaned %s.\n", cacheDir)

	// Clean legacy directory if it exists
	if _, err := os.Stat(legacyDir); err == nil {
		if err := os.RemoveAll(legacyDir); err != nil {
			fmt.Printf("⚠️  Warning: failed to clean legacy directory: %v\n", err)
		} else {
			fmt.Printf("Cleaned %s.\n", legacyDir)
		}
	}

	return 0
}

// getCacheDirectory returns the cache directory path (matches Python pre-commit logic)
func getCacheDirectory() string {
	// Check PRE_COMMIT_HOME environment variable
	if preCommitHome := os.Getenv("PRE_COMMIT_HOME"); preCommitHome != "" {
		return preCommitHome
	}

	// Check XDG_CACHE_HOME environment variable
	if xdgCacheHome := os.Getenv("XDG_CACHE_HOME"); xdgCacheHome != "" {
		return filepath.Join(xdgCacheHome, "pre-commit")
	}

	// Default: ~/.cache/pre-commit
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback if we can't get home directory
		return filepath.Join(os.TempDir(), ".cache", "pre-commit")
	}
	return filepath.Join(homeDir, ".cache", "pre-commit")
}

// CleanCommandFactory creates a new clean command instance
func CleanCommandFactory() (cli.Command, error) {
	return &CleanCommand{}, nil
}
