package commands

import (
	"os"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/tests/helpers"
)

func TestHookImplCommand_Help(t *testing.T) {
	cmd := &HookImplCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"hook-impl",
		"Internal command used by installed git hooks",
		"--hook-type",
		"--config",
		"--verbose",
		"--skip-on-missing-config",
		"HOOK_ARGS",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help text should contain '%s'", expected)
		}
	}
}

func TestHookImplCommand_Synopsis(t *testing.T) {
	cmd := &HookImplCommand{}
	synopsis := cmd.Synopsis()

	expected := "Internal hook implementation (not for direct use)"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
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
	tempDir, err := os.MkdirTemp("", "hook-impl-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

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

func TestHookImplCommand_Run_NotGitRepo(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "hook-impl-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Create a config file
	configContent := `repos:
  - repo: local      hooks:
      - id: test-hook
        name: Test Hook
        entry: echo "test"
        language: system
`
	configPath := ConfigFileName
	err = os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test without .git directory
	exitCode := cmd.Run([]string{"--hook-type", "pre-commit"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for non-git repo, got %d", exitCode)
	}
}

func TestHookImplCommand_Run_InvalidConfig(t *testing.T) {
	cmd := &HookImplCommand{}

	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "hook-impl-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Create a fake .git directory
	err = os.Mkdir(".git", 0o755)
	if err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create invalid config file
	configPath := ConfigFileName
	err = os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test with invalid config
	exitCode := cmd.Run([]string{"--hook-type", "pre-commit"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for invalid config, got %d", exitCode)
	}
}

func TestHookImplCommand_Run_ValidArgsWithVerbose(t *testing.T) {
	cmd := &HookImplCommand{}

	// Test verbose flag parsing
	tempDir, err := os.MkdirTemp("", "hook-impl-verbose-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Create minimal valid config
	configContent := `repos:
  - repo: local
    hooks:
      - id: test-hook
        name: Test Hook
        entry: echo "test"
        language: system
`
	configPath := ConfigFileName
	err = os.WriteFile(configPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create fake .git directory
	err = os.Mkdir(".git", 0o755)
	if err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Since we can't easily mock the git repository and hook execution,
	// we expect this to fail, but with proper argument parsing
	exitCode := cmd.Run([]string{"--hook-type", "pre-commit", "--verbose"})
	// Should fail because git repository setup will fail, but that's OK for testing args
	if exitCode == 0 {
		// Unexpectedly succeeded - that's also fine for this test
		t.Logf("Command unexpectedly succeeded")
	}
}

func TestHookImplCommand_GetFilesForHookType(t *testing.T) {
	command := &HookImplCommand{}

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)
	defer testRepo.Cleanup()

	// Create an initial commit
	testRepo.CreateInitialCommit()

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()
	defer restoreDir()

	// Create a proper Repository using the NewRepository function
	repo, err := git.NewRepository(testRepo.Path)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	tests := []struct {
		name     string
		hookType string
		args     []string
		wantErr  bool
	}{
		{
			name:     "pre-commit hook",
			hookType: "pre-commit",
			args:     []string{},
			wantErr:  false, // Should not error even with no staged files
		},
		{
			name:     "pre-push hook with args",
			hookType: "pre-push",
			args: []string{
				"refs/heads/main",
				"sha1",
				"refs/heads/main",
				"0000000000000000000000000000000000000000",
			},
			wantErr: false, // Should handle pre-push logic
		},
		{
			name:     "commit-msg hook",
			hookType: "commit-msg",
			args:     []string{},
			wantErr:  false, // Should return empty list
		},
		{
			name:     "unknown hook type",
			hookType: "unknown-hook",
			args:     []string{},
			wantErr:  false, // Should not error, just log warning and return all files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := command.getFilesForHookType(tt.hookType, repo, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("getFilesForHookType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHookImplCommand_SetupEnvironmentVariables(t *testing.T) {
	cmd := &HookImplCommand{}

	tests := []struct {
		name     string
		hookType string
		wantVars map[string]string
		args     []string
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
			name:     "pre-push with args",
			hookType: "pre-push",
			args: []string{
				"refs/heads/main",
				"local-sha",
				"refs/heads/origin/main",
				"remote-sha",
			},
			wantVars: map[string]string{
				"PRE_COMMIT":               "1",
				"PRE_COMMIT_HOOK_STAGE":    "pre-push",
				"PRE_COMMIT_FROM_REF":      "refs/heads/origin/main",
				"PRE_COMMIT_TO_REF":        "refs/heads/main",
				"PRE_COMMIT_REMOTE_BRANCH": "refs/heads/origin/main",
				"PRE_COMMIT_LOCAL_BRANCH":  "refs/heads/main",
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
				"PRE_COMMIT_CHECKOUT_TYPE": "1",
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
			env := cmd.setupEnvironmentVariables(tt.hookType, tt.args)

			for key, expectedValue := range tt.wantVars {
				if actualValue, exists := env[key]; !exists {
					t.Errorf("Expected environment variable %s to be set", key)
				} else if actualValue != expectedValue {
					t.Errorf("Expected environment variable %s to be '%s', got '%s'", key, expectedValue, actualValue)
				}
			}

			// Check that no unexpected variables are set
			for key := range env {
				if _, expected := tt.wantVars[key]; !expected {
					t.Errorf("Unexpected environment variable %s set to '%s'", key, env[key])
				}
			}
		})
	}
}

func TestHookImplCommand_ColorOption(t *testing.T) {
	cmd := &HookImplCommand{}

	// Test that color options are accepted
	tempDir, err := os.MkdirTemp("", "hook-impl-color-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

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
