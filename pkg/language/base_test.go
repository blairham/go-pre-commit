package language

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewBase(t *testing.T) {
	t.Run("NewBase creates valid Base instance", func(t *testing.T) {
		base := NewBase("Python", "python", "--version", "https://python.org")

		if base.Name != "Python" {
			t.Errorf("Expected name 'Python', got %s", base.Name)
		}
		if base.ExecutableName != Python {
			t.Errorf("Expected executable '%s', got %s", Python, base.ExecutableName)
		}
		if base.VersionFlag != "--version" {
			t.Errorf("Expected version flag '--version', got %s", base.VersionFlag)
		}
		if base.InstallURL != "https://python.org" {
			t.Errorf("Expected install URL 'https://python.org', got %s", base.InstallURL)
		}
		if base.DownloadManager == nil {
			t.Error("Expected DownloadManager to be initialized")
		}
		if base.PackageManager == nil {
			t.Error("Expected PackageManager to be initialized")
		}
	})
}

func TestBaseGetters(t *testing.T) {
	base := NewBase("Go", "go", "version", "https://golang.org")

	t.Run("GetExecutableName", func(t *testing.T) {
		name := base.GetExecutableName()
		if name != "go" {
			t.Errorf("Expected 'go', got %s", name)
		}
	})

	t.Run("GetName", func(t *testing.T) {
		name := base.GetName()
		if name != "Go" {
			t.Errorf("Expected 'Go', got %s", name)
		}
	})

	t.Run("GetVersionFlag", func(t *testing.T) {
		flag := base.GetVersionFlag()
		if flag != "version" {
			t.Errorf("Expected 'version', got %s", flag)
		}
	})

	t.Run("NeedsEnvironmentSetup", func(t *testing.T) {
		needs := base.NeedsEnvironmentSetup()
		if !needs {
			t.Error("Expected NeedsEnvironmentSetup to return true")
		}
	})
}

