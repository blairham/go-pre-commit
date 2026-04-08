package languages

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunCommand is a helper to run a command and capture output.
func RunCommand(ctx context.Context, dir, name string, args ...string) (int, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return -1, buf.Bytes(), err
		}
	}
	return exitCode, buf.Bytes(), nil
}

// RunHookCommand is the standard way to run a hook entry with file args.
// It splits the entry, appends args and file args, and runs the command.
func RunHookCommand(ctx context.Context, dir, entry string, args, fileArgs []string, env []string) (int, []byte, error) {
	parts := ParseEntry(entry)
	if len(parts) == 0 {
		return -1, nil, fmt.Errorf("empty entry")
	}

	cmdArgs := make([]string, 0, len(parts)-1+len(args)+len(fileArgs))
	cmdArgs = append(cmdArgs, parts[1:]...)
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, fileArgs...)

	// exec.Command resolves the binary at call-time using the CURRENT process
	// PATH, not the custom env we pass in.  Since the binary may only exist
	// inside a virtualenv bin dir (e.g. a pip-installed console script), we
	// resolve it ourselves using the custom env's PATH first – matching the
	// Python implementation where envcontext patches os.environ before exec.
	resolvedBin, err := lookPathInEnv(parts[0], env)
	if err != nil {
		resolvedBin = parts[0]
	}

	cmd := exec.CommandContext(ctx, resolvedBin, cmdArgs...)
	cmd.Dir = dir
	// Put custom env vars first so our PATH takes precedence (mirrors Python's
	// envcontext behavior of replacing os.environ entries).
	cmd.Env = append(append([]string{}, env...), os.Environ()...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return -1, buf.Bytes(), err
		}
	}
	return exitCode, buf.Bytes(), nil
}

// ParseEntry splits an entry string respecting quotes.
// Matches the behavior of Python's shlex.split() used by the upstream
// pre_commit.lang_base.hook_cmd helper: quoted empty strings (”, "") produce
// an empty-string token.
func ParseEntry(entry string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)
	wasQuoted := false // track if the current token came from a quoted region

	for i := 0; i < len(entry); i++ {
		c := entry[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(c)
			}
		} else if c == '\'' || c == '"' {
			inQuote = true
			quoteChar = c
			wasQuoted = true
		} else if c == ' ' || c == '\t' {
			if current.Len() > 0 || wasQuoted {
				parts = append(parts, current.String())
				current.Reset()
				wasQuoted = false
			}
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 || wasQuoted {
		parts = append(parts, current.String())
	}
	return parts
}

// lookPathInEnv finds an executable by searching the PATH entries in the given
// env slice (e.g. ["PATH=/venv/bin:/usr/bin", ...]). Falls back to
// exec.LookPath (current-process PATH) if not found.
// This mirrors Python's envcontext which patches os.environ before subprocess
// exec so that venv-only binaries are found.
func lookPathInEnv(name string, env []string) (string, error) {
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
		return "", &os.PathError{Op: "lookpath", Path: name, Err: os.ErrNotExist}
	}
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			pathVal := strings.TrimPrefix(e, "PATH=")
			for _, dir := range filepath.SplitList(pathVal) {
				full := filepath.Join(dir, name)
				if info, err := os.Stat(full); err == nil && !info.IsDir() {
					return full, nil
				}
			}
		}
	}
	// Fall back to current-process PATH.
	return exec.LookPath(name)
}

// FindExecutable looks for an executable in the given paths.
func FindExecutable(name string, paths ...string) (string, error) {
	for _, dir := range paths {
		full := filepath.Join(dir, name)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full, nil
		}
	}
	// Fall back to PATH.
	return exec.LookPath(name)
}

// PrependPath prepends a directory to the PATH env var.
func PrependPath(dir string) string {
	return fmt.Sprintf("PATH=%s%c%s", dir, os.PathListSeparator, os.Getenv("PATH"))
}
