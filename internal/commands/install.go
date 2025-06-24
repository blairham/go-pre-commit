package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/git"
)

// InstallCommand handles the install command functionality
type InstallCommand struct{}

// InstallOptions holds command-line options for the install command
type InstallOptions struct {
	Config             string   `short:"c" long:"config"               description:"Path to config file"                    default:".pre-commit-config.yaml"`
	Color              string   `          long:"color"                description:"Whether to use color in output"         default:"auto"                    choice:"auto"`
	HookTypes          []string `short:"t" long:"hook-type"            description:"Hook type to install (multiple times)"  default:"pre-commit"`
	Overwrite          bool     `short:"f" long:"overwrite"            description:"Overwrite existing hooks"`
	InstallHooks       bool     `          long:"install-hooks"        description:"Install environment for all repos"`
	AllowMissingConfig bool     `          long:"allow-missing-config" description:"Allow installing without a config file"`
	Help               bool     `short:"h" long:"help"                 description:"Show this help message"`
}

// Help returns the help text for the install command
func (c *InstallCommand) Help() string {
	var opts InstallOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	formatter := &HelpFormatter{
		Command:     "install",
		Description: "Install pre-commit hooks into the git repository.",
		Examples: []Example{
			{Command: "pre-commit install", Description: "Install pre-commit hook"},
			{
				Command:     "pre-commit install --hook-type pre-push",
				Description: "Install pre-push hook",
			},
			{
				Command:     "pre-commit install -t pre-commit -t pre-push",
				Description: "Install multiple hooks",
			},
			{Command: "pre-commit install --overwrite", Description: "Overwrite existing hooks"},
			{
				Command:     "pre-commit install --install-hooks",
				Description: "Also install hook environments",
			},
			{
				Command:     "pre-commit install --allow-missing-config",
				Description: "Install without config file",
			},
		},
		Notes: []string{
			"Available hook types:",
			"  pre-commit, pre-merge-commit, pre-push, prepare-commit-msg,",
			"  commit-msg, post-checkout, post-commit, post-merge, post-rewrite,",
			"  pre-rebase, pre-auto-gc",
			"",
			"By default, only the pre-commit hook is installed.",
			"Use --hook-type to install other types.",
			"Multiple hook types can be specified.",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the install command
func (c *InstallCommand) Synopsis() string {
	return "Install pre-commit hooks into git repository"
}

// Run executes the install command
func (c *InstallCommand) Run(args []string) int {
	var opts InstallOptions
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

	// Default to pre-commit if no hook types specified
	hookTypes := opts.HookTypes
	if len(hookTypes) == 0 {
		hookTypes = []string{"pre-commit"}
	}

	// Validate hook types
	validHookTypes := map[string]bool{
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

	for _, hookType := range hookTypes {
		if !validHookTypes[hookType] {
			fmt.Printf("Error: invalid hook type '%s'\n", hookType)
			return 1
		}
	}

	// Check for config file unless allow-missing-config is set
	if !opts.AllowMissingConfig {
		if _, err := os.Stat(opts.Config); os.IsNotExist(err) {
			fmt.Printf("Error: config file not found: %s\n", opts.Config)
			fmt.Println(
				"Run 'pre-commit sample-config' to generate a config file, or use --allow-missing-config",
			)
			return 1
		}
	}

	// Install each hook type
	installed := 0
	for _, hookType := range hookTypes {
		if !opts.Overwrite && repo.HasHook(hookType) {
			fmt.Printf("Hook %s already exists (use --overwrite to replace)\n", hookType)
			continue
		}

		script := c.generateHookScript(hookType)
		if err := repo.InstallHook(hookType, script); err != nil {
			fmt.Printf("Error: failed to install %s hook: %v\n", hookType, err)
			return 1
		}

		fmt.Printf("pre-commit installed at .git/hooks/%s\n", hookType)
		installed++
	}

	if installed == 0 {
		fmt.Println("No hooks were installed")
		return 1
	}

	fmt.Printf("Successfully installed %d hook(s)\n", installed)
	return 0
}

// generateHookScript generates the appropriate script for each hook type
func (c *InstallCommand) generateHookScript(hookType string) string {
	base := `#!/bin/sh
# Generated by go-pre-commit
`

	switch hookType {
	case hookTypePreCommit:
		return base + `exec pre-commit run --hook-stage=pre-commit`
	case "pre-merge-commit":
		return base + `exec pre-commit run --hook-stage=pre-merge-commit`
	case hookTypePrePush:
		return base + `exec pre-commit run --hook-stage=pre-push --from-ref="$2" --to-ref="$1"`
	case hookTypePrepareCommit:
		return base + `exec pre-commit run --hook-stage=prepare-commit-msg ` +
			`--commit-msg-filename="$1" --prepare-commit-message-source="$2" --commit-object-name="$3"`
	case hookTypeCommitMsg:
		return base + `exec pre-commit run --hook-stage=commit-msg --commit-msg-filename="$1"`
	case hookTypePostCheckout:
		return base + `exec pre-commit run --hook-stage=post-checkout --checkout-type="$3"`
	case hookTypePostCommit:
		return base + `exec pre-commit run --hook-stage=post-commit`
	case hookTypePostMerge:
		return base + `exec pre-commit run --hook-stage=post-merge --is-squash-merge="$1"`
	case hookTypePostRewrite:
		return base + `exec pre-commit run --hook-stage=post-rewrite --rewrite-command="$1"`
	case hookTypePreRebase:
		return base + `exec pre-commit run --hook-stage=pre-rebase --pre-rebase-upstream="$1" --pre-rebase-branch="$2"`
	case "pre-auto-gc":
		return base + `exec pre-commit run --hook-stage=pre-auto-gc`
	default:
		return base + fmt.Sprintf(`exec pre-commit run --hook-stage=%s`, hookType)
	}
}

// InstallCommandFactory creates a new install command instance
func InstallCommandFactory() (cli.Command, error) {
	return &InstallCommand{}, nil
}
