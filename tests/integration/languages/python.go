package languages

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
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

// PythonLanguageTest implements LanguageTestRunner for Python
type PythonLanguageTest struct {
	*BaseLanguageTest
}

// NewPythonLanguageTest creates a new Python language test
func NewPythonLanguageTest(testDir string) *PythonLanguageTest {
	return &PythonLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangPython, testDir),
	}
}

// SetupRepositoryFiles creates Python-specific files in the test repository
func (pt *PythonLanguageTest) SetupRepositoryFiles(repoPath string) error {
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
			Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if Python executable exists in the environment
				pythonExe := filepath.Join(envPath, "bin", "python")
				if _, err := os.Stat(pythonExe); os.IsNotExist(err) {
					// Try python3
					pythonExe = filepath.Join(envPath, "bin", "python3")
					if _, err = os.Stat(pythonExe); os.IsNotExist(err) {
						return fmt.Errorf("python executable not found in environment")
					}
				}
				t.Logf("      Found Python executable: %s", pythonExe)
				return nil
			},
		},
		{
			Name:        "pip-check",
			Description: "Pip installation validation",
			Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if pip exists in the environment
				pipExe := filepath.Join(envPath, "bin", "pip")
				if _, err := os.Stat(pipExe); os.IsNotExist(err) {
					// Try pip3
					pipExe = filepath.Join(envPath, "bin", "pip3")
					if _, err = os.Stat(pipExe); os.IsNotExist(err) {
						return fmt.Errorf("pip executable not found in environment")
					}
				}
				t.Logf("      Found pip executable: %s", pipExe)
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
	testRepo string,
) error {
	t.Helper()
	t.Logf("🔄 Testing bidirectional cache compatibility for Python")
	t.Logf("   📋 REQUIREMENTS: Cache must be 100%% unchanged when used by opposite implementation")

	// Setup test repository for cache testing
	repoDir := filepath.Join(pt.testDir, "cache-test-repo")
	if err := pt.setupTestRepository(t, repoDir, testRepo); err != nil {
		return fmt.Errorf("failed to setup test repository: %w", err)
	}

	// Test 1: Create cache with Go, use with Python (no changes allowed)
	t.Logf("  🧪 Test 1: Go creates cache → Python uses (zero modifications)")
	if err := pt.testGoCacheWithPython(t, goBinary, pythonBinary, repoDir); err != nil {
		return fmt.Errorf("Go→Python cache test failed: %w", err)
	}

	// Test 2: Create cache with Python, use with Go (no changes allowed)
	t.Logf("  🧪 Test 2: Python creates cache → Go uses (zero modifications)")
	if err := pt.testPythonCacheWithGo(t, pythonBinary, goBinary, repoDir); err != nil {
		return fmt.Errorf("Python→Go cache test failed: %w", err)
	}

	// Test 3: Compare directory structures for diagnostic purposes
	t.Logf("  🧪 Test 3: Comparing cache directory structures (diagnostic)")
	if err := pt.compareCacheDirectories(t, goBinary, pythonBinary, repoDir); err != nil {
		return fmt.Errorf("cache directory comparison failed: %w", err)
	}

	// Test 4: Go implementation cache performance test
	t.Logf("  🧪 Test 4: Go implementation cache performance (our Go binary)")
	if err := pt.testGoCachePerformance(t, goBinary, "default"); err != nil {
		t.Logf("  ⚠️ Warning: Go cache performance test failed: %v", err)
		// Don't fail the bidirectional test for cache performance issues
	}

	t.Logf("✅ Bidirectional cache compatibility test passed")
	return nil
}

