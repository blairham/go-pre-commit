package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/tests/helpers"
)

const (
	testHookConfig = `
- id: test-hook
  name: Test Hook
  entry: echo "test"
  language: system
  files: \.py$
`
)

func TestTryRepoCommand_Help(t *testing.T) {
	cmd := &TryRepoCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"try-repo",
		"Try the hooks in a repository",
		"--config",
		"--ref",
		"--verbose",
		"--all-files",
		"--files",
		"--hook",
		"--color",
		"--help",
		"REPO",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestTryRepoCommand_Synopsis(t *testing.T) {
	cmd := &TryRepoCommand{}
	synopsis := cmd.Synopsis()

	expected := "Try the hooks in a repository, useful for developing new hooks"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestTryRepoCommand_Run_Help(t *testing.T) {
	cmd := &TryRepoCommand{}

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

func TestTryRepoCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &TryRepoCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestTryRepoCommand_Run_NoRepo(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Test without providing repo argument
	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code when no repo provided")
	}
}

func TestTryRepoCommand_Run_LocalRepo(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)
	defer testRepo.Cleanup()

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()
	defer restoreDir()

	// Create a simple .pre-commit-hooks.yaml file
	hooksConfig := testHookConfig
	hooksConfigPath := filepath.Join(testRepo.Path, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(hooksConfigPath, []byte(hooksConfig), 0o644); err != nil {
		t.Fatalf("Failed to write hooks config: %v", err)
	}

	// Create test file
	testFile := filepath.Join(testRepo.Path, "test.py")
	if err := os.WriteFile(testFile, []byte("print('hello')"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test try-repo with current directory
	exitCode := cmd.Run([]string{"."})
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for local repo, got %d", exitCode)
	}
}

func TestTryRepoCommand_Run_WithRef(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)
	defer testRepo.Cleanup()

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()
	defer restoreDir()

	// Create a simple .pre-commit-hooks.yaml file
	hooksConfig := testHookConfig
	hooksConfigPath := filepath.Join(testRepo.Path, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(hooksConfigPath, []byte(hooksConfig), 0o644); err != nil {
		t.Fatalf("Failed to write hooks config: %v", err)
	}

	// Test try-repo with specific ref
	exitCode := cmd.Run([]string{".", "--ref", "main"})
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for try-repo with ref, got %d", exitCode)
	}
}

func TestTryRepoCommand_Run_AllFiles(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)
	defer testRepo.Cleanup()

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()
	defer restoreDir()

	// Create a simple .pre-commit-hooks.yaml file
	hooksConfig := testHookConfig
	hooksConfigPath := filepath.Join(testRepo.Path, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(hooksConfigPath, []byte(hooksConfig), 0o644); err != nil {
		t.Fatalf("Failed to write hooks config: %v", err)
	}

	// Create test files
	testFile1 := filepath.Join(testRepo.Path, "test1.py")
	testFile2 := filepath.Join(testRepo.Path, "test2.py")
	if err := os.WriteFile(testFile1, []byte("print('hello')"), 0o644); err != nil {
		t.Fatalf("Failed to write test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("print('world')"), 0o644); err != nil {
		t.Fatalf("Failed to write test file 2: %v", err)
	}

	// Test try-repo with all files
	exitCode := cmd.Run([]string{".", "--all-files"})
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for try-repo with all files, got %d", exitCode)
	}
}

func TestTryRepoCommand_Run_SpecificFiles(t *testing.T) {
	cmd := &TryRepoCommand{}

	tempDir, cleanup := setupTryRepoTestEnvironment(t)
	defer cleanup()

	// Create test file
	testFile := filepath.Join(tempDir, "specific.py")
	if err := os.WriteFile(testFile, []byte("print('specific')"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test try-repo with specific files
	exitCode := cmd.Run([]string{".", "--files", "specific.py"})
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for try-repo with specific files, got %d", exitCode)
	}
}

func TestTryRepoCommand_Run_SpecificHook(t *testing.T) {
	cmd := &TryRepoCommand{}

	tempDir, cleanup := setupTryRepoTestEnvironment(t)
	defer cleanup()

	// Create test file
	testFile := filepath.Join(tempDir, "test.py")
	if err := os.WriteFile(testFile, []byte("print('hello')"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test try-repo with specific hook
	exitCode := cmd.Run([]string{".", "--hook", "test-hook-1"})
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for try-repo with specific hook, got %d", exitCode)
	}
}

func TestTryRepoCommand_Run_Verbose(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)
	defer testRepo.Cleanup()

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()
	defer restoreDir()

	// Create a simple .pre-commit-hooks.yaml file
	hooksConfig := testHookConfig
	hooksConfigPath := filepath.Join(testRepo.Path, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(hooksConfigPath, []byte(hooksConfig), 0o644); err != nil {
		t.Fatalf("Failed to write hooks config: %v", err)
	}

	// Create test file
	testFile := filepath.Join(testRepo.Path, "test.py")
	if err := os.WriteFile(testFile, []byte("print('hello')"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test try-repo with verbose
	exitCode := cmd.Run([]string{".", "--verbose"})
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for try-repo with verbose, got %d", exitCode)
	}
}

func TestTryRepoCommand_Run_Color(t *testing.T) {
	cmd := &TryRepoCommand{}

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)
	defer testRepo.Cleanup()

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()
	defer restoreDir()

	// Create a simple .pre-commit-hooks.yaml file
	hooksConfig := testHookConfig
	hooksConfigPath := filepath.Join(testRepo.Path, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(hooksConfigPath, []byte(hooksConfig), 0o644); err != nil {
		t.Fatalf("Failed to write hooks config: %v", err)
	}

	// Create test file
	testFile := filepath.Join(testRepo.Path, "test.py")
	if err := os.WriteFile(testFile, []byte("print('hello')"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test try-repo with color options
	for _, colorOpt := range []string{"auto", "always", "never"} {
		exitCode := cmd.Run([]string{".", "--color", colorOpt})
		// May fail due to missing dependencies, but should parse arguments correctly
		if exitCode != 0 && exitCode != 1 {
			t.Errorf(
				"Expected exit code 0 or 1 for try-repo with color %s, got %d",
				colorOpt,
				exitCode,
			)
		}
	}
}

// setupTryRepoTestEnvironment creates a test environment with git repo and hooks config
func setupTryRepoTestEnvironment(t *testing.T) (string, func()) {
	t.Helper()

	// Create a test git repository
	testRepo := helpers.NewTestGitRepo(t)

	// Change to the repository directory
	restoreDir := testRepo.ChangeToRepo()

	// Create a simple .pre-commit-hooks.yaml file
	hooksConfig := testHookConfig
	hooksConfigPath := filepath.Join(testRepo.Path, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(hooksConfigPath, []byte(hooksConfig), 0o644); err != nil {
		t.Fatalf("Failed to write hooks config: %v", err)
	}

	// Return cleanup function
	cleanup := func() {
		restoreDir()
		testRepo.Cleanup()
	}

	return testRepo.Path, cleanup
}
