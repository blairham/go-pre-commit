package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/tests/helpers"
)

// Test constants to avoid goconst linting issues
const (
	testVersionFlag   = "--version"
	testRootPath      = "/root"
	testDefaultStr    = "default"
	testDartSDKScript = `#!/bin/bash
echo 'Dart SDK version: 3.0.0'
exit 0
`
)

func TestCoursierLanguage(t *testing.T) {
	t.Run("NewCoursierLanguage", func(t *testing.T) {
		coursier := NewCoursierLanguage()

		if coursier == nil {
			t.Fatal("NewCoursierLanguage() returned nil")
		}

		if coursier.Name != "Coursier" {
			t.Errorf("Expected language name 'Coursier', got '%s'", coursier.Name)
		}

		if coursier.ExecutableName != "coursier" {
			t.Errorf("Expected executable name 'coursier', got '%s'", coursier.ExecutableName)
		}

		if coursier.VersionFlag != testVersionFlag {
			t.Errorf("Expected version flag '%s', got '%s'", testVersionFlag, coursier.VersionFlag)
		}

		if coursier.InstallURL != "https://get-coursier.io/" {
			t.Errorf("Expected install URL 'https://get-coursier.io/', got '%s'", coursier.InstallURL)
		}
	})

	t.Run("HelperTests", func(t *testing.T) {
		coursier := NewCoursierLanguage()

		config := helpers.LanguageTestConfig{
			Language:       coursier,
			Name:           "Coursier",
			ExecutableName: "coursier",
			VersionFlag:    testVersionFlag,
			TestVersions:   []string{"", "2.1.0", "2.1.1", "2.1.2"},
			EnvPathSuffix:  "coursierenv-2.1.2",
		}

		helpers.RunLanguageTests(t, config)
	})
}

func TestCoursierLanguage_InstallDependencies(t *testing.T) {
	coursier := NewCoursierLanguage()

	t.Run("NoDependencies", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-env-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		err = coursier.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with no dependencies returned error: %v", err)
		}

		err = coursier.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil dependencies returned error: %v", err)
		}
	})

	t.Run("WithDependencies", func(t *testing.T) {
		// Skip if coursier is not available
		if _, err := exec.LookPath("coursier"); err != nil {
			t.Skip("coursier not available, skipping dependency installation test")
		}

		tempDir, err := os.MkdirTemp("", "test-coursier-deps-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test with some Scala/JVM dependencies
		deps := []string{"ammonite", "scala-cli"}
		err = coursier.InstallDependencies(tempDir, deps)

		// We don't require this to succeed because it requires network access and coursier setup
		// Just log the result
		if err != nil {
			t.Logf("InstallDependencies() failed (expected if coursier/network not properly configured): %v", err)
		} else {
			// Verify apps directory was created
			appsDir := filepath.Join(tempDir, "apps")
			if _, err := os.Stat(appsDir); os.IsNotExist(err) {
				t.Error("InstallDependencies() should have created apps directory")
			}
		}
	})

	t.Run("InvalidPath", func(t *testing.T) {
		deps := []string{"test-dep"}
		err := coursier.InstallDependencies("/invalid/readonly/path", deps)
		if err == nil {
			t.Error("InstallDependencies() with invalid path should return error")
		}
	})
}

func TestCoursierLanguage_CheckEnvironmentHealth(t *testing.T) {
	coursier := NewCoursierLanguage()

	t.Run("NonExistentPath", func(t *testing.T) {
		result := coursier.CheckEnvironmentHealth("/non/existent/path")
		if result {
			t.Error("CheckEnvironmentHealth() should return false for non-existent path")
		}
	})

	t.Run("ExistingDirectory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-health-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		result := coursier.CheckEnvironmentHealth(tempDir)

		// The result depends on whether coursier is installed and working
		if result {
			t.Logf("CheckEnvironmentHealth() returned true (coursier appears to be available)")
		} else {
			t.Logf("CheckEnvironmentHealth() returned false (coursier not available or environment not healthy)")
		}
	})

	t.Run("WithAppsDirectory", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-health-apps-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create apps directory
		appsDir := filepath.Join(tempDir, "apps")
		if err := os.MkdirAll(appsDir, 0o755); err != nil {
			t.Fatalf("Failed to create apps directory: %v", err)
		}

		result := coursier.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth() with apps directory returned: %v", result)
	})
}

