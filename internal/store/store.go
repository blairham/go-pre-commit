// Package store manages the cache of cloned hook repositories.
package store

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	gitutil "github.com/blairham/go-pre-commit/internal/git"
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
	return os.RemoveAll(s.dir)
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

	// Generate a unique directory name.
	hash := sha256.Sum256([]byte(repo + rev))
	dirName := fmt.Sprintf("repo%x", hash[:8])
	dest := filepath.Join(s.dir, dirName)

	// Try shallow clone first.
	err = gitutil.ShallowClone(repo, dest, rev)
	if err != nil {
		// Fall back to full clone.
		os.RemoveAll(dest)
		err = gitutil.Clone(repo, dest)
		if err != nil {
			return "", fmt.Errorf("failed to clone %s: %w", repo, err)
		}
		if err := gitutil.Checkout(dest, rev); err != nil {
			os.RemoveAll(dest)
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
	for _, entry := range db.Repos {
		key := entry.Repo + "@" + entry.Rev
		if usedRepos[key] {
			kept = append(kept, entry)
		} else {
			// Remove the directory.
			os.RemoveAll(entry.Path)
		}
	}
	db.Repos = kept
	return s.saveDB(db)
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

func (s *Store) lookup(repo, rev string) (string, error) {
	key := s.cacheKey(repo, rev)

	// Check in-memory cache first.
	if s.cache != nil {
		if path, ok := s.cache[key]; ok {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
			// Path no longer exists, remove stale entry.
			delete(s.cache, key)
		}
	}

	db, err := s.loadDB()
	if err != nil {
		return "", err
	}
	for _, entry := range db.Repos {
		if entry.Repo == repo && entry.Rev == rev {
			if _, err := os.Stat(entry.Path); err == nil {
				// Populate in-memory cache.
				if s.cache == nil {
					s.cache = make(map[string]string)
				}
				s.cache[key] = entry.Path
				return entry.Path, nil
			}
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
