// Package languages provides language-specific implementations for pre-commit hook environments
package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

const (
	condaExecutable = "conda"
)

// CondaLanguage implements conda environment management exactly like Python pre-commit
type CondaLanguage struct {
	*language.Base
}

// NewCondaLanguage creates a new conda language instance
func NewCondaLanguage() *CondaLanguage {
	return &CondaLanguage{
		Base: language.NewBase(condaExecutable, condaExecutable, "--version", ""),
	}
}

// getCondaExecutable returns the best available conda executable
// Based on testing: conda is actually faster than micromamba for env creation
func (c *CondaLanguage) getCondaExecutable() string {
	// Check environment variables for user preference
	if os.Getenv("PRE_COMMIT_USE_MICROMAMBA") != "" {
		return "micromamba"
	}

	if os.Getenv("PRE_COMMIT_USE_MAMBA") != "" {
		return "mamba"
	}

	// Default to conda
	return condaExecutable
}

// CheckHealth verifies that Conda is working correctly
func (c *CondaLanguage) CheckHealth(_ string) error {
	// Check if conda is available in the system PATH
	if !c.IsRuntimeAvailable() {
		return fmt.Errorf("conda runtime not found in PATH")
	}

	return nil
}

// IsRuntimeAvailable checks if conda/mamba/micromamba is available on the system
func (c *CondaLanguage) IsRuntimeAvailable() bool {
	// Check for micromamba first (fastest)
	if _, err := exec.LookPath("micromamba"); err == nil {
		return true
	}

	// Check for mamba next (faster than conda)
	if _, err := exec.LookPath("mamba"); err == nil {
		return true
	}

	// Check for conda last (slowest but most common)
	if _, err := exec.LookPath(condaExecutable); err == nil {
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
// Simplified to match Python pre-commit exactly - repository-local environment
func (c *CondaLanguage) SetupEnvironmentWithRepo(
	_, version, repoPath, _ string,
	additionalDeps []string,
) (envPath string, err error) {
	envPath = c.GetEnvironmentPath(repoPath, version)

	// Simple check like Python pre-commit: if conda-meta exists, environment is ready
	condaMetaDir := filepath.Join(envPath, "conda-meta")
	if _, statErr := os.Stat(condaMetaDir); statErr == nil {
		return envPath, nil
	}

	// Environment doesn't exist, create it
	// Check if conda runtime is available (fail early like Python pre-commit)
	if !c.IsRuntimeAvailable() {
		return "", fmt.Errorf("conda runtime not available - install conda")
	}

	// Check for environment.yml file (required for conda)
	envFile := filepath.Join(repoPath, "environment.yml")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		return "", fmt.Errorf("conda language requires environment.yml file: %w", err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(envPath), 0o750); err != nil {
		return "", fmt.Errorf("failed to create environment parent directory: %w", err)
	}

	// Create conda environment using optimized approach matching Python pre-commit
	if err := c.createOptimizedCondaEnvironment(envPath, envFile, additionalDeps); err != nil {
		return "", fmt.Errorf("failed to create conda environment: %w", err)
	}

	return envPath, nil
}

// createOptimizedCondaEnvironment creates a conda environment exactly like Python pre-commit
// Uses the exact same command structure as Python pre-commit with no optimization flags
func (c *CondaLanguage) createOptimizedCondaEnvironment(envPath, envFile string, additionalDeps []string) error {
	repoDir := filepath.Dir(envFile)

	// Get conda executable (conda by default, matches Python pre-commit)
	condaCmd := c.getCondaExecutable()

	// Create conda environment with EXACT same command as Python pre-commit:
	// cmd_output_b(conda_exe, 'env', 'create', '-p', env_dir, '--file', 'environment.yml', cwd=prefix.prefix_dir)
	args := []string{"env", "create", "-p", envPath, "--file", "environment.yml"}
	cmd := exec.Command(condaCmd, args...)
	cmd.Dir = repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create conda environment: %w\nOutput: %s", err, string(output))
	}

	// Install additional dependencies if specified - matches Python pre-commit exactly
	if len(additionalDeps) > 0 {
		installArgs := []string{"install", "-p", envPath}
		installArgs = append(installArgs, additionalDeps...)
		cmd := exec.Command(condaCmd, installArgs...)
		cmd.Dir = repoDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install additional dependencies: %w\nOutput: %s", err, string(output))
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

	// Install real dependencies using conda
	args := []string{"install", "-p", envPath, "-c", "conda-forge", "--yes"}
	args = append(args, deps...)
	cmd := exec.Command(c.getCondaExecutable(), args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install conda dependencies: %w", err)
	}
	return nil
}

// GetExecutableName returns the conda executable name
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
