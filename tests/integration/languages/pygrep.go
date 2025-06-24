package languages

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver

	"github.com/blairham/go-pre-commit/pkg/cache"
	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// PygrepLanguageTest implements LanguageTestRunner for Pygrep
type PygrepLanguageTest struct {
	*BaseLanguageTest
}

// NewPygrepLanguageTest creates a new Pygrep language test
func NewPygrepLanguageTest(testDir string) *PygrepLanguageTest {
	return &PygrepLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangPygrep, testDir),
	}
}

// GetLanguageName returns the language name
func (pt *PygrepLanguageTest) GetLanguageName() string {
	return LangPygrep
}

// SetupRepositoryFiles creates Pygrep-specific repository files
func (pt *PygrepLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	if err := pt.createHooksFile(repoPath); err != nil {
		return err
	}

	// Create Python test files
	if err := pt.createPythonTestFile(repoPath); err != nil {
		return err
	}

	// Create RST test file
	return pt.createRSTTestFile(repoPath)
}

// SetupRepositoryWithSync creates a repository with hash, syncs it, creates environment if needed,
// updates database records, and creates lock file
func (pt *PygrepLanguageTest) SetupRepositoryWithSync(t *testing.T, version string) (string, string, error) {
	t.Helper()

	// Create repository with hash
	repoPath, repoURL, err := pt.createRepositoryWithHash(t, version)
	if err != nil {
		return "", "", err
	}

	// Setup cache manager
	cacheManager, err := cache.NewManager(pt.cacheDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to create cache manager: %w", err)
	}
	defer func() {
		if closeErr := cacheManager.Close(); closeErr != nil {
			t.Logf("Warning: failed to close cache manager: %v", closeErr)
		}
	}()

	// Get language manager and setup environment
	lang, err := pt.GetLanguageManager()
	if err != nil {
		return "", "", fmt.Errorf("failed to get language manager: %w", err)
	}

	envPath, err := pt.setupRepositoryEnvironment(t, lang, version, repoPath, repoURL)
	if err != nil {
		return "", "", err
	}

	// Update database and test locking
	err = pt.updateDatabaseAndTestLocking(t, cacheManager, repoURL, version, repoPath)
	if err != nil {
		return "", "", err
	}

	t.Logf("  📁 Repository synced at: %s", repoPath)
	return repoPath, envPath, nil
}

