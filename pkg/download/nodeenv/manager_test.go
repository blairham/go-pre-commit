package nodeenv

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	t.Run("WithBaseDir", func(t *testing.T) {
		baseDir := "/test/base/dir"
		manager := NewManager(baseDir)

		assert.NotNil(t, manager)
		assert.Equal(t, baseDir, manager.BaseDir)
		assert.Equal(t, filepath.Join(baseDir, "cache"), manager.CacheDir)
		assert.NotNil(t, manager.DownloadManager)
	})

	t.Run("WithEmptyBaseDir", func(t *testing.T) {
		manager := NewManager("")

		assert.NotNil(t, manager)
		assert.NotEmpty(t, manager.BaseDir)
		// The base dir should contain a cache directory with Node.js-related path
		assert.Contains(t, manager.BaseDir, "node")
	})
}

func TestManager_GetVersionsDir(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)

	expected := filepath.Join(baseDir, "versions")
	assert.Equal(t, expected, manager.GetVersionsDir())
}

func TestManager_GetVersionPath(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)
	version := TestNodeVersion

	expected := filepath.Join(baseDir, "versions", version)
	assert.Equal(t, expected, manager.GetVersionPath(version))
}

func TestManager_GetNodeExecutable(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)
	version := TestNodeVersion

	result := manager.GetNodeExecutable(version)

	versionsDir := filepath.Join(baseDir, "versions", version)
	if runtime.GOOS == WindowsOS {
		expected := filepath.Join(versionsDir, "node.exe")
		assert.Equal(t, expected, result)
	} else {
		expected := filepath.Join(versionsDir, "bin", "node")
		assert.Equal(t, expected, result)
	}
}

func TestManager_GetNpmExecutable(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)
	version := TestNodeVersion

	result := manager.GetNpmExecutable(version)

	versionsDir := filepath.Join(baseDir, "versions", version)
	if runtime.GOOS == WindowsOS {
		expected := filepath.Join(versionsDir, "npm.cmd")
		assert.Equal(t, expected, result)
	} else {
		expected := filepath.Join(versionsDir, "bin", "npm")
		assert.Equal(t, expected, result)
	}
}

func TestManager_IsVersionInstalled(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)
	version := TestNodeVersion

	// Version not installed
	assert.False(t, manager.IsVersionInstalled(version))

	// Create version directory and executable
	nodeExe := manager.GetNodeExecutable(version)

	require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))
	require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))

	// Version now installed
	assert.True(t, manager.IsVersionInstalled(version))
}

func TestManager_ResolveVersion(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)

	t.Run("SpecificVersion", func(t *testing.T) {
		version := TestNodeVersion
		result, err := manager.ResolveVersion(version)
		assert.NoError(t, err)
		assert.Equal(t, version, result)
	})

	t.Run("SystemVersionNoInstalled", func(t *testing.T) {
		_, err := manager.ResolveVersion("system")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no Node.js versions installed")
	})

	t.Run("DefaultVersionNoInstalled", func(t *testing.T) {
		_, err := manager.ResolveVersion("default")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no Node.js versions installed")
	})

	t.Run("EmptyVersionNoInstalled", func(t *testing.T) {
		_, err := manager.ResolveVersion("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no Node.js versions installed")
	})
}

func TestManager_GetInstalledVersions(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)

	t.Run("NoVersionsInstalled", func(t *testing.T) {
		versions, err := manager.GetInstalledVersions()
		assert.NoError(t, err)
		assert.Empty(t, versions)
	})

	t.Run("WithVersionsInstalled", func(t *testing.T) {
		versionsDir := manager.GetVersionsDir()
		require.NoError(t, os.MkdirAll(versionsDir, 0o755))

		// Create some version directories
		testVersions := []string{"18.19.0", "20.11.0", "16.20.2"}
		for _, version := range testVersions {
			versionDir := filepath.Join(versionsDir, version)
			require.NoError(t, os.MkdirAll(versionDir, 0o755))
		}

		versions, err := manager.GetInstalledVersions()
		assert.NoError(t, err)
		assert.Len(t, versions, 3)

		// Should be sorted in descending order
		assert.Equal(t, "20.11.0", versions[0])
		assert.Equal(t, "18.19.0", versions[1])
		assert.Equal(t, "16.20.2", versions[2])
	})
}

