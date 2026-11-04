// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package store manages the cache of cloned hook repositories.
package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// RepoEntry is a row of upstream's repos table: the clone (or local
// placeholder tree) at Path backs Repo -- a DBRepoName -- at Rev, the table's
// ref column.
type RepoEntry struct {
	Repo string
	Rev  string
	Path string
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

// Init creates the store directory, with upstream's README in it, if it does
// not exist yet.
func (s *Store) Init() error {
	if _, err := os.Stat(s.dir); err == nil {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil { // #nosec G301 -- as upstream's os.makedirs
		return err
	}
	return os.WriteFile(filepath.Join(s.dir, "README"), []byte(storeReadme), 0o644) // #nosec G306 -- as upstream's open()
}

// readonly is upstream's Store.readonly: the store directory exists and this
// process may not write to it. Such a store is used as it is -- nothing is
// recorded in it.
func (s *Store) readonly() bool {
	info, err := os.Stat(s.dir)
	return err == nil && info.IsDir() && !fsutil.Writable(s.dir)
}

// Clean removes the entire store directory.
func (s *Store) Clean() error {
	return fsutil.RemoveAll(s.dir)
}

// cloneDirName derives the cache directory name for a repo+rev. It is
// deterministic, which is why a leftover directory at this path blocks every
// future clone of the same repo+rev until it is cleared.
//
// Upstream names its clones with tempfile.mkdtemp(prefix='repo') instead.
// Neither tool derives a path from a name -- both look the path up in db.db
// -- so the two schemes share a store, and they cannot collide: mkdtemp's
// suffix is 8 characters, this one 16.
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
	if err := s.readyLocked(); err != nil {
		return "", err
	}
	if path, err := s.find(name, rev, isClone); err == nil {
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
	if s.readonly() {
		return nil
	}
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

	return s.write(func(ctx context.Context, conn *sql.Conn) error {
		return inTx(ctx, conn, func(tx *sql.Tx) error {
			// Upstream creates the table here too, for a database made before
			// configs were tracked.
			if _, err := tx.ExecContext(ctx, createConfigsTable); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO configs VALUES (?)", realPath)
			return err
		})
	})
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

	return s.write(func(ctx context.Context, conn *sql.Conn) error {
		return inTx(ctx, conn, func(tx *sql.Tx) error {
			for _, path := range dead {
				if _, err := tx.ExecContext(ctx, "DELETE FROM configs WHERE path = ?", path); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// GC garbage-collects unused repos and returns how many it removed, which
// upstream's gc reports as "{n} repo(s) removed.". Every row in the store is a
// candidate, whichever tool cloned it.
func (s *Store) GC(usedRepos map[string]bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.acquireLock()
	if err != nil {
		return 0, err
	}
	defer unlock()

	repos, err := s.reposLocked()
	if err != nil {
		return 0, err
	}

	var removed []RepoEntry
	var failures []string
	for _, entry := range repos {
		if usedRepos[entry.Repo+"@"+entry.Rev] {
			continue
		}
		// If the directory cannot be removed, KEEP the database entry. Dropping
		// the row while the directory survives strands it: cache paths are
		// derived from repo+rev, so the next clone targets the leftover
		// directory, git refuses the non-empty target, and the repo is wedged
		// until someone clears the cache by hand.
		if err := fsutil.RemoveAll(entry.Path); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", entry.Path, err))
			continue
		}
		removed = append(removed, entry)
	}
	// The directories go first and the rows after, in one short transaction:
	// upstream's mark_config_used writes without the store lock, so holding
	// the database while trees are deleted would make it wait on them.
	if err := s.deleteRows(removed); err != nil {
		return 0, err
	}
	if len(failures) > 0 {
		return len(removed), fmt.Errorf("failed to remove %d cached repo(s): %s",
			len(failures), strings.Join(failures, "; "))
	}
	return len(removed), nil
}

// reposLocked is listRepos for a caller holding the store lock.
func (s *Store) reposLocked() ([]RepoEntry, error) {
	if err := s.readyLocked(); err != nil {
		return nil, err
	}
	return s.listRepos()
}

// deleteRows drops the rows of entries, in one transaction. The caller holds
// the store lock.
func (s *Store) deleteRows(entries []RepoEntry) error {
	return s.write(func(ctx context.Context, conn *sql.Conn) error {
		return inTx(ctx, conn, func(tx *sql.Tx) error {
			for _, e := range entries {
				if _, err := tx.ExecContext(ctx,
					"DELETE FROM repos WHERE repo = ? and ref = ?", e.Repo, e.Rev); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// ListRepos returns all cached repos.
func (s *Store) ListRepos() ([]RepoEntry, error) {
	if err := s.importPending(); err != nil {
		return nil, err
	}
	return s.listRepos()
}

// listRepos reads every row of the repos table, without importing db.json.
func (s *Store) listRepos() ([]RepoEntry, error) {
	var repos []RepoEntry
	err := s.query(func(ctx context.Context, conn *sql.Conn) error {
		return queryRows(ctx, conn, "SELECT repo, ref, path FROM repos ORDER BY rowid", func(rows *sql.Rows) error {
			var e RepoEntry
			if err := rows.Scan(&e.Repo, &e.Rev, &e.Path); err != nil {
				return err
			}
			repos = append(repos, e)
			return nil
		})
	})
	return repos, err
}

// GetTrackedConfigs returns the list of config files that have been tracked via MarkConfigUsed.
func (s *Store) GetTrackedConfigs() ([]string, error) {
	var configs []string
	err := s.read(func(ctx context.Context, conn *sql.Conn) error {
		// A database made before configs were tracked has no such table;
		// upstream's gc creates it, empty, before reading it.
		var tables int
		if err := conn.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'configs'").Scan(&tables); err != nil {
			return err
		}
		if tables == 0 {
			return nil
		}
		return queryRows(ctx, conn, "SELECT path FROM configs ORDER BY rowid", func(rows *sql.Rows) error {
			var path string
			if err := rows.Scan(&path); err != nil {
				return err
			}
			configs = append(configs, path)
			return nil
		})
	})
	return configs, err
}

func (s *Store) lockPath() string {
	return filepath.Join(s.dir, ".lock")
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

// errNotCached is lookup's miss.
var errNotCached = errors.New("not found")

// lookupWith finds a cached entry, using valid to decide whether what is on
// disk still backs it — a real clone for cloned repos, an intact placeholder
// tree for local ones. It is for callers without the store lock; one holding
// it calls find.
func (s *Store) lookupWith(repo, rev string, valid func(string) bool) (string, error) {
	if err := s.importPending(); err != nil {
		return "", err
	}
	return s.find(repo, rev, valid)
}

// find is lookupWith without the import, for a caller holding the store
// lock.
func (s *Store) find(repo, rev string, valid func(string) bool) (string, error) {
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

	var path string
	err := s.query(func(ctx context.Context, conn *sql.Conn) error {
		err := conn.QueryRowContext(ctx,
			"SELECT path FROM repos WHERE repo = ? AND ref = ?", repo, rev).Scan(&path)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	})
	if err != nil {
		return "", err
	}
	// A row that points at something no longer intact is a miss, so the
	// caller rebuilds it; both Clone and MakeLocal clear their path first, so
	// a leftover does not block the retry, and save replaces the row.
	if path == "" || !valid(path) {
		return "", errNotCached
	}
	if s.cache == nil {
		s.cache = make(map[string]string)
	}
	s.cache[key] = path
	return path, nil
}

// save records path as the clone of repo at rev, replacing any row already
// there for them (one find rejected as no longer intact). The caller holds
// the store lock.
func (s *Store) save(repo, rev, path string) error {
	err := s.write(func(ctx context.Context, conn *sql.Conn) error {
		return inTx(ctx, conn, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO repos (repo, ref, path) VALUES (?, ?, ?)", repo, rev, path)
			return err
		})
	})
	if err != nil {
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

	lf, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o644) // #nosec G302 -- as upstream's open()
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
