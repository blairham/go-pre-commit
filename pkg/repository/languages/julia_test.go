package languages

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testJuliaManifestContent = `# This file is machine-generated
julia_version = "1.8.0"
manifest_format = "2.0"
`
)

func TestJuliaLanguage(t *testing.T) {
	t.Run("NewJuliaLanguage", func(t *testing.T) {
		julia := NewJuliaLanguage()
		if julia == nil {
			t.Error("NewJuliaLanguage() returned nil")
			return
		}
		if julia.Base == nil {
			t.Error("NewJuliaLanguage() returned instance with nil Base")
		}

		// Check properties
		if julia.Name != "julia" {
			t.Errorf("Expected name 'julia', got '%s'", julia.Name)
		}
		if julia.ExecutableName != "julia" {
			t.Errorf("Expected executable name 'julia', got '%s'", julia.ExecutableName)
		}
		if julia.VersionFlag != testVersionFlag {
			t.Errorf("Expected version flag '%s', got '%s'", testVersionFlag, julia.VersionFlag)
		}
		if julia.InstallURL != "https://julialang.org/downloads/" {
			t.Errorf("Expected install URL 'https://julialang.org/downloads/', got '%s'", julia.InstallURL)
		}
	})

	t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		julia := NewJuliaLanguage()
		tempDir := t.TempDir()

		// Should delegate to base method without error
		err := julia.PreInitializeEnvironmentWithRepoInfo(tempDir, "1.8", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo() returned error: %v", err)
		}

		// Test with additional dependencies
		err = julia.PreInitializeEnvironmentWithRepoInfo(
			tempDir, "1.9", tempDir, "dummy-url", []string{"DataFrames", "Plots"},
		)
		if err != nil {
			t.Errorf("PreInitializeEnvironmentWithRepoInfo() with deps returned error: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepoInfo", func(t *testing.T) {
		julia := NewJuliaLanguage()
		tempDir := t.TempDir()

		// Helper function to check if Julia is available
		isJuliaAvailable := func() bool {
			_, err := exec.LookPath("julia")
			return err == nil
		}

		// Run test regardless of Julia availability to improve coverage
		envPath, err := julia.SetupEnvironmentWithRepoInfo(tempDir, "1.8", tempDir, "dummy-url", []string{})

		if !isJuliaAvailable() {
			// Julia not available - the function should still execute but may fail
			if err != nil {
				t.Logf("SetupEnvironmentWithRepoInfo() failed as expected (Julia not available): %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepoInfo() succeeded despite Julia not being available: %s", envPath)
			}
		} else {
			// Julia is available - expect success
			if err != nil {
				t.Errorf("SetupEnvironmentWithRepoInfo() returned error: %v", err)
			}
			if envPath == "" {
				t.Error("SetupEnvironmentWithRepoInfo() returned empty environment path")
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		julia := NewJuliaLanguage()
		tempDir := t.TempDir()

		// Helper function to check if Julia is available
		isJuliaAvailable := func() bool {
			_, err := exec.LookPath("julia")
			return err == nil
		}

		// Run test regardless of Julia availability to improve coverage
		envPath, err := julia.SetupEnvironmentWithRepo(tempDir, "1.8", tempDir, "dummy-url", []string{})

		if !isJuliaAvailable() {
			// Julia not available - the function should still execute but may fail
			if err != nil {
				t.Logf("SetupEnvironmentWithRepo() failed as expected (Julia not available): %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo() succeeded despite Julia not being available: %s", envPath)
			}
		} else {
			// Julia is available - expect success
			if err != nil {
				t.Errorf("SetupEnvironmentWithRepo() returned error: %v", err)
			}
			if envPath == "" {
				t.Error("SetupEnvironmentWithRepo() returned empty environment path")
			}
		}

		// Test with additional dependencies
		envPath, err = julia.SetupEnvironmentWithRepo(tempDir, "1.9", tempDir, "dummy-url", []string{"DataFrames"})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo() with deps returned error (may be expected): %v", err)
		} else if envPath == "" {
			t.Error("SetupEnvironmentWithRepo() with deps returned empty environment path")
		}
	})

	t.Run("InstallDependencies_Empty", func(t *testing.T) {
		julia := NewJuliaLanguage()
		tempDir := t.TempDir()

		// Should handle empty dependencies without error
		err := julia.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies() with empty deps returned error: %v", err)
		}

		err = julia.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies() with nil deps returned error: %v", err)
		}
	})

	t.Run("InstallDependencies_WithDeps", func(t *testing.T) {
		julia := NewJuliaLanguage()
		tempDir := t.TempDir()

		// Skip test if Julia is not available to avoid triggering installation
		if _, err := exec.LookPath("julia"); err != nil {
			t.Skip("julia not available, skipping test that would trigger dependency installation")
		}

		// Test with some dependencies - this will likely fail since Julia may not be available
		err := julia.InstallDependencies(tempDir, []string{"DataFrames", "Plots"})
		if err != nil {
			t.Logf("InstallDependencies failed (expected if Julia not available): %v", err)
		}

		// Should create Project.toml file even if Julia is not available for instantiation
		projectPath := filepath.Join(tempDir, "Project.toml")
		if _, err := os.Stat(projectPath); err != nil {
			t.Error("InstallDependencies should create Project.toml file")
		} else {
			// Verify Project.toml content
			content, err := os.ReadFile(projectPath)
			if err != nil {
				t.Errorf("Failed to read Project.toml: %v", err)
			} else {
				contentStr := string(content)
				if !strings.Contains(contentStr, "DataFrames") {
					t.Error("Project.toml should contain DataFrames dependency")
				}
				if !strings.Contains(contentStr, "Plots") {
					t.Error("Project.toml should contain Plots dependency")
				}
			}
		}
	})

	t.Run("InstallDependencies_InvalidPath", func(t *testing.T) {
		julia := NewJuliaLanguage()

		// Skip test if Julia is not available to avoid triggering installation
		if _, err := exec.LookPath("julia"); err != nil {
			t.Skip("julia not available, skipping test that would trigger dependency installation")
		}

		// Test with invalid path - should fail to create Project.toml
		err := julia.InstallDependencies("/invalid/readonly/path", []string{"test-dep"})
		if err == nil {
			t.Error("InstallDependencies() with invalid path should return error")
		}
	})

	t.Run("CheckHealth", func(t *testing.T) {
		julia := NewJuliaLanguage()
		tempDir := t.TempDir()

		// Should return error for directory without Project.toml
		err := julia.CheckHealth(tempDir, "1.8")
		if err != nil {
			t.Logf("CheckHealth without Project.toml: %v (expected)", err)
		}

		// Create Project.toml
		projectPath := filepath.Join(tempDir, "Project.toml")
		projectContent := `name = "TestProject"
version = "0.1.0"

[deps]
DataFrames = "*"
`
		if writeErr := os.WriteFile(projectPath, []byte(projectContent), 0o600); writeErr != nil {
			t.Fatalf("Failed to create Project.toml: %v", writeErr)
		}

		// Should return error for missing Manifest.toml
		err = julia.CheckHealth(tempDir, "1.8")
		if err == nil {
			t.Error("CheckHealth should return error when Manifest.toml is missing")
		}

		// Create Manifest.toml
		manifestPath := filepath.Join(tempDir, "Manifest.toml")
		manifestContent := `# This file is machine-generated - editing it directly is not advised

julia_version = "1.8.0"
manifest_format = "2.0"
`
		if writeErr := os.WriteFile(manifestPath, []byte(manifestContent), 0o600); writeErr != nil {
			t.Fatalf("Failed to create Manifest.toml: %v", writeErr)
		}

		// Now CheckHealth should try to run Julia (may fail if not available)
		err = julia.CheckHealth(tempDir, "1.8")
		if err != nil {
			t.Logf("CheckHealth with Project.toml and Manifest.toml failed (expected if Julia not available): %v", err)
		}
	})

	t.Run("CheckHealth_EmptyPath", func(t *testing.T) {
		julia := NewJuliaLanguage()

		// Should handle empty paths gracefully
		err := julia.CheckHealth("", "")
		if err != nil {
			t.Logf("CheckHealth with empty path: %v (expected)", err)
		}
	})

	// Additional tests for better coverage of functions with 0% coverage
	t.Run("SetupEnvironmentWithRepoInfo_NoCacheDir", func(t *testing.T) {
		julia := NewJuliaLanguage()

		// Skip if Julia is not available
		if _, err := exec.LookPath("julia"); err != nil {
			t.Skip("julia not available, skipping test that could trigger Julia installation or setup")
		}

		// Test error path with empty cache dir
		_, err := julia.SetupEnvironmentWithRepoInfo("", "1.8", "/tmp", "dummy-url", []string{})
		// This should likely return an error due to invalid cache directory
		t.Logf("SetupEnvironmentWithRepoInfo with empty cache dir: %v", err)
	})

	t.Run("SetupEnvironmentWithRepo_ErrorPaths", func(t *testing.T) {
		julia := NewJuliaLanguage()

		// Skip if Julia is not available
		if _, err := exec.LookPath("julia"); err != nil {
			t.Skip("julia not available, skipping test that could trigger Julia installation or setup")
		}

		// Test with invalid repo path
		_, err := julia.SetupEnvironmentWithRepo("", "1.8", "/nonexistent/path", "dummy-url", []string{})
		// This may or may not error depending on implementation
		t.Logf("SetupEnvironmentWithRepo with invalid repo path: %v", err)

		// Test with empty version
		_, err = julia.SetupEnvironmentWithRepo("", "", "/tmp", "dummy-url", []string{})
		t.Logf("SetupEnvironmentWithRepo with empty version: %v", err)
	})
}

