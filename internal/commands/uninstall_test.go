package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUninstallCommand_AlwaysReturnsZero verifies that uninstall always returns 0
// like Python's implementation (even when not in a git repo or on errors)
func TestUninstallCommand_AlwaysReturnsZero(t *testing.T) {
	cmd := &UninstallCommand{}

	// Test various scenarios that should all return 0

	// 1. With no arguments (not in a git repo context)
	// Save current dir and change to temp dir
	origDir, _ := os.Getwd()
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	result := cmd.Run([]string{})
	if result != 0 {
		t.Errorf("Expected return code 0, got %d", result)
	}

	// 2. With --help flag
	result = cmd.Run([]string{"--help"})
	if result != 0 {
		t.Errorf("Expected return code 0 for --help, got %d", result)
	}
}

// TestUninstallCommand_IsOurHook_HashDetection tests that isOurHook uses hash-based detection
func TestUninstallCommand_IsOurHook_HashDetection(t *testing.T) {
	cmd := &UninstallCommand{}
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "current hash marker",
			content:  "#!/bin/bash\n# 138fd403232d2ddd5efb44317e38bf03\npre-commit run\n",
			expected: true,
		},
		{
			name:     "prior hash v1",
			content:  "#!/bin/bash\n# 4d9958c90bc262f47553e2c073f14cfe\nsome content\n",
			expected: true,
		},
		{
			name:     "prior hash v2",
			content:  "#!/bin/bash\n# d8ee923c46731b42cd95cc869add4062\nsome content\n",
			expected: true,
		},
		{
			name:     "prior hash v3",
			content:  "#!/bin/bash\n# 49fd668cb42069aa1b6048464be5d395\nsome content\n",
			expected: true,
		},
		{
			name:     "prior hash v4",
			content:  "#!/bin/bash\n# 79f09a650522a87b0da915d0d983b2de\nsome content\n",
			expected: true,
		},
		{
			name:     "prior hash v5",
			content:  "#!/bin/bash\n# e358c9dae00eac5d06b38dfdb1e33a8c\nsome content\n",
			expected: true,
		},
		{
			name:     "not our hook - just contains pre-commit string",
			content:  "#!/bin/bash\n# This is a pre-commit hook\necho hello\n",
			expected: false,
		},
		{
			name:     "not our hook - custom hook",
			content:  "#!/bin/bash\necho 'custom hook'\n",
			expected: false,
		},
		{
			name:     "not our hook - different hash",
			content:  "#!/bin/bash\n# abcd1234abcd1234abcd1234abcd1234\necho hello\n",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hookPath := filepath.Join(tempDir, tt.name)
			if err := os.WriteFile(hookPath, []byte(tt.content), 0o755); err != nil {
				t.Fatalf("Failed to write test hook: %v", err)
			}

			result, err := cmd.isOurHook(hookPath)
			if err != nil {
				t.Fatalf("isOurHook returned error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("isOurHook(%q) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

// TestUninstallCommand_IsOurHook_NonExistentFile tests handling of non-existent files
func TestUninstallCommand_IsOurHook_NonExistentFile(t *testing.T) {
	cmd := &UninstallCommand{}

	result, err := cmd.isOurHook("/nonexistent/path/hook")
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got: %v", err)
	}
	if result != false {
		t.Errorf("Expected false for non-existent file, got true")
	}
}

// TestUninstallCommand_DoesNotRemoveNonOurHooks verifies we don't touch hooks
// that weren't installed by pre-commit
func TestUninstallCommand_DoesNotRemoveNonOurHooks(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}

	// Initialize git repo
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create HEAD: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		t.Fatalf("Failed to create refs/heads: %v", err)
	}

	// Create a custom hook (not ours)
	customHookContent := "#!/usr/bin/env bash\necho 'custom hook'\n"
	customHookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(customHookPath, []byte(customHookContent), 0o755); err != nil {
		t.Fatalf("Failed to write custom hook: %v", err)
	}

	// Run uninstall
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	cmd := &UninstallCommand{}
	result := cmd.Run([]string{})

	if result != 0 {
		t.Errorf("Expected return code 0, got %d", result)
	}

	// Verify the custom hook still exists
	if _, err := os.Stat(customHookPath); os.IsNotExist(err) {
		t.Error("Custom hook was incorrectly removed")
	}

	// Verify content is unchanged
	content, _ := os.ReadFile(customHookPath)
	if string(content) != customHookContent {
		t.Error("Custom hook content was modified")
	}
}

