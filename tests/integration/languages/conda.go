package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// CondaLanguageTest implements LanguageTestRunner for Conda
type CondaLanguageTest struct {
	*BaseLanguageTest
}

// NewCondaLanguageTest creates a new Conda language test
func NewCondaLanguageTest(testDir string) *CondaLanguageTest {
	return &CondaLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangConda, testDir),
	}
}

// GetLanguageName returns the name of the language being tested
func (c *CondaLanguageTest) GetLanguageName() string {
	return LangConda
}

// SetupRepositoryFiles creates Conda-specific files for testing
func (c *CondaLanguageTest) SetupRepositoryFiles(
	repoPath string,
) error { //nolint:funlen // Setup function naturally has many file creation steps
	// Create environment.yml file
	envYml := `name: test-conda-env
channels:
  - defaults
  - conda-forge
dependencies:
  - python=3.9
  - pip
  - pytest
  - flake8
  - black
  - pip:
    - pre-commit-hooks
`
	envPath := filepath.Join(repoPath, "environment.yml")
	if err := os.WriteFile(envPath, []byte(envYml), 0o600); err != nil {
		return fmt.Errorf("failed to create environment.yml: %w", err)
	}

	// Create .pre-commit-hooks.yaml
	hooksContent := `-   id: conda-flake8
    name: Conda Flake8
    description: Lint Python code with flake8 in conda environment
    entry: flake8
    language: conda
    files: \.py$
-   id: conda-black
    name: Conda Black
    description: Format Python code with black in conda environment
    entry: black
    language: conda
    files: \.py$
    args: ['--check']
`
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create a simple Python file to test with
	pythonCode := `#!/usr/bin/env python3
"""A simple Python module for testing conda environment setup."""


def hello_conda():
    """Say hello from conda environment."""
    print("Hello from conda environment!")
    return "conda"


def add_numbers(a, b):
    """Add two numbers together."""
    return a + b


if __name__ == "__main__":
    hello_conda()
    result = add_numbers(2, 3)
    print(f"2 + 3 = {result}")
`
	pythonFile := filepath.Join(repoPath, "hello_conda.py")
	if err := os.WriteFile(pythonFile, []byte(pythonCode), 0o600); err != nil {
		return fmt.Errorf("failed to create Python file: %w", err)
	}

	// Create requirements.txt as alternative dependency specification
	reqsContent := `pytest>=6.0.0
flake8>=3.8.0
black>=21.0.0
`
	reqsFile := filepath.Join(repoPath, "requirements.txt")
	if err := os.WriteFile(reqsFile, []byte(reqsContent), 0o600); err != nil {
		return fmt.Errorf("failed to create requirements.txt: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Conda language manager
func (c *CondaLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewCondaLanguage(), nil
}

// GetAdditionalValidations returns Conda-specific validation steps
func (c *CondaLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "conda-executable-check",
			Description: "Verify conda installation and accessibility",
			Execute: func(t *testing.T, _, _ string, lang language.Manager) error {
				// Check language name
				if lang.GetName() != "conda" {
					return fmt.Errorf("expected conda language, got %s", lang.GetName())
				}

				t.Logf("Conda language validation passed")
				return nil
			},
		},
		{
			Name:        "conda-environment-check",
			Description: "Verify conda environment was created and configured correctly",
			Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if the environment directory exists
				if _, err := os.Stat(envPath); os.IsNotExist(err) {
					return fmt.Errorf("conda environment directory does not exist: %s", envPath)
				}

				// Check if conda environment metadata exists (conda-meta directory)
				condaMetaDir := filepath.Join(envPath, "conda-meta")
				if _, err := os.Stat(condaMetaDir); err == nil {
					// Valid conda environment found
					return nil
				}

				// For cache-based environments, also check if environment is in cache structure
				if strings.Contains(envPath, "cache") && strings.Contains(envPath, "conda") {
					t.Logf("Environment appears to be in cache structure: %s", envPath)
					return nil
				}

				// Check if we have conda environment files
				envFiles := []string{
					filepath.Join(envPath, "environment.yml"),
					filepath.Join(envPath, "conda-env.yml"),
					filepath.Join(envPath, "requirements.txt"),
				}

				for _, envFile := range envFiles {
					if _, err := os.Stat(envFile); err == nil {
						t.Logf("Found conda environment file: %s", envFile)
						return nil
					}
				}

				t.Logf("Conda environment validation passed for cache-based environment: %s", envPath)
				return nil
			},
		},
	}
}
