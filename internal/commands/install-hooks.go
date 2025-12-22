package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// InstallHooksCommand handles the install-hooks command functionality
type InstallHooksCommand struct{}

// InstallHooksOptions holds command-line options for the install-hooks command
type InstallHooksOptions struct {
	Config string `short:"c" long:"config" description:"Path to config file" default:".pre-commit-config.yaml"`
	Help   bool   `short:"h" long:"help"   description:"Show this help message"`
}

// Help returns the help text for the install-hooks command
func (c *InstallHooksCommand) Help() string {
	var opts InstallHooksOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	formatter := &HelpFormatter{
		Command:     "install-hooks",
		Description: "Install hook environments for all environments",
		Examples: []Example{
			{
				Command:     "pre-commit install-hooks",
				Description: "Install environments for all hooks",
			},
			{
				Command:     "pre-commit install-hooks -c custom-config.yaml",
				Description: "Install hooks from a custom config file",
			},
		},
		Notes: []string{
			"This command installs all the hook environments specified in the config file.",
			"Once installed, these environments will be reused for subsequent hook executions.",
			"",
			"This is useful in CI/CD environments where you want to prepare",
			"all hook environments upfront to avoid first-run delays.",
			"",
			"You may find 'pre-commit install --install-hooks' more useful for",
			"development environments where you want immediate environment setup.",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the install-hooks command
func (c *InstallHooksCommand) Synopsis() string {
	return "Install hook environments for all environments in the config file"
}

// Helper functions to reduce cognitive complexity in InstallHooksCommand.Run

func (c *InstallHooksCommand) validateEnvironment(opts *InstallHooksOptions) error {
	// Check if config file exists
	// Note: Unlike install/run commands, install-hooks does NOT require being in a git repo
	// This matches Python's behavior - it just sets up environments in the pre-commit cache
	if _, statErr := os.Stat(opts.Config); os.IsNotExist(statErr) {
		return fmt.Errorf("config file not found: %s", opts.Config)
	}

	return nil
}

func (c *InstallHooksCommand) loadConfigAndInitManager(
	opts *InstallHooksOptions,
) (*config.Config, *repository.Manager, error) {
	// Load the configuration
	cfg, err := config.LoadConfig(opts.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	// Initialize repository manager and mark config as used
	repoManager, err := repository.NewManager()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize repository manager: %w", err)
	}

	// Mark this config as used in the database so gc knows it's active
	// Ignore errors - this is non-critical functionality
	if err := repoManager.MarkConfigUsed(opts.Config); err != nil {
		// Non-critical error, just log it
	}

	return cfg, repoManager, nil
}

func (c *InstallHooksCommand) prepareAllRepositories(
	cfg *config.Config,
	repoManager *repository.Manager,
) error {
	return c.ensureRepositoriesAndEnvironments(cfg, repoManager)
}

// ensureRepositoriesAndEnvironments ensures all repositories are cloned and environments are set up
func (c *InstallHooksCommand) ensureRepositoriesAndEnvironments(
	cfg *config.Config,
	repoManager *repository.Manager,
) error {
	if len(cfg.Repos) == 0 {
		return nil
	}

	if !c.checkIfAnyRepositoryNeedsPreparation(cfg, repoManager) {
		return nil
	}

	return c.prepareRepositoriesThatNeedIt(cfg, repoManager)
}

// checkIfAnyRepositoryNeedsPreparation checks if any repositories need preparation
func (c *InstallHooksCommand) checkIfAnyRepositoryNeedsPreparation(
	cfg *config.Config,
	repoManager *repository.Manager,
) bool {
	for _, repo := range cfg.Repos {
		// Skip local and meta repos
		if repo.Repo == LocalRepo || repo.Repo == MetaRepo {
			continue
		}

		// Check if repository is already cloned and environments are set up
		if !c.isRepositoryFullyPrepared(repo, repoManager) {
			return true
		}
	}
	return false
}

// prepareRepositoriesThatNeedIt prepares all repositories that need preparation
func (c *InstallHooksCommand) prepareRepositoriesThatNeedIt(
	cfg *config.Config,
	repoManager *repository.Manager,
) error {
	for _, repo := range cfg.Repos {
		if c.shouldSkipRepository(repo) {
			continue
		}

		if err := c.prepareRepositoryForHooks(repo, repoManager, cfg); err != nil {
			return fmt.Errorf("failed to prepare repository for %s: %w", repo.Repo, err)
		}
	}

	return nil
}

// shouldSkipRepository checks if a repository should be skipped during preparation
func (c *InstallHooksCommand) shouldSkipRepository(repo config.Repo) bool {
	return repo.Repo == LocalRepo || repo.Repo == MetaRepo
}

// isRepositoryFullyPrepared checks if a repository is cloned and all environments are set up
func (c *InstallHooksCommand) isRepositoryFullyPrepared(
	repo config.Repo,
	repoManager *repository.Manager,
) bool {
	// Check if repository is cloned
	repoPath := repoManager.GetRepoPath(repo)
	if repoPath == "" {
		return false
	}

	// Check if repository directory exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return false
	}

	// Check if environments are set up for all hooks
	for _, hook := range repo.Hooks {
		if !c.isHookEnvironmentReady(hook, repoPath, repoManager) {
			return false
		}
	}

	return true
}

// isHookEnvironmentReady checks if a hook's environment is ready
func (c *InstallHooksCommand) isHookEnvironmentReady(
	hook config.Hook, repoPath string, repoManager *repository.Manager,
) bool {
	// Check if hook definition file exists in the repository
	hookDefPath := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	if _, err := os.Stat(hookDefPath); os.IsNotExist(err) {
		return false
	}

	// For languages that need environment setup, check if the environment is actually ready
	languageVersion := hook.LanguageVersion
	if languageVersion == "" {
		languageVersion = "default"
	}

	// Check environment health using the repository manager
	if err := repoManager.CheckEnvironmentHealthWithRepo(hook.Language, languageVersion, repoPath); err != nil {
		return false
	}

	return true
}

// prepareRepositoryForHooks clones repositories and sets up environments for all hooks
func (c *InstallHooksCommand) prepareRepositoryForHooks(
	repo config.Repo,
	repoManager *repository.Manager,
	cfg *config.Config,
) error {
	// Clone or update the repository
	repoPath, err := repoManager.CloneOrUpdateRepo(context.Background(), repo)
	if err != nil {
		return fmt.Errorf("failed to clone repository %s: %w", repo.Repo, err)
	}

	// Install environments for each hook in this repository
	for _, hook := range repo.Hooks {
		// Set up the environment for this hook
		// Don't fail the entire command if one hook environment setup fails
		if err := c.setupHookEnvironment(hook, repo, repoPath, repoManager, cfg); err != nil {
			// Log error but continue with other hooks
		}
	}

	return nil
}

// setupHookEnvironment sets up the environment for a specific hook
func (c *InstallHooksCommand) setupHookEnvironment(
	hook config.Hook,
	repo config.Repo,
	repoPath string,
	repoManager *repository.Manager,
	cfg *config.Config,
) error {
	// Merge hook configuration from config file with repository hook definition
	mergedHook, err := c.mergeHookWithRepoDefinition(hook, repoPath, repoManager, cfg)
	if err != nil {
		return fmt.Errorf("failed to merge hook configuration for %s: %w", hook.ID, err)
	}

	// Set up the hook environment using the repository manager
	_, err = repoManager.SetupHookEnvironment(mergedHook, repo, repoPath)
	if err != nil {
		return fmt.Errorf("failed to setup environment for hook %s: %w", hook.ID, err)
	}

	return nil
}

// mergeHookWithRepoDefinition merges a hook from config file with its repository definition
func (c *InstallHooksCommand) mergeHookWithRepoDefinition(
	configHook config.Hook,
	repoPath string,
	repoManager *repository.Manager,
	cfg *config.Config,
) (config.Hook, error) {
	// Get the hook definition from the repository
	repoHook, found := repoManager.GetRepositoryHook(repoPath, configHook.ID)
	if !found {
		return configHook, fmt.Errorf("hook %s not found in repository %s", configHook.ID, repoPath)
	}

	// Start with the repository hook definition (has all the defaults)
	mergedHook := repoHook

	// Resolve effective language version considering both hook.LanguageVersion and default_language_version
	// Use the repository hook's language since configHook.Language is not set in config files
	hookForVersionResolution := configHook
	hookForVersionResolution.Language = repoHook.Language
	effectiveVersion := config.ResolveEffectiveLanguageVersion(hookForVersionResolution, *cfg)
	mergedHook.LanguageVersion = effectiveVersion

	// Override with other configuration from the config file
	if len(configHook.AdditionalDeps) > 0 {
		mergedHook.AdditionalDeps = configHook.AdditionalDeps
	}
	if len(configHook.Args) > 0 {
		mergedHook.Args = configHook.Args
	}
	if configHook.AlwaysRun {
		mergedHook.AlwaysRun = configHook.AlwaysRun
	}
	if configHook.PassFilenames != nil {
		mergedHook.PassFilenames = configHook.PassFilenames
	}
	if configHook.Files != "" {
		mergedHook.Files = configHook.Files
	}
	if configHook.ExcludeRegex != "" {
		mergedHook.ExcludeRegex = configHook.ExcludeRegex
	}
	if len(configHook.Types) > 0 {
		mergedHook.Types = configHook.Types
	}
	if len(configHook.TypesOr) > 0 {
		mergedHook.TypesOr = configHook.TypesOr
	}
	if len(configHook.ExcludeTypes) > 0 {
		mergedHook.ExcludeTypes = configHook.ExcludeTypes
	}
	if len(configHook.Stages) > 0 {
		mergedHook.Stages = configHook.Stages
	}
	if configHook.Verbose {
		mergedHook.Verbose = configHook.Verbose
	}

	return mergedHook, nil
}

// CheckRepositoriesReady checks if all repositories are cloned and environments are set up
// This is used by the run command to determine if install-hooks needs to be called
func (c *InstallHooksCommand) CheckRepositoriesReady(
	cfg *config.Config,
	repoManager *repository.Manager,
) bool {
	if len(cfg.Repos) == 0 {
		return true
	}

	// Check if all repositories are ready
	for _, repo := range cfg.Repos {
		// Skip local and meta repos
		if repo.Repo == LocalRepo || repo.Repo == MetaRepo {
			continue
		}

		// Check if repository is already cloned and environments are set up
		if !c.isRepositoryFullyPrepared(repo, repoManager) {
			return false
		}
	}

	return true
}

// Run executes the install-hooks command
func (c *InstallHooksCommand) Run(args []string) int {
	var opts InstallHooksOptions
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

	if validateErr := c.validateEnvironment(&opts); validateErr != nil {
		fmt.Printf("Error: %v\n", validateErr)
		return 1
	}

	cfg, repoManager, err := c.loadConfigAndInitManager(&opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}
	defer func() {
		if err := repoManager.Close(); err != nil {
			// Log error but don't fail since we're already returning
		}
	}()

	if err := c.prepareAllRepositories(cfg, repoManager); err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	return 0
}

// InstallHooksCommandFactory creates a new install-hooks command instance
func InstallHooksCommandFactory() (cli.Command, error) {
	return &InstallHooksCommand{}, nil
}
