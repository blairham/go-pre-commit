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

// SystemLanguageTest implements LanguageTestRunner and BidirectionalTestRunner for System
type SystemLanguageTest struct {
	*GenericLanguageTest
	*BaseBidirectionalTest
}

// NewSystemLanguageTest creates a new System language test
func NewSystemLanguageTest(testDir string) *SystemLanguageTest {
	return &SystemLanguageTest{
		GenericLanguageTest:   NewGenericLanguageTest(LangSystem, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(LangSystem),
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

// GetAdditionalValidations returns System-specific validation steps
func (st *SystemLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "system-commands-check",
			Description: "Verify system commands are available",
			Execute: func(_ *testing.T, _, _ string, _ language.Manager) error {
				// Check that system has basic commands available
				if _, err := exec.LookPath("bash"); err != nil {
					return fmt.Errorf("bash not found in PATH: %w", err)
				}
				return nil
			},
		},
	}
}

// GetPreCommitConfig returns the .pre-commit-config.yaml content for System
func (st *SystemLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-system
        name: Test System Hook
        entry: echo "Testing System"
        language: system
        files: \.txt$
`
}

// GetTestFiles returns test files needed for System testing
func (st *SystemLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"test.txt": "This is a test file for system hooks.",
	}
}

// GetExpectedDirectories returns directories expected in System environments
func (st *SystemLanguageTest) GetExpectedDirectories() []string {
	// System language doesn't create environment directories
	return []string{}
}

// GetExpectedStateFiles returns state files expected in System environments
func (st *SystemLanguageTest) GetExpectedStateFiles() []string {
	// System language doesn't create state files
	return []string{}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (st *SystemLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, _ string,
) error {
	t.Helper()

	t.Logf("🔄 Testing System language bidirectional cache compatibility")
	t.Logf("   📋 System hooks use native commands - testing cache compatibility")

	// Create a temporary directory for this test
	tempDir, err := os.MkdirTemp("", "system-bidirectional-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("🧹 Cleanup: failed to remove temp directory: %v", removeErr)
		}
	}()

	// Use the base bidirectional test implementation
	if err := st.RunBidirectionalCacheTest(t, st, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("bidirectional cache test failed: %w", err)
	}

	t.Logf("   ✅ Both Go and Python can create compatible cache structures for system hooks")
	t.Logf("✅ System language bidirectional cache compatibility test completed")
	return nil
}
