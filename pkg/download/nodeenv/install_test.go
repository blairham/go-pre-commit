package nodeenv

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// TestNodeVersion represents the Node.js version used in tests
	TestNodeVersion = "18.19.0"
)

func TestManager_installMacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Test only runs on macOS")
	}

	manager := NewManager(t.TempDir())
	version := TestNodeVersion
	installPath := filepath.Join(manager.BaseDir, "test-install")

	// This test would require a real tar.gz file, so we'll just test the directory creation
	require.NoError(t, os.MkdirAll(filepath.Dir(installPath), 0o755))

	// Test with non-existent download path
	err := manager.installMacOS(version, "/nonexistent/file.tar.gz", installPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract Node.js")
}

func TestManager_installLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test only runs on Linux")
	}

	manager := NewManager(t.TempDir())
	version := TestNodeVersion
	installPath := filepath.Join(manager.BaseDir, "test-install")

	// This test would require a real tar.xz file, so we'll just test the directory creation
	require.NoError(t, os.MkdirAll(filepath.Dir(installPath), 0o755))

	// Test with non-existent download path
	err := manager.installLinux(version, "/nonexistent/file.tar.xz", installPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract Node.js")
}

func TestManager_installWindows(t *testing.T) {
	if runtime.GOOS != WindowsOS {
		t.Skip("Test only runs on Windows")
	}

	manager := NewManager(t.TempDir())
	version := TestNodeVersion
	installPath := filepath.Join(manager.BaseDir, "test-install")

	// This test would require a real ZIP file, so we'll just test the directory creation
	require.NoError(t, os.MkdirAll(filepath.Dir(installPath), 0o755))

	// Test with non-existent download path
	err := manager.installWindows(version, "/nonexistent/file.zip", installPath)
	assert.Error(t, err)
	// The error might be about missing PowerShell or the file not existing
	assert.Error(t, err)
}

func TestFindExtractedNodeDir(t *testing.T) {
	tempDir := t.TempDir()
	version := TestNodeVersion

	// Create some test directories
	testDirs := []string{
		"node-v18.19.0-darwin-x64",
		"node-18.19.0",
		"some-other-dir",
		"node-test",
	}

	for _, dir := range testDirs {
		require.NoError(t, os.MkdirAll(filepath.Join(tempDir, dir), 0o755))
	}

	// Create a file (should be ignored)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "node-file"), []byte("test"), 0o644))

	dirs, err := findExtractedNodeDir(tempDir, version)
	assert.NoError(t, err)
	assert.Len(t, dirs, 3) // Should find 3 directories that match Node.js patterns

	// Verify the correct directories were found
	foundNames := make([]string, len(dirs))
	for i, dir := range dirs {
		foundNames[i] = filepath.Base(dir)
	}

	assert.Contains(t, foundNames, "node-v18.19.0-darwin-x64")
	assert.Contains(t, foundNames, "node-18.19.0")
	assert.Contains(t, foundNames, "node-test")
	assert.NotContains(t, foundNames, "some-other-dir")
}

func TestManager_setupNodeEnvironment(t *testing.T) {
	manager := NewManager(t.TempDir())
	envPath := filepath.Join(manager.BaseDir, "test-env")
	version := TestNodeVersion

	// Create mock Node.js installation
	nodeExe := manager.GetNodeExecutable(version)
	npmExe := manager.GetNpmExecutable(version)
	require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))
	require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))
	require.NoError(t, os.WriteFile(npmExe, []byte("#!/bin/bash\necho npm"), 0o755))

	// Create environment directory
	require.NoError(t, os.MkdirAll(envPath, 0o755))

	err := manager.setupNodeEnvironment(envPath, version)
	assert.NoError(t, err)

	// Verify activation script was created
	if runtime.GOOS == WindowsOS {
		assert.FileExists(t, filepath.Join(envPath, "activate.bat"))
	} else {
		assert.FileExists(t, filepath.Join(envPath, "activate"))
	}
}

func TestManager_createActivationScripts(t *testing.T) {
	manager := NewManager(t.TempDir())
	envPath := filepath.Join(manager.BaseDir, "test-env")
	version := TestNodeVersion

	// Create mock Node.js installation
	nodeExe := manager.GetNodeExecutable(version)
	npmExe := manager.GetNpmExecutable(version)
	require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))
	require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))
	require.NoError(t, os.WriteFile(npmExe, []byte("#!/bin/bash\necho npm"), 0o755))

	// Create environment directory and bin directory
	require.NoError(t, os.MkdirAll(envPath, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(envPath, "bin"), 0o755))

	err := manager.createActivationScripts(envPath, version)
	assert.NoError(t, err)

	// Check that the appropriate activation script was created
	if runtime.GOOS == WindowsOS {
		activateScript := filepath.Join(envPath, "activate.bat")
		assert.FileExists(t, activateScript)

		content, err := os.ReadFile(activateScript)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "NODE_VERSION="+version)
		assert.Contains(t, string(content), "Activated Node.js")
	} else {
		activateScript := filepath.Join(envPath, "activate")
		assert.FileExists(t, activateScript)

		content, err := os.ReadFile(activateScript)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "NODE_VERSION="+version)
		assert.Contains(t, string(content), "Activated Node.js")

		// Check that script is executable
		info, err := os.Stat(activateScript)
		assert.NoError(t, err)
		assert.True(t, info.Mode()&0o111 != 0) // Check executable bit
	}
}