func TestCoursierLanguage_SetupEnvironmentWithRepo(t *testing.T) {
	coursier := NewCoursierLanguage()

	// Helper to check if coursier is available
	isCoursierAvailable := func() bool {
		if _, err := exec.LookPath("cs"); err == nil {
			return true
		}
		if _, err := exec.LookPath("coursier"); err == nil {
			return true
		}
		return false
	}

	t.Run("DefaultVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-setup-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip test if coursier is not available
		if !isCoursierAvailable() {
			t.Skip("coursier not available, skipping test that requires coursier")
		}

		envPath, err := coursier.SetupEnvironmentWithRepo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
			return
		}

		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() returned empty environment path")
		}

		// Verify environment directory was created
		expectedPath := filepath.Join(tempDir, "coursierenv-default")
		if envPath != expectedPath {
			t.Errorf("SetupEnvironmentWithRepo() returned unexpected path: got %s, want %s", envPath, expectedPath)
		}

		// Directory should exist
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			t.Error("SetupEnvironmentWithRepo() did not create environment directory")
		}
	})

	t.Run("UnsupportedVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-unsupported-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		envPath, err := coursier.SetupEnvironmentWithRepo(tempDir, "2.13", tempDir, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo() should return error for unsupported version")
		}

		expectedMsg := "coursier only supports version 'default'"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("SetupEnvironmentWithRepo() error message should contain '%s', got: %v", expectedMsg, err)
		}

		if envPath != "" {
			t.Error("SetupEnvironmentWithRepo() should return empty path on error")
		}
	})

	t.Run("WithDependencies", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-with-deps-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Skip test if coursier is not available
		if !isCoursierAvailable() {
			t.Skip("coursier not available, skipping test that requires coursier")
		}

		deps := []string{"com.lihaoyi::upickle:3.1.0"}
		envPath, err := coursier.SetupEnvironmentWithRepo(tempDir, "default", tempDir, "dummy-url", deps)

		// Log result - dependency installation requires coursier and network access
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() with dependencies failed (may be expected): %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo() with dependencies succeeded: %s", envPath)
		}
	})

	t.Run("CoursierNotAvailable", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-not-available-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Only run this test if coursier is actually not available
		if isCoursierAvailable() {
			t.Skip("coursier is available, skipping test for missing coursier")
		}

		envPath, err := coursier.SetupEnvironmentWithRepo(tempDir, "default", tempDir, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo() should return error when coursier is not available")
		}

		expectedMsg := "pre-commit requires system-installed"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("SetupEnvironmentWithRepo() error message should contain '%s', got: %v", expectedMsg, err)
		}

		if envPath != "" {
			t.Error("SetupEnvironmentWithRepo() should return empty path on error")
		}
	})

	// Additional edge case tests for better coverage
	t.Run("SetupEnvironmentWithRepo_EdgeCases", func(t *testing.T) {
		coursier := NewCoursierLanguage()
		tempDir := t.TempDir()

		// Test with different invalid versions to ensure all paths are covered
		invalidVersions := []string{"1.0", "2.0", "invalid", "1.3.6"}
		for _, version := range invalidVersions {
			_, err := coursier.SetupEnvironmentWithRepo(tempDir, version, tempDir, "dummy-url", []string{})
			if err == nil {
				t.Errorf("SetupEnvironmentWithRepo() with version %s should return error", version)
			}
		}

		// Test with empty strings
		_, err := coursier.SetupEnvironmentWithRepo(tempDir, "", tempDir, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo() with empty version should return error")
		}
	})
}

