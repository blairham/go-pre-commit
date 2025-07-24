package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// init automatically sets test mode for faster tests
func init() {
	// Set test mode environment variable to speed up dependency installations
	if err := os.Setenv("GO_PRE_COMMIT_TEST_MODE", "true"); err != nil {
		// In init functions, we can't easily handle errors, but this shouldn't fail in practice
		panic(fmt.Sprintf("failed to set test mode: %v", err))
	}
}

// testSimpleLanguageInterface runs comprehensive tests for simple language implementations
// that have no specific executable (like "fail" and "script")
func testSimpleLanguageInterface(t *testing.T, lang language.Manager, expectedName string) {
	t.Helper()
	testSimpleLanguageBasics(t, lang, expectedName)
	testSimpleLanguageComprehensive(t, lang, expectedName)
}

// testSimpleLanguageBasics tests basic language functionality
func testSimpleLanguageBasics(t *testing.T, lang language.Manager, expectedName string) {
	t.Helper()

	t.Run("LanguageProperties", func(t *testing.T) {
		// Check that it has the expected language name
		if lang.GetName() != expectedName {
			t.Errorf("Expected language name '%s', got '%s'", expectedName, lang.GetName())
		}

		// Simple languages should have empty executable name
		if lang.GetExecutableName() != "" {
			t.Errorf(
				"Expected GetExecutableName() to return empty string, got '%s'",
				lang.GetExecutableName(),
			)
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		tempDir := t.TempDir()

		// Should not error when setting up environment
		envPath, err := lang.SetupEnvironmentWithRepo(
			tempDir,
			"1.0",
			tempDir,
			"dummy-url",
			[]string{},
		)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() returned empty environment path")
		}
	})

	t.Run("InstallDependencies", func(t *testing.T) {
		// Should not error when installing dependencies (no-op)
		err := lang.InstallDependencies("/dummy/path", []string{"dep1", "dep2"})
		if err != nil {
			t.Errorf("InstallDependencies() returned error: %v", err)
		}

		// Should handle empty dependencies
		err = lang.InstallDependencies("/dummy/path", []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}
	})

	t.Run("CheckHealth", func(t *testing.T) {
		tempDir := t.TempDir()

		// Should perform basic health check since simple languages have no specific executable
		err := lang.CheckHealth(tempDir, "1.0")
		if err != nil {
			t.Errorf("CheckHealth() returned error: %v", err)
		}
	})
}

// testSimpleLanguageComprehensive tests comprehensive functionality
func testSimpleLanguageComprehensive(t *testing.T, lang language.Manager, expectedName string) {
	t.Helper()

	t.Run("ComprehensiveCoverage", func(t *testing.T) {
		testSimpleLanguageSetupEnvironment(t, lang)
		testSimpleLanguageDependencies(t, lang)
		testSimpleLanguageHealth(t, lang, expectedName)
		testSimpleLanguageRuntime(t, lang, expectedName)
		testSimpleLanguageEnvironmentInfo(t, lang)
	})
}

// testSimpleLanguageSetupEnvironment tests environment setup functionality
func testSimpleLanguageSetupEnvironment(t *testing.T, lang language.Manager) {
	t.Helper()

	t.Run("SetupEnvironmentWithRepo_Comprehensive", func(t *testing.T) {
		tempDir := t.TempDir()
		repoPath := filepath.Join(tempDir, "repo")
		if err := os.MkdirAll(repoPath, 0o750); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Test various version formats
		versions := []string{"1.0", "latest", "system", "default", ""}
		for _, version := range versions {
			envPath, err := lang.SetupEnvironmentWithRepo(
				tempDir,
				version,
				repoPath,
				"https://example.com",
				[]string{},
			)
			if err != nil {
				t.Errorf(
					"SetupEnvironmentWithRepo() with version '%s' returned error: %v",
					version,
					err,
				)
			}
			if envPath == "" {
				t.Errorf(
					"SetupEnvironmentWithRepo() with version '%s' returned empty path",
					version,
				)
			}
		}

		// Test with dependencies
		envPath, err := lang.SetupEnvironmentWithRepo(tempDir, "1.0", repoPath,
			"https://example.com", []string{"dep1", "dep2"})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with dependencies returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() with dependencies returned empty path")
		}
	})
}

