package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/mitchellh/cli"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/pkg/hook"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
	"github.com/blairham/go-pre-commit/pkg/hook/formatting"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// TryRepoCommand handles the try-repo command functionality
type TryRepoCommand struct{}

// TryRepoOptions holds command-line options for the try-repo command
type TryRepoOptions struct {
	Config            string   `long:"config"              description:"Path to config file"                                short:"c"`
	Ref               string   `long:"ref"                 description:"Manually select a rev to run against"`
	Rev               string   `long:"rev"                 description:"Alias for --ref"`
	Color             string   `long:"color"               description:"Whether to use color in output"                               choice:"auto" default:"auto"`
	Files             []string `long:"files"               description:"Specific filenames to run hooks on"`
	Verbose           bool     `long:"verbose"             description:"Verbose output"                                     short:"v"`
	AllFiles          bool     `long:"all-files"           description:"Run on all files in the repo"                       short:"a"`
	ShowDiffOnFailure bool     `long:"show-diff-on-failure" description:"When hooks fail, run git diff directly afterward"`
	FailFast          bool     `long:"fail-fast"           description:"Stop after the first failing hook"`
	HookStage         string   `long:"hook-stage"          description:"The stage during which the hook is fired"                     default:"pre-commit"`
	// Diff-based options
	FromRef string `long:"from-ref" short:"s" description:"(for usage with --to-ref) original ref in from_ref...to_ref diff"`
	ToRef   string `long:"to-ref"   short:"o" description:"(for usage with --from-ref) destination ref in from_ref...to_ref diff"`
	// Git hook-specific options
	RemoteBranch               string `long:"remote-branch"                description:"Remote branch ref used by git push"`
	LocalBranch                string `long:"local-branch"                 description:"Local branch ref used by git push"`
	RemoteName                 string `long:"remote-name"                  description:"Remote name used by git push"`
	RemoteURL                  string `long:"remote-url"                   description:"Remote url used by git push"`
	CommitMsgFilename          string `long:"commit-msg-filename"          description:"Filename to check when running during commit-msg"`
	PrepareCommitMessageSource string `long:"prepare-commit-message-source" description:"Source of the commit message"`
	CommitObjectName           string `long:"commit-object-name"           description:"Commit object name"`
	PreRebaseUpstream          string `long:"pre-rebase-upstream"          description:"The upstream from which the series was forked"`
	PreRebaseBranch            string `long:"pre-rebase-branch"            description:"The branch being rebased"`
	CheckoutType               string `long:"checkout-type"                description:"Indicates whether checkout was branch or file checkout"`
	IsSquashMerge              string `long:"is-squash-merge"              description:"During post-merge, indicates if merge was squash merge"`
	RewriteCommand             string `long:"rewrite-command"              description:"During post-rewrite, specifies command that invoked rewrite"`
	Help                       bool   `long:"help"                         description:"Show this help message"                             short:"h"`
	// Positional arguments
	Positional struct {
		Repo string `positional-arg-name:"repo" description:"Repository to source hooks from"`
		Hook string `positional-arg-name:"hook" description:"A single hook-id to run (optional)"`
	} `positional-args:"yes"`
}

// Help returns the help text for the try-repo command
func (c *TryRepoCommand) Help() string {
	var opts TryRepoOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "REPO [HOOK] [OPTIONS]"

	formatter := &HelpFormatter{
		Command:     "try-repo",
		Description: "Try the hooks in a repository, useful for developing new hooks.",
		Examples: []Example{
			{
				Command:     "pre-commit try-repo https://github.com/psf/black",
				Description: "Try hooks from remote repo",
			},
			{
				Command:     "pre-commit try-repo https://github.com/psf/black black",
				Description: "Try specific hook from remote repo (positional)",
			},
			{
				Command:     "pre-commit try-repo ../my-hooks-repo --ref main",
				Description: "Try hooks from local repo with specific ref",
			},
			{
				Command:     "pre-commit try-repo https://github.com/pre-commit/mirrors-eslint --all-files",
				Description: "Run on all files",
			},
			{
				Command:     "pre-commit try-repo . --files src/main.py --fail-fast",
				Description: "Run on specific files, stop on first failure",
			},
		},
		Notes: []string{
			"positional arguments:",
			"  REPO                  git repository URL or local path",
			"  HOOK                  a single hook-id to run (optional)",
			"",
			"This command allows you to test hooks from a repository without installing",
			"them in your current project. It's particularly useful when developing",
			"new hooks or testing hooks from a fork.",
			"",
			"REPO can be:",
			"  - A git repository URL (https://github.com/user/repo)",
			"  - A local path to a git repository",
			"  - '.' for the current repository",
		},
	}

	return formatter.FormatHelp(parser)
}

