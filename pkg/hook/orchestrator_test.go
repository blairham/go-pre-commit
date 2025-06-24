package hook

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
)

// Test the apply override functions that have 0% coverage
func Test_applyStringOverride(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		override string
		expected string
	}{
		{
			name:     "empty override keeps original",
			target:   "original",
			override: "",
			expected: "original",
		},
		{
			name:     "non-empty override replaces",
			target:   "original",
			override: "new_value",
			expected: "new_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			applyStringOverride(&target, tt.override)
			if target != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, target)
			}
		})
	}
}

func Test_applySliceOverride(t *testing.T) {
	tests := []struct {
		name     string
		target   []string
		override []string
		expected []string
	}{
		{
			name:     "empty override keeps original",
			target:   []string{"original"},
			override: []string{},
			expected: []string{"original"},
		},
		{
			name:     "non-empty override replaces",
			target:   []string{"original"},
			override: []string{"new1", "new2"},
			expected: []string{"new1", "new2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			applySliceOverride(&target, tt.override)
			if len(target) != len(tt.expected) {
				t.Errorf("Expected length %d, got %d", len(tt.expected), len(target))
			}
			for i, v := range tt.expected {
				if target[i] != v {
					t.Errorf("Expected %s at index %d, got %s", v, i, target[i])
				}
			}
		})
	}
}

func Test_applyBoolOverride(t *testing.T) {
	tests := []struct {
		name     string
		target   bool
		override bool
		expected bool
	}{
		{
			name:     "false override keeps original false",
			target:   false,
			override: false,
			expected: false,
		},
		{
			name:     "false override keeps original true",
			target:   true,
			override: false,
			expected: true,
		},
		{
			name:     "true override changes to true",
			target:   false,
			override: true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			applyBoolOverride(&target, tt.override)
			if target != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, target)
			}
		})
	}
}

func Test_applyBoolPtrOverride(t *testing.T) {
	tests := []struct {
		target      *bool
		override    *bool
		name        string
		expectedVal bool
		expectNil   bool
	}{
		{
			name:        "nil override keeps original",
			target:      func() *bool { b := true; return &b }(),
			override:    nil,
			expectedVal: true,
			expectNil:   false,
		},
		{
			name:        "non-nil override replaces",
			target:      func() *bool { b := true; return &b }(),
			override:    func() *bool { b := false; return &b }(),
			expectedVal: false,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.target
			applyBoolPtrOverride(&target, tt.override)
			if tt.expectNil {
				if target != nil {
					t.Errorf("Expected nil, got %v", target)
				}
			} else {
				if target == nil {
					t.Error("Expected non-nil, got nil")
				} else if *target != tt.expectedVal {
					t.Errorf("Expected %v, got %v", tt.expectedVal, *target)
				}
			}
		})
	}
}

// Test mergeHookDefinitions function with comprehensive coverage
func TestOrchestrator_mergeHookDefinitions_Additional(t *testing.T) {
	ctx := &execution.Context{
		RepoRoot: t.TempDir(),
		Config:   &config.Config{},
	}
	orchestrator := NewOrchestrator(ctx)

	tests := []struct {
		name     string
		base     config.Hook
		override config.Hook
		expected config.Hook
	}{
		{
			name:     "empty override keeps base",
			base:     config.Hook{ID: "test", Name: "Base Hook", Entry: "test.sh"},
			override: config.Hook{},
			expected: config.Hook{ID: "test", Name: "Base Hook", Entry: "test.sh"},
		},
		{
			name:     "string overrides",
			base:     config.Hook{ID: "test", Name: "Base Hook", Entry: "test.sh", Language: "python"},
			override: config.Hook{Name: "Override Hook", Entry: "override.sh"},
			expected: config.Hook{ID: "test", Name: "Override Hook", Entry: "override.sh", Language: "python"},
		},
		{
			name:     "slice overrides",
			base:     config.Hook{ID: "test", Types: []string{"python"}, Args: []string{"--old"}},
			override: config.Hook{Types: []string{"javascript", "typescript"}, Args: []string{"--new"}},
			expected: config.Hook{ID: "test", Types: []string{"javascript", "typescript"}, Args: []string{"--new"}},
		},
		{
			name:     "bool overrides",
			base:     config.Hook{ID: "test", AlwaysRun: false, Verbose: false},
			override: config.Hook{AlwaysRun: true, Verbose: true},
			expected: config.Hook{ID: "test", AlwaysRun: true, Verbose: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orchestrator.mergeHookDefinitions(tt.base, tt.override)

			if result.ID != tt.expected.ID {
				t.Errorf("Expected ID %s, got %s", tt.expected.ID, result.ID)
			}
			if result.Name != tt.expected.Name {
				t.Errorf("Expected Name %s, got %s", tt.expected.Name, result.Name)
			}
			if result.Entry != tt.expected.Entry {
				t.Errorf("Expected Entry %s, got %s", tt.expected.Entry, result.Entry)
			}
			if result.Language != tt.expected.Language {
				t.Errorf("Expected Language %s, got %s", tt.expected.Language, result.Language)
			}
			if result.AlwaysRun != tt.expected.AlwaysRun {
				t.Errorf("Expected AlwaysRun %v, got %v", tt.expected.AlwaysRun, result.AlwaysRun)
			}
			if result.Verbose != tt.expected.Verbose {
				t.Errorf("Expected Verbose %v, got %v", tt.expected.Verbose, result.Verbose)
			}
			if len(result.Types) != len(tt.expected.Types) {
				t.Errorf("Expected Types length %d, got %d", len(tt.expected.Types), len(result.Types))
			}
			if len(result.Args) != len(tt.expected.Args) {
				t.Errorf("Expected Args length %d, got %d", len(tt.expected.Args), len(result.Args))
			}
		})
	}
}

