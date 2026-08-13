// Package repository resolves hooks from config repo definitions.
package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/hook"
	"github.com/blairham/go-pre-commit/v4/internal/languages"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// Resolver resolves hook configurations into executable hooks.
type Resolver struct {
	Store *store.Store
	Cfg   *config.Config
}

// NewResolver creates a new Resolver.
func NewResolver(s *store.Store, cfg *config.Config) *Resolver {
	return &Resolver{Store: s, Cfg: cfg}
}

// ResolveAll resolves all repos in a config into a flat list of hooks.
// Remote repos are cloned in parallel to reduce wall-clock time.
func (r *Resolver) ResolveAll(ctx context.Context, cfg *config.Config) ([]*hook.Hook, error) {
	type repoResult struct {
		hooks []*hook.Hook
		err   error
		index int
	}

	// Separate repos: local/meta can resolve instantly, remote needs cloning.
	results := make([]repoResult, len(cfg.Repos))

	// Resolve remote repos in parallel (cloning is the bottleneck).
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4) // Limit concurrent clones.

	for i := range cfg.Repos {
		repo := &cfg.Repos[i]
		if repo.IsLocal() || repo.IsMeta() {
			// Resolve local/meta repos inline — meta needs no I/O, and local
			// only touches the shared store directory, which is serialized
			// anyway.
			hooks, err := r.resolveRepo(ctx, repo)
			results[i] = repoResult{hooks: hooks, err: err, index: i}
			continue
		}
		wg.Add(1)
		go func(idx int, repo *config.RepoConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			hooks, err := r.resolveRepo(ctx, repo)
			results[idx] = repoResult{hooks: hooks, err: err, index: idx}
		}(i, repo)
	}
	wg.Wait()

	// Collect results in config order.
	var allHooks []*hook.Hook
	for i, res := range results {
		if res.err != nil {
			return nil, fmt.Errorf("resolving repo %s: %w", cfg.Repos[i].Repo, res.err)
		}
		allHooks = append(allHooks, res.hooks...)
	}

	return allHooks, nil
}

func (r *Resolver) resolveRepo(ctx context.Context, repo *config.RepoConfig) ([]*hook.Hook, error) {
	if repo.IsLocal() {
		return r.resolveLocalRepo(repo)
	}
	if repo.IsMeta() {
		return r.resolveMetaRepo(repo)
	}
	return r.resolveRemoteRepo(ctx, repo)
}

