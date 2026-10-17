// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hookOutputRepo is a repo holding the given local hook plus three files: a.txt
// matches BAD on two lines (one CRLF, one with no final newline), b.txt and
// c.txt do not.
func hookOutputRepo(t *testing.T, hook string) string {
	t.Helper()
	cfg := "repos:\n- repo: local\n  hooks:\n  - id: h\n    name: h\n    " + hook + "\n    files: \\.txt$\n"
	dir := initTestRepo(t, cfg, "")
	for name, content := range map[string]string{
		"a.txt": "xx\nBAD one\r\nok\nBAD two",
		"b.txt": "clean\n",
		"c.txt": "foo\nbar\nbaz\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runCmd(t, dir, "git", "add", ".")
	return dir
}

// TestHookOutput compares, byte for byte, what the built-in languages print
// for a failing hook. The repos differ only by their temp dir, which no line
// names, so the whole run output must be identical.
func TestHookOutput(t *testing.T) {
	pyBin := pythonPreCommit(t)

	cases := []struct {
		name string
		hook string
		// want is a line upstream must print, proving the case exercises the
		// output rather than passing on an empty one.
		want string
	}{
		{"pygrep by line", "entry: BAD\n    language: pygrep", "a.txt:2:BAD one"},
		{"pygrep by line, $ anchor", "entry: 'o$'\n    language: pygrep", "c.txt:1:foo"},
		{"pygrep --multiline", "entry: 'ar\\nba'\n    language: pygrep\n    args: [--multiline]", "c.txt:2:bar"},
		{"pygrep --negate", "entry: BAD\n    language: pygrep\n    args: [--negate]", "b.txt"},
		{"pygrep --multiline --negate", "entry: BAD\n    language: pygrep\n    args: [--multiline, --negate]", "c.txt"},
		{"fail lists its files", "entry: nope\n    language: fail", "b.txt"},
		{"fail without filenames", "entry: nope\n    language: fail\n    pass_filenames: false", "nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PRE_COMMIT_HOME", t.TempDir())
			pyOut, pyExit := runCmd(t, hookOutputRepo(t, tc.hook), pyBin, "run", "--all-files", "--color=never")
			t.Setenv("PRE_COMMIT_HOME", t.TempDir())
			goOut, goExit := runCmd(t, hookOutputRepo(t, tc.hook), goBinary, "run", "--all-files", "--color=never")

			exercised := pyExit == 1 && strings.Contains(pyOut, "\n"+tc.want+"\n")
			addOutputResult("run", "hook output: "+tc.name,
				exercised && pyExit == goExit && pyOut == goOut,
				fmt.Sprintf("exercised=%v py=%d go=%d\npy:\n%s\ngo:\n%s", exercised, pyExit, goExit, pyOut, goOut))
		})
	}
}
