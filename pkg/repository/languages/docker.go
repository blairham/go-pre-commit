package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// DockerLanguage handles Docker-based environments
type DockerLanguage struct {
	*language.Base
}

// NewDockerLanguage creates a new Docker language handler
func NewDockerLanguage() *DockerLanguage {
	return &DockerLanguage{
		Base: language.NewBase(
			"Docker",
			"docker",
			"--version",
			"https://docs.docker.com/get-docker/",
		),
	}
}

// SetupEnvironmentWithRepo sets up a Docker environment for a specific repository
func (d *DockerLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	_ []string,
) (string, error) {
	// Docker language doesn't need complex environment setup
	// Use Python-compatible structure: environments within repository directories
	envDirName := language.GetRepositoryEnvironmentName("docker", version)

	// Prevent creating environment directory in CWD if repoPath is empty
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir are empty, cannot create Docker environment")
		}
		// Use cache directory when repoPath is empty
		envPath := filepath.Join(cacheDir, "docker-"+envDirName)
		if err := os.MkdirAll(envPath, 0o750); err != nil {
			return "", fmt.Errorf("failed to create environment directory: %w", err)
		}
		return envPath, nil
	}

	envPath := filepath.Join(repoPath, envDirName)
	if err := os.MkdirAll(envPath, 0o750); err != nil {
		return "", fmt.Errorf("failed to create environment directory: %w", err)
	}
	return envPath, nil
}

// InstallDependencies does nothing for Docker (dependencies are in the image)
func (d *DockerLanguage) InstallDependencies(_ string, deps []string) error {
	// Docker language doesn't install dependencies - they're in the image
	if len(deps) > 0 {
		fmt.Printf(
			"[WARN] Docker language ignoring additional dependencies (use Docker image): %v\n",
			deps,
		)
	}
	return nil
}

// CheckHealth verifies Docker is working
func (d *DockerLanguage) CheckHealth(envPath, _ string) error {
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("docker environment directory does not exist: %s", envPath)
	}

	// Check if Docker daemon is accessible
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not accessible: %w", err)
	}

	return nil
}
