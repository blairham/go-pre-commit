package hook

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"sync"
	"time"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/hook/commands"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
	"github.com/blairham/go-pre-commit/pkg/hook/formatting"
	"github.com/blairham/go-pre-commit/pkg/hook/matching"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

// Orchestrator coordinates hook execution using the new sub-packages
type Orchestrator struct {
	ctx       *execution.Context
	repoMgr   *repository.Manager
	executor  *execution.Executor
	formatter *formatting.Formatter
	matcher   *matching.Matcher
	builder   *commands.Builder
}

// hookResultOrc represents the result of running a single hook in parallel for the orchestrator
type hookResultOrc struct {
	err    error
	result execution.Result
	index  int
}

// NewOrchestrator creates a new hook orchestrator
func NewOrchestrator(ctx *execution.Context) *Orchestrator {
	var repoMgr *repository.Manager

	// Use repository manager from context if available
	if ctx.RepoManager != nil {
		if mgr, ok := ctx.RepoManager.(*repository.Manager); ok {
			repoMgr = mgr
		}
	}

	// Fallback: create new repository manager if not provided
	if repoMgr == nil {
		var err error
		repoMgr, err = repository.NewManager()
		if err != nil {
			// If we can't create the repository manager, create a basic orchestrator without it
			// This allows local and meta hooks to still work
			repoMgr = nil
		}
	}

	return &Orchestrator{
		ctx:       ctx,
		repoMgr:   repoMgr,
		executor:  execution.NewExecutor(ctx),
		formatter: formatting.NewFormatter(ctx.Color, ctx.Verbose),
		matcher:   matching.NewMatcher(),
		builder:   commands.NewBuilder(ctx.RepoRoot),
	}
}

// RunHooks executes all hooks in the configuration using the new modular approach
func (o *Orchestrator) RunHooks(ctx context.Context) ([]execution.Result, error) {
	overallStart := time.Now()
	defer func() {
		execution.LogTiming("RunHooks overall", overallStart)
	}()

	// Collect hooks to run
	hooksToRun, err := o.collectHooksToRun(ctx)
	if err != nil {
		return nil, err
	}

	// Pre-initialize all environments
	if err := o.preInitializeEnvironments(ctx, hooksToRun); err != nil {
		return nil, fmt.Errorf("failed to pre-initialize environments: %w", err)
	}

	// Execute hooks
	return o.executeHooks(ctx, hooksToRun)
}

// collectHooksToRun gathers all hooks that should be executed based on stage and filters
func (o *Orchestrator) collectHooksToRun(ctx context.Context) ([]execution.RunItem, error) {
	collectStart := time.Now()
	defer func() {
		execution.LogTiming("hook collection", collectStart)
	}()

	hookStage := o.getHookStage()
	var hooksToRun []execution.RunItem

	for _, repo := range o.ctx.Config.Repos {
		repoHooks, err := o.collectRepoHooks(ctx, repo, hookStage)
		if err != nil {
			return nil, err
		}
		hooksToRun = append(hooksToRun, repoHooks...)
	}

	return hooksToRun, nil
}

// getHookStage returns the hook stage to run, defaulting to "pre-commit"
func (o *Orchestrator) getHookStage() string {
	if o.ctx.HookStage == "" {
		return "pre-commit"
	}
	return o.ctx.HookStage
}

// collectRepoHooks collects hooks from a single repository
func (o *Orchestrator) collectRepoHooks(
	ctx context.Context,
	repo config.Repo,
	hookStage string,
) ([]execution.RunItem, error) {
	hooksToRun := make([]execution.RunItem, 0, len(repo.Hooks))

	for _, hook := range repo.Hooks {
		if !o.shouldRunHook(hook, hookStage) {
			continue
		}

		runItem, err := o.createRunItem(ctx, repo, hook)
		if err != nil {
			return nil, err
		}

		hooksToRun = append(hooksToRun, runItem)
	}

	return hooksToRun, nil
}

// shouldRunHook determines if a hook should be executed based on stage and ID filters
func (o *Orchestrator) shouldRunHook(hook config.Hook, hookStage string) bool {
	if !o.shouldRunHookForStage(hook, hookStage) {
		return false
	}

	if len(o.ctx.HookIDs) > 0 && !o.shouldRunHookByID(hook.ID) {
		return false
	}

	return true
}

