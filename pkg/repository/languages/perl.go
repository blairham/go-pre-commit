package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// PerlLanguage handles Perl environment setup
type PerlLanguage struct {
	*language.Base
}

// NewPerlLanguage creates a new Perl language handler
func NewPerlLanguage() *PerlLanguage {
	return &PerlLanguage{
		Base: language.NewBase("perl", "perl", "--version", "https://www.perl.org/"),
	}
}

// GetDefaultVersion returns the default Perl version
// Following Python pre-commit behavior: returns 'system' if Perl is installed, otherwise 'default'
func (p *PerlLanguage) GetDefaultVersion() string {
	// Check if system Perl is available
	if p.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (p *PerlLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return p.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "perl")
}

// SetupEnvironmentWithRepoInfo sets up a Perl environment with repository URL information
func (p *PerlLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return p.CacheAwareSetupEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "perl")
}

// CheckHealth verifies that Perl is working correctly
func (p *PerlLanguage) CheckHealth(envPath string) error {
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("perl environment directory does not exist: %s", envPath)
	}

	// For Perl, we use the system runtime, so check if it's available
	if !p.IsRuntimeAvailable() {
		return fmt.Errorf("perl runtime not found in system PATH")
	}

	return nil
}

// SetupEnvironmentWithRepo sets up a Perl environment for a specific repository
func (p *PerlLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Only support 'default' or 'system' versions
	if version != language.VersionDefault && version != language.VersionSystem {
		version = language.VersionDefault
	}

	// Use repository-aware environment naming
	envDirName := language.GetRepositoryEnvironmentName(p.Name, version)
	if envDirName == "" {
		// Perl can work from the repository itself
		return repoPath, nil
	}

	// Handle empty repoPath by using cacheDir instead to avoid creating directories in CWD
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir cannot be empty")
		}
		repoPath = cacheDir
	}

	envPath := filepath.Join(repoPath, envDirName)

	// Check if environment already exists and is functional
	if p.CheckEnvironmentHealth(envPath) {
		return envPath, nil
	}

	// Environment exists but is broken, remove and recreate
	if _, err := os.Stat(envPath); err == nil {
		if err := os.RemoveAll(envPath); err != nil {
			return "", fmt.Errorf("failed to remove broken environment: %w", err)
		}
	}

	// Create environment directory and install state files (DRY)
	if err := p.SetupEnvironmentDirectory(envPath, additionalDeps); err != nil {
		return "", err
	}

	// Install additional dependencies if specified
	if len(additionalDeps) > 0 {
		if err := p.InstallDependencies(envPath, additionalDeps); err != nil {
			return "", fmt.Errorf("failed to install Perl dependencies: %w", err)
		}
	}

	return envPath, nil
}

// InstallDependencies installs Perl modules using cpanm or cpan
func (p *PerlLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Use cpanm if available, fallback to cpan
	var installer string
	if _, err := exec.LookPath("cpanm"); err == nil {
		installer = "cpanm"
	} else if _, err := exec.LookPath("cpan"); err == nil {
		installer = "cpan"
	} else {
		return fmt.Errorf("neither cpanm nor cpan found - please install a Perl package manager")
	}

	// Create local lib directory
	libPath := filepath.Join(envPath, "lib", "perl5")
	if err := os.MkdirAll(libPath, 0o750); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}

	// Install each dependency
	for _, dep := range deps {
		var cmd *exec.Cmd
		if installer == "cpanm" {
			cmd = exec.Command("cpanm", "--local-lib", envPath, dep)
		} else {
			// Using cpan with local::lib
			cmd = exec.Command("cpan", "-I", dep)
			cmd.Env = append(os.Environ(),
				"PERL_LOCAL_LIB_ROOT="+envPath,
				"PERL5LIB="+libPath,
			)
		}

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to install Perl module %s: %w\nOutput: %s", dep, err, output)
		}
	}

	return nil
}

// CheckEnvironmentHealth checks if the Perl environment is healthy
func (p *PerlLanguage) CheckEnvironmentHealth(envPath string) bool {
	// First check if the environment directory exists
	if _, err := os.Stat(envPath); err != nil {
		return false
	}

	// Try the base health check (looks for perl in environment bin directory)
	if err := p.CheckHealth(envPath); err != nil {
		// Environment perl not found, check if system perl is available as fallback
		if !p.IsRuntimeAvailable() {
			return false
		}
		// System perl is available, environment is considered healthy for execution
	}
	// Found perl in environment or system fallback available, continue with full check

	// Check if lib directory exists (if dependencies were installed)
	libPath := filepath.Join(envPath, "lib", "perl5")
	if _, err := os.Stat(libPath); err == nil {
		// lib directory exists, try to verify perl can find modules
		cmd := exec.Command("perl", "-I", libPath, "-e", "1")
		if err := cmd.Run(); err != nil {
			return false
		}
	}

	return true
}
