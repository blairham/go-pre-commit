package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/pkg/git"
)

// BaseCommand provides common functionality for all commands
type BaseCommand struct {
	Name        string
	Description string
	Examples    []Example
	Notes       []string
}

// CommonOptions defines options shared across multiple commands
type CommonOptions struct {
	Color   string `long:"color"   description:"Whether to use color in output" choice:"auto" default:"auto"`
	Config  string `long:"config"  description:"Path to config file"                          default:".pre-commit-config.yaml" short:"c"`
	Help    bool   `long:"help"    description:"Show this help message"                                                         short:"h"`
	Verbose bool   `long:"verbose" description:"Enable verbose output"                                                          short:"v"`
}

// GitRepositoryCommand provides common git repository functionality
type GitRepositoryCommand struct {
	BaseCommand
}

// RequireGitRepository ensures we're in a git repository and returns it
func (grc *GitRepositoryCommand) RequireGitRepository() (*git.Repository, error) {
	repo, err := git.NewRepository("")
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}
	return repo, nil
}

// ParseArgsWithHelp parses arguments and handles help display
func (bc *BaseCommand) ParseArgsWithHelp(opts any, args []string) ([]string, error) {
	parser := flags.NewParser(opts, flags.Default)

	remaining, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return nil, nil // Help was shown, exit gracefully
		}
		return nil, fmt.Errorf("error parsing arguments: %w", err)
	}

	return remaining, nil
}

// GenerateHelp creates standardized help output
func (bc *BaseCommand) GenerateHelp(parser *flags.Parser) string {
	formatter := &HelpFormatter{
		Command:     bc.Name,
		Description: bc.Description,
		Examples:    bc.Examples,
		Notes:       bc.Notes,
	}
	return formatter.FormatHelp(parser)
}

// ConfigFileExists checks if the config file exists
func (bc *BaseCommand) ConfigFileExists(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", configPath)
	}
	return nil
}

// HookTypeOptions provides common hook type functionality
type HookTypeOptions struct {
	HookTypes []string `short:"t" long:"hook-type" description:"Hook type to install (can be specified multiple times)"`
}

// GetDefaultHookTypes returns default hook types if none specified
func (hto *HookTypeOptions) GetDefaultHookTypes(defaultType string) []string {
	if len(hto.HookTypes) == 0 {
		return []string{defaultType}
	}
	return hto.HookTypes
}

// ValidateHookTypes validates that all specified hook types are supported
func (hto *HookTypeOptions) ValidateHookTypes() error {
	validTypes := map[string]bool{
		"pre-commit":         true,
		"pre-merge-commit":   true,
		"pre-push":           true,
		"prepare-commit-msg": true,
		"commit-msg":         true,
		"post-checkout":      true,
		"post-commit":        true,
		"post-merge":         true,
		"post-rewrite":       true,
		"pre-rebase":         true,
		"pre-auto-gc":        true,
	}

	for _, hookType := range hto.HookTypes {
		if !validTypes[hookType] {
			return fmt.Errorf("unsupported hook type: %s", hookType)
		}
	}
	return nil
}