// createRunItem creates a RunItem for a hook
func (o *Orchestrator) createRunItem(
	ctx context.Context,
	repo config.Repo,
	hook config.Hook,
) (execution.RunItem, error) {
	repoPathStart := time.Now()
	repoPath, err := o.getRepoPathForHook(ctx, repo, hook)
	execution.LogTiming(fmt.Sprintf("getRepoPathForHook for %s", hook.ID), repoPathStart)
	if err != nil {
		return execution.RunItem{}, fmt.Errorf(
			"failed to get repository path for hook %s: %w",
			hook.ID,
			err,
		)
	}

	mergedHook, err := o.mergeWithRepositoryHook(hook, repo, repoPath)
	if err != nil {
		return execution.RunItem{}, fmt.Errorf(
			"failed to merge hook definition for %s: %w",
			hook.ID,
			err,
		)
	}

	return execution.RunItem{
		Repo:     repo,
		Hook:     mergedHook,
		RepoPath: repoPath,
	}, nil
}

// executeHooks runs the collected hooks either in parallel or sequentially
func (o *Orchestrator) executeHooks(ctx context.Context, hooksToRun []execution.RunItem) ([]execution.Result, error) {
	preInitStart := time.Now()
	if err := o.preInitializeEnvironments(ctx, hooksToRun); err != nil {
		return nil, fmt.Errorf("failed to pre-initialize environments: %w", err)
	}
	execution.LogTiming("pre-initialize environments", preInitStart)

	runStart := time.Now()
	defer func() {
		execution.LogTiming("hook execution", runStart)
	}()

	if o.ctx.Parallel > 1 && !o.hasSerialRequiredHooks(hooksToRun) {
		return o.runHooksParallel(ctx, hooksToRun)
	}
	return o.runHooksSequential(ctx, hooksToRun)
}

// Helper methods that will be moved from runner.go gradually

// shouldRunHookForStage checks if a hook should run for the given stage
func (o *Orchestrator) shouldRunHookForStage(hook config.Hook, stage string) bool {
	// If no stages are specified, hook runs for all stages
	if len(hook.Stages) == 0 {
		return true
	}

	// Check if the hook is configured for this stage
	return slices.Contains(hook.Stages, stage)
}

// shouldRunHookByID checks if a hook should run based on hook ID filtering
func (o *Orchestrator) shouldRunHookByID(hookID string) bool {
	return slices.Contains(o.ctx.HookIDs, hookID)
}

// hasSerialRequiredHooks checks if any hooks require serial execution
func (o *Orchestrator) hasSerialRequiredHooks(hooksToRun []execution.RunItem) bool {
	for _, hookData := range hooksToRun {
		if hookData.Hook.RequireSerial {
			return true
		}
	}
	return false
}

// getRepoPathForHook gets the repository path for a hook, handling setup if needed
func (o *Orchestrator) getRepoPathForHook(
	ctx context.Context,
	repo config.Repo,
	hook config.Hook,
) (string, error) {
	start := time.Now()
	defer func() {
		execution.LogTiming("getRepoPathForHook total", start)
	}()

	if o.repoMgr == nil {
		execution.LogTiming("getRepoPathForHook (no repo manager)", start)
		return o.ctx.RepoRoot, nil
	}

	// Handle local and meta repositories
	checkStart := time.Now()
	isLocal := o.repoMgr.IsLocalRepo(repo)
	isMeta := o.repoMgr.IsMetaRepo(repo)
	execution.LogTiming("repository type check", checkStart)

	if isLocal || isMeta {
		execution.LogTiming("getRepoPathForHook (local/meta)", start)
		return o.ctx.RepoRoot, nil
	}

	// Handle remote repositories with dependency-aware caching
	cloneStart := time.Now()
	repoPath, err := o.repoMgr.CloneOrUpdateRepoWithDeps(ctx, repo, hook.AdditionalDeps)
	execution.LogTiming("CloneOrUpdateRepoWithDeps", cloneStart)

	if err != nil {
		return "", fmt.Errorf("failed to setup repository: %w", err)
	}

	return repoPath, nil
}

