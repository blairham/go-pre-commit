package commands

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestGcCommand_Help(t *testing.T) {
	cmd := &GcCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"gc",
		"Clean unused cached repositories",
		"--verbose",
		"--help",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help output should contain '%s', but got: %s", expected, help)
		}
	}
}

func TestGcCommand_Synopsis(t *testing.T) {
	cmd := &GcCommand{}
	synopsis := cmd.Synopsis()

	expected := "Clean unused cached data"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestGcCommand_Run_Help(t *testing.T) {
	cmd := &GcCommand{}

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

func TestGcCommand_Run_InvalidFlag(t *testing.T) {
	cmd := &GcCommand{}

	exitCode := cmd.Run([]string{"--invalid-flag"})
	if exitCode == 0 {
		t.Error("Expected non-zero exit code for invalid flag")
	}
}

func TestGcCommand_Run_Default(t *testing.T) {
	cmd := &GcCommand{}

	// Create a temporary cache directory for testing
	tempDir := t.TempDir()

	// Mock home directory for testing
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Test with no cache directory (should succeed with 0 repos removed)
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for default gc, got %d", exitCode)
	}
}

func TestGcCommand_Run_NoDatabase(t *testing.T) {
	cmd := &GcCommand{}

	// Create a temporary cache directory with no database
	tempDir := t.TempDir()

	// Mock home directory for testing
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create cache directory but no database
	cacheDir := filepath.Join(tempDir, ".cache", "pre-commit")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	// Test with no database (should succeed with 0 repos removed)
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with no database, got %d", exitCode)
	}
}

func TestGcCommand_Run_Verbose(t *testing.T) {
	cmd := &GcCommand{}

	// Create a temporary cache directory for testing
	tempDir := t.TempDir()

	// Mock home directory for testing
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Test --verbose flag
	exitCode := cmd.Run([]string{"--verbose"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for --verbose, got %d", exitCode)
	}
}

func TestGcCommand_Run_WithDatabase(t *testing.T) {
	cmd := &GcCommand{}

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Mock home directory for testing
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create cache structure and database
	cacheDir := filepath.Join(tempDir, ".cache", "pre-commit")
	dbPath := filepath.Join(cacheDir, "db.db")

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	// Create a simple database with repos and configs tables
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create tables
	_, err = db.Exec(`
		CREATE TABLE repos (
			repo TEXT,
			ref TEXT,
			path TEXT,
			PRIMARY KEY (repo, ref)
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create repos table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE configs (
			path TEXT NOT NULL,
			PRIMARY KEY (path)
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create configs table: %v", err)
	}

	// Create some test repositories
	repo1Dir := filepath.Join(cacheDir, "repo1234abcd")
	repo2Dir := filepath.Join(cacheDir, "repo5678efgh")

	if mkdirErr := os.MkdirAll(repo1Dir, 0o755); mkdirErr != nil {
		t.Fatalf("Failed to create repo1 dir: %v", mkdirErr)
	}
	if mkdirErr := os.MkdirAll(repo2Dir, 0o755); mkdirErr != nil {
		t.Fatalf("Failed to create repo2 dir: %v", mkdirErr)
	}

	// Insert test data
	_, err = db.Exec(
		"INSERT INTO repos VALUES (?, ?, ?)",
		"https://github.com/test/repo1",
		"v1.0",
		repo1Dir,
	)
	if err != nil {
		t.Fatalf("Failed to insert repo1: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO repos VALUES (?, ?, ?)",
		"https://github.com/test/repo2",
		"v2.0",
		repo2Dir,
	)
	if err != nil {
		t.Fatalf("Failed to insert repo2: %v", err)
	}

	// Create a config file that references repo1 but not repo2
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	configContent := `repos:
  - repo: https://github.com/test/repo1
    rev: v1.0
    hooks:
      - id: test-hook
`
	if writeErr := os.WriteFile(configPath, []byte(configContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to create config file: %v", writeErr)
	}

	// Mark config as used
	_, err = db.Exec("INSERT INTO configs VALUES (?)", configPath)
	if err != nil {
		t.Fatalf("Failed to insert config: %v", err)
	}

	db.Close() // Close before running gc

	// Run gc - should remove repo2 but keep repo1
	exitCode := cmd.Run([]string{"--verbose"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for gc, got %d", exitCode)
	}

	// Verify repo1 still exists (it's referenced by config)
	if _, err := os.Stat(repo1Dir); os.IsNotExist(err) {
		t.Error("Expected repo1 to still exist (it's referenced by config)")
	}

	// Verify repo2 was removed (it's not referenced)
	if _, err := os.Stat(repo2Dir); !os.IsNotExist(err) {
		t.Error("Expected repo2 to be removed (it's not referenced by any config)")
	}
}