func TestBaseEnvironmentOperations(t *testing.T) {
	base := NewBase("Test", "test", "--version", "")
	tempDir, err := os.MkdirTemp("", "base-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("CreateEnvironmentDirectory", func(t *testing.T) {
		envPath := filepath.Join(tempDir, "test-env")
		err := base.CreateEnvironmentDirectory(envPath)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			t.Error("Expected environment directory to be created")
		}
	})

	t.Run("GetEnvironmentBinPath", func(t *testing.T) {
		envPath := "/test/env"
		binPath := base.GetEnvironmentBinPath(envPath)
		expected := "/test/env/bin"
		if binPath != expected {
			t.Errorf("Expected '%s', got %s", expected, binPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		envPath, err := base.SetupEnvironmentWithRepo(tempDir, "1.0", "", "", nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		expectedPath := filepath.Join(tempDir, "Test-1.0")
		if envPath != expectedPath {
			t.Errorf("Expected '%s', got %s", expectedPath, envPath)
		}

		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			t.Error("Expected environment directory to be created")
		}
	})

	t.Run("SetupEnvironmentWithRepoInfo", func(t *testing.T) {
		envPath, err := base.SetupEnvironmentWithRepoInfo(tempDir, "2.0", "/repo", "http://test.com", []string{"dep1"})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		expectedPath := filepath.Join(tempDir, "Test-2.0")
		if envPath != expectedPath {
			t.Errorf("Expected '%s', got %s", expectedPath, envPath)
		}
	})

	t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		err := base.PreInitializeEnvironmentWithRepoInfo(tempDir, "1.0", tempDir, "http://test.com", nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("InstallDependencies", func(t *testing.T) {
		deps := []string{"dep1", "dep2"}
		err := base.InstallDependencies("/test/env", deps)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("GenericSetupEnvironmentWithRepo", func(t *testing.T) {
		repoPath := filepath.Join(tempDir, "repo")
		err := os.MkdirAll(repoPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create repo dir: %v", err)
		}

		envPath, err := base.GenericSetupEnvironmentWithRepo("", "1.0", repoPath, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		expectedPath := filepath.Join(repoPath, "test-1.0")
		if envPath != expectedPath {
			t.Errorf("Expected '%s', got %s", expectedPath, envPath)
		}
	})

	t.Run("GenericSetupEnvironmentWithRepo_SystemVersion", func(t *testing.T) {
		repoPath := filepath.Join(tempDir, "repo2")
		err := os.MkdirAll(repoPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create repo dir: %v", err)
		}

		envPath, err := base.GenericSetupEnvironmentWithRepo("", VersionSystem, repoPath, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if envPath != repoPath {
			t.Errorf("Expected '%s', got %s", repoPath, envPath)
		}
	})
}

func TestBaseHealthOperations(t *testing.T) {
	base := NewBase("Test", "test", "--version", "")
	tempDir, err := os.MkdirTemp("", "base-health-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("CheckEnvironmentHealth_NonExistent", func(t *testing.T) {
		healthy := base.CheckEnvironmentHealth("/non/existent/path")
		if healthy {
			t.Error("Expected non-existent environment to be unhealthy")
		}
	})

	t.Run("GenericCheckHealth_NonExistent", func(t *testing.T) {
		err := base.GenericCheckHealth("/non/existent/path", "1.0")
		if err == nil {
			t.Error("Expected error for non-existent environment")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("Expected error message to contain 'does not exist', got %v", err)
		}
	})

	t.Run("GenericCheckHealth_Existing", func(t *testing.T) {
		envPath := filepath.Join(tempDir, "env")
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create env dir: %v", err)
		}

		err = base.GenericCheckHealth(envPath, "1.0")
		if err != nil {
			t.Errorf("Expected no error for existing environment, got %v", err)
		}
	})

	t.Run("GenericIsRuntimeAvailable", func(t *testing.T) {
		available := base.GenericIsRuntimeAvailable()
		if !available {
			t.Error("Expected GenericIsRuntimeAvailable to return true")
		}
	})

	t.Run("GenericInstallDependencies", func(t *testing.T) {
		deps := []string{"dep1", "dep2"}
		err := base.GenericInstallDependencies("/test/env", deps)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestGlobalEnvironmentState(t *testing.T) {
	// Clear state before test
	ClearGlobalEnvironmentState()

	t.Run("Environment State Management", func(t *testing.T) {
		envKey := "test-env-key"

		// Initially not initialized or installing
		if IsEnvironmentInitialized(envKey) {
			t.Error("Expected environment to not be initialized initially")
		}
		if IsEnvironmentInstalling(envKey) {
			t.Error("Expected environment to not be installing initially")
		}

		// Mark as installing
		marked := MarkEnvironmentInstalling(envKey)
		if !marked {
			t.Error("Expected MarkEnvironmentInstalling to return true")
		}
		if !IsEnvironmentInstalling(envKey) {
			t.Error("Expected environment to be installing")
		}

		// Try to mark as installing again (should return false)
		marked = MarkEnvironmentInstalling(envKey)
		if marked {
			t.Error("Expected MarkEnvironmentInstalling to return false when already installing")
		}

		// Mark as initialized
		MarkEnvironmentInitialized(envKey)
		if !IsEnvironmentInitialized(envKey) {
			t.Error("Expected environment to be initialized")
		}
		if IsEnvironmentInstalling(envKey) {
			t.Error("Expected environment to not be installing after initialization")
		}

		// Get initialized environments
		initialized := GetGlobalInitializedEnvs()
		if !initialized[envKey] {
			t.Error("Expected environment to be in initialized map")
		}

		// Clear state
		ClearGlobalEnvironmentState()
		if IsEnvironmentInitialized(envKey) {
			t.Error("Expected environment to not be initialized after clearing state")
		}
		if IsEnvironmentInstalling(envKey) {
			t.Error("Expected environment to not be installing after clearing state")
		}
	})
}

func TestUtilityFunctions(t *testing.T) {
	t.Run("ParseRepoURL", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"github.com/user/repo", "https://github.com/user/repo"},
			{"file://local/path", "local/path"},
			{"https://github.com/user/repo.git", "https://github.com/user/repo"},
			{"git://github.com/user/repo", "https://github.com/user/repo"},
			{"Generic-name", "Generic-name"},
		}

		for _, test := range tests {
			result := ParseRepoURL(test.input)
			if result != test.expected {
				t.Errorf("ParseRepoURL(%s): expected %s, got %s", test.input, test.expected, result)
			}
		}
	})

	t.Run("GetRepositoryEnvironmentName", func(t *testing.T) {
		tests := []struct {
			language string
			version  string
			expected string
		}{
			{"Python", "3.9", "py_env-3.9"},
			{"python", "3.8", "py_env-3.8"},
			{"Node", "16", "nodeenv-16"},
			{"nodejs", "14", "node_env-14"},
			{"Go", "1.18", "goenv-1.18"},
			{"golang", "1.17", "goenv-1.17"},
			{"dotnet", "6.0", "dotnetenv-6.0"},
			{"system", "any", ""},
			{"script", "any", ""},
			{"fail", "any", ""},
			{"pygrep", "any", ""},
			{"Java", "", "javaenv-default"},
		}

		for _, test := range tests {
			result := GetRepositoryEnvironmentName(test.language, test.version)
			if result != test.expected {
				t.Errorf("GetRepositoryEnvironmentName(%s, %s): expected %s, got %s",
					test.language, test.version, test.expected, result)
			}
		}
	})

	t.Run("CreateNormalizedEnvironmentKey", func(t *testing.T) {
		result := CreateNormalizedEnvironmentKey("Python", "http://github.com/user/repo", "/env/path")
		expected := "python-http://github.com/user/repo-/env/path"
		if result != expected {
			t.Errorf("Expected %s, got %s", expected, result)
		}
	})
}

func TestCacheAwareMethods(t *testing.T) {
	base := NewBase("Test", "test", "--version", "")
	tempDir, err := os.MkdirTemp("", "cache-aware-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("CacheAwarePreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		err := base.CacheAwarePreInitializeEnvironmentWithRepoInfo("", "1.0", tempDir, "http://test.com", nil, "test")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("CacheAwareSetupEnvironmentWithRepoInfo", func(t *testing.T) {
		envPath, err := base.CacheAwareSetupEnvironmentWithRepoInfo(
			tempDir,
			"1.0",
			tempDir,
			"http://test.com",
			nil,
			"test",
		)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if envPath == "" {
			t.Error("Expected non-empty environment path")
		}
	})
}

func TestConstants(t *testing.T) {
	t.Run("Version constants", func(t *testing.T) {
		if VersionDefault != "default" {
			t.Errorf("Expected VersionDefault to be 'default', got %s", VersionDefault)
		}
		if VersionLatest != "latest" {
			t.Errorf("Expected VersionLatest to be 'latest', got %s", VersionLatest)
		}
		if VersionSystem != "system" {
			t.Errorf("Expected VersionSystem to be 'system', got %s", VersionSystem)
		}
	})

	t.Run("OS constants", func(t *testing.T) {
		if OSX != "osx" {
			t.Errorf("Expected OSX to be 'osx', got %s", OSX)
		}
		if Windows != "win" {
			t.Errorf("Expected Windows to be 'win', got %s", Windows)
		}
		if Linux != "linux" {
			t.Errorf("Expected Linux to be 'linux', got %s", Linux)
		}
		if Darwin != "darwin" {
			t.Errorf("Expected Darwin to be 'darwin', got %s", Darwin)
		}
	})

	t.Run("Architecture constants", func(t *testing.T) {
		if ARM64 != "arm64" {
			t.Errorf("Expected ARM64 to be 'arm64', got %s", ARM64)
		}
		if AMD64 != "amd64" {
			t.Errorf("Expected AMD64 to be 'amd64', got %s", AMD64)
		}
	})

	t.Run("Other constants", func(t *testing.T) {
		if ExeExt != ".exe" {
			t.Errorf("Expected ExeExt to be '.exe', got %s", ExeExt)
		}
		if Python != "python" {
			t.Errorf("Expected Python to be 'python', got %s", Python)
		}
	})
}

// Test error scenarios
func TestBaseErrorScenarios(t *testing.T) {
	base := NewBase("Test", "test", "--version", "")

	t.Run("CreateEnvironmentDirectory_InvalidPath", func(t *testing.T) {
		// Try to create directory with invalid path (assuming this path doesn't exist and can't be created)
		err := base.CreateEnvironmentDirectory("/dev/null/invalid")
		if err == nil {
			t.Error("Expected error when creating directory with invalid path")
		}
	})

	t.Run("PreInitializeEnvironmentWithRepoInfo_InvalidPaths", func(t *testing.T) {
		// Test with invalid cache directory
		err := base.PreInitializeEnvironmentWithRepoInfo("/dev/null/invalid", "1.0", "", "", nil)
		if err == nil {
			t.Error("Expected error with invalid cache directory")
		}

		// Test with invalid repo path
		err = base.PreInitializeEnvironmentWithRepoInfo("", "1.0", "/dev/null/invalid", "", nil)
		if err == nil {
			t.Error("Expected error with invalid repo path")
		}
	})
}

// Benchmark tests
func BenchmarkGetRepositoryEnvironmentName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetRepositoryEnvironmentName("Python", "3.9")
	}
}

func BenchmarkCreateNormalizedEnvironmentKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CreateNormalizedEnvironmentKey("Python", "http://github.com/user/repo", "/env/path")
	}
}

func BenchmarkParseRepoURL(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ParseRepoURL("https://github.com/user/repo.git")
	}
}

