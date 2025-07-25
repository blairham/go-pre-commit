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
			".NET",
			"dotnet",
			"--version",
			"https://dotnet.microsoft.com/download",
		),
	}
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

// CheckEnvironmentHealth checks if the .NET environment is healthy
func (d *DotnetLanguage) CheckEnvironmentHealth(envPath string) bool {
	// Check base health first
	if err := d.CheckHealth(envPath, ""); err != nil {
		return false
	}

	// Check if project exists
	projectPath := filepath.Join(envPath, "PreCommitEnv")
	csprojPath := filepath.Join(projectPath, "PreCommitEnv.csproj")

	if _, err := os.Stat(csprojPath); err == nil {
		// Project exists, check if we can build
		cmd := exec.Command("dotnet", "build", "--no-restore")
		cmd.Dir = projectPath

		if err := cmd.Run(); err != nil {
			return false
		}
	}

	return true
}

// SetupEnvironmentWithRepo sets up a .NET environment for a specific repository
func (d *DotnetLanguage) SetupEnvironmentWithRepo(
	_, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Use the simpler setup for now - can be enhanced later if needed
	return d.GenericSetupEnvironmentWithRepo("", version, repoPath, additionalDeps)
}
