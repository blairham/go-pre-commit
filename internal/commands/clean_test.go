package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanCommand_Synopsis(t *testing.T) {
	cmd := &CleanCommand{}
	synopsis := cmd.Synopsis()

	assert.NotEmpty(t, synopsis)
	assert.Contains(t, strings.ToLower(synopsis), "clean")
}

func TestCleanCommand_Help(t *testing.T) {
	cmd := &CleanCommand{}
	help := cmd.Help()

	assert.NotEmpty(t, help)
	assert.Contains(t, help, "--help")
	assert.Contains(t, help, "--color")
}

func TestCleanCommandFactory(t *testing.T) {
	cmd, err := CleanCommandFactory()

	require.NoError(t, err)
	assert.NotNil(t, cmd)

	_, ok := cmd.(*CleanCommand)
	assert.True(t, ok, "Factory should return *CleanCommand")
}

func TestCleanCommand_Run_CleansCacheDirectory(t *testing.T) {
	t.Run("cleans existing cache directory", func(t *testing.T) {
		// Create a temporary cache directory
		tmpDir := t.TempDir()
		cacheDir := filepath.Join(tmpDir, "cache", "pre-commit")
		err := os.MkdirAll(cacheDir, 0o755)
		require.NoError(t, err)

		// Create some files in the cache
		testFile := filepath.Join(cacheDir, "test-repo")
		err = os.WriteFile(testFile, []byte("test"), 0o600)
		require.NoError(t, err)

		// Set PRE_COMMIT_HOME to our test directory
		oldEnv := os.Getenv("PRE_COMMIT_HOME")
		os.Setenv("PRE_COMMIT_HOME", cacheDir)
		defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

		// Also set HOME to prevent legacy cleanup from affecting the test
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", oldHome)

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd := &CleanCommand{}
		exitCode := cmd.Run([]string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, output, "Cleaned")
		assert.Contains(t, output, cacheDir)

		// Verify directory was cleaned
		_, err = os.Stat(cacheDir)
		assert.True(t, os.IsNotExist(err), "Cache directory should be removed")
	})

	t.Run("handles non-existent cache directory gracefully", func(t *testing.T) {
		// Create a temporary directory that doesn't exist
		tmpDir := t.TempDir()
		cacheDir := filepath.Join(tmpDir, "nonexistent", "cache")

		// Set PRE_COMMIT_HOME to non-existent directory
		oldEnv := os.Getenv("PRE_COMMIT_HOME")
		os.Setenv("PRE_COMMIT_HOME", cacheDir)
		defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

		// Set HOME to prevent legacy cleanup issues
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", oldHome)

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd := &CleanCommand{}
		exitCode := cmd.Run([]string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)

		// Should still succeed even if directory doesn't exist
		assert.Equal(t, 0, exitCode)
	})
}

func TestCleanCommand_Run_CleansLegacyDirectory(t *testing.T) {
	t.Run("cleans existing legacy directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create legacy directory
		legacyDir := filepath.Join(tmpDir, ".pre-commit")
		err := os.MkdirAll(legacyDir, 0o755)
		require.NoError(t, err)

		// Create a file in legacy dir
		testFile := filepath.Join(legacyDir, "legacy-file")
		err = os.WriteFile(testFile, []byte("legacy"), 0o600)
		require.NoError(t, err)

		// Set up environment
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", oldHome)

		// Use a cache dir that exists
		cacheDir := filepath.Join(tmpDir, "cache")
		err = os.MkdirAll(cacheDir, 0o755)
		require.NoError(t, err)

		oldEnv := os.Getenv("PRE_COMMIT_HOME")
		os.Setenv("PRE_COMMIT_HOME", cacheDir)
		defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd := &CleanCommand{}
		exitCode := cmd.Run([]string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, output, "Cleaned")

		// Verify legacy directory was cleaned
		_, err = os.Stat(legacyDir)
		assert.True(t, os.IsNotExist(err), "Legacy directory should be removed")
	})

	t.Run("handles missing legacy directory gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Don't create legacy directory - it shouldn't exist

		// Set up environment
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", oldHome)

		cacheDir := filepath.Join(tmpDir, "cache")
		err := os.MkdirAll(cacheDir, 0o755)
		require.NoError(t, err)

		oldEnv := os.Getenv("PRE_COMMIT_HOME")
		os.Setenv("PRE_COMMIT_HOME", cacheDir)
		defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

		cmd := &CleanCommand{}
		exitCode := cmd.Run([]string{})

		// Should succeed without legacy directory
		assert.Equal(t, 0, exitCode)
	})
}

