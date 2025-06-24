package languages

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
)

func TestSystemLanguage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "system-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("NewSystemLanguage", func(t *testing.T) {
		sys := NewSystemLanguage()
		if sys == nil {
			t.Fatal("NewSystemLanguage returned nil")
		}
		if sys.Name != language.VersionSystem {
			t.Errorf("Expected name '%s', got '%s'", language.VersionSystem, sys.Name)
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		sys := NewSystemLanguage()

		// Create a test repository directory
		repoPath := filepath.Join(tempDir, "test_repo")
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("Failed to create test repo: %v", err)
		}

		envPath, err := sys.SetupEnvironmentWithRepo(tempDir, "default", repoPath, "", nil)
		if err != nil {
			t.Fatalf("Setup failed for system: %v", err)
		}

		t.Logf("✓ Successfully set up system at %s", envPath)

		// Verify environment path exists
		if _, statErr := os.Stat(envPath); statErr != nil {
			t.Errorf("Environment path %s does not exist", envPath)
		}
	})

	t.Run("IsRuntimeAvailable", func(t *testing.T) {
		sys := NewSystemLanguage()
		if !sys.IsRuntimeAvailable() {
			t.Error("System language should always be available")
		}
	})

	t.Run("CheckHealth", func(t *testing.T) {
		sys := NewSystemLanguage()
		envPath := filepath.Join(tempDir, "test-env")

		// Create environment directory
		os.MkdirAll(envPath, 0o755)

		// Health check should pass
		err := sys.CheckHealth(envPath, "default")
		if err != nil {
			t.Errorf("Health check failed: %v", err)
		}
	})

	// Additional comprehensive tests for better coverage
	t.Run("ComprehensiveCoverage", func(t *testing.T) {
		sys := NewSystemLanguage()

		t.Run("NewSystemLanguage_Properties", func(t *testing.T) {
			if sys.GenericLanguage == nil {
				t.Fatal("GenericLanguage should not be nil")
			}
			if sys.Base == nil {
				t.Fatal("Base should not be nil")
			}

			// Test inherited methods from SimpleLanguage work
			if sys.GetName() != "system" {
				t.Errorf("Expected GetName() to return 'system', got '%s'", sys.GetName())
			}
			if sys.GetExecutableName() != "" {
				t.Errorf("Expected GetExecutableName() to return empty string, got '%s'", sys.GetExecutableName())
			}
		})

		t.Run("SetupEnvironmentWithRepo_Comprehensive", func(t *testing.T) {
			tempDir := t.TempDir()
			repoPath := filepath.Join(tempDir, "repo")
			if err := os.MkdirAll(repoPath, 0o755); err != nil {
				t.Fatalf("Failed to create repo directory: %v", err)
			}

			// Test various version formats
			versions := []string{"1.0", "latest", "system", "default", ""}
			for _, version := range versions {
				envPath, err := sys.SetupEnvironmentWithRepo(
					tempDir,
					version,
					repoPath,
					"https://example.com",
					[]string{},
				)
				if err != nil {
					t.Errorf("SetupEnvironmentWithRepo() with version '%s' returned error: %v", version, err)
				}
				if envPath == "" {
					t.Errorf("SetupEnvironmentWithRepo() with version '%s' returned empty path", version)
				}
			}
			// Test with dependencies
			envPath, err := sys.SetupEnvironmentWithRepo(tempDir, "1.0", repoPath,
				"https://example.com", []string{"dep1", "dep2"})
			if err != nil {
				t.Errorf("SetupEnvironmentWithRepo() with dependencies returned error: %v", err)
			}
			if envPath == "" {
				t.Error("SetupEnvironmentWithRepo() with dependencies returned empty path")
			}
		})

		t.Run("InstallDependencies_Comprehensive", func(t *testing.T) {
			tempDir := t.TempDir()

			// Test with nil dependencies
			err := sys.InstallDependencies(tempDir, nil)
			if err != nil {
				t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
			}

			// Test with empty dependencies
			err = sys.InstallDependencies(tempDir, []string{})
			if err != nil {
				t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
			}

			// Test with single dependency
			err = sys.InstallDependencies(tempDir, []string{"dep1"})
			if err != nil {
				t.Errorf("InstallDependencies() with single dep returned error: %v", err)
			}

			// Test with multiple dependencies
			err = sys.InstallDependencies(tempDir, []string{"dep1", "dep2", "dep3"})
			if err != nil {
				t.Errorf("InstallDependencies() with multiple deps returned error: %v", err)
			}

			// Test with non-existent directory
			err = sys.InstallDependencies("/non/existent/path", []string{"dep1"})
			if err != nil {
				t.Errorf("InstallDependencies() with non-existent path returned error: %v", err)
			}
		})

		t.Run("CheckHealth_Comprehensive", func(t *testing.T) {
			tempDir := t.TempDir()

			// Test with valid directory and version
			err := sys.CheckHealth(tempDir, "1.0")
			if err != nil {
				t.Errorf("CheckHealth() with valid path returned error: %v", err)
			}

			// Test with empty version
			err = sys.CheckHealth(tempDir, "")
			if err != nil {
				t.Errorf("CheckHealth() with empty version returned error: %v", err)
			}

			// Test with non-existent directory
			err = sys.CheckHealth("/non/existent/path", "1.0")
			if err == nil {
				t.Error("CheckHealth() with non-existent path should return error")
			}

			// Test with empty path
			err = sys.CheckHealth("", "1.0")
			if err == nil {
				t.Error("CheckHealth() with empty path should return error")
			}
		})

		t.Run("CheckEnvironmentHealth", func(t *testing.T) {
			tempDir := t.TempDir()

			// Create an environment directory
			envPath := filepath.Join(tempDir, "env")
			if err := os.MkdirAll(envPath, 0o755); err != nil {
				t.Fatalf("Failed to create env directory: %v", err)
			}

			// Test with valid environment path - should return false because there's no executable
			// System language doesn't have a specific executable, so CheckEnvironmentHealth
			// will look for an empty executable name in bin directory and fail
			healthy := sys.CheckEnvironmentHealth(envPath)
			if healthy {
				t.Error("CheckEnvironmentHealth() should return false for system language (no executable)")
			}

			// Test with non-existent path
			healthy = sys.CheckEnvironmentHealth("/non/existent/path")
			if healthy {
				t.Error("CheckEnvironmentHealth() should return false for non-existent path")
			}

			// Test with empty path
			healthy = sys.CheckEnvironmentHealth("")
			if healthy {
				t.Error("CheckEnvironmentHealth() should return false for empty path")
			}
		})

		t.Run("IsRuntimeAvailable_Override", func(t *testing.T) {
			// System language overrides IsRuntimeAvailable to always return true
			available := sys.IsRuntimeAvailable()
			if !available {
				t.Error("IsRuntimeAvailable() should return true for system language (override)")
			}
		})

		t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
			tempDir := t.TempDir()

			// Test PreInitializeEnvironmentWithRepoInfo (should be no-op)
			err := sys.PreInitializeEnvironmentWithRepoInfo(
				tempDir,
				"1.0",
				tempDir,
				"https://example.com/repo",
				[]string{"dep1"},
			)
			if err != nil {
				t.Errorf("PreInitializeEnvironmentWithRepoInfo() returned error: %v", err)
			}
		})

		t.Run("SetupEnvironmentWithRepoInfo", func(t *testing.T) {
			tempDir := t.TempDir()

			// Test SetupEnvironmentWithRepoInfo
			envPath, err := sys.SetupEnvironmentWithRepoInfo(
				tempDir,
				"1.0",
				tempDir,
				"https://example.com/repo",
				[]string{"dep1"},
			)
			if err != nil {
				t.Errorf("SetupEnvironmentWithRepoInfo() returned error: %v", err)
			}
			if envPath == "" {
				t.Error("SetupEnvironmentWithRepoInfo() returned empty path")
			}
		})
	})
}