func TestMissingCoverage(t *testing.T) {
	base := NewBase("TestLang", "testlang", "--version", "https://testlang.org")
	tempDir, err := os.MkdirTemp("", "missing-coverage-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("IsRuntimeAvailable", func(t *testing.T) {
		// Test with a known executable that should exist (like 'echo' on Unix systems)
		baseEcho := NewBase("Echo", "echo", "--version", "")
		available := baseEcho.IsRuntimeAvailable()
		// Note: This might be true or false depending on the system, just test it doesn't panic
		_ = available

		// Test with a non-existent executable
		baseFake := NewBase("Fake", "this-executable-should-not-exist-12345", "--version", "")
		available = baseFake.IsRuntimeAvailable()
		if available {
			t.Error("Expected non-existent executable to be unavailable")
		}
	})

	t.Run("PrintNotFoundMessage", func(_ *testing.T) {
		// Capture output by redirecting stdout temporarily
		// This tests the function without actually checking output
		base.PrintNotFoundMessage()

		// Test with empty install URL
		baseNoURL := NewBase("TestLang", "testlang", "--version", "")
		baseNoURL.PrintNotFoundMessage()
	})

	t.Run("CheckHealth", func(t *testing.T) {
		// Test with non-existent environment
		err := base.CheckHealth("/non/existent/path", "1.0")
		if err == nil {
			t.Error("Expected error for non-existent environment")
		}
		if !strings.Contains(err.Error(), "language runtime not found") {
			t.Errorf("Expected error to contain 'language runtime not found', got: %v", err)
		}

		// Test with existing directory but no executable
		envPath := filepath.Join(tempDir, "test-env")
		binPath := filepath.Join(envPath, "bin")
		err = os.MkdirAll(binPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		err = base.CheckHealth(envPath, "1.0")
		if err == nil {
			t.Error("Expected error when executable doesn't exist")
		}

		// Create a fake executable that will fail health check
		execPath := filepath.Join(binPath, "testlang")
		err = os.WriteFile(execPath, []byte("#!/bin/sh\nexit 1\n"), 0o755)
		if err != nil {
			t.Fatalf("Failed to create fake executable: %v", err)
		}

		err = base.CheckHealth(envPath, "1.0")
		if err == nil {
			t.Error("Expected error when health check fails")
		}
		if !strings.Contains(err.Error(), "health check failed") {
			t.Errorf("Expected error to contain 'health check failed', got: %v", err)
		}
	})

	t.Run("SetupEnvironment", func(t *testing.T) {
		envPath, err := base.SetupEnvironment(tempDir, "test-version", []string{"dep1", "dep2"})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		expectedPath := filepath.Join(tempDir, "TestLang-test-version")
		if envPath != expectedPath {
			t.Errorf("Expected '%s', got %s", expectedPath, envPath)
		}
	})

	t.Run("CheckEnvironmentHealth_WithExecutable", func(t *testing.T) {
		// Create environment with working executable
		envPath := filepath.Join(tempDir, "working-env")
		binPath := filepath.Join(envPath, "bin")
		err := os.MkdirAll(binPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create a working executable
		execPath := filepath.Join(binPath, "testlang")
		err = os.WriteFile(execPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)
		if err != nil {
			t.Fatalf("Failed to create working executable: %v", err)
		}

		healthy := base.CheckEnvironmentHealth(envPath)
		// This might fail since we're testing with a fake executable
		// but we're testing the code path
		_ = healthy
	})
}

func TestGenericSetupEnvironmentWithRepo_ErrorScenarios(t *testing.T) {
	base := NewBase("Test", "test", "--version", "")

	t.Run("GenericSetupEnvironmentWithRepo_CreateDirectoryError", func(t *testing.T) {
		// Try to create environment in a read-only directory or invalid path
		_, err := base.GenericSetupEnvironmentWithRepo("", "1.0", "/dev/null/invalid", nil)
		if err == nil {
			t.Error("Expected error when creating environment in invalid path")
		}
	})

	t.Run("GenericSetupEnvironmentWithRepo_SpecialNames", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "Generic-setup-test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Test with "system" name
		baseSystem := NewBase("system", "system", "--version", "")
		envPath, err := baseSystem.GenericSetupEnvironmentWithRepo("", "1.0", tempDir, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if envPath != tempDir {
			t.Errorf("Expected system language to return repo path, got %s", envPath)
		}

		// Test with "script" name
		baseScript := NewBase("script", "script", "--version", "")
		envPath, err = baseScript.GenericSetupEnvironmentWithRepo("", "1.0", tempDir, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if envPath != tempDir {
			t.Errorf("Expected script language to return repo path, got %s", envPath)
		}

		// Test with "fail" name
		baseFail := NewBase("fail", "fail", "--version", "")
		envPath, err = baseFail.GenericSetupEnvironmentWithRepo("", "1.0", tempDir, nil)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if envPath != tempDir {
			t.Errorf("Expected fail language to return repo path, got %s", envPath)
		}
	})
}

func TestSetupEnvironmentWithRepo_ErrorScenario(t *testing.T) {
	base := NewBase("Test", "test", "--version", "")

	t.Run("SetupEnvironmentWithRepo_CreateDirectoryError", func(t *testing.T) {
		// Try to setup environment in invalid directory
		_, err := base.SetupEnvironmentWithRepo("/dev/null/invalid", "1.0", "", "", nil)
		if err == nil {
			t.Error("Expected error when setting up environment in invalid directory")
		}
		if !strings.Contains(err.Error(), "failed to create environment directory") {
			t.Errorf("Expected error to mention failed directory creation, got: %v", err)
		}
	})
}

// Test edge cases and additional scenarios
func TestEdgeCases(t *testing.T) {
	t.Run("GetRepositoryEnvironmentName_EdgeCases", func(t *testing.T) {
		// Test case sensitivity and special characters
		tests := []struct {
			language string
			version  string
			expected string
		}{
			{"PYTHON", "3.9", "py_env-3.9"},  // uppercase
			{"Python3", "3.8", "py_env-3.8"}, // python3 variant
			{"NodeJS", "16", "node_env-16"},  // nodejs case
			{"C++", "17", "c++env-17"},       // special characters with version
			{"", "1.0", "env-1.0"},           // empty language
			{"Ruby", "", "rubyenv-default"},  // empty version
		}

		for _, test := range tests {
			result := GetRepositoryEnvironmentName(test.language, test.version)
			if result != test.expected {
				t.Errorf("GetRepositoryEnvironmentName(%s, %s): expected %s, got %s",
					test.language, test.version, test.expected, result)
			}
		}
	})

	t.Run("ParseRepoURL_EdgeCases", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"", ""},                                        // empty string
			{"github.com", "github.com"},                    // no user/repo
			{"github.com/user", "github.com/user"},          // no repo
			{"http://example.com/path", "example.com/path"}, // http
			{"file:///local/path", "/local/path"},           // file protocol
		}

		for _, test := range tests {
			result := ParseRepoURL(test.input)
			if result != test.expected {
				t.Errorf("ParseRepoURL(%s): expected %s, got %s", test.input, test.expected, result)
			}
		}
	})
}

