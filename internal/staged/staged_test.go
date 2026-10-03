// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package staged

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/git"
)

// initTestRepo creates a temp git repo with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "file.txt")
	run("commit", "-m", "initial commit")

	return dir
}

// --- NewManager tests ---

func TestNewManager(t *testing.T) {
	m := NewManager("/some/dir", t.TempDir())
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
	if m.dir != "/some/dir" {
		t.Errorf("expected dir '/some/dir', got %q", m.dir)
	}
}

// --- IsStashed tests ---

func TestIsStashed_Default(t *testing.T) {
	m := NewManager("/tmp", t.TempDir())
	if m.IsStashed() {
		t.Error("expected IsStashed=false for new Manager")
	}
}

// --- StashUnstaged tests ---

func TestStashUnstaged_NoChanges(t *testing.T) {
	dir := initTestRepo(t)
	m := NewManager(dir, t.TempDir())

	stashed, err := m.StashUnstaged()
	if err != nil {
		t.Fatalf("StashUnstaged failed: %v", err)
	}
	if stashed {
		t.Error("expected stashed=false for clean repo")
	}
	if m.IsStashed() {
		t.Error("expected IsStashed=false for clean repo")
	}
}

func TestStashUnstaged_WithUnstagedChanges(t *testing.T) {
	dir := initTestRepo(t)

	// Stage a change.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Make an unstaged change on top.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(dir, t.TempDir())
	stashed, err := m.StashUnstaged()
	if err != nil {
		t.Fatalf("StashUnstaged failed: %v", err)
	}
	if !stashed {
		t.Error("expected stashed=true when there are unstaged changes")
	}
	if !m.IsStashed() {
		t.Error("expected IsStashed=true after stashing")
	}

	// Verify file now has staged content.
	content, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "staged\n" {
		t.Errorf("expected staged content after stash, got %q", string(content))
	}
}

func TestStashUnstaged_OnlyStagedChanges(t *testing.T) {
	dir := initTestRepo(t)

	// Stage a change but don't add unstaged modifications.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("staged only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	m := NewManager(dir, t.TempDir())
	stashed, err := m.StashUnstaged()
	if err != nil {
		t.Fatalf("StashUnstaged failed: %v", err)
	}
	if stashed {
		t.Error("expected stashed=false when only staged changes exist")
	}
}

// --- Restore tests ---

func TestRestore_NotStashed(t *testing.T) {
	m := NewManager("/tmp", t.TempDir())
	// Restore on a non-stashed manager should be a no-op.
	if err := m.Restore(); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
}

func TestRestore_RoundTrip(t *testing.T) {
	dir := initTestRepo(t)

	// Stage a change.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Make an unstaged change.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewManager(dir, t.TempDir())
	stashed, err := m.StashUnstaged()
	if err != nil {
		t.Fatalf("StashUnstaged failed: %v", err)
	}
	if !stashed {
		t.Fatal("expected stash to succeed")
	}

	// Verify staged content is checked out.
	content, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(content) != "staged\n" {
		t.Errorf("expected staged content, got %q", string(content))
	}

	// Restore — this should apply the unstaged diff back.
	if err := m.Restore(); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	if m.IsStashed() {
		t.Error("expected IsStashed=false after Restore")
	}

	// After restore, the working tree should have the unstaged content back.
	content, _ = os.ReadFile(filepath.Join(dir, "file.txt"))
	if string(content) != "unstaged\n" {
		t.Errorf("expected unstaged content after restore, got %q", string(content))
	}
}

// Upstream keeps the patch in the store after restoring, named
// patch<time>-<pid>: the path it prints is how a user recovers unstaged work
// if a hook or the process dies first.
func TestRestore_KeepsPatchFileInStore(t *testing.T) {
	dir := initTestRepo(t)

	// Stage + unstage.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := t.TempDir()
	m := NewManager(dir, store)
	m.StashUnstaged()

	patchPath := m.patchPath
	if patchPath == "" {
		t.Fatal("expected patch path to be set")
	}

	// Verify patch file exists.
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		t.Fatal("expected patch file to exist before restore")
	}

	if filepath.Dir(patchPath) != store || !regexp.MustCompile(`^patch\d+-\d+$`).MatchString(filepath.Base(patchPath)) {
		t.Errorf("patch is %q, want <store>/patch<time>-<pid>", patchPath)
	}

	m.Restore()

	if _, err := os.Stat(patchPath); err != nil {
		t.Errorf("expected the patch to be kept after restore: %v", err)
	}
}

// stageAndDirty stages "staged\n" in file.txt and leaves "unstaged\n" on top.
func stageAndDirty(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Under `git commit -a` / `git commit <paths>` git holds index.lock for the
// whole hook and points GIT_INDEX_FILE at the index being committed. The stash
// must work against that index; against the default one, write-tree dies on
// the held lock ("Unable to create .../index.lock: File exists").
func TestStashUnstaged_HostIndexWhileIndexLocked(t *testing.T) {
	dir := initTestRepo(t)
	stageAndDirty(t, dir)

	// Hold the lock the way git commit does, with the commit's index in it.
	gitDir := filepath.Join(dir, ".git")
	index, err := os.ReadFile(filepath.Join(gitDir, "index"))
	if err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(gitDir, "index.lock")
	if err := os.WriteFile(lock, index, 0o644); err != nil {
		t.Fatal(err)
	}
	git.SetHostIndexFile(lock)
	t.Cleanup(func() { git.SetHostIndexFile("") })

	m := NewManager(dir, t.TempDir())
	stashed, err := m.StashUnstaged()
	if err != nil {
		t.Fatalf("StashUnstaged with index.lock held: %v", err)
	}
	if !stashed {
		t.Fatal("expected the unstaged change to be stashed")
	}
	if got := readFile(t, filepath.Join(dir, "file.txt")); got != "staged\n" {
		t.Errorf("during the run file.txt = %q, want the staged content", got)
	}
	if err := m.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readFile(t, filepath.Join(dir, "file.txt")); got != "unstaged\n" {
		t.Errorf("after restore file.txt = %q, want the unstaged content back", got)
	}
}

// When a hook's fix conflicts with the stashed changes, the fix is rolled back
// and the stashed changes win, as upstream does.
func TestRestore_ConflictWithHookFixRollsBack(t *testing.T) {
	dir := initTestRepo(t)
	stageAndDirty(t, dir)

	m := NewManager(dir, t.TempDir())
	if stashed, err := m.StashUnstaged(); err != nil || !stashed {
		t.Fatalf("StashUnstaged = %v, %v", stashed, err)
	}
	// The "hook" rewrites the same line the stashed patch changes.
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("fixed by hook\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := m.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readFile(t, filepath.Join(dir, "file.txt")); got != "unstaged\n" {
		t.Errorf("after restore file.txt = %q, want the unstaged content", got)
	}
}

// Intent-to-add entries are dropped for the run and put back afterwards.
func TestRestore_ReaddsIntentToAdd(t *testing.T) {
	dir := initTestRepo(t)
	stageAndDirty(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "add", "--intent-to-add", "new.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add -N: %v\n%s", err, out)
	}

	m := NewManager(dir, t.TempDir())
	if stashed, err := m.StashUnstaged(); err != nil || !stashed {
		t.Fatalf("StashUnstaged = %v, %v", stashed, err)
	}
	if err := m.Restore(); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	ita, err := git.IntentToAddFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ita) != 1 || ita[0] != "new.txt" {
		t.Errorf("intent-to-add files after restore = %v, want [new.txt]", ita)
	}
}
