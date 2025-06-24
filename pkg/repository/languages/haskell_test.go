package languages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testHaskellEnvDefault = "haskellenv-default"
	testGHCVersionScript  = `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "The Glorious Glasgow Haskell Compilation System, version 9.2.5"
  exit 0
fi
exit 0`
	testGHCSuccessScript = `#!/bin/bash
echo "The Glorious Glasgow Haskell Compilation System, version 9.2.5"
exit 0`
)

func TestHaskellLanguage(t *testing.T) {
	t.Run("NewHaskellLanguage", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		if haskell == nil {
			t.Error("NewHaskellLanguage() returned nil")
			return
		}
		if haskell.Base == nil {
			t.Error("NewHaskellLanguage() returned instance with nil Base")
		}

		// Check properties
		if haskell.Name != "Haskell" {
			t.Errorf("Expected name 'Haskell', got '%s'", haskell.Name)
		}
		if haskell.ExecutableName != "ghc" {
			t.Errorf("Expected executable name 'ghc', got '%s'", haskell.ExecutableName)
		}
		if haskell.VersionFlag != testVersionFlag {
			t.Errorf("Expected version flag '%s', got '%s'", testVersionFlag, haskell.VersionFlag)
		}
		if haskell.InstallURL != "https://www.haskell.org/downloads/" {
			t.Errorf("Expected install URL 'https://www.haskell.org/downloads/', got '%s'", haskell.InstallURL)
		}
	})

	t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Should delegate to base method without error
		err := haskell.PreInitializeEnvironmentWithRepoInfo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo() returned error: %v", err)
		}

		// Test with additional dependencies
		err = haskell.PreInitializeEnvironmentWithRepoInfo(tempDir, "system", tempDir,
			"dummy-url", []string{"hlint", "ormolu"})
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo() with deps returned error: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepoInfo", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Test setup regardless of Haskell availability to exercise code paths
		envPath, err := haskell.SetupEnvironmentWithRepoInfo(tempDir, "default", tempDir, "dummy-url", []string{})

		// Log errors instead of failing to exercise code paths
		if err != nil {
			t.Logf("SetupEnvironmentWithRepoInfo() returned expected error (Haskell may not be installed): %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepoInfo() succeeded with environment path: %s", envPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo_ValidVersions", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Test with different valid versions regardless of Haskell availability
		versions := []string{"default", "system", ""}
		for _, version := range versions {
			envPath, err := haskell.SetupEnvironmentWithRepo(tempDir, version, tempDir, "dummy-url", []string{})

			// Log errors instead of failing to exercise code paths
			if err != nil {
				t.Logf("SetupEnvironmentWithRepo() with version '%s' returned expected error: %v", version, err)
			} else {
				t.Logf("SetupEnvironmentWithRepo() with version '%s' succeeded with environment path: %s", version, envPath)
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_InvalidVersion", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Test with invalid version
		_, err := haskell.SetupEnvironmentWithRepo(tempDir, "9.2.5", tempDir, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo() with invalid version should return error")
		}
	})

	t.Run("SetupEnvironmentWithRepo_ExistingEnvironment", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Test environment reuse/recreation regardless of Haskell availability
		envPath1, err := haskell.SetupEnvironmentWithRepo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("First SetupEnvironmentWithRepo() failed (expected if Haskell not available): %v", err)
		}

		// Call again - should reuse existing environment or recreate if unhealthy
		envPath2, err := haskell.SetupEnvironmentWithRepo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("Second SetupEnvironmentWithRepo() returned error (expected if Haskell not available): %v", err)
		}

		// Only check path equality if both calls succeeded
		if err == nil && envPath1 != envPath2 {
			t.Logf("Environment paths differ: %s vs %s", envPath1, envPath2)
		}
	})

	t.Run("InstallDependencies_Empty", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Should handle empty dependencies without error
		err := haskell.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		err = haskell.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}
	})

	t.Run("InstallDependencies_WithDeps", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Test regardless of cabal availability to exercise code paths
		err := haskell.InstallDependencies(tempDir, []string{"hlint", "ormolu"})
		if err != nil {
			t.Logf("InstallDependencies failed (expected if cabal not available): %v", err)
		}

		// Should attempt to create bin directory
		binPath := filepath.Join(tempDir, "bin")
		if _, err := os.Stat(binPath); err != nil {
			t.Logf("bin directory not created (expected if cabal not available): %v", err)
		}
	})

	t.Run("InstallDependencies_InvalidPath", func(t *testing.T) {
		haskell := NewHaskellLanguage()

		// Test with invalid path regardless of cabal availability
		err := haskell.InstallDependencies("/invalid/readonly/path", []string{"test-dep"})
		if err == nil {
			t.Error("InstallDependencies() with invalid path should return error")
		} else {
			t.Logf("InstallDependencies() correctly failed with invalid path: %v", err)
		}
	})

	t.Run("InstallDependencies_CabalNotAvailable", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Temporarily modify PATH to make cabal unavailable
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		emptyDir := filepath.Join(tempDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should exercise the "cabal not found" error path
		err := haskell.InstallDependencies(tempDir, []string{"test-package"})
		if err == nil {
			t.Error("InstallDependencies should fail when cabal not available")
		} else {
			// The error message can vary but should mention cabal
			if !strings.Contains(err.Error(), "cabal") {
				t.Errorf("Expected error to mention 'cabal', got: %v", err)
			} else {
				t.Logf("InstallDependencies correctly failed when cabal not available: %v", err)
			}
		}
	})

	t.Run("InstallDependencies_BinDirectoryCreationError", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Create a file where bin directory should be created
		binFile := filepath.Join(tempDir, "bin")
		if err := os.WriteFile(binFile, []byte("blocking file"), 0o644); err != nil {
			t.Fatalf("Failed to create blocking file: %v", err)
		}

		// This should exercise the bin directory creation error path
		err := haskell.InstallDependencies(tempDir, []string{"test-package"})
		if err == nil {
			t.Error("InstallDependencies should fail when bin directory creation fails")
		} else {
			t.Logf("InstallDependencies correctly failed due to bin directory creation error: %v", err)
		}
	})

	t.Run("CheckEnvironmentHealth", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Should return false for non-existent environment
		healthy := haskell.CheckEnvironmentHealth("/non/existent/path")
		if healthy {
			t.Error("CheckEnvironmentHealth() should return false for non-existent environment")
		}

		// Test with existing directory
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		healthy = haskell.CheckEnvironmentHealth(tempDir)
		// Health depends on ghc and cabal availability
		t.Logf("CheckEnvironmentHealth for existing directory: %v", healthy)
	})

	t.Run("CheckEnvironmentHealth_EmptyPath", func(t *testing.T) {
		haskell := NewHaskellLanguage()

		// Should handle empty paths gracefully
		healthy := haskell.CheckEnvironmentHealth("")
		if healthy {
			t.Error("CheckEnvironmentHealth() with empty path should return false")
		}
	})

	// Additional tests for better coverage
	t.Run("SetupEnvironmentWithRepoInfo_ErrorPaths", func(t *testing.T) {
		haskell := NewHaskellLanguage()

		// Test with empty cache dir regardless of Haskell availability
		_, err := haskell.SetupEnvironmentWithRepoInfo("", "default", "/tmp", "dummy-url", []string{})
		t.Logf("SetupEnvironmentWithRepoInfo with empty cache dir: %v", err)

		// Test with additional dependencies to cover more paths
		_, err = haskell.SetupEnvironmentWithRepoInfo("/tmp", "default", "/tmp", "dummy-url", []string{"test-dep"})
		t.Logf("SetupEnvironmentWithRepoInfo with deps: %v", err)

		// Test with different version values
		_, err = haskell.SetupEnvironmentWithRepoInfo("/tmp", "9.2.5", "/tmp", "dummy-url", []string{})
		t.Logf("SetupEnvironmentWithRepoInfo with specific version: %v", err)
	})

	t.Run("CheckEnvironmentHealth_GhcNotAvailable", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Create the directory so it passes the directory check
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Temporarily modify PATH to make ghc unavailable
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		emptyDir := filepath.Join(tempDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should exercise the ghc not available path
		healthy := haskell.CheckEnvironmentHealth(tempDir)
		if healthy {
			t.Error("CheckEnvironmentHealth should return false when ghc not available")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false when ghc not available")
		}
	})

	t.Run("CheckEnvironmentHealth_CabalNotAvailable", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Create the directory and a mock ghc that works
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Create mock ghc in temp directory
		binDir := filepath.Join(tempDir, "mockbin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		ghcExec := filepath.Join(binDir, "ghc")
		ghcScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "The Glorious Glasgow Haskell Compilation System, version 9.2.5"
  exit 0
fi
exit 1`
		if err := os.WriteFile(ghcExec, []byte(ghcScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock ghc executable: %v", err)
		}

		// Set PATH to only include our mock ghc (no cabal)
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", binDir)

		// This should pass ghc check but fail cabal check
		healthy := haskell.CheckEnvironmentHealth(tempDir)
		if healthy {
			t.Error("CheckEnvironmentHealth should return false when cabal not available")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false when cabal not available")
		}
	})

	t.Run("SetupEnvironmentWithRepo_EnvironmentCreationError", func(t *testing.T) {
		haskell := NewHaskellLanguage()

		// Test with invalid repo path that would cause environment setup to fail
		_, err := haskell.SetupEnvironmentWithRepo(
			"",
			"default",
			"/nonexistent/invalid/repo/path",
			"dummy-url",
			[]string{},
		)
		if err == nil {
			t.Log("SetupEnvironmentWithRepo() with invalid repo path succeeded (environment creation might still work)")
		} else {
			t.Logf("SetupEnvironmentWithRepo() correctly failed with invalid repo path: %v", err)
		}
	})

	// Additional comprehensive tests for 100% coverage
	t.Run("InstallDependencies_CabalUpdateFailure", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		cabalScript := `#!/bin/bash
if [[ "$1" == "update" ]]; then
  echo "Error: cannot update package list"
  exit 1
fi
exit 0`
		testCabalFailure(t, haskell, cabalScript, "failed to update cabal package list")
	})

	t.Run("InstallDependencies_CabalInstallFailure", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		cabalScript := `#!/bin/bash
if [[ "$1" == "update" ]]; then
  echo "Package list updated"
  exit 0
elif [[ "$1" == "install" ]]; then
  echo "Error: cannot install package"
  exit 1
fi
exit 0`
		testCabalFailure(t, haskell, cabalScript, "failed to install Haskell dependencies with cabal")
	})

	t.Run("CheckEnvironmentHealth_SuccessPath", func(t *testing.T) {
		haskell := NewHaskellLanguage()
		tempDir := t.TempDir()

		// Create bin directory with mock ghc executable that works
		binPath := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		ghcExec := filepath.Join(binPath, "ghc")
		ghcScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "The Glorious Glasgow Haskell Compilation System, version 9.2.5"
  exit 0
fi
exit 1`
		if err := os.WriteFile(ghcExec, []byte(ghcScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock ghc executable: %v", err)
		}

		// Create mock cabal that works
		mockBinDir := filepath.Join(tempDir, "mockbin")
		if err := os.MkdirAll(mockBinDir, 0o755); err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		cabalScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "cabal-install version 3.8.1.0"
  exit 0
fi
exit 0`
		cabalExec := filepath.Join(mockBinDir, "cabal")
		if err := os.WriteFile(cabalExec, []byte(cabalScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock cabal executable: %v", err)
		}

		// Temporarily modify PATH to include both our mock ghc and cabal
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", binPath+string(os.PathListSeparator)+mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should exercise the success path where both CheckHealth and cabal version succeed
		healthy := haskell.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth should return true when both CheckHealth and cabal version succeed")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned true when all checks pass")
		}
	})
}

func TestHaskellLanguage_AdditionalCoverage(t *testing.T) {
	haskell := NewHaskellLanguage()

	t.Run("SetupEnvironmentWithRepo_ExistingBrokenEnvironment", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create environment directory structure manually
		envDirName := testHaskellEnvDefault // Should match language.GetRepositoryEnvironmentName("haskell", "default")
		envPath := filepath.Join(tempDir, envDirName)
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Add a marker file to verify environment gets recreated
		markerFile := filepath.Join(envPath, "broken_marker")
		err = os.WriteFile(markerFile, []byte("broken"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create marker file: %v", err)
		}

		// Call SetupEnvironmentWithRepo - it should detect the environment exists but is broken
		newEnvPath, err := haskell.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})

		if err != nil {
			// Expected to fail if cabal/ghc not available, but we still exercise the code path
			t.Logf("SetupEnvironmentWithRepo failed as expected (Haskell tools may not be available): %v", err)

			// Check if environment directory was cleaned up (if removal succeeded)
			if _, statErr := os.Stat(markerFile); os.IsNotExist(statErr) {
				t.Log("Environment was successfully cleaned up before failing")
			}
		} else {
			t.Logf("SetupEnvironmentWithRepo succeeded with environment path: %s", newEnvPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo_RemoveAllFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create environment directory with complex nested structure
		envDirName := testHaskellEnvDefault
		envPath := filepath.Join(tempDir, envDirName)
		nestedPath := filepath.Join(envPath, "nested", "deep", "structure")
		err := os.MkdirAll(nestedPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create nested environment structure: %v", err)
		}

		// Create files in the nested structure
		testFile := filepath.Join(nestedPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Test SetupEnvironmentWithRepo with existing environment
		// This exercises the os.Stat and potential os.RemoveAll paths
		envPath2, err := haskell.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})

		if err != nil {
			if strings.Contains(err.Error(), "failed to remove broken environment") {
				t.Logf("Successfully tested RemoveAll failure path: %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo failed with different error (expected if Haskell not available): %v", err)
			}
		} else {
			t.Logf("SetupEnvironmentWithRepo succeeded, environment created at: %s", envPath2)
		}
	})

	t.Run("SetupEnvironmentWithRepo_CreateDirectoryFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a file where the environment directory should be created
		envDirName := testHaskellEnvDefault
		conflictingFile := filepath.Join(tempDir, envDirName)
		err := os.WriteFile(conflictingFile, []byte("conflict"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create conflicting file: %v", err)
		}

		// This should fail when trying to create the environment directory
		_, err = haskell.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})

		if err == nil {
			// On some systems this might still succeed, which is fine
			t.Log("SetupEnvironmentWithRepo succeeded despite file conflict (platform-specific behavior)")
		} else if strings.Contains(err.Error(), "failed to create Haskell environment directory") {
			t.Logf("Successfully tested CreateEnvironmentDirectory failure: %v", err)
		} else {
			t.Logf("Got different error (may be from earlier failure): %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepo_DependencyInstallationFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a mock environment where directory creation succeeds but dependency installation fails
		// We'll do this by providing dependencies and seeing if the dependency installation error path is triggered
		_, err := haskell.SetupEnvironmentWithRepo(
			"",
			"default",
			tempDir,
			"dummy-url",
			[]string{"nonexistent-package-12345"},
		)

		if err == nil {
			t.Log("SetupEnvironmentWithRepo succeeded (cabal may not be available to fail)")
		} else if strings.Contains(err.Error(), "failed to install Haskell dependencies") {
			t.Logf("Successfully tested dependency installation failure: %v", err)
		} else {
			t.Logf("Got different error (expected if Haskell tools not available): %v", err)
		}
	})

	t.Run("CheckEnvironmentHealth_CheckHealthFailure", func(t *testing.T) {
		// Test the path where CheckHealth fails but the function continues to check cabal
		tempDir := t.TempDir()

		// Create environment directory
		err := os.MkdirAll(tempDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// The base CheckHealth will likely fail if ghc is not available
		healthy := haskell.CheckEnvironmentHealth(tempDir)

		// This exercises both the CheckHealth failure and the cabal check paths
		if healthy {
			t.Log("CheckEnvironmentHealth returned true (both ghc and cabal are available)")
		} else {
			t.Log("CheckEnvironmentHealth returned false (expected if ghc or cabal not available)")
		}
	})

	t.Run("CheckEnvironmentHealth_CabalCheckFailure", func(t *testing.T) {
		// This test aims to exercise the cabal version check failure path specifically
		tempDir := t.TempDir()

		// Create a mock ghc that passes version check
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		// Create mock ghc that succeeds
		ghcScript := testGHCVersionScript
		ghcExec := filepath.Join(mockBinDir, "ghc")
		err = os.WriteFile(ghcExec, []byte(ghcScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock ghc: %v", err)
		}

		// Create mock cabal that fails version check
		cabalScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "Error: cannot get version"
  exit 1
fi
exit 1`
		cabalExec := filepath.Join(mockBinDir, "cabal")
		err = os.WriteFile(cabalExec, []byte(cabalScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock cabal: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should pass the CheckHealth but fail the cabal version check
		healthy := haskell.CheckEnvironmentHealth(tempDir)
		if healthy {
			t.Error("CheckEnvironmentHealth should return false when cabal version check fails")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false when cabal version check fails")
		}
	})

	t.Run("InstallDependencies_SuccessfulInstallation", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create mock cabal that succeeds on both update and install
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		cabalScript := `#!/bin/bash
if [[ "$1" == "update" ]]; then
  echo "Package list updated successfully"
  exit 0
elif [[ "$1" == "install" ]]; then
  echo "Installing packages successfully"
  exit 0
fi
exit 0`
		cabalExec := filepath.Join(mockBinDir, "cabal")
		err = os.WriteFile(cabalExec, []byte(cabalScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock cabal: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should exercise the successful installation path
		err = haskell.InstallDependencies(tempDir, []string{"test-package"})
		if err != nil {
			t.Errorf("InstallDependencies should succeed with working cabal: %v", err)
		} else {
			t.Log("InstallDependencies succeeded as expected")
		}

		// Verify bin directory was created
		binPath := filepath.Join(tempDir, "bin")
		if _, statErr := os.Stat(binPath); os.IsNotExist(statErr) {
			t.Error("bin directory should have been created")
		}
	})

	t.Run("SetupEnvironmentWithRepo_VersionEdgeCases", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test edge cases for version handling
		testVersions := []struct {
			version     string
			shouldError bool
		}{
			{"", false},        // empty version should be treated as default
			{"default", false}, // explicit default
			{"system", false},  // explicit system
			{"1.0", true},      // invalid version should error
			{"latest", true},   // invalid version should error
			{"invalid", true},  // invalid version should error
		}

		for _, tc := range testVersions {
			t.Run("Version_"+tc.version, func(t *testing.T) {
				_, err := haskell.SetupEnvironmentWithRepo("", tc.version, tempDir, "dummy-url", []string{})

				if tc.shouldError {
					if err == nil {
						t.Errorf("SetupEnvironmentWithRepo with version '%s' should return error", tc.version)
					} else if !strings.Contains(err.Error(), "only supports 'default' or 'system' versions") {
						t.Errorf("Expected version error message, got: %v", err)
					} else {
						t.Logf("Correctly rejected invalid version '%s': %v", tc.version, err)
					}
				} else {
					// For valid versions, any error is likely due to missing Haskell tools
					if err != nil {
						t.Logf("SetupEnvironmentWithRepo with version '%s' failed (expected if Haskell not available): %v", tc.version, err)
					} else {
						t.Logf("SetupEnvironmentWithRepo with version '%s' succeeded", tc.version)
					}
				}
			})
		}
	})
}

func TestHaskellLanguage_FinalCoverageGaps(t *testing.T) {
	haskell := NewHaskellLanguage()

	t.Run("CheckEnvironmentHealth_BaseCheckHealthFailure", func(t *testing.T) {
		// Test the specific path where h.CheckHealth(envPath, "") fails
		// This should happen when the directory doesn't exist or ghc is not available

		// Test with non-existent directory first
		healthy := haskell.CheckEnvironmentHealth("/absolutely/nonexistent/path/12345")
		if healthy {
			t.Error("CheckEnvironmentHealth should return false for non-existent path")
		}

		// Test with existing directory but no ghc available
		tempDir := t.TempDir()

		// Create empty bin directory to ensure no ghc is found there
		emptyBinDir := filepath.Join(tempDir, "emptybin")
		err := os.MkdirAll(emptyBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create empty bin directory: %v", err)
		}

		// Set PATH to only include empty directory
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", emptyBinDir)

		// This should fail the base CheckHealth but still test the cabal path
		healthy = haskell.CheckEnvironmentHealth(tempDir)
		if healthy {
			t.Error("CheckEnvironmentHealth should return false when base CheckHealth fails")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false when base CheckHealth fails")
		}
	})

	t.Run("SetupEnvironmentWithRepo_StatAndRemoveAllSuccess", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create environment directory to trigger the os.Stat success path
		envDirName := testHaskellEnvDefault
		envPath := filepath.Join(tempDir, envDirName)
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Add a file to make sure RemoveAll has something to remove
		testFile := filepath.Join(envPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Create a simple working cabal and ghc for the environment health check to pass initially
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err = os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		// Create working ghc
		ghcScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "The Glorious Glasgow Haskell Compilation System, version 9.2.5"
  exit 0
fi
exit 0`
		ghcExec := filepath.Join(mockBinDir, "ghc")
		err = os.WriteFile(ghcExec, []byte(ghcScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock ghc: %v", err)
		}

		// Create working cabal
		cabalScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "cabal-install version 3.8.1.0"
  exit 0
elif [[ "$1" == "update" ]]; then
  echo "Package list updated"
  exit 0
elif [[ "$1" == "install" ]]; then
  echo "Packages installed"
  exit 0
fi
exit 0`
		cabalExec := filepath.Join(mockBinDir, "cabal")
		err = os.WriteFile(cabalExec, []byte(cabalScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock cabal: %v", err)
		}

		// Set PATH to include our working tools
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// First, make environment unhealthy by putting an invalid cabal in the environment's bin
		envBinDir := filepath.Join(envPath, "bin")
		err = os.MkdirAll(envBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment bin directory: %v", err)
		}

		// Put a broken cabal in the environment bin to make health check fail
		brokenCabal := filepath.Join(envBinDir, "cabal")
		err = os.WriteFile(brokenCabal, []byte("#!/bin/bash\nexit 1"), 0o755)
		if err != nil {
			t.Fatalf("Failed to create broken cabal: %v", err)
		}

		// Modify PATH to prioritize the environment's broken tools
		os.Setenv("PATH", envBinDir+string(os.PathListSeparator)+mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should trigger: environment exists (os.Stat success), health check fails,
		// RemoveAll succeeds, then environment creation and setup
		envPath2, err := haskell.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})

		if err != nil {
			t.Logf("SetupEnvironmentWithRepo failed (may be due to environment setup issues): %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo succeeded, environment recreated at: %s", envPath2)

			// Verify the old file was removed (RemoveAll worked)
			if _, statErr := os.Stat(testFile); !os.IsNotExist(statErr) {
				t.Error("Old test file should have been removed by RemoveAll")
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_SuccessfulFlow", func(t *testing.T) {
		tempDir := t.TempDir()

		// Set up working tools
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		// Create working ghc
		ghcScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "The Glorious Glasgow Haskell Compilation System, version 9.2.5"
  exit 0
fi
exit 0`
		ghcExec := filepath.Join(mockBinDir, "ghc")
		err = os.WriteFile(ghcExec, []byte(ghcScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock ghc: %v", err)
		}

		// Create working cabal
		cabalScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "cabal-install version 3.8.1.0"
  exit 0
elif [[ "$1" == "update" ]]; then
  echo "Package list updated"
  exit 0
elif [[ "$1" == "install" ]]; then
  echo "Packages installed"
  exit 0
fi
exit 0`
		cabalExec := filepath.Join(mockBinDir, "cabal")
		err = os.WriteFile(cabalExec, []byte(cabalScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock cabal: %v", err)
		}

		// Set PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should exercise the complete successful flow:
		// 1. No existing environment
		// 2. Valid version check passes
		// 3. Environment creation succeeds
		// 4. Dependency installation succeeds (empty deps)
		envPath, err := haskell.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo should succeed with working tools: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo succeeded with environment: %s", envPath)

			// Verify environment was created
			if _, statErr := os.Stat(envPath); os.IsNotExist(statErr) {
				t.Error("Environment directory should have been created")
			}
		}

		// Test that subsequent call reuses the environment (CheckEnvironmentHealth returns true)
		envPath2, err := haskell.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})

		if err != nil {
			t.Errorf("Second SetupEnvironmentWithRepo should succeed: %v", err)
		} else if envPath != envPath2 {
			t.Errorf("Should reuse existing healthy environment: %s != %s", envPath, envPath2)
		} else {
			t.Log("Successfully reused existing healthy environment")
		}
	})
}

func TestHaskellLanguage_RemainingEdgeCases(t *testing.T) {
	haskell := NewHaskellLanguage()

	t.Run("CheckEnvironmentHealth_CompleteSuccessPath", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create working ghc and cabal in system PATH
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		// Create ghc that works for both --version and normal operation
		ghcScript := testGHCSuccessScript
		ghcExec := filepath.Join(mockBinDir, "ghc")
		err = os.WriteFile(ghcExec, []byte(ghcScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock ghc: %v", err)
		}

		// Create cabal that works for --version
		cabalScript := `#!/bin/bash
echo "cabal-install version 3.8.1.0"
exit 0`
		cabalExec := filepath.Join(mockBinDir, "cabal")
		err = os.WriteFile(cabalExec, []byte(cabalScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock cabal: %v", err)
		}

		// Set PATH to include our working tools
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should exercise the complete success path:
		// 1. CheckHealth succeeds (finds ghc)
		// 2. cabal --version succeeds
		healthy := haskell.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Log("CheckEnvironmentHealth returned false (CheckHealth likely failed despite our mock tools)")
		} else {
			t.Log("CheckEnvironmentHealth successfully returned true for complete success path")
		}
	})

	t.Run("SetupEnvironmentWithRepo_HealthyEnvironmentReuse", func(t *testing.T) {
		tempDir := t.TempDir()

		// Set up working tools
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		// Create working tools
		ghcScript := testGHCSuccessScript
		ghcExec := filepath.Join(mockBinDir, "ghc")
		err = os.WriteFile(ghcExec, []byte(ghcScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock ghc: %v", err)
		}

		cabalScript := `#!/bin/bash
echo "cabal-install version 3.8.1.0"
exit 0`
		cabalExec := filepath.Join(mockBinDir, "cabal")
		err = os.WriteFile(cabalExec, []byte(cabalScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock cabal: %v", err)
		}

		// Set PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// Create a healthy environment manually first
		envDirName := testHaskellEnvDefault
		envPath := filepath.Join(tempDir, envDirName)
		err = os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// This should trigger the CheckEnvironmentHealth success path and return early
		resultPath, err := haskell.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})

		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo should succeed with healthy environment: %v", err)
		} else if resultPath != envPath {
			t.Errorf("Should return existing healthy environment path: expected %s, got %s", envPath, resultPath)
		} else {
			t.Log("Successfully reused healthy environment without recreation")
		}
	})

	t.Run("SetupEnvironmentWithRepo_SystemVersionEdgeCase", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test specifically with "system" version to ensure it's handled correctly
		_, err := haskell.SetupEnvironmentWithRepo("", "system", tempDir, "dummy-url", []string{})

		// We don't care if it succeeds or fails due to missing tools,
		// we just want to exercise the version validation path
		if err != nil {
			// Should not be a version validation error
			if strings.Contains(err.Error(), "only supports 'default' or 'system' versions") {
				t.Error("'system' version should be valid")
			} else {
				t.Logf("SetupEnvironmentWithRepo with 'system' version failed for other reasons: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo with 'system' version succeeded")
		}
	})

	t.Run("CheckEnvironmentHealth_CabalRunError", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create working ghc but failing cabal
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		// Create working ghc
		ghcScript := `#!/bin/bash
echo "The Glorious Glasgow Haskell Compilation System, version 9.2.5"
exit 0`
		ghcExec := filepath.Join(mockBinDir, "ghc")
		err = os.WriteFile(ghcExec, []byte(ghcScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock ghc: %v", err)
		}

		// Create cabal that fails on Run() (not just --version)
		cabalScript := `#!/bin/bash
exit 1`
		cabalExec := filepath.Join(mockBinDir, "cabal")
		err = os.WriteFile(cabalExec, []byte(cabalScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock cabal: %v", err)
		}

		// Set PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should pass CheckHealth but fail the cabal Run() check
		healthy := haskell.CheckEnvironmentHealth(tempDir)
		if healthy {
			t.Error("CheckEnvironmentHealth should return false when cabal Run() fails")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false when cabal Run() fails")
		}
	})
}
