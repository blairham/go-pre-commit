package config

// WellKnownRepositories contains hook definitions for common repositories
// This is used to populate hook language information when loading configs
// without having to clone the actual repositories.
var WellKnownRepositories = map[string]map[string]Hook{
	"https://github.com/psf/black": {
		"black": {
			ID:       "black",
			Name:     "black",
			Entry:    "black",
			Language: "python",
		},
	},
	"https://github.com/pycqa/flake8": {
		"flake8": {
			ID:       "flake8",
			Name:     "flake8",
			Entry:    "flake8",
			Language: "python",
		},
	},
	"https://github.com/pre-commit/mirrors-eslint": {
		"eslint": {
			ID:       "eslint",
			Name:     "eslint",
			Entry:    "eslint",
			Language: "node",
		},
	},
	"https://github.com/dnephin/go-pre-commitlang": {
		"go-fmt": {
			ID:       "go-fmt",
			Name:     "go-fmt",
			Entry:    "gofmt",
			Language: "golang",
		},
		"go-vet-mod": {
			ID:       "go-vet-mod",
			Name:     "go-vet-mod",
			Entry:    "go vet",
			Language: "golang",
		},
		"go-mod-tidy": {
			ID:       "go-mod-tidy",
			Name:     "go-mod-tidy",
			Entry:    "go mod tidy",
			Language: "golang",
		},
	},
	"https://github.com/doublify/pre-commit-rust": {
		"fmt": {
			ID:       "fmt",
			Name:     "fmt",
			Entry:    "cargo fmt",
			Language: "rust",
		},
		"cargo-check": {
			ID:       "cargo-check",
			Name:     "cargo-check",
			Entry:    "cargo check",
			Language: "rust",
		},
	},
	"https://github.com/mattlqx/pre-commit-ruby": {
		"rubocop": {
			ID:       "rubocop",
			Name:     "rubocop",
			Entry:    "rubocop",
			Language: "ruby",
		},
	},
	"https://github.com/pre-commit/pre-commit-hooks": {
		"trailing-whitespace": {
			ID:       "trailing-whitespace",
			Name:     "Trim Trailing Whitespace",
			Entry:    "trailing-whitespace-fixer",
			Language: "python",
		},
		"end-of-file-fixer": {
			ID:       "end-of-file-fixer",
			Name:     "Fix End of Files",
			Entry:    "end-of-file-fixer",
			Language: "python",
		},
		"check-yaml": {
			ID:       "check-yaml",
			Name:     "Check Yaml",
			Entry:    "check-yaml",
			Language: "python",
		},
		"check-json": {
			ID:       "check-json",
			Name:     "Check JSON",
			Entry:    "check-json",
			Language: "python",
		},
		"check-toml": {
			ID:       "check-toml",
			Name:     "Check Toml",
			Entry:    "check-toml",
			Language: "python",
		},
		"check-xml": {
			ID:       "check-xml",
			Name:     "Check Xml",
			Entry:    "check-xml",
			Language: "python",
		},
	},
}

// GetWellKnownHook returns a hook definition from a well-known repository
func GetWellKnownHook(repoURL, hookID string) (Hook, bool) {
	if repoHooks, exists := WellKnownRepositories[repoURL]; exists {
		if hook, hookExists := repoHooks[hookID]; hookExists {
			return hook, true
		}
	}
	return Hook{}, false
}

// PopulateHookDefinitions populates missing hook information from well-known repositories
func (c *Config) PopulateHookDefinitions() error {
	for i := range c.Repos {
		repo := &c.Repos[i]

		// Skip local and meta repositories
		if repo.Repo == "local" || repo.Repo == "meta" {
			continue
		}

		for j := range repo.Hooks {
			hook := &repo.Hooks[j]

			// Only populate if language is not already set
			if hook.Language == "" {
				c.populateHookFromWellKnown(hook, repo.Repo)
			}
		}
	}

	return nil
}

// populateHookFromWellKnown fills in hook details from well-known repositories
func (c *Config) populateHookFromWellKnown(hook *Hook, repoURL string) {
	if wellKnownHook, exists := GetWellKnownHook(repoURL, hook.ID); exists {
		// Merge the well-known hook definition with the configured hook
		if hook.Name == "" {
			hook.Name = wellKnownHook.Name
		}
		if hook.Entry == "" {
			hook.Entry = wellKnownHook.Entry
		}
		hook.Language = wellKnownHook.Language
	}
}
