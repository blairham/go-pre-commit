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

// hookOutputRepo is a repo holding the given local hook plus four files: a.txt
// matches BAD on two lines (one CRLF, one with no final newline), b.txt and
// c.txt do not, and d.txt holds a non-ASCII digit, an ASCII one, trailing
// whitespace and a last line with no newline; e.txt has a vertical tab.
func hookOutputRepo(t *testing.T, hook string) string {
	t.Helper()
	cfg := "repos:\n- repo: local\n  hooks:\n  - id: h\n    name: h\n    " + hook + "\n    files: \\.txt$\n"
	dir := initTestRepo(t, cfg, "")
	for name, content := range map[string]string{
		"a.txt": "xx\nBAD one\r\nok\nBAD two",
		"b.txt": "clean\n",
		"c.txt": "foo\nbar\nbaz\n",
		"d.txt": "x\u0663\nnum 7\ntail \t\nbar",
		"e.txt": "a\vb\naxb\n",
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
		// Upstream searches each line with its "\n" attached (#147).
		{"pygrep by line, \\s$ takes the newline", "entry: '\\s$'\n    language: pygrep", "c.txt:1:foo"},
		{"pygrep by line, [ \\t]+$", "entry: '[ \\t]+$'\n    language: pygrep", "d.txt:3:tail"},
		{"pygrep by line, $ is not before a CR", "entry: 'e$|two$'\n    language: pygrep", "a.txt:4:BAD two"},
		{"pygrep by line, \\Z is the very end", "entry: 'r\\Z'\n    language: pygrep", "d.txt:4:bar"},
		{"pygrep by line, \\d is ASCII", "entry: '\\d'\n    language: pygrep", "d.txt:2:num 7"},
		{"pygrep by line, [^a-z] takes the newline", "entry: '[^a-z]'\n    language: pygrep", "c.txt:3:baz"},
		// Upstream's pattern is a bytes pattern searching bytes (#179).
		{"pygrep by line, . is one byte", "entry: '^x..$'\n    language: pygrep", "d.txt:1:x\u0663"},
		{"pygrep by line, \\xHH is a byte", "entry: '\\xd9\\xa3'\n    language: pygrep", "d.txt:1:x\u0663"},
		{"pygrep by line, [\\x80-\\xff]", "entry: '[\\x80-\\xff]'\n    language: pygrep", "d.txt:1:x\u0663"},
		{
			"pygrep --multiline, . is one byte",
			"entry: 'x..\\n'\n    language: pygrep\n    args: [--multiline]",
			"d.txt:1:x\u0663",
		},
		// Python's bytes \\s includes \\v; RE2's does not (#181).
		{"pygrep by line, \\s matches \\v", "entry: 'a\\sb'\n    language: pygrep", "e.txt:1:a\vb"},
		{"pygrep by line, [\\S] skips \\v", "entry: '^a[\\S]b'\n    language: pygrep", "e.txt:2:axb"},
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
