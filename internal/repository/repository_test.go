package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/internal/config"
	"github.com/blairham/go-pre-commit/internal/store"
)

func TestNewResolver(t *testing.T) {
	s := store.New("/tmp/test-store")
	cfg := config.DefaultConfig()
	r := NewResolver(s, cfg)
	if r == nil {
		t.Fatal("expected non-nil Resolver")
	}
	if r.Store != s {
		t.Error("expected Store to match")
	}
	if r.Cfg != cfg {
		t.Error("expected Cfg to match")
	}
}

// --- makeMetaHook tests ---

func TestMakeMetaHook_Identity(t *testing.T) {
	hc := &config.HookConfig{ID: "identity"}
	h, err := makeMetaHook(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ID != "identity" {
		t.Errorf("expected ID 'identity', got %q", h.ID)
	}
	if h.Language != "system" {
		t.Errorf("expected language 'system', got %q", h.Language)
	}
	if h.Entry != "echo" {
		t.Errorf("expected entry 'echo', got %q", h.Entry)
	}
	if !h.AlwaysRun {
		t.Error("expected AlwaysRun=true")
	}
	if !h.Verbose {
		t.Error("expected Verbose=true for identity hook")
	}
	if !h.PassFilenames {
		t.Error("expected PassFilenames=true")
	}
}

func TestMakeMetaHook_CheckHooksApply(t *testing.T) {
	hc := &config.HookConfig{ID: "check-hooks-apply"}
	h, err := makeMetaHook(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ID != "check-hooks-apply" {
		t.Errorf("expected ID 'check-hooks-apply', got %q", h.ID)
	}
	if !h.AlwaysRun {
		t.Error("expected AlwaysRun=true")
	}
	if h.PassFilenames {
		t.Error("expected PassFilenames=false for check-hooks-apply")
	}
}

func TestMakeMetaHook_CheckUselessExcludes(t *testing.T) {
	hc := &config.HookConfig{ID: "check-useless-excludes"}
	h, err := makeMetaHook(hc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ID != "check-useless-excludes" {
		t.Errorf("expected ID 'check-useless-excludes', got %q", h.ID)
	}
	if !h.AlwaysRun {
		t.Error("expected AlwaysRun=true")
	}
	if h.PassFilenames {
		t.Error("expected PassFilenames=false")
	}
}

func TestMakeMetaHook_Unknown(t *testing.T) {
	hc := &config.HookConfig{ID: "unknown-hook"}
	_, err := makeMetaHook(hc)
	if err == nil {
		t.Fatal("expected error for unknown meta hook")
	}
}

// --- resolveLocalRepo tests ---

func TestResolveLocalRepo(t *testing.T) {
	s := store.New(t.TempDir())
	cfg := config.DefaultConfig()
	r := NewResolver(s, cfg)

	repo := &config.RepoConfig{
		Repo: "local",
		Hooks: []config.HookConfig{
			{
				ID:       "my-hook",
				Name:     "My Hook",
				Entry:    "echo hello",
				Language: "system",
			},
		},
	}

	hooks, err := r.resolveLocalRepo(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].ID != "my-hook" {
		t.Errorf("expected hook ID 'my-hook', got %q", hooks[0].ID)
	}
}

// --- resolveMetaRepo tests ---

func TestResolveMetaRepo(t *testing.T) {
	s := store.New(t.TempDir())
	cfg := config.DefaultConfig()
	r := NewResolver(s, cfg)

	repo := &config.RepoConfig{
		Repo: "meta",
		Hooks: []config.HookConfig{
			{ID: "identity"},
			{ID: "check-hooks-apply"},
		},
	}

	hooks, err := r.resolveMetaRepo(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}
	if hooks[0].ID != "identity" {
		t.Errorf("expected first hook ID 'identity', got %q", hooks[0].ID)
	}
	if hooks[1].ID != "check-hooks-apply" {
		t.Errorf("expected second hook ID 'check-hooks-apply', got %q", hooks[1].ID)
	}
}

func TestResolveMetaRepo_UnknownHook(t *testing.T) {
	s := store.New(t.TempDir())
	cfg := config.DefaultConfig()
	r := NewResolver(s, cfg)

	repo := &config.RepoConfig{
		Repo: "meta",
		Hooks: []config.HookConfig{
			{ID: "nonexistent-meta-hook"},
		},
	}

	_, err := r.resolveMetaRepo(repo)
	if err == nil {
		t.Fatal("expected error for unknown meta hook")
	}
}

// --- loadManifest tests ---

func TestLoadManifest_PreCommitHooksYaml(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".pre-commit-hooks.yaml")
	content := `-   id: my-hook
    name: My Hook
    entry: echo hello
    language: system
`
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].ID != "my-hook" {
		t.Errorf("expected hook ID 'my-hook', got %q", hooks[0].ID)
	}
}

func TestLoadManifest_FallbackHooksYaml(t *testing.T) {
	dir := t.TempDir()
	// Only create hooks.yaml (not .pre-commit-hooks.yaml).
	manifestPath := filepath.Join(dir, "hooks.yaml")
	content := `-   id: fallback-hook
    name: Fallback Hook
    entry: echo fallback
    language: system
`
	if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks, err := loadManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if hooks[0].ID != "fallback-hook" {
		t.Errorf("expected hook ID 'fallback-hook', got %q", hooks[0].ID)
	}
}

func TestLoadManifest_NoManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := loadManifest(dir)
	if err == nil {
		t.Fatal("expected error when no manifest file exists")
	}
}

func TestLoadManifest_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte("{{{{not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadManifest(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML manifest")
	}
}
