package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiVersionPythonSupport(t *testing.T) {
	tempDir := t.TempDir()

	// Create test configuration with multiple Python versions
	configContent := `repos:
  - repo: https://github.com/psf/black
    rev: "23.7.0"
    hooks:
      - id: black
        language_version: "3.8"

  - repo: https://github.com/pycqa/flake8
    rev: "6.0.0"
    hooks:
      - id: flake8
        # Should use default_language_version

  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: "v4.4.0"
    hooks:
      - id: check-yaml
        language_version: "3.9"
      - id: trailing-whitespace
        language_version: "system"

default_language_version:
  python: "3.10"
  node: "18"`

	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Load and test configuration
	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify default_language_version parsing
	assert.Equal(t, "3.10", cfg.DefaultLanguageVersion["python"])
	assert.Equal(t, "18", cfg.DefaultLanguageVersion["node"])

	// Test version resolution for each hook
	testCases := []struct {
		expectedVersion string
		description     string
		repoIndex       int
		hookIndex       int
	}{
		{expectedVersion: "3.8", description: "Black with explicit version", repoIndex: 0, hookIndex: 0},
		{expectedVersion: "3.10", description: "Flake8 using default_language_version", repoIndex: 1, hookIndex: 0},
		{expectedVersion: "3.9", description: "Check-yaml with explicit version", repoIndex: 2, hookIndex: 0},
		{expectedVersion: "system", description: "Trailing-whitespace with system version", repoIndex: 2, hookIndex: 1},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			hook := cfg.Repos[tc.repoIndex].Hooks[tc.hookIndex]

			// Simulate what happens in the install-hooks command
			// where we need to use the repo hook's language
			hookWithLanguage := Hook{
				ID:              hook.ID,
				Language:        "python", // This comes from the repo's .pre-commit-hooks.yaml
				LanguageVersion: hook.LanguageVersion,
			}

			result := ResolveEffectiveLanguageVersion(hookWithLanguage, *cfg)
			assert.Equal(t, tc.expectedVersion, result,
				"Hook %s should resolve to version %s", hook.ID, tc.expectedVersion)
		})
	}
}

func TestResolveEffectiveLanguageVersion_MultiVersionEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		expectedResult string
		hook           Hook
		config         Config
	}{
		{
			name: "Empty default_language_version map",
			hook: Hook{
				Language: "python",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{},
			},
			expectedResult: "",
		},
		{
			name: "Nil default_language_version map",
			hook: Hook{
				Language: "python",
			},
			config: Config{
				DefaultLanguageVersion: nil,
			},
			expectedResult: "",
		},
		{
			name: "Language not in default_language_version",
			hook: Hook{
				Language: "ruby",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.9",
					"node":   "18",
				},
			},
			expectedResult: "",
		},
		{
			name: "Hook version overrides default for different language",
			hook: Hook{
				Language:        "node",
				LanguageVersion: "16",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.9",
					"node":   "18",
				},
			},
			expectedResult: "16",
		},
		{
			name: "Multiple languages with different defaults",
			hook: Hook{
				Language: "ruby",
			},
			config: Config{
				DefaultLanguageVersion: map[string]string{
					"python": "3.9",
					"node":   "18",
					"ruby":   "3.1",
					"go":     "1.19",
				},
			},
			expectedResult: "3.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveEffectiveLanguageVersion(tt.hook, tt.config)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestDefaultLanguageVersionConfigParsing(t *testing.T) {
	tests := []struct {
		expectedConfig map[string]string
		name           string
		configContent  string
		shouldError    bool
	}{
		{
			name: "Valid default_language_version",
			configContent: `repos: []
default_language_version:
  python: "3.9"
  node: "18.19.0"
  ruby: "3.1"`,
			expectedConfig: map[string]string{
				"python": "3.9",
				"node":   "18.19.0",
				"ruby":   "3.1",
			},
			shouldError: false,
		},
		{
			name: "Empty default_language_version",
			configContent: `repos: []
default_language_version: {}`,
			expectedConfig: map[string]string{},
			shouldError:    false,
		},
		{
			name:           "No default_language_version specified",
			configContent:  `repos: []`,
			expectedConfig: nil,
			shouldError:    false,
		},
		{
			name: "Single language default",
			configContent: `repos: []
default_language_version:
  python: "3.11"`,
			expectedConfig: map[string]string{
				"python": "3.11",
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")

			err := os.WriteFile(configPath, []byte(tt.configContent), 0o644)
			require.NoError(t, err)

			cfg, err := LoadConfig(configPath)

			if tt.shouldError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			if tt.expectedConfig == nil {
				assert.Nil(t, cfg.DefaultLanguageVersion)
			} else {
				assert.Equal(t, tt.expectedConfig, cfg.DefaultLanguageVersion)
			}
		})
	}
}

func TestConfigValidationWithDefaultLanguageVersion(t *testing.T) {
	tempDir := t.TempDir()

	// Test that configs with default_language_version validate correctly
	configContent := `repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: "v4.4.0"
    hooks:
      - id: trailing-whitespace
        language_version: "3.8"
      - id: end-of-file-fixer

default_language_version:
  python: "3.11"
  node: "18"`

	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Validation should pass
	err = cfg.Validate()
	assert.NoError(t, err)

	// Verify the structure
	assert.Len(t, cfg.Repos, 1)
	assert.Len(t, cfg.Repos[0].Hooks, 2)
	assert.Equal(t, "3.8", cfg.Repos[0].Hooks[0].LanguageVersion)
	assert.Equal(t, "", cfg.Repos[0].Hooks[1].LanguageVersion) // Should use default
}

func TestVersionResolutionWithRealWorldConfig(t *testing.T) {
	// Test with a realistic pre-commit configuration
	tempDir := t.TempDir()

	configContent := `repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml
        language_version: "3.8"
      - id: check-added-large-files
        language_version: "system"

  - repo: https://github.com/psf/black
    rev: 23.7.0
    hooks:
      - id: black
        language_version: "3.9"

  - repo: https://github.com/pycqa/isort
    rev: 5.12.0
    hooks:
      - id: isort
        # Should use default version

  - repo: https://github.com/pycqa/flake8
    rev: 6.0.0
    hooks:
      - id: flake8
        language_version: "3.10"

default_language_version:
  python: "3.11"`

	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Test each hook's version resolution
	expectedVersions := map[string]string{
		"trailing-whitespace":     "3.11",   // default
		"end-of-file-fixer":       "3.11",   // default
		"check-yaml":              "3.8",    // explicit
		"check-added-large-files": "system", // explicit
		"black":                   "3.9",    // explicit
		"isort":                   "3.11",   // default
		"flake8":                  "3.10",   // explicit
	}

	for repoIdx, repo := range cfg.Repos {
		for _, hook := range repo.Hooks {
			t.Run(fmt.Sprintf("repo_%d_hook_%s", repoIdx, hook.ID), func(t *testing.T) {
				hookWithLanguage := Hook{
					ID:              hook.ID,
					Language:        "python", // Simulated from repo definition
					LanguageVersion: hook.LanguageVersion,
				}

				result := ResolveEffectiveLanguageVersion(hookWithLanguage, *cfg)
				expected := expectedVersions[hook.ID]
				assert.Equal(t, expected, result,
					"Hook %s should resolve to version %s, got %s", hook.ID, expected, result)
			})
		}
	}
}