func TestCleanCommand_Run_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	// Set up environment
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	cacheDir := filepath.Join(tmpDir, "cache")
	err := os.MkdirAll(cacheDir, 0o755)
	require.NoError(t, err)

	oldEnv := os.Getenv("PRE_COMMIT_HOME")
	os.Setenv("PRE_COMMIT_HOME", cacheDir)
	defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

	cmd := &CleanCommand{}

	// Suppress output for multiple runs
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	// Run multiple times - should not fail
	exitCode1 := cmd.Run([]string{})
	exitCode2 := cmd.Run([]string{})
	exitCode3 := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	assert.Equal(t, 0, exitCode1, "First run should succeed")
	assert.Equal(t, 0, exitCode2, "Second run should succeed")
	assert.Equal(t, 0, exitCode3, "Third run should succeed")
}

func TestCleanCommand_getCacheDirectory(t *testing.T) {
	t.Run("uses PRE_COMMIT_HOME when set", func(t *testing.T) {
		// Save original values
		oldPreCommitHome := os.Getenv("PRE_COMMIT_HOME")
		oldXdgCache := os.Getenv("XDG_CACHE_HOME")

		// Set PRE_COMMIT_HOME
		testPath := "/custom/pre-commit/home"
		os.Setenv("PRE_COMMIT_HOME", testPath)
		os.Unsetenv("XDG_CACHE_HOME")

		defer func() {
			if oldPreCommitHome != "" {
				os.Setenv("PRE_COMMIT_HOME", oldPreCommitHome)
			} else {
				os.Unsetenv("PRE_COMMIT_HOME")
			}
			if oldXdgCache != "" {
				os.Setenv("XDG_CACHE_HOME", oldXdgCache)
			}
		}()

		result := getCacheDirectory()
		assert.Equal(t, testPath, result)
	})

	t.Run("uses XDG_CACHE_HOME when PRE_COMMIT_HOME not set", func(t *testing.T) {
		// Save original values
		oldPreCommitHome := os.Getenv("PRE_COMMIT_HOME")
		oldXdgCache := os.Getenv("XDG_CACHE_HOME")

		// Clear PRE_COMMIT_HOME, set XDG_CACHE_HOME
		os.Unsetenv("PRE_COMMIT_HOME")
		testPath := "/custom/xdg/cache"
		os.Setenv("XDG_CACHE_HOME", testPath)

		defer func() {
			if oldPreCommitHome != "" {
				os.Setenv("PRE_COMMIT_HOME", oldPreCommitHome)
			}
			if oldXdgCache != "" {
				os.Setenv("XDG_CACHE_HOME", oldXdgCache)
			} else {
				os.Unsetenv("XDG_CACHE_HOME")
			}
		}()

		result := getCacheDirectory()
		assert.Equal(t, filepath.Join(testPath, "pre-commit"), result)
	})

	t.Run("uses default when no env vars set", func(t *testing.T) {
		// Save original values
		oldPreCommitHome := os.Getenv("PRE_COMMIT_HOME")
		oldXdgCache := os.Getenv("XDG_CACHE_HOME")

		// Clear both environment variables
		os.Unsetenv("PRE_COMMIT_HOME")
		os.Unsetenv("XDG_CACHE_HOME")

		defer func() {
			if oldPreCommitHome != "" {
				os.Setenv("PRE_COMMIT_HOME", oldPreCommitHome)
			}
			if oldXdgCache != "" {
				os.Setenv("XDG_CACHE_HOME", oldXdgCache)
			}
		}()

		result := getCacheDirectory()

		// Should be ~/.cache/pre-commit or fallback
		homeDir, err := os.UserHomeDir()
		if err == nil {
			expected := filepath.Join(homeDir, ".cache", "pre-commit")
			assert.Equal(t, expected, result)
		} else {
			// Fallback case
			assert.Contains(t, result, "pre-commit")
		}
	})

	t.Run("PRE_COMMIT_HOME takes priority over XDG_CACHE_HOME", func(t *testing.T) {
		// Save original values
		oldPreCommitHome := os.Getenv("PRE_COMMIT_HOME")
		oldXdgCache := os.Getenv("XDG_CACHE_HOME")

		// Set both
		preCommitPath := "/pre-commit/priority"
		xdgPath := "/xdg/path"
		os.Setenv("PRE_COMMIT_HOME", preCommitPath)
		os.Setenv("XDG_CACHE_HOME", xdgPath)

		defer func() {
			if oldPreCommitHome != "" {
				os.Setenv("PRE_COMMIT_HOME", oldPreCommitHome)
			} else {
				os.Unsetenv("PRE_COMMIT_HOME")
			}
			if oldXdgCache != "" {
				os.Setenv("XDG_CACHE_HOME", oldXdgCache)
			} else {
				os.Unsetenv("XDG_CACHE_HOME")
			}
		}()

		result := getCacheDirectory()
		assert.Equal(t, preCommitPath, result, "PRE_COMMIT_HOME should take priority")
	})
}

