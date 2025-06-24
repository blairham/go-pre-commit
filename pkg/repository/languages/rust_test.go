package languages

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/tests/helpers"
)

func TestRustLanguage(t *testing.T) {
	rust := NewRustLanguage()

	config := helpers.LanguageTestConfig{
		Language:       rust,
		Name:           "Rust",
		ExecutableName: "rustc",
		VersionFlag:    testVersionFlag,
		TestVersions:   []string{"", "1.70", "1.71", "1.72", "1.73"},
		EnvPathSuffix:  "rustenv-1.73",
	}

	helpers.RunLanguageTests(t, config)
}

func TestNewRustLanguage(t *testing.T) {
	rust := NewRustLanguage()

	if rust == nil {
		t.Fatal("NewRustLanguage() returned nil")
	}

	if rust.Base == nil {
		t.Fatal("Base is nil")
	}

	// Check that the base is configured correctly
	if rust.GetName() != "Rust" {
		t.Errorf("Expected name 'Rust', got %s", rust.GetName())
	}

	if rust.GetExecutableName() != "rustc" {
		t.Errorf("Expected executable 'rustc', got %s", rust.GetExecutableName())
	}

	if rust.VersionFlag != testVersionFlag {
		t.Errorf("Expected version flag '%s', got %s", testVersionFlag, rust.VersionFlag)
	}
}

func TestRustLanguage_InstallDependencies(t *testing.T) {
	rust := NewRustLanguage()
	testInstallDependenciesBasic(t, rust, "serde", "serde:1.0", true, true)
}

func TestRustLanguage_CheckHealth(t *testing.T) {
	rust := NewRustLanguage()

	t.Run("CheckHealth_SystemVersion", func(t *testing.T) {
		// Test system version - this will check for rustc and cargo in PATH
		err := rust.CheckHealth("/dummy/path", "system")
		// The result depends on whether Rust is installed, but function shouldn't panic
		t.Logf("CheckHealth with system version returned: %v", err)
	})

	t.Run("CheckHealth_EnvironmentVersion_ExistingPath", func(t *testing.T) {
		// Create temporary directory
		tmpDir, err := os.MkdirTemp("", "rust_checkhealth_test_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Test with existing directory - should pass
		err = rust.CheckHealth(tmpDir, "1.70")
		if err != nil {
			t.Errorf("CheckHealth with existing directory should not return error: %v", err)
		}
	})

	t.Run("CheckHealth_EnvironmentVersion_NonExistentPath", func(t *testing.T) {
		// Test with non-existent directory - should fail
		err := rust.CheckHealth("/non/existent/path", "1.70")
		if err == nil {
			t.Error("CheckHealth with non-existent directory should return error")
		}
	})

	t.Run("CheckHealth_EmptyVersion", func(t *testing.T) {
		// Create temporary directory
		tmpDir, err := os.MkdirTemp("", "rust_checkhealth_empty_version_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Test with empty version (non-system) - should check directory existence
		err = rust.CheckHealth(tmpDir, "")
		if err != nil {
			t.Errorf("CheckHealth with empty version and existing directory should not return error: %v", err)
		}
	})

	t.Run("CheckHealth_DefaultVersion", func(t *testing.T) {
		// Create temporary directory
		tmpDir, err := os.MkdirTemp("", "rust_checkhealth_default_version_")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Test with default version (non-system) - should check directory existence
		err = rust.CheckHealth(tmpDir, "default")
		if err != nil {
			t.Errorf("CheckHealth with default version and existing directory should not return error: %v", err)
		}
	})

	t.Run("CheckHealth_SystemVersion_ErrorPaths", func(t *testing.T) {
		// This test covers the error paths in system version checking

		// Test system version - this will try to check for rustc and cargo in PATH
		err := rust.CheckHealth("/dummy/path", "system")
		// The result depends on whether Rust is installed
		// If Rust is not available, we should get specific error messages
		if err != nil {
			t.Logf("Expected error when Rust not available: %v", err)
			// Check that error message contains expected text
			if !strings.Contains(err.Error(), "rustc") && !strings.Contains(err.Error(), "cargo") {
				t.Logf("Error doesn't mention rustc or cargo: %v", err)
			}
		}

		// Test system version with empty path (should still check system tools)
		err = rust.CheckHealth("", "system")
		if err != nil {
			t.Logf("Expected error when Rust not available (empty path): %v", err)
		}
	})
}

func TestRustLanguage_SetupEnvironmentWithRepo(t *testing.T) {
	rust := NewRustLanguage()

	// Helper function to check if Rust is available
	isRustAvailable := func() bool {
		_, err := exec.LookPath("rustc")
		return err == nil
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "rust_setup_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name           string
		version        string
		repoPath       string
		additionalDeps []string
		wantErr        bool
	}{
		{
			name:           "basic setup",
			version:        "",
			repoPath:       tmpDir,
			additionalDeps: []string{},
			wantErr:        false,
		},
		{
			name:           "setup with version",
			version:        "1.70",
			repoPath:       tmpDir,
			additionalDeps: []string{},
			wantErr:        false,
		},
		{
			name:           "setup with dependencies",
			version:        "",
			repoPath:       tmpDir,
			additionalDeps: []string{"serde"},
			wantErr:        false, // In test mode, this should succeed; in real mode, may fail without cargo
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip test if Rust is not available to avoid triggering installation
			if !isRustAvailable() {
				t.Skip("rustc not available, skipping test that would trigger Rust installation")
			}

			envPath, err := rust.SetupEnvironmentWithRepo(
				"", // cache dir
				tt.version,
				tt.repoPath,
				"", // repo URL
				tt.additionalDeps,
			)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.wantErr && envPath == "" {
				t.Error("Expected non-empty environment path")
			}

			t.Logf("Setup returned path: %s, error: %v", envPath, err)
		})
	}

	// Additional coverage tests
	t.Run("SetupEnvironmentWithRepo_Coverage", func(t *testing.T) {
		// Skip test if Rust is not available
		if !isRustAvailable() {
			t.Skip("rustc not available, skipping test that would trigger Rust installation")
		}

		rust := NewRustLanguage()
		tempDir := t.TempDir()

		// Test with system version
		envPath, err := rust.SetupEnvironmentWithRepo("", "system", tempDir, "", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with system version returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() with system version returned empty path")
		}

		// Test with default version
		envPath, err = rust.SetupEnvironmentWithRepo("", "default", tempDir, "", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with default version returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() with default version returned empty path")
		}

		// Test with unsupported version (should default to 'default')
		envPath, err = rust.SetupEnvironmentWithRepo("", "1.999", tempDir, "", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo() with unsupported version returned error: %v", err)
		}
		if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() with unsupported version returned empty path")
		}
	})
}

