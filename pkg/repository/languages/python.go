package languages

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/language"
)

const (
	// DefaultPythonVersion is the default Python version when version determination fails
	DefaultPythonVersion = "python3"
	// LatestVersionString represents the latest version string
	LatestVersionString = "latest"
)

// PythonLanguage handles Python environment setup with support for both venv and conda
type PythonLanguage struct {
	*language.Base
	UseCondaByDefault bool
}

// createPythonStateFiles creates the .install_state_v1 and .install_state_v2 files
// that Python pre-commit uses to detect if an environment is already installed
func (p *PythonLanguage) createPythonStateFiles(envPath string, additionalDeps []string) error {
	// Create .install_state_v1 with JSON containing additional dependencies
	state := map[string][]string{
		"additional_dependencies": additionalDeps,
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state JSON: %w", err)
	}

	// Write .install_state_v1 atomically (like Python pre-commit does)
	stateFileV1 := filepath.Join(envPath, ".install_state_v1")
	stagingFile := stateFileV1 + "staging"

	if err := os.WriteFile(stagingFile, stateJSON, 0o600); err != nil {
		return fmt.Errorf("failed to write staging state file: %w", err)
	}

	if err := os.Rename(stagingFile, stateFileV1); err != nil {
		return fmt.Errorf("failed to move state file into place: %w", err)
	}

	// Create .install_state_v2 as an empty file (just needs to exist)
	stateFileV2 := filepath.Join(envPath, ".install_state_v2")
	if err := os.WriteFile(stateFileV2, []byte{}, 0o600); err != nil {
		return fmt.Errorf("failed to create state file v2: %w", err)
	}

	return nil
}

// NewPythonLanguage creates a new Python language handler
func NewPythonLanguage() *PythonLanguage {
	return &PythonLanguage{
		Base: language.NewBase(
			"Python",
			"python",
			"--version",
			"https://www.python.org/",
		),
		UseCondaByDefault: false,
	}
}

// NewPythonLanguageWithCache creates a new Python language handler with a specific cache directory
func NewPythonLanguageWithCache(_ string) *PythonLanguage {
	return &PythonLanguage{
		Base: language.NewBase(
			"Python",
			"python",
			"--version",
			"https://www.python.org/",
		),
		UseCondaByDefault: false,
	}
}

// GetDefaultVersion returns the default Python version
// Following Python pre-commit behavior: checks for system Python installation
func (p *PythonLanguage) GetDefaultVersion() string {
	// Check if system Python is available
	systemExecutables := []string{"python3", "python", "python3.12", "python3.11", "python3.10", "python3.9"}
	for _, exe := range systemExecutables {
		if _, err := exec.LookPath(exe); err == nil {
			// System Python is available, but for Python we typically want to create
			// isolated environments, so we return default to match Python pre-commit behavior
			return language.VersionDefault
		}
	}

	// No system Python found, return default (will require installation)
	return language.VersionDefault
}

// IsRuntimeAvailable checks if Python runtime is available
// This checks for system Python installations
func (p *PythonLanguage) IsRuntimeAvailable() bool {
	// Check system Python first (python3, python, python3.x)
	systemExecutables := []string{"python3", "python", "python3.12", "python3.11", "python3.10", "python3.9"}
	for _, exe := range systemExecutables {
		if _, err := exec.LookPath(exe); err == nil {
			return true
		}
	}

	// Python can be installed on-demand during environment setup
	return true
}

// InstallDependencies installs Python dependencies in the virtual environment
func (p *PythonLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Determine if this is a conda environment
	isCondaEnv := p.isCondaEnvironment(envPath)

	if isCondaEnv {
		return p.installCondaDependencies(envPath, deps)
	}
	return p.installPipDependencies(envPath, deps)
}

// CheckHealth checks if Python and pip are available and optionally installs into the environment
func (p *PythonLanguage) CheckHealth(_ string) error {
	// Python is always considered healthy
	// Additional health checks can be added here if needed
	return nil
}

// testSingleExecutable tests if a single Python executable exists and works
func (p *PythonLanguage) testSingleExecutable(execPath string) error {
	// Check if executable exists
	if _, err := os.Stat(execPath); err != nil {
		return err
	}

	// Try to run the executable with version flag
	cmd := exec.Command(execPath, p.VersionFlag)
	return cmd.Run()
}

