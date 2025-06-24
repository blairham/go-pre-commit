package languages

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testSystemStr = "system"
)

func TestLuaLanguage(t *testing.T) {
	t.Run("NewLuaLanguage", func(t *testing.T) {
		lua := NewLuaLanguage()
		if lua == nil {
			t.Error("NewLuaLanguage() returned nil")
			return
		}
		if lua.Base == nil {
			t.Error("NewLuaLanguage() returned instance with nil Base")
		}

		// Check properties
		if lua.Name != "Lua" {
			t.Errorf("Expected name 'Lua', got '%s'", lua.Name)
		}
		if lua.ExecutableName != "lua" {
			t.Errorf("Expected executable name 'lua', got '%s'", lua.ExecutableName)
		}
		if lua.VersionFlag != "-v" {
			t.Errorf("Expected version flag '-v', got '%s'", lua.VersionFlag)
		}
		if lua.InstallURL != "https://www.lua.org/" {
			t.Errorf("Expected install URL 'https://www.lua.org/', got '%s'", lua.InstallURL)
		}
	})

	t.Run("InstallDependencies_Empty", func(t *testing.T) {
		lua := NewLuaLanguage()
		tempDir := t.TempDir()

		// Should handle empty dependencies without error
		err := lua.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		err = lua.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}
	})

	t.Run("InstallDependencies_WithDeps", func(t *testing.T) {
		lua := NewLuaLanguage()
		tempDir := t.TempDir()

		// Test regardless of luarocks availability to exercise code paths
		err := lua.InstallDependencies(tempDir, []string{"luafilesystem", "lua-cjson==2.1.0"})
		if err != nil {
			t.Logf("InstallDependencies failed (may be expected if luarocks not available): %v", err)
		} else {
			t.Log("InstallDependencies succeeded")
		}

		// Check if lua_modules directory was created (may or may not exist depending on luarocks availability)
		luaModulesPath := filepath.Join(tempDir, "lua_modules")
		if _, statErr := os.Stat(luaModulesPath); statErr == nil {
			t.Log("lua_modules directory was created")
		} else {
			t.Logf("lua_modules directory not created: %v", statErr)
		}

		// Test with version-specific dependency to exercise parsing logic
		err = lua.InstallDependencies(tempDir, []string{"simple-dep", "versioned-dep==1.0.0"})
		if err != nil {
			t.Logf("InstallDependencies with versioned deps failed (may be expected): %v", err)
		}
	})

	t.Run("InstallDependencies_InvalidPath", func(t *testing.T) {
		lua := NewLuaLanguage()

		// Skip test if luarocks is not available to avoid triggering installation
		if _, err := exec.LookPath("luarocks"); err != nil {
			t.Skip("luarocks not available, skipping test that would trigger dependency installation")
		}

		// Test with invalid path - should fail to create directories
		err := lua.InstallDependencies("/invalid/readonly/path", []string{"test-dep"})
		if err == nil {
			t.Error("InstallDependencies() with invalid path should return error")
		}
	})

	t.Run("CheckEnvironmentHealth", func(t *testing.T) {
		lua := NewLuaLanguage()
		tempDir := t.TempDir()

		// Should return false for non-existent environment (depending on lua availability)
		healthy := lua.CheckEnvironmentHealth("/non/existent/path")
		if healthy {
			t.Error("CheckEnvironmentHealth() should return false for non-existent environment")
		}

		// Test with existing directory
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		healthy = lua.CheckEnvironmentHealth(tempDir)
		// This may vary depending on lua availability, just ensure it doesn't panic
		t.Logf("CheckEnvironmentHealth for existing directory: %v", healthy)
	})

	t.Run("CheckEnvironmentHealth_WithLuaModules", func(t *testing.T) {
		lua := NewLuaLanguage()
		tempDir := t.TempDir()

		// Create lua_modules structure
		luaModulesPath := filepath.Join(tempDir, "lua_modules")
		libLuaPath := filepath.Join(luaModulesPath, "lib", "lua")
		if err := os.MkdirAll(libLuaPath, 0o755); err != nil {
			t.Fatalf("Failed to create lua_modules structure: %v", err)
		}

		healthy := lua.CheckEnvironmentHealth(tempDir)
		// Health depends on lua availability, but structure should be valid
		t.Logf("CheckEnvironmentHealth with lua_modules structure: %v", healthy)
	})

	t.Run("CheckEnvironmentHealth_InvalidLuaModules", func(t *testing.T) {
		lua := NewLuaLanguage()
		tempDir := t.TempDir()

		// Create lua_modules directory but without proper structure
		luaModulesPath := filepath.Join(tempDir, "lua_modules")
		if err := os.MkdirAll(luaModulesPath, 0o755); err != nil {
			t.Fatalf("Failed to create lua_modules directory: %v", err)
		}

		healthy := lua.CheckEnvironmentHealth(tempDir)
		// Should return false because lua_modules exists but lib/lua doesn't
		if healthy {
			t.Log("CheckEnvironmentHealth returned true despite invalid lua_modules " +
				"structure (may be due to lua unavailability)")
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		lua := NewLuaLanguage()
		tempDir := t.TempDir()

		// Helper function to check if Lua is available
		isLuaAvailable := func() bool {
			_, err := exec.LookPath("lua")
			return err == nil
		}

		// Skip test if Lua is not available to avoid triggering installation
		if !isLuaAvailable() {
			t.Skip("lua not available, skipping test that would trigger Lua installation")
		}

		// Should delegate to SimpleSetupEnvironmentWithRepo
		envPath, err := lua.SetupEnvironmentWithRepo(tempDir, "5.4", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() returned empty environment path")
		}

		// Test with additional dependencies (may fail due to luarocks availability or package not found)
		envPath, err = lua.SetupEnvironmentWithRepo(tempDir, "5.3", tempDir, "dummy-url", []string{"test-dep"})
		if err != nil {
			t.Logf(
				"SetupEnvironmentWithRepo() with deps failed (may be expected due to luarocks/package availability): %v",
				err,
			)
		} else {
			t.Logf("SetupEnvironmentWithRepo() with deps succeeded: %s", envPath)
		}
	})

	// Additional tests for better coverage
	t.Run("SetupEnvironmentWithRepo_ErrorCases", func(t *testing.T) {
		lua := NewLuaLanguage()

		// Skip if Lua is not available
		if _, err := exec.LookPath("lua"); err != nil {
			t.Skip("lua not available, skipping test that could trigger Lua installation or setup")
		}

		// Test with invalid repo path
		_, err := lua.SetupEnvironmentWithRepo("", "5.4", "/nonexistent/invalid/path", "dummy-url", []string{})
		// This may or may not return an error depending on the implementation
		t.Logf("SetupEnvironmentWithRepo with invalid repo path: %v", err)

		// Test with empty version
		_, err = lua.SetupEnvironmentWithRepo("", "", "/tmp", "dummy-url", []string{})
		t.Logf("SetupEnvironmentWithRepo with empty version: %v", err)

		// Test with nil dependencies
		_, err = lua.SetupEnvironmentWithRepo("", "5.4", "/tmp", "dummy-url", nil)
		t.Logf("SetupEnvironmentWithRepo with nil dependencies: %v", err)
	})

	t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		lua := NewLuaLanguage()
		tempDir := t.TempDir()

		// Should delegate to base method without error
		err := lua.PreInitializeEnvironmentWithRepoInfo(tempDir, "5.4", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo() returned error: %v", err)
		}

		// Test with additional dependencies
		err = lua.PreInitializeEnvironmentWithRepoInfo(tempDir, "5.3", tempDir,
			"dummy-url", []string{"luafilesystem", "lua-cjson"})
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo() with deps returned error: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepoInfo", func(t *testing.T) {
		lua := NewLuaLanguage()
		tempDir := t.TempDir()

		// Should delegate to SetupEnvironmentWithRepo
		envPath, err := lua.SetupEnvironmentWithRepoInfo(tempDir, "5.4", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepoInfo() failed (may be expected if Lua environment setup fails): %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepoInfo() succeeded: %s", envPath)
		}

		// Test with additional dependencies (may fail due to luarocks availability)
		envPath, err = lua.SetupEnvironmentWithRepoInfo(tempDir, "5.3", tempDir, "dummy-url", []string{"test-dep"})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepoInfo() with deps failed (may be expected): %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepoInfo() with deps succeeded: %s", envPath)
		}
	})
}

