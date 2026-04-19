package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- LoadConfig tests ---

func TestLoadConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.0.0
    hooks:
    -   id: trailing-whitespace
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].Repo != "https://github.com/pre-commit/pre-commit-hooks" {
		t.Errorf("unexpected repo URL: %s", cfg.Repos[0].Repo)
	}
	if cfg.Repos[0].Rev != "v4.0.0" {
		t.Errorf("unexpected rev: %s", cfg.Repos[0].Rev)
	}
	if len(cfg.Repos[0].Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(cfg.Repos[0].Hooks))
	}
	if cfg.Repos[0].Hooks[0].ID != "trailing-whitespace" {
		t.Errorf("unexpected hook id: %s", cfg.Repos[0].Hooks[0].ID)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("{{{{not yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse config file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadConfig_MissingRepos(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `fail_fast: true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing repos")
	}
	if !strings.Contains(err.Error(), "'repos' is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- Validate tests ---

func TestValidate_MissingRepoField(t *testing.T) {
	cfg := &Config{
		Repos: []RepoConfig{
			{Repo: "", Hooks: []HookConfig{{ID: "test"}}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing repo field")
	}
	if !strings.Contains(err.Error(), "'repo' is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_MissingRevForRemoteRepo(t *testing.T) {
	cfg := &Config{
		Repos: []RepoConfig{
			{Repo: "https://github.com/example/repo", Rev: "", Hooks: []HookConfig{{ID: "test"}}},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing rev")
	}
	if !strings.Contains(err.Error(), "'rev' is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_LocalRepoNoRevRequired(t *testing.T) {
	cfg := &Config{
		Repos: []RepoConfig{
			{
				Repo: "local",
				Hooks: []HookConfig{
					{ID: "test", Name: "Test", Entry: "echo hi", Language: "system"},
				},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("local repo should not require rev: %v", err)
	}
}

func TestValidate_MetaRepoNoRevRequired(t *testing.T) {
	cfg := &Config{
		Repos: []RepoConfig{
			{
				Repo:  "meta",
				Hooks: []HookConfig{{ID: "check-hooks-apply"}},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("meta repo should not require rev: %v", err)
	}
}

func TestValidate_LocalHookRequiresFields(t *testing.T) {
	tests := []struct {
		name    string
		hook    HookConfig
		wantErr string
	}{
		{
			name:    "missing name",
			hook:    HookConfig{ID: "test", Entry: "echo", Language: "system"},
			wantErr: "'name' is required",
		},
		{
			name:    "missing entry",
			hook:    HookConfig{ID: "test", Name: "Test", Language: "system"},
			wantErr: "'entry' is required",
		},
		{
			name:    "missing language",
			hook:    HookConfig{ID: "test", Name: "Test", Entry: "echo"},
			wantErr: "'language' is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Repos: []RepoConfig{
					{Repo: "local", Hooks: []HookConfig{tc.hook}},
				},
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidate_InvalidRegex_TopLevel(t *testing.T) {
	tests := []struct {
		name    string
		files   string
		exclude string
		wantErr string
	}{
		{
			name:    "invalid files pattern",
			files:   "[invalid",
			wantErr: "invalid 'files' pattern",
		},
		{
			name:    "invalid exclude pattern",
			exclude: "[invalid",
			wantErr: "invalid 'exclude' pattern",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Files:   tc.files,
				Exclude: tc.exclude,
				Repos: []RepoConfig{
					{
						Repo: "https://github.com/example/repo",
						Rev:  "v1.0.0",
						Hooks: []HookConfig{
							{ID: "test"},
						},
					},
				},
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidate_InvalidRegex_HookLevel(t *testing.T) {
	tests := []struct {
		name    string
		files   string
		exclude string
		wantErr string
	}{
		{
			name:    "invalid hook files pattern",
			files:   "[invalid",
			wantErr: "invalid 'files' pattern",
		},
		{
			name:    "invalid hook exclude pattern",
			exclude: "[invalid",
			wantErr: "invalid 'exclude' pattern",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Repos: []RepoConfig{
					{
						Repo: "https://github.com/example/repo",
						Rev:  "v1.0.0",
						Hooks: []HookConfig{
							{ID: "test", Files: tc.files, Exclude: tc.exclude},
						},
					},
				},
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidate_ValidRegex(t *testing.T) {
	cfg := &Config{
		Files:   `\.go$`,
		Exclude: `^vendor/`,
		Repos: []RepoConfig{
			{
				Repo: "https://github.com/example/repo",
				Rev:  "v1.0.0",
				Hooks: []HookConfig{
					{ID: "test", Files: `\.py$`, Exclude: `test_.*\.py$`},
				},
			},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ApplyDefaults tests ---

func TestApplyDefaults_DefaultStages(t *testing.T) {
	cfg := &Config{
		DefaultStages: []Stage{HookTypePreCommit, HookTypePrePush},
		Repos: []RepoConfig{
			{
				Repo: "https://github.com/example/repo",
				Rev:  "v1.0.0",
				Hooks: []HookConfig{
					{ID: "no-stages"},
					{ID: "has-stages", Stages: []Stage{HookTypeCommitMsg}},
				},
			},
		},
	}

	cfg.ApplyDefaults()

	hook0 := cfg.Repos[0].Hooks[0]
	if len(hook0.Stages) != 2 {
		t.Fatalf("expected 2 default stages, got %d", len(hook0.Stages))
	}
	if hook0.Stages[0] != HookTypePreCommit || hook0.Stages[1] != HookTypePrePush {
		t.Errorf("unexpected stages: %v", hook0.Stages)
	}

	hook1 := cfg.Repos[0].Hooks[1]
	if len(hook1.Stages) != 1 || hook1.Stages[0] != HookTypeCommitMsg {
		t.Errorf("hook with explicit stages should not be overwritten, got: %v", hook1.Stages)
	}
}

func TestApplyDefaults_DefaultLanguageVersion(t *testing.T) {
	cfg := &Config{
		DefaultLanguageVersion: map[string]string{
			"python": "python3.9",
			"node":   "18.0.0",
		},
		Repos: []RepoConfig{
			{
				Repo: "https://github.com/example/repo",
				Rev:  "v1.0.0",
				Hooks: []HookConfig{
					{ID: "pylint", Language: "python"},
					{ID: "eslint", Language: "node", LanguageVersion: "16.0.0"},
					{ID: "rustfmt", Language: "rust"},
				},
			},
		},
	}

	cfg.ApplyDefaults()

	if cfg.Repos[0].Hooks[0].LanguageVersion != "python3.9" {
		t.Errorf("expected python3.9, got %s", cfg.Repos[0].Hooks[0].LanguageVersion)
	}
	if cfg.Repos[0].Hooks[1].LanguageVersion != "16.0.0" {
		t.Errorf("hook with explicit language_version should not be overwritten, got %s", cfg.Repos[0].Hooks[1].LanguageVersion)
	}
	if cfg.Repos[0].Hooks[2].LanguageVersion != "" {
		t.Errorf("hook with no matching default should remain empty, got %s", cfg.Repos[0].Hooks[2].LanguageVersion)
	}
}

// --- migrateLegacyStages tests ---

func TestMigrateLegacyStages(t *testing.T) {
	tests := []struct {
		name   string
		input  []Stage
		expect []Stage
	}{
		{
			name:   "commit becomes pre-commit",
			input:  []Stage{"commit"},
			expect: []Stage{HookTypePreCommit},
		},
		{
			name:   "push becomes pre-push",
			input:  []Stage{"push"},
			expect: []Stage{HookTypePrePush},
		},
		{
			name:   "merge-commit becomes pre-merge-commit",
			input:  []Stage{"merge-commit"},
			expect: []Stage{HookTypePreMergeCommit},
		},
		{
			name:   "unknown stage passes through",
			input:  []Stage{"commit-msg"},
			expect: []Stage{HookTypeCommitMsg},
		},
		{
			name:   "nil input returns nil",
			input:  nil,
			expect: nil,
		},
		{
			name:   "empty input returns empty",
			input:  []Stage{},
			expect: []Stage{},
		},
		{
			name:   "mixed legacy and modern",
			input:  []Stage{"commit", HookTypePrePush, "merge-commit"},
			expect: []Stage{HookTypePreCommit, HookTypePrePush, HookTypePreMergeCommit},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := migrateLegacyStages(tc.input)
			if tc.expect == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tc.expect) {
				t.Fatalf("expected %d stages, got %d: %v", len(tc.expect), len(result), result)
			}
			for i := range tc.expect {
				if result[i] != tc.expect[i] {
					t.Errorf("stage[%d]: expected %q, got %q", i, tc.expect[i], result[i])
				}
			}
		})
	}
}

func TestApplyDefaults_MigratesLegacyStages(t *testing.T) {
	cfg := &Config{
		DefaultStages: []Stage{"commit", "push"},
		Repos: []RepoConfig{
			{
				Repo: "local",
				Hooks: []HookConfig{
					{ID: "test", Name: "t", Entry: "e", Language: "system", Stages: []Stage{"merge-commit"}},
					{ID: "test2", Name: "t2", Entry: "e2", Language: "system"},
				},
			},
		},
	}

	cfg.ApplyDefaults()

	if cfg.DefaultStages[0] != HookTypePreCommit || cfg.DefaultStages[1] != HookTypePrePush {
		t.Errorf("default stages not migrated: %v", cfg.DefaultStages)
	}
	if cfg.Repos[0].Hooks[0].Stages[0] != HookTypePreMergeCommit {
		t.Errorf("hook stages not migrated: %v", cfg.Repos[0].Hooks[0].Stages)
	}
	// Hook without stages should get migrated default stages.
	if cfg.Repos[0].Hooks[1].Stages[0] != HookTypePreCommit {
		t.Errorf("hook without stages should get migrated defaults: %v", cfg.Repos[0].Hooks[1].Stages)
	}
}

// --- CheckMinimumVersion tests ---

func TestCheckMinimumVersion(t *testing.T) {
	tests := []struct {
		name       string
		minVersion string
		want       bool
	}{
		{name: "equal version", minVersion: Version, want: true},
		{name: "lower requirement", minVersion: "0.0.1", want: true},
		{name: "higher requirement", minVersion: "99.0.0", want: false},
		{name: "higher minor", minVersion: "4.99.0", want: false},
		{name: "higher patch", minVersion: "4.5.999", want: false},
		{name: "fewer parts in requirement", minVersion: "4.5", want: true},
		{name: "more parts all zero", minVersion: "4.5.0.0", want: false},
		{name: "more parts higher", minVersion: "4.5.0.1", want: false},
		{name: "zero requirement", minVersion: "0.0.0", want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckMinimumVersion(tc.minVersion)
			if got != tc.want {
				t.Errorf("CheckMinimumVersion(%q) = %v, want %v (Version=%s)", tc.minVersion, got, tc.want, Version)
			}
		})
	}
}

// --- LoadManifest tests ---

func TestLoadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.yaml")
	content := `- id: trailing-whitespace
  name: Trim Trailing Whitespace
  entry: trailing-whitespace-fixer
  language: python
  types: [text]
- id: end-of-file-fixer
  name: Fix End of Files
  entry: end-of-file-fixer
  language: python
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks))
	}
	if hooks[0].ID != "trailing-whitespace" {
		t.Errorf("unexpected hook id: %s", hooks[0].ID)
	}
	if hooks[0].Name != "Trim Trailing Whitespace" {
		t.Errorf("unexpected hook name: %s", hooks[0].Name)
	}
	if hooks[1].ID != "end-of-file-fixer" {
		t.Errorf("unexpected hook id: %s", hooks[1].ID)
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	_, err := LoadManifest("/nonexistent/path/hooks.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadManifest_MissingFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "missing id",
			content: "- name: Test\n  entry: echo\n  language: system\n",
			wantErr: "missing required 'id' field",
		},
		{
			name:    "missing name",
			content: "- id: test\n  entry: echo\n  language: system\n",
			wantErr: "missing required 'name' field",
		},
		{
			name:    "missing entry",
			content: "- id: test\n  name: Test\n  language: system\n",
			wantErr: "missing required 'entry' field",
		},
		{
			name:    "missing language",
			content: "- id: test\n  name: Test\n  entry: echo\n",
			wantErr: "missing required 'language' field",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "hooks.yaml")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadManifest(path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// --- SampleConfig tests ---

func TestSampleConfig_NonEmpty(t *testing.T) {
	s := SampleConfig()
	if s == "" {
		t.Fatal("SampleConfig returned empty string")
	}
	if !strings.Contains(s, "repos:") {
		t.Error("SampleConfig should contain 'repos:'")
	}
	if !strings.Contains(s, "repo:") {
		t.Error("SampleConfig should contain 'repo:'")
	}
	if !strings.Contains(s, "rev:") {
		t.Error("SampleConfig should contain 'rev:'")
	}
	if !strings.Contains(s, "hooks:") {
		t.Error("SampleConfig should contain 'hooks:'")
	}
}

// --- DefaultConfig tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if len(cfg.DefaultInstallHookTypes) != 1 || cfg.DefaultInstallHookTypes[0] != HookTypePreCommit {
		t.Errorf("unexpected default_install_hook_types: %v", cfg.DefaultInstallHookTypes)
	}
	if cfg.DefaultLanguageVersion == nil {
		t.Error("DefaultLanguageVersion should not be nil")
	}
	if cfg.Exclude != "^$" {
		t.Errorf("expected exclude '^$', got %q", cfg.Exclude)
	}
	hookTypes := AllHookTypes()
	if len(cfg.DefaultStages) != len(hookTypes) {
		t.Errorf("expected %d default stages (non-manual), got %d", len(hookTypes), len(cfg.DefaultStages))
	}
}

// --- WarnMutableRev tests ---

func TestWarnMutableRev(t *testing.T) {
	// WarnMutableRev writes to stderr for mutable refs.
	// We capture stderr to verify the warning is emitted.
	mutableRefs := []string{"main", "master", "develop", "HEAD"}
	for _, ref := range mutableRefs {
		t.Run(ref, func(t *testing.T) {
			// Redirect stderr.
			oldStderr := os.Stderr
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			os.Stderr = w

			WarnMutableRev("https://github.com/example/repo", ref)

			w.Close()
			os.Stderr = oldStderr

			buf := make([]byte, 4096)
			n, _ := r.Read(buf)
			r.Close()
			output := string(buf[:n])

			if !strings.Contains(output, "WARNING") {
				t.Errorf("expected WARNING in output for ref %q, got: %q", ref, output)
			}
			if !strings.Contains(output, "mutable reference") {
				t.Errorf("expected 'mutable reference' in output for ref %q, got: %q", ref, output)
			}
		})
	}

	// Non-mutable refs should produce no warning.
	t.Run("pinned_ref_no_warning", func(t *testing.T) {
		oldStderr := os.Stderr
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		os.Stderr = w

		WarnMutableRev("https://github.com/example/repo", "v1.2.3")

		w.Close()
		os.Stderr = oldStderr

		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		r.Close()
		output := string(buf[:n])

		if len(output) > 0 {
			t.Errorf("expected no output for pinned ref, got: %q", output)
		}
	})
}

// --- AllStages / AllHookTypes tests ---

func TestManualStageInAllStagesButNotAllHookTypes(t *testing.T) {
	hookTypes := AllHookTypes()
	for _, ht := range hookTypes {
		if ht == StageManual {
			t.Error("StageManual should not be in AllHookTypes()")
		}
	}

	stages := AllStages()
	found := false
	for _, s := range stages {
		if s == StageManual {
			found = true
			break
		}
	}
	if !found {
		t.Error("StageManual should be in AllStages()")
	}

	// AllStages should have exactly one more element than AllHookTypes.
	if len(stages) != len(hookTypes)+1 {
		t.Errorf("AllStages() has %d entries, AllHookTypes() has %d; expected difference of 1",
			len(stages), len(hookTypes))
	}
}

// --- ManifestHook.DefaultPassFilenames tests ---

func TestManifestHook_DefaultPassFilenames(t *testing.T) {
	h := ManifestHook{}
	if !h.DefaultPassFilenames() {
		t.Error("DefaultPassFilenames should return true when PassFilenames is nil")
	}

	f := false
	h.PassFilenames = &f
	if h.DefaultPassFilenames() {
		t.Error("DefaultPassFilenames should return false when set to false")
	}

	tr := true
	h.PassFilenames = &tr
	if !h.DefaultPassFilenames() {
		t.Error("DefaultPassFilenames should return true when set to true")
	}
}

// --- RepoConfig helper tests ---

func TestRepoConfig_IsLocal(t *testing.T) {
	r := RepoConfig{Repo: "local"}
	if !r.IsLocal() {
		t.Error("expected IsLocal() to be true")
	}
	r.Repo = "https://github.com/example/repo"
	if r.IsLocal() {
		t.Error("expected IsLocal() to be false")
	}
}

func TestRepoConfig_IsMeta(t *testing.T) {
	r := RepoConfig{Repo: "meta"}
	if !r.IsMeta() {
		t.Error("expected IsMeta() to be true")
	}
	r.Repo = "local"
	if r.IsMeta() {
		t.Error("expected IsMeta() to be false")
	}
}

// --- LoadConfig integration: minimum version enforcement ---

func TestLoadConfig_MinimumVersionTooHigh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `minimum_pre_commit_version: "99.0.0"
repos:
-   repo: https://github.com/example/repo
    rev: v1.0.0
    hooks:
    -   id: test
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for high minimum version")
	}
	if !strings.Contains(err.Error(), "is required but version") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadConfig_MinimumVersionSatisfied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `minimum_pre_commit_version: "0.0.1"
repos:
-   repo: https://github.com/example/repo
    rev: v1.0.0
    hooks:
    -   id: test
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

// --- Validate: missing hooks array ---

func TestValidate_MissingHooks(t *testing.T) {
	cfg := &Config{
		Repos: []RepoConfig{
			{Repo: "https://github.com/example/repo", Rev: "v1.0.0"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing hooks")
	}
	if !strings.Contains(err.Error(), "'hooks' is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Validate: missing hook ID ---

func TestValidate_MissingHookID(t *testing.T) {
	cfg := &Config{
		Repos: []RepoConfig{
			{
				Repo:  "https://github.com/example/repo",
				Rev:   "v1.0.0",
				Hooks: []HookConfig{{ID: ""}},
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing hook ID")
	}
	if !strings.Contains(err.Error(), "'id' is required") {
		t.Errorf("unexpected error: %v", err)
	}
}