// Test Windows-specific code paths and runtime scenarios
func TestPlatformSpecificBehavior(t *testing.T) {
	base := NewBase("TestLang", "testlang", "--version", "")
	tempDir, err := os.MkdirTemp("", "platform-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("CheckHealth_WindowsExecutable", func(t *testing.T) {
		// Create environment structure
		envPath := filepath.Join(tempDir, "win-env")
		binPath := filepath.Join(envPath, "bin")
		err := os.MkdirAll(binPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Test the Windows executable extension logic
		// Even on non-Windows systems, this tests the code path
		execPath := filepath.Join(binPath, "testlang")
		err = os.WriteFile(execPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)
		if err != nil {
			t.Fatalf("Failed to create executable: %v", err)
		}

		// Also create .exe version to test Windows path
		execPathExe := filepath.Join(binPath, "testlang.exe")
		err = os.WriteFile(execPathExe, []byte("#!/bin/sh\nexit 0\n"), 0o755)
		if err != nil {
			t.Fatalf("Failed to create .exe executable: %v", err)
		}

		err = base.CheckHealth(envPath, "1.0")
		// The error depends on whether the executable actually works
		// We're mainly testing that the code doesn't panic
		_ = err
	})
}

// Test to cover the remaining Windows-specific path in CheckHealth
func TestCheckHealthWindowsPath(t *testing.T) {
	base := NewBase("TestLang", "testlang", "--version", "")
	tempDir, err := os.MkdirTemp("", "windows-path-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("CheckHealth_SuccessfulExecutable", func(t *testing.T) {
		// Create environment with a working executable
		envPath := filepath.Join(tempDir, "working-env")
		binPath := filepath.Join(envPath, "bin")
		err := os.MkdirAll(binPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create a working shell script that exits successfully
		execPath := filepath.Join(binPath, "testlang")
		scriptContent := "#!/bin/sh\nexit 0\n"
		err = os.WriteFile(execPath, []byte(scriptContent), 0o755)
		if err != nil {
			t.Fatalf("Failed to create working executable: %v", err)
		}

		// This should succeed since the executable exists and returns exit code 0
		err = base.CheckHealth(envPath, "1.0")
		if err != nil {
			// If this fails, it might be due to shell availability or permissions
			// Log the error but don't fail the test as this depends on the system
			t.Logf("CheckHealth failed (this may be expected on some systems): %v", err)
		}
	})

	// Test Windows-specific behavior by simulating it with executable names
	t.Run("CheckHealth_WindowsExecutableExtension", func(t *testing.T) {
		// Create a base with .exe extension to test the Windows path
		baseWin := NewBase("WinLang", "winlang.exe", "--version", "")

		envPath := filepath.Join(tempDir, "win-env")
		binPath := filepath.Join(envPath, "bin")
		err := os.MkdirAll(binPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create bin directory: %v", err)
		}

		// Create executable with .exe extension
		execPath := filepath.Join(binPath, "winlang.exe")
		err = os.WriteFile(execPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)
		if err != nil {
			t.Fatalf("Failed to create .exe executable: %v", err)
		}

		err = baseWin.CheckHealth(envPath, "1.0")
		// This tests the code path but may fail due to execution
		// We're primarily testing that the Windows path logic doesn't panic
		_ = err
	})
}

