package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// RLanguage handles R environment setup
type RLanguage struct {
	*language.Base
}

// NewRLanguage creates a new R language handler
func NewRLanguage() *RLanguage {
	return &RLanguage{
		Base: language.NewBase("r", "R", "--version", "https://www.r-project.org/"),
	}
}

// GetDefaultVersion returns the default R version
// Following Python pre-commit behavior: returns 'system' if R is installed, otherwise 'default'
func (r *RLanguage) GetDefaultVersion() string {
	// Check if system R is available
	if r.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (r *RLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return r.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "r")
}

// SetupEnvironmentWithRepoInfo sets up an R environment with repository URL information
func (r *RLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return r.CacheAwareSetupEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "r")
}

// InstallDependencies installs R packages
func (r *RLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Create local library directory
	libPath := filepath.Join(envPath, "library")
	if err := os.MkdirAll(libPath, 0o750); err != nil {
		return fmt.Errorf("failed to create library directory: %w", err)
	}

	// Install each dependency
	for _, dep := range deps {
		// Parse dependency specification (name==version or just name)
		parts := strings.Split(dep, "==")
		var pkg, version string
		if len(parts) == 2 {
			pkg = parts[0]
			version = parts[1]
		} else {
			pkg = dep
		}

		// Create R script to install package
		var rScript string
		if version != "" {
			// Install specific version using devtools or remotes
			rScript = fmt.Sprintf(`
.libPaths("%s")
if (!require("remotes", quietly = TRUE)) {
  install.packages("remotes", lib = "%s", repos = "https://cran.r-project.org/")
}
remotes::install_version("%s", version = "%s", lib = "%s")
`, libPath, libPath, pkg, version, libPath)
		} else {
			// Install latest version
			rScript = fmt.Sprintf(`
.libPaths("%s")
install.packages("%s", lib = "%s", repos = "https://cran.r-project.org/")
`, libPath, pkg, libPath)
		}

		cmd := exec.Command("R", "--slave", "--no-restore", "-e", rScript)
		cmd.Env = append(os.Environ(), "R_LIBS="+libPath)

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to install R package %s: %w\nOutput: %s", pkg, err, output)
		}
	}

	return nil
}

// CheckEnvironmentHealth checks if the R environment is healthy
func (r *RLanguage) CheckEnvironmentHealth(envPath string) bool {
	// Check base health first
	if err := r.CheckHealth(envPath, ""); err != nil {
		return false
	}

	// Check if library directory exists (if dependencies were installed)
	libPath := filepath.Join(envPath, "library")
	if _, err := os.Stat(libPath); err == nil {
		// library directory exists, try to verify R can find packages
		rScript := fmt.Sprintf(`.libPaths(%q); .libPaths()`, libPath)
		cmd := exec.Command("R", "--slave", "--no-restore", "-e", rScript)
		cmd.Env = append(os.Environ(), "R_LIBS="+libPath)

		if err := cmd.Run(); err != nil {
			return false
		}
	}

	return true
}

// SetupEnvironmentWithRepo sets up an R environment for a specific repository
func (r *RLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Only support 'default' or 'system' versions
	if version != language.VersionDefault && version != language.VersionSystem {
		version = language.VersionDefault
	}

	// Use the centralized naming function for consistency
	envDirName := language.GetRepositoryEnvironmentName("r", version)

	// Prevent creating environment directory in CWD if repoPath is empty
	var envPath string
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir are empty, cannot create R environment")
		}
		// Use cache directory when repoPath is empty
		envPath = filepath.Join(cacheDir, "r-"+envDirName)
	} else {
		envPath = filepath.Join(repoPath, envDirName)
	}

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

	// Check if runtime is available in the system
	if !r.IsRuntimeAvailable() {
		return "", fmt.Errorf("r runtime not found. Please install R to use R hooks.\n"+
			"Installation instructions: %s", r.InstallURL)
	}

	// R exists in system, just create environment directory
	if err := r.CreateEnvironmentDirectory(envPath); err != nil {
		return "", fmt.Errorf("failed to create R environment directory: %w", err)
	}

	// Install additional dependencies if specified
	if len(additionalDeps) > 0 {
		if err := r.InstallDependencies(envPath, additionalDeps); err != nil {
			return "", fmt.Errorf("failed to install R dependencies: %w", err)
		}
	}

	return envPath, nil
}

// CheckHealth performs health check for R environments matching Python pre-commit's health_check
func (r *RLanguage) CheckHealth(envPath, version string) error {
	// Python pre-commit only supports 'default' version
	if version != language.VersionDefault {
		return fmt.Errorf("r only supports version 'default', got: %s", version)
	}

	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment directory does not exist: %s", envPath)
	}

	// Check if system R is available (required by Python pre-commit)
	if _, err := exec.LookPath("Rscript"); err != nil {
		return fmt.Errorf("pre-commit requires system-installed R (Rscript executable not found)")
	}

	// Python pre-commit does sophisticated R version checking between
	// the R version used to install packages vs current R executable
	// For basic compatibility, we'll just verify R works
	cmd := exec.Command("Rscript", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("system R installation not working: %w", err)
	}

	// Environment is healthy if directory exists and system R works
	// Note: Full implementation would check R version consistency like Python pre-commit
	return nil
}
