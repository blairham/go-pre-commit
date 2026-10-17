// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// GCCommand implements the "gc" command.
type GCCommand struct {
	Meta *Meta
}

// gcFlags is upstream's gc parser: _add_cmd adds only --color, so gc has no
// -c/--config and takes no positional arguments.
type gcFlags struct {
	Color string `long:"color" default:"auto" description:"Whether to use color in output. Options: auto, always, never."`
}

func (c *GCCommand) Run(args []string) int {
	var opts gcFlags
	if _, code, ok := parseArgs("gc", &opts, args, args); !ok {
		return code
	}

	s := store.New("")

	// Upstream's _gc reads only the configs recorded in the store: a repo
	// counts as used because a config that ran it is still on that list, not
	// because of whichever config sits in the current directory.
	usedRepos := make(map[string]bool)
	configPaths, err := s.GetTrackedConfigs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to run GC: %v\n", err)
		return 1
	}

	// A tracked config that no longer loads is dropped from the list, as
	// upstream's _gc deletes its dead configs rows.
	var dead []string
	for _, cfgPath := range configPaths {
		cfg, err := config.LoadConfig(cfgPath)
		if err != nil {
			dead = append(dead, cfgPath)
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
	removed, err := s.GC(usedRepos)
	if err != nil {
		return reportError(err)
	}
	fmt.Printf("%d repo(s) removed.\n", removed)
	return 0
}

func (c *GCCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit gc [options]

  Clean unused cached repos. Repos that are no longer referenced by any
  config file that has used this cache will be removed from it.

Options:

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
