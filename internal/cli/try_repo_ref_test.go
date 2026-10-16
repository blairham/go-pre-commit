// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Upstream's _repo_ref: an explicit ref is used as given, and a clean
// repository is used directly at its HEAD.
func TestTryRepoRefClean(t *testing.T) {
	hooks, shas := hooksRepo(t)
	tmp := t.TempDir()

	repo, ref, err := tryRepoRef(tmp, hooks, "v1.0.0")
	if err != nil || repo != hooks || ref != "v1.0.0" {
		t.Errorf("explicit ref: got %q %q %v", repo, ref, err)
	}

	var stdout string
	stdout, _ = captureOutput(t, func() { repo, ref, err = tryRepoRef(tmp, hooks, "") })
	if err != nil || repo != hooks || ref != shas[2] {
		t.Errorf("clean repo: got %q %q %v, want %q %q", repo, ref, err, hooks, shas[2])
	}
	if stdout != "" {
		t.Errorf("clean repo printed %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(tmp, "shadow-repo")); !os.IsNotExist(err) {
		t.Errorf("clean repo made a shadow repo: %v", err)
	}
}

// With uncommitted changes, upstream commits staged and tracked changes into
// a shadow repository and leaves untracked files out. Before the port, this
// ran `git clone` with its output on the terminal and copied untracked files.
func TestTryRepoRefShadowsUncommittedChanges(t *testing.T) {
	hooks, shas := hooksRepo(t)
	tmp := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(hooks, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("staged-new", "s")
	gitT(t, hooks, "add", "staged-new")
	write("a", "modified")
	write("untracked", "u")

	var repo, ref string
	var err error
	stdout, stderr := captureOutput(t, func() { repo, ref, err = tryRepoRef(tmp, hooks, "") })
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "[WARNING] Creating temporary repo with uncommitted changes...\n" || stderr != "" {
		t.Errorf("stdout = %q, stderr = %q", stdout, stderr)
	}
	shadow := filepath.Join(tmp, "shadow-repo")
	if repo != shadow {
		t.Errorf("repo = %q, want %q", repo, shadow)
	}
	if head := gitT(t, shadow, "rev-parse", "HEAD"); ref != head || ref == shas[2] {
		t.Errorf("ref = %q, shadow HEAD = %q, original HEAD = %q", ref, head, shas[2])
	}
	if parent := gitT(t, shadow, "rev-parse", "HEAD^"); parent != shas[2] {
		t.Errorf("shadow commit's parent = %q, want %q", parent, shas[2])
	}
	if got := gitT(t, shadow, "show", "HEAD:a"); got != "modified" {
		t.Errorf("shadow a = %q, want the unstaged edit", got)
	}
	if got := gitT(t, shadow, "show", "HEAD:staged-new"); got != "s" {
		t.Errorf("shadow staged-new = %q", got)
	}
	files := strings.Split(gitT(t, shadow, "ls-tree", "--name-only", "HEAD"), "\n")
	if slices.Contains(files, "untracked") {
		t.Errorf("untracked file was committed: %q", files)
	}
	const who = "pre-commit <asottile+pre-commit@umich.edu> init"
	if author := gitT(t, shadow, "log", "-1", "--format=%an <%ae> %s"); author != who {
		t.Errorf("shadow commit = %q", author)
	}
	// The user's own index is untouched.
	if staged := gitT(t, hooks, "diff", "--cached", "--name-only"); staged != "staged-new" {
		t.Errorf("user's index changed: staged = %q", staged)
	}
}
