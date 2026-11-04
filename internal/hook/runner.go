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
	// Set PRE_COMMIT=1 environment variable.
	os.Setenv("PRE_COMMIT", "1")
	defer os.Unsetenv("PRE_COMMIT")

	// Set hook-stage-specific environment variables.
	r.setEnvVars(opts)
	defer r.unsetEnvVars()

	st := runState{skip: skippedHooks(opts.SkipList), opts: &opts}

	// Apply top-level files/exclude filters from config.
	st.files = opts.Files
	if r.cfg.Files != "" || r.cfg.Exclude != "" {
		st.files = filterByIncludeExclude(st.files, r.cfg.Files, r.cfg.Exclude)
	}

	hooksToRun := r.hooksFor(opts.HookID, opts.HookStage)
	if len(hooksToRun) == 0 && opts.HookID != "" {
		// Upstream writes this with output.write_line: stdout, no [ERROR].
		fmt.Printf("No hook with id `%s` in stage `%s`\n", opts.HookID, opts.HookStage)
		st.result.Errors++
		return st.result
	}

	// Every status line in a run shares one width, from the longest name, as
	// upstream's _compute_cols does.
	names := make([]string, len(hooksToRun))
	for i, h := range hooksToRun {
		names[i] = h.Name
	}
	st.cols = output.ReportCols(names)

	for _, h := range hooksToRun {
		select {
		case <-ctx.Done():
			return st.result
		default:
		}
		if r.runHook(ctx, &st, h) {
			return st.result
		}
	}
	return st.result
}

// runState is what the hooks of one Run share: the hooks to skip, the
// options, the files, the tally and the width of the status lines.
type runState struct {
	skip   map[string]bool
	opts   *RunOptions
	files  []string
	result RunResult
	cols   int
}

// hookRun is one hook being run: when it started, its language and the
// files it matched.
type hookRun struct {
	start time.Time
	lang  languages.Language
	h     *Hook
	files []string
}

// skippedHooks is the ids and aliases of the hooks not to run: SKIP's,
// comma-separated, and those given.
func skippedHooks(skipList []string) map[string]bool {
	skip := make(map[string]bool)
	if skipEnv := os.Getenv("SKIP"); skipEnv != "" {
		for _, id := range strings.Split(skipEnv, ",") {
			skip[strings.TrimSpace(id)] = true
		}
	}
	for _, id := range skipList {
		skip[id] = true
	}
	return skip
}

// hooksFor is the hooks to run: with id or alias hookID, and for stage,
// when they are given.
func (r *Runner) hooksFor(hookID string, stage config.Stage) []*Hook {
	var hooks []*Hook
	for _, h := range r.hooks {
		if hookID != "" && h.ID != hookID && h.Alias != hookID {
			continue
		}
		if stage != "" && !h.MatchesStage(stage) {
			continue
		}
		hooks = append(hooks, h)
	}
	return hooks
}

// runHook runs h, or reports why it does not, and tallies the outcome. It
// reports whether the run stops there: after a failure with fail_fast, or
// when ctx is done while the hook runs.
func (r *Runner) runHook(ctx context.Context, st *runState, h *Hook) (stop bool) {
	// Check minimum_pre_commit_version.
	if h.MinimumPreCommitVersion != "" && h.MinimumPreCommitVersion != "0" &&
		!checkMinVersion(h.MinimumPreCommitVersion) {
		output.PrintHookHeader(h.Name, st.cols, output.ResultError)
		output.Error("hook requires pre-commit >= %s", h.MinimumPreCommitVersion)
		return r.hookError(st, h)
	}

	// Check if skipped.
	if st.skip[h.ID] || (h.Alias != "" && st.skip[h.Alias]) {
		st.skipped(h, false)
		return false
	}

	// Filter files by hook's patterns and types.
	matchedFiles := filterFiles(st.files, h)

	if len(matchedFiles) == 0 && !h.AlwaysRun {
		st.skipped(h, true)
		return false
	}

	// Get the language handler.
	lang, err := languages.Get(h.Language)
	if err != nil {
		output.PrintHookHeader(h.Name, st.cols, output.ResultError)
		output.Error("unsupported language %q: %v", h.Language, err)
		return r.hookError(st, h)
	}

	// Name and dots first, so a slow hook shows what is running.
	output.PrintHookStart(h.Name, st.cols)
	hr := &hookRun{start: time.Now(), lang: lang, h: h, files: matchedFiles}

	// Handle meta hooks specially.
	if h.ID == config.MetaHookCheckHooksApply || h.ID == config.MetaHookCheckUselessExcludes {
		r.runMeta(st, hr)
		return false
	}
	return r.runCommand(ctx, st, hr)
}

// hookError tallies a hook that could not run; the run stops there with
// fail_fast.
func (r *Runner) hookError(st *runState, h *Hook) bool {
	st.result.Errors++
	return shouldFailFast(r.cfg, h)
}

// skipped reports and tallies a skipped hook: skipped outright, or with
// no files to check.
func (st *runState) skipped(h *Hook, noFiles bool) {
	output.PrintHookSkipped(h.Name, st.cols, noFiles)
	output.PrintHookDetails(output.HookDetails{ID: h.ID, Verbose: st.verbose(h)})
	st.result.Skipped++
}

func (st *runState) verbose(h *Hook) bool { return st.opts.Verbose || h.Verbose }

