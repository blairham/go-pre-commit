package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// DotnetLanguage handles .NET environment setup
type DotnetLanguage struct {
	*language.Base
}

// NewDotnetLanguage creates a new .NET language handler
func NewDotnetLanguage() *DotnetLanguage {
	return &DotnetLanguage{
		Base: language.NewBase(
			"dotnet",
			"dotnet",
			"--version",
			"https://dotnet.microsoft.com/download",
		),
	}
}

// GetDefaultVersion returns the default .NET version
// Following Python pre-commit behavior: returns 'system' if .NET is installed, otherwise 'default'
func (d *DotnetLanguage) GetDefaultVersion() string {
	// Check if system .NET is available
	if d.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// InstallDependencies installs .NET packages
func (d *DotnetLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Create a simple console project
	cmd := exec.Command("dotnet", "new", "console", "-n", "PreCommitEnv")
	cmd.Dir = envPath

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create .NET project: %w\nOutput: %s", err, output)
	}

	projectPath := filepath.Join(envPath, "PreCommitEnv")

	// Install each dependency
	for _, dep := range deps {
		// Parse dependency specification (name:version or just name)
		parts := strings.Split(dep, ":")
		var pkg, version string
		if len(parts) == 2 {
			pkg = parts[0]
			version = parts[1]
		} else {
			pkg = dep
		}

		var addCmd *exec.Cmd
		if version != "" {
			addCmd = exec.Command("dotnet", "add", "package", pkg, "--version", version)
		} else {
			addCmd = exec.Command("dotnet", "add", "package", pkg)
		}
		addCmd.Dir = projectPath

		if output, err := addCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to add .NET package %s: %w\nOutput: %s", pkg, err, output)
		}
	}

	// Restore packages
	restoreCmd := exec.Command("dotnet", "restore")
	restoreCmd.Dir = projectPath

	if output, err := restoreCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restore .NET packages: %w\nOutput: %s", err, output)
	}

	return nil
}

// CheckHealth checks if .NET is available and functional
func (d *DotnetLanguage) CheckHealth(envPath string) error {
	// For .NET, we check the system-installed dotnet rather than an environment-specific one
	// since .NET is typically installed system-wide, not in isolated environments

	// Check if dotnet is available in PATH
	if !d.IsRuntimeAvailable() {
		return fmt.Errorf("dotnet runtime not found in PATH")
	}

	// Test basic functionality
	cmd := exec.Command("dotnet", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dotnet runtime not functional: %w", err)
	}

	// If there's an environment path, check if the project structure exists
	if envPath != "" {
		projectPath := filepath.Join(envPath, "PreCommitEnv")
		csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")

		// If project exists, try to validate it
		if _, err := os.Stat(csprojPath); err == nil {
			cmd := exec.Command("dotnet", "build", "--no-restore")
			cmd.Dir = projectPath
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("dotnet project validation failed: %w", err)
			}
		}
	}

	return nil
}

// SetupEnvironmentWithRepo sets up a .NET environment for a specific repository
func (d *DotnetLanguage) SetupEnvironmentWithRepo(
	_, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Use the simpler setup for now - can be enhanced later if needed
	return d.GenericSetupEnvironmentWithRepo("", version, repoPath, additionalDeps)
}

// CheckEnvironmentHealth checks if the .NET environment is healthy
func (d *DotnetLanguage) CheckEnvironmentHealth(envPath string) bool {
	// Check that the environment directory exists
	if _, err := os.Stat(envPath); err != nil {
		return false
	}

	// Check base health (dotnet --version works)
	if err := d.CheckHealth(envPath); err != nil {
		return false
	}

	// If there's a project, check if it can build
	projectPath := filepath.Join(envPath, "PreCommitEnv")
	csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")

	if _, err := os.Stat(csprojPath); err == nil {
		// Project exists, check if it can build
		cmd := exec.Command("dotnet", "build", "--no-restore")
		cmd.Dir = projectPath
		if err := cmd.Run(); err != nil {
			return false
		}
	}

	return true
}
