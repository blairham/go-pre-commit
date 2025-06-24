package commands

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/pkg/hook"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
	"github.com/blairham/go-pre-commit/pkg/hook/formatting"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// RunCommand handles the run command functionality
type RunCommand struct{}

// setEnvVar sets an environment variable with error checking
func setEnvVar(key, value string, verbose bool) {
	if err := os.Setenv(key, value); err != nil && verbose {
		fmt.Printf("Warning: failed to set environment variable %s: %v\n", key, err)
	}
}

// RunOptions holds command-line options for the run command
type RunOptions struct {
	Config                     string        `long:"config"                        description:"Path to config file"                              short:"c" default:".pre-commit-config.yaml"`
	HookStage                  string        `long:"hook-stage"                    description:"Hook stage to run"                                          default:"pre-commit"`
	FromRef                    string        `long:"from-ref"                      description:"From ref for diff (alias: --source)"              short:"s"`
	ToRef                      string        `long:"to-ref"                        description:"To ref for diff (alias: --origin)"                short:"o"`
	RemoteName                 string        `long:"remote-name"                   description:"Remote name used by git push"`
	RemoteURL                  string        `long:"remote-url"                    description:"Remote url used by git push"`
	LocalBranch                string        `long:"local-branch"                  description:"Local branch name"`
	RemoteBranch               string        `long:"remote-branch"                 description:"Remote branch name"`
	CommitMsgFilename          string        `long:"commit-msg-filename"           description:"Filename to check when running during commit-msg"`
	PrepareCommitMessageSource string        `long:"prepare-commit-message-source" description:"Source of the commit message"`
	CommitObjectName           string        `long:"commit-object-name"            description:"Commit object name"`
	CheckoutType               string        `long:"checkout-type"                 description:"Checkout type (0=file, 1=branch)"`
	IsSquashMerge              string        `long:"is-squash-merge"               description:"Whether merge was a squash merge"`
	RewriteCommand             string        `long:"rewrite-command"               description:"Command that invoked the rewrite"`
	PreRebaseUpstream          string        `long:"pre-rebase-upstream"           description:"Upstream from which series was forked"`
	PreRebaseBranch            string        `long:"pre-rebase-branch"             description:"Branch being rebased"`
	Color                      string        `long:"color"                         description:"Whether to use color in output"                             default:"auto"                    choice:"auto"`
	Files                      []string      `long:"files"                         description:"Specific filenames to run hooks on"`
	Timeout                    time.Duration `long:"timeout"                       description:"Hook execution timeout (e.g. 30s, 5m)"                      default:"60s"`
	Parallel                   int           `long:"jobs"                          description:"Number of hooks to run in parallel"               short:"j" default:"1"`
	AllFiles                   bool          `long:"all-files"                     description:"Run on all files in the repository"               short:"a"`
	Verbose                    bool          `long:"verbose"                       description:"Verbose output"                                   short:"v"`
	ShowDiff                   bool          `long:"show-diff-on-failure"          description:"Show diff on failure"`
	Help                       bool          `long:"help"                          description:"Show this help message"                           short:"h"`
}