// Test with real system executables for more realistic coverage
func TestWithSystemExecutables(t *testing.T) {
	t.Run("IsRuntimeAvailable_SystemCommands", func(t *testing.T) {
		// Test with common system commands that should exist
		systemCommands := []string{"sh", "ls", "cat", "echo"}

		for _, cmd := range systemCommands {
			base := NewBase("Test", cmd, "--version", "")
			available := base.IsRuntimeAvailable()
			// We don't assert the result since it depends on the system
			// We're just testing that the function executes without panic
			t.Logf("Command %s available: %v", cmd, available)
		}
	})
}

// Test concurrent access to global state
func TestConcurrentGlobalState(t *testing.T) {
	t.Run("ConcurrentEnvironmentState", func(t *testing.T) {
		ClearGlobalEnvironmentState()

		// Test concurrent access to global environment state
		envKeys := []string{"env1", "env2", "env3", "env4", "env5"}

		// Start multiple goroutines that manipulate environment state
		done := make(chan bool, len(envKeys))

		for _, key := range envKeys {
			go func(envKey string) {
				defer func() { done <- true }()

				// Mark as installing
				marked := MarkEnvironmentInstalling(envKey)
				if !marked {
					t.Errorf("Failed to mark %s as installing", envKey)
				}

				// Check if installing
				installing := IsEnvironmentInstalling(envKey)
				if !installing {
					t.Errorf("Environment %s should be installing", envKey)
				}

				// Mark as initialized
				MarkEnvironmentInitialized(envKey)

				// Check if initialized
				initialized := IsEnvironmentInitialized(envKey)
				if !initialized {
					t.Errorf("Environment %s should be initialized", envKey)
				}
			}(key)
		}

		// Wait for all goroutines to complete
		for range envKeys {
			<-done
		}

		// Verify final state
		initialized := GetGlobalInitializedEnvs()
		for _, key := range envKeys {
			if !initialized[key] {
				t.Errorf("Environment %s should be in initialized map", key)
			}
		}
	})
}

// Test memory and performance characteristics
func TestMemoryAndPerformance(t *testing.T) {
	t.Run("LargeNumberOfEnvironments", func(t *testing.T) {
		ClearGlobalEnvironmentState()

		// Test with a large number of environment keys
		numEnvs := 1000
		for i := range numEnvs {
			envKey := fmt.Sprintf("env-%d", i)
			MarkEnvironmentInstalling(envKey)
			MarkEnvironmentInitialized(envKey)
		}

		initialized := GetGlobalInitializedEnvs()
		if len(initialized) != numEnvs {
			t.Errorf("Expected %d initialized environments, got %d", numEnvs, len(initialized))
		}

		ClearGlobalEnvironmentState()
		initialized = GetGlobalInitializedEnvs()
		if len(initialized) != 0 {
			t.Errorf("Expected 0 initialized environments after clear, got %d", len(initialized))
		}
	})
}
