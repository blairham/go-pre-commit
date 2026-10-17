// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Pygrep — basic matching
// ---------------------------------------------------------------------------

func TestPygrepMatchesPattern(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.py")
	os.WriteFile(f, []byte("import os\nimport sys\n"), 0o644)

	p := &Pygrep{}
	code, out, err := p.Run(context.Background(), "", dir, `import os`, nil, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (pattern matched)", code)
	}
	if !strings.Contains(string(out), "test.py:1:") {
		t.Errorf("output %q should contain file:line reference", out)
	}
}

func TestPygrepNoMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.py")
	os.WriteFile(f, []byte("print('hello')\n"), 0o644)

	p := &Pygrep{}
	code, _, err := p.Run(context.Background(), "", dir, `import os`, nil, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no match)", code)
	}
}

// ---------------------------------------------------------------------------
// Pygrep — --negate flag
// ---------------------------------------------------------------------------

// --negate inverts the question per file, as upstream does: a file fails, and
// is named, when the pattern is nowhere in it.
func TestPygrepNegatePassesWhenMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "has.py")
	os.WriteFile(f, []byte("# license\nprint('hello')\n"), 0o644)

	p := &Pygrep{}
	code, out, err := p.Run(context.Background(), "", dir, `license`, []string{"--negate"}, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || len(out) != 0 {
		t.Errorf("got (%d, %q), want (0, \"\"): --negate passes a file that matches", code, out)
	}
}

func TestPygrepNegateFailsWhenNoMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "lacks.py")
	os.WriteFile(f, []byte("print('hello')\n"), 0o644)

	p := &Pygrep{}
	code, out, err := p.Run(context.Background(), "", dir, `license`, []string{"--negate"}, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if want := f + "\n"; code != 1 || string(out) != want {
		t.Errorf("got (%d, %q), want (1, %q)", code, out, want)
	}
}

// TestPygrepOutputMatchesUpstream pins each of upstream's four modes to the
// bytes pre_commit/languages/pygrep.py writes, measured against 4.7.0.
func TestPygrepOutputMatchesUpstream(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		f := filepath.Join(dir, name)
		if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return f
	}
	a := write("a.txt", "xx\nBAD one\r\nok\nBAD two")
	b := write("b.txt", "clean\n")
	c := write("c.txt", "foo\nbar\nbaz\n")
	files := []string{a, b, c}

	cases := []struct {
		name  string
		entry string
		want  string
		args  []string
		code  int
	}{
		{name: "by line", entry: "BAD", code: 1, want: a + ":2:BAD one\n" + a + ":4:BAD two\n"},
		{name: "by line, $ before the newline", entry: "o$", code: 1, want: a + ":4:BAD two\n" + c + ":1:foo\n"},
		{name: "by line, $ is not before a CR", entry: "e$", code: 0, want: ""},
		{name: "by line, no match", entry: "nope", code: 0, want: ""},
		{
			name:  "multiline prints the whole first line",
			entry: "ar\nba",
			args:  []string{"--multiline"},
			code:  1,
			want:  c + ":2:bar\nba\n",
		},
		{
			name:  "multiline ^ and $ are per line",
			entry: "^ba.$",
			args:  []string{"--multiline"},
			code:  1,
			want:  c + ":2:bar\n",
		},
		{
			name:  "negate names files without a match",
			entry: "BAD",
			args:  []string{"--negate"},
			code:  1,
			want:  b + "\n" + c + "\n",
		},
		{
			name:  "multiline negate",
			entry: "BAD",
			args:  []string{"--multiline", "--negate"},
			code:  1,
			want:  b + "\n" + c + "\n",
		},
		{
			name:  "abbreviated long options",
			entry: "bad",
			args:  []string{"--ign", "--neg"},
			code:  1,
			want:  b + "\n" + c + "\n",
		},
		{name: "ignore-case", entry: "bad two", args: []string{"--ignore-case"}, code: 1, want: a + ":4:BAD two\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Pygrep{}
			code, out, err := p.Run(context.Background(), "", dir, tc.entry, tc.args, files, "default")
			if err != nil {
				t.Fatal(err)
			}
			if code != tc.code || string(out) != tc.want {
				t.Errorf("got (%d, %q), want (%d, %q)", code, out, tc.code, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pygrep — case insensitive
// ---------------------------------------------------------------------------

func TestPygrepCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("TODO: fix this\n"), 0o644)

	p := &Pygrep{}
	code, _, err := p.Run(context.Background(), "", dir, `todo`, []string{"-i"}, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (case-insensitive match)", code)
	}
}

// ---------------------------------------------------------------------------
// Pygrep — multiline
// ---------------------------------------------------------------------------

func TestPygrepMultiline(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("line1\nline2\nline3\n"), 0o644)

	p := &Pygrep{}
	code, _, err := p.Run(context.Background(), "", dir, `line1.*line2`, []string{"--multiline"}, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (multiline match)", code)
	}
}

func TestPygrepMultilineNoMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("line1\nline2\n"), 0o644)

	p := &Pygrep{}
	code, _, err := p.Run(context.Background(), "", dir, `line1.*line3`, []string{"--multiline"}, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no multiline match)", code)
	}
}

