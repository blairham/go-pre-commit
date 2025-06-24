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

// DoctorCommand handles the doctor command functionality
type DoctorCommand struct{}

// DoctorOptions holds command-line options for the doctor command
type DoctorOptions struct {
	Config  string `short:"c" long:"config"  description:"Path to config file"               default:".pre-commit-config.yaml"`
	Fix     bool   `          long:"fix"     description:"Attempt to fix any problems found"`
	Verbose bool   `short:"v" long:"verbose" description:"Verbose output"`
	Help    bool   `short:"h" long:"help"    description:"Show this help message"`
}

// Help returns the help text for the doctor command
func (c *DoctorCommand) Help() string {
	var opts DoctorOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	formatter := &HelpFormatter{
		Command:     "doctor",
		Description: "Check and repair pre-commit environment health.",
		Examples: []Example{
			{Command: "pre-commit doctor", Description: "Check environment health"},
			{Command: "pre-commit doctor --fix", Description: "Check and attempt to fix problems"},
			{
				Command:     "pre-commit doctor --verbose",
				Description: "Show detailed diagnostic information",
			},
		},
		Notes: []string{
			"This command checks that all hook environments are properly set up and working.",
			"It can detect and fix common issues like corrupted language environments,",
			"missing dependencies, or outdated environments.",
			"",
			"Exit codes:",
			"  0: No problems found or all problems fixed",
			"  1: Problems found and not fixed (use --fix to attempt repair)",
			"  2: Error running doctor command",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the doctor command
func (c *DoctorCommand) Synopsis() string {
	return "Check and repair environment health"
}

// Run executes the doctor command with the given arguments
func (c *DoctorCommand) Run(args []string) int {
	var opts DoctorOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = OptionsUsage

	_, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return 2
	}

	// Load configuration and create repository manager
	cfg, repoMgr, err := c.initializeDoctorEnvironment(opts.Config)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 2
	}

	fmt.Printf("🔍 Running pre-commit environment health check...\n\n")

	var problems []string
	var warnings []string

	// Check repositories and hooks
	repoProblems, repoWarnings := c.checkRepositoriesAndHooks(cfg, repoMgr, opts)
	problems = append(problems, repoProblems...)
	warnings = append(warnings, repoWarnings...)

	// Check Git repository status
	gitProblems := c.checkGitRepository(opts.Verbose)
	problems = append(problems, gitProblems...)

	// Check cache directory permissions
	cacheProblems := c.checkCacheDirectory(opts.Verbose)
	problems = append(problems, cacheProblems...)

	// Print results and return appropriate exit code
	return c.printResultsAndExit(problems, warnings, opts.Fix)
}

// initializeDoctorEnvironment sets up configuration and repository manager
func (c *DoctorCommand) initializeDoctorEnvironment(
	configPath string,
) (*config.Config, *repository.Manager, error) {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	// Create repository manager
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil, nil, fmt.Errorf("creating repository manager: %w", err)
	}

	return cfg, repoMgr, nil
}

// checkRepositoriesAndHooks checks all repositories and their hooks
func (c *DoctorCommand) checkRepositoriesAndHooks(
	cfg *config.Config,
	repoMgr *repository.Manager,
	opts DoctorOptions,
) ([]string, []string) {
	var problems []string
	var warnings []string

	for _, repo := range cfg.Repos {
		if opts.Verbose {
			fmt.Printf("Checking repository: %s\n", repo.Repo)
		}

		repoProblems, repoWarnings := c.checkSingleRepository(repo, repoMgr, opts)
		problems = append(problems, repoProblems...)
		warnings = append(warnings, repoWarnings...)
	}

	return problems, warnings
}

// checkSingleRepository checks a single repository and its hooks
func (c *DoctorCommand) checkSingleRepository(
	repo config.Repo,
	repoMgr *repository.Manager,
	opts DoctorOptions,
) ([]string, []string) {
	var problems []string
	var warnings []string

	// Get repository path
	repoPath, repoProblems := c.getRepositoryPath(repo, repoMgr, opts.Verbose)
	problems = append(problems, repoProblems...)

	// Check each hook in the repository
	for _, hook := range repo.Hooks {
		hookProblems, hookWarnings := c.checkSingleHook(hook, repoPath, repo.Repo, repoMgr, opts)
		problems = append(problems, hookProblems...)
		warnings = append(warnings, hookWarnings...)
	}

	return problems, warnings
}

// getRepositoryPath retrieves and validates the repository path
func (c *DoctorCommand) getRepositoryPath(
	repo config.Repo,
	repoMgr *repository.Manager,
	verbose bool,
) (string, []string) {
	var problems []string

	if repo.Repo == LocalRepo || repo.Repo == MetaRepo {
		return "", problems
	}

	repoPath, repoErr := repoMgr.CloneOrUpdateRepo(context.Background(), repo)
	if repoErr != nil {
		problems = append(
			problems,
			fmt.Sprintf("Cannot access repository %s: %v", repo.Repo, repoErr),
		)
		return "", problems
	}

	if verbose {
		fmt.Printf("  ✓ Repository accessible at %s\n", repoPath)
	}

	return repoPath, problems
}

// checkSingleHook checks a single hook's environment and configuration
func (c *DoctorCommand) checkSingleHook(
	hook config.Hook,
	repoPath string,
	repoURL string,
	repoMgr *repository.Manager,
	opts DoctorOptions,
) ([]string, []string) {
	var problems []string
	var warnings []string

	if opts.Verbose {
		fmt.Printf("  Checking hook: %s (language: %s)\n", hook.ID, hook.Language)
	}

	// Check environment health
	envProblems := c.checkHookEnvironment(hook, repoPath, repoURL, repoMgr, opts)
	problems = append(problems, envProblems...)

	// Check configuration issues
	configWarnings := c.checkHookConfiguration(hook)
	warnings = append(warnings, configWarnings...)

	return problems, warnings
}