// TestUninstallCommand_RemovesOurHooks verifies we correctly remove hooks
// that were installed by pre-commit
func TestUninstallCommand_RemovesOurHooks(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}

	// Initialize git repo
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create HEAD: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		t.Fatalf("Failed to create refs/heads: %v", err)
	}

	// Create our hook (with hash marker)
	ourHookContent := "#!/bin/bash\n# 138fd403232d2ddd5efb44317e38bf03\npre-commit run\n"
	ourHookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(ourHookPath, []byte(ourHookContent), 0o755); err != nil {
		t.Fatalf("Failed to write our hook: %v", err)
	}

	// Run uninstall
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	cmd := &UninstallCommand{}
	result := cmd.Run([]string{})

	if result != 0 {
		t.Errorf("Expected return code 0, got %d", result)
	}

	// Verify our hook was removed
	if _, err := os.Stat(ourHookPath); !os.IsNotExist(err) {
		t.Error("Our hook was not removed")
	}
}

// TestUninstallCommand_RestoresLegacyHooks verifies legacy hooks are restored
func TestUninstallCommand_RestoresLegacyHooks(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}

	// Initialize git repo
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create HEAD: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		t.Fatalf("Failed to create refs/heads: %v", err)
	}

	// Create our hook
	ourHookPath := filepath.Join(hooksDir, "pre-commit")
	ourHookContent := "#!/bin/bash\n# 138fd403232d2ddd5efb44317e38bf03\npre-commit run\n"
	if err := os.WriteFile(ourHookPath, []byte(ourHookContent), 0o755); err != nil {
		t.Fatalf("Failed to write our hook: %v", err)
	}

	// Create legacy hook
	legacyPath := ourHookPath + ".legacy"
	legacyContent := "#!/bin/bash\necho 'legacy hook'\n"
	if err := os.WriteFile(legacyPath, []byte(legacyContent), 0o755); err != nil {
		t.Fatalf("Failed to write legacy hook: %v", err)
	}

	// Run uninstall
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	cmd := &UninstallCommand{}
	result := cmd.Run([]string{})

	if result != 0 {
		t.Errorf("Expected return code 0, got %d", result)
	}

	// Verify our hook was replaced with legacy
	content, err := os.ReadFile(ourHookPath)
	if err != nil {
		t.Fatalf("Failed to read restored hook: %v", err)
	}

	if string(content) != legacyContent {
		t.Errorf("Legacy hook was not restored correctly.\nGot: %s\nWant: %s", content, legacyContent)
	}

	// Verify legacy file is gone
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("Legacy file was not removed after restore")
	}
}

// TestUninstallCommand_SilentWhenHookDoesNotExist verifies silent behavior
func TestUninstallCommand_SilentWhenHookDoesNotExist(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}

	// Initialize git repo (no hooks created)
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create HEAD: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		t.Fatalf("Failed to create refs/heads: %v", err)
	}

	// Run uninstall (no hooks exist)
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	cmd := &UninstallCommand{}
	result := cmd.Run([]string{})

	// Should return 0 silently
	if result != 0 {
		t.Errorf("Expected return code 0, got %d", result)
	}
}

