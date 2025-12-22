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

// readInstalledRVersion reads the R version that was used when the environment was installed
// This matches Python pre-commit's _read_installed_version()
func (r *RLanguage) readInstalledRVersion(envPath string) (string, error) {
	// Try to get the installed version from renv settings
	// This requires executing R code in the renv environment
	code := `cat(renv::settings$r.version())`
	output, err := r.executeRInEnv(code, envPath)
	if err != nil {
		// Fallback: try to read from a version file we may have created
		versionFile := filepath.Join(envPath, ".r_version")
		content, fileErr := os.ReadFile(versionFile)
		if fileErr != nil {
			return "", fmt.Errorf("could not read installed R version: %w", err)
		}
		return strings.TrimSpace(string(content)), nil
	}
	return strings.TrimSpace(output), nil
}

// readExecutableRVersion gets the current R version from the executable
// This matches Python pre-commit's _read_executable_version()
func (r *RLanguage) readExecutableRVersion(envPath string) (string, error) {
	code := `cat(as.character(getRversion()))`
	output, err := r.executeRInEnv(code, envPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// executeRInEnv executes R code in the renv environment
func (r *RLanguage) executeRInEnv(code, envPath string) (string, error) {
	// Run R with renv activated
	renvActivate := filepath.Join(envPath, "renv", "activate.R")
	fullCode := fmt.Sprintf("source('%s'); %s", renvActivate, code)

	cmd := exec.Command("Rscript", "--vanilla", "-e", fullCode)
	cmd.Dir = envPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// writeCurrentRVersion writes the current R version to a file in the environment
// This is called during environment setup
func (r *RLanguage) writeCurrentRVersion(envPath string) error {
	// Get current R version
	cmd := exec.Command("Rscript", "-e", "cat(as.character(getRversion()))")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get R version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	versionFile := filepath.Join(envPath, ".r_version")
	return os.WriteFile(versionFile, []byte(version), 0o644)
}

// CheckHealth performs health check for R environments matching Python pre-commit's health_check
// This checks if the installed R version matches the current executable R version
func (r *RLanguage) CheckHealth(envPath, version string) error {
	// Python pre-commit only supports 'default' version
	if version != language.VersionDefault && version != "" {
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

	// Try to read the R version that was installed
	installedVersion, err := r.readInstalledRVersion(envPath)
	if err != nil {
		// If we can't read the installed version, just verify R works
		cmd := exec.Command("Rscript", "--version")
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("system R installation not working: %w", runErr)
		}
		// Environment is healthy if R works but we don't have version info
		return nil
	}

	// Get the current R version
	currentVersion, err := r.readExecutableRVersion(envPath)
	if err != nil {
		// If we can't get current version, fallback to basic check
		cmd := exec.Command("Rscript", "-e", "cat(as.character(getRversion()))")
		output, runErr := cmd.Output()
		if runErr != nil {
			return fmt.Errorf("failed to get current R version: %w", runErr)
		}
		currentVersion = strings.TrimSpace(string(output))
	}

	// Compare versions - if they don't match, environment is unhealthy
	if installedVersion != "" && currentVersion != "" && installedVersion != currentVersion {
		return fmt.Errorf(
			"Hooks were installed for R version %s but current R is version %s. "+
				"Re-install the hooks.",
			installedVersion, currentVersion,
		)
	}

	return nil
}
