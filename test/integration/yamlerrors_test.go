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

// A config that is not valid YAML is reported in PyYAML's words -- libyaml's
// scanner and parser errors with their context and marks, the composer's
// and constructor's errors -- under cfgv's frame (#186).
func TestYAMLErrors(t *testing.T) {
	pyBin := pythonPreCommit(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())

	for _, tc := range []struct{ name, cfg string }{
		{"unclosed flow sequence", "repos: [\n"},
		{"unclosed flow mapping", "repos: {a: 1\n"},
		{"unclosed single quote", "repos: 'abc\n"},
		{"unclosed double quote, later line", "repos:\n- repo: local\n  hooks:\n  - id: \"x\n"},
		{"bad indentation, sequence entry", "repos:\n  - a\n - b\n"},
		{"bad indentation, nested key", "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: y\n   entry: z\n"},
		{"sequence at mapping level", "repos: []\n- a\n"},
		{"tab in indentation", "repos:\n\t- a\n"},
		{"tab continuing a plain scalar", "repos:\n  - a\n\t- b\n"},
		{"mapping values not allowed", "a: b: c\n"},
		{"plain value continued into a key", "repos:\n- repo: local\n  hooks:\n  - id: x\n  entry: y\n   name: z\n"},
		{"junk after a quoted value", "a: 'b'c\n"},
		{"junk after a flow entry", "repos:\n- {repo: local, hooks: [{id: x}]\n- repo: meta\n"},
		{"simple key with no colon", "repos:\n- repo: local\n  hooks:\n  - id: x\n    exclude: |\n      a\n  oops\n"},
		{"unknown escape", "repos: \"\\q\"\n"},
		{"duplicate anchor", "a: &x 1\nb: &x 2\nrepos: []\n"},
		{"undefined alias", "repos: *x\n"},
		{"two documents", "repos: []\n---\nrepos: []\n"},
		{"unhashable key", "? [a]\n: b\nrepos: []\n"},
		{"impossible date", "repos: []\nx: 2020-13-01\n"},
		{"CRLF line endings", "repos: [\r\n"},
		{"no trailing newline", "repos: 'x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "c.yaml"), []byte(tc.cfg), 0o644); err != nil {
				t.Fatal(err)
			}
			pyOut, pyErr, pyExit := runSplit(t, dir, pyBin, "validate-config", "c.yaml")
			goOut, goErr, goExit := runSplit(t, dir, goBinary, "validate-config", "c.yaml")
			addExitResult("validate-config", "YAML error, "+tc.name+": exit", pyExit, goExit, pyExit == goExit, "")
			addOutputResult("validate-config", "YAML error, "+tc.name+": output",
				pyOut == goOut && pyErr == goErr,
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		})
	}
}
