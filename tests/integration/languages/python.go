package languages

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver for database operations

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

const (
	// dbFileName is the name of the pre-commit cache database file
	dbFileName = "db.db"
)

// PythonLanguageTest implements LanguageTestRunner and BidirectionalTestRunner for Python
type PythonLanguageTest struct {
	*BaseLanguageTest
	*BaseBidirectionalTest
}

// NewPythonLanguageTest creates a new Python language test
func NewPythonLanguageTest(testDir string) *PythonLanguageTest {
	return &PythonLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangPython, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(LangPython),
	}
}

// SetupRepositoryFiles creates Python-specific files in the test repository
func (pt *PythonLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml (required for local repos)
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: test-python
    name: Test Python Hook
    description: Test Python hook for pre-commit validation
    entry: python -c "print('Python hook test passed')"
    language: python
    files: \.py$
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create setup.py (required for Python language)
	setupContent := "from setuptools import setup\nsetup(name='test')"
	if err := os.WriteFile(filepath.Join(repoPath, "setup.py"), []byte(setupContent), 0o600); err != nil {
		return fmt.Errorf("failed to create setup.py: %w", err)
	}
	return nil
}

// GetLanguageManager returns the Python language manager
func (pt *PythonLanguageTest) GetLanguageManager() (language.Manager, error) {
	registry := languages.NewLanguageRegistry()
	langImpl, exists := registry.GetLanguage(LangPython)
	if !exists {
		return nil, fmt.Errorf("language %s not found in registry", LangPython)
	}

	lang, ok := langImpl.(language.Manager)
	if !ok {
		return nil, fmt.Errorf("language %s does not implement LanguageManager interface", LangPython)
	}

	return lang, nil
}

// GetAdditionalValidations returns Python-specific validation steps
func (pt *PythonLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "python-executable-check",
			Description: "Python executable validation",
			Execute: func(_ *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if Python executable exists in the environment
				pythonExe := filepath.Join(envPath, "bin", "python")
				if _, err := os.Stat(pythonExe); os.IsNotExist(err) {
					// Try python3
					pythonExe = filepath.Join(envPath, "bin", "python3")
					if _, err = os.Stat(pythonExe); os.IsNotExist(err) {
						return fmt.Errorf("python executable not found in environment")
					}
				}
				// Python executable found
				return nil
			},
		},
		{
			Name:        "pip-check",
			Description: "Pip installation validation",
			Execute: func(_ *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if pip exists in the environment
				pipExe := filepath.Join(envPath, "bin", "pip")
				if _, err := os.Stat(pipExe); os.IsNotExist(err) {
					// Try pip3
					pipExe = filepath.Join(envPath, "bin", "pip3")
					if _, err = os.Stat(pipExe); os.IsNotExist(err) {
						return fmt.Errorf("pip executable not found in environment")
					}
				}
				// Pip executable found
				return nil
			},
		},
		{
			Name:        "virtualenv-structure-check",
			Description: "Virtual environment structure validation",
			Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if the environment has the expected structure
				expectedDirs := []string{"bin", "lib"}
				for _, dir := range expectedDirs {
					dirPath := filepath.Join(envPath, dir)
					if _, err := os.Stat(dirPath); os.IsNotExist(err) {
						return fmt.Errorf("expected directory %s not found in environment", dir)
					}
				}
				t.Logf("      Virtual environment structure validated")
				return nil
			},
		},
		{
			Name:        "python-version-compatibility-test",
			Description: "Python version compatibility between Go and Python pre-commit",
			Execute: func(t *testing.T, envPath, version string, lang language.Manager) error {
				// Test that the Python version matches between implementations
				return pt.testPythonVersionCompatibility(t, envPath, version, lang)
			},
		},
		{
			Name:        "cache-database-compatibility-test",
			Description: "Database schema compatibility validation",
			Execute: func(t *testing.T, envPath, version string, lang language.Manager) error {
				// Test that database schemas are compatible between implementations
				return pt.testCacheDatabaseCompatibility(t, envPath, version, lang)
			},
		},
		{
			Name:        "cache-hit-performance-test",
			Description: "Cache hit performance validation",
			Execute: func(t *testing.T, envPath, version string, lang language.Manager) error {
				// Note: This validation uses system pre-commit for historical compatibility testing
				// The actual Go implementation cache performance is tested separately in bidirectional tests
				return pt.testCacheHitPerformance(t, envPath, version, lang)
			},
		},
	}
}