// isCondaEnvironment checks if the given path is a conda environment
func (p *PythonLanguage) isCondaEnvironment(envPath string) bool {
	// Check if conda-meta directory exists (conda environments have this)
	condaMetaPath := filepath.Join(envPath, "conda-meta")
	_, err := os.Stat(condaMetaPath)
	return err == nil
}

// SetupEnvironmentWithRepo sets up a Python virtual environment and installs the repository
// This method delegates to SetupEnvironmentWithRepoInfo for consistency
func (p *PythonLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return p.SetupEnvironmentWithRepoInfo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// Implement EnvironmentInstaller interface for Python

// CreateLanguageEnvironment creates a Python virtual environment and installs the repository
func (p *PythonLanguage) CreateLanguageEnvironment(envPath string) error {
	// This method doesn't have access to the Python version, so we need to determine it
	// from the environment path or fall back to a default version

	// Extract version from environment path (e.g., "py_env-python3.11" -> "3.11")
	version := p.extractVersionFromEnvPath(envPath)
	if version == "" {
		version = "3.11" // Default fallback version
	}

	return p.CreateLanguageEnvironmentWithVersion(envPath, version)
}

// CreateLanguageEnvironmentWithVersion creates a Python virtual environment using a specific version
func (p *PythonLanguage) CreateLanguageEnvironmentWithVersion(envPath, _ string) error {
	pythonExe, _, err := p.setupPythonExecutable()
	if err != nil {
		return err
	}

	// Create environment directory and install state files (DRY)
	if err := p.SetupEnvironmentDirectory(envPath, nil); err != nil {
		return err
	}

	cmd := exec.Command(pythonExe, "-m", "venv", envPath)
	p.configurePythonEnvironment(cmd)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create Python virtual environment: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// setupPythonExecutable determines and validates the Python executable to use
func (p *PythonLanguage) setupPythonExecutable() (pythonExe string, isSystemPython bool, err error) {
	return p.setupSystemPython()
}

// setupSystemPython sets up system Python as fallback
func (p *PythonLanguage) setupSystemPython() (string, bool, error) {
	pythonExe, err := exec.LookPath("python3")
	if err != nil {
		pythonExe, err = exec.LookPath("python")
		if err != nil {
			return "", true, fmt.Errorf("failed to ensure Python runtime: no Python executable found")
		}
	}

	return pythonExe, true, nil
}

// configurePythonEnvironment sets up environment variables for the Python command
func (p *PythonLanguage) configurePythonEnvironment(cmd *exec.Cmd) {
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")
}

// extractVersionFromEnvPath extracts the Python version from an environment path
// e.g., "/path/to/repo/py_env-python3.11" -> "3.11"
func (p *PythonLanguage) extractVersionFromEnvPath(envPath string) string {
	baseName := filepath.Base(envPath)
	if after, ok := strings.CutPrefix(baseName, "py_env-python"); ok {
		version := after
		// Return any non-empty version
		if version != "" {
			return version
		}
	}
	return ""
}

// IsEnvironmentInstalled checks if a Python environment is properly installed
func (p *PythonLanguage) IsEnvironmentInstalled(envPath, _ string) bool {
	// Check if python executable exists
	if _, err := os.Stat(filepath.Join(envPath, "bin", "python")); err != nil {
		return false
	}

	// Check if repository is installed using our existing method
	return p.isRepositoryInstalled(envPath)
}

// GetEnvironmentVersion determines the Python version for environment naming
func (p *PythonLanguage) GetEnvironmentVersion(version string) (string, error) {
	// For environment naming, preserve special version names for cache consistency
	if version == "" {
		return language.VersionDefault, nil
	}

	if version == language.VersionDefault || version == language.VersionSystem {
		return version, nil
	}

	// For specific versions, return them as-is (don't add "python" prefix for environment naming)
	// Strip "python" prefix if it exists to normalize the version
	if after, ok := strings.CutPrefix(version, "python"); ok {
		return after, nil
	}
	return version, nil
}

// GetEnvironmentPath returns the path where the Python environment should be created
func (p *PythonLanguage) GetEnvironmentPath(repoPath, version string) string {
	// Use the environment version (preserves "default" for cache compatibility)
	envVersion, _ := p.GetEnvironmentVersion( //nolint:errcheck // This function currently cannot return an error
		version,
	)

	envDirName := language.GetRepositoryEnvironmentName("python", envVersion)
	return filepath.Join(repoPath, envDirName)
}

// Refactored methods using the generic base functionality

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
// This is called in the first phase to show all "Initializing" messages before any installations
func (p *PythonLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	_ []string,
) error {
	// Handle empty repoPath by using cacheDir instead to avoid creating directories in CWD
	if repoPath == "" {
		if cacheDir == "" {
			return fmt.Errorf("both repoPath and cacheDir cannot be empty")
		}
		repoPath = cacheDir
	}

	// Determine the environment version for naming (preserves "default" for cache compatibility)
	envVersion, _ := p.GetEnvironmentVersion( //nolint:errcheck // This function currently cannot return an error
		version,
	)

	envDirName := language.GetRepositoryEnvironmentName("python", envVersion)
	envPath := filepath.Join(repoPath, envDirName)

	// Check if environment already exists and has the repository installed
	if _, err := os.Stat(filepath.Join(envPath, "bin", "python")); err == nil {
		// Environment exists, check if repository is properly installed
		if p.isRepositoryInstalled(envPath) {
			return nil // Already fully set up
		}
		// Environment exists but repo not installed, continue with installation
	}

	// Create environment directory (but don't install anything yet)
	if err := os.MkdirAll(envPath, 0o750); err != nil {
		return fmt.Errorf("failed to create Python environment directory: %w", err)
	}

	return nil
}

// ensureRuntimeAvailable ensures the Python runtime is available
func (p *PythonLanguage) ensureRuntimeAvailable() error {
	if p.Base != nil && !p.IsRuntimeAvailable() {
		return fmt.Errorf("python runtime not found. Please install Python manually: %s", p.InstallURL)
	}
	return nil
}

// prepareEnvironmentPath prepares and returns the environment path
func (p *PythonLanguage) prepareEnvironmentPath(repoPath, version string) string {
	// Use environment version for cache compatibility (preserves "default")
	envVersion, _ := p.GetEnvironmentVersion( //nolint:errcheck // This function currently cannot return an error
		version,
	)

	envDirName := language.GetRepositoryEnvironmentName("python", envVersion)
	return filepath.Join(repoPath, envDirName)
}

// showInstallationInfo shows installation progress messages
func (p *PythonLanguage) showInstallationInfo(repoURL string) {
	if repoURL != "" {
		fmt.Printf("[INFO] Installing environment for %s.\n", repoURL)
		fmt.Printf("[INFO] Once installed this environment will be reused.\n")
		fmt.Printf("[INFO] This may take a few minutes...\n")
	}
}

// installRepository installs the repository in the Python environment
func (p *PythonLanguage) installRepository(envPath, repoPath string) error {
	pythonPath := filepath.Join(envPath, "bin", "python")

	// Match Python pre-commit's exact pip install command
	// Python pre-commit uses: python -m pip install --quiet --no-compile --no-warn-script-location .
	cmd := exec.Command(
		pythonPath,
		"-m",
		"pip",
		"install",
		"--quiet",
		"--no-compile",
		"--no-warn-script-location",
		".",
	)
	cmd.Dir = repoPath // Set working directory to repo path like Python pre-commit

	// Set environment variables to match Python pre-commit's behavior exactly
	cmd.Env = append(os.Environ(),
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
		"PIP_NO_WARN_SCRIPT_LOCATION=1",
		"VIRTUAL_ENV="+envPath,
		"PATH="+filepath.Join(envPath, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install repository %s: %w", repoPath, err)
	}
	return nil
}

// SetupEnvironmentWithRepoInfo sets up a Python virtual environment with repository URL information
func (p *PythonLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	// Create environment inside the repository directory like Python pre-commit does
	// Format: py_env-pythonX.Y (matches Python pre-commit's ENVIRONMENT_DIR pattern)

	// Handle empty repoPath by using cacheDir instead to avoid creating directories in CWD
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir cannot be empty")
		}
		repoPath = cacheDir
	}

	// Check if Python runtime is available
	if err := p.ensureRuntimeAvailable(); err != nil {
		return "", err
	}

	// Prepare environment path
	envPath := p.prepareEnvironmentPath(repoPath, version)

	// Check if environment already exists and has the repository installed
	if _, err := os.Stat(filepath.Join(envPath, "bin", "python")); err == nil {
		// Environment exists, check if repository is properly installed
		if p.isRepositoryInstalled(envPath) {
			// Also verify additional dependencies match
			if p.areAdditionalDependenciesInstalled(envPath, additionalDeps) {
				return envPath, nil
			}
		}
		// Environment exists but repo not installed or deps changed, continue with installation
	}

	// Create environment directory (initialization message should have been shown in pre-init phase)
	if err := os.MkdirAll(envPath, 0o750); err != nil {
		return "", fmt.Errorf("failed to create Python environment directory: %w", err)
	}

	// Show installation progress
	p.showInstallationInfo(repoURL)

	// Create virtual environment
	if err := p.createVirtualEnvironment(envPath); err != nil {
		return "", err
	}

	// Install the repository itself
	if err := p.installRepository(envPath, repoPath); err != nil {
		return "", err
	}

	// Install additional dependencies
	if len(additionalDeps) > 0 {
		if err := p.InstallDependencies(envPath, additionalDeps); err != nil {
			return "", err
		}
	}

	// Create state files to match Python pre-commit's environment detection
	if err := p.createPythonStateFiles(envPath, additionalDeps); err != nil {
		return "", fmt.Errorf("failed to create state files: %w", err)
	}

	return envPath, nil
}

// SetupEnvironmentWithRepositoryInit handles Python environment setup assuming repository is already initialized
// This method delegates to SetupEnvironmentWithRepoInfo for consistency
func (p *PythonLanguage) SetupEnvironmentWithRepositoryInit(
	cacheDir, version, repoPath, repoURL, _ string, // repoRef is unused
	additionalDeps []string,
	_ any, // repositoryManager is unused
) (string, error) {
	return p.SetupEnvironmentWithRepoInfo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// NeedsEnvironmentSetup returns true since Python requires environment setup
func (p *PythonLanguage) NeedsEnvironmentSetup() bool {
	return true
}

// isRepositoryInstalled checks if a repository is already installed in the Python environment
func (p *PythonLanguage) isRepositoryInstalled(envPath string) bool {
	// First check if state files exist (matching Python pre-commit's logic)
	stateFileV2 := filepath.Join(envPath, ".install_state_v2")
	stateFileV1 := filepath.Join(envPath, ".install_state_v1")

	// Python pre-commit checks for v2 first, then v1
	if _, err := os.Stat(stateFileV2); err == nil {
		// v2 exists, environment is installed
		return true
	}

	if _, err := os.Stat(stateFileV1); err == nil {
		// v1 exists, environment is installed
		return true
	}

	// Fall back to checking pip packages if state files don't exist
	// Check if pip packages are installed by running pip list in the environment
	pipPath := filepath.Join(envPath, "bin", "pip")

	// Check if pip exists
	if _, err := os.Stat(pipPath); err != nil {
		return false
	}

	// Run pip list to see if any packages are installed (indicating the repo was set up)
	cmd := exec.Command(pipPath, "list", "--format=freeze")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If there are packages installed (more than just pip and basic ones),
	// the repository has been set up
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Check for any non-standard packages that indicate repo installation
			// Standard packages in a fresh venv are: pip, setuptools, wheel
			// Any additional packages indicate the repository has been installed
			if !strings.HasPrefix(line, "pip==") &&
				!strings.HasPrefix(line, "setuptools==") &&
				!strings.HasPrefix(line, "wheel==") {
				return true // Found a non-standard package, repo is installed
			}
		}
	}

	// No additional packages found, repository is not installed
	return false
}

