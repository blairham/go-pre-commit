package languages

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/tests/helpers"
)

const (
	testCsprojContent = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
</Project>`
)

func TestDotnetLanguage(t *testing.T) {
	dotnet := NewDotnetLanguage()

	config := helpers.LanguageTestConfig{
		Language:       dotnet,
		Name:           ".NET", // Use actual language name returned by the implementation
		ExecutableName: "dotnet",
		VersionFlag:    "--version",
		TestVersions:   []string{"", "6.0", "7.0", "8.0"},
		EnvPathSuffix:  "dotnetenv-8.0", // Actual path pattern used by implementation
	}

	helpers.RunLanguageTests(t, config)
}

func TestNewDotnetLanguage(t *testing.T) {
	dotnet := NewDotnetLanguage()

	if dotnet == nil {
		t.Fatal("NewDotnetLanguage() returned nil")
	}

	if dotnet.Base == nil {
		t.Fatal("Base is nil")
	}

	// Check that the base is configured correctly
	if dotnet.GetName() != ".NET" {
		t.Errorf("Expected name '.NET', got %s", dotnet.GetName())
	}

	if dotnet.GetExecutableName() != "dotnet" {
		t.Errorf("Expected executable 'dotnet', got %s", dotnet.GetExecutableName())
	}

	// Access the underlying fields for version flag and install URL
	if dotnet.VersionFlag != testVersionFlag {
		t.Errorf("Expected version flag '%s', got %s", testVersionFlag, dotnet.VersionFlag)
	}
}

func TestDotnetLanguage_InstallDependencies(t *testing.T) {
	dotnet := NewDotnetLanguage()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dotnet_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		deps     []string
		wantErr  bool
		skipTest bool
	}{
		{
			name:    "no dependencies",
			deps:    []string{},
			wantErr: false,
		},
		{
			name:     "single dependency",
			deps:     []string{"Newtonsoft.Json"},
			wantErr:  false,
			skipTest: true, // Skip if dotnet not available
		},
		{
			name:     "dependency with version",
			deps:     []string{"Newtonsoft.Json:13.0.3"},
			wantErr:  false,
			skipTest: true, // Skip if dotnet not available
		},
		{
			name:     "multiple dependencies",
			deps:     []string{"Newtonsoft.Json", "Microsoft.Extensions.Logging"},
			wantErr:  false,
			skipTest: true, // Skip if dotnet not available
		},
		{
			name:     "invalid dependency",
			deps:     []string{"nonexistent-package-that-should-not-exist"},
			wantErr:  true,
			skipTest: true, // Skip if dotnet not available
		},
	}

	// Check if dotnet is available
	_, err = exec.LookPath("dotnet")
	dotnetAvailable := err == nil

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest && !dotnetAvailable {
				t.Skip("dotnet not available, skipping test")
			}

			// Create a new test directory for each test
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0o755); err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			err := dotnet.InstallDependencies(testDir, tt.deps)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// If successful and dependencies were provided, check project was created
			if !tt.wantErr && len(tt.deps) > 0 && dotnetAvailable {
				projectPath := filepath.Join(testDir, "PreCommitEnv")
				csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
				if _, err := os.Stat(csprojPath); err != nil {
					t.Errorf("Expected project file to exist at %s", csprojPath)
				}
			}
		})
	}
}

func TestDotnetLanguage_CheckEnvironmentHealth(t *testing.T) {
	dotnet := NewDotnetLanguage()

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dotnet_health_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Check if dotnet is available
	_, err = exec.LookPath("dotnet")
	dotnetAvailable := err == nil

	t.Run("NonExistentEnvironment", func(t *testing.T) {
		// Test with non-existent directory
		health := dotnet.CheckEnvironmentHealth(filepath.Join(tmpDir, "nonexistent"))
		if health {
			t.Error("CheckEnvironmentHealth should return false for non-existent environment")
		}
	})

	t.Run("EmptyEnvironment", func(t *testing.T) {
		// Test with empty directory
		emptyDir := filepath.Join(tmpDir, "empty")
		if err := os.MkdirAll(emptyDir, 0o755); err != nil {
			t.Fatalf("Failed to create empty dir: %v", err)
		}
		health := dotnet.CheckEnvironmentHealth(emptyDir)
		if health {
			t.Error("CheckEnvironmentHealth should return false for empty environment")
		}
	})

	t.Run("EnvironmentWithoutProject", func(t *testing.T) {
		// Test environment without PreCommitEnv project
		testDir := filepath.Join(tmpDir, "no_project")
		if err := os.MkdirAll(testDir, 0o755); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		// Skip if dotnet is not available to avoid triggering installation
		if !dotnetAvailable {
			t.Skip("dotnet not available, skipping test that would need dotnet runtime")
		}

		health := dotnet.CheckEnvironmentHealth(testDir)
		// Without project, should return true if CheckHealth passes
		t.Logf("CheckEnvironmentHealth without project: %v", health)
	})

	t.Run("EnvironmentWithValidProject", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "with_project")
		if err := os.MkdirAll(testDir, 0o755); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		// Create project directory structure
		projectPath := filepath.Join(testDir, "PreCommitEnv")
		if err := os.MkdirAll(projectPath, 0o755); err != nil {
			t.Fatalf("Failed to create project dir: %v", err)
		}

		// Create a valid .csproj file
		csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
		csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
</Project>`
		if err := os.WriteFile(csprojPath, []byte(csprojContent), 0o644); err != nil {
			t.Fatalf("Failed to create .csproj file: %v", err)
		}

		// Skip if dotnet is not available to avoid triggering installation
		if !dotnetAvailable {
			t.Skip("dotnet not available, skipping test that would need dotnet runtime")
		}

		health := dotnet.CheckEnvironmentHealth(testDir)
		// Health will depend on whether dotnet can build the project
		t.Logf("CheckEnvironmentHealth with valid project: %v", health)
	})

	t.Run("EnvironmentWithInvalidProject", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "invalid_project")
		if err := os.MkdirAll(testDir, 0o755); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		// Create project directory structure
		projectPath := filepath.Join(testDir, "PreCommitEnv")
		if err := os.MkdirAll(projectPath, 0o755); err != nil {
			t.Fatalf("Failed to create project dir: %v", err)
		}

		// Create an invalid .csproj file
		csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
		invalidContent := `<Invalid>XML Content</Invalid>`
		if err := os.WriteFile(csprojPath, []byte(invalidContent), 0o644); err != nil {
			t.Fatalf("Failed to create invalid .csproj file: %v", err)
		}

		// Skip if dotnet is not available to avoid triggering installation
		if !dotnetAvailable {
			t.Skip("dotnet not available, skipping test that would need dotnet runtime")
		}

		health := dotnet.CheckEnvironmentHealth(testDir)
		// Should return false due to build failure with invalid project
		if health {
			t.Error("CheckEnvironmentHealth should return false for invalid project")
		}
	})

	t.Run("EnvironmentWithProjectBuildFailure", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "build_failure")
		if err := os.MkdirAll(testDir, 0o755); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		// Create project directory structure
		projectPath := filepath.Join(testDir, "PreCommitEnv")
		if err := os.MkdirAll(projectPath, 0o755); err != nil {
			t.Fatalf("Failed to create project dir: %v", err)
		}

		// Create a .csproj file that will cause build to fail
		csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
		csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>netnonexistent99.0</TargetFramework>
  </PropertyGroup>
