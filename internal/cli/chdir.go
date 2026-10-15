// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// rootArgs points at the path arguments upstream rewrites when it changes to
// the repository root. Config is always set; the rest only for the commands
// that have them (run and try-repo for Files and CommitMsgFilename, try-repo
// alone for Repo). An empty CommitMsgFilename is the flag not given.
type rootArgs struct {
	Config            *string
	CommitMsgFilename *string
	Repo              *string
	Files             []string
}

// adjustArgsAndChdir ports upstream's main._adjust_args_and_chdir, which
// pre-commit's main runs for every command outside COMMANDS_NO_GIT (clean,
// gc, hazmat, init-templatedir, sample-config, validate-config and
// validate-manifest). Paths given relative to where the user stands are made
// absolute, the process changes to the repository root, and they are then
// made relative to that root -- so a command run from a subdirectory finds
// .pre-commit-config.yaml at the root, and any message that prints the
// config path prints the root-relative form (`-c ../alt.yaml` from sub/ is
// reported as `alt.yaml`).
//
// The config is made absolute only if it exists from here, and is made
// root-relative whether or not it did; a missing `-c x.yaml` given from a
// subdirectory is therefore looked for at the root, as upstream does.
//
// Every path is computed against the physical working directory, which is
// what Python's os.getcwd returns, while a path the user gave absolute is
// left unresolved. Through a symlinked directory that makes the two differ,
// and upstream prints the `..`-laden result; so do we.
//
// The returned error is one of git's FatalError messages, for fatalError.
func adjustArgsAndChdir(a rootArgs) error {
	cwd, err := physicalWd()
	if err != nil {
		return err
	}
	abspath := func(p string) string { return absFrom(cwd, p) }

	if exists(*a.Config) {
		*a.Config = abspath(*a.Config)
	}
	for i, f := range a.Files {
		a.Files[i] = abspath(f)
	}
	if a.CommitMsgFilename != nil && *a.CommitMsgFilename != "" {
		*a.CommitMsgFilename = abspath(*a.CommitMsgFilename)
	}
	if a.Repo != nil && exists(*a.Repo) {
		*a.Repo = abspath(*a.Repo)
	}

	root, err := git.Toplevel(cwd)
	if err != nil {
		return err
	}
	if err := os.Chdir(root); err != nil {
		return err
	}

	relpath := func(p string) string { return relFrom(root, p) }
	*a.Config = relpath(*a.Config)
	for i, f := range a.Files {
		a.Files[i] = relpath(f)
	}
	if a.CommitMsgFilename != nil && *a.CommitMsgFilename != "" {
		*a.CommitMsgFilename = relpath(*a.CommitMsgFilename)
	}
	if a.Repo != nil && exists(*a.Repo) {
		*a.Repo = relpath(*a.Repo)
	}
	return nil
}

// physicalWd is Python's os.getcwd: the working directory with symlinks
// resolved. os.Getwd prefers $PWD when it names the same directory, which
// is the logical path a shell keeps.
func physicalWd() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(wd)
}

// absFrom is os.path.abspath with cwd as the working directory: lexical,
// so `..` is not resolved through symlinks.
func absFrom(cwd, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(cwd, p)
}

// relFrom is os.path.relpath with start as the working directory. Python
// raises where no relative path exists (another drive on Windows); keeping
// the absolute path is the closest that still names the file.
func relFrom(start, p string) string {
	abs := absFrom(start, p)
	rel, err := filepath.Rel(start, abs)
	if err != nil {
		return abs
	}
	return rel
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// fatalError reports err the way upstream's error_handler reports a
// FatalError: one line on stdout naming it, a pointer to the log, and the
// log itself in the store directory. Returns upstream's exit code, 1.
func fatalError(err error) int {
	writeFatalError(storeDirForLog(), err.Error())
	return 1
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

func writeFatalError(storeDir, msg string) {
	line := "An error has occurred: FatalError: " + msg
	fmt.Println(line)

	gitVersion := "unknown"
	if out, err := exec.Command("git", "--version").Output(); err == nil {
		gitVersion = strings.TrimRight(string(out), "\n")
	}
	log := fmt.Sprintf("### version information\n\n```\n"+
		"pre-commit version: %s\n"+
		"git --version: %s\n"+
		"go version: %s\n"+
		"runtime.GOOS: %s\n"+
		"runtime.GOARCH: %s\n"+
		"```\n\n"+
		"### error information\n\n```\n%s\n```\n",
		config.Version, gitVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH, line)

	logPath := filepath.Join(storeDir, "pre-commit.log")
	if err := os.WriteFile(logPath, []byte(log), 0o644); err != nil {
		fmt.Printf("Failed to write to log at %s\n", logPath)
		fmt.Print(log)
		return
	}
	fmt.Printf("Check the log at %s\n", logPath)
}
