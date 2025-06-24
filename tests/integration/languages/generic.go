package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// GenericLanguageTest implements LanguageTestRunner for unsupported/generic languages
// This provides basic test functionality for languages that don't have specific implementations
type GenericLanguageTest struct {
	*BaseLanguageTest
	languageName string
}

// NewGenericLanguageTest creates a new generic language test
func NewGenericLanguageTest(languageName, testDir string) *GenericLanguageTest {
	return &GenericLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(languageName, testDir),
		languageName:     languageName,
	}
}

// GetLanguageName returns the language name
func (gt *GenericLanguageTest) GetLanguageName() string {
	return gt.languageName
}

// SetupRepositoryFiles creates basic repository files for generic languages
func (gt *GenericLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create a basic pre-commit-hooks.yaml file
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	content := fmt.Sprintf(`-   id: test-%s-hook
    name: Test %s Hook
    description: Test hook for %s language
    entry: echo "Testing %s language"
    language: %s
    files: \\.txt$
`, gt.languageName, gt.languageName, gt.languageName, gt.languageName, gt.languageName)

	if err := os.WriteFile(hooksFile, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create a simple test file
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content\n"), 0o600); err != nil {
		return fmt.Errorf("failed to create test file: %w", err)
	}

	return nil
}

// GetLanguageManager returns a mock language manager for generic languages
func (gt *GenericLanguageTest) GetLanguageManager() (language.Manager, error) {
	// For generic/unsupported languages, we'll create a mock language manager
	// that simulates the behavior without actually setting up environments
	return &MockLanguageManager{
		languageName: gt.languageName,
	}, nil
}

// GetAdditionalValidations returns language-specific validation tests
func (gt *GenericLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "generic-language-check",
			Description: fmt.Sprintf("%s language basic validation", gt.languageName),
			Execute: func(_ *testing.T, _, _ string, _ language.Manager) error {
				// Basic validation - just check that we can identify the language
				if gt.languageName == "" {
					return fmt.Errorf("language name is empty")
				}
				return nil
			},
		},
	}
}

// MockLanguageManager provides a mock implementation for unsupported languages
type MockLanguageManager struct {
	languageName string
}

// GetName returns the language name
func (mlm *MockLanguageManager) GetName() string {
	return mlm.languageName
}

// GetExecutableName returns a mock executable name
func (mlm *MockLanguageManager) GetExecutableName() string {
	return mlm.languageName
}

// IsRuntimeAvailable always returns true for mock languages
func (mlm *MockLanguageManager) IsRuntimeAvailable() bool {
	return true
}

// NeedsEnvironmentSetup returns false for mock languages (no actual setup needed)
func (mlm *MockLanguageManager) NeedsEnvironmentSetup() bool {
	return false
}

// SetupEnvironment returns a mock environment path
func (mlm *MockLanguageManager) SetupEnvironment(cacheDir, _ string, _ []string) (string, error) {
	return filepath.Join(cacheDir, fmt.Sprintf("mock_%s_env", mlm.languageName)), nil
}

// SetupEnvironmentWithRepo returns a mock environment path
func (mlm *MockLanguageManager) SetupEnvironmentWithRepo(
	_, _, repoPath, _ string, _ []string,
) (string, error) {
	// Return a mock environment path
	return filepath.Join(repoPath, fmt.Sprintf("mock_%s_env", mlm.languageName)), nil
}

// SetupEnvironmentWithRepoInfo is a simplified setup for mock languages
func (mlm *MockLanguageManager) SetupEnvironmentWithRepoInfo(
	_, _, repoPath, _ string, _ []string,
) (string, error) {
	return mlm.SetupEnvironmentWithRepo("", "", repoPath, "", nil)
}

// PreInitializeEnvironmentWithRepoInfo is a no-op for mock languages
func (mlm *MockLanguageManager) PreInitializeEnvironmentWithRepoInfo(
	_, _, _, _ string, _ []string,
) error {
	return nil
}

// GetEnvironmentBinPath returns a mock bin path
func (mlm *MockLanguageManager) GetEnvironmentBinPath(envPath string) string {
	return filepath.Join(envPath, "bin")
}

// CheckEnvironmentHealth always returns true for mock languages
func (mlm *MockLanguageManager) CheckEnvironmentHealth(_ string) bool {
	return true
}

// CheckHealth always returns nil for mock languages
func (mlm *MockLanguageManager) CheckHealth(_, _ string) error {
	return nil
}

// InstallDependencies is a no-op for mock languages
func (mlm *MockLanguageManager) InstallDependencies(_ string, _ []string) error {
	return nil
}
