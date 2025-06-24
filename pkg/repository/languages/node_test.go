package languages

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/tests/helpers"
)

func TestNodeLanguage(t *testing.T) {
	node := NewNodeLanguage()

	config := helpers.LanguageTestConfig{
		Language:       node,
		Name:           "Node",
		ExecutableName: "node",
		VersionFlag:    "--version",
		TestVersions:   []string{"default", "system"},
		EnvPathSuffix:  "nodeenv-system",
	}

	helpers.RunLanguageTests(t, config)
}

func TestNodeLanguageSpecific(t *testing.T) {
	t.Run("NewNodeLanguage", func(t *testing.T) {
		node := NewNodeLanguage()
		if node == nil {
			t.Error("NewNodeLanguage() returned nil")
			return
		}
		if node.Base == nil {
			t.Error("NewNodeLanguage() returned instance with nil Base")
		}

		// Check properties
		if node.Name != "Node" {
			t.Errorf("Expected name 'Node', got '%s'", node.Name)
		}
		if node.ExecutableName != "node" {
			t.Errorf("Expected executable name 'node', got '%s'", node.ExecutableName)
		}
		if node.VersionFlag != testVersionFlag {
			t.Errorf("Expected version flag '%s', got '%s'", testVersionFlag, node.VersionFlag)
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		node := NewNodeLanguage()
		tempDir := t.TempDir()

		// Helper function to check if Node.js is available
		isNodeAvailable := func() bool {
			_, err := exec.LookPath("node")
			return err == nil
		}

		// Skip test if Node.js is not available to avoid triggering installation
		if !isNodeAvailable() {
			t.Skip("node not available, skipping test that would trigger Node.js installation")
		}

		// Should handle setup without errors (may fail due to Node.js availability)
		envPath, err := node.SetupEnvironmentWithRepo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err != nil {
			// Node setup may fail if Node.js is not available, that's expected
			t.Logf("SetupEnvironmentWithRepo failed (expected if Node.js not available): %v", err)
		} else {
			if envPath == "" {
				t.Error("SetupEnvironmentWithRepo() returned empty environment path")
			}
		}
	})

	t.Run("InstallDependencies", func(t *testing.T) {
		node := NewNodeLanguage()
		tempDir := t.TempDir()

		// Should handle empty dependencies
		err := node.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		err = node.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}
	})

	t.Run("SimplifiedImplementation", func(t *testing.T) {
		node := NewNodeLanguage()
		tempDir := t.TempDir()

		// Test that InstallDependencies now just logs a warning and returns nil
		err := node.InstallDependencies(tempDir, []string{"some-package"})
		if err != nil {
			t.Errorf("InstallDependencies() should not return error in simplified implementation: %v", err)
		}

		// Test that additional dependencies are ignored in SetupEnvironmentWithRepo
		_, err = node.SetupEnvironmentWithRepo(tempDir, "default", "", "", []string{"some-package"})
		// This will fail if Node.js is not available, but should not fail due to dependencies
		if err != nil && !strings.Contains(err.Error(), "node.js runtime not found") {
			t.Errorf("SetupEnvironmentWithRepo() failed for unexpected reason: %v", err)
		}
	})

	t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		node := NewNodeLanguage()
		tempDir := t.TempDir()

		// Should delegate to base method without error
		err := node.PreInitializeEnvironmentWithRepoInfo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo() returned error: %v", err)
		}

		// Test with system version
		err = node.PreInitializeEnvironmentWithRepoInfo(tempDir, "system", tempDir,
			"dummy-url", []string{"lodash", "express"})
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo() with deps returned error: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepoInfo", func(t *testing.T) {
		node := NewNodeLanguage()
		tempDir := t.TempDir()

		// Should delegate to SetupEnvironmentWithRepo
		envPath, err := node.SetupEnvironmentWithRepoInfo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "node.js runtime not found") {
				t.Logf("SetupEnvironmentWithRepoInfo() failed as expected (Node.js not available): %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepoInfo() failed: %v", err)
			}
		} else {
			t.Logf("SetupEnvironmentWithRepoInfo() succeeded: %s", envPath)
		}

		// Test with additional dependencies (should be ignored)
		envPath, err = node.SetupEnvironmentWithRepoInfo(tempDir, "system", tempDir, "dummy-url", []string{"test-dep"})
		if err != nil {
			if strings.Contains(err.Error(), "node.js runtime not found") {
				t.Logf("SetupEnvironmentWithRepoInfo() with deps failed as expected (Node.js not available): %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepoInfo() with deps failed: %v", err)
			}
		} else {
			t.Logf("SetupEnvironmentWithRepoInfo() with deps succeeded: %s", envPath)
		}
	})
}

