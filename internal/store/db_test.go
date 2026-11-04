// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// sqlRows runs query against the store's db.db and returns each row's
// columns joined by "|".
func sqlRows(t *testing.T, s *Store, query string) []string {
	t.Helper()
	var out []string
	err := connect(s.dbPath(), func(ctx context.Context, conn *sql.Conn) error {
		rows, err := conn.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			return err
		}
		for rows.Next() {
			vals := make([]sql.NullString, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return err
			}
			parts := make([]string, len(vals))
			for i, v := range vals {
				parts[i] = v.String
			}
			out = append(out, strings.Join(parts, "|"))
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// fakeClone makes a directory that lookups accept as a clone.
func fakeClone(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// The database is upstream's: the same file name and, byte for byte, the
// schema SQLite records for it. The expected text is what sqlite_master holds
// in a db.db Python pre-commit 4.7.0 created, which is not the text of the
// statements: SQLite drops the trailing ';' and the IF NOT EXISTS.
func TestCreatesUpstreamSchema(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "store"))
	cfg := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(cfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkConfigUsed(cfg); err != nil {
		t.Fatal(err)
	}

	got := sqlRows(t, s, "SELECT type, name, sql FROM sqlite_master ORDER BY rowid")
	want := []string{
		"table|repos|CREATE TABLE repos (    repo TEXT NOT NULL,    ref TEXT NOT NULL,    path TEXT NOT NULL,    PRIMARY KEY (repo, ref))",
		"index|sqlite_autoindex_repos_1|",
		"table|configs|CREATE TABLE configs (   path TEXT NOT NULL,   PRIMARY KEY (path))",
		"index|sqlite_autoindex_configs_1|",
	}
	if !slices.Equal(got, want) {
		t.Errorf("sqlite_master =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// Created in a temporary file and renamed into place: none is left over.
	entries, err := os.ReadDir(s.Dir())
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{".lock", "README", "db.db"}; !slices.Equal(names, want) {
		t.Errorf("store holds %q, want %q", names, want)
	}
}

// Upstream writes a README into a store directory it creates, and only then.
func TestInitWritesReadme(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "store"))
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(s.Dir(), "README"))
	if err != nil {
		t.Fatal(err)
	}
	want := "This directory is maintained by the pre-commit project.\n" +
		"Learn more: https://github.com/pre-commit/pre-commit\n"
	if string(got) != want {
		t.Errorf("README = %q, want %q", got, want)
	}

	existing := New(t.TempDir())
	if err := existing.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(existing.Dir(), "README")); !os.IsNotExist(err) {
		t.Errorf("Init wrote a README into a directory that already existed: %v", err)
	}
}

// Reading a store that has no database creates nothing: only an operation
// that records something opens one.
func TestReadsCreateNothing(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "store")
	s := New(dir)
	if got := s.GetPath("https://example.com/r", "v1"); got != "" {
		t.Errorf("GetPath = %q, want empty", got)
	}
	if repos, err := s.ListRepos(); err != nil || len(repos) != 0 {
		t.Errorf("ListRepos = %v, %v", repos, err)
	}
	if configs, err := s.GetTrackedConfigs(); err != nil || len(configs) != 0 {
		t.Errorf("GetTrackedConfigs = %v, %v", configs, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("a read created the store directory: %v", err)
	}
}

// A row another process wrote -- Python pre-commit, with its own directory
// names -- is found, and gc removes it like any other.
func TestUsesRowsWrittenElsewhere(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.createDB(); err != nil {
		t.Fatal(err)
	}
	clone := fakeClone(t, filepath.Join(s.Dir(), "repoab_cd12e"))
	err := connect(s.dbPath(), func(ctx context.Context, conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, "INSERT INTO repos (repo, ref, path) VALUES (?, ?, ?)",
			"https://example.com/r", "v1", clone)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := New(s.Dir()).GetPath("https://example.com/r", "v1"); got != clone {
		t.Errorf("GetPath = %q, want %q", got, clone)
	}
	if removed, err := New(s.Dir()).GC(nil); err != nil || removed != 1 {
		t.Fatalf("GC = %d, %v; want 1 removed", removed, err)
	}
	if _, err := os.Stat(clone); !os.IsNotExist(err) {
		t.Errorf("gc left the clone behind: %v", err)
	}
	if rows := sqlRows(t, s, "SELECT * FROM repos"); len(rows) != 0 {
		t.Errorf("gc left rows: %q", rows)
	}
}

// db.json, the index this port kept before #165, is imported into db.db on
// first use and then deleted. A row db.db already has for the same repo and
// rev keeps db.db's path, and a row whose directory is not an intact clone is
// not imported; either way the directory db.json pointed at is referenced by
// nothing afterwards, so it is removed.
func TestImportsLegacyIndex(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.createDB(); err != nil {
		t.Fatal(err)
	}
	dir := s.Dir()

	pyClone := fakeClone(t, filepath.Join(dir, "repopython"))
	err := connect(s.dbPath(), func(ctx context.Context, conn *sql.Conn) error {
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO repos VALUES ('https://example.com/both', 'v1', ?)", pyClone); err != nil {
			return err
		}
		_, err := conn.ExecContext(ctx, "INSERT INTO configs VALUES ('/py/config')")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	goOnly := fakeClone(t, filepath.Join(dir, "repo0000000000000001"))
	goDup := fakeClone(t, filepath.Join(dir, "repo0000000000000002"))
	broken := filepath.Join(dir, "repo0000000000000003")
	if err = os.MkdirAll(filepath.Join(broken, "py_env-python3"), 0o755); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, "repo0000000000000004")
	if err = writeLocalResources(local); err != nil {
		t.Fatal(err)
	}
	outside := fakeClone(t, filepath.Join(t.TempDir(), "elsewhere"))
	legacy := map[string]any{
		"repos": []map[string]string{
			{"repo": "https://example.com/go-only", "rev": "v1", "path": goOnly},
			{"repo": "https://example.com/both", "rev": "v1", "path": goDup},
			{"repo": "https://example.com/broken", "rev": "v1", "path": broken},
			{"repo": "local:pyyaml", "rev": localRepoVersion, "path": local},
			{"repo": "https://example.com/outside", "rev": "v1", "path": filepath.Join(outside, "missing")},
		},
		"configs_used": []string{"/go/config", "/py/config"},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(s.legacyPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Any operation imports it; a fresh Store, as a new process would have.
	repos, err := New(dir).ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	want := []RepoEntry{
		{Repo: "https://example.com/both", Rev: "v1", Path: pyClone},
		{Repo: "https://example.com/go-only", Rev: "v1", Path: goOnly},
		{Repo: "local:pyyaml", Rev: localRepoVersion, Path: local},
	}
	if !slices.Equal(repos, want) {
		t.Errorf("repos =\n%v\nwant\n%v", repos, want)
	}
	configs, err := New(dir).GetTrackedConfigs()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/py/config", "/go/config"}; !slices.Equal(configs, want) {
		t.Errorf("configs = %q, want %q", configs, want)
	}
	if _, err = os.Stat(s.legacyPath()); !os.IsNotExist(err) {
		t.Errorf("db.json survived the import: %v", err)
	}
	for _, gone := range []string{goDup, broken} {
		if _, err = os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("%s is referenced by nothing and should be gone: %v", gone, err)
		}
	}
	for _, kept := range []string{pyClone, goOnly, local, outside} {
		if _, err = os.Stat(kept); err != nil {
			t.Errorf("%s should be untouched: %v", kept, err)
		}
	}
}

// A directory db.db already points at is never removed as an orphan, even
// when db.json's row for it lost to db.db's: an import interrupted before
// db.json was deleted leaves exactly that.
func TestImportKeepsReferencedDirectory(t *testing.T) {
	s := New(t.TempDir())
	clone := fakeClone(t, filepath.Join(s.Dir(), "repo0000000000000001"))
	if err := s.save("https://example.com/r", "v1", clone); err != nil {
		t.Fatal(err)
	}
	data := `{"repos":[{"repo":"https://example.com/r","rev":"v1","path":"` + filepath.ToSlash(clone) + `"}]}`
	if err := os.WriteFile(s.legacyPath(), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := New(s.Dir()).GetPath("https://example.com/r", "v1"); got != clone {
		t.Errorf("GetPath = %q, want %q", got, clone)
	}
	if _, err := os.Stat(clone); err != nil {
		t.Errorf("the clone db.db uses was removed: %v", err)
	}
}

// A db.json that does not parse is reported, by name, rather than ignored.
func TestCorruptLegacyIndexIsReported(t *testing.T) {
	s := New(t.TempDir())
	if err := os.WriteFile(s.legacyPath(), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := s.ListRepos()
	if err == nil || !strings.Contains(err.Error(), s.legacyPath()) {
		t.Errorf("ListRepos error = %v, want one naming %s", err, s.legacyPath())
	}
}

// Upstream's mark_config_used does nothing in a store this process cannot
// write to.
func TestMarkConfigUsedSkipsReadonlyStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a directory is always writable to os.access on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes through any mode")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	cfg := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(cfg, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := New(dir).MarkConfigUsed(cfg); err != nil {
		t.Errorf("MarkConfigUsed in a readonly store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, dbFile)); !os.IsNotExist(err) {
		t.Errorf("a readonly store gained a database: %v", err)
	}
}

// The driver reads a '?' in its name as the start of its own parameters; a
// store whose path holds one still works.
func TestStorePathWithQuestionMark(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("'?' cannot appear in a Windows path")
	}
	s := New(filepath.Join(t.TempDir(), "odd?name#x"))
	clone := fakeClone(t, filepath.Join(s.Dir(), "repo0000000000000001"))
	if err := s.save("https://example.com/r", "v1", clone); err != nil {
		t.Fatal(err)
	}
	if got := New(s.Dir()).GetPath("https://example.com/r", "v1"); got != clone {
		t.Errorf("GetPath = %q, want %q", got, clone)
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), dbFile)); err != nil {
		t.Errorf("db.db is not where it belongs: %v", err)
	}
}
