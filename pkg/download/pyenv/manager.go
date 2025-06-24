// Package pyenv provides an isolated Python version manager for go-pre-commit
// This package handles downloading, installing, and managing Python versions
// in isolated environments for pre-commit hook execution
package pyenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/blairham/go-pre-commit/pkg/constants"
	"github.com/blairham/go-pre-commit/pkg/download"
)

// Manager handles Python version management for isolated environments
type Manager struct {
	DownloadManager *download.Manager
	BaseDir         string
	CacheDir        string
	PyenvPath       string // Path to the pyenv binary
}

// PythonVersion represents a Python version with download information
type PythonVersion struct {
	Version    string
	URL        string
	Filename   string
	SHA256     string
	Size       int64
	Available  bool
	IsPrebuilt bool
}

// PythonRelease represents a Python release with multiple download options
type PythonRelease struct {
	Downloads map[string]PythonVersion // key: platform-arch (e.g., "darwin-arm64", "linux-x86_64")
	Version   string
}

// NewManager creates a new Python version manager with isolated installation directory
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		// Default to a subdirectory in the user's cache directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "/tmp" // fallback if home dir is not accessible
		}
		baseDir = filepath.Join(homeDir, ".cache", "pre-commit", "python")
	}

	return &Manager{
		BaseDir:         baseDir,
		DownloadManager: download.NewManager(),
		CacheDir:        filepath.Join(baseDir, "cache"),
	}
}

// GetVersionsDir returns the directory where Python versions are installed
func (m *Manager) GetVersionsDir() string {
	return filepath.Join(m.BaseDir, "versions")
}

// GetVersionPath returns the path to a specific Python version installation
func (m *Manager) GetVersionPath(version string) string {
	return filepath.Join(m.GetVersionsDir(), version)
}

// GetPythonExecutable returns the path to the Python executable for a version
func (m *Manager) GetPythonExecutable(version string) string {
	// Try to resolve the version to an actual installed version
	resolvedVersion, err := m.ResolveVersion(version)
	if err != nil {
		// Fall back to using the requested version directly
		resolvedVersion = version
	}

	versionPath := m.GetVersionPath(resolvedVersion)

	switch runtime.GOOS {
	case constants.WindowsOS:
		return filepath.Join(versionPath, "python.exe")
	default:
		// Unix-like systems (including macOS)
		// Pyenv installs Python in a standard unix layout
		return filepath.Join(versionPath, "bin", "python3")
	}
}

// IsVersionInstalled checks if a Python version is already installed
func (m *Manager) IsVersionInstalled(version string) bool {
	// Try to resolve the version to an actual installed version
	resolvedVersion, err := m.ResolveVersion(version)
	if err != nil {
		return false
	}

	pythonPath := m.GetPythonExecutable(resolvedVersion)
	_, err = os.Stat(pythonPath)
	return err == nil
}

// GetInstalledVersions returns a list of installed Python versions
func (m *Manager) GetInstalledVersions() ([]string, error) {
	versionsDir := m.GetVersionsDir()

	entries, err := os.ReadDir(versionsDir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}

	sort.Strings(versions)
	return versions, nil
}

// GetLatestVersion returns the latest stable Python version
func (m *Manager) GetLatestVersion() (string, error) {
	versions, err := m.GetAvailableVersions()
	if err != nil {
		return "", err
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no Python versions available")
	}

	// Return the latest version (first version in our curated list)
	return versions[0].Version, nil
}

// GetAvailableVersions fetches available Python versions
func (m *Manager) GetAvailableVersions() ([]PythonRelease, error) {
	return m.getStableVersions(), nil
}

// GetPlatformKey returns the platform key for the current system
func (m *Manager) GetPlatformKey() string {
	goos := runtime.GOOS
	arch := runtime.GOARCH

	// Normalize architecture names
	switch arch {
	case constants.ArchAMD64:
		arch = constants.ArchAMD64
	case constants.ArchARM64:
		arch = constants.ArchARM64
	case constants.Arch386:
		arch = "x86"
	default:
		arch = constants.ArchAMD64 // fallback
	}

	return fmt.Sprintf("%s-%s", goos, arch)
}