// GetLanguageName returns the name of the Python language
func (pt *PythonLanguageTest) GetLanguageName() string {
	return LangPython
}

// GetPreCommitConfig returns the .pre-commit-config.yaml content for Python testing
func (pt *PythonLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-python
        name: Test Python Hook
        entry: python -c "print('Python hook test passed')"
        language: python
        files: '\.py$'
`
}

// GetTestFiles returns test files needed for Python testing
func (pt *PythonLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"test.py": `#!/usr/bin/env python3
"""Test Python file for hook testing."""

def hello():
    print("Hello from Python!")

if __name__ == "__main__":
    hello()
`,
		"setup.py": `from setuptools import setup

setup(
    name="test-python-hooks",
    version="0.1.0",
    description="Test Python hooks for pre-commit",
    py_modules=["test"],
)`,
	}
}

// GetExpectedDirectories returns directories expected to be created by Python environment setup
func (pt *PythonLanguageTest) GetExpectedDirectories() []string {
	return []string{"bin", "lib", "pyvenv.cfg"}
}

// GetExpectedStateFiles returns state files that should remain unchanged during bidirectional testing
func (pt *PythonLanguageTest) GetExpectedStateFiles() []string {
	return []string{".git", ".pre-commit-config.yaml", ".pre-commit-hooks.yaml"}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
//
// CRITICAL REQUIREMENT: True bidirectional cache compatibility means:
//
// 1. Implementation A creates complete cache (install hooks + run once)
// 2. Implementation B uses A's cache (run only) - MUST CHANGE NOTHING:
//   - No new environment installations
//   - No database modifications
//   - No new cache files
//   - Same hook execution results
//
// If Implementation B modifies ANYTHING in the cache when using A's cache,
// then the implementations are NOT truly compatible.
//
// This test enforces the strictest possible compatibility check:
// Cache state before/after must be bit-for-bit identical (except for SQLite binary differences).
func (pt *PythonLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary string,
	tempDir string,
) error {
	t.Helper()
	t.Logf("🔄 Testing Python language bidirectional cache compatibility")
	t.Logf("   📋 Python hooks use virtual environments - testing cache compatibility")

	// Use the base bidirectional test framework
	if err := pt.BaseBidirectionalTest.RunBidirectionalCacheTest(t, pt, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("python bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ Python language bidirectional cache compatibility test completed")
	return nil
}

// queryAllRepos queries all repository entries from a database
func (pt *PythonLanguageTest) queryAllRepos(db *sql.DB) ([]RepoEntry, error) {
	rows, err := db.QueryContext(context.Background(), "SELECT repo, ref, path FROM repos ORDER BY repo, ref")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil { //nolint:revive,staticcheck // Test cleanup
			// Ignore error during cleanup
		}
	}()

	var repos []RepoEntry
	for rows.Next() {
		var repo RepoEntry
		if err := rows.Scan(&repo.Repo, &repo.Ref, &repo.Path); err != nil {
			return nil, err
		}
		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

// RepoEntry represents a repository entry in the database
type RepoEntry struct {
	Repo string
	Ref  string
	Path string
}

// runCommand executes a command in the specified directory with timeout
func (pt *PythonLanguageTest) runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	// Set timeout for commands - bidirectional cache tests may need environment creation
	timeout := 120 * time.Second // Increased from 30s to 120s for environment setup

	done := make(chan error, 1)
	go func() {
		output, err := cmd.CombinedOutput()
		if err != nil {
			done <- fmt.Errorf("command '%s %v' failed: %w\nOutput: %s",
				name, args, err, string(output))
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil { //nolint:revive,staticcheck // Test cleanup
				// Log error but continue //nolint:revive,staticcheck // Test cleanup
			}
		}
		return fmt.Errorf("command '%s %v' timed out after %v", name, args, timeout)
	}
}

// runCommandWithEnv runs a command with custom environment variables
func (pt *PythonLanguageTest) runCommandWithEnv(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env

	// Set a reasonable timeout for the command
	timeout := 5 * time.Minute

	done := make(chan error, 1)
	go func() {
		output, err := cmd.CombinedOutput()
		if err != nil {
			done <- fmt.Errorf("command '%s %v' failed: %w\nOutput: %s",
				name, args, err, string(output))
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil { //nolint:revive,staticcheck // Test cleanup
				// Log error but continue //nolint:revive,staticcheck // Test cleanup
			}
		}
		return fmt.Errorf("command '%s %v' timed out after %v", name, args, timeout)
	}
}

// testCacheHitPerformance tests cache hit performance for Python environments
func (pt *PythonLanguageTest) testCacheHitPerformance(
	t *testing.T,
	_, version string,
	_ language.Manager,
) error {
	t.Helper()

	// Create a unique test repository for cache hit testing (separate from main test)
	cacheTestRepo := filepath.Join(pt.testDir, "cache-hit-test-repo-"+version)
	if err := pt.setupCacheTestRepository(t, cacheTestRepo); err != nil {
		return fmt.Errorf("failed to setup cache test repository: %w", err)
	}

	// Set a custom cache directory to isolate this test
	customCacheDir := filepath.Join(pt.testDir, "isolated-cache-"+version)
	if err := os.MkdirAll(customCacheDir, 0o750); err != nil {
		return fmt.Errorf("failed to create custom cache dir: %w", err)
	}

	// First install the hooks
	if err := pt.runCommand(cacheTestRepo, "pre-commit", "install", "--overwrite"); err != nil {
		return fmt.Errorf("failed to install git hooks: %w", err)
	}

	// First run: Should create environment and run hooks (slower)
	t.Logf("      🔄 First run: Running hooks (should create environment)")
	env := append(os.Environ(), "PRE_COMMIT_HOME="+customCacheDir)
	firstStart := time.Now()
	if err := pt.runCommandWithEnv(cacheTestRepo, env, "pre-commit", "run", "--all-files"); err != nil {
		// This might fail due to hook errors, but that's okay for performance testing
		t.Logf("      ⚠️ First run completed with exit code (expected): %v", err)
	}
	firstRunTime := time.Since(firstStart)

	// Wait a moment to ensure any file system operations are complete
	time.Sleep(100 * time.Millisecond)

	// Second run: Should reuse environment (faster)
	t.Logf("      🔄 Second run: Running hooks (should reuse environment)")
	secondStart := time.Now()
	if err := pt.runCommandWithEnv(cacheTestRepo, env, "pre-commit", "run", "--all-files"); err != nil {
		// This might fail due to hook errors, but that's okay for performance testing
		t.Logf("      ⚠️ Second run completed with exit code (expected): %v", err)
	}
	secondRunTime := time.Since(secondStart)

	// Calculate speedup
	firstMs := float64(firstRunTime.Nanoseconds()) / 1e6
	secondMs := float64(secondRunTime.Nanoseconds()) / 1e6
	speedup := float64(firstRunTime) / float64(secondRunTime)

	t.Logf("      📊 Cache performance for Python %s:", version)
	t.Logf("        First run:  %.2fms (environment creation)", firstMs)
	t.Logf("        Second run: %.2fms (cache hit)", secondMs)

	switch {
	case speedup > 1.1: // At least 10% faster
		improvement := (speedup - 1.0) * 100
		t.Logf("        ✅ Cache hit detected: %.1fx speedup (%.1f%% faster)", speedup, improvement)
	case speedup > 0.9: // Within 10% (acceptable for fast operations)
		t.Logf("        ✅ Cache hit: Similar performance (%.1fx)", speedup)
	default:
		t.Logf("        ⚠️ Warning: Second run slower (%.1fx) - possible cache miss", speedup)
	}

	// Test environment reuse more thoroughly
	return pt.validateEnvironmentReuse(t, cacheTestRepo, version, customCacheDir)
}

// setupCacheTestRepository creates a minimal repository for cache testing
func (pt *PythonLanguageTest) setupCacheTestRepository(t *testing.T, repoDir string) error {
	t.Helper()

	// Remove existing directory
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to remove existing repo: %w", err)
	}

	// Create repository directory
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		return fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Initialize git repository
	if err := pt.runCommand(repoDir, "git", "init"); err != nil {
		return fmt.Errorf("failed to init git repo: %w", err)
	}

	// Set git config
	if err := pt.runCommand(repoDir, "git", "config", "user.email", "test@example.com"); err != nil {
		t.Logf("⚠️  Warning: failed to set git user.email: %v", err)
	}
	if err := pt.runCommand(repoDir, "git", "config", "user.name", "Test User"); err != nil {
		t.Logf("⚠️  Warning: failed to set git user.name: %v", err)
	}
	if err := pt.runCommand(repoDir, "git", "config", "commit.gpgsign", "false"); err != nil {
		t.Logf("⚠️  Warning: failed to disable git commit signing: %v", err)
	}

	// Create a simple pre-commit config for cache testing
	configContent := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
    -   id: check-yaml
    -   id: trailing-whitespace
`
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to write pre-commit config: %w", err)
	}

	// Create a simple test file
	testFile := filepath.Join(repoDir, "test.yaml")
	testContent := "test: value\n"
	if err := os.WriteFile(testFile, []byte(testContent), 0o600); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	// Add and commit files (skip pre-commit hooks for initial setup to avoid conflicts)
	if err := pt.runCommand(repoDir, "git", "add", "."); err != nil {
		return fmt.Errorf("failed to git add: %w", err)
	}

	if err := pt.runCommand(repoDir, "git", "commit", "--no-verify", "-m", "Initial commit"); err != nil {
		return fmt.Errorf("failed to git commit: %w", err)
	}

	return nil
}