// ---------------------------------------------------------------------------
// Pygrep — invalid regex
// ---------------------------------------------------------------------------

func TestPygrepInvalidRegex(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	os.WriteFile(f, []byte("content\n"), 0o644)

	p := &Pygrep{}
	_, _, err := p.Run(context.Background(), "", dir, `[invalid`, nil, []string{f}, "default")
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

// ---------------------------------------------------------------------------
// Pygrep — multiple files
// ---------------------------------------------------------------------------

func TestPygrepMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.py")
	f2 := filepath.Join(dir, "b.py")
	f3 := filepath.Join(dir, "c.py")
	os.WriteFile(f1, []byte("import os\n"), 0o644)
	os.WriteFile(f2, []byte("print('ok')\n"), 0o644)
	os.WriteFile(f3, []byte("import os\nimport sys\n"), 0o644)

	p := &Pygrep{}
	code, out, err := p.Run(context.Background(), "", dir, `import os`, nil, []string{f1, f2, f3}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	// Should match in a.py and c.py but not b.py.
	outStr := string(out)
	if !strings.Contains(outStr, "a.py") {
		t.Error("expected a.py in output")
	}
	if strings.Contains(outStr, "b.py") {
		t.Error("b.py should not appear in output")
	}
	if !strings.Contains(outStr, "c.py") {
		t.Error("expected c.py in output")
	}
}

// ---------------------------------------------------------------------------
// Pygrep — no files
// ---------------------------------------------------------------------------

func TestPygrepNoFiles(t *testing.T) {
	p := &Pygrep{}
	code, _, err := p.Run(context.Background(), "", t.TempDir(), `pattern`, nil, nil, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (no files to check)", code)
	}
}

// ---------------------------------------------------------------------------
// Fail language
// ---------------------------------------------------------------------------

func TestFailAlwaysReturns1(t *testing.T) {
	f := &Fail{}
	code, out, err := f.Run(context.Background(), "", t.TempDir(), "this should fail", nil, nil, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(string(out), "this should fail") {
		t.Errorf("output %q should contain the entry message", out)
	}
}

// TestFailOutputMatchesUpstream pins the bytes upstream's fail.py returns:
// the entry, a blank line, then each file on its own line.
func TestFailOutputMatchesUpstream(t *testing.T) {
	for _, tc := range []struct {
		name  string
		want  string
		files []string
	}{
		{name: "two files", files: []string{"a.txt", "b.txt"}, want: "nope\n\na.txt\nb.txt\n"},
		{name: "no files", want: "nope\n\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, err := (&Fail{}).Run(context.Background(), "", t.TempDir(), "nope", nil, tc.files, "default")
			if err != nil {
				t.Fatal(err)
			}
			if code != 1 || string(out) != tc.want {
				t.Errorf("got (%d, %q), want (1, %q)", code, out, tc.want)
			}
		})
	}
}