func (r *Resolver) resolveLocalRepo(repo *config.RepoConfig) ([]*hook.Hook, error) {
	var hooks []*hook.Hook
	for i := range repo.Hooks {
		h := hook.FromLocalConfig(&repo.Hooks[i], r.Cfg)
		if err := r.setLocalPrefix(h); err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, nil
}

// setLocalPrefix gives a local hook the directory its environment is built in.
// A `repo: local` hook has nothing to clone, but a language that installs an
// environment still needs a prefix — otherwise additional_dependencies are
// never installed and the entry silently resolves against whatever is on PATH.
// Languages that install nothing (system, script, pygrep, …) keep an empty
// prefix and run from the working directory, as upstream does.
func (r *Resolver) setLocalPrefix(h *hook.Hook) error {
	lang, err := languages.Get(h.Language)
	if err != nil {
		// Unknown language — the runner reports it with better context than a
		// resolution-time error would.
		return nil
	}

	if lang.EnvironmentDir() == "" {
		return checkAdditionalDependencies(h)
	}

	if r.Store == nil {
		return nil
	}

	dir, err := r.Store.MakeLocal(h.AdditionalDependencies)
	if err != nil {
		return fmt.Errorf("creating environment for hook %q: %w", h.ID, err)
	}
	h.RepoDir = dir
	return nil
}

// checkAdditionalDependencies mirrors Python pre-commit: declaring
// additional_dependencies for a language that installs no environment is a
// config error, not something to drop on the floor.
func checkAdditionalDependencies(h *hook.Hook) error {
	if len(h.AdditionalDependencies) == 0 {
		return nil
	}
	lang, err := languages.Get(h.Language)
	if err != nil || lang.EnvironmentDir() != "" {
		return nil
	}
	return fmt.Errorf(
		"the hook `%s` specifies `additional_dependencies` but is using language `%s` "+
			"which does not install an environment. "+
			"Perhaps you meant to use a specific language?",
		h.ID, h.Language)
}

func (r *Resolver) resolveMetaRepo(repo *config.RepoConfig) ([]*hook.Hook, error) {
	var hooks []*hook.Hook
	for _, hc := range repo.Hooks {
		h, err := makeMetaHook(&hc)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}
	return hooks, nil
}

func makeMetaHook(hc *config.HookConfig) (*hook.Hook, error) {
	switch hc.ID {
	case "identity":
		return &hook.Hook{
			ID:            "identity",
			Name:          "identity",
			Language:      "system",
			Entry:         "echo",
			AlwaysRun:     true,
			Verbose:       true,
			Stages:        []config.Stage{config.HookTypePreCommit},
			Types:         []string{"file"},
			PassFilenames: true,
		}, nil
	case "check-hooks-apply":
		// This meta hook checks that all hooks in the config match at least one file.
		// It uses a special language="fail" with an entry that will be replaced at runtime
		// by the runner, which checks hook file matching. In practice, we implement
		// the check in the runner itself via AlwaysRun + the meta ID.
		return &hook.Hook{
			ID:            "check-hooks-apply",
			Name:          "check hooks apply to the repository",
			Language:      "system",
			Entry:         "pre-commit-meta-check-hooks-apply",
			AlwaysRun:     true,
			PassFilenames: false,
			Stages:        []config.Stage{config.HookTypePreCommit},
			Types:         []string{"file"},
		}, nil
	case "check-useless-excludes":
		return &hook.Hook{
			ID:            "check-useless-excludes",
			Name:          "check for useless excludes",
			Language:      "system",
			Entry:         "pre-commit-meta-check-useless-excludes",
			AlwaysRun:     true,
			PassFilenames: false,
			Stages:        []config.Stage{config.HookTypePreCommit},
			Types:         []string{"file"},
		}, nil
	default:
		return nil, fmt.Errorf("unknown meta hook: %s", hc.ID)
	}
}

func (r *Resolver) resolveRemoteRepo(ctx context.Context, repo *config.RepoConfig) ([]*hook.Hook, error) {
	// Clone (or retrieve cached clone) via the store.
	repoDir, err := r.Store.Clone(repo.Repo, repo.Rev)
	if err != nil {
		return nil, fmt.Errorf("cloning %s@%s: %w", repo.Repo, repo.Rev, err)
	}

	// Read manifest from the cloned repo.
	manifest, err := loadManifest(repoDir)
	if err != nil {
		return nil, fmt.Errorf("loading manifest from %s: %w", repo.Repo, err)
	}

	// Build a map of manifest hooks by ID.
	manifestByID := make(map[string]*config.ManifestHook, len(manifest))
	for i := range manifest {
		manifestByID[manifest[i].ID] = &manifest[i]
	}

	// Resolve each hook in the repo config.
	var hooks []*hook.Hook
	for i := range repo.Hooks {
		hc := &repo.Hooks[i]
		mh, ok := manifestByID[hc.ID]
		if !ok {
			return nil, fmt.Errorf("hook %q not found in manifest for %s@%s", hc.ID, repo.Repo, repo.Rev)
		}

		h := hook.MergeManifest(mh, hc, repo, r.Cfg)
		h.RepoDir = repoDir
		if err := checkAdditionalDependencies(h); err != nil {
			return nil, err
		}
		hooks = append(hooks, h)
	}

	return hooks, nil
}

func loadManifest(repoDir string) ([]config.ManifestHook, error) {
	// Try .pre-commit-hooks.yaml first.
	manifestPath := filepath.Join(repoDir, ".pre-commit-hooks.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		// Fall back to hooks.yaml.
		manifestPath = filepath.Join(repoDir, "hooks.yaml")
		if _, err := os.Stat(manifestPath); err != nil {
			return nil, fmt.Errorf("no manifest file found in %s", repoDir)
		}
	}
	return config.LoadManifest(manifestPath)
}