// testSimpleLanguageDependencies tests dependency management
func testSimpleLanguageDependencies(t *testing.T, lang language.Manager) {
	t.Helper()

	t.Run("InstallDependencies_Comprehensive", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with nil dependencies
		err := lang.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}

		// Test with empty dependencies
		err = lang.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		// Test with single dependency
		err = lang.InstallDependencies(tempDir, []string{"dep1"})
		if err != nil {
			t.Errorf("InstallDependencies() with single dep returned error: %v", err)
		}

		// Test with multiple dependencies
		err = lang.InstallDependencies(tempDir, []string{"dep1", "dep2", "dep3"})
		if err != nil {
			t.Errorf("InstallDependencies() with multiple deps returned error: %v", err)
		}

		// Test with non-existent directory
		err = lang.InstallDependencies("/non/existent/path", []string{"dep1"})
		if err != nil {
			t.Errorf("InstallDependencies() with non-existent path returned error: %v", err)
		}
	})
}

// testSimpleLanguageHealth tests health check functionality
func testSimpleLanguageHealth(t *testing.T, lang language.Manager, expectedName string) {
	t.Helper()

	t.Run("CheckHealth_Comprehensive", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with valid directory and version
		err := lang.CheckHealth(tempDir, "1.0")
		if err != nil {
			t.Errorf("CheckHealth() with valid path returned error: %v", err)
		}

		// Test with empty version
		err = lang.CheckHealth(tempDir, "")
		if err != nil {
			t.Errorf("CheckHealth() with empty version returned error: %v", err)
		}

		// Test with non-existent directory
		err = lang.CheckHealth("/non/existent/path", "1.0")
		if err == nil {
			t.Error("CheckHealth() with non-existent path should return error")
		}

		// Test with empty path
		err = lang.CheckHealth("", "1.0")
		if err == nil {
			t.Error("CheckHealth() with empty path should return error")
		}
	})

	t.Run("CheckEnvironmentHealth", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create an environment directory
		envPath := filepath.Join(tempDir, "env")
		if err := os.MkdirAll(envPath, 0o750); err != nil {
			t.Fatalf("Failed to create env directory: %v", err)
		}

		// Test with valid environment path - should return false because there's no executable
		// Simple languages don't have a specific executable, so CheckEnvironmentHealth
		// will look for an empty executable name in bin directory and fail
		healthy := lang.CheckEnvironmentHealth(envPath)
		if healthy {
			t.Errorf(
				"CheckEnvironmentHealth() should return false for %s language (no executable)",
				expectedName,
			)
		}

		// Test with non-existent path
		healthy = lang.CheckEnvironmentHealth("/non/existent/path")
		if healthy {
			t.Error("CheckEnvironmentHealth() should return false for non-existent path")
		}

		// Test with empty path
		healthy = lang.CheckEnvironmentHealth("")
		if healthy {
			t.Error("CheckEnvironmentHealth() should return false for empty path")
		}
	})
}

// testSimpleLanguageRuntime tests runtime availability
func testSimpleLanguageRuntime(t *testing.T, lang language.Manager, expectedName string) {
	t.Helper()

	t.Run("IsRuntimeAvailable", func(t *testing.T) {
		available := lang.IsRuntimeAvailable()

		// Special cases: fail and script languages always return true
		// - fail language: designed to be available but fail during execution
		// - script language: uses shell commands so always available
		if expectedName == "fail" || expectedName == "script" {
			if !available {
				t.Errorf(
					"IsRuntimeAvailable() should return true for %s language (always available)",
					expectedName,
				)
			}
		} else {
			// Simple languages should return false since they have no executable name to check
			// The base implementation checks for the executable in PATH, but simple languages have empty executable name
			if available {
				t.Errorf(
					"IsRuntimeAvailable() should return false for %s language (no executable)",
					expectedName,
				)
			}
		}
	})
}

