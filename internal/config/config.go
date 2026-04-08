// Package config provides configuration types and parsing for pre-commit.
package config

import (
	"fmt"
	"os"

	"github.com/blairham/go-pre-commit/internal/pcre"
	"gopkg.in/yaml.v3"
)

// Version is the current version of go-pre-commit.
// This tracks the Python pre-commit version we are compatible with,
// so that minimum_pre_commit_version checks in hook manifests pass.
const Version = "4.5.0"

// Default file names.
const (
	ConfigFile   = ".pre-commit-config.yaml"
	ManifestFile = ".pre-commit-hooks.yaml"
)

// Exit codes matching the Python pre-commit behavior.
const (
	ExitCodeSuccess    = 0
	ExitCodeError      = 1
	ExitCodeUnexpected = 3
	ExitCodeInterrupt  = 130
)

// HookType represents a supported git hook type.
type HookType string

const (
	HookTypePreCommit        HookType = "pre-commit"
	HookTypePreMergeCommit   HookType = "pre-merge-commit"
	HookTypePrePush          HookType = "pre-push"
	HookTypePreRebase        HookType = "pre-rebase"
	HookTypeCommitMsg        HookType = "commit-msg"
	HookTypePrepareCommitMsg HookType = "prepare-commit-msg"
	HookTypePostCheckout     HookType = "post-checkout"
	HookTypePostCommit       HookType = "post-commit"
	HookTypePostMerge        HookType = "post-merge"
	HookTypePostRewrite      HookType = "post-rewrite"
)

// StageManual is a special stage for hooks that only run when explicitly
// invoked via `pre-commit run --hook-stage manual`. Manual hooks are never
// triggered automatically by git hooks.
const StageManual HookType = "manual"

// AllHookTypes returns all supported git hook types (excludes manual).
func AllHookTypes() []HookType {
	return []HookType{
		HookTypePreCommit,
		HookTypePreMergeCommit,
		HookTypePrePush,
		HookTypePreRebase,
		HookTypeCommitMsg,
		HookTypePrepareCommitMsg,
		HookTypePostCheckout,
		HookTypePostCommit,
		HookTypePostMerge,
		HookTypePostRewrite,
	}
}

// Stage represents the git hook stage.
type Stage = HookType

// AllStages returns all supported stages, including "manual".
func AllStages() []Stage {
	stages := AllHookTypes()
	stages = append(stages, StageManual)
	return stages
}

// Config represents the top-level .pre-commit-config.yaml structure.
type Config struct {
	Repos                   []RepoConfig      `yaml:"repos"`
	DefaultInstallHookTypes []HookType        `yaml:"default_install_hook_types,omitempty"`
	DefaultLanguageVersion  map[string]string  `yaml:"default_language_version,omitempty"`
	DefaultStages           []Stage            `yaml:"default_stages,omitempty"`
	Files                   string             `yaml:"files,omitempty"`
	Exclude                 string             `yaml:"exclude,omitempty"`
	FailFast                bool               `yaml:"fail_fast,omitempty"`
	MinimumPreCommitVersion string             `yaml:"minimum_pre_commit_version,omitempty"`
	CIConfig                map[string]any     `yaml:"ci,omitempty"`
}

// RepoConfig represents a single repo entry in the config.
type RepoConfig struct {
	Repo  string       `yaml:"repo"`
	Rev   string       `yaml:"rev,omitempty"`
	Hooks []HookConfig `yaml:"hooks"`
}

// IsLocal returns true if this is a local repo config.
func (r *RepoConfig) IsLocal() bool {
	return r.Repo == "local"
}

// IsMeta returns true if this is a meta repo config.
func (r *RepoConfig) IsMeta() bool {
	return r.Repo == "meta"
}

// HookConfig represents a hook entry within a repo config.
type HookConfig struct {
	ID                     string   `yaml:"id"`
	Alias                  string   `yaml:"alias,omitempty"`
	Name                   string   `yaml:"name,omitempty"`
	Language               string   `yaml:"language,omitempty"`
	LanguageVersion        string   `yaml:"language_version,omitempty"`
	Entry                  string   `yaml:"entry,omitempty"`
	Files                  string   `yaml:"files,omitempty"`
	Exclude                string   `yaml:"exclude,omitempty"`
	Types                  []string `yaml:"types,omitempty"`
	TypesOr                []string `yaml:"types_or,omitempty"`
	ExcludeTypes           []string `yaml:"exclude_types,omitempty"`
	Args                   []string `yaml:"args,omitempty"`
	Stages                 []Stage  `yaml:"stages,omitempty"`
	AdditionalDependencies []string `yaml:"additional_dependencies,omitempty"`
	AlwaysRun              *bool    `yaml:"always_run,omitempty"`
	Verbose                *bool    `yaml:"verbose,omitempty"`
	PassFilenames          *bool    `yaml:"pass_filenames,omitempty"`
	RequireSerial          *bool    `yaml:"require_serial,omitempty"`
	FailFast               *bool    `yaml:"fail_fast,omitempty"`
	Description            string   `yaml:"description,omitempty"`
	LogFile                string   `yaml:"log_file,omitempty"`
}

