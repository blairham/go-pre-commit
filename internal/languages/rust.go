// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// rustToolchain is upstream's _rust_toolchain: `default` is stable.
func rustToolchain(version string) string {
	if version == "default" {
		return "stable"
	}
	return version
}

// rustEnvPatch is upstream's get_env_patch: the env's bin on PATH, and the
// toolchain pinned through RUSTUP_TOOLCHAIN unless the system's is wanted.
func rustEnvPatch(envDir, version string) []string {
	env := []string{PrependPath(filepath.Join(envDir, "bin"))}
	if version != "system" {
		env = append(env, "RUSTUP_TOOLCHAIN="+rustToolchain(version))
	}
	return env
}

// rustupInitURL is where upstream downloads rustup-init; a variable so tests
// can serve their own.
var rustupInitURL = func() string {
	if runtime.GOOS == "windows" {
		return "https://win.rustup.rs/x86_64"
	}
	return "https://sh.rustup.rs"
}()

// rustCmd runs argv in dir with env applied over the process environment,
// its executable resolved on env's PATH, as upstream's cmd_output_b inside
// an envcontext does.
func rustCmd(dir string, env []string, argv ...string) error {
	bin, err := lookPathInEnv(argv[0], env)
	if err != nil {
		bin = argv[0]
	}
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(environWithout(os.Environ(), envKeys(env)), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s failed: %s: %w", strings.Join(argv, " "), string(out), err)
	}
	return nil
}

func envKeys(env []string) []string {
	keys := make([]string, 0, len(env))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		keys = append(keys, k)
	}
	return keys
}

// withEnv returns env with each of set's keys replaced.
func withEnv(env []string, set ...string) []string {
	return append(environWithout(env, envKeys(set)), set...)
}

// installRustWithToolchain is upstream's install_rust_with_toolchain: with
// CARGO_HOME at the env and a throwaway RUSTUP_HOME, rustup-init is
// downloaded and run when there is no rustup (putting rustup into the env's
// bin), then the toolchain is installed.
func installRustWithToolchain(toolchain, envDir string, env []string) (err error) {
	rustupDir, err := os.MkdirTemp("", "pre-commit-rustup-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(rustupDir)) }()
	env = withEnv(env, "CARGO_HOME="+envDir, "RUSTUP_HOME="+rustupDir)

	if _, findErr := lookPathInEnv("rustup", env); findErr != nil {
		name := "rustup-init"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		rustupInit := filepath.Join(rustupDir, name)
		if dlErr := download(rustupInitURL, rustupInit); dlErr != nil {
			return dlErr
		}
		if initErr := rustCmd(
			"",
			env,
			rustupInit,
			"-y",
			"--quiet",
			"--no-modify-path",
			"--default-toolchain",
			"none",
		); initErr != nil {
			return initErr
		}
	}
	return rustCmd("", env, "rustup", "toolchain", "install", "--no-self-update", toolchain)
}

// download writes url's body to path, executable.
func download(url, path string) (err error) {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	return errors.Join(copyErr, f.Close())
}

// InstallEnvironment is upstream rust.install_environment (4.7.0). A
// `cli:` dependency is a binary to `cargo install` into the env; any other
// is a library crate the hook builds against, added with `cargo add`. For
// any version but `system` the toolchain is installed first through rustup,
// and the builds run with a throwaway RUSTUP_HOME.
func (r *Rust) InstallEnvironment(prefix, version string, additionalDeps []string) (err error) {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return err
	}

	var libDeps []string
	packages := [][]string{{"--path", "."}}
	for _, dep := range additionalDeps {
		cli, ok := strings.CutPrefix(dep, "cli:")
		if !ok {
			libDeps = append(libDeps, dep)
			continue
		}
		pkg, crateVersion, _ := strings.Cut(cli, ":")
		if crateVersion != "" {
			packages = append(packages, []string{pkg, "--version", crateVersion})
		} else {
			packages = append(packages, []string{pkg})
		}
	}

	env := rustEnvPatch(envDir, version)
	if version != "system" {
		if err := installRustWithToolchain(rustToolchain(version), envDir, env); err != nil {
			return err
		}
		tmp, err := os.MkdirTemp("", "pre-commit-rustup-")
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, os.RemoveAll(tmp)) }()
		env = withEnv(env, "RUSTUP_HOME="+tmp)
	}

	if len(libDeps) > 0 {
		crates := make([]string, 0, len(libDeps))
		for _, dep := range libDeps {
			name, spec, _ := strings.Cut(dep, ":")
			if spec == "" {
				spec = "*"
			}
			crates = append(crates, name+"@"+spec)
		}
		if err := rustCmd(prefix, env, append([]string{"cargo", "add"}, crates...)...); err != nil {
			return err
		}
	}
	for _, pkg := range packages {
		argv := append([]string{"cargo", "install", "--bins", "--root", envDir}, pkg...)
		if err := rustCmd(prefix, env, argv...); err != nil {
			return err
		}
	}
	return nil
}

// Run is upstream's basic_run_hook inside in_env.
func (r *Rust) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, rustEnvPatch(envDir, version))
}