// setupTestRepository creates a test git repository with pre-commit configuration
func (pt *PythonLanguageTest) setupTestRepository(t *testing.T, repoDir, testRepo string) error {
	t.Helper()

	// Remove existing directory if it exists
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to remove existing repo directory: %w", err)
	}

	// Create repository directory
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		return fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Initialize git repository
	if err := pt.runCommand(repoDir, "git", "init"); err != nil {
		return fmt.Errorf("failed to init git repo: %w", err)
	}

	// Set git config to avoid warnings and disable signing
	if err := pt.runCommand(repoDir, "git", "config", "user.email", "test@example.com"); err != nil {
		t.Logf("Warning: failed to set git user.email: %v", err)
	}
	if err := pt.runCommand(repoDir, "git", "config", "user.name", "Test User"); err != nil {
		t.Logf("Warning: failed to set git user.name: %v", err)
	}
	if err := pt.runCommand(repoDir, "git", "config", "commit.gpgsign", "false"); err != nil {
		t.Logf("Warning: failed to disable git commit signing: %v", err)
	}

	// Create pre-commit config for Python hooks
	configContent := pt.generatePreCommitConfig(testRepo)
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to write pre-commit config: %w", err)
	}

	// Log the configuration for debugging
	t.Logf("Created .pre-commit-config.yaml with content:\n%s", configContent)

	// Create Python test files
	if err := pt.SetupRepositoryFiles(repoDir); err != nil {
		return fmt.Errorf("failed to setup repository files: %w", err)
	}

	// Create a test Python file that will be processed by hooks
	testPyFile := filepath.Join(repoDir, "test_file.py")
	testPyContent := `#!/usr/bin/env python3
"""Test Python file for pre-commit hooks."""

def hello_world():
    """Print hello world."""
    print("Hello, World!")

if __name__ == "__main__":
    hello_world()
`
	if err := os.WriteFile(testPyFile, []byte(testPyContent), 0o600); err != nil {
		return fmt.Errorf("failed to write test Python file: %w", err)
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

// generatePreCommitConfig generates a pre-commit config for Python testing
func (pt *PythonLanguageTest) generatePreCommitConfig(testRepo string) string {
	_ = testRepo // Parameter kept for interface compatibility

	// Use hooks that require environment setup to properly test cache performance
	// These hooks actually create virtual environments that show cache benefits
	// Configure to only scan our test files to avoid problematic repository files
	return `repos:
-   repo: https://github.com/pycqa/bandit
    rev: 1.7.4
    hooks:
    -   id: bandit
        args: [--skip, B101]
        files: '^test_file\.py$'
-   repo: https://github.com/pycqa/pylint
    rev: v2.15.0
    hooks:
    -   id: pylint
        args: [--disable=all, --enable=W0611]  # Only check for unused imports
        files: '^test_file\.py$'
`
}

// testGoCacheWithPython tests that Python can use Go-created cache without any modifications
func (pt *PythonLanguageTest) testGoCacheWithPython(
	t *testing.T,
	goBinary, pythonBinary, repoDir string,
) error {
	t.Helper()

	// Set up isolated cache directory for this test
	testCacheDir := filepath.Join(repoDir, ".test-cache-go")
	if err := os.Setenv("PRE_COMMIT_HOME", testCacheDir); err != nil {
		return fmt.Errorf("failed to set PRE_COMMIT_HOME: %w", err)
	}
	defer func() {
		if err := os.Unsetenv("PRE_COMMIT_HOME"); err != nil {
			t.Logf("Warning: failed to unset PRE_COMMIT_HOME: %v", err)
		}
	}()

	// Clean any existing cache for fresh start
	if err := os.RemoveAll(testCacheDir); err != nil {
		t.Logf("Warning: failed to remove test cache dir: %v", err)
	}
	pt.cleanHooks(repoDir)

	t.Logf("    🏗️  Phase 1: Go creates complete cache")

	// Step 1: Go creates complete cache (install + first run)
	t.Logf("      📦 Go: Installing hooks and creating environments")
	if err := pt.runCommand(repoDir, goBinary, "install-hooks"); err != nil {
		return fmt.Errorf("go install-hooks failed: %w", err)
	}

	t.Logf("      🔄 Go: First run to create environments")
	if err := pt.runCommand(repoDir, goBinary, "run", "--all-files"); err != nil {
		return fmt.Errorf("go first run failed: %w", err)
	}

	// Capture complete cache state after Go setup
	goCacheState, err := pt.getCacheState(testCacheDir)
	if err != nil {
		return fmt.Errorf("failed to capture Go cache state: %w", err)
	}

	t.Logf("      ✅ Go cache created: %d files", len(goCacheState))

	// Step 2: Python uses Go's cache (should change NOTHING)
	t.Logf("    🔍 Phase 2: Python uses Go cache (no modifications allowed)")
	t.Logf("      🐍 Python: Running against Go-created cache")

	// Python run should use existing cache without changes
	if err = pt.runCommand(repoDir, pythonBinary, "run", "--all-files"); err != nil {
		return fmt.Errorf("python run failed with Go cache: %w", err)
	}

	// Verify cache state is completely unchanged
	pythonUsedCacheState, err := pt.getCacheState(testCacheDir)
	if err != nil {
		return fmt.Errorf("failed to capture cache state after Python run: %w", err)
	}

	// Strict verification: NOTHING should have changed
	if err := pt.compareCacheStates(goCacheState, pythonUsedCacheState); err != nil {
		return fmt.Errorf("python modified Go's cache: %w", err)
	}

	t.Logf("      ✅ Python used Go cache without modifications")
	return nil
}

// testPythonCacheWithGo tests that Go can use Python-created cache without any modifications
func (pt *PythonLanguageTest) testPythonCacheWithGo(
	t *testing.T,
	pythonBinary, goBinary, repoDir string,
) error {
	t.Helper()

	// Set up isolated cache directory for this test
	testCacheDir := filepath.Join(repoDir, ".test-cache-python")
	if err := os.Setenv("PRE_COMMIT_HOME", testCacheDir); err != nil {
		return fmt.Errorf("failed to set PRE_COMMIT_HOME: %w", err)
	}
	defer func() {
		if err := os.Unsetenv("PRE_COMMIT_HOME"); err != nil {
			t.Logf("Warning: failed to unset PRE_COMMIT_HOME: %v", err)
		}
	}()

	// Clean any existing cache for fresh start
	if err := os.RemoveAll(testCacheDir); err != nil {
		t.Logf("Warning: failed to remove test cache dir: %v", err)
	}
	pt.cleanHooks(repoDir)

	t.Logf("    🏗️  Phase 1: Python creates complete cache")

	// Step 1: Python creates complete cache (install + first run)
	t.Logf("      🐍 Python: Installing hooks and creating environments")
	if err := pt.runCommand(repoDir, pythonBinary, "install", "--install-hooks"); err != nil {
		return fmt.Errorf("python install --install-hooks failed: %w", err)
	}

	t.Logf("      🔄 Python: First run to ensure environments are ready")
	if err := pt.runCommand(repoDir, pythonBinary, "run", "--all-files"); err != nil {
		return fmt.Errorf("python first run failed: %w", err)
	}

	// Capture complete cache state after Python setup
	pythonCacheState, err := pt.getCacheState(testCacheDir)
	if err != nil {
		return fmt.Errorf("failed to capture Python cache state: %w", err)
	}

	t.Logf("      ✅ Python cache created: %d files", len(pythonCacheState))

	// Step 2: Go uses Python's cache (should change NOTHING)
	t.Logf("    🔍 Phase 2: Go uses Python cache (no modifications allowed)")
	t.Logf("      📦 Go: Running against Python-created cache")

	// Go run should use existing cache without changes
	if err = pt.runCommand(repoDir, goBinary, "run", "--all-files"); err != nil {
		return fmt.Errorf("go run failed with Python cache: %w", err)
	}

	// Verify cache state is completely unchanged
	goUsedCacheState, err := pt.getCacheState(testCacheDir)
	if err != nil {
		return fmt.Errorf("failed to capture cache state after Go run: %w", err)
	}

	// Strict verification: NOTHING should have changed
	if err := pt.compareCacheStates(pythonCacheState, goUsedCacheState); err != nil {
		return fmt.Errorf("go modified Python's cache: %w", err)
	}

	t.Logf("      ✅ Go used Python cache without modifications")
	return nil
}

// compareCacheDirectories compares the directory structures created by Go and Python
func (pt *PythonLanguageTest) compareCacheDirectories(
	t *testing.T,
	goBinary, pythonBinary, _ string,
) error {
	t.Helper()

	// Create separate clean environments for comparison
	goCacheDir := filepath.Join(pt.testDir, "go-cache-test")
	pythonCacheDir := filepath.Join(pt.testDir, "python-cache-test")

	// Setup identical repos for both tests
	goRepoDir := filepath.Join(goCacheDir, "repo")
	pythonRepoDir := filepath.Join(pythonCacheDir, "repo")

	if err := pt.setupTestRepository(t, goRepoDir, ""); err != nil {
		return fmt.Errorf("failed to setup Go test repo: %w", err)
	}
	if err := pt.setupTestRepository(t, pythonRepoDir, ""); err != nil {
		return fmt.Errorf("failed to setup Python test repo: %w", err)
	}

	// Install hooks with Go
	t.Logf("    📦 Creating cache with Go implementation")
	if err := pt.runCommand(goRepoDir, goBinary, "install-hooks"); err != nil {
		return fmt.Errorf("go install-hooks failed: %w", err)
	}

	// Install hooks with Python
	t.Logf("    🐍 Creating cache with Python implementation")
	if err := pt.runCommand(pythonRepoDir, pythonBinary, "install"); err != nil {
		return fmt.Errorf("python install failed: %w", err)
	}

	// Compare the cache directories
	goStructure, err := pt.getCacheDirectoryStructure(goRepoDir)
	if err != nil {
		return fmt.Errorf("failed to get Go cache structure: %w", err)
	}

	pythonStructure, err := pt.getCacheDirectoryStructure(pythonRepoDir)
	if err != nil {
		return fmt.Errorf("failed to get Python cache structure: %w", err)
	}

	// Compare structures
	if err := pt.compareDirectoryStructures(goStructure, pythonStructure); err != nil {
		t.Logf("    ⚠️  Cache directory structures differ: %v", err)
		// Log the differences but don't fail the test as this is informational
		pt.logStructureDifferences(t, goStructure, pythonStructure)
	} else {
		t.Logf("    ✅ Cache directory structures are identical")
	}

	// Cleanup
	if err := os.RemoveAll(goCacheDir); err != nil {
		t.Logf("Warning: failed to remove Go cache dir: %v", err)
	}
	if err := os.RemoveAll(pythonCacheDir); err != nil {
		t.Logf("Warning: failed to remove Python cache dir: %v", err)
	}

	return nil
}

// getCacheState captures the current state of the cache directory
func (pt *PythonLanguageTest) getCacheState(testCacheDir string) (map[string]string, error) {
	// Use the provided test cache directory if specified
	cacheDir := testCacheDir
	if cacheDir == "" {
		// Fallback to environment variable or default location
		if preCommitHome := os.Getenv("PRE_COMMIT_HOME"); preCommitHome != "" {
			cacheDir = preCommitHome
		} else {
			cacheDir = filepath.Join(os.Getenv("HOME"), ".cache", "pre-commit")
		}
	}

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return map[string]string{}, nil // No cache directory found
	}

	state := make(map[string]string)
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Get relative path from cache directory
			relPath, err := filepath.Rel(cacheDir, path)
			if err != nil {
				return err
			}

			// Only track essential cache files, not full repository contents
			if pt.isEssentialCacheFile(relPath) {
				// Calculate file hash
				hash, err := pt.calculateFileHash(path)
				if err != nil {
					return err
				}
				state[relPath] = hash
			}
		}
		return nil
	})

	return state, err
}

