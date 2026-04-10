package hook

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/dlclark/regexp2"

	"github.com/blairham/go-pre-commit/internal/config"
	"github.com/blairham/go-pre-commit/internal/identify"
	"github.com/blairham/go-pre-commit/internal/languages"
	"github.com/blairham/go-pre-commit/internal/output"
	"github.com/blairham/go-pre-commit/internal/pcre"
	"github.com/blairham/go-pre-commit/internal/xargs"
)

// FixedRandomSeed is used for deterministic file shuffling (matches Python).
const FixedRandomSeed = 1542676187

// RunOptions controls how hooks are run.
type RunOptions struct {
	AllFiles  bool
	Files     []string
	HookID    string
	HookStage config.Stage
	FromRef   string
	ToRef     string
	ShowDiff  bool
	Verbose   bool
	Color     string
	SkipList  []string

	// Environment variables to pass to hooks.
	CommitMsgFilename          string
	PrepareCommitMessageSource string
	CommitObjectName           string
	RemoteName                 string
	RemoteURL                  string
	RemoteBranch               string
	LocalBranch                string
	CheckoutType               string
	IsSquashMerge              string
	RewriteCommand             string
	PreRebaseUpstream          string
	PreRebaseBranch            string
}

// RunResult holds the overall result of running hooks.
type RunResult struct {
	Passed  int
	Failed  int
	Skipped int
	Errors  int
}

// Runner executes hooks.
type Runner struct {
	cfg   *config.Config
	hooks []*Hook
	root  string
}

// NewRunner creates a new hook Runner.
func NewRunner(cfg *config.Config, hooks []*Hook, root string) *Runner {
	return &Runner{
		cfg:   cfg,
		hooks: hooks,
		root:  root,
	}
}

