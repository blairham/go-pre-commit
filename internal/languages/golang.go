package languages

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Golang implements the Language interface for Go hooks.
type Golang struct{}

func (g *Golang) Name() string           { return "golang" }
func (g *Golang) EnvironmentDir() string  { return "go_env" }
func (g *Golang) GetDefaultVersion() string { return "default" }

func (g *Golang) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)
	goPath := filepath.Join(envDir, "bin")
	cmd := exec.Command("ls", goPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("golang environment unhealthy: %w", err)
	}
	return nil
}

func (g *Golang) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)

	env := []string{
		fmt.Sprintf("GOPATH=%s", envDir),
		fmt.Sprintf("GOBIN=%s", filepath.Join(envDir, "bin")),
	}

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
