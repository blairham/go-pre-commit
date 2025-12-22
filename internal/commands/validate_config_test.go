package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateConfigCommand_ValidFile tests validating a valid config file
func TestValidateConfigCommand_ValidFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a valid config file
	validConfig := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.0.0
    hooks:
    -   id: trailing-whitespace
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cmd := &ValidateConfigCommand{}
	result := cmd.Run([]string{configPath})

	if result != 0 {
		t.Errorf("Expected return code 0 for valid config, got %d", result)
	}
}

// TestValidateConfigCommand_InvalidFile tests validating an invalid config file
func TestValidateConfigCommand_InvalidFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create an invalid config file (invalid YAML)
	invalidConfig := `{`
	configPath := filepath.Join(tempDir, "invalid.yaml")
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cmd := &ValidateConfigCommand{}
	result := cmd.Run([]string{configPath})

	if result != 1 {
		t.Errorf("Expected return code 1 for invalid config, got %d", result)
	}
}

// TestValidateConfigCommand_NonExistentFile tests validating a non-existent file
func TestValidateConfigCommand_NonExistentFile(t *testing.T) {
	cmd := &ValidateConfigCommand{}
	result := cmd.Run([]string{"does-not-exist.yaml"})

	if result != 1 {
		t.Errorf("Expected return code 1 for non-existent file, got %d", result)
	}
}

// TestValidateConfigCommand_MultipleFiles tests validating multiple files
func TestValidateConfigCommand_MultipleFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create two valid config files
	validConfig := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.0.0
    hooks:
    -   id: trailing-whitespace
`
	config1 := filepath.Join(tempDir, "config1.yaml")
	config2 := filepath.Join(tempDir, "config2.yaml")
	if err := os.WriteFile(config1, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	if err := os.WriteFile(config2, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cmd := &ValidateConfigCommand{}
	result := cmd.Run([]string{config1, config2})

	if result != 0 {
		t.Errorf("Expected return code 0 for valid configs, got %d", result)
	}
}

// TestValidateConfigCommand_MultipleFilesOneFails tests that all files are validated
// even if one fails (matching Python behavior)
func TestValidateConfigCommand_MultipleFilesOneFails(t *testing.T) {
	tempDir := t.TempDir()

	// Create one valid and one invalid config file
	validConfig := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.0.0
    hooks:
    -   id: trailing-whitespace
`
	invalidConfig := `{`

	validPath := filepath.Join(tempDir, "valid.yaml")
	invalidPath := filepath.Join(tempDir, "invalid.yaml")
	if err := os.WriteFile(validPath, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	if err := os.WriteFile(invalidPath, []byte(invalidConfig), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cmd := &ValidateConfigCommand{}
	// Invalid file first, valid file second - both should be processed
	result := cmd.Run([]string{invalidPath, validPath})

	// Should return 1 because one file was invalid
	if result != 1 {
		t.Errorf("Expected return code 1 when one file is invalid, got %d", result)
	}
}

// TestValidateConfigCommand_NoFiles tests behavior with no files
func TestValidateConfigCommand_NoFiles(t *testing.T) {
	cmd := &ValidateConfigCommand{}
	result := cmd.Run([]string{})

	// With no files, should return 0 (nothing to validate = success)
	// This matches Python's behavior
	if result != 0 {
		t.Errorf("Expected return code 0 for no files, got %d", result)
	}
}

// TestValidateConfigCommand_SilentOnSuccess tests that there's no output on success
func TestValidateConfigCommand_SilentOnSuccess(t *testing.T) {
	tempDir := t.TempDir()

	validConfig := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.0.0
    hooks:
    -   id: trailing-whitespace
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(validConfig), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &ValidateConfigCommand{}
	_ = cmd.Run([]string{configPath})

	w.Close()
	os.Stdout = oldStdout

	var buf [1024]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	// Should be silent on success (matching Python)
	if output != "" {
		t.Errorf("Expected no output on success, got: %q", output)
	}
}

// TestValidateConfigCommand_InvalidSchema tests a file with valid YAML but invalid schema
func TestValidateConfigCommand_InvalidSchema(t *testing.T) {
	tempDir := t.TempDir()

	// Valid YAML but not a valid pre-commit config (repo without required fields)
	invalidSchema := `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    hooks: []
`
	configPath := filepath.Join(tempDir, "invalid-schema.yaml")
	if err := os.WriteFile(configPath, []byte(invalidSchema), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cmd := &ValidateConfigCommand{}
	result := cmd.Run([]string{configPath})

	// This should fail because repo has no rev and no hooks
	if result != 1 {
		t.Errorf("Expected return code 1 for invalid schema, got %d", result)
	}
}

// TestValidateConfigCommand_Help tests help output
func TestValidateConfigCommand_Help(t *testing.T) {
	cmd := &ValidateConfigCommand{}
	help := cmd.Help()

	// Verify help contains expected content
	expectedStrings := []string{
		"validate-config",
		"filenames",
		".pre-commit-config.yaml",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output missing expected string: %q", expected)
		}
	}
}

// TestValidateConfigCommand_Synopsis tests synopsis
func TestValidateConfigCommand_Synopsis(t *testing.T) {
	cmd := &ValidateConfigCommand{}
	synopsis := cmd.Synopsis()

	if !strings.Contains(synopsis, "Validate") {
		t.Errorf("Synopsis should mention 'Validate', got: %q", synopsis)
	}
}