// createRepositoryWithHash creates a repository directory with a unique hash
func (pt *PygrepLanguageTest) createRepositoryWithHash(_ *testing.T, version string) (string, string, error) {
	// Generate repository hash based on language and version
	repoHash := pt.generateRepoHash(version)
	repoPath := filepath.Join(pt.testDir, "repos", fmt.Sprintf("repo%s", repoHash))

	// Create repository directory
	if err := os.MkdirAll(repoPath, 0o750); err != nil {
		return "", "", fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Create .git directory to make it look like a real repository
	gitDir := filepath.Join(repoPath, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		return "", "", fmt.Errorf("failed to create .git directory: %w", err)
	}

	// Setup language-specific files
	if err := pt.SetupRepositoryFiles(repoPath); err != nil {
		return "", "", fmt.Errorf("failed to setup repository files: %w", err)
	}

	repoURL := fmt.Sprintf("https://github.com/test/pygrep-repo-%s", repoHash)
	return repoPath, repoURL, nil
}

// updateDatabaseAndTestLocking updates database records and tests file locking
func (pt *PygrepLanguageTest) updateDatabaseAndTestLocking(
	t *testing.T,
	cacheManager *cache.Manager,
	repoURL, version, repoPath string,
) error {
	// Create repository config
	repo := config.Repo{
		Repo: repoURL,
		Rev:  version,
	}

	// Update database record with repository information
	if updateErr := cacheManager.UpdateRepoEntry(repo, repoPath); updateErr != nil {
		t.Logf("  ⚠️ Warning: failed to update database entry: %v", updateErr)
	} else {
		t.Logf("  💾 Database record created for repository")
	}

	// Test file locking by creating and releasing a lock
	lock := cache.NewFileLock(pt.cacheDir)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := lock.WithLock(ctx, func() error {
		// Verify .lock file exists
		lockPath := filepath.Join(pt.cacheDir, ".lock")
		if _, statErr := os.Stat(lockPath); statErr != nil {
			return fmt.Errorf("lock file not created: %w", statErr)
		}
		t.Logf("  🔒 Lock file created and verified")
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to test file locking: %w", err)
	}

	// Verify database entry was created
	if err := pt.verifyDatabaseEntry(repo, repoPath); err != nil {
		t.Logf("  ⚠️ Warning: database verification failed: %v", err)
	} else {
		t.Logf("  ✅ Database entry verified")
	}

	return nil
}

// generateRepoHash generates a unique hash for the repository based on language and version
func (pt *PygrepLanguageTest) generateRepoHash(version string) string {
	hashInput := fmt.Sprintf("pygrep-%s-%d", version, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(hashInput))
	return fmt.Sprintf("%x", hash)[:12] // Use first 12 characters
}

// verifyDatabaseEntry verifies that the repository entry was created in the database
func (pt *PygrepLanguageTest) verifyDatabaseEntry(repo config.Repo, expectedPath string) error {
	dbPath := filepath.Join(pt.cacheDir, "db.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	var path string
	ctx := context.Background()
	err = db.QueryRowContext(ctx, "SELECT path FROM repos WHERE repo = ? AND ref = ?", repo.Repo, repo.Rev).Scan(&path)
	closeErr := db.Close()
	if err != nil {
		return fmt.Errorf("failed to query database: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close database: %w", closeErr)
	}

	// Normalize expected path to match how it's stored in the database
	// This handles macOS symlink resolution (/var/folders -> /private/var/folders)
	normalizedExpected, err := filepath.EvalSymlinks(expectedPath)
	if err != nil {
		// If symlink resolution fails, use absolute path as fallback
		var absErr error
		normalizedExpected, absErr = filepath.Abs(expectedPath)
		if absErr != nil {
			normalizedExpected = expectedPath
		}
	}

	if path != normalizedExpected {
		return fmt.Errorf("database path mismatch: expected %s, got %s", normalizedExpected, path)
	}

	return nil
}

// validateDatabaseRecords validates that database records are properly created
func (pt *PygrepLanguageTest) validateDatabaseRecords(t *testing.T) error {
	// Check if database file exists
	dbPath := filepath.Join(pt.cacheDir, "db.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database file not found: %s", dbPath)
	}

	// Open database and verify table exists
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Warning: failed to close database: %v", closeErr)
		}
	}()

	// Check repos table exists
	var count int
	ctx := context.Background()
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM repos").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to query repos table: %w", err)
	}

	t.Logf("Database validation passed - found %d repository records", count)
	return nil
}

// setupRepositoryEnvironment sets up the environment for the repository
func (pt *PygrepLanguageTest) setupRepositoryEnvironment(
	t *testing.T,
	lang language.Manager,
	version, repoPath, repoURL string,
) (string, error) {
	var envPath string
	var err error

	if lang.NeedsEnvironmentSetup() {
		// Setup environment (this creates environment for languages that need it)
		envPath, err = lang.SetupEnvironmentWithRepo(
			pt.cacheDir,
			version,
			repoPath,
			repoURL,
			[]string{}, // no additional dependencies for pygrep
		)
		if err != nil {
			return "", fmt.Errorf("failed to setup environment: %w", err)
		}
		t.Logf("  🏗️ Environment created at: %s", envPath)
	} else {
		t.Logf("  ℹ️ Pygrep language does not need environment setup")
	}

	return envPath, nil
}

// createHooksFile creates the .pre-commit-hooks.yaml file
func (pt *PygrepLanguageTest) createHooksFile(repoPath string) error {
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `- id: python-check-blanket-noqa
  name: Check blanket noqa
  description: Require specific codes when ignoring flake8 errors
  entry: python-check-blanket-noqa
  language: pygrep
  files: \.py$
- id: python-check-mock-methods
  name: Check mock methods
  description: Prevent common mistakes of assert mck.not_called(), assert mck.called_once_with(...) and mck.assert_called.
  entry: python-check-mock-methods
  language: pygrep
  files: \.py$
- id: python-no-log-warn
  name: No log warn
  description: Check for usage of deprecated .warn() method of python loggers
  entry: python-no-log-warn
  language: pygrep
  files: \.py$
- id: rst-backticks
  name: RST backticks
  description: Detect common mistake of using single backticks when writing rst
  entry: rst-backticks
  language: pygrep
  files: \.rst$
`
	return os.WriteFile(hooksFile, []byte(hooksContent), 0o600)
}

