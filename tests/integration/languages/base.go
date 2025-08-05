// Package languages provides integration test implementations for different programming languages.
package languages

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver

	"github.com/blairham/go-pre-commit/pkg/language"
)

// Language constants for testing
const (
	LangPython      = "python"
	LangNode        = "node"
	LangGolang      = "golang"
	LangRuby        = "ruby"
	LangRust        = "rust"
	LangDart        = "dart"
	LangSwift       = "swift"
	LangLua         = "lua"
	LangPerl        = "perl"
	LangR           = "r"
	LangHaskell     = "haskell"
	LangJulia       = "julia"
	LangDotnet      = "dotnet"
	LangCoursier    = "coursier"
	LangDocker      = "docker"
	LangDockerImage = "docker_image"
	LangConda       = "conda"
	LangSystem      = "system"
	LangScript      = "script"
	LangFail        = "fail"
	LangPygrep      = "pygrep"
)

// BaseLanguageTest provides common functionality for all language tests
type BaseLanguageTest struct {
	language string
	testDir  string
	cacheDir string
}

// LanguageTestRunner defines the interface that each language test must implement
type LanguageTestRunner interface {
	// SetupRepositoryFiles creates language-specific files in the test repository
	SetupRepositoryFiles(repoPath string) error

	// GetLanguageManager returns the language manager for this language
	GetLanguageManager() (language.Manager, error)

	// GetAdditionalValidations returns language-specific validation steps
	GetAdditionalValidations() []ValidationStep

	// GetLanguageName returns the name of the language being tested
	GetLanguageName() string
}

// BidirectionalTestRunner defines the interface for languages that support bidirectional cache testing
type BidirectionalTestRunner interface {
	LanguageTestRunner

	// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
	TestBidirectionalCacheCompatibility(t *testing.T, pythonBinary, goBinary, testRepo string) error
}

// ValidationStep represents a custom validation step for language testing
type ValidationStep struct {
	Execute     func(t *testing.T, envPath, version string, lang language.Manager) error
	Name        string
	Description string
}

// LanguageCompatibilityTest represents a comprehensive test for language compatibility
type LanguageCompatibilityTest struct {
	PythonPrecommitBinary    string
	Language                 string
	TestRepository           string
	TestCommit               string
	HookID                   string
	GoPrecommitBinary        string
	Name                     string
	ExpectedFiles            []string
	TestVersions             []string
	AdditionalDependencies   []string
	TestTimeout              time.Duration
	NeedsRuntimeInstalled    bool
	CacheTestEnabled         bool
	BiDirectionalTestEnabled bool
}

// NewBaseLanguageTest creates a new base language test
func NewBaseLanguageTest(lang, testDir string) *BaseLanguageTest {
	return &BaseLanguageTest{
		language: lang,
		testDir:  testDir,
		cacheDir: filepath.Join(testDir, "cache"),
	}
}

// SetupTestEnvironment sets up the common test environment (database, cache directory)
func (bt *BaseLanguageTest) SetupTestEnvironment(t *testing.T) error {
	t.Helper()

	// Create a minimal cache directory structure
	if err := os.MkdirAll(bt.cacheDir, 0o750); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create and initialize the database
	dbPath := filepath.Join(bt.cacheDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("⚠️  Warning: failed to close database: %v", closeErr)
		}
	}()

	// Create the repos table
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS repos (
			repo TEXT,
			ref TEXT,
			path TEXT,
			PRIMARY KEY (repo, ref)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create repos table: %w", err)
	}

	return nil
}

