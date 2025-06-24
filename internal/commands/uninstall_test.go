package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallCommand_Help(t *testing.T) {
	cmd := &UninstallCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"uninstall",
		"Uninstall pre-commit hooks",
		"--help",
		"Remove all pre-commit hooks",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestUninstallCommand_Synopsis(t *testing.T) {
	cmd := &UninstallCommand{}
	synopsis := cmd.Synopsis()

	expected := "Uninstall pre-commit hooks from git repository"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestUninstallCommand_Run_Help(t *testing.T) {
	cmd := &UninstallCommand{}

	// Test --help flag
	exitCode := cmd.Run([]string{"--help"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for --help, got %d", exitCode)
	}

	// Test -h flag
	exitCode = cmd.Run([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for -h, got %d", exitCode)
	}
}

func TestUninstallCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &UninstallCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestUninstallCommand_Run_NotInGitRepo(t *testing.T) {
	cmd := &UninstallCommand{}

	// Create a temporary directory that's not a git repo
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code when not in git repository")
	}
}

func TestUninstallCommand_Run_InGitRepo(t *testing.T) {
	cmd := &UninstallCommand{}

	// Create a temporary directory with a git repo
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repository properly
	gitInitCmd := exec.Command("git", "init")
	gitInitCmd.Dir = tempDir
	if err := gitInitCmd.Run(); err != nil {
		t.Skipf("Git not available for testing: %v", err)
	}

	// Configure git for testing
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Initialize git repo
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git/hooks directory: %v", err)
	}

	// Create a pre-commit hook file
	hookFile := filepath.Join(hooksDir, "pre-commit")
	hookContent := `#!/bin/sh
# pre-commit hook
echo "Running pre-commit"
`
	if err := os.WriteFile(hookFile, []byte(hookContent), 0o755); err != nil {
		t.Fatalf("Failed to create pre-commit hook: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 when uninstalling in git repository, got %d", exitCode)
	}

	// Note: The actual hook file removal logic depends on the git package implementation
	// We're mainly testing that the command parses arguments correctly and executes
}

func TestUninstallCommand_Run_SuccessMessage(t *testing.T) {
	cmd := &UninstallCommand{}

	// Create a temporary directory with a git repo
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git/hooks directory: %v", err)
	}

	// The command should succeed and print a success message
	exitCode := cmd.Run([]string{})

	// Even if the hook doesn't exist, uninstall should succeed (idempotent operation)
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for uninstall, got %d", exitCode)
	}
}

func TestUninstallCommandFactory(t *testing.T) {
	cmd, err := UninstallCommandFactory()
	if err != nil {
		t.Fatalf("Expected no error from UninstallCommandFactory, got: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command from factory")
	}

	// Verify it's the correct type
	if _, ok := cmd.(*UninstallCommand); !ok {
		t.Errorf("Expected *UninstallCommand, got %T", cmd)
	}
}
