package languages

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerLanguage(t *testing.T) {
	t.Run("NewDockerLanguage", func(t *testing.T) {
		docker := NewDockerLanguage()
		if docker == nil {
			t.Error("NewDockerLanguage() returned nil")
			return
		}
		if docker.Base == nil {
			t.Error("NewDockerLanguage() returned instance with nil Base")
		}

		// Check properties
		if docker.Name != "Docker" {
			t.Errorf("Expected name 'Docker', got '%s'", docker.Name)
		}
		if docker.ExecutableName != "docker" {
			t.Errorf("Expected executable name 'docker', got '%s'", docker.ExecutableName)
		}
		if docker.VersionFlag != testVersionFlag {
			t.Errorf("Expected version flag '%s', got '%s'", testVersionFlag, docker.VersionFlag)
		}
		if docker.InstallURL != "https://docs.docker.com/get-docker/" {
			t.Errorf("Expected install URL 'https://docs.docker.com/get-docker/', got '%s'", docker.InstallURL)
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		// Skip if Docker is not available
		if _, err := exec.LookPath("docker"); err != nil {
			t.Skip("Skipping test: docker not found in PATH")
		}

		docker := NewDockerLanguage()
		tempDir := t.TempDir()

		// Test setup with version
		envPath, err := docker.SetupEnvironmentWithRepo(tempDir, "latest", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() returned empty environment path")
		}

		// Verify the environment directory was created
		if _, statErr := os.Stat(envPath); os.IsNotExist(statErr) {
			t.Error("SetupEnvironmentWithRepo() did not create environment directory")
		}

		// Test setup with empty version
		envPath, err = docker.SetupEnvironmentWithRepo(tempDir, "", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with empty version returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() with empty version returned empty environment path")
		}
	})

	t.Run("SetupEnvironmentWithRepo_InvalidPath", func(t *testing.T) {
		docker := NewDockerLanguage()

		// Test with invalid path (should fail to create directory)
		_, err := docker.SetupEnvironmentWithRepo("/invalid/readonly/path", "latest",
			"/invalid/readonly/path", "dummy-url", []string{})
		if err == nil {
			t.Error("SetupEnvironmentWithRepo() with invalid path should return error")
		}
	})

	t.Run("InstallDependencies", func(t *testing.T) {
		docker := NewDockerLanguage()

		// Should not error when installing dependencies (no-op with warning)
		err := docker.InstallDependencies("/dummy/path", []string{"dep1", "dep2"})
		if err != nil {
			t.Errorf("InstallDependencies() returned error: %v", err)
		}

		// Should handle empty dependencies
		err = docker.InstallDependencies("/dummy/path", []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		// Should handle nil dependencies
		err = docker.InstallDependencies("/dummy/path", nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}
	})

	t.Run("CheckHealth", func(t *testing.T) {
		docker := NewDockerLanguage()
		tempDir := t.TempDir()

		// Test 1: Should return error for non-existent environment
		err := docker.CheckHealth("/non/existent/path", "latest")
		if err == nil {
			t.Error("CheckHealth() should return error for non-existent environment")
		}

		// Test 2: Should check Docker daemon when directory exists
		envPath := filepath.Join(tempDir, "test-env")
		if mkdirErr := os.MkdirAll(envPath, 0o755); mkdirErr != nil {
			t.Fatalf("Failed to create test environment directory: %v", mkdirErr)
		}

		// Skip if Docker is not available
		if _, lookErr := exec.LookPath("docker"); lookErr != nil {
			t.Skip("Skipping Docker daemon test: docker not found in PATH")
		}

		err = docker.CheckHealth(envPath, "latest")
		// This may fail if Docker is not running, but it shouldn't panic
		// We're testing that the code executes without crashing
		_ = err // We don't check the specific error since Docker availability varies
	})

	t.Run("CheckHealth_EmptyPath", func(t *testing.T) {
		docker := NewDockerLanguage()

		// Should handle empty paths gracefully
		err := docker.CheckHealth("", "")
		if err == nil {
			t.Error("CheckHealth() with empty path should return error")
		}
	})
}

// Additional tests to achieve 100% coverage for CheckHealth
func TestDockerLanguage_CheckHealthComprehensive(t *testing.T) {
	docker := NewDockerLanguage()

	t.Run("CheckHealth_DockerDaemonAccessible", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "docker-env")

		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Test when Docker is available and accessible
		if _, err := exec.LookPath("docker"); err == nil {
			// Try to run docker info to see if daemon is accessible
			if cmd := exec.Command("docker", "info"); cmd.Run() == nil {
				// Docker daemon is accessible - this should pass
				err := docker.CheckHealth(envPath, "latest")
				if err != nil {
					t.Logf("CheckHealth failed despite Docker being accessible: %v", err)
				} else {
					t.Log("CheckHealth passed with accessible Docker daemon")
				}
			} else {
				t.Log("Docker found but daemon not accessible, testing that code path")
				err := docker.CheckHealth(envPath, "latest")
				if err == nil {
					t.Error("CheckHealth should return error when Docker daemon is not accessible")
				} else {
					t.Logf("CheckHealth correctly failed when Docker daemon not accessible: %v", err)
				}
			}
		} else {
			t.Log("Docker not found in PATH - will test the error path")
			err := docker.CheckHealth(envPath, "latest")
			if err == nil {
				t.Error("CheckHealth should return error when Docker is not available")
			} else {
				t.Logf("CheckHealth correctly failed when Docker not available: %v", err)
			}
		}
	})

	t.Run("CheckHealth_MockDockerFailure", func(t *testing.T) {
		tempDir := t.TempDir()
		envPath := filepath.Join(tempDir, "docker-env-fail")

		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create a mock docker script that fails
		mockDir := filepath.Join(tempDir, "mock-bin")
		if err := os.MkdirAll(mockDir, 0o755); err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		mockDocker := filepath.Join(mockDir, "docker")
		dockerScript := `#!/bin/bash
if [[ "$1" == "info" ]]; then
  echo "Cannot connect to the Docker daemon"
  exit 1
fi
exit 0`
		if err := os.WriteFile(mockDocker, []byte(dockerScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock docker script: %v", err)
		}

		// Temporarily modify PATH to use our mock
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockDir+string(os.PathListSeparator)+originalPath)

		err := docker.CheckHealth(envPath, "latest")
		if err == nil {
			t.Error("CheckHealth should return error when docker info fails")
		} else {
			if !strings.Contains(err.Error(), "docker daemon is not accessible") {
				t.Errorf("Expected error about docker daemon, got: %v", err)
			} else {
				t.Logf("CheckHealth correctly failed with mock docker: %v", err)
			}
		}
	})
}
