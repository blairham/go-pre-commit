package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
	"gopkg.in/yaml.v3"
)

// MigrateConfigCommand handles the migrate-config command functionality
type MigrateConfigCommand struct{}

// MigrateConfigOptions holds command-line options for the migrate-config command
type MigrateConfigOptions struct {
	Config  string `short:"c" long:"config"  description:"Path to config file"    default:".pre-commit-config.yaml"`
	Verbose bool   `short:"v" long:"verbose" description:"Verbose output"`
	Help    bool   `short:"h" long:"help"    description:"Show this help message"`
}

// Help returns the help text for the migrate-config command
func (c *MigrateConfigCommand) Help() string {
	var opts MigrateConfigOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	formatter := &HelpFormatter{
		Command:     "migrate-config",
		Description: "Migrate list configuration to new map configuration.",
		Examples: []Example{
			{
				Command:     "pre-commit migrate-config",
				Description: "Migrate .pre-commit-config.yaml to new format",
			},
			{
				Command:     "pre-commit migrate-config --verbose",
				Description: "Show detailed migration output",
			},
		},
		Notes: []string{
			"This command migrates old-style pre-commit configuration files to the",
			"newer format. The old format used a list of repositories, while the",
			"new format uses a 'repos' key with a list of repositories.",
			"",
			"Old format:",
			"  - repo: https://github.com/example/repo",
			"    hooks:",
			"    - id: example-hook",
			"",
			"New format:",
			"  repos:",
			"  - repo: https://github.com/example/repo",
			"    hooks:",
			"    - id: example-hook",
			"",
			"This migration is typically only needed when upgrading from very old",
			"versions of pre-commit.",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the migrate-config command
func (c *MigrateConfigCommand) Synopsis() string {
	return "Migrate list configuration to new map configuration"
}

// Run executes the migrate-config command
func (c *MigrateConfigCommand) Run(args []string) int {
	var opts MigrateConfigOptions
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

	// Check if config file exists
	if _, statErr := os.Stat(opts.Config); os.IsNotExist(statErr) {
		fmt.Printf("Error: config file not found: %s\n", opts.Config)
		return 1
	}

	if opts.Verbose {
		fmt.Printf("Checking configuration file: %s\n", opts.Config)
	}

	// Read and validate the config file
	content, err := c.readAndValidateConfig(opts.Config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Check if migration is needed
	needsMigration := c.needsMigration(string(content))
	if !needsMigration {
		c.printNoMigrationNeeded(opts.Verbose)
		return 0
	}

	if opts.Verbose {
		fmt.Println("Configuration needs migration from old format to new format")
	}

	// Perform the migration
	migratedContent := c.migrateConfig(string(content))

	// Write the migrated configuration back to the file
	if err := os.WriteFile(opts.Config, []byte(migratedContent), 0o600); err != nil {
		fmt.Printf("Error writing migrated config: %v\n", err)
		return 1
	}

	c.printMigrationSuccess(opts.Verbose, opts.Config)
	return 0
}

// readAndValidateConfig reads and validates the configuration file
func (c *MigrateConfigCommand) readAndValidateConfig(configPath string) ([]byte, error) {
	// nolint:gosec // User-specified config file is expected
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Validate YAML syntax first
	var yamlData any
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return nil, fmt.Errorf("invalid YAML syntax in config file: %w", err)
	}

	return content, nil
}

// needsMigration checks if the configuration needs migration
func (c *MigrateConfigCommand) needsMigration(configStr string) bool {
	// Simple heuristic: if the file doesn't start with "repos:" and contains "- repo:"
	// then it's likely the old format
	return !strings.Contains(configStr, "repos:") && strings.Contains(configStr, "- repo:")
}

// printNoMigrationNeeded prints message when no migration is needed
func (c *MigrateConfigCommand) printNoMigrationNeeded(verbose bool) {
	if verbose {
		fmt.Println("Configuration is already in the new format")
	} else {
		fmt.Println("No migration needed")
	}
}

// migrateConfig performs the actual migration
func (c *MigrateConfigCommand) migrateConfig(configStr string) string {
	// Add "repos:" at the beginning and indent the rest
	lines := strings.Split(configStr, "\n")
	var migratedLines []string
	migratedLines = append(migratedLines, "repos:")

	for _, line := range lines {
		// Skip empty lines at the beginning
		if len(migratedLines) == 1 && strings.TrimSpace(line) == "" {
			continue
		}

		// Indent each line by 2 spaces (or keep existing indentation if already indented)
		if strings.TrimSpace(line) != "" {
			migratedLines = append(migratedLines, "  "+line)
		} else {
			migratedLines = append(migratedLines, line)
		}
	}

	return strings.Join(migratedLines, "\n")
}

// printMigrationSuccess prints success message after migration
func (c *MigrateConfigCommand) printMigrationSuccess(verbose bool, configPath string) {
	fmt.Println("Configuration has been migrated.")

	if verbose {
		fmt.Printf("Updated configuration file: %s\n", configPath)
		fmt.Println("Please review the changes and commit them to your repository")
	}
}

// MigrateConfigCommandFactory creates a new migrate-config command instance
func MigrateConfigCommandFactory() (cli.Command, error) {
	return &MigrateConfigCommand{}, nil
}
