//go:build integration

// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// envDirNames lists the environment directories inside each clone in a
// store: <home>/<clone>/<env dir>-<version>.
func envDirNames(t *testing.T, home string) []string {
	t.Helper()
	clones, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range clones {
		if !c.IsDir() {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(home, c.Name()))
		if err != nil {
			continue
		}
		for _, e := range entries {
			for _, prefix := range []string{"golangenv-", "py_env-", "go_env-"} {
				if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
					names = append(names, e.Name())
				}
			}
		}
	}
	slices.Sort(names)
	return names
}

// TestEnvDirNames compares the environment directories each tool builds for
// the same hooks with `language_version` left at default and with
// default_language_version set: upstream resolves `default` before naming
// the directory, so a store's layout is the same under either tool
// (golangenv-system, py_env-python3.13, …).
func TestEnvDirNames(t *testing.T) {
	pyBin := pythonPreCommit(t)
	for _, tool := range []string{"go", "python3"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}

	hookRepo := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":   "module example.com/hook\n\ngo 1.21\n",
		"main.go":  "package main\n\nfunc main() {}\n",
		"setup.py": "from setuptools import setup\nsetup(name='hook', version='0.0.0', py_modules=[])\n",
		".pre-commit-hooks.yaml": "-   id: go-hook\n    name: go hook\n    entry: hook\n    language: golang\n" +
			"-   id: py-hook\n    name: py hook\n    entry: python -c pass\n    language: python\n",
	} {
		if err := os.WriteFile(filepath.Join(hookRepo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-qm", "hook"},
	} {
		if out, code := runCmd(t, hookRepo, "git", args...); code != 0 {
			t.Fatalf("git %v:\n%s", args, out)
		}
	}
	rev, _ := runCmd(t, hookRepo, "git", "rev-parse", "HEAD")
	rev = strings.TrimSpace(rev)

	for _, tc := range []struct{ name, extra string }{
		{"default versions", ""},
		{"default_language_version", "default_language_version:\n    python: python3\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fmt.Sprintf("%srepos:\n-   repo: %s\n    rev: %s\n    hooks:\n    -   id: go-hook\n    -   id: py-hook\n",
				tc.extra, hookRepo, rev)
			pyRepo := initTestRepo(t, cfg, "x\n")
			goRepo := initTestRepo(t, cfg, "x\n")
			pyHome, goHome := t.TempDir(), t.TempDir()
			t.Setenv("PRE_COMMIT_HOME", pyHome)
			pyOut, pyExit := runCmd(t, pyRepo, pyBin, "install-hooks")
			t.Setenv("PRE_COMMIT_HOME", goHome)
			goOut, goExit := runCmd(t, goRepo, goBinary, "install-hooks")

			addExitResult("install-hooks", "env dirs: "+tc.name, pyExit, goExit,
				pyExit == 0 && goExit == 0, fmt.Sprintf("py:\n%s\ngo:\n%s", pyOut, goOut))
			pyNames, goNames := envDirNames(t, pyHome), envDirNames(t, goHome)
			addFSResult("install-hooks", "env dir names: "+tc.name,
				len(pyNames) == 2 && slices.Equal(pyNames, goNames),
				fmt.Sprintf("py: %v\ngo: %v", pyNames, goNames))
		})
	}
}
