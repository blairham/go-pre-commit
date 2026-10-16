// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// GCCommand implements the "gc" command.
type GCCommand struct {
	Meta *Meta
}

func (c *GCCommand) Run(args []string) int {
	var opts GlobalFlags
	_, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	s := store.New("")

	// Gather used repos from all known config files.
	usedRepos := make(map[string]bool)

	// Always check the current config.
	configPaths := []string{opts.Config}

	// Also check any configs tracked by MarkConfigUsed.
	trackedConfigs, err := s.GetTrackedConfigs()
	if err == nil {
		for _, c := range trackedConfigs {
			// Deduplicate.
			found := false
			for _, existing := range configPaths {
				if existing == c {
					found = true
					break
				}
			}
			if !found {
				configPaths = append(configPaths, c)
			}
		}
	}

	tracked := make(map[string]bool, len(trackedConfigs))
	for _, c := range trackedConfigs {
		tracked[c] = true
	}

	// A tracked config that no longer loads is dropped from the list, as
	// upstream's _gc deletes its dead configs rows.
	var dead []string
	for _, cfgPath := range configPaths {
		cfg, err := config.LoadConfig(cfgPath)
		if err != nil {
			if tracked[cfgPath] {
				dead = append(dead, cfgPath)
			}
			continue
		}
		for _, repo := range cfg.Repos {
			switch {
			case repo.IsMeta():
				// Meta hooks are built in — nothing cached.
			case repo.IsLocal():
				// Local hooks that install an environment are backed by a
				// synthetic repo keyed on their additional_dependencies.
				for _, h := range repo.Hooks {
					usedRepos[store.LocalRepoKey(h.AdditionalDependencies)] = true
				}
			default:
				usedRepos[repo.Repo+"@"+repo.Rev] = true
				markDepsClonesUsed(s, repo, usedRepos)
			}
		}
	}

	if err := s.ForgetConfigs(dead); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to run GC: %v\n", err)
		return 1
	}
	if err := s.GC(usedRepos); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to run GC: %v\n", err)
		return 1
	}
	fmt.Println("Garbage collection complete.")
	return 0
}

func (c *GCCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit gc [options]

  Clean unused cached repos. Repos that are no longer referenced by any
  config file will be removed from the cache.

Options:

  -c, --config=FILE   Path to alternate config file.
      --color=MODE    Whether to use color (auto, always, never).
`)
}

func (c *GCCommand) Synopsis() string {
	return "Clean unused cached repos"
}

// markDepsClonesUsed is the dependency half of upstream's _mark_used_repos: a
// hook whose additional_dependencies (its own, or else the manifest's) are
// non-empty runs from a clone of its own, recorded under DBRepoName, and that
// clone is in use too. Like upstream, it needs the repo's manifest to know the
// manifest's dependencies, so a repo that is not cloned marks nothing more.
func markDepsClonesUsed(s *store.Store, repo config.RepoConfig, used map[string]bool) {
	path := s.GetPath(repo.Repo, repo.Rev)
	if path == "" {
		return
	}
	manifest, err := config.LoadManifest(filepath.Join(path, config.ManifestFile))
	if err != nil {
		return
	}
	byID := make(map[string]*config.ManifestHook, len(manifest))
	for i := range manifest {
		byID[manifest[i].ID] = &manifest[i]
	}
	for _, h := range repo.Hooks {
		mh, ok := byID[h.ID]
		if !ok {
			continue
		}
		deps := h.AdditionalDependencies
		if deps == nil {
			deps = mh.AdditionalDependencies
		}
		used[store.DBRepoName(repo.Repo, deps)+"@"+repo.Rev] = true
	}
}
