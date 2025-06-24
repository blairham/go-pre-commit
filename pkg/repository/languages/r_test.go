package languages

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/language"
)

func TestRLanguage(t *testing.T) {
	t.Run("NewRLanguage", func(t *testing.T) {
		r := NewRLanguage()

		if r == nil {
			t.Fatal("NewRLanguage() returned nil")
		}

		if r.Name != "r" {
			t.Errorf("Expected language name 'r', got '%s'", r.Name)
		}

		if r.ExecutableName != "R" {
			t.Errorf("Expected executable name 'R', got '%s'", r.ExecutableName)
		}

		if r.VersionFlag != testVersionFlag {
			t.Errorf("Expected version flag '%s', got '%s'", testVersionFlag, r.VersionFlag)
		}

		if r.InstallURL != "https://www.r-project.org/" {
			t.Errorf("Expected install URL 'https://www.r-project.org/', got '%s'", r.InstallURL)
		}
	})
}

func TestRLanguage_InstallDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow R dependency installation tests in short mode")
	}

	r := NewRLanguage()

	t.Run("NoDependencies", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-env-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		err = r.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with no dependencies returned error: %v", err)
		}
	})

	t.Run("WithDependencies", func(t *testing.T) {
		// Skip if R is not available
		if _, err := exec.LookPath("R"); err != nil {
			t.Skip("R not available, skipping dependency installation test")
		}

		tempDir, err := os.MkdirTemp("", "test-r-deps-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test with valid package names (these might fail if CRAN is unreachable, which is OK)
		deps := []string{"base", "utils"}
		err = r.InstallDependencies(tempDir, deps)

		// We don't require this to succeed because it requires network access and R setup
		// Just log the result
		if err != nil {
			t.Logf("InstallDependencies() failed (expected if R/CRAN not properly configured): %v", err)
		} else {
			// Verify library directory was created
			libPath := filepath.Join(tempDir, "library")
			if _, err := os.Stat(libPath); os.IsNotExist(err) {
				t.Error("InstallDependencies() should have created library directory")
			}
		}
	})

	t.Run("WithVersionedDependency", func(t *testing.T) {
		// Skip if R is not available
		if _, err := exec.LookPath("R"); err != nil {
			t.Skip("R not available, skipping versioned dependency test")
		}

		tempDir, err := os.MkdirTemp("", "test-r-versioned-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test with versioned dependency
		deps := []string{"jsonlite==1.8.0"}
		err = r.InstallDependencies(tempDir, deps)
		// Log result without requiring success (network/R setup dependent)
		if err != nil {
			t.Logf(
				"InstallDependencies() with versioned dependency failed (expected if R/remotes not available): %v",
				err,
			)
		}
	})
}

func TestRLanguage_CheckEnvironmentHealth(t *testing.T) {
	r := NewRLanguage()

	t.Run("NonExistentPath", func(t *testing.T) {
		result := r.CheckEnvironmentHealth("/non/existent/path")
		if result {
			t.Error("CheckEnvironmentHealth() should return false for non-existent path")
		}
	})

	t.Run("EmptyDirectory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-health-empty-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip if R is not available to avoid triggering installation
		if _, err := exec.LookPath("R"); err != nil {
			t.Skip("R not available, skipping test that would need R runtime")
		}

		result := r.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth() for empty directory returned: %v", result)
	})

	t.Run("DirectoryWithoutLibrary", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-health-nolib-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip if R is not available to avoid triggering installation
		if _, err := exec.LookPath("R"); err != nil {
			t.Skip("R not available, skipping test that would need R runtime")
		}

		// Create some other files but no library directory
		if err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test"), 0o644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		result := r.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth() without library directory returned: %v", result)
	})

	t.Run("DirectoryWithLibrary", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-health-lib-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip if R is not available to avoid triggering installation
		if _, err := exec.LookPath("R"); err != nil {
			t.Skip("R not available, skipping test that would need R runtime")
		}

		// Create library directory
		libPath := filepath.Join(tempDir, "library")
		if err := os.MkdirAll(libPath, 0o755); err != nil {
			t.Fatalf("Failed to create library directory: %v", err)
		}

		result := r.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth() with library directory returned: %v", result)
	})

	t.Run("DirectoryWithLibraryAndPackages", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-health-packages-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip if R is not available to avoid triggering installation
		if _, err := exec.LookPath("R"); err != nil {
			t.Skip("R not available, skipping test that would need R runtime")
		}

		// Create library directory with some fake package structure
		libPath := filepath.Join(tempDir, "library")
		if err := os.MkdirAll(libPath, 0o755); err != nil {
			t.Fatalf("Failed to create library directory: %v", err)
		}

		// Create a fake package directory
		packagePath := filepath.Join(libPath, "testpackage")
		if err := os.MkdirAll(packagePath, 0o755); err != nil {
			t.Fatalf("Failed to create package directory: %v", err)
		}

		result := r.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth() with library and packages returned: %v", result)
	})
}