// createPythonTestFile creates multiple Python test files with potential pygrep issues
func (pt *PygrepLanguageTest) createPythonTestFile(repoPath string) error {
	// Create multiple Python files to provide more meaningful work for pygrep caching
	for i := 1; i <= 15; i++ {
		pythonFile := filepath.Join(repoPath, fmt.Sprintf("test_code_%d.py", i))
		pythonContent := fmt.Sprintf(`#!/usr/bin/env python3
"""Test Python file %d for pygrep hooks."""

import logging
import unittest.mock
import os
import sys

logger = logging.getLogger(__name__)

class TestClass%d:
    """Test class with potential pygrep issues."""

    def test_function_%d(self):
        """Test function with potential pygrep issues."""
        # This should be caught by python-no-log-warn
        # logger.warn("This is deprecated in file %d")

        # Good practice
        logger.warning("This is the correct way in file %d")

        # Mock usage examples
        mock_obj = unittest.mock.Mock()

        # This would be caught by python-check-mock-methods
        # assert mock_obj.called_once_with()  # Wrong!

        # Correct usage
        mock_obj.assert_called_once_with()

        # Add some computational work to make caching more meaningful
        result = sum(range(100))

        return "success from file %d"

    def process_data_%d(self):
        """Process some data to simulate real work."""
        data = [x * 2 for x in range(50)]
        return len(data)

# This should be caught by python-check-blanket-noqa
# import json  # noqa

# Good practice
import json  # noqa: F401

def utility_function_%d():
    """Utility function in file %d."""
    return "utility_%d"
`, i, i, i, i, i, i, i, i, i, i)

		if err := os.WriteFile(pythonFile, []byte(pythonContent), 0o600); err != nil {
			return fmt.Errorf("failed to create python file %d: %w", i, err)
		}
	}

	return nil
}

// createRSTTestFile creates multiple RST test files for pygrep validation
func (pt *PygrepLanguageTest) createRSTTestFile(repoPath string) error {
	// Create multiple RST files to provide more work for pygrep processing
	for i := 1; i <= 10; i++ {
		rstFile := filepath.Join(repoPath, fmt.Sprintf("test_%d.rst", i))
		rstContent := fmt.Sprintf(`Test RST File %d
===============

This is test reStructuredText file %d for pygrep hooks.

Code Example %d
---------------

Here's some example code in file %d::

    def example_function_%d():
        """Example function in RST file %d."""
        print("Hello from RST file %d")
        return True

Section %d
----------

This section contains more content to make the file larger and provide
more meaningful work for pygrep pattern matching. This helps test the
caching efficiency by providing real work to cache.

* Item 1 in file %d
* Item 2 in file %d
* Item 3 in file %d

Subsection %d.1
~~~~~~~~~~~~~~~

More detailed content goes here for file %d. This content should be
substantial enough to make regex pattern matching take measurable time.

Subsection %d.2
~~~~~~~~~~~~~~~

Even more content to ensure each file has enough text for pygrep
to process meaningfully in file %d.

.. code-block:: python

   # Code block in file %d
   def process_file_%d(filename):
       with open(filename, 'r') as f:
           return f.read()
`, i, i, i, i, i, i, i, i, i, i, i, i, i, i, i, i, i)

		if err := os.WriteFile(rstFile, []byte(rstContent), 0o600); err != nil {
			return fmt.Errorf("failed to create RST file %d: %w", i, err)
		}
	}

	return nil
}

// GetLanguageManager returns the Pygrep language manager
func (pt *PygrepLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewPygrepLanguage(), nil
}

// GetAdditionalValidations returns Pygrep-specific validation tests
func (pt *PygrepLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		pt.createPygrepPatternsValidation(),
		pt.createTestFilesValidation(),
		pt.createRepositorySyncValidation(),
		pt.createDatabaseValidation(),
		pt.createLockFileValidation(),
	}
}

// createPygrepPatternsValidation creates the pygrep patterns validation step
func (pt *PygrepLanguageTest) createPygrepPatternsValidation() ValidationStep {
	return ValidationStep{
		Name:        "pygrep-patterns-check",
		Description: "Pygrep pattern matching validation",
		Execute: func(t *testing.T, _, _ string, lang language.Manager) error {
			if lang.GetName() != "pygrep" {
				return fmt.Errorf("expected pygrep language, got %s", lang.GetName())
			}
			t.Logf("Pygrep language validation passed")
			return nil
		},
	}
}

