// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package hook provides the Hook type and hook resolution logic.
package hook

import (
	"fmt"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/languages"
	"github.com/blairham/go-pre-commit/v4/internal/pcre"
)

// Hook represents a fully resolved hook ready for execution.
type Hook struct {
	ID                      string
	Alias                   string
	Name                    string
	Entry                   string
	Language                string
	LanguageVersion         string
	Files                   string
	Exclude                 string
	Types                   []string
	TypesOr                 []string
	ExcludeTypes            []string
	Args                    []string
	Stages                  []config.Stage
	AdditionalDependencies  []string
	AlwaysRun               bool
	FailFast                bool
	Verbose                 bool
	PassFilenames           bool
	RequireSerial           bool
	Description             string
	MinimumPreCommitVersion string
	LogFile                 string

	// Repo information.
	Repo    string
	Rev     string
	RepoDir string // Local clone directory.
}

// IsLocal reports whether the hook comes from a `repo: local` block —
// upstream's `hook.src == 'local'`.
func (h *Hook) IsLocal() bool {
	return h.Repo == "local"
}

// InstallKey returns a unique key for deduplication of hook environments.
func (h *Hook) InstallKey() string {
	deps := strings.Join(h.AdditionalDependencies, ",")
	return fmt.Sprintf("%s:%s:%s:%s", h.RepoDir, h.Language, h.LanguageVersion, deps)
}

// MatchesFiles returns true if the given filename matches this hook's file filters.
func (h *Hook) MatchesFiles(filename string) bool {
	// Check include pattern.
	if h.Files != "" {
		matched, err := pcre.MatchString(h.Files, filename)
		if err != nil || !matched {
			return false
		}
	}
	// Check exclude pattern.
	if h.Exclude != "" {
		matched, err := pcre.MatchString(h.Exclude, filename)
		if err == nil && matched {
			return false
		}
	}
	return true
}

// MatchesStage returns true if this hook should run at the given stage.
// Hooks with stages: [manual] only run when --hook-stage manual is explicit.
func (h *Hook) MatchesStage(stage config.Stage) bool {
	if len(h.Stages) == 0 {
		// Upstream's default_stages defaults to every stage, manual included.
		return true
	}
	for _, s := range h.Stages {
		if s == stage {
			return true
		}
	}
	return false
}

// MergeManifest creates a Hook by merging config overrides onto a manifest hook.
func MergeManifest(
	manifest *config.ManifestHook,
	hookCfg *config.HookConfig,
	repoCfg *config.RepoConfig,
	globalCfg *config.Config,
) *Hook {
	h := &Hook{
		ID:                      manifest.ID,
		Name:                    manifest.Name,
		Entry:                   manifest.Entry,
		Language:                manifest.Language,
		LanguageVersion:         manifest.LanguageVersion,
		Files:                   manifest.Files,
		Exclude:                 manifest.Exclude,
		Types:                   manifest.Types,
		TypesOr:                 manifest.TypesOr,
		ExcludeTypes:            manifest.ExcludeTypes,
		Args:                    manifest.Args,
		Stages:                  manifest.Stages,
		AdditionalDependencies:  manifest.AdditionalDependencies,
		AlwaysRun:               manifest.AlwaysRun,
		FailFast:                manifest.FailFast,
		Verbose:                 manifest.Verbose,
		PassFilenames:           manifest.DefaultPassFilenames(),
		RequireSerial:           manifest.RequireSerial,
		Description:             manifest.Description,
		MinimumPreCommitVersion: manifest.MinimumPreCommitVersion,
		Repo:                    repoCfg.Repo,
		Rev:                     repoCfg.Rev,
	}

	// Apply defaults.
	if len(h.Types) == 0 && len(h.TypesOr) == 0 {
		h.Types = []string{"file"}
	}

	// Apply config overrides.
	if hookCfg.Alias != "" {
		h.Alias = hookCfg.Alias
	}
	if hookCfg.Name != "" {
		h.Name = hookCfg.Name
	}
	if hookCfg.LanguageVersion != "" {
		h.LanguageVersion = hookCfg.LanguageVersion
	}
	if hookCfg.Files != "" {
		h.Files = hookCfg.Files
	}
	if hookCfg.Exclude != "" {
		h.Exclude = hookCfg.Exclude
	}
	if len(hookCfg.Types) > 0 {
		h.Types = hookCfg.Types
	}
	if len(hookCfg.TypesOr) > 0 {
		h.TypesOr = hookCfg.TypesOr
	}
	if len(hookCfg.ExcludeTypes) > 0 {
		h.ExcludeTypes = hookCfg.ExcludeTypes
	}
	if len(hookCfg.Args) > 0 {
		h.Args = hookCfg.Args
	}
	if len(hookCfg.Stages) > 0 {
		h.Stages = hookCfg.Stages
	}
	if len(hookCfg.AdditionalDependencies) > 0 {
		h.AdditionalDependencies = hookCfg.AdditionalDependencies
	}
	if hookCfg.AlwaysRun != nil {
		h.AlwaysRun = *hookCfg.AlwaysRun
	}
	if hookCfg.Verbose != nil {
		h.Verbose = *hookCfg.Verbose
	}
	if hookCfg.PassFilenames != nil {
		h.PassFilenames = *hookCfg.PassFilenames
	}
	if hookCfg.RequireSerial != nil {
		h.RequireSerial = *hookCfg.RequireSerial
	}
	if hookCfg.FailFast != nil {
		h.FailFast = *hookCfg.FailFast
	}
	if hookCfg.LogFile != "" {
		h.LogFile = hookCfg.LogFile
	}

	// Apply global config defaults.
	if globalCfg != nil {
		if len(h.Stages) == 0 && len(globalCfg.DefaultStages) > 0 {
			h.Stages = globalCfg.DefaultStages
		}
	}
	resolveLanguageVersion(h, globalCfg)

	return h
}

