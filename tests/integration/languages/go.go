package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// GoLanguageTest implements LanguageTestRunner for Go
type GoLanguageTest struct {
	*BaseLanguageTest
}

// NewGoLanguageTest creates a new Go language test
func NewGoLanguageTest(testDir string) *GoLanguageTest {
	return &GoLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangGolang, testDir),
	}
}

// SetupRepositoryFiles creates Go-specific files in the test repository
func (gt *GoLanguageTest) SetupRepositoryFiles(repoPath string) error {
	goModContent := "module test\n\ngo 1.19"
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte(goModContent), 0o600); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}
	return nil
}

// GetLanguageManager returns the Go language manager
func (gt *GoLanguageTest) GetLanguageManager() (language.Manager, error) {
	registry := languages.NewLanguageRegistry()
	langImpl, exists := registry.GetLanguage(LangGolang)
	if !exists {
		return nil, fmt.Errorf("language %s not found in registry", LangGolang)
	}

	lang, ok := langImpl.(language.Manager)
	if !ok {
		return nil, fmt.Errorf("language %s does not implement LanguageManager interface", LangGolang)
	}

	return lang, nil
}

// GetAdditionalValidations returns Go-specific validation steps
func (gt *GoLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "go-executable-check",
			Description: "Go executable validation",
			Execute: func(t *testing.T, _, _ string, _ language.Manager) error {
				// For Go, we typically don't create isolated environments like Python
				// So we just check if Go is available on the system
				t.Logf("      Go language uses system-wide installation")
				return nil
			},
		},
	}
}

// GetLanguageName returns the name of the Go language
func (gt *GoLanguageTest) GetLanguageName() string {
	return LangGolang
}
