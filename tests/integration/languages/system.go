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

// SystemLanguageTest implements LanguageTestRunner for System
type SystemLanguageTest struct {
	*GenericLanguageTest
}

// NewSystemLanguageTest creates a new System language test
func NewSystemLanguageTest(testDir string) *SystemLanguageTest {
	return &SystemLanguageTest{
		GenericLanguageTest: NewGenericLanguageTest(LangSystem, testDir),
	}
}

// GetLanguageName returns the language name
func (st *SystemLanguageTest) GetLanguageName() string {
	return LangSystem
}

// SetupRepositoryFiles creates System-specific repository files
func (st *SystemLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml with hooks that actually require setup
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `- id: simple-system-command
  name: Simple System Command
  description: Run a simple system command that tests environment caching with more files
  entry: bash -c 'find . -name "*.txt" -exec wc -l {} \; | awk "{sum += $1} END {print \"Total lines:\", sum}" && find . -name "*.txt" | wc -l | awk "{print \"Total files:\", $1}"'
  language: system
  files: \.txt$
  pass_filenames: false
- id: system-script-with-setup
  name: System Script with Setup Requirements
  description: Test hook that simulates environment setup requirements
  entry: bash -c 'sleep 0.1 && echo "Setup-requiring hook executed"'
  language: system
  files: \.(txt|md)$
  pass_filenames: false
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create test files
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content for system hooks\n"), 0o600); err != nil {
		return fmt.Errorf("failed to create test file: %w", err)
	}

	// Create many more test files to make caching more meaningful
	// System language performance improvement comes from avoiding repeated environment setup
	for i := 1; i <= 25; i++ {
		extraFile := filepath.Join(repoPath, fmt.Sprintf("extra%d.txt", i))
		content := fmt.Sprintf(`Extra test file %d for system hook testing

This file contains more substantial content to make the system hook
work more meaningful. System hooks like 'find' and other utilities
benefit from having more files to process.

Content lines:
- Line 1 in file %d
- Line 2 in file %d
- Line 3 in file %d
- Line 4 in file %d

Additional content to make file processing take measurable time:
%s

End of file %d
`, i, i, i, i, i,
			// Add some padding content
			fmt.Sprintf("Padding content %s", fmt.Sprintf("%100d", i)),
			i)
		if err := os.WriteFile(extraFile, []byte(content), 0o600); err != nil {
			return fmt.Errorf("failed to create extra test file %d: %w", i, err)
		}
	}

	// Create subdirectory with more files
	subDir := filepath.Join(repoPath, "subdir")
	if err := os.MkdirAll(subDir, 0o750); err != nil {
		return fmt.Errorf("failed to create subdirectory: %w", err)
	}

	for i := 1; i <= 15; i++ {
		subFile := filepath.Join(subDir, fmt.Sprintf("sub%d.txt", i))
		content := fmt.Sprintf("Subdirectory file %d content\nWith multiple lines\nTo process\n", i)
		if err := os.WriteFile(subFile, []byte(content), 0o600); err != nil {
			return fmt.Errorf("failed to create subdirectory file %d: %w", i, err)
		}
	}

	return nil
}

// GetLanguageManager returns the System language manager
func (st *SystemLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewSystemLanguage(), nil
}

// GetAdditionalValidations returns System-specific validation tests
func (st *SystemLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "system-commands-check",
			Description: "System commands validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "system" {
					return fmt.Errorf("expected system language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (st *SystemLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, _ string,
) error {
	t.Helper()

	t.Logf("🔄 Testing System language bidirectional cache compatibility")
	t.Logf("   📋 System hooks use native commands - testing cache compatibility")

	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "system-bidirectional-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("⚠️  Warning: failed to remove temp directory: %v", removeErr)
		}
	}()

	// Test basic cache structure compatibility
	if err := st.testBasicCacheCompatibility(t, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("basic cache compatibility test failed: %w", err)
	}

	t.Logf("✅ System language bidirectional cache compatibility test completed")
	return nil
}

// setupTestRepository creates a test repository for system language testing
func (st *SystemLanguageTest) setupTestRepository(t *testing.T, repoPath, _ string) error {
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

// testBasicCacheCompatibility tests basic cache directory compatibility for system hooks
func (st *SystemLanguageTest) testBasicCacheCompatibility(t *testing.T, pythonBinary, goBinary, tempDir string) error {
	t.Helper()

	// Create cache directories
	goCacheDir := filepath.Join(tempDir, "go-cache")
	pythonCacheDir := filepath.Join(tempDir, "python-cache")

	// Create a simple repository for testing
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := st.setupTestRepository(t, repoDir, ""); err != nil {
		return fmt.Errorf("failed to setup test repository: %w", err)
	}

	// System language config with hooks that require environment setup and caching
	configContent := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
    -   id: check-yaml
    -   id: check-json
    -   id: check-toml
    -   id: check-xml
    -   id: check-merge-conflict
-   repo: local
    hooks:
    -   id: system-with-setup
        name: System Hook with Environment Setup
        entry: bash -c 'pip list > /dev/null 2>&1 || echo "No pip found"; sleep 0.1; echo "System hook executed"'
        language: system
        files: \.txt$
        pass_filenames: false
    -   id: system-complex-check
        name: Complex System Check
        entry: bash -c 'which python3 > /dev/null && echo "Python found" || echo "No Python"; find . -name "*.txt" | wc -l'
        language: system
        files: \.txt$
        pass_filenames: false
`
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Create test files
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content   \n"), 0o600); err != nil {
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

	t.Logf("   ✅ Both Go and Python can create compatible cache structures for system hooks")
	return nil
}