func TestRLanguage_SetupEnvironmentWithRepo(t *testing.T) {
	r := NewRLanguage()

	// Helper function to check if R is available without triggering installation
	isRAvailable := func() bool {
		_, err := exec.LookPath("R")
		return err == nil
	}

	t.Run("DefaultVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-setup-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test behavior when R is not available
		if !isRAvailable() {
			_, setupErr := r.SetupEnvironmentWithRepo(
				tempDir,
				language.VersionDefault,
				tempDir,
				"dummy-url",
				[]string{},
			)
			if setupErr == nil {
				t.Error("SetupEnvironmentWithRepo() should fail when R is not available")
			} else {
				expectedMsg := "r runtime not found"
				if !strings.Contains(setupErr.Error(), expectedMsg) {
					t.Errorf("Expected error message to contain '%s', got: %v", expectedMsg, setupErr)
				}
			}
			return
		}

		envPath, err := r.SetupEnvironmentWithRepo(tempDir, language.VersionDefault, tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
			return
		}

		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() returned empty environment path")
		}

		// Verify environment directory was created
		expectedPath := filepath.Join(tempDir, "renv-default")
		if envPath != expectedPath {
			t.Errorf("SetupEnvironmentWithRepo() returned unexpected path: got %s, want %s", envPath, expectedPath)
		}

		// Directory should exist
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			t.Error("SetupEnvironmentWithRepo() did not create environment directory")
		}
	})

	t.Run("SystemVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-system-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		if !isRAvailable() {
			_, setupErr := r.SetupEnvironmentWithRepo(tempDir, language.VersionSystem, tempDir, "dummy-url", []string{})
			if setupErr == nil {
				t.Error("SetupEnvironmentWithRepo() should fail when R is not available")
			}
			return
		}

		envPath, err := r.SetupEnvironmentWithRepo(tempDir, language.VersionSystem, tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with system version returned error: %v", err)
			return
		}

		expectedPath := filepath.Join(tempDir, "renv-system")
		if envPath != expectedPath {
			t.Errorf("SetupEnvironmentWithRepo() returned unexpected path: got %s, want %s", envPath, expectedPath)
		}
	})

	t.Run("UnsupportedVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-unsupported-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		if !isRAvailable() {
			t.Skip("R not available, skipping test that would fail anyway")
		}

		// Unsupported versions should be normalized to default
		envPath, err := r.SetupEnvironmentWithRepo(tempDir, "4.2.0", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() with unsupported version failed: %v", err)
			return
		}

		// Should use default version environment name
		expectedPath := filepath.Join(tempDir, "renv-default")
		if envPath != expectedPath {
			t.Errorf("SetupEnvironmentWithRepo() should normalize unsupported version "+
				"to default: got %s, want %s", envPath, expectedPath)
		}
	})

	t.Run("WithDependencies", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping slow R dependency test in short mode")
		}

		tempDir, err := os.MkdirTemp("", "test-r-with-deps-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		if !isRAvailable() {
			t.Skip("R not available, skipping dependency test")
		}

		deps := []string{"base", "utils"}
		envPath, err := r.SetupEnvironmentWithRepo(tempDir, language.VersionDefault, tempDir, "dummy-url", deps)

		// Log result - dependency installation requires R and network access
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() with dependencies failed (may be expected): %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo() with dependencies succeeded: %s", envPath)
		}
	})
}

func TestRLanguage_CheckHealth(t *testing.T) {
	r := NewRLanguage()

	t.Run("DefaultVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-health-check-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		err = r.CheckHealth(tempDir, language.VersionDefault)

		// This depends on whether R is installed
		if err != nil {
			if strings.Contains(err.Error(), "Rscript executable not found") {
				t.Logf("CheckHealth() failed as expected (R not installed): %v", err)
			} else if strings.Contains(err.Error(), "environment directory does not exist") {
				t.Logf("CheckHealth() failed as expected (environment not setup): %v", err)
			} else {
				t.Logf("CheckHealth() failed with error: %v", err)
			}
		} else {
			t.Logf("CheckHealth() succeeded (R is available)")
		}
	})

	t.Run("UnsupportedVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-health-unsupported-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		err = r.CheckHealth(tempDir, "4.2.0")
		if err == nil {
			t.Error("CheckHealth() should return error for unsupported version")
		}

		expectedMsg := "r only supports version 'default'"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("CheckHealth() error message should contain '%s', got: %v", expectedMsg, err)
		}
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		err := r.CheckHealth("/non/existent/directory", language.VersionDefault)
		if err == nil {
			t.Error("CheckHealth() should return error for non-existent directory")
		}

		expectedMsg := "environment directory does not exist"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("CheckHealth() error message should contain '%s', got: %v", expectedMsg, err)
		}
	})
}

