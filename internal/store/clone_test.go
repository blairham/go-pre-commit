// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gitutil "github.com/blairham/go-pre-commit/v4/internal/git"
)

// Tests for the clone strategy ported from upstream's Store.clone (#117).

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// hookOrigin makes a local hook repository with one commit and returns its
// path and HEAD sha.
func hookOrigin(t *testing.T) (dir, sha string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir = t.TempDir()
	gitIn(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, ".pre-commit-hooks.yaml"), []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	return dir, gitIn(t, dir, "rev-parse", "HEAD")
}

// captureOutput runs fn with os.Stdout and os.Stderr redirected, and returns
// what was written to each.
func captureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	read := func(r *os.File, into *string, done chan<- struct{}) {
		b, _ := io.ReadAll(r)
		*into = string(b)
		close(done)
	}
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
	outDone, errDone := make(chan struct{}), make(chan struct{})
	go read(outR, &stdout, outDone)
	go read(errR, &stderr, errDone)
	func() {
		defer func() { os.Stdout, os.Stderr = origOut, origErr }()
		fn()
	}()
	outW.Close()
	errW.Close()
	<-outDone
	<-errDone
	return stdout, stderr
}

// A sha rev is fetched shallowly. `git clone --branch` cannot take a sha, so
// the old strategy failed its shallow attempt on every sha-pinned rev and fell
// back to a full clone.
func TestCloneFetchesShaShallowly(t *testing.T) {
	origin, sha := hookOrigin(t)
	// A second commit, so a full clone and a shallow one differ.
	if err := os.WriteFile(filepath.Join(origin, "more"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, origin, "add", "-A")
	gitIn(t, origin, "commit", "-q", "-m", "more")
	head := gitIn(t, origin, "rev-parse", "HEAD")

	s := New(t.TempDir())
	got, err := s.Clone(origin, head)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if h := gitIn(t, got, "rev-parse", "HEAD"); h != head {
		t.Errorf("checked out %s, want %s", h, head)
	}
	if _, err := os.Stat(filepath.Join(got, ".git", "shallow")); err != nil {
		t.Errorf("expected a shallow clone: %v", err)
	}
	if n := gitIn(t, got, "rev-list", "--count", "HEAD"); n != "1" {
		t.Errorf("history has %s commits, want 1", n)
	}
	if _, err := exec.Command("git", "-C", got, "cat-file", "-e", sha).CombinedOutput(); err == nil {
		t.Errorf("the parent commit %s should not have been fetched", sha)
	}
}

// git's chatter stays off the terminal; the only thing a first install prints
// is upstream's "Initializing environment" line.
func TestCloneOutputIsOnlyTheInitializingLine(t *testing.T) {
	origin, sha := hookOrigin(t)
	s := New(t.TempDir())

	var cloneErr error
	stdout, stderr := captureOutput(t, func() { _, cloneErr = s.Clone(origin, sha) })
	if cloneErr != nil {
		t.Fatalf("Clone: %v", cloneErr)
	}
	if want := "[INFO] Initializing environment for " + origin + ".\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}

	// A cache hit prints nothing at all.
	stdout, stderr = captureOutput(t, func() { _, cloneErr = s.Clone(origin, sha) })
	if cloneErr != nil || stdout != "" || stderr != "" {
		t.Errorf("cached Clone: err=%v stdout=%q stderr=%q", cloneErr, stdout, stderr)
	}
}

// A rev the shallow fetch cannot name falls back to fetching tags and
// checking the rev out, as upstream's _complete_clone does. An abbreviated
// sha is such a rev: fetch wants a full object name or a ref.
func TestCloneFallsBackToCompleteClone(t *testing.T) {
	origin, sha := hookOrigin(t)
	s := New(t.TempDir())
	got, err := s.Clone(origin, sha[:10])
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if h := gitIn(t, got, "rev-parse", "HEAD"); h != sha {
		t.Errorf("checked out %s, want %s", h, sha)
	}
}

// When both strategies fail, git's own output comes back in the error, in
// upstream's CalledProcessError shape, and nothing is left in the cache.
func TestCloneFailureCarriesGitOutput(t *testing.T) {
	origin, _ := hookOrigin(t)
	s := New(t.TempDir())

	var cloneErr error
	stdout, stderr := captureOutput(t, func() { _, cloneErr = s.Clone(origin, "nonexistent") })
	if cloneErr == nil {
		t.Fatal("Clone of a nonexistent rev succeeded")
	}
	var cpe *gitutil.CalledProcessError
	if !errors.As(cloneErr, &cpe) {
		t.Fatalf("error is %T, want *CalledProcessError: %v", cloneErr, cloneErr)
	}
	msg := cloneErr.Error()
	for _, want := range []string{
		"', 'checkout', 'nonexistent')\n",
		"return code: 1\n",
		"stdout: (none)\n",
		"stderr:\n    error: pathspec 'nonexistent' did not match",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error lacks %q:\n%s", want, msg)
		}
	}
	if strings.Contains(stdout, "pathspec") || stderr != "" {
		t.Errorf("git output leaked: stdout=%q stderr=%q", stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), cloneDirName(origin, "nonexistent"))); !os.IsNotExist(err) {
		t.Errorf("failed clone left its directory behind: %v", err)
	}
}

// Submodules are checked out, by the shallow path's
// `submodule update --init --recursive --depth=1`.
func TestCloneChecksOutSubmodules(t *testing.T) {
	sub, _ := hookOrigin(t)
	origin, _ := hookOrigin(t)
	// git refuses file:// submodules by default since 2.38.1; allow it for
	// this test's git and for the store's, which keeps GIT_CONFIG_* through
	// NoGitEnv the way upstream's no_git_env does.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "protocol.file.allow")
	t.Setenv("GIT_CONFIG_VALUE_0", "always")
	gitIn(t, origin, "submodule", "add", "-q", sub, "vendored")
	gitIn(t, origin, "commit", "-q", "-m", "submodule")
	head := gitIn(t, origin, "rev-parse", "HEAD")

	s := New(t.TempDir())
	got, err := s.Clone(origin, head)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(got, "vendored", ".pre-commit-hooks.yaml")); err != nil {
		t.Errorf("submodule was not checked out: %v", err)
	}
}