// InstallVersion downloads and installs a specific Python version using pyenv
func (m *Manager) InstallVersion(version string) error {
	if m.IsVersionInstalled(version) {
		return nil // Already installed
	}

	// Ensure pyenv is available
	if err := m.ensurePyenv(); err != nil {
		return fmt.Errorf("failed to setup pyenv: %w", err)
	}

	// Set up environment variables for pyenv with optimization flags
	pyenvRoot := m.BaseDir
	pyenvBinDir := filepath.Join(pyenvRoot, "pyenv", "bin")
	pyenvLibexecDir := filepath.Join(pyenvRoot, "pyenv", "libexec")
	pyenvShimsDir := filepath.Join(pyenvRoot, "shims")

	// Create shims directory
	if err := os.MkdirAll(pyenvShimsDir, 0o750); err != nil {
		return fmt.Errorf("failed to create shims directory: %w", err)
	}

	// Try to use pre-built binary first for speed
	if err := m.tryDirectPythonDownload(version); err == nil {
		return nil
	}

	if err := m.tryPrebuiltInstallation(version, pyenvRoot, pyenvBinDir, pyenvLibexecDir, pyenvShimsDir); err == nil {
		return nil
	}

	return m.compileFromSource(version, pyenvRoot, pyenvBinDir, pyenvLibexecDir, pyenvShimsDir)
}

// tryDirectPythonDownload attempts to download and extract official Python binaries directly
func (m *Manager) tryDirectPythonDownload(version string) error {
	// Map common version requests to full versions
	fullVersion := version
	switch version {
	case Python312:
		fullVersion = "3.12.7" // Use 3.12.7 which is available
	case "3.11":
		fullVersion = "3.11.10"
	case "3.10":
		fullVersion = "3.10.15"
	case "3.9":
		fullVersion = "3.9.20"
	}

	// Use python-build-standalone for fast downloads (now owned by astral-sh)
	var downloadURL string
	var archiveName string

	if runtime.GOOS == "darwin" {
		if runtime.GOARCH == "arm64" {
			archiveName = fmt.Sprintf("cpython-%s+20241002-aarch64-apple-darwin-install_only.tar.gz", fullVersion)
		} else {
			archiveName = fmt.Sprintf("cpython-%s+20241002-x86_64-apple-darwin-install_only.tar.gz", fullVersion)
		}
		downloadURL = fmt.Sprintf(
			"https://github.com/astral-sh/python-build-standalone/releases/download/20241002/%s",
			archiveName,
		)
	} else {
		return fmt.Errorf("direct download not supported on %s", runtime.GOOS)
	}

	downloadPath := filepath.Join(m.CacheDir, archiveName)
	installPath := m.GetVersionPath(fullVersion)

	// Download the Python standalone build
	if err := m.DownloadManager.DownloadFile(downloadURL, downloadPath); err != nil {
		return fmt.Errorf("failed to download Python standalone: %w", err)
	}

	// Extract directly to install path
	if err := m.extractStandalonePython(downloadPath, installPath); err != nil {
		return fmt.Errorf("failed to extract Python: %w", err)
	}

	// Install virtualenv for isolated environments
	if err := m.installVirtualenv(fullVersion); err != nil {
		// Don't fail the entire installation if virtualenv fails
		_ = err // Explicitly ignore error
	}

	return nil
}