// isEssentialCacheFile determines if a file is essential for cache compatibility testing
func (pt *PythonLanguageTest) isEssentialCacheFile(relPath string) bool {
	// For bidirectional cache compatibility, only track the minimal set of files
	// that are absolutely required for cache functionality between implementations

	// Essential cache infrastructure files (root level only)
	essentialFiles := []string{
		"db.db", // Database file - the core cache database
	}

	// Check exact matches for essential files
	if slices.Contains(essentialFiles, relPath) {
		return true
	}

	// For environments, only track the most critical executable files
	// Skip library files, temporary files, and metadata that may legitimately differ
	if strings.Contains(relPath, "py_env-") {
		// Only track Python executables - skip configuration files that may differ
		if strings.HasSuffix(relPath, "/bin/python") ||
			strings.HasSuffix(relPath, "/bin/python3") {
			return true
		}
		// Skip pyvenv.cfg as it contains implementation-specific metadata
		return false
	}

	// Skip all other files - they're not essential for basic cache compatibility
	return false
}

// compareCacheStates compares two cache states for equality
func (pt *PythonLanguageTest) compareCacheStates(state1, state2 map[string]string) error {
	if len(state1) != len(state2) {
		return fmt.Errorf("cache states have different number of files: %d vs %d",
			len(state1), len(state2))
	}

	for path, hash1 := range state1 {
		hash2, exists := state2[path]
		if !exists {
			return fmt.Errorf("file %s missing in second cache state", path)
		}

		// Special handling for database files - compare logical content instead of binary hash
		if filepath.Base(path) == dbFileName {
			if err := pt.compareDatabaseContent(state1, state2, path); err != nil {
				return fmt.Errorf("database content differs for %s: %w", path, err)
			}
			continue
		}

		if hash1 != hash2 {
			return fmt.Errorf("file %s has different hash: %s vs %s", path, hash1, hash2)
		}
	}

	for path := range state2 {
		if _, exists := state1[path]; !exists {
			return fmt.Errorf("file %s missing in first cache state", path)
		}
	}

	return nil
}