// testSimpleLanguageEnvironmentInfo tests environment info functionality
func testSimpleLanguageEnvironmentInfo(t *testing.T, lang language.Manager) {
	t.Helper()

	t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test PreInitializeEnvironmentWithRepoInfo (should be no-op)
		err := lang.PreInitializeEnvironmentWithRepoInfo(
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
		envPath, err := lang.SetupEnvironmentWithRepoInfo(
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
}

// testEnvironmentNaming runs tests for environment directory naming conventions
func testEnvironmentNaming(
	t *testing.T,
	lang language.Manager,
	expectedVersion, expectedEnvPrefix string,
) {
	t.Helper()
	tempDir := t.TempDir()

	// Test that environment directories use correct naming convention
	envPath, err := lang.SetupEnvironmentWithRepo(
		"",
		expectedVersion,
		tempDir,
		"dummy-url",
		[]string{},
	)
	if err != nil {
		t.Logf("SetupEnvironmentWithRepo failed (may be expected if not available): %v", err)
	} else {
		expectedPath := filepath.Join(tempDir, expectedEnvPrefix+"-"+expectedVersion)
		if envPath != expectedPath {
			t.Errorf("Expected environment path %s, got %s", expectedPath, envPath)
		}
	}

	// Test default version
	envPath, err = lang.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})
	if err != nil {
		t.Logf("SetupEnvironmentWithRepo with default failed: %v", err)
	} else {
		expectedPath := filepath.Join(tempDir, expectedEnvPrefix+"-default")
		if envPath != expectedPath {
			t.Errorf("Expected environment path %s, got %s", expectedPath, envPath)
		}
	}

	// Test empty version (should default to default)
	envPath, err = lang.SetupEnvironmentWithRepo("", "", tempDir, "dummy-url", []string{})
	if err != nil {
		t.Logf("SetupEnvironmentWithRepo with empty version failed: %v", err)
	} else {
		expectedPath := filepath.Join(tempDir, expectedEnvPrefix+"-default")
		if envPath != expectedPath {
			t.Errorf("Expected environment path %s for empty version, got %s", expectedPath, envPath)
		}
	}
}

// testInstallDependenciesBasic runs basic dependency installation tests
func testInstallDependenciesBasic(
	t *testing.T, lang language.Manager, sampleDep, versionedDep string,
	expectSampleDepError, expectVersionedDepError bool,
) {
	t.Helper()

	// In test mode, dependency installations should succeed
	isTestMode := os.Getenv("GO_PRE_COMMIT_TEST_MODE") == testModeEnvValue
	if isTestMode {
		expectSampleDepError = false
		expectVersionedDepError = false
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "deps_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	tests := []struct {
		name    string
		deps    []string
		wantErr bool
	}{
		{
			name:    "no dependencies",
			deps:    []string{},
			wantErr: false,
		},
		{
			name:    "nil dependencies",
			deps:    nil,
			wantErr: false,
		},
		{
			name:    "single dependency",
			deps:    []string{sampleDep},
			wantErr: expectSampleDepError,
		},
		{
			name:    "dependency with version",
			deps:    []string{versionedDep},
			wantErr: expectVersionedDepError,
		},
		{
			name:    "multiple dependencies",
			deps:    []string{sampleDep, "second-dep"},
			wantErr: expectSampleDepError, // Same expectation as single dep
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := lang.InstallDependencies(tmpDir, tt.deps)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// testCabalFailure tests cabal failure scenarios for Haskell language
func testCabalFailure(t *testing.T, lang language.Manager, cabalScript, expectedError string) {
	t.Helper()
	tempDir := t.TempDir()

	// Create a mock cabal executable
	mockBinDir := filepath.Join(tempDir, "mockbin")
	if err := os.MkdirAll(mockBinDir, 0o750); err != nil {
		t.Fatalf("Failed to create mock bin directory: %v", err)
	}

	cabalExec := filepath.Join(mockBinDir, "cabal")
	if err := os.WriteFile(cabalExec, []byte(cabalScript), 0o600); err != nil {
		t.Fatalf("Failed to create mock cabal executable: %v", err)
	}
	if err := os.Chmod(cabalExec, 0o700); err != nil { //nolint:gosec // Test executable needs exec permission
		t.Fatalf("Failed to make cabal script executable: %v", err)
	}

	// Temporarily modify PATH
	originalPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", originalPath); err != nil {
			t.Logf("Failed to restore PATH: %v", err)
		}
	}()
	if err := os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("Failed to set PATH: %v", err)
	}

	// Test the failure scenario
	err := lang.InstallDependencies(tempDir, []string{"test-package"})
	if err == nil {
		t.Error("InstallDependencies should fail with mock cabal failure")
	} else {
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain '%s', got: %v", expectedError, err)
		} else {
			t.Logf("InstallDependencies correctly failed: %v", err)
		}
	}
}