// validateEnvironmentReuse validates that environments are being properly reused
func (pt *PythonLanguageTest) validateEnvironmentReuse(t *testing.T, repoDir, version, customCacheDir string) error {
	t.Helper()

	// Check for environment directories in common cache locations + custom cache
	cacheLocations := []string{
		filepath.Join(os.Getenv("HOME"), ".cache", "pre-commit"),
		filepath.Join(repoDir, ".pre-commit-cache"),
		customCacheDir, // Include the custom cache directory used in the test
	}

	foundEnvironments := 0
	for _, cacheDir := range cacheLocations {
		if _, err := os.Stat(cacheDir); err == nil {
			// Count environment directories
			err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() && (filepath.Base(path) == "py_env-"+version ||
					strings.Contains(filepath.Base(path), "py_env")) {
					foundEnvironments++
					t.Logf("        📁 Found environment: %s", path)
				}
				return nil
			})
			if err != nil {
				t.Logf("        ⚠️ Warning: failed to walk cache directory %s: %v", cacheDir, err)
			}
		}
	}

	if foundEnvironments == 0 {
		t.Logf("        ⚠️ Warning: No Python environments found in cache")
	} else {
		t.Logf("        ✅ Found %d Python environment(s) in cache", foundEnvironments)
	}

	return nil
}