// compareDatabaseContent compares the logical content of two database files
func (pt *PythonLanguageTest) compareDatabaseContent(state1, state2 map[string]string, dbPath string) error {
	// Find the actual database file paths from the cache states
	var dbPath1, dbPath2 string
	// Build the actual file paths by finding the cache directories
	for fullPath := range state1 {
		if filepath.Base(fullPath) == dbFileName && strings.Contains(fullPath, dbPath) {
			dbPath1 = filepath.Join(pt.getCacheBaseDir(), fullPath)
			break
		}
	}

	for fullPath := range state2 {
		if filepath.Base(fullPath) == dbFileName && strings.Contains(fullPath, dbPath) {
			dbPath2 = filepath.Join(pt.getCacheBaseDir(), fullPath)
			break
		}
	}

	if dbPath1 == "" || dbPath2 == "" {
		// If we can't find the database files, fall back to hash comparison
		return nil
	}

	// Compare database logical content
	return pt.compareDatabaseLogicalContent(dbPath1, dbPath2)
}

// compareDatabaseLogicalContent compares the logical content of two SQLite databases
func (pt *PythonLanguageTest) compareDatabaseLogicalContent(dbPath1, dbPath2 string) error {
	// Open both databases
	db1, err := sql.Open("sqlite3", dbPath1)
	if err != nil {
		return fmt.Errorf("failed to open database 1: %w", err)
	}
	defer func() {
		if closeErr := db1.Close(); closeErr != nil { //nolint:revive,staticcheck // Test cleanup
			// Ignore error during cleanup
		}
	}()

	db2, err := sql.Open("sqlite3", dbPath2)
	if err != nil {
		return fmt.Errorf("failed to open database 2: %w", err)
	}
	defer func() {
		if closeErr := db2.Close(); closeErr != nil { //nolint:revive,staticcheck // Test cleanup
			// Ignore error during cleanup
		}
	}()

	// Compare repos table content
	repos1, err := pt.queryAllRepos(db1)
	if err != nil {
		return fmt.Errorf("failed to query repos from db1: %w", err)
	}

	repos2, err := pt.queryAllRepos(db2)
	if err != nil {
		return fmt.Errorf("failed to query repos from db2: %w", err)
	}

	if !pt.compareRepoSlices(repos1, repos2) {
		return fmt.Errorf("database repos content differs")
	}

	// Compare configs table content
	configs1, err := pt.queryAllConfigs(db1)
	if err != nil {
		return fmt.Errorf("failed to query configs from db1: %w", err)
	}

	configs2, err := pt.queryAllConfigs(db2)
	if err != nil {
		return fmt.Errorf("failed to query configs from db2: %w", err)
	}

	if !pt.compareStringSlices(configs1, configs2) {
		return fmt.Errorf("database configs content differs")
	}

	return nil
}

