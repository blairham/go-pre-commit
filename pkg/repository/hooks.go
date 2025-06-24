package repository

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// HookManager handles hook definitions and meta hooks
type HookManager struct{}

// NewHookManager creates a new hook manager
func NewHookManager() *HookManager {
	return &HookManager{}
}

// IsMetaRepo checks if a repository is a meta/built-in repository
func (hm *HookManager) IsMetaRepo(repo config.Repo) bool {
	return repo.Repo == "meta"
}

// IsLocalRepo checks if a repository is local
func (hm *HookManager) IsLocalRepo(repo config.Repo) bool {
	return repo.Repo == "local"
}

// GetMetaHook returns a built-in meta hook definition
func (hm *HookManager) GetMetaHook(hookID string) (config.Hook, bool) {
	// Define built-in meta hooks
	metaHooks := map[string]config.Hook{
		"check-added-large-files": {
			ID:       "check-added-large-files",
			Name:     "Check for added large files",
			Entry:    "check-added-large-files",
			Language: "system",
		},
		"check-case-conflict": {
			ID:       "check-case-conflict",
			Name:     "Check for case conflicts",
			Entry:    "check-case-conflict",
			Language: "system",
		},
		"check-merge-conflict": {
			ID:       "check-merge-conflict",
			Name:     "Check for merge conflicts",
			Entry:    "check-merge-conflict",
			Language: "system",
		},
		"check-yaml": {
			ID:       "check-yaml",
			Name:     "Check YAML",
			Entry:    "check-yaml",
			Language: "system",
		},
		"check-json": {
			ID:       "check-json",
			Name:     "Check JSON",
			Entry:    "check-json",
			Language: "system",
		},
		"check-toml": {
			ID:       "check-toml",
			Name:     "Check TOML",
			Entry:    "check-toml",
			Language: "system",
		},
		"check-xml": {
			ID:       "check-xml",
			Name:     "Check XML",
			Entry:    "check-xml",
			Language: "system",
		},
		"end-of-file-fixer": {
			ID:       "end-of-file-fixer",
			Name:     "Fix End of Files",
			Entry:    "end-of-file-fixer",
			Language: "system",
		},
		"trailing-whitespace": {
			ID:       "trailing-whitespace",
			Name:     "Trim Trailing Whitespace",
			Entry:    "trailing-whitespace",
			Language: "system",
		},
		"mixed-line-ending": {
			ID:       "mixed-line-ending",
			Name:     "Mixed line ending",
			Entry:    "mixed-line-ending",
			Language: "system",
		},
		"check-executables-have-shebangs": {
			ID:       "check-executables-have-shebangs",
			Name:     "Check that executables have shebangs",
			Entry:    "check-executables-have-shebangs",
			Language: "system",
		},
	}

	hook, exists := metaHooks[hookID]
	return hook, exists
}

// GetRepositoryHook loads a hook definition from a repository's .pre-commit-hooks.yaml
func (hm *HookManager) GetRepositoryHook(repoPath, hookID string) (config.Hook, bool) {
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	data, err := os.ReadFile(hooksFile) // #nosec G304 -- reading repository hook configs
	if err != nil {
		// Try alternative file names
		hooksFile = filepath.Join(repoPath, ".pre-commit-hooks.yml")
		data, err = os.ReadFile(hooksFile) // #nosec G304 -- reading repository hook configs
		if err != nil {
			return config.Hook{}, false
		}
	}

	var hooks []config.Hook
	if err := yaml.Unmarshal(data, &hooks); err != nil {
		return config.Hook{}, false
	}

	for _, hook := range hooks {
		if hook.ID == hookID {
			return hook, true
		}
	}

	return config.Hook{}, false
}

// GetHookExecutablePath returns the path to a hook's executable within a repository
func (hm *HookManager) GetHookExecutablePath(repoPath string, hook config.Hook) (string, error) {
	// If entry is specified, use it as the executable path
	if hook.Entry != "" {
		// If entry is an absolute path, return it
		if filepath.IsAbs(hook.Entry) {
			return hook.Entry, nil
		}

		// Otherwise, resolve relative to repository path
		execPath := filepath.Join(repoPath, hook.Entry)
		if _, err := os.Stat(execPath); err == nil {
			return execPath, nil
		}
	}

	// Fall back to hook ID as executable name
	execPath := filepath.Join(repoPath, hook.ID)
	if _, err := os.Stat(execPath); err == nil {
		return execPath, nil
	}

	// Check common executable locations
	for _, dir := range []string{"bin", "scripts", "."} {
		execPath := filepath.Join(repoPath, dir, hook.ID)
		if _, err := os.Stat(execPath); err == nil {
			return execPath, nil
		}
	}

	// If not found, return the hook ID (let the system handle it)
	return hook.ID, nil
}