func TestRustLanguage_PreInitializeEnvironmentWithRepoInfo(t *testing.T) {
	rust := NewRustLanguage()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "rust_preinit_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	err = rust.PreInitializeEnvironmentWithRepoInfo(
		tmpDir,
		"default",
		tmpDir,
		"https://github.com/test/repo",
		[]string{},
	)

	// Just check that it doesn't panic and returns an error or nil
	t.Logf("PreInitializeEnvironmentWithRepoInfo returned: %v", err)
}

func TestRustLanguage_SetupEnvironmentWithRepoInfo(t *testing.T) {
	rust := NewRustLanguage()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "rust_setup_info_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envPath, err := rust.SetupEnvironmentWithRepoInfo(
		tmpDir,
		"default",
		tmpDir,
		"https://github.com/test/repo",
		[]string{},
	)

	// Just check that it returns a path and doesn't panic
	t.Logf("SetupEnvironmentWithRepoInfo returned path: %s, error: %v", envPath, err)

	if envPath == "" && err == nil {
		t.Error("Expected either a path or an error")
	}
}

func TestRustLanguage_Implementation(t *testing.T) {
	rust := NewRustLanguage()

	// Test basic language interface methods
	if rust.GetName() == "" {
		t.Error("GetName() returned empty string")
	}

	if rust.GetExecutableName() == "" {
		t.Error("GetExecutableName() returned empty string")
	}

	if rust.VersionFlag == "" {
		t.Error("VersionFlag is empty string")
	}

	if rust.InstallURL == "" {
		t.Error("InstallURL is empty string")
	}

	// Check if rust is available
	_, err := exec.LookPath("rustc")
	rustAvailable := err == nil

	if rustAvailable {
		t.Log("Rust compiler is available on this system")
	} else {
		t.Log("Rust compiler is not available on this system")
	}
}

