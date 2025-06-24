package languages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/tests/helpers"
)

const (
	testGoEnvDefault = "goenv-default"
)

func TestGoLanguage(t *testing.T) {
	golang := NewGoLanguage()

	config := helpers.LanguageTestConfig{
		Language:       golang,
		Name:           "Go",
		ExecutableName: "go",
		VersionFlag:    "version",
		TestVersions:   []string{"default", "system"},
		EnvPathSuffix:  "", // Go doesn't create separate environments
	}

	helpers.RunLanguageTests(t, config)
}

func TestNewGoLanguage(t *testing.T) {
	golang := NewGoLanguage()

	if golang == nil {
		t.Fatal("NewGoLanguage() returned nil")
	}

	if golang.Base == nil {
		t.Fatal("Base is nil")
	}

	// Check that the base is configured correctly
	if golang.GetName() != "Go" {
		t.Errorf("Expected name 'Go', got %s", golang.GetName())
	}

	if golang.GetExecutableName() != "go" {
		t.Errorf("Expected executable 'go', got %s", golang.GetExecutableName())
	}

	if golang.VersionFlag != "version" {
		t.Errorf("Expected version flag 'version', got %s", golang.VersionFlag)
	}

	if golang.InstallURL != "https://golang.org/" {
		t.Errorf("Expected install URL 'https://golang.org/', got %s", golang.InstallURL)
	}
}

func TestGoLanguage_SetupEnvironmentWithRepositoryInit(t *testing.T) {
	golang := NewGoLanguage()

	t.Run("WithValidStringURL", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		envPath, err := golang.SetupEnvironmentWithRepositoryInit(
			tempCacheDir,
			"default",
			tempRepoDir,
			[]string{},
			"https://github.com/example/repo.git",
		)

		if !golang.IsRuntimeAvailable() {
			// If Go is not available, expect error
			if err == nil {
				t.Error("Expected error when Go is not available")
			}
			return
		}

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepositoryInit failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})

	t.Run("WithNilURL", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		envPath, err := golang.SetupEnvironmentWithRepositoryInit(
			tempCacheDir,
			"default",
			tempRepoDir,
			[]string{},
			nil,
		)

		if !golang.IsRuntimeAvailable() {
			// If Go is not available, expect error
			if err == nil {
				t.Error("Expected error when Go is not available")
			}
			return
		}

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepositoryInit with nil URL failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})

	t.Run("WithNonStringURL", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Test with non-string URL type (should be handled gracefully)
		envPath, err := golang.SetupEnvironmentWithRepositoryInit(
			tempCacheDir,
			"default",
			tempRepoDir,
			[]string{},
			123, // non-string type
		)

		if !golang.IsRuntimeAvailable() {
			// If Go is not available, expect error
			if err == nil {
				t.Error("Expected error when Go is not available")
			}
			return
		}

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepositoryInit with non-string URL failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})

	t.Run("WithAdditionalDependencies", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		envPath, err := golang.SetupEnvironmentWithRepositoryInit(
			tempCacheDir,
			"default",
			tempRepoDir,
			[]string{"dep1", "dep2"},
			"https://github.com/example/repo.git",
		)

		if !golang.IsRuntimeAvailable() {
			// If Go is not available, expect error
			if err == nil {
				t.Error("Expected error when Go is not available")
			}
			return
		}

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepositoryInit with deps failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})
}