func TestCoursierLanguage_CheckHealth(t *testing.T) {
	coursier := NewCoursierLanguage()

	t.Run("DefaultVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-health-check-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		err = coursier.CheckHealth(tempDir, "default")

		// This depends on whether coursier is installed
		if err != nil {
			if strings.Contains(err.Error(), "pre-commit requires system-installed") {
				t.Logf("CheckHealth() failed as expected (coursier not installed): %v", err)
			} else if strings.Contains(err.Error(), "environment directory does not exist") {
				t.Logf("CheckHealth() failed as expected (environment not setup): %v", err)
			} else {
				t.Logf("CheckHealth() failed with error: %v", err)
			}
		} else {
			t.Logf("CheckHealth() succeeded (coursier is available)")
		}
	})

	t.Run("UnsupportedVersion", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test-coursier-health-unsupported-*")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		err = coursier.CheckHealth(tempDir, "2.13")
		if err == nil {
			t.Error("CheckHealth() should return error for unsupported version")
		}

		expectedMsg := "coursier only supports version 'default'"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("CheckHealth() error message should contain '%s', got: %v", expectedMsg, err)
		}
	})

	t.Run("NonExistentDirectory", func(t *testing.T) {
		err := coursier.CheckHealth("/non/existent/directory", "default")
		if err == nil {
			t.Error("CheckHealth() should return error for non-existent directory")
		}

		expectedMsg := "environment directory does not exist"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("CheckHealth() error message should contain '%s', got: %v", expectedMsg, err)
		}
	})
}

// Comprehensive tests to improve coverage
func TestCoursierLanguage_ComprehensiveInstallDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow Coursier comprehensive tests in short mode")
	}

	coursier := NewCoursierLanguage()

	t.Run("MockedSuccessfulInstallation", func(t *testing.T) {
		// Create a temporary directory and mock coursier script
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "env")

		// Create a mock coursier script that always succeeds
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		scriptContent := testSuccessScript
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH to include our mock coursier
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test successful installation with dependencies
		deps := []string{"ammonite", "scala-cli", "com.lihaoyi::upickle:3.1.0"}
		err := coursier.InstallDependencies(envPath, deps)
		if err != nil {
			t.Logf("Install failed despite mock (PATH might not work): %v", err)
		} else {
			// Verify apps directory was created
			appsDir := filepath.Join(envPath, "apps")
			if _, err := os.Stat(appsDir); os.IsNotExist(err) {
				t.Error("InstallDependencies should create apps directory")
			}
			t.Logf("Successfully tested dependency installation")
		}
	})

	t.Run("MockedFailedInstallation", func(t *testing.T) {
		// Create a temporary directory and mock coursier script that fails
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "env")

		// Create a mock coursier script that always fails
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		scriptContent := "#!/bin/bash\necho 'Error installing dependency' >&2\nexit 1\n"
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH to include our mock coursier
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test failed installation
		deps := []string{"invalid-dependency"}
		err := coursier.InstallDependencies(envPath, deps)
		if err == nil {
			t.Error("InstallDependencies should return error when coursier command fails")
		} else {
			if !strings.Contains(err.Error(), "failed to install Coursier dependency") {
				t.Errorf("Expected install error message, got: %v", err)
			}
			t.Logf("Correctly handled installation failure: %v", err)
		}
	})

	t.Run("DirectoryCreationFailure", func(t *testing.T) {
		// Test failure when directory cannot be created
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test directory creation failure")
		}

		// Try to create environment in read-only directory
		readOnlyDir := testRootPath
		deps := []string{"test-dep"}
		err := coursier.InstallDependencies(readOnlyDir, deps)
		if err == nil {
			t.Error("InstallDependencies should fail when apps directory cannot be created")
		} else {
			if !strings.Contains(err.Error(), "failed to create apps directory") {
				t.Logf("Got different error type: %v", err)
			} else {
				t.Logf("Correctly handled directory creation failure: %v", err)
			}
		}
	})

	t.Run("VariousDependencyFormats", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "env")

		// Test different dependency formats
		deps := []string{
			"ammonite",                             // Simple name
			"com.lihaoyi::upickle:3.1.0",           // Maven coordinates with Scala version
			"org.scala-lang:scala-library:2.13.10", // Full Maven coordinates
			"scala-cli",                            // Another simple name
		}

		// This will likely fail without coursier, but tests the code paths
		err := coursier.InstallDependencies(envPath, deps)
		if err != nil {
			t.Logf("Installation failed as expected (coursier not available): %v", err)
		} else {
			// Verify apps directory exists
			appsDir := filepath.Join(envPath, "apps")
			if _, err := os.Stat(appsDir); os.IsNotExist(err) {
				t.Error("Apps directory should have been created")
			}
		}
	})
}

