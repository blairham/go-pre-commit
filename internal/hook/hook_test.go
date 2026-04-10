package hook

import (
	"os"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/internal/config"
)

// ---------------------------------------------------------------------------
// MatchesFiles
// ---------------------------------------------------------------------------

func TestMatchesFiles(t *testing.T) {
	tests := []struct {
		name     string
		files    string // include pattern
		exclude  string // exclude pattern
		filename string
		want     bool
	}{
		{
			name:     "empty patterns match everything",
			files:    "",
			exclude:  "",
			filename: "anything.txt",
			want:     true,
		},
		{
			name:     "include pattern matches",
			files:    `\.py$`,
			filename: "foo.py",
			want:     true,
		},
		{
			name:     "include pattern does not match",
			files:    `\.py$`,
			filename: "foo.js",
			want:     false,
		},
		{
			name:     "exclude pattern filters out matched file",
			files:    `\.py$`,
			exclude:  `test_`,
			filename: "test_foo.py",
			want:     false,
		},
		{
			name:     "exclude pattern does not filter non-matching file",
			files:    `\.py$`,
			exclude:  `test_`,
			filename: "foo.py",
			want:     true,
		},
		{
			name:     "exclude only, no include",
			files:    "",
			exclude:  `vendor/`,
			filename: "vendor/lib.go",
			want:     false,
		},
		{
			name:     "exclude only, file not excluded",
			files:    "",
			exclude:  `vendor/`,
			filename: "src/lib.go",
			want:     true,
		},
		{
			name:     "PCRE lookahead pattern",
			files:    `(?=.*\.go$)(?=.*_test)`,
			filename: "foo_test.go",
			want:     true,
		},
		{
			name:     "PCRE lookahead pattern no match",
			files:    `(?=.*\.go$)(?=.*_test)`,
			filename: "foo.go",
			want:     false,
		},
		{
			name:     "partial match in filename",
			files:    `src/`,
			filename: "src/main.go",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hook{
				Files:   tt.files,
				Exclude: tt.exclude,
			}
			got := h.MatchesFiles(tt.filename)
			if got != tt.want {
				t.Errorf("MatchesFiles(%q) = %v, want %v (files=%q, exclude=%q)",
					tt.filename, got, tt.want, tt.files, tt.exclude)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// MatchesStage
// ---------------------------------------------------------------------------

func TestMatchesStage(t *testing.T) {
	tests := []struct {
		name   string
		stages []config.Stage
		stage  config.Stage
		want   bool
	}{
		{
			name:   "empty stages matches pre-commit",
			stages: nil,
			stage:  config.HookTypePreCommit,
			want:   true,
		},
		{
			name:   "empty stages matches pre-push",
			stages: nil,
			stage:  config.HookTypePrePush,
			want:   true,
		},
		{
			name:   "empty stages does not match manual",
			stages: nil,
			stage:  config.StageManual,
			want:   false,
		},
		{
			name:   "explicit stage matches",
			stages: []config.Stage{config.HookTypePreCommit},
			stage:  config.HookTypePreCommit,
			want:   true,
		},
		{
			name:   "explicit stage does not match different stage",
			stages: []config.Stage{config.HookTypePreCommit},
			stage:  config.HookTypePrePush,
			want:   false,
		},
		{
			name:   "manual in stages matches manual",
			stages: []config.Stage{config.StageManual},
			stage:  config.StageManual,
			want:   true,
		},
		{
			name:   "manual-only does not match pre-commit",
			stages: []config.Stage{config.StageManual},
			stage:  config.HookTypePreCommit,
			want:   false,
		},
		{
			name:   "multiple stages matches one",
			stages: []config.Stage{config.HookTypePreCommit, config.HookTypePrePush},
			stage:  config.HookTypePrePush,
			want:   true,
		},
		{
			name:   "multiple stages does not match unlisted",
			stages: []config.Stage{config.HookTypePreCommit, config.HookTypePrePush},
			stage:  config.HookTypeCommitMsg,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Hook{Stages: tt.stages}
			got := h.MatchesStage(tt.stage)
			if got != tt.want {
				t.Errorf("MatchesStage(%q) = %v, want %v", tt.stage, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InstallKey
// ---------------------------------------------------------------------------

func TestInstallKey(t *testing.T) {
	t.Run("includes all relevant fields", func(t *testing.T) {
		h := &Hook{
			RepoDir:                "/tmp/repo",
			Language:               "python",
			LanguageVersion:        "3.11",
			AdditionalDependencies: []string{"foo", "bar"},
		}
		key := h.InstallKey()
		if !strings.Contains(key, "/tmp/repo") {
			t.Error("InstallKey missing RepoDir")
		}
		if !strings.Contains(key, "python") {
			t.Error("InstallKey missing Language")
		}
		if !strings.Contains(key, "3.11") {
			t.Error("InstallKey missing LanguageVersion")
		}
		if !strings.Contains(key, "foo,bar") {
			t.Error("InstallKey missing AdditionalDependencies")
		}
	})

	t.Run("different deps produce different keys", func(t *testing.T) {
		h1 := &Hook{
			RepoDir:                "/tmp/repo",
			Language:               "python",
			LanguageVersion:        "3.11",
			AdditionalDependencies: []string{"foo"},
		}
		h2 := &Hook{
			RepoDir:                "/tmp/repo",
			Language:               "python",
			LanguageVersion:        "3.11",
			AdditionalDependencies: []string{"bar"},
		}
		if h1.InstallKey() == h2.InstallKey() {
			t.Error("expected different InstallKeys for different deps")
		}
	})

	t.Run("same fields produce same key", func(t *testing.T) {
		h1 := &Hook{
			RepoDir:         "/tmp/repo",
			Language:        "golang",
			LanguageVersion: "1.21",
		}
		h2 := &Hook{
			RepoDir:         "/tmp/repo",
			Language:        "golang",
			LanguageVersion: "1.21",
		}
		if h1.InstallKey() != h2.InstallKey() {
			t.Errorf("expected same InstallKey, got %q vs %q", h1.InstallKey(), h2.InstallKey())
		}
	})

	t.Run("empty deps produces empty dep segment", func(t *testing.T) {
		h := &Hook{
			RepoDir:         "/tmp/repo",
			Language:        "python",
			LanguageVersion: "3.11",
		}
		key := h.InstallKey()
		want := "/tmp/repo:python:3.11:"
		if key != want {
			t.Errorf("InstallKey() = %q, want %q", key, want)
		}
	})
}

// ---------------------------------------------------------------------------
// MergeManifest
// ---------------------------------------------------------------------------

func TestMergeManifest(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	t.Run("manifest values used as base", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "My Hook",
			Entry:    "my-hook-entry",
			Language: "python",
			Files:    `\.py$`,
		}
		hookCfg := &config.HookConfig{ID: "my-hook"}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}

		h := MergeManifest(manifest, hookCfg, repoCfg, nil)

		if h.ID != "my-hook" {
			t.Errorf("ID = %q, want %q", h.ID, "my-hook")
		}
		if h.Name != "My Hook" {
			t.Errorf("Name = %q, want %q", h.Name, "My Hook")
		}
		if h.Entry != "my-hook-entry" {
			t.Errorf("Entry = %q, want %q", h.Entry, "my-hook-entry")
		}
		if h.Language != "python" {
			t.Errorf("Language = %q, want %q", h.Language, "python")
		}
		if h.Files != `\.py$` {
			t.Errorf("Files = %q, want %q", h.Files, `\.py$`)
		}
		if h.Repo != "https://github.com/example/repo" {
			t.Errorf("Repo = %q, want %q", h.Repo, "https://github.com/example/repo")
		}
		if h.Rev != "v1.0.0" {
			t.Errorf("Rev = %q, want %q", h.Rev, "v1.0.0")
		}
	})

	t.Run("config overrides manifest values", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "Original Name",
			Entry:    "entry",
			Language: "python",
			Files:    `\.py$`,
		}
		hookCfg := &config.HookConfig{
			ID:                     "my-hook",
			Name:                   "Overridden Name",
			Alias:                  "mh",
			Files:                  `\.pyx$`,
			Exclude:                `test_`,
			LanguageVersion:        "3.12",
			Types:                  []string{"python"},
			TypesOr:                []string{"cython"},
			ExcludeTypes:           []string{"text"},
			Args:                   []string{"--fix"},
			Stages:                 []config.Stage{config.HookTypePrePush},
			AdditionalDependencies: []string{"dep1"},
			AlwaysRun:              boolPtr(true),
			Verbose:                boolPtr(true),
			PassFilenames:          boolPtr(false),
			RequireSerial:          boolPtr(true),
			FailFast:               boolPtr(true),
			LogFile:                "/tmp/log",
		}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}

		h := MergeManifest(manifest, hookCfg, repoCfg, nil)

		if h.Name != "Overridden Name" {
			t.Errorf("Name = %q, want %q", h.Name, "Overridden Name")
		}
		if h.Alias != "mh" {
			t.Errorf("Alias = %q, want %q", h.Alias, "mh")
		}
		if h.Files != `\.pyx$` {
			t.Errorf("Files = %q, want %q", h.Files, `\.pyx$`)
		}
		if h.Exclude != "test_" {
			t.Errorf("Exclude = %q, want %q", h.Exclude, "test_")
		}
		if h.LanguageVersion != "3.12" {
			t.Errorf("LanguageVersion = %q, want %q", h.LanguageVersion, "3.12")
		}
		if len(h.Types) != 1 || h.Types[0] != "python" {
			t.Errorf("Types = %v, want [python]", h.Types)
		}
		if len(h.TypesOr) != 1 || h.TypesOr[0] != "cython" {
			t.Errorf("TypesOr = %v, want [cython]", h.TypesOr)
		}
		if len(h.ExcludeTypes) != 1 || h.ExcludeTypes[0] != "text" {
			t.Errorf("ExcludeTypes = %v, want [text]", h.ExcludeTypes)
		}
		if len(h.Args) != 1 || h.Args[0] != "--fix" {
			t.Errorf("Args = %v, want [--fix]", h.Args)
		}
		if len(h.Stages) != 1 || h.Stages[0] != config.HookTypePrePush {
			t.Errorf("Stages = %v, want [pre-push]", h.Stages)
		}
		if len(h.AdditionalDependencies) != 1 || h.AdditionalDependencies[0] != "dep1" {
			t.Errorf("AdditionalDependencies = %v, want [dep1]", h.AdditionalDependencies)
		}
		if !h.AlwaysRun {
			t.Error("AlwaysRun = false, want true")
		}
		if !h.Verbose {
			t.Error("Verbose = false, want true")
		}
		if h.PassFilenames {
			t.Error("PassFilenames = true, want false")
		}
		if !h.RequireSerial {
			t.Error("RequireSerial = false, want true")
		}
		if !h.FailFast {
			t.Error("FailFast = false, want true")
		}
		if h.LogFile != "/tmp/log" {
			t.Errorf("LogFile = %q, want %q", h.LogFile, "/tmp/log")
		}
	})

	t.Run("PassFilenames defaults to true from manifest", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "python",
			// PassFilenames is nil, so DefaultPassFilenames() returns true
		}
		hookCfg := &config.HookConfig{ID: "my-hook"}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}

		h := MergeManifest(manifest, hookCfg, repoCfg, nil)
		if !h.PassFilenames {
			t.Error("PassFilenames = false, want true (default)")
		}
	})

	t.Run("Types defaults to file when no types or types_or", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "python",
		}
		hookCfg := &config.HookConfig{ID: "my-hook"}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}

		h := MergeManifest(manifest, hookCfg, repoCfg, nil)
		if len(h.Types) != 1 || h.Types[0] != "file" {
			t.Errorf("Types = %v, want [file]", h.Types)
		}
	})

	t.Run("Types default not applied when TypesOr set", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "python",
			TypesOr:  []string{"python", "cython"},
		}
		hookCfg := &config.HookConfig{ID: "my-hook"}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}

		h := MergeManifest(manifest, hookCfg, repoCfg, nil)
		if len(h.Types) != 0 {
			t.Errorf("Types = %v, want empty (TypesOr is set)", h.Types)
		}
	})

	t.Run("LanguageVersion defaults to default", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "python",
		}
		hookCfg := &config.HookConfig{ID: "my-hook"}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}

		h := MergeManifest(manifest, hookCfg, repoCfg, nil)
		if h.LanguageVersion != "default" {
			t.Errorf("LanguageVersion = %q, want %q", h.LanguageVersion, "default")
		}
	})

	t.Run("global config default_language_version applied", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "python",
		}
		hookCfg := &config.HookConfig{ID: "my-hook"}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}
		globalCfg := &config.Config{
			DefaultLanguageVersion: map[string]string{"python": "3.10"},
		}

		h := MergeManifest(manifest, hookCfg, repoCfg, globalCfg)
		if h.LanguageVersion != "3.10" {
			t.Errorf("LanguageVersion = %q, want %q", h.LanguageVersion, "3.10")
		}
	})

	t.Run("global config default_stages applied when hook has none", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "python",
		}
		hookCfg := &config.HookConfig{ID: "my-hook"}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}
		globalCfg := &config.Config{
			DefaultStages: []config.Stage{config.HookTypePreCommit, config.HookTypePrePush},
		}

		h := MergeManifest(manifest, hookCfg, repoCfg, globalCfg)
		if len(h.Stages) != 2 {
			t.Errorf("Stages = %v, want [pre-commit pre-push]", h.Stages)
		}
	})

	t.Run("hook-level stages override global default_stages", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "my-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "python",
		}
		hookCfg := &config.HookConfig{
			ID:     "my-hook",
			Stages: []config.Stage{config.HookTypeCommitMsg},
		}
		repoCfg := &config.RepoConfig{Repo: "https://github.com/example/repo", Rev: "v1.0.0"}
		globalCfg := &config.Config{
			DefaultStages: []config.Stage{config.HookTypePreCommit},
		}

		h := MergeManifest(manifest, hookCfg, repoCfg, globalCfg)
		if len(h.Stages) != 1 || h.Stages[0] != config.HookTypeCommitMsg {
			t.Errorf("Stages = %v, want [commit-msg]", h.Stages)
		}
	})
}