// checkHookEnvironment checks the health of a hook's language environment
func (c *DoctorCommand) checkHookEnvironment(
	hook config.Hook,
	repoPath string,
	repoURL string,
	repoMgr *repository.Manager,
	opts DoctorOptions,
) []string {
	var problems []string

	languageVersion := hook.LanguageVersion
	if languageVersion == "" {
		languageVersion = "default"
	}

	var healthErr error
	if repoPath != "" {
		healthErr = repoMgr.CheckEnvironmentHealthWithRepo(hook.Language, languageVersion, repoPath)
	} else {
		fmt.Printf("  ⚠️  Hook %s has no repository context, skipping environment health check\n", hook.ID)
		return problems
	}

	if healthErr != nil {
		problems = append(
			problems,
			fmt.Sprintf("Hook %s environment unhealthy: %v", hook.ID, healthErr),
		)

		if opts.Fix {
			problems = c.attemptEnvironmentFix(
				hook,
				repoPath,
				repoURL,
				repoMgr,
				languageVersion,
				problems,
			)
		}
	} else if opts.Verbose {
		fmt.Printf("    ✓ Environment healthy\n")
	}

	return problems
}

// attemptEnvironmentFix attempts to fix an unhealthy environment
func (c *DoctorCommand) attemptEnvironmentFix(
	hook config.Hook,
	repoPath string,
	repoURL string,
	repoMgr *repository.Manager,
	languageVersion string,
	problems []string,
) []string {
	fmt.Printf("  🔧 Attempting to fix environment for %s...\n", hook.ID)

	var fixErr error
	if repoPath != "" {
		fixErr = repoMgr.RebuildEnvironmentWithRepoInfo(
			hook.Language,
			languageVersion,
			repoPath,
			repoURL,
		)
	} else {
		fmt.Printf("  ⚠️  Hook %s has no repository context, cannot rebuild environment\n", hook.ID)
		return problems
	}

	if fixErr != nil {
		problems = append(
			problems,
			fmt.Sprintf("Failed to rebuild environment for %s: %v", hook.ID, fixErr),
		)
	} else {
		fmt.Printf("  ✅ Successfully rebuilt environment for %s\n", hook.ID)
	}

	return problems
}

// checkHookConfiguration checks for common hook configuration issues
func (c *DoctorCommand) checkHookConfiguration(hook config.Hook) []string {
	var warnings []string

	// Check for common configuration issues
	if hook.Entry == "" && hook.Language != "meta" && hook.Language != "fail" {
		warnings = append(warnings, fmt.Sprintf("Hook %s has no entry command specified", hook.ID))
	}

	// Check for deprecated configurations
	if hook.Language == "python2" {
		warnings = append(
			warnings,
			fmt.Sprintf("Hook %s uses deprecated python2 language", hook.ID),
		)
	}

	return warnings
}

// checkGitRepository checks Git repository status
func (c *DoctorCommand) checkGitRepository(verbose bool) []string {
	var problems []string

	if verbose {
		fmt.Printf("\nChecking Git repository...\n")
	}

	if _, statErr := os.Stat(".git"); os.IsNotExist(statErr) {
		problems = append(problems, "Not in a Git repository")
	} else if verbose {
		fmt.Printf("  ✓ Git repository detected\n")
	}

	return problems
}

// checkCacheDirectory checks cache directory permissions
func (c *DoctorCommand) checkCacheDirectory(verbose bool) []string {
	var problems []string

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return problems
	}

	cacheDir := filepath.Join(homeDir, ".cache", "pre-commit")
	if verbose {
		fmt.Printf("Checking cache directory: %s\n", cacheDir)
	}

	// Check if cache directory is writable
	testFile := filepath.Join(cacheDir, ".test")
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		problems = append(problems, fmt.Sprintf("Cache directory not writable: %v", err))
	} else if err := os.WriteFile(testFile, []byte("test"), 0o600); err != nil {
		problems = append(problems, fmt.Sprintf("Cache directory not writable: %v", err))
	} else {
		if removeErr := os.Remove(testFile); removeErr != nil && verbose {
			fmt.Printf("⚠️  Warning: failed to clean up test file: %v\n", removeErr)
		}
		if verbose {
			fmt.Printf("  ✓ Cache directory writable\n")
		}
	}

	return problems
}

// printResultsAndExit prints the final results and returns appropriate exit code
func (c *DoctorCommand) printResultsAndExit(problems, warnings []string, fix bool) int {
	fmt.Printf("\n📋 Health Check Results:\n")

	if len(problems) == 0 && len(warnings) == 0 {
		fmt.Printf("✅ All checks passed! Your pre-commit environment is healthy.\n")
		return 0
	}

	if len(warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings:\n")
		for _, warning := range warnings {
			fmt.Printf("  • %s\n", warning)
		}
	}

	if len(problems) > 0 {
		fmt.Printf("\n❌ Problems found:\n")
		for _, problem := range problems {
			fmt.Printf("  • %s\n", problem)
		}

		if !fix {
			fmt.Printf("\nRun 'pre-commit doctor --fix' to attempt automatic repairs.\n")
		}
		return 1
	}

	if len(warnings) > 0 && len(problems) == 0 {
		fmt.Printf("\nNo critical problems found, but please review the warnings above.\n")
		return 0
	}

	return 0
}

// DoctorCommandFactory creates a new doctor command instance
func DoctorCommandFactory() (cli.Command, error) {
	return &DoctorCommand{}, nil
}
