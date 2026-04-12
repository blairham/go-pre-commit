package hook

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/internal/config"
)

// ---------------------------------------------------------------------------
// filterByIncludeExclude
// ---------------------------------------------------------------------------

func TestFilterByIncludeExclude(t *testing.T) {
	files := []string{"src/main.go", "src/main_test.go", "vendor/lib.go", "README.md"}

	tests := []struct {
		name    string
		include string
		exclude string
		want    []string
	}{
		{
			name: "no filters returns all",
			want: files,
		},
		{
			name:    "include only",
			include: `\.go$`,
			want:    []string{"src/main.go", "src/main_test.go", "vendor/lib.go"},
		},
		{
			name:    "exclude only",
			exclude: `vendor/`,
			want:    []string{"src/main.go", "src/main_test.go", "README.md"},
		},
		{
			name:    "include and exclude",
			include: `\.go$`,
			exclude: `_test\.go$`,
			want:    []string{"src/main.go", "vendor/lib.go"},
		},
		{
			name:    "include matches nothing",
			include: `\.rs$`,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterByIncludeExclude(files, tt.include, tt.exclude)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// filterFiles
// ---------------------------------------------------------------------------

func TestFilterFiles(t *testing.T) {
	dir := t.TempDir()

	// Create test files.
	for _, name := range []string{"foo.go", "bar.py", "baz_test.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files := []string{
		filepath.Join(dir, "foo.go"),
		filepath.Join(dir, "bar.py"),
		filepath.Join(dir, "baz_test.go"),
	}

	t.Run("include pattern filters", func(t *testing.T) {
		h := &Hook{Files: `\.go$`, Types: []string{"file"}}
		got := filterFiles(files, h)
		if len(got) != 2 {
			t.Fatalf("expected 2 files, got %d: %v", len(got), got)
		}
	})

	t.Run("exclude pattern filters", func(t *testing.T) {
		h := &Hook{Files: `\.go$`, Exclude: `_test\.go$`, Types: []string{"file"}}
		got := filterFiles(files, h)
		if len(got) != 1 {
			t.Fatalf("expected 1 file, got %d: %v", len(got), got)
		}
	})

	t.Run("no patterns matches all", func(t *testing.T) {
		h := &Hook{Types: []string{"file"}}
		got := filterFiles(files, h)
		if len(got) != 3 {
			t.Fatalf("expected 3 files, got %d: %v", len(got), got)
		}
	})
}

// ---------------------------------------------------------------------------
// targetConcurrency
// ---------------------------------------------------------------------------

func TestTargetConcurrency(t *testing.T) {
	t.Run("respects jobs parameter", func(t *testing.T) {
		got := targetConcurrency(4)
		if got != 4 {
			t.Errorf("targetConcurrency(4) = %d, want 4", got)
		}
	})

	t.Run("zero jobs falls back to CPU count", func(t *testing.T) {
		got := targetConcurrency(0)
		if got < 1 {
			t.Errorf("targetConcurrency(0) = %d, want >= 1", got)
		}
	})

	t.Run("NO_CONCURRENCY overrides everything", func(t *testing.T) {
		t.Setenv("PRE_COMMIT_NO_CONCURRENCY", "1")
		got := targetConcurrency(8)
		if got != 1 {
			t.Errorf("targetConcurrency(8) with NO_CONCURRENCY = %d, want 1", got)
		}
	})
}

// ---------------------------------------------------------------------------
// batchFileArgs
// ---------------------------------------------------------------------------

func TestBatchFileArgs(t *testing.T) {
	t.Run("single batch when under limit", func(t *testing.T) {
		files := []string{"a", "b", "c"}
		batches := batchFileArgs(files, 10)
		if len(batches) != 1 {
			t.Fatalf("expected 1 batch, got %d", len(batches))
		}
		if len(batches[0]) != 3 {
			t.Errorf("batch[0] len = %d, want 3", len(batches[0]))
		}
	})

	t.Run("splits into multiple batches", func(t *testing.T) {
		files := []string{"a", "b", "c", "d", "e"}
		batches := batchFileArgs(files, 2)
		if len(batches) != 3 {
			t.Fatalf("expected 3 batches, got %d", len(batches))
		}
		if len(batches[0]) != 2 {
			t.Errorf("batch[0] len = %d, want 2", len(batches[0]))
		}
		if len(batches[2]) != 1 {
			t.Errorf("batch[2] len = %d, want 1", len(batches[2]))
		}
	})

	t.Run("zero batch size returns single batch", func(t *testing.T) {
		files := []string{"a", "b"}
		batches := batchFileArgs(files, 0)
		if len(batches) != 1 {
			t.Fatalf("expected 1 batch, got %d", len(batches))
		}
	})

	t.Run("empty files returns single empty batch", func(t *testing.T) {
		batches := batchFileArgs(nil, 10)
		if len(batches) != 1 {
			t.Fatalf("expected 1 batch, got %d", len(batches))
		}
	})
}

// ---------------------------------------------------------------------------
// shouldFailFast
// ---------------------------------------------------------------------------

func TestShouldFailFast(t *testing.T) {
	t.Run("config fail_fast", func(t *testing.T) {
		cfg := &config.Config{FailFast: true}
		h := &Hook{}
		if !shouldFailFast(cfg, h) {
			t.Error("expected true when config.FailFast is true")
		}
	})

	t.Run("hook fail_fast", func(t *testing.T) {
		cfg := &config.Config{}
		h := &Hook{FailFast: true}
		if !shouldFailFast(cfg, h) {
			t.Error("expected true when hook.FailFast is true")
		}
	})

	t.Run("neither fail_fast", func(t *testing.T) {
		cfg := &config.Config{}
		h := &Hook{}
		if shouldFailFast(cfg, h) {
			t.Error("expected false when neither has fail_fast")
		}
	})
}

// ---------------------------------------------------------------------------
// checkMinVersion
// ---------------------------------------------------------------------------

func TestCheckMinVersion(t *testing.T) {
	tests := []struct {
		name       string
		minVersion string
		want       bool
	}{
		{"zero always passes", "0", true},
		{"0.0.0 always passes", "0.0.0", true},
		{"very high version fails", "999.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkMinVersion(tt.minVersion)
			if got != tt.want {
				t.Errorf("checkMinVersion(%q) = %v, want %v", tt.minVersion, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// fingerprintFiles
// ---------------------------------------------------------------------------

func TestFingerprintFiles(t *testing.T) {
	dir := t.TempDir()

	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f1, []byte("hello"), 0o644)
	os.WriteFile(f2, []byte("world"), 0o644)

	t.Run("returns fingerprints for existing files", func(t *testing.T) {
		fps := fingerprintFiles([]string{f1, f2})
		if len(fps) != 2 {
			t.Fatalf("expected 2 fingerprints, got %d", len(fps))
		}
		if fps[f1].size != 5 {
			t.Errorf("f1 size = %d, want 5", fps[f1].size)
		}
	})

	t.Run("skips non-existent files", func(t *testing.T) {
		fps := fingerprintFiles([]string{f1, filepath.Join(dir, "ghost.txt")})
		if len(fps) != 1 {
			t.Fatalf("expected 1 fingerprint, got %d", len(fps))
		}
	})

	t.Run("detects modification", func(t *testing.T) {
		before := fingerprintFiles([]string{f1})
		os.WriteFile(f1, []byte("changed content"), 0o644)
		after := fingerprintFiles([]string{f1})

		b := before[f1]
		a := after[f1]
		if b.size == a.size && b.modTime == a.modTime {
			t.Error("expected fingerprint to change after modification")
		}
	})
}

// ---------------------------------------------------------------------------
// Runner.Run — integration tests with system language hooks
// ---------------------------------------------------------------------------

func TestRunnerRun_BasicPass(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello\n"), 0o644)

	cfg := &config.Config{}
	hooks := []*Hook{{
		ID:            "echo-test",
		Name:          "Echo Test",
		Language:      "system",
		Entry:         "echo",
		Types:         []string{"file"},
		PassFilenames: true,
		Stages:        []config.Stage{config.HookTypePreCommit},
	}}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		Files:     []string{f},
		HookStage: config.HookTypePreCommit,
	})

	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1", result.Passed)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
}

func TestRunnerRun_BasicFail(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello\n"), 0o644)

	cfg := &config.Config{}
	hooks := []*Hook{{
		ID:            "fail-hook",
		Name:          "Fail Hook",
		Language:      "system",
		Entry:         "false",
		Types:         []string{"file"},
		PassFilenames: true,
		Stages:        []config.Stage{config.HookTypePreCommit},
	}}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		Files:     []string{f},
		HookStage: config.HookTypePreCommit,
	})

	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
}

func TestRunnerRun_SkipByID(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello\n"), 0o644)

	cfg := &config.Config{}
	hooks := []*Hook{{
		ID:            "skip-me",
		Name:          "Skip Me",
		Language:      "system",
		Entry:         "echo",
		Types:         []string{"file"},
		PassFilenames: true,
		Stages:        []config.Stage{config.HookTypePreCommit},
	}}

	t.Run("SKIP env var", func(t *testing.T) {
		t.Setenv("SKIP", "skip-me")
		runner := NewRunner(cfg, hooks, dir)
		result := runner.Run(context.Background(), RunOptions{
			Files:     []string{f},
			HookStage: config.HookTypePreCommit,
		})
		if result.Skipped != 1 {
			t.Errorf("Skipped = %d, want 1", result.Skipped)
		}
	})

	t.Run("SkipList option", func(t *testing.T) {
		runner := NewRunner(cfg, hooks, dir)
		result := runner.Run(context.Background(), RunOptions{
			Files:     []string{f},
			HookStage: config.HookTypePreCommit,
			SkipList:  []string{"skip-me"},
		})
		if result.Skipped != 1 {
			t.Errorf("Skipped = %d, want 1", result.Skipped)
		}
	})
}

func TestRunnerRun_FilterByHookID(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello\n"), 0o644)

	cfg := &config.Config{}
	hooks := []*Hook{
		{
			ID: "hook-a", Name: "Hook A", Language: "system", Entry: "echo",
			Types: []string{"file"}, PassFilenames: true,
			Stages: []config.Stage{config.HookTypePreCommit},
		},
		{
			ID: "hook-b", Name: "Hook B", Language: "system", Entry: "echo",
			Types: []string{"file"}, PassFilenames: true,
			Stages: []config.Stage{config.HookTypePreCommit},
		},
	}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		HookID:    "hook-a",
		Files:     []string{f},
		HookStage: config.HookTypePreCommit,
	})

	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1 (only hook-a)", result.Passed)
	}
}

