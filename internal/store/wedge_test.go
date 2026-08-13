package store

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for the wedged-cache failure.
//
// A `language: golang` hook leaves a module cache of 0444 files inside 0555
// directories. GC used to call os.RemoveAll, ignore the EACCES it returned,
// and drop the database row anyway. Because cache paths are derived from
// repo+rev, the next clone targeted the surviving directory, git refused the
// non-empty destination, and that repo stayed unusable until the cache was
// cleared by hand.

// writeReadOnlyEnv creates the shape `go install` leaves behind inside dir.
func writeReadOnlyEnv(t *testing.T, dir string) {
	t.Helper()

	modDir := filepath.Join(dir, "go_env-default", "pkg", "mod", "example.com", "dep@v1.0.0")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "dep.go"), []byte("package dep\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{modDir, filepath.Dir(modDir)} {
		if err := os.Chmod(d, 0o555); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = os.Chmod(p, 0o755)
			}
			return nil
		})
	})
}

// The core defect: a directory GC could not remove must keep its database row,
// or the cache entry is stranded forever.
func TestGCKeepsEntryWhenDirectoryCannotBeRemoved(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	// The entry lives inside a holder directory that stays unwritable. Removing
	// repoPath needs write on its PARENT, and fsutil.RemoveAll only repairs
	// permissions from repoPath downward — so this is genuinely irrecoverable,
	// which is the case GC has to survive. The store dir itself stays writable
	// so db.json can still be saved.
	holder := filepath.Join(s.Dir(), "holder")
	repoPath := filepath.Join(holder, "repo-readonly")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	writeReadOnlyEnv(t, repoPath)

	if err := s.save("https://example.com/repo", "v1", repoPath); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(holder, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(holder, 0o755) })

	// GC with nothing in use: it will try to remove repoPath and fail.
	err := s.GC(map[string]bool{})

	if err == nil {
		t.Fatal("GC should report the directories it could not remove")
	}
	if !strings.Contains(err.Error(), repoPath) {
		t.Errorf("GC error should name the offending path, got: %v", err)
	}

	// The row must still be there, pointing at the surviving directory.
	if _, statErr := os.Stat(repoPath); statErr != nil {
		t.Fatalf("precondition: directory should have survived, got %v", statErr)
	}
	repos, listErr := s.ListRepos()
	if listErr != nil {
		t.Fatal(listErr)
	}
	found := false
	for _, r := range repos {
		if r.Path == repoPath {
			found = true
		}
	}
	if !found {
		t.Error("GC dropped the database row for a directory that still exists — " +
			"this is the bug that wedges the cache")
	}
}

// The happy path must keep working: a removable directory is removed and its
// row dropped.
func TestGCRemovesUnusedEntry(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	repoPath := filepath.Join(s.Dir(), "repo-plain")
	if err := os.MkdirAll(filepath.Join(repoPath, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.save("https://example.com/repo", "v1", repoPath); err != nil {
		t.Fatal(err)
	}

	if err := s.GC(map[string]bool{}); err != nil {
		t.Fatalf("GC: %v", err)
	}
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Errorf("directory should be gone, stat err = %v", err)
	}
	repos, err := s.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Errorf("row should be dropped, got %d", len(repos))
	}
}

// GC must remove a read-only environment rather than silently leaving it.
func TestGCRemovesReadOnlyEnvironment(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	repoPath := filepath.Join(s.Dir(), "repo-golang")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	writeReadOnlyEnv(t, repoPath)

	if err := s.save("https://example.com/repo", "v1", repoPath); err != nil {
		t.Fatal(err)
	}
	if err := s.GC(map[string]bool{}); err != nil {
		t.Fatalf("GC should clear a read-only env: %v", err)
	}
	if _, err := os.Stat(repoPath); !os.IsNotExist(err) {
		t.Errorf("read-only environment survived GC, stat err = %v", err)
	}
}

// GC must not touch a repo that is still in use.
func TestGCKeepsUsedEntry(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(s.Dir(), "repo-used")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.save("https://example.com/repo", "v1", repoPath); err != nil {
		t.Fatal(err)
	}

	if err := s.GC(map[string]bool{"https://example.com/repo@v1": true}); err != nil {
		t.Fatalf("GC: %v", err)
	}
	if _, err := os.Stat(repoPath); err != nil {
		t.Errorf("in-use repo was removed: %v", err)
	}
}

// A row pointing at a directory that is not a clone must be a miss, not a hit.
// Otherwise hooks run against a tree with no .pre-commit-hooks.yaml.
func TestLookupRejectsDirectoryThatIsNotAClone(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	repoPath := filepath.Join(s.Dir(), "repo-leftover")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	writeReadOnlyEnv(t, repoPath) // env present, no .git — the wedged shape
	if err := s.save("https://example.com/repo", "v1", repoPath); err != nil {
		t.Fatal(err)
	}

	if _, err := s.lookup("https://example.com/repo", "v1"); err == nil {
		t.Error("lookup returned a directory with no .git as a cache hit")
	}
	if got := s.GetPath("https://example.com/repo", "v1"); got != "" {
		t.Errorf("GetPath should be empty for a non-clone, got %q", got)
	}
}

func TestLookupAcceptsARealClone(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	repoPath := filepath.Join(s.Dir(), "repo-valid")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.save("https://example.com/repo", "v1", repoPath); err != nil {
		t.Fatal(err)
	}

	got, err := s.lookup("https://example.com/repo", "v1")
	if err != nil {
		t.Fatalf("lookup on a real clone: %v", err)
	}
	if got != repoPath {
		t.Errorf("got %q, want %q", got, repoPath)
	}
}

// End-to-end: a leftover directory sitting exactly where Clone wants to write
// must not block the clone. This is what produced
// "destination path ... already exists and is not an empty directory".
func TestCloneClearsLeftoverDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	// A local origin to clone from, so the test needs no network.
	origin := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(origin, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(origin, ".pre-commit-hooks.yaml"), []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(origin, "add", "-A")
	run(origin, "commit", "-q", "-m", "init")
	run(origin, "tag", "v1")

	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	// Pre-create the exact destination Clone will compute, and fill it with a
	// read-only environment — the wedged state, reproduced.
	dest := filepath.Join(s.Dir(), cloneDirName(origin, "v1"))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	writeReadOnlyEnv(t, dest)

	got, err := s.Clone(origin, "v1")
	if err != nil {
		t.Fatalf("Clone should clear the leftover and succeed, got: %v", err)
	}
	if got != dest {
		t.Errorf("cloned to %q, want %q", got, dest)
	}
	if _, err := os.Stat(filepath.Join(got, ".git")); err != nil {
		t.Errorf("expected a real clone at %s: %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(got, "go_env-default")); !os.IsNotExist(err) {
		t.Errorf("stale environment should be gone, stat err = %v", err)
	}
}
