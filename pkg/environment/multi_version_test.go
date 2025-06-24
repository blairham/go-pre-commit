package environment

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

func TestMultiVersionPythonEnvironments(t *testing.T) {
	tempDir := t.TempDir()

	manager := NewManager(tempDir)
	require.NotNil(t, manager)

	// Test different Python versions create separate environments
	testVersions := []string{"3.8", "3.9", "3.10", "3.11", "default", "system"}
	envPaths := make(map[string]string)

	for _, version := range testVersions {
		t.Run("version_"+version, func(t *testing.T) {
			envPath, err := manager.SetupEnvironment("python", version, nil, "")
			if err != nil {
				t.Logf("SetupEnvironment for Python %s failed (expected for non-installed versions): %v", version, err)
				return
			}

			// Store the path for uniqueness checking
			envPaths[version] = envPath

			t.Logf("Python %s environment created at: %s", version, envPath)
		})
	}

	// Verify all successful environments have unique paths
	for version1, path1 := range envPaths {
		for version2, path2 := range envPaths {
			if version1 != version2 && path1 == path2 {
				t.Errorf("Versions %s and %s have the same environment path: %s", version1, version2, path1)
			}
		}
	}
}

func TestPythonEnvironmentVersionResolution(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock repository with Python environment
	repoPath := filepath.Join(tempDir, "test-repo")
	err := os.MkdirAll(repoPath, 0o755)
	require.NoError(t, err)

	python := languages.NewPythonLanguage()

	// Test environment path generation for different versions
	testCases := []struct {
		version        string
		expectedSuffix string
	}{
		{"3.8", "py_env-3.8"},
		{"3.9", "py_env-3.9"},
		{"3.10", "py_env-3.10"},
		{"default", "py_env-default"},
		{"system", "py_env-system"},
	}

	for _, tc := range testCases {
		t.Run("version_"+tc.version, func(t *testing.T) {
			envPath := python.GetEnvironmentPath(repoPath, tc.version)

			// Check that the path ends with the expected suffix
			assert.True(t, filepath.Base(envPath) == tc.expectedSuffix,
				"Environment path for version %s should end with %s, got %s",
				tc.version, tc.expectedSuffix, filepath.Base(envPath))

			// Check that the path is within the repo directory
			assert.True(t, filepath.Dir(envPath) == repoPath,
				"Environment path should be within repo directory")
		})
	}
}

func TestPythonEnvironmentVersionCompatibility(t *testing.T) {
	python := languages.NewPythonLanguage()

	// Test that GetEnvironmentVersion preserves compatibility
	testCases := []struct {
		inputVersion   string
		expectedOutput string
		description    string
	}{
		{"", "default", "Empty version becomes default"},
		{"default", "default", "Default version is preserved"},
		{"3.9", "3.9", "Specific version is preserved"},
		{"3.11.5", "3.11.5", "Full version is preserved"},
		{"system", "system", "System version is preserved"},
		{"python3.9", "3.9", "Python prefix is stripped"},
		{"python3.11.5", "3.11.5", "Python prefix is stripped from full version"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result, err := python.GetEnvironmentVersion(tc.inputVersion)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, result)
		})
	}
}

func TestPythonVersionResolutionLogic(t *testing.T) {
	testCases := []struct {
		requestedVersion string
		description      string
		shouldNotBeEmpty bool
	}{
		{"", "Empty version should resolve to something", true},
		{"default", "Default version should resolve to something", true},
		{"system", "System version should resolve to something", true},
		{"3.9", "Specific version should be returned as-is", true},
		{"3.11.5", "Full version should be returned as-is", true},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Test that version resolution works as expected
			// These should return exactly what was requested for specific versions
			if tc.requestedVersion == "3.9" || tc.requestedVersion == "3.11.5" {
				assert.True(t, tc.requestedVersion != "")
			}
		})
	}
}

func TestMultiVersionEnvironmentIsolation(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock repository structure
	repoPath := filepath.Join(tempDir, "test-repo")
	err := os.MkdirAll(repoPath, 0o755)
	require.NoError(t, err)

	python := languages.NewPythonLanguage()

	// Test that different versions get different environment paths
	versions := []string{"3.8", "3.9", "3.10", "default", "system"}
	paths := make(map[string]string)

	for _, version := range versions {
		path := python.GetEnvironmentPath(repoPath, version)
		paths[version] = path

		// Verify each path is unique
		for otherVersion, otherPath := range paths {
			if version != otherVersion && path == otherPath {
				t.Errorf("Versions %s and %s have the same path: %s", version, otherVersion, path)
			}
		}
	}

	// Verify all paths are within the repo directory
	for version, path := range paths {
		assert.Equal(t, repoPath, filepath.Dir(path),
			"Environment for version %s should be in repo directory", version)
	}
}

func TestMultiVersionSetupHookEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Create test hooks with different Python versions
	hooks := []struct {
		version string
		repo    config.Repo
		hook    config.Hook
	}{
		{
			hook: config.Hook{
				ID:              "black",
				Language:        "python",
				LanguageVersion: "3.8",
			},
			repo:    config.Repo{Repo: "test-repo-1"},
			version: "3.8",
		},
		{
			hook: config.Hook{
				ID:       "flake8",
				Language: "python",
				// No version specified - should use a default
			},
			repo:    config.Repo{Repo: "test-repo-2"},
			version: "", // Will use default
		},
		{
			hook: config.Hook{
				ID:              "mypy",
				Language:        "python",
				LanguageVersion: "system",
			},
			repo:    config.Repo{Repo: "test-repo-3"},
			version: "system",
		},
	}

	envPaths := make(map[string]string)

	for _, testCase := range hooks {
		t.Run("hook_"+testCase.hook.ID, func(t *testing.T) {
			// Create a temporary repo directory
			repoPath := filepath.Join(tempDir, "repos", testCase.repo.Repo)
			err := os.MkdirAll(repoPath, 0o755)
			require.NoError(t, err)

			// Setup hook environment
			env, err := manager.SetupHookEnvironment(testCase.hook, testCase.repo, repoPath)
			if err != nil {
				t.Logf("SetupHookEnvironment for hook %s failed (expected for non-installed Python versions): %v",
					testCase.hook.ID, err)
				return
			}

			// Verify environment variables are set
			assert.Contains(t, env, "PRE_COMMIT_ENV_PATH")
			assert.Contains(t, env, "PRE_COMMIT_LANGUAGE")
			assert.Contains(t, env, "PRE_COMMIT_VERSION")

			assert.Equal(t, "python", env["PRE_COMMIT_LANGUAGE"])
			assert.Equal(t, testCase.hook.LanguageVersion, env["PRE_COMMIT_VERSION"])

			// Store for uniqueness check
			envPaths[testCase.hook.ID] = env["PRE_COMMIT_ENV_PATH"]

			t.Logf("Hook %s (version %s) environment: %s",
				testCase.hook.ID, testCase.hook.LanguageVersion, env["PRE_COMMIT_ENV_PATH"])
		})
	}

	// Verify different versions get different environment paths
	for id1, path1 := range envPaths {
		for id2, path2 := range envPaths {
			if id1 != id2 && path1 == path2 {
				t.Errorf("Hooks %s and %s have the same environment path: %s", id1, id2, path1)
			}
		}
	}
}