// extractStandalonePython extracts a Python standalone build
func (m *Manager) extractStandalonePython(archivePath, installPath string) error {
	// Create installation directory
	if err := os.MkdirAll(installPath, 0o750); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Extract the tar.gz archive
	cmd := exec.Command("tar", "-xzf", archivePath, "-C", installPath, "--strip-components=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	// Check if Python binary exists and is executable
	pythonBin := filepath.Join(installPath, "bin", "python3")
	if _, err := os.Stat(pythonBin); err == nil {
		// Make the bin directory executable
		if err := m.makeExecutable(filepath.Join(installPath, "bin")); err != nil {
			return fmt.Errorf("failed to make bin directory executable: %w", err)
		}
		return nil
	}

	// Try alternative locations and specific version executables
	altPaths := []string{
		filepath.Join(installPath, "python", "bin", "python3"),
		filepath.Join(installPath, "bin", "python3.11"),
		filepath.Join(installPath, "bin", "python3.12"),
		filepath.Join(installPath, "bin", "python3.10"),
		filepath.Join(installPath, "bin", "python3.9"),
	}

	for _, pythonBin := range altPaths {
		if _, err := os.Stat(pythonBin); err != nil {
			continue
		}

		// Make the bin directory executable
		if err := m.makeExecutable(filepath.Dir(pythonBin)); err != nil {
			continue
		}

		// Create symlink in standard location if needed
		return m.ensurePythonSymlink(installPath, pythonBin)
	}

	return fmt.Errorf("could not find Python executable after extraction")
}

// ensurePythonSymlink creates a symlink in the standard location if needed
func (m *Manager) ensurePythonSymlink(installPath, pythonBin string) error {
	standardBin := filepath.Join(installPath, "bin", "python3")
	if _, err := os.Stat(standardBin); os.IsNotExist(err) {
		binDir := filepath.Join(installPath, "bin")
		if err := os.MkdirAll(binDir, 0o750); err != nil {
			return fmt.Errorf("failed to create bin directory: %w", err)
		}
		if err := os.Symlink(pythonBin, standardBin); err != nil {
			return fmt.Errorf("failed to create python3 symlink: %w", err)
		}
	}
	return nil
}

// tryPrebuiltInstallation attempts to install Python using pre-built binaries
func (m *Manager) tryPrebuiltInstallation(
	version, pyenvRoot, pyenvBinDir, pyenvLibexecDir, pyenvShimsDir string,
) error {
	env := append(os.Environ(),
		"PYENV_ROOT="+pyenvRoot,
		"PATH="+pyenvBinDir+":"+pyenvLibexecDir+":"+pyenvShimsDir+":"+os.Getenv("PATH"),
		// Enable faster download and installation
		"PYTHON_BUILD_MIRROR_URL=https://github.com/indygreg/python-build-standalone/releases/download",
		"PYTHON_BUILD_SKIP_MIRROR=0",
		// Use faster download options
		"PYTHON_BUILD_CURL_OPTS=--connect-timeout 10 --max-time 300 --retry 3",
		// Skip unnecessary steps for speed
		"PYTHON_BUILD_SKIP_MIRROR_CLEANUP=1",
		// Enable parallel downloads if available
		"MAKE_OPTS=-j"+fmt.Sprintf("%d", runtime.NumCPU()),
	)

	start := time.Now()

	// Add timeout to prevent hanging indefinitely - reduced to 5 minutes for pre-built
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Use more aggressive flags for faster installation
	cmdWithTimeout := exec.CommandContext(ctx, m.PyenvPath, "install", "--skip-existing", "--keep", version)
	cmdWithTimeout.Env = env
	cmdWithTimeout.Dir = pyenvRoot

	// Capture output for better debugging but don't show verbose output
	_, err := cmdWithTimeout.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("python %s installation timed out after %v", version, duration)
		}
		// Log the error output for debugging, but keep it quiet
		return fmt.Errorf("pre-built installation failed, will try compilation: %w", err)
	}

	// Install virtualenv for isolated environments
	if err := m.installVirtualenv(version); err != nil {
		// Don't fail the entire installation if virtualenv fails
		_ = err // Explicitly ignore error
	}

	return nil
}

// compileFromSource compiles Python from source code (fallback method)
func (m *Manager) compileFromSource(version, pyenvRoot, pyenvBinDir, pyenvLibexecDir, pyenvShimsDir string) error {
	// Optimize compilation with multiple cores and faster builds
	numCPU := runtime.NumCPU()

	env := append(os.Environ(),
		"PYENV_ROOT="+pyenvRoot,
		"PATH="+pyenvBinDir+":"+pyenvLibexecDir+":"+pyenvShimsDir+":"+os.Getenv("PATH"),
		// Enable parallel compilation
		fmt.Sprintf("MAKE_OPTS=-j%d", numCPU),
		// Optimize Python build for speed - disable optional modules for faster builds
		"PYTHON_CONFIGURE_OPTS=--enable-shared --disable-test-modules --without-doc-strings",
		// Use optimized compiler flags
		"CFLAGS=-O2 -pipe",
		// Skip building optional modules that slow down compilation
		"PYTHON_DISABLE_MODULES=_tkinter,_gdbm,_lzma,nis,ossaudiodev,_curses,_curses_panel",
	)

	start := time.Now()

	// Add timeout to prevent hanging indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	cmdWithTimeout := exec.CommandContext(ctx, m.PyenvPath, "install", "--verbose", version)
	cmdWithTimeout.Env = env
	cmdWithTimeout.Dir = pyenvRoot

	err := cmdWithTimeout.Run()
	duration := time.Since(start)

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("python %s compilation timed out after %v", version, duration)
		}
		return fmt.Errorf("failed to compile Python %s from source in %v: %w", version, duration, err)
	}

	// Install virtualenv for isolated environments
	if err := m.installVirtualenv(version); err != nil {
		// Don't fail the entire installation if virtualenv fails
		_ = err // Explicitly ignore error
	}

	return nil
}

