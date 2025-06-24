package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/language"
)

const (
	pythonExecutable = "python"
)

// PygrepLanguage handles Python-based grep pattern matching
type PygrepLanguage struct {
	*language.Base
}

// NewPygrepLanguage creates a new pygrep language handler
func NewPygrepLanguage() *PygrepLanguage {
	return &PygrepLanguage{
		Base: language.NewBase(
			"pygrep",
			pythonExecutable,
			"--version",
			"https://www.python.org/",
		),
	}
}

// IsRuntimeAvailable checks if Python is available, trying both python and python3
func (p *PygrepLanguage) IsRuntimeAvailable() bool {
	// First try the configured executable name (for test compatibility)
	if p.ExecutableName != "" {
		if _, err := exec.LookPath(p.ExecutableName); err == nil {
			return true
		}

		// If the configured executable is "python", also try "python3" as fallback
		if p.ExecutableName == pythonExecutable {
			if _, err := exec.LookPath("python3"); err == nil {
				return true
			}
		}
	}

	return false
}

// SetupEnvironmentWithRepo sets up a pygrep environment for a specific repository
func (p *PygrepLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	_ []string,
) (string, error) {
	// Check if Python is available - this is required for pygrep to work
	if !p.IsRuntimeAvailable() {
		p.PrintNotFoundMessage()
		return "", fmt.Errorf("python runtime not found in PATH, cannot setup pygrep environment")
	}

	envDirName := language.GetRepositoryEnvironmentName(p.Name, version)
	if envDirName == "" {
		// Pygrep doesn't need a separate environment, use the repo path
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

	// Create environment directory
	if err := p.CreateEnvironmentDirectory(envPath); err != nil {
		return "", fmt.Errorf("failed to create pygrep environment directory: %w", err)
	}

	// Pygrep uses Python's built-in capabilities, no special setup needed
	return envPath, nil
}

// InstallDependencies does nothing for pygrep (uses Python built-ins)
func (p *PygrepLanguage) InstallDependencies(_ string, deps []string) error {
	// Pygrep language uses Python built-in modules
	if len(deps) > 0 {
		fmt.Printf(
			"[WARN] Pygrep language ignoring additional dependencies (uses Python built-ins): %v\n",
			deps,
		)
	}
	return nil
}

// CheckHealth verifies Python is available for pygrep
func (p *PygrepLanguage) CheckHealth(envPath, _ string) error {
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("pygrep environment directory does not exist: %s", envPath)
	}

	// Verify Python is available
	if !p.IsRuntimeAvailable() {
		return fmt.Errorf("python runtime not available for pygrep")
	}

	return nil
}