// Help returns the help text for the run command
func (c *RunCommand) Help() string {
	var opts RunOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--all-files] [--files FILES] [--config CONFIG] [--verbose] [--show-diff-on-failure] [--hook-stage HOOK_STAGE] [--from-ref FROM_REF] [--to-ref TO_REF] [--jobs JOBS] [--timeout TIMEOUT] [--color {auto,always,never}] [hook_id [hook_id ...]]"

	helpText := `usage: pre-commit run ` + parser.Usage + `

Run hooks.

optional arguments:
  -h, --help            show this help message and exit
  -a, --all-files       run on all files in the repository
  --files FILES         specific filenames to run hooks on
  -c, --config CONFIG   path to config file
  -v, --verbose         verbose output
  --show-diff-on-failure
                        show diff on failure
  --hook-stage HOOK_STAGE
                        the stage during which the hook is fired. One of
                        commit-msg, manual, post-checkout, post-commit, post-
                        merge, post-rewrite, pre-commit (default), pre-merge-
                        commit, pre-push, pre-rebase, prepare-commit-msg
  --from-ref FROM_REF   (for usage with --to-ref) run against the files
                        changed running git diff --name-only FROM_REF...TO_REF
  --to-ref TO_REF       (for usage with --from-ref) run against the files
                        changed running git diff --name-only FROM_REF...TO_REF
  --remote-name REMOTE_NAME
                        remote name used by git push
  --remote-url REMOTE_URL
                        remote url used by git push
  --local-branch LOCAL_BRANCH
                        local branch name
  --remote-branch REMOTE_BRANCH
                        remote branch name
  -j, --jobs JOBS       number of hooks to run in parallel
  --timeout TIMEOUT     hook execution timeout (e.g. 30s, 5m)
  --color {auto,always,never}
                        whether to use color in output (default: auto)
`

	return helpText
}

// Synopsis returns a short description of the run command
func (c *RunCommand) Synopsis() string {
	return "Run hooks on files"
}

// RunCommandFactory creates a new run command instance
func RunCommandFactory() (cli.Command, error) {
	return &RunCommand{}, nil
}