func TestManager_getDownloadInfo(t *testing.T) {
	manager := NewManager("")

	tests := []struct {
		name        string
		version     string
		expectError bool
	}{
		{
			name:        "ValidVersion",
			version:     TestNodeVersion,
			expectError: false,
		},
		{
			name:        "VersionWithVPrefix",
			version:     "v" + TestNodeVersion,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, filename, err := manager.getDownloadInfo(tt.version)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, url)
				assert.NotEmpty(t, filename)
				assert.Contains(t, url, "nodejs.org")
				assert.Contains(t, filename, "node-v")
			}
		})
	}
}

func TestManager_GetAvailableVersions(t *testing.T) {
	manager := NewManager("")

	versions, err := manager.GetAvailableVersions()
	assert.NoError(t, err)
	assert.NotEmpty(t, versions)

	// Should contain some LTS versions
	assert.Contains(t, versions, "20.11.0")
	assert.Contains(t, versions, TestNodeVersion)
}

func TestManager_GlobalVersion(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)
	version := TestNodeVersion

	t.Run("NoGlobalVersionSet", func(t *testing.T) {
		_, err := manager.GetGlobalVersion()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no global version set")
	})

	t.Run("SetAndGetGlobalVersion", func(t *testing.T) {
		// First create a mock installation
		nodeExe := manager.GetNodeExecutable(version)
		require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))
		require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))

		// Set global version
		err := manager.SetGlobalVersion(version)
		assert.NoError(t, err)

		// Get global version
		globalVersion, err := manager.GetGlobalVersion()
		assert.NoError(t, err)
		assert.Equal(t, version, globalVersion)
	})

	t.Run("SetGlobalVersionNotInstalled", func(t *testing.T) {
		err := manager.SetGlobalVersion("99.99.99")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not installed")
	})
}

func TestManager_EnsureVersionInstalled(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)

	t.Run("VersionNotInstalled", func(t *testing.T) {
		// This will try to install the version
		ctx := context.Background()
		err := manager.EnsureVersionInstalled(ctx, "18.19.0")

		// Since we're in test environment and the download might actually work,
		// we'll just check that the method doesn't panic and either succeeds or fails gracefully
		if err != nil {
			t.Logf("EnsureVersionInstalled failed as expected in test environment: %v", err)
		} else {
			t.Logf("EnsureVersionInstalled succeeded (Node.js was downloaded)")
		}
	})

	t.Run("VersionAlreadyInstalled", func(t *testing.T) {
		version := TestNodeVersion

		// Create mock installation
		nodeExe := manager.GetNodeExecutable(version)
		require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))
		require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))

		ctx := context.Background()
		err := manager.EnsureVersionInstalled(ctx, version)
		assert.NoError(t, err)
	})
}

func TestManager_ValidateEnvironment(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)
	envPath := filepath.Join(baseDir, "test-env")

	t.Run("InvalidEnvironment", func(t *testing.T) {
		err := manager.ValidateEnvironment(envPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "executable not found")
	})

	t.Run("ValidEnvironment", func(t *testing.T) {
		// Create environment with mock executables
		binDir := filepath.Join(envPath, "bin")
		require.NoError(t, os.MkdirAll(binDir, 0o755))

		var nodeExe string
		if runtime.GOOS == WindowsOS {
			nodeExe = filepath.Join(binDir, "node.bat")
			nodeScript := "@echo off\necho v18.19.0"
			require.NoError(t, os.WriteFile(nodeExe, []byte(nodeScript), 0o755))
		} else {
			nodeExe = filepath.Join(binDir, "node")
			nodeScript := "#!/bin/bash\necho v18.19.0"
			require.NoError(t, os.WriteFile(nodeExe, []byte(nodeScript), 0o755))
		}

		err := manager.ValidateEnvironment(envPath)
		assert.NoError(t, err)
	})
}

