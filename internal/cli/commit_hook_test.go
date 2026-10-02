// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests drive the hook the way a user does: a real `git commit` that
// invokes this test binary (acting as the pre-commit binary, see TestMain) as
// the repo's pre-commit hook. Running `pre-commit run` directly cannot catch
// what they guard, because the bugs live in the environment git hands a hook:
//
//   - `git commit -a` and `git commit <paths>` hold the index lock for the
//     whole hook and point GIT_INDEX_FILE at the index the commit will
//     record. Stashing against the default index instead ran `git write-tree`
//     into that held lock ("Unable to create .../index.lock: File exists"),
//     so unstaged changes were never stashed.
//   - a hook that rewrites files must fail even when it is not passed
//     filenames (golangci-lint-fmt, `pass_filenames: false`), as upstream
//     decides it from `git diff` around the hook, not from the file list.

const commitHookConfig = `repos:
- repo: local
  hooks:
  - id: snapshot
    name: snapshot
    entry: sh -c 'cat b.txt > "$SNAPSHOT"; if [ -n "$MODIFY" ]; then echo appended >> a.txt; fi'
    language: system
    files: '\.txt$'
    pass_filenames: false
`

type commitRepo struct {
	t        *testing.T
	dir      string // working tree the commit runs in
	snapshot string // where the hook writes what it saw of b.txt
	env      []string
}

func (r *commitRepo) git(args ...string) (string, error) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *commitRepo) mustGit(args ...string) string {
	r.t.Helper()
	out, err := r.git(args...)
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (r *commitRepo) write(name, content string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *commitRepo) read(name string) string {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.dir, name))
	if err != nil {
		r.t.Fatal(err)
	}
	return string(b)
}

// newCommitRepo creates a repo with a.txt and b.txt committed and this test
// binary installed as its pre-commit hook. With worktree set, the commit runs
// in a linked worktree, whose index lives at .git/worktrees/<name>/index.
func newCommitRepo(t *testing.T, worktree bool) *commitRepo {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("drives a POSIX shell hook script")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	base := t.TempDir()
	mainDir := filepath.Join(base, "main")
	if err := os.Mkdir(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := &commitRepo{
		t:        t,
		dir:      mainDir,
		snapshot: filepath.Join(base, "snapshot"),
	}
	r.env = append(os.Environ(),
		actAsBinaryEnv+"=1",
		"PRE_COMMIT_HOME="+filepath.Join(base, "home"),
		"SNAPSHOT="+r.snapshot,
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
	)

	r.mustGit("init", "-q", "-b", "main")
	r.mustGit("config", "commit.gpgsign", "false")
	r.write(".pre-commit-config.yaml", commitHookConfig)
	r.write("a.txt", "a\n")
	r.write("b.txt", "b\n")
	r.mustGit("add", ".")
	r.mustGit("commit", "-q", "--no-verify", "-m", "init")

	hooks := filepath.Join(mainDir, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nexec '" + self + "' hook-impl --config=.pre-commit-config.yaml" +
		" --hook-type=pre-commit --hook-dir \"$(dirname \"$0\")\" -- \"$@\"\n"
	if err := os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	if worktree {
		wt := filepath.Join(base, "wt")
		r.mustGit("worktree", "add", "-q", wt, "-b", "wt")
		r.dir = wt
	}
	return r
}

func TestCommitHook_StashAndModificationDetection(t *testing.T) {
	modes := []struct {
		name string
		args []string
		// whether b.txt's unstaged change is part of the commit (commit -a).
		commitsB bool
	}{
		{name: "staged", args: []string{"commit", "-m", "test"}},
		{name: "all", args: []string{"commit", "-a", "-m", "test"}, commitsB: true},
		{name: "paths", args: []string{"commit", "-m", "test", "--", "a.txt"}},
	}

	for _, worktree := range []bool{false, true} {
		for _, mode := range modes {
			for _, modify := range []bool{false, true} {
				name := mode.name
				if worktree {
					name += "/worktree"
				}
				if modify {
					name += "/modifying-hook"
				}
				t.Run(name, func(t *testing.T) {
					r := newCommitRepo(t, worktree)
					if modify {
						r.env = append(r.env, "MODIFY=1")
					}
					r.write("a.txt", "a2\n")
					r.mustGit("add", "a.txt")
					r.write("b.txt", "b\nunstaged\n")

					out, err := r.git(mode.args...)

					if strings.Contains(out, "Failed to stash") || strings.Contains(out, "index.lock") {
						t.Errorf("stashing failed under git commit:\n%s", out)
					}

					// The hook sees only what is being committed.
					seen, readErr := os.ReadFile(r.snapshot)
					if readErr != nil {
						t.Fatalf("hook did not run: %v\n%s", readErr, out)
					}
					wantSeen := "b\n"
					if mode.commitsB {
						wantSeen = "b\nunstaged\n"
					}
					if string(seen) != wantSeen {
						t.Errorf("hook saw b.txt = %q, want %q\n%s", seen, wantSeen, out)
					}

					// Unstaged changes survive the run either way.
					if got := r.read("b.txt"); got != "b\nunstaged\n" {
						t.Errorf("b.txt after commit = %q, want the unstaged change restored", got)
					}

					if modify {
						if err == nil {
							t.Errorf("commit succeeded although the hook modified a.txt:\n%s", out)
						}
						if !strings.Contains(out, "Failed") || !strings.Contains(out, "- files were modified by this hook") {
							t.Errorf("hook not reported as modifying files:\n%s", out)
						}
						return
					}

					if err != nil {
						t.Fatalf("commit failed: %v\n%s", err, out)
					}
					wantB := "b\n"
					if mode.commitsB {
						wantB = "b\nunstaged\n"
					}
					if got := r.mustGit("show", "HEAD:b.txt"); got != wantB {
						t.Errorf("committed b.txt = %q, want %q", got, wantB)
					}
				})
			}
		}
	}
}
