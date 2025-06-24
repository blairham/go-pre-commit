package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.Repos, "Default config should have at least one repository")
	assert.NotEmpty(t, cfg.DefaultStages, "Default config should have default stages")
	assert.Contains(t, cfg.DefaultStages, "commit")

	// Verify default repo structure
	assert.Len(t, cfg.Repos, 1)
	defaultRepo := cfg.Repos[0]
	assert.Equal(t, "https://github.com/pre-commit/pre-commit-hooks", defaultRepo.Repo)
	assert.NotEmpty(t, defaultRepo.Rev)
	assert.NotEmpty(t, defaultRepo.Hooks)

	// Verify default hooks
	expectedHooks := []string{"trailing-whitespace", "end-of-file-fixer", "check-yaml", "check-added-large-files"}
	assert.Len(t, defaultRepo.Hooks, len(expectedHooks))

	for i, expectedID := range expectedHooks {
		assert.Equal(t, expectedID, defaultRepo.Hooks[i].ID)
	}

	// Verify the config is valid
	err := cfg.Validate()
	assert.NoError(t, err, "Default config should be valid")
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		config  *Config
		name    string
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "https://github.com/test/repo",
						Rev:  "v1.0.0",
						Hooks: []Hook{
							{ID: "test-hook"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no repos",
			config: &Config{
				Repos: []Repo{},
			},
			wantErr: false, // Empty repos list is valid
		},
		{
			name: "repo without URL",
			config: &Config{
				Repos: []Repo{
					{
						Rev: "v1.0.0",
						Hooks: []Hook{
							{ID: "test-hook"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "repo without revision (non-local)",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "https://github.com/test/repo",
						Hooks: []Hook{
							{ID: "test-hook"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "local repo without revision",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "local",
						Hooks: []Hook{
							{ID: "test-hook"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "meta repo without revision",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "meta",
						Hooks: []Hook{
							{ID: "test-hook"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "repo without hooks",
			config: &Config{
				Repos: []Repo{
					{
						Repo:  "https://github.com/test/repo",
						Rev:   "v1.0.0",
						Hooks: []Hook{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "hook without ID",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "https://github.com/test/repo",
						Rev:  "v1.0.0",
						Hooks: []Hook{
							{Name: "test-hook"}, // Missing ID
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple repos with mixed validity",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "https://github.com/valid/repo",
						Rev:  "v1.0.0",
						Hooks: []Hook{
							{ID: "valid-hook"},
						},
					},
					{
						Repo: "local",
						Hooks: []Hook{
							{ID: "local-hook"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		setupFunc   func(t *testing.T) string
		name        string
		content     string
		expectError bool
	}{
		{
			name: "valid config file",
			content: `repos:
  - repo: https://github.com/psf/black
    rev: 22.3.0
    hooks:
      - id: black
default_stages: [commit]
fail_fast: true`,
			expectError: false,
		},
		{
			name:        "empty config file",
			content:     "",
			expectError: true,
		},
		{
			name:        "only whitespace",
			content:     "   \n  \t  \n",
			expectError: true,
		},
		{
			name: "invalid yaml",
			content: `repos:
  - repo: https://github.com/psf/black
    rev: 22.3.0
    hooks:
      - id: black
    invalid: [unclosed`,
			expectError: true,
		},
		{
			name: "config with all fields",
			content: `default_language_version:
  python: python3.9
  node: "16"
ci:
  autofix_commit_msg: "Auto-fix from pre-commit"
  autofix_prs: true
files: '\.py$'
exclude: '^(docs/|migrations/)'
minimum_pre_commit_version: "2.15.0"
repos:
  - repo: https://github.com/psf/black
    rev: 22.3.0
    hooks:
      - id: black
        name: Black formatter
        entry: black --check
        language: python
        files: '\.py$'
        exclude: '(migrations|tests)/.*\.py$'
        args: ["--line-length=88"]
        stages: [commit, push]
        types: [python]
        types_or: [python, pyi]
        exclude_types: [markdown]
        additional_dependencies: ["black[d]"]
        language_version: python3.9
        minimum_pre_commit_version: "2.15.0"
        pass_filenames: true
        always_run: false
        verbose: true
        require_serial: false
        log_file: black.log
        description: "Black Python formatter"
default_stages: [commit, push]
fail_fast: true`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, ConfigFileName)

			err := os.WriteFile(configPath, []byte(tt.content), 0o644)
			require.NoError(t, err)

			config, err := LoadConfig(configPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestLoadConfig_DefaultPath(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	content := `repos:
  - repo: https://github.com/psf/black
    rev: 22.3.0
    hooks:
      - id: black`

	err = os.WriteFile(ConfigFileName, []byte(content), 0o644)
	require.NoError(t, err)

	// Test loading with empty path (should use default)
	config, err := LoadConfig("")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Len(t, config.Repos, 1)
}

func TestLoadConfig_PathValidation(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		errorMsg    string
		expectError bool
	}{
		{
			name:        "absolute path with dot-dot",
			path:        "/some/path/../config.yaml",
			expectError: true,
			errorMsg:    "invalid config path",
		},
		{
			name:        "relative path that becomes valid after absolute",
			path:        "../config.yaml", // This becomes valid after filepath.Join
			expectError: true,
			errorMsg:    "failed to read config file", // File not found, but path is valid
		},
		{
			name:        "valid relative path",
			path:        "config.yaml",
			expectError: true,
			errorMsg:    "failed to read config file", // File not found, but path is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(tt.path)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadHooksConfig(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
		expectedLen int
	}{
		{
			name: "valid hooks config",
			content: `- id: black
  name: Black
  entry: black
  language: python
  types: [python]
- id: flake8
  name: Flake8
  entry: flake8
  language: python
  types: [python]`,
			expectError: false,
			expectedLen: 2,
		},
		{
			name: "single hook",
			content: `- id: mypy
  name: MyPy
  entry: mypy
  language: python`,
			expectError: false,
			expectedLen: 1,
		},
		{
			name: "invalid yaml",
			content: `- id: test
  invalid: [unclosed`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, ".pre-commit-hooks.yaml")

			err := os.WriteFile(configPath, []byte(tt.content), 0o644)
			require.NoError(t, err)

			hooks, err := LoadHooksConfig(configPath)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, hooks)
			} else {
				assert.NoError(t, err)
				assert.Len(t, hooks, tt.expectedLen)
			}
		})
	}
}

func TestLoadHooksConfig_PathValidation(t *testing.T) {
	_, err := LoadHooksConfig("../invalid/path.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid config path")
}

func TestLoadHooksConfig_FileNotFound(t *testing.T) {
	_, err := LoadHooksConfig("/nonexistent/hooks.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestGetRepoPath(t *testing.T) {
	tests := []struct {
		name     string
		cacheDir string
		expected string
		repo     Repo
	}{
		{
			name: "github repo",
			repo: Repo{
				Repo: "https://github.com/psf/black",
			},
			cacheDir: "/tmp/cache",
			expected: "https_//github.com/psf/black-b5b8c3a5b9cf",
		},
		{
			name: "gitlab repo",
			repo: Repo{
				Repo: "https://gitlab.com/user/repo",
			},
			cacheDir: "/tmp/cache",
			expected: "https_//gitlab.com/user/repo-b8e5d5e9c4b8",
		},
		{
			name: "repo with complex path",
			repo: Repo{
				Repo: "https://github.com/organization/repo-name.git",
			},
			cacheDir: "/home/user/.cache",
			expected: "https_//github.com/organization/repo-name.git-e6a1f5d2a8c3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GetRepoPath(tt.repo, tt.cacheDir)
			assert.NoError(t, err)

			// Check that the path starts with the cache directory
			assert.True(t, strings.HasPrefix(path, tt.cacheDir))

			// Check that the path contains sanitized URL elements
			filename := filepath.Base(path)
			assert.Contains(t, filename, "https_")
			assert.Contains(t, filename, "-")

			// Check that the path is deterministic
			path2, err2 := GetRepoPath(tt.repo, tt.cacheDir)
			assert.NoError(t, err2)
			assert.Equal(t, path, path2)
		})
	}
}

func TestPopulateHookDefinitions(t *testing.T) {
	tests := []struct {
		config       *Config
		expectedLang map[string]string
		name         string
	}{
		{
			name: "populate black hook",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "https://github.com/psf/black",
						Rev:  "22.3.0",
						Hooks: []Hook{
							{ID: "black"}, // Should get populated
						},
					},
				},
			},
			expectedLang: map[string]string{
				"black": "python",
			},
		},
		{
			name: "skip local repo",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "local",
						Hooks: []Hook{
							{ID: "custom-hook"},
						},
					},
				},
			},
			expectedLang: map[string]string{
				"custom-hook": "", // Should remain empty
			},
		},
		{
			name: "skip meta repo",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "meta",
						Hooks: []Hook{
							{ID: "check-hooks-apply"},
						},
					},
				},
			},
			expectedLang: map[string]string{
				"check-hooks-apply": "", // Should remain empty
			},
		},
		{
			name: "hook with existing language",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "https://github.com/psf/black",
						Rev:  "22.3.0",
						Hooks: []Hook{
							{
								ID:       "black",
								Language: "custom", // Pre-existing language
							},
						},
					},
				},
			},
			expectedLang: map[string]string{
				"black": "custom", // Should not change
			},
		},
		{
			name: "unknown hook in known repo",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "https://github.com/psf/black",
						Rev:  "22.3.0",
						Hooks: []Hook{
							{ID: "unknown-hook"},
						},
					},
				},
			},
			expectedLang: map[string]string{
				"unknown-hook": "", // Should remain empty
			},
		},
		{
			name: "multiple hooks from different repos",
			config: &Config{
				Repos: []Repo{
					{
						Repo: "https://github.com/psf/black",
						Rev:  "22.3.0",
						Hooks: []Hook{
							{ID: "black"},
						},
					},
					{
						Repo: "https://github.com/pycqa/flake8",
						Rev:  "4.0.1",
						Hooks: []Hook{
							{ID: "flake8"},
						},
					},
				},
			},
			expectedLang: map[string]string{
				"black":  "python",
				"flake8": "python",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.PopulateHookDefinitions()
			assert.NoError(t, err)

			// Check that languages were populated as expected
			for _, repo := range tt.config.Repos {
				for _, hook := range repo.Hooks {
					expectedLang := tt.expectedLang[hook.ID]
					assert.Equal(t, expectedLang, hook.Language,
						"Hook %s should have language %s, got %s",
						hook.ID, expectedLang, hook.Language)
				}
			}
		})
	}
}

func TestGetWellKnownHook(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		hookID   string
		expected Hook
		found    bool
	}{
		{
			name:    "black hook",
			repoURL: "https://github.com/psf/black",
			hookID:  "black",
			expected: Hook{
				ID:       "black",
				Name:     "black",
				Entry:    "black",
				Language: "python",
			},
			found: true,
		},
		{
			name:    "flake8 hook",
			repoURL: "https://github.com/pycqa/flake8",
			hookID:  "flake8",
			expected: Hook{
				ID:       "flake8",
				Name:     "flake8",
				Entry:    "flake8",
				Language: "python",
			},
			found: true,
		},
		{
			name:    "eslint hook",
			repoURL: "https://github.com/pre-commit/mirrors-eslint",
			hookID:  "eslint",
			expected: Hook{
				ID:       "eslint",
				Name:     "eslint",
				Entry:    "eslint",
				Language: "node",
			},
			found: true,
		},
		{
			name:     "unknown repo",
			repoURL:  "https://github.com/unknown/repo",
			hookID:   "some-hook",
			expected: Hook{},
			found:    false,
		},
		{
			name:     "unknown hook in known repo",
			repoURL:  "https://github.com/psf/black",
			hookID:   "unknown-hook",
			expected: Hook{},
			found:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook, found := GetWellKnownHook(tt.repoURL, tt.hookID)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.expected.ID, hook.ID)
				assert.Equal(t, tt.expected.Name, hook.Name)
				assert.Equal(t, tt.expected.Entry, hook.Entry)
				assert.Equal(t, tt.expected.Language, hook.Language)
			}
		})
	}
}