func TestRLanguage_PreInitializeEnvironmentWithRepoInfo(t *testing.T) {
	r := NewRLanguage()

	tempDir, err := os.MkdirTemp("", "test-r-preinit-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = r.PreInitializeEnvironmentWithRepoInfo(tempDir, language.VersionDefault, tempDir, "test-repo", []string{})
	// This should not fail regardless of R availability
	if err != nil {
		t.Errorf("PreInitializeEnvironmentWithRepoInfo() returned error: %v", err)
	}
}

func TestRLanguage_SetupEnvironmentWithRepoInfo(t *testing.T) {
	r := NewRLanguage()

	tempDir, err := os.MkdirTemp("", "test-r-setup-info-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	envPath, err := r.SetupEnvironmentWithRepoInfo(tempDir, language.VersionDefault, tempDir, "test-repo", []string{})

	// This uses the cache-aware setup which may have different behavior
	if err != nil {
		t.Logf("SetupEnvironmentWithRepoInfo() failed: %v", err)
	} else {
		t.Logf("SetupEnvironmentWithRepoInfo() succeeded: %s", envPath)
	}
}

// Additional tests for CheckEnvironmentHealth edge cases
func TestRLanguage_CheckEnvironmentHealth_AdditionalCases(t *testing.T) {
	r := NewRLanguage()

	t.Run("WithLibraryButNoR", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-health-lib-no-r-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create library directory
		libPath := filepath.Join(tempDir, "library")
		if err := os.MkdirAll(libPath, 0o755); err != nil {
			t.Fatalf("Failed to create library directory: %v", err)
		}

		// This might return false if R is not available
		result := r.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth with library but potentially no R: %v", result)
	})

	t.Run("WithLibraryAndValidRPath", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-health-valid-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create library directory with some package structure
		libPath := filepath.Join(tempDir, "library")
		packagePath := filepath.Join(libPath, "base")
		if err := os.MkdirAll(packagePath, 0o755); err != nil {
			t.Fatalf("Failed to create package directory: %v", err)
		}

		// Create a DESCRIPTION file to make it look like a real R package
		descFile := filepath.Join(packagePath, "DESCRIPTION")
		descContent := `Package: base
Version: 4.3.0
Title: The R Base Package
`
		if err := os.WriteFile(descFile, []byte(descContent), 0o644); err != nil {
			t.Fatalf("Failed to create DESCRIPTION file: %v", err)
		}

		result := r.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth with valid library structure: %v", result)
	})
}