func TestCleanCommand_Run_HelpFlag(t *testing.T) {
	t.Run("--help shows help and returns 0", func(t *testing.T) {
		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd := &CleanCommand{}
		exitCode := cmd.Run([]string{"--help"})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, output, "--color")
	})

	t.Run("-h shows help and returns 0", func(t *testing.T) {
		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd := &CleanCommand{}
		exitCode := cmd.Run([]string{"-h"})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		assert.Equal(t, 0, exitCode)
		assert.Contains(t, output, "--color")
	})
}

func TestCleanCommand_Run_ColorOption(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "auto color",
			args:     []string{"--color", "auto"},
			wantCode: 0,
		},
		{
			name:     "always color",
			args:     []string{"--color", "always"},
			wantCode: 0,
		},
		{
			name:     "never color",
			args:     []string{"--color", "never"},
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Set up environment
			oldHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpDir)
			defer os.Setenv("HOME", oldHome)

			cacheDir := filepath.Join(tmpDir, "cache")
			err := os.MkdirAll(cacheDir, 0o755)
			require.NoError(t, err)

			oldEnv := os.Getenv("PRE_COMMIT_HOME")
			os.Setenv("PRE_COMMIT_HOME", cacheDir)
			defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

			// Suppress output
			oldStdout := os.Stdout
			_, w, _ := os.Pipe()
			os.Stdout = w

			cmd := &CleanCommand{}
			exitCode := cmd.Run(tt.args)

			w.Close()
			os.Stdout = oldStdout

			assert.Equal(t, tt.wantCode, exitCode)
		})
	}
}