// TestUninstallCommand_MultipleHookTypes verifies uninstalling multiple hook types
func TestUninstallCommand_MultipleHookTypes(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}

	// Initialize git repo
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create HEAD: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		t.Fatalf("Failed to create refs/heads: %v", err)
	}

	ourHookContent := "#!/bin/bash\n# 138fd403232d2ddd5efb44317e38bf03\npre-commit run\n"

	// Create pre-commit hook
	preCommitPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(preCommitPath, []byte(ourHookContent), 0o755); err != nil {
		t.Fatalf("Failed to write pre-commit hook: %v", err)
	}

	// Create pre-push hook
	prePushPath := filepath.Join(hooksDir, "pre-push")
	if err := os.WriteFile(prePushPath, []byte(ourHookContent), 0o755); err != nil {
		t.Fatalf("Failed to write pre-push hook: %v", err)
	}

	// Run uninstall with both hook types
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	cmd := &UninstallCommand{}
	result := cmd.Run([]string{"-t", "pre-commit", "-t", "pre-push"})

	if result != 0 {
		t.Errorf("Expected return code 0, got %d", result)
	}

	// Verify both hooks were removed
	if _, err := os.Stat(preCommitPath); !os.IsNotExist(err) {
		t.Error("pre-commit hook was not removed")
	}
	if _, err := os.Stat(prePushPath); !os.IsNotExist(err) {
		t.Error("pre-push hook was not removed")
	}
}

// TestUninstallCommand_HashConstants verifies the hash constants match Python
func TestUninstallCommand_HashConstants(t *testing.T) {
	// Verify CURRENT_HASH matches Python's current hash
	expectedCurrentHash := "138fd403232d2ddd5efb44317e38bf03"
	if string(CURRENT_HASH) != expectedCurrentHash {
		t.Errorf("CURRENT_HASH mismatch.\nGot: %s\nWant: %s", CURRENT_HASH, expectedCurrentHash)
	}

	// Verify PRIOR_HASHES match Python's prior hashes
	expectedPriorHashes := []string{
		"4d9958c90bc262f47553e2c073f14cfe",
		"d8ee923c46731b42cd95cc869add4062",
		"49fd668cb42069aa1b6048464be5d395",
		"79f09a650522a87b0da915d0d983b2de",
		"e358c9dae00eac5d06b38dfdb1e33a8c",
	}

	if len(PRIOR_HASHES) != len(expectedPriorHashes) {
		t.Errorf("PRIOR_HASHES count mismatch. Got %d, want %d", len(PRIOR_HASHES), len(expectedPriorHashes))
	}

	for i, expected := range expectedPriorHashes {
		if i >= len(PRIOR_HASHES) {
			break
		}
		if string(PRIOR_HASHES[i]) != expected {
			t.Errorf("PRIOR_HASHES[%d] mismatch.\nGot: %s\nWant: %s", i, PRIOR_HASHES[i], expected)
		}
	}
}

// TestUninstallCommand_OutputFormat tests that output matches Python format
func TestUninstallCommand_OutputFormat(t *testing.T) {
	// Create a temporary git repository
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")

	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create hooks dir: %v", err)
	}

	// Initialize git repo
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create HEAD: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		t.Fatalf("Failed to create refs/heads: %v", err)
	}

	// Create our hook with legacy
	ourHookPath := filepath.Join(hooksDir, "pre-commit")
	ourHookContent := "#!/bin/bash\n# 138fd403232d2ddd5efb44317e38bf03\npre-commit run\n"
	if err := os.WriteFile(ourHookPath, []byte(ourHookContent), 0o755); err != nil {
		t.Fatalf("Failed to write our hook: %v", err)
	}

	legacyPath := ourHookPath + ".legacy"
	if err := os.WriteFile(legacyPath, []byte("#!/bin/bash\necho legacy\n"), 0o755); err != nil {
		t.Fatalf("Failed to write legacy hook: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run uninstall
	origDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origDir)

	cmd := &UninstallCommand{}
	_ = cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	// Check output format matches Python
	if !strings.Contains(output, "pre-commit uninstalled") {
		t.Errorf("Expected 'pre-commit uninstalled' in output, got: %s", output)
	}

	// Python outputs full path for restored hooks
	if !strings.Contains(output, "Restored previous hooks to") {
		t.Errorf("Expected 'Restored previous hooks to' in output, got: %s", output)
	}

	// Verify it uses full path (contains the temp dir path)
	if !strings.Contains(output, hooksDir) {
		t.Errorf("Expected full path in output containing %s, got: %s", hooksDir, output)
	}
}