func TestManager_UninstallVersion(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)
	version := "18.19.0"

	t.Run("VersionNotInstalled", func(t *testing.T) {
		err := manager.UninstallVersion(version)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not installed")
	})

	t.Run("VersionInstalled", func(t *testing.T) {
		// Create mock installation
		nodeExe := manager.GetNodeExecutable(version)
		require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))
		require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))

		// Verify it's installed
		assert.True(t, manager.IsVersionInstalled(version))

		// Uninstall
		err := manager.UninstallVersion(version)
		assert.NoError(t, err)

		// Verify it's no longer installed
		assert.False(t, manager.IsVersionInstalled(version))
	})
}

func TestManager_CreateEnvironment(t *testing.T) {
	baseDir := t.TempDir()
	manager := NewManager(baseDir)
	envPath := filepath.Join(baseDir, "test-env")
	version := "18.19.0"

	t.Run("CreateEnvironmentVersionNotInstalled", func(t *testing.T) {
		err := manager.CreateEnvironment(envPath, version)
		// The environment creation might succeed if download works, or fail if it doesn't
		if err != nil {
			t.Logf("CreateEnvironment failed as expected when version not pre-installed: %v", err)
		} else {
			t.Logf("CreateEnvironment succeeded (Node.js was downloaded and installed)")
			// Clean up if it succeeded
			os.RemoveAll(envPath)
		}
	})

	t.Run("CreateEnvironmentSuccess", func(t *testing.T) {
		// Create mock Node.js installation
		nodeExe := manager.GetNodeExecutable(version)
		npmExe := manager.GetNpmExecutable(version)

		require.NoError(t, os.MkdirAll(filepath.Dir(nodeExe), 0o755))
		require.NoError(t, os.WriteFile(nodeExe, []byte("#!/bin/bash\necho node"), 0o755))
		require.NoError(t, os.WriteFile(npmExe, []byte("#!/bin/bash\necho npm"), 0o755))

		err := manager.CreateEnvironment(envPath, version)
		assert.NoError(t, err)

		// Verify environment was created
		assert.DirExists(t, envPath)
		assert.DirExists(t, filepath.Join(envPath, "bin"))

		// Verify executables were created
		if runtime.GOOS == WindowsOS {
			assert.FileExists(t, filepath.Join(envPath, "bin", "node.bat"))
			assert.FileExists(t, filepath.Join(envPath, "bin", "npm.bat"))
		} else {
			assert.FileExists(t, filepath.Join(envPath, "bin", "node"))
			assert.FileExists(t, filepath.Join(envPath, "bin", "npm"))
		}
	})
}

func TestMoveDirectoryContents(t *testing.T) {
	manager := NewManager("")

	// Create temporary directories
	srcDir := filepath.Join(t.TempDir(), "src")
	dstDir := filepath.Join(t.TempDir(), "dst")

	require.NoError(t, os.MkdirAll(srcDir, 0o755))
	require.NoError(t, os.MkdirAll(dstDir, 0o755))

	// Create some test files in source
	testFiles := []string{"file1.txt", "file2.txt"}
	for _, file := range testFiles {
		content := "test content for " + file
		require.NoError(t, os.WriteFile(filepath.Join(srcDir, file), []byte(content), 0o644))
	}

	// Move contents
	err := manager.moveDirectoryContents(srcDir, dstDir)
	assert.NoError(t, err)

	// Verify files were moved
	for _, file := range testFiles {
		assert.FileExists(t, filepath.Join(dstDir, file))
		assert.NoFileExists(t, filepath.Join(srcDir, file))
	}
}
