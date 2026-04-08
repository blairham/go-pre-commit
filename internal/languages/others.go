package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Conda implements the Language interface for Conda hooks.
type Conda struct{}

func (c *Conda) Name() string           { return "conda" }
func (c *Conda) EnvironmentDir() string  { return "conda_env" }
func (c *Conda) GetDefaultVersion() string { return "default" }

func (c *Conda) HealthCheck(prefix, version string) error {
	condaExe := condaExecutable()
	cmd := exec.Command(condaExe, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s not available: %w", condaExe, err)
	}
	return nil
}

func (c *Conda) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, c.EnvironmentDir()+"-"+version)
	condaExe := condaExecutable()

	cmd := exec.Command(condaExe, "env", "create", "--file", "environment.yml", "--prefix", envDir)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s env create failed: %s: %w", condaExe, string(out), err)
	}

	if len(additionalDeps) > 0 {
		args := append([]string{"install", "--prefix", envDir, "-y"}, additionalDeps...)
		cmd := exec.Command(condaExe, args...)
		cmd.Dir = prefix
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s install failed: %s: %w", condaExe, string(out), err)
		}
	}

	return nil
}

func (c *Conda) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, c.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{
		PrependPath(binDir),
		fmt.Sprintf("CONDA_PREFIX=%s", envDir),
	}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// condaExecutable returns the conda-like executable to use, respecting
// PRE_COMMIT_USE_MICROMAMBA and PRE_COMMIT_USE_MAMBA environment variables.
func condaExecutable() string {
	if os.Getenv("PRE_COMMIT_USE_MICROMAMBA") != "" {
		return "micromamba"
	}
	if os.Getenv("PRE_COMMIT_USE_MAMBA") != "" {
		return "mamba"
	}
	return "conda"
}

// Coursier implements the Language interface for Coursier (JVM) hooks.
type Coursier struct{}

func (c *Coursier) Name() string           { return "coursier" }
func (c *Coursier) EnvironmentDir() string  { return "coursier_env" }
func (c *Coursier) GetDefaultVersion() string { return "default" }

func (c *Coursier) HealthCheck(prefix, version string) error {
	if _, err := exec.LookPath("cs"); err != nil {
		if _, err := exec.LookPath("coursier"); err != nil {
			return fmt.Errorf("coursier (cs) not available")
		}
	}
	return nil
}

func (c *Coursier) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, c.EnvironmentDir()+"-"+version)

	csCmd := "cs"
	if _, err := exec.LookPath(csCmd); err != nil {
		csCmd = "coursier"
	}

	channelDir := filepath.Join(prefix, ".pre-commit-channel")
	cmd := exec.Command(csCmd, "install", "--install-dir", envDir, "--default-channels=false", "--channel", channelDir)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("coursier install failed: %s: %w", string(out), err)
	}

	return nil
}

func (c *Coursier) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, c.EnvironmentDir()+"-"+version)
	env := []string{PrependPath(envDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// Dart implements the Language interface for Dart hooks.
type Dart struct{}

func (d *Dart) Name() string           { return "dart" }
func (d *Dart) EnvironmentDir() string  { return "dart_env" }
func (d *Dart) GetDefaultVersion() string { return "default" }

func (d *Dart) HealthCheck(prefix, version string) error {
	cmd := exec.Command("dart", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dart not available: %w", err)
	}
	return nil
}

func (d *Dart) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, d.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")

	// Compile dart executables.
	matches, _ := filepath.Glob(filepath.Join(prefix, "bin", "*.dart"))
	for _, m := range matches {
		name := filepath.Base(m)
		name = name[:len(name)-5] // Remove .dart extension.
		outPath := filepath.Join(binDir, name)
		cmd := exec.Command("dart", "compile", "exe", m, "-o", outPath)
		cmd.Dir = prefix
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("dart compile failed: %s: %w", string(out), err)
		}
	}

	return nil
}

func (d *Dart) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, d.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{PrependPath(binDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// Dotnet implements the Language interface for .NET hooks.
type Dotnet struct{}

func (d *Dotnet) Name() string           { return "dotnet" }
func (d *Dotnet) EnvironmentDir() string  { return "dotnet_env" }
func (d *Dotnet) GetDefaultVersion() string { return "default" }

func (d *Dotnet) HealthCheck(prefix, version string) error {
	cmd := exec.Command("dotnet", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dotnet not available: %w", err)
	}
	return nil
}

func (d *Dotnet) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, d.EnvironmentDir()+"-"+version)

	cmd := exec.Command("dotnet", "tool", "install", "--tool-path", envDir, "--add-source", ".")
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("dotnet tool install failed: %s: %w", string(out), err)
	}

	return nil
}

func (d *Dotnet) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, d.EnvironmentDir()+"-"+version)
	env := []string{PrependPath(envDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// Haskell implements the Language interface for Haskell hooks.
type Haskell struct{}

func (h *Haskell) Name() string           { return "haskell" }
func (h *Haskell) EnvironmentDir() string  { return "hs_env" }
func (h *Haskell) GetDefaultVersion() string { return "default" }

func (h *Haskell) HealthCheck(prefix, version string) error {
	cmd := exec.Command("cabal", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cabal not available: %w", err)
	}
	return nil
}

func (h *Haskell) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, h.EnvironmentDir()+"-"+version)

	cmd := exec.Command("cabal", "install", "--install-method=copy", "--installdir="+envDir)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cabal install failed: %s: %w", string(out), err)
	}

	return nil
}