// testEnvironmentHealthComprehensive tests comprehensive environment health scenarios
func testEnvironmentHealthComprehensive(
	t *testing.T,
	lang language.Manager,
	configFileName, validConfig, invalidConfig string,
) {
	t.Helper()
	tempDir := t.TempDir()

	t.Run("HealthyEnvironmentWithConfig", func(t *testing.T) {
		// Create mock environment with config file
		envPath := filepath.Join(tempDir, lang.GetName()+"-env-with-config")
		err := os.MkdirAll(envPath, 0o750)
		if err != nil {
			t.Fatalf("Failed to create env directory: %v", err)
		}

		// Create mock config file
		configPath := filepath.Join(envPath, configFileName)
		err = os.WriteFile(configPath, []byte(validConfig), 0o600)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", configFileName, err)
		}

		// Test health check
		isHealthy := lang.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth with %s: %v", configFileName, isHealthy)
	})

	t.Run("EnvironmentWithoutConfig", func(t *testing.T) {
		// Test environment without config file
		envPath := filepath.Join(tempDir, lang.GetName()+"-env-no-config")
		err := os.MkdirAll(envPath, 0o750)
		if err != nil {
			t.Fatalf("Failed to create env directory: %v", err)
		}

		isHealthy := lang.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth without %s: %v", configFileName, isHealthy)
	})

	t.Run("NonExistentEnvironment", func(t *testing.T) {
		// Test with non-existent environment
		isHealthy := lang.CheckEnvironmentHealth("/non/existent/path")
		if isHealthy {
			t.Error("CheckEnvironmentHealth should return false for non-existent environment")
		}
	})

	t.Run("EnvironmentWithInvalidConfig", func(t *testing.T) {
		// Create environment with invalid config file
		envPath := filepath.Join(tempDir, lang.GetName()+"-env-invalid-config")
		err := os.MkdirAll(envPath, 0o750)
		if err != nil {
			t.Fatalf("Failed to create env directory: %v", err)
		}

		// Create invalid config file
		configPath := filepath.Join(envPath, configFileName)
		err = os.WriteFile(configPath, []byte(invalidConfig), 0o600)
		if err != nil {
			t.Fatalf("Failed to create invalid %s: %v", configFileName, err)
		}

		isHealthy := lang.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth with invalid %s: %v", configFileName, isHealthy)
	})

	t.Run("CheckBaseHealthFailure", func(t *testing.T) {
		// Test when base health check fails
		isHealthy := lang.CheckEnvironmentHealth("")
		t.Logf("CheckEnvironmentHealth with empty path: %v", isHealthy)
	})
}

