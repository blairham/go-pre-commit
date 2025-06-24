package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanCommand_Help(t *testing.T) {
	cmd := &CleanCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"Clean cached repositories",
		"--verbose",
		"--help",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestCleanCommand_Synopsis(t *testing.T) {
	cmd := &CleanCommand{}
	synopsis := cmd.Synopsis()

	expected := "Clean cached repositories and environments"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestCleanCommand_Run_Help(t *testing.T) {
	cmd := &CleanCommand{}

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

func TestCleanCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &CleanCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestCleanCommand_Run_Default(t *testing.T) {
	cmd := &CleanCommand{}

	// Create a temporary cache directory for testing
	tempDir := t.TempDir()

	// Mock environment variable for testing
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create cache structure
	cacheDir := filepath.Join(tempDir, ".cache", "pre-commit")
	legacyDir := filepath.Join(tempDir, ".pre-commit")

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("Failed to create legacy dir: %v", err)
	}

	// Create some test files
	cacheFile := filepath.Join(cacheDir, "test-file")
	legacyFile := filepath.Join(legacyDir, "test-file")

	if err := os.WriteFile(cacheFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create cache test file: %v", err)
	}
	if err := os.WriteFile(legacyFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create legacy test file: %v", err)
	}

	// Test default behavior (should clean entire cache and legacy directories)
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for default clean, got %d", exitCode)
	}

	// Verify directories were cleaned
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Error("Expected cache directory to be removed")
	}
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Error("Expected legacy directory to be removed")
	}
}

func TestCleanCommand_Run_Verbose(t *testing.T) {
	cmd := &CleanCommand{}

	// Create a temporary cache directory for testing
	tempDir := t.TempDir()

	// Mock environment variable for testing
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create cache structure
	cacheDir := filepath.Join(tempDir, ".cache", "pre-commit")

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	// Create some test files
	testFile := filepath.Join(cacheDir, "test-file")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test --verbose flag
	exitCode := cmd.Run([]string{"--verbose"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for --verbose, got %d", exitCode)
	}
}

func TestCleanCommand_Run_NoCacheDirectory(t *testing.T) {
	cmd := &CleanCommand{}

	// Create a temporary directory with no cache
	tempDir := t.TempDir()

	// Mock environment variable for testing
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Test with no cache directory (should succeed)
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for no cache directory, got %d", exitCode)
	}
}
