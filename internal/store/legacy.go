// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
	"github.com/blairham/go-pre-commit/v4/internal/output"
)

// legacyFile is the index this port kept before it used upstream's db.db.
// Python pre-commit never read it, so a store either tool wrote was invisible
// to the other (#165).
const legacyFile = "db.json"

// legacyDB is db.json's layout.
type legacyDB struct {
	Repos []struct {
		Repo string `json:"repo"`
		Rev  string `json:"rev"`
		Path string `json:"path"`
	} `json:"repos"`
	ConfigsUsed []string `json:"configs_used"`
}

func (s *Store) legacyPath() string {
	return filepath.Join(s.dir, legacyFile)
}

// importLegacy moves db.json's rows into db.db and deletes db.json, with the
// store lock held and db.db in place. It runs whenever a db.json is there, so
// a store last used by an older release is adopted on first use, clones and
// installed environments included, and so is one an older release goes on
// writing to alongside this one.
//
// A row whose directory is no longer an intact clone is not imported: Python
// pre-commit trusts every row it reads, and would run hooks from it. A row
// that db.db already has for the same repo and rev is not imported either --
// db.db's is the one Python is using. The directory of a row that was not
// imported is then referenced by nothing, and neither tool's gc would ever
// find it, so it is removed if it sits directly in the store.
//
// db.json is deleted once its rows are in db.db rather than kept: its rows
// live on there, and a db.json that is present always means rows not yet
// imported, with no marker needed to say otherwise.
func (s *Store) importLegacy() error {
	old, err := s.readLegacy()
	if err != nil || old == nil {
		return err
	}
	orphans, err := s.importRows(old)
	if err != nil {
		return fmt.Errorf("failed to import %s into %s: %w", s.legacyPath(), s.dbPath(), err)
	}
	if err := ignoreNotExist(os.Remove(s.legacyPath())); err != nil {
		return err
	}
	for _, p := range orphans {
		// A directory that will not go is no worse off than before, when
		// nothing referenced it either; it is reported, not fatal.
		if rmErr := fsutil.RemoveAll(p); rmErr != nil {
			output.Warn("Failed to remove %s, which nothing references any more: %v", p, rmErr)
		}
	}
	return nil
}

// readLegacy reads db.json, or returns nil if there is none.
func (s *Store) readLegacy() (*legacyDB, error) {
	data, err := os.ReadFile(s.legacyPath())
	if err != nil {
		return nil, ignoreNotExist(err)
	}
	var old legacyDB
	if err := json.Unmarshal(data, &old); err != nil {
		return nil, fmt.Errorf("failed to import %s: %w (remove it to continue; the repos it lists will be cloned again)",
			s.legacyPath(), err)
	}
	return &old, nil
}

// importRows copies old's rows into db.db in one transaction, and returns
// the directories in the store that db.json pointed at and db.db does not.
func (s *Store) importRows(old *legacyDB) ([]string, error) {
	var orphans []string
	err := connect(s.dbPath(), func(ctx context.Context, conn *sql.Conn) error {
		var skipped []string
		if txErr := inTx(ctx, conn, func(tx *sql.Tx) error {
			var insErr error
			skipped, insErr = insertLegacyRows(ctx, tx, old)
			return insErr
		}); txErr != nil {
			return txErr
		}
		for _, p := range skipped {
			if filepath.Dir(filepath.Clean(p)) != filepath.Clean(s.dir) {
				continue
			}
			var n int
			if qErr := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM repos WHERE path = ?", p).Scan(&n); qErr != nil {
				return qErr
			}
			if n == 0 {
				orphans = append(orphans, p)
			}
		}
		return nil
	})
	return orphans, err
}

// insertLegacyRows inserts old's intact rows and its configs, and returns
// the paths of the rows it did not insert.
func insertLegacyRows(ctx context.Context, tx *sql.Tx, old *legacyDB) ([]string, error) {
	if _, err := tx.ExecContext(ctx, createConfigsTable); err != nil {
		return nil, err
	}
	var skipped []string
	for _, r := range old.Repos {
		if !intact(r.Repo, r.Path) {
			skipped = append(skipped, r.Path)
			continue
		}
		res, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO repos (repo, ref, path) VALUES (?, ?, ?)", r.Repo, r.Rev, r.Path)
		if err != nil {
			return nil, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if n == 0 {
			skipped = append(skipped, r.Path)
		}
	}
	for _, c := range old.ConfigsUsed {
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO configs VALUES (?)", c); err != nil {
			return nil, err
		}
	}
	return skipped, nil
}

// intact reports whether path still backs the row it was recorded under: a
// placeholder tree for `repo: local`, a clone for everything else.
func intact(repo, path string) bool {
	if repo == "local" || strings.HasPrefix(repo, "local:") {
		return isLocalRepo(path)
	}
	return isClone(path)
}