// TestRLanguage_100PercentCoverage tests for complete code coverage
func TestRLanguage_100PercentCoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow R comprehensive coverage tests in short mode")
	}

	r := NewRLanguage()

	t.Run("InstallDependencies_ComprehensiveCoverage", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test 1: No dependencies (early return)
		err := r.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies with no deps should succeed, got: %v", err)
		}

		// Test 2: nil dependencies (early return)
		err = r.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies with nil deps should succeed, got: %v", err)
		}

		// Test 3: Library directory creation error
		err = r.InstallDependencies("/dev/null", []string{"test-package"})
		if err == nil {
			t.Error("InstallDependencies should fail when library directory cannot be created")
		}
		if !strings.Contains(err.Error(), "failed to create library directory") {
			t.Errorf("Expected library directory creation error, got: %v", err)
		}

		// Test 4: Simple package name (no version)
		if _, rErr := exec.LookPath("R"); rErr == nil {
			// R is available - test the actual installation logic
			deps := []string{"nonexistent-test-package-12345"}
			err = r.InstallDependencies(tempDir, deps)
			if err == nil {
				t.Log("InstallDependencies unexpectedly succeeded with nonexistent package")
			} else {
				// Expected to fail with nonexistent package
				if !strings.Contains(err.Error(), "failed to install R package") {
					t.Errorf("Expected R package installation error, got: %v", err)
				}
			}
		} else {
			// R not available - test the failure path
			deps := []string{"test-package"}
			err = r.InstallDependencies(tempDir, deps)
			if err == nil {
				t.Error("InstallDependencies should fail when R is not available")
			}
		}

		// Test 5: Versioned package name (package==version)
		if _, rErr := exec.LookPath("R"); rErr == nil {
			deps := []string{"nonexistent-package==1.0.0"}
			err = r.InstallDependencies(tempDir, deps)
			if err == nil {
				t.Log("InstallDependencies unexpectedly succeeded with versioned nonexistent package")
			} else {
				// Expected to fail
				if !strings.Contains(err.Error(), "failed to install R package") {
					t.Errorf("Expected R package installation error, got: %v", err)
				}
			}
		}
	})

	t.Run("CheckEnvironmentHealth_ComprehensiveCoverage", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test 1: Health check fails (no environment directory)
		nonExistentPath := filepath.Join(tempDir, "nonexistent")
		result := r.CheckEnvironmentHealth(nonExistentPath)
		if result {
			t.Error("CheckEnvironmentHealth should return false for nonexistent environment")
		}

		// Test 2: Environment exists, no library directory (basic health check)
		envPath := filepath.Join(tempDir, "env")
		os.MkdirAll(envPath, 0o755)

		// This will depend on whether R is available
		result = r.CheckEnvironmentHealth(envPath)
		// We don't assert the result since it depends on R availability
		t.Logf("CheckEnvironmentHealth for basic env (R availability dependent): %v", result)

		// Test 3: Environment with library directory
		libPath := filepath.Join(envPath, "library")
		os.MkdirAll(libPath, 0o755)

		result = r.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth with library directory: %v", result)

		// Test 4: Library directory exists but R command fails (simulate)
		// This is harder to test without mocking, but the logic is covered by the R availability check
	})

	t.Run("SetupEnvironmentWithRepo_ComprehensiveCoverage", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo")
		os.MkdirAll(repoPath, 0o755)

		// Test 1: Version normalization (unsupported version)
		envPath, err := r.SetupEnvironmentWithRepo("", "unsupported-version", repoPath, "", nil)
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with unsupported version failed: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo with unsupported version succeeded: %s", envPath)
		}

		// Test 2: Default version
		envPath, err = r.SetupEnvironmentWithRepo("", "default", repoPath, "", nil)
		if err != nil {
			// Expected if R is not available
			if strings.Contains(err.Error(), "r runtime not found") {
				t.Logf("SetupEnvironmentWithRepo correctly failed when R not available: %v", err)
			} else {
				t.Errorf("Unexpected error: %v", err)
			}
		} else {
			t.Logf("SetupEnvironmentWithRepo succeeded: %s", envPath)

			// Verify directory was created
			if _, statErr := os.Stat(envPath); os.IsNotExist(statErr) {
				t.Error("Environment directory should have been created")
			}
		}

		// Test 3: System version
		envPath, err = r.SetupEnvironmentWithRepo("", "system", repoPath, "", nil)
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with system version failed: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo with system version succeeded: %s", envPath)
		}

		// Test 4: Environment already exists and is healthy (reuse path)
		if _, rErr := exec.LookPath("R"); rErr == nil {
			// Create a healthy environment first
			existingEnvPath := filepath.Join(repoPath, "renv-default")
			os.MkdirAll(existingEnvPath, 0o755)

			// Try to set up again - should reuse if healthy
			envPath, err = r.SetupEnvironmentWithRepo("", "default", repoPath, "", nil)
			if err != nil {
				t.Logf("SetupEnvironmentWithRepo with existing env failed: %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo reused existing env: %s", envPath)
			}
		}

		// Test 5: Broken environment removal
		brokenEnvPath := filepath.Join(repoPath, "renv-broken")
		os.MkdirAll(brokenEnvPath, 0o755)

		// Create a nested structure that might be harder to remove
		nestedPath := filepath.Join(brokenEnvPath, "nested")
		os.MkdirAll(nestedPath, 0o755)
		testFile := filepath.Join(nestedPath, "test.txt")
		os.WriteFile(testFile, []byte("test"), 0o644)

		// Change permissions to make removal difficult (may not work on all systems)
		os.Chmod(nestedPath, 0o000)
		defer os.Chmod(nestedPath, 0o755) // Cleanup

		if _, rErr := exec.LookPath("R"); rErr == nil {
			envPath, err = r.SetupEnvironmentWithRepo("", "default", repoPath, "", nil)
			if err != nil {
				if strings.Contains(err.Error(), "failed to remove broken environment") {
					t.Logf("SetupEnvironmentWithRepo correctly failed with removal error: %v", err)
				} else {
					t.Logf("SetupEnvironmentWithRepo failed for other reason: %v", err)
				}
			} else {
				t.Logf("SetupEnvironmentWithRepo succeeded despite broken env: %s", envPath)
			}
		}

		// Test 6: Directory creation error
		_, err = r.SetupEnvironmentWithRepo("", "default", "/dev/null", "", nil)
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when directory cannot be created")
		} else {
			if strings.Contains(err.Error(), "failed to create R environment directory") {
				t.Logf("SetupEnvironmentWithRepo correctly failed with directory creation error: %v", err)
			} else if strings.Contains(err.Error(), "r runtime not found") {
				t.Logf("SetupEnvironmentWithRepo failed due to R not available: %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo failed for other reason: %v", err)
			}
		}

		// Test 7: With dependencies (installation error path)
		deps := []string{"nonexistent-package-12345"}
		_, err = r.SetupEnvironmentWithRepo("", "default", repoPath, "", deps)
		if err != nil {
			if strings.Contains(err.Error(), "failed to install R dependencies") {
				t.Logf("SetupEnvironmentWithRepo correctly failed with dependency installation error: %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo failed for other reason: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo unexpectedly succeeded with nonexistent dependencies")
		}
	})

	t.Run("CheckHealth_ComprehensiveCoverage", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "env")
		os.MkdirAll(envPath, 0o755)

		// Test 1: Unsupported version
		err := r.CheckHealth(envPath, "unsupported-version")
		if err == nil {
			t.Error("CheckHealth should fail with unsupported version")
		}
		if !strings.Contains(err.Error(), "r only supports version 'default'") {
			t.Errorf("Expected version error, got: %v", err)
		}

		// Test 2: Non-existent environment directory
		err = r.CheckHealth("/nonexistent/path", "default")
		if err == nil {
			t.Error("CheckHealth should fail for nonexistent environment")
		}
		if !strings.Contains(err.Error(), "environment directory does not exist") {
			t.Errorf("Expected directory error, got: %v", err)
		}

		// Test 3: Environment exists, check Rscript availability
		err = r.CheckHealth(envPath, "default")
		if err != nil {
			// Expected if R is not available
			if strings.Contains(err.Error(), "Rscript executable not found") {
				t.Logf("CheckHealth correctly failed when Rscript not available: %v", err)
			} else if strings.Contains(err.Error(), "R scripting front-end version 4.0.0") {
				t.Logf("CheckHealth correctly failed when R installation not working: %v", err)
			} else {
				t.Errorf("Unexpected error: %v", err)
			}
		} else {
			t.Log("CheckHealth succeeded (R is available)")
		}

		// Test 4: Empty version (should be treated as default)
		err = r.CheckHealth(envPath, "")
		// The function doesn't explicitly handle empty version, it will fail with version check
		if err != nil {
			t.Logf("CheckHealth with empty version failed: %v", err)
		}
	})
}

