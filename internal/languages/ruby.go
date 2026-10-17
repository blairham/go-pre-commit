// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// rubyResources are upstream's vendored rbenv, ruby-build and ruby-download
// tarballs, byte for byte from pre-commit 4.7.0's pre_commit/resources (built
// by its testing/make-archives at rbenv 10e96bfc and ruby-build 447468b1).
//
//go:embed resources/rbenv.tar.gz resources/ruby-build.tar.gz resources/ruby-download.tar.gz
var rubyResources embed.FS

// Ruby implements the Language interface for Ruby hooks.
type Ruby struct{}

// Name is the language's name in a hook manifest.
func (r *Ruby) Name() string { return "ruby" }

// EnvironmentDir is upstream's ENVIRONMENT_DIR.
func (r *Ruby) EnvironmentDir() string { return "rbenv" }

// GetDefaultVersion is upstream ruby.get_default_version (4.7.0), cached for
// the process as upstream's lru_cache does: "system" when both ruby and gem
// are real installs (exeExists), otherwise "default", for which the install
// builds an rbenv inside the env.
func (r *Ruby) GetDefaultVersion() string { return rubyDefaultVersionOnce() }

var rubyDefaultVersionOnce = sync.OnceValue(rubyDefaultVersion)

func rubyDefaultVersion() string {
	if exeExists("ruby") && exeExists("gem") {
		return "system"
	}
	return "default"
}

// effectiveVersion resolves `default` through GetDefaultVersion, as upstream
// does before the hook reaches the backend; the env directory keeps the name
// it was given.
func (r *Ruby) effectiveVersion(version string) string {
	if version == "default" {
		return r.GetDefaultVersion()
	}
	return version
}

// HealthCheck is upstream's lang_base.basic_health_check: an installed
// environment is always healthy. A missing toolchain shows when the hook runs.
func (r *Ruby) HealthCheck(_, _ string) error { return nil }

// rubyUnset is the GEM_PATH upstream's get_env_patch removes.
var rubyUnset = []string{"GEM_PATH"}

// rubyEnvPatch is upstream's ruby.get_env_patch: gems install into and run
// from the env; a non-system ruby also runs through the env's rbenv, with
// RBENV_VERSION naming any version other than system or default.
func rubyEnvPatch(envDir, version string) []string {
	gems := filepath.Join(envDir, "gems")
	env := []string{
		"GEM_HOME=" + gems,
		"BUNDLE_IGNORE_CONFIG=1",
	}
	path := []string{filepath.Join(gems, "bin")}
	if version != "system" {
		env = append(env, "RBENV_ROOT="+envDir)
		path = append(path, filepath.Join(envDir, "shims"), filepath.Join(envDir, "bin"))
	}
	path = append(path, os.Getenv("PATH"))
	env = append(env, "PATH="+strings.Join(path, string(os.PathListSeparator)))
	if version != "system" && version != "default" {
		env = append(env, "RBENV_VERSION="+version)
	}
	return env
}

// extractRubyResource is upstream's _extract_resource.
func extractRubyResource(name, dest string) error {
	data, err := rubyResources.ReadFile("resources/" + name)
	if err != nil {
		return err
	}
	return extractTarGz(bytes.NewReader(data), dest)
}

// installRbenv is upstream's _install_rbenv: rbenv unpacked into the env,
// and ruby-build and ruby-download as its plugins only when a specific
// version must be installed.
func installRbenv(prefix, envDir, version string) error {
	if err := extractRubyResource("rbenv.tar.gz", prefix); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(prefix, "rbenv"), envDir); err != nil {
		return err
	}
	if version != "default" {
		plugins := filepath.Join(envDir, "plugins")
		for _, name := range []string{"ruby-download.tar.gz", "ruby-build.tar.gz"} {
			if err := extractRubyResource(name, plugins); err != nil {
				return err
			}
		}
	}
	return nil
}

// rubySetupCmd is upstream's lang_base.setup_cmd inside in_env: argv runs in
// prefix with the env patch applied, its executable resolved on the
// patched PATH.
func rubySetupCmd(prefix, envDir, version string, argv ...string) error {
	env := rubyEnvPatch(envDir, version)
	bin, err := lookPathInEnv(argv[0], env)
	if err != nil {
		bin = argv[0]
	}
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Dir = prefix
	cmd.Env = append(environWithout(os.Environ(), rubyUnset), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s failed: %s: %w", strings.Join(argv, " "), string(out), err)
	}
	return nil
}

// prefixStar is upstream's Prefix.star: the names (not paths) of the prefix's
// entries ending in suffix, sorted.
func prefixStar(prefix, suffix string) ([]string, error) {
	entries, err := os.ReadDir(prefix)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), suffix) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// InstallEnvironment is upstream's ruby.install_environment (4.7.0).
func (r *Ruby) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	version = r.effectiveVersion(version)

	if version != "system" {
		if err := installRbenv(prefix, envDir, version); err != nil {
			return err
		}
		// rbenv init sets up rbenv's directories before anything installs.
		if err := rubySetupCmd(prefix, envDir, version, "rbenv", "init", "-"); err != nil {
			return err
		}
		if version != "default" {
			// upstream's _install_ruby: download a prebuilt ruby, and build
			// one only when the download fails.
			if err := rubySetupCmd(prefix, envDir, version, "rbenv", "download", version); err != nil {
				if err := rubySetupCmd(prefix, envDir, version, "rbenv", "install", version); err != nil {
					return err
				}
			}
		}
		// rbenv rehash sets up the shims after installing.
		if err := rubySetupCmd(prefix, envDir, version, "rbenv", "rehash"); err != nil {
			return err
		}
	}

	gemspecs, err := prefixStar(prefix, ".gemspec")
	if err != nil {
		return err
	}
	build := append([]string{"gem", "build"}, gemspecs...)
	if buildErr := rubySetupCmd(prefix, envDir, version, build...); buildErr != nil {
		return buildErr
	}
	gems, err := prefixStar(prefix, ".gem")
	if err != nil {
		return err
	}
	install := []string{
		"gem", "install",
		"--no-document", "--no-format-executable",
		"--no-user-install",
		"--install-dir", filepath.Join(envDir, "gems"),
		"--bindir", filepath.Join(envDir, "gems", "bin"),
	}
	install = append(append(install, gems...), additionalDeps...)
	return rubySetupCmd(prefix, envDir, version, install...)
}

// Run implements Language.
func (r *Ruby) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	envDir := filepath.Join(prefix, r.EnvironmentDir()+"-"+version)
	argv := append(append(ParseEntry(entry), args...), fileArgs...)
	if len(argv) == 0 {
		return -1, nil, fmt.Errorf("empty entry")
	}
	return runHookArgv(ctx, workDir, argv, rubyEnvPatch(envDir, r.effectiveVersion(version)), rubyUnset)
}
