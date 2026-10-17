// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/fatal"
	"github.com/blairham/go-pre-commit/v4/internal/git"
)

// captureStdout runs fn with os.Stdout redirected, and returns what it
// printed alongside fn's result.
func captureStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	code := fn()
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out), code
}

func TestReportError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PRE_COMMIT_HOME", home)
	store := storeDirForLog()
	logPath := filepath.Join(store, "pre-commit.log")
	cpe := &git.CalledProcessError{Cmd: []string{"git", "fetch"}, ReturnCode: 128}

	for _, tc := range []struct {
		name     string
		err      error
		line     string
		wantCode int
	}{
		{
			"FatalError", fatal.New(git.ErrInsideGitDir.Error()),
			"An error has occurred: FatalError: git toplevel unexpectedly empty! make sure you are " +
				"not inside the `.git` directory of your repository.",
			1,
		},
		{
			"InvalidConfigError", &fatal.Error{Type: "InvalidConfigError", Msg: "\n=====> x is not a file"},
			"An error has occurred: InvalidConfigError: \n=====> x is not a file",
			1,
		},
		{
			// A fatal error keeps its own message through a wrapper, as
			// str(exc) would.
			"wrapped fatal", fmt.Errorf("context: %w", fatal.New("boom")),
			"An error has occurred: FatalError: boom",
			1,
		},
		{
			"unexpected", &fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist},
			"An unexpected error has occurred: PathError: open x: file does not exist",
			3,
		},
		{
			// A named error keeps its own message through a wrapper too.
			"wrapped CalledProcessError",
			fmt.Errorf("resolve: %w", cpe),
			"An unexpected error has occurred: CalledProcessError: " + cpe.Error(),
			3,
		},
		{
			"unexpected, a plain message", errors.New("boom"),
			"An unexpected error has occurred: Error: boom",
			3,
		},
		{
			"interrupt",
			keyboardInterrupt{},
			"Interrupted (^C): KeyboardInterrupt: ",
			130,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interrupted.Store(false)
			out, code := captureStdout(t, func() int { return reportError(tc.err) })
			if code != tc.wantCode {
				t.Errorf("code = %d, want %d", code, tc.wantCode)
			}
			if want := tc.line + "\nCheck the log at " + logPath + "\n"; out != want {
				t.Errorf("stdout = %q, want %q", out, want)
			}
			log, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, section := range []string{
				"### version information\n\n```\npre-commit version: ",
				"\ngit --version: ",
				"\n### error information\n\n```\n" + tc.line + "\n```\n\n```\n",
			} {
				if !strings.Contains(string(log), section) {
					t.Errorf("log lacks %q:\n%s", section, log)
				}
			}
		})
	}
}

// After SIGINT, the error a command unwinds with is the interrupt.
func TestReportErrorAfterInterrupt(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	interrupted.Store(true)
	t.Cleanup(func() { interrupted.Store(false) })
	out, code := captureStdout(t, func() int { return reportError(errors.New("context canceled")) })
	if code != 130 || !strings.HasPrefix(out, "Interrupted (^C): KeyboardInterrupt: \n") {
		t.Errorf("code = %d, out = %q", code, out)
	}
}

func TestReportPanic(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	out, code := captureStdout(t, func() int { return reportPanic("oops", []byte("goroutine 1 [running]:")) })
	if code != 3 || !strings.HasPrefix(out, "An unexpected error has occurred: panic: oops\n") {
		t.Errorf("code = %d, out = %q", code, out)
	}
	log, _ := os.ReadFile(filepath.Join(storeDirForLog(), "pre-commit.log"))
	if !strings.Contains(string(log), "```\ngoroutine 1 [running]:\n```") {
		t.Errorf("log lacks the stack:\n%s", log)
	}
}

// An unwritable store prints the log to stdout instead.
func TestLogAndExitUnwritableStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	out, code := captureStdout(t, func() int {
		return logAndExit(dir, "An error has occurred", 1, "FatalError", "x", "")
	})
	if code != 1 {
		t.Errorf("code = %d", code)
	}
	want := "An error has occurred: FatalError: x\nFailed to write to log at " +
		filepath.Join(dir, "pre-commit.log") + "\n### version information\n"
	if !strings.HasPrefix(out, want) {
		t.Errorf("out = %q, want prefix %q", out, want)
	}
}

func TestValidateConfigOutput(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	os.WriteFile("bad.yaml", []byte("repos: 5\n"), 0o644)
	os.WriteFile("good.yaml", []byte("repos: []\n"), 0o644)

	out, code := captureStdout(t, func() int {
		return (&ValidateConfigCommand{}).Run([]string{"good.yaml"})
	})
	if code != 0 || out != "" {
		t.Errorf("valid: code = %d, out = %q; upstream prints nothing", code, out)
	}

	out, code = captureStdout(t, func() int {
		return (&ValidateConfigCommand{}).Run([]string{"bad.yaml", "good.yaml", "missing.yaml"})
	})
	want := "\n==> File bad.yaml\n==> At Config()\n==> At key: repos\n=====> Expected array but got 'int'\n" +
		"\n=====> missing.yaml is not a file\n"
	if code != 1 || out != want {
		t.Errorf("code = %d, out = %q, want %q", code, out, want)
	}
}

// An OSError reads as Python's: its subclass, errno, strerror and path.
func TestErrorTypeAndMessageOSError(t *testing.T) {
	_, err := os.ReadFile(filepath.Join(t.TempDir(), "nope.yaml"))
	typ, msg := errorTypeAndMessage(err)
	if typ != "FileNotFoundError" || !strings.HasPrefix(msg, "[Errno 2] No such file or directory: '") {
		t.Errorf("got %s: %s", typ, msg)
	}
}