// ---------------------------------------------------------------------------
// FromLocalConfig
// ---------------------------------------------------------------------------

func TestFromLocalConfig(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	t.Run("all fields set correctly", func(t *testing.T) {
		hookCfg := &config.HookConfig{
			ID:                     "local-hook",
			Name:                   "Local Hook",
			Entry:                  "echo hello",
			Language:               "system",
			LanguageVersion:        "default",
			Files:                  `\.go$`,
			Exclude:                `_test\.go$`,
			Types:                  []string{"go"},
			TypesOr:                []string{"golang"},
			ExcludeTypes:           []string{"text"},
			Args:                   []string{"--verbose"},
			Stages:                 []config.Stage{config.HookTypePreCommit},
			AdditionalDependencies: []string{"dep1"},
			AlwaysRun:              boolPtr(true),
			Verbose:                boolPtr(true),
			PassFilenames:          boolPtr(false),
			RequireSerial:          boolPtr(true),
			FailFast:               boolPtr(true),
			Description:            "A local hook",
			LogFile:                "/tmp/hook.log",
		}

		h := FromLocalConfig(hookCfg, nil)

		if h.ID != "local-hook" {
			t.Errorf("ID = %q, want %q", h.ID, "local-hook")
		}
		if h.Name != "Local Hook" {
			t.Errorf("Name = %q, want %q", h.Name, "Local Hook")
		}
		if h.Entry != "echo hello" {
			t.Errorf("Entry = %q, want %q", h.Entry, "echo hello")
		}
		if h.Language != "system" {
			t.Errorf("Language = %q, want %q", h.Language, "system")
		}
		if h.Repo != "local" {
			t.Errorf("Repo = %q, want %q", h.Repo, "local")
		}
		if h.Files != `\.go$` {
			t.Errorf("Files = %q, want %q", h.Files, `\.go$`)
		}
		if h.Exclude != `_test\.go$` {
			t.Errorf("Exclude = %q, want %q", h.Exclude, `_test\.go$`)
		}
		if len(h.Types) != 1 || h.Types[0] != "go" {
			t.Errorf("Types = %v, want [go]", h.Types)
		}
		if len(h.TypesOr) != 1 || h.TypesOr[0] != "golang" {
			t.Errorf("TypesOr = %v, want [golang]", h.TypesOr)
		}
		if len(h.ExcludeTypes) != 1 || h.ExcludeTypes[0] != "text" {
			t.Errorf("ExcludeTypes = %v, want [text]", h.ExcludeTypes)
		}
		if len(h.Args) != 1 || h.Args[0] != "--verbose" {
			t.Errorf("Args = %v, want [--verbose]", h.Args)
		}
		if len(h.Stages) != 1 || h.Stages[0] != config.HookTypePreCommit {
			t.Errorf("Stages = %v, want [pre-commit]", h.Stages)
		}
		if len(h.AdditionalDependencies) != 1 || h.AdditionalDependencies[0] != "dep1" {
			t.Errorf("AdditionalDependencies = %v, want [dep1]", h.AdditionalDependencies)
		}
		if !h.AlwaysRun {
			t.Error("AlwaysRun = false, want true")
		}
		if !h.Verbose {
			t.Error("Verbose = false, want true")
		}
		if h.PassFilenames {
			t.Error("PassFilenames = true, want false")
		}
		if !h.RequireSerial {
			t.Error("RequireSerial = false, want true")
		}
		if !h.FailFast {
			t.Error("FailFast = false, want true")
		}
		if h.Description != "A local hook" {
			t.Errorf("Description = %q, want %q", h.Description, "A local hook")
		}
		if h.LogFile != "/tmp/hook.log" {
			t.Errorf("LogFile = %q, want %q", h.LogFile, "/tmp/hook.log")
		}
	})

	t.Run("PassFilenames defaults to true", func(t *testing.T) {
		hookCfg := &config.HookConfig{
			ID:       "local-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "system",
		}

		h := FromLocalConfig(hookCfg, nil)
		if !h.PassFilenames {
			t.Error("PassFilenames = false, want true (default)")
		}
	})

	t.Run("Types defaults to file when no types or types_or", func(t *testing.T) {
		hookCfg := &config.HookConfig{
			ID:       "local-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "system",
		}

		h := FromLocalConfig(hookCfg, nil)
		if len(h.Types) != 1 || h.Types[0] != "file" {
			t.Errorf("Types = %v, want [file]", h.Types)
		}
	})

	t.Run("Types default not applied when TypesOr set", func(t *testing.T) {
		hookCfg := &config.HookConfig{
			ID:       "local-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "system",
			TypesOr:  []string{"python"},
		}

		h := FromLocalConfig(hookCfg, nil)
		if len(h.Types) != 0 {
			t.Errorf("Types = %v, want empty (TypesOr is set)", h.Types)
		}
	})

	t.Run("LanguageVersion defaults to default", func(t *testing.T) {
		hookCfg := &config.HookConfig{
			ID:       "local-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "system",
		}

		h := FromLocalConfig(hookCfg, nil)
		if h.LanguageVersion != "default" {
			t.Errorf("LanguageVersion = %q, want %q", h.LanguageVersion, "default")
		}
	})

	t.Run("global config applied", func(t *testing.T) {
		hookCfg := &config.HookConfig{
			ID:       "local-hook",
			Name:     "Hook",
			Entry:    "entry",
			Language: "python",
		}
		globalCfg := &config.Config{
			DefaultLanguageVersion: map[string]string{"python": "3.9"},
			DefaultStages:          []config.Stage{config.HookTypePrePush},
		}

		h := FromLocalConfig(hookCfg, globalCfg)
		if h.LanguageVersion != "3.9" {
			t.Errorf("LanguageVersion = %q, want %q", h.LanguageVersion, "3.9")
		}
		if len(h.Stages) != 1 || h.Stages[0] != config.HookTypePrePush {
			t.Errorf("Stages = %v, want [pre-push]", h.Stages)
		}
	})
}

