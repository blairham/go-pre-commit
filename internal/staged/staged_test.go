package staged

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	m := NewManager("/some/dir")
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
	if m.dir != "/some/dir" {
		t.Errorf("expected dir '/some/dir', got %q", m.dir)
	}
}

// --- IsStashed tests ---

func TestIsStashed_Default(t *testing.T) {
	m := NewManager("/tmp")
	if m.IsStashed() {
		t.Error("expected IsStashed=false for new Manager")
	}
}

// --- StashUnstaged tests ---

func TestStashUnstaged_NoChanges(t *testing.T) {
	dir := initTestRepo(t)
	m := NewManager(dir)

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

	m := NewManager(dir)
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

	m := NewManager(dir)
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
	m := NewManager("/tmp")
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

	m := NewManager(dir)
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

func TestRestore_CleansPatchFile(t *testing.T) {
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

	m := NewManager(dir)
	m.StashUnstaged()

	patchPath := m.patchPath
	if patchPath == "" {
		t.Fatal("expected patch path to be set")
	}

	// Verify patch file exists.
	if _, err := os.Stat(patchPath); os.IsNotExist(err) {
		t.Fatal("expected patch file to exist before restore")
	}

	m.Restore()

	// Verify patch file is cleaned up.
	if _, err := os.Stat(patchPath); !os.IsNotExist(err) {
		t.Error("expected patch file to be cleaned up after restore")
	}
}