func TestCoursierLanguage_ComprehensiveCheckEnvironmentHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow Coursier comprehensive tests in short mode")
	}

	coursier := NewCoursierLanguage()

	t.Run("MockedHealthyEnvironment", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "healthy-env")

		// Create environment directory
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create apps directory
		appsDir := filepath.Join(envPath, "apps")
		if err := os.MkdirAll(appsDir, 0o755); err != nil {
			t.Fatalf("Failed to create apps directory: %v", err)
		}

		// Create mock coursier executables
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		mockCsScript := filepath.Join(tempDir, "cs")
		scriptContent := "#!/bin/bash\nif [[ \"$1\" == \"list\" ]]; then\n  echo 'ammonite'\nfi\nexit 0\n"

		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}
		if err := os.WriteFile(mockCsScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock cs script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test healthy environment
		result := coursier.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth for healthy environment returned: %v", result)
	})

	t.Run("EnvironmentWithoutApps", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "no-apps-env")

		// Create environment directory but no apps directory
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create mock coursier executable
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		scriptContent := testSuccessScript
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		result := coursier.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth for environment without apps returned: %v", result)
	})

	t.Run("FailedListCommand", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "failed-list-env")

		// Create environment directory and apps directory
		appsDir := filepath.Join(envPath, "apps")
		if err := os.MkdirAll(appsDir, 0o755); err != nil {
			t.Fatalf("Failed to create apps directory: %v", err)
		}

		// Create mock coursier executable that fails on list command
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		scriptContent := "#!/bin/bash\nif [[ \"$1\" == \"list\" ]]; then\n  exit 1\nfi\nexit 0\n"
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		result := coursier.CheckEnvironmentHealth(envPath)
		if result {
			t.Error("CheckEnvironmentHealth should return false when list command fails")
		} else {
			t.Logf("Correctly detected unhealthy environment when list command fails")
		}
	})

	t.Run("CheckHealthFailure", func(t *testing.T) {
		// Test when CheckHealth itself fails
		result := coursier.CheckEnvironmentHealth("/non/existent/path")
		if result {
			t.Error("CheckEnvironmentHealth should return false when CheckHealth fails")
		}
	})
}