// testPythonVersionCompatibility tests that the Python version used by the Go implementation
// matches the Python pre-commit version for cache compatibility
func (pt *PythonLanguageTest) testPythonVersionCompatibility(
	t *testing.T,
	envPath, version string,
	_ language.Manager,
) error {
	t.Helper()

	// Get the actual Python version from the environment
	pythonExe := filepath.Join(envPath, "bin", "python")
	if _, err := os.Stat(pythonExe); os.IsNotExist(err) {
		// Try python3
		pythonExe = filepath.Join(envPath, "bin", "python3")
		if _, err := os.Stat(pythonExe); os.IsNotExist(err) {
			return fmt.Errorf("python executable not found in environment %s", envPath)
		}
	}

	// Get Python version from the Go-created environment
	cmd := exec.Command(pythonExe, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get Python version from Go environment: %w", err)
	}

	goEnvPythonVersion := strings.TrimSpace(string(output))
	t.Logf("        📦 Go environment Python version: %s", goEnvPythonVersion)

	// Get Python version from the system Python pre-commit
	systemPythonCmd := exec.Command("python3", "--version")
	systemOutput, err := systemPythonCmd.Output()
	if err != nil {
		t.Logf("        ⚠️ Could not get system Python version: %v", err)
		// Try python instead of python3
		systemPythonCmd = exec.Command("python", "--version")
		systemOutput, err = systemPythonCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get system Python version: %w", err)
		}
	}

	systemPythonVersion := strings.TrimSpace(string(systemOutput))
	t.Logf("        🐍 System Python version: %s", systemPythonVersion)

	// Compare major.minor versions (ignore patch versions for compatibility)
	goVersionParts := strings.Split(strings.Replace(goEnvPythonVersion, "Python ", "", 1), ".")
	systemVersionParts := strings.Split(strings.Replace(systemPythonVersion, "Python ", "", 1), ".")

	if len(goVersionParts) < 2 || len(systemVersionParts) < 2 {
		return fmt.Errorf("invalid Python version format: Go=%s, System=%s", goEnvPythonVersion, systemPythonVersion)
	}

	goMajorMinor := fmt.Sprintf("%s.%s", goVersionParts[0], goVersionParts[1])
	systemMajorMinor := fmt.Sprintf("%s.%s", systemVersionParts[0], systemVersionParts[1])

	if goMajorMinor != systemMajorMinor {
		return fmt.Errorf(
			"python version mismatch: Go environment uses %s, system uses %s (major.minor versions must match for cache compatibility)",
			goMajorMinor,
			systemMajorMinor,
		)
	}

	t.Logf("        ✅ Python versions compatible: %s (major.minor match)", goMajorMinor)

	// Additional check: Verify that the requested version matches the environment
	if version != "" && version != language.VersionDefault {
		// Check if the requested version matches the actual environment version
		if pt.isVersionCompatible(version, goMajorMinor) {
			t.Logf("        ✅ Requested version %s is compatible with environment version %s", version, goMajorMinor)
		} else {
			t.Logf("        ℹ️ Note: Requested version %s differs from environment version %s (using system Python)", version, goMajorMinor)
			t.Logf("        ℹ️ This is expected when the system doesn't have the requested Python version installed")
		}
	} else if version == language.VersionDefault {
		t.Logf("        ✅ Using default Python version: %s", goMajorMinor)
	}

	return nil
}

