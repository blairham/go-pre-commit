package languages

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// SimpleLanguage is a declarative, config-driven implementation of the
// Language interface. It eliminates boilerplate for languages that follow
// the standard health-check / install / prepend-PATH-and-run pattern.
//
// Adding a new language is as simple as declaring a &SimpleLanguage{...}
// with the appropriate fields and registering it in init().
type SimpleLanguage struct {
	// --- Identity ---
	LangName       string // returned by Name()
	EnvDirName     string // returned by EnvironmentDir()
	DefaultVersion string // returned by GetDefaultVersion(); defaults to "default"

	// --- Health Check ---
	// HealthCmd runs a command to verify the runtime exists (e.g. ["dotnet", "--version"]).
	// HealthCheckFn is a full override; when set, HealthCmd is ignored.
	HealthCmd     []string
	HealthCheckFn func(prefix, version string) error

	// --- Install ---
	// InstallCmd returns the command name and args for the primary install step.
	// The command's Dir is set to prefix automatically.
	// InstallDepsFn handles additional_dependencies (called after InstallCmd).
	// InstallFn is a full override; when set, InstallCmd and InstallDepsFn are ignored.
	InstallCmd    func(envDir, prefix string) (name string, args []string)
	InstallDepsFn func(envDir, prefix string, deps []string) error
	InstallFn     func(prefix, version, envDirName string, additionalDeps []string) error

	// --- Run ---
	// RunBinSubdir is appended to envDir for the PATH entry.
	// "" means PrependPath(envDir); "bin" means PrependPath(envDir/bin).
	RunBinSubdir string
	// RunEnvFn builds custom env vars given envDir. Overrides RunBinSubdir.
	RunEnvFn func(envDir string) []string
	// RunFn is a full override for Run. When set, RunEnvFn and RunBinSubdir are ignored.
	RunFn func(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version, envDirName string) (int, []byte, error)
}

func (s *SimpleLanguage) Name() string { return s.LangName }

func (s *SimpleLanguage) EnvironmentDir() string { return s.EnvDirName }

func (s *SimpleLanguage) GetDefaultVersion() string {
	if s.DefaultVersion != "" {
		return s.DefaultVersion
	}
	return "default"
}

func (s *SimpleLanguage) HealthCheck(prefix, version string) error {
	if s.HealthCheckFn != nil {
		return s.HealthCheckFn(prefix, version)
	}
	if len(s.HealthCmd) == 0 {
		return nil
	}
	cmd := exec.Command(s.HealthCmd[0], s.HealthCmd[1:]...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s not available: %w", s.HealthCmd[0], err)
	}
	return nil
}

func (s *SimpleLanguage) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, s.EnvDirName+"-"+version)

	if s.InstallFn != nil {
		return s.InstallFn(prefix, version, s.EnvDirName, additionalDeps)
	}

	if s.InstallCmd != nil {
		name, args := s.InstallCmd(envDir, prefix)
		cmd := exec.Command(name, args...)
		cmd.Dir = prefix
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s failed: %s: %w", name, string(out), err)
		}
	}

	if len(additionalDeps) > 0 && s.InstallDepsFn != nil {
		if err := s.InstallDepsFn(envDir, prefix, additionalDeps); err != nil {
			return err
		}
	}

	return nil
}

func (s *SimpleLanguage) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	if s.RunFn != nil {
		return s.RunFn(ctx, prefix, workDir, entry, args, fileArgs, version, s.EnvDirName)
	}

	envDir := filepath.Join(prefix, s.EnvDirName+"-"+version)

	var env []string
	if s.RunEnvFn != nil {
		env = s.RunEnvFn(envDir)
	} else {
		binDir := envDir
		if s.RunBinSubdir != "" {
			binDir = filepath.Join(envDir, s.RunBinSubdir)
		}
		env = []string{PrependPath(binDir)}
	}

	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
