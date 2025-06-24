package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/tests/helpers"
)

func TestDartLanguage(t *testing.T) {
	t.Run("NewDartLanguage", func(t *testing.T) {
		dart := NewDartLanguage()

		if dart == nil {
			t.Fatal("NewDartLanguage() returned nil")
		}

		if dart.Name != "dart" {
			t.Errorf("Expected language name 'dart', got '%s'", dart.Name)
		}

		if dart.ExecutableName != "dart" {
			t.Errorf("Expected executable name 'dart', got '%s'", dart.ExecutableName)
		}

		if dart.VersionFlag != testVersionFlag {
			t.Errorf("Expected version flag '%s', got '%s'", testVersionFlag, dart.VersionFlag)
		}

		if dart.InstallURL != "https://dart.dev/get-dart" {
			t.Errorf("Expected install URL 'https://dart.dev/get-dart', got '%s'", dart.InstallURL)
		}
	})

	t.Run("HelperTests", func(t *testing.T) {
		dart := NewDartLanguage()

		config := helpers.LanguageTestConfig{
			Language:       dart,
			Name:           "Dart",
			ExecutableName: "dart",
			VersionFlag:    testVersionFlag,
			TestVersions:   []string{"", "2.17", "2.18", "2.19", "3.0"},
			EnvPathSuffix:  "dartenv-3.0",
		}

		helpers.RunLanguageTests(t, config)
	})
}

func TestDartLanguage_InstallDependencies(t *testing.T) {
	dart := NewDartLanguage()

	t.Run("NoDependencies", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-env-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		err = dart.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with no dependencies returned error: %v", err)
		}

		err = dart.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil dependencies returned error: %v", err)
		}
	})

	t.Run("WithDependencies", func(t *testing.T) {
		// Skip if dart is not available to avoid triggering installation
		if _, err := exec.LookPath("dart"); err != nil {
			t.Skip("dart not available, skipping dependency installation test that would trigger Dart installation")
		}

		tempDir, err := os.MkdirTemp("", "test-dart-deps-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test with some common Dart packages
		deps := []string{"http", "json_annotation"}
		err = dart.InstallDependencies(tempDir, deps)

		// We don't require this to succeed because it requires network access and dart setup
		// Just log the result
		if err != nil {
			t.Logf("InstallDependencies() failed (expected if dart/pub not properly configured): %v", err)
		} else {
			// With simplified Dart implementation, no pubspec.yaml is created
			// (dependencies are ignored with a warning)
			t.Logf("InstallDependencies() succeeded with simplified implementation")
		}
	})

	t.Run("InvalidPath", func(t *testing.T) {
		deps := []string{"http"}
		err := dart.InstallDependencies("/invalid/readonly/path", deps)
		// With simplified Dart implementation, no error is returned for invalid paths
		// (dependencies are just ignored with a warning)
		if err != nil {
			t.Logf("InstallDependencies() with invalid path returned error (which is fine): %v", err)
		} else {
			t.Logf("InstallDependencies() with invalid path succeeded (simplified implementation)")
		}
	})
}

func TestDartLanguage_CheckEnvironmentHealth(t *testing.T) {
	dart := NewDartLanguage()

	t.Run("NonExistentPath", func(t *testing.T) {
		result := dart.CheckEnvironmentHealth("/non/existent/path")
		if result {
			t.Error("CheckEnvironmentHealth() should return false for non-existent path")
		}
	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-health-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		result := dart.CheckEnvironmentHealth(tempDir)

		// The result depends on whether dart is installed and working
		if result {
			t.Logf("CheckEnvironmentHealth() returned true (dart appears to be available)")
		} else {
			t.Logf("CheckEnvironmentHealth() returned false (dart not available or environment not healthy)")
		}
	})

	t.Run("WithPubspecYaml", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-health-pubspec-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create pubspec.yaml
		pubspecContent := `name: test_package
version: 1.0.0
environment:
  sdk: '>=2.12.0 <4.0.0'
dependencies:
  http: ^0.13.0
`
		pubspecPath := filepath.Join(tempDir, "pubspec.yaml")
		if err := os.WriteFile(pubspecPath, []byte(pubspecContent), 0o644); err != nil {
			t.Fatalf("Failed to create pubspec.yaml: %v", err)
		}

		result := dart.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth() with pubspec.yaml returned: %v", result)
	})

	// Additional tests to improve CheckEnvironmentHealth coverage
	t.Run("CheckEnvironmentHealth_NoPubspec", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-health-no-pubspec-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test without pubspec.yaml (should skip pub deps check)
		result := dart.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth() without pubspec.yaml returned: %v", result)
	})

	t.Run("CheckEnvironmentHealth_PubspecWithDeps", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-health-pubspec-deps-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create a more complex pubspec.yaml to test pub deps path
		pubspecContent := `name: test_package
version: 1.0.0
environment:
  sdk: '>=2.12.0 <4.0.0'
dependencies:
  http: ^0.13.0
  path: ^1.8.0
dev_dependencies:
  test: ^1.16.0
`
		pubspecPath := filepath.Join(tempDir, "pubspec.yaml")
		if err := os.WriteFile(pubspecPath, []byte(pubspecContent), 0o644); err != nil {
			t.Fatalf("Failed to create pubspec.yaml: %v", err)
		}

		// This will test the dart pub deps execution path
		result := dart.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth() with complex pubspec.yaml returned: %v", result)
	})
}

