package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// GoLanguage handles Go environment setup with goenv-like functionality
type GoLanguage struct {
	*language.Base
}

// NewGoLanguage creates a new Go language handler
func NewGoLanguage() *GoLanguage {
	return &GoLanguage{
		Base: language.NewBase(
			"golang",
			"go",
			"version",
			"https://golang.org/",
		),
	}
}

// GetDefaultVersion returns the default Go version
// Following Python pre-commit behavior: returns 'system' if Go is installed, otherwise 'default'
func (g *GoLanguage) GetDefaultVersion() string {
	// Check if system Go is available
	if g.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (g *GoLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	// Use the cache-aware pre-initialization for proper cache tracking
	return g.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "go")
}

// SetupEnvironmentWithRepoInfo sets up a Go environment with repository URL information
func (g *GoLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return g.CacheAwareSetupEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "go")
}

// SetupEnvironmentWithRepo sets up a Go environment for a specific repository
func (g *GoLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return g.setupEnvironmentWithRepoInternal(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// setupEnvironmentWithRepoInternal contains the actual environment setup logic
func (g *GoLanguage) setupEnvironmentWithRepoInternal(
	cacheDir, version, repoPath, _ string,
	additionalDeps []string,
) (string, error) {
	// Determine Go version
	goVersion := g.determineGoVersion(version)

	// Create environment path
	envDirName := language.GetRepositoryEnvironmentName("go", goVersion)
	envPath := filepath.Join(cacheDir, envDirName)

	// Check if environment already exists and is functional
	if g.IsEnvironmentInstalled(envPath, repoPath) {
		return envPath, nil
	}

	// Environment exists but might be broken, remove and recreate
	if _, err := os.Stat(envPath); err == nil {
		if err := os.RemoveAll(envPath); err != nil {
			return "", fmt.Errorf("failed to remove broken Go environment: %w", err)
		}
	}

	// Check if Go runtime is available in the system
	if !g.IsRuntimeAvailable() {
		return "", fmt.Errorf("go runtime not found. Please install Go to use Go hooks.\n"+
			"Installation instructions: %s", g.InstallURL)
	}

	// Create environment directory
	if err := g.CreateEnvironmentDirectory(envPath); err != nil {
		return "", fmt.Errorf("failed to create Go environment directory: %w", err)
	}

	// Create symlinks to system Go binaries for environment consistency
	if err := g.setupSystemGoSymlinks(envPath); err != nil {
		return "", fmt.Errorf("failed to setup Go symlinks: %w", err)
	}

	// Log warning if additional dependencies are specified (not supported without package management)
	if len(additionalDeps) > 0 {
		fmt.Printf("⚠️  Warning: Go language ignoring additional dependencies "+
			"(only uses pre-installed Go runtime): %v\n", additionalDeps)
	}

	return envPath, nil
}

// determineGoVersion determines which Go version to use
func (g *GoLanguage) determineGoVersion(_ string) string {
	// For simplified implementation, always use system Go
	return language.VersionDefault
}

// InstallDependencies does nothing for Go (only uses pre-installed runtime)
func (g *GoLanguage) InstallDependencies(_ string, deps []string) error {
	// Go language uses pre-installed runtime only
	if len(deps) > 0 {
		fmt.Printf(
			"⚠️  Warning: Go language ignoring additional dependencies (only uses pre-installed Go runtime): %v\n",
			deps,
		)
	}
	return nil
}

// isRepositoryInstalled checks if the repository is properly set up in the environment
func (g *GoLanguage) isRepositoryInstalled(envPath, _ string) bool {
	// For simplified implementation, just check if environment directory exists
	_, err := os.Stat(envPath)
	return err == nil
}

// SetupEnvironmentWithRepositoryInit handles Go environment setup assuming repository is already initialized
//
//nolint:revive // function name is part of interface contract
func (g *GoLanguage) SetupEnvironmentWithRepositoryInit(
	cacheDir, version, repoPath string,
	additionalDeps []string,
	repoURLAny any,
) (string, error) {
	// Convert repoURLAny to string if it's not nil
	repoURL := ""
	if repoURLAny != nil {
		if url, ok := repoURLAny.(string); ok {
			repoURL = url
		}
	}

	return g.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// IsEnvironmentInstalled checks if the Go environment is properly installed and functional
func (g *GoLanguage) IsEnvironmentInstalled(envPath, repoPath string) bool {
	return g.isRepositoryInstalled(envPath, repoPath)
}

// CacheAwareSetupEnvironmentWithRepoInfo provides cache-aware environment setup for Go
//
//nolint:revive // function name is part of interface contract
func (g *GoLanguage) CacheAwareSetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
	_ string, // language name parameter (unused)
) (string, error) {
	return g.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// setupSystemGoSymlinks creates symlinks to system Go binaries in the environment
func (g *GoLanguage) setupSystemGoSymlinks(envPath string) error {
	binDir := filepath.Join(envPath, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Find system Go executable
	goExecPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("system Go executable not found: %w", err)
	}

	// Create symlink for go executable
	goSymlink := filepath.Join(binDir, "go")
	if err := os.Symlink(goExecPath, goSymlink); err != nil {
		return fmt.Errorf("failed to create go symlink: %w", err)
	}

	// Find and symlink gofmt if available
	if gofmtPath, err := exec.LookPath("gofmt"); err == nil {
		gofmtSymlink := filepath.Join(binDir, "gofmt")
		if err := os.Symlink(gofmtPath, gofmtSymlink); err != nil {
			// Non-fatal error - gofmt is optional
			fmt.Printf("⚠️  Warning: Failed to create gofmt symlink: %v\n", err)
		}
	}

	fmt.Printf("Info: Created symlinks to system Go: %s -> %s\n", goSymlink, goExecPath)
	return nil
}

// CheckHealth checks the health of a Go environment
func (g *GoLanguage) CheckHealth(envPath, _ string) error {
	binPath := filepath.Join(envPath, "bin")
	goExecPath := filepath.Join(binPath, "go")

	// Check if go executable exists in environment
	if _, err := os.Stat(goExecPath); err != nil {
		// If symlink doesn't exist, try to create it
		if err := g.setupSystemGoSymlinks(envPath); err != nil {
			return fmt.Errorf("failed to setup Go symlinks during health check: %w", err)
		}
	}

	// Test if Go is functional
	cmd := exec.Command(goExecPath, "version")
	// Set up proper environment for Go
	cmd.Env = append(os.Environ(), g.getGoEnvironment(envPath)...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go runtime health check failed: %w", err)
	}

	return nil
}

// getGoEnvironment returns environment variables needed for Go execution
func (g *GoLanguage) getGoEnvironment(envPath string) []string {
	var env []string

	// Set GOCACHE to prevent cache errors
	goCacheDir := filepath.Join(envPath, "gocache")
	if err := os.MkdirAll(goCacheDir, 0o750); err == nil {
		env = append(env, fmt.Sprintf("GOCACHE=%s", goCacheDir))
	}

	// Set GOPATH if needed (optional for Go modules)
	goPath := filepath.Join(envPath, "gopath")
	if err := os.MkdirAll(goPath, 0o750); err == nil {
		env = append(env, fmt.Sprintf("GOPATH=%s", goPath))
	}

	return env
}
