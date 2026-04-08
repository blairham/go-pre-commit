package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewWithDir(t *testing.T) {
	s := New("/tmp/my-store")
	if s.Dir() != "/tmp/my-store" {
		t.Fatalf("expected /tmp/my-store, got %s", s.Dir())
	}
}

func TestNewDefaultDir(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", "/tmp/pch")
	s := New("")
	if s.Dir() != "/tmp/pch" {
		t.Fatalf("expected /tmp/pch, got %s", s.Dir())
	}
}

func TestDefaultDirPreCommitHome(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", "/custom/pre-commit")
	t.Setenv("XDG_CACHE_HOME", "")
	got := DefaultDir()
	if got != "/custom/pre-commit" {
		t.Fatalf("expected /custom/pre-commit, got %s", got)
	}
}

func TestDefaultDirXDGCacheHome(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	got := DefaultDir()
	want := filepath.Join("/xdg/cache", "pre-commit")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestDefaultDirFallback(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".cache", "pre-commit")
	got := DefaultDir()
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestInitCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	s := New(storeDir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(storeDir)
	if err != nil {
		t.Fatalf("store dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("store dir is not a directory")
	}
}

func TestCleanRemovesDirectory(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "store")
	s := New(storeDir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Clean(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(storeDir); !os.IsNotExist(err) {
		t.Fatal("expected store dir to be removed")
	}
}

func TestGetPathUnknownRepo(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	got := s.GetPath("https://example.com/repo", "abc123")
	if got != "" {
		t.Fatalf("expected empty string for unknown repo, got %s", got)
	}
}

func TestMarkConfigUsedAndDedup(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(dir, ".pre-commit-config.yaml")
	// Mark same config twice.
	if err := s.MarkConfigUsed(configPath); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkConfigUsed(configPath); err != nil {
		t.Fatal(err)
	}

	configs, err := s.GetTrackedConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
}

func TestGetTrackedConfigs(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	configs, err := s.GetTrackedConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 0 {
		t.Fatalf("expected 0 configs initially, got %d", len(configs))
	}

	if err := s.MarkConfigUsed("/a"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkConfigUsed("/b"); err != nil {
		t.Fatal(err)
	}

	configs, err = s.GetTrackedConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
}

func TestListReposEmptyInitially(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	repos, err := s.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}
}

func TestGCRemovesUnusedRepos(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	// Create mock repo directories.
	usedDir := filepath.Join(dir, "repo-used")
	unusedDir := filepath.Join(dir, "repo-unused")
	os.MkdirAll(usedDir, 0o755)
	os.MkdirAll(unusedDir, 0o755)

	// Write a DB with two repos.
	db := storeDB{
		Repos: []RepoEntry{
			{Repo: "https://example.com/used", Rev: "v1", Path: usedDir},
			{Repo: "https://example.com/unused", Rev: "v2", Path: unusedDir},
		},
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.dbPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// GC keeping only the used repo.
	used := map[string]bool{
		"https://example.com/used@v1": true,
	}
	if err := s.GC(used); err != nil {
		t.Fatal(err)
	}

	// Verify unused directory was removed.
	if _, err := os.Stat(unusedDir); !os.IsNotExist(err) {
		t.Fatal("expected unused repo directory to be removed")
	}
	// Verify used directory still exists.
	if _, err := os.Stat(usedDir); err != nil {
		t.Fatal("expected used repo directory to still exist")
	}

	repos, err := s.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo after GC, got %d", len(repos))
	}
	if repos[0].Repo != "https://example.com/used" {
		t.Fatalf("expected used repo, got %s", repos[0].Repo)
	}
}

func TestDBPersistence(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}

	// Write a DB manually via save.
	repoDir := filepath.Join(dir, "repo1")
	os.MkdirAll(repoDir, 0o755)
	db := storeDB{
		Repos: []RepoEntry{
			{Repo: "https://example.com/repo", Rev: "abc", Path: repoDir},
		},
		ConfigsUsed: []string{"/some/config"},
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.dbPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a new store pointing to same dir and verify it loads the data.
	s2 := New(dir)
	repos, err := s2.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo from persisted DB, got %d", len(repos))
	}
	if repos[0].Repo != "https://example.com/repo" || repos[0].Rev != "abc" {
		t.Fatalf("unexpected repo entry: %+v", repos[0])
	}

	configs, err := s2.GetTrackedConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if len(configs) != 1 || configs[0] != "/some/config" {
		t.Fatalf("unexpected configs: %v", configs)
	}
}