func (h *Haskell) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, h.EnvironmentDir()+"-"+version)
	env := []string{PrependPath(envDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// Julia implements the Language interface for Julia hooks.
type Julia struct{}

func (j *Julia) Name() string           { return "julia" }
func (j *Julia) EnvironmentDir() string  { return "julia_env" }
func (j *Julia) GetDefaultVersion() string { return "default" }

func (j *Julia) HealthCheck(prefix, version string) error {
	cmd := exec.Command("julia", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("julia not available: %w", err)
	}
	return nil
}

func (j *Julia) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, j.EnvironmentDir()+"-"+version)

	// Instantiate the environment.
	installScript := `
using Pkg
Pkg.activate("` + envDir + `")
Pkg.develop(path="` + prefix + `")
Pkg.instantiate()
`
	for _, dep := range additionalDeps {
		installScript += `Pkg.add("` + dep + `")` + "\n"
	}

	cmd := exec.Command("julia", "-e", installScript)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("julia install failed: %s: %w", string(out), err)
	}

	return nil
}

func (j *Julia) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, j.EnvironmentDir()+"-"+version)

	parts := ParseEntry(entry)
	if len(parts) == 0 {
		return -1, nil, fmt.Errorf("empty entry")
	}

	allArgs := []string{"--project=" + envDir}
	allArgs = append(allArgs, parts[1:]...)
	allArgs = append(allArgs, args...)
	allArgs = append(allArgs, fileArgs...)

	return RunCommand(ctx, workDir, "julia", allArgs...)
}

// Lua implements the Language interface for Lua hooks.
type Lua struct{}

func (l *Lua) Name() string           { return "lua" }
func (l *Lua) EnvironmentDir() string  { return "lua_env" }
func (l *Lua) GetDefaultVersion() string { return "default" }

func (l *Lua) HealthCheck(prefix, version string) error {
	cmd := exec.Command("luarocks", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("luarocks not available: %w", err)
	}
	return nil
}

func (l *Lua) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, l.EnvironmentDir()+"-"+version)

	cmd := exec.Command("luarocks", "install", "--tree", envDir, prefix)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("luarocks install failed: %s: %w", string(out), err)
	}

	return nil
}

func (l *Lua) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, l.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{PrependPath(binDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// Perl implements the Language interface for Perl hooks.
type Perl struct{}

func (p *Perl) Name() string           { return "perl" }
func (p *Perl) EnvironmentDir() string  { return "perl_env" }
func (p *Perl) GetDefaultVersion() string { return "default" }

func (p *Perl) HealthCheck(prefix, version string) error {
	cmd := exec.Command("perl", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("perl not available: %w", err)
	}
	return nil
}

func (p *Perl) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)

	cmd := exec.Command("cpan", "-T", "-l", envDir, ".")
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cpan install failed: %s: %w", string(out), err)
	}

	for _, dep := range additionalDeps {
		cmd := exec.Command("cpan", "-T", "-l", envDir, dep)
		cmd.Dir = prefix
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("cpan install %s failed: %s: %w", dep, string(out), err)
		}
	}

	return nil
}

func (p *Perl) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, p.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{PrependPath(binDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// R implements the Language interface for R hooks.
type R struct{}

func (r *R) Name() string           { return "r" }
func (r *R) EnvironmentDir() string  { return "r_env" }
func (r *R) GetDefaultVersion() string { return "default" }

func (r *R) HealthCheck(prefix, version string) error {
	cmd := exec.Command("Rscript", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("R not available: %w", err)
	}
	return nil
}

func (r *R) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)

	// Restore renv environment.
	script := fmt.Sprintf(`
renv::activate(project = '%s')
renv::restore(project = '%s')
`, envDir, prefix)
	cmd := exec.Command("Rscript", "--vanilla", "-e", script)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("renv restore failed: %s: %w", string(out), err)
	}

	return nil
}

func (r *R) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	env := []string{fmt.Sprintf("RENV_PROJECT=%s", envDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}

// Swift implements the Language interface for Swift hooks.
type Swift struct{}

func (s *Swift) Name() string           { return "swift" }
func (s *Swift) EnvironmentDir() string  { return "swift_env" }
func (s *Swift) GetDefaultVersion() string { return "default" }

func (s *Swift) HealthCheck(prefix, version string) error {
	cmd := exec.Command("swift", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("swift not available: %w", err)
	}
	return nil
}

func (s *Swift) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, s.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")

	cmd := exec.Command("swift", "build", "-c", "release", "--build-path", envDir)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("swift build failed: %s: %w", string(out), err)
	}

	// Copy built executables.
	releaseBin := filepath.Join(envDir, "release")
	entries, _ := filepath.Glob(filepath.Join(releaseBin, "*"))
	_ = exec.Command("mkdir", "-p", binDir).Run()
	for _, e := range entries {
		exec.Command("cp", e, binDir).Run()
	}

	return nil
}

func (s *Swift) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, s.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{PrependPath(binDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