// ManifestHook represents a hook entry in .pre-commit-hooks.yaml.
type ManifestHook struct {
	ID                      string   `yaml:"id"`
	Name                    string   `yaml:"name"`
	Entry                   string   `yaml:"entry"`
	Language                string   `yaml:"language"`
	LanguageVersion         string   `yaml:"language_version,omitempty"`
	Files                   string   `yaml:"files,omitempty"`
	Exclude                 string   `yaml:"exclude,omitempty"`
	Types                   []string `yaml:"types,omitempty"`
	TypesOr                 []string `yaml:"types_or,omitempty"`
	ExcludeTypes            []string `yaml:"exclude_types,omitempty"`
	Args                    []string `yaml:"args,omitempty"`
	Stages                  []Stage  `yaml:"stages,omitempty"`
	PassFilenames           *bool    `yaml:"pass_filenames,omitempty"`
	AlwaysRun               bool     `yaml:"always_run,omitempty"`
	Verbose                 bool     `yaml:"verbose,omitempty"`
	FailFast                bool     `yaml:"fail_fast,omitempty"`
	RequireSerial           bool     `yaml:"require_serial,omitempty"`
	Description             string   `yaml:"description,omitempty"`
	MinimumPreCommitVersion string   `yaml:"minimum_pre_commit_version,omitempty"`
}

// DefaultPassFilenames returns the pass_filenames value, defaulting to true.
func (h *ManifestHook) DefaultPassFilenames() bool {
	if h.PassFilenames == nil {
		return true
	}
	return *h.PassFilenames
}

// LoadConfig reads and parses a .pre-commit-config.yaml file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}

	// Enforce minimum_pre_commit_version.
	if cfg.MinimumPreCommitVersion != "" {
		if !CheckMinimumVersion(cfg.MinimumPreCommitVersion) {
			return nil, fmt.Errorf(
				"pre-commit version %s is required but version %s is installed. "+
					"Update using: pip install --upgrade pre-commit (or go install this binary)",
				cfg.MinimumPreCommitVersion, Version,
			)
		}
	}

	// Apply defaults.
	cfg.ApplyDefaults()

	// Warn about mutable revs.
	for _, repo := range cfg.Repos {
		if !repo.IsLocal() && !repo.IsMeta() && repo.Rev != "" {
			WarnMutableRev(repo.Repo, repo.Rev)
		}
	}

	return &cfg, nil
}

// ApplyDefaults applies default_stages and default_language_version to hooks.
func (c *Config) ApplyDefaults() {
	// Migrate legacy stage names at load time.
	c.DefaultStages = migrateLegacyStages(c.DefaultStages)

	for i := range c.Repos {
		for j := range c.Repos[i].Hooks {
			hc := &c.Repos[i].Hooks[j]
			// Migrate legacy stage names.
			hc.Stages = migrateLegacyStages(hc.Stages)
			// Apply default_stages if the hook doesn't specify stages.
			if len(hc.Stages) == 0 && len(c.DefaultStages) > 0 {
				hc.Stages = c.DefaultStages
			}
			// Apply default_language_version if specified.
			if hc.LanguageVersion == "" && c.DefaultLanguageVersion != nil {
				if v, ok := c.DefaultLanguageVersion[hc.Language]; ok {
					hc.LanguageVersion = v
				}
			}
		}
	}
}

// migrateLegacyStages maps legacy stage names to their current equivalents.
func migrateLegacyStages(stages []Stage) []Stage {
	if len(stages) == 0 {
		return stages
	}
	legacyMap := map[Stage]Stage{
		"commit":       HookTypePreCommit,
		"merge-commit": HookTypePreMergeCommit,
		"push":         HookTypePrePush,
	}
	result := make([]Stage, len(stages))
	for i, s := range stages {
		if mapped, ok := legacyMap[s]; ok {
			result[i] = mapped
		} else {
			result[i] = s
		}
	}
	return result
}

// CheckMinimumVersion checks if the current version meets the minimum requirement.
func CheckMinimumVersion(minVersion string) bool {
	cParts := splitVersionParts(Version)
	rParts := splitVersionParts(minVersion)

	for i := 0; i < len(rParts); i++ {
		if i >= len(cParts) {
			return false
		}
		if cParts[i] < rParts[i] {
			return false
		}
		if cParts[i] > rParts[i] {
			return true
		}
	}
	return true
}

func splitVersionParts(v string) []int {
	var parts []int
	for _, s := range splitDot(v) {
		n := 0
		for _, c := range s {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			} else {
				break
			}
		}
		parts = append(parts, n)
	}
	return parts
}