// testRMockEnvironment tests R environment health with mocked R executables
func testRMockEnvironment(t *testing.T, lang language.Manager, rScript, rscriptScript string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "test-r-mock-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	// Create library subdirectory
	libPath := filepath.Join(tempDir, "library")
	err = os.MkdirAll(libPath, 0o750)
	if err != nil {
		t.Fatalf("Failed to create library directory: %v", err)
	}

	// Create mock R executable
	mockRPath := filepath.Join(tempDir, "R")
	err = os.WriteFile(mockRPath, []byte(rScript), 0o600)
	if err != nil {
		t.Fatalf("Failed to create mock R executable: %v", err)
	}
	if err = os.Chmod(mockRPath, 0o700); err != nil { //nolint:gosec // Test executable needs exec permission
		t.Fatalf("Failed to make R script executable: %v", err)
	}

	// Create mock Rscript executable
	mockRscriptPath := filepath.Join(tempDir, "Rscript")
	err = os.WriteFile(mockRscriptPath, []byte(rscriptScript), 0o600)
	if err != nil {
		t.Fatalf("Failed to create mock Rscript executable: %v", err)
	}
	if err = os.Chmod(mockRscriptPath, 0o700); err != nil { //nolint:gosec // Test executable needs exec permission
		t.Fatalf("Failed to make Rscript script executable: %v", err)
	}

	// Temporarily modify PATH to include our mock executables
	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", tempDir+":"+originalPath); err != nil {
		t.Fatalf("Failed to set PATH: %v", err)
	}
	defer func() {
		if err := os.Setenv("PATH", originalPath); err != nil {
			t.Logf("Failed to restore PATH: %v", err)
		}
	}()

	// Test CheckEnvironmentHealth
	result := lang.CheckEnvironmentHealth(tempDir)
	t.Logf("CheckEnvironmentHealth result: %v", result)
}

// testDotnetEnvironmentHealthWithMock tests dotnet environment health with a mock dotnet executable
func testDotnetEnvironmentHealthWithMock(
	t *testing.T,
	lang language.Manager,
	csprojContent string,
	dotnetScript string,
	expectedResult bool,
	testDescription string,
) {
	t.Helper()
	tempDir := t.TempDir()
	projectPath := filepath.Join(tempDir, "PreCommitEnv")

	if err := os.MkdirAll(projectPath, 0o750); err != nil {
		t.Fatalf("Failed to create project directory: %v", err)
	}

	// Create .csproj file
	csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
	if err := os.WriteFile(csprojPath, []byte(csprojContent), 0o600); err != nil {
		t.Fatalf("Failed to create .csproj file: %v", err)
	}

	// Create environment bin directory with mock dotnet executable
	binPath := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binPath, 0o750); err != nil {
		t.Fatalf("Failed to create bin directory: %v", err)
	}

	mockDotnet := filepath.Join(binPath, "dotnet")
	if err := os.WriteFile(mockDotnet, []byte(dotnetScript), 0o600); err != nil {
		t.Fatalf("Failed to create mock dotnet script: %v", err)
	}
	if err := os.Chmod(mockDotnet, 0o700); err != nil { //nolint:gosec // Test executable needs exec permission
		t.Fatalf("Failed to make dotnet script executable: %v", err)
	}

	// ALSO add mock dotnet to PATH for the build command
	tempBinDir := t.TempDir()
	pathMockDotnet := filepath.Join(tempBinDir, "dotnet")
	if err := os.WriteFile(pathMockDotnet, []byte(dotnetScript), 0o600); err != nil {
		t.Fatalf("Failed to create PATH mock dotnet script: %v", err)
	}
	if err := os.Chmod(pathMockDotnet, 0o700); err != nil { //nolint:gosec // Test executable needs exec permission
		t.Fatalf("Failed to make PATH dotnet script executable: %v", err)
	}

	// Temporarily modify PATH
	originalPath := os.Getenv("PATH")
	defer func() {
		if err := os.Setenv("PATH", originalPath); err != nil {
			t.Logf("Failed to restore PATH: %v", err)
		}
	}()

	if err := os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("Failed to set PATH: %v", err)
	}

	// Pass tempDir as the environment path, function will look for bin/dotnet inside it
	result := lang.CheckEnvironmentHealth(tempDir)
	if result != expectedResult {
		t.Errorf("%s: expected %v, got %v", testDescription, expectedResult, result)
	} else {
		t.Logf("%s: correctly returned %v", testDescription, result)
	}
}

// testRubyEnvironmentHealthWithMock tests Ruby environment health with mocked gem/bundle executables
func testRubyEnvironmentHealthWithMock(t *testing.T, ruby language.Manager) {
	t.Helper()

	mockEnv := setupRubyMockEnvironment(t)
	defer mockEnv.cleanup()

	runRubyHealthTests(t, ruby, mockEnv.tmpDir)
}

