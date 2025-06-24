package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

// InitTemplatedirCommand handles the init-templatedir command functionality
type InitTemplatedirCommand struct{}

// InitTemplatedirOptions holds command-line options for the init-templatedir command
type InitTemplatedirOptions struct {
	Config             string   `short:"c" long:"config"               description:"Path to config file"       default:".pre-commit-config.yaml"`
	HookTypes          []string `short:"t" long:"hook-type"            description:"Hook types to install"     default:"pre-commit"`
	AllowMissingConfig bool     `          long:"allow-missing-config" description:"Allow missing config file"`
	Verbose            bool     `short:"v" long:"verbose"              description:"Verbose output"`
	Help               bool     `short:"h" long:"help"                 description:"Show this help message"`
}

// Help returns the help text for the init-templatedir command
func (c *InitTemplatedirCommand) Help() string {
	var opts InitTemplatedirOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "DIRECTORY [OPTIONS]"

	formatter := &HelpFormatter{
		Command:     "init-templatedir",
		Description: "Install hook script in a directory intended for use with 'git config init.templateDir'.",
		Examples: []Example{
			{
				Command:     "pre-commit init-templatedir ~/.git-template",
				Description: "Set up template directory",
			},
			{
				Command:     "pre-commit init-templatedir /opt/git-template --hook-type pre-push",
				Description: "Set up with specific hook type",
			},
			{
				Command:     "git config --global init.templateDir ~/.git-template",
				Description: "Configure git to use template",
			},
		},
		Notes: []string{
			"positional arguments:",
			"  DIRECTORY             path where the git template will be created",
			"",
			"This command sets up pre-commit hooks in a template directory that can be",
			"used when initializing new git repositories. This is useful for organizations",
			"that want to automatically set up pre-commit hooks in all new repositories.",
			"",
			"After running this command, you can configure git to use the template directory:",
			"  git config --global init.templateDir /path/to/template/directory",
			"",
			"Then all new repositories created with 'git init' will automatically have",
			"pre-commit hooks installed.",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the init-templatedir command
func (c *InitTemplatedirCommand) Synopsis() string {
	return "Install hook script in a directory intended for use with git init templateDir"
}

// Run executes the init-templatedir command
func (c *InitTemplatedirCommand) Run(args []string) int {
	opts, templateDir, rc := c.parseAndValidateArgs(args)
	if rc != -1 {
		return rc
	}

	if err := c.createTemplateStructure(templateDir, opts); err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	if opts.Verbose {
		fmt.Printf("Successfully initialized template directory: %s\n", templateDir)
	}

	return 0
}

// parseAndValidateArgs parses command arguments and validates them
func (c *InitTemplatedirCommand) parseAndValidateArgs(
	args []string,
) (*InitTemplatedirOptions, string, int) {
	var opts InitTemplatedirOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "DIRECTORY [OPTIONS]"

	remaining, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return nil, "", 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return nil, "", 1
	}

	if len(remaining) == 0 {
		fmt.Println("Error: directory argument is required")
		fmt.Println("Usage: pre-commit init-templatedir DIRECTORY [OPTIONS]")
		return nil, "", 1
	}

	templateDir := remaining[0]

	if opts.Verbose {
		fmt.Printf("Initializing template directory: %s\n", templateDir)
		fmt.Printf("Hook types: %v\n", opts.HookTypes)
		fmt.Printf("Config file: %s\n", opts.Config)
	}

	return &opts, templateDir, -1
}

// createTemplateStructure creates the template directory structure and installs hooks
func (c *InitTemplatedirCommand) createTemplateStructure(
	templateDir string,
	opts *InitTemplatedirOptions,
) error {
	// Create template directory structure
	hooksDir := filepath.Join(templateDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		return fmt.Errorf("error creating hooks directory: %w", err)
	}

	// Check if config file exists (unless we allow missing config)
	if !opts.AllowMissingConfig {
		if _, err := os.Stat(opts.Config); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", opts.Config)
		}
	}

	// Install hook scripts for each hook type
	for _, hookType := range opts.HookTypes {
		hookPath := filepath.Join(hooksDir, hookType)

		// Create the hook script
		hookScript := fmt.Sprintf(`#!/bin/sh
# Installed by pre-commit
# See https://pre-commit.com for more information

if [ -x "$(command -v pre-commit)" ]; then
    exec pre-commit hook-impl --config=%s --hook-type=%s "$@"
else
    echo "pre-commit not found. Install pre-commit to use this hook."
    exit 1
fi
`, opts.Config, hookType)

		if err := os.WriteFile(hookPath, []byte(hookScript), 0o600); err != nil {
			return fmt.Errorf("error writing hook script: %w", err)
		}

		// Make the hook script executable
		// #nosec G302 - Hook scripts need to be executable
		if err := os.Chmod(hookPath, 0o700); err != nil {
			return fmt.Errorf("error making hook script executable: %w", err)
		}
	}

	return nil
}

// InitTemplatedirCommandFactory creates a new init-templatedir command instance
func InitTemplatedirCommandFactory() (cli.Command, error) {
	return &InitTemplatedirCommand{}, nil
}
