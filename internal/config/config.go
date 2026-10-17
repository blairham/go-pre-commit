// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration types and parsing for pre-commit.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/blairham/go-pre-commit/v4/internal/output"
)

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
	DefaultLanguageVersion  map[string]string `yaml:"default_language_version,omitempty"`
	DefaultStages           []Stage           `yaml:"default_stages,omitempty"`
	Files                   string            `yaml:"files,omitempty"`
	Exclude                 string            `yaml:"exclude,omitempty"`
	FailFast                bool              `yaml:"fail_fast,omitempty"`
	MinimumPreCommitVersion string            `yaml:"minimum_pre_commit_version,omitempty"`
	CIConfig                map[string]any    `yaml:"ci,omitempty"`
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
	AdditionalDependencies  []string `yaml:"additional_dependencies,omitempty"`
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

// LoadConfig reads, validates and parses a .pre-commit-config.yaml file, as
// upstream's clientlib.load_config does. A file that fails validation is a
// *fatal.Error of type InvalidConfigError carrying cfgv's message.
func LoadConfig(path string) (*Config, error) {
	data, err := loadFromFilename(path, invalidConfig, configSchema().check)
	if err != nil {
		return nil, err
	}

	// Validation has ruled out every shape that cannot bind to the struct;
	// an error here is a gap in that port, reported as upstream would a
	// validation failure.
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, invalidConfig(inFile(path, newValidationError("%s", err.Error())))
	}

	// Upstream's schema warnings, in the order its schema checks them: each
	// repo's hooks, then that repo's rev, then the top-level default_stages.
	// They read the stages as written, so they run before ApplyDefaults
	// migrates the legacy names.
	for _, repo := range cfg.Repos {
		for _, hook := range repo.Hooks {
			warnDeprecatedStages("hook id `"+hook.ID+"`", hook.Stages)
		}
		if !repo.IsLocal() && !repo.IsMeta() && repo.Rev != "" {
			WarnMutableRev(repo.Repo, repo.Rev)
		}
	}
	warnDeprecatedStages("top-level `default_stages`", cfg.DefaultStages)

	// Apply defaults.
	cfg.ApplyDefaults()

	return &cfg, nil
}

// legacyStages maps upstream's deprecated stage names (clientlib._STAGES) to
// the names that replaced them.
var legacyStages = map[Stage]Stage{
	"commit":       HookTypePreCommit,
	"merge-commit": HookTypePreMergeCommit,
	"push":         HookTypePrePush,
}

// warnDeprecatedStages ports upstream's DeprecatedStagesWarning and
// DeprecatedDefaultStagesWarning; what names the hook or `default_stages`.
func warnDeprecatedStages(what string, stages []Stage) {
	var legacy []string
	for _, s := range stages {
		if _, ok := legacyStages[s]; ok {
			legacy = append(legacy, string(s))
		}
	}
	if len(legacy) == 0 {
		return
	}
	output.Warn(
		"%s uses deprecated stage names (%s) which will be removed in a "+
			"future version.  "+
			"run: `pre-commit migrate-config` to automatically fix this.",
		what, strings.Join(legacy, ", "),
	)
}

// WarnForStagesOnRepoInit ports upstream's warn_for_stages_on_repo_init,
// called once a repo has been cloned into directory: if the repo's manifest
// uses deprecated stage names, say so. A manifest that does not load is left
// alone, as upstream does; the real load reports it properly.
func WarnForStagesOnRepoInit(repo, directory string) {
	data, err := os.ReadFile(filepath.Join(directory, ManifestFile))
	if err != nil {
		return
	}
	// Upstream's MINIMAL_MANIFEST_SCHEMA: a list of hooks, each with a
	// string id and an optional list of string stages.
	var hooks []map[string]any
	if err := yaml.Unmarshal(data, &hooks); err != nil || hooks == nil {
		return
	}
	var legacy []string
	seen := map[string]bool{}
	for _, h := range hooks {
		if _, ok := h["id"].(string); !ok {
			return
		}
		raw, ok := h["stages"]
		if !ok {
			continue
		}
		stages, ok := raw.([]any)
		if !ok {
			return
		}
		for _, v := range stages {
			stage, ok := v.(string)
			if !ok {
				return
			}
			if _, isLegacy := legacyStages[Stage(stage)]; isLegacy && !seen[stage] {
				seen[stage] = true
				legacy = append(legacy, stage)
			}
		}
	}
	if len(legacy) == 0 {
		return
	}
	output.Warn(
		"repo `%s` uses deprecated stage names (%s) which will be removed in a "+
			"future version.  "+
			"Hint: often `pre-commit autoupdate --repo %s` will fix this.  "+
			"if it does not -- consider reporting an issue to that repo.",
		repo, strings.Join(legacy, ", "), ShlexQuote(repo),
	)
}

// ShlexQuote is Python's shlex.quote. install also uses it, to write the
// hook script's ARGS line as upstream's shlex.join does.
func ShlexQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && !strings.ContainsRune("_@%+=:,./-", c) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
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
	result := make([]Stage, len(stages))
	for i, s := range stages {
		if mapped, ok := legacyStages[s]; ok {
			result[i] = mapped
		} else {
			result[i] = s
		}
	}
	return result
}

// TransformStage maps a legacy stage name given to --hook-stage onto its
// current one, as upstream's clientlib.transform_stage does.
func TransformStage(s Stage) Stage {
	if mapped, ok := legacyStages[s]; ok {
		return mapped
	}
	return s
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
	// Dev builds inject `git describe` output (e.g. "v4.6.0-1-gabc123");
	// strip the leading "v" so the first component parses as a number.
	v = strings.TrimPrefix(v, "v")
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

// isHexRev reports whether rev matches upstream's ^[a-fA-F0-9]+$.
func isHexRev(rev string) bool {
	if rev == "" {
		return false
	}
	for _, c := range rev {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// WarnMutableRev ports upstream's clientlib.WarnMutableRev: a rev with no
// '.' that is not all hex looks like a moving tag or branch, so it is
// reported through the logger, which prints a [WARNING] line on stdout.
func WarnMutableRev(repo, rev string) {
	if strings.Contains(rev, ".") || isHexRev(rev) {
		return
	}
	output.Warn(
		"The 'rev' field of repo %s appears to be a mutable reference "+
			"(moving tag / branch).  Mutable references are never "+
			"updated after first install and are not supported.  "+
			"See https://pre-commit.com/#using-the-latest-version-for-a-repository "+
			"for more details.  "+
			"Hint: `pre-commit autoupdate` often fixes this.",
		output.PyRepr(repo),
	)
}

// LoadManifest reads, validates and parses a .pre-commit-hooks.yaml file,
// as upstream's clientlib.load_manifest does. A file that fails validation
// is a *fatal.Error of type InvalidManifestError.
func LoadManifest(path string) ([]ManifestHook, error) {
	data, err := loadFromFilename(path, invalidManifest, validateManifest)
	if err != nil {
		return nil, err
	}

	var hooks []ManifestHook
	if err := yaml.Unmarshal(data, &hooks); err != nil {
		return nil, invalidManifest(inFile(path, newValidationError("%s", err.Error())))
	}

	// Upstream's manifest schema migrates legacy stage names too, so a hook
	// repo that still says `commit` runs at pre-commit.
	for i := range hooks {
		hooks[i].Stages = migrateLegacyStages(hooks[i].Stages)
	}
	return hooks, nil
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
		DefaultStages:           AllHookTypes(),
		Exclude:                 "^$",
	}
}
