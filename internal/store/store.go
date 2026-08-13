// Package store manages the cache of cloned hook repositories.
package store

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
	gitutil "github.com/blairham/go-pre-commit/v4/internal/git"
)

// Store manages the cache of cloned hook repositories.
type Store struct {
	dir   string
	mu    sync.Mutex
	cache map[string]string // repo@rev -> path, in-memory lookup cache
}

// RepoEntry tracks a cloned repository.
type RepoEntry struct {
	Repo string `json:"repo"`
	Rev  string `json:"rev"`
	Path string `json:"path"`
}

// storeDB is the JSON-based database for tracking repos.
type storeDB struct {
	Repos       []RepoEntry `json:"repos"`
	ConfigsUsed []string    `json:"configs_used,omitempty"`
}

// DefaultDir returns the default store directory.
func DefaultDir() string {
	// Check PRE_COMMIT_HOME first.
	if home := os.Getenv("PRE_COMMIT_HOME"); home != "" {
		return home
	}
	// Check XDG_CACHE_HOME.
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "pre-commit")
	}
	// Default to ~/.cache/pre-commit.
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "pre-commit")
}

// New creates a new Store at the given directory.
func New(dir string) *Store {
	if dir == "" {
		dir = DefaultDir()
	}
	return &Store{dir: dir}
}

// Dir returns the store directory path.
func (s *Store) Dir() string {
	return s.dir
}

// Init initializes the store directory.
func (s *Store) Init() error {
	return os.MkdirAll(s.dir, 0o755)
}

// Clean removes the entire store directory.
func (s *Store) Clean() error {
	return fsutil.RemoveAll(s.dir)
}

// cloneDirName derives the cache directory name for a repo+rev. It is
// deterministic, which is why a leftover directory at this path blocks every
// future clone of the same repo+rev until it is cleared.
func cloneDirName(repo, rev string) string {
	hash := sha256.Sum256([]byte(repo + rev))
	return fmt.Sprintf("repo%x", hash[:8])
}

// Clone clones a hook repository and returns the local path.
func (s *Store) Clone(repo, rev string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already cloned.
	if path, err := s.lookup(repo, rev); err == nil {
		return path, nil
	}

	if err := s.Init(); err != nil {
		return "", err
	}

	// Acquire file lock for concurrent process safety.
	unlock, err := s.acquireLock()
	if err != nil {
		return "", fmt.Errorf("failed to acquire store lock: %w", err)
	}
	defer unlock()

	// Double-check after acquiring lock (another process may have cloned).
	if path, err := s.lookup(repo, rev); err == nil {
		return path, nil
	}

	dest := filepath.Join(s.dir, cloneDirName(repo, rev))

	// The path is derived from repo+rev, so a directory left behind by a failed
	// clone or a partial GC sits exactly where this clone wants to go — and
	// `git clone` refuses a non-empty target. Clear it first, and if it will not
	// clear, say so plainly rather than letting git fail with "destination path
	// already exists" on every run from here on.
	if err := fsutil.RemoveAll(dest); err != nil {
		return "", fmt.Errorf(
			"failed to clear stale cache directory %s (remove it manually to recover): %w",
			dest, err)
	}

	// Try shallow clone first.
	err = gitutil.ShallowClone(repo, dest, rev)
	if err != nil {
		// Fall back to full clone.
		if rmErr := fsutil.RemoveAll(dest); rmErr != nil {
			return "", fmt.Errorf(
				"failed to clear %s after an unsuccessful shallow clone: %w", dest, rmErr)
		}
		err = gitutil.Clone(repo, dest)
		if err != nil {
			return "", fmt.Errorf("failed to clone %s: %w", repo, err)
		}
		if err := gitutil.Checkout(dest, rev); err != nil {
			_ = fsutil.RemoveAll(dest)
			return "", fmt.Errorf("failed to checkout %s at %s: %w", repo, rev, err)
		}
	}

	// Save to database.
	if err := s.save(repo, rev, dest); err != nil {
		return "", err
	}

	return dest, nil
}

// GetPath returns the cached path for a repo+rev, or empty string if not cached.
func (s *Store) GetPath(repo, rev string) string {
	path, err := s.lookup(repo, rev)
	if err != nil {
		return ""
	}
	return path
}

// MarkConfigUsed records that a config file is actively using this store.
func (s *Store) MarkConfigUsed(configPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer unlock()

	db, err := s.loadDB()
	if err != nil {
		return err
	}

	// Resolve to absolute path.
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		absPath = configPath
	}

	// Check if already recorded.
	for _, c := range db.ConfigsUsed {
		if c == absPath {
			return nil
		}
	}

	db.ConfigsUsed = append(db.ConfigsUsed, absPath)
	return s.saveDB(db)
}