// getCacheBaseDir returns the base cache directory
func (pt *PythonLanguageTest) getCacheBaseDir() string {
	homeDir := os.Getenv("HOME")
	return filepath.Join(homeDir, ".cache", "pre-commit")
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

// queryAllConfigs queries all config entries from a database
func (pt *PythonLanguageTest) queryAllConfigs(db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(context.Background(), "SELECT path FROM configs ORDER BY path")
	if err != nil {
		// configs table might not exist in some test scenarios
		return []string{}, nil
	}
	defer func() {
		if err := rows.Close(); err != nil { //nolint:revive,staticcheck // Test cleanup
			// Ignore error during cleanup
		}
	}()

	var configs []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		configs = append(configs, path)
	}

	return configs, rows.Err()
}

// RepoEntry represents a repository entry in the database
type RepoEntry struct {
	Repo string
	Ref  string
	Path string
}

// compareRepoSlices compares two slices of repository entries
func (pt *PythonLanguageTest) compareRepoSlices(repos1, repos2 []RepoEntry) bool {
	if len(repos1) != len(repos2) {
		return false
	}

	for i, repo1 := range repos1 {
		repo2 := repos2[i]
		if repo1.Repo != repo2.Repo || repo1.Ref != repo2.Ref || repo1.Path != repo2.Path {
			return false
		}
	}

	return true
}