// rubyMockEnvironment holds mock environment setup for Ruby tests
type rubyMockEnvironment struct {
	cleanup      func()
	tmpDir       string
	binDir       string
	originalPath string
}

// setupRubyMockEnvironment creates a mock Ruby environment with executables
func setupRubyMockEnvironment(t *testing.T) *rubyMockEnvironment {
	t.Helper()

	// Create temporary directory for test environment
	tmpDir, err := os.MkdirTemp("", "ruby_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create mock bin directory
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	// Create mock executables
	createRubyMockExecutables(t, binDir)

	// Set PATH to include our mock binaries
	originalPath := os.Getenv("PATH")
	mockPath := binDir + ":" + originalPath
	if err := os.Setenv("PATH", mockPath); err != nil {
		t.Fatalf("Failed to set PATH: %v", err)
	}

	cleanup := func() {
		if err := os.Setenv("PATH", originalPath); err != nil {
			t.Logf("Warning: failed to restore PATH: %v", err)
		}
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}

	return &rubyMockEnvironment{
		tmpDir:       tmpDir,
		binDir:       binDir,
		originalPath: originalPath,
		cleanup:      cleanup,
	}
}

// runRubyHealthTests runs Ruby health check tests in various scenarios
func runRubyHealthTests(t *testing.T, ruby language.Manager, tmpDir string) {
	t.Helper()

	t.Run("NonExistentEnvironment", func(t *testing.T) {
		health := ruby.CheckEnvironmentHealth(filepath.Join(tmpDir, "nonexistent"))
		if health {
			t.Error("CheckEnvironmentHealth should return false for non-existent environment")
		}
	})

	t.Run("EmptyEnvironment", func(t *testing.T) {
		emptyDir := filepath.Join(tmpDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o750); err != nil {
			t.Fatalf("Failed to create empty dir: %v", err)
		}

		health := ruby.CheckEnvironmentHealth(emptyDir)
		t.Logf("CheckEnvironmentHealth for empty environment: %t", health)
	})

	t.Run("EnvironmentWithoutGemfile", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "no_gemfile")
		if err := os.MkdirAll(testDir, 0o750); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		health := ruby.CheckEnvironmentHealth(testDir)
		t.Logf("CheckEnvironmentHealth without Gemfile: %t", health)
	})

	t.Run("EnvironmentWithGemfile", func(t *testing.T) {
		testDir := createRubyTestDirWithGemfile(t, tmpDir, "with_gemfile", "valid")
		health := ruby.CheckEnvironmentHealth(testDir)
		if !health {
			t.Error(
				"CheckEnvironmentHealth should return true for environment with valid Gemfile and mocked bundle",
			)
		}
	})

	t.Run("EnvironmentWithInvalidGemfile", func(t *testing.T) {
		testDir := createRubyTestDirWithGemfile(t, tmpDir, "invalid_gemfile", "invalid")
		health := ruby.CheckEnvironmentHealth(testDir)
		t.Logf("CheckEnvironmentHealth with invalid Gemfile: %t", health)
	})
}