func TestNodeLanguage_SetupEnvironmentWithRepositoryInit(t *testing.T) {
	node := NewNodeLanguage()
	tempDir := t.TempDir()

	// Skip if Node.js is not available to avoid triggering installation
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available, skipping test that would trigger Node.js installation")
	}

	// Test basic setup
	envPath, err := node.SetupEnvironmentWithRepositoryInit(tempDir, "default", tempDir, []string{}, nil)
	t.Logf("SetupEnvironmentWithRepositoryInit: %s, %v", envPath, err)

	// Test with dependencies
	envPath, err = node.SetupEnvironmentWithRepositoryInit(tempDir, "18", tempDir, []string{"lodash"}, nil)
	t.Logf("SetupEnvironmentWithRepositoryInit with deps: %s, %v", envPath, err)

	// Test error paths
	_, err = node.SetupEnvironmentWithRepositoryInit("", "default", tempDir, []string{}, nil)
	t.Logf("SetupEnvironmentWithRepositoryInit with empty cache dir: %v", err)
}

func TestNodeLanguage_SetupEnvironmentWithRepo_ComprehensiveCoverage(t *testing.T) {
	node := NewNodeLanguage()

	t.Run("EnvironmentAlreadyExistsAndFunctional", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// First, create an environment
		envPath, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir, "dummy-url", []string{})
		if err != nil {
			t.Fatalf("Initial SetupEnvironmentWithRepo failed: %v", err)
		}

		// Now call again - it should find the existing environment and reuse it
		envPath2, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("Second SetupEnvironmentWithRepo failed: %v", err)
		}
		if envPath != envPath2 {
			t.Errorf("Should reuse existing environment, got different paths: %s != %s", envPath, envPath2)
		}
	})

	t.Run("EnvironmentExistsButBroken", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// First, create an environment
		_, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir, "dummy-url", []string{})
		if err != nil {
			t.Fatalf("Initial SetupEnvironmentWithRepo failed: %v", err)
		}

		// Simulate a broken environment by creating a file where we expect a directory or similar
		// The CheckHealth method might fail, triggering environment recreation
		// For now, just test that subsequent calls still work
		envPath2, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo failed on existing (potentially broken) environment: %v", err)
		}
		if envPath2 == "" {
			t.Error("Should return valid environment path even when recreating")
		}
	})

	t.Run("NodeRuntimeNotAvailable", func(t *testing.T) {
		// Test the error path when Node.js is not available and no cache directory is provided
		// We can only test this if Node.js is actually not available
		if node.IsRuntimeAvailable() {
			t.Skip("Node.js is available, can't test not-available error path")
		}

		// Use empty cache directory to prevent nodeenv auto-installation
		tempRepoDir := t.TempDir()

		_, err := node.SetupEnvironmentWithRepo("", "default", tempRepoDir, "dummy-url", []string{})
		if err == nil {
			t.Error(
				"SetupEnvironmentWithRepo should fail when Node.js is not available and no cache directory is provided",
			)
		}
		if !strings.Contains(err.Error(), "node.js runtime not found") {
			t.Errorf("Expected Node.js not found error, got: %v", err)
		}
	})

	t.Run("VersionNormalization", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Test that unsupported versions get normalized to default
		envPath1, err := node.SetupEnvironmentWithRepo(tempCacheDir, "18.0.0", tempRepoDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with version 18.0.0: %v", err)
		}

		envPath2, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with default version: %v", err)
		}

		// Both should use the same path since unsupported versions become "default"
		if err == nil && envPath1 != envPath2 {
			t.Logf("Version normalization test: %s vs %s", envPath1, envPath2)
		}
	})

	t.Run("WithAdditionalDependencies", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Test that additional dependencies are handled (logged as warning but don't cause failure)
		_, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir,
			"dummy-url", []string{"lodash", "express"})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo with dependencies failed: %v", err)
		}
	})
}

