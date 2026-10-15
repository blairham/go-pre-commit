// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// prePushCfg is a hook that prints the files it was handed. verbose makes
// the output visible on success, so the line says what was checked.
const prePushCfg = `repos:
- repo: local
  hooks:
  - id: echo
    name: echo
    entry: sh -c 'echo FILES "$@"' --
    language: system
    stages: [pre-push]
    verbose: true
`

const z40 = "0000000000000000000000000000000000000000"

// prePushRepo builds the scenario from #103: main (a.txt) pushed to origin,
// an unpushed feat branch adding b.txt, and c.txt staged but uncommitted, so
// "checked the pushed commits" (b.txt) and "checked the staged files" (c.txt)
// give different answers. The remote "empty" has nothing, so every commit is
// new to it, root included.
func prePushRepo(t *testing.T) (dir, mainSHA, featSHA string) {
	t.Helper()
	base := t.TempDir()
	dir = filepath.Join(base, "repo")
	git := func(cwd string, args ...string) string {
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
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(base, "init", "-q", "--bare", "origin.git")
	git(base, "init", "-q", "--bare", "empty.git")
	git(base, "init", "-q", "-b", "main", "repo")
	git(dir, "config", "commit.gpgsign", "false")
	git(dir, "remote", "add", "origin", "../origin.git")
	git(dir, "remote", "add", "empty", "../empty.git")
	write(".pre-commit-config.yaml", prePushCfg)
	write("a.txt", "a\n")
	git(dir, "add", ".")
	git(dir, "commit", "-q", "-m", "a")
	git(dir, "push", "-q", "origin", "main")
	mainSHA = git(dir, "rev-parse", "HEAD")
	git(dir, "checkout", "-q", "-b", "feat")
	write("b.txt", "b\n")
	git(dir, "add", "b.txt")
	git(dir, "commit", "-q", "-m", "b")
	featSHA = git(dir, "rev-parse", "HEAD")
	write("c.txt", "c\n")
	git(dir, "add", "c.txt")
	return dir, mainSHA, featSHA
}

// runHookImpl invokes `hook-impl --hook-type=pre-push` the way the installed
// hook script does, feeding git's ref lines on stdin.
func runHookImpl(t *testing.T, bin, dir, remote, stdin string) (string, int) {
	t.Helper()
	var buf bytes.Buffer
	cmd := exec.Command(bin, "hook-impl", "--config=.pre-commit-config.yaml",
		"--hook-type=pre-push", "--hook-dir="+filepath.Join(dir, ".git", "hooks"),
		"--color=never", "--", remote, "../"+remote+".git")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return buf.String(), e.ExitCode()
		}
		t.Fatalf("running %s: %v", bin, err)
	}
	return buf.String(), 0
}

// echoedFiles returns the files the hook printed, in the order it received
// them, or nil when the hook never ran.
func echoedFiles(out string) []string {
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "FILES"); ok {
			files := strings.Fields(rest)
			if files == nil {
				files = []string{}
			}
			return files
		}
	}
	return nil
}

func TestPrePushHookImpl(t *testing.T) {
	pyBin := pythonPreCommit(t)
	const unknown = "1234567890abcdef1234567890abcdef12345678"

	cases := []struct {
		name   string
		remote string
		stdin  func(mainSHA, featSHA string) string
		want   []string // nil: no hook runs
	}{
		{
			name:   "new branch checks the pushed commits",
			remote: "origin",
			stdin: func(_, feat string) string {
				return "refs/heads/feat " + feat + " refs/heads/feat " + z40 + "\n"
			},
			want: []string{"b.txt"},
		},
		{
			name:   "deletion runs nothing",
			remote: "origin",
			stdin: func(_, feat string) string {
				return "(delete) " + z40 + " refs/heads/feat " + feat + "\n"
			},
		},
		{
			name:   "unknown remote sha falls back to the ancestor walk",
			remote: "origin",
			stdin: func(_, feat string) string {
				return "refs/heads/feat " + feat + " refs/heads/feat " + unknown + "\n"
			},
			want: []string{"b.txt"},
		},
		{
			name:   "root commit pushed runs all files",
			remote: "empty",
			stdin: func(_, feat string) string {
				return "refs/heads/feat " + feat + " refs/heads/feat " + z40 + "\n"
			},
			// Upstream's seeded shuffle of the sorted list (#120).
			want: []string{"a.txt", ".pre-commit-config.yaml", "b.txt", "c.txt"},
		},
		{
			name:   "multi-line stdin: first yielding line wins",
			remote: "origin",
			stdin: func(main, feat string) string {
				return "(delete) " + z40 + " refs/heads/x " + feat + "\n" +
					"refs/heads/main " + main + " refs/heads/other " + z40 + "\n" +
					"refs/heads/feat " + feat + " refs/heads/feat " + z40 + "\n" +
					"refs/heads/main " + main + " refs/heads/main " + main + "\n"
			},
			want: []string{"b.txt"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PRE_COMMIT_HOME", t.TempDir())
			pyDir, pyMain, pyFeat := prePushRepo(t)
			goDir, goMain, goFeat := prePushRepo(t)
			pyOut, pyExit := runHookImpl(t, pyBin, pyDir, tc.remote, tc.stdin(pyMain, pyFeat))
			goOut, goExit := runHookImpl(t, goBinary, goDir, tc.remote, tc.stdin(goMain, goFeat))

			addExitResult("hook-impl", "pre-push: "+tc.name, pyExit, goExit,
				pyExit == 0 && goExit == 0, fmt.Sprintf("py=%d go=%d", pyExit, goExit))

			pyFiles, goFiles := echoedFiles(pyOut), echoedFiles(goOut)
			// Each side must match the expected answer, not merely each
			// other: agreement alone would also pass if both were wrong.
			addOutputResult("hook-impl", "pre-push files: "+tc.name,
				slices.Equal(pyFiles, tc.want) && slices.Equal(goFiles, tc.want),
				fmt.Sprintf("want=%q py=%q go=%q\npy output:\n%s\ngo output:\n%s", tc.want, pyFiles, goFiles, pyOut, goOut))
		})
	}
}
