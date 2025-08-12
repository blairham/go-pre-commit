// Package cache provides repository caching functionality with database management
package cache

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/interfaces"
)

// Manager handles cache operations and database management
// Refactored from the original CacheManager to be more focused and maintainable
type Manager struct {
	db       *sql.DB
	cacheDir string
	dbPath   string
}

// NewManager creates a new cache manager
func NewManager(cacheDir string) (*Manager, error) {
	// Ensure cache directory exists (like Python pre-commit does)
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create an empty .lock file if it doesn't exist (used for file-based locking like Python pre-commit)
	lockPath := filepath.Join(cacheDir, ".lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		// Create empty file for locking purposes
		if err := os.WriteFile(lockPath, []byte{}, 0o600); err != nil {
			return nil, fmt.Errorf("failed to create lock file: %w", err)
		}
	}

	// Initialize SQLite database (compatible with Python pre-commit)
	dbPath := filepath.Join(cacheDir, "db.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}

	// Create tables if they don't exist (same schema as Python pre-commit)
	if err := initDatabase(db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			fmt.Printf("⚠️  Warning: failed to close database: %v\n", closeErr)
		}
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return &Manager{
		db:       db,
		cacheDir: cacheDir,
		dbPath:   dbPath,
	}, nil
}

// GetRepoPath returns the cached path for a repository
func (m *Manager) GetRepoPath(repo config.Repo) string {
	return m.GetRepoPathWithDeps(repo, nil)
}

// GetRepoPathWithDeps returns the cached path considering dependencies
func (m *Manager) GetRepoPathWithDeps(repo config.Repo, additionalDeps []string) string {
	dbRepoName := createDBRepoName(repo.Repo, additionalDeps)

	// Check if we already have this repo+ref combination in the database
	if path := m.getExistingRepoPath(dbRepoName, repo.Rev); path != "" {
		return path
	}

	// No existing repository found, create a new one
	repoDir := m.generateRandomRepoDir()
	fullPath := filepath.Join(m.cacheDir, repoDir)

	// Insert into database for future lookups
	if err := m.insertRepoEntry(dbRepoName, repo.Rev, fullPath); err != nil {
		fmt.Printf("⚠️  Warning: failed to insert repository into database: %v\n", err)
	}

	return fullPath
}

// UpdateRepoEntry updates the database entry for a repository
func (m *Manager) UpdateRepoEntry(repo config.Repo, path string) error {
	return m.UpdateRepoEntryWithDeps(repo, nil, path)
}

// UpdateRepoEntryWithDeps updates the database entry with dependencies
func (m *Manager) UpdateRepoEntryWithDeps(repo config.Repo, additionalDeps []string, path string) error {
	dbRepoName := createDBRepoName(repo.Repo, additionalDeps)
	return m.insertRepoEntry(dbRepoName, repo.Rev, path)
}

// CleanCache removes all cached repositories
func (m *Manager) CleanCache() error {
	return m.CleanCacheWithTimeout(30 * time.Second)
}

// CleanCacheWithTimeout removes all cached repositories using file-based locking
func (m *Manager) CleanCacheWithTimeout(timeout time.Duration) error {
	lock := NewFileLock(m.cacheDir)
	return lock.WithLockTimeout(timeout, func() error {
		return m.removeAllRepoDirectories()
	})
}

// MarkConfigUsed marks a config file as used in the database
func (m *Manager) MarkConfigUsed(configPath string) error {
	// Normalize the path to resolve symlinks
	normalizedPath, err := m.normalizePath(configPath)
	if err != nil {
		return err
	}

	// Don't insert config files that do not exist (matches Python behavior)
	if _, statErr := os.Stat(normalizedPath); os.IsNotExist(statErr) {
		return nil
	}

	// Insert or ignore if already exists
	_, err = m.db.ExecContext(context.Background(), "INSERT OR IGNORE INTO configs VALUES (?)", normalizedPath)
	return err
}

// normalizePath normalizes a path by resolving symlinks like Python's os.path.realpath
func (m *Manager) normalizePath(path string) (string, error) {
	// Get absolute path first
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Resolve symlinks to get the real path (like Python's os.path.realpath)
	// This ensures /tmp/... becomes /private/tmp/... on macOS
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If symlink resolution fails, fall back to absolute path
		// This is intentional fallback behavior, not an error
		return absPath, nil //nolint:nilerr // Intentional fallback on symlink resolution failure
	}

	return realPath, nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

// GetCacheDir returns the cache directory
func (m *Manager) GetCacheDir() string {
	return m.cacheDir
}

// GetDBPath returns the database path
func (m *Manager) GetDBPath() string {
	return m.dbPath
}

// getExistingRepoPath checks if repo exists and returns valid path
func (m *Manager) getExistingRepoPath(dbRepoName, rev string) string {
	var path string
	err := m.db.QueryRowContext(
		context.Background(),
		"SELECT path FROM repos WHERE repo = ? AND ref = ?",
		dbRepoName, rev,
	).Scan(&path)
	if err != nil {
		return ""
	}

	// Verify the path still exists
	if _, statErr := os.Stat(filepath.Join(path, ".git")); statErr == nil {
		return path
	}

	// Database entry exists but repository directory doesn't, remove stale entry
	if _, deleteErr := m.db.ExecContext(
		context.Background(),
		"DELETE FROM repos WHERE repo = ? AND ref = ?",
		dbRepoName, rev,
	); deleteErr != nil {
		fmt.Printf("⚠️  Warning: failed to remove stale entry: %v\n", deleteErr)
	}

	return ""
}

// insertRepoEntry inserts or replaces a repository entry
func (m *Manager) insertRepoEntry(dbRepoName, rev, path string) error {
	// Normalize the path to resolve symlinks (like we do for config paths)
	// This ensures consistent path storage between repos and configs
	normalizedPath, err := m.normalizePath(path)
	if err != nil {
		// If normalization fails, use the original path
		normalizedPath = path
	}

	_, err = m.db.ExecContext(
		context.Background(),
		"INSERT OR REPLACE INTO repos (repo, ref, path) VALUES (?, ?, ?)",
		dbRepoName, rev, normalizedPath,
	)
	if err != nil {
		fmt.Printf("⚠️  Warning: failed to update database entry for %s: %v\n", dbRepoName, err)
	}
	return err
}

// generateRandomRepoDir creates a random directory name with "repo" prefix
func (m *Manager) generateRandomRepoDir() string {
	// Python's tempfile.mkdtemp creates a directory like repo{6_random_chars}
	tempDir, err := os.MkdirTemp(m.cacheDir, "repo")
	if err != nil {
		// Fallback to manual generation if temp creation fails
		return m.generateFallbackRepoDir()
	}

	// Extract just the directory name (not the full path)
	dirName := filepath.Base(tempDir)

	// Remove the temporary directory since we only needed the name
	_ = os.Remove(tempDir) //nolint:errcheck // intentionally ignoring error

	return dirName
}

// generateFallbackRepoDir generates a repo directory name manually
func (m *Manager) generateFallbackRepoDir() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	randBytes := make([]byte, 8)

	if _, err := rand.Read(randBytes); err != nil {
		// Final fallback
		for i := range b {
			b[i] = charset[i%len(charset)]
		}
	} else {
		for i, rb := range randBytes {
			b[i] = charset[int(rb)%len(charset)]
		}
	}
	return "repo" + string(b)
}

// removeAllRepoDirectories removes all repo* directories from cache
func (m *Manager) removeAllRepoDirectories() error {
	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "repo") {
			repoPath := filepath.Join(m.cacheDir, entry.Name())
			if err := os.RemoveAll(repoPath); err != nil {
				return fmt.Errorf("failed to remove repository cache %s: %w", repoPath, err)
			}
		}
	}
	return nil
}