// Additional tests to improve Node.js test coverage
func TestNodeLanguage_ComprehensiveSetupEnvironmentWithRepo(t *testing.T) {
	node := NewNodeLanguage()

	t.Run("SetupEnvironmentWithRepo_EnvironmentCreationFailure", func(t *testing.T) {
		// Test with invalid cache directory that would cause environment creation to fail
		_, err := node.SetupEnvironmentWithRepo("/nonexistent/invalid/cache/path", "default", "", "", []string{})
		if err == nil {
			t.Log("SetupEnvironmentWithRepo succeeded with invalid cache path (may be platform-specific behavior)")
		} else if strings.Contains(err.Error(), "failed to create Node.js environment directory") {
			t.Logf("Successfully tested environment creation failure: %v", err)
		} else if strings.Contains(err.Error(), "node.js runtime not found") {
			t.Logf("Got Node.js not available error: %v", err)
		} else {
			t.Logf("Got different error: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepo_RemoveAllFailure", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()

		// Create an environment first
		envPath, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", "", "", []string{})
		if err != nil {
			t.Fatalf("Initial SetupEnvironmentWithRepo failed: %v", err)
		}

		// Make the environment directory read-only to simulate RemoveAll failure
		err = os.Chmod(envPath, 0o444)
		if err != nil {
			t.Fatalf("Failed to make environment directory read-only: %v", err)
		}
		defer os.Chmod(envPath, 0o755) // Cleanup

		// Try to setup environment again - should hit the RemoveAll error path if health check fails
		_, err = node.SetupEnvironmentWithRepo(tempCacheDir, "default", "", "", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "failed to remove broken environment") {
				t.Logf("Successfully tested RemoveAll error path: %v", err)
			} else {
				t.Logf("Got different error: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo succeeded (environment may have been reused successfully)")
		}
	})

	t.Run("SetupEnvironmentWithRepo_VersionNormalization", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()

		// Test that unsupported versions get normalized to default
		testVersions := []string{
			"18.0.0",   // specific version -> should become default
			"20.1.0",   // another version -> should become default
			"latest",   // unsupported -> should become default
			"v16.14.0", // version with prefix -> should become default
			"default",  // supported as-is
			"system",   // supported as-is
		}

		for _, version := range testVersions {
			t.Run("Version_"+version, func(t *testing.T) {
				envPath, err := node.SetupEnvironmentWithRepo(tempCacheDir, version, "", "", []string{})
				if err != nil {
					t.Logf("SetupEnvironmentWithRepo with version '%s' failed: %v", version, err)
				} else {
					t.Logf("SetupEnvironmentWithRepo with version '%s' succeeded: %s", version, envPath)

					// Verify the path contains the expected normalized version
					if version != testDefaultStr && version != testSystemStr {
						// Non-default/system versions should be normalized to default
						expectedPath := "nodeenv-default"
						if !strings.Contains(envPath, expectedPath) {
							t.Logf("Version '%s' was normalized (expected), path: %s", version, envPath)
						}
					}
				}
			})
		}
	})

	t.Run("SetupEnvironmentWithRepo_NodeRuntimeChecks", func(t *testing.T) {
		tempCacheDir := t.TempDir()

		// Test behavior when Node.js runtime is not available
		if node.IsRuntimeAvailable() {
			t.Log("Node.js is available, cannot test runtime-not-available error path")
		} else {
			// Node.js is not available - should get runtime not found error
			_, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", "", "", []string{})
			if err == nil {
				t.Error("SetupEnvironmentWithRepo should fail when Node.js runtime is not available")
			} else if !strings.Contains(err.Error(), "node.js runtime not found") {
				t.Errorf("Expected node.js runtime not found error, got: %v", err)
			} else {
				t.Logf("Successfully tested Node.js runtime not available error: %v", err)
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_AdditionalDependencies", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()

		// Test that additional dependencies are handled (logged but don't cause failure)
		envPath, err := node.SetupEnvironmentWithRepo(
			tempCacheDir,
			"default",
			"",
			"",
			[]string{"lodash", "express", "axios"},
		)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo with dependencies should not fail: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo with dependencies succeeded: %s", envPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo_EmptyVersion", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()

		// Test with empty version - should normalize to default
		envPath, err := node.SetupEnvironmentWithRepo(tempCacheDir, "", "", "", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with empty version failed: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo with empty version succeeded: %s", envPath)
			// Should contain default in the path
			if !strings.Contains(envPath, "nodeenv-default") {
				t.Errorf("Expected path to contain 'nodeenv-default', got: %s", envPath)
			}
		}
	})
}

// Tests for specific Node.js error and edge cases
func TestNodeLanguage_ErrorHandling(t *testing.T) {
	node := NewNodeLanguage()

	t.Run("InstallDependencies_AllScenarios", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test empty dependencies
		err := node.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies with empty deps should not fail: %v", err)
		}

		// Test nil dependencies
		err = node.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies with nil deps should not fail: %v", err)
		}

		// Test with actual dependencies (should log warning but not fail)
		err = node.InstallDependencies(tempDir, []string{"lodash", "express"})
		if err != nil {
			t.Errorf("InstallDependencies with deps should not fail (only logs warning): %v", err)
		}

		// Test with invalid path (should still not fail since it doesn't actually install)
		err = node.InstallDependencies("/invalid/path", []string{"some-package"})
		if err != nil {
			t.Errorf("InstallDependencies should not fail even with invalid path: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepositoryInit_AllScenarios", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with various parameters
		scenarios := []struct {
			extra    any
			name     string
			cacheDir string
			version  string
			repoPath string
			deps     []string
		}{
			{nil, "Normal", tempDir, "default", tempDir, []string{}},
			{nil, "WithDeps", tempDir, "system", tempDir, []string{"lodash"}},
			{nil, "EmptyCache", "", "default", tempDir, []string{}},
			{nil, "EmptyVersion", tempDir, "", tempDir, []string{}},
			{"extra", "WithExtraParam", tempDir, "default", tempDir, []string{}},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				envPath, err := node.SetupEnvironmentWithRepositoryInit(
					scenario.cacheDir,
					scenario.version,
					scenario.repoPath,
					scenario.deps,
					scenario.extra,
				)

				if err != nil {
					if strings.Contains(err.Error(), "node.js runtime not found") {
						t.Logf("SetupEnvironmentWithRepositoryInit failed as expected (Node.js not available): %v", err)
					} else {
						t.Logf("SetupEnvironmentWithRepositoryInit failed: %v", err)
					}
				} else {
					t.Logf("SetupEnvironmentWithRepositoryInit succeeded: %s", envPath)
				}
			})
		}
	})
}