func splitDot(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == '.' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// WarnMutableRev warns if a rev looks like a branch name rather than a tag/SHA.
func WarnMutableRev(repo, rev string) {
	// Check for common mutable rev patterns.
	mutablePrefixes := []string{"master", "main", "develop", "HEAD"}
	for _, prefix := range mutablePrefixes {
		if rev == prefix {
			fmt.Fprintf(os.Stderr,
				"WARNING: The 'rev' field of repo %q appears to be a mutable reference (%q).\n"+
					"Mutable references are never updated after first install and are not "+
					"supported. Use `pre-commit autoupdate` to update to a pinned revision.\n",
				repo, rev,
			)
			return
		}
	}
}

// LoadManifest reads and parses a .pre-commit-hooks.yaml file.
func LoadManifest(path string) ([]ManifestHook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file %s: %w", path, err)
	}

	var hooks []ManifestHook
	if err := yaml.Unmarshal(data, &hooks); err != nil {
		return nil, fmt.Errorf("failed to parse manifest file %s: %w", path, err)
	}

	for _, h := range hooks {
		if h.ID == "" {
			return nil, fmt.Errorf("hook missing required 'id' field in %s", path)
		}
		if h.Name == "" {
			return nil, fmt.Errorf("hook %q missing required 'name' field in %s", h.ID, path)
		}
		if h.Entry == "" {
			return nil, fmt.Errorf("hook %q missing required 'entry' field in %s", h.ID, path)
		}
		if h.Language == "" {
			return nil, fmt.Errorf("hook %q missing required 'language' field in %s", h.ID, path)
		}
	}

	return hooks, nil
}

// Validate validates the config structure.
func (c *Config) Validate() error {
	if len(c.Repos) == 0 {
		return fmt.Errorf("'repos' is required")
	}
	for i, repo := range c.Repos {
		if repo.Repo == "" {
			return fmt.Errorf("repos[%d]: 'repo' is required", i)
		}
		if !repo.IsLocal() && !repo.IsMeta() && repo.Rev == "" {
			return fmt.Errorf("repos[%d]: 'rev' is required for repo %q", i, repo.Repo)
		}
		if len(repo.Hooks) == 0 {
			return fmt.Errorf("repos[%d]: 'hooks' is required for repo %q", i, repo.Repo)
		}
		for j, hook := range repo.Hooks {
			if hook.ID == "" {
				return fmt.Errorf("repos[%d].hooks[%d]: 'id' is required", i, j)
			}
			// Local hooks require additional fields.
			if repo.IsLocal() {
				if hook.Name == "" {
					return fmt.Errorf("repos[%d].hooks[%d]: 'name' is required for local hook %q", i, j, hook.ID)
				}
				if hook.Entry == "" {
					return fmt.Errorf("repos[%d].hooks[%d]: 'entry' is required for local hook %q", i, j, hook.ID)
				}
				if hook.Language == "" {
					return fmt.Errorf("repos[%d].hooks[%d]: 'language' is required for local hook %q", i, j, hook.ID)
				}
			}
		}
	}

	// Validate regex patterns.
	if c.Files != "" {
		if _, err := pcre.Compile(c.Files); err != nil {
			return fmt.Errorf("invalid 'files' pattern: %w", err)
		}
	}
	if c.Exclude != "" {
		if _, err := pcre.Compile(c.Exclude); err != nil {
			return fmt.Errorf("invalid 'exclude' pattern: %w", err)
		}
	}

	// Validate hook-level regex patterns.
	for i, repo := range c.Repos {
		for j, hook := range repo.Hooks {
			if hook.Files != "" {
				if _, err := pcre.Compile(hook.Files); err != nil {
					return fmt.Errorf("repos[%d].hooks[%d] (%s): invalid 'files' pattern: %w", i, j, hook.ID, err)
				}
			}
			if hook.Exclude != "" {
				if _, err := pcre.Compile(hook.Exclude); err != nil {
					return fmt.Errorf("repos[%d].hooks[%d] (%s): invalid 'exclude' pattern: %w", i, j, hook.ID, err)
				}
			}
		}
	}

	return nil
}

// ValidateConfigFile validates a .pre-commit-config.yaml file.
func ValidateConfigFile(path string) error {
	_, err := LoadConfig(path)
	return err
}

// ValidateManifestFile validates a .pre-commit-hooks.yaml file.
func ValidateManifestFile(path string) error {
	_, err := LoadManifest(path)
	return err
}

// SampleConfig returns a sample .pre-commit-config.yaml content.
func SampleConfig() string {
	return `# See https://pre-commit.com for more information
# See https://pre-commit.com/hooks.html for more hooks
repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
    -   id: trailing-whitespace
    -   id: end-of-file-fixer
    -   id: check-yaml
    -   id: check-added-large-files
`
}

// DefaultConfig returns a Config with default values applied.
func DefaultConfig() *Config {
	return &Config{
		DefaultInstallHookTypes: []HookType{HookTypePreCommit},
		DefaultLanguageVersion:  make(map[string]string),
		DefaultStages:           AllStages(),
		Exclude:                 "^$",
	}
}
