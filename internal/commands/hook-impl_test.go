package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"

	"github.com/blairham/go-pre-commit/pkg/git"
)

func TestHookImplCommand_Synopsis(t *testing.T) {
	cmd := &HookImplCommand{}
	synopsis := cmd.Synopsis()
	expected := "Internal hook implementation (not for direct use)"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestHookImplCommand_Help(t *testing.T) {
	cmd := &HookImplCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"hook-impl",
		"--hook-type",
		"--config",
		"--skip-on-missing-config",
		"--color",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help text should contain '%s'", expected)
		}
	}
}

func TestHookImplCommand_ParsePrePushStdin_EmptyStdin(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary git repo for testing
	tempDir := t.TempDir()
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755)
	repo, _ := git.NewRepository(tempDir)

	ctx := cmd.parsePrePushStdin(repo, "origin", "https://github.com/example/repo.git", []byte{})

	// Empty stdin should return nil (nothing to push)
	if ctx != nil {
		t.Errorf("Expected nil for empty stdin, got %+v", ctx)
	}
}

func TestHookImplCommand_ParsePrePushStdin_SingleRef(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary git repo for testing
	tempDir := t.TempDir()
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755)
	repo, _ := git.NewRepository(tempDir)

	stdin := []byte("refs/heads/main abc123def456 refs/heads/main 0000000000000000000000000000000000000000\n")
	ctx := cmd.parsePrePushStdin(repo, "origin", "https://github.com/example/repo.git", stdin)

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	if ctx.RemoteName != "origin" {
		t.Errorf("RemoteName = %q, want 'origin'", ctx.RemoteName)
	}

	if ctx.RemoteURL != "https://github.com/example/repo.git" {
		t.Errorf("RemoteURL = %q, want 'https://github.com/example/repo.git'", ctx.RemoteURL)
	}

	if len(ctx.Refs) != 1 {
		t.Fatalf("Expected 1 ref, got %d", len(ctx.Refs))
	}

	ref := ctx.Refs[0]
	if ref.LocalBranch != "refs/heads/main" {
		t.Errorf("LocalBranch = %q, want 'refs/heads/main'", ref.LocalBranch)
	}
	if ref.LocalSHA != "abc123def456" {
		t.Errorf("LocalSHA = %q, want 'abc123def456'", ref.LocalSHA)
	}
	if ref.RemoteBranch != "refs/heads/main" {
		t.Errorf("RemoteBranch = %q, want 'refs/heads/main'", ref.RemoteBranch)
	}
	if ref.RemoteSHA != Z40 {
		t.Errorf("RemoteSHA = %q, want Z40", ref.RemoteSHA)
	}
}

func TestHookImplCommand_ParsePrePushStdin_MultipleRefs(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary git repo for testing
	tempDir := t.TempDir()
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755)
	repo, _ := git.NewRepository(tempDir)

	stdin := []byte("refs/heads/main abc123 refs/heads/main def456\nrefs/heads/feature ghi789 refs/heads/feature jkl012\n")
	ctx := cmd.parsePrePushStdin(repo, "origin", "https://github.com/example/repo.git", stdin)

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	if len(ctx.Refs) != 2 {
		t.Fatalf("Expected 2 refs, got %d", len(ctx.Refs))
	}

	if ctx.Refs[0].LocalBranch != "refs/heads/main" {
		t.Errorf("First ref LocalBranch = %q, want 'refs/heads/main'", ctx.Refs[0].LocalBranch)
	}
	if ctx.Refs[1].LocalBranch != "refs/heads/feature" {
		t.Errorf("Second ref LocalBranch = %q, want 'refs/heads/feature'", ctx.Refs[1].LocalBranch)
	}
}

