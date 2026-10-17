// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/languages"
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
	unset := func(t *testing.T, keys ...string) {
		t.Helper()
		for _, k := range keys {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}

	t.Run("explicit jobs wins over TRAVIS and the CPU count", func(t *testing.T) {
		unset(t, "PRE_COMMIT_NO_CONCURRENCY")
		t.Setenv("TRAVIS", "1")
		if got := targetConcurrency(8); got != 8 {
			t.Errorf("targetConcurrency(8) = %d, want 8", got)
		}
	})

	t.Run("unset jobs is upstream's CPU count", func(t *testing.T) {
		unset(t, "PRE_COMMIT_NO_CONCURRENCY", "TRAVIS")
		if got, want := targetConcurrency(0), runtime.NumCPU(); got != want {
			t.Errorf("targetConcurrency(0) = %d, want %d", got, want)
		}
	})

	t.Run("TRAVIS caps unset jobs at 2", func(t *testing.T) {
		unset(t, "PRE_COMMIT_NO_CONCURRENCY")
		t.Setenv("TRAVIS", "")
		if got := targetConcurrency(0); got != 2 {
			t.Errorf("targetConcurrency(0) under TRAVIS = %d, want 2", got)
		}
	})

	t.Run("NO_CONCURRENCY overrides everything, even set empty", func(t *testing.T) {
		t.Setenv("PRE_COMMIT_NO_CONCURRENCY", "")
		if got := targetConcurrency(8); got != 1 {
			t.Errorf("targetConcurrency(8) with NO_CONCURRENCY = %d, want 1", got)
		}
	})
}

// ---------------------------------------------------------------------------
// runHookXargs partitioning
// ---------------------------------------------------------------------------

// recordingLang records each invocation's files and answers with a line per
// invocation and a scripted exit code.
type recordingLang struct {
	exit  func(files []string) int
	calls [][]string
	mu    sync.Mutex
}

func (l *recordingLang) Name() string                                      { return "system" }
func (l *recordingLang) EnvironmentDir() string                            { return "" }
func (l *recordingLang) GetDefaultVersion() string                         { return "default" }
func (l *recordingLang) HealthCheck(string, string) error                  { return nil }
func (l *recordingLang) InstallEnvironment(string, string, []string) error { return nil }

func (l *recordingLang) Run(
	_ context.Context, _, _, _ string, _, files []string, _ string,
) (int, []byte, error) {
	l.mu.Lock()
	l.calls = append(l.calls, slices.Clone(files))
	l.mu.Unlock()
	code := 0
	if l.exit != nil {
		code = l.exit(files)
	}
	return code, []byte(strings.Join(files, " ") + "\n"), nil
}