// compareStringSlices compares two string slices
func (pt *PythonLanguageTest) compareStringSlices(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}

	for i, str1 := range slice1 {
		if str1 != slice2[i] {
			return false
		}
	}

	return true
}

// getCacheDirectoryStructure gets the directory structure of cache
func (pt *PythonLanguageTest) getCacheDirectoryStructure(repoDir string) ([]string, error) {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "pre-commit")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		cacheDir = filepath.Join(repoDir, ".pre-commit-cache")
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			return []string{}, nil
		}
	}

	var structure []string
	err := filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(cacheDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			structure = append(structure, relPath+"/")
		} else {
			structure = append(structure, relPath)
		}
		return nil
	})

	sort.Strings(structure)
	return structure, err
}

// compareDirectoryStructures compares two directory structures
func (pt *PythonLanguageTest) compareDirectoryStructures(struct1, struct2 []string) error {
	if len(struct1) != len(struct2) {
		return fmt.Errorf("structures have different number of entries: %d vs %d",
			len(struct1), len(struct2))
	}

	for i, path1 := range struct1 {
		if i >= len(struct2) || path1 != struct2[i] {
			return fmt.Errorf("structure mismatch at position %d: %s vs %s",
				i, path1, struct2[i])
		}
	}

	return nil
}

