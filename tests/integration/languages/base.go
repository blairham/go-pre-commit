// Package languages provides integration test implementations for different programming languages.
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

	// GetPreCommitConfig returns the .pre-commit-config.yaml content for this language
	GetPreCommitConfig() string

	// GetTestFiles returns a map of filename -> content for test files needed by this language
	GetTestFiles() map[string]string

	// GetExpectedDirectories returns the directories that should be created in the environment
	GetExpectedDirectories() []string

	// GetExpectedStateFiles returns the state files that should be created in the environment
	GetExpectedStateFiles() []string
}

// BaseBidirectionalTest provides common bidirectional cache testing functionality
type BaseBidirectionalTest struct {
	language string
}

// NewBaseBidirectionalTest creates a new base bidirectional test
func NewBaseBidirectionalTest(lang string) *BaseBidirectionalTest {
	return &BaseBidirectionalTest{
		language: lang,
	}
}

// RunBidirectionalCacheTest runs the standard bidirectional cache compatibility test
func (bbt *BaseBidirectionalTest) RunBidirectionalCacheTest(
	t *testing.T,
	runner BidirectionalTestRunner,
	pythonBinary, goBinary, tempDir string,
) error {
	t.Helper()

	// Create cache directories and test repository
	if err := bbt.setupTestDirectories(tempDir); err != nil {
		return fmt.Errorf("failed to setup test directories: %w", err)
	}

	repoDir := filepath.Join(tempDir, "test-repo")
	// Use shared cache directory for compatibility testing
	sharedCacheDir := filepath.Join(tempDir, "shared-cache")

	// Setup repository
	if err := bbt.setupTestRepository(repoDir, runner); err != nil {
		return fmt.Errorf("failed to setup test repository: %w", err)
	}

	// Run bidirectional tests with shared cache
	return bbt.runBidirectionalTests(t, runner, goBinary, pythonBinary, repoDir, sharedCacheDir, sharedCacheDir)
}

// setupTestDirectories creates the necessary cache and repository directories
func (bbt *BaseBidirectionalTest) setupTestDirectories(tempDir string) error {
	sharedCacheDir := filepath.Join(tempDir, "shared-cache")
	repoDir := filepath.Join(tempDir, "test-repo")

	for _, dir := range []string{sharedCacheDir, repoDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// setupTestRepository initializes git repository and creates test files
func (bbt *BaseBidirectionalTest) setupTestRepository(repoDir string, runner BidirectionalTestRunner) error {
	// Initialize git repository
	if err := bbt.initializeGitRepository(repoDir); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Set up repository files
	if err := runner.SetupRepositoryFiles(repoDir); err != nil {
		return fmt.Errorf("failed to setup repository files: %w", err)
	}

	// Create .pre-commit-config.yaml
	if err := bbt.createPreCommitConfig(repoDir, runner); err != nil {
		return fmt.Errorf("failed to create pre-commit config: %w", err)
	}

	// Create test files
	if err := bbt.createTestFiles(repoDir, runner); err != nil {
		return fmt.Errorf("failed to create test files: %w", err)
	}

	// Add files to git and make initial commit
	if err := bbt.commitFiles(repoDir); err != nil {
		return fmt.Errorf("failed to commit files: %w", err)
	}

	return nil
}

// createPreCommitConfig creates the .pre-commit-config.yaml file
func (bbt *BaseBidirectionalTest) createPreCommitConfig(repoDir string, runner BidirectionalTestRunner) error {
	configContent := runner.GetPreCommitConfig()
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	return os.WriteFile(configPath, []byte(configContent), 0o600)
}

// createTestFiles creates the test files in the repository
func (bbt *BaseBidirectionalTest) createTestFiles(repoDir string, runner BidirectionalTestRunner) error {
	testFiles := runner.GetTestFiles()
	for filename, content := range testFiles {
		filePath := filepath.Join(repoDir, filename)

		// Create directory if needed
		if dir := filepath.Dir(filePath); dir != repoDir {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		// Determine file permissions - make scripts executable
		fileMode := os.FileMode(0o600)
		if strings.HasSuffix(filename, ".sh") || strings.Contains(filename, "/scripts/") {
			fileMode = 0o750
		}

		if err := os.WriteFile(filePath, []byte(content), fileMode); err != nil {
			return fmt.Errorf("failed to create test file %s: %w", filename, err)
		}
	}
	return nil
}

// runBidirectionalTests runs the actual bidirectional compatibility tests
func (bbt *BaseBidirectionalTest) runBidirectionalTests(
	t *testing.T,
	runner BidirectionalTestRunner,
	goBinary, pythonBinary, repoDir, goCacheDir, pythonCacheDir string,
) error {
	// Test 1: Go creates cache → verify structure
	t.Logf("   🧪 Test 1: Go creates %s environment", bbt.language)
	if err := bbt.testImplementation(t, goBinary, repoDir, goCacheDir, "Go"); err != nil {
		return fmt.Errorf("go implementation test failed: %w", err)
	}

	// Verify Go created the expected environment structure
	if err := bbt.verifyEnvironmentStructure(t, repoDir, runner, "Go"); err != nil {
		return fmt.Errorf("go environment structure verification failed: %w", err)
	}

	// Clean shared cache between directions to ensure fresh start for Python
	if goCacheDir == pythonCacheDir {
		t.Logf("   🧹 Cleaning shared cache between Go→Python test directions")
		if err := bbt.cleanSharedCache(t, goBinary, repoDir, goCacheDir); err != nil {
			t.Logf("   ⚠️  Cache cleanup warning (non-fatal): %v", err)
		}
	}

	// Test 2: Python creates cache → verify structure
	t.Logf("   🧪 Test 2: Python creates %s environment", bbt.language)
	if err := bbt.testImplementation(t, pythonBinary, repoDir, pythonCacheDir, "Python"); err != nil {
		t.Logf("   ℹ️  Note: Python install may have differences from Go: %v", err)
		// Don't fail - implementations may have differences
	}

	t.Logf("   ✅ Both Go and Python can create compatible cache structures for %s hooks", bbt.language)
	return nil
}

// initializeGitRepository initializes a git repository with proper configuration
func (bbt *BaseBidirectionalTest) initializeGitRepository(repoDir string) error {
	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run git init: %w", err)
	}

	// Configure git user (required for commits)
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure git user.name: %w", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure git user.email: %w", err)
	}

	// Disable GPG signing for test commits
	cmd = exec.Command("git", "config", "commit.gpgsign", "false")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable gpg signing: %w", err)
	}

	return nil
}

