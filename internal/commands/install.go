package commands

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/pkg/repository"
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
	opts, err := c.parseArguments(args)
	if err != nil {
		return c.handleParseError(err)
	}

	repo, err := git.NewRepository("")
	if err != nil {
		fmt.Printf("Error: not in a git repository: %v\n", err)
		return 1
	}

	hookTypes := c.getHookTypes(opts)
	if !c.validateHookTypes(hookTypes) {
		return 1
	}

	if !c.validateConfig(opts) {
		return 1
	}

	installed := c.installHooks(repo, hookTypes, opts)
	if installed == 0 {
		fmt.Println("No hooks were installed")
		return 1
	}

	fmt.Printf("Successfully installed %d hook(s)\n", installed)

	if opts.InstallHooks {
		if err := c.installHookEnvironments(opts.Config); err != nil {
			fmt.Printf("Error: failed to install hook environments: %v\n", err)
			return 1
		}
		fmt.Println("Hook environments installed successfully")
	}

	return 0
}

// parseArguments parses command line arguments
func (c *InstallCommand) parseArguments(args []string) (*InstallOptions, error) {
	var opts InstallOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	_, err := parser.ParseArgs(args)
	return &opts, err
}

// handleParseError handles argument parsing errors
func (c *InstallCommand) handleParseError(err error) int {
	var flagsErr *flags.Error
	if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
		return 0
	}
	fmt.Printf("Error parsing arguments: %v\n", err)
	return 1
}

// getHookTypes returns the hook types to install
func (c *InstallCommand) getHookTypes(opts *InstallOptions) []string {
	hookTypes := opts.HookTypes
	if len(hookTypes) == 0 {
		hookTypes = []string{"pre-commit"}
	}
	return hookTypes
}

// validateHookTypes validates that all hook types are valid
func (c *InstallCommand) validateHookTypes(hookTypes []string) bool {
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
			return false
		}
	}
	return true
}

// validateConfig checks if the config file exists when required
func (c *InstallCommand) validateConfig(opts *InstallOptions) bool {
	if !opts.AllowMissingConfig {
		if _, err := os.Stat(opts.Config); os.IsNotExist(err) {
			fmt.Printf("Error: config file not found: %s\n", opts.Config)
			fmt.Println("Run 'pre-commit sample-config' to generate a config file, or use --allow-missing-config")
			return false
		}
	}
	return true
}

// installHooks installs all the specified hook types
func (c *InstallCommand) installHooks(repo *git.Repository, hookTypes []string, opts *InstallOptions) int {
	installed := 0
	for _, hookType := range hookTypes {
		if !opts.Overwrite && repo.HasHook(hookType) {
			fmt.Printf("Hook %s already exists (use --overwrite to replace)\n", hookType)
			continue
		}

		script := c.generateHookScript(hookType)
		if err := repo.InstallHook(hookType, script); err != nil {
			fmt.Printf("Error: failed to install %s hook: %v\n", hookType, err)
			return 0 // Return 0 to indicate failure
		}

		fmt.Printf("pre-commit installed at .git/hooks/%s\n", hookType)
		installed++
	}
	return installed
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

// installHookEnvironments installs all hook environments specified in the config file
func (c *InstallCommand) installHookEnvironments(configPath string) error {
	// Load the configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize repository manager
	repoManager, err := repository.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize repository manager: %w", err)
	}
	defer func() {
		if closeErr := repoManager.Close(); closeErr != nil {
			fmt.Printf("⚠️  Warning: failed to close repository manager: %v\n", closeErr)
		}
	}()

	// Mark this config as used in the database so gc knows it's active
	if err := repoManager.MarkConfigUsed(configPath); err != nil {
		fmt.Printf("⚠️  Warning: failed to mark config as used: %v\n", err)
	}

	// Install environments for all repositories
	return c.ensureRepositoriesAndEnvironments(cfg, repoManager)
}

// ensureRepositoriesAndEnvironments ensures all repositories are cloned and environments are set up
func (c *InstallCommand) ensureRepositoriesAndEnvironments(
	cfg *config.Config,
	repoManager *repository.Manager,
) error {
	if len(cfg.Repos) == 0 {
		return nil
	}

	// Prepare repositories that need it
	for _, repo := range cfg.Repos {
		// For local and meta repos, only process environment setup for specific languages
		if repo.Repo == LocalRepo || repo.Repo == MetaRepo {
			// Check if any hooks need environment creation (e.g., conda)
			if c.repoNeedsEnvironmentSetup(repo) {
				fmt.Printf("[INFO] Initializing environment for %s.\n", repo.Repo)
				if err := c.setupLocalRepoEnvironments(repo, repoManager); err != nil {
					return fmt.Errorf("failed to setup environments for %s: %w", repo.Repo, err)
				}
			}
			continue
		}

		fmt.Printf("[INFO] Initializing environment for %s.\n", repo.Repo)

		// Clone or update the repository with dependencies (this will set up environments)
		_, err := repoManager.CloneOrUpdateRepoWithDeps(
			context.Background(),
			repo,
			[]string{}, // no additional dependencies
		)
		if err != nil {
			return fmt.Errorf("failed to prepare repository %s: %w", repo.Repo, err)
		}

		fmt.Printf("[INFO] Installing environment for %s.\n", repo.Repo)
		fmt.Printf("[INFO] Once installed this environment will be reused.\n")
		fmt.Printf("[INFO] This may take a few minutes...\n")
	}

	return nil
}

// repoNeedsEnvironmentSetup checks if a repository has hooks that require environment setup
func (c *InstallCommand) repoNeedsEnvironmentSetup(repo config.Repo) bool {
	// Languages that require environment setup even for local repos
	environmentLanguages := map[string]bool{
		"conda":  true,
		"python": true,
		"node":   true,
		// Add other languages that need environments as needed
	}

	for _, hook := range repo.Hooks {
		if environmentLanguages[hook.Language] {
			return true
		}
	}
	return false
}

// setupLocalRepoEnvironments sets up environments for local repository hooks
func (c *InstallCommand) setupLocalRepoEnvironments(
	repo config.Repo,
	repoManager *repository.Manager,
) error {
	// Get current working directory as the repo path for local repos
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Process each hook that needs environment setup
	for _, hook := range repo.Hooks {
		if hook.Language == "conda" {
			fmt.Printf("[INFO] Installing environment for %s.\n", repo.Repo)
			fmt.Printf("[INFO] Once installed this environment will be reused.\n")
			fmt.Printf("[INFO] This may take a few minutes...\n")

			// Set up conda environment for this hook
			if err := c.setupCondaEnvironment(hook, cwd, repoManager); err != nil {
				return fmt.Errorf("failed to setup conda environment for hook %s: %w", hook.ID, err)
			}
		}
		// Add other language environment setups as needed
	}

	return nil
}

// setupCondaEnvironment creates a conda environment for a specific hook
func (c *InstallCommand) setupCondaEnvironment(
	hook config.Hook,
	repoPath string,
	repoManager *repository.Manager,
) error {
	// Create mock repository info for local repo
	repoInfo := config.Repo{
		Repo:  "local",
		Rev:   "HEAD",
		Hooks: []config.Hook{hook},
	}

	// Setup environment using the repository manager's environment manager
	_, err := repoManager.SetupHookEnvironment(hook, repoInfo, repoPath)
	if err != nil {
		return fmt.Errorf("failed to setup conda environment: %w", err)
	}

	return nil
}

// InstallCommandFactory creates a new install command instance
func InstallCommandFactory() (cli.Command, error) {
	return &InstallCommand{}, nil
}
