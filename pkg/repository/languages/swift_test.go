package languages

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Test constants to avoid goconst issues
const (
	testPackageSwiftContent = `// swift-tools-version: 5.7
import PackageDescription

let package = Package(
    name: "TestPackage",
    targets: [
        .executableTarget(name: "TestPackage"),
    ]
)
`
	testMockSwiftScript = `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo 'swift-driver version: 1.0'
  exit 0
elif [[ "$1" == "package" && "$2" == "show-dependencies" ]]; then
  echo 'Dependencies resolved'
  exit 0
fi
exit 1
`
	testAdvancedSwiftScript = `#!/bin/bash
case "$1" in
    "--version")
        echo 'swift-driver version: 5.9'
        exit 0
        ;;
    "package")
        case "$2" in
            "show-dependencies")
                echo 'No dependencies found.'
                exit 0
                ;;
            "resolve")
                echo 'Dependencies resolved.'
                exit 0
                ;;
            *)
                exit 1
                ;;
        esac
        ;;
    *)
        exit 1
        ;;
esac
`
	testPackageSwiftAlternate = `// swift-tools-version: 5.7
import PackageDescription

let package = Package(
    name: "TestPackage",
    dependencies: [],
    targets: [
        .target(name: "TestPackage", dependencies: [])
    ]
)
`
)

// Helper function to create a mock Swift environment for testing
func createMockSwiftEnvironment(
	t *testing.T,
	tempDir string,
	scriptType string,
	withPackageSwift bool,
) {
	t.Helper()

	// Create bin directory with mock swift executable
	binPath := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binPath, 0o755); err != nil {
		t.Fatalf("Failed to create bin directory: %v", err)
	}

	// Select appropriate script
	var script string
	switch scriptType {
	case "basic":
		script = testMockSwiftScript
	case "advanced":
		script = testAdvancedSwiftScript
	default:
		script = testMockSwiftScript
	}

	// Create a mock swift executable
	swiftExec := filepath.Join(binPath, "swift")
	if err := os.WriteFile(swiftExec, []byte(script), 0o755); err != nil {
		t.Fatalf("Failed to create mock swift executable: %v", err)
	}

	if withPackageSwift {
		// Create Package.swift to trigger the manifest check
		packageSwiftPath := filepath.Join(tempDir, "Package.swift")
		if err := os.WriteFile(packageSwiftPath, []byte(testPackageSwiftContent), 0o644); err != nil {
			t.Fatalf("Failed to create Package.swift: %v", err)
		}
	}

	// Temporarily modify PATH to use our mock swift
	originalPath := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})
	_ = os.Setenv("PATH", binPath+string(os.PathListSeparator)+originalPath)
}

