package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiVersionIntegration(t *testing.T) {
	// Create a temporary directory for our test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")

	// Create a comprehensive test configuration
	configContent := `default_language_version:
  python: python3.9

repos:
  - repo: https://github.com/psf/black
    rev: 23.7.0
    hooks:
      - id: black
        language_version: python3.8
  - repo: https://github.com/pycqa/flake8
    rev: 6.0.0
    hooks:
      - id: flake8
        # Should use default_language_version (python3.9)
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: check-yaml
        language_version: system
`

	err := os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Load the configuration
	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify the configuration was loaded correctly
	assert.Equal(t, "python3.9", cfg.DefaultLanguageVersion["python"])
	assert.Len(t, cfg.Repos, 3)

	// Test version resolution for each hook with simulated language info
	testCases := []struct {
		repoName        string
		hookID          string
		language        string
		hookVersion     string
		expectedVersion string
		description     string
	}{
		{
			repoName:        "https://github.com/psf/black",
			hookID:          "black",
			language:        "python",
			hookVersion:     "python3.8",
			expectedVersion: "python3.8",
			description:     "Hook-specific version override",
		},
		{
			repoName:        "https://github.com/pycqa/flake8",
			hookID:          "flake8",
			language:        "python",
			hookVersion:     "",
			expectedVersion: "python3.9",
			description:     "Default language version",
		},
		{
			repoName:        "https://github.com/pre-commit/pre-commit-hooks",
			hookID:          "check-yaml",
			language:        "python",
			hookVersion:     "system",
			expectedVersion: "system",
			description:     "System version override",
		},
	}

	t.Run("VersionResolution", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				// Find the hook in the configuration
				var foundHook Hook
				found := false

				for _, repo := range cfg.Repos {
					if repo.Repo == tc.repoName {
						for _, hook := range repo.Hooks {
							if hook.ID == tc.hookID {
								foundHook = hook
								foundHook.Language = tc.language // Simulate repository hook language
								found = true
								break
							}
						}
					}
				}

				require.True(t, found, "Hook %s should be found in config", tc.hookID)
				assert.Equal(t, tc.hookVersion, foundHook.LanguageVersion)

				// Test version resolution
				effectiveVersion := ResolveEffectiveLanguageVersion(foundHook, *cfg)
				assert.Equal(t, tc.expectedVersion, effectiveVersion,
					"Effective version for %s should be %s", tc.hookID, tc.expectedVersion)

				t.Logf("Hook %s: configured=%s, effective=%s ✅",
					tc.hookID, tc.hookVersion, effectiveVersion)
			})
		}
	})

	t.Run("EdgeCases", func(t *testing.T) {
		// Test with empty hook language version
		emptyHook := Hook{ID: "test", Language: "python", LanguageVersion: ""}
		result := ResolveEffectiveLanguageVersion(emptyHook, *cfg)
		assert.Equal(t, "python3.9", result, "Should use default language version")

		// Test with non-existent language in defaults
		jsHook := Hook{ID: "test", Language: "javascript", LanguageVersion: ""}
		result = ResolveEffectiveLanguageVersion(jsHook, *cfg)
		assert.Equal(t, "", result, "Should return empty for non-configured language")

		// Test with empty default config
		emptyDefaultCfg := &Config{DefaultLanguageVersion: map[string]string{}}
		result = ResolveEffectiveLanguageVersion(emptyHook, *emptyDefaultCfg)
		assert.Equal(t, "", result, "Should return empty when no defaults")

		// Test hook-specific version overrides default
		overrideHook := Hook{ID: "test", Language: "python", LanguageVersion: "python3.11"}
		result = ResolveEffectiveLanguageVersion(overrideHook, *cfg)
		assert.Equal(t, "python3.11", result, "Hook version should override default")
	})

	t.Run("CompatibilityWithPythonPreCommit", func(t *testing.T) {
		// Test scenarios that should match Python pre-commit behavior
		scenarios := []struct {
			name     string
			expected string
			hook     Hook
			config   Config
		}{
			{
				name:     "Hook version overrides default",
				hook:     Hook{Language: "python", LanguageVersion: "python3.10"},
				config:   Config{DefaultLanguageVersion: map[string]string{"python": "python3.8"}},
				expected: "python3.10",
			},
			{
				name:     "System version handling",
				hook:     Hook{Language: "python", LanguageVersion: "system"},
				config:   Config{DefaultLanguageVersion: map[string]string{"python": "python3.9"}},
				expected: "system",
			},
			{
				name:     "Empty version uses default",
				hook:     Hook{Language: "python", LanguageVersion: ""},
				config:   Config{DefaultLanguageVersion: map[string]string{"python": "python3.7"}},
				expected: "python3.7",
			},
			{
				name:     "No default available",
				hook:     Hook{Language: "ruby", LanguageVersion: ""},
				config:   Config{DefaultLanguageVersion: map[string]string{"python": "python3.9"}},
				expected: "",
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				result := ResolveEffectiveLanguageVersion(scenario.hook, scenario.config)
				assert.Equal(t, scenario.expected, result)
			})
		}
	})
}