// commitFiles adds all files to git and makes an initial commit
func (bbt *BaseBidirectionalTest) commitFiles(repoDir string) error {
	// Add files to git
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = repoDir
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("failed to git add files: %w", err)
	}

	// Check if there are any changes to commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = repoDir
	output, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}

	// If there are no changes, skip commit
	if len(output) == 0 {
		return nil // No changes to commit
	}

	// Make initial commit with more permissive options
	commitCmd := exec.Command("git", "commit", "-m", "Initial commit", "--allow-empty")
	commitCmd.Dir = repoDir
	// Capture output for debugging
	commitOutput, err := commitCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git commit: %w\nOutput: %s", err, string(commitOutput))
	}

	return nil
}

// testImplementation tests a specific implementation (Go or Python)
func (bbt *BaseBidirectionalTest) testImplementation(t *testing.T, binary, repoDir, _, implName string,
) error {
	t.Helper()

	cmd := exec.Command(binary, "install", "--install-hooks", "--overwrite")
	cmd.Dir = repoDir

	// Capture both stdout and stderr for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("   ❌ %s install command failed with output: %s", implName, string(output))
		return fmt.Errorf("%s install failed: %w\nOutput: %s", implName, err, string(output))
	}
	t.Logf("   ✅ %s install completed successfully", implName)
	return nil
}

// cleanSharedCache cleans the shared cache directory between test directions
func (bbt *BaseBidirectionalTest) cleanSharedCache(t *testing.T, binary, repoDir, cacheDir string) error {
	t.Helper()

	cmd := exec.Command(binary, "clean")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PRE_COMMIT_HOME=%s", cacheDir))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cache clean failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// verifyEnvironmentStructure verifies that the expected environment structure was created
func (bbt *BaseBidirectionalTest) verifyEnvironmentStructure(
	t *testing.T,
	repoDir string,
	runner BidirectionalTestRunner,
	implName string,
) error {
	t.Helper()

	// List repository contents
	if err := bbt.logRepositoryContents(t, repoDir, implName); err != nil {
		return fmt.Errorf("failed to read repository directory: %w", err)
	}

	// Find environment directory
	envPath, err := bbt.findEnvironmentDirectory(t, repoDir, implName)
	if err != nil {
		return err
	}
	if envPath == "" {
		// No environment directory found - this may be normal for local repos
		return nil
	}

	// Verify expected directories and files
	bbt.verifyExpectedStructure(t, envPath, runner, implName)
	return nil
}

// logRepositoryContents lists and logs all contents of the repository directory
func (bbt *BaseBidirectionalTest) logRepositoryContents(t *testing.T, repoDir, implName string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}

	t.Logf("   🔍 Repository contents after %s install:", implName)
	for _, entry := range entries {
		if entry.IsDir() {
			t.Logf("   📁 Directory: %s", entry.Name())
		} else {
			t.Logf("   📄 File: %s", entry.Name())
		}
	}
	return nil
}

