package nodeenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// WindowsOS represents the Windows operating system
	WindowsOS = "windows"
)

// installMacOS handles Node.js installation on macOS using tar.gz files
func (m *Manager) installMacOS(version, downloadPath, installPath string) error {
	// On macOS, we'll extract the tar.gz directly to our managed location

	// Create installation directory
	if err := os.MkdirAll(installPath, 0o750); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Extract using tar
	cmd := exec.Command("tar", "-xzf", downloadPath, "-C", installPath, "--strip-components=1")
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

// installLinux handles Node.js installation on Linux using tar.xz files
func (m *Manager) installLinux(version, downloadPath, installPath string) error {
	// Create installation directory
	if err := os.MkdirAll(installPath, 0o750); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Extract using tar with xz compression
	cmd := exec.Command("tar", "-xJf", downloadPath, "-C", installPath, "--strip-components=1")
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

// installWindows handles Node.js installation on Windows using ZIP files
func (m *Manager) installWindows(version, downloadPath, installPath string) error {
	// Create installation directory
	if err := os.MkdirAll(installPath, 0o750); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	// Create a temporary directory for extraction
	tempDir := filepath.Join(m.CacheDir, "temp_"+version)
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Printf("Warning: failed to cleanup temp directory: %v\n", err)
		}
	}()

	// Use PowerShell to extract ZIP file
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("Expand-Archive -Path '%s' -DestinationPath '%s' -Force", downloadPath, tempDir))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to extract Node.js: %w\nOutput: %s", err, output)
	}

	// Node.js on Windows extracts to a subdirectory, find it and move contents
	extractedDirs, err := findExtractedNodeDir(tempDir, version)
	if err != nil {
		return fmt.Errorf("failed to find extracted Node.js directory: %w", err)
	}

	if len(extractedDirs) == 0 {
		return fmt.Errorf("no extracted Node.js directory found")
	}

	// Move contents from the first (should be only) extracted directory
	extractedDir := extractedDirs[0]
	if err := m.moveDirectoryContents(extractedDir, installPath); err != nil {
		return fmt.Errorf("failed to move extracted files: %w", err)
	}

	// Verify installation
	nodeExe := m.GetNodeExecutable(version)
	if _, err := os.Stat(nodeExe); err != nil {
		return fmt.Errorf("node.js executable not found after installation: %w", err)
	}

	return nil
}

// findExtractedNodeDir finds directories that look like extracted Node.js
func findExtractedNodeDir(tempDir, version string) ([]string, error) {
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return nil, err
	}

	var nodeDirs []string
	versionStr := strings.TrimPrefix(version, "v")

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			// Look for directories that match Node.js naming patterns
			if strings.HasPrefix(name, "node-v"+versionStr) ||
				strings.HasPrefix(name, "node-"+versionStr) ||
				strings.Contains(name, "node") {
				nodeDirs = append(nodeDirs, filepath.Join(tempDir, name))
			}
		}
	}

	return nodeDirs, nil
}

// setupNodeEnvironment sets up environment variables for Node.js
func (m *Manager) setupNodeEnvironment(envPath, version string) error {
	// Create environment activation scripts
	if err := m.createActivationScripts(envPath, version); err != nil {
		return fmt.Errorf("failed to create activation scripts: %w", err)
	}

	return nil
}

// createActivationScripts creates scripts to activate the Node.js environment
func (m *Manager) createActivationScripts(envPath, version string) error {
	binDir := filepath.Join(envPath, "bin")
	nodeExe := m.GetNodeExecutable(version)
	npmExe := m.GetNpmExecutable(version)

	// Create Unix activation script
	if runtime.GOOS != WindowsOS {
		activateScript := fmt.Sprintf(`#!/bin/bash
# Node.js environment activation script
export NODE_VERSION=%s
export NODE_HOME=%s
export PATH=%s:$PATH
export NODE_PATH=%s/lib/node_modules

echo "Activated Node.js %s environment"
echo "Node.js executable: %s"
echo "npm executable: %s"
`,
			version, filepath.Dir(nodeExe), binDir, filepath.Dir(nodeExe), version, nodeExe, npmExe)

		activatePath := filepath.Join(envPath, "activate")
		if err := os.WriteFile(activatePath, []byte(activateScript), 0o600); err != nil {
			return fmt.Errorf("failed to create activation script: %w", err)
		}
		// Make the activation script executable
		// #nosec G302 - activation scripts need to be executable to work properly
		if err := os.Chmod(activatePath, 0o700); err != nil {
			return fmt.Errorf("failed to make activation script executable: %w", err)
		}
	}

	// Create Windows activation script
	if runtime.GOOS == WindowsOS {
		activateScript := fmt.Sprintf(`@echo off
REM Node.js environment activation script
set NODE_VERSION=%s
set NODE_HOME=%s
set PATH=%s;%%PATH%%
set NODE_PATH=%s\node_modules

echo Activated Node.js %s environment
echo Node.js executable: %s
echo npm executable: %s
`,
			version, filepath.Dir(nodeExe), binDir, filepath.Dir(nodeExe), version, nodeExe, npmExe)

		activatePath := filepath.Join(envPath, "activate.bat")
		if err := os.WriteFile(activatePath, []byte(activateScript), 0o600); err != nil {
			return fmt.Errorf("failed to create activation script: %w", err)
		}
	}

	return nil
}

// moveDirectoryContents moves all contents from src to dst directory
func (m *Manager) moveDirectoryContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if err := os.Rename(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

// validateNodeInstallation performs comprehensive validation of a Node.js installation
func (m *Manager) validateNodeInstallation(version string) error {
	versionPath := m.GetVersionPath(version)
	nodeExe := m.GetNodeExecutable(version)
	npmExe := m.GetNpmExecutable(version)

	// Check if installation directory exists
	if _, err := os.Stat(versionPath); err != nil {
		return fmt.Errorf("installation directory not found: %w", err)
	}

	// Check if Node.js executable exists and is executable
	if _, err := os.Stat(nodeExe); err != nil {
		return fmt.Errorf("node.js executable not found: %w", err)
	}

	// Check if npm executable exists
	if _, err := os.Stat(npmExe); err != nil {
		return fmt.Errorf("npm executable not found: %w", err)
	}

	// Test Node.js execution
	cmd := exec.Command(nodeExe, "--version")
	nodeOutput, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("node.js execution test failed: %w\nOutput: %s", err, nodeOutput)
	}

	// Test npm execution
	cmd = exec.Command(npmExe, "--version")
	npmOutput, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm execution test failed: %w\nOutput: %s", err, npmOutput)
	}

	return nil
}