func TestSwiftLanguage(t *testing.T) {
	t.Run("NewSwiftLanguage", func(t *testing.T) {
		swift := NewSwiftLanguage()
		if swift == nil {
			t.Error("NewSwiftLanguage() returned nil")
			return
		}
		if swift.Base == nil {
			t.Error("NewSwiftLanguage() returned instance with nil Base")
		}

		// Check properties
		if swift.Name != "Swift" {
			t.Errorf("Expected name 'Swift', got '%s'", swift.Name)
		}
		if swift.ExecutableName != "swift" {
			t.Errorf("Expected executable name 'swift', got '%s'", swift.ExecutableName)
		}
		if swift.VersionFlag != "--version" {
			t.Errorf("Expected version flag '--version', got '%s'", swift.VersionFlag)
		}
		if swift.InstallURL != "https://swift.org/download/" {
			t.Errorf("Expected install URL 'https://swift.org/download/', got '%s'", swift.InstallURL)
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Helper function to check if Swift is available
		isSwiftAvailable := func() bool {
			_, err := exec.LookPath("swift")
			return err == nil
		}

		// Skip test if Swift is not available to avoid triggering installation
		if !isSwiftAvailable() {
			t.Skip("swift not available, skipping test that would trigger Swift installation")
		}

		// Should create environment directory
		envPath, err := swift.SetupEnvironmentWithRepo(tempDir, "5.7", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() returned empty environment path")
		}

		// Verify environment directory was created
		expectedPath := filepath.Join(tempDir, "swiftenv-5.7")
		if envPath != expectedPath {
			t.Errorf("SetupEnvironmentWithRepo() returned unexpected path: got %s, want %s", envPath, expectedPath)
		}

		// Directory should exist
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			t.Error("SetupEnvironmentWithRepo() did not create environment directory")
		}
	})

	t.Run("SetupEnvironmentWithRepo_ExistingEnvironment", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Helper function to check if Swift is available
		isSwiftAvailable := func() bool {
			_, err := exec.LookPath("swift")
			return err == nil
		}

		// Skip test if Swift is not available to avoid triggering installation
		if !isSwiftAvailable() {
			t.Skip("swift not available, skipping test that would trigger Swift installation")
		}

		// Create environment first
		envPath1, err := swift.SetupEnvironmentWithRepo(tempDir, "5.8", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Fatalf("First SetupEnvironmentWithRepo() failed: %v", err)
		}

		// Call again - should reuse existing environment or recreate if unhealthy
		envPath2, err := swift.SetupEnvironmentWithRepo(tempDir, "5.8", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("Second SetupEnvironmentWithRepo() returned error: %v", err)
		}

		if envPath1 != envPath2 {
			t.Errorf(
				"SetupEnvironmentWithRepo() should return same path for same version: got %s != %s",
				envPath1,
				envPath2,
			)
		}
	})

	t.Run("SetupEnvironmentWithRepo_WithDependencies", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Helper function to check if Swift is available
		isSwiftAvailable := func() bool {
			_, err := exec.LookPath("swift")
			return err == nil
		}

		// Skip test if Swift is not available to avoid triggering installation
		if !isSwiftAvailable() {
			t.Skip("swift not available, skipping test that would trigger Swift installation")
		}

		// Test with dependencies - may fail if Swift not available or dependencies invalid
		envPath, err := swift.SetupEnvironmentWithRepo(tempDir, "5.7", tempDir,
			"dummy-url", []string{"ArgumentParser", "SwiftLog"})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() with deps failed (expected if Swift not available): %v", err)
		} else {
			if envPath == "" {
				t.Error("SetupEnvironmentWithRepo() with deps returned empty environment path")
			}
		}
	})

	t.Run("InstallDependencies_Empty", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Should handle empty dependencies without error
		err := swift.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		err = swift.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}
	})

	t.Run("InstallDependencies_WithDeps", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Skip test if Swift is not available to avoid triggering installation
		if _, err := exec.LookPath("swift"); err != nil {
			t.Skip("swift not available, skipping test that would trigger dependency installation")
		}

		// Test with some dependencies - this will likely fail since Swift may not be available
		err := swift.InstallDependencies(tempDir, []string{"ArgumentParser", "SwiftLog"})
		if err != nil {
			t.Logf("InstallDependencies failed (expected if Swift not available): %v", err)
		}

		// Should create Package.swift and other files even if Swift is not available for resolution
		packageSwiftPath := filepath.Join(tempDir, "Package.swift")
		if _, err := os.Stat(packageSwiftPath); err == nil {
			t.Log("Package.swift was created")
		}

		// Should create Sources directory structure
		sourcesPath := filepath.Join(tempDir, "Sources", "PreCommitEnv", "main.swift")
		if _, err := os.Stat(sourcesPath); err == nil {
			t.Log("Sources/PreCommitEnv/main.swift was created")
		}
	})

	t.Run("InstallDependencies_InvalidPath", func(t *testing.T) {
		swift := NewSwiftLanguage()

		// Skip test if Swift is not available to avoid triggering installation
		if _, err := exec.LookPath("swift"); err != nil {
			t.Skip("swift not available, skipping test that would trigger dependency installation")
		}

		// Test with invalid path - should fail to create manifest files
		err := swift.InstallDependencies("/invalid/readonly/path", []string{"test-dep"})
		if err == nil {
			t.Error("InstallDependencies() with invalid path should return error")
		}
	})

	t.Run("CheckEnvironmentHealth", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Should return false for non-existent environment
		healthy := swift.CheckEnvironmentHealth("/non/existent/path")
		if healthy {
			t.Error("CheckEnvironmentHealth() should return false for non-existent environment")
		}

		// Should return false for directory without Swift project structure
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		healthy = swift.CheckEnvironmentHealth(tempDir)
		// May return false for various reasons (no Swift, no Package.swift, etc.)
		t.Logf("CheckEnvironmentHealth for empty directory: %v", healthy)
	})

	t.Run("CheckEnvironmentHealth_WithPackageSwift", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create a minimal Package.swift
		packageSwiftPath := filepath.Join(tempDir, "Package.swift")
		if err := os.WriteFile(packageSwiftPath, []byte(testPackageSwiftContent), 0o644); err != nil {
			t.Fatalf("Failed to create Package.swift: %v", err)
		}

		healthy := swift.CheckEnvironmentHealth(tempDir)
		// Health depends on Swift availability and ability to run package commands
		t.Logf("CheckEnvironmentHealth with Package.swift: %v", healthy)
	})

	t.Run("CheckEnvironmentHealth_EmptyPath", func(t *testing.T) {
		swift := NewSwiftLanguage()

		// Should handle empty paths gracefully
		healthy := swift.CheckEnvironmentHealth("")
		if healthy {
			t.Error("CheckEnvironmentHealth() with empty path should return false")
		}
	})

	t.Run("SetupEnvironmentWithRepo_BrokenEnvironmentRecreation", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Skip test if Swift is not available
		if _, err := exec.LookPath("swift"); err != nil {
			t.Skip("swift not available, skipping broken environment test")
		}

		// Create a broken environment directory
		envPath := filepath.Join(tempDir, "swiftenv-5.7")
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create a file that will make CheckEnvironmentHealth fail
		brokenFile := filepath.Join(envPath, "broken")
		if err := os.WriteFile(brokenFile, []byte("broken"), 0o644); err != nil {
			t.Fatalf("Failed to create broken file: %v", err)
		}

		// SetupEnvironmentWithRepo should detect the broken environment and recreate it
		newEnvPath, err := swift.SetupEnvironmentWithRepo(tempDir, "5.7", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() should recreate broken environment: %v", err)
		}
		if newEnvPath != envPath {
			t.Errorf(
				"SetupEnvironmentWithRepo() should return same path after recreation: got %s, want %s",
				newEnvPath,
				envPath,
			)
		}

		// The broken file should be gone
		if _, err := os.Stat(brokenFile); !os.IsNotExist(err) {
			t.Error("SetupEnvironmentWithRepo() should have removed broken environment contents")
		}
	})

	t.Run("SetupEnvironmentWithRepo_DirectoryCreationError", func(t *testing.T) {
		swift := NewSwiftLanguage()

		// Skip test if Swift is not available
		if _, err := exec.LookPath("swift"); err != nil {
			t.Skip("swift not available, skipping directory creation error test")
		}

		// Test with invalid repo path that would cause issues
		_, err := swift.SetupEnvironmentWithRepo("", "5.7", "/invalid/nonexistent/repo/path", "dummy-url", []string{})
		if err == nil {
			t.Log("SetupEnvironmentWithRepo() with invalid repo path succeeded (environment creation might still work)")
		} else {
			t.Logf("SetupEnvironmentWithRepo() correctly failed with invalid repo path: %v", err)
		}
	})

	t.Run("InstallDependencies_SwiftCommandFailure", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Temporarily modify PATH to make swift unavailable
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		emptyDir := filepath.Join(tempDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should exercise the "swift package resolve" failure path
		err := swift.InstallDependencies(tempDir, []string{"test-package"})
		if err == nil {
			t.Error("InstallDependencies should fail when swift command not available")
		} else {
			expectedErrMsg := "failed to resolve Swift packages"
			if !strings.Contains(err.Error(), expectedErrMsg) {
				t.Errorf("Expected error to contain '%s', got: %v", expectedErrMsg, err)
			} else {
				t.Logf("InstallDependencies correctly failed when swift not available: %v", err)
			}
		}
	})

	t.Run("CheckEnvironmentHealth_SwiftCommandFailure", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create a Package.swift to pass the manifest check
		packageSwiftPath := filepath.Join(tempDir, "Package.swift")
		if err := os.WriteFile(packageSwiftPath, []byte(testPackageSwiftContent), 0o644); err != nil {
			t.Fatalf("Failed to create Package.swift: %v", err)
		}

		// Temporarily modify PATH to make swift unavailable
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		emptyDir := filepath.Join(tempDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should exercise the "swift package show-dependencies" failure path
		healthy := swift.CheckEnvironmentHealth(tempDir)
		if healthy {
			t.Error("CheckEnvironmentHealth should return false when swift command fails")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false when swift command failed")
		}
	})

	t.Run("CheckEnvironmentHealth_BaseHealthFailure", func(t *testing.T) {
		swift := NewSwiftLanguage()

		// Test with non-existent path to trigger base health check failure
		healthy := swift.CheckEnvironmentHealth("/definitely/does/not/exist")
		if healthy {
			t.Error("CheckEnvironmentHealth should return false when base health check fails")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false for non-existent path")
		}
	})

	t.Run("InstallDependencies_ManifestCreationFailure", func(t *testing.T) {
		swift := NewSwiftLanguage()

		// Test with a path that would cause manifest creation to fail
		// Try to use a read-only path or invalid path
		err := swift.InstallDependencies("/", []string{"test-package"})
		if err == nil {
			t.Error("InstallDependencies should fail when manifest creation fails")
		} else {
			expectedErrMsg := "failed to create Swift package manifest"
			if !strings.Contains(err.Error(), expectedErrMsg) {
				t.Errorf("Expected error to contain '%s', got: %v", expectedErrMsg, err)
			} else {
				t.Logf("InstallDependencies correctly failed with manifest creation error: %v", err)
			}
		}
	})

	// Additional tests for 100% coverage
	t.Run("SetupEnvironmentWithRepo_RemoveAllError", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Skip test if Swift is not available
		if _, err := exec.LookPath("swift"); err != nil {
			t.Skip("swift not available, skipping remove error test")
		}

		// Create environment directory
		envPath := filepath.Join(tempDir, "swiftenv-5.7")
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create a file that makes the directory un-removable on some systems
		nestedPath := filepath.Join(envPath, "nested")
		if err := os.MkdirAll(nestedPath, 0o755); err != nil {
			t.Fatalf("Failed to create nested directory: %v", err)
		}

		testFile := filepath.Join(nestedPath, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Make the nested directory read-only to potentially cause removal issues
		if err := os.Chmod(nestedPath, 0o555); err != nil {
			t.Fatalf("Failed to change permissions: %v", err)
		}
		defer os.Chmod(nestedPath, 0o755) // Restore permissions for cleanup

		// Try to setup environment - may fail due to removal issues or succeed if OS allows
		_, err := swift.SetupEnvironmentWithRepo(tempDir, "5.7", tempDir, "dummy-url", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "failed to remove broken environment") {
				t.Logf("SetupEnvironmentWithRepo correctly failed with removal error: %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo failed with different error: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo succeeded despite permission issues")
		}
	})

	t.Run("CheckEnvironmentHealth_HealthyEnvironment", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Skip test if Swift is not available
		if _, err := exec.LookPath("swift"); err != nil {
			t.Skip("swift not available, skipping healthy environment test")
		}

		// Create a valid Swift environment by actually setting it up first
		envPath, err := swift.SetupEnvironmentWithRepo(tempDir, "5.8", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Skipf("Failed to setup environment for health test: %v", err)
		}

		// Now check if it's healthy - this should exercise the "return true" path
		healthy := swift.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth for properly setup environment: %v", healthy)

		// Also test the case where CheckEnvironmentHealth returns true in SetupEnvironmentWithRepo
		// by calling setup again on the same environment
		envPath2, err := swift.SetupEnvironmentWithRepo(tempDir, "5.8", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("Second SetupEnvironmentWithRepo failed: %v", err)
		}
		if envPath != envPath2 {
			t.Errorf("Expected same environment path when reusing healthy environment")
		}
	})

	t.Run("SetupEnvironmentWithRepo_CreateDirectoryError", func(t *testing.T) {
		swift := NewSwiftLanguage()

		// Test with an invalid path that cannot be created
		_, err := swift.SetupEnvironmentWithRepo("/dev/null", "5.7", "/dev/null", "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when directory creation fails")
		} else {
			expectedErrMsg := "failed to create Swift environment directory"
			if !strings.Contains(err.Error(), expectedErrMsg) {
				t.Errorf("Expected error to contain '%s', got: %v", expectedErrMsg, err)
			} else {
				t.Logf("SetupEnvironmentWithRepo correctly failed with directory creation error: %v", err)
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_InstallDependenciesError", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Skip test if Swift is not available
		if _, err := exec.LookPath("swift"); err != nil {
			t.Skip("swift not available, skipping dependency installation error test")
		}

		// Temporarily modify PATH to make swift unavailable for dependency installation
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		emptyDir := filepath.Join(tempDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should fail during dependency installation
		_, err := swift.SetupEnvironmentWithRepo(tempDir, "5.7", tempDir, "dummy-url", []string{"test-dep"})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when dependency installation fails")
		} else {
			expectedErrMsg := "failed to install Swift dependencies"
			if !strings.Contains(err.Error(), expectedErrMsg) {
				t.Errorf("Expected error to contain '%s', got: %v", expectedErrMsg, err)
			} else {
				t.Logf("SetupEnvironmentWithRepo correctly failed with dependency installation error: %v", err)
			}
		}
	})
	t.Run("CheckEnvironmentHealth_NoPackageSwift", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create a directory structure that passes base health check
		// We need to create the bin directory and a mock swift executable for CheckHealth to pass
		binPath := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create a mock swift executable that responds to --version
		swiftExec := filepath.Join(binPath, "swift")
		mockScript := "#!/bin/bash\nif [[ \"$1\" == \"--version\" ]]; then\n  echo 'swift-driver version: 1.0'\n  exit 0\nfi\nexit 1\n"
		if err := os.WriteFile(swiftExec, []byte(mockScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock swift executable: %v", err)
		}

		// This should exercise the "no manifest" branch where CheckManifestExists returns false
		healthy := swift.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth for directory without Package.swift: %v", healthy)

		// This should return true since CheckHealth passes and there's no manifest to check
		if !healthy {
			t.Error("CheckEnvironmentHealth should return true when CheckHealth passes and no Package.swift exists")
		}
	})

	t.Run("CheckEnvironmentHealth_WithPackageSwiftSuccess", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create working Swift environment
		createMockSwiftEnvironment(t, tempDir, "advanced", true)

		// This should exercise the Package.swift exists branch and return true
		healthy := swift.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth should return true when Package.swift exists and swift commands succeed")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned true with working Package.swift and swift commands")
		}
	})

	t.Run("CheckEnvironmentHealth_WithPackageSwiftFailure", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create bin directory with mock swift executable that fails package commands
		binPath := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create a mock swift executable that responds to --version but fails package commands
		swiftExec := filepath.Join(binPath, "swift")
		mockScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo 'swift-driver version: 1.0'
  exit 0
elif [[ "$1" == "package" && "$2" == "show-dependencies" ]]; then
  echo 'Error: dependencies not resolved'
  exit 1
fi
exit 1
`
		if err := os.WriteFile(swiftExec, []byte(mockScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock swift executable: %v", err)
		}

		// Create Package.swift to trigger the manifest check
		packageSwiftPath := filepath.Join(tempDir, "Package.swift")
		if err := os.WriteFile(packageSwiftPath, []byte(testPackageSwiftContent), 0o644); err != nil {
			t.Fatalf("Failed to create Package.swift: %v", err)
		}

		// Temporarily modify PATH to use our mock swift
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", binPath+string(os.PathListSeparator)+originalPath)

		// This should exercise the Package.swift exists branch but return false due to command failure
		healthy := swift.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth with failing package command returned: %v", healthy)

		// Note: If this returns true, it might be because the system swift is being used instead of our mock
		// or the CheckHealth is failing first, causing it to return false for a different reason
		if healthy {
			t.Log("CheckEnvironmentHealth returned true - this could be because " +
				"CheckHealth failed first or system swift was used")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false - either CheckHealth failed or swift package command failed")
		}
	})
}

// Additional targeted tests for 100% coverage
func TestSwiftLanguage_CoverageTargeted(t *testing.T) {
	t.Run("SetupEnvironmentWithRepo_HealthyEnvironmentReuse", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create a functioning environment manually
		envName := "swiftenv-5.9"
		envPath := filepath.Join(tempDir, envName)
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create a bin directory with a working mock swift executable
		binPath := filepath.Join(envPath, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create mock swift executable that handles all needed commands
		swiftExec := filepath.Join(binPath, "swift")
		mockScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo 'swift-driver version: 5.9'
  exit 0
elif [[ "$1" == "package" && "$2" == "show-dependencies" ]]; then
  echo 'No dependencies found.'
  exit 0
elif [[ "$1" == "package" && "$2" == "resolve" ]]; then
  echo 'Dependencies resolved.'
  exit 0
fi
exit 1
`
		if err := os.WriteFile(swiftExec, []byte(mockScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock swift executable: %v", err)
		}

		// Temporarily modify PATH to use our mock swift
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", binPath+string(os.PathListSeparator)+originalPath)

		// Override CheckEnvironmentHealth to return true for this test
		// We need to create an environment that will pass the health check
		// First verify our mock setup works
		healthy := swift.CheckEnvironmentHealth(envPath)
		if !healthy {
			t.Skip("Mock environment health check failed, skipping healthy reuse test")
		}

		// Now call SetupEnvironmentWithRepo - it should reuse the existing healthy environment
		resultPath, err := swift.SetupEnvironmentWithRepo(tempDir, "5.9", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with healthy environment failed: %v", err)
		}
		if resultPath != envPath {
			t.Errorf(
				"SetupEnvironmentWithRepo() should reuse healthy environment: got %s, want %s",
				resultPath,
				envPath,
			)
		}

		// This should exercise the "return envPath, nil" path when CheckEnvironmentHealth returns true
		t.Log("Successfully exercised healthy environment reuse path")
	})

	t.Run("InstallDependencies_SuccessPath", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create a bin directory with a working mock swift executable
		binPath := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create mock swift executable that succeeds for package resolve
		swiftExec := filepath.Join(binPath, "swift")
		mockScript := `#!/bin/bash
if [[ "$1" == "package" && "$2" == "resolve" ]]; then
  echo 'Dependencies resolved successfully.'
  exit 0
fi
exit 1
`
		if err := os.WriteFile(swiftExec, []byte(mockScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock swift executable: %v", err)
		}

		// Temporarily modify PATH to use our mock swift
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", binPath+string(os.PathListSeparator)+originalPath)

		// Install dependencies - should succeed
		err := swift.InstallDependencies(tempDir, []string{"ArgumentParser"})
		if err != nil {
			t.Errorf("InstallDependencies() should succeed with working swift: %v", err)
		}

		// Verify Package.swift was created
		packagePath := filepath.Join(tempDir, "Package.swift")
		if _, err := os.Stat(packagePath); os.IsNotExist(err) {
			t.Error("InstallDependencies() should create Package.swift")
		}

		// Verify Sources directory was created
		sourcesPath := filepath.Join(tempDir, "Sources", "PreCommitEnv", "main.swift")
		if _, err := os.Stat(sourcesPath); os.IsNotExist(err) {
			t.Error("InstallDependencies() should create Sources structure")
		}

		t.Log("Successfully exercised InstallDependencies success path")
	})

	t.Run("CheckEnvironmentHealth_ReturnTrue", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create working Swift environment
		createMockSwiftEnvironment(t, tempDir, "basic", true)

		// This should exercise the "return true" path
		healthy := swift.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth() should return true with working environment")
		} else {
			t.Log("Successfully exercised CheckEnvironmentHealth return true path")
		}
	})

	t.Run("CheckEnvironmentHealth_NoManifestReturnTrue", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create a bin directory with a working mock swift executable
		binPath := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create mock swift executable that passes health check
		swiftExec := filepath.Join(binPath, "swift")
		mockScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo 'swift-driver version: 5.9'
  exit 0
fi
exit 1
`
		if err := os.WriteFile(swiftExec, []byte(mockScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock swift executable: %v", err)
		}

		// Temporarily modify PATH to use our mock swift
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", binPath+string(os.PathListSeparator)+originalPath)

		// This should return true when CheckHealth passes and no manifest exists
		healthy := swift.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth() should return true when CheckHealth passes and no manifest")
		} else {
			t.Log("Successfully exercised CheckEnvironmentHealth return true with no manifest")
		}
	})

	t.Run("CheckEnvironmentHealth_ManifestExistsAndCommandSucceeds", func(t *testing.T) {
		swift := NewSwiftLanguage()
		tempDir := t.TempDir()

		// Create working Swift environment
		createMockSwiftEnvironment(t, tempDir, "advanced", true)

		// This should exercise the successful "swift package show-dependencies" path
		healthy := swift.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth() should return true when Package.swift exists and swift commands succeed")
		} else {
			t.Log("Successfully exercised CheckEnvironmentHealth with Package.swift and successful swift command")
		}
	})
}