func (o *Orchestrator) preInitializeEnvironments(
	ctx context.Context,
	hooksToRun []execution.RunItem,
) error {
	if o.repoMgr == nil {
		return nil // No repository manager, skip pre-initialization
	}

	// Convert to the format expected by the repository manager
	hookEnvData := make([]config.HookEnvItem, 0, len(hooksToRun))

	for _, hookData := range hooksToRun {
		hookEnvData = append(hookEnvData, config.HookEnvItem{
			Hook:     hookData.Hook,
			Repo:     hookData.Repo,
			RepoPath: hookData.RepoPath,
		})
	}

	return o.repoMgr.PreInitializeHookEnvironments(ctx, hookEnvData)
}

func (o *Orchestrator) runHooksSequential(
	ctx context.Context,
	hooksToRun []execution.RunItem,
) ([]execution.Result, error) {
	results := make([]execution.Result, 0, len(hooksToRun))

	for _, hookData := range hooksToRun {
		result, err := o.runHookWithPath(ctx, hookData.Hook, hookData.Repo, hookData.RepoPath)
		if err != nil {
			return results, fmt.Errorf("failed to run hook %s: %w", hookData.Hook.ID, err)
		}
		results = append(results, result)

		// Fail fast if enabled and hook failed
		if o.ctx.Config.FailFast && !result.Success {
			return results, nil
		}
	}

	return results, nil
}

func (o *Orchestrator) runHooksParallel(
	ctx context.Context,
	hooksToRun []execution.RunItem,
) ([]execution.Result, error) {
	resultsChan := make(chan hookResultOrc, len(hooksToRun))

	o.startHookWorkers(ctx, hooksToRun, resultsChan)

	results, firstError := o.collectResults(resultsChan, len(hooksToRun))

	if firstError != nil {
		return results, firstError
	}

	return o.handleFailFast(results)
}