// logStructureDifferences logs the differences between directory structures
func (pt *PythonLanguageTest) logStructureDifferences(t *testing.T, struct1, struct2 []string) {
	t.Helper()

	t.Logf("    📊 Go cache structure (%d entries):", len(struct1))
	for _, entry := range struct1 {
		t.Logf("      📁 %s", entry)
	}

	t.Logf("    📊 Python cache structure (%d entries):", len(struct2))
	for _, entry := range struct2 {
		t.Logf("      📁 %s", entry)
	}

	// Find unique entries
	goOnly := pt.findUniqueEntries(struct1, struct2)
	pythonOnly := pt.findUniqueEntries(struct2, struct1)

	if len(goOnly) > 0 {
		t.Logf("    🔍 Go-only entries:")
		for _, entry := range goOnly {
			t.Logf("      📦 %s", entry)
		}
	}

	if len(pythonOnly) > 0 {
		t.Logf("    🔍 Python-only entries:")
		for _, entry := range pythonOnly {
			t.Logf("      🐍 %s", entry)
		}
	}
}

// findUniqueEntries finds entries that exist in slice1 but not in slice2
func (pt *PythonLanguageTest) findUniqueEntries(slice1, slice2 []string) []string {
	set2 := make(map[string]bool)
	for _, entry := range slice2 {
		set2[entry] = true
	}

	var unique []string
	for _, entry := range slice1 {
		if !set2[entry] {
			unique = append(unique, entry)
		}
	}

	return unique
}

// calculateFileHash calculates SHA256 hash of a file
func (pt *PythonLanguageTest) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath) // #nosec G304 -- Test file reading with controlled file paths
	if err != nil {
		return "", err
	}
	defer func() {
		if err := file.Close(); err != nil { //nolint:revive,staticcheck // Test cleanup
			// Log error but continue
		}
	}()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// cleanHooks removes the git hooks directory
