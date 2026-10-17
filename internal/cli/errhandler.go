// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/fatal"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// This file ports pre_commit/error_handler.py. Upstream runs every command
// inside error_handler(), which turns an exception into one line on stdout,
// a pointer to pre-commit.log in the store, and an exit code:
//
//	FatalError (and InvalidConfigError, InvalidManifestError, ...)
//	    "An error has occurred: <Type>: <msg>"             exit 1
//	KeyboardInterrupt
//	    "Interrupted (^C): KeyboardInterrupt: "            exit 130
//	anything else
//	    "An unexpected error has occurred: <Type>: <msg>"  exit 3
//
// Go has no exceptions, so a command hands its error to reportError and
// returns what it returns; Run recovers a panic and reports it as
// unexpected, and reports an interrupt once the command has unwound.

var (
	// interrupted is set when SIGINT arrives while a command runs.
	interrupted atomic.Bool
	// errorReported is set once reportError has run, so an interrupt that a
	// command already reported is not reported twice.
	errorReported atomic.Bool
)

// keyboardInterrupt stands for Python's KeyboardInterrupt, whose str() is
// empty.
type keyboardInterrupt struct{}

func (keyboardInterrupt) Error() string     { return "" }
func (keyboardInterrupt) ErrorType() string { return "KeyboardInterrupt" }

// fatalError reports err as upstream's plain FatalError.
func fatalError(err error) int {
	return reportError(fatal.New(err.Error()))
}

// unexpectedError reports err as an exception upstream does not catch
// (exit 3). It is reportError, named for the call sites that mean it.
func unexpectedError(err error) int {
	return reportError(err)
}

// reportError is error_handler's except clause: it classifies err, prints
// the line, writes the log, and returns the exit code. An error that comes
// back after SIGINT is the interrupt unwinding, and is reported as one.
func reportError(err error) int {
	if interrupted.Load() {
		err = keyboardInterrupt{}
	}
	prefix, code := classifyError(err)
	typ, msg := errorTypeAndMessage(err)
	return logAndExit(storeDirForLog(), prefix, code, typ, msg, string(debug.Stack()))
}

// reportPanic reports a recovered panic as the unexpected error it would
// have been upstream, with the panicking goroutine's stack as the traceback.
func reportPanic(r any, stack []byte) int {
	err, ok := r.(error)
	if !ok {
		err = &typedError{typ: "panic", msg: fmt.Sprint(r)}
	}
	typ, msg := errorTypeAndMessage(err)
	return logAndExit(
		storeDirForLog(),
		"An unexpected error has occurred",
		config.ExitCodeUnexpected,
		typ,
		msg,
		string(stack),
	)
}

func classifyError(err error) (string, int) {
	var fe *fatal.Error
	switch {
	case errors.As(err, new(keyboardInterrupt)):
		return "Interrupted (^C)", config.ExitCodeInterrupt
	case errors.As(err, &fe):
		return "An error has occurred", config.ExitCodeError
	default:
		return "An unexpected error has occurred", config.ExitCodeUnexpected
	}
}

// typedError is an error that names the type upstream would have raised.
type typedError struct {
	typ string
	msg string
}

func (e *typedError) Error() string     { return e.msg }
func (e *typedError) ErrorType() string { return e.typ }

// errorTypeAndMessage is `type(exc).__name__` and `str(exc)`. A fatal error
// carries upstream's class name; an error that implements ErrorType names
// its own; otherwise it is the Go type of the first error in the chain that
// is more than a message or a wrapper, which is the nearest thing Go has to
// an exception class.
func errorTypeAndMessage(err error) (string, string) {
	var fe *fatal.Error
	if errors.As(err, &fe) {
		return fe.Type, fe.Msg
	}
	// As with a fatal error, the named error's own message: a wrapper's
	// context is not part of str(exc).
	var named interface {
		error
		ErrorType() string
	}
	if errors.As(err, &named) {
		return named.ErrorType(), named.Error()
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		if errno, ok := pe.Err.(syscall.Errno); ok {
			return osErrorType(errno), osErrorMessage(errno, pe.Path)
		}
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		t := reflect.TypeOf(e)
		switch t.String() {
		case "*errors.errorString", "*fmt.wrapError", "*fmt.wrapErrors", "*errors.joinError":
			continue
		}
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Name() != "" {
			return t.Name(), err.Error()
		}
	}
	return "Error", err.Error()
}

// osErrorType is the OSError subclass Python raises for errno.
func osErrorType(errno syscall.Errno) string {
	switch errno {
	case syscall.ENOENT:
		return "FileNotFoundError"
	case syscall.EACCES, syscall.EPERM:
		return "PermissionError"
	case syscall.EISDIR:
		return "IsADirectoryError"
	case syscall.ENOTDIR:
		return "NotADirectoryError"
	case syscall.EEXIST:
		return "FileExistsError"
	}
	return "OSError"
}

// osErrorMessage is str() of Python's OSError for a path:
// "[Errno 2] No such file or directory: 'x'".
func osErrorMessage(errno syscall.Errno, path string) string {
	msg := errno.Error()
	if msg != "" {
		msg = strings.ToUpper(msg[:1]) + msg[1:]
	}
	return fmt.Sprintf("[Errno %d] %s: %s", int(errno), msg, pyQuote(path))
}

// pyQuote is Python's repr() of a path string.
func pyQuote(s string) string {
	quote := "'"
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = `"`
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	if quote == "'" {
		s = strings.ReplaceAll(s, "'", `\'`)
	}
	return quote + s + quote
}

// storeDirForLog is the directory upstream's Store() resolves -- realpath'd,
// and created if missing -- which is where it writes pre-commit.log.
func storeDirForLog() string {
	dir := store.DefaultDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dir
	}
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	return dir
}

// logAndExit is upstream's _log_and_exit: the error line and the log
// pointer on stdout, and the log itself -- version information, the error,
// and the traceback -- in storeDir/pre-commit.log, or on stdout when the
// store is not writable. Go's runtime details stand in for Python's.
func logAndExit(storeDir, prefix string, code int, typ, msg, traceback string) int {
	errorReported.Store(true)
	errorMsg := prefix + ": " + typ + ": " + msg
	fmt.Println(errorMsg)

	gitVersion := ""
	if out, err := exec.Command("git", "--version").Output(); err == nil {
		gitVersion = strings.TrimRight(string(out), "\n")
	}
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	log := fmt.Sprintf("### version information\n\n```\n"+
		"pre-commit version: %s\n"+
		"git --version: %s\n"+
		"go version:\n    %s\n"+
		"executable: %s\n"+
		"runtime.GOOS: %s\n"+
		"runtime.GOARCH: %s\n"+
		"```\n\n"+
		"### error information\n\n```\n%s\n```\n\n```\n%s\n```\n",
		config.Version, gitVersion, runtime.Version(), executable, runtime.GOOS, runtime.GOARCH,
		errorMsg, strings.TrimRight(traceback, " \t\r\n"))

	// Upstream checks the store is writable, then writes; a write that
	// fails after that is not reported either.
	logPath := filepath.Join(storeDir, "pre-commit.log")
	if err := os.WriteFile(logPath, []byte(log), 0o644); err != nil {
		fmt.Printf("Failed to write to log at %s\n", logPath)
		fmt.Print(log)
		return code
	}
	fmt.Printf("Check the log at %s\n", logPath)
	return code
}
