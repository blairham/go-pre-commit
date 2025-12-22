package commands

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
	"gopkg.in/yaml.v3"
)

// MigrateConfigCommand handles the migrate-config command functionality
type MigrateConfigCommand struct{}

// MigrateConfigOptions holds command-line options for the migrate-config command
type MigrateConfigOptions struct {
	Config string `short:"c" long:"config" description:"Path to config file" default:".pre-commit-config.yaml"`
	Help   bool   `short:"h" long:"help"   description:"Show this help message"`
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
		},
		Notes: []string{
			"This command migrates old-style pre-commit configuration files to the",
			"newer format. The old format used a list of repositories, while the",
			"new format uses a 'repos' key with a list of repositories.",
			"",
			"Migrations performed:",
			"  - List format → 'repos:' key format",
			"  - 'sha:' → 'rev:' key rename",
			"  - 'language: python_venv' → 'language: python'",
			"  - Stage names: 'commit' → 'pre-commit', 'push' → 'pre-push',",
			"                 'merge-commit' → 'pre-merge-commit'",
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

	// Read and validate the config file
	content, err := c.readAndValidateConfig(opts.Config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	return c.migrateConfigFile(opts.Config, string(content), false)
}

// MigrateConfigQuiet is called internally by other commands (like autoupdate)
// with quiet=true to suppress output
func (c *MigrateConfigCommand) MigrateConfigQuiet(configPath string) error {
	// Check if config file exists
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	content, err := c.readAndValidateConfig(configPath)
	if err != nil {
		return err
	}

	exitCode := c.migrateConfigFile(configPath, string(content), true)
	if exitCode != 0 {
		return fmt.Errorf("migration failed")
	}
	return nil
}

// migrateConfigFile performs migration and handles output based on quiet flag
func (c *MigrateConfigCommand) migrateConfigFile(configPath, content string, quiet bool) int {
	origContent := content

	// Apply all migrations
	content = c.migrateMap(content)
	content = c.migrateComposed(content)

	// Check if any changes were made
	if content == origContent {
		if !quiet {
			fmt.Println("Configuration is already migrated.")
		}
		return 0
	}

	// Write the migrated configuration back to the file
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		if !quiet {
			fmt.Printf("Error writing migrated config: %v\n", err)
		}
		return 1
	}

	if !quiet {
		fmt.Println("Configuration has been migrated.")
	}
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

// isHeaderLine checks if a line is a header line (comment, document marker, or empty)
// This matches Python's _is_header_line function
func (c *MigrateConfigCommand) isHeaderLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---")
}

// migrateMap migrates list format to repos: map format
// This matches Python's _migrate_map function
func (c *MigrateConfigCommand) migrateMap(content string) string {
	// Check if content is a list (old format)
	var yamlData any
	if err := yaml.Unmarshal([]byte(content), &yamlData); err != nil {
		return content
	}

	// Only migrate if it's a list at the top level
	if _, isList := yamlData.([]any); !isList {
		return content
	}

	// Split into lines, preserving line endings
	lines := strings.Split(content, "\n")

	// Find header lines (comments, ---, empty lines at the start)
	headerEnd := 0
	for i, line := range lines {
		if !c.isHeaderLine(line) {
			headerEnd = i
			break
		}
		// If we reach the end without finding non-header content
		if i == len(lines)-1 {
			headerEnd = len(lines)
		}
	}

	header := strings.Join(lines[:headerEnd], "\n")
	rest := strings.Join(lines[headerEnd:], "\n")

	// If header is non-empty, add newline after it
	if header != "" && !strings.HasSuffix(header, "\n") {
		header += "\n"
	}

	// Try the simple approach first (just prepend repos:)
	trialContent := header + "repos:\n" + rest
	var testData any
	if err := yaml.Unmarshal([]byte(trialContent), &testData); err == nil {
		return trialContent
	}

	// If that fails, indent the rest by 4 spaces (matching Python's textwrap.indent)
	indentedLines := []string{}
	for _, line := range strings.Split(rest, "\n") {
		if strings.TrimSpace(line) != "" {
			indentedLines = append(indentedLines, "    "+line)
		} else {
			indentedLines = append(indentedLines, line)
		}
	}
	return header + "repos:\n" + strings.Join(indentedLines, "\n")
}

// migrateComposed applies composed migrations (sha→rev, python_venv→python, stages)
// This matches Python's _migrate_composed function
func (c *MigrateConfigCommand) migrateComposed(content string) string {
	// Apply sha → rev migration
	content = c.migrateShaToRev(content)

	// Apply python_venv → python migration
	content = c.migratePythonVenv(content)

	// Apply stages migration
	content = c.migrateStages(content)

	return content
}