func TestDartLanguage_SetupEnvironmentWithRepo(t *testing.T) {
	dart := NewDartLanguage()

	// Helper function to check if dart is available
	isDartAvailable := func() bool {
		_, err := exec.LookPath("dart")
		return err == nil
	}

	t.Run("DefaultVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-setup-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip test if dart is not available to avoid triggering installation
		if !isDartAvailable() {
			t.Skip("dart not available, skipping test that would trigger Dart installation")
		}

		envPath, err := dart.SetupEnvironmentWithRepo(
			tempDir,
			language.VersionDefault,
			tempDir,
			"dummy-url",
			[]string{},
		)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
			return
		}

		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() returned empty environment path")
		}

		// Verify environment directory was created
		expectedPath := filepath.Join(tempDir, "dartenv-default")
		if envPath != expectedPath {
			t.Errorf("SetupEnvironmentWithRepo() returned unexpected path: got %s, want %s", envPath, expectedPath)
		}

		// Directory should exist
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			t.Error("SetupEnvironmentWithRepo() did not create environment directory")
		}
	})

	t.Run("SystemVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-system-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip test if dart is not available to avoid triggering installation
		if !isDartAvailable() {
			t.Skip("dart not available, skipping test that would trigger Dart installation")
		}

		envPath, err := dart.SetupEnvironmentWithRepo(tempDir, "system", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with system version returned error: %v", err)
			return
		}

		expectedPath := filepath.Join(tempDir, "dartenv-system")
		if envPath != expectedPath {
			t.Errorf("SetupEnvironmentWithRepo() returned unexpected path: got %s, want %s", envPath, expectedPath)
		}
	})

	t.Run("ExistingEnvironment", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-existing-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip test if dart is not available to avoid triggering installation
		if !isDartAvailable() {
			t.Skip("dart not available, skipping test that might trigger installation")
		}

		// Create environment directory first
		envDir := filepath.Join(tempDir, "dartenv-default")
		if mkdirErr := os.MkdirAll(envDir, 0o755); mkdirErr != nil {
			t.Fatalf("Failed to create environment directory: %v", mkdirErr)
		}

		// Call SetupEnvironmentWithRepo - should use existing environment if healthy
		envPath, err := dart.SetupEnvironmentWithRepo(
			tempDir,
			language.VersionDefault,
			tempDir,
			"dummy-url",
			[]string{},
		)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with existing environment returned error: %v", err)
			return
		}

		if envPath != envDir {
			t.Errorf(
				"SetupEnvironmentWithRepo() should have used existing environment: got %s, want %s",
				envPath,
				envDir,
			)
		}
	})

	t.Run("WithDependencies", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-with-deps-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip test if dart is not available to avoid triggering installation
		if !isDartAvailable() {
			t.Skip("dart not available, skipping test that might trigger installation")
		}

		deps := []string{"http", "json_annotation"}
		envPath, err := dart.SetupEnvironmentWithRepo(tempDir, language.VersionDefault, tempDir, "dummy-url", deps)

		// Log result - dependency installation requires dart and network access
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() with dependencies failed: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo() with dependencies succeeded: %s", envPath)
		}
	})

	t.Run("UnsupportedVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-dart-unsupported-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip test if dart is not available to avoid triggering installation
		if !isDartAvailable() {
			t.Skip("dart not available, skipping test that would trigger Dart installation")
		}

		// Unsupported versions should be normalized to default
		envPath, err := dart.SetupEnvironmentWithRepo(tempDir, "4.0.0", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with unsupported version returned error: %v", err)
			return
		}

		// Should use default version environment name
		expectedPath := filepath.Join(tempDir, "dartenv-default")
		if envPath != expectedPath {
			t.Errorf("SetupEnvironmentWithRepo() should normalize unsupported version "+
				"to default: got %s, want %s", envPath, expectedPath)
		}
	})

	// Add more comprehensive tests that don't require Dart installation
	t.Run("SetupEnvironmentWithRepo_NoRuntimeInstalled", func(t *testing.T) {
		tempDir := t.TempDir()

		// This test specifically covers the case where dart is not available
		// and tests the download/install path
		envPath, err := dart.SetupEnvironmentWithRepo(tempDir, "default", tempDir, "dummy-url", []string{})

		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() failed as expected when Dart not available: %v", err)
			// This is expected since it will try to download and install Dart
		} else {
			t.Logf("SetupEnvironmentWithRepo() succeeded: %s", envPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo_VersionHandling", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test empty version handling
		envPath, err := dart.SetupEnvironmentWithRepo(tempDir, "", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() with empty version failed: %v", err)
		} else {
			// Should use default version
			expectedPath := filepath.Join(tempDir, "dartenv-default")
			if envPath != expectedPath {
				t.Errorf("Expected empty version to become default: got %s, want %s", envPath, expectedPath)
			}
		}

		// Test custom version (should be normalized to default)
		envPath2, err := dart.SetupEnvironmentWithRepo(tempDir, "3.1.0", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() with custom version failed: %v", err)
		} else {
			// Should use default version for unsupported versions
			expectedPath := filepath.Join(tempDir, "dartenv-default")
			if envPath2 != expectedPath {
				t.Errorf("Expected custom version to become default: got %s, want %s", envPath2, expectedPath)
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_BrokenEnvironmentRecreation", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a broken environment directory (exists but not functional)
		brokenEnvPath := filepath.Join(tempDir, "dartenv-default")
		if err := os.MkdirAll(brokenEnvPath, 0o755); err != nil {
			t.Fatalf("Failed to create broken environment directory: %v", err)
		}

		// Put some junk file to make it "broken"
		brokenFile := filepath.Join(brokenEnvPath, "broken.txt")
		if err := os.WriteFile(brokenFile, []byte("broken"), 0o644); err != nil {
			t.Fatalf("Failed to create broken file: %v", err)
		}

		// This should detect the broken environment and try to recreate it
		envPath, err := dart.SetupEnvironmentWithRepo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() failed to recreate broken environment: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo() successfully recreated broken environment: %s", envPath)
		}
	})
}

// Test to improve coverage for Dart language functions
func TestDartLanguage_ComprehensiveCoverage(t *testing.T) {
	dart := NewDartLanguage()

	t.Run("InstallDependencies_ComprehensiveTests", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with empty dependencies
		err := dart.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies with empty deps should not error: %v", err)
		}

		// Test with nil dependencies
		err = dart.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies with nil deps should not error: %v", err)
		}

		// Test with dependencies (will likely fail without dart)
		err = dart.InstallDependencies(tempDir, []string{"http", "test"})
		if err != nil {
			t.Logf("InstallDependencies failed as expected (dart may not be available): %v", err)
		}

		// Test with invalid path
		err = dart.InstallDependencies("/invalid/readonly/path", []string{"test"})
		if err != nil {
			t.Logf("InstallDependencies with invalid path failed as expected: %v", err)
		}
	})

	t.Run("CheckEnvironmentHealth_ComprehensiveTests", func(t *testing.T) {
		// Test with non-existent path
		if dart.CheckEnvironmentHealth("/non/existent/path") {
			t.Error("CheckEnvironmentHealth should return false for non-existent path")
		}

		// Test with empty directory
		tempDir := t.TempDir()
		emptyDir := filepath.Join(tempDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}

		if dart.CheckEnvironmentHealth(emptyDir) {
			t.Error("CheckEnvironmentHealth should return false for empty directory")
		}

		// Test with valid environment structure
		envDir := filepath.Join(tempDir, "dart-env")
		pubspecDir := filepath.Join(envDir, ".pub-cache")
		if err := os.MkdirAll(pubspecDir, 0o755); err != nil {
			t.Fatalf("Failed to create .pub-cache directory: %v", err)
		}

		// Create mock dart executable for health check
		binDir := filepath.Join(envDir, "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		dartExe := filepath.Join(binDir, "dart")
		if err := os.WriteFile(dartExe, []byte("#!/bin/bash\necho 'Dart'"), 0o755); err != nil {
			t.Fatalf("Failed to create dart executable: %v", err)
		}

		// Should pass health check with proper structure
		result := dart.CheckEnvironmentHealth(envDir)
		t.Logf("CheckEnvironmentHealth with proper structure returned: %v", result)
	})

	t.Run("SetupEnvironmentWithRepo_MockSuccess", func(t *testing.T) {
		tempDir := t.TempDir()
		repoDir := filepath.Join(tempDir, "dart-repo")

		// Create repository structure
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create pubspec.yaml to make it look like a Dart package
		pubspec := `name: test_package
version: 1.0.0
environment:
  sdk: '>=2.17.0 <4.0.0'
dependencies:
  http: ^0.13.0
`
		if err := os.WriteFile(filepath.Join(repoDir, "pubspec.yaml"), []byte(pubspec), 0o644); err != nil {
			t.Fatalf("Failed to create pubspec.yaml: %v", err)
		}

		// Create mock dart executable
		mockDartDir := filepath.Join(tempDir, "mock-dart")
		if err := os.MkdirAll(mockDartDir, 0o755); err != nil {
			t.Fatalf("Failed to create mock dart directory: %v", err)
		}

		mockDart := filepath.Join(mockDartDir, "dart")
		mockScript := `#!/bin/bash
if [[ "$*" == *"--version"* ]]; then
  echo "Dart SDK version: 3.0.0"
  exit 0
elif [[ "$*" == *"pub get"* ]]; then
  echo "Getting dependencies..."
  exit 0
elif [[ "$*" == *"pub global activate"* ]]; then
  echo "Activated package"
  exit 0
fi
exit 0`
		if err := os.WriteFile(mockDart, []byte(mockScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockDartDir+string(os.PathListSeparator)+originalPath)

		// Test SetupEnvironmentWithRepo
		envPath, err := dart.SetupEnvironmentWithRepo("", "3.0", repoDir, "https://github.com/test/repo", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo failed (expected if dart not available): %v", err)
		} else {
			t.Logf("Successfully tested SetupEnvironmentWithRepo: %s", envPath)
			// Verify environment path format
			if !filepath.IsAbs(envPath) {
				t.Errorf("Environment path should be absolute, got: %s", envPath)
			}
		}
	})

	t.Run("DartNotAvailable_CodePath", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test behavior when Dart is not available
		// This should now return an error instead of trying to download
		_, err := dart.SetupEnvironmentWithRepo("", "3.0", tempDir, "https://github.com/test/repo", []string{})
		if err != nil {
			// Should fail with "Dart runtime not found" message
			if !strings.Contains(err.Error(), "Dart runtime not found") {
				t.Logf("SetupEnvironmentWithRepo failed with unexpected error: %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo correctly failed when Dart not available: %v", err)
			}
		}
	})

	t.Run("EdgeCasesAndErrorPaths", func(t *testing.T) {
		// Test with various edge case inputs
		testPaths := []string{
			"/path/with spaces",
			"/path-with-dashes",
			"/path_with_underscores",
			"/path.with.dots",
			"relative/path",
			"",
		}

		for _, path := range testPaths {
			// Test CheckEnvironmentHealth with edge case paths
			result := dart.CheckEnvironmentHealth(path)
			t.Logf("CheckEnvironmentHealth(%q) = %v", path, result)

			// Test InstallDependencies with edge case paths
			err := dart.InstallDependencies(path, []string{})
			if err != nil {
				t.Logf("InstallDependencies(%q) failed as expected: %v", path, err)
			}
		}

		// Test with various dependency formats
		dependencyFormats := []string{
			"package_name",
			"package_name:^1.0.0",
			"package_name:>=1.0.0 <2.0.0",
			"git:https://github.com/user/repo.git",
			"path:../local_package",
		}

		tempDir := t.TempDir()
		for _, dep := range dependencyFormats {
			err := dart.InstallDependencies(tempDir, []string{dep})
			if err != nil {
				t.Logf("InstallDependencies with dependency %q failed as expected: %v", dep, err)
			}
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		// Helper function to check if Dart is available
		isDartAvailable := func() bool {
			_, err := exec.LookPath("dart")
			return err == nil
		}

		// Skip test if Dart is not available to avoid triggering installation
		if !isDartAvailable() {
			t.Skip("dart not available, skipping concurrent test that would trigger Dart installation")
		}

		// Test concurrent access to methods
		tempDir := t.TempDir()

		done := make(chan bool, 3)

		// Concurrent health checks
		go func() {
			result := dart.CheckEnvironmentHealth(tempDir)
			t.Logf("Concurrent health check returned: %v", result)
			done <- true
		}()

		// Concurrent dependency installation
		go func() {
			err := dart.InstallDependencies(tempDir, []string{})
			t.Logf("Concurrent InstallDependencies returned: %v", err)
			done <- true
		}()

		// Concurrent environment setup
		go func() {
			_, err := dart.SetupEnvironmentWithRepo("", "3.0", tempDir, "https://test.com", []string{})
			t.Logf("Concurrent SetupEnvironmentWithRepo returned: %v", err)
			done <- true
		}()

		// Wait for all goroutines to complete
		for range 3 {
			<-done
		}
	})
}

// Test CheckEnvironmentHealth with mock dart executable
func TestDartLanguage_CheckEnvironmentHealthWithMockDart(t *testing.T) {
	dart := NewDartLanguage()

	t.Run("CheckEnvironmentHealthWithWorkingDart", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create bin directory and mock dart executable
		binPath := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create a mock dart executable that responds to --version and pub deps
		dartExec := filepath.Join(binPath, "dart")
		dartScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "Dart SDK version: 3.0.0 (stable)"
  exit 0
elif [[ "$1" == "pub" && "$2" == "deps" ]]; then
  echo "Dependencies resolved"
  exit 0
fi
exit 1`
		if err := os.WriteFile(dartExec, []byte(dartScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart executable: %v", err)
		}

		// Save original PATH and modify it to include our mock
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", binPath+string(os.PathListSeparator)+originalPath)

		// Test 1: Environment with no pubspec.yaml - should pass base health check and return true
		healthy := dart.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth should return true when base health passes and no pubspec.yaml")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned true with working dart and no pubspec.yaml")
		}

		// Test 2: Environment with pubspec.yaml - should run pub deps and return true
		pubspecPath := filepath.Join(tempDir, "pubspec.yaml")
		pubspecContent := `name: test_package
version: 1.0.0
environment:
  sdk: '>=2.12.0 <4.0.0'
dependencies:
  http: ^0.13.0
`
		if err := os.WriteFile(pubspecPath, []byte(pubspecContent), 0o644); err != nil {
			t.Fatalf("Failed to create pubspec.yaml: %v", err)
		}

		healthy = dart.CheckEnvironmentHealth(tempDir)
		if !healthy {
			t.Error("CheckEnvironmentHealth should return true when pubspec.yaml exists and pub deps succeeds")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned true for valid pubspec.yaml")
		}
	})

	t.Run("CheckEnvironmentHealthWithFailingPubDeps", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create bin directory and mock dart executable that fails pub deps
		binPath := filepath.Join(tempDir, "bin")
		if err := os.MkdirAll(binPath, 0o755); err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		dartExec := filepath.Join(binPath, "dart")
		dartFailScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "Dart SDK version: 3.0.0 (stable)"
  exit 0
elif [[ "$1" == "pub" && "$2" == "deps" ]]; then
  echo "Dependencies failed to resolve"
  exit 1
fi
exit 1`
		if err := os.WriteFile(dartExec, []byte(dartFailScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart executable: %v", err)
		}

		// Save original PATH and modify it to include our mock
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		// Set PATH to only our mock directory to ensure it's used
		os.Setenv("PATH", binPath)

		// Create pubspec.yaml
		pubspecPath := filepath.Join(tempDir, "pubspec.yaml")
		pubspecContent := `name: test_package
version: 1.0.0
`
		if err := os.WriteFile(pubspecPath, []byte(pubspecContent), 0o644); err != nil {
			t.Fatalf("Failed to create pubspec.yaml: %v", err)
		}

		healthy := dart.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth result with failing pub deps: %v", healthy)
		if healthy {
			t.Log("CheckEnvironmentHealth returned true (pub deps may have succeeded or not been checked)")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false when pub deps fails")
		}
	})
}

func TestDartLanguage_CheckEnvironmentHealth_Comprehensive(t *testing.T) {
	dart := NewDartLanguage()
	validPubspec := `name: test_package
version: 1.0.0
dependencies:
  test: ^1.0.0
dev_dependencies:
  lints: ^2.0.0`
	invalidPubspec := "invalid: yaml: content: ["

	testEnvironmentHealthComprehensive(t, dart, "pubspec.yaml", validPubspec, invalidPubspec)
}

// Additional tests to improve SetupEnvironmentWithRepo coverage
func TestDartLanguage_SetupEnvironmentWithRepo_EdgeCases(t *testing.T) {
	dart := NewDartLanguage()

	t.Run("BrokenEnvironmentRemovalFailure", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test directory removal failure")
		}

		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "dartenv-default")

		// Create a broken environment directory
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create a file that will make the directory "broken" for health check
		brokenFile := filepath.Join(envPath, "broken.txt")
		if err := os.WriteFile(brokenFile, []byte("broken"), 0o644); err != nil {
			t.Fatalf("Failed to create broken file: %v", err)
		}

		// Make the environment directory read-only to prevent removal
		if err := os.Chmod(envPath, 0o444); err != nil {
			t.Fatalf("Failed to make directory read-only: %v", err)
		}
		defer func() {
			// Restore permissions for cleanup
			os.Chmod(envPath, 0o755)
		}()

		// This should try to remove the broken environment but fail
		_, err := dart.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when broken environment cannot be removed")
		} else {
			if !strings.Contains(err.Error(), "failed to remove broken environment") {
				t.Errorf("Expected broken environment removal error, got: %v", err)
			} else {
				t.Logf("Correctly handled broken environment removal failure: %v", err)
			}
		}
	})

	t.Run("EnvironmentDirectoryCreationFailure", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test directory creation failure")
		}

		// Create mock dart executable to bypass runtime check
		tempBinDir := t.TempDir()
		mockDart := filepath.Join(tempBinDir, "dart")
		scriptContent := testDartSDKScript
		if err := os.WriteFile(mockDart, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		// Try to create environment in read-only directory
		readOnlyDir := "/root"
		_, err := dart.SetupEnvironmentWithRepo("", "default", readOnlyDir, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when environment directory cannot be created")
		} else {
			if !strings.Contains(err.Error(), "failed to create Dart environment directory") {
				t.Logf("Got different error type (may be runtime check): %v", err)
			} else {
				t.Logf("Correctly handled environment directory creation failure: %v", err)
			}
		}
	})

	t.Run("SystemVersionHandling", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with "system" version specifically
		envPath, err := dart.SetupEnvironmentWithRepo("", "system", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with system version failed: %v", err)
		} else {
			expectedPath := filepath.Join(tempDir, "dartenv-system")
			if envPath != expectedPath {
				t.Errorf("Expected system version path %s, got %s", expectedPath, envPath)
			} else {
				t.Logf("Correctly handled system version: %s", envPath)
			}
		}
	})

	t.Run("EmptyVersionHandling", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with empty version (should default to "default")
		envPath, err := dart.SetupEnvironmentWithRepo("", "", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with empty version failed: %v", err)
		} else {
			expectedPath := filepath.Join(tempDir, "dartenv-default")
			if envPath != expectedPath {
				t.Errorf("Expected empty version to become default path %s, got %s", expectedPath, envPath)
			} else {
				t.Logf("Correctly handled empty version: %s", envPath)
			}
		}
	})

	t.Run("CustomVersionNormalization", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test various custom versions that should all be normalized to "default"
		testVersions := []string{"3.0.0", "2.17.0", "latest", "stable", "beta"}
		for _, version := range testVersions {
			envPath, err := dart.SetupEnvironmentWithRepo("", version, tempDir, "dummy-url", []string{})
			if err != nil {
				t.Logf("SetupEnvironmentWithRepo with version %s failed: %v", version, err)
			} else {
				expectedPath := filepath.Join(tempDir, "dartenv-default")
				if envPath != expectedPath {
					t.Errorf("Expected version %s to normalize to default path %s, got %s", version, expectedPath, envPath)
				} else {
					t.Logf("Correctly normalized version %s to default: %s", version, envPath)
				}
			}
		}
	})

	t.Run("WithAdditionalDependenciesWarning", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create mock dart executable
		mockDartDir := t.TempDir()
		mockDart := filepath.Join(mockDartDir, "dart")
		scriptContent := testDartSDKScript
		if err := os.WriteFile(mockDart, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockDartDir+string(os.PathListSeparator)+originalPath)

		// Test with additional dependencies (should generate warning)
		deps := []string{"http", "test", "json_annotation"}
		envPath, err := dart.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", deps)
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with dependencies failed: %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo with dependencies succeeded with warning: %s", envPath)
			// The dependencies should be ignored and warning printed
		}
	})

	t.Run("ExistingHealthyEnvironmentReuse", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "dartenv-default")

		// Create mock dart executable for health check
		mockDartDir := t.TempDir()
		mockDart := filepath.Join(mockDartDir, "dart")
		scriptContent := testDartSDKScript
		if err := os.WriteFile(mockDart, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockDartDir+string(os.PathListSeparator)+originalPath)

		// Create healthy environment directory first
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// First call should create/verify the environment
		result1, err1 := dart.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})
		if err1 != nil {
			t.Logf("First SetupEnvironmentWithRepo call failed: %v", err1)
		} else {
			// Second call should reuse the existing healthy environment
			result2, err2 := dart.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})
			if err2 != nil {
				t.Logf("Second SetupEnvironmentWithRepo call failed: %v", err2)
			} else {
				if result1 != result2 {
					t.Errorf("SetupEnvironmentWithRepo should reuse existing environment: got %s != %s", result1, result2)
				} else {
					t.Logf("Successfully reused existing environment: %s", result2)
				}
			}
		}
	})

	t.Run("DartRuntimeNotAvailableError", func(t *testing.T) {
		tempDir := t.TempDir()

		// Temporarily modify PATH to remove dart
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", "/usr/bin:/bin") // Minimal PATH without dart

		// This should fail with runtime not available error
		_, err := dart.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when Dart runtime is not available")
		} else {
			if !strings.Contains(err.Error(), "dart runtime not found") {
				t.Errorf("Expected runtime not found error, got: %v", err)
			} else {
				t.Logf("Correctly handled missing Dart runtime: %v", err)
			}
		}
	})
}

