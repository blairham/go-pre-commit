package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// RustLanguage handles Rust environment setup with rustup
type RustLanguage struct {
	*language.Base
}

// NewRustLanguage creates a new Rust language handler
func NewRustLanguage() *RustLanguage {
	return &RustLanguage{
		Base: language.NewBase(
			"rust",
			"rustc",
			"--version",
			"https://rustup.rs/",
		),
	}
}

// GetDefaultVersion returns the default Rust version
// Following Python pre-commit behavior: returns 'system' if Rust is installed, otherwise 'default'
func (r *RustLanguage) GetDefaultVersion() string {
	// Check if system Rust is available
	if r.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (r *RustLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return r.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "rust")
}

// SetupEnvironmentWithRepoInfo sets up a Rust environment with repository URL information
func (r *RustLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return r.CacheAwareSetupEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "rust")
}

// InstallDependencies installs Rust dependencies (crates) in the environment
func (r *RustLanguage) InstallDependencies(envPath string, deps []string) error {
	cargoBin := filepath.Join(envPath, "bin", "cargo")

	// If cargo is not in the environment, try to use system cargo
	if _, err := os.Stat(cargoBin); err != nil {
		if _, err := exec.LookPath("cargo"); err != nil {
			return fmt.Errorf("cargo not found in environment or system PATH")
		}
		cargoBin = "cargo"
	}

	for _, dep := range deps {
		cmd := exec.Command(cargoBin, "install", dep)
		cmd.Env = append(os.Environ(), "CARGO_HOME="+envPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install Rust dependency %s: %w", dep, err)
		}
	}

	return nil
}

// SetupEnvironmentWithRepo sets up a Rust environment in the repository directory
func (r *RustLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Only support 'default' or 'system' versions
	if version != language.VersionDefault && version != language.VersionSystem {
		version = language.VersionDefault
	}

	// Handle empty repoPath by using cacheDir instead to avoid creating directories in CWD
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir cannot be empty")
		}
		repoPath = cacheDir
	}

	// Create environment in the repository directory (like Python pre-commit)
	envDirName := language.GetRepositoryEnvironmentName("rust", version)
	envPath := filepath.Join(repoPath, envDirName)

	// Check if environment already exists and is functional
	if r.CheckEnvironmentHealth(envPath) {
		return envPath, nil
	}

	// Environment exists but is broken, remove and recreate
	if _, err := os.Stat(envPath); err == nil {
		if err := os.RemoveAll(envPath); err != nil {
			return "", fmt.Errorf("failed to remove broken environment: %w", err)
		}
	}

	// Create environment directory and install state files (DRY)
	if err := r.SetupEnvironmentDirectory(envPath, additionalDeps); err != nil {
		return "", err
	}

	// Install dependencies if needed
	if len(additionalDeps) > 0 {
		if err := r.InstallDependencies(envPath, additionalDeps); err != nil {
			return "", fmt.Errorf("failed to install Rust dependencies: %w", err)
		}
	}

	return envPath, nil
}

// SetupEnvironment sets up a Rust environment
func (r *RustLanguage) SetupEnvironment(cacheDir, version string, additionalDeps []string) (string, error) {
	// For Rust, we use the repository-based setup with cache directory as repo path
	return r.SetupEnvironmentWithRepo(cacheDir, version, cacheDir, "", additionalDeps)
}

// CheckEnvironmentHealth checks if the Rust environment is healthy
func (r *RustLanguage) CheckEnvironmentHealth(envPath string) bool {
	// First check if the environment directory exists
	if _, err := os.Stat(envPath); err != nil {
		return false
	}

	// Check if Rust runtime is available on the system (required for Rust environments)
	if !r.IsRuntimeAvailable() {
		return false
	}

	// Try the health check with default version
	if err := r.CheckHealthWithVersion(envPath, language.VersionDefault); err != nil {
		return false
	}

	return true
}

// GetEnvironmentBinPath returns the bin directory path for the Rust environment
func (r *RustLanguage) GetEnvironmentBinPath(envPath string) string {
	return filepath.Join(envPath, "bin")
}

// CheckHealth verifies the environment is healthy (interface method)
func (r *RustLanguage) CheckHealth(envPath string) error {
	return r.CheckHealthWithVersion(envPath, language.VersionDefault)
}

// CheckHealthWithVersion performs health check for rust environments with version
func (r *RustLanguage) CheckHealthWithVersion(envPath, version string) error {
	// For system version, check if rust is available in system PATH
	if version == language.VersionSystem {
		if _, err := exec.LookPath("rustc"); err != nil {
			return fmt.Errorf("system rust (rustc) not available: %w", err)
		}
		if _, err := exec.LookPath("cargo"); err != nil {
			return fmt.Errorf("system cargo not available: %w", err)
		}
		return nil
	}

	// For environment versions, first check if Rust runtime is available
	// since environment versions still need the underlying rust tools
	if !r.IsRuntimeAvailable() {
		return fmt.Errorf("rust runtime not available on system")
	}

	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment directory does not exist: %s", envPath)
	}

	return nil
}
