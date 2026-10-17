// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package pyyaml

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Errors as PyYAML's CSafeLoader raises them. Upstream's yaml_load uses the
// libyaml-backed loader whenever PyYAML ships it (every wheel does), so its
// scanner and parser errors are libyaml's -- the same words yaml.v3, a port
// of libyaml, reports -- while composer and constructor errors come from
// PyYAML's _yaml.pyx and constructor.py. All print through
// MarkedYAMLError.__str__.

// Mark is a position as libyaml counts it: zero-based line and column, the
// column in characters.
type Mark struct {
	Line, Column int
}

// String is PyYAML's Mark.__str__ for a mark with no buffer, which is what
// the libyaml loader's marks are: no snippet, and the name pre-commit's
// yaml_load(str) gives the stream.
func (m Mark) String() string {
	return fmt.Sprintf("  in \"<unicode string>\", line %d, column %d", m.Line+1, m.Column+1)
}

// MarkedError is PyYAML's MarkedYAMLError.
type MarkedError struct {
	ContextMark *Mark
	ProblemMark *Mark
	Context     string
	Problem     string
}

// Error is MarkedYAMLError.__str__: the context mark is shown only when it
// says something the problem mark does not.
func (e *MarkedError) Error() string {
	var lines []string
	if e.Context != "" {
		lines = append(lines, e.Context)
	}
	if e.ContextMark != nil && (e.Problem == "" || e.ProblemMark == nil || *e.ContextMark != *e.ProblemMark) {
		lines = append(lines, e.ContextMark.String())
	}
	if e.Problem != "" {
		lines = append(lines, e.Problem)
	}
	if e.ProblemMark != nil {
		lines = append(lines, e.ProblemMark.String())
	}
	return strings.Join(lines, "\n")
}

func nodeMark(n *yaml.Node) *Mark {
	return &Mark{Line: n.Line - 1, Column: n.Column - 1}
}

// markAt is the mark of byte offset off in src, counting the line breaks
// libyaml counts.
func markAt(src string, off int) *Mark {
	m := &Mark{}
	for i, r := range src {
		if i >= off {
			break
		}
		switch r {
		case '\n', '\u0085', '\u2028', '\u2029':
			m.Line++
			m.Column = 0
		default:
			m.Column++
		}
	}
	return m
}

// streamEndMark is where libyaml puts STREAM-END: at the end, forced onto a
// fresh line when the last one has no break.
func streamEndMark(src string) *Mark {
	m := markAt(src, len(src))
	if m.Column != 0 {
		m.Line++
		m.Column = 0
	}
	return m
}