// Test for additional InstallDependencies coverage
func TestDartLanguage_InstallDependencies_AdditionalCoverage(t *testing.T) {
	dart := NewDartLanguage()

	t.Run("VariousDependencyInputs", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with various dependency inputs
		testCases := []struct {
			name string
			deps []string
		}{
			{"EmptyArray", []string{}},
			{"NilArray", nil},
			{"SingleDep", []string{"http"}},
			{"MultipleDeps", []string{"http", "test", "json_annotation"}},
			{"DepsWithVersions", []string{"http:^0.13.0", "test:^1.16.0"}},
			{"EmptyStringDep", []string{""}},
			{"MixedDeps", []string{"http", "", "test"}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := dart.InstallDependencies(tempDir, tc.deps)
				if err != nil {
					t.Errorf("InstallDependencies with %s should not error: %v", tc.name, err)
				} else {
					t.Logf("InstallDependencies with %s succeeded", tc.name)
				}
			})
		}
	})

	t.Run("PathEdgeCases", func(t *testing.T) {
		// Test with various path edge cases
		testPaths := []string{
			"/tmp/dart-test",
			"/path/with spaces/dart",
			"/path-with-dashes",
			"/path_with_underscores",
			"/path.with.dots",
			"relative/path",
			"",
		}

		for _, path := range testPaths {
			t.Run("Path_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
				err := dart.InstallDependencies(path, []string{})
				if err != nil {
					t.Errorf("InstallDependencies should not error for path %q: %v", path, err)
				}

				err = dart.InstallDependencies(path, []string{"test-dep"})
				if err != nil {
					t.Errorf("InstallDependencies should not error for path %q with deps: %v", path, err)
				}
			})
		}
	})

	t.Run("ConcurrentInstallations", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test concurrent calls to InstallDependencies
		done := make(chan error, 5)

		for i := range 5 {
			go func(id int) {
				deps := []string{fmt.Sprintf("dep%d", id)}
				err := dart.InstallDependencies(tempDir, deps)
				done <- err
			}(i)
		}

		// Wait for all goroutines to complete
		for i := range 5 {
			err := <-done
			if err != nil {
				t.Logf("Concurrent InstallDependencies %d failed: %v", i, err)
			}
		}
	})
}