// Package-level helper functions

// initDatabase creates the necessary tables if they don't exist
func initDatabase(db *sql.DB) error {
	// This matches Python pre-commit's database schema
	createReposTable := `
	CREATE TABLE IF NOT EXISTS repos (
		repo TEXT,
		ref TEXT,
		path TEXT,
		PRIMARY KEY (repo, ref)
	);`

	createConfigsTable := `
	CREATE TABLE IF NOT EXISTS configs (
		path TEXT NOT NULL,
		PRIMARY KEY (path)
	);`

	if _, err := db.ExecContext(context.Background(), createReposTable); err != nil {
		return fmt.Errorf("failed to create repos table: %w", err)
	}

	if _, err := db.ExecContext(context.Background(), createConfigsTable); err != nil {
		return fmt.Errorf("failed to create configs table: %w", err)
	}

	return nil
}

// createDBRepoName creates the database repository name exactly like Python pre-commit
// Format: repo_url for no dependencies, repo_url:dep1,dep2,dep3 for dependencies
func createDBRepoName(repoURL string, additionalDeps []string) string {
	if len(additionalDeps) == 0 {
		return repoURL
	}
	// Note: Do NOT sort - Python pre-commit uses the order as provided
	return fmt.Sprintf("%s:%s", repoURL, strings.Join(additionalDeps, ","))
}

// Ensure Manager implements the CacheManager interface
var _ interfaces.CacheManager = (*Manager)(nil)
