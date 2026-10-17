// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
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

	pattern := pygrepBytesPattern(entry)
	if opts.multiline {
		// Upstream compiles with re.MULTILINE | re.DOTALL.
		pattern = "(?ms)" + pattern
	} else {
		pattern = pygrepLineAnchors(pattern)
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
// in Python: each keeps its "\n", and a final newline adds no empty line
// after it. Upstream searches each line with that "\n" attached, so a pattern
// that can consume it (`\s$`, `[^a-z]`) matches there too.
func pygrepLines(contents string) []string {
	lines := strings.SplitAfter(contents, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// pygrepLineAnchors rewrites a by-line pattern's end anchors to Python's
// meaning for a line that still carries its "\n". pcre's RE2 mode reads `$`
// as the very end of the text, where Python's `$` also matches just before a
// final "\n"; and Python's `\Z` is the very end, which `\z` spells in every
// regexp2 mode. A `$` or `\Z` inside a character class, or escaped, is left
// alone.
func pygrepLineAnchors(pattern string) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\\' && i+1 < len(pattern):
			if !inClass && pattern[i+1] == 'Z' {
				b.WriteString(`\z`)
			} else {
				b.WriteString(pattern[i : i+2])
			}
			i++
		case inClass:
			b.WriteByte(c)
			if c == ']' {
				inClass = false
			}
		case c == '[':
			inClass = true
			b.WriteByte(c)
			// A ']' first in the set (after an optional '^') is literal.
			if i+1 < len(pattern) && pattern[i+1] == '^' {
				b.WriteByte('^')
				i++
			}
			if i+1 < len(pattern) && pattern[i+1] == ']' {
				b.WriteByte(']')
				i++
			}
		case c == '$':
			b.WriteString(`(?=\n?\z)`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// pygrepByLine is upstream's _process_filename_by_line: every matching line,
// printed without its line ending straight after "file:N:".
func pygrepByLine(out *bytes.Buffer, re *regexp2.Regexp, filename, contents string) bool {
	failed := false
	for i, line := range pygrepLines(contents) {
		if pcre.Match(re, pygrepSubject(line)) {
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
	m, err := re.FindStringMatch(pygrepSubject(contents))
	if err != nil || m == nil {
		return false
	}
	// One rune of the subject is one byte of contents.
	start, end := m.Index, m.Index+m.Length
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
		if pcre.Match(re, pygrepSubject(line)) {
			return false
		}
	}
	fmt.Fprintf(out, "%s\n", filename)
	return true
}

// pygrepAtOnceNegated is upstream's _process_filename_at_once_negated.
func pygrepAtOnceNegated(out *bytes.Buffer, re *regexp2.Regexp, filename, contents string) bool {
	if pcre.Match(re, pygrepSubject(contents)) {
		return false
	}
	fmt.Fprintf(out, "%s\n", filename)
	return true
}

// Upstream compiles the entry as a bytes pattern and searches bytes, so `.`,
// a negated class or a counted repeat steps one byte at a time, and \w, \d,
// \s, \b and IGNORECASE know only ASCII. regexp2 steps through runes, so the
// subject is handed to it one rune per byte: an ASCII byte as itself, and a
// byte from 0x80 up as a private-use rune (pygrepHighBase plus the byte),
// which no class, word test or case fold treats as anything but itself. The
// pattern's own non-ASCII bytes, and its \xHH and octal escapes for them,
// are mapped the same way.
const pygrepHighBase = 0xE000

// pygrepSubject maps s byte by byte into the runes the pattern is matched
// against; a match's rune offsets are byte offsets into s.
func pygrepSubject(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		b.WriteRune(pygrepByteRune(s[i]))
	}
	return b.String()
}

func pygrepByteRune(c byte) rune {
	if c < 0x80 {
		return rune(c)
	}
	return pygrepHighBase + rune(c)
}

// pygrepBytesPattern is the entry as the bytes pattern upstream compiles
// (args.pattern.encode()), spelled in pygrepSubject's runes: each non-ASCII
// byte of it, and each \xHH or three-digit octal escape for a byte from 0x80
// up, becomes that byte's rune. Every other escape is left as written.
func pygrepBytesPattern(pattern string) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c >= 0x80:
			b.WriteRune(pygrepByteRune(c))
			continue
		case c == '[' && !inClass:
			inClass = true
			b.WriteByte(c)
			// A ']' first in the set (after an optional '^') is literal.
			if i+1 < len(pattern) && pattern[i+1] == '^' {
				b.WriteByte('^')
				i++
			}
			if i+1 < len(pattern) && pattern[i+1] == ']' {
				b.WriteByte(']')
				i++
			}
			continue
		case c == ']' && inClass:
			inClass = false
			b.WriteByte(c)
			continue
		case c != '\\' || i+1 >= len(pattern):
			b.WriteByte(c)
			continue
		}
		if v, n, ok := pygrepHighEscape(pattern[i+1:]); ok {
			b.WriteRune(pygrepByteRune(v))
			i += n
			continue
		}
		switch pattern[i+1] {
		case 's':
			b.WriteString(pygrepSpace(inClass, false))
		case 'S':
			b.WriteString(pygrepSpace(inClass, true))
		default:
			b.WriteString(pattern[i : i+2])
		}
		i++
	}
	return b.String()
}

// pygrepSpace spells a bytes pattern's \s or \S. Python's is the six ASCII
// whitespace bytes, \v among them; regexp2's RE2-mode \s leaves \v out. In
// a class, \S is written as the ranges of every other subject rune: the rest
// of ASCII, and the runes pygrepSubject gives the high bytes.
func pygrepSpace(inClass, negated bool) string {
	const space = `\t\n\v\f\r `
	switch {
	case !inClass && !negated:
		return "[" + space + "]"
	case !inClass:
		return "[^" + space + "]"
	case !negated:
		return space
	default:
		return `\x00-\x08\x0e-\x1f!-\x7f` + string(pygrepByteRune(0x80)) + "-" + string(pygrepByteRune(0xff))
	}
}

// pygrepHighEscape reads, after a backslash, an escape Python's re turns into
// a byte from 0x80 up: \xHH, or an octal \ooo (sre reads three octal digits
// as a character, not a group reference). It reports the byte and how many
// characters the escape takes after the backslash.
func pygrepHighEscape(s string) (byte, int, bool) {
	if len(s) >= 3 && s[0] == 'x' {
		if v, err := strconv.ParseUint(s[1:3], 16, 8); err == nil && v >= 0x80 {
			return byte(v), 3, true
		}
		return 0, 0, false
	}
	if len(s) >= 3 && (s[0] == '2' || s[0] == '3') && isOctal(s[1]) && isOctal(s[2]) {
		return (s[0]-'0')<<6 | (s[1]-'0')<<3 | (s[2] - '0'), 3, true
	}
	return 0, 0, false
}

func isOctal(c byte) bool { return c >= '0' && c <= '7' }

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
