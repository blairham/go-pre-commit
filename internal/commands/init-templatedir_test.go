package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitTemplatedirCommand_Help(t *testing.T) {
	cmd := &InitTemplatedirCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"init-templatedir",
		"Install hook script in a directory intended for use",
		"DIRECTORY",
		"--config",
		"--hook-type",
		"--allow-missing-config",
		"--verbose",
		"--help",
		"git config init.templateDir",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestInitTemplatedirCommand_Synopsis(t *testing.T) {
	cmd := &InitTemplatedirCommand{}
	synopsis := cmd.Synopsis()

	expected := "Install hook script in a directory intended for use with git init templateDir"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestInitTemplatedirCommand_Run_Help(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

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

func TestInitTemplatedirCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestInitTemplatedirCommand_Run_NoDirectory(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Test without providing directory argument
	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code when no directory provided")
	}
}

func TestInitTemplatedirCommand_Run_ValidDirectory(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Create a temporary directory for the template
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create working directory with config
	workDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	// Create valid config file in work directory
	configContent := ValidRepoConfigWithFiles
	configPath := filepath.Join(workDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{templateDir})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for valid directory, got %d", exitCode)
	}

	// Check that template directory was created
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Error("Expected template directory to be created")
	}

	// Check that hooks directory was created
	hooksDir := filepath.Join(templateDir, "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		t.Error("Expected hooks directory to be created in template")
	}
}

func TestInitTemplatedirCommand_Run_CustomConfig(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Create a temporary directory for the template
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "custom-template")

	// Create working directory with custom config
	workDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	// Create custom config file
	configContent := `repos:
- repo: local
  hooks:
  - id: custom-hook
    name: Custom Hook
    entry: echo "custom"
    language: system
    files: \.js$
`
	customConfigPath := filepath.Join(workDir, "custom-config.yaml")
	if err := os.WriteFile(customConfigPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write custom config file: %v", err)
	}

	exitCode := cmd.Run([]string{templateDir, "--config", "custom-config.yaml"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for custom config, got %d", exitCode)
	}

	// Check that template directory was created
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Error("Expected template directory to be created")
	}
}

func TestInitTemplatedirCommand_Run_HookTypes(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Create a temporary directory for the template
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "hook-types-template")

	// Create working directory with config
	workDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	// Create valid config file
	configContent := ValidRepoConfigWithFiles
	configPath := filepath.Join(workDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run(
		[]string{templateDir, "--hook-type", "pre-push", "--hook-type", "pre-commit"},
	)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for multiple hook types, got %d", exitCode)
	}

	// Check that template directory was created
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Error("Expected template directory to be created")
	}
}

func TestInitTemplatedirCommand_Run_AllowMissingConfig(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Create a temporary directory for the template
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "missing-config-template")

	// Create working directory without config
	workDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	exitCode := cmd.Run([]string{templateDir, "--allow-missing-config"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for allow missing config, got %d", exitCode)
	}

	// Check that template directory was created
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Error("Expected template directory to be created")
	}
}

func TestInitTemplatedirCommand_Run_Verbose(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Create a temporary directory for the template
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "verbose-template")

	// Create working directory with config
	workDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	// Create valid config file
	configContent := `repos:
- repo: local
  hooks:
  - id: verbose-hook
    name: Verbose Hook
    entry: echo "verbose"
    language: system
    files: \.py$
`
	configPath := filepath.Join(workDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{templateDir, "--verbose"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for verbose, got %d", exitCode)
	}

	// Check that template directory was created
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Error("Expected template directory to be created")
	}
}

func TestInitTemplatedirCommand_Run_ExistingDirectory(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Create a temporary directory for the template
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "existing-template")

	// Create the directory first
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("Failed to create existing directory: %v", err)
	}

	// Create working directory with config
	workDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	// Create valid config file
	configContent := `repos:
- repo: local
  hooks:
  - id: existing-hook
    name: Existing Hook
    entry: echo "existing"
    language: system
    files: \.py$
`
	configPath := filepath.Join(workDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{templateDir})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for existing directory, got %d", exitCode)
	}

	// Check that hooks directory was created
	hooksDir := filepath.Join(templateDir, "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		t.Error("Expected hooks directory to be created in existing template")
	}
}

func TestInitTemplatedirCommand_Run_NoConfigNoAllowMissing(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Create a temporary directory for the template
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "no-config-template")

	// Create working directory without config and without allow-missing-config
	workDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(workDir); err != nil {
		t.Fatalf("Failed to change to work directory: %v", err)
	}

	exitCode := cmd.Run([]string{templateDir})
	if exitCode == 0 {
		t.Error(
			"Expected non-zero exit code when config is missing and allow-missing-config is not set",
		)
	}
}
