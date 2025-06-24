// Package config provides configuration parsing and validation for pre-commit.
package config

import (
	"crypto/sha1" // nolint:gosec // Used for non-cryptographic hash generation
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the .pre-commit-config.yaml structure
type Config struct {
	DefaultLanguageVersion  map[string]string `yaml:"default_language_version,omitempty"`
	CIConfig                map[string]any    `yaml:"ci,omitempty"`
	Files                   string            `yaml:"files,omitempty"`
	ExcludeRegex            string            `yaml:"exclude,omitempty"`
	MinimumPreCommitVersion string            `yaml:"minimum_pre_commit_version,omitempty"`
	Repos                   []Repo            `yaml:"repos"`
	DefaultStages           []string          `yaml:"default_stages,omitempty"`
	FailFast                bool              `yaml:"fail_fast,omitempty"`
}

// Repo represents a repository configuration
type Repo struct {
	Repo  string `yaml:"repo"`
	Rev   string `yaml:"rev"`
	Hooks []Hook `yaml:"hooks"`
}

// Hook represents a hook configuration
type Hook struct {
	PassFilenames           *bool    `yaml:"pass_filenames,omitempty"`
	ID                      string   `yaml:"id"`
	Name                    string   `yaml:"name,omitempty"`
	Entry                   string   `yaml:"entry,omitempty"`
	Language                string   `yaml:"language,omitempty"`
	Files                   string   `yaml:"files,omitempty"`
	ExcludeRegex            string   `yaml:"exclude,omitempty"`
	LogFile                 string   `yaml:"log_file,omitempty"`
	Description             string   `yaml:"description,omitempty"`
	LanguageVersion         string   `yaml:"language_version,omitempty"`
	MinimumPreCommitVersion string   `yaml:"minimum_pre_commit_version,omitempty"`
	Types                   []string `yaml:"types,omitempty"`
	TypesOr                 []string `yaml:"types_or,omitempty"`
	ExcludeTypes            []string `yaml:"exclude_types,omitempty"`
	AdditionalDeps          []string `yaml:"additional_dependencies,omitempty"`
	Args                    []string `yaml:"args,omitempty"`
	Stages                  []string `yaml:"stages,omitempty"`
	AlwaysRun               bool     `yaml:"always_run,omitempty"`
	Verbose                 bool     `yaml:"verbose,omitempty"`
	RequireSerial           bool     `yaml:"require_serial,omitempty"`
}

// ConfigFileName is the default name for the pre-commit configuration file
const ConfigFileName = ".pre-commit-config.yaml"

// LoadConfig loads the pre-commit configuration from file
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = ConfigFileName
	}

	if !filepath.IsAbs(configPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		configPath = filepath.Join(cwd, configPath)
	}

	// Basic path validation to address gosec G304
	if strings.Contains(configPath, "..") {
		return nil, fmt.Errorf("invalid config path: %s", configPath)
	}

	data, err := os.ReadFile(configPath) // #nosec G304 -- path is validated above
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Check for empty files
	if strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("config file %s is empty", configPath)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	return &config, nil
}

// LoadHooksConfig loads hooks from a .pre-commit-hooks.yaml file
func LoadHooksConfig(configPath string) ([]Hook, error) {
	// Basic path validation to address gosec G304
	if strings.Contains(configPath, "..") {
		return nil, fmt.Errorf("invalid config path: %s", configPath)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath) // #nosec G304 -- path is validated above
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var hooks []Hook
	if err := yaml.Unmarshal(data, &hooks); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return hooks, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if len(c.Repos) == 0 {
		// An empty repositories list is valid - just means no hooks are configured
		return nil
	}

	// Populate hook definitions from well-known repositories
	if err := c.PopulateHookDefinitions(); err != nil {
		return fmt.Errorf("failed to populate hook definitions: %w", err)
	}

	for i, repo := range c.Repos {
		if repo.Repo == "" {
			return fmt.Errorf("repo %d: repository URL is required", i)
		}
		// Skip revision requirement for local and meta repositories
		if repo.Rev == "" && repo.Repo != "local" && repo.Repo != "meta" {
			return fmt.Errorf("repo %d: revision is required", i)
		}
		if len(repo.Hooks) == 0 {
			return fmt.Errorf("repo %d: no hooks configured", i)
		}

		for j, hook := range repo.Hooks {
			if hook.ID == "" {
				return fmt.Errorf("repo %d, hook %d: hook ID is required", i, j)
			}
		}
	}

	return nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultStages: []string{"commit"},
		Repos: []Repo{
			{
				Repo: "https://github.com/pre-commit/pre-commit-hooks",
				Rev:  "v4.5.0",
				Hooks: []Hook{
					{
						ID: "trailing-whitespace",
					},
					{
						ID: "end-of-file-fixer",
					},
					{
						ID: "check-yaml",
					},
					{
						ID: "check-added-large-files",
					},
				},
			},
		},
	}
}

// HookEnvItem represents hook environment data for initialization
// nolint:govet // fieldalignment optimization not critical for this type
type HookEnvItem struct {
	RepoPath string
	Repo     Repo
	Hook     Hook
}

// GetRepoPath returns the repository path for a given repo configuration and cache directory
func GetRepoPath(repo Repo, cacheDir string) (string, error) {
	// Sanitize the repo URL to create a valid directory name
	sanitizedURL := strings.ReplaceAll(repo.Repo, "://", "_")
	sanitizedURL = strings.ReplaceAll(sanitizedURL, "/", "_")

	// Use a hash of the repo URL to keep the path shorter and more consistent
	hasher := sha1.New() // nolint:gosec // Used for non-cryptographic hash generation
	hasher.Write([]byte(repo.Repo))
	hash := hex.EncodeToString(hasher.Sum(nil))[:12]

	// Combine sanitized URL and hash for a unique, readable directory name
	dirName := fmt.Sprintf("%s-%s", sanitizedURL, hash)

	return filepath.Join(cacheDir, dirName), nil
}