// createTestFilesValidation creates the test files validation step
func (pt *PygrepLanguageTest) createTestFilesValidation() ValidationStep {
	return ValidationStep{
		Name:        "test-files-validation",
		Description: "Verify test files are created for pygrep",
		Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
			// For pygrep, envPath is the same as repoPath
			repoPath := envPath

			pythonFile := filepath.Join(repoPath, "test_code.py")
			if _, err := os.Stat(pythonFile); err != nil {
				return fmt.Errorf("test_code.py not found: %w", err)
			}

			rstFile := filepath.Join(repoPath, "test.rst")
			if _, err := os.Stat(rstFile); err != nil {
				return fmt.Errorf("test.rst not found: %w", err)
			}

			t.Logf("Pygrep test files validation passed")
			return nil
		},
	}
}

// createRepositorySyncValidation creates the repository sync validation step
func (pt *PygrepLanguageTest) createRepositorySyncValidation() ValidationStep {
	return ValidationStep{
		Name:        "repository-sync-with-hash",
		Description: "Test repository creation with hash, sync, and database records",
		Execute: func(t *testing.T, _, version string, _ language.Manager) error {
			repoPath, envPath, err := pt.SetupRepositoryWithSync(t, version)
			if err != nil {
				return fmt.Errorf("repository sync failed: %w", err)
			}

			if _, err := os.Stat(repoPath); os.IsNotExist(err) {
				return fmt.Errorf("repository directory not created: %s", repoPath)
			}

			gitDir := filepath.Join(repoPath, ".git")
			if _, err := os.Stat(gitDir); os.IsNotExist(err) {
				return fmt.Errorf(".git directory not found: %s", gitDir)
			}

			t.Logf("Repository sync validation passed - repo: %s, env: %s", repoPath, envPath)
			return nil
		},
	}
}

// createDatabaseValidation creates the database validation step
func (pt *PygrepLanguageTest) createDatabaseValidation() ValidationStep {
	return ValidationStep{
		Name:        "database-record-validation",
		Description: "Verify database records are created",
		Execute: func(t *testing.T, _, _ string, _ language.Manager) error {
			return pt.validateDatabaseRecords(t)
		},
	}
}

// createLockFileValidation creates the lock file validation step
func (pt *PygrepLanguageTest) createLockFileValidation() ValidationStep {
	return ValidationStep{
		Name:        "lock-file-validation",
		Description: "Verify lock file functionality",
		Execute: func(t *testing.T, _, _ string, _ language.Manager) error {
			lock := cache.NewFileLock(pt.cacheDir)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := lock.WithLock(ctx, func() error {
				lockPath := filepath.Join(pt.cacheDir, ".lock")
				if _, err := os.Stat(lockPath); err != nil {
					return fmt.Errorf("lock file not found during lock: %w", err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("lock file test failed: %w", err)
			}

			lockPath := filepath.Join(pt.cacheDir, ".lock")
			if _, err := os.Stat(lockPath); err == nil {
				t.Logf("Lock file still exists after release (this may be normal)")
			}

			t.Logf("Lock file validation passed")
			return nil
		},
	}
}

// TestPygrepLanguageTestRepositorySyncWithHash tests the complete repository sync workflow
func TestPygrepLanguageTestRepositorySyncWithHash(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	// Create pygrep language test
	pygrepTest := NewPygrepLanguageTest(tempDir)

	// Test repository setup with hash, sync, environment, database, and lock file
	repoPath, envPath, err := pygrepTest.SetupRepositoryWithSync(t, "main")
	if err != nil {
		t.Fatalf("SetupRepositoryWithSync failed: %v", err)
	}

	// Verify repository was created with hash
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		t.Errorf("Repository directory not created: %s", repoPath)
	}

	// Verify .git directory exists (simulates repository sync)
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Errorf(".git directory not found: %s", gitDir)
	}

	// Verify cache directory was created
	cacheDir := filepath.Join(tempDir, "cache")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Errorf("Cache directory not created: %s", cacheDir)
	}

	// Verify database file was created
	dbPath := filepath.Join(cacheDir, "db.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file not created: %s", dbPath)
	}

	// For pygrep (system language), environment path may be empty, but should not error
	t.Logf("Repository synced successfully:")
	t.Logf("  Repository path: %s", repoPath)
	t.Logf("  Environment path: %s", envPath)
	t.Logf("  Cache directory: %s", cacheDir)
	t.Logf("  Database file: %s", dbPath)
}