// createRubyMockExecutables creates mock ruby, gem, and bundle executables
func createRubyMockExecutables(t *testing.T, binDir string) {
	t.Helper()

	// Create mock ruby executable
	rubyPath := filepath.Join(binDir, "ruby")
	rubyScript := `#!/bin/bash
# Mock ruby executable that always succeeds
echo "Mock ruby command: $*"
exit 0
`
	//nolint:gosec // Test file with controlled content
	if err := os.WriteFile(rubyPath, []byte(rubyScript), 0o600); err != nil {
		t.Fatalf("Failed to create mock ruby: %v", err)
	}
	//nolint:gosec // Test executable needs exec permission
	if err := os.Chmod(rubyPath, 0o700); err != nil {
		t.Fatalf("Failed to make ruby executable: %v", err)
	}

	// Create mock gem executable
	gemPath := filepath.Join(binDir, "gem")
	gemScript := `#!/bin/bash
# Mock gem executable that logs commands and succeeds
echo "Mock gem install: $*" >> ` + filepath.Join(filepath.Dir(binDir), "gem.log") + `
if [ "$1" = "install" ]; then
    echo "Successfully installed $2"
    exit 0
fi
exit 0
`
	//nolint:gosec // Test file with controlled content
	if err := os.WriteFile(gemPath, []byte(gemScript), 0o600); err != nil {
		t.Fatalf("Failed to create mock gem: %v", err)
	}
	//nolint:gosec // Test executable needs exec permission
	if err := os.Chmod(gemPath, 0o700); err != nil {
		t.Fatalf("Failed to make gem executable: %v", err)
	}

	// Create mock bundle executable
	bundlePath := filepath.Join(binDir, "bundle")
	bundleScript := `#!/bin/bash
# Mock bundle executable that logs commands and succeeds
echo "Mock bundle: $*" >> ` + filepath.Join(filepath.Dir(binDir), "bundle.log") + `
if [ "$1" = "install" ]; then
    echo "Bundle complete! Successfully installed gems"
    exit 0
elif [ "$1" = "check" ]; then
    echo "The Gemfile's dependencies are satisfied"
    exit 0
fi
exit 0
`
	//nolint:gosec // Test file with controlled content
	if err := os.WriteFile(bundlePath, []byte(bundleScript), 0o600); err != nil {
		t.Fatalf("Failed to create mock bundle: %v", err)
	}
	//nolint:gosec // Test executable needs exec permission
	if err := os.Chmod(bundlePath, 0o700); err != nil {
		t.Fatalf("Failed to make bundle executable: %v", err)
	}
}

// createRubyTestDirWithGemfile creates a test directory with a Gemfile
func createRubyTestDirWithGemfile(t *testing.T, tmpDir, dirName, gemfileType string) string {
	t.Helper()

	testDir := filepath.Join(tmpDir, dirName)
	if err := os.MkdirAll(testDir, 0o750); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	// Create gems directory (required by Ruby CheckHealth)
	gemsDir := filepath.Join(testDir, "gems")
	if err := os.MkdirAll(gemsDir, 0o750); err != nil {
		t.Fatalf("Failed to create gems dir: %v", err)
	}

	// Create Gemfile with appropriate content
	gemfilePath := filepath.Join(testDir, "Gemfile")
	var gemfileContent string
	if gemfileType == "valid" {
		gemfileContent = `source 'https://rubygems.org'
gem 'json'
`
	} else {
		gemfileContent = `invalid ruby syntax here!`
	}

	if err := os.WriteFile(gemfilePath, []byte(gemfileContent), 0o600); err != nil {
		t.Fatalf("Failed to create Gemfile: %v", err)
	}

	return testDir
}

// runRubyDependencyTests runs Ruby dependency installation tests
func runRubyDependencyTests(t *testing.T, ruby language.Manager, tmpDir string) {
	t.Helper()

	t.Run("InstallSingleDependency", func(t *testing.T) {
		testDir := createRubyTestDirWithGems(t, tmpDir, "single_dep")
		deps := []string{"json"}
		err := ruby.InstallDependencies(testDir, deps)
		if err != nil {
			t.Errorf("InstallDependencies failed: %v", err)
		}

		verifyGemfileCreated(t, testDir)
		verifyBundleExecuted(t, tmpDir)
	})

	t.Run("InstallMultipleDependencies", func(t *testing.T) {
		testDir := createRubyTestDirWithGems(t, tmpDir, "multi_deps")
		deps := []string{"json", "rake", "minitest"}
		err := ruby.InstallDependencies(testDir, deps)
		if err != nil {
			t.Errorf("InstallDependencies failed: %v", err)
		}

		verifyGemfileContainsDeps(t, testDir, deps)
	})

	t.Run("InstallNoDependencies", func(t *testing.T) {
		testDir := createRubyTestDirWithGems(t, tmpDir, "no_deps")
		deps := []string{}
		err := ruby.InstallDependencies(testDir, deps)
		if err != nil {
			t.Errorf("InstallDependencies with no deps should succeed: %v", err)
		}
	})
}

