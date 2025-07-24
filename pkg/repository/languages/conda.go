// Package languages provides language-specific implementations for pre-commit hook environments
package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// CondaLanguage implements conda environment management exactly like Python pre-commit
type CondaLanguage struct {
	*language.Base
}

// NewCondaLanguage creates a new conda language instance
func NewCondaLanguage() *CondaLanguage {
	return &CondaLanguage{
		Base: language.NewBase("conda", "conda", "--version", ""),
	}
}

// getCondaExecutable returns the conda executable to use
// Supports conda, mamba, and micromamba based on environment variables
func (c *CondaLanguage) getCondaExecutable() string {
	// Check for micromamba preference first (highest priority)
	if os.Getenv("PRE_COMMIT_USE_MICROMAMBA") != "" {
		return "micromamba"
	}

	// Check for mamba preference (medium priority)
	if os.Getenv("PRE_COMMIT_USE_MAMBA") != "" {
		return "mamba"
	}

	// Default to conda (lowest priority)
	return "conda"
}

// CheckHealth implements health check - conda uses basic_health_check (always healthy)
func (c *CondaLanguage) CheckHealth(_, _ string) error {
	// Python pre-commit: health_check = lang_base.basic_health_check
	// basic_health_check returns None (always healthy)
	return nil
}

// IsRuntimeAvailable checks if the configured conda executable is available on the system
func (c *CondaLanguage) IsRuntimeAvailable() bool {
	// Check for the configured executable (conda, mamba, or micromamba)
	executable := c.getCondaExecutable()
	if _, err := exec.LookPath(executable); err == nil {
		return true
	}

	return false
}

// GetEnvironmentPath returns the conda environment path
// Matches Python: lang_base.environment_dir(prefix, ENVIRONMENT_DIR, version)
func (c *CondaLanguage) GetEnvironmentPath(repoPath, version string) string {
	// Handle empty repoPath to avoid creating directories in CWD
	if repoPath == "" {
		// This should not happen in normal usage, but we handle it defensively
		repoPath = "."
	}

	// Handle empty version by defaulting to "default"
	if version == "" {
		version = "default"
	}

	// Python ENVIRONMENT_DIR = 'conda'
	envDir := "conda-" + version
	return filepath.Join(repoPath, envDir)
}

// NeedsEnvironmentSetup returns true as conda needs environment setup
func (c *CondaLanguage) NeedsEnvironmentSetup() bool {
	return true
}

// SetupEnvironmentWithRepo creates conda environment from environment.yml
// Matches Python pre-commit's install_environment exactly
func (c *CondaLanguage) SetupEnvironmentWithRepo(
	_, version, repoPath, _ string,
	additionalDeps []string,
) (string, error) {
	envPath := c.GetEnvironmentPath(repoPath, version)

	// Check if conda runtime is available (fail early like Python pre-commit)
	if !c.IsRuntimeAvailable() {
		return "", fmt.Errorf("conda runtime not available - install conda")
	}

	// Check for environment.yml file (required for conda)
	envFile := filepath.Join(repoPath, "environment.yml")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		return "", fmt.Errorf("conda language requires environment.yml file: %w", err)
	}

	// Create real conda environment using conda commands (like Python pre-commit)
	if err := c.createRealCondaEnvironment(envPath, envFile, additionalDeps); err != nil {
		return "", fmt.Errorf("failed to create conda environment: %w", err)
	}

	return envPath, nil
} // createRealCondaEnvironment creates a real conda environment using the configured conda executable
// Matches Python pre-commit's install_environment exactly
func (c *CondaLanguage) createRealCondaEnvironment(envPath, envFile string, additionalDeps []string) error {
	repoDir := filepath.Dir(envFile)
	condaExe := c.getCondaExecutable()

	// Create conda environment: {conda_exe} env create -p envdir --file environment.yml
	// Matches Python: cmd_output_b(conda_exe, 'env', 'create', '-p', env_dir, '--file', 'environment.yml', cwd=prefix.prefix_dir)
	cmd := exec.Command(condaExe, "env", "create", "-p", envPath, "--file", "environment.yml")
	cmd.Dir = repoDir

	// Capture output for debugging (matches Python's cmd_output_b behavior)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create conda environment: %w\nOutput: %s", err, string(output))
	}

	// Install additional dependencies if specified
	// Matches Python: if additional_dependencies: cmd_output_b(conda_exe, 'install', '-p', env_dir, *additional_dependencies, cwd=prefix.prefix_dir)
	if len(additionalDeps) > 0 {
		args := []string{"install", "-p", envPath, "-c", "conda-forge", "--yes"}
		args = append(args, additionalDeps...)
		cmd := exec.Command(condaExe, args...)
		cmd.Dir = repoDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install additional conda dependencies: %w\nOutput: %s", err, string(output))
		}
	}

	return nil
}

// SetupEnvironmentWithRepoInfo is an alias for SetupEnvironmentWithRepo
func (c *CondaLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return c.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// PreInitializeEnvironmentWithRepoInfo does nothing for conda
func (c *CondaLanguage) PreInitializeEnvironmentWithRepoInfo(
	_, _, _, _ string,
	_ []string,
) error {
	return nil
}

// InstallDependencies installs additional conda packages
func (c *CondaLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Check if conda runtime is available
	if !c.IsRuntimeAvailable() {
		return fmt.Errorf("conda runtime not available - install conda")
	}

	// Install real dependencies using the configured conda executable
	condaExe := c.getCondaExecutable()
	args := []string{"install", "-p", envPath, "-c", "conda-forge", "--yes"}
	args = append(args, deps...)
	cmd := exec.Command(condaExe, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install conda dependencies: %w", err)
	}
	return nil
}

// GetExecutableName returns the configured conda executable name
func (c *CondaLanguage) GetExecutableName() string {
	return c.getCondaExecutable()
}

// GetEnvironmentBinPath returns the bin path for the conda environment
func (c *CondaLanguage) GetEnvironmentBinPath(envPath string) string {
	if isWindows() {
		// On Windows, conda puts executables in multiple places
		return envPath // Return the base path, executables are in subdirs
	}
	return filepath.Join(envPath, "bin")
}

// CheckEnvironmentHealth checks if conda environment is healthy
func (c *CondaLanguage) CheckEnvironmentHealth(envPath string) bool {
	// Check if conda-meta directory exists (conda environments have this)
	condaMetaPath := filepath.Join(envPath, "conda-meta")
	_, err := os.Stat(condaMetaPath)
	return err == nil
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return os.PathSeparator == '\\'
}
