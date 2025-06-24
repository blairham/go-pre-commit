package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSampleConfigCommand_Help(t *testing.T) {
	cmd := &SampleConfigCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"sample-config",
		"Generate a sample .pre-commit-config.yaml",
		"--force",
		"--help",
		"Overwrite existing configuration",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestSampleConfigCommand_Synopsis(t *testing.T) {
	cmd := &SampleConfigCommand{}
	synopsis := cmd.Synopsis()

	expected := "Generate a sample configuration file"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestSampleConfigCommand_Run_Help(t *testing.T) {
	cmd := &SampleConfigCommand{}

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

func TestSampleConfigCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &SampleConfigCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestSampleConfigCommand_Run_GenerateConfig(t *testing.T) {
	cmd := &SampleConfigCommand{}

	// Create a temporary directory
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Generate sample config
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for generating config, got %d", exitCode)
	}

	// Check that config file was created
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		t.Error("Expected config file to be created")
	}

	// Check config file content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "repos:") {
		t.Error("Expected config file to contain 'repos:'")
	}
}

func TestSampleConfigCommand_Run_ExistingConfig(t *testing.T) {
	cmd := &SampleConfigCommand{}

	// Create a temporary directory
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Create existing config file
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	existingContent := "# existing config\nrepos: []"
	if writeErr := os.WriteFile(configPath, []byte(existingContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to create existing config: %v", writeErr)
	}

	// Try to generate sample config without force (should fail)
	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code when config already exists")
	}

	// Content should remain unchanged
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if string(content) != existingContent {
		t.Error("Expected existing config to remain unchanged")
	}
}

func TestSampleConfigCommand_Run_ForceOverwrite(t *testing.T) {
	cmd := &SampleConfigCommand{}

	// Create a temporary directory
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Create existing config file
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	existingContent := "# existing config\nrepos: []"
	if writeErr := os.WriteFile(configPath, []byte(existingContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to create existing config: %v", writeErr)
	}

	// Generate sample config with force
	exitCode := cmd.Run([]string{"--force"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for force overwrite, got %d", exitCode)
	}

	// Content should be different (overwritten)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if string(content) == existingContent {
		t.Error("Expected existing config to be overwritten with --force")
	}

	// New content should contain standard sample config elements
	contentStr := string(content)
	if !strings.Contains(contentStr, "repos:") {
		t.Error("Expected new config file to contain 'repos:'")
	}
}

func TestSampleConfigCommand_Run_ForceFlag(t *testing.T) {
	cmd := &SampleConfigCommand{}

	// Create a temporary directory
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Test -f flag (short form)
	exitCode := cmd.Run([]string{"-f"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for -f flag, got %d", exitCode)
	}

	// Check that config file was created
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config file to be created with -f flag")
	}
}

func TestSampleConfigCommand_Run_ValidYAML(t *testing.T) {
	cmd := &SampleConfigCommand{}

	// Create a temporary directory
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Generate sample config
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for generating config, got %d", exitCode)
	}

	// Check that the generated YAML is valid
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Try to parse as YAML to ensure it's valid
	var yamlContent any
	if err := yaml.Unmarshal(content, &yamlContent); err != nil {
		t.Errorf("Generated config is not valid YAML: %v", err)
	}
}
