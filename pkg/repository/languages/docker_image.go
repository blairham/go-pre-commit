package languages

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// DockerImageLanguage handles pre-built Docker images
type DockerImageLanguage struct {
	*language.Base
}

// NewDockerImageLanguage creates a new Docker image language handler
func NewDockerImageLanguage() *DockerImageLanguage {
	return &DockerImageLanguage{
		Base: language.NewBase(
			"Docker Image",
			"docker",
			"--version",
			"https://docs.docker.com/get-docker/",
		),
	}
}

// SetupEnvironmentWithRepo sets up a Docker image environment for a specific repository
func (d *DockerImageLanguage) SetupEnvironmentWithRepo(
	_, _, repoPath, _ string, // repoURL is unused
	_ []string,
) (string, error) {
	// Check if Docker is available
	if !d.IsRuntimeAvailable() {
		d.PrintNotFoundMessage()
		return "", fmt.Errorf("docker runtime not found in PATH, cannot setup environment")
	}

	// Docker image doesn't need a separate environment - use the repository path
	return repoPath, nil
}

// InstallDependencies does nothing for Docker image (image is pre-built)
func (d *DockerImageLanguage) InstallDependencies(_ string, deps []string) error {
	// Docker image language doesn't install dependencies - use pre-built image
	if len(deps) > 0 {
		fmt.Printf(
			"[WARN] Docker image language ignoring additional dependencies (use pre-built image): %v\n",
			deps,
		)
	}
	return nil
}

// CheckHealth verifies Docker is working
func (d *DockerImageLanguage) CheckHealth(envPath, _ string) error {
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("docker image environment directory does not exist: %s", envPath)
	}

	// Check if Docker daemon is accessible
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon is not accessible: %w", err)
	}

	return nil
}
