// Package repository provides functionality for managing pre-commit hook repositories
// and their associated environments.
package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/cache"
	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/environment"
)

// Manager handles repository management and hook environment setup
type Manager struct {
	cacheManager   *cache.Manager
	repositoryOps  *Operations
	environmentMgr *environment.Manager
	hookMgr        *HookManager
	cacheDir       string
}

// NewManager creates a new repository manager
func NewManager() (*Manager, error) {
	var cacheDir string

	// Check PRE_COMMIT_HOME environment variable first (like Python pre-commit)
	if preCommitHome := os.Getenv("PRE_COMMIT_HOME"); preCommitHome != "" {
		cacheDir = preCommitHome
	} else if xdgCacheHome := os.Getenv("XDG_CACHE_HOME"); xdgCacheHome != "" {
		// Check XDG_CACHE_HOME as fallback (like Python pre-commit)
		// Python pre-commit uses XDG_CACHE_HOME/pre-commit, so we add the pre-commit subdirectory
		cacheDir = filepath.Join(xdgCacheHome, "pre-commit")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		cacheDir = filepath.Join(homeDir, ".cache", "pre-commit")
	}

	if mkdirErr := os.MkdirAll(cacheDir, 0o750); mkdirErr != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", mkdirErr)
	}

	// Initialize the cache manager
	cacheManager, err := cache.NewManager(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache manager: %w", err)
	}

	// Initialize other components
	repositoryOps := NewRepositoryOperations(cacheManager)
	environmentMgr := environment.NewManager(cacheDir)
	hookMgr := NewHookManager()

	return &Manager{
		cacheManager:   cacheManager,
		repositoryOps:  repositoryOps,
		environmentMgr: environmentMgr,
		hookMgr:        hookMgr,
		cacheDir:       cacheDir,
	}, nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	return m.cacheManager.Close()
}

// GetCacheDir returns the cache directory path
func (m *Manager) GetCacheDir() string {
	return m.cacheDir
}

// GetRepoPath returns the path where a repository should be cached
func (m *Manager) GetRepoPath(repo config.Repo) string {
	return m.cacheManager.GetRepoPath(repo)
}

// GetRepoPathWithDeps returns the path where a repository should be cached, considering additional dependencies
func (m *Manager) GetRepoPathWithDeps(repo config.Repo, additionalDeps []string) string {
	return m.cacheManager.GetRepoPathWithDeps(repo, additionalDeps)
}

// CloneOrUpdateRepo ensures a repository is cloned and at the correct revision
func (m *Manager) CloneOrUpdateRepo(ctx context.Context, repo config.Repo) (string, error) {
	return m.repositoryOps.CloneOrUpdateRepo(ctx, repo)
}

// CloneOrUpdateRepoWithDeps ensures a repository is cloned and at the correct revision, considering additional dependencies
func (m *Manager) CloneOrUpdateRepoWithDeps(
	ctx context.Context,
	repo config.Repo,
	additionalDeps []string,
) (string, error) {
	return m.repositoryOps.CloneOrUpdateRepoWithDeps(ctx, repo, additionalDeps)
}

// CleanCache removes all cached repositories
func (m *Manager) CleanCache() error {
	return m.cacheManager.CleanCache()
}

// IsMetaRepo checks if a repository is a meta/built-in repository
func (m *Manager) IsMetaRepo(repo config.Repo) bool {
	return m.hookMgr.IsMetaRepo(repo)
}

// IsLocalRepo checks if a repository is local
func (m *Manager) IsLocalRepo(repo config.Repo) bool {
	return m.hookMgr.IsLocalRepo(repo)
}

// GetMetaHook returns a built-in meta hook definition
func (m *Manager) GetMetaHook(hookID string) (config.Hook, bool) {
	return m.hookMgr.GetMetaHook(hookID)
}

// GetRepositoryHook loads a hook definition from a repository's .pre-commit-hooks.yaml
func (m *Manager) GetRepositoryHook(repoPath, hookID string) (config.Hook, bool) {
	return m.hookMgr.GetRepositoryHook(repoPath, hookID)
}

// SetupHookEnvironment sets up the environment for running a hook
func (m *Manager) SetupHookEnvironment(
	hook config.Hook,
	repo config.Repo,
	repoPath string,
) (map[string]string, error) {
	return m.environmentMgr.SetupHookEnvironment(hook, repo, repoPath)
}

// GetHookExecutablePath returns the path to a hook's executable within a repository
func (m *Manager) GetHookExecutablePath(repoPath string, hook config.Hook) (string, error) {
	return m.hookMgr.GetHookExecutablePath(repoPath, hook)
}

// CheckEnvironmentHealthWithRepo checks if a language environment is healthy within a repository context
func (m *Manager) CheckEnvironmentHealthWithRepo(language, version, repoPath string) error {
	return m.environmentMgr.CheckEnvironmentHealthWithRepo(language, version, repoPath)
}

// RebuildEnvironmentWithRepo rebuilds a language environment within a repository context
func (m *Manager) RebuildEnvironmentWithRepo(language, version, repoPath string) error {
	return m.environmentMgr.RebuildEnvironmentWithRepo(language, version, repoPath)
}

// RebuildEnvironmentWithRepoInfo rebuilds a language environment within a repository context with repo URL
func (m *Manager) RebuildEnvironmentWithRepoInfo(
	language, version, repoPath, repoURL string,
) error {
	return m.environmentMgr.RebuildEnvironmentWithRepoInfo(language, version, repoPath, repoURL)
}

// MarkConfigUsed marks a config file as used in the database (like Python pre-commit)
func (m *Manager) MarkConfigUsed(configPath string) error {
	return m.cacheManager.MarkConfigUsed(configPath)
}

// UpdateRepoEntryWithDeps updates the database entry for a repository with dependencies
func (m *Manager) UpdateRepoEntryWithDeps(
	repo config.Repo,
	additionalDeps []string,
	path string,
) error {
	return m.cacheManager.UpdateRepoEntryWithDeps(repo, additionalDeps, path)
}

// PreInitializeHookEnvironments performs the pre-initialization phase for all hook environments
func (m *Manager) PreInitializeHookEnvironments(
	ctx context.Context,
	hooks []config.HookEnvItem,
) error {
	return m.environmentMgr.PreInitializeHookEnvironments(ctx, hooks, m.repositoryOps)
}

// SetupEnvironmentWithRepositoryInit sets up an environment assuming the repository is already initialized
func (m *Manager) SetupEnvironmentWithRepositoryInit(
	repo config.Repo, language, version string, additionalDeps []string,
) (string, error) {
	return m.environmentMgr.SetupEnvironmentWithRepositoryInit(
		repo,
		language,
		version,
		additionalDeps,
	)
}

// GetCommonRepositoryManager returns a repository manager interface that languages can use
func (m *Manager) GetCommonRepositoryManager(
	ctx context.Context,
) any {
	return m.environmentMgr.GetCommonRepositoryManager(ctx, m.repositoryOps)
}
