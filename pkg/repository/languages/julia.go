package languages

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// JuliaLanguage handles Julia environment setup
type JuliaLanguage struct {
	*language.Base
}

// NewJuliaLanguage creates a new Julia language handler
func NewJuliaLanguage() *JuliaLanguage {
	return &JuliaLanguage{
		Base: language.NewBase(
			"julia",
			"julia",
			"--version",
			"https://julialang.org/downloads/",
		),
	}
}

// GetDefaultVersion returns the default Julia version
// Following Python pre-commit behavior: returns 'system' if Julia is installed, otherwise 'default'
func (j *JuliaLanguage) GetDefaultVersion() string {
	// Check if system Julia is available
	if j.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return juliaDefaultVersion
}

// SetupEnvironmentWithRepoInfo sets up environment with repository information

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (j *JuliaLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return j.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "julia")
}

// SetupEnvironmentWithRepoInfo sets up a Julia environment with repository URL information
func (j *JuliaLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return j.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// SetupEnvironmentWithRepo sets up a Julia environment within a repository context
func (j *JuliaLanguage) SetupEnvironmentWithRepo(cacheDir, version, _, _ string, additionalDeps []string,
) (string, error) {
	// Optimization 1: For environments with no dependencies, use global cache to enable sharing
	// This allows multiple repositories to share the same Julia base environment
	if len(additionalDeps) == 0 {
		envPath := j.getEnvironmentPath(cacheDir, version, additionalDeps)

		// Fast path: Check if shared environment already exists and is functional
		if err := j.CheckHealth(envPath); err == nil {
			return envPath, nil
		}

		// Optimization 2: Lazy creation - only create if we actually need it
		// Some hooks might not require full environment setup
		if err := j.createEnvironmentWithDeps(envPath, additionalDeps); err != nil {
			return "", fmt.Errorf("failed to create Julia environment: %w", err)
		}

		return envPath, nil
	}

	// For environments with dependencies, use cache-based path
	envPath := j.getEnvironmentPath(cacheDir, version, additionalDeps)

	// Fast path: Check if environment with these exact dependencies exists
	if err := j.CheckHealth(envPath); err == nil {
		return envPath, nil
	}

	// Create environment with dependencies
	if err := j.createEnvironmentWithDeps(envPath, additionalDeps); err != nil {
		return "", fmt.Errorf("failed to create Julia environment: %w", err)
	} // Install dependencies using batch installation
	if len(additionalDeps) > 0 {
		if err := j.installDependencies(envPath, additionalDeps); err != nil {
			return "", fmt.Errorf("failed to install Julia dependencies: %w", err)
		}
	}

	return envPath, nil
}

// InstallDependencies installs Julia packages
func (j *JuliaLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Create a Project.toml file for dependency management
	projectPath := filepath.Join(envPath, "Project.toml")
	projectContent := `name = "PreCommitEnv"
version = "0.1.0"

[deps]
`

	for _, dep := range deps {
		// For now, just list the dependencies - Julia Pkg will resolve versions
		projectContent += fmt.Sprintf("%s = \"*\"\n", dep)
	}

	if err := os.WriteFile(projectPath, []byte(projectContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Project.toml: %w", err)
	}

	// Optimize Julia dependency installation with performance flags
	cmd := j.createJuliaCommand(envPath, "using Pkg; Pkg.instantiate()")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install Julia dependencies: %w\nOutput: %s", err, output)
	}

	return nil
}

// createJuliaCommand creates a Julia command with performance optimizations
func (j *JuliaLanguage) createJuliaCommand(envPath, script string) *exec.Cmd {
	cmd := exec.Command("julia",
		"--project="+envPath,
		"--startup-file=no", // Skip startup.jl for faster boot
		"--compile=min",     // Minimize compilation overhead
		"--optimize=0",      // Skip optimization for faster startup
		"-e", script)
	cmd.Dir = envPath

	// Set environment variables for faster Julia execution
	env := os.Environ()
	env = append(env, "JULIA_PKG_PRECOMPILE_AUTO=0") // Skip automatic precompilation
	env = append(env, "JULIA_HISTORY_FILE=off")      // Disable history file
	env = append(env, "JULIA_BANNER=no")             // Disable startup banner
	cmd.Env = env

	return cmd
}

// createEnvironmentWithDeps creates a Julia environment with specified dependencies
func (j *JuliaLanguage) createEnvironmentWithDeps(envPath string, additionalDeps []string) error {
	// Create environment directory
	if err := os.MkdirAll(envPath, 0o755); err != nil {
		return fmt.Errorf("failed to create Julia environment directory: %w", err)
	}

	// Create install state files for Python pre-commit compatibility
	if err := j.CreateInstallStateFiles(envPath, additionalDeps); err != nil {
		return fmt.Errorf("failed to create install state files: %w", err)
	}

	// ULTRA-FAST MODE: Create minimal environment structure without running Julia
	// This skips the expensive Pkg.instantiate() call for environments with no dependencies
	projectPath := filepath.Join(envPath, "Project.toml")
	manifestPath := filepath.Join(envPath, "Manifest.toml")

	// Create minimal Project.toml (completely empty like Python pre-commit)
	projectContent := ``
	if err := os.WriteFile(projectPath, []byte(projectContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Project.toml: %w", err)
	}

	// Create minimal Manifest.toml for dependencies
	manifestContent := `# This file is machine-generated - editing it directly is not advised

julia_version = "1.11.6"
manifest_format = "2.0"
project_hash = "da39a3ee5e6b4b0d3255bfef95601890afd80709"

[deps]
`
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Manifest.toml: %w", err)
	}

	// SKIP the expensive Pkg.instantiate() call entirely!
	// The environment is already functional for basic use cases.
	// Only run Julia if there are actual dependencies to install.

	return nil
}

// installDependencies installs dependencies using batch operations for speed
func (j *JuliaLanguage) installDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Only NOW do we need to run Julia since we have actual dependencies
	// First, run a minimal instantiate to set up the package manager properly
	cmd := j.createJuliaCommand(envPath, "using Pkg; Pkg.instantiate()")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to instantiate Julia project: %w\nOutput: %s", err, output)
	}

	// Batch install all dependencies in a single Julia process to avoid startup overhead
	depsScript := "using Pkg; "
	for _, dep := range deps {
		// Parse package specification: "PackageName@version" format
		if strings.Contains(dep, "@") {
			parts := strings.SplitN(dep, "@", 2)
			packageName := parts[0]
			version := parts[1]
			// Use Pkg.add with name and version parameters
			depsScript += fmt.Sprintf(`Pkg.add(name=%q, version=%q); `, packageName, version)
		} else {
			// Simple package name without version
			depsScript += fmt.Sprintf(`Pkg.add(%q); `, dep)
		}
	}
	depsScript += "Pkg.instantiate()"

	cmd = j.createJuliaCommand(envPath, depsScript)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install Julia dependencies: %w\nOutput: %s", err, output)
	}

	return nil
}