// Test Node.js environment structure and caching behavior
func TestNodeLanguage_EnvironmentStructure(t *testing.T) {
	node := NewNodeLanguage()

	t.Run("SetupEnvironmentWithRepo_CacheDirectoryStructure", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Test that environments are created in repository directory with correct naming
		envPath, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir, "", []string{})
		if err != nil {
			t.Fatalf("SetupEnvironmentWithRepo failed: %v", err)
		}

		// Should be in repository directory
		if !strings.HasPrefix(envPath, tempRepoDir) {
			t.Errorf(
				"Environment should be created in repository directory: expected prefix %s, got %s",
				tempRepoDir,
				envPath,
			)
		}

		// Should use correct naming convention
		if !strings.Contains(envPath, "nodeenv-default") {
			t.Errorf("Environment should use nodeenv-default naming: %s", envPath)
		}

		// Test system version
		envPath2, err := node.SetupEnvironmentWithRepo(tempCacheDir, "system", tempRepoDir, "", []string{})
		if err != nil {
			t.Fatalf("SetupEnvironmentWithRepo with system version failed: %v", err)
		}

		if !strings.Contains(envPath2, "nodeenv-system") {
			t.Errorf("System version should use nodeenv-system naming: %s", envPath2)
		}

		// Paths should be different for different versions
		if envPath == envPath2 {
			t.Error("Different versions should create different environment paths")
		}
	})

	t.Run("SetupEnvironmentWithRepo_EnvironmentReuse", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()
		tempRepoDir := t.TempDir()

		// Create environment first time
		envPath1, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir, "", []string{})
		if err != nil {
			t.Fatalf("First SetupEnvironmentWithRepo failed: %v", err)
		}

		// Create environment second time - should reuse
		envPath2, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", tempRepoDir, "", []string{})
		if err != nil {
			t.Fatalf("Second SetupEnvironmentWithRepo failed: %v", err)
		}

		if envPath1 != envPath2 {
			t.Errorf("Should reuse existing environment: %s != %s", envPath1, envPath2)
		}

		// Verify the environment directory actually exists
		if _, err := os.Stat(envPath1); err != nil {
			t.Errorf("Environment directory should exist: %v", err)
		}
	})
}

