package languages

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Ruby implements the Language interface for Ruby hooks.
type Ruby struct{}

func (r *Ruby) Name() string           { return "ruby" }
func (r *Ruby) EnvironmentDir() string  { return "rbenv" }
func (r *Ruby) GetDefaultVersion() string { return "default" }

func (r *Ruby) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	gemHome := filepath.Join(envDir, "gems")
	cmd := exec.Command("ruby", "--version")
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("GEM_HOME=%s", gemHome))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ruby environment unhealthy: %w", err)
	}
	return nil
}

func (r *Ruby) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	gemHome := filepath.Join(envDir, "gems")
	binDir := filepath.Join(gemHome, "bin")

	env := []string{
		fmt.Sprintf("GEM_HOME=%s", gemHome),
		PrependPath(binDir),
	}

	// Build and install the gem.
	// Find gemspec.
	matches, _ := filepath.Glob(filepath.Join(prefix, "*.gemspec"))
	if len(matches) > 0 {
		cmd := exec.Command("gem", "build", filepath.Base(matches[0]))
		cmd.Dir = prefix
		cmd.Env = append(cmd.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gem build failed: %s: %w", string(out), err)
		}

		gemFiles, _ := filepath.Glob(filepath.Join(prefix, "*.gem"))
		if len(gemFiles) > 0 {
			cmd = exec.Command("gem", "install", "--no-document", filepath.Base(gemFiles[0]))
			cmd.Dir = prefix
			cmd.Env = append(cmd.Environ(), env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("gem install failed: %s: %w", string(out), err)
			}
		}
	}

	// Install additional dependencies.
	for _, dep := range additionalDeps {
		cmd := exec.Command("gem", "install", "--no-document", dep)
		cmd.Dir = prefix
		cmd.Env = append(cmd.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("gem install %s failed: %s: %w", dep, string(out), err)
		}
	}

	return nil
}

func (r *Ruby) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	gemHome := filepath.Join(envDir, "gems")
	binDir := filepath.Join(gemHome, "bin")
	env := []string{
		PrependPath(binDir),
		fmt.Sprintf("GEM_HOME=%s", gemHome),
	}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
