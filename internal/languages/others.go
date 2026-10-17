// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// condaLang is the Conda language backend.
var condaLang = &SimpleLanguage{
	LangName:   "conda",
	EnvDirName: "conda",
	InstallFn: func(prefix, version, envDirName string, additionalDeps []string) error {
		envDir := filepath.Join(prefix, envDirName+"-"+version)
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
	},
	RunEnvFn: func(envDir string) []string {
		return []string{
			PrependPath(filepath.Join(envDir, "bin")),
			fmt.Sprintf("CONDA_PREFIX=%s", envDir),
		}
	},
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

// coursierLang is the Coursier (JVM) language backend.
var coursierLang = &SimpleLanguage{
	LangName:   "coursier",
	EnvDirName: "coursier",
	InstallCmd: func(envDir, prefix string) (string, []string) {
		csCmd := "cs"
		if _, err := exec.LookPath(csCmd); err != nil {
			csCmd = "coursier"
		}
		channelDir := filepath.Join(prefix, ".pre-commit-channel")
		return csCmd, []string{"install", "--install-dir", envDir, "--default-channels=false", "--channel", channelDir}
	},
}

// dartLang is the Dart language backend.
var dartLang = &SimpleLanguage{
	LangName:     "dart",
	EnvDirName:   "dartenv",
	RunBinSubdir: "bin",
	InstallFn: func(prefix, version, envDirName string, _ []string) error {
		envDir := filepath.Join(prefix, envDirName+"-"+version)
		binDir := filepath.Join(envDir, "bin")

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
	},
}

// dotnetLang is the .NET language backend.
var dotnetLang = &SimpleLanguage{
	LangName:   "dotnet",
	EnvDirName: "dotnetenv",
	InstallCmd: func(envDir, prefix string) (string, []string) {
		return "dotnet", []string{"tool", "install", "--tool-path", envDir, "--add-source", "."}
	},
}

// haskellLang is the Haskell language backend.
var haskellLang = &SimpleLanguage{
	LangName:   "haskell",
	EnvDirName: "hs_env",
	InstallCmd: func(envDir, prefix string) (string, []string) {
		return "cabal", []string{"install", "--install-method=copy", "--installdir=" + envDir}
	},
}

// luaLang is the Lua language backend.
var luaLang = &SimpleLanguage{
	LangName:     "lua",
	EnvDirName:   "lua_env",
	RunBinSubdir: "bin",
	InstallCmd: func(envDir, prefix string) (string, []string) {
		return "luarocks", []string{"install", "--tree", envDir, prefix}
	},
}

// perlLang is the Perl language backend.
var perlLang = &SimpleLanguage{
	LangName:     "perl",
	EnvDirName:   "perl_env",
	RunBinSubdir: "bin",
	InstallCmd: func(envDir, prefix string) (string, []string) {
		return "cpan", []string{"-T", "-l", envDir, "."}
	},
	InstallDepsFn: func(envDir, prefix string, deps []string) error {
		for _, dep := range deps {
			cmd := exec.Command("cpan", "-T", "-l", envDir, dep)
			cmd.Dir = prefix
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("cpan install %s failed: %s: %w", dep, string(out), err)
			}
		}
		return nil
	},
}

// rLang is the R language backend.
var rLang = &SimpleLanguage{
	LangName:   "r",
	EnvDirName: "renv",
	HealthCmd:  []string{"Rscript", "--version"},
	InstallCmd: func(envDir, prefix string) (string, []string) {
		script := fmt.Sprintf(
			"renv::activate(project = '%s')\nrenv::restore(project = '%s')\n",
			envDir, prefix,
		)
		return "Rscript", []string{"--vanilla", "-e", script}
	},
	RunEnvFn: func(envDir string) []string {
		return []string{fmt.Sprintf("RENV_PROJECT=%s", envDir)}
	},
}

// Swift implements the Language interface for Swift hooks.
// Swift is kept as a standalone struct because its install involves a multi-step
// build + copy workflow that doesn't fit the SimpleLanguage pattern.
type Swift struct{}

func (s *Swift) Name() string              { return "swift" }
func (s *Swift) EnvironmentDir() string    { return "swift_env" }
func (s *Swift) GetDefaultVersion() string { return "default" }

// HealthCheck is upstream's lang_base.basic_health_check: an installed
// environment is always healthy. A missing toolchain shows when the hook runs.
func (s *Swift) HealthCheck(_, _ string) error { return nil }

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

// Run implements Language.
func (s *Swift) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	envDir := filepath.Join(prefix, s.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{PrependPath(binDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