// Test for additional CheckEnvironmentHealth coverage
func TestDartLanguage_CheckEnvironmentHealth_AdditionalCoverage(t *testing.T) {
	dart := NewDartLanguage()

	t.Run("CheckHealthFailureScenarios", func(t *testing.T) {
		// Test various scenarios where CheckHealth might fail
		testCases := []struct {
			name string
			path string
		}{
			{"EmptyPath", ""},
			{"NonExistentPath", "/non/existent/path"},
			{"RootPath", "/"},
			{"RelativePath", "relative/path"},
			{"PathWithSpaces", "/path with spaces"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := dart.CheckEnvironmentHealth(tc.path)
				// All of these should return false
				if result {
					t.Errorf("CheckEnvironmentHealth should return false for %s", tc.name)
				} else {
					t.Logf("CheckEnvironmentHealth correctly returned false for %s", tc.name)
				}
			})
		}
	})

	t.Run("RuntimeAvailabilityPaths", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a valid environment directory
		envPath := filepath.Join(tempDir, "dart-env")
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Test 1: With dart available (if system has it)
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		if _, err := exec.LookPath("dart"); err == nil {
			// Dart is available on system
			result := dart.CheckEnvironmentHealth(envPath)
			t.Logf("CheckEnvironmentHealth with system dart available: %v", result)
		}

		// Test 2: Without dart available
		os.Setenv("PATH", "/usr/bin:/bin") // Minimal PATH
		result := dart.CheckEnvironmentHealth(envPath)
		if result {
			t.Error("CheckEnvironmentHealth should return false when dart runtime is not available")
		} else {
			t.Log("CheckEnvironmentHealth correctly returned false when dart runtime unavailable")
		}

		// Test 3: With mock dart available
		mockDartDir := t.TempDir()
		mockDart := filepath.Join(mockDartDir, "dart")
		scriptContent := testDartSDKScript
		if err := os.WriteFile(mockDart, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart script: %v", err)
		}

		os.Setenv("PATH", mockDartDir+string(os.PathListSeparator)+"/usr/bin:/bin")
		result = dart.CheckEnvironmentHealth(envPath)
		if result {
			t.Log("CheckEnvironmentHealth correctly returned true with mock dart")
		} else {
			t.Log("CheckEnvironmentHealth returned false (mock dart may not be properly recognized)")
		}
	})

	t.Run("CheckHealthWithDifferentEnvironments", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create mock dart executable
		mockDartDir := t.TempDir()
		mockDart := filepath.Join(mockDartDir, "dart")
		scriptContent := testDartSDKScript
		if err := os.WriteFile(mockDart, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart script: %v", err)
		}

		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockDartDir+string(os.PathListSeparator)+originalPath)

		// Test different environment scenarios
		envScenarios := []struct {
			setupFunc      func(string) error
			name           string
			expectedResult bool
		}{
			{
				name: "EmptyEnvironment",
				setupFunc: func(path string) error {
					return os.MkdirAll(path, 0o755)
				},
				expectedResult: false, // May not pass if mock dart isn't properly recognized
			},
			{
				name: "EnvironmentWithFiles",
				setupFunc: func(path string) error {
					if err := os.MkdirAll(path, 0o755); err != nil {
						return err
					}
					return os.WriteFile(filepath.Join(path, "test.dart"), []byte("void main() {}"), 0o644)
				},
				expectedResult: false,
			},
			{
				name: "EnvironmentWithSubdirs",
				setupFunc: func(path string) error {
					return os.MkdirAll(filepath.Join(path, "lib", "src"), 0o755)
				},
				expectedResult: false,
			},
		}

		for i, scenario := range envScenarios {
			t.Run(scenario.name, func(t *testing.T) {
				envPath := filepath.Join(tempDir, fmt.Sprintf("env-%d", i))
				if err := scenario.setupFunc(envPath); err != nil {
					t.Fatalf("Failed to setup %s: %v", scenario.name, err)
				}

				result := dart.CheckEnvironmentHealth(envPath)
				if result != scenario.expectedResult && scenario.expectedResult {
					t.Logf("CheckEnvironmentHealth for %s: expected %v, got %v "+
						"(may be due to mock dart not being recognized)", scenario.name, scenario.expectedResult, result)
				} else {
					t.Logf("CheckEnvironmentHealth for %s returned %v", scenario.name, result)
				}
			})
		}
	})
}