// Run executes the run command
func (c *RunCommand) Run(args []string) int {
	// Parse and validate arguments
	opts, remainingArgs, exitCode := c.parseAndValidateRunArgs(args)
	if exitCode != -1 {
		return exitCode
	}

	// Validate options
	if err := c.validateRunOptions(opts); err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Set environment variables for hooks
	env := c.setEnvironmentVariables(opts)

	// Initialize git repository and configuration
	repo, cfg, err := c.initializeGitAndConfig(opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Initialize repository manager
	repoManager, err := c.initializeRepoManager(opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}
	defer func() {
		if closeErr := repoManager.Close(); closeErr != nil {
			if opts.Verbose {
				fmt.Printf("Warning: failed to close repository manager: %v\n", closeErr)
			}
		}
	}()

	// Ensure all repositories are cloned and environments are set up
	// If not ready, run install-hooks first
	installHooksCmd := &InstallHooksCommand{}
	if !installHooksCmd.CheckRepositoriesReady(cfg, repoManager, opts.Verbose) {
		if opts.Verbose {
			fmt.Println("Repositories not ready, running install-hooks first...")
		}

		// Create install-hooks options based on run options
		installOpts := &InstallHooksOptions{
			Config:  opts.Config,
			Verbose: opts.Verbose,
		}

		if prepareErr := installHooksCmd.prepareAllRepositories(cfg, installOpts, repoManager); prepareErr != nil {
			fmt.Printf("Error preparing repositories and environments: %v\n", prepareErr)
			return 1
		}
	}

	// Set up stashing if needed
	stashInfo, _, err := c.setupStashingIfNeeded(repo, opts)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Get files to process (after potential stashing)
	files, err := c.getFilesToProcess(opts, repo)
	if err != nil {
		// Clean up stash if there was an error
		if stashInfo != nil {
			repo.CleanupStash(stashInfo)
		}
		return 1
	}

	// Create execution context
	ctx := c.createExecutionContext(cfg, files, repo, opts, env, remainingArgs, repoManager)

	// Execute hooks and handle results
	return c.executeHooksAndHandleResults(ctx, repo, stashInfo, opts)
}

// setEnvironmentVariables sets up environment variables that hooks expect
func (c *RunCommand) setEnvironmentVariables(opts *RunOptions) map[string]string {
	env := make(map[string]string)

	// Set PRE_COMMIT flag
	env["PRE_COMMIT"] = "1"
	setEnvVar("PRE_COMMIT", "1", opts.Verbose)

	// Set hook stage
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_HOOK_STAGE", opts.HookStage, opts.Verbose)

	// Set pre-push related variables
	if opts.FromRef != "" && opts.ToRef != "" {
		env["PRE_COMMIT_FROM_REF"] = opts.FromRef
		env["PRE_COMMIT_TO_REF"] = opts.ToRef
		env["PRE_COMMIT_ORIGIN"] = opts.FromRef // Legacy name
		env["PRE_COMMIT_SOURCE"] = opts.ToRef   // Legacy name

		setEnvVar("PRE_COMMIT_FROM_REF", opts.FromRef, opts.Verbose)
		setEnvVar("PRE_COMMIT_TO_REF", opts.ToRef, opts.Verbose)
		setEnvVar("PRE_COMMIT_ORIGIN", opts.FromRef, opts.Verbose)
		setEnvVar("PRE_COMMIT_SOURCE", opts.ToRef, opts.Verbose)
	}

	// Set other variables using helper
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_REMOTE_NAME", opts.RemoteName, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_REMOTE_URL", opts.RemoteURL, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_LOCAL_BRANCH", opts.LocalBranch, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_REMOTE_BRANCH", opts.RemoteBranch, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_COMMIT_MSG_FILENAME", opts.CommitMsgFilename, opts.Verbose)
	c.setEnvIfNotEmpty(
		env,
		"PRE_COMMIT_COMMIT_MSG_SOURCE",
		opts.PrepareCommitMessageSource,
		opts.Verbose,
	)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_COMMIT_OBJECT_NAME", opts.CommitObjectName, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_CHECKOUT_TYPE", opts.CheckoutType, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_IS_SQUASH_MERGE", opts.IsSquashMerge, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_REWRITE_COMMAND", opts.RewriteCommand, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_PRE_REBASE_UPSTREAM", opts.PreRebaseUpstream, opts.Verbose)
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_PRE_REBASE_BRANCH", opts.PreRebaseBranch, opts.Verbose)

	return env
}

// setEnvIfNotEmpty sets an environment variable if the value is not empty
func (c *RunCommand) setEnvIfNotEmpty(env map[string]string, key, value string, verbose bool) {
	if value != "" {
		env[key] = value
		setEnvVar(key, value, verbose)
	}
}

// getFilesToProcess determines which files to run hooks against based on options
func (c *RunCommand) getFilesToProcess(
	opts *RunOptions,
	repo *git.Repository,
) ([]string, error) {
	if opts.AllFiles {
		return repo.GetAllFiles()
	}

	if len(opts.Files) > 0 {
		// Validate that specified files exist
		return c.validateFiles(opts.Files), nil
	}

	if opts.FromRef != "" && opts.ToRef != "" {
		return repo.GetChangedFiles(opts.FromRef, opts.ToRef)
	}

	// Handle different hook stages
	switch opts.HookStage {
	case hookTypePreCommit:
		// Default: get staged files
		return c.handlePreCommitStage(repo, opts)

	case hookTypePrePush:
		// For pre-push, we need files in commits being pushed
		return c.handlePrePushStage(repo, opts)

	case hookTypeCommitMsg, hookTypePrepareCommit:
		// For commit message hooks, no files needed (they work on the message)
		return []string{}, nil

	case hookTypePostCheckout, hookTypePostCommit, hookTypePostMerge, hookTypePostRewrite:
		// For post hooks, get all files by default
		return repo.GetAllFiles()

	case hookTypePreRebase:
		// For pre-rebase, get all files
		return repo.GetAllFiles()

	default:
		// Default behavior for unknown stages
		fmt.Printf("Warning: Unknown hook stage '%s', using staged files\n", opts.HookStage)
		return repo.GetStagedFiles()
	}
}

// validateFiles checks that specified files exist and returns valid ones
func (c *RunCommand) validateFiles(files []string) []string {
	var validFiles []string
	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			validFiles = append(validFiles, file)
		} else {
			fmt.Printf("Warning: file not found: %s\n", file)
		}
	}
	return validFiles
}