func TestCleanCommand_Run_InvalidColorOption(t *testing.T) {
	// Capture stderr for error output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &CleanCommand{}
	exitCode := cmd.Run([]string{"--color", "invalid"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	// Should fail with invalid color option
	assert.Equal(t, 1, exitCode)
}

func TestCleanCommand_CleansBothDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both cache and legacy directories
	cacheDir := filepath.Join(tmpDir, "cache", "pre-commit")
	legacyDir := filepath.Join(tmpDir, ".pre-commit")

	err := os.MkdirAll(cacheDir, 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(legacyDir, 0o755)
	require.NoError(t, err)

	// Add files to both
	err = os.WriteFile(filepath.Join(cacheDir, "repo1"), []byte("cache"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(legacyDir, "old-repo"), []byte("legacy"), 0o600)
	require.NoError(t, err)

	// Verify both exist
	assert.DirExists(t, cacheDir)
	assert.DirExists(t, legacyDir)

	// Set up environment
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldEnv := os.Getenv("PRE_COMMIT_HOME")
	os.Setenv("PRE_COMMIT_HOME", cacheDir)
	defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &CleanCommand{}
	exitCode := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should succeed
	assert.Equal(t, 0, exitCode)

	// Should have cleaned both directories
	_, err = os.Stat(cacheDir)
	assert.True(t, os.IsNotExist(err), "Cache directory should be removed")

	_, err = os.Stat(legacyDir)
	assert.True(t, os.IsNotExist(err), "Legacy directory should be removed")

	// Output should mention both
	assert.Contains(t, output, "Cleaned")
}

func TestCleanCommand_OutputFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache directory
	cacheDir := filepath.Join(tmpDir, "cache")
	err := os.MkdirAll(cacheDir, 0o755)
	require.NoError(t, err)

	// Set up environment
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldEnv := os.Getenv("PRE_COMMIT_HOME")
	os.Setenv("PRE_COMMIT_HOME", cacheDir)
	defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &CleanCommand{}
	cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Output should match format "Cleaned {path}."
	assert.Contains(t, output, "Cleaned ")
	assert.Contains(t, output, ".\n")
}

func TestCleanCommand_NestedCacheContents(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a cache directory with nested structure (like real pre-commit cache)
	cacheDir := filepath.Join(tmpDir, "cache")
	repoDir := filepath.Join(cacheDir, "repos", "git@github.com_user_repo")
	envDir := filepath.Join(cacheDir, "environments", "python-3.9")

	err := os.MkdirAll(repoDir, 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(envDir, 0o755)
	require.NoError(t, err)

	// Add some files
	err = os.WriteFile(filepath.Join(repoDir, ".pre-commit-hooks.yaml"), []byte("hooks"), 0o600)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(envDir, "bin", "python"), []byte("python"), 0o755)

	// Set up environment
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldEnv := os.Getenv("PRE_COMMIT_HOME")
	os.Setenv("PRE_COMMIT_HOME", cacheDir)
	defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

	// Suppress output
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &CleanCommand{}
	exitCode := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	assert.Equal(t, 0, exitCode)

	// Verify entire cache structure is removed
	_, err = os.Stat(cacheDir)
	assert.True(t, os.IsNotExist(err), "Cache directory and all contents should be removed")
}

// Test: Output consistency - only prints when directory exists (matches Python behavior)
func TestCleanCommand_OutputOnlyWhenDirectoryExists(t *testing.T) {
	t.Run("no output when directories don't exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Set up environment with non-existent directories
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", oldHome)

		// Point to non-existent cache dir
		cacheDir := filepath.Join(tmpDir, "nonexistent", "cache")
		oldEnv := os.Getenv("PRE_COMMIT_HOME")
		os.Setenv("PRE_COMMIT_HOME", cacheDir)
		defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd := &CleanCommand{}
		exitCode := cmd.Run([]string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		assert.Equal(t, 0, exitCode)
		// Should have no "Cleaned" output since no directories exist
		assert.NotContains(t, output, "Cleaned", "Should not print 'Cleaned' when directory doesn't exist")
	})

	t.Run("output only for existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create only the cache directory (not legacy)
		cacheDir := filepath.Join(tmpDir, "cache")
		err := os.MkdirAll(cacheDir, 0o755)
		require.NoError(t, err)

		// Set up environment
		oldHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", oldHome)

		oldEnv := os.Getenv("PRE_COMMIT_HOME")
		os.Setenv("PRE_COMMIT_HOME", cacheDir)
		defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		cmd := &CleanCommand{}
		exitCode := cmd.Run([]string{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		assert.Equal(t, 0, exitCode)
		// Should only have one "Cleaned" line (for cache dir, not legacy)
		assert.Equal(t, 1, strings.Count(output, "Cleaned"), "Should only print 'Cleaned' once for the existing directory")
		assert.Contains(t, output, cacheDir, "Output should mention the cache directory")
	})
}

// Test: Symlink resolution - getCacheDirectory resolves symlinks like Python's realpath()
func TestCleanCommand_getCacheDirectory_SymlinkResolution(t *testing.T) {
	t.Run("resolves symlink in PRE_COMMIT_HOME", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create actual directory
		actualDir := filepath.Join(tmpDir, "actual-cache")
		err := os.MkdirAll(actualDir, 0o755)
		require.NoError(t, err)

		// Create symlink pointing to actual directory
		symlinkDir := filepath.Join(tmpDir, "symlink-cache")
		err = os.Symlink(actualDir, symlinkDir)
		require.NoError(t, err)

		// Set PRE_COMMIT_HOME to symlink
		oldEnv := os.Getenv("PRE_COMMIT_HOME")
		os.Setenv("PRE_COMMIT_HOME", symlinkDir)
		defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

		result := getCacheDirectory()

		// Resolve both paths to handle OS-level symlinks (e.g., /var -> /private/var on macOS)
		expectedResolved, _ := filepath.EvalSymlinks(actualDir)
		if expectedResolved == "" {
			expectedResolved = actualDir
		}

		// Should resolve to actual path (accounting for OS symlinks)
		assert.Equal(t, expectedResolved, result, "Should resolve symlink to actual path")
	})

	t.Run("resolves symlink in XDG_CACHE_HOME", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create actual directory
		actualXdgDir := filepath.Join(tmpDir, "actual-xdg")
		err := os.MkdirAll(actualXdgDir, 0o755)
		require.NoError(t, err)

		// Create symlink
		symlinkXdgDir := filepath.Join(tmpDir, "symlink-xdg")
		err = os.Symlink(actualXdgDir, symlinkXdgDir)
		require.NoError(t, err)

		// Set up environment
		oldPreCommit := os.Getenv("PRE_COMMIT_HOME")
		os.Unsetenv("PRE_COMMIT_HOME")
		defer func() {
			if oldPreCommit != "" {
				os.Setenv("PRE_COMMIT_HOME", oldPreCommit)
			}
		}()

		oldXdg := os.Getenv("XDG_CACHE_HOME")
		os.Setenv("XDG_CACHE_HOME", symlinkXdgDir)
		defer func() {
			if oldXdg != "" {
				os.Setenv("XDG_CACHE_HOME", oldXdg)
			} else {
				os.Unsetenv("XDG_CACHE_HOME")
			}
		}()

		result := getCacheDirectory()

		// Resolve the expected path to handle OS-level symlinks
		expectedResolved, _ := filepath.EvalSymlinks(actualXdgDir)
		if expectedResolved == "" {
			expectedResolved = actualXdgDir
		}
		expected := filepath.Join(expectedResolved, "pre-commit")

		assert.Equal(t, expected, result, "Should resolve XDG symlink to actual path")
	})

	t.Run("handles non-existent path gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Set PRE_COMMIT_HOME to non-existent path
		nonExistentPath := filepath.Join(tmpDir, "does", "not", "exist")
		oldEnv := os.Getenv("PRE_COMMIT_HOME")
		os.Setenv("PRE_COMMIT_HOME", nonExistentPath)
		defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

		result := getCacheDirectory()

		// Should return the path even if it doesn't exist (parent might be resolvable)
		assert.NotEmpty(t, result)
		// The result should be based on the non-existent path
		assert.Contains(t, result, "exist")
	})
}