// Comprehensive tests to achieve 100% coverage for Rust language handler
func TestRustLanguage_100PercentCoverage(t *testing.T) {
	rust := NewRustLanguage()
	tempDir := t.TempDir()

	t.Run("InstallDependencies_ErrorPaths", func(t *testing.T) {
		// Test early return with empty dependencies
		err := rust.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies should not fail with empty dependencies: %v", err)
		}

		err = rust.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies should not fail with nil dependencies: %v", err)
		}
	})
}

func TestRustLanguage_ErrorPaths(t *testing.T) {
	rust := NewRustLanguage()

	t.Run("InstallDependencies_CargoNotAvailable", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a fake environment directory without cargo
		envDir := strings.Join([]string{tempDir, "cargo-not-available-test"}, "/")
		if err := os.MkdirAll(envDir, 0o755); err != nil {
			t.Fatalf("Failed to create env directory: %v", err)
		}

		// Try to install dependencies when cargo is not available anywhere
		oldPath := os.Getenv("PATH")
		defer os.Setenv("PATH", oldPath)
		os.Setenv("PATH", "/non/existent/path")

		err := rust.InstallDependencies(envDir, []string{"serde"})
		if err == nil {
			t.Errorf("Expected error when cargo is not available")
		}
		if !strings.Contains(err.Error(), "cargo not found") {
			t.Errorf("Expected 'cargo not found' error, got: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepo_CreateDirectoryError", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a file with the same name as the environment directory to cause creation failure
		repoDir := strings.Join([]string{tempDir, "repo"}, "/")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}
		envName := "rustenv-default"
		conflictingFile := strings.Join([]string{repoDir, envName}, "/")

		// Create a file instead of directory
		if err := os.WriteFile(conflictingFile, []byte("conflict"), 0o644); err != nil {
			t.Fatalf("Failed to create conflicting file: %v", err)
		}

		_, err := rust.SetupEnvironmentWithRepo("", "default", repoDir, "", nil)
		if err == nil {
			// The setup may succeed by removing and recreating the environment
			t.Log("SetupEnvironmentWithRepo succeeded by recreating environment")
		} else if !strings.Contains(err.Error(), "failed to create Rust environment directory") {
			t.Errorf("Expected directory creation error or success, got: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepo_VersionNormalization", func(t *testing.T) {
		tempDir := t.TempDir()
		repoDir := strings.Join([]string{tempDir, "repo"}, "/")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Test that non-default/non-system versions get normalized to default
		envPath, err := rust.SetupEnvironmentWithRepo("", "1.75.0", repoDir, "", nil)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo failed: %v", err)
		}

		// Should create default environment even with custom version
		if !strings.Contains(envPath, "rustenv-default") {
			t.Errorf("Expected default environment name, got: %s", envPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo_RemoveBrokenEnvironment", func(t *testing.T) {
		tempDir := t.TempDir()
		repoDir := strings.Join([]string{tempDir, "repo"}, "/")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create a broken environment directory first
		envPath := strings.Join([]string{repoDir, "rustenv-default"}, "/")
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Make the directory unreadable to simulate a broken environment
		stat, err := os.Stat(envPath)
		if err != nil {
			t.Fatalf("Failed to stat directory: %v", err)
		}
		oldPerm := stat.Mode()
		os.Chmod(envPath, 0o000)
		defer os.Chmod(envPath, oldPerm) // Restore permissions for cleanup

		// This should detect broken environment and recreate it
		result, err := rust.SetupEnvironmentWithRepo("", "default", repoDir, "", nil)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo should handle broken environment: %v", err)
		}

		// Restore permissions to check result
		os.Chmod(envPath, oldPerm)
		if result != envPath {
			t.Errorf("Expected recreated environment path %s, got: %s", envPath, result)
		}
	})

	t.Run("SetupEnvironmentWithRepo_RemoveFailure", func(t *testing.T) {
		tempDir := t.TempDir()
		repoDir := strings.Join([]string{tempDir, "repo"}, "/")
		if err := os.MkdirAll(repoDir, 0o755); err != nil {
			t.Fatalf("Failed to create repo directory: %v", err)
		}

		// Create a broken environment directory first
		envPath := strings.Join([]string{repoDir, "rustenv-default"}, "/")
		if err := os.MkdirAll(envPath, 0o755); err != nil {
			t.Fatalf("Failed to create env directory: %v", err)
		}

		// Create a subdirectory that we'll make non-removable
		subDir := strings.Join([]string{envPath, "subdir"}, "/")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}

		// Make parent directory read-only to prevent removal of subdirectory
		if err := os.Chmod(envPath, 0o444); err != nil {
			t.Fatalf("Failed to make directory read-only: %v", err)
		}
		defer os.Chmod(envPath, 0o755) // Restore permissions for cleanup

		// This should fail to remove the broken environment
		_, err := rust.SetupEnvironmentWithRepo("", "default", repoDir, "", nil)
		if err == nil {
			t.Errorf("Expected error when removing broken environment fails")
		} else if !strings.Contains(err.Error(), "failed to remove broken environment") {
			t.Errorf("Expected remove broken environment error, got: %v", err)
		}
	})

	t.Run("CheckHealth_SystemVersion_RustcNotAvailable", func(t *testing.T) {
		// Test when rustc is not available but cargo is
		oldPath := os.Getenv("PATH")
		defer os.Setenv("PATH", oldPath)

		// Create temp directory with only a fake cargo
		tempBin := t.TempDir()
		cargoPath := strings.Join([]string{tempBin, "cargo"}, "/")
		if err := os.WriteFile(cargoPath, []byte("#!/bin/sh\necho 'fake cargo'\n"), 0o755); err != nil {
			t.Fatalf("Failed to create fake cargo: %v", err)
		}

		os.Setenv("PATH", tempBin)

		err := rust.CheckHealth("", "system")
		if err == nil {
			t.Errorf("Expected error when rustc is not available")
		}
		if !strings.Contains(err.Error(), "system rust (rustc) not available") {
			t.Errorf("Expected rustc not available error, got: %v", err)
		}
	})

	t.Run("CheckHealth_SystemVersion_CargoNotAvailable", func(t *testing.T) {
		// Test when cargo is not available but rustc is
		oldPath := os.Getenv("PATH")
		defer os.Setenv("PATH", oldPath)

		// Create temp directory with only a fake rustc
		tempBin := t.TempDir()
		rustcPath := strings.Join([]string{tempBin, "rustc"}, "/")
		if err := os.WriteFile(rustcPath, []byte("#!/bin/sh\necho 'fake rustc'\n"), 0o755); err != nil {
			t.Fatalf("Failed to create fake rustc: %v", err)
		}

		os.Setenv("PATH", tempBin)

		err := rust.CheckHealth("", "system")
		if err == nil {
			t.Errorf("Expected error when cargo is not available")
		}
		if !strings.Contains(err.Error(), "system cargo not available") {
			t.Errorf("Expected cargo not available error, got: %v", err)
		}
	})

	t.Run("CheckHealth_EnvironmentVersion_NonExistentDirectory", func(t *testing.T) {
		err := rust.CheckHealth("/non/existent/path", "default")
		if err == nil {
			t.Errorf("Expected error for non-existent environment directory")
		}
		if !strings.Contains(err.Error(), "environment directory does not exist") {
			t.Errorf("Expected directory not exist error, got: %v", err)
		}
	})

	t.Run("InstallDependencies_SystemCargo", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create environment without cargo binary
		envDir := strings.Join([]string{tempDir, "rustenv"}, "/")
		if err := os.MkdirAll(envDir, 0o755); err != nil {
			t.Fatalf("Failed to create env directory: %v", err)
		}

		// Ensure system cargo is available (if it exists on the system)
		if _, err := exec.LookPath("cargo"); err == nil {
			// Test using system cargo when env cargo doesn't exist
			err := rust.InstallDependencies(envDir, []string{})
			if err != nil {
				t.Errorf("InstallDependencies should work with system cargo: %v", err)
			}
		}
	})
}