// Synopsis returns a short description of the try-repo command
func (c *TryRepoCommand) Synopsis() string {
	return "Try the hooks in a repository, useful for developing new hooks"
}

// Helper functions to reduce cognitive complexity in TryRepoCommand.Run

func (c *TryRepoCommand) parseAndValidateTryRepoArgs(args []string) (*TryRepoOptions, string, string, int) {
	var opts TryRepoOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "REPO [HOOK] [OPTIONS]"

	_, err := parser.ParseArgs(args)
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			return &opts, "", "", 0
		}
		fmt.Printf("Error parsing arguments: %v\n", err)
		return &opts, "", "", 1
	}

	// Get repo from positional args
	repoURL := opts.Positional.Repo
	if repoURL == "" {
		fmt.Println("Error: repository argument is required")
		fmt.Println("Usage: pre-commit try-repo REPO [HOOK] [OPTIONS]")
		return &opts, "", "", 1
	}

	// Handle --rev as alias for --ref
	if opts.Rev != "" && opts.Ref == "" {
		opts.Ref = opts.Rev
	}

	// Validate --from-ref and --to-ref must be used together
	if (opts.FromRef != "") != (opts.ToRef != "") {
		fmt.Println("Error: --from-ref and --to-ref must be used together")
		return &opts, "", "", 1
	}

	// Get hook ID from positional args (optional)
	hookID := opts.Positional.Hook

	if opts.Verbose {
		fmt.Printf("Trying repository: %s\n", repoURL)
		if opts.Ref != "" {
			fmt.Printf("Using ref: %s\n", opts.Ref)
		}
	}

	return &opts, repoURL, hookID, -1 // Continue processing
}

func (c *TryRepoCommand) prepareRepository(
	repoURL, ref string,
	verbose bool,
	tempDir string,
) (*repository.Manager, string, string, error) {
	// Check if this is a local repository path
	isLocalRepo := c.isLocalPath(repoURL)

	// For local repos, check if we need to create a shadow repo
	if isLocalRepo {
		return c.prepareLocalRepository(repoURL, ref, verbose, tempDir)
	}

	// For remote repos, use the standard clone approach
	return c.prepareRemoteRepository(repoURL, ref, verbose)
}

// isLocalPath checks if the given repo URL is a local file path
func (c *TryRepoCommand) isLocalPath(repoURL string) bool {
	// Check for common URL schemes
	if strings.HasPrefix(repoURL, "https://") ||
		strings.HasPrefix(repoURL, "http://") ||
		strings.HasPrefix(repoURL, "git://") ||
		strings.HasPrefix(repoURL, "git@") ||
		strings.HasPrefix(repoURL, "ssh://") {
		return false
	}

	// Check if path exists on disk
	_, err := os.Stat(repoURL)
	return err == nil
}