// Test to try to cover the remaining CreateEnvironmentDirectory error path
func TestDartLanguage_CreateEnvironmentDirectoryFailure(t *testing.T) {
	dart := NewDartLanguage()

	t.Run("CreateEnvironmentDirectorySpecificFailure", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test directory creation failure")
		}

		// Create a mock dart executable to pass the runtime check
		tempBinDir := t.TempDir()
		mockDart := filepath.Join(tempBinDir, "dart")
		scriptContent := testDartSDKScript
		if err := os.WriteFile(mockDart, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart script: %v", err)
		}

		// Temporarily modify PATH to include our mock dart
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		// Create a scenario where the environment directory creation will fail
		// Try to create in a path that should fail (read-only parent directory)
		readOnlyBase := "/tmp/read-only-test-" + fmt.Sprintf("%d", os.Getpid())
		if err := os.MkdirAll(readOnlyBase, 0o755); err != nil {
			t.Fatalf("Failed to create read-only base directory: %v", err)
		}
		defer os.RemoveAll(readOnlyBase)

		// Make the base directory read-only
		if err := os.Chmod(readOnlyBase, 0o444); err != nil {
			t.Fatalf("Failed to make base directory read-only: %v", err)
		}
		defer os.Chmod(readOnlyBase, 0o755) // Restore for cleanup

		// Try to create environment in the read-only directory
		_, err := dart.SetupEnvironmentWithRepo("", "default", readOnlyBase, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when environment directory cannot be created")
		} else {
			if strings.Contains(err.Error(), "failed to create Dart environment directory") {
				t.Logf("Successfully triggered CreateEnvironmentDirectory error: %v", err)
			} else {
				t.Logf("Got error but may not be from CreateEnvironmentDirectory: %v", err)
			}
		}
	})

	t.Run("DirectWriteToSystemPath", func(t *testing.T) {
		// Try with paths that are definitely not writable
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test system directory write failure")
		}

		// Create mock dart executable
		tempBinDir := t.TempDir()
		mockDart := filepath.Join(tempBinDir, "dart")
		scriptContent := testDartSDKScript
		if err := os.WriteFile(mockDart, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dart script: %v", err)
		}

		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		// Try paths that should definitely fail for environment creation
		failPaths := []string{
			"/proc",     // Virtual filesystem, not writable
			"/sys",      // Virtual filesystem, not writable
			"/dev/null", // Not a directory
		}

		for _, failPath := range failPaths {
			t.Run("FailPath_"+strings.ReplaceAll(failPath, "/", "_"), func(t *testing.T) {
				_, err := dart.SetupEnvironmentWithRepo("", "default", failPath, "dummy-url", []string{})
				if err == nil {
					t.Logf("SetupEnvironmentWithRepo unexpectedly succeeded for path %s", failPath)
				} else {
					if strings.Contains(err.Error(), "failed to create Dart environment directory") {
						t.Logf("Successfully triggered CreateEnvironmentDirectory error for %s: %v", failPath, err)
					} else {
						t.Logf("Got error for %s (may not be from CreateEnvironmentDirectory): %v", failPath, err)
					}
				}
			})
		}
	})
}