// Run executes all hooks and returns the result.
func (r *Runner) Run(ctx context.Context, opts RunOptions) RunResult {
	result := RunResult{}

	// Set PRE_COMMIT=1 environment variable.
	os.Setenv("PRE_COMMIT", "1")
	defer os.Unsetenv("PRE_COMMIT")

	// Set hook-stage-specific environment variables.
	r.setEnvVars(opts)
	defer r.unsetEnvVars()

	// Parse SKIP env var.
	skipSet := make(map[string]bool)
	if skipEnv := os.Getenv("SKIP"); skipEnv != "" {
		for _, id := range strings.Split(skipEnv, ",") {
			skipSet[strings.TrimSpace(id)] = true
		}
	}
	for _, id := range opts.SkipList {
		skipSet[id] = true
	}

	// Apply top-level files/exclude filters from config.
	files := opts.Files
	if r.cfg.Files != "" || r.cfg.Exclude != "" {
		files = filterByIncludeExclude(files, r.cfg.Files, r.cfg.Exclude)
	}

	// Filter hooks by stage and ID.
	var hooksToRun []*Hook
	for _, h := range r.hooks {
		if opts.HookID != "" && h.ID != opts.HookID && h.Alias != opts.HookID {
			continue
		}
		if opts.HookStage != "" && !h.MatchesStage(opts.HookStage) {
			continue
		}
		hooksToRun = append(hooksToRun, h)
	}

	if len(hooksToRun) == 0 && opts.HookID != "" {
		output.Error("No hook with id %q found", opts.HookID)
		result.Errors++
		return result
	}

	for _, h := range hooksToRun {
		select {
		case <-ctx.Done():
			return result
		default:
		}

		// Check minimum_pre_commit_version.
		if h.MinimumPreCommitVersion != "" && h.MinimumPreCommitVersion != "0" {
			if !checkMinVersion(h.MinimumPreCommitVersion) {
				output.PrintHookHeader(h.Name, output.ResultError)
				output.Error("hook requires pre-commit >= %s", h.MinimumPreCommitVersion)
				result.Errors++
				if shouldFailFast(r.cfg, h) {
					return result
				}
				continue
			}
		}

		// Check if skipped.
		if skipSet[h.ID] || (h.Alias != "" && skipSet[h.Alias]) {
			output.PrintHookHeader(h.Name, output.ResultSkipped)
			result.Skipped++
			continue
		}

		// Filter files by hook's patterns and types.
		matchedFiles := filterFiles(files, h)

		if len(matchedFiles) == 0 && !h.AlwaysRun {
			output.PrintHookHeader(h.Name, output.ResultSkipped)
			result.Skipped++
			continue
		}

		// Get the language handler.
		lang, err := languages.Get(h.Language)
		if err != nil {
			output.PrintHookHeader(h.Name, output.ResultError)
			output.Error("unsupported language %q: %v", h.Language, err)
			result.Errors++
			if shouldFailFast(r.cfg, h) {
				return result
			}
			continue
		}

		// Handle meta hooks specially.
		if h.ID == "check-hooks-apply" || h.ID == "check-useless-excludes" {
			metaExit, metaOut := r.runMetaHook(h, files)
			if metaExit != 0 {
				output.PrintHookHeader(h.Name, output.ResultFailed)
				output.PrintHookOutput(metaOut, h.ID, metaExit, true)
				result.Failed++
			} else {
				output.PrintHookHeader(h.Name, output.ResultPassed)
			}
			continue
		}

		// Determine file args to pass.
		var fileArgs []string
		if h.PassFilenames {
			fileArgs = matchedFiles
		}

		// Capture file state before running hook (for modification detection).
		var fileHashesBefore map[string]string
		if !opts.AllFiles {
			fileHashesBefore = hashFiles(fileArgs)
		}

		// Run the hook using xargs for batching.
		var exitCode int
		var hookOutput []byte
		exitCode, hookOutput, err = runHookXargs(ctx, lang, h, fileArgs, r.root)
		if err != nil {
			output.PrintHookHeader(h.Name, output.ResultError)
			output.Error("hook execution error: %v", err)
			result.Errors++
			if shouldFailFast(r.cfg, h) {
				return result
			}
			continue
		}

		// Detect if files were modified by the hook.
		filesModified := false
		if fileHashesBefore != nil && exitCode == 0 {
			fileHashesAfter := hashFiles(fileArgs)
			for f, hashBefore := range fileHashesBefore {
				if hashAfter, ok := fileHashesAfter[f]; ok && hashBefore != hashAfter {
					filesModified = true
					break
				}
			}
		}

		if exitCode != 0 || filesModified {
			output.PrintHookHeader(h.Name, output.ResultFailed)
			output.PrintHookOutput(hookOutput, h.ID, exitCode, opts.Verbose || h.Verbose)
			result.Failed++

			// Write to log file if configured.
			if h.LogFile != "" {
				_ = os.WriteFile(h.LogFile, hookOutput, 0o644)
			}

			if shouldFailFast(r.cfg, h) {
				return result
			}
		} else {
			output.PrintHookHeader(h.Name, output.ResultPassed)
			if opts.Verbose || h.Verbose {
				output.PrintHookOutput(hookOutput, h.ID, exitCode, true)
			}
			result.Passed++
		}
	}

	return result
}

// setEnvVars sets hook-stage-specific environment variables.
func (r *Runner) setEnvVars(opts RunOptions) {
	setIfNonEmpty := func(key, value string) {
		if value != "" {
			os.Setenv(key, value)
		}
	}

	setIfNonEmpty("PRE_COMMIT_COMMIT_MSG_FILENAME", opts.CommitMsgFilename)
	setIfNonEmpty("PRE_COMMIT_COMMIT_MSG_SOURCE", opts.PrepareCommitMessageSource)
	setIfNonEmpty("PRE_COMMIT_COMMIT_OBJECT_NAME", opts.CommitObjectName)

	if opts.FromRef != "" && opts.ToRef != "" {
		os.Setenv("PRE_COMMIT_FROM_REF", opts.FromRef)
		os.Setenv("PRE_COMMIT_TO_REF", opts.ToRef)
		// Legacy aliases.
		os.Setenv("PRE_COMMIT_SOURCE", opts.FromRef)
		os.Setenv("PRE_COMMIT_ORIGIN", opts.ToRef)
	}

	setIfNonEmpty("PRE_COMMIT_LOCAL_BRANCH", opts.LocalBranch)
	setIfNonEmpty("PRE_COMMIT_REMOTE_BRANCH", opts.RemoteBranch)
	setIfNonEmpty("PRE_COMMIT_REMOTE_NAME", opts.RemoteName)
	setIfNonEmpty("PRE_COMMIT_REMOTE_URL", opts.RemoteURL)
	setIfNonEmpty("PRE_COMMIT_CHECKOUT_TYPE", opts.CheckoutType)
	setIfNonEmpty("PRE_COMMIT_IS_SQUASH_MERGE", opts.IsSquashMerge)
	setIfNonEmpty("PRE_COMMIT_REWRITE_COMMAND", opts.RewriteCommand)
	setIfNonEmpty("PRE_COMMIT_PRE_REBASE_UPSTREAM", opts.PreRebaseUpstream)
	setIfNonEmpty("PRE_COMMIT_PRE_REBASE_BRANCH", opts.PreRebaseBranch)
}

