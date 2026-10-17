// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Upstream loads configs and manifests into Python dicts, so a repeated
// mapping key is not an error: the last value wins (#167).
func TestRepeatedKeys(t *testing.T) {
	pyBin := pythonPreCommit(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())

	const lastName = "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: first\n    entry: e\n    language: system\n    name: second\n"
	files := map[string]string{
		"a.yaml": lastName,
		"b.yaml": "repos:\n- repo: local\n  hooks: []\nrepos:\n- repo: local\n  hooks:\n  - {id: y}\n",
		"m.yaml": "- id: x\n  name: a\n  entry: e\n  language: system\n  language: nope\n",
	}
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"validate-config, repeated hook key", []string{"validate-config", "a.yaml"}},
		{"validate-config, the last repos is the one validated", []string{"validate-config", "b.yaml"}},
		{"validate-manifest, the last language is the one validated", []string{"validate-manifest", "m.yaml"}},
		{"run names the hook by its last name", []string{"run", "--all-files", "--color=never"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pyRepo := initTestRepo(t, lastName, "x\n")
			goRepo := initTestRepo(t, lastName, "x\n")
			for _, repo := range []string{pyRepo, goRepo} {
				for name, body := range files {
					if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			pyOut, pyErr, pyExit := runSplit(t, pyRepo, pyBin, tc.args...)
			goOut, goErr, goExit := runSplit(t, goRepo, goBinary, tc.args...)
			addExitResult("repeated keys", tc.name+": exit", pyExit, goExit, pyExit == goExit, "")
			addOutputResult("repeated keys", tc.name+": full output",
				pyOut == goOut && pyErr == goErr,
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		})
	}

	// The issue's case: migration turns sha into a second rev.
	t.Run("autoupdate after sha becomes a second rev", func(t *testing.T) {
		hooks, _ := localHooksRepo(t)
		norm := sameTempDir(t)
		cfg := fmt.Sprintf("repos:\n-   repo: %s\n    rev: v1.1.0\n    sha: v1.0.0\n    hooks:\n    -   id: no-todo\n", hooks)
		pyRepo := initTestRepo(t, cfg, "")
		goRepo := initTestRepo(t, cfg, "")
		pyOut, pyErr, pyExit := runSplit(t, pyRepo, pyBin, "autoupdate", "--color=never")
		goOut, goErr, goExit := runSplit(t, goRepo, goBinary, "autoupdate", "--color=never")
		addExitResult("autoupdate", "repeated rev: exit", pyExit, goExit, pyExit == goExit, "")
		pyOut, goOut = norm(pyOut), norm(goOut)
		addOutputResult("autoupdate", "repeated rev: full output", pyOut == goOut && pyErr == goErr,
			fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		pyCfg := readFile(filepath.Join(pyRepo, ".pre-commit-config.yaml"))
		goCfg := readFile(filepath.Join(goRepo, ".pre-commit-config.yaml"))
		addFSResult("autoupdate", "repeated rev: same config written", pyCfg == goCfg,
			fmt.Sprintf("py=%q go=%q", pyCfg, goCfg))
	})
}