// PreDownloadPythonSources downloads Python source archives in parallel for faster installation
func (m *Manager) PreDownloadPythonSources(versions []string) error {
	if len(versions) == 0 {
		return nil
	}

	// Ensure pyenv is available
	if err := m.ensurePyenv(); err != nil {
		return fmt.Errorf("failed to setup pyenv: %w", err)
	}

	pyenvRoot := m.BaseDir
	pyenvBinDir := filepath.Join(pyenvRoot, "pyenv", "bin")
	pyenvLibexecDir := filepath.Join(pyenvRoot, "pyenv", "libexec")
	pyenvShimsDir := filepath.Join(pyenvRoot, "shims")

	env := append(os.Environ(),
		"PYENV_ROOT="+pyenvRoot,
		"PATH="+pyenvBinDir+":"+pyenvLibexecDir+":"+pyenvShimsDir+":"+os.Getenv("PATH"),
	)

	// Download sources in parallel for speed
	type downloadResult struct {
		err      error
		version  string
		duration time.Duration
	}

	results := make(chan downloadResult, len(versions))

	for _, version := range versions {
		go func(v string) {
			start := time.Now()
			// Use python-build to just download without installing
			cmd := exec.Command(m.PyenvPath, "install", "--keep", "--skip-existing", v)
			cmd.Env = env
			cmd.Dir = pyenvRoot

			err := cmd.Run()
			results <- downloadResult{
				version:  v,
				err:      err,
				duration: time.Since(start),
			}
		}(version)
	}

	// Collect results
	successful := 0
	for range versions {
		result := <-results
		if result.err == nil {
			successful++
		}
	}

	return nil
}

