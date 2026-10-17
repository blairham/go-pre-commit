// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package store manages the cache of cloned hook repositories.
package store

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
	gitutil "github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/output"
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

// DBRepoName is upstream's Store.db_repo_name: the name a clone is recorded
// under, which encodes the additional_dependencies installed into it — the
// repo on its own, or `repo:dep1,dep2`. Hooks of one repo that declare
// different dependencies therefore get separate clones, and with them separate
// environments, instead of sharing one directory and each rewriting the
// other's install state on every run.
func DBRepoName(repo string, deps []string) string {
	if len(deps) == 0 {
		return repo
	}
	return repo + ":" + strings.Join(deps, ",")
}

// Clone clones a hook repository and returns the local path.
func (s *Store) Clone(repo, rev string) (string, error) {
	return s.CloneWithDeps(repo, rev, nil)
}

// CloneWithDeps is upstream's Store.clone(repo, ref, deps): the clone that
// backs hooks of repo@rev declaring deps as their additional_dependencies. It
// is recorded under DBRepoName(repo, deps), so each distinct dependency set
// has a clone of its own; with no deps it is the plain clone.
func (s *Store) CloneWithDeps(repo, rev string, deps []string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := DBRepoName(repo, deps)

	// Check if already cloned.
	if path, err := s.lookup(name, rev); err == nil {
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
	if path, err := s.lookup(name, rev); err == nil {
		return path, nil
	}

	output.Info("Initializing environment for %s.", name)

	dest := filepath.Join(s.dir, cloneDirName(name, rev))

	// The path is derived from repo+rev, so a directory left behind by a failed
	// clone or a partial GC sits exactly where this clone wants to go. Clear it
	// first, and if it will not clear, say so plainly rather than failing on
	// every run from here on.
	if err := fsutil.RemoveAll(dest); err != nil {
		return "", fmt.Errorf(
			"failed to clear stale cache directory %s (remove it manually to recover): %w",
			dest, err,
		)
	}

	if err := cloneInto(dest, repo, rev); err != nil {
		// Upstream's clean_path_on_failure.
		if rmErr := fsutil.RemoveAll(dest); rmErr != nil {
			return "", errors.Join(err, fmt.Errorf(
				"failed to clear %s after an unsuccessful clone: %w", dest, rmErr,
			))
		}
		return "", err
	}

	// Save to database.
	if err := s.save(name, rev, dest); err != nil {
		return "", err
	}

	config.WarnForStagesOnRepoInit(repo, dest)

	return dest, nil
}

// cloneInto is upstream's Store.clone strategy: an empty repository whose
// origin is repo, then a shallow fetch of exactly rev. Fetching the ref, rather
// than `git clone --branch`, is what lets a commit sha be fetched shallowly;
// only when that fails (a server that will not serve an unadvertised sha, say)
// does it fall back to fetching every tag and checking rev out. git's output
// is captured throughout and reaches the user only in the error.
func cloneInto(dest, repo, rev string) error {
	if err := gitutil.InitRepo(dest, repo); err != nil {
		return err
	}
	git := func(args ...string) error { return gitutil.RunCaptured(dest, args...) }
	if err := shallowClone(rev, git); err != nil {
		return completeClone(rev, git)
	}
	return nil
}

// shallowClone is upstream's Store._shallow_clone.
func shallowClone(rev string, git func(...string) error) error {
	const cfg = "protocol.version=2"
	if err := git("-c", cfg, "fetch", "origin", rev, "--depth=1"); err != nil {
		return err
	}
	if err := git("checkout", "FETCH_HEAD"); err != nil {
		return err
	}
	return git("-c", cfg, "submodule", "update", "--init", "--recursive", "--depth=1")
}

// completeClone is upstream's Store._complete_clone.
func completeClone(rev string, git func(...string) error) error {
	if err := git("fetch", "origin", "--tags"); err != nil {
		return err
	}
	if err := git("checkout", rev); err != nil {
		return err
	}
	return git("submodule", "update", "--init", "--recursive")
}

// GetPath returns the cached path for a repo+rev, or empty string if not cached.
func (s *Store) GetPath(repo, rev string) string {
	path, err := s.lookup(repo, rev)
	if err != nil {
		return ""
	}
	return path
}

// MarkConfigUsed records that a config file is using this store, so gc keeps
// every repo it references. It ports upstream's Store.mark_config_used: the
// path is made real (symlinks resolved) before it is stored, a config that
// does not exist is not recorded, and a store the user cannot write to is
// left alone rather than reported -- upstream skips a readonly store.
func (s *Store) MarkConfigUsed(configPath string) error {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}
	// Upstream's os.path.exists: a config that is not there is not recorded.
	if _, statErr := os.Stat(absPath); statErr != nil {
		return nil
	}
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.acquireLock()
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil
		}
		return err
	}
	defer unlock()

	db, err := s.loadDB()
	if err != nil {
		return err
	}
	if slices.Contains(db.ConfigsUsed, realPath) {
		return nil
	}
	db.ConfigsUsed = append(db.ConfigsUsed, realPath)
	return s.saveDB(db)
}

// ForgetConfigs drops configs from the tracked list. gc calls it for every
// tracked config that no longer loads, as upstream's _gc deletes its dead
// configs rows.
func (s *Store) ForgetConfigs(dead []string) error {
	if len(dead) == 0 {
		return nil
	}
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
	db.ConfigsUsed = slices.DeleteFunc(db.ConfigsUsed, func(c string) bool {
		return slices.Contains(dead, c)
	})
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
	return s.lookupWith(repo, rev, isClone)
}

// lookupWith finds a cached entry, using valid to decide whether what is on
// disk still backs it — a real clone for cloned repos, an intact placeholder
// tree for local ones.
func (s *Store) lookupWith(repo, rev string, valid func(string) bool) (string, error) {
	key := s.cacheKey(repo, rev)

	// Check in-memory cache first.
	if s.cache != nil {
		if path, ok := s.cache[key]; ok {
			if valid(path) {
				return path, nil
			}
			// Gone or no longer intact — drop the stale entry.
			delete(s.cache, key)
		}
	}

	db, err := s.loadDB()
	if err != nil {
		return "", err
	}
	for _, entry := range db.Repos {
		if entry.Repo == repo && entry.Rev == rev {
			if valid(entry.Path) {
				// Populate in-memory cache.
				if s.cache == nil {
					s.cache = make(map[string]string)
				}
				s.cache[key] = entry.Path
				return entry.Path, nil
			}
			// The row points at something that is no longer intact. Treat it as
			// a miss so the caller rebuilds it; both Clone and MakeLocal clear
			// the path first, so the leftover does not block the retry.
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
	// Replace any existing row for this repo+rev rather than appending a second
	// one. lookup stops at the first matching row, so a duplicate left by a
	// rebuild (the old row pointing at a directory that no longer exists) would
	// shadow the new entry and force a rebuild on every single run.
	entry := RepoEntry{Repo: repo, Rev: rev, Path: path}
	replaced := false
	for i := range db.Repos {
		if db.Repos[i].Repo == repo && db.Repos[i].Rev == rev {
			db.Repos[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		db.Repos = append(db.Repos, entry)
	}
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

// ExclusiveLock is upstream's Store.exclusive_lock: the store-wide file lock
// that serializes work on the store across pre-commit processes. It returns
// the function that releases it.
func (s *Store) ExclusiveLock() (func(), error) {
	return s.acquireLock()
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
