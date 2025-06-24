package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestMultiVersionPythonCacheCompatibility(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Test repository configurations with different Python versions
	repos := []config.Repo{
		{
			Repo: "https://github.com/psf/black",
			Rev:  "23.7.0",
			Hooks: []config.Hook{
				{
					ID:              "black",
					LanguageVersion: "3.8",
				},
			},
		},
		{
			Repo: "https://github.com/pycqa/flake8",
			Rev:  "6.0.0",
			Hooks: []config.Hook{
				{
					ID: "flake8",
					// No version - should use default_language_version
				},
			},
		},
		{
			Repo: "https://github.com/pre-commit/pre-commit-hooks",
			Rev:  "v4.4.0",
			Hooks: []config.Hook{
				{
					ID:              "check-yaml",
					LanguageVersion: "system",
				},
			},
		},
	}

	// Test that each repository gets a unique path
	repoPaths := make(map[string]string)

	for _, repo := range repos {
		repoPath := manager.GetRepoPath(repo)

		// Verify path is unique
		for otherURL, otherPath := range repoPaths {
			if repo.Repo != otherURL && repoPath == otherPath {
				t.Errorf("Repositories %s and %s have the same path: %s", repo.Repo, otherURL, repoPath)
			}
		}

		repoPaths[repo.Repo] = repoPath

		// Verify path is in cache directory
		assert.True(t, strings.HasPrefix(repoPath, tempDir),
			"Repository path should be in cache directory")

		t.Logf("Repository %s -> %s", repo.Repo, repoPath)
	}
}

func TestCacheIntegrityWithMultiVersionSupport(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Test repositories with mixed version requirements
	testRepos := []struct {
		name string
		repo config.Repo
	}{
		{
			name: "Black with Python 3.8",
			repo: config.Repo{
				Repo: "https://github.com/psf/black",
				Rev:  "23.7.0",
				Hooks: []config.Hook{
					{
						ID:              "black",
						LanguageVersion: "3.8",
					},
				},
			},
		},
		{
			name: "Black with Python 3.10",
			repo: config.Repo{
				Repo: "https://github.com/psf/black",
				Rev:  "23.7.0",
				Hooks: []config.Hook{
					{
						ID:              "black",
						LanguageVersion: "3.10",
					},
				},
			},
		},
		{
			name: "Flake8 with system Python",
			repo: config.Repo{
				Repo: "https://github.com/pycqa/flake8",
				Rev:  "6.0.0",
				Hooks: []config.Hook{
					{
						ID:              "flake8",
						LanguageVersion: "system",
					},
				},
			},
		},
	}

	// Process each repository and verify cache behavior
	for _, testRepo := range testRepos {
		t.Run(testRepo.name, func(t *testing.T) {
			// Get repository path
			repoPath := manager.GetRepoPath(testRepo.repo)
			require.NotEmpty(t, repoPath)

			// Update repository entry in cache
			err := manager.UpdateRepoEntry(testRepo.repo, repoPath)
			require.NoError(t, err)

			// Verify the path is within cache directory
			assert.True(t, strings.HasPrefix(repoPath, tempDir),
				"Repository path should be in cache directory")

			t.Logf("Repository %s cached at: %s", testRepo.name, repoPath)
		})
	}
}

func TestCacheConfigurationTracking(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Test configuration files with multi-version Python support
	configs := []string{
		filepath.Join(tempDir, "config1.yaml"),
		filepath.Join(tempDir, "config2.yaml"),
		filepath.Join(tempDir, "config3.yaml"),
	}

	// Create test configuration files
	for i, configPath := range configs {
		config := `repos:
  - repo: https://github.com/psf/black
    rev: 23.7.0
    hooks:
      - id: black`

		switch i {
		case 1:
			config += `
        language_version: python3.8`
		case 2:
			config += `
        language_version: system`
		}

		err := os.WriteFile(configPath, []byte(config), 0o644)
		require.NoError(t, err)
	}

	// Test configuration tracking
	for _, configPath := range configs {
		// Mark configuration as used
		err := manager.MarkConfigUsed(configPath)
		require.NoError(t, err)

		t.Logf("Marked config as used: %s", configPath)
	}
}

func TestCacheDirectoryStructure(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Verify cache directory structure
	assert.Equal(t, tempDir, manager.GetCacheDir())

	// Verify database file exists
	dbPath := manager.GetDBPath()
	assert.True(t, strings.HasPrefix(dbPath, tempDir))

	// Check if database file exists
	_, err = os.Stat(dbPath)
	assert.NoError(t, err, "Database file should exist")

	// Check if lock file exists (Python pre-commit compatibility)
	lockPath := filepath.Join(tempDir, ".lock")
	_, err = os.Stat(lockPath)
	assert.NoError(t, err, "Lock file should exist for Python pre-commit compatibility")

	t.Logf("Cache directory: %s", manager.GetCacheDir())
	t.Logf("Database path: %s", dbPath)
}

func TestCacheWithAdditionalDependencies(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Test repository with additional dependencies (affects cache path)
	repo := config.Repo{
		Repo: "https://github.com/psf/black",
		Rev:  "23.7.0",
		Hooks: []config.Hook{
			{
				ID:              "black",
				LanguageVersion: "3.9",
				AdditionalDeps:  []string{"click>=8.0.0", "typing-extensions>=3.10.0"},
			},
		},
	}

	// Get path without additional dependencies
	pathWithoutDeps := manager.GetRepoPath(repo)

	// Get path with additional dependencies
	pathWithDeps := manager.GetRepoPathWithDeps(repo, repo.Hooks[0].AdditionalDeps)

	// Paths should be different when additional dependencies are involved
	assert.NotEqual(t, pathWithoutDeps, pathWithDeps,
		"Paths should differ when additional dependencies are specified")

	// Both paths should be in cache directory
	assert.True(t, strings.HasPrefix(pathWithoutDeps, tempDir))
	assert.True(t, strings.HasPrefix(pathWithDeps, tempDir))

	// Update cache entries
	err = manager.UpdateRepoEntry(repo, pathWithoutDeps)
	require.NoError(t, err)

	err = manager.UpdateRepoEntryWithDeps(repo, repo.Hooks[0].AdditionalDeps, pathWithDeps)
	require.NoError(t, err)

	t.Logf("Path without deps: %s", pathWithoutDeps)
	t.Logf("Path with deps: %s", pathWithDeps)
}
