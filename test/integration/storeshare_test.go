//go:build integration

// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// goHookRepo commits a hook repository with one golang hook, whose
// environment installs from the repository alone (no network), and returns
// it with the commit's sha.
func goHookRepo(t *testing.T) (dir, rev string) {
	t.Helper()
	dir = t.TempDir()
	for name, body := range map[string]string{
		"go.mod":                 "module example.com/hook\n\ngo 1.21\n",
		"main.go":                "package main\n\nfunc main() {}\n",
		".pre-commit-hooks.yaml": "-   id: go-hook\n    name: go hook\n    entry: hook\n    language: golang\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-qm", "hook"},
	} {
		if out, code := runCmd(t, dir, "git", args...); code != 0 {
			t.Fatalf("git %v:\n%s", args, out)
		}
	}
	rev, _ = runCmd(t, dir, "git", "rev-parse", "HEAD")
	return dir, strings.TrimSpace(rev)
}

// storeDirs lists the directories directly in a store: one per clone.
func storeDirs(t *testing.T, home string) []string {
	t.Helper()
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	return dirs
}

// lastLine is the last line of out, which is where gc reports its count.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	return lines[len(lines)-1]
}

// golangEnvs lists the golang environments in a store.
func golangEnvs(t *testing.T, home string) []string {
	t.Helper()
	envs, err := filepath.Glob(filepath.Join(home, "*", "golangenv-*"))
	if err != nil {
		t.Fatal(err)
	}
	return envs
}

// TestStoreSharedWithPython runs Python pre-commit and this port against one
// PRE_COMMIT_HOME (#165). Whichever tool goes second finds the hook repository
// the first cloned and the environment it installed, through upstream's
// db.db and install-state files: it clones nothing and installs nothing. And
// each tool's gc sees, and collects, the repos the other cloned.
//
// The first run is checked to have cloned and installed, so a second run
// that prints neither is a reuse rather than a run that had nothing to do.
// The environment carries a marker file the second run must leave alone:
// installing removes the environment directory first.
func TestStoreSharedWithPython(t *testing.T) {
	pyBin := pythonPreCommit(t)
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	hookRepo, rev := goHookRepo(t)
	cfg := fmt.Sprintf("repos:\n-   repo: %s\n    rev: %s\n    hooks:\n    -   id: go-hook\n", hookRepo, rev)
	const (
		cloned    = "Initializing environment for "
		installed = "Installing environment for "
	)

	tools := []struct{ name, bin string }{{"python", pyBin}, {"go", goBinary}}
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		first, second := tools[order[0]], tools[order[1]]
		label := first.name + " then " + second.name

		t.Run("reuse/"+label, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("PRE_COMMIT_HOME", home)
			repo := initTestRepo(t, cfg, "x\n")

			out1, code1 := runCmd(t, repo, first.bin, "install-hooks")
			envs := golangEnvs(t, home)
			if code1 != 0 || !strings.Contains(out1, cloned) || !strings.Contains(out1, installed) || len(envs) != 1 {
				t.Fatalf("%s install-hooks did not clone and install (exit %d, envs %q):\n%s", first.name, code1, envs, out1)
			}
			marker := filepath.Join(envs[0], "marker-from-"+first.name)
			if err := os.WriteFile(marker, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			before := storeDirs(t, home)

			out2, code2 := runCmd(t, repo, second.bin, "run", "--all-files")
			after := storeDirs(t, home)
			_, markerErr := os.Stat(marker)
			addFSResult("shared PRE_COMMIT_HOME", label+": second tool reuses the clone and env",
				code2 == 0 && !strings.Contains(out2, cloned) && !strings.Contains(out2, installed) &&
					slices.Equal(before, after) && markerErr == nil,
				fmt.Sprintf("%s run exit %d, marker: %v\nstore before %q, after %q\noutput:\n%s",
					second.name, code2, markerErr, before, after, out2))
		})

		t.Run("gc/"+first.name+" clones, "+second.name+" collects", func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("PRE_COMMIT_HOME", home)
			repo := initTestRepo(t, cfg, "x\n")

			out, code := runCmd(t, repo, first.bin, "install-hooks")
			clones := storeDirs(t, home)
			if code != 0 || len(clones) != 1 {
				t.Fatalf("%s install-hooks: exit %d, clones %q:\n%s", first.name, code, clones, out)
			}

			// Still used by a tracked config: kept.
			keep, _ := runCmd(t, repo, second.bin, "gc")
			// No longer used: collected.
			if err := os.WriteFile(filepath.Join(repo, ".pre-commit-config.yaml"), []byte("repos: []\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			drop, dropCode := runCmd(t, repo, second.bin, "gc")
			left := storeDirs(t, home)
			addFSResult("gc", first.name+"'s repo, "+second.name+"'s gc: kept while used, removed after",
				lastLine(keep) == "0 repo(s) removed." &&
					dropCode == 0 && lastLine(drop) == "1 repo(s) removed." && len(left) == 0,
				fmt.Sprintf("while used: %q\nafter: %q (exit %d), store left %q", keep, drop, dropCode, left))
		})
	}

	// A store either tool creates holds the same files, and the same schema:
	// SQLite keeps each table's CREATE statement in sqlite_master.
	t.Run("layout", func(t *testing.T) {
		python3, err := exec.LookPath("python3")
		if err != nil {
			t.Skip("python3 not on PATH")
		}
		const dump = "import sqlite3, sys; " +
			"print(sqlite3.connect(sys.argv[1]).execute('SELECT type, name, sql FROM sqlite_master ORDER BY rowid').fetchall())"
		layout := func(bin string) string {
			home := filepath.Join(t.TempDir(), "store")
			t.Setenv("PRE_COMMIT_HOME", home)
			if out, code := runCmd(t, ".", bin, "gc"); code != 0 {
				t.Fatalf("%s gc: exit %d\n%s", bin, code, out)
			}
			entries, err := os.ReadDir(home)
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			schema, _ := runCmd(t, ".", python3, "-I", "-c", dump, filepath.Join(home, "db.db"))
			readme := readFile(filepath.Join(home, "README"))
			return fmt.Sprintf("files: %q\nREADME: %q\nschema: %s", names, readme, schema)
		}
		py, gobin := layout(pyBin), layout(goBinary)
		addFSResult("gc", "a new store's files and db.db schema", py == gobin,
			fmt.Sprintf("python:\n%s\ngo:\n%s", py, gobin))
	})
}
