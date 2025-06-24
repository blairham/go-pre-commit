package execution

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestTimingDebug(t *testing.T) {
	// Save original value
	originalVal := os.Getenv("GO_PRECOMMIT_TIMING_DEBUG")
	defer os.Setenv("GO_PRECOMMIT_TIMING_DEBUG", originalVal)

	// Test with timing debug enabled
	os.Setenv("GO_PRECOMMIT_TIMING_DEBUG", "1")
	assert.True(t, isTimingDebugEnabled())

	// Test with timing debug disabled
	os.Setenv("GO_PRECOMMIT_TIMING_DEBUG", "")
	assert.False(t, isTimingDebugEnabled())

	// Test with empty value
	os.Unsetenv("GO_PRECOMMIT_TIMING_DEBUG")
	assert.False(t, isTimingDebugEnabled())
}

func TestLogTiming(t *testing.T) {
	// Save original value
	originalVal := os.Getenv("GO_PRECOMMIT_TIMING_DEBUG")
	defer os.Setenv("GO_PRECOMMIT_TIMING_DEBUG", originalVal)

	// Test with timing debug enabled
	os.Setenv("GO_PRECOMMIT_TIMING_DEBUG", "1")
	start := time.Now()
	// This function just logs, so we can't easily test output
	// But we can ensure it doesn't panic
	assert.NotPanics(t, func() {
		LogTiming("test phase", start)
	})

	// Test with timing debug disabled
	os.Setenv("GO_PRECOMMIT_TIMING_DEBUG", "")
	assert.NotPanics(t, func() {
		LogTiming("test phase", start)
	})
}

func TestConstants(t *testing.T) {
	// Test that all constants are defined correctly
	assert.Equal(t, ".js", JSExt)
	assert.Equal(t, ".jsx", JSXExt)
	assert.Equal(t, ".ts", TSExt)
	assert.Equal(t, ".tsx", TSXExt)
	assert.Equal(t, ".yaml", YamlExt)
	assert.Equal(t, ".yml", YmlExt)
	assert.Equal(t, ".html", HTMLExt)
	assert.Equal(t, "docker", DockerCmd)
	assert.Equal(t, "python", PythonCmd)
	assert.Equal(t, "python3", Python3Cmd)
}

func TestContext(t *testing.T) {
	ctx := &Context{
		Config:      &config.Config{},
		Environment: map[string]string{"TEST": "value"},
		RepoRoot:    "/test/repo",
		HookStage:   "pre-commit",
		HookType:    "pre-commit",
		Color:       "auto",
		Files:       []string{"file1.go", "file2.py"},
		HookIDs:     []string{"hook1", "hook2"},
		Timeout:     30 * time.Second,
		Parallel:    2,
		AllFiles:    false,
		Verbose:     true,
		ShowDiff:    false,
	}

	assert.NotNil(t, ctx.Config)
	assert.Equal(t, "value", ctx.Environment["TEST"])
	assert.Equal(t, "/test/repo", ctx.RepoRoot)
	assert.Equal(t, "pre-commit", ctx.HookStage)
	assert.Equal(t, "pre-commit", ctx.HookType)
	assert.Equal(t, "auto", ctx.Color)
	assert.Len(t, ctx.Files, 2)
	assert.Len(t, ctx.HookIDs, 2)
	assert.Equal(t, 30*time.Second, ctx.Timeout)
	assert.Equal(t, 2, ctx.Parallel)
	assert.False(t, ctx.AllFiles)
	assert.True(t, ctx.Verbose)
	assert.False(t, ctx.ShowDiff)
}

func TestResult(t *testing.T) {
	hook := config.Hook{
		ID:   "test-hook",
		Name: "Test Hook",
	}

	result := Result{
		Output:   "test output",
		Error:    "test error",
		Files:    []string{"file1.go"},
		Hook:     hook,
		Duration: 5 * time.Second,
		ExitCode: 1,
		Success:  false,
		Timeout:  false,
		Skipped:  false,
	}

	assert.Equal(t, "test output", result.Output)
	assert.Equal(t, "test error", result.Error)
	assert.Len(t, result.Files, 1)
	assert.Equal(t, "test-hook", result.Hook.ID)
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.Equal(t, 1, result.ExitCode)
	assert.False(t, result.Success)
	assert.False(t, result.Timeout)
	assert.False(t, result.Skipped)
}

func TestRunItem(t *testing.T) {
	repo := config.Repo{
		Repo: "https://github.com/test/repo",
		Rev:  "main",
	}

	hook := config.Hook{
		ID:   "test-hook",
		Name: "Test Hook",
	}

	runItem := RunItem{
		RepoPath: "/test/repo/path",
		Repo:     repo,
		Hook:     hook,
	}

	assert.Equal(t, "/test/repo/path", runItem.RepoPath)
	assert.Equal(t, "https://github.com/test/repo", runItem.Repo.Repo)
	assert.Equal(t, "main", runItem.Repo.Rev)
	assert.Equal(t, "test-hook", runItem.Hook.ID)
}

func TestSkipResult(t *testing.T) {
	result := Result{
		Success: true,
		Skipped: true,
	}

	skipResult := SkipResult{
		Result: result,
		Skip:   true,
	}

	assert.True(t, skipResult.Result.Success)
	assert.True(t, skipResult.Result.Skipped)
	assert.True(t, skipResult.Skip)
}

func TestHookResult(t *testing.T) {
	result := Result{
		Success: true,
	}

	hookResult := HookResult{
		Result: result,
		Index:  5,
	}

	assert.True(t, hookResult.Result.Success)
	assert.Equal(t, 5, hookResult.Index)
}