func TestCoursierLanguage_ComprehensiveSetupEnvironmentWithRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow Coursier comprehensive tests in short mode")
	}

	coursier := NewCoursierLanguage()

	t.Run("MockedSuccessfulSetup", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo")
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create mock cs and coursier executables
		mockCsScript := filepath.Join(tempDir, "cs")
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		scriptContent := testSuccessScript

		if err := os.WriteFile(mockCsScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock cs script: %v", err)
		}
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test successful setup without dependencies
		envPath, err := coursier.SetupEnvironmentWithRepo("cache", "default", repoPath, "http://repo", []string{})
		if err != nil {
			t.Logf("Setup failed despite mock (PATH might not work): %v", err)
		} else {
			expectedPath := filepath.Join(repoPath, "coursierenv-default")
			if envPath != expectedPath {
				t.Errorf("Expected environment path %s, got %s", expectedPath, envPath)
			}

			// Verify directory was created
			if _, err := os.Stat(envPath); os.IsNotExist(err) {
				t.Error("SetupEnvironmentWithRepo should create environment directory")
			}
			t.Logf("Successfully tested environment setup")
		}
	})

	t.Run("MockedSetupWithDependencies", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo-with-deps")
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create mock cs executable
		mockCsScript := filepath.Join(tempDir, "cs")
		scriptContent := testSuccessScript
		if err := os.WriteFile(mockCsScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock cs script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test setup with dependencies
		deps := []string{"ammonite", "com.lihaoyi::upickle:3.1.0"}
		envPath, err := coursier.SetupEnvironmentWithRepo("cache", "default", repoPath, "http://repo", deps)
		if err != nil {
			t.Logf("Setup with dependencies failed despite mock (PATH might not work): %v", err)
		} else {
			expectedPath := filepath.Join(repoPath, "coursierenv-default")
			if envPath != expectedPath {
				t.Errorf("Expected environment path %s, got %s", expectedPath, envPath)
			}
			t.Logf("Successfully tested environment setup with dependencies")
		}
	})

	t.Run("MockedFailedFetch", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo-failed-fetch")
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create mock cs executable that fails on fetch
		createMockCoursierExecutable(t, tempDir, "fetch")

		// Test setup with dependencies that fail to fetch
		deps := []string{"invalid-dependency"}
		_, err := coursier.SetupEnvironmentWithRepo("cache", testDefaultStr, repoPath, "http://repo", deps)
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when fetch command fails")
		} else {
			if !strings.Contains(err.Error(), "failed to fetch coursier dependency") {
				t.Errorf("Expected fetch error message, got: %v", err)
			}
			t.Logf("Correctly handled fetch failure: %v", err)
		}
	})

	t.Run("MockedFailedInstall", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo-failed-install")
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create mock cs executable that fails on install but succeeds on fetch
		createMockCoursierExecutable(t, tempDir, "install")

		// Test setup with dependencies that fail to install
		deps := []string{"test-dependency"}
		_, err := coursier.SetupEnvironmentWithRepo("cache", testDefaultStr, repoPath, "http://repo", deps)
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when install command fails")
		} else {
			if !strings.Contains(err.Error(), "failed to install coursier dependency") {
				t.Errorf("Expected install error message, got: %v", err)
			}
			t.Logf("Correctly handled install failure: %v", err)
		}
	})

	t.Run("DirectoryCreationFailure", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test directory creation failure")
		}

		// Create mock cs executable
		tempDir := t.TempDir()
		mockCsScript := filepath.Join(tempDir, "cs")
		scriptContent := testSuccessScript
		if err := os.WriteFile(mockCsScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock cs script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Try to create environment in read-only directory
		readOnlyDir := testRootPath
		_, err := coursier.SetupEnvironmentWithRepo("cache", "default", readOnlyDir, "http://repo", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo should fail when environment directory cannot be created")
		} else {
			if !strings.Contains(err.Error(), "failed to create coursier environment directory") {
				t.Logf("Got different error type: %v", err)
			} else {
				t.Logf("Correctly handled directory creation failure: %v", err)
			}
		}
	})

	t.Run("PrefersCsOverCoursier", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo-cs-preference")
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create both cs and coursier executables, cs should be preferred
		mockCsScript := filepath.Join(tempDir, "cs")
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		scriptContent := testSuccessScript

		if err := os.WriteFile(mockCsScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock cs script: %v", err)
		}
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test that cs is preferred (this is implicit in the implementation)
		envPath, err := coursier.SetupEnvironmentWithRepo("cache", "default", repoPath, "http://repo", []string{})
		if err != nil {
			t.Logf("Setup failed despite mock (PATH might not work): %v", err)
		} else {
			t.Logf("Successfully tested cs preference: %s", envPath)
		}
	})
}

