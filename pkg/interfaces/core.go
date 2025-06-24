// Package interfaces defines core interfaces for the repository system.
// This package provides interfaces for repository management, caching, and language handling.
package interfaces

import (
	"context"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/language"
)

// CacheManager defines the interface for cache operations
type CacheManager interface {
	// GetRepoPath returns the cached path for a repository
	GetRepoPath(repo config.Repo) string

	// GetRepoPathWithDeps returns the cached path considering dependencies
	GetRepoPathWithDeps(repo config.Repo, additionalDeps []string) string

	// UpdateRepoEntry updates the database entry for a repository
	UpdateRepoEntry(repo config.Repo, path string) error

	// UpdateRepoEntryWithDeps updates the database entry with dependencies
	UpdateRepoEntryWithDeps(repo config.Repo, additionalDeps []string, path string) error

	// CleanCache removes all cached repositories
	CleanCache() error

	// MarkConfigUsed marks a config file as used
	MarkConfigUsed(configPath string) error

	// Close closes the cache manager
	Close() error
}

// RepositoryManager defines the interface for repository operations
type RepositoryManager interface {
	// CloneOrUpdateRepo clones or updates a repository
	CloneOrUpdateRepo(ctx context.Context, repo config.Repo) (string, error)

	// CloneOrUpdateRepoWithDeps clones/updates with dependency awareness
	CloneOrUpdateRepoWithDeps(ctx context.Context, repo config.Repo, additionalDeps []string) (string, error)

	// SetupHookEnvironment sets up the environment for a specific hook
	SetupHookEnvironment(hook config.Hook, repo config.Repo, repoPath string) (map[string]string, error)

	// IsLocalRepo checks if the repository is local
	IsLocalRepo(repo config.Repo) bool

	// IsMetaRepo checks if the repository is meta
	IsMetaRepo(repo config.Repo) bool

	// Common repository management operations
	InitializeRepositoryCommon(repoURL, repoRef, repoPath string) error
	UpdateDatabaseEntry(repoURL, repoRef, repoPath string) error
}

// DownloadManager interface for downloading and extracting files
type DownloadManager interface {
	DownloadFile(url, dest string) error
	ExtractTarGz(archivePath, destDir string) error
	ExtractZip(archivePath, destDir string) error
	InstallBinary(srcPath, envPath, binaryName string) error
	MakeBinaryExecutable(path string) error
	GetNormalizedOS() string
	GetNormalizedArch() string
}

// PackageManager interface for managing package manifests and dependencies
type PackageManager interface {
	CreateManifest(envPath string, manifest any) error
	RunInstallCommand(envPath string, manifestType any) error
	CheckManifestExists(envPath string, manifestType any) bool
}

// LanguageManager is an alias for language.LanguageManager for backward compatibility
type LanguageManager = language.Manager

// StateManager interface for managing state
type StateManager interface {
	GetStatistics() map[string]any
	IsEnvironmentInitialized(envKey string) bool
	IsEnvironmentInstalling(envKey string) bool
	MarkEnvironmentInstalling(envKey string) error
	ClearEnvironmentInstalling(envKey string)
	MarkEnvironmentInitialized(envKey string)
	GetEnvironmentStats() map[string]any
	Reset()
}