func TestManager_validateNodeInstallation(t *testing.T) {
	manager := NewManager(t.TempDir())
	version := TestNodeVersion

	t.Run("InvalidInstallation", func(t *testing.T) {
		err := manager.validateNodeInstallation(version)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "installation directory not found")
	})

	t.Run("ValidInstallation", func(t *testing.T) {
		// Create mock installation
		nodeExe := manager.GetNodeExecutable(version)
		npmExe := manager.GetNpmExecutable(version)

		require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))

		// Create executable scripts that will work with the validation
		if runtime.GOOS == WindowsOS {
			nodeScript := "@echo off\necho v18.19.0"
			npmScript := "@echo off\necho 9.2.0"
			require.NoError(t, os.WriteFile(nodeExe, []byte(nodeScript), 0o755))
			require.NoError(t, os.WriteFile(npmExe, []byte(npmScript), 0o755))
		} else {
			nodeScript := "#!/bin/bash\necho v18.19.0"
			npmScript := "#!/bin/bash\necho 9.2.0"
			require.NoError(t, os.WriteFile(nodeExe, []byte(nodeScript), 0o755))
			require.NoError(t, os.WriteFile(npmExe, []byte(npmScript), 0o755))
		}

		err := manager.validateNodeInstallation(version)
		assert.NoError(t, err)
	})

	t.Run("NodeExecutableNotFound", func(t *testing.T) {
		version := "19.0.0"

		// Create version directory but no node executable
		versionPath := manager.GetVersionPath(version)
		require.NoError(t, os.MkdirAll(versionPath, 0o755))

		err := manager.validateNodeInstallation(version)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "node.js executable not found")
	})

	t.Run("NpmExecutableNotFound", func(t *testing.T) {
		version := "19.1.0"

		// Create node executable but not npm
		nodeExe := manager.GetNodeExecutable(version)
		require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))
		require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))

		err := manager.validateNodeInstallation(version)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "npm executable not found")
	})
}

func TestManager_createUnixExecutables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Test only runs on Unix-like systems")
	}

	manager := NewManager(t.TempDir())
	envPath := filepath.Join(manager.BaseDir, "test-env")
	binDir := filepath.Join(envPath, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	// Create mock source executables
	nodeExe := filepath.Join(manager.BaseDir, "node")
	npmExe := filepath.Join(manager.BaseDir, "npm")
	require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))
	require.NoError(t, os.WriteFile(npmExe, []byte("#!/bin/bash\necho npm"), 0o755))

	// Create target paths
	envNodeExe := filepath.Join(binDir, "node")
	envNpmExe := filepath.Join(binDir, "npm")

	err := manager.createUnixExecutables(nodeExe, npmExe, envNodeExe, envNpmExe)
	assert.NoError(t, err)

	// Verify symlinks were created
	_, err = os.Lstat(envNodeExe)
	assert.NoError(t, err)
	_, err = os.Lstat(envNpmExe)
	assert.NoError(t, err)
}

func TestManager_createWindowsExecutables(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Test only runs on Windows")
	}

	manager := NewManager(t.TempDir())
	envPath := filepath.Join(manager.BaseDir, "test-env")
	binDir := filepath.Join(envPath, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))

	// Create mock source executables
	nodeExe := filepath.Join(manager.BaseDir, "node.exe")
	npmExe := filepath.Join(manager.BaseDir, "npm.cmd")

	// Create target paths
	envNodeExe := filepath.Join(binDir, "node")
	envNpmExe := filepath.Join(binDir, "npm")

	err := manager.createWindowsExecutables(nodeExe, npmExe, envNodeExe, envNpmExe)
	assert.NoError(t, err)

	// Verify batch files were created
	assert.FileExists(t, envNodeExe+".bat")
	assert.FileExists(t, envNpmExe+".bat")

	// Verify batch file content
	nodeContent, err := os.ReadFile(envNodeExe + ".bat")
	assert.NoError(t, err)
	assert.Contains(t, string(nodeContent), nodeExe)

	npmContent, err := os.ReadFile(envNpmExe + ".bat")
	assert.NoError(t, err)
	assert.Contains(t, string(npmContent), npmExe)
}