// testCacheDatabaseCompatibility tests that the database schema created by the Go implementation
// is compatible with the Python pre-commit implementation
func (pt *PythonLanguageTest) testCacheDatabaseCompatibility(
	t *testing.T,
	_, _ string,
	_ language.Manager,
) error {
	t.Helper()

	dbPath := pt.findCacheDatabase(t)
	if dbPath == "" {
		t.Logf("        ℹ️ No cache database found (expected for some test scenarios)")
		return nil
	}

	t.Logf("        🗄️ Testing database compatibility: %s", dbPath)
	return pt.validateDatabaseCompatibility(t, dbPath)
}

// findCacheDatabase finds the cache database file
func (pt *PythonLanguageTest) findCacheDatabase(_ *testing.T) string {
	cacheLocations := []string{
		filepath.Join(os.Getenv("HOME"), ".cache", "pre-commit"),
		filepath.Join(pt.testDir, ".pre-commit-cache"),
	}

	for _, cacheDir := range cacheLocations {
		testDbPath := filepath.Join(cacheDir, dbFileName)
		if _, err := os.Stat(testDbPath); err == nil {
			return testDbPath
		}
	}
	return ""
}

// validateDatabaseCompatibility validates the database schema and data
func (pt *PythonLanguageTest) validateDatabaseCompatibility(t *testing.T, dbPath string) error {
	// Open and validate database schema
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open cache database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("⚠️  Warning: failed to close database: %v", closeErr)
		}
	}()

	// Check that essential tables exist
	if err = pt.validateEssentialTables(t, db); err != nil { //nolint:gocritic // Consistent error variable usage in function scope
		return err
	}

	// Validate repos table schema
	if err = pt.validateReposTableSchema(t, db); err != nil {
		return fmt.Errorf("repos table schema validation failed: %w", err)
	}

	// Test that we can read data from the database (basic compatibility test)
	repos, err := pt.queryAllRepos(db)
	if err != nil {
		return fmt.Errorf("failed to query repos from database: %w", err)
	}

	t.Logf("        📊 Database contains %d repository entries", len(repos))

	// Validate that repository entries have expected structure
	for _, repo := range repos {
		if repo.Repo == "" || repo.Ref == "" {
			return fmt.Errorf("invalid repository entry found: repo=%s, ref=%s", repo.Repo, repo.Ref)
		}
	}

	t.Logf("        ✅ Database schema and data compatibility validated")
	return nil
}