func TestPopulateHookFromWellKnown(t *testing.T) {
	config := &Config{}

	tests := []struct {
		name         string
		hook         Hook
		repoURL      string
		expectedHook Hook
	}{
		{
			name:    "populate all fields",
			hook:    Hook{ID: "black"},
			repoURL: "https://github.com/psf/black",
			expectedHook: Hook{
				ID:       "black",
				Name:     "black",
				Entry:    "black",
				Language: "python",
			},
		},
		{
			name: "preserve existing name",
			hook: Hook{
				ID:   "black",
				Name: "Custom Black",
			},
			repoURL: "https://github.com/psf/black",
			expectedHook: Hook{
				ID:       "black",
				Name:     "Custom Black", // Should keep existing
				Entry:    "black",
				Language: "python",
			},
		},
		{
			name: "preserve existing entry",
			hook: Hook{
				ID:    "black",
				Entry: "black --custom-arg",
			},
			repoURL: "https://github.com/psf/black",
			expectedHook: Hook{
				ID:       "black",
				Name:     "black",
				Entry:    "black --custom-arg", // Should keep existing
				Language: "python",
			},
		},
		{
			name:    "unknown hook",
			hook:    Hook{ID: "unknown"},
			repoURL: "https://github.com/psf/black",
			expectedHook: Hook{
				ID: "unknown", // Should remain unchanged
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := tt.hook // Copy to avoid modifying test data
			config.populateHookFromWellKnown(&hook, tt.repoURL)

			assert.Equal(t, tt.expectedHook.ID, hook.ID)
			assert.Equal(t, tt.expectedHook.Name, hook.Name)
			assert.Equal(t, tt.expectedHook.Entry, hook.Entry)
			assert.Equal(t, tt.expectedHook.Language, hook.Language)
		})
	}
}
