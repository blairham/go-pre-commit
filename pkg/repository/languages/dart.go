package languages

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// DartLanguage handles Dart environment setup
type DartLanguage struct {
	*language.Base
}

// NewDartLanguage creates a new Dart language handler
func NewDartLanguage() *DartLanguage {
	return &DartLanguage{
		Base: language.NewBase("dart", "dart", "--version", "https://dart.dev/get-dart"),
	}
}

// GetDefaultVersion returns the default Dart version
// Following Python pre-commit behavior: returns 'system' if Dart is installed, otherwise 'default'
func (d *DartLanguage) GetDefaultVersion() string {
	// Check if system Dart is available
	if d.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// SetupEnvironmentWithRepo sets up a Dart environment for a specific repository
func (d *DartLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Only support 'default' or 'system' versions
	if version == "" {
		version = d.GetDefaultVersion()
	}
	if version != language.VersionDefault && version != language.VersionSystem {
		version = language.VersionDefault
	}

	// Use the centralized naming function for consistency
	envDirName := language.GetRepositoryEnvironmentName("dart", version)

	// Prevent creating environment directory in CWD if repoPath is empty
	var envPath string
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir are empty, cannot create Dart environment")
		}
		// Use cache directory when repoPath is empty
		envPath = filepath.Join(cacheDir, "dart-"+envDirName)
	} else {
		envPath = filepath.Join(repoPath, envDirName)
	}

	// Check if environment already exists
	if _, err := os.Stat(envPath); err == nil {
		// Environment exists, verify it's functional
		if err := d.CheckHealth(envPath, ""); err == nil {
			return envPath, nil
		}
		// Environment exists but is broken, remove and recreate
		if err := os.RemoveAll(envPath); err != nil {
			return "", fmt.Errorf("failed to remove broken environment: %w", err)
		}
	}

	// Check if runtime is available in the system
	if !d.IsRuntimeAvailable() {
		return "", fmt.Errorf("dart runtime not found. Please install Dart to use Dart hooks.\n"+
			"Installation instructions: %s", d.InstallURL)
	}

	// Dart exists in system, just create environment directory
	if err := d.CreateEnvironmentDirectory(envPath); err != nil {
		return "", fmt.Errorf("failed to create Dart environment directory: %w", err)
	}

	// Log warning if additional dependencies are specified (not supported without package management)
	if len(additionalDeps) > 0 {
		fmt.Printf("[WARN] Dart language ignoring additional dependencies "+
			"(only uses pre-installed Dart runtime): %v\n", additionalDeps)
	}

	return envPath, nil
}

// InstallDependencies does nothing for Dart (only uses pre-installed runtime)
func (d *DartLanguage) InstallDependencies(_ string, deps []string) error {
	// Dart language uses pre-installed runtime only
	if len(deps) > 0 {
		fmt.Printf(
			"[WARN] Dart language ignoring additional dependencies (only uses pre-installed Dart runtime): %v\n",
			deps,
		)
	}
	return nil
}

// CheckEnvironmentHealth checks if the Dart environment is healthy
func (d *DartLanguage) CheckEnvironmentHealth(envPath string) bool {
	// Check base health first
	if err := d.CheckHealth(envPath, ""); err != nil {
		return false
	}

	// For simplified Dart, we only check if the environment directory exists
	// and Dart runtime is available (no package dependency checks)
	return d.IsRuntimeAvailable()
}
