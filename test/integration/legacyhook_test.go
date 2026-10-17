// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestLegacyHook grades a commit through an installed hook that has a
// .legacy hook beside it (#170), as upstream's _run_legacy runs it: a failing
// legacy hook fails the commit, a non-executable one is skipped, one that is
// itself a pre-commit script stops with upstream's bug message rather than
// recursing, and the legacy hook gets empty stdin, PRE_COMMIT_RUNNING_LEGACY
// and git's own environment.
func TestLegacyHook(t *testing.T) {
	pyBin := pythonPreCommit(t)
	// This port's script runs `pre-commit` from PATH; Python's runs its own
	// interpreter.
	t.Setenv("PATH", filepath.Dir(goBinary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	t.Setenv("GIT_TEMPLATE_DIR", t.TempDir())

	const cfg = "repos:\n- repo: local\n  hooks:\n  - id: ok\n    name: ok\n    entry: \"true\"\n    language: system\n"
	writeLegacy := func(body string, mode os.FileMode) func(string) {
		return func(hooks string) {
			if err := os.WriteFile(filepath.Join(hooks, "pre-commit.legacy"), []byte(body), mode); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		name string
		prep func(hooks string)
	}{
		{"a failing legacy hook fails the commit", writeLegacy("#!/bin/sh\necho legacy ran\nexit 1\n", 0o755)},
		{"a non-executable legacy hook is skipped", writeLegacy("#!/bin/sh\necho legacy ran\nexit 1\n", 0o644)},
		{"a pre-commit script as the legacy hook", func(hooks string) {
			b, err := os.ReadFile(filepath.Join(hooks, "pre-commit"))
			if err != nil {
				t.Fatal(err)
			}
			writeLegacy(string(b), 0o755)(hooks)
		}},
		{"the legacy hook's stdin and environment", writeLegacy(
			"#!/bin/sh\necho \"stdin=[$(cat)] legacy=$PRE_COMMIT_RUNNING_LEGACY index=${GIT_INDEX_FILE:+set}\"\n", 0o755,
		)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var outs [2]string
			var codes [2]int
			for i, bin := range []string{pyBin, goBinary} {
				repo := initTestRepo(t, cfg, "x\n")
				runCmd(t, repo, "git", "config", "commit.gpgsign", "false")
				if out, code := runCmd(t, repo, bin, "install"); code != 0 {
					t.Fatalf("%s install: %s", bin, out)
				}
				tc.prep(filepath.Join(repo, ".git", "hooks"))
				outs[i], codes[i] = commitWithTimeout(t, repo)
			}
			addExitResult("hook-impl", tc.name, codes[0], codes[1], codes[0] == codes[1], "")
			addOutputResult("hook-impl", tc.name+": output", outs[0] == outs[1] && outs[0] != "",
				fmt.Sprintf("py=%q go=%q", outs[0], outs[1]))
		})
	}
}

// commitWithTimeout runs `git commit` under a deadline, so a legacy hook that
// recurses fails its row instead of hanging the suite.
func commitWithTimeout(t *testing.T, repo string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "commit", "-q", "-m", "x")
	cmd.Dir = repo
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if ctx.Err() != nil {
		return "timed out:\n" + buf.String(), -1
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return buf.String(), exitErr.ExitCode()
	}
	if err != nil {
		t.Fatalf("git commit: %v", err)
	}
	return buf.String(), 0
}
