package languages

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Python implements the Language interface for Python hooks.
type Python struct{}

func (p *Python) Name() string           { return "python" }
func (p *Python) EnvironmentDir() string  { return "py_env" }
func (p *Python) GetDefaultVersion() string { return "python3" }

func (p *Python) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	pythonPath := filepath.Join(binDir, "python")
	cmd := exec.Command(pythonPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("python environment unhealthy: %w", err)
	}
	return nil
}

func (p *Python) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)

	python := version
	if python == "default" {
		python = p.GetDefaultVersion()
	}

	// Create virtualenv.
	cmd := exec.Command(python, "-mvirtualenv", envDir)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fall back to venv.
		cmd = exec.Command(python, "-m", "venv", envDir)
		cmd.Dir = prefix
		if out2, err2 := cmd.CombinedOutput(); err2 != nil {
			return fmt.Errorf("failed to create virtualenv: %s\nfailed to create venv: %s", string(out), string(out2))
		}
	}

	// Install the hook package.
	pip := filepath.Join(envDir, "bin", "pip")
	args := []string{"install", "."}
	args = append(args, additionalDeps...)
	cmd = exec.Command(pip, args...)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pip install failed: %s: %w", string(out), err)
	}

	return nil
}

func (p *Python) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{
		PrependPath(binDir),
		fmt.Sprintf("VIRTUAL_ENV=%s", envDir),
	}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