func TestHookImplCommand_ParsePrePushStdin_SkipsDeletion(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary git repo for testing
	tempDir := t.TempDir()
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755)
	repo, _ := git.NewRepository(tempDir)

	// Deletion: local sha is Z40
	stdin := []byte(Z40 + " 0000000000000000000000000000000000000000 refs/heads/main abc123\n")
	ctx := cmd.parsePrePushStdin(repo, "origin", "https://github.com/example/repo.git", stdin)

	// Deletion should be skipped, so context should be nil or have empty from/to refs
	// Since no valid refs to process, returns nil
	if ctx != nil && ctx.FromRef != "" {
		t.Errorf("Expected no FromRef for deletion, got %q", ctx.FromRef)
	}
}

func TestHookImplCommand_ParsePrePushStdin_MalformedLine(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary git repo for testing
	tempDir := t.TempDir()
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755)
	repo, _ := git.NewRepository(tempDir)

	// Malformed line (only 2 parts instead of 4)
	stdin := []byte("refs/heads/main abc123\n")
	ctx := cmd.parsePrePushStdin(repo, "origin", "https://github.com/example/repo.git", stdin)

	// Malformed lines should be skipped
	if ctx != nil && len(ctx.Refs) > 0 {
		t.Errorf("Expected no refs for malformed input, got %d", len(ctx.Refs))
	}
}

func TestHookImplCommand_ParsePrePushStdin_WithNewlines(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary git repo for testing
	tempDir := t.TempDir()
	os.MkdirAll(filepath.Join(tempDir, ".git"), 0o755)
	repo, _ := git.NewRepository(tempDir)

	stdin := []byte("\nrefs/heads/main abc123 refs/heads/main def456\n\n")
	ctx := cmd.parsePrePushStdin(repo, "origin", "https://github.com/example/repo.git", stdin)

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	if len(ctx.Refs) != 1 {
		t.Errorf("Expected 1 ref (blank lines ignored), got %d", len(ctx.Refs))
	}
}

func TestHookImplCommand_RevExists(t *testing.T) {
	// This test runs in a real git repo
	cmd := &HookImplCommand{}

	// Create a temporary git repo for testing
	repo, err := git.NewRepository("")
	if err != nil {
		t.Skip("Not in a git repository, skipping test")
	}

	// HEAD should exist in the current repo
	if !cmd.revExists(repo, "HEAD") {
		t.Log("Note: HEAD doesn't exist, may not be in a git repo")
	}

	// Non-existent ref should return false
	// Use a refs path that definitely doesn't exist
	if cmd.revExists(repo, "refs/heads/this-branch-absolutely-does-not-exist-12345") {
		t.Error("Non-existent branch ref should not exist")
	}
}

func TestHookImplCommand_Run_InvalidArgs(t *testing.T) {
	cmd := &HookImplCommand{}

	// Test with help flag
	exitCode := cmd.Run([]string{"--help"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for help, got %d", exitCode)
	}

	// Test missing hook-type
	exitCode = cmd.Run([]string{})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for missing hook-type, got %d", exitCode)
	}

	// Test with explicit empty hook-type
	exitCode = cmd.Run([]string{"--hook-type", ""})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for empty hook-type, got %d", exitCode)
	}
}