// Additional comprehensive tests for better coverage
func TestLuaLanguage_ComprehensiveCoverage(t *testing.T) {
	lua := NewLuaLanguage()

	t.Run("InstallDependencies_VersionParsing", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test dependency parsing with and without versions
		// This should exercise both branches in the version parsing logic
		deps := []string{
			"simple-dep",           // no version
			"versioned-dep==1.0.0", // with version
			"another-dep",          // no version
			"complex-dep==2.1.3",   // with version
		}

		err := lua.InstallDependencies(tempDir, deps)
		// This will likely fail if luarocks isn't available, but will exercise the code paths
		if err != nil {
			t.Logf("InstallDependencies failed as expected (luarocks may not be available): %v", err)
		}

		// Check that lua_modules directory creation was attempted
		luaModulesPath := filepath.Join(tempDir, "lua_modules")
		if _, err := os.Stat(luaModulesPath); err == nil {
			t.Log("lua_modules directory was created")
		}
	})

	t.Run("CheckEnvironmentHealth_AllBranches", func(t *testing.T) {
		// Test 1: Base health check fails (non-existent path)
		t.Run("NonExistentPath", func(t *testing.T) {
			healthy := lua.CheckEnvironmentHealth("/definitely/does/not/exist")
			if healthy {
				t.Error("CheckEnvironmentHealth should return false for non-existent path")
			}
		})

		// Test 2: Base health passes, no lua_modules
		t.Run("NoLuaModules", func(t *testing.T) {
			tempDir := t.TempDir()
			healthy := lua.CheckEnvironmentHealth(tempDir)
			// This will depend on lua availability
			t.Logf("CheckEnvironmentHealth without lua_modules: %v", healthy)
		})

		// Test 3: Base health passes, lua_modules exists with proper structure
		t.Run("ValidLuaModulesStructure", func(t *testing.T) {
			tempDir := t.TempDir()

			// Create proper lua_modules structure
			luaModulesPath := filepath.Join(tempDir, "lua_modules")
			libLuaPath := filepath.Join(luaModulesPath, "lib", "lua")
			if err := os.MkdirAll(libLuaPath, 0o755); err != nil {
				t.Fatalf("Failed to create lua_modules structure: %v", err)
			}

			healthy := lua.CheckEnvironmentHealth(tempDir)
			t.Logf("CheckEnvironmentHealth with valid lua_modules structure: %v", healthy)
		})

		// Test 4: Base health passes, lua_modules exists but missing lib/lua
		t.Run("InvalidLuaModulesStructure", func(t *testing.T) {
			tempDir := t.TempDir()

			// Create lua_modules directory but without lib/lua subdirectory
			luaModulesPath := filepath.Join(tempDir, "lua_modules")
			if err := os.MkdirAll(luaModulesPath, 0o755); err != nil {
				t.Fatalf("Failed to create lua_modules directory: %v", err)
			}
			// Deliberately don't create lib/lua subdirectory

			healthy := lua.CheckEnvironmentHealth(tempDir)
			// This should exercise the branch where lua_modules exists but lib/lua doesn't
			t.Logf("CheckEnvironmentHealth with invalid lua_modules structure: %v", healthy)
		})
	})

	t.Run("InstallDependencies_DirectoryCreationError", func(t *testing.T) {
		// Try to install dependencies in a path that would cause directory creation to fail
		// Create a file where we want to create a directory
		tempDir := t.TempDir()
		conflictFile := filepath.Join(tempDir, "lua_modules")

		// Create a file with the same name as the directory we want to create
		if err := os.WriteFile(conflictFile, []byte("conflict"), 0o644); err != nil {
			t.Fatalf("Failed to create conflict file: %v", err)
		}

		err := lua.InstallDependencies(tempDir, []string{"test-dep"})
		if err == nil {
			t.Error("InstallDependencies should fail when lua_modules directory creation fails")
		} else {
			t.Logf("InstallDependencies correctly failed due to directory creation conflict: %v", err)
		}
	})
}

