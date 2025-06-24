package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateManifestCommand_Help(t *testing.T) {
	cmd := &ValidateManifestCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"validate-manifest",
		"Validate .pre-commit-hooks.yaml files",
		"--verbose",
		"--help",
		"FILENAMES",
		"manifest files to validate",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestValidateManifestCommand_Synopsis(t *testing.T) {
	cmd := &ValidateManifestCommand{}
	synopsis := cmd.Synopsis()

	expected := "Validate .pre-commit-hooks.yaml files"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestValidateManifestCommand_Run_Help(t *testing.T) {
	cmd := &ValidateManifestCommand{}

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

func TestValidateManifestCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestValidateManifestCommand_Run_NoManifestFile(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory without manifest file
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
		t.Error("Expected non-zero exit code when manifest file doesn't exist")
	}
}

func TestValidateManifestCommand_Run_ValidManifest(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory with valid manifest
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create valid manifest file
	manifestContent := `- id: test-hook
  name: Test Hook
  entry: echo "test"
  language: system
  files: \.py$
- id: another-hook
  name: Another Hook
  entry: echo "another"
  language: python
  files: \.txt$
  args: [--fix]
`
	manifestPath := filepath.Join(tempDir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("Failed to write manifest file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for valid manifest, got %d", exitCode)
	}
}

func TestValidateManifestCommand_Run_InvalidManifest(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory with invalid manifest
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create invalid manifest file (malformed YAML)
	manifestContent := `- id: test-hook
  name: Test Hook
  entry: echo "test"
  language: system
  files: \.py$
  invalid_yaml: [unclosed bracket
`
	manifestPath := filepath.Join(tempDir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("Failed to write manifest file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid manifest")
	}
}

func TestValidateManifestCommand_Run_SpecificFile(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory with custom manifest file
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create custom manifest file
	manifestContent := `- id: custom-hook
  name: Custom Hook
  entry: echo "custom"
  language: system
  files: \.js$
`
	customManifestPath := filepath.Join(tempDir, "custom-hooks.yaml")
	if err := os.WriteFile(customManifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("Failed to write custom manifest file: %v", err)
	}

	exitCode := cmd.Run([]string{"custom-hooks.yaml"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for valid custom manifest, got %d", exitCode)
	}
}

func TestValidateManifestCommand_Run_MultipleFiles(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory with multiple manifest files
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create first manifest file
	manifest1Content := `- id: hook1
  name: Hook 1
  entry: echo "hook1"
  language: system
  files: \.py$
`
	manifest1Path := filepath.Join(tempDir, "hooks1.yaml")
	if err := os.WriteFile(manifest1Path, []byte(manifest1Content), 0o644); err != nil {
		t.Fatalf("Failed to write first manifest file: %v", err)
	}

	// Create second manifest file
	manifest2Content := `- id: hook2
  name: Hook 2
  entry: echo "hook2"
  language: system
  files: \.js$
`
	manifest2Path := filepath.Join(tempDir, "hooks2.yaml")
	if err := os.WriteFile(manifest2Path, []byte(manifest2Content), 0o644); err != nil {
		t.Fatalf("Failed to write second manifest file: %v", err)
	}

	exitCode := cmd.Run([]string{"hooks1.yaml", "hooks2.yaml"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for multiple valid manifests, got %d", exitCode)
	}
}

func TestValidateManifestCommand_Run_Verbose(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory with valid manifest
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create valid manifest file
	manifestContent := `- id: verbose-hook
  name: Verbose Hook
  entry: echo "verbose"
  language: system
  files: \.py$
  types: [python]
  args: [--check]
`
	manifestPath := filepath.Join(tempDir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("Failed to write manifest file: %v", err)
	}

	exitCode := cmd.Run([]string{"--verbose"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for verbose validation, got %d", exitCode)
	}
}

func TestValidateManifestCommand_Run_EmptyManifest(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory with empty manifest
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create empty manifest file
	manifestPath := filepath.Join(tempDir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte("[]"), 0o644); err != nil {
		t.Fatalf("Failed to write empty manifest file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for empty manifest, got %d", exitCode)
	}
}

func TestValidateManifestCommand_Run_ManifestMissingRequiredFields(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory with manifest missing required fields
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create manifest with missing required fields (no id)
	manifestContent := `- name: Hook without ID
  entry: echo "test"
  language: system
`
	manifestPath := filepath.Join(tempDir, ".pre-commit-hooks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("Failed to write manifest file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for manifest missing required fields")
	}
}

func TestValidateManifestCommand_Run_NonexistentFile(t *testing.T) {
	cmd := &ValidateManifestCommand{}

	// Create a temporary directory
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	exitCode := cmd.Run([]string{"nonexistent.yaml"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for nonexistent file")
	}
}