// areAdditionalDependenciesInstalled checks if the expected additional dependencies are installed
func (p *PythonLanguage) areAdditionalDependenciesInstalled(envPath string, expectedDeps []string) bool {
	// Read the state file to see what dependencies were installed
	stateFileV1 := filepath.Join(envPath, ".install_state_v1")
	if _, err := os.Stat(stateFileV1); err != nil {
		// State file doesn't exist, assume dependencies don't match
		return false
	}

	// Read the state file
	// #nosec G304 -- stateFileV1 is constructed from envPath which is controlled
	stateData, err := os.ReadFile(stateFileV1)
	if err != nil {
		return false
	}

	// Parse the JSON state
	var state map[string][]string
	if err := json.Unmarshal(stateData, &state); err != nil {
		return false
	}

	installedDeps, exists := state["additional_dependencies"]
	if !exists {
		installedDeps = []string{}
	}

	// Compare expected vs installed dependencies
	if len(expectedDeps) != len(installedDeps) {
		return false
	}

	// Create maps for easy comparison
	expectedMap := make(map[string]bool)
	for _, dep := range expectedDeps {
		expectedMap[dep] = true
	}

	installedMap := make(map[string]bool)
	for _, dep := range installedDeps {
		installedMap[dep] = true
	}

	// Check if all expected dependencies are installed
	for dep := range expectedMap {
		if !installedMap[dep] {
			return false
		}
	}

	// Check if any extra dependencies are installed
	for dep := range installedMap {
		if !expectedMap[dep] {
			return false
		}
	}

	return true
}