// GC garbage-collects unused repos.
func (s *Store) GC(usedRepos map[string]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer unlock()

	db, err := s.loadDB()
	if err != nil {
		return err
	}

	var kept []RepoEntry
	var failures []string
	for _, entry := range db.Repos {
		key := entry.Repo + "@" + entry.Rev
		if usedRepos[key] {
			kept = append(kept, entry)
			continue
		}
		// If the directory cannot be removed, KEEP the database entry. Dropping
		// the row while the directory survives strands it: cache paths are
		// derived from repo+rev, so the next clone targets the leftover
		// directory, git refuses the non-empty target, and the repo is wedged
		// until someone clears the cache by hand.
		if err := fsutil.RemoveAll(entry.Path); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", entry.Path, err))
			kept = append(kept, entry)
		}
	}
	db.Repos = kept
	if err := s.saveDB(db); err != nil {
		return err
	}
	if len(failures) > 0 {
		return fmt.Errorf("failed to remove %d cached repo(s): %s",
			len(failures), strings.Join(failures, "; "))
	}
	return nil
}

// ListRepos returns all cached repos.
func (s *Store) ListRepos() ([]RepoEntry, error) {
	db, err := s.loadDB()
	if err != nil {
		return nil, err
	}
	return db.Repos, nil
}

// GetTrackedConfigs returns the list of config files that have been tracked via MarkConfigUsed.
func (s *Store) GetTrackedConfigs() ([]string, error) {
	db, err := s.loadDB()
	if err != nil {
		return nil, err
	}
	return db.ConfigsUsed, nil
}

func (s *Store) dbPath() string {
	return filepath.Join(s.dir, "db.json")
}

func (s *Store) lockPath() string {
	return filepath.Join(s.dir, ".lock")
}

func (s *Store) loadDB() (*storeDB, error) {
	data, err := os.ReadFile(s.dbPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &storeDB{}, nil
		}
		return nil, err
	}
	var db storeDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, err
	}
	return &db, nil
}

func (s *Store) saveDB(db *storeDB) error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.dbPath(), data, 0o644)
}

func (s *Store) cacheKey(repo, rev string) string {
	return repo + "@" + rev
}

// isClone reports whether path holds an actual git clone. A bare os.Stat is
// not enough: a directory can survive with only an installed language
// environment inside it (a partially-removed cache entry), and treating that
// as a hit runs hooks against a tree with no .pre-commit-hooks.yaml.
func isClone(path string) bool {
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return false
	}
	return true
}

func (s *Store) lookup(repo, rev string) (string, error) {
	key := s.cacheKey(repo, rev)

	// Check in-memory cache first.
	if s.cache != nil {
		if path, ok := s.cache[key]; ok {
			if isClone(path) {
				return path, nil
			}
			// Gone or no longer a clone — drop the stale entry.
			delete(s.cache, key)
		}
	}

	db, err := s.loadDB()
	if err != nil {
		return "", err
	}
	for _, entry := range db.Repos {
		if entry.Repo == repo && entry.Rev == rev {
			if isClone(entry.Path) {
				// Populate in-memory cache.
				if s.cache == nil {
					s.cache = make(map[string]string)
				}
				s.cache[key] = entry.Path
				return entry.Path, nil
			}
			// The row points at something that is not a clone. Treat it as a
			// miss so Clone rebuilds it; Clone clears the path first, so the
			// leftover does not block the retry.
			break
		}
	}
	return "", fmt.Errorf("not found")
}

func (s *Store) save(repo, rev, path string) error {
	db, err := s.loadDB()
	if err != nil {
		return err
	}
	db.Repos = append(db.Repos, RepoEntry{
		Repo: repo,
		Rev:  rev,
		Path: path,
	})
	if err := s.saveDB(db); err != nil {
		return err
	}
	// Update in-memory cache.
	if s.cache == nil {
		s.cache = make(map[string]string)
	}
	s.cache[s.cacheKey(repo, rev)] = path
	return nil
}

// acquireLock acquires file-level locking for concurrent process safety.
// Returns an unlock function.
func (s *Store) acquireLock() (func(), error) {
	if err := s.Init(); err != nil {
		return nil, err
	}

	lf, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Use platform-specific file locking.
	if err := lockFile(lf); err != nil {
		lf.Close()
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}

	return func() {
		_ = unlockFile(lf)
		lf.Close()
	}, nil
}