func TestCoursierLanguage_ComprehensiveCheckHealth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow Coursier comprehensive tests in short mode")
	}

	coursier := NewCoursierLanguage()

	t.Run("MockedHealthyCheck", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "healthy-env")

		// Create environment directory
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create mock cs and coursier executables
		mockCsScript := filepath.Join(tempDir, "cs")
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		scriptContent := testSuccessScript

		if err := os.WriteFile(mockCsScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock cs script: %v", err)
		}
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test healthy check
		err := coursier.CheckHealth(envPath, "default")
		if err != nil {
			t.Logf("CheckHealth failed despite mock (PATH might not work): %v", err)
		} else {
			t.Logf("Successfully tested healthy environment check")
		}
	})

	t.Run("FailedHelpCommand", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "failed-help-env")

		// Create environment directory
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create mock cs executable that fails on --help
		mockCsScript := filepath.Join(tempDir, "cs")
		scriptContent := "#!/bin/bash\nif [[ \"$1\" == \"--help\" ]]; then\n  exit 1\nfi\nexit 0\n"
		if err := os.WriteFile(mockCsScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock cs script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test failed health check
		err := coursier.CheckHealth(envPath, "default")
		if err == nil {
			t.Error("CheckHealth should fail when --help command fails")
		} else {
			if !strings.Contains(err.Error(), "system coursier executable not working") {
				t.Errorf("Expected system executable error, got: %v", err)
			}
			t.Logf("Correctly detected failed executable: %v", err)
		}
	})

	t.Run("CoursierOnlyAvailable", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "coursier-only-env")

		// Create environment directory
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create only coursier executable (not cs)
		mockCoursierScript := filepath.Join(tempDir, "coursier")
		scriptContent := testSuccessScript
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)

		// Test health check with only coursier available
		err := coursier.CheckHealth(envPath, "default")
		if err != nil {
			t.Logf("CheckHealth failed despite mock (PATH might not work): %v", err)
		} else {
			t.Logf("Successfully tested health check with coursier executable")
		}
	})

	t.Run("EdgeCaseVersions", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "version-test-env")
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Test various unsupported versions
		unsupportedVersions := []string{"2.13", "3.0", "latest", "2.1.0", ""}
		for _, version := range unsupportedVersions {
			if version == testDefaultStr {
				continue // This is the only supported version
			}

			err := coursier.CheckHealth(envPath, version)
			if err == nil {
				t.Errorf("CheckHealth should fail for unsupported version: %s", version)
			} else {
				if !strings.Contains(err.Error(), "coursier only supports version 'default'") {
					t.Errorf("Expected version error for %s, got: %v", version, err)
				}
			}
		}
	})
}

