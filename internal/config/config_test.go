// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"io"
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
	if want := "\n=====> /nonexistent/path/config.yaml is not a file"; err.Error() != want {
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
	// What pre-commit 4.7.0 (PyYAML's libyaml loader) prints for it.
	want := "\n==> File " + path + "\n=====> while parsing a flow mapping\n" +
		"  in \"<unicode string>\", line 1, column 4\n" +
		"did not find expected ',' or '}'\n" +
		"  in \"<unicode string>\", line 2, column 1"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
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
	if want := "\n==> File " + path + "\n==> At Config()\n=====> Missing required key: repos"; err.Error() != want {
		t.Errorf("unexpected error message: %v", err)
	}
}

// Upstream's schema is cfgv.Required('repos', check_array(...)): an empty
// list satisfies it, while a missing or null key does not.
func TestLoadConfig_EmptyRepos(t *testing.T) {
	for _, tc := range []struct {
		content string
		wantErr bool
	}{
		{"repos: []\n", false},
		{"repos:\n", true},
		{"fail_fast: true\n", true},
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(path)
		if (err != nil) != tc.wantErr {
			t.Errorf("LoadConfig(%q) error = %v, want error %v", tc.content, err, tc.wantErr)
			continue
		}
		if err == nil && len(cfg.Repos) != 0 {
			t.Errorf("LoadConfig(%q) repos = %v, want none", tc.content, cfg.Repos)
		}
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
		t.Errorf(
			"hook with explicit language_version should not be overwritten, got %s",
			cfg.Repos[0].Hooks[1].LanguageVersion,
		)
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
		{name: "lower minor", minVersion: "4.6.999", want: true},
		{name: "higher patch", minVersion: "4.7.999", want: false},
		{name: "fewer parts in requirement", minVersion: "4.7", want: true},
		{name: "more parts all zero", minVersion: "4.7.0.0", want: false},
		{name: "more parts higher", minVersion: "4.7.0.1", want: false},
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

	// Dev builds inject `git describe` output with a leading "v"; the check
	// must still parse the numeric components.
	t.Run("git describe dev version", func(t *testing.T) {
		orig := Version
		Version = "v4.6.0-1-gabc1234-dirty"
		defer func() { Version = orig }()

		if !CheckMinimumVersion("3.2.0") {
			t.Error("CheckMinimumVersion(\"3.2.0\") = false with dev version, want true")
		}
		if CheckMinimumVersion("999.0.0") {
			t.Error("CheckMinimumVersion(\"999.0.0\") = true with dev version, want false")
		}
	})
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
			wantErr: "==> At Hook(id=MISSING)\n=====> Missing required key: id",
		},
		{
			name:    "missing name",
			content: "- id: test\n  entry: echo\n  language: system\n",
			wantErr: "==> At Hook(id='test')\n=====> Missing required key: name",
		},
		{
			name:    "missing entry",
			content: "- id: test\n  name: Test\n  language: system\n",
			wantErr: "==> At Hook(id='test')\n=====> Missing required key: entry",
		},
		{
			name:    "missing language",
			content: "- id: test\n  name: Test\n  entry: echo\n",
			wantErr: "==> At Hook(id='test')\n=====> Missing required key: language",
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

func TestTransformStage(t *testing.T) {
	for in, want := range map[Stage]Stage{
		"commit":       HookTypePreCommit,
		"merge-commit": HookTypePreMergeCommit,
		"push":         HookTypePrePush,
		"pre-push":     HookTypePrePush,
		"manual":       StageManual,
	} {
		if got := TransformStage(in); got != want {
			t.Errorf("TransformStage(%q) = %q, want %q", in, got, want)
		}
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

// captureStdout runs fn with os.Stdout redirected and returns what it wrote,
// with color forced off so the bytes are upstream's uncolored log line.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	t.Setenv("PRE_COMMIT_COLOR", "never")
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	w.Close()
	data, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWarnMutableRev(t *testing.T) {
	const repo = "https://github.com/example/repo"
	want := "[WARNING] The 'rev' field of repo 'https://github.com/example/repo' " +
		"appears to be a mutable reference (moving tag / branch).  Mutable " +
		"references are never updated after first install and are not " +
		"supported.  See https://pre-commit.com/#using-the-latest-version-for-a-repository " +
		"for more details.  Hint: `pre-commit autoupdate` often fixes this.\n"

	// Upstream's rule: warn when there is no '.' and the rev is not all hex.
	cases := []struct {
		rev  string
		warn bool
	}{
		{"main", true},
		{"master", true},
		{"HEAD", true},
		{"stable", true},
		{"v1", true},
		{"v1.2.3", false},
		{"1.0", false},
		{"release.candidate", false},
		{"deadbeef", false},
		{"DEADBEEF", false},
		{"0123456789abcdef0123456789abcdef01234567", false},
		{"cafe-1", true},
	}
	for _, tc := range cases {
		t.Run(tc.rev, func(t *testing.T) {
			got := captureStdout(t, func() { WarnMutableRev(repo, tc.rev) })
			if tc.warn && got != want {
				t.Errorf("rev %q:\n got: %q\nwant: %q", tc.rev, got, want)
			}
			if !tc.warn && got != "" {
				t.Errorf("rev %q: expected no warning, got %q", tc.rev, got)
			}
		})
	}
}

func TestWarnMutableRevQuotesRepoLikePython(t *testing.T) {
	got := captureStdout(t, func() { WarnMutableRev("/tmp/it's", "v1") })
	if !strings.Contains(got, `repo "/tmp/it's" appears`) {
		t.Errorf("repo not rendered as Python repr: %q", got)
	}
}

func TestLoadConfigWarnsMutableRevForPathRepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".pre-commit-config.yaml")
	cfg := "repos:\n" +
		"-   repo: /some/path\n    rev: v1\n    hooks:\n    -   id: a\n" +
		"-   repo: local\n    hooks:\n    -   id: b\n        name: b\n        entry: b\n        language: system\n" +
		"-   repo: https://example.com/r\n    rev: v1.0.0\n    hooks:\n    -   id: c\n"
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	var err error
	got := captureStdout(t, func() { _, err = LoadConfig(path) })
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(got, "[WARNING]"); n != 1 || !strings.Contains(got, "'/some/path'") {
		t.Errorf("want exactly one warning, for /some/path; got %q", got)
	}
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

// --- Deprecated stage-name warnings ---

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigWarnsDeprecatedStages(t *testing.T) {
	path := writeTemp(t, ".pre-commit-config.yaml", "default_stages: [commit, push]\n"+
		"repos:\n"+
		"-   repo: https://example.com/r\n    rev: v1\n    hooks:\n"+
		"    -   id: a\n        stages: [push, pre-commit, commit, push]\n"+
		"    -   id: b\n        stages: [pre-push]\n"+
		"-   repo: local\n    hooks:\n"+
		"    -   id: c\n        name: c\n        entry: c\n        language: system\n        stages: [merge-commit]\n")
	var cfg *Config
	var err error
	got := captureStdout(t, func() { cfg, err = LoadConfig(path) })
	if err != nil {
		t.Fatal(err)
	}
	const tail = " which will be removed in a future version.  " +
		"run: `pre-commit migrate-config` to automatically fix this.\n"
	// Upstream's order: each repo's hooks, then its rev, then default_stages.
	want := "[WARNING] hook id `a` uses deprecated stage names (push, commit, push)" + tail +
		"[WARNING] The 'rev' field of repo 'https://example.com/r' appears to be a mutable reference " +
		"(moving tag / branch).  Mutable references are never updated after first install and are not " +
		"supported.  See https://pre-commit.com/#using-the-latest-version-for-a-repository for more " +
		"details.  Hint: `pre-commit autoupdate` often fixes this.\n" +
		"[WARNING] hook id `c` uses deprecated stage names (merge-commit)" + tail +
		"[WARNING] top-level `default_stages` uses deprecated stage names (commit, push)" + tail
	if got != want {
		t.Errorf("warnings:\n got: %q\nwant: %q", got, want)
	}

	// The old names still work: they are migrated after warning.
	if s := cfg.DefaultStages; len(s) != 2 || s[0] != HookTypePreCommit || s[1] != HookTypePrePush {
		t.Errorf("default_stages not migrated: %v", s)
	}
	if s := cfg.Repos[1].Hooks[0].Stages; len(s) != 1 || s[0] != HookTypePreMergeCommit {
		t.Errorf("hook stages not migrated: %v", s)
	}
}

func TestLoadConfigNoDeprecatedStagesNoWarning(t *testing.T) {
	path := writeTemp(t, ".pre-commit-config.yaml", "default_stages: [pre-commit]\n"+
		"repos:\n-   repo: https://example.com/r\n    rev: v1.0.0\n    hooks:\n"+
		"    -   id: a\n        stages: [pre-push, manual]\n")
	var err error
	got := captureStdout(t, func() { _, err = LoadConfig(path) })
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("expected no warnings, got %q", got)
	}
}

func TestLoadManifestMigratesLegacyStages(t *testing.T) {
	path := writeTemp(t, ManifestFile,
		"- id: a\n  name: a\n  entry: a\n  language: system\n  stages: [commit, merge-commit, push, manual]\n")
	hooks, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Stage{HookTypePreCommit, HookTypePreMergeCommit, HookTypePrePush, "manual"}
	if got := hooks[0].Stages; len(got) != len(want) {
		t.Fatalf("stages = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("stages = %v, want %v", got, want)
			}
		}
	}
}

func TestWarnForStagesOnRepoInit(t *testing.T) {
	cases := []struct {
		name, manifest, repo, want string
	}{
		{
			name:     "deduplicated in first-seen order",
			manifest: "- id: a\n  stages: [push, commit]\n- id: b\n  stages: [commit, pre-commit, merge-commit]\n- id: c\n",
			repo:     "https://example.com/r",
			want: "[WARNING] repo `https://example.com/r` uses deprecated stage names " +
				"(push, commit, merge-commit) which will be removed in a future version.  " +
				"Hint: often `pre-commit autoupdate --repo https://example.com/r` will fix this.  " +
				"if it does not -- consider reporting an issue to that repo.\n",
		},
		{
			name:     "repo shell-quoted in the hint",
			manifest: "- id: a\n  stages: [commit]\n",
			repo:     "/tmp/my repo",
			want: "[WARNING] repo `/tmp/my repo` uses deprecated stage names " +
				"(commit) which will be removed in a future version.  " +
				"Hint: often `pre-commit autoupdate --repo '/tmp/my repo'` will fix this.  " +
				"if it does not -- consider reporting an issue to that repo.\n",
		},
		{name: "current names", manifest: "- id: a\n  stages: [pre-commit, pre-push]\n", repo: "r"},
		{name: "no manifest", repo: "r"},
		{name: "invalid manifest", manifest: "- id: a\n  stages: commit\n", repo: "r"},
		{name: "hook without id", manifest: "- stages: [commit]\n", repo: "r"},
		{name: "not a list", manifest: "id: a\n", repo: "r"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.manifest != "" {
				if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(tc.manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := captureStdout(t, func() { WarnForStagesOnRepoInit(tc.repo, dir) })
			if got != tc.want {
				t.Errorf("\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestShlexQuote(t *testing.T) {
	for in, want := range map[string]string{
		"":                          "''",
		"https://example.com/r.git": "https://example.com/r.git",
		"a@b%c+d=e:f,g.h/i-j_k":     "a@b%c+d=e:f,g.h/i-j_k",
		"my repo":                   "'my repo'",
		"it's":                      `'it'"'"'s'`,
		"$HOME":                     "'$HOME'",
	} {
		if got := ShlexQuote(in); got != want {
			t.Errorf("ShlexQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// Upstream loads the config into a Python dict, so a repeated key is not
// an error: the last value wins (#167).
func TestLoadConfig_RepeatedKeyLastWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".pre-commit-config.yaml")
	const cfg = "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: first\n    entry: e\n    language: system\n    name: second\n"
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := c.Repos[0].Hooks[0].Name; got != "second" {
		t.Errorf("name = %q, want the last value, %q", got, "second")
	}
}