// prepareLocalRepository handles local repos, creating shadow repos when needed
func (c *TryRepoCommand) prepareLocalRepository(
	repoURL, ref string,
	verbose bool,
	tempDir string,
) (*repository.Manager, string, string, error) {
	// Resolve the absolute path
	absPath, err := filepath.Abs(repoURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Open the local repository
	localRepo, err := git.NewRepository(absPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to open local repository: %w", err)
	}

	// Get HEAD rev if ref not specified
	actualRef := ref
	if actualRef == "" {
		headRev, err := localRepo.GetHeadRev()
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to get HEAD rev: %w", err)
		}
		actualRef = headRev
	}

	// Check if we have uncommitted changes
	hasDiff, err := localRepo.HasDiff()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	if hasDiff && ref == "" {
		// Create shadow repo with uncommitted changes
		if verbose {
			fmt.Println("Creating temporary repo with uncommitted changes...")
		}
		return c.createShadowRepo(localRepo, absPath, actualRef, verbose, tempDir)
	}

	// No uncommitted changes or explicit ref specified - use repo manager to clone
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create repository manager: %w", err)
	}

	if verbose {
		fmt.Printf("Preparing repository: %s\n", repoURL)
	}

	tempRepo := config.Repo{
		Repo: absPath,
		Rev:  actualRef,
	}

	repoPath, err := repoMgr.CloneOrUpdateRepo(context.Background(), tempRepo)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to prepare repository: %w", err)
	}

	return repoMgr, repoPath, actualRef, nil
}

// createShadowRepo creates a temporary clone with uncommitted changes applied
func (c *TryRepoCommand) createShadowRepo(
	sourceRepo *git.Repository,
	sourcePath, ref string,
	verbose bool,
	tempDir string,
) (*repository.Manager, string, string, error) {
	shadowDir := filepath.Join(tempDir, "shadow-repo")

	// Clone the local repo to shadow directory
	if err := sourceRepo.CloneTo(shadowDir); err != nil {
		return nil, "", "", fmt.Errorf("failed to clone to shadow repo: %w", err)
	}

	// Open the shadow repo
	shadowRepo, err := git.NewRepository(shadowDir)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to open shadow repo: %w", err)
	}

	// Checkout the ref to a temp branch
	if err := shadowRepo.CheckoutBranch("_pc_tmp", ref); err != nil {
		return nil, "", "", fmt.Errorf("failed to checkout ref in shadow repo: %w", err)
	}

	// Get staged files from source repo and stage them in shadow
	stagedFiles, err := sourceRepo.GetStagedFiles()
	if err != nil {
		if verbose {
			fmt.Printf("Warning: failed to get staged files: %v\n", err)
		}
	} else if len(stagedFiles) > 0 {
		// Copy staged files from source to shadow and stage them
		for _, file := range stagedFiles {
			srcFile := filepath.Join(sourcePath, file)
			dstFile := filepath.Join(shadowDir, file)

			// Ensure destination directory exists
			if err := os.MkdirAll(filepath.Dir(dstFile), 0o755); err != nil {
				return nil, "", "", fmt.Errorf("failed to create directory: %w", err)
			}

			// Copy the file
			if err := copyFile(srcFile, dstFile); err != nil {
				if verbose {
					fmt.Printf("Warning: failed to copy staged file %s: %v\n", file, err)
				}
				continue
			}
		}
		if err := shadowRepo.AddFiles(stagedFiles); err != nil {
			return nil, "", "", fmt.Errorf("failed to add staged files to shadow: %w", err)
		}
	}

	// Copy and add unstaged changes
	unstagedFiles, err := sourceRepo.GetUnstagedChangesFiles()
	if err != nil {
		if verbose {
			fmt.Printf("Warning: failed to get unstaged files: %v\n", err)
		}
	} else if len(unstagedFiles) > 0 {
		for _, file := range unstagedFiles {
			srcFile := filepath.Join(sourcePath, file)
			dstFile := filepath.Join(shadowDir, file)

			// Ensure destination directory exists
			if err := os.MkdirAll(filepath.Dir(dstFile), 0o755); err != nil {
				return nil, "", "", fmt.Errorf("failed to create directory: %w", err)
			}

			// Copy the file
			if err := copyFile(srcFile, dstFile); err != nil {
				if verbose {
					fmt.Printf("Warning: failed to copy unstaged file %s: %v\n", file, err)
				}
				continue
			}
		}
	}

	// Stage all changes and commit
	if err := shadowRepo.AddAllTracked(); err != nil {
		return nil, "", "", fmt.Errorf("failed to stage changes in shadow: %w", err)
	}

	if err := shadowRepo.Commit("pre-commit try-repo shadow commit"); err != nil {
		return nil, "", "", fmt.Errorf("failed to commit in shadow: %w", err)
	}

	// Get the new HEAD ref
	newRef, err := shadowRepo.GetHeadRev()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get shadow HEAD: %w", err)
	}

	// Create a repository manager (for interface compatibility)
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create repository manager: %w", err)
	}

	return repoMgr, shadowDir, newRef, nil
}

