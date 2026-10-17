// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
)

// Upstream parses with argparse, which rejects bad arguments with the
// parser's usage block and "<prog>: error: <message>" on stderr, and exit 2
// (#175). Arguments no subcommand consumed are reported by the top-level
// parser as unrecognized.
func TestArgparseErrors(t *testing.T) {
	pyBin := pythonPreCommit(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())

	for _, tc := range []struct {
		columns string
		args    []string
	}{
		{"", []string{"init-templatedir", "-t", "bogus", "dir"}}, // invalid choice
		{"", []string{"install", "-t", "pre-commit", "-t", "nope"}},
		{"", []string{"run", "--hook-stage", "bogus"}},
		{"", []string{"run", "--color", "bogus"}},
		{"", []string{"run", "--bogus"}}, // unknown option
		{"", []string{"run", "--bogus", "-x", "a", "b"}},
		{"", []string{"gc", "extra"}},
		{"", []string{"init-templatedir"}}, // missing positional
		{"", []string{"try-repo"}},
		{"", []string{"hook-impl"}}, // missing required option
		{"", []string{"run", "-c"}}, // missing value
		{"", []string{"autoupdate", "-j", "x"}},
		{"", []string{"run", "--all-files", "--files", "a"}}, // mutually exclusive
		{"", []string{"run", "--verbose=x"}},
		{"", []string{"bogus"}}, // unknown command
		{"", []string{"--bogus"}},
		{"60", []string{"install", "-t", "bogus"}}, // usage wrapped to COLUMNS
		{"200", []string{"try-repo"}},
	} {
		name := strings.Join(tc.args, " ")
		if tc.columns != "" {
			name = "COLUMNS=" + tc.columns + " " + name
		}
		t.Run(name, func(t *testing.T) {
			t.Setenv("COLUMNS", tc.columns)
			dir := initTestRepo(t, "", "")
			pyOut, pyErr, pyExit := runSplit(t, dir, pyBin, tc.args...)
			goOut, goErr, goExit := runSplit(t, dir, goBinary, tc.args...)
			addExitResult(tc.args[0], "argparse: "+name, pyExit, goExit, pyExit == goExit && goExit == 2, "")
			addOutputResult(tc.args[0], "argparse: "+name,
				pyOut == goOut && pyErr == goErr && strings.Contains(goErr, ": error: "),
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		})
	}
}
