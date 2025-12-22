package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"
)

// InitTemplatedirCommand handles the init-templatedir command functionality
type InitTemplatedirCommand struct {
	installer *HookInstaller
}

// InitTemplatedirOptions holds command-line options for the init-templatedir command
type InitTemplatedirOptions struct {
	Config                string   `short:"c" long:"config"                  description:"Path to config file"                                           default:".pre-commit-config.yaml"`
	HookTypes             []string `short:"t" long:"hook-type"               description:"Hook types to install (default: from config or pre-commit)"`
	NoAllowMissingConfig  bool     `          long:"no-allow-missing-config" description:"Assume cloned repos should have a pre-commit config"`
	Color                 string   `          long:"color"                   description:"Whether to use color in output. Defaults to BTICK_auto_BTICK." default:"auto" choice:"auto" choice:"always" choice:"never"`
	Help                  bool     `short:"h" long:"help"                    description:"Show this help message"`
}

// Help returns the help text for the init-templatedir command
func (c *InitTemplatedirCommand) Help() string {
	var opts InitTemplatedirOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--color {auto,always,never}] [-c CONFIG] [--no-allow-missing-config] [-t HOOK_TYPE] DIRECTORY"

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

	// Check if git config init.templateDir is set to this directory
	c.checkGitTemplateDir(templateDir)

	return 0
}

// checkGitTemplateDir checks if git init.templateDir is configured and warns if not
// This mirrors Python's init_templatedir() which checks git config init.templateDir
func (c *InitTemplatedirCommand) checkGitTemplateDir(templateDir string) {
	// Get the absolute path to the template directory
	absTemplateDir, err := filepath.Abs(templateDir)
	if err != nil {
		return // Skip warning if we can't resolve the path
	}

	// Read git config to check init.templateDir (checks all levels: system, global, local)
	// Python uses: cmd_output('git', 'config', 'init.templateDir')
	// We use go-git to read the config instead of shelling out to git CLI
	cfg, err := config.LoadConfig(config.GlobalScope)
	if err != nil {
		// Try system scope if global fails
		cfg, err = config.LoadConfig(config.SystemScope)
	}
	if err != nil {
		// Config not accessible - print warning
		fmt.Printf(
			"[WARNING] `init.templateDir` not set to the target directory:\n"+
				"    git config --global init.templateDir '%s'\n",
			absTemplateDir,
		)
		return
	}

	// Get init.templateDir value from config
	currentValue := cfg.Raw.Section("init").Option("templateDir")
	if currentValue == "" {
		// Config not set - print warning
		fmt.Printf(
			"[WARNING] `init.templateDir` not set to the target directory:\n"+
				"    git config --global init.templateDir '%s'\n",
			absTemplateDir,
		)
		return
	}

	currentValue = strings.TrimSpace(currentValue)
	// Expand ~ in the current value for comparison
	if strings.HasPrefix(currentValue, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			currentValue = filepath.Join(home, currentValue[1:])
		}
	}

	// Normalize both paths for comparison
	currentValueAbs, err := filepath.Abs(currentValue)
	if err != nil {
		currentValueAbs = currentValue
	}

	if currentValueAbs != absTemplateDir {
		fmt.Printf(
			"[WARNING] `init.templateDir` not set to the target directory:\n"+
				"    git config --global init.templateDir '%s'\n",
			absTemplateDir,
		)
	}
}

// parseAndValidateArgs parses command arguments and validates them
func (c *InitTemplatedirCommand) parseAndValidateArgs(
	args []string,
) (*InitTemplatedirOptions, string, int) {
	var opts InitTemplatedirOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--color {auto,always,never}] [-c CONFIG] [--no-allow-missing-config] [-t HOOK_TYPE] DIRECTORY"

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

	return &opts, templateDir, -1
}

// createTemplateStructure creates the template directory structure and installs hooks
// This delegates to the shared HookInstaller, mirroring Python's delegation to install()
func (c *InitTemplatedirCommand) createTemplateStructure(
	templateDir string,
	opts *InitTemplatedirOptions,
) error {
	// Create the installer if not already set (allows for dependency injection in tests)
	if c.installer == nil {
		c.installer = NewHookInstaller()
	}

	// Delegate to the shared install function with init-templatedir specific options:
	// - gitDir: the template directory (not .git)
	// - overwrite: true (always overwrite for init-templatedir, like Python)
	// - skipOnMissingConfig: true (template dirs need this since repos may not have config)
	// - allowMissingConfig: true by default (inverted from NoAllowMissingConfig flag)
	//   Python uses --no-allow-missing-config to disable this, defaulting to allow
	installOpts := &HookInstallOptions{
		Config:              opts.Config,
		HookTypes:           opts.HookTypes,
		GitDir:              templateDir,
		Overwrite:           true,                       // Python uses overwrite=True for init-templatedir
		SkipOnMissingConfig: true,                       // Python uses skip_on_missing_config=True for init-templatedir
		AllowMissingConfig:  !opts.NoAllowMissingConfig, // Default true, --no-allow-missing-config sets to false
	}

	return c.installer.Install(installOpts)
}

// InitTemplatedirCommandFactory creates a new init-templatedir command instance
func InitTemplatedirCommandFactory() (cli.Command, error) {
	return &InitTemplatedirCommand{}, nil
}
