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
	"github.com/blairham/go-pre-commit/pkg/hook"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
	"github.com/blairham/go-pre-commit/pkg/hook/formatting"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// HookImplCommand handles the hook-impl command functionality
type HookImplCommand struct{}

// HookImplOptions holds command-line options for the hook-impl command
type HookImplOptions struct {
	Config              string `long:"config"                 description:"Path to config file"                      default:".pre-commit-config.yaml"`
	HookType            string `long:"hook-type"              description:"Type of hook being run"                                                     required:"true"`
	HookDir             string `long:"hook-dir"               description:"Directory where hooks are stored"`
	Color               string `long:"color"                  description:"Whether to use color in output"           default:"auto"                                    choice:"auto"`
	SkipOnMissingConfig bool   `long:"skip-on-missing-config" description:"Skip execution if config file is missing"`
	Verbose             bool   `long:"verbose"                description:"Verbose output"                                                                                           short:"v"`
	Help                bool   `long:"help"                   description:"Show this help message"                                                                                   short:"h"`
}

// Help returns the help text for the hook-impl command
func (c *HookImplCommand) Help() string {
	var opts HookImplOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[OPTIONS] [HOOK_ARGS...]"

	formatter := &HelpFormatter{
		Command:     "hook-impl",
		Description: "Internal command used by installed git hooks.",
		Examples: []Example{
			{
				Command:     "pre-commit hook-impl --hook-type pre-commit",
				Description: "Run pre-commit hooks (internal use)",
			},
		},
		Notes: []string{
			"positional arguments:",
			"  HOOK_ARGS             arguments passed to the hook",
			"",
			"This command is not intended to be called directly by users. It is invoked",
			"automatically by the git hooks that are installed by 'pre-commit install'.",
			"",
			"The hook implementation loads the configuration, determines which hooks should",
			"run for the current hook type, and executes them against the appropriate files.",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the hook-impl command
func (c *HookImplCommand) Synopsis() string {
	return "Internal hook implementation (not for direct use)"
}

// Run executes the hook-impl command
func (c *HookImplCommand) Run(args []string) int {
	opts, remaining, parseCode := c.parseArguments(args)
	if parseCode != -1 {
		return parseCode
	}

	if validationCode := c.validateOptions(opts); validationCode != -1 {
		return validationCode
	}

	c.logVerboseInfo(opts, remaining)

	if configValidationCode := c.validateConfigFile(opts); configValidationCode != -1 {
		return configValidationCode
	}

	cfg, repoManager, setupCode := c.setupConfigAndManager(opts)
	if setupCode != -1 {
		return setupCode
	}
	defer c.closeRepoManager(repoManager, opts.Verbose)

	repo, files, contextCode := c.setupExecutionContext(opts, remaining)
	if contextCode != -1 {
		return contextCode
	}

	return c.executeHooks(opts, remaining, cfg, repo, files)
}

// parseArguments parses command line arguments
func (c *HookImplCommand) parseArguments(args []string) (*HookImplOptions, []string, int) {
	var opts HookImplOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[OPTIONS] [HOOK_ARGS...]"

	remaining, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return nil, nil, 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return nil, nil, 1
	}

	return &opts, remaining, -1
}

// validateOptions validates the parsed options
func (c *HookImplCommand) validateOptions(opts *HookImplOptions) int {
	if opts.HookType == "" {
		fmt.Println("Error: --hook-type is required")
		return 1
	}

	return -1
}

// logVerboseInfo logs verbose information about the execution
func (c *HookImplCommand) logVerboseInfo(opts *HookImplOptions, remaining []string) {
	if opts.Verbose {
		fmt.Printf("Hook implementation running for: %s\n", opts.HookType)
		fmt.Printf("Config file: %s\n", opts.Config)
		if len(remaining) > 0 {
			fmt.Printf("Hook arguments: %v\n", remaining)
		}
	}
}

// validateConfigFile checks if the config file exists and is accessible
func (c *HookImplCommand) validateConfigFile(opts *HookImplOptions) int {
	if _, statErr := os.Stat(opts.Config); os.IsNotExist(statErr) {
		if opts.SkipOnMissingConfig {
			if opts.Verbose {
				fmt.Printf("Config file not found, skipping: %s\n", opts.Config)
			}
			return 0
		}
		fmt.Printf("Error: config file not found: %s\n", opts.Config)
		return 1
	}

	// Check if we're in a git repository
	if _, statErr := os.Stat(".git"); os.IsNotExist(statErr) {
		fmt.Println("Error: not in a git repository")
		return 1
	}

	return -1
}

// setupConfigAndManager loads configuration and initializes repository manager
func (c *HookImplCommand) setupConfigAndManager(
	opts *HookImplOptions,
) (*config.Config, *repository.Manager, int) {
	cfg, err := config.LoadConfig(opts.Config)
	if err != nil {
		fmt.Printf("Error: failed to load configuration: %v\n", err)
		return nil, nil, 1
	}

	if validateErr := cfg.Validate(); validateErr != nil {
		fmt.Printf("Error: invalid configuration: %v\n", validateErr)
		return nil, nil, 1
	}

	repoManager, err := repository.NewManager()
	if err != nil {
		fmt.Printf("Error: failed to initialize repository manager: %v\n", err)
		return nil, nil, 1
	}

	// Mark this config as used in the database so gc knows it's active
	if markErr := repoManager.MarkConfigUsed(opts.Config); markErr != nil && opts.Verbose {
		fmt.Printf("Warning: failed to mark config as used: %v\n", markErr)
	}

	return cfg, repoManager, -1
}

// closeRepoManager safely closes the repository manager
func (c *HookImplCommand) closeRepoManager(repoManager *repository.Manager, verbose bool) {
	if closeErr := repoManager.Close(); closeErr != nil && verbose {
		fmt.Printf("Warning: failed to close repository manager: %v\n", closeErr)
	}
}

// setupExecutionContext sets up the git repository and determines files to process
func (c *HookImplCommand) setupExecutionContext(
	opts *HookImplOptions,
	remaining []string,
) (*git.Repository, []string, int) {
	repo, err := git.NewRepository("")
	if err != nil {
		fmt.Printf("Error: not in a git repository: %v\n", err)
		return nil, nil, 1
	}

	files, err := c.getFilesForHookType(opts.HookType, repo, remaining)
	if err != nil {
		fmt.Printf("Error: failed to get files for hook type %s: %v\n", opts.HookType, err)
		return nil, nil, 1
	}

	return repo, files, -1
}

// executeHooks runs the hooks and returns the result code
func (c *HookImplCommand) executeHooks(
	opts *HookImplOptions,
	remaining []string,
	cfg *config.Config,
	repo *git.Repository,
	files []string,
) int {
	env := c.setupEnvironmentVariables(opts.HookType, remaining)

	// Create execution context directly for the orchestrator
	execCtx := &execution.Context{
		Config:      cfg,
		Files:       files,
		AllFiles:    false,
		Verbose:     opts.Verbose,
		ShowDiff:    false,
		RepoRoot:    repo.Root,
		HookStage:   opts.HookType,
		Environment: env,
		HookIDs:     nil,
		Parallel:    0,
		Timeout:     0,
		Color:       opts.Color,
	}

	// Create orchestrator and run hooks directly
	orchestrator := hook.NewOrchestrator(execCtx)
	results, err := orchestrator.RunHooks(context.Background())
	if err != nil {
		fmt.Printf("Error: failed to run hooks: %v\n", err)
		return 1
	}

	// Print results using the new formatting package
	formatter := formatting.NewFormatter(opts.Color, opts.Verbose)
	formatter.PrintResults(results)

	// Check if any hooks failed
	for _, result := range results {
		if !result.Success {
			return 1
		}
	}

	return 0
}

// getFilesForHookType determines which files to process based on the hook type
func (c *HookImplCommand) getFilesForHookType(
	hookType string,
	repo *git.Repository,
	args []string,
) ([]string, error) {
	switch hookType {
	case "pre-commit":
		return repo.GetStagedFiles()

	case "pre-push":
		// For pre-push hooks, we need to parse the pre-push arguments
		// Format: <local ref> <local sha1> <remote ref> <remote sha1>
		if len(args) >= 4 {
			localRef := args[0]
			remoteSha := args[3]
			if remoteSha == "0000000000000000000000000000000000000000" {
				// New branch, get all files
				return repo.GetAllFiles()
			}
			// Get files in commits being pushed
			return repo.GetChangedFiles(remoteSha, localRef)
		}
		return repo.GetAllFiles()

	case "commit-msg", "prepare-commit-msg":
		// Commit message hooks don't process files
		return []string{}, nil

	case "post-checkout":
		// Post-checkout hook gets previous HEAD, new HEAD, and branch flag
		// We'll get all files for now
		return repo.GetAllFiles()

	case "post-commit":
		// Post-commit processes the files in the commit
		return repo.GetCommitFiles("HEAD")

	case "post-merge":
		// Post-merge processes changed files
		return repo.GetChangedFiles("HEAD~1", "HEAD")

	case "post-rewrite":
		// Post-rewrite processes all files
		return repo.GetAllFiles()

	case "pre-rebase":
		// Pre-rebase processes all files
		return repo.GetAllFiles()

	default:
		fmt.Printf("Warning: Unknown hook type '%s', using all files\n", hookType)
		return repo.GetAllFiles()
	}
}

// setupEnvironmentVariables sets up environment variables for the hook type
func (c *HookImplCommand) setupEnvironmentVariables(
	hookType string,
	args []string,
) map[string]string {
	env := make(map[string]string)

	// Always set the PRE_COMMIT flag
	env["PRE_COMMIT"] = "1"
	env["PRE_COMMIT_HOOK_STAGE"] = hookType

	// Set hook-type specific environment variables
	c.setHookTypeSpecificEnvVars(env, hookType, args)

	return env
}

// setHookTypeSpecificEnvVars sets environment variables specific to each hook type
func (c *HookImplCommand) setHookTypeSpecificEnvVars(
	env map[string]string,
	hookType string,
	args []string,
) {
	switch hookType {
	case "pre-push":
		c.setPrePushEnvVars(env, args)
	case "commit-msg":
		c.setCommitMsgEnvVars(env, args)
	case "prepare-commit-msg":
		c.setPrepareCommitMsgEnvVars(env, args)
	case "post-checkout":
		c.setPostCheckoutEnvVars(env, args)
	case "post-rewrite":
		c.setPostRewriteEnvVars(env, args)
	case "pre-rebase":
		c.setPreRebaseEnvVars(env, args)
	}
}

// setPrePushEnvVars sets environment variables for pre-push hooks
func (c *HookImplCommand) setPrePushEnvVars(env map[string]string, args []string) {
	if len(args) >= 4 {
		env["PRE_COMMIT_FROM_REF"] = args[2] // remote ref
		env["PRE_COMMIT_TO_REF"] = args[0]   // local ref
		env["PRE_COMMIT_REMOTE_BRANCH"] = args[2]
		env["PRE_COMMIT_LOCAL_BRANCH"] = args[0]
	}
}

// setCommitMsgEnvVars sets environment variables for commit-msg hooks
func (c *HookImplCommand) setCommitMsgEnvVars(env map[string]string, args []string) {
	if len(args) >= 1 {
		env["PRE_COMMIT_COMMIT_MSG_FILENAME"] = args[0]
	}
}

// setPrepareCommitMsgEnvVars sets environment variables for prepare-commit-msg hooks
func (c *HookImplCommand) setPrepareCommitMsgEnvVars(env map[string]string, args []string) {
	if len(args) >= 1 {
		env["PRE_COMMIT_COMMIT_MSG_FILENAME"] = args[0]
	}
	if len(args) >= 2 {
		env["PRE_COMMIT_COMMIT_MSG_SOURCE"] = args[1]
	}
	if len(args) >= 3 {
		env["PRE_COMMIT_COMMIT_OBJECT_NAME"] = args[2]
	}
}

// setPostCheckoutEnvVars sets environment variables for post-checkout hooks
func (c *HookImplCommand) setPostCheckoutEnvVars(env map[string]string, args []string) {
	if len(args) >= 3 {
		env["PRE_COMMIT_CHECKOUT_TYPE"] = args[2]
	}
}

// setPostRewriteEnvVars sets environment variables for post-rewrite hooks
func (c *HookImplCommand) setPostRewriteEnvVars(env map[string]string, args []string) {
	if len(args) >= 1 {
		env["PRE_COMMIT_REWRITE_COMMAND"] = args[0]
	}
}

// setPreRebaseEnvVars sets environment variables for pre-rebase hooks
func (c *HookImplCommand) setPreRebaseEnvVars(env map[string]string, args []string) {
	if len(args) >= 1 {
		env["PRE_COMMIT_PRE_REBASE_UPSTREAM"] = args[0]
	}
	if len(args) >= 2 {
		env["PRE_COMMIT_PRE_REBASE_BRANCH"] = args[1]
	}
}

// HookImplCommandFactory creates a new hook-impl command instance
func HookImplCommandFactory() (cli.Command, error) {
	return &HookImplCommand{}, nil
}