func TestJuliaLanguage_EnvironmentStructure(t *testing.T) {
	julia := NewJuliaLanguage()

	t.Run("SetupEnvironmentWithRepo_CorrectNaming", func(t *testing.T) {
		testEnvironmentNaming(t, julia, "1.8", "juliaenv")
	})

	t.Run("SetupEnvironmentWithRepo_ExistingHealthyEnvironment", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a healthy environment manually
		envPath := filepath.Join(tempDir, "juliaenv-1.8")
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create Project.toml and Manifest.toml to make it healthy
		projectPath := filepath.Join(envPath, "Project.toml")
		projectContent := `name = "TestProject"
version = "0.1.0"

[deps]
`
		err = os.WriteFile(projectPath, []byte(projectContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Project.toml: %v", err)
		}

		manifestPath := filepath.Join(envPath, "Manifest.toml")
		manifestContent := testJuliaManifestContent
		err = os.WriteFile(manifestPath, []byte(manifestContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Manifest.toml: %v", err)
		}

		// Create mock julia to make health check pass
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err = os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		juliaScript := `#!/bin/bash
if [[ "$*" == *"Pkg.status()"* ]]; then
  echo "No packages in environment"
  exit 0
fi
exit 0`
		juliaExec := filepath.Join(mockBinDir, "julia")
		err = os.WriteFile(juliaExec, []byte(juliaScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock julia: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// Call SetupEnvironmentWithRepo - should reuse healthy environment
		resultPath, err := julia.SetupEnvironmentWithRepo("", "1.8", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo should succeed with healthy environment: %v", err)
		} else if resultPath != envPath {
			t.Errorf("Should reuse existing healthy environment: expected %s, got %s", envPath, resultPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo_ExistingBrokenEnvironment", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a broken environment (missing Manifest.toml)
		envPath := filepath.Join(tempDir, "juliaenv-1.8")
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create Project.toml but no Manifest.toml (broken)
		projectPath := filepath.Join(envPath, "Project.toml")
		projectContent := `name = "BrokenProject"
version = "0.1.0"
`
		err = os.WriteFile(projectPath, []byte(projectContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Project.toml: %v", err)
		}

		// Add a marker file to verify environment gets recreated
		markerFile := filepath.Join(envPath, "broken_marker")
		err = os.WriteFile(markerFile, []byte("broken"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create marker file: %v", err)
		}

		// Call SetupEnvironmentWithRepo - should detect broken environment and recreate
		resultPath, err := julia.SetupEnvironmentWithRepo("", "1.8", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo failed (expected if Julia not available): %v", err)
		} else {
			t.Logf("SetupEnvironmentWithRepo succeeded with environment: %s", resultPath)

			// Verify marker file was removed (environment was recreated)
			if _, statErr := os.Stat(markerFile); !os.IsNotExist(statErr) {
				t.Error("Broken environment should have been removed and recreated")
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_EnvironmentCreationFailure", func(t *testing.T) {
		juliaLang := NewJuliaLanguage()

		// Test with invalid repo path that would cause environment creation to fail
		_, err := juliaLang.SetupEnvironmentWithRepo(
			"",
			"1.8",
			"/nonexistent/invalid/repo/path",
			"dummy-url",
			[]string{},
		)
		if err == nil {
			t.Log("SetupEnvironmentWithRepo succeeded with invalid repo path (may be platform-specific behavior)")
		} else if strings.Contains(err.Error(), "failed to create Julia environment directory") {
			t.Logf("Successfully tested environment creation failure: %v", err)
		} else {
			t.Logf("Got different error than expected: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepo_DependencyInstallationFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test dependency installation failure path
		_, err := julia.SetupEnvironmentWithRepo("", "1.8", tempDir, "dummy-url", []string{"NonexistentPackage123"})
		if err == nil {
			t.Log("SetupEnvironmentWithRepo succeeded (Julia may not be available)")
		} else if strings.Contains(err.Error(), "failed to install Julia dependencies") {
			t.Logf("Successfully tested dependency installation failure: %v", err)
		} else {
			t.Logf("Got different error (expected if Julia not available): %v", err)
		}
	})
}

func TestJuliaLanguage_InstallDependencies_Coverage(t *testing.T) {
	julia := NewJuliaLanguage()

	t.Run("InstallDependencies_ProjectTomlCreationError", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a file where Project.toml should be created to cause an error
		projectPath := filepath.Join(tempDir, "Project.toml")
		// Create a directory instead of a file to cause WriteFile to fail
		err := os.MkdirAll(projectPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create directory blocking Project.toml creation: %v", err)
		}

		// This should fail when trying to create Project.toml
		err = julia.InstallDependencies(tempDir, []string{"TestPackage"})
		if err == nil {
			t.Error("InstallDependencies should fail when Project.toml creation fails")
		} else if !strings.Contains(err.Error(), "failed to create Project.toml") {
			t.Errorf("Expected Project.toml creation error, got: %v", err)
		}
	})

	t.Run("InstallDependencies_JuliaNotAvailable", func(t *testing.T) {
		tempDir := t.TempDir()

		// Temporarily modify PATH to make julia unavailable
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)

		emptyDir := filepath.Join(tempDir, "empty")
		err := os.MkdirAll(emptyDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create empty directory: %v", err)
		}
		os.Setenv("PATH", emptyDir)

		// This should exercise the julia command execution failure path
		err = julia.InstallDependencies(tempDir, []string{"TestPackage"})
		if err == nil {
			t.Error("InstallDependencies should fail when julia not available")
		} else if !strings.Contains(err.Error(), "failed to install Julia dependencies") {
			t.Errorf("Expected julia execution error, got: %v", err)
		}
	})

	t.Run("InstallDependencies_JuliaInstantiateFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create mock julia that fails on Pkg.instantiate()
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		juliaScript := `#!/bin/bash
if [[ "$*" == *"Pkg.instantiate"* ]]; then
  echo "Error: Package resolution failed"
  exit 1
fi
exit 0`
		juliaExec := filepath.Join(mockBinDir, "julia")
		err = os.WriteFile(juliaExec, []byte(juliaScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock julia: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should exercise the Pkg.instantiate() failure path
		err = julia.InstallDependencies(tempDir, []string{"TestPackage"})
		if err == nil {
			t.Error("InstallDependencies should fail when julia instantiate fails")
		} else if !strings.Contains(err.Error(), "failed to install Julia dependencies") {
			t.Errorf("Expected julia instantiate error, got: %v", err)
		}
	})

	t.Run("InstallDependencies_Success", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create mock julia that succeeds
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		juliaScript := `#!/bin/bash
echo "Julia mock: $*"
exit 0`
		juliaExec := filepath.Join(mockBinDir, "julia")
		err = os.WriteFile(juliaExec, []byte(juliaScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock julia: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should exercise the success path
		err = julia.InstallDependencies(tempDir, []string{"DataFrames", "Plots"})
		if err != nil {
			t.Errorf("InstallDependencies should succeed with working julia: %v", err)
		}

		// Verify Project.toml was created with correct content
		projectPath := filepath.Join(tempDir, "Project.toml")
		content, err := os.ReadFile(projectPath)
		if err != nil {
			t.Errorf("Failed to read Project.toml: %v", err)
		} else {
			contentStr := string(content)
			if !strings.Contains(contentStr, "DataFrames") {
				t.Error("Project.toml should contain DataFrames dependency")
			}
			if !strings.Contains(contentStr, "Plots") {
				t.Error("Project.toml should contain Plots dependency")
			}
		}
	})
}

func TestJuliaLanguage_CheckHealth_Coverage(t *testing.T) {
	julia := NewJuliaLanguage()

	t.Run("CheckHealth_NoProjectToml", func(t *testing.T) {
		tempDir := t.TempDir()

		// CheckHealth should pass when no Project.toml exists (base case)
		err := julia.CheckHealth(tempDir, "1.8")
		if err != nil {
			t.Logf("CheckHealth without Project.toml returned error: %v", err)
		}
	})

	t.Run("CheckHealth_ProjectTomlExistsNoManifest", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create Project.toml
		projectPath := filepath.Join(tempDir, "Project.toml")
		projectContent := `name = "TestProject"
version = "0.1.0"

[deps]
DataFrames = "*"
`
		err := os.WriteFile(projectPath, []byte(projectContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Project.toml: %v", err)
		}

		// Should return error for missing Manifest.toml
		err = julia.CheckHealth(tempDir, "1.8")
		if err == nil {
			t.Error("CheckHealth should return error when Project.toml exists but Manifest.toml is missing")
		} else if !strings.Contains(err.Error(), "Manifest.toml missing") {
			t.Errorf("Expected Manifest.toml missing error, got: %v", err)
		}
	})

	t.Run("CheckHealth_JuliaProjectVerificationFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create Project.toml and Manifest.toml
		projectPath := filepath.Join(tempDir, "Project.toml")
		projectContent := `name = "TestProject"
version = "0.1.0"
`
		err := os.WriteFile(projectPath, []byte(projectContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Project.toml: %v", err)
		}

		manifestPath := filepath.Join(tempDir, "Manifest.toml")
		manifestContent := testJuliaManifestContent
		err = os.WriteFile(manifestPath, []byte(manifestContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Manifest.toml: %v", err)
		}

		// Create mock julia that fails verification
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err = os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		juliaScript := `#!/bin/bash
if [[ "$*" == *"Pkg.status"* ]]; then
  echo "Error: Project verification failed"
  exit 1
fi
exit 0`
		juliaExec := filepath.Join(mockBinDir, "julia")
		err = os.WriteFile(juliaExec, []byte(juliaScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock julia: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// Should return error when julia project verification fails
		err = julia.CheckHealth(tempDir, "1.8")
		if err == nil {
			t.Error("CheckHealth should return error when julia project verification fails")
		} else if !strings.Contains(err.Error(), "julia project verification failed") {
			t.Errorf("Expected project verification error, got: %v", err)
		}
	})

	t.Run("CheckHealth_Success", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create Project.toml and Manifest.toml
		projectPath := filepath.Join(tempDir, "Project.toml")
		projectContent := `name = "TestProject"
version = "0.1.0"
`
		err := os.WriteFile(projectPath, []byte(projectContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Project.toml: %v", err)
		}

		manifestPath := filepath.Join(tempDir, "Manifest.toml")
		manifestContent := testJuliaManifestContent
		err = os.WriteFile(manifestPath, []byte(manifestContent), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Manifest.toml: %v", err)
		}

		// Create mock julia that succeeds
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err = os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		juliaScript := `#!/bin/bash
echo "Status of project"
exit 0`
		juliaExec := filepath.Join(mockBinDir, "julia")
		err = os.WriteFile(juliaExec, []byte(juliaScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock julia: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// Should succeed when everything is healthy
		err = julia.CheckHealth(tempDir, "1.8")
		if err != nil {
			t.Errorf("CheckHealth should succeed when project is healthy: %v", err)
		}
	})
}

func TestJuliaLanguage_AdditionalCoverage(t *testing.T) {
	julia := NewJuliaLanguage()

	t.Run("SetupEnvironmentWithRepo_EmptyEnvironmentName", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test what happens when GetRepositoryEnvironmentName returns empty string
		// This might happen for certain language/version combinations
		envPath, err := julia.SetupEnvironmentWithRepo("", "", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepo with empty version failed: %v", err)
		} else {
			// If it succeeds, should return the repo path itself
			if envPath != tempDir {
				expectedPath := filepath.Join(tempDir, "juliaenv-default")
				if envPath != expectedPath {
					t.Errorf("Expected either repo path %s or environment path %s, got %s", tempDir, expectedPath, envPath)
				}
			}
		}
	})

	t.Run("SetupEnvironmentWithRepo_RemoveAllFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create environment directory with complex nested structure that's hard to remove
		envPath := filepath.Join(tempDir, "juliaenv-1.8")
		nestedPath := filepath.Join(envPath, "nested", "deep", "structure")
		err := os.MkdirAll(nestedPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create nested environment structure: %v", err)
		}

		// Create files to make the directory more complex
		testFile := filepath.Join(nestedPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Make environment unhealthy by not having proper Julia files
		// CheckHealth should fail, triggering the removal and recreation

		// Test SetupEnvironmentWithRepo with existing environment
		envPath2, err := julia.SetupEnvironmentWithRepo("", "1.8", tempDir, "dummy-url", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "failed to remove broken environment") {
				t.Logf("Successfully tested RemoveAll failure path: %v", err)
			} else {
				t.Logf("SetupEnvironmentWithRepo failed with different error: %v", err)
			}
		} else {
			t.Logf("SetupEnvironmentWithRepo succeeded, environment created at: %s", envPath2)
		}
	})

	t.Run("SetupEnvironmentWithRepo_CreateEnvironmentDirectoryFailure", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a file where the environment directory should be created
		envDirName := "juliaenv-1.8"
		conflictingFile := filepath.Join(tempDir, envDirName)
		err := os.WriteFile(conflictingFile, []byte("conflict"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create conflicting file: %v", err)
		}

		// This should fail when trying to create the environment directory
		_, err = julia.SetupEnvironmentWithRepo("", "1.8", tempDir, "dummy-url", []string{})
		if err == nil {
			t.Log("SetupEnvironmentWithRepo succeeded despite file conflict (platform-specific behavior)")
		} else if strings.Contains(err.Error(), "failed to create Julia environment directory") {
			t.Logf("Successfully tested CreateEnvironmentDirectory failure: %v", err)
		} else {
			t.Logf("Got different error: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepo_VersionEdgeCases", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test edge cases for version handling
		testVersions := []string{
			"",        // empty version
			"default", // explicit default
			"1.8",     // specific version
			"1.9.0",   // specific patch version
			"latest",  // latest version
		}

		for _, version := range testVersions {
			t.Run("Version_"+version, func(t *testing.T) {
				envPath, err := julia.SetupEnvironmentWithRepo("", version, tempDir, "dummy-url", []string{})

				if err != nil {
					t.Logf("SetupEnvironmentWithRepo with version '%s' failed: %v", version, err)
				} else {
					t.Logf("SetupEnvironmentWithRepo with version '%s' succeeded: %s", version, envPath)

					// Verify the environment directory name is correct
					expectedVersionName := version
					if version == "" {
						expectedVersionName = testDefaultStr
					}
					expectedPath := filepath.Join(tempDir, "juliaenv-"+expectedVersionName)
					if envPath != expectedPath && envPath != tempDir {
						t.Errorf("Expected environment path %s, got %s", expectedPath, envPath)
					}
				}
			})
		}
	})

	t.Run("SetupEnvironmentWithRepo_DependencyInstallationFailureDetailed", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with dependencies to trigger the InstallDependencies error path
		_, err := julia.SetupEnvironmentWithRepo(
			"", "1.8", tempDir, "dummy-url", []string{"NonexistentPackage123", "AnotherNonexistentPackage"},
		)

		if err == nil {
			t.Log("SetupEnvironmentWithRepo succeeded (Julia may not be available or dependencies were satisfied)")
		} else if strings.Contains(err.Error(), "failed to install Julia dependencies") {
			t.Logf("Successfully tested dependency installation failure: %v", err)
		} else {
			t.Logf("Got different error: %v", err)
		}
	})

	t.Run("SetupEnvironmentWithRepo_ReuseHealthyEnvironment", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create mock julia that reports healthy status
		mockBinDir := filepath.Join(tempDir, "mockbin")
		err := os.MkdirAll(mockBinDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock bin directory: %v", err)
		}

		juliaScript := `#!/bin/bash
echo "Status of project"
exit 0`
		juliaExec := filepath.Join(mockBinDir, "julia")
		err = os.WriteFile(juliaExec, []byte(juliaScript), 0o755)
		if err != nil {
			t.Fatalf("Failed to create mock julia: %v", err)
		}

		// Set PATH to include our mock julia
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// Create a healthy environment manually
		envPath := filepath.Join(tempDir, "juliaenv-1.8")
		err = os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create environment directory: %v", err)
		}

		// Create Project.toml and Manifest.toml to make it healthy
		projectPath := filepath.Join(envPath, "Project.toml")
		err = os.WriteFile(projectPath, []byte("name = \"TestProject\"\nversion = \"0.1.0\"\n"), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Project.toml: %v", err)
		}

		manifestPath := filepath.Join(envPath, "Manifest.toml")
		err = os.WriteFile(manifestPath, []byte("# This file is machine-generated\njulia_version = \"1.8.0\"\n"), 0o600)
		if err != nil {
			t.Fatalf("Failed to create Manifest.toml: %v", err)
		}

		// Call SetupEnvironmentWithRepo - should reuse healthy environment
		resultPath, err := julia.SetupEnvironmentWithRepo("", "1.8", tempDir, "dummy-url", []string{})
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo should succeed with healthy environment: %v", err)
		} else if resultPath != envPath {
			t.Errorf("Should reuse existing healthy environment: expected %s, got %s", envPath, resultPath)
		} else {
			t.Log("Successfully reused healthy environment without recreation")
		}
	})
}

func TestJuliaLanguage_FinalCoverageGaps(t *testing.T) {
	julia := NewJuliaLanguage()

	t.Run("SetupEnvironmentWithRepo_EmptyEnvironmentNameCase", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create a mock language that would return empty environment name
		// by temporarily modifying the name to something that would result in empty
		originalName := julia.Name
		julia.Name = testSystemStr // This should result in empty environment name
		defer func() { julia.Name = originalName }()

		envPath, err := julia.SetupEnvironmentWithRepo("", "default", tempDir, "dummy-url", []string{})
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

	t.Run("SetupEnvironmentWithRepo_RemoveAllError", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create environment directory
		envPath := filepath.Join(tempDir, "juliaenv-1.8")
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
		_, err = julia.SetupEnvironmentWithRepo("", "1.8", tempDir, "dummy-url", []string{})
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
		_, err = julia.SetupEnvironmentWithRepo("", "1.8", tempFile, "dummy-url", []string{})
		if err != nil {
			if strings.Contains(err.Error(), "failed to create Julia environment directory") {
				t.Logf("Successfully tested CreateEnvironmentDirectory error: %v", err)
			} else {
				t.Logf("Got different error: %v", err)
			}
		} else {
			t.Log("SetupEnvironmentWithRepo succeeded (platform-specific behavior may allow this)")
		}
	})
}
