// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installStep is one command in TestInstallMessages; prep runs first in the
// repo, and dir is relative to the repo root.
type installStep struct {
	name string
	dir  string
	args []string
	prep func(t *testing.T, repo string)
}

func writeHook(name, content string) func(*testing.T, string) {
	return func(t *testing.T, repo string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, ".git", "hooks", name), []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func gitIn(args ...string) func(*testing.T, string) {
	return func(t *testing.T, repo string) {
		t.Helper()
		if out, code := runCmd(t, repo, "git", args...); code != 0 {
			t.Fatalf("git %v: %s", args, out)
		}
	}
}

// TestInstallMessages compares what install and uninstall print, step by
// step, on stdout and stderr separately (#130). Upstream prints the hook path
// from git's common dir after changing to the root: relative in a plain
// checkout, absolute in a linked worktree; it refuses to install under
// core.hooksPath, and uninstall names the hook type rather than the path.
func TestInstallMessages(t *testing.T) {
	pyBin := pythonPreCommit(t)

	steps := []installStep{
		{name: "install", args: []string{"install"}},
		{name: "install from a subdirectory", dir: "sub", args: []string{"install"}},
		{name: "install -t pre-push", args: []string{"install", "-t", "pre-push"}},
		{
			name: "install over a foreign hook", args: []string{"install", "-t", "commit-msg"},
			prep: writeHook("commit-msg", "#!/bin/sh\necho legacy\n"),
		},
		{name: "install again in migration mode", args: []string{"install", "-t", "commit-msg"}},
		{name: "uninstall restores the legacy hook", args: []string{"uninstall", "-t", "commit-msg"}},
		{name: "uninstall a foreign hook", args: []string{"uninstall", "-t", "commit-msg"}},
		{name: "uninstall", dir: "sub", args: []string{"uninstall"}},
		{name: "uninstall a missing hook", args: []string{"uninstall", "-t", "post-merge"}},
		{
			name: "install -f drops the legacy hook", args: []string{"install", "-t", "post-merge", "-f"},
			prep: func(t *testing.T, repo string) {
				writeHook("post-merge", "#!/bin/sh\n")(t, repo)
				writeHook("post-merge.legacy", "old\n")(t, repo)
			},
		},
		{
			name: "install in a linked worktree", dir: "wt", args: []string{"install", "-t", "pre-merge-commit"},
			prep: gitIn("worktree", "add", "-q", "wt"),
		},
		{name: "uninstall in a linked worktree", dir: "wt", args: []string{"uninstall", "-t", "pre-merge-commit"}},
		{
			name: "install refuses core.hooksPath", args: []string{"install"},
			prep: gitIn("config", "core.hooksPath", "myhooks"),
		},
		{name: "uninstall ignores core.hooksPath", args: []string{"uninstall", "-t", "pre-push"}},
	}

	// Each side gets its own repo and runs every step in order, so a step
	// sees the state the steps before it left.
	newRepo := func() string {
		repo := initTestRepo(t, "repos: []\n", "x\n")
		if err := os.Mkdir(filepath.Join(repo, "sub"), 0o755); err != nil {
			t.Fatal(err)
		}
		gitIn("-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false",
			"commit", "-q", "--no-verify", "-m", "init")(t, repo)
		return repo
	}
	// git init must not copy in the machine's own template hooks.
	t.Setenv("GIT_TEMPLATE_DIR", t.TempDir())
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	pyRepo, goRepo := newRepo(), newRepo()

	norm := func(s, repo string) string {
		if real, err := filepath.EvalSymlinks(repo); err == nil {
			s = strings.ReplaceAll(s, real, "<REPO>")
		}
		return strings.ReplaceAll(s, repo, "<REPO>")
	}

	for _, st := range steps {
		var res [2][2]string
		var codes [2]int
		for i, side := range []struct{ repo, bin string }{{pyRepo, pyBin}, {goRepo, goBinary}} {
			if st.prep != nil {
				st.prep(t, side.repo)
			}
			out, errOut, code := runSplit(t, filepath.Join(side.repo, st.dir), side.bin, st.args...)
			res[i] = [2]string{norm(out, side.repo), norm(errOut, side.repo)}
			codes[i] = code
		}
		// Every step but the two silent uninstalls prints something; a step
		// where upstream printed nothing must be one of those.
		silent := strings.Contains(st.name, "foreign hook") && strings.HasPrefix(st.name, "uninstall") ||
			strings.Contains(st.name, "missing hook")
		exercised := (res[0][0] != "") != silent
		addOutputResult(st.args[0], "message: "+st.name,
			exercised && res[0] == res[1] && codes[0] == codes[1],
			fmt.Sprintf("exercised=%v py=%d go=%d\npy stdout:\n%s\npy stderr:\n%s\ngo stdout:\n%s\ngo stderr:\n%s",
				exercised, codes[0], codes[1], res[0][0], res[0][1], res[1][0], res[1][1]))
	}
}
