// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dlclark/regexp2"

	"github.com/blairham/go-pre-commit/v4/internal/pcre"
)

// Fail implements the Language interface for the "fail" pseudo-language.
// It always fails with the entry message.
type Fail struct{}

func (f *Fail) Name() string              { return "fail" }
func (f *Fail) EnvironmentDir() string    { return "" }
func (f *Fail) GetDefaultVersion() string { return "default" }

func (f *Fail) HealthCheck(prefix, version string) error { return nil }

func (f *Fail) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	return nil
}

// Run implements Language. As upstream's fail.py does, it prints the entry, a
// blank line, then the files it failed on, one per line.
func (f *Fail) Run(
	_ context.Context,
	_, _, entry string,
	_, fileArgs []string,
	_ string,
) (int, []byte, error) {
	out := entry + "\n\n" + strings.Join(fileArgs, "\n") + "\n"
	return 1, []byte(out), nil
}

// Pygrep implements the Language interface for pygrep hooks.
// pygrep is a regex-based grep that uses Go's regexp.
type Pygrep struct{}

func (p *Pygrep) Name() string              { return "pygrep" }
func (p *Pygrep) EnvironmentDir() string    { return "" }
func (p *Pygrep) GetDefaultVersion() string { return "default" }

func (p *Pygrep) HealthCheck(prefix, version string) error { return nil }

func (p *Pygrep) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	return nil
}

// Run implements Language. It is a port of upstream's
// pre_commit/languages/pygrep.py: the four modes (by line or the whole file
// at once, plain or --negate) print exactly what upstream prints, because a
// hook's output is part of the contract.
func (p *Pygrep) Run(
	_ context.Context,
	_, _, entry string,
	args, fileArgs []string,
	_ string,
) (int, []byte, error) {
	opts := parsePygrepArgs(args)

	pattern := entry
	if opts.multiline {
		// Upstream compiles with re.MULTILINE | re.DOTALL.
		pattern = "(?ms)" + pattern
	}
	if opts.ignoreCase {
		pattern = "(?i)" + pattern
	}

	re, err := pcre.Compile(pattern)
	if err != nil {
		return -1, nil, fmt.Errorf("invalid regex pattern %q: %w", entry, err)
	}

	var out bytes.Buffer
	retv := 0
	for _, filename := range fileArgs {
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}
		var failed bool
		switch {
		case opts.multiline && opts.negate:
			failed = pygrepAtOnceNegated(&out, re, filename, string(data))
		case opts.multiline:
			failed = pygrepAtOnce(&out, re, filename, string(data))
		case opts.negate:
			failed = pygrepByLineNegated(&out, re, filename, string(data))
		default:
			failed = pygrepByLine(&out, re, filename, string(data))
		}
		if failed {
			retv = 1
		}
	}
	if retv == 0 {
		return 0, nil, nil
	}
	return 1, out.Bytes(), nil
}

type pygrepOptions struct {
	ignoreCase, multiline, negate bool
}

// parsePygrepArgs reads pygrep's flags as upstream's argparse parser does,
// including its unique-prefix abbreviations of the long options.
func parsePygrepArgs(args []string) pygrepOptions {
	var o pygrepOptions
	for _, arg := range args {
		if arg == "-i" {
			o.ignoreCase = true
			continue
		}
		if len(arg) <= 2 || !strings.HasPrefix(arg, "--") {
			continue
		}
		switch {
		case abbreviates(arg, "--ignore-case"):
			o.ignoreCase = true
		case abbreviates(arg, "--multiline"):
			o.multiline = true
		case abbreviates(arg, "--negate"):
			o.negate = true
		}
	}
	return o
}

// abbreviates reports whether arg names option the way argparse accepts it:
// spelled out, or as a prefix of it (the three options share none).
func abbreviates(arg, option string) bool {
	return strings.HasPrefix(option, arg)
}

// pygrepLines splits contents into the lines iterating a binary file yields
// in Python, so a final newline adds no empty line after it.
//
// Python searches each line with its "\n" still attached, where a bare `$`
// matches just before it. pcre's `$` matches only at the very end, so the
// line is searched without its "\n" instead, which is what keeps the common
// `[ \t]+$` matching as upstream does.
func pygrepLines(contents string) []string {
	lines := strings.SplitAfter(contents, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\n")
	}
	return lines
}

