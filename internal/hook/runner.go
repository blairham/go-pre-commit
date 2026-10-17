// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dlclark/regexp2"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
	"github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/identify"
	"github.com/blairham/go-pre-commit/v4/internal/languages"
	"github.com/blairham/go-pre-commit/v4/internal/output"
	"github.com/blairham/go-pre-commit/v4/internal/pcre"
	"github.com/blairham/go-pre-commit/v4/internal/pyrandom"
	"github.com/blairham/go-pre-commit/v4/internal/xargs"
)

// FixedRandomSeed seeds the deterministic file shuffle, as upstream's
// lang_base.FIXED_RANDOM_SEED does.
const FixedRandomSeed = 1542676187

// shuffledFileArgs returns the order upstream hands a hook its files in.
// Upstream's lang_base.run_xargs shuffles a copy of file_args with
// random.Random seeded by FIXED_RANDOM_SEED unless the hook is
// require_serial, so batches fill evenly yet a hook sees a stable order.
// The fail and pygrep languages never go through run_xargs (fail echoes
// its files, pygrep calls xargs directly), so they keep the given order.
func shuffledFileArgs(language string, requireSerial bool, fileArgs []string) []string {
	if requireSerial || language == "fail" || language == "pygrep" || len(fileArgs) < 2 {
		return fileArgs
	}
	shuffled := slices.Clone(fileArgs)
	pyrandom.Shuffle(pyrandom.New(FixedRandomSeed), shuffled)
	return shuffled
}

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
	Jobs      int

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
		// Upstream writes this with output.write_line: stdout, no [ERROR].
		fmt.Printf("No hook with id `%s` in stage `%s`\n", opts.HookID, opts.HookStage)
		result.Errors++
		return result
	}

	// Every status line in a run shares one width, from the longest name, as
	// upstream's _compute_cols does.
	names := make([]string, len(hooksToRun))
	for i, h := range hooksToRun {
		names[i] = h.Name
	}
	cols := output.ReportCols(names)
	verbose := func(h *Hook) bool { return opts.Verbose || h.Verbose }

	for _, h := range hooksToRun {
		select {
		case <-ctx.Done():
			return result
		default:
		}

		// Check minimum_pre_commit_version.
		if h.MinimumPreCommitVersion != "" && h.MinimumPreCommitVersion != "0" {
			if !checkMinVersion(h.MinimumPreCommitVersion) {
				output.PrintHookHeader(h.Name, cols, output.ResultError)
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
			output.PrintHookSkipped(h.Name, cols, false)
			output.PrintHookDetails(output.HookDetails{ID: h.ID, Verbose: verbose(h)})
			result.Skipped++
			continue
		}

		// Filter files by hook's patterns and types.
		matchedFiles := filterFiles(files, h)

		if len(matchedFiles) == 0 && !h.AlwaysRun {
			output.PrintHookSkipped(h.Name, cols, true)
			output.PrintHookDetails(output.HookDetails{ID: h.ID, Verbose: verbose(h)})
			result.Skipped++
			continue
		}

		// Get the language handler.
		lang, err := languages.Get(h.Language)
		if err != nil {
			output.PrintHookHeader(h.Name, cols, output.ResultError)
			output.Error("unsupported language %q: %v", h.Language, err)
			result.Errors++
			if shouldFailFast(r.cfg, h) {
				return result
			}
			continue
		}

		// Name and dots first, so a slow hook shows what is running.
		output.PrintHookStart(h.Name, cols)
		start := time.Now()

		// Handle meta hooks specially.
		if h.ID == "check-hooks-apply" || h.ID == "check-useless-excludes" {
			metaExit, metaOut := r.runMetaHook(h, files)
			elapsed := time.Since(start)
			output.PrintHookStatus(metaExit == 0)
			output.PrintHookDetails(output.HookDetails{
				ID: h.ID, Verbose: verbose(h), Duration: &elapsed,
				ExitCode: metaExit, Output: metaOut, LogFile: h.LogFile,
			})
			if metaExit != 0 {
				result.Failed++
			} else {
				result.Passed++
			}
			continue
		}

		// Determine file args to pass.
		var fileArgs []string
		if h.PassFilenames {
			fileArgs = matchedFiles
		}

		// Snapshot the working tree's diff against the index: a hook that
		// changes it has modified files, whether or not it was passed them.
		diffBefore := r.worktreeDiff()

		// Run the hook using xargs for batching.
		var exitCode int
		var hookOutput []byte
		exitCode, hookOutput, err = runHookXargs(ctx, lang, h, fileArgs, r.root, opts.Jobs)
		if ctx.Err() != nil {
			// Interrupted mid-hook: upstream's KeyboardInterrupt leaves the
			// name and dots on the line with no status, and the error
			// handler's message completes it.
			return result
		}
		if err != nil {
			if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, fs.ErrNotExist) {
				fmt.Println()
				output.Error("hook execution error: %v", err)
				result.Errors++
				if shouldFailFast(r.cfg, h) {
					return result
				}
				continue
			}
			// Upstream reports a missing executable as the hook failing,
			// with its own wording as the output.
			exe := h.Entry
			if parts := languages.ParseEntry(h.Entry); len(parts) > 0 {
				exe = parts[0]
			}
			exitCode, hookOutput = 1, []byte(fmt.Sprintf("Executable %#q not found", exe))
		}
		elapsed := time.Since(start)

		filesModified := !bytes.Equal(diffBefore, r.worktreeDiff())
		failed := exitCode != 0 || filesModified

		output.PrintHookStatus(!failed)
		output.PrintHookDetails(output.HookDetails{
			ID: h.ID, Verbose: verbose(h), Duration: &elapsed,
			ExitCode: exitCode, FilesModified: filesModified,
			Output: hookOutput, LogFile: h.LogFile,
		})
		if failed {
			result.Failed++
			if shouldFailFast(r.cfg, h) {
				return result
			}
		} else {
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

// runHookXargs runs a hook the way upstream's lang_base.run_xargs and
// xargs.xargs do: the files are partitioned into runs of at most
// max(4, ceil(files/jobs)) arguments that also fit the platform's command
// length, the partitions run on min(partitions, jobs) workers, and their
// output is concatenated in partition order with the exit code of largest
// magnitude. jobs is 1 for a require_serial hook. All execution goes through
// lang.Run so language-specific environment setup (e.g. a virtualenv's PATH)
// applies to every partition.
func runHookXargs(
	ctx context.Context,
	lang languages.Language,
	h *Hook,
	fileArgs []string,
	workDir string,
	jobs int,
) (int, []byte, error) {
	ctx = languages.WithLocalHook(ctx, h.IsLocal())
	// fail never reaches xargs upstream, and pygrep runs in-process here, so
	// a partition would only split output that upstream prints in one piece.
	if len(fileArgs) == 0 || h.Language == "fail" || h.Language == "pygrep" {
		return lang.Run(ctx, h.RepoDir, workDir, h.Entry, h.Args, fileArgs, h.LanguageVersion)
	}

	fileArgs = shuffledFileArgs(h.Language, h.RequireSerial, fileArgs)

	concurrency := 1
	if !h.RequireSerial {
		concurrency = targetConcurrency(jobs)
	}

	cmd := hookCommand(h)
	partitions, err := xargs.Partition(cmd, fileArgs, concurrency, xargs.MaxLength(cmd))
	if err != nil {
		return -1, nil, err
	}

	type partitionResult struct {
		err      error
		output   []byte
		exitCode int
	}
	results := make([]partitionResult, len(partitions))
	run := func(i int) {
		exitCode, out, err := lang.Run(ctx, h.RepoDir, workDir, h.Entry, h.Args, partitions[i], h.LanguageVersion)
		results[i] = partitionResult{exitCode: exitCode, output: out, err: err}
	}

	if threads := min(len(partitions), concurrency); threads <= 1 {
		for i := range partitions {
			run(i)
		}
	} else {
		var wg sync.WaitGroup
		sem := make(chan struct{}, threads)
		for i := range partitions {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				run(i)
			})
		}
		wg.Wait()
	}

	var allOutput []byte
	exitCode := 0
	for _, r := range results {
		if r.err != nil {
			return -1, allOutput, r.err
		}
		allOutput = append(allOutput, r.output...)
		if absInt(r.exitCode) > absInt(exitCode) {
			exitCode = r.exitCode
		}
	}
	return exitCode, allOutput, nil
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// hookCommand approximates the command upstream partitions a hook's files
// against: the split entry plus args, with the executable resolved on PATH
// as parse_shebang.normalize_cmd resolves it. Only its length matters, and
// only when a partition nears the platform's command-length limit.
func hookCommand(h *Hook) []string {
	cmd := append(languages.ParseEntry(h.Entry), h.Args...)
	if len(cmd) > 0 && !strings.ContainsAny(cmd[0], `/\`) {
		if exe, err := exec.LookPath(cmd[0]); err == nil {
			cmd[0] = exe
		}
	}
	return cmd
}

// targetConcurrency is the number of partitions a non-serial hook targets.
// It is upstream's lang_base.target_concurrency (PRE_COMMIT_NO_CONCURRENCY,
// then TRAVIS, then the CPU count) unless --jobs gave an explicit count,
// which PRE_COMMIT_NO_CONCURRENCY still overrides.
func targetConcurrency(jobs int) int {
	if _, ok := os.LookupEnv("PRE_COMMIT_NO_CONCURRENCY"); ok {
		return 1
	}
	if jobs > 0 {
		return jobs
	}
	return xargs.TargetConcurrency()
}

// worktreeDiff returns the working tree's diff against the host index, the
// same command upstream pre-commit compares around each hook to decide that
// "files were modified by this hook". Outside a repository it returns nil both
// times, so nothing is reported.
func (r *Runner) worktreeDiff() []byte {
	cmd := exec.Command("git", "diff", "--no-ext-diff", "--no-textconv", "--ignore-submodules")
	cmd.Dir = r.root
	cmd.Env = git.HostIndexEnv()
	out, _ := cmd.Output()
	return out
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
	// Dev builds inject `git describe` output (e.g. "v4.6.0-1-gabc123");
	// strip the leading "v" so the first component parses as a number.
	v = strings.TrimPrefix(v, "v")
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

// installTask represents a single environment install job.
type installTask struct {
	hook *Hook
	lang languages.Language
}

// InstallEnvironments installs environments for all provided hooks.
// Installs are run in parallel since each operates on a separate directory.
//
// lock, when non-nil, is the store's exclusive lock, taken around the
// installs as upstream's install_hook_envs does. Without it two pre-commit
// processes sharing a store (two commits in sibling worktrees) each install
// into the same environment directory at once, and one's cleanup removes the
// other's half-built module cache.
func InstallEnvironments(_ context.Context, hooks []*Hook, lock func() (func(), error)) error {
	tasks, err := environmentsToInstall(hooks)
	if err != nil || len(tasks) == 0 {
		return err
	}
	if lock != nil {
		unlock, err := lock()
		if err != nil {
			return fmt.Errorf("failed to acquire store lock: %w", err)
		}
		defer unlock()
		// Another process may have already completed this work.
		if tasks, err = environmentsToInstall(hooks); err != nil || len(tasks) == 0 {
			return err
		}
	}
	for _, t := range tasks {
		// Leftovers at the path (a stale install state, a partial install)
		// are removed before installing, as upstream's _hook_install does.
		envPath := languages.EnvPath(t.hook.RepoDir, t.lang, t.hook.LanguageVersion)
		if err := fsutil.RemoveAll(envPath); err != nil {
			return fmt.Errorf("failed to clear %s before installing: %w", envPath, err)
		}
	}
	return installTasks(tasks)
}

// environmentsToInstall is upstream's _need_installed: one task per distinct
// install key whose environment is not already installed with the same
// dependencies.
func environmentsToInstall(hooks []*Hook) ([]installTask, error) {
	// Deduplicate and filter to only hooks that need installation.
	seen := make(map[string]bool)
	var tasks []installTask

	for _, h := range hooks {
		key := h.InstallKey()
		if seen[key] {
			continue
		}
		seen[key] = true

		lang, err := languages.Get(h.Language)
		if err != nil {
			return nil, fmt.Errorf("unsupported language %q for hook %q: %w", h.Language, h.ID, err)
		}

		if lang.EnvironmentDir() == "" {
			continue
		}

		envDir := h.RepoDir
		if envDir == "" {
			continue
		}

		envPath := languages.EnvPath(envDir, lang, h.LanguageVersion)
		stateFile := filepath.Join(envPath, installStateFile)
		// upstream's _hook_installed: the install state matches and the
		// language's health_check finds the env still healthy.
		if data, err := os.ReadFile(stateFile); err == nil && string(data) == h.InstallKey() &&
			lang.HealthCheck(envDir, h.LanguageVersion) == nil {
			continue // Already installed with same deps.
		}
		// A state mismatch (deps changed), or an env dir with no valid state
		// (a partial install, an interrupted run, or a CI cache restore that
		// dropped the state file): either way it is reinstalled, and the
		// caller clears the directory first. Installing OVER such leftovers
		// is not safe — golang's `go install ./...` runs in the hook repo with
		// the env (and its module cache) nested inside it, so a pre-populated
		// pkg/mod makes the pattern walk fail with "directory … outside main
		// module"; and a golang/cargo env is read-only, so fsutil.RemoveAll is
		// what clears it.
		tasks = append(tasks, installTask{hook: h, lang: lang})
	}
	return tasks, nil
}

// installTasks runs the installs and writes each one's install state.
func installTasks(tasks []installTask) error {
	// Run installs in parallel with bounded concurrency.
	maxWorkers := runtime.NumCPU()
	if maxWorkers > len(tasks) {
		maxWorkers = len(tasks)
	}
	if maxWorkers > 4 {
		maxWorkers = 4
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)
	errs := make([]error, len(tasks))

	for i, task := range tasks {
		output.Info("Installing environment for %s.", task.hook.Repo)
		output.Info("Once installed this environment will be reused.")
		output.Info("This may take a few minutes...")

		wg.Add(1)
		go func(idx int, t installTask) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			envPath := languages.EnvPath(t.hook.RepoDir, t.lang, t.hook.LanguageVersion)
			if err := t.lang.InstallEnvironment(
				t.hook.RepoDir,
				t.hook.LanguageVersion,
				t.hook.AdditionalDependencies,
			); err != nil {
				_ = fsutil.RemoveAll(envPath)
				errs[idx] = fmt.Errorf("failed to install environment for hook %q: %w", t.hook.ID, err)
				return
			}
			// As upstream's _hook_install: a fresh env must be healthy.
			if healthErr := t.lang.HealthCheck(t.hook.RepoDir, t.hook.LanguageVersion); healthErr != nil {
				errs[idx] = errors.Join(fmt.Errorf(
					"BUG: expected environment for %s to be healthy immediately after install, "+
						"please open an issue describing your environment\n\nmore info:\n\n%v",
					t.hook.Language, healthErr,
				), fsutil.RemoveAll(envPath))
				return
			}

			// Write install state file.
			stateFile := filepath.Join(envPath, installStateFile)
			stateDir := filepath.Dir(stateFile)
			os.MkdirAll(stateDir, 0o755)
			if err := os.WriteFile(stateFile, []byte(t.hook.InstallKey()), 0o644); err != nil {
				output.Warn("Failed to write install state: %v", err)
			}
		}(i, task)
	}

	wg.Wait()

	// Return the first error encountered.
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// ShowDiffOnFailure prints what the hooks changed, as upstream's _run_hooks
// does: only when there is a diff to show, with upstream's guidance lines
// first under --all-files, and before unstaged changes are restored, so the
// diff is the hooks' alone.
func ShowDiffOnFailure(allFiles bool) {
	check := exec.Command("git", "diff", "--no-ext-diff", "--no-textconv", "--ignore-submodules")
	check.Env = git.HostIndexEnv()
	if out, err := check.Output(); err != nil || len(out) == 0 {
		return
	}
	if allFiles {
		fmt.Println("pre-commit hook(s) made changes.\n" +
			"If you are seeing this message in CI, reproduce locally with: `pre-commit run --all-files`.\n" +
			"To run `pre-commit` as part of git workflow, use `pre-commit install`.")
	}
	fmt.Println("All changes made by hooks:")
	useColor := "never"
	if output.UseColor() {
		useColor = "always"
	}
	cmd := exec.Command("git", "--no-pager", "diff", "--no-ext-diff", "--color="+useColor)
	cmd.Env = git.HostIndexEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
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