func TestRLanguage_ErrorPathsCoverage(t *testing.T) {
	r := NewRLanguage()

	t.Run("InstallDependencies_RNotAvailable", func(t *testing.T) {
		tempDir := t.TempDir()
		libPath := filepath.Join(tempDir, "library")
		os.MkdirAll(libPath, 0o755)

		// Test with dependencies when R is not available
		deps := []string{"test-package"}

		// Save original PATH and modify it to exclude R
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", "/nonexistent")

		err := r.InstallDependencies(tempDir, deps)
		if err == nil {
			t.Log("InstallDependencies unexpectedly succeeded without R")
		} else {
			// Should fail because R command not found
			t.Logf("InstallDependencies correctly failed without R: %v", err)
		}
	})

	t.Run("CheckEnvironmentHealth_EdgeCases", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test 1: Environment path that doesn't exist
		result := r.CheckEnvironmentHealth("/completely/nonexistent/path")
		if result {
			t.Error("CheckEnvironmentHealth should return false for nonexistent path")
		}

		// Test 2: Environment with library directory but R command fails
		envPath := filepath.Join(tempDir, "env-with-lib")
		libPath := filepath.Join(envPath, "library")
		os.MkdirAll(libPath, 0o755)

		// Test with R not available
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", "/nonexistent")

		result = r.CheckEnvironmentHealth(envPath)
		if result {
			t.Error("CheckEnvironmentHealth should return false when R is not available")
		}
	})

	t.Run("SetupEnvironmentWithRepo_ErrorPaths", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test 1: Empty version (should be normalized to default)
		_, err := r.SetupEnvironmentWithRepo("", "", tempDir, "", nil)
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with empty version failed: %v", err)
		}

		// Test 2: Version normalization behavior
		_, err = r.SetupEnvironmentWithRepo("", "1.4.0", tempDir, "", nil)
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with custom version failed: %v", err)
		}

		// Test 3: Test the healthy environment reuse path
		// First, let's create what appears to be a healthy environment
		healthyEnvPath := filepath.Join(tempDir, "renv-default")
		os.MkdirAll(healthyEnvPath, 0o755)

		// Since CheckEnvironmentHealth likely returns false without R,
		// we need to test this logic differently

		// Test 4: Test broken environment removal failure
		brokenEnvPath := filepath.Join(tempDir, "broken")
		os.MkdirAll(brokenEnvPath, 0o755)

		// Create a file that might prevent removal
		nestedDir := filepath.Join(brokenEnvPath, "readonly")
		os.MkdirAll(nestedDir, 0o755)
		testFile := filepath.Join(nestedDir, "test.txt")
		os.WriteFile(testFile, []byte("test"), 0o644)

		// Make it read-only to potentially cause removal issues
		os.Chmod(nestedDir, 0o444)
		defer func() {
			os.Chmod(nestedDir, 0o755) // Cleanup
			os.RemoveAll(brokenEnvPath)
		}()

		// Now attempt setup which should try to remove broken environment
		_, err = r.SetupEnvironmentWithRepo("", "default", tempDir, "", nil)
		if err != nil {
			if strings.Contains(err.Error(), "failed to remove broken environment") {
				t.Logf("SetupEnvironmentWithRepo correctly failed with removal error: %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo failed for other reason: %v", err)
			}
		}
	})

	t.Run("CheckHealth_AllBranches", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "test-env")
		os.MkdirAll(envPath, 0o755)

		// Test 1: Supported version (default)
		err := r.CheckHealth(envPath, language.VersionDefault)
		if err != nil {
			// Expected if R not available
			t.Logf("CheckHealth with default version failed (expected): %v", err)
		}

		// Test 2: System version (should also fail with version error since only default is supported)
		err = r.CheckHealth(envPath, language.VersionSystem)
		if err == nil {
			t.Error("CheckHealth should fail with system version")
		}
		if !strings.Contains(err.Error(), "r only supports version 'default'") {
			t.Errorf("Expected version error, got: %v", err)
		}

		// Test 3: Custom version
		err = r.CheckHealth(envPath, "3.6.0")
		if err == nil {
			t.Error("CheckHealth should fail with custom version")
		}
		if !strings.Contains(err.Error(), "r only supports version 'default'") {
			t.Errorf("Expected version error, got: %v", err)
		}
	})
}