// unsetEnvVars cleans up hook-stage-specific environment variables.
func (r *Runner) unsetEnvVars() {
	for _, key := range []string{
		"PRE_COMMIT_COMMIT_MSG_FILENAME",
		"PRE_COMMIT_COMMIT_MSG_SOURCE",
		"PRE_COMMIT_COMMIT_OBJECT_NAME",
		"PRE_COMMIT_FROM_REF",
		"PRE_COMMIT_TO_REF",
		"PRE_COMMIT_SOURCE",
		"PRE_COMMIT_ORIGIN",
		"PRE_COMMIT_LOCAL_BRANCH",
		"PRE_COMMIT_REMOTE_BRANCH",
		"PRE_COMMIT_REMOTE_NAME",
		"PRE_COMMIT_REMOTE_URL",
		"PRE_COMMIT_CHECKOUT_TYPE",
		"PRE_COMMIT_IS_SQUASH_MERGE",
		"PRE_COMMIT_REWRITE_COMMAND",
		"PRE_COMMIT_PRE_REBASE_UPSTREAM",
		"PRE_COMMIT_PRE_REBASE_BRANCH",
	} {
		os.Unsetenv(key)
	}
}

// shouldFailFast checks whether execution should stop after a failure.
func shouldFailFast(cfg *config.Config, h *Hook) bool {
	return cfg.FailFast || h.FailFast
}

// filterByIncludeExclude filters filenames using include/exclude regex patterns.
func filterByIncludeExclude(names []string, include, exclude string) []string {
	var result []string
	var includeRe, excludeRe *regexp2.Regexp
	if include != "" {
		includeRe, _ = pcre.Compile(include)
	}
	if exclude != "" {
		excludeRe, _ = pcre.Compile(exclude)
	}
	for _, name := range names {
		if includeRe != nil && !pcre.Match(includeRe, name) {
			continue
		}
		if excludeRe != nil && pcre.Match(excludeRe, name) {
			continue
		}
		result = append(result, name)
	}
	return result
}

// filterFiles filters files based on hook include/exclude patterns and type filters.
func filterFiles(files []string, h *Hook) []string {
	var matched []string

	var includeRe, excludeRe *regexp2.Regexp
	if h.Files != "" {
		includeRe, _ = pcre.Compile(h.Files)
	}
	if h.Exclude != "" {
		excludeRe, _ = pcre.Compile(h.Exclude)
	}

	for _, f := range files {
		// Skip files that do not exist on disk (e.g. staged deletions
		// or files removed from the working tree without git rm).
		// Matches Python identify library which raises ValueError for
		// non-existent paths.
		if _, err := os.Lstat(f); err != nil {
			continue
		}
		// Check include pattern.
		if includeRe != nil && !pcre.Match(includeRe, f) {
			continue
		}
		// Check exclude pattern.
		if excludeRe != nil && pcre.Match(excludeRe, f) {
			continue
		}
		// Check types.
		tags := identify.TagsForFile(f)
		if !identify.MatchesTypes(tags, h.Types, h.TypesOr, h.ExcludeTypes) {
			continue
		}
		matched = append(matched, f)
	}

	return matched
}