// syntaxError rewrites a yaml.v3 scanner or parser error as libyaml's
// ScannerError or ParserError, with the context and marks PyYAML prints.
// yaml.v3 keeps only the problem text; the rest is recovered by rescanning
// src for the cases below. Any other error keeps yaml.v3's wording.
func syntaxError(src string, err error) error {
	problem := strings.TrimPrefix(err.Error(), "yaml: ")
	if _, rest, ok := strings.Cut(problem, ": "); ok && strings.HasPrefix(problem, "line ") {
		problem = rest
	}
	s := scanText(src)
	end := streamEndMark(src)
	switch problem {
	case "found unexpected end of stream":
		if s.unclosedQuote >= 0 {
			return &MarkedError{
				Context: "while scanning a quoted scalar", ContextMark: markAt(src, s.unclosedQuote),
				Problem: problem, ProblemMark: markAt(src, len(src)),
			}
		}
	case "did not find expected node content":
		if len(s.flow) > 0 && s.atEnd {
			return &MarkedError{Context: "while parsing a flow node", ContextMark: end, Problem: problem, ProblemMark: end}
		}
	case "did not find expected ',' or ']'", "did not find expected ',' or '}'":
		if n := len(s.flow); n > 0 && (s.flowColon >= 0 || s.flowJunk >= 0) {
			at := s.flowColon
			if at < 0 {
				at = s.flowJunk
			}
			open := s.flow[n-1]
			ctx := "while parsing a flow sequence"
			if src[open] == '{' {
				ctx = "while parsing a flow mapping"
			}
			if (src[open] == '[') == strings.HasSuffix(problem, "']'") {
				return &MarkedError{Context: ctx, ContextMark: markAt(src, open), Problem: problem, ProblemMark: markAt(src, at)}
			}
		}
		if n := len(s.flow); n > 0 && s.atEnd {
			open := s.flow[n-1]
			ctx := "while parsing a flow sequence"
			if src[open] == '{' {
				ctx = "while parsing a flow mapping"
			}
			if (src[open] == '[') == strings.HasSuffix(problem, "']'") {
				return &MarkedError{Context: ctx, ContextMark: markAt(src, open), Problem: problem, ProblemMark: end}
			}
		}
	case "found unknown escape character", "did not find expected hexdecimal number":
		if s.badEscape >= 0 && s.badEscapeProblem == problem {
			return &MarkedError{
				Context: "while parsing a quoted scalar", ContextMark: markAt(src, s.badEscapeQuote),
				Problem: problem, ProblemMark: markAt(src, s.badEscape),
			}
		}
	case "found character that cannot start any token":
		if s.badStart >= 0 {
			m := markAt(src, s.badStart)
			return &MarkedError{Context: "while scanning for the next token", ContextMark: m, Problem: problem, ProblemMark: m}
		}
	case "did not find expected alphabetic or numeric character", "did not find expected comment or line break",
		"found an indentation indicator equal to 0",
		"found unknown directive name", "did not find the expected '>'":
		if s.tokenStart >= 0 && s.tokenProblem == problem {
			return &MarkedError{
				Context: s.tokenContext, ContextMark: markAt(src, s.tokenStart),
				Problem: problem, ProblemMark: markAt(src, s.tokenAt),
			}
		}
	case "did not find expected key":
		if e := blockCollectionError(src, problem, yaml.MappingNode); e != nil {
			return e
		}
	case "did not find expected '-' indicator":
		if e := blockCollectionError(src, problem, yaml.SequenceNode); e != nil {
			return e
		}
	case "found a tab character that violates indentation":
		if s.badStart >= 0 && s.lastPlain >= 0 {
			return &MarkedError{
				Context: "while scanning a plain scalar", ContextMark: markAt(src, s.lastPlain),
				Problem: problem, ProblemMark: markAt(src, s.badStart),
			}
		}
	case "could not find expected ':'":
		if s.staleKey >= 0 {
			at := streamEndMark(src)
			if s.staleAt < len(src) {
				at = markAt(src, s.staleAt)
			}
			return &MarkedError{
				Context: "while scanning a simple key", ContextMark: markAt(src, s.staleKey),
				Problem: problem, ProblemMark: at,
			}
		}
	case "did not find expected <document start>":
		if off := afterDocumentEnd(src); off >= 0 {
			return &MarkedError{Problem: problem, ProblemMark: markAt(src, off)}
		}
	case "found incompatible YAML document":
		if i := strings.Index(src, "%YAML"); i >= 0 && lineStartOffset(src, i) == i {
			return &MarkedError{Problem: problem, ProblemMark: markAt(src, i)}
		}
	case "mapping values are not allowed in this context":
		if s.secondValue >= 0 {
			return &MarkedError{Problem: problem, ProblemMark: markAt(src, s.secondValue)}
		}
	}
	return err
}

// textScan is what a light rescan of a document that failed to parse
// finds: the first of each kind of fault, by byte offset (-1 for none).
type textScan struct {
	badEscapeProblem string
	tokenProblem     string
	tokenContext     string
	flow             []int
	flowJunk         int
	badEscapeQuote   int
	lastPlain        int
	flowColon        int
	badEscape        int
	staleKey         int
	staleAt          int
	badStart         int
	secondValue      int
	unclosedQuote    int
	tokenStart       int
	tokenAt          int
	atEnd            bool
}

const (
	flowIndicators = ",[]{}"
	validEscapes   = "0abt\tnvfre \"/\\N_LPxuU"
)