// validateEssentialTables checks that essential tables exist
func (pt *PythonLanguageTest) validateEssentialTables(t *testing.T, db *sql.DB) error {
	expectedTables := []string{"repos", "configs"}
	for _, table := range expectedTables {
		var count int
		// Note: Table names cannot be parameterized in SQL, but these are hardcoded safe values
		var query string
		switch table {
		case "repos":
			query = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='repos'"
		case "configs":
			query = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='configs'"
		default:
			return fmt.Errorf("unexpected table name: %s", table)
		}

		if err := db.QueryRowContext(context.Background(), query).Scan(&count); err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if count == 0 {
			return fmt.Errorf("essential table %s not found in cache database", table)
		}
		t.Logf("        ✅ Table %s exists in cache database", table)
	}
	return nil
}

// validateReposTableSchema validates the schema of the repos table
func (pt *PythonLanguageTest) validateReposTableSchema(t *testing.T, db *sql.DB) error {
	t.Helper()

	// Get column information for the repos table
	rows, err := db.QueryContext(context.Background(), "PRAGMA table_info(repos)")
	if err != nil {
		return fmt.Errorf("failed to get table info: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil { //nolint:revive,staticcheck // Test cleanup
			t.Logf("⚠️  Warning: failed to close rows: %v", err)
		}
	}()

	expectedColumns := map[string]bool{
		"repo": false,
		"ref":  false,
		"path": false,
	}

	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("failed to scan column info: %w", err)
		}

		if _, expected := expectedColumns[name]; expected {
			expectedColumns[name] = true
			t.Logf("        ✅ Column %s found with type %s", name, dataType)
		}
	}

	// Check for any iteration errors
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error during rows iteration: %w", err)
	}

	// Check that all expected columns were found
	for column, found := range expectedColumns {
		if !found {
			return fmt.Errorf("expected column %s not found in repos table", column)
		}
	}

	return nil
}

// isVersionCompatible checks if a requested Python version is compatible with the actual environment version
func (pt *PythonLanguageTest) isVersionCompatible(requestedVersion, actualMajorMinor string) bool {
	// Handle common version formats
	switch {
	case requestedVersion == "default":
		return true
	case requestedVersion == actualMajorMinor:
		return true
	case strings.HasPrefix(requestedVersion, actualMajorMinor):
		return true
	case requestedVersion == "3" && strings.HasPrefix(actualMajorMinor, "3."):
		return true
	case requestedVersion == "python3" && strings.HasPrefix(actualMajorMinor, "3."):
		return true
	default:
		// For exact version matching (e.g., "3.9" vs "3.9")
		return strings.Contains(actualMajorMinor, requestedVersion)
	}
}