// TestRLanguage_MockRAvailable tests code paths when R is available using a mock
func TestRLanguage_MockRAvailable(t *testing.T) {
	r := NewRLanguage()

	t.Run("InstallDependencies_WithMockR", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a mock R script that always succeeds
		mockRDir := filepath.Join(tempDir, "mock-bin")
		os.MkdirAll(mockRDir, 0o755)

		mockRScript := filepath.Join(mockRDir, "R")
		mockRContent := `#!/bin/bash
echo "Mock R script"
exit 0
`
		os.WriteFile(mockRScript, []byte(mockRContent), 0o755)

		// Temporarily modify PATH to include our mock R
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockRDir+":"+originalPath)

		// Test simple package installation
		deps := []string{"test-package"}
		err := r.InstallDependencies(tempDir, deps)
		if err != nil {
			t.Logf("InstallDependencies with mock R failed: %v", err)
		} else {
			t.Log("InstallDependencies with mock R succeeded")

			// Verify library directory was created
			libPath := filepath.Join(tempDir, "library")
			if _, statErr := os.Stat(libPath); os.IsNotExist(statErr) {
				t.Error("Library directory should have been created")
			}
		}

		// Test versioned package installation
		deps = []string{"versioned-package==1.2.3"}
		err = r.InstallDependencies(tempDir, deps)
		if err != nil {
			t.Logf("InstallDependencies with versioned package failed: %v", err)
		} else {
			t.Log("InstallDependencies with versioned package succeeded")
		}
	})

	t.Run("CheckEnvironmentHealth_WithMockR", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create mock R and Rscript
		mockRDir := filepath.Join(tempDir, "mock-bin")
		os.MkdirAll(mockRDir, 0o755)

		// Mock R script
		mockRScript := filepath.Join(mockRDir, "R")
		mockRContent := `#!/bin/bash
echo "Mock R output"
exit 0
`
		os.WriteFile(mockRScript, []byte(mockRContent), 0o755)

		// Mock Rscript
		mockRscriptScript := filepath.Join(mockRDir, "Rscript")
		os.WriteFile(mockRscriptScript, []byte(mockRContent), 0o755)

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockRDir+":"+originalPath)

		// Test environment health with mock R
		envPath := filepath.Join(tempDir, "test-env")
		os.MkdirAll(envPath, 0o755)

		result := r.CheckEnvironmentHealth(envPath)
		if !result {
			t.Log("CheckEnvironmentHealth returned false despite mock R")
		} else {
			t.Log("CheckEnvironmentHealth succeeded with mock R")
		}

		// Test environment with library directory
		libPath := filepath.Join(envPath, "library")
		os.MkdirAll(libPath, 0o755)

		result = r.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth with library and mock R: %v", result)
	})

	t.Run("SetupEnvironmentWithRepo_WithMockR", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo")
		os.MkdirAll(repoPath, 0o755)

		// Create mock R and Rscript
		mockRDir := filepath.Join(tempDir, "mock-bin")
		os.MkdirAll(mockRDir, 0o755)

		mockContent := `#!/bin/bash
echo "Mock R output"
exit 0
`
		mockRScript := filepath.Join(mockRDir, "R")
		os.WriteFile(mockRScript, []byte(mockContent), 0o755)

		mockRscriptScript := filepath.Join(mockRDir, "Rscript")
		os.WriteFile(mockRscriptScript, []byte(mockContent), 0o755)

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockRDir+":"+originalPath)

		// Test basic setup
		envPath, err := r.SetupEnvironmentWithRepo("", "default", repoPath, "", nil)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo with mock R failed: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo with mock R succeeded: %s", envPath)

			// Verify environment was created
			if _, statErr := os.Stat(envPath); os.IsNotExist(statErr) {
				t.Error("Environment directory should have been created")
			}
		}

		// Test setup with dependencies
		deps := []string{"test-dep"}
		envPath, err = r.SetupEnvironmentWithRepo("", "default", repoPath, "", deps)
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with dependencies failed: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo with dependencies succeeded: %s", envPath)
		}

		// Test reuse of existing healthy environment
		// Set up again - should reuse existing
		envPath2, err := r.SetupEnvironmentWithRepo("", "default", repoPath, "", nil)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo reuse failed: %v", err)
		} else {
			if envPath2 != envPath {
				t.Errorf("Expected to reuse environment %s, got %s", envPath, envPath2)
			} else {
				t.Log("Successfully reused existing environment")
			}
		}
	})

	t.Run("CheckHealth_WithMockR", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "test-env")
		os.MkdirAll(envPath, 0o755)

		// Create mock Rscript
		mockRDir := filepath.Join(tempDir, "mock-bin")
		os.MkdirAll(mockRDir, 0o755)

		mockRscriptScript := filepath.Join(mockRDir, "Rscript")
		mockContent := `#!/bin/bash
echo "R version mock"
exit 0
`
		os.WriteFile(mockRscriptScript, []byte(mockContent), 0o755)

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockRDir+":"+originalPath)

		// Test CheckHealth with mock Rscript
		err := r.CheckHealth(envPath, "default")
		if err != nil {
			t.Errorf("CheckHealth with mock Rscript failed: %v", err)
		} else {
			t.Log("CheckHealth with mock Rscript succeeded")
		}
	})

	t.Run("InstallDependencies_MockRFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a mock R script that always fails
		mockRDir := filepath.Join(tempDir, "mock-bin")
		os.MkdirAll(mockRDir, 0o755)

		mockRScript := filepath.Join(mockRDir, "R")
		mockRContent := `#!/bin/bash
echo "Mock R installation failed"
exit 1
`
		os.WriteFile(mockRScript, []byte(mockRContent), 0o755)

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockRDir+":"+originalPath)

		// Test package installation failure
		deps := []string{"failing-package"}
		err := r.InstallDependencies(tempDir, deps)
		if err == nil {
			t.Error("InstallDependencies should fail with failing mock R")
		} else {
			if strings.Contains(err.Error(), "failed to install R package") {
				t.Logf("InstallDependencies correctly failed with mock R error: %v", err)
			} else {
				t.Errorf("Unexpected error format: %v", err)
			}
		}
	})
}

