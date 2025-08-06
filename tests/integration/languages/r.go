package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// RLanguageTest implements LanguageTestRunner for R
type RLanguageTest struct {
	*BaseLanguageTest
}

// NewRLanguageTest creates a new R language test
func NewRLanguageTest(testDir string) *RLanguageTest {
	return &RLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangR, testDir),
	}
}

// GetLanguageName returns the language name
func (rt *RLanguageTest) GetLanguageName() string {
	return LangR
}

// SetupRepositoryFiles creates R-specific repository files
func (rt *RLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: r-syntax-check
    name: R Syntax Check
    description: Check R syntax
    entry: Rscript
    language: r
    files: \.[rR]$
    args: ['-e', 'print("R syntax OK")']
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create DESCRIPTION file (R package format)
	descFile := filepath.Join(repoPath, "DESCRIPTION")
	descContent := `Package: TestRHooks
Title: Test R Hooks for Pre-commit
Version: 0.1.0
Description: Test R hooks for pre-commit validation
Authors@R: person("Test", "User", email = "test@example.com", role = c("aut", "cre"))
License: MIT
Encoding: UTF-8
Roxygen: list(markdown = TRUE)
RoxygenNote: 7.2.0
Imports:
    styler,
    lintr
`
	if err := os.WriteFile(descFile, []byte(descContent), 0o600); err != nil {
		return fmt.Errorf("failed to create DESCRIPTION: %w", err)
	}

	// Create R directory and test script
	rDir := filepath.Join(repoPath, "R")
	if err := os.MkdirAll(rDir, 0o750); err != nil {
		return fmt.Errorf("failed to create R directory: %w", err)
	}

	rFile := filepath.Join(rDir, "test.R")
	rContent := `#' Hello R Function
#'
#' @return A greeting message
#' @export
hello_r <- function() {
  print("Hello, R!")
}
`
	if err := os.WriteFile(rFile, []byte(rContent), 0o600); err != nil {
		return fmt.Errorf("failed to create test.R: %w", err)
	}

	return nil
}

// GetLanguageManager returns the R language manager
func (rt *RLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewRLanguage(), nil
}

// GetAdditionalValidations returns R-specific validation tests
func (rt *RLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "r-version-check",
			Description: "R version validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "r" {
					return fmt.Errorf("expected r language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}