// pygrepByLine is upstream's _process_filename_by_line: every matching line,
// printed without its line ending straight after "file:N:".
func pygrepByLine(out *bytes.Buffer, re *regexp2.Regexp, filename, contents string) bool {
	failed := false
	for i, line := range pygrepLines(contents) {
		if pcre.Match(re, line) {
			failed = true
			fmt.Fprintf(out, "%s:%d:%s\n", filename, i+1, strings.TrimRight(line, "\r\n"))
		}
	}
	return failed
}

// pygrepAtOnce is upstream's _process_filename_at_once: the first match in the
// whole file, reported at the line it starts on, printed as that whole line
// followed by the rest of the matched text.
func pygrepAtOnce(out *bytes.Buffer, re *regexp2.Regexp, filename, contents string) bool {
	m, err := re.FindStringMatch(contents)
	if err != nil || m == nil {
		return false
	}
	start := runeToByteOffset(contents, m.Index)
	end := runeToByteOffset(contents, m.Index+m.Length)
	lineNo := strings.Count(contents[:start], "\n")

	matched := strings.Split(contents[start:end], "\n")
	matched[0] = strings.Split(contents, "\n")[lineNo]
	fmt.Fprintf(out, "%s:%d:%s\n", filename, lineNo+1, strings.Join(matched, "\n"))
	return true
}

// pygrepByLineNegated is upstream's _process_filename_by_line_negated: a file
// fails, and is named, when no line of it matches.
func pygrepByLineNegated(out *bytes.Buffer, re *regexp2.Regexp, filename, contents string) bool {
	for _, line := range pygrepLines(contents) {
		if pcre.Match(re, line) {
			return false
		}
	}
	fmt.Fprintf(out, "%s\n", filename)
	return true
}

// pygrepAtOnceNegated is upstream's _process_filename_at_once_negated.
func pygrepAtOnceNegated(out *bytes.Buffer, re *regexp2.Regexp, filename, contents string) bool {
	if pcre.Match(re, contents) {
		return false
	}
	fmt.Fprintf(out, "%s\n", filename)
	return true
}

// runeToByteOffset converts regexp2's rune index into a byte offset in s.
// Ranging over a string steps one byte at a time through invalid UTF-8, just
// as regexp2's rune conversion turns each such byte into one U+FFFD, so the
// two count alike.
func runeToByteOffset(s string, runeIdx int) int {
	n := 0
	for i := range s {
		if n == runeIdx {
			return i
		}
		n++
	}
	return len(s)
}

// Unsupported implements the Language interface for system hooks.
type Unsupported struct{}

func (u *Unsupported) Name() string                             { return "unsupported" }
func (u *Unsupported) EnvironmentDir() string                   { return "" }
func (u *Unsupported) GetDefaultVersion() string                { return "default" }
func (u *Unsupported) HealthCheck(prefix, version string) error { return nil }

func (u *Unsupported) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	return nil
}

// Run implements Language.
func (u *Unsupported) Run(
	ctx context.Context,
	_, workDir, entry string,
	args, fileArgs []string,
	_ string,
) (int, []byte, error) {
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, nil)
}

// UnsupportedScript implements the Language interface for script hooks.
type UnsupportedScript struct{}

func (u *UnsupportedScript) Name() string                             { return "unsupported_script" }
func (u *UnsupportedScript) EnvironmentDir() string                   { return "" }
func (u *UnsupportedScript) GetDefaultVersion() string                { return "default" }
func (u *UnsupportedScript) HealthCheck(prefix, version string) error { return nil }

func (u *UnsupportedScript) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	return nil
}

// Run implements Language.
func (u *UnsupportedScript) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	_ string,
) (int, []byte, error) {
	// For script hooks, entry is relative to the hook repo.
	fullEntry := entry
	if prefix != "" {
		fullEntry = prefix + "/" + entry
	}
	return RunHookCommand(ctx, workDir, fullEntry, args, fileArgs, nil)
}