func TestHookImplCommand_Run_MissingConfig(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary directory
	tempDir := t.TempDir()

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Initialize a git repository using go-git
	_, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Test with missing config file (default path)
	exitCode := cmd.Run([]string{"--hook-type", "pre-commit"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for missing config, got %d", exitCode)
	}

	// Test with skip-on-missing-config
	exitCode = cmd.Run([]string{"--hook-type", "pre-commit", "--skip-on-missing-config"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with skip-on-missing-config, got %d", exitCode)
	}

	// Test with custom missing config file
	exitCode = cmd.Run([]string{"--hook-type", "pre-commit", "--config", "nonexistent.yaml"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for missing custom config, got %d", exitCode)
	}
}

func TestHookImplCommand_SetupEnvironmentVariables(t *testing.T) {
	cmd := &HookImplCommand{}

	tests := []struct {
		name     string
		hookType string
		args     []string
		wantVars map[string]string
	}{
		{
			name:     "basic pre-commit",
			hookType: "pre-commit",
			args:     []string{},
			wantVars: map[string]string{
				"PRE_COMMIT":            "1",
				"PRE_COMMIT_HOOK_STAGE": "pre-commit",
			},
		},
		{
			name:     "commit-msg with filename",
			hookType: "commit-msg",
			args:     []string{".git/COMMIT_EDITMSG"},
			wantVars: map[string]string{
				"PRE_COMMIT":                     "1",
				"PRE_COMMIT_HOOK_STAGE":          "commit-msg",
				"PRE_COMMIT_COMMIT_MSG_FILENAME": ".git/COMMIT_EDITMSG",
			},
		},
		{
			name:     "prepare-commit-msg with args",
			hookType: "prepare-commit-msg",
			args:     []string{".git/COMMIT_EDITMSG", "message", "commit-sha"},
			wantVars: map[string]string{
				"PRE_COMMIT":                     "1",
				"PRE_COMMIT_HOOK_STAGE":          "prepare-commit-msg",
				"PRE_COMMIT_COMMIT_MSG_FILENAME": ".git/COMMIT_EDITMSG",
				"PRE_COMMIT_COMMIT_MSG_SOURCE":   "message",
				"PRE_COMMIT_COMMIT_OBJECT_NAME":  "commit-sha",
			},
		},
		{
			name:     "post-checkout",
			hookType: "post-checkout",
			args:     []string{"old-sha", "new-sha", "1"},
			wantVars: map[string]string{
				"PRE_COMMIT":               "1",
				"PRE_COMMIT_HOOK_STAGE":    "post-checkout",
				"PRE_COMMIT_FROM_REF":      "old-sha",
				"PRE_COMMIT_TO_REF":        "new-sha",
				"PRE_COMMIT_CHECKOUT_TYPE": "1",
			},
		},
		{
			name:     "post-merge",
			hookType: "post-merge",
			args:     []string{"1"},
			wantVars: map[string]string{
				"PRE_COMMIT":                 "1",
				"PRE_COMMIT_HOOK_STAGE":      "post-merge",
				"PRE_COMMIT_IS_SQUASH_MERGE": "1",
			},
		},
		{
			name:     "post-rewrite",
			hookType: "post-rewrite",
			args:     []string{"rebase"},
			wantVars: map[string]string{
				"PRE_COMMIT":                 "1",
				"PRE_COMMIT_HOOK_STAGE":      "post-rewrite",
				"PRE_COMMIT_REWRITE_COMMAND": "rebase",
			},
		},
		{
			name:     "pre-rebase",
			hookType: "pre-rebase",
			args:     []string{"upstream", "branch"},
			wantVars: map[string]string{
				"PRE_COMMIT":                     "1",
				"PRE_COMMIT_HOOK_STAGE":          "pre-rebase",
				"PRE_COMMIT_PRE_REBASE_UPSTREAM": "upstream",
				"PRE_COMMIT_PRE_REBASE_BRANCH":   "branch",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := cmd.setupEnvironmentVariables(tt.hookType, tt.args, nil)

			for key, expectedValue := range tt.wantVars {
				if actualValue, exists := env[key]; !exists {
					t.Errorf("Expected environment variable %s to be set", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected environment variable %s to be '%s', got '%s'", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestHookImplCommand_ColorOption(t *testing.T) {
	cmd := &HookImplCommand{}

	// Test that color options are accepted
	tempDir := t.TempDir()

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Test color options (these will fail due to missing config, but should parse correctly)
	colorOptions := []string{"auto", "always", "never"}

	for _, colorOption := range colorOptions {
		exitCode := cmd.Run([]string{"--hook-type", "pre-commit", "--color", colorOption})
		// Should fail due to missing config, but not due to invalid color option
		if exitCode != 1 {
			t.Errorf(
				"Expected exit code 1 for missing config with color=%s, got %d",
				colorOption,
				exitCode,
			)
		}
	}
}

func TestHookImplCommand_Factory(t *testing.T) {
	cmd, err := HookImplCommandFactory()
	if err != nil {
		t.Fatalf("Factory should not return error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Factory should return a command")
	}

	if _, ok := cmd.(*HookImplCommand); !ok {
		t.Fatal("Factory should return a HookImplCommand")
	}
}
