// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/store"
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
	// `system` installs no environment, so the hook runs from the working
	// directory and gets no prefix.
	if hooks[0].RepoDir != "" {
		t.Errorf("expected no RepoDir for a system hook, got %q", hooks[0].RepoDir)
	}
}

func TestResolveLocalRepo_EnvironmentForAdditionalDependencies(t *testing.T) {
	storeDir := t.TempDir()
	s := store.New(storeDir)
	r := NewResolver(s, config.DefaultConfig())

	repo := &config.RepoConfig{
		Repo: "local",
		Hooks: []config.HookConfig{
			{
				ID:                     "dep-probe",
				Name:                   "dep probe",
				Entry:                  "python probe.py",
				Language:               "python",
				AdditionalDependencies: []string{"pyyaml>=6"},
			},
			{
				ID:       "no-deps",
				Name:     "no deps",
				Entry:    "python -c pass",
				Language: "python",
			},
		},
	}

	hooks, err := r.resolveLocalRepo(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}

	withDeps, noDeps := hooks[0], hooks[1]
	if withDeps.RepoDir == "" {
		t.Fatal("expected a prefix for a python hook with additional_dependencies")
	}
	if noDeps.RepoDir == "" {
		t.Fatal("expected a prefix for a python hook without additional_dependencies")
	}
	if withDeps.RepoDir == noDeps.RepoDir {
		t.Errorf("expected different dependency sets to get different environments, both got %q", withDeps.RepoDir)
	}
	if filepath.Dir(withDeps.RepoDir) != storeDir {
		t.Errorf("expected the prefix inside the store %s, got %s", storeDir, withDeps.RepoDir)
	}
}

func TestResolveLocalRepo_AdditionalDependenciesWithoutEnvironment(t *testing.T) {
	s := store.New(t.TempDir())
	r := NewResolver(s, config.DefaultConfig())

	repo := &config.RepoConfig{
		Repo: "local",
		Hooks: []config.HookConfig{
			{
				ID:                     "sys-probe",
				Name:                   "sys probe",
				Entry:                  "echo hi",
				Language:               "system",
				AdditionalDependencies: []string{"pyyaml"},
			},
		},
	}

	_, err := r.resolveLocalRepo(repo)
	if err == nil {
		t.Fatal("expected an error for additional_dependencies on a language with no environment")
	}
	if !strings.Contains(err.Error(), "sys-probe") ||
		!strings.Contains(err.Error(), "additional_dependencies") {
		t.Errorf("expected the error to name the hook and the setting, got: %v", err)
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

// gitRun runs git in dir and returns its trimmed stdout.
func gitRun(t *testing.T, dir string, args ...string) string {
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

// Upstream gives each remote hook the prefix store.clone(repo, rev,
// additional_dependencies). Two hooks of one repo with different dependency
// sets sharing one prefix put both installs in one environment directory, and
// each overwrote the other's install state, so every run reinstalled both.
func TestResolveRemoteRepo_ClonePerDependencySet(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	origin := t.TempDir()
	gitRun(t, origin, "init", "-q", "-b", "main")
	manifest := `- id: plain
  name: plain
  entry: plain
  language: golang
- id: vuln
  name: vuln
  entry: govulncheck
  language: golang
  additional_dependencies: [golang.org/x/vuln/cmd/govulncheck@v1.8.0]
- id: vuln-too
  name: vuln too
  entry: govulncheck
  language: golang
  additional_dependencies: [golang.org/x/vuln/cmd/govulncheck@v1.8.0]
`
	if err := os.WriteFile(filepath.Join(origin, ".pre-commit-hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, origin, "add", "-A")
	gitRun(t, origin, "commit", "-q", "-m", "init")
	sha := gitRun(t, origin, "rev-parse", "HEAD")

	s := store.New(t.TempDir())
	r := NewResolver(s, config.DefaultConfig())
	repo := &config.RepoConfig{Repo: origin, Rev: sha, Hooks: []config.HookConfig{
		{ID: "plain"},
		{ID: "vuln"},
		{ID: "vuln-too"},
		{ID: "plain", AdditionalDependencies: []string{"example.com/extra@v1"}},
	}}
	hooks, err := r.resolveRemoteRepo(context.Background(), repo)
	if err != nil {
		t.Fatalf("resolveRemoteRepo: %v", err)
	}
	if len(hooks) != 4 {
		t.Fatalf("got %d hooks, want 4", len(hooks))
	}
	plain, vuln, vulnToo, extra := hooks[0].RepoDir, hooks[1].RepoDir, hooks[2].RepoDir, hooks[3].RepoDir
	if plain != s.GetPath(origin, sha) {
		t.Errorf("a hook without deps should use the plain clone %s, got %s", s.GetPath(origin, sha), plain)
	}
	if vuln == plain || extra == plain || vuln == extra {
		t.Errorf("each dependency set needs its own clone: plain %s, vuln %s, extra %s", plain, vuln, extra)
	}
	if vulnToo != vuln {
		t.Errorf("hooks with the same dependency set should share a clone: %s vs %s", vuln, vulnToo)
	}
	want := s.GetPath(store.DBRepoName(origin, []string{"golang.org/x/vuln/cmd/govulncheck@v1.8.0"}), sha)
	if vuln != want {
		t.Errorf("vuln clone %s, want the one recorded under db_repo_name %s", vuln, want)
	}
}

// upstream's repository._hook refuses a language_version on a language that
// installs no environment, before it looks at additional_dependencies, and
// says so in these words.
func TestResolveLocalRepo_LanguageVersionWithoutEnvironment(t *testing.T) {
	r := NewResolver(store.New(t.TempDir()), config.DefaultConfig())
	for _, c := range []struct {
		hc   config.HookConfig
		want string
	}{
		{
			hc: config.HookConfig{ID: "v", Name: "v", Entry: "echo", Language: "system", LanguageVersion: "python3"},
			want: "The hook `v` specifies `language_version` but is using language `system` " +
				"which does not install an environment.  Perhaps you meant to use a specific language?",
		},
		{
			hc: config.HookConfig{
				ID: "b", Name: "b", Entry: "echo", Language: "pygrep",
				LanguageVersion: "3", AdditionalDependencies: []string{"x"},
			},
			want: "The hook `b` specifies `language_version` but is using language `pygrep` " +
				"which does not install an environment.  Perhaps you meant to use a specific language?",
		},
		{
			hc: config.HookConfig{ID: "d", Name: "d", Entry: "echo", Language: "fail", AdditionalDependencies: []string{"x"}},
			want: "The hook `d` specifies `additional_dependencies` but is using language `fail` " +
				"which does not install an environment.  Perhaps you meant to use a specific language?",
		},
	} {
		_, err := r.resolveLocalRepo(&config.RepoConfig{Repo: "local", Hooks: []config.HookConfig{c.hc}})
		if err == nil || err.Error() != c.want {
			t.Errorf("hook %s: err = %v\nwant %s", c.hc.ID, err, c.want)
		}
	}
	// `default` is no version at all.
	_, err := r.resolveLocalRepo(&config.RepoConfig{Repo: "local", Hooks: []config.HookConfig{
		{ID: "ok", Name: "ok", Entry: "echo", Language: "system", LanguageVersion: "default"},
	}})
	if err != nil {
		t.Errorf("language_version: default on system: %v", err)
	}
}