// handlePreCommitStage handles file processing for pre-commit hooks
func (c *RunCommand) handlePreCommitStage(
	repo *git.Repository,
	opts *RunOptions,
) ([]string, error) {
	// Get staged files
	files, err := repo.GetStagedFiles()
	if err != nil {
		return nil, err
	}

	// Check for merge conflicts
	if c.hasUnmergedPaths(repo) {
		fmt.Println("Error: Unmerged files. Resolve before committing.")
		return nil, fmt.Errorf("unmerged files detected")
	}

	// Check if config file itself is unstaged
	if c.hasUnstagedConfig(repo, opts.Config) {
		fmt.Printf("Error: Your pre-commit configuration is unstaged: %s\n", opts.Config)
		fmt.Printf("`git add %s` to fix this.\n", opts.Config)
		return nil, fmt.Errorf("unstaged config file")
	}

	return files, nil
}

// handlePrePushStage handles file processing for pre-push hooks
func (c *RunCommand) handlePrePushStage(repo *git.Repository, opts *RunOptions) ([]string, error) {
	// For pre-push, we need files in commits being pushed
	if opts.LocalBranch != "" && opts.RemoteBranch != "" {
		return repo.GetPushFiles(opts.LocalBranch, opts.RemoteBranch)
	}
	// Fallback to all files if branch info not available
	return repo.GetAllFiles()
}

// hasUnmergedPaths checks if there are unmerged files in the repository
func (c *RunCommand) hasUnmergedPaths(repo *git.Repository) bool {
	return repo.HasUnmergedFiles()
}

// hasUnstagedConfig checks if the configuration file has unstaged changes
func (c *RunCommand) hasUnstagedConfig(repo *git.Repository, configFile string) bool {
	return repo.HasUnstagedChangesForFile(configFile)
}