// installPython handles the actual installation of Python
func (m *Manager) installPython(version, downloadPath string, isPrebuilt bool) error {
	installPath := m.GetVersionPath(version)

	// Create installation directory
	if err := os.MkdirAll(installPath, 0o750); err != nil {
		return fmt.Errorf("failed to create installation directory: %w", err)
	}

	var err error
	switch runtime.GOOS {
	case constants.DarwinOS:
		err = m.installMacOS(version, downloadPath, installPath)
	case constants.LinuxOS:
		if isPrebuilt {
			err = m.installLinuxPrebuilt(version, downloadPath, installPath)
		} else {
			err = m.installLinuxFromSource(version, downloadPath, installPath)
		}
	case constants.WindowsOS:
		err = m.installWindows(version, downloadPath, installPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err != nil {
		return err
	}

	return nil
}

// upgradePipAndInstallPackages upgrades pip and optionally installs essential packages for the specified Python version
func (m *Manager) upgradePipAndInstallPackages(version string) error {
	pythonPath := m.GetPythonExecutable(version)

	// First, upgrade pip to the latest version
	cmd := exec.Command(pythonPath, "-m", "pip", "install", "--upgrade", "pip")
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upgrade pip: %w", err)
	}

	// Optionally install setuptools and wheel for better package management
	cmd = exec.Command(pythonPath, "-m", "pip", "install", "--upgrade", "setuptools", "wheel")
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")

	if err := cmd.Run(); err != nil {
		// Don't fail for this - they're nice to have but not essential
		_ = err // Explicitly ignore error
	}

	return nil
}

// EnsureVersion ensures a Python version is installed, installing it if necessary
func (m *Manager) EnsureVersion(version string) (string, error) {
	// Handle special version names
	switch version {
	case "latest", "default", "":
		latestVersion, err := m.GetLatestVersion()
		if err != nil {
			return "", err
		}
		version = latestVersion
	}

	if !m.IsVersionInstalled(version) {
		if err := m.InstallVersion(version); err != nil {
			return "", err
		}
	}

	return m.GetPythonExecutable(version), nil
}

// GetSystemPython attempts to find system Python installation
func (m *Manager) GetSystemPython() (string, error) {
	// Try common Python executable names
	candidates := []string{"python3", "python", "python3.12", "python3.11", "python3.10"}

	for _, candidate := range candidates {
		if path, err := m.findExecutable(candidate); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no system Python installation found")
}

// InstallToDirectory installs Python to a specific directory instead of the cache
// This is used for repository-specific Python installations
func (m *Manager) InstallToDirectory(version, targetDir string) (string, error) {
	// Handle version specification
	if version == "latest" || version == "default" {
		var err error
		version, err = m.GetLatestVersion()
		if err != nil {
			return "", fmt.Errorf("failed to get latest version: %w", err)
		}
	}

	// Check if Python is already installed in the target directory
	pythonExe := filepath.Join(targetDir, "bin", "python3")
	if _, err := os.Stat(pythonExe); err == nil {
		// Verify the installation works
		if err := exec.Command(pythonExe, "--version").Run(); err == nil {
			return pythonExe, nil
		}
	}

	// Create target directory
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	// First install Python in the cache (if not already installed)
	if !m.IsVersionInstalled(version) {
		if err := m.InstallVersion(version); err != nil {
			return "", fmt.Errorf("failed to install Python %s: %w", version, err)
		}
	}

	// Copy the installed Python to the target directory
	sourceDir := m.GetVersionPath(version)
	if err := m.copyPythonInstallation(sourceDir, targetDir); err != nil {
		return "", fmt.Errorf("failed to copy Python installation: %w", err)
	}

	// Upgrade pip in the target directory for isolated Python environment
	if err := m.upgradePipInDirectory(targetDir); err != nil {
		// Ignore pip upgrade failures in target directory
		_ = err // Explicitly ignore error
	}

	return pythonExe, nil
}

// copyPythonInstallation copies a Python installation from source to target directory
func (m *Manager) copyPythonInstallation(sourceDir, targetDir string) error {
	// Use cp command for efficient copying
	cmd := exec.Command("cp", "-R", sourceDir+"/.", targetDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy Python installation: %w", err)
	}

	// Make sure the Python executable is executable
	pythonExe := filepath.Join(targetDir, "bin", "python3")
	if err := os.Chmod(pythonExe, 0o750); err != nil { //nolint:gosec // Executable needs execute permissions
		return fmt.Errorf("failed to make Python executable: %w", err)
	}

	return nil
}

// upgradePipInDirectory upgrades pip in a specific Python directory (for isolated environments)
func (m *Manager) upgradePipInDirectory(pythonDir string) error {
	pythonExe := filepath.Join(pythonDir, "bin", "python3")

	// Upgrade pip to the latest version
	cmd := exec.Command(pythonExe, "-m", "pip", "install", "--upgrade", "pip")
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to upgrade pip in directory: %w", err)
	}

	// Install setuptools and wheel for better package management
	cmd = exec.Command(pythonExe, "-m", "pip", "install", "--upgrade", "setuptools", "wheel")
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")
	if err := cmd.Run(); err != nil {
		// Don't fail for this - they're nice to have but not essential
		_ = err // Explicitly ignore error
	}

	return nil
}

// ensurePyenv downloads and sets up pyenv if not already available
func (m *Manager) ensurePyenv() error {
	if m.PyenvPath != "" {
		// Check if the pyenv binary still exists
		if _, err := os.Stat(m.PyenvPath); err == nil {
			return nil
		}
	}

	// Create pyenv directory in our cache
	pyenvDir := filepath.Join(m.BaseDir, "pyenv")
	if err := os.MkdirAll(pyenvDir, 0o750); err != nil {
		return fmt.Errorf("failed to create pyenv directory: %w", err)
	}

	// Use specific pyenv version for faster, more reliable downloads
	pyenvVersion := "v2.4.18" // Latest stable release
	pyenvURL := fmt.Sprintf("https://github.com/pyenv/pyenv/archive/refs/tags/%s.tar.gz", pyenvVersion)
	pyenvArchive := filepath.Join(m.CacheDir, fmt.Sprintf("pyenv-%s.tar.gz", pyenvVersion))

	if err := m.DownloadManager.DownloadFile(pyenvURL, pyenvArchive); err != nil {
		return fmt.Errorf("failed to download pyenv: %w", err)
	}

	// Extract pyenv using faster tar extraction

	// Create a temporary directory for extraction
	tempDir := filepath.Join(m.CacheDir, "pyenv-temp")
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Printf("⚠️  Warning: failed to clean up temp directory: %v\n", err)
		}
	}()

	// Extract to temp directory first
	cmd := exec.Command("tar", "-xzf", pyenvArchive, "-C", tempDir, "--strip-components=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract pyenv: %w", err)
	}

	// Move extracted contents to final pyenv directory
	if err := os.Rename(tempDir, pyenvDir); err != nil {
		// If rename fails, the directory might exist, remove it and try again
		if rmErr := os.RemoveAll(pyenvDir); rmErr != nil {
			return fmt.Errorf("failed to remove existing pyenv directory: %w", rmErr)
		}
		if err := os.Rename(tempDir, pyenvDir); err != nil {
			return fmt.Errorf("failed to move pyenv to final location: %w", err)
		}
	}

	// Set the pyenv path
	m.PyenvPath = filepath.Join(pyenvDir, "bin", "pyenv")

	// Make binaries executable in parallel
	binDirs := []string{
		filepath.Join(pyenvDir, "bin"),
		filepath.Join(pyenvDir, "libexec"),
	}

	for _, binDir := range binDirs {
		if err := m.makeExecutable(binDir); err != nil {
			return fmt.Errorf("failed to make binaries executable in %s: %w", binDir, err)
		}
	}

	return nil
}

