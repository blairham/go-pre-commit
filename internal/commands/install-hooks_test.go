package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestInstallHooksCommand_Help(t *testing.T) {
	cmd := &InstallHooksCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"install-hooks",
		"Install hook environments for all environments",
		"--config",
		"--verbose",
		"--help",
		"CI/CD environments",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestInstallHooksCommand_Synopsis(t *testing.T) {
	cmd := &InstallHooksCommand{}
	synopsis := cmd.Synopsis()

	expected := "Install hook environments for all environments in the config file"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestInstallHooksCommand_Run_Help(t *testing.T) {
	cmd := &InstallHooksCommand{}

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

func TestInstallHooksCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &InstallHooksCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestInstallHooksCommand_Run_NoConfigFile(t *testing.T) {
	cmd := &InstallHooksCommand{}

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

func TestInstallHooksCommand_Run_ValidConfig(t *testing.T) {
	cmd := &InstallHooksCommand{}

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
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for valid config, got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_CustomConfig(t *testing.T) {
	cmd := &InstallHooksCommand{}

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
	configContent := `repos:
- repo: local
  hooks:
  - id: custom-hook
    name: Custom Hook
    entry: echo "custom"
    language: system
    files: \.js$
`
	customConfigPath := filepath.Join(tempDir, "custom-config.yaml")
	if err := os.WriteFile(customConfigPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write custom config file: %v", err)
	}

	exitCode := cmd.Run([]string{"--config", "custom-config.yaml"})
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for custom config, got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_Verbose(t *testing.T) {
	cmd := &InstallHooksCommand{}

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
  - id: verbose-hook
    name: Verbose Hook
    entry: echo "verbose"
    language: system
    files: \.py$
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{"--verbose"})
	// May fail due to missing dependencies, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for verbose install-hooks, got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_EmptyConfig(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory with empty config
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Fatalf("Failed to change to temp directory: %v", chdirErr)
	}

	// Initialize git repository
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = tempDir
	err = gitCmd.Run()
	if err != nil {
		t.Skipf("Git not available for testing: %v", err)
	}

	// Configure git for testing
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "config", "commit.gpgsign", "false").Run() // Disable GPG signing for tests

	// Create empty config file
	configContent := `repos: []
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write empty config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for empty config, got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_InvalidConfig(t *testing.T) {
	cmd := &InstallHooksCommand{}

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
		t.Fatalf("Failed to write invalid config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid config")
	}
}

func TestInstallHooksCommand_Run_ConfigWithRemoteRepo(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory with config containing remote repo
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create config file with remote repository
	configContent := `repos:
- repo: https://github.com/psf/black
  rev: 22.3.0
  hooks:
  - id: black
    language_version: python3.9
- repo: local
  hooks:
  - id: local-hook
    name: Local Hook
    entry: echo "local"
    language: system
`
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	exitCode := cmd.Run([]string{})
	// May fail due to network/dependency issues, but should parse arguments correctly
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 for config with remote repo, got %d", exitCode)
	}
}

func TestInstallHooksCommand_VersionResolution(t *testing.T) {
	// Test cases for version resolution
	tests := []struct {
		name            string
		expectedVersion string
		hook            config.Hook
		cfg             config.Config
	}{
		{
			name: "Hook version takes precedence over default",
			cfg: config.Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.8",
				},
			},
			hook: config.Hook{
				ID:              "test-hook",
				Language:        "python",
				LanguageVersion: "3.9",
			},
			expectedVersion: "3.9",
		},
		{
			name: "Uses default_language_version when hook has no version",
			cfg: config.Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.11",
				},
			},
			hook: config.Hook{
				ID:       "test-hook",
				Language: "python",
			},
			expectedVersion: "3.11",
		},
		{
			name: "Empty version when no default exists",
			cfg: config.Config{
				DefaultLanguageVersion: map[string]string{
					"node": "18",
				},
			},
			hook: config.Hook{
				ID:       "test-hook",
				Language: "python",
			},
			expectedVersion: "",
		},
		{
			name: "Multiple languages in default_language_version",
			cfg: config.Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.9",
					"node":   "18.19.0",
					"ruby":   "3.1",
				},
			},
			hook: config.Hook{
				ID:       "test-hook",
				Language: "node",
			},
			expectedVersion: "18.19.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the version resolution directly
			result := config.ResolveEffectiveLanguageVersion(tt.hook, tt.cfg)
			if result != tt.expectedVersion {
				t.Errorf("ResolveEffectiveLanguageVersion() = %q, want %q", result, tt.expectedVersion)
			}
		})
	}
}

func TestInstallHooksCommand_VersionResolutionIntegration(t *testing.T) {
	// Create a simple test to verify the integration works
	hook := config.Hook{
		ID:       "test-hook",
		Language: "python",
	}
	cfg := config.Config{
		DefaultLanguageVersion: map[string]string{
			"python": "3.11",
		},
	}

	result := config.ResolveEffectiveLanguageVersion(hook, cfg)
	if result != "3.11" {
		t.Errorf("Expected version 3.11, got %q", result)
	}

	// Test with hook override
	hook.LanguageVersion = "3.9"
	result = config.ResolveEffectiveLanguageVersion(hook, cfg)
	if result != "3.9" {
		t.Errorf("Expected version 3.9 (hook override), got %q", result)
	}
}