func TestRunnerRun_StageFiltering(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello\n"), 0o644)

	cfg := &config.Config{}
	hooks := []*Hook{
		{
			ID: "pre-commit-hook", Name: "Pre-commit", Language: "system", Entry: "echo",
			Types: []string{"file"}, PassFilenames: true,
			Stages: []config.Stage{config.HookTypePreCommit},
		},
		{
			ID: "pre-push-hook", Name: "Pre-push", Language: "system", Entry: "echo",
			Types: []string{"file"}, PassFilenames: true,
			Stages: []config.Stage{config.HookTypePrePush},
		},
	}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		Files:     []string{f},
		HookStage: config.HookTypePrePush,
	})

	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1 (only pre-push hook)", result.Passed)
	}
}

func TestRunnerRun_FailFastStops(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("hello\n"), 0o644)

	cfg := &config.Config{FailFast: true}
	hooks := []*Hook{
		{
			ID: "fail-hook", Name: "Fail", Language: "system", Entry: "false",
			Types: []string{"file"}, PassFilenames: true,
			Stages: []config.Stage{config.HookTypePreCommit},
		},
		{
			ID: "pass-hook", Name: "Pass", Language: "system", Entry: "echo",
			Types: []string{"file"}, PassFilenames: true,
			Stages: []config.Stage{config.HookTypePreCommit},
		},
	}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		Files:     []string{f},
		HookStage: config.HookTypePreCommit,
	})

	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
	// Second hook should not have run due to fail-fast.
	if result.Passed != 0 {
		t.Errorf("Passed = %d, want 0 (fail-fast should stop)", result.Passed)
	}
}

