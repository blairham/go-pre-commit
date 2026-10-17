// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// upstream's ruby.get_env_patch for each kind of version.
func TestRubyEnvPatch(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	sep := string(os.PathListSeparator)
	env := "/p/rbenv-x"
	gems := filepath.Join(env, "gems")
	assertSliceEqual(t, rubyEnvPatch(env, "system"), []string{
		"GEM_HOME=" + gems,
		"BUNDLE_IGNORE_CONFIG=1",
		"PATH=" + filepath.Join(gems, "bin") + sep + "/usr/bin",
	})
	rbenvPath := "PATH=" + strings.Join([]string{
		filepath.Join(gems, "bin"), filepath.Join(env, "shims"), filepath.Join(env, "bin"), "/usr/bin",
	}, sep)
	assertSliceEqual(t, rubyEnvPatch(env, "default"), []string{
		"GEM_HOME=" + gems,
		"BUNDLE_IGNORE_CONFIG=1",
		"RBENV_ROOT=" + env,
		rbenvPath,
	})
	assertSliceEqual(t, rubyEnvPatch(env, "3.2.0"), []string{
		"GEM_HOME=" + gems,
		"BUNDLE_IGNORE_CONFIG=1",
		"RBENV_ROOT=" + env,
		rbenvPath,
		"RBENV_VERSION=3.2.0",
	})
}

// fakeGem puts a gem on PATH that records each call's arguments and the
// rbenv/gem environment it saw, and whose `build` writes a .gem.
func fakeGem(t *testing.T) (record string) {
	t.Helper()
	dir := t.TempDir()
	record = filepath.Join(t.TempDir(), "gem.calls")
	script := "#!/bin/sh\n" +
		"echo \"$*|RBENV_ROOT=$RBENV_ROOT|GEM_PATH=${GEM_PATH-unset}|GEM_HOME=$GEM_HOME\" >> " + record + "\n" +
		"[ \"$1\" = build ] && : > hook-0.gem\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "gem"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return record
}

func rubyPrefix(t *testing.T) string {
	t.Helper()
	prefix := t.TempDir()
	for _, name := range []string{"b.gemspec", "a.gemspec"} {
		if err := os.WriteFile(filepath.Join(prefix, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return prefix
}

// The gem build and install are upstream's: every gemspec, then every .gem
// plus the additional dependencies, into the env's gems with its own bindir.
func TestRubyInstallSystem(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake gem is a sh script")
	}
	t.Setenv("GEM_PATH", "/somewhere")
	record := fakeGem(t)
	prefix := rubyPrefix(t)
	if err := (&Ruby{}).InstallEnvironment(prefix, "system", []string{"rake:13"}); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	envDir := filepath.Join(prefix, "rbenv-system")
	gems := filepath.Join(envDir, "gems")
	assertSliceEqual(t, strings.Split(strings.TrimSpace(string(raw)), "\n"), []string{
		"build a.gemspec b.gemspec|RBENV_ROOT=|GEM_PATH=unset|GEM_HOME=" + gems,
		"install --no-document --no-format-executable --no-user-install --install-dir " + gems +
			" --bindir " + filepath.Join(gems, "bin") + " hook-0.gem rake:13|RBENV_ROOT=|GEM_PATH=unset|GEM_HOME=" + gems,
	})
	if _, err := os.Stat(filepath.Join(envDir, "bin", "rbenv")); !os.IsNotExist(err) {
		t.Errorf("a system ruby env should not get an rbenv (%v)", err)
	}
}

// Without a system ruby, `default` builds upstream's vendored rbenv into the
// env (without ruby-build: no version to install) and runs the gems through
// it.
func TestRubyInstallDefaultUsesRbenv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rbenv is bash")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	t.Setenv("HOME", t.TempDir())
	withDefaultVersion(t, &rubyDefaultVersionOnce, "default")
	record := fakeGem(t)
	prefix := rubyPrefix(t)
	if err := (&Ruby{}).InstallEnvironment(prefix, "default", nil); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	envDir := filepath.Join(prefix, "rbenv-default")
	if _, err := os.Stat(filepath.Join(envDir, "libexec", "rbenv")); err != nil {
		t.Errorf("rbenv not unpacked into the env: %v", err)
	}
	if _, err := os.Stat(filepath.Join(envDir, "plugins")); !os.IsNotExist(err) {
		t.Errorf("ruby-build is only for a specific version (%v)", err)
	}
	if _, err := os.Stat(filepath.Join(prefix, "rbenv")); !os.IsNotExist(err) {
		t.Errorf("the unpacked rbenv should be moved into the env (%v)", err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "|RBENV_ROOT="+envDir+"|") {
		t.Errorf("gem should run under the env's rbenv, saw: %s", raw)
	}
}

// A specific version also gets the ruby-build and ruby-download plugins.
func TestInstallRbenvPlugins(t *testing.T) {
	prefix := t.TempDir()
	envDir := filepath.Join(prefix, "rbenv-3.2.0")
	if err := installRbenv(prefix, envDir, "3.2.0"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"plugins/ruby-build/bin/ruby-build", "plugins/ruby-download/bin/rbenv-download"} {
		if _, err := os.Stat(filepath.Join(envDir, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}