// Test: Clean resolves symlinks before cleaning
func TestCleanCommand_CleansSymlinkedDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create actual cache directory with content
	actualCacheDir := filepath.Join(tmpDir, "actual-cache")
	err := os.MkdirAll(actualCacheDir, 0o755)
	require.NoError(t, err)

	// Add a file to the cache
	testFile := filepath.Join(actualCacheDir, "test-repo")
	err = os.WriteFile(testFile, []byte("test"), 0o600)
	require.NoError(t, err)

	// Create symlink to the cache directory
	symlinkCacheDir := filepath.Join(tmpDir, "symlink-cache")
	err = os.Symlink(actualCacheDir, symlinkCacheDir)
	require.NoError(t, err)

	// Verify symlink and actual dir exist
	assert.FileExists(t, testFile)

	// Set up environment - point PRE_COMMIT_HOME to symlink
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldEnv := os.Getenv("PRE_COMMIT_HOME")
	os.Setenv("PRE_COMMIT_HOME", symlinkCacheDir)
	defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

	// Suppress output
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &CleanCommand{}
	exitCode := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	assert.Equal(t, 0, exitCode)

	// The actual directory should be cleaned
	_, err = os.Stat(actualCacheDir)
	assert.True(t, os.IsNotExist(err), "Actual directory should be removed")
}

// Test: Python behavior parity - exact output format
func TestCleanCommand_PythonParityOutputFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both directories
	cacheDir := filepath.Join(tmpDir, "cache")
	legacyDir := filepath.Join(tmpDir, ".pre-commit")

	err := os.MkdirAll(cacheDir, 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(legacyDir, 0o755)
	require.NoError(t, err)

	// Set up environment
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", oldHome)

	oldEnv := os.Getenv("PRE_COMMIT_HOME")
	os.Setenv("PRE_COMMIT_HOME", cacheDir)
	defer os.Setenv("PRE_COMMIT_HOME", oldEnv)

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &CleanCommand{}
	exitCode := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Equal(t, 0, exitCode)

	// Python output format: "Cleaned {path}.\n" for each directory
	// Should have exactly two lines (one for each directory)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Len(t, lines, 2, "Should have two output lines for two directories")

	// Each line should match the format "Cleaned {path}."
	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, "Cleaned "), "Line should start with 'Cleaned '")
		assert.True(t, strings.HasSuffix(line, "."), "Line should end with '.'")
	}

	// First line should be cache dir, second should be legacy dir (same order as Python)
	assert.Contains(t, lines[0], cacheDir, "First line should mention cache directory")
	assert.Contains(t, lines[1], legacyDir, "Second line should mention legacy directory")
}