// Test setupHookDefinition when repoMgr is nil
func TestOrchestrator_setupHookDefinition_NilRepoMgr(t *testing.T) {
	ctx := &execution.Context{
		RepoRoot: t.TempDir(),
		Config:   &config.Config{},
	}
	orchestrator := NewOrchestrator(ctx)
	// repoMgr is nil by default

	hook := config.Hook{ID: "test", Name: "Test Hook"}
	repo := config.Repo{Repo: "local"}

	result, err := orchestrator.setupHookDefinition(hook, repo)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.ID != hook.ID || result.Name != hook.Name {
		t.Errorf("Expected hook unchanged when repoMgr is nil")
	}
}

// Test addContextEnvironment with additional coverage
func TestOrchestrator_addContextEnvironment_Additional(t *testing.T) {
	tests := []struct {
		contextEnv  map[string]string
		name        string
		expectedLen int
	}{
		{
			name:        "nil context environment",
			contextEnv:  nil,
			expectedLen: 0,
		},
		{
			name:        "empty context environment",
			contextEnv:  map[string]string{},
			expectedLen: 0,
		},
		{
			name: "context with environment variables",
			contextEnv: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &execution.Context{
				RepoRoot:    t.TempDir(),
				Config:      &config.Config{},
				Environment: tt.contextEnv,
			}
			orchestrator := NewOrchestrator(ctx)

			cmd := &exec.Cmd{}
			orchestrator.addContextEnvironment(cmd)

			if len(cmd.Env) != tt.expectedLen {
				t.Errorf("Expected %d environment variables, got %d", tt.expectedLen, len(cmd.Env))
			}

			// Verify the environment variables are correctly formatted
			if tt.expectedLen > 0 {
				envMap := make(map[string]string)
				for _, env := range cmd.Env {
					parts := strings.SplitN(env, "=", 2)
					if len(parts) == 2 {
						envMap[parts[0]] = parts[1]
					}
				}
				for key, expectedValue := range tt.contextEnv {
					if actualValue, exists := envMap[key]; !exists {
						t.Errorf("Expected environment variable %s not found", key)
					} else if actualValue != expectedValue {
						t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
					}
				}
			}
		})
	}
}

// Test setupCommandEnvironmentWithEnv with nil repoMgr
func TestOrchestrator_setupCommandEnvironmentWithEnv_NilRepoMgr(t *testing.T) {
	tempDir := t.TempDir()

	ctx := &execution.Context{
		RepoRoot: tempDir,
		Config:   &config.Config{},
		Environment: map[string]string{
			"CONTEXT_VAR": "context_value",
		},
	}
	orchestrator := NewOrchestrator(ctx)
	// repoMgr is nil by default

	cmd := &exec.Cmd{}
	hook := config.Hook{ID: "test"}
	repo := config.Repo{Repo: "test"}
	repoPath := "/test/path"
	hookEnv := map[string]string{
		"HOOK_VAR": "hook_value",
	}

	orchestrator.setupCommandEnvironmentWithEnv(cmd, hook, repo, repoPath, hookEnv)

	if cmd.Dir != tempDir {
		t.Errorf("Expected cmd.Dir to be %s, got %s", tempDir, cmd.Dir)
	}

	// Should have 2 environment variables (1 from context, 1 from hookEnv since repoMgr is nil)
	if len(cmd.Env) != 2 {
		t.Errorf("Expected 2 environment variables, got %d", len(cmd.Env))
	}
}