// prepareRemoteRepository handles remote repos using the standard clone approach
func (c *TryRepoCommand) prepareRemoteRepository(
	repoURL, ref string,
	verbose bool,
) (*repository.Manager, string, string, error) {
	// Create repository manager
	repoMgr, err := repository.NewManager()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create repository manager: %w", err)
	}

	if verbose {
		fmt.Printf("Preparing repository: %s\n", repoURL)
	}

	// Create a temporary repo config to use existing functionality
	tempRepo := config.Repo{
		Repo: repoURL,
		Rev:  ref,
	}
	if tempRepo.Rev == "" {
		tempRepo.Rev = "HEAD"
	}

	repoPath, err := repoMgr.CloneOrUpdateRepo(context.Background(), tempRepo)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to prepare repository: %w", err)
	}

	return repoMgr, repoPath, tempRepo.Rev, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (c *TryRepoCommand) loadAndFilterHooks(
	repoURL, repoPath, hookID string,
	verbose bool,
) ([]config.Hook, error) {
	// Load the hooks configuration from the repository
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	if _, statErr := os.Stat(hooksFile); os.IsNotExist(statErr) {
		return nil, fmt.Errorf("no .pre-commit-hooks.yaml found in repository %s", repoURL)
	}

	// Parse hooks configuration
	hooks, err := config.LoadHooksConfig(hooksFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load hooks configuration: %w", err)
	}

	if verbose {
		fmt.Printf("Found %d hooks in repository:\n", len(hooks))
		for _, h := range hooks {
			fmt.Printf("  - %s: %s\n", h.ID, h.Name)
		}
	}

	// Filter hooks if specific hook requested
	if hookID != "" {
		for _, h := range hooks {
			if h.ID == hookID {
				return []config.Hook{h}, nil
			}
		}
		return nil, fmt.Errorf("hook '%s' not found in repository", hookID)
	}

	return hooks, nil
}

func (c *TryRepoCommand) determineFilesToProcess(
	opts *TryRepoOptions,
	currentDir string,
) ([]string, error) {
	var files []string
	var err error

	switch {
	case opts.FromRef != "" && opts.ToRef != "":
		// Get files changed between refs
		repo, repoErr := git.NewRepository(currentDir)
		if repoErr != nil {
			return nil, fmt.Errorf("--from-ref/--to-ref requires a git repository: %w", repoErr)
		}
		files, err = repo.GetChangedFiles(opts.FromRef, opts.ToRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get changed files between refs: %w", err)
		}
	case opts.AllFiles:
		// Get all files in current directory
		files, err = getAllFiles(currentDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get files: %w", err)
		}
	case len(opts.Files) > 0:
		files = opts.Files
	default:
		// Try to get staged files if in git repo, otherwise use current directory files
		if repo, repoErr := git.NewRepository(currentDir); repoErr == nil {
			files, err = repo.GetStagedFiles()
			if err != nil || len(files) == 0 {
				// Fallback to modified files
				files, err = repo.GetUnstagedFiles()
				if err != nil {
					fmt.Printf("⚠️  Warning: Failed to get unstaged files: %v\n", err)
				}
			}
		}
		if len(files) == 0 {
			// Fallback to all files
			files, err = getAllFiles(currentDir)
			if err != nil {
				fmt.Printf("⚠️  Warning: Failed to get all files: %v\n", err)
			}
		}
	}

	return files, nil
}