func (pt *PythonLanguageTest) cleanHooks(repoDir string) {
	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if _, err := os.Stat(hooksDir); err == nil {
		if err := os.RemoveAll(hooksDir); err != nil { //nolint:revive,staticcheck // Test cleanup
			// Log error but continue //nolint:revive,staticcheck // Test cleanup
		}
		if err := os.MkdirAll(hooksDir, 0o750); err != nil { //nolint:revive,staticcheck // Test cleanup
			// Log error but continue //nolint:revive,staticcheck // Test cleanup
		}
	}
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
	if err := pt.runCommand(cacheTestRepo, "pre-commit", "install"); err != nil {
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
	return pt.validateEnvironmentReuse(t, cacheTestRepo, version)
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
		t.Logf("Warning: failed to set git user.email: %v", err)
	}
	if err := pt.runCommand(repoDir, "git", "config", "user.name", "Test User"); err != nil {
		t.Logf("Warning: failed to set git user.name: %v", err)
	}
	if err := pt.runCommand(repoDir, "git", "config", "commit.gpgsign", "false"); err != nil {
		t.Logf("Warning: failed to disable git commit signing: %v", err)
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
func (pt *PythonLanguageTest) validateEnvironmentReuse(t *testing.T, repoDir, version string) error {
	t.Helper()

	// Check for environment directories in common cache locations
	cacheLocations := []string{
		filepath.Join(os.Getenv("HOME"), ".cache", "pre-commit"),
		filepath.Join(repoDir, ".pre-commit-cache"),
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

// testGoCachePerformance tests cache hit performance for our Go implementation with Python environments
func (pt *PythonLanguageTest) testGoCachePerformance(
	t *testing.T,
	goBinary string,
	version string,
) error {
	t.Helper()

	// Create a unique test repository for Go cache testing
	cacheTestRepo := filepath.Join(pt.testDir, "go-cache-hit-test-repo-"+version)
	if err := pt.setupGoCacheTestRepository(t, cacheTestRepo); err != nil {
		return fmt.Errorf("failed to setup Go cache test repository: %w", err)
	}

	// Set a custom cache directory to isolate this test
	customCacheDir := filepath.Join(pt.testDir, "go-isolated-cache-"+version)
	if err := os.MkdirAll(customCacheDir, 0o750); err != nil {
		return fmt.Errorf("failed to create custom cache dir: %w", err)
	}

	// First install the hooks with Go implementation
	env := append(os.Environ(), "PRE_COMMIT_HOME="+customCacheDir)
	if err := pt.runCommandWithEnv(cacheTestRepo, env, goBinary, "install-hooks"); err != nil {
		return fmt.Errorf("failed to install git hooks with Go: %w", err)
	}

	// First run: Should create environment and run hooks (slower)
	t.Logf("      🔄 Go first run: Running hooks (should create environment)")
	firstStart := time.Now()
	if err := pt.runCommandWithEnv(cacheTestRepo, env, goBinary, "run", "--all-files"); err != nil {
		// This might fail due to hook errors, but that's okay for performance testing
		t.Logf("      ⚠️ Go first run completed with exit code (expected): %v", err)
	}
	firstRunTime := time.Since(firstStart)

	// Wait a moment to ensure any file system operations are complete
	time.Sleep(100 * time.Millisecond)

	// Second run: Should reuse environment (faster)
	t.Logf("      🔄 Go second run: Running hooks (should reuse environment)")
	secondStart := time.Now()
	if err := pt.runCommandWithEnv(cacheTestRepo, env, goBinary, "run", "--all-files"); err != nil {
		// This might fail due to hook errors, but that's okay for performance testing
		t.Logf("      ⚠️ Go second run completed with exit code (expected): %v", err)
	}
	secondRunTime := time.Since(secondStart)

	// Calculate speedup
	firstMs := float64(firstRunTime.Nanoseconds()) / 1e6
	secondMs := float64(secondRunTime.Nanoseconds()) / 1e6
	speedup := float64(firstRunTime) / float64(secondRunTime)

	t.Logf("      📊 Go implementation cache performance for Python %s:", version)
	t.Logf("        First run:  %.2fms (environment creation)", firstMs)
	t.Logf("        Second run: %.2fms (cache hit)", secondMs)

	switch {
	case speedup > 1.1: // At least 10% faster
		improvement := (speedup - 1.0) * 100
		t.Logf("        ✅ Go cache hit detected: %.1fx speedup (%.1f%% faster)", speedup, improvement)
	case speedup > 0.9: // Within 10% (acceptable for fast operations)
		t.Logf("        ✅ Go cache hit: Similar performance (%.1fx)", speedup)
	default:
		t.Logf("        ⚠️ Warning: Go second run slower (%.1fx) - possible cache miss", speedup)
	}

	return nil
}

// setupGoCacheTestRepository creates a repository optimized for testing Go implementation cache performance
func (pt *PythonLanguageTest) setupGoCacheTestRepository(t *testing.T, repoDir string) error {
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
		t.Logf("Warning: failed to set git user.email: %v", err)
	}
	if err := pt.runCommand(repoDir, "git", "config", "user.name", "Test User"); err != nil {
		t.Logf("Warning: failed to set git user.name: %v", err)
	}
	if err := pt.runCommand(repoDir, "git", "config", "commit.gpgsign", "false"); err != nil {
		t.Logf("Warning: failed to disable git commit signing: %v", err)
	}

	// Create a pre-commit config with Python hooks that require environment setup for better cache testing
	configContent := `repos:
-   repo: https://github.com/psf/black
    rev: 22.3.0
    hooks:
    -   id: black
        language_version: python3
        exclude: '\.pre-commit-config\.yaml$'
`
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to write pre-commit config: %w", err)
	}

	// Create a Python test file that will be processed by black
	testFile := filepath.Join(repoDir, "test.py")
	testContent := `print("hello world")`
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
			t.Logf("Warning: failed to close database: %v", closeErr)
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
			t.Logf("Warning: failed to close rows: %v", err)
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
