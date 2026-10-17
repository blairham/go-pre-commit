// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Upstream's schemas warn as they validate -- unknown keys, regex-not-glob,
// deprecated stages, mutable revs -- and a non-string key makes its
// warning crash with a TypeError (#187).
func TestSchemaWarnings(t *testing.T) {
	pyBin := pythonPreCommit(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	logPath := regexp.MustCompile(`Check the log at \S+`)

	cases := []struct {
		name, cfg string
		exact     bool // false: the TypeError's two type names depend on
		// CPython's per-process string hashing, so only exit and the
		// error's first words are compared
	}{
		{"unknown root keys", "repos: []\nfoo: 1\nbar: 2\n", true},
		{"unknown repo key", "repos:\n- repo: local\n  hooks: []\n  zz: 1\n", true},
		{"regex not glob and slashes", "repos:\n- repo: local\n  hooks:\n  - id: a\n    name: a\n    entry: e\n" +
			"    language: system\n    files: \"src/*\"\n    exclude: \"a[\\\\\\\\/]b\"\n", true},
		{"top-level regex warnings", "repos: []\nfiles: \"x/*\"\nexclude: \"[/\\\\\\\\]\"\n", true},
		{"warnings in schema order, then a failure", "repos:\n- repo: https://x\n  rev: main\n  hooks:\n  - id: a\n" +
			"    stages: [commit]\n  zz: 1\ndefault_stages: [push]\nfoo: 1\nfiles: \"*.py\"\n", true},
		{"int key", "repos: []\n1: a\n", true},
		{"float key", "repos: []\n1.5: a\n", true},
		{"YAML 1.1 bool key", "repos: []\non: 1\n", true},
		{"None and bool keys", "repos: []\nnull: a\ntrue: b\n", true},
		{"int and str keys", "repos: []\n1: a\nfoo: 2\n", false},
		{"non-string key in default_language_version", "repos: []\ndefault_language_version:\n  python: x\n  1: y\n", true},
		{
			"YAML 1.1 bool value",
			"repos:\n- repo: local\n  hooks:\n  - id: a\n    name: a\n    entry: e\n    language: system\n    args: [yes]\n",
			true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "c.yaml"), []byte(tc.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			pyOut, pyErr, pyExit := runSplit(t, dir, pyBin, "validate-config", "c.yaml")
			goOut, goErr, goExit := runSplit(t, dir, goBinary, "validate-config", "c.yaml")
			addExitResult("validate-config", tc.name+": exit", pyExit, goExit, pyExit == goExit, "")
			pyOut, goOut = logPath.ReplaceAllString(pyOut, "LOG"), logPath.ReplaceAllString(goOut, "LOG")
			match := pyOut == goOut && pyErr == goErr
			if !tc.exact {
				const prefix = "An unexpected error has occurred: TypeError: '<' not supported between instances of "
				match = len(pyOut) > len(prefix) && pyOut[:len(prefix)] == prefix && goOut[:min(len(goOut), len(prefix))] == prefix
			}
			addOutputResult("validate-config", tc.name+": output", match,
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		})
	}
}