// runHookXargs runs a hook using xargs-style batching and concurrency.
// All execution goes through lang.Run to ensure language-specific environment
// setup (e.g. virtualenv PATH for Python hooks).
func runHookXargs(ctx context.Context, lang languages.Language, h *Hook, fileArgs []string, workDir string) (int, []byte, error) {
	if len(fileArgs) == 0 {
		return lang.Run(ctx, h.RepoDir, workDir, h.Entry, h.Args, nil, h.LanguageVersion)
	}

	// Determine batch size and concurrency.
	maxJobs := 1
	if !h.RequireSerial {
		maxJobs = targetConcurrency()
	}

	// Batch the file arguments.
	batches := batchFileArgs(fileArgs, xargs.DefaultMaxBatchSize())

	type batchResult struct {
		exitCode int
		output   []byte
		err      error
	}

	results := make([]batchResult, len(batches))

	if maxJobs <= 1 || len(batches) <= 1 {
		// Sequential execution.
		for i, batch := range batches {
			exitCode, out, err := lang.Run(ctx, h.RepoDir, workDir, h.Entry, h.Args, batch, h.LanguageVersion)
			results[i] = batchResult{exitCode: exitCode, output: out, err: err}
		}
	} else {
		// Parallel execution.
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxJobs)
		for i, batch := range batches {
			wg.Add(1)
			go func(idx int, files []string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				exitCode, out, err := lang.Run(ctx, h.RepoDir, workDir, h.Entry, h.Args, files, h.LanguageVersion)
				results[idx] = batchResult{exitCode: exitCode, output: out, err: err}
			}(i, batch)
		}
		wg.Wait()
	}

	// Aggregate results.
	var allOutput []byte
	exitCode := 0
	for _, r := range results {
		if r.err != nil {
			return -1, allOutput, r.err
		}
		allOutput = append(allOutput, r.output...)
		if r.exitCode != 0 {
			exitCode = r.exitCode
		}
	}

	return exitCode, allOutput, nil
}

// batchFileArgs splits file arguments into batches.
func batchFileArgs(files []string, maxBatchSize int) [][]string {
	if maxBatchSize <= 0 || len(files) <= maxBatchSize {
		return [][]string{files}
	}
	var batches [][]string
	for i := 0; i < len(files); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(files) {
			end = len(files)
		}
		batches = append(batches, files[i:end])
	}
	return batches
}

// targetConcurrency returns the target number of parallel jobs.
func targetConcurrency() int {
	if os.Getenv("PRE_COMMIT_NO_CONCURRENCY") != "" {
		return 1
	}
	if os.Getenv("TRAVIS") != "" {
		return 2
	}
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	return n
}

// hashFiles returns a map of filename to SHA256 hash of file contents.
func hashFiles(files []string) map[string]string {
	hashes := make(map[string]string, len(files))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		hashes[f] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	return hashes
}

// checkMinVersion checks if the current version meets the minimum requirement.
func checkMinVersion(minVersion string) bool {
	current := parseVersionParts(config.Version)
	required := parseVersionParts(minVersion)

	for i := 0; i < len(required); i++ {
		if i >= len(current) {
			return false
		}
		if current[i] < required[i] {
			return false
		}
		if current[i] > required[i] {
			return true
		}
	}
	return true
}

func parseVersionParts(v string) []int {
	var parts []int
	for _, s := range strings.Split(v, ".") {
		n := 0
		for _, c := range s {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			} else {
				break
			}
		}
		parts = append(parts, n)
	}
	return parts
}

// installStateFile is the filename used to track installed environment state.
// This matches Python pre-commit's install_state tracking to avoid unnecessary
// reinstalls and to detect when dependencies have changed.
const installStateFile = "install_state_v2"

// InstallEnvironments installs environments for all provided hooks.
func InstallEnvironments(ctx context.Context, hooks []*Hook) error {
	installed := make(map[string]bool)
	var mu sync.Mutex

	for _, h := range hooks {
		key := h.InstallKey()
		mu.Lock()
		if installed[key] {
			mu.Unlock()
			continue
		}
		installed[key] = true
		mu.Unlock()

		lang, err := languages.Get(h.Language)
		if err != nil {
			return fmt.Errorf("unsupported language %q for hook %q: %w", h.Language, h.ID, err)
		}

		if lang.EnvironmentDir() == "" {
			continue // No environment to install.
		}

		// Check if environment is already installed with correct state.
		envDir := h.RepoDir
		if envDir == "" {
			continue
		}
		stateFile := filepath.Join(envDir, lang.EnvironmentDir(), installStateFile)
		expectedState := h.InstallKey()

		if data, err := os.ReadFile(stateFile); err == nil {
			if string(data) == expectedState {
				continue // Already installed with same deps.
			}
			// State mismatch — deps changed, need reinstall.
			envPath := filepath.Join(envDir, lang.EnvironmentDir())
			os.RemoveAll(envPath)
		}

		output.Info("Installing environment for %s.", h.Repo)
		output.Info("Once installed this environment will be reused.")
		output.Info("This may take a few minutes...")

		if err := lang.InstallEnvironment(h.RepoDir, h.LanguageVersion, h.AdditionalDependencies); err != nil {
			// Clean up partial install.
			envPath := filepath.Join(envDir, lang.EnvironmentDir())
			os.RemoveAll(envPath)
			return fmt.Errorf("failed to install environment for hook %q: %w", h.ID, err)
		}

		// Write install state file.
		stateDir := filepath.Dir(stateFile)
		os.MkdirAll(stateDir, 0o755)
		if err := os.WriteFile(stateFile, []byte(expectedState), 0o644); err != nil {
			output.Warn("Failed to write install state: %v", err)
		}
	}

	return nil
}

