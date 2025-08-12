package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// DockerLanguageTest implements LanguageTestRunner and BidirectionalTestRunner for Docker
type DockerLanguageTest struct {
	*BaseLanguageTest
	*BaseBidirectionalTest
}

// NewDockerLanguageTest creates a new Docker language test
func NewDockerLanguageTest(testDir string) *DockerLanguageTest {
	return &DockerLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangDocker, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(LangDocker),
	}
}

// GetLanguageName returns the language name
func (dt *DockerLanguageTest) GetLanguageName() string {
	return LangDocker
}

// SetupRepositoryFiles creates Docker-specific repository files
func (dt *DockerLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: dockerfile-lint
    name: Dockerfile Lint
    description: Lint Dockerfile using hadolint
    entry: hadolint
    language: docker
    files: Dockerfile.*
-   id: docker-compose-check
    name: Docker Compose Check
    description: Validate docker-compose files
    entry: docker-compose config
    language: docker
    files: docker-compose.*\.ya?ml$
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create Dockerfile
	dockerFile := filepath.Join(repoPath, "Dockerfile")
	dockerContent := `FROM alpine:latest

RUN apk add --no-cache bash

WORKDIR /app

COPY . .

CMD ["echo", "Hello, Docker!"]
`
	if err := os.WriteFile(dockerFile, []byte(dockerContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}

	// Create docker-compose.yml
	composeFile := filepath.Join(repoPath, "docker-compose.yml")
	composeContent := `version: '3.8'

services:
  app:
    build: .
    container_name: test-docker-hooks
    command: echo "Hello, Docker Compose!"
`
	if err := os.WriteFile(composeFile, []byte(composeContent), 0o600); err != nil {
		return fmt.Errorf("failed to create docker-compose.yml: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Docker language manager
func (dt *DockerLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewDockerLanguage(), nil
}

// GetAdditionalValidations returns Docker-specific validation tests
func (dt *DockerLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "docker-version-check",
			Description: "Docker version validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "docker" {
					return fmt.Errorf("expected docker language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}

// GetPreCommitConfig returns the .pre-commit-config.yaml content for Docker testing
func (dt *DockerLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-docker
        name: Test Docker Hook
        entry: echo "Testing Docker"
        language: docker
        files: \.sh$
`
}

// GetTestFiles returns test files needed for Docker testing
func (dt *DockerLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"test.sh": `#!/bin/bash
echo "Hello from Docker!"
`,
	}
}

// GetExpectedDirectories returns the directories expected in Docker environments
func (dt *DockerLanguageTest) GetExpectedDirectories() []string {
	return []string{
		"docker",     // Docker context
		"cache",      // Docker cache
		"containers", // Container storage
	}
}

// GetExpectedStateFiles returns state files expected in Docker environments
func (dt *DockerLanguageTest) GetExpectedStateFiles() []string {
	return []string{
		"Dockerfile",         // Docker image definition
		".dockerignore",      // Docker ignore file
		"docker-compose.yml", // Docker compose file
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (dt *DockerLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, tempDir string,
) error {
	t.Helper()
	t.Logf("🔄 Testing Docker bidirectional cache compatibility")
	t.Logf("   📋 Docker environments manage container images - testing cache compatibility")

	// Use the base bidirectional test framework
	if err := dt.BaseBidirectionalTest.RunBidirectionalCacheTest(t, dt, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("docker bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ Docker bidirectional cache compatibility test completed")
	return nil
}