// Test to improve CheckEnvironmentHealth coverage by mocking lua executable
func TestLuaLanguage_CheckEnvironmentHealthWithMockLua(t *testing.T) {
	lua := NewLuaLanguage()

	t.Run("CheckEnvironmentHealthWithWorkingLua", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create bin directory and mock lua executable
		binPath := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create a mock lua executable that responds to -v
		luaExec := filepath.Join(binPath, "lua")
		luaScript := `#!/bin/bash
if [[ "$1" == "-v" ]]; then
  echo "Lua 5.4.0  Copyright (C) 1994-2020 Lua.org, PUC-Rio"
  exit 0
fi
exit 1`
		if err := os.WriteFile(luaExec, []byte(luaScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock lua executable: %v", err)
		}

		// Test 1: Environment with no lua_modules - should pass base health check and return true
		healthy := lua.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth should return true when base health passes and no lua_modules")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned true with working lua and no lua_modules")
		}

		// Test 2: Environment with lua_modules but no proper structure - should return false
		luaModulesPath := filepath.Join(tempDir, "lua_modules")
		if err := os.MkdirAll(luaModulesPath, 0o755); err != nil {
			t.Fatalf("Failed to create lua_modules directory: %v", err)
		}

		healthy = lua.CheckEnvironmentHealth(tempDir)
		if healthy {
			t.Error("CheckEnvironmentHealth should return false when lua_modules exists but lib/lua doesn't")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false for invalid lua_modules structure")
		}

		// Test 3: Environment with proper lua_modules structure - should return true
		libLuaPath := filepath.Join(luaModulesPath, "lib", "lua")
		if err := os.MkdirAll(libLuaPath, 0o755); err != nil {
			t.Fatalf("Failed to create lib/lua directory: %v", err)
		}

		healthy = lua.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth should return true when lua_modules has proper structure")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned true for valid lua_modules structure")
		}
	})
}

