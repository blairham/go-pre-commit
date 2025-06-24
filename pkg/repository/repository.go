package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/blairham/go-pre-commit/pkg/cache"
	"github.com/blairham/go-pre-commit/pkg/config"
)

// Operations handles Git repository cloning and updating operations
type Operations struct {
	cacheManager *cache.Manager
}

// NewRepositoryOperations creates a new repository operations handler
func NewRepositoryOperations(cacheManager *cache.Manager) *Operations {
	return &Operations{
		cacheManager: cacheManager,
	}
}

// CloneOrUpdateRepo ensures a repository is cloned and at the correct revision
func (ops *Operations) CloneOrUpdateRepo(ctx context.Context, repo config.Repo) (string, error) {
	//nolint:contextcheck // Cache operations are local and don't need context cancellation
	repoPath := ops.cacheManager.GetRepoPath(repo)

	// Check if repository already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		// Repository exists, check if we need to update
		if err := ops.updateRepo(repoPath, repo.Rev); err != nil {
			// If update fails, remove and re-clone with locking
			if rmErr := os.RemoveAll(repoPath); rmErr != nil {
				fmt.Printf("Warning: failed to remove repository directory: %v\n", rmErr)
			}
			return ops.cloneWithLock(ctx, repo, repoPath)
		}
		return repoPath, nil
	}

	// Repository doesn't exist, clone it with file-based locking
	return ops.cloneWithLock(ctx, repo, repoPath)
}

// CloneOrUpdateRepoWithDeps ensures a repository is cloned and at the correct revision, considering additional dependencies
func (ops *Operations) CloneOrUpdateRepoWithDeps(
	ctx context.Context,
	repo config.Repo,
	additionalDeps []string,
) (string, error) {
	//nolint:contextcheck // Cache operations are local and don't need context cancellation
	repoPath := ops.cacheManager.GetRepoPathWithDeps(repo, additionalDeps)

	// Check if repository already exists
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
		// Repository exists, check if we need to update
		if err := ops.updateRepo(repoPath, repo.Rev); err != nil {
			// If update fails, remove and re-clone with locking
			if rmErr := os.RemoveAll(repoPath); rmErr != nil {
				fmt.Printf("Warning: failed to remove repository directory: %v\n", rmErr)
			}
			return ops.cloneWithLockAndDeps(ctx, repo, repoPath, additionalDeps)
		}
		return repoPath, nil
	}

	// Repository doesn't exist, clone it with file-based locking
	return ops.cloneWithLockAndDeps(ctx, repo, repoPath, additionalDeps)
}

// cloneRepo clones a repository to the specified path using go-git
func (ops *Operations) cloneRepo(ctx context.Context, repo config.Repo, repoPath string) (string, error) {
	// Check context before starting
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(repoPath), 0o750); err != nil {
		return "", fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Clone with tags included
	cloneOptions := &git.CloneOptions{
		URL:  repo.Repo,
		Tags: git.AllTags, // Fetch all tags
	}

	gitRepo, err := git.PlainClone(repoPath, false, cloneOptions)
	if err != nil {
		// Check if error was due to context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		return "", fmt.Errorf("failed to clone repository %s: %w", repo.Repo, err)
	}

	// If the revision is not the default branch, checkout the specific revision
	if repo.Rev != "" && repo.Rev != "master" && repo.Rev != "main" {
		worktree, err := gitRepo.Worktree()
		if err != nil {
			return "", fmt.Errorf("failed to get worktree: %w", err)
		}

		// Try to checkout the specified revision
		err = ops.checkoutRevision(gitRepo, worktree, repo.Rev)
		if err != nil {
			return "", fmt.Errorf("failed to checkout revision %s: %w", repo.Rev, err)
		}
	}

	return repoPath, nil
}

// checkoutRevision checks out a specific revision in the repository
func (ops *Operations) checkoutRevision(
	repo *git.Repository,
	worktree *git.Worktree,
	revision string,
) error {
	// Try to resolve the revision as a hash first (only if it looks like a valid hash)
	if isValidCommitHash(revision) {
		targetHash := plumbing.NewHash(revision)
		// Check if the commit actually exists in the repository
		if _, err := repo.CommitObject(targetHash); err == nil {
			return worktree.Checkout(&git.CheckoutOptions{
				Hash: targetHash,
			})
		}
	}

	// Try as a tag reference first (most common for versions like "22.3.0")
	ref, err := repo.Reference(plumbing.ReferenceName("refs/tags/"+revision), true)
	if err == nil {
		return worktree.Checkout(&git.CheckoutOptions{
			Hash: ref.Hash(),
		})
	}

	// Try as a remote branch
	ref, err = repo.Reference(plumbing.ReferenceName("refs/remotes/origin/"+revision), true)
	if err == nil {
		return worktree.Checkout(&git.CheckoutOptions{
			Hash: ref.Hash(),
		})
	}

	// Try as a local branch
	ref, err = repo.Reference(plumbing.ReferenceName("refs/heads/"+revision), true)
	if err == nil {
		return worktree.Checkout(&git.CheckoutOptions{
			Hash: ref.Hash(),
		})
	}

	return fmt.Errorf("failed to resolve revision %s", revision)
}