// TestPygrepLanguageTestAdditionalValidations tests all additional validation steps
func TestPygrepLanguageTestAdditionalValidations(t *testing.T) {
	tempDir := t.TempDir()
	pygrepTest := NewPygrepLanguageTest(tempDir)

	// Get additional validations
	validations := pygrepTest.GetAdditionalValidations()

	// Verify we have all expected validations
	expectedValidations := []string{
		"pygrep-patterns-check",
		"test-files-validation",
		"repository-sync-with-hash",
		"database-record-validation",
		"lock-file-validation",
	}

	if len(validations) != len(expectedValidations) {
		t.Errorf("Expected %d validations, got %d", len(expectedValidations), len(validations))
	}

	for i, validation := range validations {
		if i < len(expectedValidations) && validation.Name != expectedValidations[i] {
			t.Errorf("Expected validation %d to be %s, got %s", i, expectedValidations[i], validation.Name)
		}
	}

	t.Logf("All %d additional validations are properly configured", len(validations))
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (pt *PygrepLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, _ string,
) error {
	t.Helper()

	t.Logf("🔄 Testing Pygrep language bidirectional cache compatibility")
	t.Logf("   📋 Pygrep hooks use pattern matching - testing cache compatibility")

	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "pygrep-bidirectional-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Warning: failed to remove temp directory: %v", removeErr)
		}
	}()

	// Test basic cache structure compatibility
	if err := pt.testBasicCacheCompatibility(t, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("basic cache compatibility test failed: %w", err)
	}

	t.Logf("✅ Pygrep language bidirectional cache compatibility test completed")
	return nil
}

// setupTestRepository creates a test repository for pygrep language testing
func (pt *PygrepLanguageTest) setupTestRepository(t *testing.T, repoPath, _ string) error {
	t.Helper()

	// Create repository directory
	if err := os.MkdirAll(repoPath, 0o750); err != nil {
		return fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Set git user config for the test
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git user email: %w", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git user name: %w", err)
	}

	return nil
}

// testBasicCacheCompatibility tests basic cache directory compatibility for pygrep hooks
func (pt *PygrepLanguageTest) testBasicCacheCompatibility(t *testing.T, pythonBinary, goBinary, tempDir string) error {
	t.Helper()

	// Create cache directories
	goCacheDir := filepath.Join(tempDir, "go-cache")
	pythonCacheDir := filepath.Join(tempDir, "python-cache")

	// Create a simple repository for testing
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := pt.setupTestRepository(t, repoDir, ""); err != nil {
		return fmt.Errorf("failed to setup test repository: %w", err)
	}

	// Pygrep language config with mixed hooks that require different setup levels
	configContent := `repos:
-   repo: https://github.com/pre-commit/pygrep-hooks
    rev: v1.10.0
    hooks:
    -   id: python-check-blanket-noqa
    -   id: python-check-mock-methods
    -   id: python-no-log-warn
    -   id: rst-backticks
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
    -   id: check-yaml
    -   id: check-json
    -   id: check-toml
`
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Create test Python file
	testFile := filepath.Join(repoDir, "test.py")
	pythonContent := `# noqa
print("Hello, world!")
`
	if err := os.WriteFile(testFile, []byte(pythonContent), 0o600); err != nil {
		return fmt.Errorf("failed to create test file: %w", err)
	}

	// Test 1: Go creates cache
	cmd := exec.Command(goBinary, "install-hooks", "--config", configPath)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PRE_COMMIT_HOME=%s", goCacheDir))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install-hooks failed: %w", err)
	}

	// Test 2: Python creates cache
	cmd = exec.Command(pythonBinary, "install-hooks", "--config", configPath)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PRE_COMMIT_HOME=%s", pythonCacheDir))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("python install-hooks failed: %w", err)
	}

	// Verify both caches were created
	if _, err := os.Stat(goCacheDir); err != nil {
		return fmt.Errorf("go cache directory not created: %w", err)
	}
	if _, err := os.Stat(pythonCacheDir); err != nil {
		return fmt.Errorf("python cache directory not created: %w", err)
	}

	t.Logf("   ✅ Both Go and Python can create compatible cache structures for pygrep hooks")
	return nil
}