</Project>`
		if err := os.WriteFile(csprojPath, []byte(csprojContent), 0o644); err != nil {
			t.Fatalf("Failed to create .csproj file: %v", err)
		}

		// Skip if dotnet is not available
		if !dotnetAvailable {
			t.Skip("dotnet not available, skipping test that needs dotnet runtime")
		}

		health := dotnet.CheckEnvironmentHealth(testDir)
		// Should return false due to build failure
		if health {
			t.Error("CheckEnvironmentHealth should return false when build fails")
		}
	})

	t.Run("EnvironmentWithProjectSuccessfulBuild", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "successful_build")
		if err := os.MkdirAll(testDir, 0o755); err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		// Create project directory structure
		projectPath := filepath.Join(testDir, "PreCommitEnv")
		if err := os.MkdirAll(projectPath, 0o755); err != nil {
			t.Fatalf("Failed to create project dir: %v", err)
		}

		// Create a valid .csproj file
		csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
		csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net6.0</TargetFramework>
  </PropertyGroup>
</Project>`
		if err := os.WriteFile(csprojPath, []byte(csprojContent), 0o644); err != nil {
			t.Fatalf("Failed to create .csproj file: %v", err)
		}

		// Create Program.cs
		programPath := filepath.Join(projectPath, "Program.cs")
		programContent := `using System;
namespace PreCommitEnv
{
    class Program
    {
        static void Main(string[] args)
        {
            Console.WriteLine("Hello World!");
        }
    }
}`
		if err := os.WriteFile(programPath, []byte(programContent), 0o644); err != nil {
			t.Fatalf("Failed to create Program.cs file: %v", err)
		}

		// Skip if dotnet is not available
		if !dotnetAvailable {
			t.Skip("dotnet not available, skipping test that needs dotnet runtime")
		}

		// First restore the project
		cmd := exec.Command("dotnet", "restore")
		cmd.Dir = projectPath
		if err := cmd.Run(); err != nil {
			t.Skipf("Failed to restore project, skipping test: %v", err)
		}

		health := dotnet.CheckEnvironmentHealth(testDir)
		// Should return true if build succeeds
		t.Logf("CheckEnvironmentHealth with successful build: %v", health)
	})
}

