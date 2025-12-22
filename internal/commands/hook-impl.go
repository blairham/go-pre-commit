package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// Z40 is the null SHA (40 zeros) indicating a non-existent ref
const Z40 = "0000000000000000000000000000000000000000"

// prePushRef represents a parsed pre-push stdin line
type prePushRef struct {
	LocalBranch  string
	LocalSHA     string
	RemoteBranch string
	RemoteSHA    string
}

// prePushContext holds parsed pre-push information
type prePushContext struct {
	RemoteName string
	RemoteURL  string
	Refs       []prePushRef
	AllFiles   bool
	FromRef    string
	ToRef      string
}

// HookImplCommand handles the hook-impl command functionality
type HookImplCommand struct{}

// HookImplOptions holds command-line options for the hook-impl command
type HookImplOptions struct {
	Help                bool   `long:"help"                   description:"show this help message and exit"                  short:"h"`
	Color               string `long:"color"                  description:"Whether to use color in output. Defaults to BTICK_auto_BTICK." choice:"auto" choice:"always" choice:"never"`
	Config              string `long:"config"                 description:"Path to alternate config file"                    short:"c" value-name:"CONFIG"`
	HookType            string `long:"hook-type"              description:""                                                  value-name:"HOOK_TYPE"`
	HookDir             string `long:"hook-dir"               description:""                                                  value-name:"HOOK_DIR"`
	SkipOnMissingConfig bool   `long:"skip-on-missing-config" description:""`
	Verbose             bool   // Internal flag, not exposed in help
}

