// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// autoupdate and try-repo used to run their own `git clone`, whose progress,
// warnings and detached-HEAD advice reached the terminal (#134). These rows
// compare the whole output, which the older rows (exit codes, a keyword, the
// generated config) could not see.

// localHooksRepo is a hook repository with v1.0.0 on its first commit,
// v1.1.0 and the moving tag "latest" on its second, and an untagged third.
func localHooksRepo(t *testing.T) (dir string, shas []string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		full := append(
			[]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"},
			args...,
		)
		cmd := exec.Command("git", full...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	manifest := "-   id: no-todo\n    name: no TODO\n    entry: TODO\n    language: pygrep\n"
	if err := os.WriteFile(filepath.Join(dir, ".pre-commit-hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "-A")
		git("commit", "-qm", name)
		shas = append(shas, git("rev-parse", "HEAD"))
		switch i {
		case 0:
			git("tag", "v1.0.0")
		case 1:
			git("tag", "latest")
			git("tag", "v1.1.0")
		}
	}
	return dir, shas
}

// sameTempDir points both tools' temporary directories at one place and
// returns a function that replaces each throwaway directory under it with
// TMP, so their random names do not count as a difference.
func sameTempDir(t *testing.T) func(string) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	re := regexp.MustCompile(regexp.QuoteMeta(tmp) + `/tmp[A-Za-z0-9_]+`)
	return func(s string) string { return re.ReplaceAllString(s, "TMP") }
}

func TestAutoupdateFullOutput(t *testing.T) {
	pyBin := pythonPreCommit(t)
	hooks, shas := localHooksRepo(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	norm := sameTempDir(t)

	const tmpl = "repos:\n-   repo: %s\n    rev: %s\n    hooks:\n    -   id: %s\n"
	cases := []struct {
		args                  []string
		name, repo, rev, hook string
	}{
		{name: "local repo, comment kept", repo: hooks, rev: "v1.0.0  # keep", hook: "no-todo"},
		{name: "local repo, --freeze", repo: hooks, rev: "v1.0.0", hook: "no-todo", args: []string{"--freeze"}},
		{
			name: "local repo, --bleeding-edge drops frozen", repo: hooks,
			rev: shas[1] + "  # frozen: v1.1.0", hook: "no-todo", args: []string{"--bleeding-edge"},
		},
		{name: "local repo, already up to date", repo: hooks, rev: "v1.1.0", hook: "no-todo"},
		{name: "local repo, hook missing at new rev", repo: hooks, rev: "v1.0.0", hook: "gone"},
		{name: "unreachable repo", repo: filepath.Join(hooks, "nonexistent"), rev: "v1.0.0", hook: "x"},
		{
			name: "remote repo", repo: "https://github.com/pre-commit/pre-commit-hooks",
			rev: "v4.0.0", hook: "check-yaml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fmt.Sprintf(tmpl, tc.repo, tc.rev, tc.hook)
			pyRepo := initTestRepo(t, cfg, "")
			goRepo := initTestRepo(t, cfg, "")
			args := append([]string{"autoupdate", "--color=never"}, tc.args...)

			pyOut, pyErr, pyExit := runSplit(t, pyRepo, pyBin, args...)
			goOut, goErr, goExit := runSplit(t, goRepo, goBinary, args...)
			addExitResult("autoupdate", tc.name+": exit", pyExit, goExit, pyExit == goExit, "")
			pyOut, goOut = norm(pyOut), norm(goOut)
			addOutputResult("autoupdate", tc.name+": full output",
				pyOut == goOut && pyErr == goErr,
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))

			pyCfg := readFile(filepath.Join(pyRepo, ".pre-commit-config.yaml"))
			goCfg := readFile(filepath.Join(goRepo, ".pre-commit-config.yaml"))
			addFSResult("autoupdate", tc.name+": same config written", pyCfg == goCfg,
				fmt.Sprintf("py=%q go=%q", pyCfg, goCfg))
		})
	}
}

var shaRe = regexp.MustCompile(`\b[0-9a-f]{40}\b`)

func TestTryRepoFullOutput(t *testing.T) {
	pyBin := pythonPreCommit(t)
	norm := sameTempDir(t)

	// Every run here passes: a failing pygrep line differs on its own (#127),
	// and that is not what these rows are about.
	cases := []struct {
		name  string
		args  []string
		dirty bool
	}{
		{name: "local repo, named hook", args: []string{"no-todo"}},
		{name: "local repo, every hook"},
		{name: "local repo, --ref", args: []string{"no-todo", "--ref", "v1.0.0"}},
		{name: "local repo with uncommitted changes", dirty: true},
		{name: "nonexistent repo"},
		{name: "remote repo, named hook"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PRE_COMMIT_HOME", t.TempDir())
			hooks, _ := localHooksRepo(t)
			repo := hooks
			switch tc.name {
			case "nonexistent repo":
				repo = filepath.Join(hooks, "nonexistent")
			case "remote repo, named hook":
				repo = "https://github.com/pre-commit/pre-commit-hooks"
				tc.args = []string{"check-yaml", "--ref", "v5.0.0"}
			}
			if tc.dirty {
				// A second hook, staged and edited, and an untracked file
				// upstream leaves out of its shadow repository.
				extra := "-   id: no-fixme\n    name: no FIXME\n    entry: FIXME\n    language: pygrep\n"
				f, err := os.OpenFile(filepath.Join(hooks, ".pre-commit-hooks.yaml"), os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = f.WriteString(extra)
				f.Close()
				if err := os.WriteFile(filepath.Join(hooks, "untracked"), []byte("u"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			pyRepo := initTestRepoFile(t, "t.yaml", "a: 1\n")
			goRepo := initTestRepoFile(t, "t.yaml", "a: 1\n")
			args := append(append([]string{"try-repo", repo}, tc.args...), "--all-files", "--color=never")
			pyOut, pyErr, pyExit := runSplit(t, pyRepo, pyBin, args...)
			goOut, goErr, goExit := runSplit(t, goRepo, goBinary, args...)
			addExitResult("try-repo", tc.name+": exit", pyExit, goExit, pyExit == goExit, "")
			pyOut, goOut = norm(pyOut), norm(goOut)
			if tc.dirty {
				// The shadow commit's sha depends on the second it was made.
				pyOut, goOut = shaRe.ReplaceAllString(pyOut, "SHA"), shaRe.ReplaceAllString(goOut, "SHA")
			}
			addOutputResult("try-repo", tc.name+": full output",
				pyOut != "" && pyOut == goOut && pyErr == goErr,
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		})
	}
}