// getFileHash computes SHA256 hash of a file
func getFileHash(filePath string) (string, error) {
	// Basic path validation to address gosec G304
	if filePath == "" || strings.Contains(filePath, "..") {
		return "", fmt.Errorf("invalid file path: %s", filePath)
	}

	file, err := os.Open(filePath) // #nosec G304 -- path is validated above
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// getStagedFileHash computes SHA256 hash of a file's staged content
func getStagedFileHash(repo *git.Repository, filePath string) (string, error) {
	content, err := repo.GetStagedFileContent(filePath)
	if err != nil {
		return "", err
	}

	hash := sha256.New()
	hash.Write(content)
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// shouldUseColor determines if color output should be enabled
func shouldUseColor(colorMode string) bool {
	switch colorMode {
	case "always":
		return true
	case "never":
		return false
	case "auto":
		// Check if output is to a terminal
		if os.Getenv("NO_COLOR") != "" {
			return false
		}
		// Simple check - this could be enhanced to detect terminal capabilities
		return os.Getenv("TERM") != ""
	default:
		return false
	}
}

// handleStashing manages stashing of unstaged changes when needed
func (c *RunCommand) handleStashing(
	repo *git.Repository,
	opts *RunOptions,
	cacheDir string,
) (*git.StashInfo, error) {
	hasUnstaged, err := repo.HasUnstagedChanges()
	if err != nil {
		return nil, fmt.Errorf("failed to check for unstaged changes: %w", err)
	}

	if !hasUnstaged {
		return nil, ErrNoStashRequired
	}

	if shouldUseColor(opts.Color) {
		fmt.Printf("\033[43m\033[30m[WARNING]\033[0m Unstaged files detected.\n")
	} else {
		fmt.Println("[WARNING] Unstaged files detected.")
	}

	stashInfo, err := repo.StashUnstagedChanges(cacheDir)
	if err != nil {
		if errors.Is(err, git.ErrNoUnstagedChanges) {
			// This is normal when there are no unstaged changes
			return nil, ErrNoStashRequired
		}
		return nil, fmt.Errorf("failed to stash unstaged changes: %w", err)
	}

	if stashInfo != nil {
		fmt.Printf("[INFO] Stashing unstaged files to %s.\n", stashInfo.PatchFile)
	}

	return stashInfo, nil
}

// handleStashRestoration manages the restoration of stashed changes after hook execution
func (c *RunCommand) handleStashRestoration(
	repo *git.Repository,
	stashInfo *git.StashInfo,
	opts *RunOptions,
) error {
	// Check if any hooks modified files by examining the working directory
	hooksModifiedFiles := c.checkIfHooksModifiedFiles(repo, stashInfo)

	if !hooksModifiedFiles {
		// No file modifications by hooks - simply restore stash
		return repo.RestoreFromStash(stashInfo)
	}

	// Check if stash can be applied without conflicts
	canApply, err := repo.CanApplyStash(stashInfo)
	if err != nil {
		return fmt.Errorf("failed to check stash conflicts: %w", err)
	}

	if !canApply {
		return c.handleStashConflicts(repo, stashInfo, opts)
	}

	// No conflicts - apply stash to keep both hook changes and user changes
	return repo.RestoreFromStash(stashInfo)
}

// checkIfHooksModifiedFiles checks if any hooks modified the files
func (c *RunCommand) checkIfHooksModifiedFiles(
	repo *git.Repository,
	stashInfo *git.StashInfo,
) bool {
	for _, file := range stashInfo.Files {
		// Get current file hash
		currentHash, err := getFileHash(filepath.Join(repo.Root, file))
		if err != nil {
			continue // Skip files that can't be read
		}

		// Get staged file hash
		stagedHash, err := getStagedFileHash(repo, file)
		if err != nil {
			continue // Skip files that can't be read
		}

		// If current file differs from staged file, hooks modified it
		if currentHash != stagedHash {
			return true
		}
	}
	return false
}

// handleStashConflicts handles conflicts when restoring stashed changes
func (c *RunCommand) handleStashConflicts(
	repo *git.Repository,
	stashInfo *git.StashInfo,
	opts *RunOptions,
) error {
	// Conflicts detected - rollback hook changes and restore original state
	if shouldUseColor(opts.Color) {
		fmt.Printf(
			"\033[43m\033[30m[WARNING]\033[0m Stashed changes conflicted with hook auto-fixes... Rolling back fixes.\n",
		)
	} else {
		fmt.Println("[WARNING] Stashed changes conflicted with hook auto-fixes... Rolling back fixes.")
	}
	fmt.Print("..")

	// Reset to original staged content
	if err := repo.ResetToStaged(); err != nil {
		return fmt.Errorf("failed to reset to staged content: %w", err)
	}

	// Restore original unstaged changes
	if err := repo.RestoreFromStash(stashInfo); err != nil {
		return fmt.Errorf("failed to restore stashed changes: %w", err)
	}

	fmt.Printf("\n[INFO] Restored changes from %s.\n", stashInfo.PatchFile)
	return fmt.Errorf("conflicts detected during stash restoration")
}

// ErrNoStashRequired indicates that no stash is required
var ErrNoStashRequired = errors.New("no stash required")

// Helper functions to reduce cognitive complexity in RunCommand.Run

func (c *RunCommand) parseAndValidateRunArgs(args []string) (*RunOptions, []string, int) {
	var opts RunOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[OPTIONS] [hook-id]"

	remainingArgs, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) {
			if flagsErr.Type == flags.ErrHelp {
				return &opts, remainingArgs, 0
			}
		}
		fmt.Printf("Error parsing flags: %v\n", err)
		return &opts, remainingArgs, 1
	}

	return &opts, remainingArgs, -1 // Continue processing
}

func (c *RunCommand) validateRunOptions(opts *RunOptions) error {
	// Validate ref arguments
	if opts.FromRef != "" && opts.ToRef == "" {
		return fmt.Errorf("--to-ref is required when --from-ref is specified")
	}
	if opts.ToRef != "" && opts.FromRef == "" {
		return fmt.Errorf("--from-ref is required when --to-ref is specified")
	}

	// Validate mutually exclusive options
	exclusiveCount := 0
	if opts.AllFiles {
		exclusiveCount++
	}
	if len(opts.Files) > 0 {
		exclusiveCount++
	}
	if opts.FromRef != "" && opts.ToRef != "" {
		exclusiveCount++
	}

	if exclusiveCount > 1 {
		return fmt.Errorf("--all-files, --files, and --from-ref/--to-ref are mutually exclusive")
	}

	return nil
}