// CreateMockRepository creates a mock repository for testing
func (bt *BaseLanguageTest) CreateMockRepository(
	t *testing.T, version string, runner LanguageTestRunner,
) (string, error) {
	t.Helper()

	// Create repository path
	repoPath := filepath.Join(
		bt.cacheDir,
		"repos",
		fmt.Sprintf("test_%s_%s", runner.GetLanguageName(), version),
	)
	if err := os.MkdirAll(repoPath, 0o750); err != nil {
		return "", fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Mock successful repository cloning by creating .git directory
	gitDir := filepath.Join(repoPath, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create .git directory: %w", err)
	}

	// Setup language-specific files
	if err := runner.SetupRepositoryFiles(repoPath); err != nil {
		return "", fmt.Errorf("failed to setup repository files: %w", err)
	}

	t.Logf("    📁 Mock repository created at: %s", repoPath)

	// Verify repository was created (mock .git directory exists)
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", fmt.Errorf(
			"repository not created - .git directory missing for %s version %s",
			runner.GetLanguageName(),
			version,
		)
	}
	t.Logf("    ✅ Repository creation verified for %s version %s", runner.GetLanguageName(), version)

	return repoPath, nil
}

// TestEnvironmentSetup tests environment setup for a specific version
func (bt *BaseLanguageTest) TestEnvironmentSetup(
	t *testing.T,
	version string,
	repoPath string,
	test LanguageCompatibilityTest,
	runner LanguageTestRunner,
) error {
	t.Helper()

	lang, err := runner.GetLanguageManager()
	if err != nil {
		return fmt.Errorf("failed to get language manager: %w", err)
	}

	if !lang.NeedsEnvironmentSetup() {
		t.Logf("    ℹ️ Language %s does not need environment setup", runner.GetLanguageName())
		return nil
	}

	t.Logf("    🏗️ Testing environment setup for %s version %s", runner.GetLanguageName(), version)

	// Test environment setup
	envPath, err := lang.SetupEnvironmentWithRepo(
		bt.cacheDir, version, repoPath, test.TestRepository, test.AdditionalDependencies,
	)
	if err != nil {
		return fmt.Errorf(
			"environment setup failed for %s version %s: %w",
			runner.GetLanguageName(),
			version,
			err,
		)
	}

	// Verify environment was created
	if envPath == "" {
		return fmt.Errorf(
			"environment path is empty for %s version %s",
			runner.GetLanguageName(),
			version,
		)
	}

	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"environment directory not created for %s version %s at path: %s",
			runner.GetLanguageName(), version, envPath,
		)
	}

	// Environment created successfully

	// Run environment health check
	return bt.TestEnvironmentHealth(t, version, envPath, lang, runner, test)
}

// TestEnvironmentHealth tests the health of an environment
func (bt *BaseLanguageTest) TestEnvironmentHealth(
	t *testing.T,
	version string,
	envPath string,
	lang language.Manager,
	runner LanguageTestRunner,
	test LanguageCompatibilityTest,
) error {
	t.Helper()

	if !lang.NeedsEnvironmentSetup() {
		return nil
	}

	if err := lang.CheckHealth(envPath, version); err != nil {
		if test.NeedsRuntimeInstalled {
			// If runtime is required, health check failure should fail the test
			return fmt.Errorf(
				"environment health check failed for %s version %s (runtime required): %w",
				runner.GetLanguageName(),
				version,
				err,
			)
		} else {
			// If runtime is optional, just log a warning
			t.Logf(
				"    ⚠️ Warning: Environment health check failed for %s version %s: %v",
				runner.GetLanguageName(),
				version,
				err,
			)
		}
	} else {
		t.Logf("    ✅ Environment health check passed for %s version %s", runner.GetLanguageName(), version)
	}

	// Run additional language-specific validations
	for _, validation := range runner.GetAdditionalValidations() {
		t.Logf("    🔍 Running validation: %s", validation.Name)
		if err := validation.Execute(t, envPath, version, lang); err != nil {
			t.Logf("    ⚠️ Warning: %s failed: %v", validation.Description, err)
			// Don't fail the test for additional validations, just log them
		} else {
			t.Logf("    ✅ %s passed", validation.Description)
		}
	}

	return nil
}