func isBlank(r rune) bool { return r == ' ' || r == '\t' }

func isBreakRune(r rune) bool { return r == '\n' || r == '\u0085' || r == '\u2028' || r == '\u2029' }

// scanText walks src as libyaml's scanner would far enough to place the
// faults syntaxError reports. It stops at the first of them.
func scanText(src string) textScan {
	s := textScan{
		unclosedQuote:  -1,
		badEscape:      -1,
		badEscapeQuote: -1,
		badStart:       -1,
		secondValue:    -1,
		tokenStart:     -1,
		lastPlain:      -1,
		flowColon:      -1,
		flowJunk:       -1,
		staleKey:       -1,
	}
	tokenStart := true // at a place a token may begin
	lineStart := true  // nothing but indentation so far on this line
	valueSeen := false // a block ':' has been seen on this line
	inPlainValue := false
	keyIndent := 0              // column of the key whose value is being read
	keyStart := 0               // offset of the last token begun
	inFlowPlain := false        // inside a plain scalar in a flow collection
	flowPlainMultiline := false // ... which has crossed a line break
	flowEntryDone := false      // a flow entry ended; ',' or a close must follow
	lineKey, lineColon := -1, false
	var keyCols []int // columns of the block mappings' keys still open
	for i := 0; i < len(src); {
		r, w := utf8.DecodeRuneInString(src[i:])
		switch {
		case isBreakRune(r):
			// A line whose only token is a scalar at a key's column is a
			// simple key that never found its ':'.
			if lineKey >= 0 && !lineColon && s.staleKey < 0 {
				s.staleKey, s.staleAt = lineKey, nextTokenOffset(src, i+w)
			}
			lineKey, lineColon = -1, false
			// A plain value continues onto a line indented past the line
			// its key is on; anything else ends it.
			cont := inPlainValue && len(s.flow) == 0 && lineIndent(src, i+w) > keyIndent
			tokenStart, lineStart = true, true
			if !cont {
				valueSeen, inPlainValue = false, false
			}
			if inFlowPlain {
				flowPlainMultiline = true
			}
			i += w
			if cont {
				// Skip the indentation, so the continuation is not a token.
				for i < len(src) && src[i] == ' ' {
					i++
				}
				tokenStart, lineStart = false, false
			}
			continue
		case isBlank(r):
			if r == '\t' && lineStart && len(s.flow) == 0 {
				// A tab in block indentation, followed by content.
				j := i
				for j < len(src) && isBlank(rune(src[j])) {
					j++
				}
				if j < len(src) && !isBreakRune(rune(src[j])) && src[j] != '#' {
					s.badStart = i
					return s
				}
			}
			tokenStart = true
			i += w
			continue
		case r == '#' && tokenStart:
			for i < len(src) {
				r, w = utf8.DecodeRuneInString(src[i:])
				if isBreakRune(r) {
					break
				}
				i += w
			}
			continue
		}
		wasLineStart := lineStart
		lineStart = false
		if len(s.flow) > 0 && flowEntryDone && !strings.ContainsRune(",:]}", r) {
			s.flowJunk = i
			return s
		}
		if wasLineStart && len(s.flow) == 0 {
			col := i - lineStartOffset(src, i)
			for len(keyCols) > 0 && keyCols[len(keyCols)-1] > col {
				keyCols = keyCols[:len(keyCols)-1]
			}
			if (r == '\'' || r == '"' || !strings.ContainsRune("-?:[{&*!|>%#", r)) &&
				len(keyCols) > 0 && keyCols[len(keyCols)-1] == col {
				lineKey = i
			}
		}
		if !tokenStart && (len(s.flow) == 0 || !strings.ContainsRune(flowIndicators, r)) && r != ':' {
			i += w
			continue
		}
		switch r {
		case '\'', '"':
			keyStart = i
			j, closed := scanQuoted(src, i, &s)
			if s.badEscape >= 0 {
				return s
			}
			if !closed {
				s.unclosedQuote = i
				return s
			}
			i = j
			tokenStart = false
			inPlainValue = false
			flowEntryDone = len(s.flow) > 0
			continue
		case '[', '{':
			s.flow = append(s.flow, i)
			tokenStart, inFlowPlain, flowEntryDone = true, false, false
		case ']', '}':
			if n := len(s.flow); n > 0 {
				if (src[s.flow[n-1]] == '[') != (r == ']') {
					// The wrong close: the open collection still wants
					// its own.
					s.flowJunk = i
					return s
				}
				s.flow = s.flow[:n-1]
			}
			tokenStart, inFlowPlain = false, false
			flowEntryDone = len(s.flow) > 0
		case ',':
			tokenStart, inFlowPlain, flowEntryDone = true, false, false
		case ':':
			next := rune(0)
			if i+1 < len(src) {
				next, _ = utf8.DecodeRuneInString(src[i+1:])
			}
			if next == 0 || isBlank(next) || isBreakRune(next) ||
				(len(s.flow) > 0 && strings.ContainsRune(flowIndicators, next)) {
				lineColon = true
				flowEntryDone = false
				if len(s.flow) == 0 {
					if valueSeen && inPlainValue {
						s.secondValue = i
						return s
					}
					valueSeen = true
					keyIndent = keyStart - lineStartOffset(src, keyStart)
					for len(keyCols) > 0 && keyCols[len(keyCols)-1] >= keyIndent {
						keyCols = keyCols[:len(keyCols)-1]
					}
					keyCols = append(keyCols, keyIndent)
				} else if inFlowPlain && flowPlainMultiline && s.flowColon < 0 {
					// A key in a flow collection must be on one line.
					s.flowColon = i
					return s
				}
				inFlowPlain, flowPlainMultiline = false, false
				inPlainValue = false
				tokenStart = true
			}
		case '@', '`':
			s.badStart = i
			return s
		case '&', '*':
			j := i + 1
			for j < len(src) && isAnchorChar(src[j]) {
				j++
			}
			if j == i+1 || (j < len(src) && !strings.ContainsRune(" \t\n?:,]}%@`", rune(src[j]))) {
				s.tokenContext = "while scanning an anchor"
				if r == '*' {
					s.tokenContext = "while scanning an alias"
				}
				s.tokenProblem = "did not find expected alphabetic or numeric character"
				s.tokenStart, s.tokenAt = i, j
				return s
			}
			i = j
			tokenStart = false
			continue
		case '|', '>':
			if len(s.flow) == 0 {
				j := i + 1
				for k := 0; k < 2 && j < len(src) && (src[j] == '+' || src[j] == '-' || (src[j] >= '0' && src[j] <= '9')); k++ {
					if src[j] == '0' {
						s.tokenContext, s.tokenProblem = "while scanning a block scalar", "found an indentation indicator equal to 0"
						s.tokenStart, s.tokenAt = i, j
						return s
					}
					j++
				}
				for j < len(src) && isBlank(rune(src[j])) {
					j++
				}
				if j < len(src) && src[j] != '#' && src[j] != '\n' {
					s.tokenContext, s.tokenProblem = "while scanning a block scalar", "did not find expected comment or line break"
					s.tokenStart, s.tokenAt = i, j
					return s
				}
				// Skip the scalar's content: the lines indented past the
				// first token of the header's line.
				parent := lineIndent(src, lineStartOffset(src, i))
				if valueSeen {
					parent = keyStart - lineStartOffset(src, keyStart)
				}
				for j < len(src) && src[j] != '\n' {
					j++
				}
				for j < len(src) {
					next := j + 1
					end := strings.IndexByte(src[next:], '\n')
					line := src[next:]
					if end >= 0 {
						line = src[next : next+end]
					}
					if strings.TrimSpace(line) != "" && lineIndent(src, next) <= parent {
						break
					}
					if end < 0 {
						j = len(src)
						break
					}
					j = next + end
				}
				i = j
				valueSeen, inPlainValue = false, false
				continue
			}
		case '%':
			if lineStartOffset(src, i) == i {
				j := i + 1
				for j < len(src) && isAnchorChar(src[j]) {
					j++
				}
				if name := src[i+1 : j]; name != "YAML" && name != "TAG" && j > i+1 {
					s.tokenContext, s.tokenProblem = "while scanning a directive", "found unknown directive name"
					s.tokenStart, s.tokenAt = i, j
					return s
				}
			}
		case '!':
			if i+1 < len(src) && src[i+1] == '<' {
				j := i + 2
				for j < len(src) && isURIChar(src[j]) {
					j++
				}
				if j >= len(src) || src[j] != '>' {
					s.tokenContext, s.tokenProblem = "while scanning a tag", "did not find the expected '>'"
					s.tokenStart, s.tokenAt = i, j
					return s
				}
			}
		case '-', '?':
			next := rune(0)
			if i+1 < len(src) {
				next, _ = utf8.DecodeRuneInString(src[i+1:])
			}
			if next == 0 || isBlank(next) || isBreakRune(next) {
				tokenStart = true
				break
			}
			fallthrough
		default:
			if tokenStart && r != '!' && r != '|' && r != '>' {
				s.lastPlain = i
				if len(s.flow) > 0 && !inFlowPlain {
					inFlowPlain, flowPlainMultiline = true, false
				}
				keyStart = i
				if valueSeen && len(s.flow) == 0 {
					inPlainValue = true
				}
			}
			tokenStart = false
		}
		i += w
	}
	s.atEnd = true
	return s
}