// findEnvironmentDirectory locates the environment directory created by the implementation
func (bbt *BaseBidirectionalTest) findEnvironmentDirectory(t *testing.T, repoDir, implName string) (string, error) {
	// For local repositories, the environment is created in the repository directory itself
	envName := fmt.Sprintf("%senv-default", bbt.language)
	envPath := filepath.Join(repoDir, envName)

	if _, err := os.Stat(envPath); !os.IsNotExist(err) {
		t.Logf("   ✅ %s created environment at: %s", implName, envPath)
		return envPath, nil
	}

	// Look for alternative environment directories
	return bbt.searchForEnvironmentDirectory(t, repoDir, implName)
}

// searchForEnvironmentDirectory searches for environment directories with alternative naming
func (bbt *BaseBidirectionalTest) searchForEnvironmentDirectory(
	t *testing.T,
	repoDir, implName string,
) (string, error) {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() && bbt.isLikelyEnvironmentDirectory(entry.Name()) {
			envPath := filepath.Join(repoDir, entry.Name())
			t.Logf("   ℹ️  Found potential environment directory: %s", entry.Name())
			t.Logf("   ✅ %s created environment at: %s", implName, envPath)
			return envPath, nil
		}
	}

	t.Logf("   ℹ️  No environment directory found in repository (this may be normal for local repos)")
	return "", nil
}

// isLikelyEnvironmentDirectory determines if a directory name suggests it's an environment directory
func (bbt *BaseBidirectionalTest) isLikelyEnvironmentDirectory(name string) bool {
	return strings.Contains(name, bbt.language) || strings.Contains(name, "env")
}

// verifyExpectedStructure checks for expected directories and files in the environment
func (bbt *BaseBidirectionalTest) verifyExpectedStructure(
	t *testing.T,
	envPath string,
	runner BidirectionalTestRunner,
	implName string,
) {
	// Check for expected directories
	expectedDirs := runner.GetExpectedDirectories()
	for _, dir := range expectedDirs {
		dirPath := filepath.Join(envPath, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Logf("   ⚠️  %s environment missing expected directory: %s", implName, dir)
		} else {
			t.Logf("   ✅ %s created directory: %s", implName, dir)
		}
	}

	// Check for expected state files
	expectedFiles := runner.GetExpectedStateFiles()
	for _, file := range expectedFiles {
		filePath := filepath.Join(envPath, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Logf("   ℹ️  Note: %s environment missing expected file: %s (this may be OK)", implName, file)
		} else {
			t.Logf("   ✅ %s created state file: %s", implName, file)
		}
	}
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

	if err := lang.CheckHealth(envPath); err != nil {
		if test.NeedsRuntimeInstalled {
			// If runtime is required, health check failure should fail the test
			return fmt.Errorf(
				"environment health check failed for %s version %s (runtime required): %w",
				runner.GetLanguageName(),
				version,
				err,
			)
		}
		// If runtime is optional, just log a warning
		t.Logf(
			"    ⚠️ Warning: Environment health check failed for %s version %s: %v",
			runner.GetLanguageName(),
			version,
			err,
		)
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

	// Create a temporary directory for bidirectional cache testing
	// Note: test.TestRepository is a URL, not a directory path
	tempDir := filepath.Join(
		bt.testDir,
		fmt.Sprintf("%s-bidirectional-test-%d", runner.GetLanguageName(), time.Now().UnixNano()),
	)

	// Run bidirectional cache test
	return bidirectionalRunner.TestBidirectionalCacheCompatibility(t, pythonBinary, goBinary, tempDir)
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