// RunRepositoryAndEnvironmentSetup runs the complete repository and environment setup test
func (bt *BaseLanguageTest) RunRepositoryAndEnvironmentSetup(
	t *testing.T,
	test LanguageCompatibilityTest,
	runner LanguageTestRunner,
) error {
	t.Helper()

	t.Logf("🔍 Testing repository and environment setup for %s", runner.GetLanguageName())

	// Setup test environment
	if err := bt.SetupTestEnvironment(t); err != nil {
		return fmt.Errorf("failed to setup test environment: %w", err)
	}

	// For each test version, verify repository cloning and environment creation
	for _, version := range test.TestVersions {
		t.Logf("  🧪 Testing version: %s", version)

		// Create mock repository
		repoPath, err := bt.CreateMockRepository(t, version, runner)
		if err != nil {
			return fmt.Errorf("failed to create mock repository: %w", err)
		}

		// Test environment setup
		if err := bt.TestEnvironmentSetup(t, version, repoPath, test, runner); err != nil {
			return fmt.Errorf("failed to test environment setup: %w", err)
		}
	}

	t.Logf("✅ Repository and environment setup verification completed for %s", runner.GetLanguageName())
	return nil
}

// RunBidirectionalCacheTest runs bidirectional cache compatibility tests if the language supports it
func (bt *BaseLanguageTest) RunBidirectionalCacheTest(
	t *testing.T,
	pythonBinary, goBinary string,
	test LanguageCompatibilityTest,
	runner LanguageTestRunner,
) error {
	t.Helper()

	// Check if the runner supports bidirectional testing
	bidirectionalRunner, supportsBidirectional := runner.(BidirectionalTestRunner)
	if !supportsBidirectional {
		t.Logf("ℹ️ Language %s does not support bidirectional cache testing", runner.GetLanguageName())
		return nil
	}

	if !test.BiDirectionalTestEnabled {
		t.Logf("ℹ️ Bidirectional testing disabled for %s", runner.GetLanguageName())
		return nil
	}

	t.Logf("🔄 Running bidirectional cache compatibility test for %s", runner.GetLanguageName())

	// Setup test environment
	if err := bt.SetupTestEnvironment(t); err != nil {
		return fmt.Errorf("failed to setup test environment: %w", err)
	}

	// Run bidirectional cache test
	return bidirectionalRunner.TestBidirectionalCacheCompatibility(t, pythonBinary, goBinary, test.TestRepository)
}

// Common helper functions for language implementations

// WriteTestFile writes a test file with proper error handling and logging
func (bt *BaseLanguageTest) WriteTestFile(t *testing.T, filePath, content string) error {
	t.Helper()

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	t.Logf("    📄 Created test file: %s", filePath)
	return nil
}

// SetupGitConfig sets up basic git configuration for testing
func (bt *BaseLanguageTest) SetupGitConfig(t *testing.T, repoDir string) error {
	t.Helper()

	configs := map[string]string{
		"user.email":     "test@example.com",
		"user.name":      "Test User",
		"commit.gpgsign": "false",
	}

	for key, value := range configs {
		cmd := exec.Command("git", "config", key, value)
		cmd.Dir = repoDir
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set git config %s=%s: %w", key, value, err)
		}
	}

	t.Logf("    ⚙️ Git configuration set up for repository")
	return nil
}

// CheckExecutableExists checks if an executable exists at the given path
func (bt *BaseLanguageTest) CheckExecutableExists(t *testing.T, execPath, execName string) error {
	t.Helper()

	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		return fmt.Errorf("%s executable not found at %s", execName, execPath)
	}

	// Executable found and verified
	return nil
}

// SafeRemoveAll removes a directory with proper error handling
func (bt *BaseLanguageTest) SafeRemoveAll(t *testing.T, path string) {
	t.Helper()

	if err := os.RemoveAll(path); err != nil {
		t.Logf("    ⚠️ Warning: failed to remove directory %s: %v", path, err)
	}
}