// ShowDiffOnFailure runs git diff to show changes made by hooks.
func ShowDiffOnFailure(allFiles bool) {
	useColor := "never"
	if output.UseColor() {
		useColor = "always"
	}
	cmd := exec.Command("git", "--no-pager", "diff", "--no-ext-diff", "--color="+useColor)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	if allFiles {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Hint: You may want to review the changes and commit them.")
	}
}

// runMetaHook performs meta hook checks (check-hooks-apply, check-useless-excludes).
func (r *Runner) runMetaHook(metaHook *Hook, allFiles []string) (int, []byte) {
	switch metaHook.ID {
	case "check-hooks-apply":
		return r.checkHooksApply(allFiles)
	case "check-useless-excludes":
		return r.checkUselessExcludes(allFiles)
	default:
		return 0, nil
	}
}

// checkHooksApply checks that every hook in the config matches at least one file.
func (r *Runner) checkHooksApply(allFiles []string) (int, []byte) {
	var msgs []string
	exitCode := 0

	for _, h := range r.hooks {
		// Skip meta hooks themselves.
		if h.ID == "check-hooks-apply" || h.ID == "check-useless-excludes" || h.ID == "identity" {
			continue
		}
		// Skip always_run hooks (they run regardless of files).
		if h.AlwaysRun {
			continue
		}
		matched := filterFiles(allFiles, h)
		if len(matched) == 0 {
			msgs = append(msgs, fmt.Sprintf("%s does not apply to this repository", h.ID))
			exitCode = 1
		}
	}

	if exitCode != 0 {
		return exitCode, []byte(strings.Join(msgs, "\n") + "\n")
	}
	return 0, nil
}

// checkUselessExcludes checks that exclude patterns actually exclude something.
func (r *Runner) checkUselessExcludes(allFiles []string) (int, []byte) {
	var msgs []string
	exitCode := 0

	// Check top-level exclude.
	if r.cfg.Exclude != "" && r.cfg.Exclude != "^$" {
		excludeRe, err := pcre.Compile(r.cfg.Exclude)
		if err == nil {
			matched := false
			for _, f := range allFiles {
				if pcre.Match(excludeRe, f) {
					matched = true
					break
				}
			}
			if !matched {
				msgs = append(msgs, fmt.Sprintf("The top-level exclude pattern %q does not match any files", r.cfg.Exclude))
				exitCode = 1
			}
		}
	}

	for _, h := range r.hooks {
		if h.ID == "check-hooks-apply" || h.ID == "check-useless-excludes" || h.ID == "identity" {
			continue
		}
		if h.Exclude == "" || h.Exclude == "^$" {
			continue
		}

		// Find files that match the hook's include pattern and types.
		var included []string
		includeRe, _ := pcre.Compile(h.Files)
		for _, f := range allFiles {
			if includeRe != nil && !pcre.Match(includeRe, f) {
				continue
			}
			tags := identify.TagsForFile(f)
			if !identify.MatchesTypes(tags, h.Types, h.TypesOr, h.ExcludeTypes) {
				continue
			}
			included = append(included, f)
		}

		// Check if exclude actually excludes anything from the included set.
		excludeRe, err := pcre.Compile(h.Exclude)
		if err != nil {
			continue
		}
		matched := false
		for _, f := range included {
			if pcre.Match(excludeRe, f) {
				matched = true
				break
			}
		}
		if !matched {
			msgs = append(msgs, fmt.Sprintf("The exclude pattern %q for hook %q does not match any files", h.Exclude, h.ID))
			exitCode = 1
		}
	}

	if exitCode != 0 {
		return exitCode, []byte(strings.Join(msgs, "\n") + "\n")
	}
	return 0, nil
}