// startHookWorkers starts goroutines to execute hooks in parallel
func (o *Orchestrator) startHookWorkers(
	ctx context.Context,
	hooksToRun []execution.RunItem,
	resultsChan chan hookResultOrc,
) {
	semaphore := make(chan struct{}, o.ctx.Parallel)
	var wg sync.WaitGroup

	// Start workers
	for i, hookData := range hooksToRun {
		wg.Add(1)
		go func(index int, hook config.Hook, repo config.Repo, repoPath string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result, err := o.runHookWithPath(ctx, hook, repo, repoPath)
			resultsChan <- hookResultOrc{err: err, result: result, index: index}
		}(i, hookData.Hook, hookData.Repo, hookData.RepoPath)
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
}

// collectResults collects results from the results channel
func (o *Orchestrator) collectResults(
	resultsChan chan hookResultOrc,
	expectedCount int,
) ([]execution.Result, error) {
	results := make([]execution.Result, expectedCount)
	var firstError error

	for resultData := range resultsChan {
		if resultData.err != nil && firstError == nil {
			firstError = resultData.err
		}
		results[resultData.index] = resultData.result
	}

	return results, firstError
}

// handleFailFast handles fail-fast logic and returns appropriate results
func (o *Orchestrator) handleFailFast(results []execution.Result) ([]execution.Result, error) {
	if !o.ctx.Config.FailFast {
		return results, nil
	}

	for _, result := range results {
		if !result.Success {
			return o.getCompletedResults(results), nil
		}
	}

	return results, nil
}

// getCompletedResults filters out incomplete results and preserves order
func (o *Orchestrator) getCompletedResults(results []execution.Result) []execution.Result {
	var orderedResults []execution.Result
	for i := range results {
		if results[i].Hook.ID != "" { // Only include completed results
			orderedResults = append(orderedResults, results[i])
		}
	}
	return orderedResults
}

// runHookWithPath executes a single hook with a pre-determined repository path
func (o *Orchestrator) runHookWithPath(
	ctx context.Context,
	hook config.Hook,
	repo config.Repo,
	repoPath string,
) (execution.Result, error) {
	start := time.Now()
	result := execution.Result{Hook: hook}

	// Setup hook definition (port this from runner.go gradually)
	setupStart := time.Now()
	actualHook, hookSetupErr := o.setupHookDefinition(hook, repo)
	if hookSetupErr != nil {
		return result, hookSetupErr
	}
	execution.LogTiming("hook definition setup", setupStart)

	result.Hook = actualHook

	// Get files for hook using matching sub-package
	filesStart := time.Now()
	result.Files = o.getFilesForHook(actualHook)
	execution.LogTiming("getting files for hook", filesStart)

	// Check if hook should be skipped
	if shouldSkip := o.shouldSkipHook(actualHook, result.Files, start); shouldSkip.Skip {
		return shouldSkip.Result, nil
	}

	// Set up environment once and reuse it
	envStart := time.Now()
	var hookEnv map[string]string
	if o.repoMgr != nil {
		var envErr error
		hookEnv, envErr = o.repoMgr.SetupHookEnvironment(actualHook, repo, repoPath)
		if envErr != nil {
			return result, fmt.Errorf("failed to setup hook environment: %w", envErr)
		}
	}
	execution.LogTiming("environment setup", envStart)

	// Build command using the commands sub-package
	buildStart := time.Now()
	cmd, buildErr := o.buildCommandWithEnv(actualHook, result.Files, repoPath, repo, hookEnv)
	if buildErr != nil {
		return result, fmt.Errorf("failed to build command: %w", buildErr)
	}
	execution.LogTiming("command building", buildStart)

	// Setup command environment
	cmdEnvStart := time.Now()
	o.setupCommandEnvironmentWithEnv(cmd, actualHook, repo, repoPath, hookEnv)
	execution.LogTiming("command environment setup", cmdEnvStart)

	// Execute command using the executor
	execStart := time.Now()
	output, execErr := o.executor.ExecuteWithTimeout(ctx, cmd)
	execution.LogTiming("command execution", execStart)

	// Process result using the executor
	processStart := time.Now()
	o.executor.ProcessExecutionResult(&result, output, execErr, hook, start)
	execution.LogTiming("result processing", processStart)

	execution.LogTiming("runHookWithPath total", start)

	return result, nil
}

// setupHookDefinition handles meta hook merging and returns the actual hook to execute
func (o *Orchestrator) setupHookDefinition(
	hook config.Hook,
	repo config.Repo,
) (config.Hook, error) {
	if o.repoMgr == nil || !o.repoMgr.IsMetaRepo(repo) {
		return hook, nil
	}

	metaHook, exists := o.repoMgr.GetMetaHook(hook.ID)
	if !exists {
		return hook, fmt.Errorf("unknown meta hook: %s", hook.ID)
	}

	return o.mergeHookDefinitions(metaHook, hook), nil
}

func (o *Orchestrator) mergeHookDefinitions(base, override config.Hook) config.Hook {
	result := base // Start with base definition

	// Override fields that are explicitly set in config
	applyStringOverride(&result.Name, override.Name)
	applyStringOverride(&result.Entry, override.Entry)
	applyStringOverride(&result.Language, override.Language)
	applyStringOverride(&result.Files, override.Files)
	applyStringOverride(&result.ExcludeRegex, override.ExcludeRegex)
	applySliceOverride(&result.Types, override.Types)
	applySliceOverride(&result.TypesOr, override.TypesOr)
	applySliceOverride(&result.ExcludeTypes, override.ExcludeTypes)
	applySliceOverride(&result.AdditionalDeps, override.AdditionalDeps)
	applySliceOverride(&result.Args, override.Args)
	applyBoolOverride(&result.AlwaysRun, override.AlwaysRun)
	applyBoolOverride(&result.Verbose, override.Verbose)
	applyStringOverride(&result.LogFile, override.LogFile)
	applyBoolPtrOverride(&result.PassFilenames, override.PassFilenames)
	applyStringOverride(&result.Description, override.Description)
	applyStringOverride(&result.LanguageVersion, override.LanguageVersion)
	applyStringOverride(&result.MinimumPreCommitVersion, override.MinimumPreCommitVersion)
	applyBoolOverride(&result.RequireSerial, override.RequireSerial)
	applySliceOverride(&result.Stages, override.Stages)

	return result
}

func (o *Orchestrator) getFilesForHook(hook config.Hook) []string {
	return o.matcher.GetFilesForHook(hook, o.ctx.Files, o.ctx.AllFiles)
}

func (o *Orchestrator) shouldSkipHook(
	hook config.Hook,
	files []string,
	_ time.Time,
) execution.SkipResult {
	if len(files) == 0 && !hook.AlwaysRun {
		return execution.SkipResult{
			Skip: true,
			Result: execution.Result{
				Hook:     hook,
				Files:    files,
				Success:  true,
				Skipped:  true,
				Duration: 0,
			},
		}
	}
	return execution.SkipResult{Skip: false}
}

func (o *Orchestrator) buildCommandWithEnv(
	hook config.Hook,
	files []string,
	repoPath string,
	repo config.Repo,
	env map[string]string,
) (*exec.Cmd, error) {
	return o.builder.BuildCommand(hook, files, repoPath, repo, env)
}

func (o *Orchestrator) setupCommandEnvironmentWithEnv(
	cmd *exec.Cmd,
	hook config.Hook,
	repo config.Repo,
	repoPath string,
	hookEnv map[string]string,
) {
	cmd.Dir = o.ctx.RepoRoot

	if o.repoMgr != nil {
		if hookEnv != nil {
			// Use pre-setup environment
			for key, value := range hookEnv {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
			}
		} else {
			// Fall back to setting up environment
			o.addHookEnvironment(cmd, hook, repo, repoPath)
		}
	}

	o.addContextEnvironment(cmd)
}

// addHookEnvironment adds language-specific environment variables
func (o *Orchestrator) addHookEnvironment(
	cmd *exec.Cmd,
	hook config.Hook,
	repo config.Repo,
	repoPath string,
) map[string]string {
	hookEnv, envErr := o.repoMgr.SetupHookEnvironment(hook, repo, repoPath)
	if envErr != nil {
		// Don't fail the hook, just log if verbose
		return nil
	}

	for key, value := range hookEnv {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	return hookEnv
}

// addContextEnvironment adds environment variables from execution context
func (o *Orchestrator) addContextEnvironment(cmd *exec.Cmd) {
	if o.ctx.Environment != nil {
		for key, value := range o.ctx.Environment {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}
}

// applyStringOverride applies a string override if the override value is not empty
func applyStringOverride(target *string, override string) {
	if override != "" {
		*target = override
	}
}

// applySliceOverride applies a slice override if the override slice is not empty
func applySliceOverride[T any](target *[]T, override []T) {
	if len(override) > 0 {
		*target = override
	}
}

// applyBoolPtrOverride applies a bool pointer override if the override is not nil
func applyBoolPtrOverride(target **bool, override *bool) {
	if override != nil {
		*target = override
	}
}

// mergeWithRepositoryHook merges a config hook with its repository definition to get complete hook information
func (o *Orchestrator) mergeWithRepositoryHook(
	configHook config.Hook,
	repo config.Repo,
	repoPath string,
) (config.Hook, error) {
	// For local and meta repositories, return the config hook as-is
	if repo.Repo == "local" || repo.Repo == "meta" {
		// For meta hooks, try to get the definition and merge
		if repo.Repo == "meta" && o.repoMgr != nil {
			if metaHook, found := o.repoMgr.GetMetaHook(configHook.ID); found {
				return o.mergeHookDefinitions(metaHook, configHook), nil
			}
		}
		return configHook, nil
	}

	// For regular repositories, get the hook definition from .pre-commit-hooks.yaml
	if o.repoMgr == nil {
		return configHook, nil // No repository manager available
	}

	repoHook, found := o.repoMgr.GetRepositoryHook(repoPath, configHook.ID)
	if !found {
		return configHook, fmt.Errorf("hook %s not found in repository %s", configHook.ID, repo.Repo)
	}

	// Merge repository hook (base) with config hook (override)
	return o.mergeHookDefinitions(repoHook, configHook), nil
}

// applyBoolOverride applies a bool override if the override is true
func applyBoolOverride(target *bool, override bool) {
	if override {
		*target = override
	}
}
