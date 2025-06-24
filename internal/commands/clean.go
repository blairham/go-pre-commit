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
	Verbose bool `short:"v" long:"verbose" description:"Verbose output showing what is being cleaned"`
	Help    bool `short:"h" long:"help"    description:"Show this help message"`
}

// Help returns the help text for the clean command
func (c *CleanCommand) Help() string {
	var opts CleanOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	formatter := &HelpFormatter{
		Command:     "clean",
		Description: "Clean cached repositories and hook environments.",
		Examples: []Example{
			{Command: "pre-commit clean", Description: "Clean all cached data"},
			{Command: "pre-commit clean --verbose", Description: "Show detailed output"},
		},
		Notes: []string{
			"Cleans all cached data including repositories and environments.",
			"This removes the entire cache directory and forces repositories to be re-cloned.",
		},
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

	// Get cache directory (matches Python pre-commit behavior)
	cacheDir := getCacheDirectory()
	legacyDir := filepath.Join(os.Getenv("HOME"), ".pre-commit") // Legacy directory

	cleanedDirs := []string{}

	// Clean main cache directory
	if _, err := os.Stat(cacheDir); err == nil {
		if opts.Verbose {
			fmt.Printf("Cleaning cache directory: %s\n", cacheDir)
		}
		if err := os.RemoveAll(cacheDir); err != nil {
			fmt.Printf("Error: failed to clean cache directory: %v\n", err)
			return 1
		}
		cleanedDirs = append(cleanedDirs, cacheDir)
		fmt.Printf("Cleaned %s.\n", cacheDir)
	}

	// Clean legacy directory if it exists
	if _, err := os.Stat(legacyDir); err == nil {
		if opts.Verbose {
			fmt.Printf("Cleaning legacy directory: %s\n", legacyDir)
		}
		if err := os.RemoveAll(legacyDir); err != nil {
			fmt.Printf("Warning: failed to clean legacy directory: %v\n", err)
		} else {
			cleanedDirs = append(cleanedDirs, legacyDir)
			fmt.Printf("Cleaned %s.\n", legacyDir)
		}
	}

	if len(cleanedDirs) == 0 {
		if opts.Verbose {
			fmt.Printf("No cache directories found to clean.\n")
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