func TestRunnerRun_NoMatchingFilesSkips(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "readme.md")
	os.WriteFile(f, []byte("# Hello\n"), 0o644)

	cfg := &config.Config{}
	hooks := []*Hook{{
		ID: "go-only", Name: "Go Only", Language: "system", Entry: "echo",
		Files: `\.go$`, Types: []string{"file"}, PassFilenames: true,
		Stages: []config.Stage{config.HookTypePreCommit},
	}}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		Files:     []string{f},
		HookStage: config.HookTypePreCommit,
	})

	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (no .go files)", result.Skipped)
	}
}

func TestRunnerRun_AlwaysRunIgnoresEmptyFiles(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{}
	hooks := []*Hook{{
		ID: "always", Name: "Always Run", Language: "system", Entry: "echo ok",
		Types: []string{"file"}, AlwaysRun: true,
		Stages: []config.Stage{config.HookTypePreCommit},
	}}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		Files:     nil, // no files
		HookStage: config.HookTypePreCommit,
	})

	if result.Passed != 1 {
		t.Errorf("Passed = %d, want 1 (always_run should execute)", result.Passed)
	}
}

func TestRunnerRun_FileModificationDetected(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "fix.txt")
	os.WriteFile(f, []byte("bad content"), 0o644)

	cfg := &config.Config{}
	hooks := []*Hook{{
		ID: "fixer", Name: "Fixer", Language: "system",
		Entry:         "sh -c 'echo fixed > \"$1\"' --",
		Types:         []string{"file"},
		PassFilenames: true,
		Stages:        []config.Stage{config.HookTypePreCommit},
	}}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		Files:     []string{f},
		HookStage: config.HookTypePreCommit,
	})

	// Hook exits 0 but modifies the file, so it should be marked as failed.
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1 (file was modified)", result.Failed)
	}
}

func TestRunnerRun_HookNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}

	runner := NewRunner(cfg, nil, dir)
	result := runner.Run(context.Background(), RunOptions{
		HookID:    "nonexistent",
		HookStage: config.HookTypePreCommit,
	})

	if result.Errors != 1 {
		t.Errorf("Errors = %d, want 1", result.Errors)
	}
}
