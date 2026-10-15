// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLegacyHookDir(t *testing.T) {
	t.Setenv("GIT_DIR", "")

	if got, want := legacyHookDir("/custom/hooks"), "/custom/hooks"; got != want {
		t.Errorf("legacyHookDir with --hook-dir = %q, want %q", got, want)
	}
	if got, want := legacyHookDir(""), filepath.Join(".git", "hooks"); got != want {
		t.Errorf("legacyHookDir without GIT_DIR = %q, want %q", got, want)
	}

	t.Setenv("GIT_DIR", "/repo/.git")
	if got, want := legacyHookDir(""), filepath.Join("/repo/.git", "hooks"); got != want {
		t.Errorf("legacyHookDir with GIT_DIR = %q, want %q", got, want)
	}
	// --hook-dir wins over GIT_DIR, matching Python pre-commit's hook-impl.
	if got, want := legacyHookDir("/custom/hooks"), "/custom/hooks"; got != want {
		t.Errorf("legacyHookDir precedence = %q, want %q", got, want)
	}
}

// prePushRepo builds the repo the pre-push cases share: main (a.txt) pushed to
// origin, then an unpushed feat branch adding b.txt on top of it. It also adds
// a remote "empty" that has nothing, so every commit is new to it.
func prePushRepo(t *testing.T) (dir, mainSHA, featSHA string) {
	t.Helper()
	base := t.TempDir()
	dir = filepath.Join(base, "repo")
	run := func(cwd string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = cwd
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run(base, "init", "-q", "--bare", "origin.git")
	run(base, "init", "-q", "--bare", "empty.git")
	run(base, "init", "-q", "-b", "main", "repo")
	run(dir, "remote", "add", "origin", "../origin.git")
	run(dir, "remote", "add", "empty", "../empty.git")
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt")
	run(dir, "add", "a.txt")
	run(dir, "commit", "-q", "-m", "a")
	run(dir, "push", "-q", "origin", "main")
	mainSHA = run(dir, "rev-parse", "HEAD")
	run(dir, "checkout", "-q", "-b", "feat")
	write("b.txt")
	run(dir, "add", "b.txt")
	run(dir, "commit", "-q", "-m", "b")
	featSHA = run(dir, "rev-parse", "HEAD")
	return dir, mainSHA, featSHA
}

// The rows of upstream's _pre_push_ns, one case per branch it can take.
func TestPrePushArgs(t *testing.T) {
	dir, mainSHA, featSHA := prePushRepo(t)
	const unknown = "1234567890abcdef1234567890abcdef12345678"
	tail := func(remote, remoteBranch, localBranch string) []string {
		return []string{
			"--remote-name", remote, "--remote-url", "../" + remote + ".git",
			"--remote-branch", remoteBranch, "--local-branch", localBranch,
		}
	}
	fromTo := func(from, to string, rest []string) []string {
		return append([]string{"--from-ref", from, "--to-ref", to}, rest...)
	}

	cases := []struct {
		name   string
		remote string
		stdin  string
		want   []string // nil: nothing runs
	}{
		{
			name:   "remote sha known",
			remote: "origin",
			stdin:  "refs/heads/feat " + featSHA + " refs/heads/main " + mainSHA + "\n",
			want:   fromTo(mainSHA, featSHA, tail("origin", "refs/heads/main", "refs/heads/feat")),
		},
		{
			// Checks the pushed commits, not whatever happens to be staged.
			name:   "new branch walks the commits the remote lacks",
			remote: "origin",
			stdin:  "refs/heads/feat " + featSHA + " refs/heads/feat " + z40 + "\n",
			want:   fromTo(mainSHA, featSHA, tail("origin", "refs/heads/feat", "refs/heads/feat")),
		},
		{
			name:   "deletion runs nothing",
			remote: "origin",
			stdin:  "(delete) " + z40 + " refs/heads/feat " + featSHA + "\n",
		},
		{
			// A remote tip we never fetched must not reach `git diff`, which
			// failed the push; upstream falls back to the ancestor walk.
			name:   "unknown remote sha falls back to the ancestor walk",
			remote: "origin",
			stdin:  "refs/heads/feat " + featSHA + " refs/heads/feat " + unknown + "\n",
			want:   fromTo(mainSHA, featSHA, tail("origin", "refs/heads/feat", "refs/heads/feat")),
		},
		{
			name:   "commits the remote already has run nothing",
			remote: "origin",
			stdin:  "refs/heads/main " + mainSHA + " refs/heads/other " + z40 + "\n",
		},
		{
			name:   "root commit among the new ones means all files",
			remote: "empty",
			stdin:  "refs/heads/feat " + featSHA + " refs/heads/feat " + z40 + "\n",
			want:   append([]string{"--all-files"}, tail("empty", "refs/heads/feat", "refs/heads/feat")...),
		},
		{
			name:   "first yielding line wins",
			remote: "origin",
			stdin: "(delete) " + z40 + " refs/heads/x " + featSHA + "\n" +
				"refs/heads/main " + mainSHA + " refs/heads/other " + z40 + "\n" +
				"refs/heads/feat " + featSHA + " refs/heads/feat " + z40 + "\n" +
				"refs/heads/feat " + featSHA + " refs/heads/main " + mainSHA + "\n",
			want: fromTo(mainSHA, featSHA, tail("origin", "refs/heads/feat", "refs/heads/feat")),
		},
		{
			name:   "empty stdin runs nothing",
			remote: "origin",
			stdin:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := prePushArgs(dir, tc.remote, "../"+tc.remote+".git", []byte(tc.stdin))
			if err != nil {
				t.Fatalf("prePushArgs: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("prePushArgs =\n  %q\nwant\n  %q", got, tc.want)
			}
		})
	}
}