func TestRunHookXargsPartitions(t *testing.T) {
	files := make([]string, 22)
	for i := range files {
		files[i] = fmt.Sprintf("f%02d", i)
	}

	t.Run("splits into upstream's partitions and joins output in order", func(t *testing.T) {
		lang := &recordingLang{}
		h := &Hook{ID: "x", Entry: "true", Language: "system"}
		code, out, err := runHookXargs(context.Background(), lang, h, files, t.TempDir(), 4)
		if err != nil || code != 0 {
			t.Fatalf("runHookXargs = %d, %v", code, err)
		}
		// max(4, ceil(22/4)) = 6 per partition, as upstream's
		// test_partition_target_concurrency.
		if len(lang.calls) != 4 {
			t.Fatalf("got %d invocations, want 4: %q", len(lang.calls), lang.calls)
		}
		shuffled := shuffledFileArgs("system", false, files)
		var want strings.Builder
		for i := 0; i < len(shuffled); i += 6 {
			want.WriteString(strings.Join(shuffled[i:min(i+6, len(shuffled))], " ") + "\n")
		}
		if string(out) != want.String() {
			t.Errorf("output =\n%s\nwant\n%s", out, want.String())
		}
	})

	t.Run("require_serial runs once in the given order", func(t *testing.T) {
		lang := &recordingLang{}
		h := &Hook{ID: "x", Entry: "true", Language: "system", RequireSerial: true}
		if _, _, err := runHookXargs(context.Background(), lang, h, files, t.TempDir(), 4); err != nil {
			t.Fatal(err)
		}
		if len(lang.calls) != 1 || !slices.Equal(lang.calls[0], files) {
			t.Errorf("calls = %q, want one call with %q", lang.calls, files)
		}
	})

	t.Run("PRE_COMMIT_NO_CONCURRENCY runs once", func(t *testing.T) {
		t.Setenv("PRE_COMMIT_NO_CONCURRENCY", "1")
		lang := &recordingLang{}
		h := &Hook{ID: "x", Entry: "true", Language: "system"}
		if _, _, err := runHookXargs(context.Background(), lang, h, files, t.TempDir(), 4); err != nil {
			t.Fatal(err)
		}
		if len(lang.calls) != 1 {
			t.Errorf("got %d invocations, want 1", len(lang.calls))
		}
	})

	t.Run("fail and pygrep run once", func(t *testing.T) {
		for _, language := range []string{"fail", "pygrep"} {
			lang := &recordingLang{}
			h := &Hook{ID: "x", Entry: "x", Language: language}
			if _, _, err := runHookXargs(context.Background(), lang, h, files, t.TempDir(), 4); err != nil {
				t.Fatal(err)
			}
			if len(lang.calls) != 1 {
				t.Errorf("%s: got %d invocations, want 1", language, len(lang.calls))
			}
		}
	})

	t.Run("exit code of largest magnitude wins", func(t *testing.T) {
		// Partitions start at shuffled[0], [6], [12], [18]; key each
		// partition's exit code on its first file.
		shuffled := shuffledFileArgs("system", false, files)
		codes := map[string]int{shuffled[0]: 1, shuffled[6]: -5, shuffled[12]: 3, shuffled[18]: 0}
		lang := &recordingLang{exit: func(files []string) int { return codes[files[0]] }}
		h := &Hook{ID: "x", Entry: "true", Language: "system"}
		code, _, err := runHookXargs(context.Background(), lang, h, files, t.TempDir(), 4)
		if err != nil {
			t.Fatal(err)
		}
		if len(lang.calls) != 4 || code != -5 {
			t.Errorf("exit = %d over %d invocations, want -5 over 4", code, len(lang.calls))
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

	// Dev builds inject `git describe` output with a leading "v"; the check
	// must still parse the numeric components.
	t.Run("git describe dev version", func(t *testing.T) {
		orig := config.Version
		config.Version = "v4.6.0-1-gabc1234-dirty"
		defer func() { config.Version = orig }()

		if !checkMinVersion("3.2.0") {
			t.Error("checkMinVersion(\"3.2.0\") = false with dev version, want true")
		}
		if checkMinVersion("999.0.0") {
			t.Error("checkMinVersion(\"999.0.0\") = true with dev version, want false")
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

// initModRepo creates a git repo with fix.txt committed.
func initModRepo(t *testing.T) (dir, file string) {
	t.Helper()
	dir = t.TempDir()
	file = filepath.Join(dir, "fix.txt")
	if err := os.WriteFile(file, []byte("bad content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "fix.txt"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir, file
}

// A hook that exits 0 but changes the working tree fails, as upstream decides
// it from `git diff` around the hook — so it must fire whether or not the hook
// was handed the files (golangci-lint-fmt has pass_filenames: false), and
// under --all-files too.
func TestRunnerRun_FileModificationDetected(t *testing.T) {
	for _, tc := range []struct {
		name          string
		passFilenames bool
		allFiles      bool
	}{
		{name: "pass_filenames", passFilenames: true},
		{name: "no pass_filenames", passFilenames: false},
		{name: "all-files", passFilenames: false, allFiles: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, f := initModRepo(t)

			cfg := &config.Config{}
			hooks := []*Hook{{
				ID: "fixer", Name: "Fixer", Language: "system",
				Entry:         "sh -c 'echo fixed > fix.txt' --",
				Types:         []string{"file"},
				PassFilenames: tc.passFilenames,
				Stages:        []config.Stage{config.HookTypePreCommit},
			}}

			runner := NewRunner(cfg, hooks, dir)
			result := runner.Run(context.Background(), RunOptions{
				Files:     []string{f},
				AllFiles:  tc.allFiles,
				HookStage: config.HookTypePreCommit,
			})

			if result.Failed != 1 {
				t.Errorf("Failed = %d, want 1 (file was modified)", result.Failed)
			}
		})
	}
}

// A hook that leaves the working tree alone passes even when the tree already
// had unstaged changes before it ran.
func TestRunnerRun_PreexistingChangesNotBlamed(t *testing.T) {
	dir, f := initModRepo(t)
	if err := os.WriteFile(f, []byte("already dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	hooks := []*Hook{{
		ID: "noop", Name: "Noop", Language: "system", Entry: "true",
		Types: []string{"file"}, Stages: []config.Stage{config.HookTypePreCommit},
	}}

	runner := NewRunner(cfg, hooks, dir)
	result := runner.Run(context.Background(), RunOptions{
		Files:     []string{f},
		AllFiles:  true,
		HookStage: config.HookTypePreCommit,
	})
	if result.Passed != 1 || result.Failed != 0 {
		t.Errorf("Passed = %d, Failed = %d; want 1, 0", result.Passed, result.Failed)
	}
}

func TestRunnerRun_HookNotFound(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	// A hook that exists but not in the requested stage is not found either.
	hooks := []*Hook{{ID: "pushy", Name: "pushy", Stages: []config.Stage{config.HookTypePrePush}}}

	for _, stage := range []config.Stage{config.HookTypePreCommit, config.StageManual} {
		var result RunResult
		out := captureStdout(t, func() {
			result = NewRunner(cfg, hooks, dir).Run(context.Background(), RunOptions{
				HookID:    "pushy",
				HookStage: stage,
			})
		})
		if result.Errors != 1 {
			t.Errorf("stage %s: Errors = %d, want 1", stage, result.Errors)
		}
		// Upstream's wording, written to stdout with no [ERROR] prefix.
		if want := "No hook with id `pushy` in stage `" + string(stage) + "`\n"; out != want {
			t.Errorf("stage %s: stdout = %q, want %q", stage, out, want)
		}
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
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

// A local hook is found by its alias, as upstream's `hook.alias == args.hook`.
func TestRunnerRun_LocalHookAlias(t *testing.T) {
	h := FromLocalConfig(
		&config.HookConfig{ID: "real", Name: "real", Alias: "nick", Entry: "x", Language: "fail"},
		&config.Config{},
	)
	if h.Alias != "nick" {
		t.Fatalf("FromLocalConfig dropped the alias: %q", h.Alias)
	}
}

// A leftover env dir with no install-state marker (partial install, interrupted
// run, or a CI cache restore that dropped the state file) must be wiped before
// installing, not installed over: golang's `go install ./...` runs in the hook
// repo with the env's module cache nested inside it, so leftovers in pkg/mod
// make the pattern walk fail with "directory … outside main module or its
// selected dependencies" (the 2026-08-15 CI failure across ghorg,
// go-pre-commit, and aws-sso-config).
func TestInstallEnvironmentsWipesStatelessLeftoverEnv(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	repoDir := t.TempDir()

	// Minimal installable golang hook repo.
	if err := os.WriteFile(
		filepath.Join(repoDir, "go.mod"),
		[]byte("module example.com/hookrepo\n\ngo 1.21\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(repoDir, "main.go"),
		[]byte("package main\n\nfunc main() {}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Simulate the poisoned state: env dir with a populated module cache but
	// no install_state_v2. The junk module has its own go.mod, which is what
	// breaks the `./...` pattern walk when it survives into the install.
	junk := filepath.Join(repoDir, "golangenv-system", "pkg", "mod", "example.com", "junk@v1.0.0")
	if err := os.MkdirAll(junk, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(junk, "go.mod"), []byte("module example.com/junk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(junk, "junk.go"), []byte("package junk\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &Hook{
		ID:              "hookrepo",
		Repo:            "example.com/hookrepo",
		RepoDir:         repoDir,
		Language:        "golang",
		LanguageVersion: "system",
	}
	if err := InstallEnvironments(context.Background(), []*Hook{h}, nil); err != nil {
		t.Fatalf("InstallEnvironments over a stateless leftover env: %v", err)
	}

	envPath := filepath.Join(repoDir, "golangenv-system")
	if _, err := os.Stat(filepath.Join(envPath, "bin", "hookrepo")); err != nil {
		t.Errorf("hook binary not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(envPath, installStateFile)); err != nil {
		t.Errorf("install state not written: %v", err)
	}
	if _, err := os.Stat(junk); !os.IsNotExist(err) {
		t.Errorf("leftover env content survived the install (err=%v)", err)
	}
}

// ---------------------------------------------------------------------------
// shuffledFileArgs
// ---------------------------------------------------------------------------

func TestShuffledFileArgs(t *testing.T) {
	in := []string{".pre-commit-config.yaml", "a.txt", "b.txt", "c.txt"}
	// Captured from upstream's lang_base._shuffled on CPython.
	shuffled := []string{"a.txt", ".pre-commit-config.yaml", "b.txt", "c.txt"}

	for _, tc := range []struct {
		language      string
		want          []string
		requireSerial bool
	}{
		{"unsupported", shuffled, false},
		{"python", shuffled, false},
		{"docker_image", shuffled, false},
		{"unsupported", in, true},
		{"fail", in, false},
		{"pygrep", in, false},
	} {
		orig := slices.Clone(in)
		got := shuffledFileArgs(tc.language, tc.requireSerial, orig)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s require_serial=%v: got %q, want %q", tc.language, tc.requireSerial, got, tc.want)
		}
		if !slices.Equal(orig, in) {
			t.Errorf("%s: shuffled the caller's slice in place: %q", tc.language, orig)
		}
	}
}

// Upstream's install_hook_envs takes the store's exclusive lock and asks again
// what needs installing once it holds it: another process may have finished
// the same install while this one waited. Installing anyway put two processes
// into one environment directory at once.
func TestInstallEnvironmentsRechecksUnderLock(t *testing.T) {
	repoDir := t.TempDir() // no go.mod: an install attempt here would fail
	h := &Hook{
		ID:              "hookrepo",
		Repo:            "example.com/hookrepo",
		RepoDir:         repoDir,
		Language:        "golang",
		LanguageVersion: "default",
	}
	envPath := filepath.Join(repoDir, "golangenv-default")

	locked, unlocked := 0, 0
	lock := func() (func(), error) {
		locked++
		// The other process finishes while this one waits for the lock.
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(envPath, installStateFile), []byte(h.InstallKey()), 0o644); err != nil {
			t.Fatal(err)
		}
		return func() { unlocked++ }, nil
	}
	if err := InstallEnvironments(context.Background(), []*Hook{h}, lock); err != nil {
		t.Fatalf("InstallEnvironments should find the env installed once it holds the lock: %v", err)
	}
	if locked != 1 || unlocked != 1 {
		t.Errorf("lock taken %d times and released %d, want 1 and 1", locked, unlocked)
	}

	// Nothing to install: the lock is not taken at all.
	if err := InstallEnvironments(context.Background(), []*Hook{h}, lock); err != nil {
		t.Fatal(err)
	}
	if locked != 1 {
		t.Errorf("an installed env should not take the lock, taken %d times", locked)
	}
}

// healthLang is a language whose health and installs a test controls.
type healthLang struct {
	healthy  bool
	installs int
}

func (l *healthLang) Name() string              { return "healthtest" }
func (l *healthLang) EnvironmentDir() string    { return "health_env" }
func (l *healthLang) GetDefaultVersion() string { return "default" }
func (l *healthLang) HealthCheck(_, _ string) error {
	if l.healthy {
		return nil
	}
	return errors.New("the env went bad")
}

func (l *healthLang) InstallEnvironment(prefix, version string, _ []string) error {
	l.installs++
	return os.MkdirAll(filepath.Join(prefix, "health_env-"+version), 0o755)
}

func (l *healthLang) Run(context.Context, string, string, string, []string, []string, string) (int, []byte, error) {
	return 0, nil, nil
}

// upstream's _hook_installed counts an env as installed only while its
// health_check passes, and _hook_install refuses an env that is unhealthy
// straight after installing it.
func TestInstallEnvironmentsHealthCheck(t *testing.T) {
	lang := &healthLang{healthy: true}
	languages.Register("healthtest", lang)
	h := &Hook{ID: "h", Repo: "r", RepoDir: t.TempDir(), Language: "healthtest", LanguageVersion: "default"}

	if err := InstallEnvironments(context.Background(), []*Hook{h}, nil); err != nil {
		t.Fatal(err)
	}
	if err := InstallEnvironments(context.Background(), []*Hook{h}, nil); err != nil {
		t.Fatal(err)
	}
	if lang.installs != 1 {
		t.Fatalf("a healthy installed env was installed %d times, want 1", lang.installs)
	}

	lang.healthy = false
	err := InstallEnvironments(context.Background(), []*Hook{h}, nil)
	if lang.installs != 2 {
		t.Errorf("an unhealthy env should be reinstalled; installs = %d", lang.installs)
	}
	want := "BUG: expected environment for healthtest to be healthy immediately after install, " +
		"please open an issue describing your environment\n\nmore info:\n\nthe env went bad"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v\nwant %s", err, want)
	}
	if _, statErr := os.Stat(filepath.Join(h.RepoDir, "health_env-default")); !os.IsNotExist(statErr) {
		t.Errorf("the unhealthy env should be removed (%v)", statErr)
	}
}