// installCondaDependencies installs dependencies using conda
func (p *PythonLanguage) installCondaDependencies(envPath string, deps []string) error {
	for _, dep := range deps {
		cmd := exec.Command("conda", "install", "-p", envPath, dep, "-y")
		if err := cmd.Run(); err != nil {
			// Fall back to pip if conda install fails
			cmd = exec.Command("conda", "run", "-p", envPath, "pip", "install", dep)
			cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to install Python dependency %s with conda: %w", dep, err)
			}
		}
	}
	return nil
}

// installPipDependencies installs dependencies using pip
func (p *PythonLanguage) installPipDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	pythonPath := filepath.Join(envPath, "bin", "python")

	// Install all dependencies in one command like Python pre-commit does
	// This is more efficient and matches the exact behavior
	args := []string{
		"-m",
		"pip",
		"install",
		"--quiet",
		"--no-compile",
		"--no-warn-script-location",
	}
	args = append(args, deps...)

	cmd := exec.Command(pythonPath, args...)
	cmd.Env = append(os.Environ(),
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
		"PIP_NO_WARN_SCRIPT_LOCATION=1",
		"VIRTUAL_ENV="+envPath,
		"PATH="+filepath.Join(envPath, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Python dependencies [%s]: %w",
			strings.Join(deps, ", "), err)
	}

	return nil
}

