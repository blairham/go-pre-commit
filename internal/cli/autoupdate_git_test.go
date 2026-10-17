// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitT runs git in dir with a fixed identity, failing the test on error.
func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append(
		[]string{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"},
		args...,
	)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// captureOutput runs fn with stdout and stderr redirected and returns both.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	done := make(chan [2]string)
	go func() {
		o, _ := io.ReadAll(outR)
		e, _ := io.ReadAll(errR)
		done <- [2]string{string(o), string(e)}
	}()
	fn()
	os.Stdout, os.Stderr = origOut, origErr
	outW.Close()
	errW.Close()
	res := <-done
	return res[0], res[1]
}

// hooksRepo is a hook repository with v1.0.0 on its first commit, v1.1.0 and
// the moving tag "latest" on its second, and an untagged third commit.
// Returns its path and the sha of each commit.
func hooksRepo(t *testing.T) (dir string, shas []string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "init", "-q")
	manifest := "-   id: no-todo\n    name: no TODO\n    entry: TODO\n    language: pygrep\n"
	if err := os.WriteFile(filepath.Join(dir, ".pre-commit-hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
		gitT(t, dir, "add", "-A")
		gitT(t, dir, "commit", "-qm", name)
		shas = append(shas, gitT(t, dir, "rev-parse", "HEAD"))
	}
	commit("a")
	gitT(t, dir, "tag", "v1.0.0")
	commit("b")
	// "latest" sorts before "v1.1.0", so it is what tag --points-at lists
	// first; upstream picks the tag with a '.' in it.
	gitT(t, dir, "tag", "latest")
	gitT(t, dir, "tag", "v1.1.0")
	commit("c")
	return dir, shas
}

// Expectations are pre-commit 4.7.0's RevInfo.update against the same repo.
func TestUpdateRevInfo(t *testing.T) {
	dir, shas := hooksRepo(t)

	cases := []struct {
		name     string
		rev      string
		frozen   string
		tagsOnly bool
		freeze   bool
	}{
		{name: "newest tag, the one that looks like a version", tagsOnly: true, rev: "v1.1.0"},
		{name: "frozen to the tag's sha", tagsOnly: true, freeze: true, rev: shas[1], frozen: "v1.1.0"},
		{name: "bleeding edge, HEAD untagged", rev: shas[2]},
		{name: "bleeding edge frozen, already a sha", freeze: true, rev: shas[2]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := updateRevInfo(dir, tc.tagsOnly, tc.freeze)
			if err != nil {
				t.Fatal(err)
			}
			if info.rev != tc.rev || info.frozen != tc.frozen {
				t.Errorf("got rev=%q frozen=%q, want rev=%q frozen=%q", info.rev, info.frozen, tc.rev, tc.frozen)
			}
			if !info.hookIDs["no-todo"] || len(info.hookIDs) != 1 {
				t.Errorf("hookIDs = %v, want just no-todo", info.hookIDs)
			}
		})
	}

	t.Run("bleeding edge takes a tag exactly on HEAD", func(t *testing.T) {
		gitT(t, dir, "tag", "v2.0.0")
		info, err := updateRevInfo(dir, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if info.rev != "v2.0.0" {
			t.Errorf("rev = %q, want v2.0.0", info.rev)
		}
	})
}

