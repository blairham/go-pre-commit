package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateConfigCommand_Help(t *testing.T) {
	cmd := &MigrateConfigCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"migrate-config",
		"Migrate list configuration to new map configuration",
		"--config",
		"--verbose",
		"--help",
		"Old format:",
		"New format:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestMigrateConfigCommand_Synopsis(t *testing.T) {
	cmd := &MigrateConfigCommand{}
	synopsis := cmd.Synopsis()

	expected := "Migrate list configuration to new map configuration"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestMigrateConfigCommand_Run_Help(t *testing.T) {
	cmd := &MigrateConfigCommand{}

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

func TestMigrateConfigCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestMigrateConfigCommand_Run_NoConfigFile(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	// Create a temporary directory without config file
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code when config file doesn't exist")
	}
}

func TestMigrateConfigCommand_Run_OldFormatConfig(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	// Create a temporary directory with old format config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Create old format config (list format)
	oldConfigContent := `- repo: https://github.com/psf/black
  rev: 22.3.0
  hooks:
  - id: black
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if writeErr := os.WriteFile(configPath, []byte(oldConfigContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write old config file: %v", writeErr)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for migrating old config, got %d", exitCode)
	}

	// Check that config was migrated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read migrated config: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "repos:") {
		t.Error("Expected migrated config to contain 'repos:' key")
	}
}

func TestMigrateConfigCommand_Run_NewFormatConfig(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	// Create a temporary directory with new format config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Create new format config (already has repos key)
	newConfigContent := `repos:
- repo: https://github.com/psf/black
  rev: 22.3.0
  hooks:
  - id: black
- repo: local
  hooks:
  - id: test-hook
    name: Test Hook
    entry: echo "test"
    language: system
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if writeErr := os.WriteFile(configPath, []byte(newConfigContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write new config file: %v", writeErr)
	}

	exitCode := cmd.Run([]string{})
	// Should succeed (no migration needed) or give appropriate message
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for new format config, got %d", exitCode)
	}

	// Config should remain the same (already in new format)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if string(content) != newConfigContent {
		t.Error("Expected new format config to remain unchanged")
	}
}

func TestMigrateConfigCommand_Run_CustomConfig(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	// Create a temporary directory with custom config path
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Create old format config with custom name
	oldConfigContent := `- repo: https://github.com/psf/black
  rev: 22.3.0
  hooks:
  - id: black
`
	customConfigPath := filepath.Join(tempDir, "custom-config.yaml")
	if writeErr := os.WriteFile(customConfigPath, []byte(oldConfigContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write custom config file: %v", writeErr)
	}

	exitCode := cmd.Run([]string{"--config", "custom-config.yaml"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for custom config migration, got %d", exitCode)
	}

	// Check that custom config was migrated
	content, err := os.ReadFile(customConfigPath)
	if err != nil {
		t.Fatalf("Failed to read migrated custom config: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "repos:") {
		t.Error("Expected migrated custom config to contain 'repos:' key")
	}
}

func TestMigrateConfigCommand_Run_Verbose(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	// Create a temporary directory with old format config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Create old format config
	oldConfigContent := `- repo: https://github.com/psf/black
  rev: 22.3.0
  hooks:
  - id: black
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if writeErr := os.WriteFile(configPath, []byte(oldConfigContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write old config file: %v", writeErr)
	}

	exitCode := cmd.Run([]string{"--verbose"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for verbose migration, got %d", exitCode)
	}

	// Check that config was migrated
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read migrated config: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "repos:") {
		t.Error("Expected migrated config to contain 'repos:' key")
	}
}

func TestMigrateConfigCommand_Run_EmptyConfig(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	// Create a temporary directory with empty config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Create empty config file
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if writeErr := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatalf("Failed to write empty config file: %v", writeErr)
	}

	exitCode := cmd.Run([]string{})
	// Should handle empty config gracefully
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for empty config, got %d", exitCode)
	}
}

func TestMigrateConfigCommand_Run_InvalidYAML(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	// Create a temporary directory with invalid YAML config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Create invalid YAML config
	invalidConfigContent := `- repo: https://github.com/psf/black
  rev: 22.3.0
  hooks:
  - id: black
    invalid_yaml: [unclosed bracket
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if writeErr := os.WriteFile(configPath, []byte(invalidConfigContent), 0o644); err != nil {
		t.Fatalf("Failed to write invalid config file: %v", writeErr)
	}

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid YAML config")
	}
}
