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

	// Get legacy directory path - use expanduser-like behavior for ~
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.Getenv("HOME")
	}
	legacyDir := filepath.Join(homeDir, ".pre-commit")

	// Clean directories - only print if directory exists (matches Python behavior exactly)
	for _, directory := range []string{cacheDir, legacyDir} {
		if _, err := os.Stat(directory); err == nil {
			if err := os.RemoveAll(directory); err != nil {
				fmt.Printf("Error: failed to clean directory %s: %v\n", directory, err)
				return 1
			}
			fmt.Printf("Cleaned %s.\n", directory)
		}
	}

	return 0
}

// getCacheDirectory returns the cache directory path (matches Python pre-commit logic)
// Uses filepath.EvalSymlinks to resolve symlinks like Python's os.path.realpath()
func getCacheDirectory() string {
	var path string

	// Check PRE_COMMIT_HOME environment variable
	if preCommitHome := os.Getenv("PRE_COMMIT_HOME"); preCommitHome != "" {
		path = preCommitHome
	} else if xdgCacheHome := os.Getenv("XDG_CACHE_HOME"); xdgCacheHome != "" {
		// Check XDG_CACHE_HOME environment variable
		path = filepath.Join(xdgCacheHome, "pre-commit")
	} else {
		// Default: ~/.cache/pre-commit
		homeDir, err := os.UserHomeDir()
		if err != nil {
			// Fallback if we can't get home directory
			return filepath.Join(os.TempDir(), ".cache", "pre-commit")
		}
		path = filepath.Join(homeDir, ".cache", "pre-commit")
	}

	// Resolve symlinks like Python's os.path.realpath()
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	// If path doesn't exist yet, try resolving parent directory
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		return filepath.Join(resolved, filepath.Base(path))
	}
	return path
}

// CleanCommandFactory creates a new clean command instance
func CleanCommandFactory() (cli.Command, error) {
	return &CleanCommand{}, nil
}