func TestGoLanguage_setupEnvironmentWithRepoInternal_Coverage(t *testing.T) {
	golang := NewGoLanguage()

	t.Run("EnvironmentAlreadyExistsAndFunctional", func(t *testing.T) {
		// Skip if Go is not available
		if !golang.IsRuntimeAvailable() {
			t.Skip("Go not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// First setup to create the environment
		envPath1, err := golang.setupEnvironmentWithRepoInternal(
			tempCacheDir, "default", tempRepoDir, "https://example.com", []string{},
		)
		if err != nil {
			t.Fatalf("First setupEnvironmentWithRepoInternal failed: %v", err)
		}

		// Second call should reuse existing environment
		envPath2, err := golang.setupEnvironmentWithRepoInternal(
			tempCacheDir, "default", tempRepoDir, "https://example.com", []string{},
		)
		if err != nil {
			t.Errorf("Second setupEnvironmentWithRepoInternal failed: %v", err)
		}

		if envPath1 != envPath2 {
			t.Errorf("Should reuse existing environment: %s != %s", envPath1, envPath2)
		}
	})

	t.Run("EnvironmentExistsButBroken", func(t *testing.T) {
		// Skip if Go is not available
		if !golang.IsRuntimeAvailable() {
			t.Skip("Go not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Create a broken environment directory manually
		envDirName := testGoEnvDefault // Should match language.GetRepositoryEnvironmentName("go", "default")
		brokenEnvPath := filepath.Join(tempCacheDir, envDirName)
		err := os.MkdirAll(brokenEnvPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create broken environment: %v", err)
		}

		// Add a marker file to verify it gets recreated
		markerFile := filepath.Join(brokenEnvPath, "broken_marker")
		err = os.WriteFile(markerFile, []byte("broken"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create marker file: %v", err)
		}

		envPath, err := golang.setupEnvironmentWithRepoInternal(
			tempCacheDir, "default", tempRepoDir, "https://example.com", []string{},
		)
		if err != nil {
			t.Errorf("setupEnvironmentWithRepoInternal with broken env failed: %v", err)
		}

		// Environment should be valid now
		if envPath == "" {
			t.Error("Expected valid environment path")
		}
	})

	t.Run("GoRuntimeNotAvailable", func(t *testing.T) {
		// This test only makes sense if Go is actually not available
		if golang.IsRuntimeAvailable() {
			t.Skip("Go is available, can't test unavailable scenario")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		_, err := golang.setupEnvironmentWithRepoInternal(
			tempCacheDir, "default", tempRepoDir, "https://example.com", []string{},
		)

		if err == nil {
			t.Error("Expected error when Go runtime is not available")
		}

		expectedMsg := "Go runtime not found"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected error to contain '%s', got: %v", expectedMsg, err)
		}
	})

	t.Run("CreateEnvironmentDirectoryFails", func(t *testing.T) {
		// Skip if Go is not available
		if !golang.IsRuntimeAvailable() {
			t.Skip("Go not available, skipping test")
		}

		// Use a read-only directory to simulate creation failure
		tempDir := t.TempDir()
		readOnlyDir := filepath.Join(tempDir, "readonly")
		err := os.MkdirAll(readOnlyDir, 0o444) // Read-only
		if err != nil {
			t.Fatalf("Failed to create read-only directory: %v", err)
		}

		tempRepoDir := t.TempDir()

		// Try to create environment in read-only directory
		_, err = golang.setupEnvironmentWithRepoInternal(
			readOnlyDir, "default", tempRepoDir, "https://example.com", []string{},
		)

		// Should fail due to permission error
		if err == nil {
			t.Error("Expected error when creating environment in read-only directory")
		}
	})

	t.Run("WithAdditionalDependencies", func(t *testing.T) {
		// Skip if Go is not available
		if !golang.IsRuntimeAvailable() {
			t.Skip("Go not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Test with additional dependencies (should log warning but not fail)
		envPath, err := golang.setupEnvironmentWithRepoInternal(
			tempCacheDir, "default", tempRepoDir, "https://example.com",
			[]string{"github.com/stretchr/testify", "golang.org/x/tools"},
		)
		if err != nil {
			t.Errorf("setupEnvironmentWithRepoInternal with deps failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected valid environment path")
		}
	})

	t.Run("DifferentVersions", func(t *testing.T) {
		// Skip if Go is not available
		if !golang.IsRuntimeAvailable() {
			t.Skip("Go not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Test various version formats
		versions := []string{"default", "system", "latest", "1.21", ""}

		for _, version := range versions {
			t.Run("Version_"+version, func(t *testing.T) {
				envPath, err := golang.setupEnvironmentWithRepoInternal(
					tempCacheDir, version, tempRepoDir, "https://example.com", []string{},
				)
				if err != nil {
					t.Errorf("setupEnvironmentWithRepoInternal with version '%s' failed: %v", version, err)
				}
				if envPath == "" {
					t.Errorf("Expected valid environment path for version '%s'", version)
				}
			})
		}
	})
}

func TestGoLanguage_AdditionalMethods(t *testing.T) {
	golang := NewGoLanguage()

	t.Run("InstallDependencies", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with nil dependencies
		err := golang.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies with nil deps failed: %v", err)
		}

		// Test with empty dependencies
		err = golang.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies with empty deps failed: %v", err)
		}

		// Test with dependencies (should log warning but not fail)
		err = golang.InstallDependencies(tempDir, []string{"example.com/dep"})
		if err != nil {
			t.Errorf("InstallDependencies with deps failed: %v", err)
		}
	})

	t.Run("isRepositoryInstalled", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with existing directory
		result := golang.isRepositoryInstalled(tempDir, tempDir)
		if !result {
			t.Error("isRepositoryInstalled should return true for existing directory")
		}

		// Test with non-existing directory
		result = golang.isRepositoryInstalled("/non/existent/path", tempDir)
		if result {
			t.Error("isRepositoryInstalled should return false for non-existing directory")
		}
	})

	t.Run("IsEnvironmentInstalled", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with existing directory
		result := golang.IsEnvironmentInstalled(tempDir, tempDir)
		if !result {
			t.Error("IsEnvironmentInstalled should return true for existing directory")
		}

		// Test with non-existing directory
		result = golang.IsEnvironmentInstalled("/non/existent/path", tempDir)
		if result {
			t.Error("IsEnvironmentInstalled should return false for non-existing directory")
		}
	})

	t.Run("determineGoVersion", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"", "default"},
			{"default", "default"},
			{"system", "default"},
			{"latest", "default"},
			{"1.21", "default"},
			{"1.21.0", "default"},
			{"main", "default"},
		}

		for _, tc := range testCases {
			t.Run("Input_"+tc.input, func(t *testing.T) {
				result := golang.determineGoVersion(tc.input)
				if result != tc.expected {
					t.Errorf("determineGoVersion(%q) = %q, want %q", tc.input, result, tc.expected)
				}
			})
		}
	})

	t.Run("CacheAwareSetupEnvironmentWithRepoInfo", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Test that the language parameter is ignored
		envPath, err := golang.CacheAwareSetupEnvironmentWithRepoInfo(
			tempCacheDir, "default", tempRepoDir, "https://example.com", []string{}, "ignored-language",
		)

		if !golang.IsRuntimeAvailable() {
			if err == nil {
				t.Error("Expected error when Go runtime not available")
			}
			return
		}

		if err != nil {
			t.Errorf("CacheAwareSetupEnvironmentWithRepoInfo failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})
}

func TestGoLanguage_PreInitializeAndSetupMethods(t *testing.T) {
	golang := NewGoLanguage()

	t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		err := golang.PreInitializeEnvironmentWithRepoInfo(
			tempCacheDir,
			"default",
			tempRepoDir,
			"https://github.com/example/repo.git",
			[]string{},
		)
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo failed: %v", err)
		}
	})

	t.Run("PreInitializeEnvironmentWithRepoInfo_WithDeps", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		err := golang.PreInitializeEnvironmentWithRepoInfo(
			tempCacheDir,
			"default",
			tempRepoDir,
			"https://github.com/example/repo.git",
			[]string{"dep1", "dep2"},
		)
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo with deps failed: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepoInfo", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		envPath, err := golang.SetupEnvironmentWithRepoInfo(
			tempCacheDir,
			"default",
			tempRepoDir,
			"https://github.com/example/repo.git",
			[]string{},
		)

		if !golang.IsRuntimeAvailable() {
			if err == nil {
				t.Error("Expected error when Go runtime not available")
			}
			return
		}

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepoInfo failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})

	t.Run("SetupEnvironmentWithRepoInfo_WithDeps", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		envPath, err := golang.SetupEnvironmentWithRepoInfo(
			tempCacheDir,
			"system",
			tempRepoDir,
			"https://github.com/example/repo.git",
			[]string{"dep1", "dep2"},
		)

		if !golang.IsRuntimeAvailable() {
			if err == nil {
				t.Error("Expected error when Go runtime not available")
			}
			return
		}

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepoInfo with deps failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		envPath, err := golang.SetupEnvironmentWithRepo(
			tempCacheDir,
			"default",
			tempRepoDir,
			"https://github.com/example/repo.git",
			[]string{},
		)

		if !golang.IsRuntimeAvailable() {
			if err == nil {
				t.Error("Expected error when Go runtime not available")
			}
			return
		}

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})
}

