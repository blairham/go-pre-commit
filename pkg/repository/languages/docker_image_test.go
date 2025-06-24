package languages

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

const (
	testDockerExecutable = "docker"
)

func TestDockerImageLanguage(t *testing.T) {
	t.Run("NewDockerImageLanguage", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()
		if dockerImage == nil {
			t.Error("NewDockerImageLanguage() returned nil")
			return
		}
		if dockerImage.Base == nil {
			t.Error("NewDockerImageLanguage() returned instance with nil Base")
		}

		// Check properties
		if dockerImage.Name != "Docker Image" {
			t.Errorf("Expected name 'Docker Image', got '%s'", dockerImage.Name)
		}
		if dockerImage.ExecutableName != testDockerExecutable {
			t.Errorf("Expected executable name '%s', got '%s'", testDockerExecutable, dockerImage.ExecutableName)
		}
		if dockerImage.VersionFlag != testVersionFlag {
			t.Errorf("Expected version flag '%s', got '%s'", testVersionFlag, dockerImage.VersionFlag)
		}
		if dockerImage.InstallURL != "https://docs.docker.com/get-docker/" {
			t.Errorf("Expected install URL 'https://docs.docker.com/get-docker/', got '%s'", dockerImage.InstallURL)
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()
		tempDir := t.TempDir()

		// Should return the repository path since Docker images don't need separate environments
		envPath, err := dockerImage.SetupEnvironmentWithRepo(tempDir, "latest", tempDir, "dummy-url", []string{})
		if err != nil {
			// May fail if Docker is not available, but should handle gracefully
			t.Logf("SetupEnvironmentWithRepo failed (expected if Docker not available): %v", err)
		} else {
			if envPath != tempDir {
				t.Errorf("SetupEnvironmentWithRepo() should return repo path, got: %s, want: %s", envPath, tempDir)
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_EmptyPaths", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()

		// Test with empty paths
		envPath, err := dockerImage.SetupEnvironmentWithRepo("", "", "", "", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with empty paths failed (expected): %v", err)
		} else {
			if envPath != "" {
				t.Errorf("SetupEnvironmentWithRepo() with empty repo path should return empty path, got: %s", envPath)
			}
		}
	})

	t.Run("InstallDependencies", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()

		// Should not error when installing dependencies (no-op with warning)
		err := dockerImage.InstallDependencies("/dummy/path", []string{"dep1", "dep2"})
		if err != nil {
			t.Errorf("InstallDependencies() returned error: %v", err)
		}

		// Should handle empty dependencies
		err = dockerImage.InstallDependencies("/dummy/path", []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		// Should handle nil dependencies
		err = dockerImage.InstallDependencies("/dummy/path", nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}
	})

	t.Run("CheckHealth", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()
		tempDir := t.TempDir()

		// Test 1: Should return error for non-existent environment
		err := dockerImage.CheckHealth("/non/existent/path", "latest")
		if err == nil {
			t.Error("CheckHealth() should return error for non-existent environment")
		}

		// Test 2: Should check Docker daemon when directory exists
		if mkdirErr := os.MkdirAll(tempDir, 0o755); mkdirErr != nil {
			t.Fatalf("Failed to create test directory: %v", mkdirErr)
		}

		// Skip if Docker is not available
		if _, lookErr := exec.LookPath(testDockerExecutable); lookErr != nil {
			t.Skip("Skipping Docker daemon test: docker not found in PATH")
		}

		err = dockerImage.CheckHealth(tempDir, "latest")
		// This may fail if Docker is not running, but it shouldn't panic
		// We're testing that the code executes without crashing
		_ = err // We don't check the specific error since Docker availability varies
	})

	t.Run("CheckHealth_EmptyPath", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()

		// Should handle empty paths gracefully
		err := dockerImage.CheckHealth("", "")
		if err == nil {
			t.Error("CheckHealth() with empty path should return error")
		}
	})

	t.Run("SetupEnvironmentWithRepo_ErrorCoverage", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()
		tempDir := t.TempDir()

		// Test the case where Docker might not be available
		// We can't easily mock IsRuntimeAvailable, but we can test different scenarios
		envPath, err := dockerImage.SetupEnvironmentWithRepo(tempDir, "latest", tempDir, "dummy-url", []string{})

		// The function should either succeed (if Docker is available) or fail gracefully (if not)
		if err != nil {
			// Docker not available - this should trigger the error path
			if envPath != "" {
				t.Error("SetupEnvironmentWithRepo() should return empty path when error occurs")
			}
			t.Logf("Expected error when Docker not available: %v", err)
		} else {
			// Docker available - should return repo path
			if envPath != tempDir {
				t.Errorf("SetupEnvironmentWithRepo() should return repo path, got: %s, want: %s", envPath, tempDir)
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_DockerNotAvailable", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()
		tempDir := t.TempDir()

		// Temporarily modify PATH to make docker unavailable
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		// Set PATH to a directory that doesn't contain docker
		emptyDir := tempDir + "/empty"
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should trigger the "docker runtime not found" path
		envPath, err := dockerImage.SetupEnvironmentWithRepo(tempDir, "latest", tempDir, "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo() should return error when Docker not available")
		} else {
			// Should return the expected error message
			expectedErrMsg := "docker runtime not found in PATH"
			if !strings.Contains(err.Error(), expectedErrMsg) {
				t.Errorf("Expected error to contain '%s', got: %v", expectedErrMsg, err)
			}
		}

		if envPath != "" {
			t.Error("SetupEnvironmentWithRepo() should return empty path when Docker not available")
		}
	})

	t.Run("CheckHealth_DockerDaemonError", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()
		tempDir := t.TempDir()

		// Create the directory so it passes the directory check
		if err := os.MkdirAll(tempDir, 0o755); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		// Temporarily modify PATH to make docker unavailable to force daemon error
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		emptyDir := tempDir + "/empty"
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should pass the directory check but fail the docker daemon check
		err := dockerImage.CheckHealth(tempDir, "latest")
		if err == nil {
			t.Error("CheckHealth() should return error when Docker daemon not accessible")
		} else {
			expectedErrMsg := "docker daemon is not accessible"
			if !strings.Contains(err.Error(), expectedErrMsg) {
				t.Errorf("Expected error to contain '%s', got: %v", expectedErrMsg, err)
			}
		}
	})

	// Additional comprehensive tests for better coverage
	t.Run("ComprehensiveCoverage", func(t *testing.T) {
		dockerImage := NewDockerImageLanguage()

		t.Run("NewDockerImageLanguage_Properties", func(t *testing.T) {
			if dockerImage.Base == nil {
				t.Fatal("Base should not be nil")
			}

			// Test inherited methods from Base work
			if dockerImage.GetName() != "Docker Image" {
				t.Errorf("Expected GetName() to return 'Docker Image', got '%s'", dockerImage.GetName())
			}
			if dockerImage.GetExecutableName() != testDockerExecutable {
				t.Errorf("Expected GetExecutableName() to return 'docker', got '%s'", dockerImage.GetExecutableName())
			}
		})

		t.Run("SetupEnvironmentWithRepo_VariousVersions", func(t *testing.T) {
			tempDir := t.TempDir()

			// Test various version formats
			versions := []string{"latest", "1.0", "alpine", "ubuntu:20.04", ""}
			for _, version := range versions {
				envPath, err := dockerImage.SetupEnvironmentWithRepo(
					tempDir,
					version,
					tempDir,
					"https://example.com",
					[]string{},
				)
				if err != nil {
					t.Logf("SetupEnvironmentWithRepo() with version '%s' failed "+
						"(may be expected if Docker not available): %v", version, err)
				} else {
					if envPath != tempDir {
						t.Errorf("SetupEnvironmentWithRepo() with version '%s' should return repo path %s, got %s", version, tempDir, envPath)
					}
				}
			}
		})

		t.Run("InstallDependencies_EdgeCases", func(t *testing.T) {
			tempDir := t.TempDir()

			// Test with very long dependency list
			longDepList := make([]string, 100)
			for i := range 100 {
				longDepList[i] = "dep" + string(rune(i))
			}
			err := dockerImage.InstallDependencies(tempDir, longDepList)
			if err != nil {
				t.Errorf("InstallDependencies() with long dep list returned error: %v", err)
			}

			// Test with special characters in dependency names
			specialDeps := []string{"dep-with-dash", "dep.with.dots", "dep_with_underscores", "dep@version"}
			err = dockerImage.InstallDependencies(tempDir, specialDeps)
			if err != nil {
				t.Errorf("InstallDependencies() with special deps returned error: %v", err)
			}

			// Test with non-existent directory
			err = dockerImage.InstallDependencies("/non/existent/path", []string{"dep1"})
			if err != nil {
				t.Errorf("InstallDependencies() with non-existent path returned error: %v", err)
			}
		})

		t.Run("CheckEnvironmentHealth", func(t *testing.T) {
			tempDir := t.TempDir()

			// Create an environment directory
			envPath := tempDir
			if err := os.MkdirAll(envPath, 0o755); err != nil {
				t.Fatalf("Failed to create env directory: %v", err)
			}
			// Test CheckEnvironmentHealth method if it exists
			// Docker Image doesn't override CheckEnvironmentHealth, so it uses the base implementation
			healthy := dockerImage.CheckEnvironmentHealth(envPath)
			if healthy {
				t.Error("CheckEnvironmentHealth() should return false for Docker Image language " +
					"(no specific executable in environment)")
			}

			// Test with non-existent path
			healthy = dockerImage.CheckEnvironmentHealth("/non/existent/path")
			if healthy {
				t.Error("CheckEnvironmentHealth() should return false for non-existent path")
			}

			// Test with empty path
			healthy = dockerImage.CheckEnvironmentHealth("")
			if healthy {
				t.Error("CheckEnvironmentHealth() should return false for empty path")
			}
		})

		t.Run("IsRuntimeAvailable", func(t *testing.T) {
			// Test IsRuntimeAvailable method - should check for docker executable in PATH
			available := dockerImage.IsRuntimeAvailable()
			// Don't assert specific value since Docker availability varies
			t.Logf("IsRuntimeAvailable() returned: %v", available)
		})

		t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
			tempDir := t.TempDir()

			// Test PreInitializeEnvironmentWithRepoInfo method if available
			// Docker Image inherits this from Base, so should work
			err := dockerImage.PreInitializeEnvironmentWithRepoInfo(
				tempDir,
				"latest",
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

			// Test SetupEnvironmentWithRepoInfo method if available
			envPath, err := dockerImage.SetupEnvironmentWithRepoInfo(
				tempDir,
				"latest",
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
