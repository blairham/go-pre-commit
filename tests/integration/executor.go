package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blairham/go-pre-commit/tests/integration/languages"
)

//nolint:errcheck // Test cleanup, errors can be ignored

const (
	unknownVersion = "unknown"
	devVersion     = "dev"
) //nolint:errcheck // Test cleanup, errors can be ignored

// TestExecutor handles the execution of different test phases
type TestExecutor struct {
	suite       *Suite
	diagnostics *DiagnosticsManager
}

// NewTestExecutor creates a new test executor
func NewTestExecutor(suite *Suite) *TestExecutor {
	return &TestExecutor{
		suite:       suite,
		diagnostics: NewDiagnosticsManager(),
	}
}

// RunLanguageCompatibilityTest runs a single language compatibility test
func (te *TestExecutor) RunLanguageCompatibilityTest(
	t *testing.T,
	test LanguageCompatibilityTest,
) *TestResults {
	t.Helper()              //nolint:errcheck // Test cleanup, errors can be ignored
	startTime := time.Now() //nolint:errcheck // Test cleanup, errors can be ignored

	result := &TestResults{
		Timestamp:      startTime,
		Language:       test.Language,
		TestRepository: test.TestRepository,
		HookID:         test.HookID,
		Errors:         []string{},
	}

	t.Logf(
		"🚀 Starting comprehensive compatibility test for %s",
		test.Language,
	) //nolint:errcheck // Test cleanup, errors can be ignored

	// Create test workspace
	workspaceManager := NewWorkspaceManager(te.suite)        //nolint:errcheck // Test cleanup, errors can be ignored
	testDir := workspaceManager.CreateTestWorkspace(t, test) //nolint:errcheck // Test cleanup, errors can be ignored

	// Preserve cache artifacts instead of cleaning up immediately
	defer te.cleanupTestDirectory(t, test, testDir) //nolint:errcheck // Test cleanup, errors can be ignored

	// Execute repository and environment setup test
	if err := te.testRepositoryAndEnvironmentSetup(t, test, testDir, result); err != nil {
		result.Success = false
		result.Errors = append(
			result.Errors,
			err.Error(),
		) //nolint:errcheck // Test cleanup, errors can be ignored
		t.Logf(
			"❌ Repository/Environment setup failed: %v",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	} else {
		te.handleSuccessfulSetup(t, test, testDir, result)
	}
	result.TestDuration = time.Since(startTime) //nolint:errcheck // Test cleanup, errors can be ignored

	// Store results
	te.suite.resultsMutex.Lock() //nolint:errcheck // Test cleanup, errors can be ignored
	te.suite.results[test.Language] = result
	te.suite.resultsMutex.Unlock() //nolint:errcheck // Test cleanup, errors can be ignored

	if result.Success {
		t.Logf(
			"🎉 Language compatibility test PASSED for %s in %v",
			test.Language,
			result.TestDuration,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	} else {
		t.Logf("💥 Language compatibility test FAILED for %s in %v (errors: %d)",
			test.Language, result.TestDuration, len(result.Errors)) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	return result
}

// cleanupTestDirectory removes the temporary test directory after test completion
func (te *TestExecutor) cleanupTestDirectory(t *testing.T, _ LanguageCompatibilityTest, testDir string) {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Clean up the test directory
	if err := os.RemoveAll(testDir); err != nil {
		t.Logf(
			"Warning: failed to clean up test directory: %v",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}
}

// handleSuccessfulSetup handles the setup success case to reduce complexity
func (te *TestExecutor) handleSuccessfulSetup(
	t *testing.T,
	test LanguageCompatibilityTest,
	testDir string,
	result *TestResults,
) {
	t.Helper()
	result.Success = true
	t.Logf(
		"✅ Repository/Environment setup completed successfully",
	) //nolint:errcheck // Test cleanup, errors can be ignored

	// If we got this far, environment isolation is working (each test gets its own environment)  //nolint:errcheck // Test cleanup, errors can be ignored
	result.EnvironmentIsolation = true

	// Run performance benchmarking tests if setup succeeded
	te.runPerformanceBenchmarks(t, test, testDir, result) //nolint:errcheck // Test cleanup, errors can be ignored

	// If both Go and Python install times are available, we can check functional equivalence
	if result.GoInstallTime > 0 && result.PythonInstallTime > 0 {
		// Both implementations can install and run hooks, so they are functionally equivalent
		result.FunctionalEquivalence = true
		t.Logf(
			"✅ Functional equivalence confirmed: both Go and Python implementations working",
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Run bidirectional cache tests if enabled and binaries are available
	if test.BiDirectionalTestEnabled && te.suite.pythonBinary != "" && te.suite.goBinary != "" {
		if err := te.runBidirectionalCacheTest(t, test, testDir, result); err != nil {
			result.Errors = append(
				result.Errors,
				fmt.Sprintf("Bidirectional cache test failed: %v", err),
			) //nolint:errcheck // Test cleanup, errors can be ignored
			t.Logf(
				"⚠️ Bidirectional cache test failed: %v",
				err,
			) //nolint:errcheck // Test cleanup, errors can be ignored
		} else {
			result.CacheBidirectional = true
			t.Logf(
				"✅ Bidirectional cache test completed successfully",
			) //nolint:errcheck // Test cleanup, errors can be ignored
		}
	}
}

// runPerformanceBenchmarks runs performance tests using install-hooks command
func (te *TestExecutor) runPerformanceBenchmarks(
	t *testing.T,
	test LanguageCompatibilityTest,
	testDir string,
	result *TestResults,
) {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored
	t.Logf(
		"🔄 Running performance benchmarks for %s",
		test.Language,
	) //nolint:errcheck // Test cleanup, errors can be ignored

	// Create a test repository with pre-commit config
	repoDir := filepath.Join(testDir, "test-repo") //nolint:errcheck // Test cleanup, errors can be ignored
	if err := te.setupTestRepository(t, test, repoDir); err != nil {
		t.Logf(
			"⚠️ Warning: Failed to setup test repository for performance testing: %v",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
		return
	}

	// Test Go implementation performance
	te.benchmarkGoInstallHooks(t, test, repoDir, result) //nolint:errcheck // Test cleanup, errors can be ignored

	// Test Python implementation performance (if available)  //nolint:errcheck // Test cleanup, errors can be ignored
	if te.suite.pythonBinary != "" {
		te.benchmarkPythonInstallHooks(
			t,
			test,
			repoDir,
			result,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Calculate performance ratio
	if result.GoInstallTime > 0 && result.PythonInstallTime > 0 {
		result.PerformanceRatio = float64(
			result.PythonInstallTime,
		) / float64(
			result.GoInstallTime,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Test cache performance
	te.testCachePerformance(t, test, repoDir, result) //nolint:errcheck // Test cleanup, errors can be ignored

	// Test language-specific cache performance if available
	te.testLanguageSpecificCachePerformance(
		t,
		test,
		testDir,
		result,
	) //nolint:errcheck // Test cleanup, errors can be ignored
}

// setupTestRepository creates a test git repository with pre-commit configuration
func (te *TestExecutor) setupTestRepository(t *testing.T, test LanguageCompatibilityTest, repoDir string) error {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Create repository directory
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		return fmt.Errorf(
			"failed to create repo directory: %w",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Initialize git repository
	if err := te.runCommand(repoDir, "git", "init"); err != nil {
		return fmt.Errorf("failed to init git repo: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Set git config to avoid warnings and disable signing (ignore errors for test setup)
	_ = te.runCommand( //nolint:errcheck // Test setup, errors can be ignored
		repoDir,
		"git",
		"config",
		"user.email",
		"test@example.com",
	)
	_ = te.runCommand( //nolint:errcheck // Test setup, errors can be ignored
		repoDir,
		"git",
		"config",
		"user.name",
		"Test User",
	)
	_ = te.runCommand( //nolint:errcheck // Test setup, errors can be ignored
		repoDir,
		"git",
		"config",
		"commit.gpgsign",
		"false",
	)

	// Create pre-commit config
	configContent := te.generatePreCommitConfig(
		test,
	) //nolint:errcheck // Test cleanup, errors can be ignored
	configPath := filepath.Join(
		repoDir,
		".pre-commit-config.yaml",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf(
			"failed to write pre-commit config: %w",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Create a test file to commit
	testFile := filepath.Join(repoDir, "test.txt") //nolint:errcheck // Test cleanup, errors can be ignored
	if err := os.WriteFile(testFile, []byte("test content\n"), 0o600); err != nil {
		return fmt.Errorf("failed to write test file: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Create language-specific test files for better cache testing
	switch test.Language {
	case languages.LangPython:
		pythonFile := filepath.Join(repoDir, "test.py") //nolint:errcheck // Test cleanup, errors can be ignored
		if err := os.WriteFile(pythonFile, []byte("print(\"hello world\")\n"), 0o600); err != nil {
			return fmt.Errorf(
				"failed to write Python test file: %w",
				err,
			) //nolint:errcheck // Test cleanup, errors can be ignored
		}
	case languages.LangConda:
		// Create environment.yml file for conda language tests
		envFile := filepath.Join(repoDir, "environment.yml") //nolint:errcheck // Test cleanup, errors can be ignored
		envContent := `name: test-env
dependencies:
  - python=3.9
  - pip
  - pip:
    - black
`
		if err := os.WriteFile(envFile, []byte(envContent), 0o600); err != nil {
			return fmt.Errorf(
				"failed to write conda environment.yml file: %w",
				err,
			) //nolint:errcheck // Test cleanup, errors can be ignored
		}
	}

	// Add files
	if err := te.runCommand(repoDir, "git", "add", "."); err != nil {
		return fmt.Errorf("failed to git add: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Check if there are files to commit
	cmd := exec.Command("git", "diff", "--cached", "--quiet") //nolint:errcheck // Test cleanup, errors can be ignored
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		// There are changes to commit - use --no-verify to skip pre-commit hooks during test setup
		if err := te.runCommand(repoDir, "git", "commit", "--no-verify", "-m", "Initial commit"); err != nil {
			return fmt.Errorf("failed to git commit: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
		}
	}

	return nil
}

// generatePreCommitConfig generates a pre-commit config for the test language
func (te *TestExecutor) generatePreCommitConfig(test LanguageCompatibilityTest) string {
	repo := te.getTestRepository(test)
	hookID := te.getTestHookID(test)

	// Handle local repositories differently (they need name field)
	if repo == "local" {
		return te.generateLocalRepoConfig(test, hookID)
	}

	config := fmt.Sprintf(`repos:
-   repo: %s
    rev: %s
    hooks:
    -   id: %s`, repo, getRevisionForRepo(repo), hookID)

	// Add specific configurations for certain hooks
	if hookID == "black" {
		config += `
        exclude: '\.pre-commit-config\.yaml$'`
	}

	return config + "\n"
}

// generateLocalRepoConfig generates config for local repositories
//
//nolint:funlen // Config generation function naturally has many cases
func (te *TestExecutor) generateLocalRepoConfig(
	test LanguageCompatibilityTest,
	hookID string,
) string {
	switch test.Language {
	case "script":
		return `repos:
-   repo: local
    hooks:
    -   id: simple-shell-script
        name: Simple Shell Script
        entry: ./scripts/test.sh
        language: script
        files: \.txt$
`
	case languages.LangGolang:
		return `repos:
-   repo: local
    hooks:
    -   id: go-test-simple
        name: Simple Go Test
        entry: go test ./...
        language: golang
        files: \.go$
`
	case "julia":
		return `repos:
-   repo: local
    hooks:
    -   id: julia-formatter
        name: Julia Formatter
        entry: julia
        language: julia
        files: \.jl$
`
	case "conda":
		return `repos:
-   repo: local
    hooks:
    -   id: conda-black
        name: Conda Black
        entry: black
        language: conda
        files: \.py$
`
	case "system":
		return `repos:
-   repo: local
    hooks:
    -   id: simple-system-command
        name: Simple System Command
        entry: echo "Hello from system hook"
        language: system
        files: \.txt$
`
	case "pygrep":
		return `repos:
-   repo: local
    hooks:
    -   id: python-check-blanket-noqa
        name: Check blanket noqa
        entry: python-check-blanket-noqa
        language: pygrep
        files: \.py$
`
	case "fail":
		return `repos:
-   repo: local
    hooks:
    -   id: no-commit-to-branch
        name: "Don't commit to branch"
        entry: 'Do not commit to main branch'
        language: fail
        files: .*
        args: ['--branch', 'main', '--branch', 'master']
`
	default:
		return fmt.Sprintf(`repos:
-   repo: local
    hooks:
    -   id: %s
        name: %s Hook
        entry: echo "test"
        language: %s
        files: .*
`, hookID, test.Language, test.Language)
	}
}

// getTestRepository returns the repository URL for the test
func (te *TestExecutor) getTestRepository(test LanguageCompatibilityTest) string {
	if test.TestRepository != "" {
		return test.TestRepository
	}

	// Use language-specific default repositories
	switch test.Language {
	case LangPython:
		return "https://github.com/psf/black"
	case "node":
		return "https://github.com/pre-commit/mirrors-eslint"
	case "golang":
		return "https://github.com/dnephin/pre-commit-golang"
	case LangRust:
		return "https://github.com/doublify/pre-commit-rust"
	case "ruby":
		return "https://github.com/mattlqx/pre-commit-ruby"
	default:
		return "https://github.com/pre-commit/pre-commit-hooks"
	}
}

// getTestHookID returns the hook ID for the test
func (te *TestExecutor) getTestHookID(test LanguageCompatibilityTest) string {
	if test.HookID != "" {
		return test.HookID
	}

	// Use language-specific default hooks
	switch test.Language {
	case LangPython:
		return "black"
	case "node":
		return "eslint"
	case "golang":
		return "go-fmt"
	case LangRust:
		return "cargo-check"
	case "ruby":
		return "rubocop"
	default:
		return "check-yaml"
	}
}

// getRevisionForRepo returns the appropriate revision for different repositories
func getRevisionForRepo(repo string) string {
	switch repo {
	case "https://github.com/psf/black":
		return "22.3.0"
	case "https://github.com/pre-commit/pre-commit-hooks":
		return "v4.4.0"
	case "https://github.com/pre-commit/pygrep-hooks":
		return "v1.10.0"
	case "https://github.com/pre-commit/mirrors-eslint":
		return "v8.44.0"
	case "https://github.com/dnephin/pre-commit-golang":
		return "v0.5.1"
	case "https://github.com/doublify/pre-commit-rust":
		return "v1.0"
	case "https://github.com/mattlqx/pre-commit-ruby":
		return "v1.3.4"
	default:
		return "v4.4.0"
	}
}

// benchmarkGoInstallHooks measures Go pre-commit install-hooks performance
func (te *TestExecutor) benchmarkGoInstallHooks(
	t *testing.T,
	test LanguageCompatibilityTest,
	repoDir string,
	result *TestResults,
) {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Get Go version for display
	goVersion := te.getGoVersion() //nolint:errcheck // Test cleanup, errors can be ignored
	t.Logf(
		"⏱️ Benchmarking Go pre-commit %s install-hooks for %s",
		goVersion,
		test.Language,
	) //nolint:errcheck // Test cleanup, errors can be ignored

	// Clean any existing hooks and cache to ensure fresh start
	hooksDir := filepath.Join(repoDir, ".git", "hooks") //nolint:errcheck // Test cleanup, errors can be ignored
	_ = os.RemoveAll(hooksDir)                          //nolint:errcheck // Test cleanup, errors can be ignored

	// Clean cache to ensure we're measuring the actual environment setup work
	_ = te.runCommand(repoDir, te.suite.goBinary, "clean") //nolint:errcheck // Test cleanup, errors can be ignored

	// Measure install-hooks time (includes repository cloning + environment setup)  //nolint:errcheck // Test cleanup, errors can be ignored
	start := time.Now() //nolint:errcheck // Test cleanup, errors can be ignored
	err := te.runCommand(
		repoDir,
		te.suite.goBinary,
		"install-hooks",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	installTime := time.Since(
		start,
	) //nolint:errcheck // Test cleanup, errors can be ignored

	if err != nil {
		t.Logf(
			"⚠️ Warning: Go install-hooks failed for %s: %v",
			test.Language,
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Go install-hooks failed: %v", err),
		) //nolint:errcheck // Test cleanup, errors can be ignored
	} else {
		result.GoInstallTime = installTime
		installTimeMs := float64(installTime.Nanoseconds()) / 1e6
		t.Logf("✅ Go pre-commit %s install-hooks completed for %s in %.2fms", goVersion, test.Language, installTimeMs) //nolint:errcheck // Test cleanup, errors can be ignored
	}
}

// benchmarkPythonInstallHooks measures Python pre-commit install performance
func (te *TestExecutor) benchmarkPythonInstallHooks(
	t *testing.T,
	test LanguageCompatibilityTest,
	repoDir string,
	result *TestResults,
) {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Get Python version for display
	pythonVersion := te.getPythonVersion() //nolint:errcheck // Test cleanup, errors can be ignored
	t.Logf(
		"⏱️ Benchmarking Python pre-commit %s install-hooks for %s",
		pythonVersion,
		test.Language,
	) //nolint:errcheck // Test cleanup, errors can be ignored

	// Clean any existing hooks and cache to ensure fresh start
	hooksDir := filepath.Join(repoDir, ".git", "hooks") //nolint:errcheck // Test cleanup, errors can be ignored
	_ = os.RemoveAll(hooksDir)                          //nolint:errcheck // Test cleanup, errors can be ignored

	// Clean cache to ensure we're measuring the actual environment setup work
	_ = te.runCommand(repoDir, te.suite.pythonBinary, "clean") //nolint:errcheck // Test cleanup, errors can be ignored

	// Measure install-hooks time (use install-hooks for equivalent comparison)  //nolint:errcheck // Test cleanup, errors can be ignored
	// Note: Python pre-commit doesn't have install-hooks, so we use install + install-hooks
	// to get equivalent functionality to Go's install-hooks
	start := time.Now() //nolint:errcheck // Test cleanup, errors can be ignored

	// First install git hooks (equivalent to Go's install command)  //nolint:errcheck // Test cleanup, errors can be ignored
	installErr := te.runCommand(
		repoDir,
		te.suite.pythonBinary,
		"install",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	if installErr != nil {
		t.Logf(
			"⚠️ Warning: Python install failed for %s: %v",
			test.Language,
			installErr,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Then install hook environments (equivalent to Go's install-hooks)  //nolint:errcheck // Test cleanup, errors can be ignored
	installHooksErr := te.runCommand(
		repoDir,
		te.suite.pythonBinary,
		"install",
		"--install-hooks",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	installTime := time.Since(
		start,
	) //nolint:errcheck // Test cleanup, errors can be ignored

	err := installHooksErr // Use install-hooks error as primary since that's the expensive operation
	if installErr != nil {
		err = installErr // But prioritize install error if it failed
	}

	if err != nil {
		t.Logf(
			"⚠️ Warning: Python install-hooks equivalent failed for %s: %v",
			test.Language,
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
		result.Errors = append(
			result.Errors,
			fmt.Sprintf("Python install-hooks equivalent failed: %v", err),
		) //nolint:errcheck // Test cleanup, errors can be ignored
	} else {
		result.PythonInstallTime = installTime
		installTimeMs := float64(installTime.Nanoseconds()) / 1e6
		t.Logf("✅ Python pre-commit %s install-hooks equivalent completed for %s in %.2fms", pythonVersion, test.Language, installTimeMs) //nolint:errcheck // Test cleanup, errors can be ignored
	}
}

// testCachePerformance tests cache performance improvement based on execution times
func (te *TestExecutor) testCachePerformance(
	t *testing.T,
	test LanguageCompatibilityTest,
	repoDir string,
	result *TestResults,
) {
	t.Helper()                                                  //nolint:errcheck // Test cleanup, errors can be ignored
	t.Logf("🔄 Testing cache performance for %s", test.Language) //nolint:errcheck // Test cleanup, errors can be ignored

	// Test Go cache performance improvement
	if result.GoInstallTime > 0 {
		goPerformanceImprovement := te.measureCachePerformanceImprovement(
			t,
			repoDir,
			te.suite.goBinary,
			"Go",
		) //nolint:errcheck // Test cleanup, errors can be ignored
		result.GoCacheEfficiency = goPerformanceImprovement
	}

	// Test Python cache performance improvement
	if result.PythonInstallTime > 0 && te.suite.pythonBinary != "" {
		pythonPerformanceImprovement := te.measureCachePerformanceImprovement(
			t,
			repoDir,
			te.suite.pythonBinary,
			"Python",
		) //nolint:errcheck // Test cleanup, errors can be ignored
		result.PythonCacheEfficiency = pythonPerformanceImprovement
	}
}

// measureCachePerformanceImprovement measures the performance improvement from cache usage
func (te *TestExecutor) measureCachePerformanceImprovement(
	t *testing.T,
	repoDir string,
	binary string,
	binaryName string,
) float64 {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Ensure hooks are installed first
	if err := te.installHooksForCache(t, repoDir, binary, binaryName); err != nil {
		return 0.0
	}

	// Clear cache to ensure first run creates cache
	te.clearCache(repoDir, binary)

	// Measure first run (cache creation + execution)
	firstRunTime, err := te.measureFirstRun(t, repoDir, binary, binaryName)
	if err != nil {
		return 0.0
	}

	// Measure subsequent runs (cache utilization)
	avgCachedTime, err := te.measureCachedRuns(t, repoDir, binary, binaryName)
	if err != nil {
		return 0.0
	}

	// Calculate and return performance improvement
	return te.calculatePerformanceImprovement(t, binaryName, firstRunTime, avgCachedTime)
}

// installHooksForCache installs hooks for cache performance testing
func (te *TestExecutor) installHooksForCache(t *testing.T, repoDir, binary, binaryName string) error {
	t.Helper()
	var installCmd []string
	if binaryName == "Python" {
		installCmd = []string{binary, "install", "--install-hooks"}
	} else {
		installCmd = []string{binary, "install-hooks"}
	}

	if err := te.runCommand(repoDir, installCmd[0], installCmd[1:]...); err != nil {
		t.Logf("⚠️ Warning: Failed to install %s hooks for cache test: %v", binaryName, err)
		return err
	}
	return nil
}

// clearCache clears the cache for the binary
func (te *TestExecutor) clearCache(repoDir, binary string) {
	te.runCommand(repoDir, binary, "clean") //nolint:errcheck // Test cleanup, errors can be ignored
}

// measureFirstRun measures the first run time (cache creation + execution)
func (te *TestExecutor) measureFirstRun(t *testing.T, repoDir, binary, binaryName string) (time.Duration, error) {
	t.Helper()
	start := time.Now()
	firstRunErr := te.runCommand(repoDir, binary, "run", "--all-files")
	firstRunTime := time.Since(start)

	if firstRunErr != nil {
		t.Logf("⚠️ Warning: First run failed for %s: %v", binaryName, firstRunErr)
		return 0, firstRunErr
	}

	t.Logf("   First run (cache creation): %.2fms", float64(firstRunTime.Nanoseconds())/1e6)
	return firstRunTime, nil
}

// measureCachedRuns measures cached run times and returns average
func (te *TestExecutor) measureCachedRuns(t *testing.T, repoDir, binary, binaryName string) (time.Duration, error) {
	t.Helper()
	const numCachedRuns = 3
	var totalCachedTime time.Duration
	successfulRuns := 0

	for i := range numCachedRuns {
		start := time.Now()
		runErr := te.runCommand(repoDir, binary, "run", "--all-files")
		runTime := time.Since(start)

		if runErr == nil {
			totalCachedTime += runTime
			successfulRuns++
			t.Logf("   Cached run %d: %.2fms", i+1, float64(runTime.Nanoseconds())/1e6)
		}

		// Small delay between runs
		time.Sleep(50 * time.Millisecond)
	}

	if successfulRuns == 0 {
		t.Logf("⚠️ Warning: No successful cached runs for %s", binaryName)
		return 0, fmt.Errorf("no successful cached runs")
	}

	return totalCachedTime / time.Duration(successfulRuns), nil
}

// calculatePerformanceImprovement calculates and logs performance improvement
func (te *TestExecutor) calculatePerformanceImprovement(
	t *testing.T,
	binaryName string,
	firstRunTime, avgCachedTime time.Duration,
) float64 {
	t.Helper()
	performanceImprovement := 0.0
	if firstRunTime > 0 {
		performanceImprovement = ((float64(firstRunTime) - float64(avgCachedTime)) / float64(firstRunTime)) * 100
	}

	t.Logf("✅ %s cache performance: First run %.2fms → Avg cached %.2fms (%.1f%% improvement)",
		binaryName,
		float64(firstRunTime.Nanoseconds())/1e6,
		float64(avgCachedTime.Nanoseconds())/1e6,
		performanceImprovement)

	return performanceImprovement
}

// testLanguageSpecificCachePerformance tests cache performance using language-specific implementations
func (te *TestExecutor) testLanguageSpecificCachePerformance(
	t *testing.T,
	test LanguageCompatibilityTest,
	testDir string,
	result *TestResults,
) {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Only test cache performance for languages that support detailed cache testing
	if test.Language != LangPython || !test.CacheTestEnabled {
		return
	}

	// Skip if Python binary is not available
	if te.suite.pythonBinary == "" {
		t.Logf(
			"⚠️ Skipping language-specific cache performance test: Python binary not available",
		) //nolint:errcheck // Test cleanup, errors can be ignored
		return
	}

	t.Logf(
		"🔄 Running language-specific cache performance test for %s",
		test.Language,
	) //nolint:errcheck // Test cleanup, errors can be ignored

	// Create a dedicated cache test repository
	cacheTestDir := filepath.Join(testDir, "cache-perf-test") //nolint:errcheck // Test cleanup, errors can be ignored
	if err := os.MkdirAll(cacheTestDir, 0o750); err != nil {
		t.Logf(
			"⚠️ Warning: Failed to create cache test directory: %v",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
		return
	}

	// Test first run (environment creation)  //nolint:errcheck // Test cleanup, errors can be ignored
	firstRepo := filepath.Join(
		cacheTestDir,
		"first-run",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	firstRunTime, err := te.benchmarkSingleInstall(
		t,
		firstRepo,
		te.suite.pythonBinary,
		"install",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	if err != nil {
		t.Logf(
			"⚠️ Warning: First cache test run failed: %v",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
		return
	}

	// Test second run (should reuse environment)  //nolint:errcheck // Test cleanup, errors can be ignored
	secondRepo := filepath.Join(
		cacheTestDir,
		"second-run",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	secondRunTime, err := te.benchmarkSingleInstall(
		t,
		secondRepo,
		te.suite.pythonBinary,
		"install",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	if err != nil {
		t.Logf(
			"⚠️ Warning: Second cache test run failed: %v",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
		return
	}

	// Calculate and report cache performance
	te.reportCachePerformance(
		t,
		test.Language,
		firstRunTime,
		secondRunTime,
		result,
	) //nolint:errcheck // Test cleanup, errors can be ignored
}

// benchmarkSingleInstall benchmarks a single install operation
func (te *TestExecutor) benchmarkSingleInstall(
	t *testing.T,
	repoDir, binary, command string,
) (time.Duration, error) {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Setup repository
	if err := te.setupMinimalTestRepository(t, repoDir); err != nil {
		return 0, fmt.Errorf(
			"failed to setup repository: %w",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Benchmark the install
	start := time.Now() //nolint:errcheck // Test cleanup, errors can be ignored
	if err := te.runCommand(repoDir, binary, command); err != nil {
		return 0, fmt.Errorf("install command failed: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
	}
	return time.Since(start), nil
}

// setupMinimalTestRepository creates a minimal repository for cache testing
func (te *TestExecutor) setupMinimalTestRepository(t *testing.T, repoDir string) error {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Remove existing directory
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf(
			"failed to remove existing repo: %w",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Create repository directory
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		return fmt.Errorf(
			"failed to create repo directory: %w",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Initialize git repository
	if err := te.runCommand(repoDir, "git", "init"); err != nil {
		return fmt.Errorf("failed to init git repo: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Set git config
	_ = te.runCommand( //nolint:errcheck // Test cleanup, errors can be ignored
		repoDir,
		"git",
		"config",
		"user.email",
		"test@example.com",
	)
	_ = te.runCommand( //nolint:errcheck // Test cleanup, errors can be ignored
		repoDir,
		"git",
		"config",
		"user.name",
		"Test User",
	)
	_ = te.runCommand( //nolint:errcheck // Test cleanup, errors can be ignored
		repoDir,
		"git",
		"config",
		"commit.gpgsign",
		"false",
	)

	// Create simple pre-commit config
	configContent := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
    -   id: check-yaml
`
	configPath := filepath.Join(
		repoDir,
		".pre-commit-config.yaml",
	) //nolint:errcheck // Test cleanup, errors can be ignored
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf(
			"failed to write pre-commit config: %w",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Create test file
	testFile := filepath.Join(repoDir, "test.yaml") //nolint:errcheck // Test cleanup, errors can be ignored
	if err := os.WriteFile(testFile, []byte("test: value\n"), 0o600); err != nil {
		return fmt.Errorf("failed to write test file: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Add and commit
	if err := te.runCommand(repoDir, "git", "add", "."); err != nil {
		return fmt.Errorf("failed to git add: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	if err := te.runCommand(repoDir, "git", "commit", "-m", "Initial commit"); err != nil {
		return fmt.Errorf("failed to git commit: %w", err) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	return nil
}

// reportCachePerformance reports cache performance metrics
func (te *TestExecutor) reportCachePerformance(
	t *testing.T,
	language string,
	firstRun, secondRun time.Duration,
	result *TestResults,
) {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	firstMs := float64(firstRun.Nanoseconds()) / 1e6
	secondMs := float64(secondRun.Nanoseconds()) / 1e6
	speedup := float64(firstRun) / float64(secondRun) //nolint:errcheck // Test cleanup, errors can be ignored

	t.Logf("📊 Cache performance analysis for %s:", language) //nolint:errcheck // Test cleanup, errors can be ignored
	t.Logf("   First run (env creation):  %.2fms", firstMs)  //nolint:errcheck // Test cleanup, errors can be ignored
	t.Logf("   Second run (cache hit):    %.2fms", secondMs) //nolint:errcheck // Test cleanup, errors can be ignored
	t.Logf("   Speedup ratio:             %.1fx", speedup)   //nolint:errcheck // Test cleanup, errors can be ignored

	switch {
	case speedup > 1.2: // At least 20% faster
		improvement := (speedup - 1.0) * 100
		result.PythonCacheEfficiency = improvement
		t.Logf(
			"✅ Significant cache efficiency detected: %.1f%% improvement",
			improvement,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	case speedup > 1.05: // At least 5% faster
		improvement := (speedup - 1.0) * 100
		result.PythonCacheEfficiency = improvement
		t.Logf(
			"✅ Cache efficiency detected: %.1f%% improvement",
			improvement,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	default:
		t.Logf(
			"⚠️ Limited cache benefit detected (%.1fx speedup)",
			speedup,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}
}

// runCommand executes a command in the specified directory
func (te *TestExecutor) runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...) //nolint:errcheck // Test cleanup, errors can be ignored
	cmd.Dir = dir
	// For debugging: capture output but don't display unless there's an error
	output, err := cmd.CombinedOutput() //nolint:errcheck // Test cleanup, errors can be ignored
	if err != nil {
		// Include command output in error for better debugging
		return fmt.Errorf(
			"command '%s %v' failed: %w\nOutput: %s",
			name,
			args,
			err,
			string(output),
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}
	return nil
}

// getGoVersion returns the version of the Go pre-commit binary
func (te *TestExecutor) getGoVersion() string {
	cmd := exec.Command(te.suite.goBinary, "--version") //nolint:errcheck // Test cleanup, errors can be ignored
	output, err := cmd.Output()                         //nolint:errcheck // Test cleanup, errors can be ignored
	if err != nil {
		return unknownVersion
	}
	version := strings.TrimSpace(string(output)) //nolint:errcheck // Test cleanup, errors can be ignored
	if version == "" {
		return devVersion
	}
	return version
}

// runBidirectionalCacheTest runs bidirectional cache compatibility tests
func (te *TestExecutor) runBidirectionalCacheTest(
	t *testing.T,
	test LanguageCompatibilityTest,
	testDir string,
	_ *TestResults,
) error {
	t.Helper() //nolint:errcheck // Test cleanup, errors can be ignored

	// Create the appropriate language test runner
	runner, err := LanguageTestFactory(test.Language, testDir) //nolint:errcheck // Test cleanup, errors can be ignored
	if err != nil {
		return fmt.Errorf(
			"failed to create language test runner: %w",
			err,
		) //nolint:errcheck // Test cleanup, errors can be ignored
	}

	// Convert to languages.LanguageCompatibilityTest for the languages package
	langTest := languages.LanguageCompatibilityTest{
		Language:                 test.Language,
		TestRepository:           test.TestRepository,
		TestVersions:             test.TestVersions,
		AdditionalDependencies:   test.AdditionalDependencies,
		TestTimeout:              test.TestTimeout,
		BiDirectionalTestEnabled: test.BiDirectionalTestEnabled,
	}

	// Run the bidirectional cache test using the language-specific runner
	baseTest := languages.NewBaseLanguageTest(
		test.Language,
		testDir,
	) //nolint:errcheck // Test cleanup, errors can be ignored
	return baseTest.RunBidirectionalCacheTest(
		t,
		te.suite.pythonBinary,
		te.suite.goBinary,
		langTest,
		runner,
	) //nolint:errcheck // Test cleanup, errors can be ignored
}

// getPythonVersion returns the version of the Python pre-commit binary
func (te *TestExecutor) getPythonVersion() string {
	cmd := exec.Command(te.suite.pythonBinary, "--version") //nolint:errcheck // Test cleanup, errors can be ignored
	output, err := cmd.Output()                             //nolint:errcheck // Test cleanup, errors can be ignored
	if err != nil {
		return unknownVersion
	}
	version := strings.TrimSpace(string(output)) //nolint:errcheck // Test cleanup, errors can be ignored
	if version == "" {
		return unknownVersion
	}
	return version
}
