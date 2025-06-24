package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// HaskellLanguage handles Haskell environment setup
type HaskellLanguage struct {
	*language.Base
}

// NewHaskellLanguage creates a new Haskell language handler
func NewHaskellLanguage() *HaskellLanguage {
	return &HaskellLanguage{
		Base: language.NewBase(
			"Haskell",
			"ghc",
			"--version",
			"https://www.haskell.org/downloads/",
		),
	}
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (h *HaskellLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return h.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "haskell")
}

// SetupEnvironmentWithRepoInfo sets up a Haskell environment with repository URL information
func (h *HaskellLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return h.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// InstallDependencies installs Haskell packages (matches original pre-commit logic)
func (h *HaskellLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// In the original pre-commit, it expects either .cabal files in the repo or additional_dependencies
	// Let's mimic this behavior
	binDir := filepath.Join(envPath, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Update cabal package list first
	cmd := exec.Command("cabal", "update")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update cabal package list: %w\nOutput: %s", err, output)
	}

	// Install packages to the environment bin directory
	args := []string{
		"install",
		"--install-method", "copy",
		"--installdir", binDir,
	}
	args = append(args, deps...)

	cmd = exec.Command("cabal", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"failed to install Haskell dependencies with cabal: %w\nOutput: %s",
			err,
			output,
		)
	}

	return nil
}

// CheckEnvironmentHealth checks if the Haskell environment is healthy (simplified like original)
func (h *HaskellLanguage) CheckEnvironmentHealth(envPath string) bool {
	// Check base health first
	if err := h.CheckHealth(envPath, ""); err != nil {
		return false
	}

	// Simple check: can we run cabal?
	cmd := exec.Command("cabal", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// SetupEnvironmentWithRepo sets up a Haskell environment for a specific repository
func (h *HaskellLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Assert version is default (like original pre-commit)
	if version != "" && version != language.VersionDefault && version != language.VersionSystem {
		return "", fmt.Errorf(
			"haskell language only supports 'default' or 'system' versions, got: %s",
			version,
		)
	}

	// Handle empty repoPath by using cacheDir instead to avoid creating directories in CWD
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir cannot be empty")
		}
		repoPath = cacheDir
	}

	// Use the centralized naming function for consistency
	envDirName := language.GetRepositoryEnvironmentName("haskell", version)
	envPath := filepath.Join(repoPath, envDirName)

	// Check if environment already exists and is functional
	if h.CheckEnvironmentHealth(envPath) {
		return envPath, nil
	}

	// Environment exists but is broken, remove and recreate
	if _, err := os.Stat(envPath); err == nil {
		if err := os.RemoveAll(envPath); err != nil {
			return "", fmt.Errorf("failed to remove broken environment: %w", err)
		}
	}

	// Create environment directory
	if err := h.CreateEnvironmentDirectory(envPath); err != nil {
		return "", fmt.Errorf("failed to create Haskell environment directory: %w", err)
	}

	// Install dependencies if specified
	if err := h.InstallDependencies(envPath, additionalDeps); err != nil {
		return "", fmt.Errorf("failed to install Haskell dependencies: %w", err)
	}

	return envPath, nil
}