// makeExecutable makes all files in a directory executable
func (m *Manager) makeExecutable(binDir string) error {
	files, err := os.ReadDir(binDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			filePath := filepath.Join(binDir, file.Name())
			// Use 0o755 for executable files as this is standard for binaries
			if err := os.Chmod(filePath, 0o755); err != nil { //nolint:gosec // executable permission required for pyenv binaries
				return fmt.Errorf("failed to make %s executable: %w", filePath, err)
			}
		}
	}
	return nil
}

// ResolveVersion finds the installed version that matches the requested version
// For example, if "3.11" is requested and "3.11.10" is installed, it returns "3.11.10"
func (m *Manager) ResolveVersion(requestedVersion string) (string, error) {
	installedVersions, err := m.GetInstalledVersions()
	if err != nil {
		return "", fmt.Errorf("failed to get installed versions: %w", err)
	}

	// If exact match, return it
	for _, installed := range installedVersions {
		if installed == requestedVersion {
			return installed, nil
		}
	}

	// If partial match (e.g., "3.11" matches "3.11.10"), return the first match
	// Sort versions in descending order to get the latest patch version
	sort.Slice(installedVersions, func(i, j int) bool {
		return installedVersions[i] > installedVersions[j]
	})

	for _, installed := range installedVersions {
		if strings.HasPrefix(installed, requestedVersion+".") {
			return installed, nil
		}
	}

	return "", fmt.Errorf("no installed version matches %s (available: %v)", requestedVersion, installedVersions)
}

// installVirtualenv installs virtualenv for a specific Python version
func (m *Manager) installVirtualenv(version string) error {
	pythonPath := m.GetPythonExecutable(version)

	// Check if virtualenv is already installed
	checkCmd := exec.Command(pythonPath, "-m", "virtualenv", "--version")
	if err := checkCmd.Run(); err == nil {
		// virtualenv is already available
		return nil
	}

	// Install virtualenv using pip
	cmd := exec.Command(pythonPath, "-m", "pip", "install", "virtualenv")
	cmd.Env = append(os.Environ(), "PIP_DISABLE_PIP_VERSION_CHECK=1")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install virtualenv: %w", err)
	}

	// Verify installation
	verifyCmd := exec.Command(pythonPath, "-m", "virtualenv", "--version")
	if err := verifyCmd.Run(); err != nil {
		return fmt.Errorf("virtualenv installation verification failed: %w", err)
	}

	return nil
}

// CleanCache removes all cached Python installations and downloads
func (m *Manager) CleanCache() error {
	// Remove the entire base directory which contains versions and cache
	if err := os.RemoveAll(m.BaseDir); err != nil {
		return fmt.Errorf("failed to clean pyenv cache: %w", err)
	}

	// Recreate the base directory structure
	if err := os.MkdirAll(m.BaseDir, 0o750); err != nil {
		return fmt.Errorf("failed to recreate pyenv base directory: %w", err)
	}

	if err := os.MkdirAll(m.CacheDir, 0o750); err != nil {
		return fmt.Errorf("failed to recreate pyenv cache directory: %w", err)
	}

	// Clear the pyenv path since we removed the pyenv installation
	m.PyenvPath = ""

	return nil
}

// GetCacheSize returns the total size of the pyenv cache in bytes
func (m *Manager) GetCacheSize() (int64, error) {
	var totalSize int64

	err := filepath.Walk(m.BaseDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if os.IsNotExist(err) {
		return 0, nil
	}

	return totalSize, err
}
