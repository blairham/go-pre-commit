// Package repository resolves hooks from config repo definitions.
package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blairham/go-pre-commit/internal/config"
	"github.com/blairham/go-pre-commit/internal/hook"
	"github.com/blairham/go-pre-commit/internal/store"
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
func (r *Resolver) ResolveAll(ctx context.Context, cfg *config.Config) ([]*hook.Hook, error) {
	var allHooks []*hook.Hook

	for i := range cfg.Repos {
		hooks, err := r.resolveRepo(ctx, &cfg.Repos[i])
		if err != nil {
			return nil, fmt.Errorf("resolving repo %s: %w", cfg.Repos[i].Repo, err)
		}
		allHooks = append(allHooks, hooks...)
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
		hooks = append(hooks, h)
	}
	return hooks, nil
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