// TestRLanguage_FinalCoverageTests covers the remaining uncovered lines to achieve 100% coverage
func TestRLanguage_FinalCoverageTests(t *testing.T) {
	r := NewRLanguage()

	t.Run("CheckEnvironmentHealth_LibraryPathWithRSuccess", func(t *testing.T) {
		rScript := `#!/bin/bash
# This mock R should succeed for the specific command used in CheckEnvironmentHealth
if [[ "$1" == "--slave" && "$2" == "--no-restore" && "$3" == "-e" ]]; then
    # The R script being executed: .libPaths("/path/to/lib"); .libPaths()
    echo ".libPaths executed successfully"
    exit 0
fi
# For other commands, just succeed
exit 0
`
		rscriptScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
    echo "R scripting front-end version 4.0.0"
    exit 0
fi
exit 0
`
		testRMockEnvironment(t, r, rScript, rscriptScript)
	})

	t.Run("SetupEnvironmentWithRepo_HealthyEnvironmentReuse", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "test-r-setup-healthy-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// First, create a "healthy" environment
		envPath := filepath.Join(tempDir, "renv-default")
		err = os.MkdirAll(envPath, 0o755)
		require.NoError(t, err)

		// Create a mock R and Rscript that make the environment appear healthy
		mockRScript := testSuccessScript
		mockRPath := filepath.Join(tempDir, "R")
		err = os.WriteFile(mockRPath, []byte(mockRScript), 0o755)
		require.NoError(t, err)

		mockRscriptScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
    echo "R scripting front-end version 4.0.0"
    exit 0
fi
exit 0
`
		mockRscriptPath := filepath.Join(tempDir, "Rscript")
		err = os.WriteFile(mockRscriptPath, []byte(mockRscriptScript), 0o755)
		require.NoError(t, err)

		// Temporarily modify PATH to include our mock executables
		originalPath := os.Getenv("PATH")
		os.Setenv("PATH", tempDir+":"+originalPath)
		defer os.Setenv("PATH", originalPath)

		// Test SetupEnvironmentWithRepo - should reuse the healthy environment
		resultPath, err := r.SetupEnvironmentWithRepo("", "default", tempDir, "", nil)
		require.NoError(t, err)
		assert.Equal(t, envPath, resultPath)
		t.Logf("SetupEnvironmentWithRepo correctly reused healthy environment: %s", resultPath)
	})

	t.Run("SetupEnvironmentWithRepo_BrokenEnvironmentRemoval", func(t *testing.T) {
		// Create a temporary directory
		tempDir, err := os.MkdirTemp("", "test-r-setup-broken-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create a "broken" environment (directory exists but CheckEnvironmentHealth fails)
		envPath := filepath.Join(tempDir, "renv-default")
		err = os.MkdirAll(envPath, 0o755)
		require.NoError(t, err)

		// Create some content in the broken environment
		brokenFile := filepath.Join(envPath, "broken.txt")
		err = os.WriteFile(brokenFile, []byte("broken environment"), 0o644)
		require.NoError(t, err)

		// Create mock R that succeeds (available) but Rscript that fails health check initially
		mockRScript := testSuccessScript
		mockRPath := filepath.Join(tempDir, "R")
		err = os.WriteFile(mockRPath, []byte(mockRScript), 0o755)
		require.NoError(t, err)

		// Create a Rscript that fails the first time (broken env) but would succeed after recreation
		mockRscriptScript := `#!/bin/bash
# First call should fail (broken environment), subsequent calls succeed
if [[ "$1" == "--version" ]]; then
    if [ -f "` + filepath.Join(envPath, "broken.txt") + `" ]; then
        exit 1  # Fail when broken file exists
    else
        echo "R scripting front-end version 4.0.0"
        exit 0  # Succeed after broken file is removed
    fi
fi
exit 0
`
		mockRscriptPath := filepath.Join(tempDir, "Rscript")
		err = os.WriteFile(mockRscriptPath, []byte(mockRscriptScript), 0o755)
		require.NoError(t, err)

		// Temporarily modify PATH to include our mock executables
		originalPath := os.Getenv("PATH")
		os.Setenv("PATH", tempDir+":"+originalPath)
		defer os.Setenv("PATH", originalPath)

		// Test SetupEnvironmentWithRepo - should remove broken environment and recreate
		resultPath, err := r.SetupEnvironmentWithRepo("", "default", tempDir, "", nil)
		require.NoError(t, err)
		assert.Equal(t, envPath, resultPath)

		// Verify the broken file was removed (environment was recreated)
		_, statErr := os.Stat(brokenFile)
		assert.True(t, os.IsNotExist(statErr), "Broken file should have been removed during environment recreation")

		t.Logf("SetupEnvironmentWithRepo correctly removed broken environment and recreated: %s", resultPath)
	})
}

// TestRLanguage_FinalPush tests to achieve 100% coverage on the last remaining lines
func TestRLanguage_FinalPush(t *testing.T) {
	r := NewRLanguage()

	t.Run("CheckEnvironmentHealth_SuccessPath", func(t *testing.T) {
		rScript := `#!/bin/bash
exit 0
`
		rscriptScript := `#!/bin/bash
echo "R scripting front-end version 4.0.0"
exit 0
`
		testRMockEnvironment(t, r, rScript, rscriptScript)
	})

	t.Run("SetupEnvironmentWithRepo_AllPaths", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-r-all-paths-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		// Create simple mock executables that work
		mockRScript := `#!/bin/bash
exit 0
`
		mockRPath := filepath.Join(tempDir, "R")
		err = os.WriteFile(mockRPath, []byte(mockRScript), 0o755)
		require.NoError(t, err)

		mockRscriptScript := `#!/bin/bash
echo "R scripting front-end version 4.0.0"
exit 0
`
		mockRscriptPath := filepath.Join(tempDir, "Rscript")
		err = os.WriteFile(mockRscriptPath, []byte(mockRscriptScript), 0o755)
		require.NoError(t, err)

		// Add to PATH
		originalPath := os.Getenv("PATH")
		os.Setenv("PATH", tempDir+":"+originalPath)
		defer os.Setenv("PATH", originalPath)

		// Test various paths through SetupEnvironmentWithRepo
		envPath, err := r.SetupEnvironmentWithRepo("", "default", tempDir, "", []string{"dep1"})
		require.NoError(t, err)
		t.Logf("SetupEnvironmentWithRepo with dependencies: %s", envPath)

		// Test reusing the environment
		envPath2, err := r.SetupEnvironmentWithRepo("", "default", tempDir, "", nil)
		require.NoError(t, err)
		assert.Equal(t, envPath, envPath2)
		t.Log("Successfully reused environment")
	})
}