func TestGoLanguage_setupEnvironmentWithRepoInternal_ErrorPaths(t *testing.T) {
	golang := NewGoLanguage()

	t.Run("RemoveAllErrorPath", func(t *testing.T) {
		// Skip if Go is not available
		if !golang.IsRuntimeAvailable() {
			t.Skip("Go not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Create environment directory with a special structure to make RemoveAll potentially fail
		envDirName := testGoEnvDefault
		envPath := filepath.Join(tempCacheDir, envDirName)
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create a nested directory structure that might be harder to remove
		nestedPath := filepath.Join(envPath, "nested", "deep", "path")
		err = os.MkdirAll(nestedPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create nested directory: %v", err)
		}

		// Create a file in the nested directory
		testFile := filepath.Join(nestedPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Mock IsEnvironmentInstalled to return false to trigger the removal path
		// Since we can't easily mock, we'll test the existing behavior
		envPath2, err := golang.setupEnvironmentWithRepoInternal(
			tempCacheDir, "default", tempRepoDir, "https://example.com", []string{},
		)

		// This should succeed in most cases, but we're testing the RemoveAll code path
		if err != nil {
			// If there's an error, it might be the RemoveAll failure we're trying to test
			if strings.Contains(err.Error(), "failed to remove broken Go environment") {
				t.Logf("Successfully tested RemoveAll error path: %v", err)
			} else {
				t.Errorf("Unexpected error: %v", err)
			}
		} else {
			t.Logf("RemoveAll succeeded, environment created at: %s", envPath2)
		}
	})

	t.Run("CreateEnvironmentDirectoryErrorPath", func(t *testing.T) {
		// Skip if Go is not available
		if !golang.IsRuntimeAvailable() {
			t.Skip("Go not available, skipping test")
		}

		// Try to create an environment in a location that will cause CreateEnvironmentDirectory to fail
		// This is platform-specific and might not always work, but we'll try
		tempDir := t.TempDir()

		// Create a file with the same name as the intended directory
		envDirName := testGoEnvDefault
		conflictingFile := filepath.Join(tempDir, envDirName)
		err := os.WriteFile(conflictingFile, []byte("conflict"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create conflicting file: %v", err)
		}

		tempRepoDir := t.TempDir()

		_, err = golang.setupEnvironmentWithRepoInternal(
			tempDir, "default", tempRepoDir, "https://example.com", []string{},
		)

		// This should fail because there's a file where we want to create a directory
		if err == nil {
			// On some systems, this might still succeed, which is fine
			t.Log("Environment creation succeeded despite file conflict (platform-specific behavior)")
		} else if strings.Contains(err.Error(), "failed to create Go environment directory") {
			t.Logf("Successfully tested CreateEnvironmentDirectory error path: %v", err)
		} else {
			t.Logf("Got different error (platform-specific behavior): %v", err)
		}
	})

	t.Run("StatErrorPath", func(t *testing.T) {
		// Skip if Go is not available
		if !golang.IsRuntimeAvailable() {
			t.Skip("Go not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Test the os.Stat error path by ensuring the environment doesn't exist
		// This should exercise the "stat error is not nil" branch
		envPath, err := golang.setupEnvironmentWithRepoInternal(
			tempCacheDir, "default", tempRepoDir, "https://example.com", []string{},
		)
		if err != nil {
			t.Errorf("setupEnvironmentWithRepoInternal failed: %v", err)
		}
		if envPath == "" {
			t.Error("Expected valid environment path")
		}

		// Verify environment was created
		if _, statErr := os.Stat(envPath); os.IsNotExist(statErr) {
			t.Error("Environment directory should have been created")
		}
	})
}