// Test getRepoPathForHook with improved coverage
func TestOrchestrator_getRepoPathForHook_Coverage(t *testing.T) {
	tempDir := t.TempDir()

	ctx := &execution.Context{
		RepoRoot: tempDir,
		Config:   &config.Config{},
	}
	orchestrator := NewOrchestrator(ctx)
	// repoMgr is nil by default

	hook := config.Hook{ID: "test-hook"}
	repo := config.Repo{Repo: "local"}

	repoPath, err := orchestrator.getRepoPathForHook(context.Background(), repo, hook)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if repoPath != tempDir {
		t.Errorf("Expected %s, got %s", tempDir, repoPath)
	}
}

// Test shouldSkipHook edge cases
func TestOrchestrator_shouldSkipHook_Coverage(t *testing.T) {
	ctx := &execution.Context{
		RepoRoot: t.TempDir(),
		Config:   &config.Config{},
	}
	orchestrator := NewOrchestrator(ctx)

	tests := []struct {
		name         string
		files        []string
		hook         config.Hook
		expectedSkip bool
	}{
		{
			name:         "no files, no always run - should skip",
			hook:         config.Hook{ID: "test", AlwaysRun: false},
			files:        []string{},
			expectedSkip: true,
		},
		{
			name:         "no files, always run enabled - should not skip",
			hook:         config.Hook{ID: "test", AlwaysRun: true},
			files:        []string{},
			expectedSkip: false,
		},
		{
			name:         "with files - should not skip",
			hook:         config.Hook{ID: "test", AlwaysRun: false},
			files:        []string{"file1.py"},
			expectedSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orchestrator.shouldSkipHook(tt.hook, tt.files, time.Now())

			if result.Skip != tt.expectedSkip {
				t.Errorf("Expected skip %v, got %v", tt.expectedSkip, result.Skip)
			}

			if result.Skip {
				if !result.Result.Success {
					t.Error("Skipped result should be successful")
				}
				if !result.Result.Skipped {
					t.Error("Skipped result should have Skipped=true")
				}
				if result.Result.Duration != 0 {
					t.Error("Skipped result should have Duration=0")
				}
			}
		})
	}
}

// Test NewOrchestrator improved coverage
func TestNewOrchestrator_Coverage(t *testing.T) {
	tempDir := t.TempDir()

	// Test with minimal context
	ctx := &execution.Context{
		RepoRoot: tempDir,
		Config:   &config.Config{},
	}

	orchestrator := NewOrchestrator(ctx)

	// Verify all components are initialized
	if orchestrator == nil {
		t.Fatal("Expected non-nil orchestrator")
	}
	if orchestrator.ctx != ctx {
		t.Error("Expected context to be set correctly")
	}
	if orchestrator.executor == nil {
		t.Error("Expected executor to be initialized")
	}
	if orchestrator.formatter == nil {
		t.Error("Expected formatter to be initialized")
	}
	if orchestrator.matcher == nil {
		t.Error("Expected matcher to be initialized")
	}
	if orchestrator.builder == nil {
		t.Error("Expected builder to be initialized")
	}
	// repoMgr may or may not be nil depending on the environment
	// This is acceptable behavior - log what we got for debugging
	if orchestrator.repoMgr == nil {
		t.Log("repoMgr is nil")
	} else {
		t.Log("repoMgr is not nil")
	}
}

// Test collectResults with errors
func TestOrchestrator_collectResults_WithErrors(t *testing.T) {
	ctx := &execution.Context{
		RepoRoot: t.TempDir(),
		Config:   &config.Config{},
	}
	orchestrator := NewOrchestrator(ctx)

	resultsChan := make(chan hookResultOrc, 2)
	resultsChan <- hookResultOrc{
		result: execution.Result{Hook: config.Hook{ID: "hook1"}, Success: true},
		err:    nil,
		index:  0,
	}
	resultsChan <- hookResultOrc{
		result: execution.Result{Hook: config.Hook{ID: "hook2"}, Success: false},
		err:    os.ErrNotExist,
		index:  1,
	}
	close(resultsChan)

	results, err := orchestrator.collectResults(resultsChan, 2)

	// Should return the first error encountered
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected error %v, got %v", os.ErrNotExist, err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}