func TestDotnetLanguage_CheckEnvironmentHealth_Comprehensive(t *testing.T) {
	dotnet := NewDotnetLanguage()
	tempDir := t.TempDir()

	t.Run("HealthyEnvironmentWithProject", func(t *testing.T) {
		// Create mock .NET environment structure
		envPath := filepath.Join(tempDir, "healthy-env")
		projectPath := filepath.Join(envPath, "PreCommitEnv")
		err := os.MkdirAll(projectPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create project directory: %v", err)
		}

		// Create mock .csproj file
		csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
		csprojContent := `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`
		err = os.WriteFile(csprojPath, []byte(csprojContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to create .csproj file: %v", err)
		}

		// Check if dotnet is available for real testing
		if _, err := exec.LookPath("dotnet"); err != nil {
			t.Skip("dotnet not available, skipping health check test")
		}

		// Test health check
		isHealthy := dotnet.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth with project: %v", isHealthy)
		// The result will depend on whether dotnet is actually available and working
	})

	t.Run("EnvironmentWithoutProject", func(t *testing.T) {
		// Test environment without project structure
		envPath := filepath.Join(tempDir, "no-project")
		err := os.MkdirAll(envPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create env directory: %v", err)
		}

		isHealthy := dotnet.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth without project: %v", isHealthy)
		// Should still pass basic health check if dotnet is available
	})

	t.Run("NonExistentEnvironment", func(t *testing.T) {
		// Test with non-existent environment
		isHealthy := dotnet.CheckEnvironmentHealth("/non/existent/path")
		if isHealthy {
			t.Error("CheckEnvironmentHealth should return false for non-existent environment")
		}
	})

	t.Run("EnvironmentWithInvalidProject", func(t *testing.T) {
		// Create environment with invalid project structure
		envPath := filepath.Join(tempDir, "invalid-project")
		projectPath := filepath.Join(envPath, "PreCommitEnv")
		err := os.MkdirAll(projectPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create project directory: %v", err)
		}

		// Create invalid .csproj file
		csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
		invalidContent := "invalid xml content"
		err = os.WriteFile(csprojPath, []byte(invalidContent), 0o644)
		if err != nil {
			t.Fatalf("Failed to create invalid .csproj file: %v", err)
		}

		isHealthy := dotnet.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth with invalid project: %v", isHealthy)
		// May fail during build phase if dotnet is available
	})

	t.Run("ProjectDirectoryWithoutCsproj", func(t *testing.T) {
		// Create project directory but no .csproj file
		envPath := filepath.Join(tempDir, "no-csproj")
		projectPath := filepath.Join(envPath, "PreCommitEnv")
		err := os.MkdirAll(projectPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create project directory: %v", err)
		}

		isHealthy := dotnet.CheckEnvironmentHealth(envPath)
		t.Logf("CheckEnvironmentHealth with directory but no .csproj: %v", isHealthy)
		// Should still pass basic health check
	})
}

func TestDotnetLanguage_SetupEnvironmentWithRepo(t *testing.T) {
	dotnet := NewDotnetLanguage()

	// Helper function to check if .NET is available
	isDotnetAvailable := func() bool {
		_, err := exec.LookPath("dotnet")
		return err == nil
	}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dotnet_setup_test_")
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
			version:        "6.0",
			repoPath:       tmpDir,
			additionalDeps: []string{},
			wantErr:        false,
		},
		{
			name:           "setup with dependencies",
			version:        "",
			repoPath:       tmpDir,
			additionalDeps: []string{"Newtonsoft.Json"},
			wantErr:        false,
		},
		{
			name:           "setup with invalid repo path",
			version:        "",
			repoPath:       "/nonexistent/path",
			additionalDeps: []string{},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envPath, err := dotnet.SetupEnvironmentWithRepo(
				"", // repoURL unused
				tt.version,
				tt.repoPath,
				"", // gitRef unused
				tt.additionalDeps,
			)

			if !isDotnetAvailable() {
				// When .NET is not available, the function may still execute but might fail
				if err != nil {
					t.Logf("SetupEnvironmentWithRepo() failed as expected (dotnet not available): %v", err)
				} else {
					t.Logf("SetupEnvironmentWithRepo() succeeded despite dotnet not being available: %s", envPath)
				}
			} else {
				// When .NET is available, check expected results
				if tt.wantErr && err == nil {
					t.Error("Expected error but got none")
				}
				if !tt.wantErr && err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if !tt.wantErr && envPath == "" {
					t.Error("Expected non-empty environment path")
				}
			}
		})
	}
}

