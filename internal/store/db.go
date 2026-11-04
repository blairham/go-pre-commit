// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	// The pure-Go SQLite: releases are built with CGO_ENABLED=0, so a driver
	// that needs cgo would not link.
	_ "modernc.org/sqlite"
)

// dbFile is upstream's Store.db_path: the SQLite database that indexes the
// store. Python pre-commit and this port read and write the same file, which
// is what lets one PRE_COMMIT_HOME serve both.
const dbFile = "db.db"

// The schema, as upstream's Store.__init__ and _create_configs_table write it.
// SQLite keeps each table's CREATE statement verbatim in sqlite_master, so the
// text is copied exactly, spacing included: a database this port creates is
// then indistinguishable from one Python created.
const (
	createReposTable = "CREATE TABLE repos (" +
		"    repo TEXT NOT NULL," +
		"    ref TEXT NOT NULL," +
		"    path TEXT NOT NULL," +
		"    PRIMARY KEY (repo, ref)" +
		");"
	createConfigsTable = "CREATE TABLE IF NOT EXISTS configs (" +
		"   path TEXT NOT NULL," +
		"   PRIMARY KEY (path)" +
		");"
)

// storeReadme is the README upstream writes into a store directory it
// creates.
const storeReadme = "This directory is maintained by the pre-commit project.\n" +
	"Learn more: https://github.com/pre-commit/pre-commit\n"

// busyTimeout is how long a statement waits for another process's lock on
// the database before failing: Python's sqlite3.connect default of 5 seconds.
// Upstream's mark_config_used writes without the store's .lock, so a Python
// process and this one can reach the database at the same moment, and SQLite's
// own locking is what serializes them.
const busyTimeout = "PRAGMA busy_timeout = 5000"

func (s *Store) dbPath() string {
	return filepath.Join(s.dir, dbFile)
}

// sqliteName is the name the driver opens path by. The driver takes whatever
// follows a '?' as its own parameters, so a path holding one is given as an
// escaped file: URI instead, which SQLite decodes back to the path.
func sqliteName(path string) string {
	if !strings.Contains(path, "?") {
		return path
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

// connect is upstream's Store.connect: a connection to the database at path
// for one operation, closed when it ends. Nothing holds the database open
// between operations, so a command that never reaches the store never opens
// it, and `clean` can remove the store on Windows, where an open file cannot
// be deleted.
func connect(path string, fn func(context.Context, *sql.Conn) error) (err error) {
	db, err := sql.Open("sqlite", sqliteName(path))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, conn.Close()) }()

	if _, err = conn.ExecContext(ctx, busyTimeout); err != nil {
		return err
	}
	return fn(ctx, conn)
}

// inTx runs fn in a transaction on conn, committing if it succeeds -- the
// transaction upstream's `with db:` wraps around each connection.
func inTx(ctx context.Context, conn *sql.Conn, fn func(*sql.Tx) error) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

// createDB is the database half of upstream's Store.__init__, run with the
// store lock held. The tables are created in a temporary file that is then
// renamed over db.db, so a db.db that exists always has its tables: an
// interrupt between creating the file and creating the tables leaves a stray
// temporary file, never an empty database.
func (s *Store) createDB() error {
	f, err := os.CreateTemp(s.dir, "tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err = f.Close(); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	err = createTables(tmp)
	if err == nil {
		err = os.Rename(tmp, s.dbPath())
	}
	if err != nil {
		return errors.Join(
			fmt.Errorf("failed to create %s: %w", s.dbPath(), err),
			ignoreNotExist(os.Remove(tmp)),
		)
	}
	return nil
}

// createTables creates upstream's tables in the database at path.
func createTables(path string) error {
	return connect(path, func(ctx context.Context, conn *sql.Conn) error {
		for _, stmt := range []string{createReposTable, createConfigsTable} {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				return err
			}
		}
		return nil
	})
}

// queryRows runs query on conn and calls scan for each row it returns.
func queryRows(ctx context.Context, conn *sql.Conn, query string, scan func(*sql.Rows) error) (err error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		if scanErr := scan(rows); scanErr != nil {
			return scanErr
		}
	}
	return rows.Err()
}

// ready makes the database usable for an operation that does not hold the
// store lock: created if the store has none, and holding db.json's rows if
// that is still waiting to be imported. The lock is taken only when there is
// something to do, so an established store pays two stats for it.
func (s *Store) ready() error {
	if fileExists(s.dbPath()) && !fileExists(s.legacyPath()) {
		return nil
	}
	unlock, err := s.acquireLock()
	if err != nil {
		return err
	}
	defer unlock()
	return s.readyLocked()
}

// readyLocked is ready for a caller that already holds the store lock.
func (s *Store) readyLocked() error {
	if !fileExists(s.dbPath()) {
		if err := s.createDB(); err != nil {
			return err
		}
	}
	return s.importLegacy()
}

// read runs fn against the database for an operation that only reads it and
// does not hold the store lock: db.json is imported first if one is waiting.
func (s *Store) read(fn func(context.Context, *sql.Conn) error) error {
	if err := s.importPending(); err != nil {
		return err
	}
	return s.query(fn)
}

// importPending imports db.json if one is waiting, for a caller without the
// store lock.
func (s *Store) importPending() error {
	if !fileExists(s.legacyPath()) {
		return nil
	}
	return s.ready()
}

// query runs fn against the database as it is, importing nothing, so a
// caller holding the store lock can read with it: taking the lock again from
// the same process would wait forever. A store with no database has nothing
// in it, so fn is not called and nothing is created.
func (s *Store) query(fn func(context.Context, *sql.Conn) error) error {
	if !fileExists(s.dbPath()) {
		return nil
	}
	return connect(s.dbPath(), fn)
}

// write runs fn against the database for a caller holding the store lock,
// creating the database first if it must.
func (s *Store) write(fn func(context.Context, *sql.Conn) error) error {
	if err := s.readyLocked(); err != nil {
		return err
	}
	return connect(s.dbPath(), fn)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func ignoreNotExist(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
