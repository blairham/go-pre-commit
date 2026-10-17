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
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// Upstream runs every command inside error_handler, which reports a
// FatalError (InvalidConfigError, ...) as "An error has occurred" and exit 1,
// any other exception as "An unexpected error has occurred" and exit 3, and
// KeyboardInterrupt as "Interrupted (^C)" and exit 130 -- each followed by a
// pointer to pre-commit.log in the store (#126). Config errors carry cfgv's
// "==> context" / "=====> message" form.
func TestErrorHandler(t *testing.T) {
	pyBin := pythonPreCommit(t)
	home := t.TempDir()
	t.Setenv("PRE_COMMIT_HOME", home)

	for _, tc := range []struct {
		name, cfg string
		want      int
	}{
		{"missing config", "", 1},
		{"no repos key", "foo: 1\n", 1},
		{"repos not a list", "repos: 5\n", 1},
		{"repo not a map", "repos: [5]\n", 1},
		{"remote repo without rev", "repos:\n- repo: https://example.com/x\n  hooks: [{id: a}]\n", 1},
		{"local hook without name", "repos:\n- repo: local\n  hooks: [{id: a, entry: b, language: system}]\n", 1},
		{"hook without id", "repos:\n- repo: local\n  hooks: [{name: a, entry: b, language: system}]\n", 1},
		{"unknown language", "repos:\n- repo: local\n  hooks: [{id: a, name: a, entry: b, language: cobol}]\n", 1},
		{"invalid hook regex", "repos:\n- repo: local\n  hooks: [{id: a, name: a, entry: b, language: system, files: '('}]\n", 1},
		{"invalid top-level regex", "files: '('\nrepos: []\n", 1},
		{"rev on a local repo", "repos:\n- repo: local\n  rev: v1\n  hooks: [{id: a, name: a, entry: b, language: system}]\n", 1},
		{"args not strings", "repos:\n- repo: local\n  hooks: [{id: a, name: a, entry: b, language: system, args: [1]}]\n", 1},
		{"unknown stage", "default_stages: [nope]\nrepos: []\n", 1},
		{"empty file", "\n", 1},
		{"minimum version too new", "minimum_pre_commit_version: '99.0'\nrepos: []\n", 1},
		// parse_version is int() of each part: a ValueError, which nothing
		// catches, so it is the unexpected-error form and exit 3.
		{"unexpected error: unparsable minimum version", "minimum_pre_commit_version: 'x'\nrepos: []\n", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pyDir, goDir := initTestRepo(t, tc.cfg, ""), initTestRepo(t, tc.cfg, "")
			if tc.cfg == "" {
				// initTestRepo writes no config for an empty one.
				os.Remove(filepath.Join(pyDir, ".pre-commit-config.yaml"))
				os.Remove(filepath.Join(goDir, ".pre-commit-config.yaml"))
			}
			pyOut, pyErr, pyExit := runSplit(t, pyDir, pyBin, "run", "--all-files", "--color=never")
			goOut, goErr, goExit := runSplit(t, goDir, goBinary, "run", "--all-files", "--color=never")
			pyOut, goOut = installedVersion(pyOut), installedVersion(goOut)
			addExitResult("run", "error handler: "+tc.name, pyExit, goExit, pyExit == goExit && goExit == tc.want, "")
			addOutputResult("run", "error handler: "+tc.name,
				pyOut == goOut && pyErr == goErr && strings.Contains(goOut, "\nCheck the log at "),
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
			checkErrorLog(t, home, goOut)
		})
	}

	// A missing config from the commands that open it first is open()'s
	// FileNotFoundError (unexpected, exit 3); from those that load it, an
	// InvalidConfigError (exit 1).
	for _, tc := range []struct {
		cmd  string
		want int
	}{{"migrate-config", 3}, {"autoupdate", 3}, {"install-hooks", 1}} {
		t.Run(tc.cmd+" missing config", func(t *testing.T) {
			pyDir, goDir := initTestRepo(t, "", ""), initTestRepo(t, "", "")
			pyOut, pyErr, pyExit := runSplit(t, pyDir, pyBin, tc.cmd, "-c", "nope.yaml")
			goOut, goErr, goExit := runSplit(t, goDir, goBinary, tc.cmd, "-c", "nope.yaml")
			addExitResult(tc.cmd, "error handler: missing config", pyExit, goExit, pyExit == goExit && goExit == tc.want, "")
			addOutputResult(tc.cmd, "error handler: missing config", pyOut == goOut && pyErr == goErr && goOut != "",
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		})
	}

	// validate-config and validate-manifest print each file's error and go
	// on; success prints nothing.
	t.Run("validate-config", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("repos: 5\n"), 0o644)
		os.WriteFile(filepath.Join(dir, "good.yaml"), []byte("repos: []\n"), 0o644)
		args := []string{"validate-config", "bad.yaml", "good.yaml", "missing.yaml"}
		pyOut, pyErr, pyExit := runSplit(t, dir, pyBin, args...)
		goOut, goErr, goExit := runSplit(t, dir, goBinary, args...)
		addExitResult("validate-config", "cfgv errors", pyExit, goExit, pyExit == goExit && goExit == 1, "")
		addOutputResult("validate-config", "cfgv errors", pyOut == goOut && pyErr == goErr && goOut != "",
			fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
	})
	t.Run("validate-manifest", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("- id: a\n"), 0o644)
		os.WriteFile(filepath.Join(dir, "v5.yaml"), []byte("x: 1\n"), 0o644)
		args := []string{"validate-manifest", "bad.yaml", "v5.yaml", "missing.yaml"}
		pyOut, pyErr, pyExit := runSplit(t, dir, pyBin, args...)
		goOut, goErr, goExit := runSplit(t, dir, goBinary, args...)
		pyOut, goOut = installedVersion(pyOut), installedVersion(goOut)
		addExitResult("validate-manifest", "cfgv errors", pyExit, goExit, pyExit == goExit && goExit == 1, "")
		addOutputResult("validate-manifest", "cfgv errors", pyOut == goOut && pyErr == goErr && goOut != "",
			fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
	})

	// SIGINT while a hook runs: the hook's name and dots, then the
	// interrupt in place of its status, and exit 130.
	t.Run("interrupted", func(t *testing.T) {
		const cfg = "repos:\n- repo: local\n  hooks:\n" +
			"  - {id: s, name: slow, entry: sleep 10, language: system, pass_filenames: false, always_run: true}\n" +
			"  - {id: t, name: never, entry: 'true', language: system, pass_filenames: false, always_run: true}\n"
		pyOut, pyExit := interruptRun(t, initTestRepo(t, cfg, ""), pyBin)
		goOut, goExit := interruptRun(t, initTestRepo(t, cfg, ""), goBinary)
		addExitResult("run", "error handler: interrupted", pyExit, goExit, pyExit == goExit && goExit == 130, "")
		addOutputResult("run", "error handler: interrupted",
			pyOut == goOut && strings.Contains(goOut, "Interrupted (^C): KeyboardInterrupt: \nCheck the log at "),
			fmt.Sprintf("py=%q go=%q", pyOut, goOut))
	})
}