// nextTokenOffset is where the scanner next stops from off: past blank
// lines, indentation and comments; len(src) at the end.
func nextTokenOffset(src string, off int) int {
	for off < len(src) {
		switch c := src[off]; c {
		case ' ', '\t', '\n':
			off++
		case '#':
			for off < len(src) && src[off] != '\n' {
				off++
			}
		default:
			return off
		}
	}
	return off
}

// lineIndent is the number of spaces the line starting at off begins with.
func lineIndent(src string, off int) int {
	n := 0
	for off+n < len(src) && src[off+n] == ' ' {
		n++
	}
	return n
}

// afterDocumentEnd is the offset of the first content after a "..."
// document end marker, or -1.
func afterDocumentEnd(src string) int {
	seenEnd := false
	off := 0
	for _, line := range strings.SplitAfter(src, "\n") {
		text := strings.TrimRight(line, "\n")
		switch {
		case text == "..." || strings.HasPrefix(text, "... "):
			seenEnd = true
		case seenEnd && strings.TrimSpace(text) != "" && !strings.HasPrefix(strings.TrimSpace(text), "#"):
			if strings.HasPrefix(text, "---") || strings.HasPrefix(text, "%") {
				seenEnd = false
				break
			}
			return off + len(text) - len(strings.TrimLeft(text, " "))
		}
		off += len(line)
	}
	return -1
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// isAnchorChar is libyaml's IS_ALPHA: what an anchor or directive name is
// made of.
func isAnchorChar(c byte) bool {
	return c == '_' || c == '-' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isURIChar is the set libyaml's yaml_parser_scan_tag_uri accepts.
func isURIChar(c byte) bool {
	return isAnchorChar(c) || strings.IndexByte(";/?:@&=+$,.!~*'()[]%", c) >= 0
}

func lineStartOffset(src string, i int) int {
	return strings.LastIndexByte(src[:i], '\n') + 1
}

// scanQuoted reads the quoted scalar opening at i, returning the offset
// after its closing quote, or closed false if the text ends first. An
// unknown escape in a double-quoted scalar is recorded in s.
func scanQuoted(src string, i int, s *textScan) (int, bool) {
	q := src[i]
	j := i + 1
	for j < len(src) {
		c := src[j]
		switch {
		case q == '\'' && c == '\'':
			if j+1 < len(src) && src[j+1] == '\'' {
				j += 2
				continue
			}
			return j + 1, true
		case q == '"' && c == '\\':
			if j+1 < len(src) {
				n, _ := utf8.DecodeRuneInString(src[j+1:])
				if !strings.ContainsRune(validEscapes, n) && !isBreakRune(n) && n != '\r' {
					s.badEscape, s.badEscapeQuote, s.badEscapeProblem = j, i, "found unknown escape character"
					return j, false
				}
				if width := map[rune]int{'x': 2, 'u': 4, 'U': 8}[n]; width > 0 {
					for k := j + 2; k < j+2+width; k++ {
						if k >= len(src) || !isHex(src[k]) {
							// libyaml marks where the digits begin.
							s.badEscape, s.badEscapeQuote, s.badEscapeProblem = j+2, i, "did not find expected hexdecimal number"
							return j, false
						}
					}
					j += 2 + width
					continue
				}
			}
			j += 2
			continue
		case q == '"' && c == '"':
			return j + 1, true
		}
		j++
	}
	return j, false
}

// blockCollectionError places libyaml's "did not find expected key" (in a
// block mapping) or "did not find expected '-' indicator" (in a block
// sequence): a token where the collection needs its next entry. The problem
// line is the first whose addition makes the document fail that way; the
// collection is the innermost block one of that kind open at that point
// that the token is not left of.
func blockCollectionError(src, problem string, kind yaml.Kind) *MarkedError {
	lines := strings.SplitAfter(src, "\n")
	p := -1
	for k := 1; k <= len(lines); k++ {
		err := yaml.NewDecoder(strings.NewReader(strings.Join(lines[:k], ""))).Decode(new(yaml.Node))
		if err == nil || errors.Is(err, io.EOF) {
			continue
		}
		if !strings.HasSuffix(err.Error(), problem) {
			return nil
		}
		p = k - 1
		break
	}
	if p < 0 {
		return nil
	}
	line := strings.TrimRight(lines[p], "\n")
	first := len(line) - len(strings.TrimLeft(line, " "))
	col := utf8.RuneCountInString(line[:first])
	at := first
	if junk := afterClosedValue(line[first:]); junk >= 0 {
		at = first + junk
	}

	var inner *yaml.Node
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines[:p], "")), &doc); err == nil && len(doc.Content) > 0 {
		for n := doc.Content[0]; n != nil; {
			if n.Kind == kind && n.Style&yaml.FlowStyle == 0 && n.Column-1 <= col {
				inner = n
			}
			if len(n.Content) == 0 {
				break
			}
			n = n.Content[len(n.Content)-1]
		}
	}
	var collection *Mark
	switch {
	case at != first && kind == yaml.MappingNode && (inner == nil || inner.Column-1 != col):
		// The token follows a complete value on a line whose first key
		// starts the mapping.
		collection = &Mark{Line: p, Column: col}
	case inner != nil:
		collection = nodeMark(inner)
	default:
		return nil
	}
	ctx := "while parsing a block mapping"
	if kind == yaml.SequenceNode {
		ctx = "while parsing a block collection"
	}
	return &MarkedError{
		Context: ctx, ContextMark: collection,
		Problem: problem, ProblemMark: &Mark{Line: p, Column: utf8.RuneCountInString(line[:at])},
	}
}

// afterClosedValue is the offset in a line of the first token that follows
// a quoted scalar or flow collection on it, or -1.
func afterClosedValue(line string) int {
	var s textScan
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c != '\'' && c != '"' && c != '[' && c != '{' {
			continue
		}
		if i > 0 && !strings.ContainsRune(" \t", rune(line[i-1])) {
			continue
		}
		var end int
		if c == '\'' || c == '"' {
			var closed bool
			end, closed = scanQuoted(line, i, &s)
			if !closed {
				return -1
			}
		} else {
			depth := 0
			for end = i; end < len(line); end++ {
				switch line[end] {
				case '[', '{':
					depth++
				case ']', '}':
					depth--
				}
				if depth == 0 {
					end++
					break
				}
			}
		}
		j := end
		for j < len(line) && isBlank(rune(line[j])) {
			j++
		}
		if j < len(line) && line[j] != '#' && line[j] != ':' {
			return j
		}
		return -1
	}
	return -1
}
