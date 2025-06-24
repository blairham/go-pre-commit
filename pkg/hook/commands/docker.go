package commands

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// buildDockerCommand builds a Docker command
func (b *Builder) buildDockerCommand(
	entry string,
	args []string,
	hook config.Hook,
) (*exec.Cmd, error) {
	dockerArgs := []string{"run", "--rm"}

	// Add volume mounts for the current directory
	dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/src", b.repoRoot), "-w", "/src")

	// Use language version as image if provided
	image := entry
	if hook.LanguageVersion != "" {
		image = hook.LanguageVersion
	}

	// Add the image
	dockerArgs = append(dockerArgs, image)

	// Add the command arguments
	if entry != image {
		// entry is the command, not the image
		dockerArgs = append(dockerArgs, strings.Fields(entry)...)
	}
	dockerArgs = append(dockerArgs, args...)

	return exec.Command("docker", dockerArgs...), nil
}

// buildDockerImageCommand builds a Docker image command
func (b *Builder) buildDockerImageCommand(
	entry string,
	args []string,
	hook config.Hook,
) (*exec.Cmd, error) {
	return b.buildDockerCommand(entry, args, hook)
}
