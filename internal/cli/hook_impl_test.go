// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// runLegacyHook is upstream's _run_legacy: nothing to run without a hook
// dir or an executable legacy hook; otherwise the hook gets exactly the
// stdin handed to it (empty, not ours, outside pre-push), the environment
// plus PRE_COMMIT_RUNNING_LEGACY=1, and its status comes back as retv.
func TestRunLegacyHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script legacy hooks")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	legacy := filepath.Join(dir, "pre-commit.legacy")
	script := "#!/bin/sh\n{ echo \"args=$*\"; echo \"env=$PRE_COMMIT_RUNNING_LEGACY $KEEP\"; echo \"stdin=$(cat)\"; } > " + out + "\nexit 3\n"
	if err := os.WriteFile(legacy, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	retv, err := runLegacyHook(
		"pre-commit",
		dir,
		[]string{"a", "b"},
		[]byte("refs\n"),
		[]string{"KEEP=kept", "PATH=" + os.Getenv("PATH")},
	)
	if err != nil || retv != 3 {
		t.Fatalf("retv=%d err=%v, want 3", retv, err)
	}
	if got, want := readTestFile(t, out), "args=a b\nenv=1 kept\nstdin=refs\n"; got != want {
		t.Errorf("legacy hook saw %q, want %q", got, want)
	}

	// No hook dir: git 2.54+ config-based hooks, nothing to run, even with
	// an executable legacy hook in the current directory.
	if err := os.Remove(out); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if retv, err := runLegacyHook("pre-commit", "", nil, nil, os.Environ()); retv != 0 || err != nil {
		t.Errorf("no hook dir: retv=%d err=%v", retv, err)
	}
	// Not executable: skipped, as os.access(X_OK) does.
	if err := os.Chmod(legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	if retv, err := runLegacyHook("pre-commit", dir, nil, nil, os.Environ()); retv != 0 || err != nil {
		t.Errorf("non-executable: retv=%d err=%v", retv, err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Errorf("a skipped legacy hook ran: %v", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
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

// Upstream's _validate_config, measured against pre-commit 4.7.0: the skip
// message exits with the legacy hook's status, the other fails.
func TestValidateHookConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("PRE_COMMIT_ALLOW_NO_CONFIG", "")
	noConfig := "No .pre-commit-config.yaml file was found\n" +
		"- To temporarily silence this, run `PRE_COMMIT_ALLOW_NO_CONFIG=1 git ...`\n" +
		"- To permanently silence this, install pre-commit with the --allow-missing-config option\n" +
		"- To uninstall pre-commit run `pre-commit uninstall`\n"
	skipping := "`.pre-commit-config.yaml` config file not found. Skipping `pre-commit`.\n"
	var stop bool
	validate := func(retv int, skip bool) func() int {
		return func() int {
			var exit int
			exit, stop = validateHookConfig(retv, ".pre-commit-config.yaml", skip)
			return exit
		}
	}

	out, code := runStdout(t, validate(0, false))
	wantOut(t, "missing config", out, code, noConfig, 1)
	out, code = runStdout(t, validate(3, true))
	wantOut(t, "--skip-on-missing-config", out, code, skipping, 3)
	t.Setenv("PRE_COMMIT_ALLOW_NO_CONFIG", "1")
	out, code = runStdout(t, validate(0, false))
	wantOut(t, "PRE_COMMIT_ALLOW_NO_CONFIG", out, code, skipping, 0)

	// A directory is not a config file (os.path.isfile).
	if err := os.Mkdir(".pre-commit-config.yaml", 0o755); err != nil {
		t.Fatal(err)
	}
	out, code = runStdout(t, validate(0, true))
	wantOut(t, "config is a directory", out, code, skipping, 0)

	if err := os.Remove(".pre-commit-config.yaml"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".pre-commit-config.yaml", []byte("repos: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ = runStdout(t, validate(0, false))
	if out != "" || stop {
		t.Errorf("with a config: stop=%v out=%q", stop, out)
	}
}
