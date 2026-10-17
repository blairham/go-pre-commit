// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
)

// Rust implements the Language interface for Rust hooks.
type Rust struct{}

// Name is the language's name in a hook manifest.
func (r *Rust) Name() string { return "rust" }

// EnvironmentDir is upstream's ENVIRONMENT_DIR.
func (r *Rust) EnvironmentDir() string { return "rustenv" }

// GetDefaultVersion is upstream rust.get_default_version (4.7.0), cached for
// the process: "system" when `cargo --version` succeeds (a rustup with no
// toolchain has a cargo that fails it), otherwise "default".
func (r *Rust) GetDefaultVersion() string { return rustDefaultVersionOnce() }

var rustDefaultVersionOnce = sync.OnceValue(rustDefaultVersion)

func rustDefaultVersion() string {
	cmd := exec.Command("cargo", "--version")
	cmd.Dir = string(filepath.Separator)
	if cmd.Run() == nil {
		return "system"
	}
	return "default"
}

// HealthCheck is upstream's lang_base.basic_health_check: an installed
// environment is always healthy. A missing toolchain shows when the hook runs.
func (r *Rust) HealthCheck(_, _ string) error { return nil }

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

// Run implements Language.
func (r *Rust) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
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