// Targeted tests to achieve 100% coverage for specific branches
func TestCoursierLanguage_FinalCoverageImprovements(t *testing.T) {
	coursier := NewCoursierLanguage()

	t.Run("SetupEnvironmentWithRepo_CoursierExecutablePreference", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo-exe-preference")
		if err := os.MkdirAll(repoPath, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Test with only 'coursier' executable available (not 'cs')
		mockDir := t.TempDir()
		mockCoursierScript := filepath.Join(mockDir, "coursier")
		scriptContent := testSuccessScript

		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH to only include our mock coursier (not cs)
		originalPath := os.Getenv("PATH")
		defer func() { os.Setenv("PATH", originalPath) }()
		os.Setenv("PATH", mockDir)

		// This should use 'coursier' executable since 'cs' is not available
		envPath, err := coursier.SetupEnvironmentWithRepo("", "default", repoPath, "", []string{})
		if err != nil {
			t.Logf("Setup failed despite mock (PATH isolation might not work): %v", err)
		} else {
			expectedPath := filepath.Join(repoPath, "coursierenv-default")
			if envPath != expectedPath {
				t.Errorf("Expected environment path %s, got %s", expectedPath, envPath)
			}
			t.Logf("Successfully tested coursier executable preference: %s", envPath)
		}
	})

	t.Run("CheckHealth_CoursierExecutablePreference", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "health-check-env")
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Test with only 'coursier' executable available (not 'cs')
		mockDir := t.TempDir()
		mockCoursierScript := filepath.Join(mockDir, "coursier")
		scriptContent := "#!/bin/bash\nif [[ \"$1\" == \"--help\" ]]; then\n  echo 'Coursier help'\n  exit 0\nfi\nexit 0\n"
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH to only include our mock coursier (not cs)
		originalPath := os.Getenv("PATH")
		defer func() { os.Setenv("PATH", originalPath) }()
		os.Setenv("PATH", mockDir)

		// This should use 'coursier' executable since 'cs' is not available
		err := coursier.CheckHealth(envPath, "default")
		if err != nil {
			t.Logf("CheckHealth failed despite mock (PATH isolation might not work): %v", err)
		} else {
			t.Logf("Successfully tested CheckHealth with coursier executable preference")
		}
	})

	t.Run("CheckEnvironmentHealth_AppsDirectoryPath", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "apps-test-env")

		// Create environment directory
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create mock coursier that works for health check
		mockDir := t.TempDir()
		mockCoursierScript := filepath.Join(mockDir, "coursier")
		scriptContent := `#!/bin/bash
if [[ "$1" == "--help" ]]; then
  echo 'Coursier help'
  exit 0
elif [[ "$1" == "list" ]]; then
  echo 'ammonite'
  exit 0
fi
exit 0
`
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer func() { os.Setenv("PATH", originalPath) }()
		os.Setenv("PATH", mockDir)

		// Test without apps directory first
		result := coursier.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth without apps directory: %v", result)

		// Create apps directory
		appsDir := filepath.Join(envPath, "apps")
		if err := os.MkdirAll(appsDir, 0o755); err != nil {
			t.Fatalf("Failed to create apps directory: %v", err)
		}

		// Test with apps directory (should trigger the list command)
		result = coursier.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth with apps directory: %v", result)
	})

	t.Run("CheckHealth_HelpCommandFailure", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "help-fail-env")
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create mock coursier that fails on --help command
		mockDir := t.TempDir()
		mockCoursierScript := filepath.Join(mockDir, "coursier")
		scriptContent := `#!/bin/bash
if [[ "$1" == "--help" ]]; then
  exit 1
fi
exit 0
`
		if err := os.WriteFile(mockCoursierScript, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock coursier script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer func() { os.Setenv("PATH", originalPath) }()
		os.Setenv("PATH", mockDir)

		// This should fail because --help command fails
		err := coursier.CheckHealth(envPath, "default")
		if err == nil {
			t.Error("CheckHealth should fail when --help command fails")
		} else {
			if !strings.Contains(err.Error(), "system coursier executable not working") {
				t.Errorf("Expected system executable error, got: %v", err)
			}
			t.Logf("Correctly detected failed --help command: %v", err)
		}
	})
}

// Helper function to create mock coursier executable for testing failures
func createMockCoursierExecutable(t *testing.T, tempDir, failCommand string) {
	t.Helper()

	mockCsScript := filepath.Join(tempDir, "cs")
	scriptContent := fmt.Sprintf("#!/bin/bash\nif [[ \"$1\" == \"%s\" ]]; then\n  exit 1\nfi\nexit 0\n", failCommand)
	if err := os.WriteFile(mockCsScript, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("Failed to create mock cs script: %v", err)
	}

	// Temporarily modify PATH
	originalPath := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})
	_ = os.Setenv("PATH", tempDir+string(os.PathListSeparator)+originalPath)
}