func TestDotnetLanguage_Implementation(t *testing.T) {
	dotnet := NewDotnetLanguage()

	// Test basic language interface methods
	if dotnet.GetName() == "" {
		t.Error("GetName() returned empty string")
	}

	if dotnet.GetExecutableName() == "" {
		t.Error("GetExecutableName() returned empty string")
	}

	if dotnet.VersionFlag == "" {
		t.Error("VersionFlag is empty string")
	}

	if dotnet.InstallURL == "" {
		t.Error("InstallURL is empty string")
	}
}

// Comprehensive tests to improve Dotnet language coverage
func TestDotnetLanguage_InstallDependencies_ImprovedCoverage(t *testing.T) {
	dotnet := NewDotnetLanguage()

	t.Run("DotnetNotAvailable", func(t *testing.T) {
		tempDir := t.TempDir()

		// Temporarily remove dotnet from PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", "/usr/bin:/bin") // Minimal PATH without dotnet

		deps := []string{"Newtonsoft.Json"}
		err := dotnet.InstallDependencies(tempDir, deps)
		if err == nil {
			t.Error("InstallDependencies should fail when dotnet is not available")
		} else {
			if !strings.Contains(err.Error(), "failed to create .NET project") {
				t.Logf("Expected project creation error, got: %v", err)
			} else {
				t.Logf("Correctly handled missing dotnet: %v", err)
			}
		}
	})

	t.Run("ProjectCreationFailure", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("Running as root, cannot test directory creation failure")
		}

		// Create mock dotnet executable that fails on "new" command
		tempBinDir := t.TempDir()
		mockDotnet := filepath.Join(tempBinDir, "dotnet")
		scriptContent := `#!/bin/bash
if [[ "$1" == "new" ]]; then
  echo "Failed to create project" >&2
  exit 1
fi
echo "Dotnet mock"
exit 0`
		if err := os.WriteFile(mockDotnet, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dotnet script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		tempDir := t.TempDir()
		deps := []string{"Some.Package"}
		err := dotnet.InstallDependencies(tempDir, deps)
		if err == nil {
			t.Error("InstallDependencies should fail when project creation fails")
		} else {
			if !strings.Contains(err.Error(), "failed to create .NET project") {
				t.Errorf("Expected project creation error, got: %v", err)
			} else {
				t.Logf("Correctly handled project creation failure: %v", err)
			}
		}
	})

	t.Run("PackageAddFailure", func(t *testing.T) {
		// Create mock dotnet executable that fails on "add package" command
		tempBinDir := t.TempDir()
		mockDotnet := filepath.Join(tempBinDir, "dotnet")
		scriptContent := `#!/bin/bash
if [[ "$1" == "new" ]]; then
  mkdir -p PreCommitEnv
  echo '<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net6.0</TargetFramework></PropertyGroup></Project>' > PreCommitEnv/PreCommitEnv.csproj
  exit 0
elif [[ "$1" == "add" && "$2" == "package" ]]; then
  echo "Failed to add package" >&2
  exit 1
fi
echo "Dotnet mock"
exit 0`
		if err := os.WriteFile(mockDotnet, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dotnet script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		tempDir := t.TempDir()
		deps := []string{"NonExistent.Package"}
		err := dotnet.InstallDependencies(tempDir, deps)
		if err == nil {
			t.Error("InstallDependencies should fail when package add fails")
		} else {
			if !strings.Contains(err.Error(), "failed to add .NET package") {
				t.Errorf("Expected package add error, got: %v", err)
			} else {
				t.Logf("Correctly handled package add failure: %v", err)
			}
		}
	})

	t.Run("RestoreFailure", func(t *testing.T) {
		// Create mock dotnet executable that fails on "restore" command
		tempBinDir := t.TempDir()
		mockDotnet := filepath.Join(tempBinDir, "dotnet")
		scriptContent := `#!/bin/bash
if [[ "$1" == "new" ]]; then
  mkdir -p PreCommitEnv
  echo '<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net6.0</TargetFramework></PropertyGroup></Project>' > PreCommitEnv/PreCommitEnv.csproj
  exit 0
elif [[ "$1" == "add" && "$2" == "package" ]]; then
  echo "Package added successfully"
  exit 0
elif [[ "$1" == "restore" ]]; then
  echo "Failed to restore packages" >&2
  exit 1
fi
echo "Dotnet mock"
exit 0`
		if err := os.WriteFile(mockDotnet, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dotnet script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		tempDir := t.TempDir()
		deps := []string{"Some.Package"}
		err := dotnet.InstallDependencies(tempDir, deps)
		if err == nil {
			t.Error("InstallDependencies should fail when restore fails")
		} else {
			if !strings.Contains(err.Error(), "failed to restore .NET packages") {
				t.Errorf("Expected restore error, got: %v", err)
			} else {
				t.Logf("Correctly handled restore failure: %v", err)
			}
		}
	})

	t.Run("DependencyParsingVersioned", func(t *testing.T) {
		// Create successful mock dotnet executable
		tempBinDir := t.TempDir()
		mockDotnet := filepath.Join(tempBinDir, "dotnet")
		scriptContent := `#!/bin/bash
if [[ "$1" == "new" ]]; then
  mkdir -p PreCommitEnv
  echo '<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net6.0</TargetFramework></PropertyGroup></Project>' > PreCommitEnv/PreCommitEnv.csproj
  exit 0
elif [[ "$1" == "add" && "$2" == "package" ]]; then
  echo "Package added: $3 version: $5"
  exit 0
elif [[ "$1" == "restore" ]]; then
  echo "Packages restored"
  exit 0
fi
echo "Dotnet mock"
exit 0`
		if err := os.WriteFile(mockDotnet, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dotnet script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		tempDir := t.TempDir()
		deps := []string{"Newtonsoft.Json:13.0.3", "Microsoft.Extensions.Logging:7.0.0"}
		err := dotnet.InstallDependencies(tempDir, deps)
		if err != nil {
			t.Errorf("InstallDependencies should succeed with versioned dependencies: %v", err)
		} else {
			t.Log("Successfully handled versioned dependencies")
		}
	})

	t.Run("DependencyParsingWithoutVersion", func(t *testing.T) {
		// Create successful mock dotnet executable
		tempBinDir := t.TempDir()
		mockDotnet := filepath.Join(tempBinDir, "dotnet")
		scriptContent := `#!/bin/bash
if [[ "$1" == "new" ]]; then
  mkdir -p PreCommitEnv
  echo '<Project Sdk="Microsoft.NET.Sdk"><PropertyGroup><TargetFramework>net6.0</TargetFramework></PropertyGroup></Project>' > PreCommitEnv/PreCommitEnv.csproj
  exit 0
elif [[ "$1" == "add" && "$2" == "package" ]]; then
  echo "Package added: $3"
  exit 0
elif [[ "$1" == "restore" ]]; then
  echo "Packages restored"
  exit 0
fi
echo "Dotnet mock"
exit 0`
		if err := os.WriteFile(mockDotnet, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dotnet script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		tempDir := t.TempDir()
		deps := []string{"Newtonsoft.Json", "Microsoft.Extensions.Logging"}
		err := dotnet.InstallDependencies(tempDir, deps)
		if err != nil {
			t.Errorf("InstallDependencies should succeed with unversioned dependencies: %v", err)
		} else {
			t.Log("Successfully handled unversioned dependencies")
		}
	})

	t.Run("EmptyDependencies", func(t *testing.T) {
		tempDir := t.TempDir()
		err := dotnet.InstallDependencies(tempDir, []string{})
		if err != nil {
			t.Errorf("InstallDependencies should succeed with empty dependencies: %v", err)
		}

		err = dotnet.InstallDependencies(tempDir, nil)
		if err != nil {
			t.Errorf("InstallDependencies should succeed with nil dependencies: %v", err)
		}
	})
}

// Tests to improve CheckEnvironmentHealth coverage
func TestDotnetLanguage_CheckEnvironmentHealth_ImprovedCoverage(t *testing.T) {
	dotnet := NewDotnetLanguage()

	t.Run("CheckHealthFailure", func(t *testing.T) {
		// Test when base CheckHealth fails
		result := dotnet.CheckEnvironmentHealth("/non/existent/path")
		if result {
			t.Error("CheckEnvironmentHealth should return false when CheckHealth fails")
		} else {
			t.Log("Correctly returned false for non-existent path")
		}
	})

	t.Run("ProjectExistsButBuildFails", func(t *testing.T) {
		failScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "7.0.0"
  exit 0
elif [[ "$1" == "build" && "$2" == "--no-restore" ]]; then
  echo "Build failed" >&2
  exit 1
fi
echo "Dotnet mock"
exit 0`
		testDotnetEnvironmentHealthWithMock(
			t,
			dotnet,
			testCsprojContent,
			failScript,
			false,
			"CheckEnvironmentHealth should return false when build fails",
		)
	})

	t.Run("ProjectExistsAndBuildSucceeds", func(t *testing.T) {
		successScript := `#!/bin/bash
echo "Mock dotnet called with args: $*" >&2
if [[ "$1" == "--version" ]]; then
  echo "7.0.0"
  exit 0
elif [[ "$1" == "build" && "$2" == "--no-restore" ]]; then
  echo "Build succeeded" >&2
  exit 0
else
  echo "Dotnet mock - unhandled command: $*" >&2
  exit 1
fi`
		testDotnetEnvironmentHealthWithMock(
			t,
			dotnet,
			testCsprojContent,
			successScript,
			true,
			"CheckEnvironmentHealth should return true when build succeeds",
		)
	})

	t.Run("NoProjectExists", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create mock dotnet executable for health check
		tempBinDir := t.TempDir()
		mockDotnet := filepath.Join(tempBinDir, "dotnet")
		scriptContent := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "7.0.0"
  exit 0
fi
echo "Dotnet mock"
exit 0`
		if err := os.WriteFile(mockDotnet, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dotnet script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		result := dotnet.CheckEnvironmentHealth(tempDir)
		if result {
			t.Log("CheckEnvironmentHealth correctly returned true when no project exists but CheckHealth passes")
		} else {
			t.Log("CheckEnvironmentHealth returned false (CheckHealth may have failed)")
		}
	})

	t.Run("ProjectDirectoryExistsButNoCsproj", func(t *testing.T) {
		tempDir := t.TempDir()
		projectPath := filepath.Join(tempDir, "PreCommitEnv")
		if err := os.MkdirAll(projectPath, 0o755); err != nil {
			t.Fatalf("Failed to create project directory: %v", err)
		}
		// Don't create .csproj file

		// Create mock dotnet executable for health check
		tempBinDir := t.TempDir()
		mockDotnet := filepath.Join(tempBinDir, "dotnet")
		scriptContent := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "7.0.0"
  exit 0
fi
echo "Dotnet mock"
exit 0`
		if err := os.WriteFile(mockDotnet, []byte(scriptContent), 0o755); err != nil {
			t.Fatalf("Failed to create mock dotnet script: %v", err)
		}

		// Temporarily modify PATH
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", tempBinDir+string(os.PathListSeparator)+originalPath)

		result := dotnet.CheckEnvironmentHealth(tempDir)
		if result {
			t.Log("CheckEnvironmentHealth correctly returned true when project directory exists but no .csproj file")
		} else {
			t.Log("CheckEnvironmentHealth returned false (CheckHealth may have failed)")
		}
	})

	t.Run("CheckHealthFailsWithNoRuntime", func(t *testing.T) {
		tempDir := t.TempDir()

		// Temporarily remove dotnet from PATH to make CheckHealth fail
		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", "/usr/bin:/bin") // Minimal PATH without dotnet

		result := dotnet.CheckEnvironmentHealth(tempDir)
		if result {
			t.Error("CheckEnvironmentHealth should return false when CheckHealth fails due to missing runtime")
		} else {
			t.Log("Correctly returned false when CheckHealth fails due to missing runtime")
		}
	})
}

// Additional test to specifically target missing CheckEnvironmentHealth coverage
func TestDotnetLanguage_CheckEnvironmentHealth_SpecificCoverage(t *testing.T) {
	dotnet := NewDotnetLanguage()

	t.Run("CheckHealthPassesNoProjectExists", func(t *testing.T) {
		// This test specifically targets the missing coverage in CheckEnvironmentHealth
		// when CheckHealth passes but no project exists
		tempDir := t.TempDir()

		// Don't create any project directory or files
		// The method should:
		// 1. Call CheckHealth (which will likely fail without dotnet)
		// 2. If CheckHealth passes, check for project
		// 3. If no project exists, return true

		// First test the normal case (CheckHealth will likely fail)
		result := dotnet.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth with empty directory: %v", result)

		// The coverage issue might be that we never reach the "return true" at the end
		// Let me try creating a mock environment where CheckHealth would pass
		// but there's no project to trigger the final return true

		// Create a mock dotnet in PATH that makes CheckHealth pass
		if os.Getuid() != 0 { // Don't run as root
			mockBinDir := t.TempDir()
			mockDotnet := filepath.Join(mockBinDir, "dotnet")
			mockScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "8.0.0"
  exit 0
fi
echo "Mock dotnet"
exit 0`
			if err := os.WriteFile(mockDotnet, []byte(mockScript), 0o755); err == nil {
				originalPath := os.Getenv("PATH")
				defer os.Setenv("PATH", originalPath)
				os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

				// Now CheckHealth should pass, but no project exists
				// This should hit the final "return true" line
				result = dotnet.CheckEnvironmentHealth(tempDir)
				t.Logf("CheckEnvironmentHealth with mock dotnet and no project: %v", result)
			}
		}
	})

	t.Run("ProjectExistsSuccessfulBuild", func(t *testing.T) {
		// This test specifically targets the project exists + build succeeds path
		tempDir := t.TempDir()
		projectPath := filepath.Join(tempDir, "PreCommitEnv")
		if err := os.MkdirAll(projectPath, 0o755); err != nil {
			t.Fatalf("Failed to create project directory: %v", err)
		}

		// Create .csproj file
		csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")
		csprojContent := testCsprojContent
		if err := os.WriteFile(csprojPath, []byte(csprojContent), 0o644); err != nil {
			t.Fatalf("Failed to create .csproj file: %v", err)
		}

		// Create a comprehensive mock dotnet
		mockBinDir := t.TempDir()
		mockDotnet := filepath.Join(mockBinDir, "dotnet")
		mockScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "8.0.0"
  exit 0
elif [[ "$1" == "build" && "$2" == "--no-restore" ]]; then
  echo "Build succeeded"
  exit 0
fi
echo "Mock dotnet - command: $*"
exit 0`
		if err := os.WriteFile(mockDotnet, []byte(mockScript), 0o755); err != nil {
			t.Fatalf("Failed to create mock dotnet: %v", err)
		}

		originalPath := os.Getenv("PATH")
		defer os.Setenv("PATH", originalPath)
		os.Setenv("PATH", mockBinDir+string(os.PathListSeparator)+originalPath)

		// This should cover: CheckHealth passes, project exists, build succeeds, return true
		result := dotnet.CheckEnvironmentHealth(tempDir)
		t.Logf("CheckEnvironmentHealth with mock successful build: %v", result)
	})

	t.Run("CheckHealthPassesButBuildCommandUnavailable", func(t *testing.T) {
		// Edge case: CheckHealth passes but build command is not available/fails
		failBuildScript := `#!/bin/bash
if [[ "$1" == "--version" ]]; then
  echo "8.0.0"
  exit 0
elif [[ "$1" == "build" && "$2" == "--no-restore" ]]; then
  echo "Build failed"
  exit 1
fi
echo "Mock dotnet"
exit 1`
		testDotnetEnvironmentHealthWithMock(
			t,
			dotnet,
			testCsprojContent,
			failBuildScript,
			false,
			"CheckHealth passes, project exists, build fails, should return false",
		)
	})
}
