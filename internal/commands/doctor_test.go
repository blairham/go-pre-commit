package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test constants for repeated strings
const (
	testRepoConfig = `repos:
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
    invalid_yaml: [unclosed bracket
`
	emptyReposConfig = `repos: []
`
	validRepoConfig = `repos:
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
`
	validRepoConfigWithFiles = `repos:
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
    files: \.py$
`
)

func TestDoctorCommand_Help(t *testing.T) {
	cmd := &DoctorCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"doctor",
		"Check and repair pre-commit environment",
		"--config",
		"--fix",
		"--verbose",
		"--help",
		"Exit codes:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestDoctorCommand_Synopsis(t *testing.T) {
	cmd := &DoctorCommand{}
	synopsis := cmd.Synopsis()

	expected := "Check and repair environment health"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestDoctorCommand_Run_Help(t *testing.T) {
	cmd := &DoctorCommand{}

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

func TestDoctorCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &DoctorCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestDoctorCommand_Run_NoConfigFile(t *testing.T) {
	cmd := &DoctorCommand{}

	// Create a temporary directory without config file
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
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 when config file doesn't exist, got %d", exitCode)
	}
}

func TestDoctorCommand_Run_CustomConfigFile(t *testing.T) {
	cmd := &DoctorCommand{}

	// Create a temporary directory with custom config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create custom config file
	configContent := validRepoConfig
	customConfigPath := filepath.Join(tempDir, "custom-config.yaml")
	if err := os.WriteFile(customConfigPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write custom config file: %v", err)
	}

	exitCode := cmd.Run([]string{"--config", "custom-config.yaml"})
	// Should succeed or have a specific doctor exit code (0, 1, or 2)
	if exitCode != 0 && exitCode != 1 && exitCode != 2 {
		t.Errorf("Expected exit code 0, 1, or 2 for custom config, got %d", exitCode)
	}
}

func TestDoctorCommand_Run_ValidConfig(t *testing.T) {
	cmd := &DoctorCommand{}

	// Create a temporary directory with valid config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create valid config file
	configContent := validRepoConfigWithFiles
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	// Doctor command may succeed or find issues, but shouldn't crash
	if exitCode != 0 && exitCode != 1 && exitCode != 2 {
		t.Errorf("Expected exit code 0, 1, or 2 for valid config, got %d", exitCode)
	}
}

func TestDoctorCommand_Run_Verbose(t *testing.T) {
	cmd := &DoctorCommand{}

	// Create a temporary directory with valid config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create valid config file
	configContent := validRepoConfig
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{"--verbose"})
	// Doctor command may succeed or find issues, but shouldn't crash
	if exitCode != 0 && exitCode != 1 && exitCode != 2 {
		t.Errorf("Expected exit code 0, 1, or 2 for verbose mode, got %d", exitCode)
	}
}

func TestDoctorCommand_Run_Fix(t *testing.T) {
	cmd := &DoctorCommand{}

	// Create a temporary directory with valid config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create valid config file
	configContent := validRepoConfig
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{"--fix"})
	// Doctor command may succeed or find issues, but shouldn't crash
	if exitCode != 0 && exitCode != 1 && exitCode != 2 {
		t.Errorf("Expected exit code 0, 1, or 2 for fix mode, got %d", exitCode)
	}
}

func TestDoctorCommand_Run_VerboseAndFix(t *testing.T) {
	cmd := &DoctorCommand{}

	// Create a temporary directory with valid config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create valid config file
	configContent := validRepoConfig
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{"--verbose", "--fix"})
	// Doctor command may succeed or find issues, but shouldn't crash
	if exitCode != 0 && exitCode != 1 && exitCode != 2 {
		t.Errorf("Expected exit code 0, 1, or 2 for verbose and fix mode, got %d", exitCode)
	}
}

func TestDoctorCommand_Run_InvalidConfig(t *testing.T) {
	cmd := &DoctorCommand{}

	// Create a temporary directory with invalid config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create invalid config file (malformed YAML)
	configContent := testRepoConfig
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 2 {
		t.Errorf("Expected exit code 2 for invalid config, got %d", exitCode)
	}
}

func TestDoctorCommand_Run_EmptyRepos(t *testing.T) {
	cmd := &DoctorCommand{}

	// Create a temporary directory with empty repos config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create config file with empty repos
	configContent := emptyReposConfig
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	// Should succeed with empty repos
	if exitCode != 0 && exitCode != 1 && exitCode != 2 {
		t.Errorf("Expected exit code 0, 1, or 2 for empty repos, got %d", exitCode)
	}
}
