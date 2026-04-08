package languages

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Rust implements the Language interface for Rust hooks.
type Rust struct{}

func (r *Rust) Name() string           { return "rust" }
func (r *Rust) EnvironmentDir() string  { return "rustenv" }
func (r *Rust) GetDefaultVersion() string { return "default" }

func (r *Rust) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	cmd := exec.Command(filepath.Join(binDir, "cargo"), "--version")
	if err := cmd.Run(); err != nil {
		// Fall back to system cargo.
		cmd = exec.Command("cargo", "--version")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("rust environment unhealthy: %w", err)
		}
	}
	return nil
}

func (r *Rust) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)

	env := []string{
		fmt.Sprintf("CARGO_HOME=%s", envDir),
		PrependPath(filepath.Join(envDir, "bin")),
	}

	// Install the hook binaries.
	cmd := exec.Command("cargo", "install", "--bins", "--root", envDir, "--path", ".")
	cmd.Dir = prefix
	cmd.Env = append(cmd.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cargo install failed: %s: %w", string(out), err)
	}

	// Install additional dependencies.
	for _, dep := range additionalDeps {
		args := []string{"install", "--root", envDir}
		// Handle cli: prefix for CLI dependencies.
		if len(dep) > 4 && dep[:4] == "cli:" {
			parts := splitDep(dep[4:])
			args = append(args, parts...)
		} else {
			parts := splitDep(dep)
			args = append(args, parts...)
		}
		cmd := exec.Command("cargo", args...)
		cmd.Dir = prefix
		cmd.Env = append(cmd.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("cargo install %s failed: %s: %w", dep, string(out), err)
		}
	}

	return nil
}

func (r *Rust) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{
		PrependPath(binDir),
		fmt.Sprintf("CARGO_HOME=%s", envDir),
	}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// splitDep splits a dependency spec like "name:version" into cargo install args.
func splitDep(dep string) []string {
	for i, c := range dep {
		if c == ':' {
			return []string{dep[:i], "--version", dep[i+1:]}
		}
	}
	return []string{dep}
}