// createRubyTestDirWithGems creates a test directory with gems subdirectory
func createRubyTestDirWithGems(t *testing.T, tmpDir, dirName string) string {
	t.Helper()

	testDir := filepath.Join(tmpDir, dirName)
	if err := os.MkdirAll(testDir, 0o750); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	// Create gems directory (required by Ruby CheckHealth)
	gemsDir := filepath.Join(testDir, "gems")
	if err := os.MkdirAll(gemsDir, 0o750); err != nil {
		t.Fatalf("Failed to create gems dir: %v", err)
	}

	return testDir
}

// verifyGemfileCreated verifies that a Gemfile was created
func verifyGemfileCreated(t *testing.T, testDir string) {
	t.Helper()

	gemfilePath := filepath.Join(testDir, "Gemfile")
	if _, err := os.Stat(gemfilePath); os.IsNotExist(err) {
		t.Error("Gemfile was not created")
	}
}

// verifyBundleExecuted verifies that bundle command was executed
func verifyBundleExecuted(t *testing.T, tmpDir string) {
	t.Helper()

	bundleLogPath := filepath.Join(tmpDir, "bundle.log")
	if _, err := os.Stat(bundleLogPath); os.IsNotExist(err) {
		t.Error("Bundle command was not executed")
	}
}

// verifyGemfileContainsDeps verifies that Gemfile contains specified dependencies
func verifyGemfileContainsDeps(t *testing.T, testDir string, deps []string) {
	t.Helper()

	gemfilePath := filepath.Join(testDir, "Gemfile")
	//nolint:gosec // Test file with controlled path
	content, err := os.ReadFile(gemfilePath)
	if err != nil {
		t.Fatalf("Failed to read Gemfile: %v", err)
	}

	for _, dep := range deps {
		if !strings.Contains(string(content), dep) {
			t.Errorf("Gemfile does not contain dependency: %s", dep)
		}
	}
}

// testRubyInstallDependenciesWithMock tests Ruby dependency installation with mocked executables
func testRubyInstallDependenciesWithMock(t *testing.T, ruby language.Manager) {
	t.Helper()

	mockEnv := setupRubyMockEnvironment(t)
	defer mockEnv.cleanup()

	runRubyDependencyTests(t, ruby, mockEnv.tmpDir)
}

// testRubyWithComprehensiveMocks creates a comprehensive Ruby test environment with all mocks
func testRubyWithComprehensiveMocks(t *testing.T, ruby language.Manager,
	testFunc func(*testing.T, language.Manager, string),
) {
	t.Helper()

	mockEnv := setupRubyMockEnvironment(t)
	defer mockEnv.cleanup()

	testFunc(t, ruby, mockEnv.tmpDir)
}

// testRubyCheckHealthWithMocks tests Ruby CheckHealth with mocks
func testRubyCheckHealthWithMocks(t *testing.T, ruby language.Manager, tmpDir string) {
	t.Helper()

	// Test environment health check
	testDir := createRubyTestDirWithGemfile(t, tmpDir, "health_check", "valid")
	health := ruby.CheckEnvironmentHealth(testDir)
	if !health {
		t.Error("CheckEnvironmentHealth should return true with mocked executables")
	}
}

// testRubySetupEnvironmentWithMocks tests Ruby SetupEnvironmentWithRepo with mocks
func testRubySetupEnvironmentWithMocks(t *testing.T, ruby language.Manager, tmpDir string) {
	t.Helper()

	// Test setting up environment with repo
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	envPath, err := ruby.SetupEnvironmentWithRepo(
		tmpDir,
		"2.7.0",
		repoDir,
		"https://example.com/repo.git",
		[]string{},
	)
	if err != nil {
		t.Errorf("SetupEnvironmentWithRepo failed: %v", err)
	}
	if envPath == "" {
		t.Error("SetupEnvironmentWithRepo returned empty environment path")
	}
}
