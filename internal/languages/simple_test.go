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

func TestPygrepNegatePassesWhenNoMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "clean.py")
	os.WriteFile(f, []byte("print('hello')\n"), 0o644)

	p := &Pygrep{}
	code, _, err := p.Run(context.Background(), "", dir, `debugger`, []string{"--negate"}, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (--negate: no files matched, should pass)", code)
	}
}

func TestPygrepNegateFailsWhenMatch(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.py")
	os.WriteFile(f, []byte("debugger\n"), 0o644)

	p := &Pygrep{}
	code, out, err := p.Run(context.Background(), "", dir, `debugger`, []string{"--negate"}, []string{f}, "default")
	if err != nil {
		t.Fatal(err)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (--negate: file matched, should fail)", code)
	}
	if !strings.Contains(string(out), "bad.py") {
		t.Errorf("output %q should reference the matching file", out)
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