// isValidCommitHash checks if a string looks like a git commit hash
func isValidCommitHash(s string) bool {
	if len(s) != 40 && len(s) != 7 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// updateRepo updates a repository to the specified revision using go-git
func (ops *Operations) updateRepo(repoPath, revision string) error {
	// Open the repository using go-git
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open repository: %w", err)
	}

	// Get the working tree
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get current HEAD
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Try to resolve the revision as a hash first
	var targetHash plumbing.Hash
	if hash := plumbing.NewHash(revision); !hash.IsZero() {
		targetHash = hash
	} else {
		// Try to resolve as a reference (tag, branch, etc.)
		targetHash, err = ops.resolveRevision(repo, revision)
		if err != nil {
			// Reference not found locally, need to fetch
			return ops.fetchAndCheckout(repo, worktree, revision)
		}
	}

	// Check if we're already on the target revision
	if head.Hash() == targetHash {
		return nil
	}

	// Check if the target hash exists locally
	_, err = repo.CommitObject(targetHash)
	if err == nil {
		// Target exists locally, just checkout
		return worktree.Checkout(&git.CheckoutOptions{
			Hash: targetHash,
		})
	}

	// Target doesn't exist locally, need to fetch
	return ops.fetchAndCheckout(repo, worktree, revision)
}

// resolveRevision tries to resolve a revision string to a hash
func (ops *Operations) resolveRevision(
	repo *git.Repository,
	revision string,
) (plumbing.Hash, error) {
	// Try as a tag first (most common for versions like "22.3.0")
	if ref, err := repo.Reference(plumbing.ReferenceName("refs/tags/"+revision), true); err == nil {
		return ref.Hash(), nil
	}

	// Try as a remote branch
	if ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/"+revision), true); err == nil {
		return ref.Hash(), nil
	}

	// Try as a local branch
	if ref, err := repo.Reference(plumbing.ReferenceName("refs/heads/"+revision), true); err == nil {
		return ref.Hash(), nil
	}

	return plumbing.ZeroHash, fmt.Errorf("revision %s not found locally", revision)
}

// fetchAndCheckout fetches from remote and checks out the specified revision
func (ops *Operations) fetchAndCheckout(
	repo *git.Repository,
	worktree *git.Worktree,
	revision string,
) error {
	// Fetch from origin with tags
	err := repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		RefSpecs: []gitconfig.RefSpec{
			"+refs/heads/*:refs/remotes/origin/*",
			"+refs/tags/*:refs/tags/*",
		},
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("failed to fetch updates: %w", err)
	}

	// Try to resolve the revision again after fetch
	targetHash, err := ops.resolveRevision(repo, revision)
	if err != nil {
		return fmt.Errorf("failed to resolve revision %s after fetch: %w", revision, err)
	}

	// Checkout the target revision
	return worktree.Checkout(&git.CheckoutOptions{
		Hash: targetHash,
	})
}

// cloneWithLock clones a repository using file-based locking to prevent race conditions
func (ops *Operations) cloneWithLock(
	ctx context.Context,
	repo config.Repo,
	repoPath string,
) (string, error) {
	lock := cache.NewFileLock(ops.cacheManager.GetCacheDir())

	var result string
	var resultErr error

	lockErr := lock.WithLock(ctx, func() error {
		// Check if context is already canceled before doing any work
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Double-check if another process already cloned the repository
		if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
			result = repoPath
			return nil
		}

		// Check context again before expensive clone operation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Clone the repository
		clonedPath, err := ops.cloneRepo(ctx, repo, repoPath)
		if err != nil {
			resultErr = err
			return err
		}

		// Update database entry
		if err := ops.cacheManager.UpdateRepoEntry(repo, clonedPath); err != nil { //nolint:contextcheck // Cache operations are local and don't need context cancellation
			// Log error but don't fail - the cache will still work
			fmt.Printf("Warning: failed to update database entry for %s: %v\n", repo.Repo, err)
		}

		result = clonedPath
		return nil
	})

	if lockErr != nil {
		return "", fmt.Errorf("failed to acquire lock for cloning: %w", lockErr)
	}

	if resultErr != nil {
		return "", resultErr
	}

	return result, nil
}

// cloneWithLockAndDeps clones a repository using file-based locking with dependency tracking
func (ops *Operations) cloneWithLockAndDeps(
	ctx context.Context,
	repo config.Repo,
	repoPath string,
	additionalDeps []string,
) (string, error) {
	lock := cache.NewFileLock(ops.cacheManager.GetCacheDir())

	var result string
	var resultErr error

	lockErr := lock.WithLock(ctx, func() error {
		// Check if context is already canceled before doing any work
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Double-check if another process already cloned the repository
		if _, err := os.Stat(filepath.Join(repoPath, ".git")); err == nil {
			result = repoPath
			return nil
		}

		// Check context again before expensive clone operation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Clone the repository
		clonedPath, err := ops.cloneRepo(ctx, repo, repoPath)
		if err != nil {
			resultErr = err
			return err
		}

		// Update database entry with dependencies
		if err := ops.cacheManager.UpdateRepoEntryWithDeps(repo, additionalDeps, clonedPath); err != nil { //nolint:contextcheck // Cache operations are local and don't need context cancellation
			fmt.Printf("Warning: failed to update cache database: %v\n", err)
		}

		result = clonedPath
		return nil
	})

	if lockErr != nil {
		return "", fmt.Errorf("failed to acquire lock for cloning: %w", lockErr)
	}

	if resultErr != nil {
		return "", resultErr
	}

	return result, nil
}