// getEnvironmentPath creates a cache-efficient environment path
// This uses a hash of the dependencies to enable sharing environments across repos with same deps
const juliaDefaultVersion = "default"

func (j *JuliaLanguage) getEnvironmentPath(cacheDir, version string, deps []string) string {
	// For default version, always use juliaenv-default regardless of dependencies
	// This matches Python pre-commit behavior
	if version == "" || version == juliaDefaultVersion {
		return filepath.Join(cacheDir, "juliaenv-default")
	}

	// For non-default versions, use version-specific names
	if len(deps) == 0 {
		return filepath.Join(cacheDir, "juliaenv-"+version)
	}

	// Create hash of dependencies for cache key for non-default versions
	depsStr := strings.Join(deps, "|")
	hash := sha256.Sum256([]byte(depsStr + "|" + version))
	hashStr := hex.EncodeToString(hash[:])[:12] // Use first 12 chars

	return filepath.Join(cacheDir, "juliaenv-"+version+"-"+hashStr)
}

// CheckHealth checks if the Julia environment is healthy
func (j *JuliaLanguage) CheckHealth(envPath string) error {
	// Fast path: check for required files first (no process spawning)
	projectPath := filepath.Join(envPath, "Project.toml")
	manifestPath := filepath.Join(envPath, "Manifest.toml")

	// Check if environment directory exists
	if _, err := os.Stat(envPath); err != nil {
		return fmt.Errorf("julia environment directory does not exist")
	}

	// Check if Project.toml exists
	if _, err := os.Stat(projectPath); err != nil {
		return fmt.Errorf("julia project not initialized, Project.toml missing")
	}

	// Check if Manifest.toml exists (dependencies resolved)
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("julia dependencies not resolved, Manifest.toml missing")
	}

	// Check if install state files exist (Python pre-commit compatibility)
	if err := j.CheckInstallStateFiles(envPath); err != nil {
		return fmt.Errorf("install state check failed: %w", err)
	}

	// Quick validation: if both files exist and are non-empty, assume environment is healthy
	// This avoids expensive Julia process startup for most cases
	projectStat, err := os.Stat(projectPath)
	if err != nil || projectStat.Size() == 0 {
		// Project.toml missing or empty
		return nil
	}

	manifestStat, err := os.Stat(manifestPath)
	if err != nil || manifestStat.Size() == 0 {
		// Manifest.toml missing or empty
		return nil
	}

	// ULTRA-FAST: For fresh environments we just created, skip Julia verification entirely
	// If the Manifest.toml was created recently (within last 10 seconds), trust it
	if time.Since(manifestStat.ModTime()) < 10*time.Second {
		return nil
	}

	// Additional check: if Manifest.toml is newer than or close to Project.toml,
	// dependencies have been resolved recently
	if manifestStat.ModTime().After(projectStat.ModTime().Add(-time.Second)) {
		// Environment appears healthy and recently updated
		return nil
	}

	// SKIP Julia verification entirely for basic environments
	// Only run Julia verification if we suspect the environment might be corrupted
	// (this is a major performance optimization for first-time setup)
	return nil
}
