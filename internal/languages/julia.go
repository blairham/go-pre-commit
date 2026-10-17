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

// Julia implements the Language interface for Julia hooks, ported from
// upstream's pre_commit/languages/julia.py (4.7.0).
type Julia struct{}

// Name implements Language.
func (j *Julia) Name() string { return "julia" }

// EnvironmentDir is upstream's ENVIRONMENT_DIR. It was julia_env before
// #118; an environment built under that name is not found under this one, so
// the hook is reinstalled once and the old directory is left for gc.
func (j *Julia) EnvironmentDir() string { return "juliaenv" }

// GetDefaultVersion is upstream's basic_get_default_version.
func (j *Julia) GetDefaultVersion() string { return "default" }

// HealthCheck is upstream's lang_base.basic_health_check: an installed
// environment is always healthy. A missing toolchain shows when the hook runs.
func (j *Julia) HealthCheck(_, _ string) error { return nil }

// juliaUnset is the part of upstream's get_env_patch that removes a variable:
// a JULIA_PROJECT in the caller's environment would override the load path.
var juliaUnset = []string{"JULIA_PROJECT"}

// juliaEnv is upstream's julia get_env_patch. JULIA_LOAD_PATH is limited to
// the hook env, and the depot lives inside it so installed packages and
// precompile caches stay with it instead of leaking into a shared ~/.julia
// (4.6.1, pre-commit/pre-commit#3711). The depot's trailing separator leaves
// an empty entry, which julia expands to its default depots, so the bundled
// stdlib and its precompile caches remain available.
func juliaEnv(envDir string) []string {
	return []string{
		"JULIA_LOAD_PATH=" + envDir,
		"JULIA_DEPOT_PATH=" + filepath.Join(envDir, "depot") + string(os.PathListSeparator),
	}
}

// juliaInstallScript is upstream's install_environment julia_code, verbatim.
// The env dir and additional dependencies arrive as ARGS, never spliced into
// the source.
const juliaInstallScript = `
        @assert length(ARGS) > 0
        hook_env = ARGS[1]
        deps = join(ARGS[2:end], " ")

        # We prepend @stdlib here so that we can load the package manager even
        # though ` + "`get_env_patch`" + ` limits ` + "`JULIA_LOAD_PATH`" + ` to just the hook env.
        pushfirst!(LOAD_PATH, "@stdlib")
        using Pkg
        popfirst!(LOAD_PATH)

        # Instantiate the environment shipped with the hook repo. If we have
        # additional dependencies we disable precompilation in this step to
        # avoid double work.
        precompile = isempty(deps) ? "1" : "0"
        withenv("JULIA_PKG_PRECOMPILE_AUTO" => precompile) do
            Pkg.instantiate()
        end

        # Add additional dependencies (with precompilation)
        if !isempty(deps)
            withenv("JULIA_PKG_PRECOMPILE_AUTO" => "1") do
                Pkg.REPLMode.pkgstr("add " * deps)
            end
        end
        `

// InstallEnvironment is upstream's install_environment: copy the hook repo's
// project and manifest into the env (or start from an empty Project.toml),
// then instantiate it and add any additional dependencies.
func (j *Julia) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, j.EnvironmentDir()+"-"+version)
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		return err
	}

	projectFound, err := copyFirstFile(prefix, envDir, "JuliaProject.toml", "Project.toml")
	if err != nil {
		return err
	}
	// Without a project file the package manager errors, so start empty.
	if !projectFound {
		f, err := os.OpenFile(filepath.Join(envDir, "Project.toml"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	if _, err := copyFirstFile(prefix, envDir, "JuliaManifest.toml", "Manifest.toml"); err != nil {
		return err
	}

	args := append([]string{"--startup-file=no", "-e", juliaInstallScript, "--", envDir}, additionalDeps...)
	cmd := exec.Command("julia", args...)
	cmd.Dir = prefix
	cmd.Env = append(environWithout(cmd.Environ(), juliaUnset), juliaEnv(envDir)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("julia install failed: %s: %w", string(out), err)
	}
	return nil
}

// copyFirstFile copies the first of names that is a regular file in src into
// dst, keeping its mode as shutil.copy does, and reports whether one was.
func copyFirstFile(src, dst string, names ...string) (bool, error) {
	for _, name := range names {
		from := filepath.Join(src, name)
		info, err := os.Stat(from)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(from)
		if err != nil {
			return false, err
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, info.Mode().Perm()); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// Run is upstream's run_hook: the entry's first word is a julia script,
// relative to the hook repo for a remote hook and left as written for a
// `repo: local` one, run as `julia --startup-file=no <script> <args> <files>`.
func (j *Julia) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	envDir := filepath.Join(prefix, j.EnvironmentDir()+"-"+version)

	parts := ParseEntry(entry)
	if len(parts) == 0 {
		return -1, nil, fmt.Errorf("empty entry")
	}

	script := parts[0]
	if !isLocalHook(ctx) && !filepath.IsAbs(script) {
		// prefix.path(cmd[0]): os.path.join, which keeps an absolute path.
		script = filepath.Join(prefix, script)
	}

	argv := make([]string, 0, 3+len(parts)+len(args)+len(fileArgs))
	argv = append(argv, "julia", "--startup-file=no", script)
	argv = append(argv, parts[1:]...)
	argv = append(argv, args...)
	argv = append(argv, fileArgs...)
	return runHookArgv(ctx, workDir, argv, juliaEnv(envDir), juliaUnset)
}
