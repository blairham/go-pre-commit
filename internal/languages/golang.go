package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Golang implements the Language interface for Go hooks.
type Golang struct{}

func (g *Golang) Name() string              { return "golang" }
func (g *Golang) EnvironmentDir() string    { return "go_env" }
func (g *Golang) GetDefaultVersion() string { return "default" }

func (g *Golang) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("golang environment unhealthy: no binaries in %s", binDir)
	}
	return nil
}

// goInstallEnv builds the env overrides for installing a golang hook env.
// GOTOOLCHAIN defaults to "local" so a hook repo's go.mod can't pull in a
// different toolchain — unless the caller set GOTOOLCHAIN explicitly. CI pins
// a repo-matching toolchain (e.g. GOTOOLCHAIN=go1.26.4) so hooks whose module
// requires a newer Go than the one on PATH can still build; forcing "local"
// over that pin makes such installs unbuildable.
func goInstallEnv(envDir string) []string {
	env := []string{
		fmt.Sprintf("GOPATH=%s", envDir),
		fmt.Sprintf("GOBIN=%s", filepath.Join(envDir, "bin")),
	}
	if os.Getenv("GOTOOLCHAIN") == "" {
		env = append(env, "GOTOOLCHAIN=local")
	}
	return env
}

func (g *Golang) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)

	env := goInstallEnv(envDir)

	// Install the hook package.
	args := []string{"install", "./..."}
	cmd := exec.Command("go", args...)
	cmd.Dir = prefix
	cmd.Env = append(cmd.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go install failed: %s: %w", string(out), err)
	}

	// Install additional dependencies.
	for _, dep := range additionalDeps {
		cmd := exec.Command("go", "install", dep)
		cmd.Dir = prefix
		cmd.Env = append(cmd.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go install %s failed: %s: %w", dep, string(out), err)
		}
	}

	return nil
}

func (g *Golang) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{
		PrependPath(binDir),
		fmt.Sprintf("GOPATH=%s", envDir),
	}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
