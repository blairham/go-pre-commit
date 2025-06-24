package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfigCommand_Help(t *testing.T) {
	cmd := &ValidateConfigCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"validate-config",
		"Validate the .pre-commit-config.yaml",
		"--help",
		"Checks the syntax and structure",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestValidateConfigCommand_Synopsis(t *testing.T) {
	cmd := &ValidateConfigCommand{}
	synopsis := cmd.Synopsis()

	expected := "Validate configuration file"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestValidateConfigCommand_Run_Help(t *testing.T) {
	cmd := &ValidateConfigCommand{}

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

func TestValidateConfigCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &ValidateConfigCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestValidateConfigCommand_Run_NoConfigFile(t *testing.T) {
	cmd := &ValidateConfigCommand{}

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
	if exitCode == 0 {
		t.Error("Expected non-zero exit code when config file doesn't exist")
	}
}

func TestValidateConfigCommand_Run_ValidConfig(t *testing.T) {
	cmd := &ValidateConfigCommand{}

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
	configContent := `repos:
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
    files: \.py$
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for valid config, got %d", exitCode)
	}
}

func TestValidateConfigCommand_Run_InvalidConfig(t *testing.T) {
	cmd := &ValidateConfigCommand{}

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
	configContent := `repos:
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
    invalid_yaml: [unclosed bracket
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid config")
	}
}

func TestValidateConfigCommand_Run_EmptyConfig(t *testing.T) {
	cmd := &ValidateConfigCommand{}

	// Create a temporary directory with empty config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create empty config file
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for empty config")
	}
}

func TestValidateConfigCommand_Run_MinimalValidConfig(t *testing.T) {
	cmd := &ValidateConfigCommand{}

	// Create a temporary directory with minimal valid config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create minimal valid config file
	configContent := `repos: []
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for minimal valid config, got %d", exitCode)
	}
}

func TestValidateConfigCommand_Run_ConfigWithRepos(t *testing.T) {
	cmd := &ValidateConfigCommand{}

	// Create a temporary directory with config containing repos
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create config file with multiple repos
	configContent := `repos:
- repo: https://github.com/psf/black
  rev: 22.3.0
  hooks:
  - id: black
    language_version: python3.9
- repo: local
  hooks:
  - id: pylint
    name: pylint
    entry: pylint
    language: system
    types: [python]
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for config with repos, got %d", exitCode)
	}
}

func TestValidateConfigCommandFactory(t *testing.T) {
	cmd, err := ValidateConfigCommandFactory()
	if err != nil {
		t.Fatalf("Expected no error from ValidateConfigCommandFactory, got: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command from factory")
	}

	// Verify it's the correct type
	if _, ok := cmd.(*ValidateConfigCommand); !ok {
		t.Errorf("Expected *ValidateConfigCommand, got %T", cmd)
	}
}