// runMeta runs one of the meta hooks, which are built in.
func (r *Runner) runMeta(st *runState, hr *hookRun) {
	h := hr.h
	metaExit, metaOut := r.runMetaHook(h, st.files)
	elapsed := time.Since(hr.start)
	output.PrintHookStatus(metaExit == 0)
	output.PrintHookDetails(output.HookDetails{
		ID: h.ID, Verbose: st.verbose(h), Duration: &elapsed,
		ExitCode: metaExit, Output: metaOut, LogFile: h.LogFile,
	})
	if metaExit != 0 {
		st.result.Failed++
	} else {
		st.result.Passed++
	}
}

// runCommand runs a hook's entry over its files and reports it; the hook
// fails if it exits non-zero or changes the working tree.
func (r *Runner) runCommand(ctx context.Context, st *runState, hr *hookRun) (stop bool) {
	h := hr.h

	// Determine file args to pass.
	var fileArgs []string
	if h.PassFilenames {
		fileArgs = hr.files
	}

	// Snapshot the working tree's diff against the index: a hook that
	// changes it has modified files, whether or not it was passed them.
	diffBefore := r.worktreeDiff()

	// Run the hook using xargs for batching.
	exitCode, hookOutput, err := runHookXargs(ctx, hr.lang, h, fileArgs, r.root, st.opts.Jobs)
	if ctx.Err() != nil {
		// Interrupted mid-hook: upstream's KeyboardInterrupt leaves the
		// name and dots on the line with no status, and the error
		// handler's message completes it.
		return true
	}
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) && !errors.Is(err, fs.ErrNotExist) {
			fmt.Println()
			output.Error("hook execution error: %v", err)
			return r.hookError(st, h)
		}
		// Upstream reports a missing executable as the hook failing,
		// with its own wording as the output.
		exitCode, hookOutput = 1, notFoundOutput(h.Entry)
	}
	elapsed := time.Since(hr.start)

	filesModified := !bytes.Equal(diffBefore, r.worktreeDiff())
	failed := exitCode != 0 || filesModified

	output.PrintHookStatus(!failed)
	output.PrintHookDetails(output.HookDetails{
		ID: h.ID, Verbose: st.verbose(h), Duration: &elapsed,
		ExitCode: exitCode, FilesModified: filesModified,
		Output: hookOutput, LogFile: h.LogFile,
	})
	if !failed {
		st.result.Passed++
		return false
	}
	st.result.Failed++
	return shouldFailFast(r.cfg, h)
}

// notFoundOutput is upstream's output for a hook whose executable is
// missing.
func notFoundOutput(entry string) []byte {
	exe := entry
	if parts := languages.ParseEntry(entry); len(parts) > 0 {
		exe = parts[0]
	}
	return []byte(fmt.Sprintf("Executable %#q not found", exe))
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
	cmd := git.Command("diff", "--no-ext-diff", "--no-textconv", "--ignore-submodules")
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
		// upstream's _hook_installed: the install state matches and the
		// language's health_check finds the env still healthy.
		if installStateMatches(envPath, h) && lang.HealthCheck(envDir, h.LanguageVersion) == nil {
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

			if err := os.MkdirAll(envPath, 0o755); err != nil { // #nosec G301 -- as upstream's os.makedirs
				output.Warn("Failed to write install state: %v", err)
			} else if err := writeInstallState(envPath, t.hook.AdditionalDependencies); err != nil {
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
	check := git.Command("diff", "--no-ext-diff", "--no-textconv", "--ignore-submodules")
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
	cmd := git.Command("--no-pager", "diff", "--no-ext-diff", "--color="+useColor)
	cmd.Env = git.HostIndexEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// runMetaHook performs meta hook checks (check-hooks-apply, check-useless-excludes).
func (r *Runner) runMetaHook(metaHook *Hook, allFiles []string) (int, []byte) {
	switch metaHook.ID {
	case config.MetaHookCheckHooksApply:
		return r.checkHooksApply(allFiles)
	case config.MetaHookCheckUselessExcludes:
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
		if config.IsMetaHookID(h.ID) {
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
	if isUselessExclude(r.cfg.Exclude, allFiles) {
		msgs = append(msgs, fmt.Sprintf("The top-level exclude pattern %q does not match any files", r.cfg.Exclude))
		exitCode = 1
	}

	for _, h := range r.hooks {
		if config.IsMetaHookID(h.ID) {
			continue
		}
		if h.Exclude == "" || h.Exclude == "^$" {
			continue
		}
		// Check if exclude actually excludes anything from the files that
		// match the hook's include pattern and types.
		if isUselessExclude(h.Exclude, includedFiles(allFiles, h)) {
			msgs = append(msgs, fmt.Sprintf("The exclude pattern %q for hook %q does not match any files", h.Exclude, h.ID))
			exitCode = 1
		}
	}

	if exitCode != 0 {
		return exitCode, []byte(strings.Join(msgs, "\n") + "\n")
	}
	return 0, nil
}

// isUselessExclude reports whether exclude, a pattern that compiles and
// excludes anything at all, matches none of files.
func isUselessExclude(exclude string, files []string) bool {
	if exclude == "" || exclude == "^$" {
		return false
	}
	excludeRe, err := pcre.Compile(exclude)
	if err != nil {
		return false
	}
	for _, f := range files {
		if pcre.Match(excludeRe, f) {
			return false
		}
	}
	return true
}

// includedFiles is the files that match h's include pattern and types.
func includedFiles(allFiles []string, h *Hook) []string {
	var included []string
	var includeRe *regexp2.Regexp
	if re, err := pcre.Compile(h.Files); err == nil {
		includeRe = re
	}
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
	return included
}
