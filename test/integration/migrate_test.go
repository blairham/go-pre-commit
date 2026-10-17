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

// migrate-config rewrites the config in place, so these rows compare the
// whole output and the whole file it leaves behind (#160).
func TestMigrateConfigFullOutput(t *testing.T) {
	pyBin := pythonPreCommit(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())

	cases := []struct{ name, cfg string }{
		{"already migrated", "repos:\n-   repo: https://x\n    rev: v1\n    hooks:\n    -   id: a\n"},
		{"empty file", ""},
		{"list config with a header", "# header\n\n---\n-   repo: https://x\n    sha: v1\n    hooks:\n    -   id: a\n"},
		{"list config at column 0", "- repo: https://x\n  rev: v1\n  hooks:\n  - id: a\n"},
		{"flow list config", "# c\n[{repo: https://x, sha: v1, hooks: [{id: a}]}]\n"},
		{
			"sha to rev, quoted key",
			"repos:\n-   repo: https://x\n    sha: v1\n    hooks: []\n-   repo: https://y\n    'sha': v2\n    hooks: []\n",
		},
		{
			"stages and python_venv, quotes kept",
			"default_stages: [commit, 'push', \"merge-commit\", manual]\nrepos:\n- repo: local\n  hooks:\n" +
				"  - id: a\n    name: a\n    entry: echo\n    language: \"python_venv\"\n    stages:\n    - commit   # c\n    - push\n",
		},
		{"tagged and anchored stages", "default_stages: [!!str commit, &a push]\nrepos: []\n"},
		{"CRLF config migrated", "repos:\r\n-   repo: https://x\r\n    sha: v1\r\n    hooks:\r\n    -   id: a\r\n"},
		{"CRLF config already migrated", "repos:\r\n-   repo: https://x\r\n    rev: v1\r\n    hooks:\r\n    -   id: a\r\n"},
		{
			"non-ASCII config",
			"# café\nrepos:\n- repo: local  # é\n  hooks:\n  - {id: é, name: é, entry: é, language: python_venv, stages: [commit]}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pyRepo := initTestRepo(t, tc.cfg, "")
			goRepo := initTestRepo(t, tc.cfg, "")
			// initTestRepo writes no config for "", and an empty file is
			// what the row is about.
			for _, repo := range []string{pyRepo, goRepo} {
				if err := os.WriteFile(filepath.Join(repo, ".pre-commit-config.yaml"), []byte(tc.cfg), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			pyOut, pyErr, pyExit := runSplit(t, pyRepo, pyBin, "migrate-config")
			goOut, goErr, goExit := runSplit(t, goRepo, goBinary, "migrate-config")
			addExitResult("migrate-config", tc.name+": exit", pyExit, goExit, pyExit == goExit, "")
			addOutputResult("migrate-config", tc.name+": full output",
				pyOut == goOut && pyErr == goErr,
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
			pyCfg := readFile(filepath.Join(pyRepo, ".pre-commit-config.yaml"))
			goCfg := readFile(filepath.Join(goRepo, ".pre-commit-config.yaml"))
			addFSResult("migrate-config", tc.name+": same config written", pyCfg == goCfg,
				fmt.Sprintf("py=%q go=%q", pyCfg, goCfg))
		})
	}

	// PyYAML's parse error cannot be matched word for word; the exit code
	// and the untouched file can.
	t.Run("not YAML", func(t *testing.T) {
		const cfg = "repos: [\n"
		pyRepo := initTestRepo(t, cfg, "")
		goRepo := initTestRepo(t, cfg, "")
		_, _, pyExit := runSplit(t, pyRepo, pyBin, "migrate-config")
		_, _, goExit := runSplit(t, goRepo, goBinary, "migrate-config")
		addExitResult("migrate-config", "not YAML: exit", pyExit, goExit, pyExit == goExit, "")
		pyCfg := readFile(filepath.Join(pyRepo, ".pre-commit-config.yaml"))
		goCfg := readFile(filepath.Join(goRepo, ".pre-commit-config.yaml"))
		addFSResult("migrate-config", "not YAML: config untouched", pyCfg == goCfg && goCfg == cfg,
			fmt.Sprintf("py=%q go=%q", pyCfg, goCfg))
	})
}