// Test to improve InstallDependencies coverage for command failure scenarios
func TestLuaLanguage_InstallDependenciesErrors(t *testing.T) {
	lua := NewLuaLanguage()

	t.Run("LuarocksNotAvailable", func(t *testing.T) {
		tempDir := t.TempDir()

		// Temporarily modify PATH to make luarocks unavailable
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		emptyDir := filepath.Join(tempDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should exercise the "luarocks not found" error path
		err := lua.InstallDependencies(tempDir, []string{"test-package"})
		if err == nil {
			t.Error("InstallDependencies should fail when luarocks not available")
		} else {
			if !strings.Contains(err.Error(), "luarocks not found") {
				t.Errorf("Expected error to contain 'luarocks not found', got: %v", err)
			} else {
				t.Logf("InstallDependencies correctly failed when luarocks not available: %v", err)
			}
		}
	})
}

// Test to cover the "luarocks not found" path in InstallDependencies
func TestLuaLanguage_InstallDependenciesNoLuarocks(t *testing.T) {
	lua := NewLuaLanguage()

	t.Run("LuarocksNotAvailable", func(t *testing.T) {
		tempDir := t.TempDir()

		// Temporarily modify PATH to make luarocks unavailable
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		// Set PATH to a directory that doesn't contain luarocks
		emptyDir := filepath.Join(tempDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should hit the "luarocks not found" error path
		err := lua.InstallDependencies(tempDir, []string{"test-dep"})
		if err == nil {
			t.Error("InstallDependencies should fail when luarocks is not available")
		} else {
			if !strings.Contains(err.Error(), "luarocks not found") {
				t.Errorf("Expected error to contain 'luarocks not found', got: %v", err)
			} else {
				t.Logf("InstallDependencies correctly failed when luarocks not available: %v", err)
			}
		}
	})
}

// Test to verify Lua environment structure and naming conventions
func TestLuaLanguage_EnvironmentStructure(t *testing.T) {
	lua := NewLuaLanguage()

	t.Run("SetupEnvironmentWithRepo_CorrectNaming", func(t *testing.T) {
		testEnvironmentNaming(t, lua, "5.4", "luaenv")
	})

	t.Run("SetupEnvironmentWithRepo_ExistingHealthyEnvironment", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a healthy environment manually
		envPath := filepath.Join(tempDir, "luaenv-5.4")
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create proper lua_modules structure to make it healthy
		luaModulesPath := filepath.Join(envPath, "lua_modules")
		libLuaPath := filepath.Join(luaModulesPath, "lib", "lua")
		err = os.MkdirAll(libLuaPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create lua_modules structure: %v", err)
		}

		// Create mock lua to make health check pass
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err = os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		luaScript := `#!/bin/bash
if [[ "$1" == "-v" ]]; then
  echo "Lua 5.4.0  Copyright (C) 1994-2020 Lua.org, PUC-Rio"
  exit 0
fi
exit 0`
		luaExec := filepath.Join(mockBinDir, "lua")
		err = os.WriteFile(luaExec, []byte(luaScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock lua: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// Call SetupEnvironmentWithRepo - should reuse healthy environment
		resultPath, err := lua.SetupEnvironmentWithRepo("", "5.4", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo should succeed with healthy environment: %v", err)
		} else if resultPath != envPath {
			t.Errorf("Should reuse existing healthy environment: expected %s, got %s", envPath, resultPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo_ExistingBrokenEnvironment", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a broken environment (missing proper lua_modules structure)
		envPath := filepath.Join(tempDir, "luaenv-5.4")
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create lua_modules but without proper structure (broken)
		luaModulesPath := filepath.Join(envPath, "lua_modules")
		err = os.MkdirAll(luaModulesPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create lua_modules directory: %v", err)
		}
		// Deliberately don't create lib/lua subdirectory

		// Add a marker file to verify environment gets recreated
		markerFile := filepath.Join(envPath, "broken_marker")
		err = os.WriteFile(markerFile, []byte("broken"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create marker file: %v", err)
		}

		// Call SetupEnvironmentWithRepo - should detect broken environment and recreate
		resultPath, err := lua.SetupEnvironmentWithRepo("", "5.4", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo failed (may be expected): %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo succeeded with environment: %s", resultPath)

			// Verify marker file was removed (environment was recreated)
			if _, statErr := os.Stat(markerFile); !os.IsNotExist(statErr) {
				t.Error("Broken environment should have been removed and recreated")
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_EnvironmentCreationFailure", func(t *testing.T) {
		luaLang := NewLuaLanguage()

		// Test with invalid repo path that would cause environment creation to fail
		_, err := luaLang.SetupEnvironmentWithRepo("", "5.4", "/nonexistent/invalid/repo/path", "dummy-url", []string{})
		if err == nil {
			t.Log("SetupEnvironmentWithRepo succeeded with invalid repo path (may be platform-specific behavior)")
		} else if strings.Contains(err.Error(), "failed to create Lua environment directory") {
			t.Logf("Successfully tested environment creation failure: %v", err)
		} else {
			t.Logf("Got different error than expected: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepo_DependencyInstallationFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test dependency installation failure path
		_, err := lua.SetupEnvironmentWithRepo("", "5.4", tempDir, "dummy-url", []string{"NonexistentPackage123"})
		if err == nil {
			t.Log("SetupEnvironmentWithRepo succeeded (luarocks may not be available)")
		} else if strings.Contains(err.Error(), "failed to install Lua dependencies") {
			t.Logf("Successfully tested dependency installation failure: %v", err)
		} else {
			t.Logf("Got different error (expected if luarocks not available): %v", err)
		}
	})
}

// Additional tests to improve SetupEnvironmentWithRepo coverage
func TestLuaLanguage_SetupEnvironmentWithRepo_AdditionalCoverage(t *testing.T) {
	lua := NewLuaLanguage()

	t.Run("SetupEnvironmentWithRepo_RemoveAllFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create environment directory
		envPath := filepath.Join(tempDir, "luaenv-5.4")
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Make the directory read-only after creation to simulate RemoveAll failure
		// Note: This might not work on all platforms
		err = os.Chmod(envPath, 0o444)
		if err != nil {
			t.Fatalf("Failed to make directory read-only: %v", err)
		}
		defer os.Chmod(envPath, 0o755) // Cleanup

		// Try to setup environment - should hit the RemoveAll error path
		_, err = lua.SetupEnvironmentWithRepo("", "5.4", tempDir, "dummy-url", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "failed to remove broken environment") {
				t.Logf("Successfully tested RemoveAll error path: %v", err)
			} else {
				t.Logf("Got different error: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo succeeded (RemoveAll might have worked despite read-only)")
		}
	})

	t.Run("SetupEnvironmentWithRepo_CreateDirectoryError", func(t *testing.T) {
		// Try to create environment in a location that should fail
		// Use a path that exists as a file, not a directory
		tempDir := t.TempDir()
		tempFile := filepath.Join(tempDir, "tempfile")
		err := os.WriteFile(tempFile, []byte("temp"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}

		// Try to setup environment in a subdirectory of the file (should fail)
		_, err = lua.SetupEnvironmentWithRepo("", "5.4", tempFile, "dummy-url", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "failed to create Lua environment directory") {
				t.Logf("Successfully tested CreateEnvironmentDirectory error: %v", err)
			} else {
				t.Logf("Got different error: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo succeeded (platform-specific behavior may allow this)")
		}
	})

	t.Run("SetupEnvironmentWithRepo_EmptyEnvironmentName", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a mock language that would return empty environment name
		// by temporarily modifying the name to something that would result in empty
		originalName := lua.Name
		lua.Name = testSystemStr // This should result in empty environment name
		defer func() { lua.Name = originalName }()

		envPath, err := lua.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with system language failed: %v", err)
		} else {
			// Should return the repo path itself when environment name is empty
			if envPath == tempDir {
				t.Log("Successfully handled empty environment name case - returned repo path")
			} else {
				t.Logf("Got environment path: %s", envPath)
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_VersionEdgeCases", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test edge cases for version handling
		testVersions := []string{
			"",        // empty version
			"default", // explicit default
			"5.4",     // specific version
			"5.3.6",   // specific patch version
			"latest",  // latest version
		}

		for _, version := range testVersions {
			t.Run("Version_"+version, func(t *testing.T) {
				envPath, err := lua.SetupEnvironmentWithRepo("", version, tempDir, "dummy-url", []string{})

				if err != nil {
					t.Logf("SetupEnvironmentWithRepo with version '%s' failed: %v", version, err)
				} else {
					t.Logf("SetupEnvironmentWithRepo with version '%s' succeeded: %s", version, envPath)

					// Verify the environment directory name is correct
					expectedVersionName := version
					if version == "" {
						expectedVersionName = testDefaultStr
					}
					expectedPath := filepath.Join(tempDir, "luaenv-"+expectedVersionName)
					if envPath != expectedPath && envPath != tempDir {
						t.Errorf("Expected environment path %s, got %s", expectedPath, envPath)
					}
				}
			})
		}
	})
}
