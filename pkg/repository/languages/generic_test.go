package languages

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSimpleLanguage(t *testing.T) {
	t.Run("NewSimpleLanguage", func(t *testing.T) {
		simple := NewGenericLanguage("test", "test-exe", "--version", "http://test.com")
		if simple == nil {
			t.Error("NewSimpleLanguage() returned nil")
			return
		}
		if simple.Base == nil {
			t.Error("NewSimpleLanguage() returned instance with nil Base")
		}

		// Check properties are set correctly
		if simple.Name != "test" {
			t.Errorf("Expected name 'test', got '%s'", simple.Name)
		}
		if simple.ExecutableName != "test-exe" {
			t.Errorf("Expected executable name 'test-exe', got '%s'", simple.ExecutableName)
		}
		if simple.VersionFlag != "--version" {
			t.Errorf("Expected version flag '--version', got '%s'", simple.VersionFlag)
		}
		if simple.InstallURL != "http://test.com" {
			t.Errorf("Expected install URL 'http://test.com', got '%s'", simple.InstallURL)
		}
	})

	t.Run("NewSimpleLanguage_EmptyValues", func(t *testing.T) {
		simple := NewGenericLanguage("", "", "", "")
		if simple == nil {
			t.Error("NewSimpleLanguage() with empty values returned nil")
			return
		}
		if simple.Base == nil {
			t.Error("NewSimpleLanguage() with empty values returned instance with nil Base")
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		simple := NewGenericLanguage("test", "test-exe", "--version", "")
		tempDir := t.TempDir()

		// Should not error when setting up environment
		envPath, err := simple.SetupEnvironmentWithRepo(tempDir, "1.0", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() returned empty environment path")
		}

		// Test with additional dependencies
		envPath, err = simple.SetupEnvironmentWithRepo(tempDir, "2.0", tempDir, "dummy-url", []string{"dep1", "dep2"})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with deps returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() with deps returned empty environment path")
		}
	})

	t.Run("InstallDependencies", func(t *testing.T) {
		simple := NewGenericLanguage("test", "test-exe", "--version", "")

		// Should not error when installing dependencies (no-op)
		err := simple.InstallDependencies("/dummy/path", []string{"dep1", "dep2"})
		if err != nil {
			t.Errorf("InstallDependencies() returned error: %v", err)
		}

		// Should handle empty dependencies
		err = simple.InstallDependencies("/dummy/path", []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		// Should handle nil dependencies
		err = simple.InstallDependencies("/dummy/path", nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}
	})

	t.Run("CheckHealth_WithExecutable", func(t *testing.T) {
		simple := NewGenericLanguage("test", "test-exe", "--version", "")
		tempDir := t.TempDir()

		// Should delegate to base health check when executable name is set
		err := simple.CheckHealth(tempDir, "1.0")
		// This may error since test-exe doesn't exist, but it should call the base method
		// The important thing is that it doesn't panic
		_ = err // We don't check the error since test-exe doesn't exist
	})

	t.Run("CheckHealth_NoExecutable", func(t *testing.T) {
		simple := NewGenericLanguage("test", "", "--version", "")
		tempDir := t.TempDir()

		// Should use SimpleCheckHealth when executable name is empty
		err := simple.CheckHealth(tempDir, "1.0")
		if err != nil {
			t.Errorf("CheckHealth() with no executable returned error: %v", err)
		}
	})

	t.Run("CheckHealth_EmptyPaths", func(_ *testing.T) {
		// Test with executable
		simple := NewGenericLanguage("test", "test-exe", "--version", "")
		err := simple.CheckHealth("", "")
		_ = err // May error, but shouldn't panic

		// Test without executable
		simple = NewGenericLanguage("test", "", "--version", "")
		err = simple.CheckHealth("", "")
		_ = err // May error, but shouldn't panic
	})

	t.Run("CheckHealth_WithExecutable_AllPaths", func(t *testing.T) {
		simple := NewGenericLanguage("test", "test-exe", "--version", "")
		tempDir := t.TempDir()

		// Create a mock executable in the environment path to test the base health check more thoroughly
		binDir := filepath.Join(tempDir, "bin")
		err := os.MkdirAll(binDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		exePath := filepath.Join(binDir, "test-exe")
		err = os.WriteFile(exePath, []byte("#!/bin/sh\necho '1.0'\n"), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock executable: %v", err)
		}

		// Test with mock executable - should use base health check
		err = simple.CheckHealth(tempDir, "1.0")
		// Don't check for specific error since the base health check implementation varies
		_ = err // Important that it doesn't panic and executes the base health check path
	})

	t.Run("CheckHealth_NoExecutable_ComprehensiveTest", func(t *testing.T) {
		simple := NewGenericLanguage("test", "", "--version", "")

		// Test with non-existent directory
		err := simple.CheckHealth("/non/existent/path", "1.0")
		if err == nil {
			t.Error("CheckHealth() with non-existent path should return error")
		}

		// Test with existing directory
		tempDir := t.TempDir()
		err = simple.CheckHealth(tempDir, "1.0")
		if err != nil {
			t.Errorf("CheckHealth() with valid path returned error: %v", err)
		}

		// Test with empty version
		err = simple.CheckHealth(tempDir, "")
		if err != nil {
			t.Errorf("CheckHealth() with empty version returned error: %v", err)
		}
	})
}