// ---------------------------------------------------------------------------
// FromManifestHook
// ---------------------------------------------------------------------------

func TestFromManifestHook(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	t.Run("basic creation", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:                      "check-yaml",
			Name:                    "Check YAML",
			Entry:                   "check-yaml",
			Language:                "python",
			LanguageVersion:         "3.11",
			Files:                   `\.yaml$`,
			Exclude:                 `^vendor/`,
			Types:                   []string{"yaml"},
			Args:                    []string{"--unsafe"},
			Stages:                  []config.Stage{config.HookTypePreCommit},
			AlwaysRun:               true,
			FailFast:                true,
			Verbose:                 true,
			RequireSerial:           true,
			PassFilenames:           boolPtr(false),
			Description:             "Check YAML files",
			MinimumPreCommitVersion: "2.0.0",
		}

		h := FromManifestHook(manifest)

		if h.ID != "check-yaml" {
			t.Errorf("ID = %q, want %q", h.ID, "check-yaml")
		}
		if h.Name != "Check YAML" {
			t.Errorf("Name = %q, want %q", h.Name, "Check YAML")
		}
		if h.Entry != "check-yaml" {
			t.Errorf("Entry = %q, want %q", h.Entry, "check-yaml")
		}
		if h.Language != "python" {
			t.Errorf("Language = %q, want %q", h.Language, "python")
		}
		if h.LanguageVersion != "3.11" {
			t.Errorf("LanguageVersion = %q, want %q", h.LanguageVersion, "3.11")
		}
		if h.Files != `\.yaml$` {
			t.Errorf("Files = %q, want %q", h.Files, `\.yaml$`)
		}
		if h.Exclude != `^vendor/` {
			t.Errorf("Exclude = %q, want %q", h.Exclude, `^vendor/`)
		}
		if len(h.Types) != 1 || h.Types[0] != "yaml" {
			t.Errorf("Types = %v, want [yaml]", h.Types)
		}
		if len(h.Args) != 1 || h.Args[0] != "--unsafe" {
			t.Errorf("Args = %v, want [--unsafe]", h.Args)
		}
		if !h.AlwaysRun {
			t.Error("AlwaysRun = false, want true")
		}
		if !h.FailFast {
			t.Error("FailFast = false, want true")
		}
		if !h.Verbose {
			t.Error("Verbose = false, want true")
		}
		if !h.RequireSerial {
			t.Error("RequireSerial = false, want true")
		}
		if h.PassFilenames {
			t.Error("PassFilenames = true, want false")
		}
		if h.Description != "Check YAML files" {
			t.Errorf("Description = %q, want %q", h.Description, "Check YAML files")
		}
		if h.MinimumPreCommitVersion != "2.0.0" {
			t.Errorf("MinimumPreCommitVersion = %q, want %q", h.MinimumPreCommitVersion, "2.0.0")
		}
	})

	t.Run("defaults applied", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "minimal-hook",
			Name:     "Minimal",
			Entry:    "minimal",
			Language: "system",
		}

		h := FromManifestHook(manifest)

		// Types defaults to ["file"]
		if len(h.Types) != 1 || h.Types[0] != "file" {
			t.Errorf("Types = %v, want [file]", h.Types)
		}
		// LanguageVersion defaults to "default"
		if h.LanguageVersion != "default" {
			t.Errorf("LanguageVersion = %q, want %q", h.LanguageVersion, "default")
		}
		// PassFilenames defaults to true
		if !h.PassFilenames {
			t.Error("PassFilenames = false, want true (default)")
		}
	})

	t.Run("Types default not applied when TypesOr set", func(t *testing.T) {
		manifest := &config.ManifestHook{
			ID:       "hook",
			Name:     "Hook",
			Entry:    "hook",
			Language: "python",
			TypesOr:  []string{"python", "pyi"},
		}

		h := FromManifestHook(manifest)
		if len(h.Types) != 0 {
			t.Errorf("Types = %v, want empty (TypesOr is set)", h.Types)
		}
	})
}

// ---------------------------------------------------------------------------
// filterFiles
// ---------------------------------------------------------------------------

func TestFilterFiles_SkipsNonExistentFiles(t *testing.T) {
	// Create a real file.
	dir := t.TempDir()
	realFile := dir + "/exists.yaml"
	if err := os.WriteFile(realFile, []byte("key: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ghostFile := dir + "/ghost.yaml"

	h := &Hook{
		Types: []string{"file"},
	}

	result := filterFiles([]string{realFile, ghostFile}, h)

	if len(result) != 1 {
		t.Fatalf("expected 1 file, got %d: %v", len(result), result)
	}
	if result[0] != realFile {
		t.Errorf("expected %q, got %q", realFile, result[0])
	}
}