// displayConfig prints the generated config like Python does
func (c *TryRepoCommand) displayConfig(repoURL, ref string, hooksToRun []config.Hook) {
	separator := strings.Repeat("=", 79)
	fmt.Println(separator)
	fmt.Println("Using config:")
	fmt.Println(separator)

	// Build hooks list for display
	hooksList := make([]string, 0, len(hooksToRun))
	for _, h := range hooksToRun {
		hooksList = append(hooksList, fmt.Sprintf("    -   id: %s", h.ID))
	}

	fmt.Println("repos:")
	fmt.Printf("-   repo: %s\n", repoURL)
	if ref != "" {
		fmt.Printf("    rev: %s\n", ref)
	} else {
		fmt.Println("    rev: HEAD")
	}
	fmt.Println("    hooks:")
	for _, hookLine := range hooksList {
		fmt.Println(hookLine)
	}
	fmt.Println(separator)
}

// setEnvironmentVariables sets up environment variables that hooks expect
// This mirrors what the run command does to ensure consistent behavior
func (c *TryRepoCommand) setEnvironmentVariables(opts *TryRepoOptions) map[string]string {
	env := make(map[string]string)

	// Set PRE_COMMIT flag
	env["PRE_COMMIT"] = "1"
	c.setEnvVar("PRE_COMMIT", "1", opts.Verbose)

	// Set hook stage
	c.setEnvIfNotEmpty(env, "PRE_COMMIT_HOOK_STAGE", opts.HookStage, opts.Verbose)

	// Set from-ref/to-ref related variables
	if opts.FromRef != "" && opts.ToRef != "" {
		env["PRE_COMMIT_FROM_REF"] = opts.FromRef
		env["PRE_COMMIT_TO_REF"] = opts.ToRef
		env["PRE_COMMIT_ORIGIN"] = opts.FromRef // Legacy name
		env["PRE_COMMIT_SOURCE"] = opts.ToRef   // Legacy name

		c.setEnvVar("PRE_COMMIT_FROM_REF", opts.FromRef, opts.Verbose)
		c.setEnvVar("PRE_COMMIT_TO_REF", opts.ToRef, opts.Verbose)
		c.setEnvVar("PRE_COMMIT_ORIGIN", opts.FromRef, opts.Verbose)
		c.setEnvVar("PRE_COMMIT_SOURCE", opts.ToRef, opts.Verbose)
	}

	// Set other hook-specific variables
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

// setEnvVar sets an environment variable with error checking
func (c *TryRepoCommand) setEnvVar(key, value string, verbose bool) {
	if err := os.Setenv(key, value); err != nil && verbose {
		fmt.Printf("⚠️  Warning: failed to set environment variable %s: %v\n", key, err)
	}
}

// setEnvIfNotEmpty sets an environment variable if the value is not empty
func (c *TryRepoCommand) setEnvIfNotEmpty(env map[string]string, key, value string, verbose bool) {
	if value != "" {
		env[key] = value
		c.setEnvVar(key, value, verbose)
	}
}

func (c *TryRepoCommand) executeHooksAndPrintResults(
	repoURL, ref string,
	hooksToRun []config.Hook,
	files []string,
	opts *TryRepoOptions,
	currentDir string,
) int {
	if len(files) == 0 {
		fmt.Println("No files to process")
		return 0
	}

	// Display the config being used (like Python does)
	c.displayConfig(repoURL, ref, hooksToRun)

	// Set up environment variables for hooks (matching what run command does)
	env := c.setEnvironmentVariables(opts)

	// Create a temporary config for the try-repo run
	tempConfig := &config.Config{
		Repos: []config.Repo{
			{
				Repo:  repoURL,
				Rev:   ref,
				Hooks: hooksToRun,
			},
		},
		FailFast: opts.FailFast,
	}

	// Create execution context directly for the orchestrator
	execCtx := &execution.Context{
		Config:      tempConfig,
		Files:       files,
		AllFiles:    opts.AllFiles,
		Verbose:     opts.Verbose,
		ShowDiff:    opts.ShowDiffOnFailure,
		RepoRoot:    currentDir,
		HookStage:   opts.HookStage,
		Environment: env,
		HookIDs:     nil,
		Parallel:    0,
		Timeout:     0,
		Color:       opts.Color,
		FailFast:    opts.FailFast,
		// Diff-based execution
		FromRef: opts.FromRef,
		ToRef:   opts.ToRef,
		// Git hook-specific context
		RemoteBranch:               opts.RemoteBranch,
		LocalBranch:                opts.LocalBranch,
		RemoteName:                 opts.RemoteName,
		RemoteURL:                  opts.RemoteURL,
		CommitMsgFilename:          opts.CommitMsgFilename,
		PrepareCommitMessageSource: opts.PrepareCommitMessageSource,
		CommitObjectName:           opts.CommitObjectName,
		PreRebaseUpstream:          opts.PreRebaseUpstream,
		PreRebaseBranch:            opts.PreRebaseBranch,
		CheckoutType:               opts.CheckoutType,
		IsSquashMerge:              opts.IsSquashMerge,
		RewriteCommand:             opts.RewriteCommand,
	}

	// Create orchestrator and run hooks directly
	orchestrator := hook.NewOrchestrator(execCtx)
	results, err := orchestrator.RunHooks(context.Background())
	if err != nil {
		fmt.Printf("Error running hooks: %v\n", err)
		return 1
	}

	// Print results using the new formatting package
	formatter := formatting.NewFormatter(opts.Color, opts.Verbose)
	formatter.PrintResults(results)

	// Return appropriate exit code
	failed := 0
	for _, result := range results {
		if !result.Success {
			failed++
		}
	}

	if failed > 0 {
		return 1
	}

	return 0
}

// Run executes the try-repo command
func (c *TryRepoCommand) Run(args []string) int {
	opts, repoURL, hookID, rc := c.parseAndValidateTryRepoArgs(args)
	if rc != -1 {
		return rc
	}

	// Create a temp directory for shadow repos and config
	tempDir, err := os.MkdirTemp("", "pre-commit-try-repo-*")
	if err != nil {
		fmt.Printf("Error: Failed to create temp directory: %v\n", err)
		return 1
	}
	defer func() {
		if rmErr := os.RemoveAll(tempDir); rmErr != nil && opts.Verbose {
			fmt.Printf("⚠️  Warning: failed to cleanup temp directory: %v\n", rmErr)
		}
	}()

	// Prepare the repository (may create shadow repo for local repos with uncommitted changes)
	repoMgr, repoPath, actualRef, err := c.prepareRepository(repoURL, opts.Ref, opts.Verbose, tempDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}
	defer func() {
		if closeErr := repoMgr.Close(); closeErr != nil && opts.Verbose {
			fmt.Printf("⚠️  Warning: failed to close repository manager: %v\n", closeErr)
		}
	}()

	// Load and filter hooks (hookID comes from positional arg)
	hooksToRun, err := c.loadAndFilterHooks(repoURL, repoPath, hookID, opts.Verbose)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Get current working directory for context
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: Failed to get current directory: %v\n", err)
		return 1
	}

	// Determine files to process
	files, err := c.determineFilesToProcess(opts, currentDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	// Execute hooks and print results (use actual ref for display)
	displayRef := opts.Ref
	if displayRef == "" {
		displayRef = actualRef
	}
	return c.executeHooksAndPrintResults(repoURL, displayRef, hooksToRun, files, opts, currentDir)
}

// getAllFiles gets all files in a directory recursively
func getAllFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// Skip hidden files and directories
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(relPath, ".") && !strings.Contains(relPath, "/.") {
				files = append(files, relPath)
			}
		}
		return nil
	})
	return files, err
}

// TryRepoCommandFactory creates a new try-repo command instance
func TryRepoCommandFactory() (cli.Command, error) {
	return &TryRepoCommand{}, nil
}
