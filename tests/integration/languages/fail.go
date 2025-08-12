package languages

import (
	"fmt"
	"os"
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
