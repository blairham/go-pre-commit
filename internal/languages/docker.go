package languages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
)

// containerIDPattern matches the container id in /proc/1/mountinfo when
// running inside docker or podman (works for both cgroups v1 and v2).
var containerIDPattern = regexp.MustCompile(
	`/containers(?:/overlay-containers)?/([a-z0-9]{64})(?:/userdata)?/hostname`)

func containerIDFromMountinfo(mountinfo []byte) string {
	m := containerIDPattern.FindSubmatch(mountinfo)
	if m == nil {
		return ""
	}
	return string(m[1])
}

// currentContainerID returns the id of the container this process runs in,
// or "" when not running inside a container.
func currentContainerID() string {
	data, err := os.ReadFile("/proc/1/mountinfo")
	if err != nil {
		return ""
	}
	return containerIDFromMountinfo(data)
}

// translateMountPath maps path through the container's bind mounts described
// by `docker inspect` output, returning the path as seen by the host.
func translateMountPath(path string, inspectOut []byte) string {
	var containers []struct {
		Mounts []struct {
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
		} `json:"Mounts"`
	}
	if err := json.Unmarshal(inspectOut, &containers); err != nil || len(containers) == 0 {
		return path
	}
	for _, mount := range containers[0].Mounts {
		if path == mount.Destination {
			return mount.Source
		}
		prefix := strings.TrimSuffix(mount.Destination, "/") + "/"
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			return strings.TrimSuffix(mount.Source, "/") + "/" + rest
		}
	}
	return path
}

// dockerPath translates path for docker-in-docker: -v mount sources are
// resolved by the host daemon, so a container-local path must be rewritten
// to the corresponding host path.
func dockerPath(path string) string {
	id := currentContainerID()
	if id == "" {
		return path
	}
	out, err := exec.Command("docker", "inspect", id).Output()
	if err != nil {
		return path
	}
	return translateMountPath(path, out)
}

func rootlessFromInfo(infoJSON []byte) bool {
	var info struct {
		SecurityOptions []string `json:"SecurityOptions"`
		Host            struct {
			Security struct {
				Rootless bool `json:"rootless"`
			} `json:"security"`
		} `json:"host"`
	}
	if err := json.Unmarshal(infoJSON, &info); err != nil {
		return false
	}
	if info.Host.Security.Rootless { // podman
		return true
	}
	// docker; a null SecurityOptions list unmarshals to nil.
	return slices.Contains(info.SecurityOptions, "name=rootless")
}

// isRootless reports whether the daemon runs rootless (docker or podman),
// in which case -u would map to an unprivileged user inside the container.
var isRootless = sync.OnceValue(func() bool {
	out, err := exec.Command("docker", "system", "info", "--format", "{{ json . }}").Output()
	if err != nil {
		return false
	}
	return rootlessFromInfo(out)
})

// dockerRunArgs returns the common `docker run` arguments for hooks.
func dockerRunArgs() []string {
	cwd, _ := os.Getwd()
	args := []string{"run", "--rm"}
	if runtime.GOOS != "windows" && !isRootless() {
		args = append(args, "-u", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()))
	}
	return append(args, "-v", dockerPath(cwd)+":/src:rw,Z", "--workdir", "/src")
}

// Docker implements the Language interface for Docker hooks.
type Docker struct{}

func (d *Docker) Name() string              { return "docker" }
func (d *Docker) EnvironmentDir() string    { return "docker" }
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
	dockerArgs := dockerRunArgs()

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

func (d *DockerImage) Name() string              { return "docker_image" }
func (d *DockerImage) EnvironmentDir() string    { return "" }
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
	parts := ParseEntry(entry)
	if len(parts) == 0 {
		return -1, nil, fmt.Errorf("docker_image entry is required")
	}

	dockerArgs := dockerRunArgs()

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