// createVirtualEnvironment creates a Python virtual environment using virtualenv or venv
func (p *PythonLanguage) createVirtualEnvironment(envPath string) error {
	// Ensure Python is available, using pyenv to install if necessary
	pythonExe, err := p.EnsurePythonRuntime(LatestVersionString)
	if err != nil {
		return fmt.Errorf("failed to ensure Python runtime: %w", err)
	}

	// Ensure virtualenv is available (Python pre-commit prefers virtualenv over venv)
	if err := p.ensureVirtualenv(pythonExe); err != nil {
		if os.Getenv("DEBUG") != "" || os.Getenv("VERBOSE") != "" {
			fmt.Printf("⚠️  Warning: failed to ensure virtualenv: %v\n", err)
		}
		// Continue with fallback to venv
	}

	// Create virtual environment using virtualenv (faster than venv, matches Python pre-commit exactly)
	// Python pre-commit uses: python -m virtualenv --quiet --no-download env_path
	cmd := exec.Command(
		pythonExe,
		"-m",
		"virtualenv",
		"--quiet",
		"--no-download",
		envPath,
	)
	cmd.Env = append(os.Environ(),
		"PIP_DISABLE_PIP_VERSION_CHECK=1",
		"PIP_NO_WARN_SCRIPT_LOCATION=1",
	)

	if err := cmd.Run(); err != nil {
		// Fall back to venv if virtualenv is not available (like Python pre-commit does)
		if os.Getenv("DEBUG") != "" || os.Getenv("VERBOSE") != "" {
			fmt.Printf("virtualenv failed, falling back to venv: %v\n", err)
		}
		cmd = exec.Command(pythonExe, "-m", "venv", envPath)
		cmd.Env = append(os.Environ(),
			"PIP_DISABLE_PIP_VERSION_CHECK=1",
			"PIP_NO_WARN_SCRIPT_LOCATION=1",
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to create Python virtual environment with both virtualenv and venv: %w", err)
		}
	}
	return nil
}

// ensureVirtualenv ensures that virtualenv is available for the Python executable
func (p *PythonLanguage) ensureVirtualenv(pythonExe string) error {
	// Check if virtualenv is available
	cmd := exec.Command(pythonExe, "-m", "virtualenv", "--version")
	if cmd.Run() == nil {
		return nil // virtualenv is already available
	}

	// Try to install virtualenv
	if os.Getenv("DEBUG") != "" || os.Getenv("VERBOSE") != "" {
		fmt.Println("Installing virtualenv...")
	}
	cmd = exec.Command(pythonExe, "-m", "pip", "install", "virtualenv")
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install virtualenv: %w", err)
	}

	// Verify installation
	cmd = exec.Command(pythonExe, "-m", "virtualenv", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("virtualenv installation failed verification: %w", err)
	}

	// Only show installation success message if debug/verbose mode is enabled
	if os.Getenv("DEBUG") != "" || os.Getenv("VERBOSE") != "" {
		fmt.Println("Successfully installed virtualenv")
	}
	return nil
}