func (c *RunCommand) initializeGitAndConfig(
	opts *RunOptions,
) (*git.Repository, *config.Config, error) {
	// Find git repository
	repo, err := git.NewRepository("")
	if err != nil {
		return nil, nil, fmt.Errorf("not in a git repository: %w", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig(opts.Config)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf(
				"config file not found: %s. Run 'pre-commit sample-config' first",
				opts.Config,
			)
		}
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, nil, fmt.Errorf("invalid configuration: %w", validateErr)
	}

	return repo, cfg, nil
}

func (c *RunCommand) initializeRepoManager(opts *RunOptions) (*repository.Manager, error) {
	repoManager, err := repository.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository manager: %w", err)
	}

	// Mark this config as used in the database so gc knows it's active
	if markErr := repoManager.MarkConfigUsed(opts.Config); markErr != nil {
		// Don't fail the command if this fails, just warn
		if opts.Verbose {
			fmt.Printf("Warning: failed to mark config as used: %v\n", markErr)
		}
	}

	return repoManager, nil
}

func (c *RunCommand) setupStashingIfNeeded(
	repo *git.Repository,
	opts *RunOptions,
) (*git.StashInfo, string, error) {
	isStashEnabled := opts.HookStage == hookTypePreCommit || opts.HookStage == ""
	if !isStashEnabled {
		return nil, "", nil
	}

	// Get cache directory using proper environment variable resolution
	cacheDir := getCacheDirectory()

	// Check for unstaged changes and stash them if needed
	stashInfo, err := c.handleStashing(repo, opts, cacheDir)
	if err != nil && !errors.Is(err, ErrNoStashRequired) {
		return nil, "", err
	}

	return stashInfo, cacheDir, nil
}

func (c *RunCommand) createExecutionContext(
	cfg *config.Config,
	files []string,
	repo *git.Repository,
	opts *RunOptions,
	env map[string]string,
	hookIDs []string,
	repoManager *repository.Manager,
) *execution.Context {
	return &execution.Context{
		Config:      cfg,
		Files:       files,
		AllFiles:    opts.AllFiles,
		Verbose:     opts.Verbose,
		ShowDiff:    opts.ShowDiff,
		RepoRoot:    repo.Root,
		HookStage:   opts.HookStage,
		Environment: env,
		HookIDs:     hookIDs,
		Parallel:    opts.Parallel,
		Timeout:     opts.Timeout,
		Color:       opts.Color,
		RepoManager: repoManager,
	}
}

func (c *RunCommand) executeHooksAndHandleResults(
	ctx *execution.Context,
	repo *git.Repository,
	stashInfo *git.StashInfo,
	opts *RunOptions,
) int {
	// Create orchestrator and run hooks directly
	orchestrator := hook.NewOrchestrator(ctx)
	results, err := orchestrator.RunHooks(context.Background())
	if err != nil {
		// Clean up stash on error
		if stashInfo != nil {
			repo.CleanupStash(stashInfo)
		}
		fmt.Printf("Error: failed to run hooks: %v\n", err)
		return 1
	}

	// Print results using the new formatting package
	formatter := formatting.NewFormatter(opts.Color, opts.Verbose)
	formatter.PrintResults(results)

	// Handle stash restoration and conflict detection
	if stashInfo != nil {
		if err := c.handleStashRestoration(repo, stashInfo, opts); err != nil {
			if err.Error() == "conflicts detected during stash restoration" {
				repo.CleanupStash(stashInfo)
				return 1 // Prevent commit due to conflicts
			}
			fmt.Printf("Error: %v\n", err)
			repo.CleanupStash(stashInfo)
			return 1
		}
	}

	// Check if any hooks failed
	for _, result := range results {
		if !result.Success {
			return 1
		}
	}

	return 0
}