// installedVersionRe is the running version in check_min_version's
// message. A build from a checkout stamps a pseudo-version (4.7.1-0.2026...),
// which is the binary under test, not a divergence.
var installedVersionRe = regexp.MustCompile(`but version \S+ is installed`)

func installedVersion(s string) string {
	return installedVersionRe.ReplaceAllString(s, "but version <version> is installed")
}

// checkErrorLog checks the log the error line pointed at has upstream's
// sections and records the error.
func checkErrorLog(t *testing.T, home, out string) {
	t.Helper()
	first, _, _ := strings.Cut(out, "\n")
	log := readFile(filepath.Join(home, "pre-commit.log"))
	if real, err := filepath.EvalSymlinks(home); err == nil && log == "" {
		log = readFile(filepath.Join(real, "pre-commit.log"))
	}
	ok := strings.HasPrefix(log, "### version information\n") &&
		strings.Contains(log, "\n### error information\n\n```\n"+first+"\n")
	addFSResult("run", "error handler writes pre-commit.log: "+first, ok, fmt.Sprintf("log=%q", log))
}

// lockedBuffer is a bytes.Buffer safe to read while a process writes it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// interruptRun starts `run --all-files`, waits for the slow hook to start,
// and sends the process SIGINT.
func interruptRun(t *testing.T, dir, bin string) (string, int) {
	t.Helper()
	var out lockedBuffer
	cmd := exec.Command(bin, "run", "--all-files", "--color=never")
	cmd.Dir = dir
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for !strings.HasPrefix(out.String(), "slow...") && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	// The name is printed just before the hook is started.
	time.Sleep(500 * time.Millisecond)
	_ = cmd.Process.Signal(os.Interrupt)
	err := cmd.Wait()
	code := 0
	if e, ok := err.(*exec.ExitError); ok {
		code = e.ExitCode()
	}
	return out.String(), code
}