// autoupdate prints upstream's lines and nothing of git's: before the port,
// `git clone` wrote "Cloning into ..." and its warnings to the terminal.
// Output captured from pre-commit 4.7.0.
func TestAutoupdateOutputMatchesUpstream(t *testing.T) {
	hooks, shas := hooksRepo(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())

	cases := []struct {
		name   string
		rev    string
		hook   string
		flags  string
		stdout string
		rev2   string
		code   int
	}{
		{
			name: "update keeps an unrelated comment", rev: "v1.0.0  # keep", hook: "no-todo",
			stdout: "[" + hooks + "] updating v1.0.0 -> v1.1.0\n", rev2: "v1.1.0  # keep",
		},
		{
			name: "freeze", rev: "v1.0.0", hook: "no-todo", flags: "--freeze",
			stdout: "[" + hooks + "] updating v1.0.0 -> v1.1.0 (frozen)\n", rev2: shas[1] + "  # frozen: v1.1.0",
		},
		{
			name: "bleeding edge drops a frozen comment", rev: shas[1] + "  # frozen: v1.1.0", hook: "no-todo",
			flags:  "--bleeding-edge",
			stdout: "[" + hooks + "] updating " + shas[1] + " -> " + shas[2] + "\n", rev2: shas[2],
		},
		{
			name: "already up to date", rev: "v1.1.0", hook: "no-todo",
			stdout: "[" + hooks + "] already up to date!\n", rev2: "v1.1.0",
		},
		{
			name: "hook missing at the new rev", rev: "v1.0.0", hook: "gone", code: 1,
			stdout: "[" + hooks + "] Cannot update because the update target is missing these hooks: gone\n",
			rev2:   "v1.0.0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tempRepo(t)
			t.Chdir(root)
			const tmpl = "repos:\n-   repo: %s\n    rev: %s\n    hooks:\n    -   id: %s\n"
			cfg := filepath.Join(root, ".pre-commit-config.yaml")
			write := func(rev string) string { return fmt.Sprintf(tmpl, hooks, rev, tc.hook) }
			if err := os.WriteFile(cfg, []byte(write(tc.rev)), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"--color=never"}
			if tc.flags != "" {
				args = append(args, tc.flags)
			}
			var code int
			stdout, stderr := captureOutput(t, func() { code = (&AutoupdateCommand{}).Run(args) })
			if code != tc.code {
				t.Errorf("exit = %d, want %d", code, tc.code)
			}
			if stdout != tc.stdout || stderr != "" {
				t.Errorf("stdout = %q, stderr = %q; want stdout %q and no stderr", stdout, stderr, tc.stdout)
			}
			got, _ := os.ReadFile(cfg)
			if want := write(tc.rev2); string(got) != want {
				t.Errorf("config:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// A repo that cannot be fetched is upstream's uncaught CalledProcessError:
// exit 3, git's explanation inside the error, the config left alone.
func TestAutoupdateFetchFailureIsUnexpectedError(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	root := tempRepo(t)
	t.Chdir(root)
	missing := filepath.Join(root, "nonexistent")
	cfg := "repos:\n-   repo: " + missing + "\n    rev: v1.0.0\n    hooks:\n    -   id: x\n"
	if err := os.WriteFile(".pre-commit-config.yaml", []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	var code int
	stdout, stderr := captureOutput(t, func() { code = (&AutoupdateCommand{}).Run([]string{"--color=never"}) })
	if code != 3 {
		t.Errorf("exit = %d, want 3", code)
	}
	if !strings.HasPrefix(stdout, "An unexpected error has occurred: CalledProcessError: command: (") ||
		!strings.Contains(
			stdout,
			"'fetch', 'origin', 'HEAD', '--quiet', '--filter=blob:none', '--tags')\nreturn code: 128\n",
		) ||
		!strings.Contains(stdout, "\nCheck the log at ") {
		t.Errorf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want none", stderr)
	}
	if got, _ := os.ReadFile(".pre-commit-config.yaml"); string(got) != cfg {
		t.Errorf("config was rewritten:\n%s", got)
	}
}

func TestRewriteRevLines(t *testing.T) {
	const cfg = "repos:\n" +
		"-   repo: a\n    rev: v1  # keep me\n" +
		"-   repo: b\n    rev: 'v1'\n" +
		"-   repo: c\n    rev:   \"abc\"  # frozen: v1\r\n" +
		"-   repo: d\n    rev: v9\n"
	infos := []*revInfo{
		{rev: "v2"},
		{rev: "v2"},
		{rev: "v3"},
		nil,
	}
	got, ok := rewriteRevLines(cfg, infos)
	if !ok {
		t.Fatal("rev lines not found")
	}
	want := "repos:\n" +
		"-   repo: a\n    rev: v2  # keep me\n" +
		"-   repo: b\n    rev: 'v2'\n" +
		// REV_LINE_RE's greedy (.*) takes the \r into the comment, so
		// dropping a frozen comment drops it too; upstream does the same.
		"-   repo: c\n    rev:   \"v3\"\n" +
		"-   repo: d\n    rev: v9\n"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}

	frozen, _ := rewriteRevLines("    rev: v1  # keep\n", []*revInfo{{rev: "deadbeef", frozen: "v2"}})
	if frozen != "    rev: deadbeef  # frozen: v2\n" {
		t.Errorf("frozen = %q", frozen)
	}

	if _, ok := rewriteRevLines("repos: [{repo: a, rev: v1}]\n", []*revInfo{{rev: "v2"}}); ok {
		t.Error("flow-style config: rev lines reported found")
	}
}

// PyYAML plain-dumps a string only when it reads back as that string.
func TestYAMLScalar(t *testing.T) {
	cases := []struct{ s, style, want string }{
		{"v1.2.3", "", "v1.2.3"},
		{"1.0", "", "'1.0'"},
		{"123", "", "'123'"},
		{"true", "", "'true'"},
		{"v1", "'", "'v1'"},
		{"it's", "'", "'it''s'"},
		{"v1", `"`, `"v1"`},
	}
	for _, tc := range cases {
		if got := yamlScalar(tc.s, tc.style); got != tc.want {
			t.Errorf("yamlScalar(%q, %q) = %q, want %q", tc.s, tc.style, got, tc.want)
		}
	}
}

// Upstream's autoupdate runs migrate_config first, and a config whose rev
// lines it cannot find is re-dumped by PyYAML. "has been migrated" is a
// print(), so into a pipe it follows the update lines. Expected output and
// files captured from pre-commit 4.7.0.
func TestAutoupdateMigratesFirst(t *testing.T) {
	hooks, _ := hooksRepo(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())

	for _, tc := range []struct{ name, in, want string }{
		{
			name: "list config with sha",
			in:   "-   repo: %s\n    sha: v1.0.0\n    hooks:\n    -   id: no-todo\n",
			want: "repos:\n-   repo: %s\n    rev: v1.1.0\n    hooks:\n    -   id: no-todo\n",
		},
		{
			name: "flow config reformatted",
			in:   "repos: [{repo: %s, rev: v1.0.0, hooks: [{id: no-todo, args: [--x, 'y z', 'yes'], stages: [commit]}]}]\n",
			want: "repos:\n-   repo: %s\n    rev: v1.1.0\n    hooks:\n    -   id: no-todo\n        args:\n" +
				"        - --x\n        - y z\n        - 'yes'\n        stages:\n        - pre-commit\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tempRepo(t)
			t.Chdir(root)
			cfg := filepath.Join(root, ".pre-commit-config.yaml")
			if err := os.WriteFile(cfg, []byte(fmt.Sprintf(tc.in, hooks)), 0o644); err != nil {
				t.Fatal(err)
			}
			var code int
			stdout, stderr := captureOutput(t, func() { code = (&AutoupdateCommand{}).Run([]string{"--color=never"}) })
			if code != 0 {
				t.Errorf("exit = %d, want 0", code)
			}
			wantOut := "[" + hooks + "] updating v1.0.0 -> v1.1.0\nConfiguration has been migrated.\n"
			if stdout != wantOut || stderr != "" {
				t.Errorf("stdout = %q, stderr = %q; want stdout %q and no stderr", stdout, stderr, wantOut)
			}
			got, _ := os.ReadFile(cfg)
			if want := fmt.Sprintf(tc.want, hooks); string(got) != want {
				t.Errorf("config:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
