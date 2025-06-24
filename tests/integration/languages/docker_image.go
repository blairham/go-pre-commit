package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// DockerImageLanguageTest implements LanguageTestRunner for Docker Image
type DockerImageLanguageTest struct {
	*BaseLanguageTest
}

// NewDockerImageLanguageTest creates a new Docker Image language test
func NewDockerImageLanguageTest(testDir string) *DockerImageLanguageTest {
	return &DockerImageLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangDockerImage, testDir),
	}
}

// GetLanguageName returns the language name
func (dit *DockerImageLanguageTest) GetLanguageName() string {
	return LangDockerImage
}

// SetupRepositoryFiles creates Docker Image-specific repository files
func (dit *DockerImageLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: shellcheck
    name: Shellcheck
    description: Lint shell scripts using shellcheck in Docker
    entry: koalaman/shellcheck:stable
    language: docker_image
    files: \.sh$
-   id: yamllint
    name: YAML Lint
    description: Lint YAML files using yamllint in Docker
    entry: cytopia/yamllint
    language: docker_image
    files: \.ya?ml$
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create test shell script
	shellFile := filepath.Join(repoPath, "test.sh")
	shellContent := `#!/bin/bash
echo "Hello, Docker Image!"
`
	if err := os.WriteFile(shellFile, []byte(shellContent), 0o600); err != nil {
		return fmt.Errorf("failed to create test.sh: %w", err)
	}

	// Create test YAML file
	yamlFile := filepath.Join(repoPath, "test.yml")
	yamlContent := `test:
  message: "Hello, Docker Image!"
  items:
    - one
    - two
    - three
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0o600); err != nil {
		return fmt.Errorf("failed to create test.yml: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Docker Image language manager
func (dit *DockerImageLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewDockerImageLanguage(), nil
}

// GetAdditionalValidations returns Docker Image-specific validation tests
func (dit *DockerImageLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "docker-image-check",
			Description: "Docker Image validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "docker_image" {
					return fmt.Errorf("expected docker_image language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}
