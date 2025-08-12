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
	*BaseBidirectionalTest
}

// NewRLanguageTest creates a new R language test
func NewRLanguageTest(testDir string) *RLanguageTest {
	return &RLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangR, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(testDir),
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

// GetPreCommitConfig returns the .pre-commit-config.yaml content for R testing
func (rt *RLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-r
        name: Test R Hook
        entry: echo "Testing R"
        language: r
        files: \.R$
`
}

// GetTestFiles returns test files needed for R testing
func (rt *RLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"main.R": `# R script
cat("Hello from R!\n")

greet <- function(name) {
    cat("Hello,", name, "!\n")
}

greet("World")
`,
		"test.R": `# Test R script
source("main.R")

cat("Test completed\n")
`,
		"DESCRIPTION": `Package: TestPackage
Title: Test R Package
Version: 0.1.0
Description: A test R package for pre-commit testing.
Author: Test Author
Maintainer: Test Maintainer <test@example.com>
License: MIT
`,
	}
}

// GetExpectedDirectories returns the directories expected in R environments
func (rt *RLanguageTest) GetExpectedDirectories() []string {
	return []string{
		"R",         // R source code directory
		"man",       // R documentation
		"tests",     // R tests
		"vignettes", // R vignettes
		"renv",      // R environment management
	}
}

// GetExpectedStateFiles returns state files expected in R environments
func (rt *RLanguageTest) GetExpectedStateFiles() []string {
	return []string{
		"DESCRIPTION", // R package description
		"NAMESPACE",   // R namespace file
		"renv.lock",   // R environment lock file
		".Rprofile",   // R profile
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (rt *RLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, tempDir string,
) error {
	t.Helper()
	t.Logf("🔄 Testing R bidirectional cache compatibility")
	t.Logf("   📋 R environments manage packages and libraries - testing cache compatibility")

	// Use the base bidirectional test framework
	if err := rt.BaseBidirectionalTest.RunBidirectionalCacheTest(t, rt, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("r bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ R bidirectional cache compatibility test completed")
	return nil
}
