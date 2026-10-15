// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func mkNodePrefix(t *testing.T, withGit, withPackageJSON bool) string {
	t.Helper()
	prefix := t.TempDir()
	if withGit {
		if err := os.Mkdir(filepath.Join(prefix, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if withPackageJSON {
		if err := os.WriteFile(
			filepath.Join(prefix, "package.json"),
			[]byte(`{"name":"h","version":"1.0.0"}`),
			0o644,
		); err != nil {
			t.Fatal(err)
		}
	}
	return prefix
}

func TestNodeInstallPackages(t *testing.T) {
	cases := []struct {
		name          string
		deps          []string
		wantDepsAfter []string
		git           bool
		pkgJSON       bool
		wantSelf      bool
		wantErr       bool
	}{
		{name: "git repo with package.json", git: true, pkgJSON: true, wantSelf: true},
		{
			name:          "git repo with package.json and deps",
			git:           true,
			pkgJSON:       true,
			deps:          []string{"lodash@4"},
			wantSelf:      true,
			wantDepsAfter: []string{"lodash@4"},
		},
		{name: "package.json without .git", pkgJSON: true, deps: []string{"lodash@4"}, wantDepsAfter: []string{"lodash@4"}},
		{name: ".git without package.json", git: true, deps: []string{"lodash@4"}, wantDepsAfter: []string{"lodash@4"}},
		{name: "local hook, deps only", deps: []string{"a", "b"}, wantDepsAfter: []string{"a", "b"}},
		{name: "package.json without .git and no deps", pkgJSON: true, wantErr: true},
		{name: "nothing", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefix := mkNodePrefix(t, tc.git, tc.pkgJSON)
			got, err := nodeInstallPackages(prefix, tc.deps)
			if tc.wantErr {
				if err == nil || err.Error() != "`language: node` must have package.json or additional_dependencies" {
					t.Fatalf("want upstream's error, got %v (pkgs %q)", err, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var want []string
			if tc.wantSelf {
				want = append(want, "git+file://"+prefix)
			}
			want = append(want, tc.wantDepsAfter...)
			assertSliceEqual(t, got, want)
		})
	}
}

func TestNodeInstallArgsMatchUpstream(t *testing.T) {
	got := nodeInstallArgs([]string{"git+file:///p", "dep"})
	assertSliceEqual(t, got, []string{"install", "--allow-git=root", "--install-links", "-g", "git+file:///p", "dep"})
}

// End to end through InstallEnvironment with fake nodeenv and npm on PATH:
// npm must get upstream's exact argv, run in the prefix, see the env's npm
// prefix, and not inherit the host's repository-pinning GIT_* variables
// (upstream 4.7.0, #3774) while keeping the allowed ones.
func TestNodeInstallEnvironmentRunsNpmWithoutGitEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tools are sh scripts")
	}
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "npm.log")
	writeScript := func(name, body string) {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeScript("nodeenv", "exit 0\n")
	writeScript("npm", `{
  echo "cwd=$(pwd -P)"
  for a in "$@"; do echo "arg=$a"; done
  env | grep -E '^(GIT_|NPM_CONFIG_PREFIX=)' | sed 's/^/env=/'
} > "`+record+`"
`)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_INDEX_FILE", "/host/.git/index")
	t.Setenv("GIT_DIR", "/host/.git")
	t.Setenv("GIT_WORK_TREE", "/host")
	t.Setenv("GIT_SSH_COMMAND", "ssh -o Kept=yes")

	prefix := mkNodePrefix(t, true, true)
	n := &Node{}
	if err := n.InstallEnvironment(prefix, "default", []string{"extra@1"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")

	var args, envs []string
	var cwd string
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "arg="):
			args = append(args, strings.TrimPrefix(l, "arg="))
		case strings.HasPrefix(l, "env="):
			envs = append(envs, strings.TrimPrefix(l, "env="))
		case strings.HasPrefix(l, "cwd="):
			cwd = strings.TrimPrefix(l, "cwd=")
		}
	}
	assertSliceEqual(
		t,
		args,
		[]string{"install", "--allow-git=root", "--install-links", "-g", "git+file://" + prefix, "extra@1"},
	)
	if realPrefix, _ := filepath.EvalSymlinks(prefix); cwd != realPrefix {
		t.Errorf("npm ran in %q, want the prefix %q", cwd, realPrefix)
	}
	joined := strings.Join(envs, "\n")
	for _, leaked := range []string{"GIT_INDEX_FILE=", "GIT_DIR=", "GIT_WORK_TREE="} {
		if strings.Contains(joined, leaked) {
			t.Errorf("npm inherited %s from the host:\n%s", leaked, joined)
		}
	}
	if !strings.Contains(joined, "GIT_SSH_COMMAND=ssh -o Kept=yes") {
		t.Errorf("npm lost the allowed GIT_SSH_COMMAND:\n%s", joined)
	}
	if !strings.Contains(joined, "NPM_CONFIG_PREFIX="+filepath.Join(prefix, "node_env-default")) {
		t.Errorf("npm did not get the env's prefix:\n%s", joined)
	}
}

// No package to install and nothing to add fails before nodeenv runs, as
// upstream asserts before creating the environment.
func TestNodeInstallEnvironmentNothingToInstall(t *testing.T) {
	prefix := mkNodePrefix(t, false, false)
	t.Setenv("PATH", t.TempDir()) // no nodeenv: reaching it would be a different error
	err := (&Node{}).InstallEnvironment(prefix, "default", nil)
	if err == nil || !strings.Contains(err.Error(), "must have package.json or additional_dependencies") {
		t.Fatalf("got %v", err)
	}
}

// When nodeenv installs its own npm (any version but system), that npm runs
// the install, not whichever npm the host has first on PATH: upstream runs
// npm inside in_env, whose PATH starts at the env's bin dir.
func TestNodeInstallEnvironmentPrefersEnvNpm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tools are sh scripts")
	}
	bin := t.TempDir()
	record := filepath.Join(t.TempDir(), "which")
	write := func(path, body string) {
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// nodeenv's third argument is the env dir; give it an npm of its own.
	write(
		filepath.Join(bin, "nodeenv"),
		`mkdir -p "$3/bin" && printf '#!/bin/sh\necho env > `+record+`\n' > "$3/bin/npm" && chmod +x "$3/bin/npm"`+"\n",
	)
	write(filepath.Join(bin, "npm"), "echo host > "+record+"\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	prefix := mkNodePrefix(t, true, true)
	if err := (&Node{}).InstallEnvironment(prefix, "18.0.0", nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "env" {
		t.Errorf("install ran the %s npm, want the env's", strings.TrimSpace(string(got)))
	}
}