// migrateShaToRev replaces 'sha:' with 'rev:' in repo definitions
// Preserves quote style around the key
func (c *MigrateConfigCommand) migrateShaToRev(content string) string {
	// Match sha: key in YAML (with optional quotes)
	// Pattern matches: sha:, 'sha':, "sha":
	patterns := []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		// Unquoted sha: at start of line or after whitespace
		{regexp.MustCompile(`(\n\s*)sha:`), "${1}rev:"},
		// Single-quoted 'sha':
		{regexp.MustCompile(`(\n\s*)'sha':`), "${1}'rev':"},
		// Double-quoted "sha":
		{regexp.MustCompile(`(\n\s*)"sha":`), `${1}"rev":`},
	}

	for _, p := range patterns {
		content = p.pattern.ReplaceAllString(content, p.replacement)
	}

	return content
}

// migratePythonVenv replaces 'python_venv' with 'python' for language
// Preserves quote style around the value
func (c *MigrateConfigCommand) migratePythonVenv(content string) string {
	// Match language: python_venv (with optional quotes around value)
	patterns := []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		// Unquoted python_venv
		{regexp.MustCompile(`(language:\s*)python_venv(\s*)$`), "${1}python${2}"},
		{regexp.MustCompile(`(language:\s*)python_venv(\s*\n)`), "${1}python${2}"},
		// Single-quoted 'python_venv'
		{regexp.MustCompile(`(language:\s*)'python_venv'`), "${1}'python'"},
		// Double-quoted "python_venv"
		{regexp.MustCompile(`(language:\s*)"python_venv"`), `${1}"python"`},
	}

	for _, p := range patterns {
		content = p.pattern.ReplaceAllString(content, p.replacement)
	}

	return content
}

// migrateStages migrates old stage names to new format
// commit → pre-commit, push → pre-push, merge-commit → pre-merge-commit
func (c *MigrateConfigCommand) migrateStages(content string) string {
	// Old stage names that need the "pre-" prefix
	oldStages := map[string]string{
		"commit":       "pre-commit",
		"push":         "pre-push",
		"merge-commit": "pre-merge-commit",
	}

	// We need to be careful to only replace stage names in stages: or default_stages: contexts
	// Use regex to match stage values in array contexts

	for old, new := range oldStages {
		// Match in array context: [commit, ...] or - commit
		// Be careful not to replace "pre-commit" with "pre-pre-commit"

		// Array syntax: [commit] or [commit, push]
		// Match word boundaries to avoid partial matches
		patterns := []struct {
			pattern     *regexp.Regexp
			replacement string
		}{
			// In square brackets: [commit or , commit or commit]
			{regexp.MustCompile(`(\[|\s*,\s*)` + regexp.QuoteMeta(old) + `(\s*,|\s*\])`), "${1}" + new + "${2}"},
			// Single-quoted in array
			{regexp.MustCompile(`(\[|\s*,\s*)'` + regexp.QuoteMeta(old) + `'(\s*,|\s*\])`), "${1}'" + new + "'${2}"},
			// Double-quoted in array
			{regexp.MustCompile(`(\[|\s*,\s*)"` + regexp.QuoteMeta(old) + `"(\s*,|\s*\])`), `${1}"` + new + `"${2}`},
			// List syntax: - commit (for stages: or default_stages:)
			{regexp.MustCompile(`(stages:\s*\n(?:\s*-\s*\S+\n)*\s*-\s*)` + regexp.QuoteMeta(old) + `(\s*\n)`), "${1}" + new + "${2}"},
			{regexp.MustCompile(`(default_stages:\s*\n(?:\s*-\s*\S+\n)*\s*-\s*)` + regexp.QuoteMeta(old) + `(\s*\n)`), "${1}" + new + "${2}"},
		}

		for _, p := range patterns {
			content = p.pattern.ReplaceAllString(content, p.replacement)
		}
	}

	return content
}

// needsMigration checks if the configuration needs migration (legacy method for compatibility)
func (c *MigrateConfigCommand) needsMigration(configStr string) bool {
	// Check for list format (no repos: key but has - repo:)
	if !strings.Contains(configStr, "repos:") && strings.Contains(configStr, "- repo:") {
		return true
	}

	// Check for sha: key
	if regexp.MustCompile(`\n\s*['"]?sha['"]?:`).MatchString(configStr) {
		return true
	}

	// Check for python_venv language
	if strings.Contains(configStr, "python_venv") {
		return true
	}

	// Check for old stage names (only in stages context)
	stagesPattern := regexp.MustCompile(`stages:.*\b(commit|push|merge-commit)\b`)
	defaultStagesPattern := regexp.MustCompile(`default_stages:.*\b(commit|push|merge-commit)\b`)
	if stagesPattern.MatchString(configStr) || defaultStagesPattern.MatchString(configStr) {
		return true
	}

	return false
}

// migrateConfig performs the actual migration (legacy method for compatibility)
func (c *MigrateConfigCommand) migrateConfig(configStr string) string {
	content := c.migrateMap(configStr)
	content = c.migrateComposed(content)
	return content
}

// MigrateConfigCommandFactory creates a new migrate-config command instance
func MigrateConfigCommandFactory() (cli.Command, error) {
	return &MigrateConfigCommand{}, nil
}
