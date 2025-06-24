// Package nodeenv provides an isolated Node.js version manager for go-pre-commit
// This package handles downloading, installing, and managing Node.js versions
// in isolated environments for pre-commit hook execution
package nodeenv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/constants"
	"github.com/blairham/go-pre-commit/pkg/download"
)

// Manager handles Node.js version management for isolated environments
type Manager struct {
	DownloadManager *download.Manager
	BaseDir         string
	CacheDir        string
	NodeenvPath     string // Path to the nodeenv binary
}

// NodeVersion represents a Node.js version with download information
type NodeVersion struct {
	Version    string
	URL        string
	Filename   string
	SHA256     string
	Size       int64
	Available  bool
	IsPrebuilt bool
}

// NodeRelease represents a Node.js release with multiple download options
type NodeRelease struct {
	Downloads map[string]NodeVersion // key: platform-arch (e.g., "darwin-arm64", "linux-x64")
	Version   string
}

// NewManager creates a new Node.js version manager with isolated installation directory
func NewManager(baseDir string) *Manager {
	if baseDir == "" {
		// Default to a subdirectory in the user's cache directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "/tmp" // fallback if home dir is not accessible
		}
		baseDir = filepath.Join(homeDir, ".cache", "pre-commit", "node")
	}

	return &Manager{
		BaseDir:         baseDir,
		DownloadManager: download.NewManager(),
		CacheDir:        filepath.Join(baseDir, "cache"),
	}
}

// GetVersionsDir returns the directory where Node.js versions are installed
func (m *Manager) GetVersionsDir() string {
	return filepath.Join(m.BaseDir, "versions")
}

// GetVersionPath returns the path to a specific Node.js version installation
func (m *Manager) GetVersionPath(version string) string {
	return filepath.Join(m.GetVersionsDir(), version)
}

// GetNodeExecutable returns the path to the Node.js executable for a version
func (m *Manager) GetNodeExecutable(version string) string {
	// Try to resolve the version to an actual installed version
	resolvedVersion, err := m.ResolveVersion(version)
	if err != nil {
		// Fall back to using the requested version directly
		resolvedVersion = version
	}

	versionPath := m.GetVersionPath(resolvedVersion)

	switch runtime.GOOS {
	case constants.WindowsOS:
		return filepath.Join(versionPath, "node.exe")
	default:
		// Unix-like systems (including macOS)
		return filepath.Join(versionPath, "bin", "node")
	}
}

// GetNpmExecutable returns the path to the npm executable for a version
func (m *Manager) GetNpmExecutable(version string) string {
	// Try to resolve the version to an actual installed version
	resolvedVersion, err := m.ResolveVersion(version)
	if err != nil {
		// Fall back to using the requested version directly
		resolvedVersion = version
	}

	versionPath := m.GetVersionPath(resolvedVersion)

	switch runtime.GOOS {
	case constants.WindowsOS:
		return filepath.Join(versionPath, "npm.cmd")
	default:
		// Unix-like systems (including macOS)
		return filepath.Join(versionPath, "bin", "npm")
	}
}

// IsVersionInstalled checks if a Node.js version is already installed
func (m *Manager) IsVersionInstalled(version string) bool {
	// Try to resolve the version to an actual installed version
	resolvedVersion, err := m.ResolveVersion(version)
	if err != nil {
		return false
	}

	executable := m.GetNodeExecutable(resolvedVersion)
	if _, err := os.Stat(executable); err != nil {
		return false
	}

	return true
}

// ResolveVersion resolves a version specification to an actual version
func (m *Manager) ResolveVersion(versionSpec string) (string, error) {
	switch versionSpec {
	case "system", "default", "":
		// For system/default, check if we have any installed versions
		installedVersions, err := m.GetInstalledVersions()
		if err != nil || len(installedVersions) == 0 {
			return "", fmt.Errorf("no Node.js versions installed")
		}
		// Return the first (typically latest) version
		return installedVersions[0], nil
	default:
		// Specific version requested
		return versionSpec, nil
	}
}

// GetInstalledVersions returns a list of installed Node.js versions
func (m *Manager) GetInstalledVersions() ([]string, error) {
	versionsDir := m.GetVersionsDir()
	if _, err := os.Stat(versionsDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}

	// Sort versions in descending order (latest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i] > versions[j]
	})

	return versions, nil
}