// Help returns the help text for the hook-impl command
func (c *HookImplCommand) Help() string {
	var opts HookImplOptions
	parser := flags.NewParser(&opts, flags.Default)
	parser.Usage = "[-h] [--color {auto,always,never}] [-c CONFIG] [--hook-type HOOK_TYPE] [--hook-dir HOOK_DIR] [--skip-on-missing-config] ..."

	formatter := &HelpFormatter{
		Command:     "hook-impl",
		Description: "",
		Examples:    []Example{},
		Notes: []string{
			"positional arguments:",
			"  rest",
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

	if validationCode := c.validateOptions(opts, remaining); validationCode != -1 {
		return validationCode
	}

	c.logVerboseInfo(opts, remaining)

	// Run legacy hook first (matches Python's _run_legacy behavior)
	legacyRetv, stdin := c.runLegacyHook(opts.HookType, opts.HookDir, remaining)

	// Create repository early so it's available for pre-push parsing
	repo, err := git.NewRepository("")
	if err != nil {
		fmt.Printf("Error: not in a git repository: %v\n", err)
		return 1
	}

	// Parse pre-push stdin if applicable
	var prePushCtx *prePushContext
	if opts.HookType == "pre-push" && len(remaining) >= 2 {
		prePushCtx = c.parsePrePushStdin(repo, remaining[0], remaining[1], stdin)
		if prePushCtx == nil {
			// Nothing to push
			return legacyRetv
		}
	}

	if configValidationCode := c.validateConfigFile(opts, legacyRetv); configValidationCode != -1 {
		return configValidationCode
	}

	cfg, repoManager, setupCode := c.setupConfigAndManager(opts)
	if setupCode != -1 {
		return setupCode
	}
	defer c.closeRepoManager(repoManager, opts.Verbose)

	files, contextCode := c.getFilesForContext(opts, repo, remaining, prePushCtx)
	if contextCode != -1 {
		return contextCode
	}

	// OR legacy return code with hook execution result (matches Python's `return retv | run(...)`)
	hookRetv := c.executeHooks(opts, remaining, cfg, repo, files, prePushCtx)
	return legacyRetv | hookRetv
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

	// Set default config file if not specified
	if opts.Config == "" {
		opts.Config = ".pre-commit-config.yaml"
	}

	return &opts, remaining, -1
}

// validateOptions validates the parsed options and arguments
func (c *HookImplCommand) validateOptions(opts *HookImplOptions, remaining []string) int {
	if opts.HookType == "" {
		fmt.Println("Error: --hook-type is required")
		return 1
	}

	// Validate argument count based on hook type
	argCount := len(remaining)
	switch opts.HookType {
	case "commit-msg":
		if argCount != 1 {
			fmt.Printf("hook-impl for commit-msg expected 1 argument but got %d: %v\n", argCount, remaining)
			return 1
		}
	case "prepare-commit-msg":
		if argCount < 1 || argCount > 3 {
			fmt.Printf("hook-impl for prepare-commit-msg expected 1, 2, or 3 arguments but got %d: %v\n", argCount, remaining)
			return 1
		}
	case "post-checkout":
		if argCount != 3 {
			fmt.Printf("hook-impl for post-checkout expected 3 arguments but got %d: %v\n", argCount, remaining)
			return 1
		}
	case "post-merge":
		if argCount != 1 {
			fmt.Printf("hook-impl for post-merge expected 1 argument but got %d: %v\n", argCount, remaining)
			return 1
		}
	case "post-rewrite":
		if argCount != 1 {
			fmt.Printf("hook-impl for post-rewrite expected 1 argument but got %d: %v\n", argCount, remaining)
			return 1
		}
	case "pre-push":
		if argCount != 2 {
			fmt.Printf("hook-impl for pre-push expected 2 arguments but got %d: %v\n", argCount, remaining)
			return 1
		}
	// Other hook types like pre-commit, pre-merge-commit, post-commit don't require specific argument counts
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

// runLegacyHook runs the legacy hook if it exists (matches Python's _run_legacy)
// Returns the legacy hook's return code and any stdin data (for pre-push hooks)
func (c *HookImplCommand) runLegacyHook(hookType, hookDir string, args []string) (int, []byte) {
	// Check for recursive legacy hook execution
	if os.Getenv("PRE_COMMIT_RUNNING_LEGACY") != "" {
		fmt.Fprintf(os.Stderr, "bug: pre-commit's script is installed in migration mode\n"+
			"run `pre-commit install -f --hook-type %s` to fix this\n\n"+
			"Please report this bug at https://github.com/pre-commit/pre-commit/issues\n", hookType)
		os.Exit(1)
	}

	// Read stdin for pre-push hooks
	var stdin []byte
	if hookType == "pre-push" {
		var err error
		stdin, err = io.ReadAll(os.Stdin)
		if err != nil {
			stdin = []byte{}
		}
	}

	// If no hook dir specified, use default .git/hooks
	if hookDir == "" {
		hookDir = ".git/hooks"
	}

	// Check for legacy hook file
	legacyHook := filepath.Join(hookDir, hookType+".legacy")
	info, err := os.Stat(legacyHook)
	if err != nil || info.IsDir() {
		// No legacy hook exists
		return 0, stdin
	}

	// Check if legacy hook is executable
	if info.Mode()&0o111 == 0 {
		// Not executable
		return 0, stdin
	}

	// Run legacy hook with PRE_COMMIT_RUNNING_LEGACY set
	cmd := exec.Command(legacyHook, args...)
	cmd.Env = append(os.Environ(), "PRE_COMMIT_RUNNING_LEGACY=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Pass stdin to the legacy hook if it's a pre-push hook
	if hookType == "pre-push" && len(stdin) > 0 {
		cmd.Stdin = io.NopCloser(ioReader(stdin))
	}

	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), stdin
		}
		// Non-exit error (e.g., command not found)
		return 1, stdin
	}

	return 0, stdin
}

// ioReader is a helper to create a reader from bytes
func ioReader(b []byte) io.Reader {
	return &bytesReader{data: b}
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// rsplitN splits a string from the right into at most n parts, similar to Python's rsplit(maxsplit=n-1)
// For example, rsplitN("a b c d", 4) returns ["a", "b", "c", "d"]
// This handles edge cases where fields might have different spacing
func rsplitN(s string, n int) []string {
	// Trim leading/trailing whitespace first
	s = strings.TrimSpace(s)

	if n <= 0 || s == "" {
		return nil
	}
	if n == 1 {
		return []string{s}
	}

	// Find split points from the right
	var parts []string
	remaining := s

	for i := 0; i < n-1; i++ {
		// Find the last space in remaining
		idx := strings.LastIndex(remaining, " ")
		if idx == -1 {
			// No more spaces, we're done
			break
		}
		// Extract the part after the last space (trimmed to handle multiple spaces)
		part := strings.TrimSpace(remaining[idx+1:])
		if part != "" {
			parts = append([]string{part}, parts...)
		}
		remaining = strings.TrimSpace(remaining[:idx])
	}

	// Add the remaining part as the first element
	if remaining != "" {
		parts = append([]string{remaining}, parts...)
	}

	return parts
}

// validateConfigFile checks if the config file exists and is accessible
// Takes legacyRetv to return if skipping due to missing config (matches Python behavior)
func (c *HookImplCommand) validateConfigFile(opts *HookImplOptions, legacyRetv int) int {
	if _, statErr := os.Stat(opts.Config); os.IsNotExist(statErr) {
		// Check both --skip-on-missing-config flag and PRE_COMMIT_ALLOW_NO_CONFIG env var
		if opts.SkipOnMissingConfig || os.Getenv("PRE_COMMIT_ALLOW_NO_CONFIG") != "" {
			fmt.Printf("`%s` config file not found. Skipping `pre-commit`.\n", opts.Config)
			return legacyRetv // Return legacy hook's return code (matches Python)
		}
		// Print helpful error message (matches Python's output)
		fmt.Printf("No %s file was found\n", opts.Config)
		fmt.Printf("- To temporarily silence this, run `PRE_COMMIT_ALLOW_NO_CONFIG=1 git ...`\n")
		fmt.Printf("- To permanently silence this, install pre-commit with the --allow-missing-config option\n")
		fmt.Printf("- To uninstall pre-commit run `pre-commit uninstall`\n")
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
		fmt.Printf("⚠️  Warning: failed to mark config as used: %v\n", markErr)
	}

	return cfg, repoManager, -1
}

// closeRepoManager safely closes the repository manager
func (c *HookImplCommand) closeRepoManager(repoManager *repository.Manager, verbose bool) {
	if closeErr := repoManager.Close(); closeErr != nil && verbose {
		fmt.Printf("⚠️  Warning: failed to close repository manager: %v\n", closeErr)
	}
}

// getFilesForContext determines which files to process
func (c *HookImplCommand) getFilesForContext(
	opts *HookImplOptions,
	repo *git.Repository,
	remaining []string,
	prePushCtx *prePushContext,
) ([]string, int) {
	files, err := c.getFilesForHookType(opts.HookType, repo, remaining, prePushCtx)
	if err != nil {
		fmt.Printf("Error: failed to get files for hook type %s: %v\n", opts.HookType, err)
		return nil, 1
	}

	return files, -1
}

// executeHooks runs the hooks and returns the result code
func (c *HookImplCommand) executeHooks(
	opts *HookImplOptions,
	remaining []string,
	cfg *config.Config,
	repo *git.Repository,
	files []string,
	prePushCtx *prePushContext,
) int {
	env := c.setupEnvironmentVariables(opts.HookType, remaining, prePushCtx)

	// Determine if all_files mode should be used
	allFiles := false
	if prePushCtx != nil && prePushCtx.AllFiles {
		allFiles = true
	}

	// Create execution context directly for the orchestrator
	execCtx := &execution.Context{
		Config:      cfg,
		Files:       files,
		AllFiles:    allFiles,
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
	prePushCtx *prePushContext,
) ([]string, error) {
	switch hookType {
	case "pre-commit":
		return repo.GetStagedFiles()

	case "pre-push":
		// Use the pre-push context parsed from stdin
		if prePushCtx != nil {
			if prePushCtx.AllFiles {
				return repo.GetAllFiles()
			}
			if prePushCtx.FromRef != "" && prePushCtx.ToRef != "" {
				return repo.GetChangedFiles(prePushCtx.FromRef, prePushCtx.ToRef)
			}
		}
		// Fallback to getting all files
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
		fmt.Printf("⚠️  Warning: Unknown hook type '%s', using all files\n", hookType)
		return repo.GetAllFiles()
	}
}

// setupEnvironmentVariables sets up environment variables for the hook type
func (c *HookImplCommand) setupEnvironmentVariables(
	hookType string,
	args []string,
	prePushCtx *prePushContext,
) map[string]string {
	env := make(map[string]string)

	// Always set the PRE_COMMIT flag
	env["PRE_COMMIT"] = "1"
	env["PRE_COMMIT_HOOK_STAGE"] = hookType

	// Set hook-type specific environment variables
	c.setHookTypeSpecificEnvVars(env, hookType, args, prePushCtx)

	return env
}

// setHookTypeSpecificEnvVars sets environment variables specific to each hook type
func (c *HookImplCommand) setHookTypeSpecificEnvVars(
	env map[string]string,
	hookType string,
	args []string,
	prePushCtx *prePushContext,
) {
	switch hookType {
	case "pre-push":
		c.setPrePushEnvVars(env, args, prePushCtx)
	case "commit-msg":
		c.setCommitMsgEnvVars(env, args)
	case "prepare-commit-msg":
		c.setPrepareCommitMsgEnvVars(env, args)
	case "post-checkout":
		c.setPostCheckoutEnvVars(env, args)
	case "post-merge":
		c.setPostMergeEnvVars(env, args)
	case "post-rewrite":
		c.setPostRewriteEnvVars(env, args)
	case "pre-rebase":
		c.setPreRebaseEnvVars(env, args)
	}
}

// setPrePushEnvVars sets environment variables for pre-push hooks
func (c *HookImplCommand) setPrePushEnvVars(env map[string]string, args []string, prePushCtx *prePushContext) {
	// Set remote_name and remote_url from args
	if len(args) >= 2 {
		env["PRE_COMMIT_REMOTE_NAME"] = args[0]
		env["PRE_COMMIT_REMOTE_URL"] = args[1]
	}

	// Set refs from parsed stdin context
	if prePushCtx != nil {
		if prePushCtx.FromRef != "" {
			env["PRE_COMMIT_FROM_REF"] = prePushCtx.FromRef
		}
		if prePushCtx.ToRef != "" {
			env["PRE_COMMIT_TO_REF"] = prePushCtx.ToRef
		}
		// Use first ref's branch info
		if len(prePushCtx.Refs) > 0 {
			env["PRE_COMMIT_REMOTE_BRANCH"] = prePushCtx.Refs[0].RemoteBranch
			env["PRE_COMMIT_LOCAL_BRANCH"] = prePushCtx.Refs[0].LocalBranch
		}
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
		env["PRE_COMMIT_FROM_REF"] = args[0]
		env["PRE_COMMIT_TO_REF"] = args[1]
	}
}

// setPostMergeEnvVars sets environment variables for post-merge hooks
func (c *HookImplCommand) setPostMergeEnvVars(env map[string]string, args []string) {
	if len(args) >= 1 {
		env["PRE_COMMIT_IS_SQUASH_MERGE"] = args[0]
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

// parsePrePushStdin parses the stdin data from git pre-push hook
// Returns a prePushContext with the parsed refs, or nil if nothing to push
func (c *HookImplCommand) parsePrePushStdin(repo *git.Repository, remoteName, remoteURL string, stdin []byte) *prePushContext {
	ctx := &prePushContext{
		RemoteName: remoteName,
		RemoteURL:  remoteURL,
		Refs:       []prePushRef{},
	}

	// Parse stdin lines: each line is "<local_branch> <local_sha> <remote_branch> <remote_sha>"
	lines := strings.Split(strings.TrimSpace(string(stdin)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Use rsplit behavior (split from right with max 3 splits) to match Python
		// This handles edge cases better than strings.Fields()
		parts := rsplitN(line, 4)
		if len(parts) != 4 {
			continue
		}

		ref := prePushRef{
			LocalBranch:  parts[0],
			LocalSHA:     parts[1],
			RemoteBranch: parts[2],
			RemoteSHA:    parts[3],
		}
		ctx.Refs = append(ctx.Refs, ref)
	}

	// Process refs to determine from_ref and to_ref
	for _, ref := range ctx.Refs {
		// Skip deletions (local sha is Z40)
		if ref.LocalSHA == Z40 {
			continue
		}

		// Check if remote sha exists and is known
		if ref.RemoteSHA != Z40 && c.revExists(repo, ref.RemoteSHA) {
			// Normal push - remote sha exists
			ctx.FromRef = ref.RemoteSHA
			ctx.ToRef = ref.LocalSHA
			return ctx
		}

		// Remote sha doesn't exist or is Z40 (new branch) - find ancestors
		ancestors := c.findAncestors(repo, ref.LocalSHA, remoteName)
		if len(ancestors) == 0 {
			// No new commits to push
			continue
		}

		firstAncestor := ancestors[0]
		roots := c.getRootCommits(repo, ref.LocalSHA)

		if roots[firstAncestor] {
			// Pushing the whole tree including root commit
			ctx.AllFiles = true
			return ctx
		}

		// Find the parent of the first ancestor
		parent := c.getParentCommit(repo, firstAncestor)
		if parent != "" {
			ctx.FromRef = parent
			ctx.ToRef = ref.LocalSHA
			return ctx
		}
	}

	// Nothing to push
	if len(ctx.Refs) == 0 {
		return nil
	}

	return ctx
}

// revExists checks if a git revision exists
func (c *HookImplCommand) revExists(repo *git.Repository, rev string) bool {
	if repo == nil {
		return false
	}
	return repo.RevExists(rev)
}

// findAncestors finds ancestors of a commit that aren't in the remote
func (c *HookImplCommand) findAncestors(repo *git.Repository, localSHA, remoteName string) []string {
	if repo == nil {
		return nil
	}
	ancestors, err := repo.FindAncestors(localSHA, remoteName)
	if err != nil {
		return nil
	}
	return ancestors
}

// getRootCommits returns a set of root commits (commits with no parents)
func (c *HookImplCommand) getRootCommits(repo *git.Repository, localSHA string) map[string]bool {
	if repo == nil {
		return nil
	}
	roots, err := repo.GetRootCommits(localSHA)
	if err != nil {
		return nil
	}
	return roots
}

// getParentCommit gets the parent of a commit
func (c *HookImplCommand) getParentCommit(repo *git.Repository, commit string) string {
	if repo == nil {
		return ""
	}
	parent, err := repo.GetParentCommit(commit)
	if err != nil {
		return ""
	}
	return parent
}

// HookImplCommandFactory creates a new hook-impl command instance
func HookImplCommandFactory() (cli.Command, error) {
	return &HookImplCommand{}, nil
}