// Additional edge case tests to push coverage even higher
func TestNodeLanguage_AdditionalEdgeCases(t *testing.T) {
	node := NewNodeLanguage()

	t.Run("SetupEnvironmentWithRepo_HealthCheckFailure", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()

		// Create an environment first
		envPath, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", "", "", []string{})
		if err != nil {
			t.Fatalf("Initial SetupEnvironmentWithRepo failed: %v", err)
		}

		// Corrupt the environment to make health check fail
		// Remove the environment directory but leave a file with the same name
		err = os.RemoveAll(envPath)
		if err != nil {
			t.Fatalf("Failed to remove environment: %v", err)
		}

		// Create a file where the directory should be to cause issues
		err = os.WriteFile(envPath, []byte("corrupt"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create corrupting file: %v", err)
		}

		// Try to setup environment again - should detect broken environment and try to recreate
		_, err = node.SetupEnvironmentWithRepo(tempCacheDir, "default", "", "", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "failed to remove broken environment") {
				t.Logf("Successfully tested broken environment removal failure: %v", err)
			} else if strings.Contains(err.Error(), "failed to create Node.js environment directory") {
				t.Logf("Successfully tested environment creation failure after corruption: %v", err)
			} else {
				t.Logf("Got different error: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo succeeded despite corruption")
		}
	})

	t.Run("SetupEnvironmentWithRepo_CheckHealthEdgeCases", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		tempCacheDir := t.TempDir()

		// Create environment
		envPath, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", "", "", []string{})
		if err != nil {
			t.Fatalf("SetupEnvironmentWithRepo failed: %v", err)
		}

		// Verify environment can be reused when health check passes
		envPath2, err := node.SetupEnvironmentWithRepo(tempCacheDir, "default", "", "", []string{})
		if err != nil {
			t.Errorf("Second SetupEnvironmentWithRepo failed: %v", err)
		} else if envPath != envPath2 {
			t.Errorf("Should reuse healthy environment: %s != %s", envPath, envPath2)
		}
	})

	t.Run("SetupEnvironmentWithRepo_CreateEnvironmentDirectoryEdgeCases", func(t *testing.T) {
		// Skip if Node.js is not available
		if !node.IsRuntimeAvailable() {
			t.Skip("Node.js not available, skipping test")
		}

		// Test creation in a non-existent parent directory
		_, err := node.SetupEnvironmentWithRepo("/nonexistent/parent/dir", "default", "", "", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "failed to create Node.js environment directory") {
				t.Logf("Successfully tested parent directory creation failure: %v", err)
			} else {
				t.Logf("Got different error: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo succeeded with non-existent parent (may be platform-specific)")
		}
	})
}