// InstallVersion downloads and installs a specific Node.js version
func (m *Manager) InstallVersion(ctx context.Context, version string) error {
	if m.IsVersionInstalled(version) {
		return nil
	}

	// Create directories
	if err := os.MkdirAll(m.CacheDir, 0o750); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	if err := os.MkdirAll(m.GetVersionsDir(), 0o750); err != nil {
		return fmt.Errorf("failed to create versions directory: %w", err)
	}

	// Get the download URL for the current platform
	downloadURL, filename, err := m.getDownloadInfo(version)
	if err != nil {
		return fmt.Errorf("failed to get download info: %w", err)
	}

	// Download Node.js
	downloadPath := filepath.Join(m.CacheDir, filename)
	if err := m.DownloadManager.DownloadFileWithContext(ctx, downloadURL, downloadPath); err != nil {
		return fmt.Errorf("failed to download Node.js: %w", err)
	}

	// Install Node.js
	installPath := m.GetVersionPath(version)
	if err := m.installNodeJS(version, downloadPath, installPath); err != nil {
		return fmt.Errorf("failed to install Node.js: %w", err)
	}

	return nil
}

// getDownloadInfo returns the download URL and filename for a Node.js version
func (m *Manager) getDownloadInfo(version string) (string, string, error) {
	// Clean up version string (remove 'v' prefix if present)
	version = strings.TrimPrefix(version, "v")

	var platform, arch, ext string

	switch runtime.GOOS {
	case constants.DarwinOS:
		platform = "darwin"
		ext = "tar.gz"
	case constants.LinuxOS:
		platform = "linux"
		ext = "tar.xz"
	case constants.WindowsOS:
		platform = "win"
		ext = "zip"
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	case "386":
		arch = "x86"
	default:
		return "", "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	filename := fmt.Sprintf("node-v%s-%s-%s.%s", version, platform, arch, ext)
	downloadURL := fmt.Sprintf("https://nodejs.org/dist/v%s/%s", version, filename)

	return downloadURL, filename, nil
}

// installNodeJS extracts and installs Node.js from the downloaded archive
func (m *Manager) installNodeJS(version, downloadPath, installPath string) error {
	// Create installation directory
	if err := os.MkdirAll(installPath, 0o750); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	switch runtime.GOOS {
	case constants.WindowsOS:
		return m.installWindows(version, downloadPath, installPath)
	case constants.DarwinOS, constants.LinuxOS:
		return m.installUnix(version, downloadPath, installPath)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// installUnix handles Node.js installation on Unix-like systems
func (m *Manager) installUnix(version, downloadPath, installPath string) error {
	// Determine extraction command based on file extension
	var cmd *exec.Cmd
	switch {
	case strings.HasSuffix(downloadPath, ".tar.xz"):
		cmd = exec.Command("tar", "-xJf", downloadPath, "-C", installPath, "--strip-components=1")
	case strings.HasSuffix(downloadPath, ".tar.gz"):
		cmd = exec.Command("tar", "-xzf", downloadPath, "-C", installPath, "--strip-components=1")
	default:
		return fmt.Errorf("unsupported archive format")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to extract Node.js: %w\nOutput: %s", err, output)
	}

	// Verify installation
	nodeExe := m.GetNodeExecutable(version)
	if _, err := os.Stat(nodeExe); err != nil {
		return fmt.Errorf("node.js executable not found after installation: %w", err)
	}

	return nil
}

// UninstallVersion removes an installed Node.js version
func (m *Manager) UninstallVersion(version string) error {
	if !m.IsVersionInstalled(version) {
		return fmt.Errorf("node.js %s is not installed", version)
	}

	versionPath := m.GetVersionPath(version)
	if err := os.RemoveAll(versionPath); err != nil {
		return fmt.Errorf("failed to remove Node.js %s: %w", version, err)
	}

	return nil
}

// GetAvailableVersions returns a list of available Node.js versions for download
func (m *Manager) GetAvailableVersions() ([]string, error) {
	// This would typically fetch from Node.js release API
	// For now, return some common LTS versions
	return []string{
		"20.11.0",
		"18.19.0",
		"16.20.2",
		"14.21.3",
	}, nil
}

// SetGlobalVersion sets the global Node.js version
func (m *Manager) SetGlobalVersion(version string) error {
	if !m.IsVersionInstalled(version) {
		return fmt.Errorf("node.js %s is not installed", version)
	}

	globalFile := filepath.Join(m.BaseDir, "global")
	if err := os.WriteFile(globalFile, []byte(version), 0o600); err != nil {
		return fmt.Errorf("failed to set global version: %w", err)
	}

	return nil
}

// GetGlobalVersion returns the global Node.js version
func (m *Manager) GetGlobalVersion() (string, error) {
	globalFile := filepath.Join(m.BaseDir, "global")
	content, err := os.ReadFile(filepath.Clean(globalFile))
	if err != nil {
		if os.IsNotExist(err) {
			// No global version set, try to find any installed version
			versions, versionErr := m.GetInstalledVersions()
			if versionErr != nil || len(versions) == 0 {
				return "", fmt.Errorf("no global version set and no versions installed")
			}
			return versions[0], nil
		}
		return "", fmt.Errorf("failed to read global version: %w", err)
	}

	return strings.TrimSpace(string(content)), nil
}

// CreateEnvironment creates a Node.js environment with the specified version
func (m *Manager) CreateEnvironment(envPath, version string) error {
	return m.CreateEnvironmentWithOptions(envPath, version, true, true)
}

// CreateEnvironmentWithOptions creates a Node.js environment with specified options
// prebuilt: Use prebuilt binaries when possible (saves download time)
// cleanSrc: Remove source files after installation (saves ~70MB per environment)
func (m *Manager) CreateEnvironmentWithOptions(envPath, version string, _, cleanSrc bool) error {
	if err := m.EnsureVersionInstalled(context.Background(), version); err != nil {
		return fmt.Errorf("failed to ensure Node.js version: %w", err)
	}

	// Create environment directory
	if err := os.MkdirAll(envPath, 0o750); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	// Create bin directory and symlinks
	binDir := filepath.Join(envPath, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Create symlinks to Node.js and npm
	nodeExe := m.GetNodeExecutable(version)
	npmExe := m.GetNpmExecutable(version)

	envNodeExe := filepath.Join(binDir, "node")
	envNpmExe := filepath.Join(binDir, "npm")

	if err := m.createEnvironmentExecutables(nodeExe, npmExe, envNodeExe, envNpmExe); err != nil {
		return err
	}

	// Clean source files if requested (following Python pre-commit --clean-src behavior)
	if cleanSrc {
		m.cleanSourceFiles(version)
	}

	return nil
}

// createEnvironmentExecutables creates the necessary executables for the environment
func (m *Manager) createEnvironmentExecutables(nodeExe, npmExe, envNodeExe, envNpmExe string) error {
	if runtime.GOOS == constants.WindowsOS {
		return m.createWindowsExecutables(nodeExe, npmExe, envNodeExe, envNpmExe)
	}
	return m.createUnixExecutables(nodeExe, npmExe, envNodeExe, envNpmExe)
}

// createUnixExecutables creates symlinks for Unix-like systems
func (m *Manager) createUnixExecutables(nodeExe, npmExe, envNodeExe, envNpmExe string) error {
	if err := os.Symlink(nodeExe, envNodeExe); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create node symlink: %w", err)
	}
	if err := os.Symlink(npmExe, envNpmExe); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create npm symlink: %w", err)
	}
	return nil
}

// createWindowsExecutables creates batch files for Windows
func (m *Manager) createWindowsExecutables(nodeExe, npmExe, envNodeExe, envNpmExe string) error {
	nodeBat := fmt.Sprintf("@echo off\n%q %%*\n", nodeExe)
	if err := os.WriteFile(envNodeExe+".bat", []byte(nodeBat), 0o600); err != nil {
		return fmt.Errorf("failed to create node batch file: %w", err)
	}

	npmBat := fmt.Sprintf("@echo off\n%q %%*\n", npmExe)
	if err := os.WriteFile(envNpmExe+".bat", []byte(npmBat), 0o600); err != nil {
		return fmt.Errorf("failed to create npm batch file: %w", err)
	}
	return nil
}

// EnsureVersionInstalled ensures a Node.js version is installed, installing it if necessary
func (m *Manager) EnsureVersionInstalled(ctx context.Context, version string) error {
	resolvedVersion, err := m.ResolveVersion(version)
	if err != nil {
		// If we can't resolve the version, try to install the requested version
		return m.InstallVersion(ctx, version)
	}

	if !m.IsVersionInstalled(resolvedVersion) {
		return m.InstallVersion(ctx, resolvedVersion)
	}

	return nil
}

// ValidateEnvironment checks if a Node.js environment is valid and functional
func (m *Manager) ValidateEnvironment(envPath string) error {
	nodeExe := filepath.Join(envPath, "bin", "node")
	if runtime.GOOS == constants.WindowsOS {
		nodeExe = filepath.Join(envPath, "bin", "node.bat")
	}

	if _, err := os.Stat(nodeExe); err != nil {
		return fmt.Errorf("node executable not found in environment: %w", err)
	}

	// Test that Node.js can run
	cmd := exec.Command(nodeExe, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("node executable is not functional: %w", err)
	}

	return nil
}

// cleanSourceFiles removes unnecessary source files to save disk space
// This implements the --clean-src behavior from Python pre-commit
func (m *Manager) cleanSourceFiles(version string) {
	versionPath := m.GetVersionPath(version)

	// List of directories/files that can be safely removed to save space
	cleanTargets := []string{
		"lib/node_modules/npm/docs",
		"lib/node_modules/npm/man",
		"lib/node_modules/npm/test",
		"include",
		"share",
		"CHANGELOG.md",
		"README.md",
		"LICENSE",
	}

	for _, target := range cleanTargets {
		targetPath := filepath.Join(versionPath, target)
		if _, err := os.Stat(targetPath); err == nil {
			if err := os.RemoveAll(targetPath); err != nil {
				// Continue with other targets even if one fails
				continue
			}
		}
	}
}
