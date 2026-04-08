package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Docker implements the Language interface for Docker hooks.
type Docker struct{}

func (d *Docker) Name() string           { return "docker" }
func (d *Docker) EnvironmentDir() string  { return "docker" }
func (d *Docker) GetDefaultVersion() string { return "default" }

func (d *Docker) HealthCheck(prefix, version string) error {
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}
	return nil
}

func (d *Docker) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	// Build the Docker image.
	cmd := exec.Command("docker", "build", "-t", d.imageTag(prefix), ".")
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build failed: %s: %w", string(out), err)
	}
	return nil
}

func (d *Docker) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	cwd, _ := os.Getwd()
	dockerArgs := []string{
		"run", "--rm",
		"-v", cwd + ":/src:rw,Z",
		"--workdir", "/src",
	}

	// Parse entry for entrypoint.
	parts := ParseEntry(entry)
	if len(parts) > 0 {
		dockerArgs = append(dockerArgs, "--entrypoint", parts[0])
	}
	dockerArgs = append(dockerArgs, d.imageTag(prefix))
	if len(parts) > 1 {
		dockerArgs = append(dockerArgs, parts[1:]...)
	}
	dockerArgs = append(dockerArgs, args...)
	dockerArgs = append(dockerArgs, fileArgs...)

	return RunCommand(ctx, workDir, "docker", dockerArgs...)
}

func (d *Docker) imageTag(prefix string) string {
	// Generate a tag from the prefix path.
	tag := strings.ReplaceAll(filepath.Base(prefix), "/", "-")
	return fmt.Sprintf("pre-commit-%s:latest", strings.ToLower(tag))
}

// DockerImage implements the Language interface for pre-built Docker image hooks.
type DockerImage struct{}

func (d *DockerImage) Name() string           { return "docker_image" }
func (d *DockerImage) EnvironmentDir() string  { return "" }
func (d *DockerImage) GetDefaultVersion() string { return "default" }

func (d *DockerImage) HealthCheck(prefix, version string) error {
	cmd := exec.Command("docker", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}
	return nil
}

func (d *DockerImage) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	// No installation needed for docker_image.
	return nil
}

func (d *DockerImage) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	cwd, _ := os.Getwd()
	parts := ParseEntry(entry)
	if len(parts) == 0 {
		return -1, nil, fmt.Errorf("docker_image entry is required")
	}

	dockerArgs := []string{
		"run", "--rm",
		"-v", cwd + ":/src:rw,Z",
		"--workdir", "/src",
	}

	// Check if entry has --entrypoint flag.
	image := parts[0]
	extraArgs := parts[1:]
	if image == "--entrypoint" && len(parts) >= 3 {
		dockerArgs = append(dockerArgs, "--entrypoint", parts[1])
		image = parts[2]
		extraArgs = parts[3:]
	}

	dockerArgs = append(dockerArgs, image)
	dockerArgs = append(dockerArgs, extraArgs...)
	dockerArgs = append(dockerArgs, args...)
	dockerArgs = append(dockerArgs, fileArgs...)

	return RunCommand(ctx, workDir, "docker", dockerArgs...)
}
