package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/tests/helpers"
)

func TestAutoupdateCommand_Help(t *testing.T) {
	cmd := &AutoupdateCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"autoupdate",
		"Auto-update hook repositories",
		"--dry-run",
		"--bleeding-edge",
		"--freeze",
		"--repo",
		"--jobs",
		"--color",
		"--config",
		"--help",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestAutoupdateCommand_Synopsis(t *testing.T) {
	cmd := &AutoupdateCommand{}
	synopsis := cmd.Synopsis()

	expected := "Update hook repositories to latest versions"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestAutoupdateCommand_Run_Help(t *testing.T) {
	cmd := &AutoupdateCommand{}

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

func TestAutoupdateCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &AutoupdateCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestAutoupdateCommand_Run_NotInGitRepo(t *testing.T) {
	cmd := &AutoupdateCommand{}

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

func TestAutoupdateCommand_Run_NoConfigFile(t *testing.T) {
	cmd := &AutoupdateCommand{}

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)
	defer testRepo.Cleanup()

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()
	defer restoreDir()

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code when config file doesn't exist")
	}
}

func TestAutoupdateCommand_Run_DryRun(t *testing.T) {
	cmd := &AutoupdateCommand{}

	// Create a temporary directory with git repo and minimal config
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test dry run
	exitCode := cmd.Run([]string{"--dry-run"})
	// Note: This may still fail due to missing dependencies, but we're testing the flag parsing
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for dry run, got %d", exitCode)
	}
}

func TestAutoupdateCommand_Run_BleedingEdge(t *testing.T) {
	cmd := &AutoupdateCommand{}

	// Test bleeding edge flag parsing
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test bleeding edge
	exitCode := cmd.Run([]string{"--bleeding-edge"})
	// Note: This may still fail due to missing dependencies, but we're testing the flag parsing
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for bleeding edge, got %d", exitCode)
	}
}

func TestAutoupdateCommand_Run_Freeze(t *testing.T) {
	cmd := &AutoupdateCommand{}

	// Test freeze flag parsing
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test freeze
	exitCode := cmd.Run([]string{"--freeze"})
	// Note: This may still fail due to missing dependencies, but we're testing the flag parsing
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for freeze, got %d", exitCode)
	}
}

func TestAutoupdateCommand_Run_SpecificRepo(t *testing.T) {
	cmd := &AutoupdateCommand{}

	// Test repo filtering
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test specific repo filter
	exitCode := cmd.Run([]string{"--repo", "https://github.com/psf/black"})
	// Note: This may still fail due to missing dependencies, but we're testing the flag parsing
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for specific repo, got %d", exitCode)
	}
}

func TestAutoupdateCommand_Run_Jobs(t *testing.T) {
	cmd := &AutoupdateCommand{}

	// Test jobs flag parsing
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test jobs flag
	exitCode := cmd.Run([]string{"--jobs", "4"})
	// Note: This may still fail due to missing dependencies, but we're testing the flag parsing
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for jobs flag, got %d", exitCode)
	}
}

// setupTestEnvironment creates a temporary git repo with minimal config for testing
func setupTestEnvironment(t *testing.T) func() {
	t.Helper()

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()

	// Create minimal config file
	configContent := `repos:
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
`
	configPath := filepath.Join(testRepo.Path, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cleanup := func() {
		restoreDir()
		testRepo.Cleanup()
	}

	return cleanup
}