// checkSystemPythonSatisfiesVersion checks if system Python satisfies version requirements
func (p *PythonLanguage) checkSystemPythonSatisfiesVersion(version string) (string, bool) {
	// Base or Runtime not available
	if p.Base == nil || !p.IsRuntimeAvailable() {
		return "", false
	}

	// Try to find system Python
	systemPython, err := exec.LookPath("python3")
	if err != nil {
		systemPython, err = exec.LookPath("python")
		if err != nil {
			return "", false
		}
	}

	// Get version using command
	cmd := exec.Command(systemPython, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", false
	}

	versionStr := strings.TrimSpace(string(output))
	versionStr = strings.TrimPrefix(versionStr, "Python ")

	// Check if version is acceptable
	if p.isVersionAcceptable(versionStr, version) {
		return systemPython, true
	}

	return "", false
}

// EnsurePythonRuntime ensures Python runtime is available
func (p *PythonLanguage) EnsurePythonRuntime(version string) (string, error) {
	// Handle version specification
	if version == "" || version == language.VersionDefault {
		version = LatestVersionString
	}

	// Check if system Python is available and satisfies requirements
	if systemPython, ok := p.checkSystemPythonSatisfiesVersion(version); ok {
		return systemPython, nil
	}

	return "", fmt.Errorf("python runtime not found. Please install Python manually")
}

// EnsurePythonRuntimeInRepo ensures Python runtime is available in the repository directory
// This installs Python directly in the repo (e.g., py_env-default) instead of using a global cache
func (p *PythonLanguage) EnsurePythonRuntimeInRepo(envPath, version string) (string, error) {
	// Handle version specification
	if version == "" || version == language.VersionDefault {
		version = LatestVersionString
	}

	// Check system Python first
	if python, ok := p.checkSystemPython(version); ok {
		return python, nil
	}

	// Check if Python is already installed in the repo directory
	if python, ok := p.checkExistingRepoPython(envPath); ok {
		return python, nil
	}

	// Install Python to the repository directory
	return p.installPythonToRepo()
}

// checkSystemPython checks if system Python is available and acceptable for the given version
func (p *PythonLanguage) checkSystemPython(version string) (string, bool) {
	if p.Base == nil || !p.IsRuntimeAvailable() {
		return "", false
	}

	// Try to find system Python
	systemPython, err := exec.LookPath("python3")
	if err != nil {
		systemPython, err = exec.LookPath("python")
		if err != nil {
			return "", false
		}
	}

	// Get version using command
	cmd := exec.Command(systemPython, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", false
	}

	versionStr := strings.TrimSpace(string(output))
	versionStr = strings.TrimPrefix(versionStr, "Python ")

	if p.isVersionAcceptable(versionStr, version) {
		return systemPython, true
	}

	return "", false
}

// checkExistingRepoPython checks if Python is already installed in the repository directory
func (p *PythonLanguage) checkExistingRepoPython(envPath string) (string, bool) {
	repoDir := filepath.Dir(envPath)

	// Use the same environment naming as the main environment setup
	version := "default"
	envVersion, _ := p.GetEnvironmentVersion(version) //nolint:errcheck // Default version, error can be ignored
	envDirName := language.GetRepositoryEnvironmentName("python", envVersion)
	pythonInstallDir := filepath.Join(repoDir, envDirName)
	pythonExe := filepath.Join(pythonInstallDir, "bin", "python3")

	if _, err := os.Stat(pythonExe); err == nil {
		// Python is already installed, verify it works
		if err := p.testSingleExecutable(pythonExe); err == nil {
			return pythonExe, true
		}
	}

	return "", false
}

// installPythonToRepo installs Python to the repository directory (simplified without pyenv)
func (p *PythonLanguage) installPythonToRepo() (string, error) {
	return "", fmt.Errorf("python runtime not found. Please install Python manually")
}

// isVersionAcceptable checks if a Python version satisfies the requirements
func (p *PythonLanguage) isVersionAcceptable(actualVersion, requestedVersion string) bool {
	if requestedVersion == LatestVersionString || requestedVersion == language.VersionDefault {
		// Any Python 3.x version is acceptable for "latest" or "default"
		return strings.HasPrefix(actualVersion, "3.")
	}

	// For specific versions, check if it matches
	return strings.HasPrefix(actualVersion, requestedVersion)
}