// FromLocalConfig creates a Hook from a local hook config (repo: local).
func FromLocalConfig(hookCfg *config.HookConfig, globalCfg *config.Config) *Hook {
	h := &Hook{
		ID:       hookCfg.ID,
		Name:     hookCfg.Name,
		Entry:    hookCfg.Entry,
		Language: hookCfg.Language,
		Repo:     "local",
		Alias:    hookCfg.Alias,
	}

	if hookCfg.Files != "" {
		h.Files = hookCfg.Files
	}
	if hookCfg.Exclude != "" {
		h.Exclude = hookCfg.Exclude
	}
	if len(hookCfg.Types) > 0 {
		h.Types = hookCfg.Types
	}
	if len(hookCfg.TypesOr) > 0 {
		h.TypesOr = hookCfg.TypesOr
	}
	if len(hookCfg.ExcludeTypes) > 0 {
		h.ExcludeTypes = hookCfg.ExcludeTypes
	}
	if len(hookCfg.Args) > 0 {
		h.Args = hookCfg.Args
	}
	if len(hookCfg.Stages) > 0 {
		h.Stages = hookCfg.Stages
	}
	if len(hookCfg.AdditionalDependencies) > 0 {
		h.AdditionalDependencies = hookCfg.AdditionalDependencies
	}
	if hookCfg.AlwaysRun != nil {
		h.AlwaysRun = *hookCfg.AlwaysRun
	}
	if hookCfg.Verbose != nil {
		h.Verbose = *hookCfg.Verbose
	}
	if hookCfg.PassFilenames != nil {
		h.PassFilenames = *hookCfg.PassFilenames
	} else {
		h.PassFilenames = true
	}
	if hookCfg.RequireSerial != nil {
		h.RequireSerial = *hookCfg.RequireSerial
	}
	if hookCfg.FailFast != nil {
		h.FailFast = *hookCfg.FailFast
	}
	if hookCfg.LanguageVersion != "" {
		h.LanguageVersion = hookCfg.LanguageVersion
	}
	if hookCfg.LogFile != "" {
		h.LogFile = hookCfg.LogFile
	}
	if hookCfg.Description != "" {
		h.Description = hookCfg.Description
	}

	// Apply defaults.
	if len(h.Types) == 0 && len(h.TypesOr) == 0 {
		h.Types = []string{"file"}
	}

	// Apply global defaults.
	if globalCfg != nil {
		if len(h.Stages) == 0 && len(globalCfg.DefaultStages) > 0 {
			h.Stages = globalCfg.DefaultStages
		}
	}
	resolveLanguageVersion(h, globalCfg)

	return h
}

// FromManifestHook creates a Hook directly from a manifest hook (used by try-repo).
func FromManifestHook(manifest *config.ManifestHook) *Hook {
	h := &Hook{
		ID:                      manifest.ID,
		Name:                    manifest.Name,
		Entry:                   manifest.Entry,
		Language:                manifest.Language,
		LanguageVersion:         manifest.LanguageVersion,
		Files:                   manifest.Files,
		Exclude:                 manifest.Exclude,
		Types:                   manifest.Types,
		TypesOr:                 manifest.TypesOr,
		ExcludeTypes:            manifest.ExcludeTypes,
		Args:                    manifest.Args,
		Stages:                  manifest.Stages,
		AdditionalDependencies:  manifest.AdditionalDependencies,
		AlwaysRun:               manifest.AlwaysRun,
		FailFast:                manifest.FailFast,
		Verbose:                 manifest.Verbose,
		PassFilenames:           manifest.DefaultPassFilenames(),
		RequireSerial:           manifest.RequireSerial,
		Description:             manifest.Description,
		MinimumPreCommitVersion: manifest.MinimumPreCommitVersion,
	}

	if len(h.Types) == 0 && len(h.TypesOr) == 0 {
		h.Types = []string{"file"}
	}
	resolveLanguageVersion(h, nil)

	return h
}

// resolveLanguageVersion is upstream's repository._hook (4.7.0): a
// `language_version` of `default` (or none) becomes the config's
// default_language_version for the language, and if that is `default` too,
// the language's get_default_version — so the version the env directory is
// named after, and the install and health check see, is the concrete one
// upstream uses (`node_env-system`, `py_env-python3.12`, …).
func resolveLanguageVersion(h *Hook, globalCfg *config.Config) {
	if h.LanguageVersion == "" {
		h.LanguageVersion = "default"
	}
	if h.LanguageVersion == "default" && globalCfg != nil {
		if v := globalCfg.DefaultLanguageVersion[h.Language]; v != "" {
			h.LanguageVersion = v
		}
	}
	if h.LanguageVersion == "default" {
		if lang, err := languages.Get(h.Language); err == nil {
			h.LanguageVersion = lang.GetDefaultVersion()
		}
	}
}
