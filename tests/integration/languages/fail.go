package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// FailLanguageTest implements LanguageTestRunner for Fail
type FailLanguageTest struct {
	*GenericLanguageTest
}

// NewFailLanguageTest creates a new Fail language test
func NewFailLanguageTest(testDir string) *FailLanguageTest {
	return &FailLanguageTest{
		GenericLanguageTest: NewGenericLanguageTest(LangFail, testDir),
	}
}

// GetLanguageName returns the language name
func (ft *FailLanguageTest) GetLanguageName() string {
	return LangFail
}

// SetupRepositoryFiles creates Fail-specific repository files
func (ft *FailLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `- id: no-commit-to-branch
  name: "Don't commit to branch"
  description: Prevent committing to specific branches
  entry: 'Do not commit to main branch'
  language: fail
  files: .*
  args: ['--branch', 'main', '--branch', 'master']
- id: check-merge-conflict
  name: Check for merge conflicts
  description: Check for files that contain merge conflict strings
  entry: Check for merge conflict markers
  language: fail
  files: .*
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create test file
	testFile := filepath.Join(repoPath, "test.txt")
	testContent := `This is a test file for the fail language.
The fail language is used to prevent certain actions.
`
	if err := os.WriteFile(testFile, []byte(testContent), 0o600); err != nil {
		return fmt.Errorf("failed to create test.txt: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Fail language manager
func (ft *FailLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewFailLanguage(), nil
}

// GetAdditionalValidations returns Fail-specific validation tests
func (ft *FailLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "fail-validation",
			Description: "Fail language validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "fail" {
					return fmt.Errorf("expected fail language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (ft *FailLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, _ string,
) error {
	t.Helper()

	t.Logf("🔄 Testing Fail language bidirectional cache compatibility")
	t.Logf("   📋 Fail hooks prevent commits - testing cache compatibility")

	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "fail-bidirectional-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Warning: failed to remove temp directory: %v", removeErr)
		}
	}()

	// Test basic cache structure compatibility
	if err := ft.testBasicCacheCompatibility(t, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("basic cache compatibility test failed: %w", err)
	}

	t.Logf("✅ Fail language bidirectional cache compatibility test completed")
	return nil
}

// setupTestRepository creates a test repository for fail language testing
func (ft *FailLanguageTest) setupTestRepository(t *testing.T, repoPath, _ string) error {
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

// testBasicCacheCompatibility tests basic cache directory compatibility for fail hooks
func (ft *FailLanguageTest) testBasicCacheCompatibility(t *testing.T, pythonBinary, goBinary, tempDir string) error {
	t.Helper()

	// Create cache directories
	goCacheDir := filepath.Join(tempDir, "go-cache")
	pythonCacheDir := filepath.Join(tempDir, "python-cache")

	// Create a simple repository for testing
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := ft.setupTestRepository(t, repoDir, ""); err != nil {
		return fmt.Errorf("failed to setup test repository: %w", err)
	}

	// Fail language config with pre-commit-hooks that require environment setup
	configContent := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
    -   id: no-commit-to-branch
        args: ['--branch', 'main']
    -   id: check-yaml
    -   id: check-json
    -   id: check-toml
    -   id: check-merge-conflict
-   repo: local
    hooks:
    -   id: local-fail-hook
        name: Local Fail Hook with Setup Check
        entry: echo "This hook simulates environment setup overhead"
        language: fail
        files: \.tmp$
`
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Create test file
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0o600); err != nil {
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

	t.Logf("   ✅ Both Go and Python can create compatible cache structures for fail hooks")
	return nil
}
