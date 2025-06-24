package repository

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestHookManager_IsMetaRepo(t *testing.T) {
	hm := NewHookManager()

	tests := []struct {
		name     string
		repo     config.Repo
		expected bool
	}{
		{
			name:     "meta repository",
			repo:     config.Repo{Repo: "meta"},
			expected: true,
		},
		{
			name:     "local repository",
			repo:     config.Repo{Repo: "local"},
			expected: false,
		},
		{
			name:     "remote repository",
			repo:     config.Repo{Repo: "https://github.com/user/repo"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hm.IsMetaRepo(tt.repo)
			if result != tt.expected {
				t.Errorf("IsMetaRepo() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHookManager_IsLocalRepo(t *testing.T) {
	hm := NewHookManager()

	tests := []struct {
		name     string
		repo     config.Repo
		expected bool
	}{
		{
			name:     "local repository",
			repo:     config.Repo{Repo: "local"},
			expected: true,
		},
		{
			name:     "meta repository",
			repo:     config.Repo{Repo: "meta"},
			expected: false,
		},
		{
			name:     "remote repository",
			repo:     config.Repo{Repo: "https://github.com/user/repo"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hm.IsLocalRepo(tt.repo)
			if result != tt.expected {
				t.Errorf("IsLocalRepo() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHookManager_GetMetaHook(t *testing.T) {
	hm := NewHookManager()

	tests := []struct {
		name     string
		hookID   string
		hookName string
		expected bool
	}{
		{
			name:     "check-yaml hook exists",
			hookID:   "check-yaml",
			expected: true,
			hookName: "Check YAML",
		},
		{
			name:     "check-json hook exists",
			hookID:   "check-json",
			expected: true,
			hookName: "Check JSON",
		},
		{
			name:     "trailing-whitespace hook exists",
			hookID:   "trailing-whitespace",
			expected: true,
			hookName: "Trim Trailing Whitespace",
		},
		{
			name:     "nonexistent hook",
			hookID:   "nonexistent-hook",
			expected: false,
			hookName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook, exists := hm.GetMetaHook(tt.hookID)
			if exists != tt.expected {
				t.Errorf("GetMetaHook() exists = %v, want %v", exists, tt.expected)
			}
			if exists && hook.Name != tt.hookName {
				t.Errorf("GetMetaHook() hook.Name = %v, want %v", hook.Name, tt.hookName)
			}
			if exists && hook.Language != "system" {
				t.Errorf("GetMetaHook() hook.Language = %v, want system", hook.Language)
			}
		})
	}
}

func TestHookManager_GetRepositoryHook(t *testing.T) {
	hm := NewHookManager()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-repo-hooks")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test case 1: Hook file exists with valid YAML
	hookContent := `- id: test-hook
  name: Test Hook
  entry: test-command
  language: python
  files: \.py$
- id: another-hook
  name: Another Hook
  entry: another-command
  language: node
`
	hookFile := filepath.Join(tempDir, ".pre-commit-hooks.yaml")
	err = os.WriteFile(hookFile, []byte(hookContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write hook file: %v", err)
	}

	// Test finding existing hook
	hook, exists := hm.GetRepositoryHook(tempDir, "test-hook")
	if !exists {
		t.Error("Expected hook to exist")
	}
	if hook.Name != "Test Hook" {
		t.Errorf("Expected hook name 'Test Hook', got %s", hook.Name)
	}
	if hook.Language != "python" {
		t.Errorf("Expected language 'python', got %s", hook.Language)
	}

	// Test finding another hook
	hook2, exists2 := hm.GetRepositoryHook(tempDir, "another-hook")
	if !exists2 {
		t.Error("Expected second hook to exist")
	}
	if hook2.Language != "node" {
		t.Errorf("Expected language 'node', got %s", hook2.Language)
	}

	// Test hook that doesn't exist
	_, exists3 := hm.GetRepositoryHook(tempDir, "nonexistent")
	if exists3 {
		t.Error("Expected hook to not exist")
	}

	// Test case 2: No hook file exists
	tempDir2, err := os.MkdirTemp("", "test-no-hooks")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	_, exists4 := hm.GetRepositoryHook(tempDir2, "any-hook")
	if exists4 {
		t.Error("Expected no hooks when file doesn't exist")
	}
}

func TestHookManager_GetHookExecutablePath(t *testing.T) {
	hm := NewHookManager()

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-hook-exec")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test executable files
	binDir := filepath.Join(tempDir, "bin")
	os.MkdirAll(binDir, 0o755)

	execFile := filepath.Join(binDir, "test-hook")
	err = os.WriteFile(execFile, []byte("#!/bin/bash\necho test"), 0o755)
	if err != nil {
		t.Fatalf("Failed to write executable: %v", err)
	}

	rootExec := filepath.Join(tempDir, "root-hook")
	err = os.WriteFile(rootExec, []byte("#!/bin/bash\necho root"), 0o755)
	if err != nil {
		t.Fatalf("Failed to write root executable: %v", err)
	}

	tests := []struct {
		name     string
		expected string
		hook     config.Hook
	}{
		{
			name: "hook with absolute entry path",
			hook: config.Hook{
				ID:    "abs-hook",
				Entry: "/usr/bin/test",
			},
			expected: "/usr/bin/test",
		},
		{
			name: "hook with relative entry path in bin",
			hook: config.Hook{
				ID:    "rel-hook",
				Entry: "bin/test-hook",
			},
			expected: filepath.Join(tempDir, "bin/test-hook"),
		},
		{
			name: "hook executable in root directory",
			hook: config.Hook{
				ID: "root-hook",
			},
			expected: filepath.Join(tempDir, "root-hook"),
		},
		{
			name: "hook executable in bin directory",
			hook: config.Hook{
				ID: "test-hook",
			},
			expected: filepath.Join(tempDir, "bin/test-hook"),
		},
		{
			name: "hook not found, returns ID",
			hook: config.Hook{
				ID: "missing-hook",
			},
			expected: "missing-hook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := hm.GetHookExecutablePath(tempDir, tt.hook)
			if err != nil {
				t.Fatalf("GetHookExecutablePath() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("GetHookExecutablePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}
