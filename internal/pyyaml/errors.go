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
// src for the problems in syntaxFaults. Any other error keeps yaml.v3's
// wording.
func syntaxError(src string, err error) error {
	problem := strings.TrimPrefix(err.Error(), "yaml: ")
	if _, rest, ok := strings.Cut(problem, ": "); ok && strings.HasPrefix(problem, "line ") {
		problem = rest
	}
	s := scanText(src)
	if place, ok := syntaxFaults[problem]; ok {
		if e := place(src, problem, &s); e != nil {
			return e
		}
	}
	return err
}

// syntaxFaults places each problem syntaxError knows: given the text, the
// problem and the rescan of the text, the error PyYAML raises, or nil when
// the rescan did not find the fault yaml.v3 reported.
var syntaxFaults = map[string]func(src, problem string, s *textScan) *MarkedError{
	"found unexpected end of stream":                        unclosedQuoteFault,
	"did not find expected node content":                    flowNodeFault,
	"did not find expected ',' or ']'":                      flowEndFault,
	"did not find expected ',' or '}'":                      flowEndFault,
	"found unknown escape character":                        escapeFault,
	"did not find expected hexdecimal number":               escapeFault,
	"found character that cannot start any token":           badStartFault,
	"did not find expected alphabetic or numeric character": tokenFault,
	"did not find expected comment or line break":           tokenFault,
	"found an indentation indicator equal to 0":             tokenFault,
	"found unknown directive name":                          tokenFault,
	"did not find the expected '>'":                         tokenFault,
	"did not find expected key": func(src, problem string, _ *textScan) *MarkedError {
		return blockCollectionError(src, problem, yaml.MappingNode)
	},
	"did not find expected '-' indicator": func(src, problem string, _ *textScan) *MarkedError {
		return blockCollectionError(src, problem, yaml.SequenceNode)
	},
	"found a tab character that violates indentation": tabFault,
	"could not find expected ':'":                     simpleKeyFault,
	"did not find expected <document start>":          documentStartFault,
	"found incompatible YAML document":                incompatibleFault,
	"mapping values are not allowed in this context":  secondValueFault,
}

func unclosedQuoteFault(src, problem string, s *textScan) *MarkedError {
	if s.unclosedQuote < 0 {
		return nil
	}
	return &MarkedError{
		Context: "while scanning a quoted scalar", ContextMark: markAt(src, s.unclosedQuote),
		Problem: problem, ProblemMark: markAt(src, len(src)),
	}
}

func flowNodeFault(src, problem string, s *textScan) *MarkedError {
	if len(s.flow) == 0 || !s.atEnd {
		return nil
	}
	end := streamEndMark(src)
	return &MarkedError{Context: "while parsing a flow node", ContextMark: end, Problem: problem, ProblemMark: end}
}

// flowEndFault places a flow collection that problem says was not closed
// by the close it wants: at the token that cannot continue it, or at the
// end of the stream.
func flowEndFault(src, problem string, s *textScan) *MarkedError {
	n := len(s.flow)
	if n == 0 {
		return nil
	}
	open := s.flow[n-1]
	if (src[open] == '[') != strings.HasSuffix(problem, "']'") {
		return nil
	}
	var at *Mark
	switch {
	case s.flowColon >= 0:
		at = markAt(src, s.flowColon)
	case s.flowJunk >= 0:
		at = markAt(src, s.flowJunk)
	case s.atEnd:
		at = streamEndMark(src)
	default:
		return nil
	}
	ctx := "while parsing a flow sequence"
	if src[open] == '{' {
		ctx = "while parsing a flow mapping"
	}
	return &MarkedError{Context: ctx, ContextMark: markAt(src, open), Problem: problem, ProblemMark: at}
}

func escapeFault(src, problem string, s *textScan) *MarkedError {
	if s.badEscape < 0 || s.badEscapeProblem != problem {
		return nil
	}
	return &MarkedError{
		Context: "while parsing a quoted scalar", ContextMark: markAt(src, s.badEscapeQuote),
		Problem: problem, ProblemMark: markAt(src, s.badEscape),
	}
}

func badStartFault(src, problem string, s *textScan) *MarkedError {
	if s.badStart < 0 {
		return nil
	}
	m := markAt(src, s.badStart)
	return &MarkedError{Context: "while scanning for the next token", ContextMark: m, Problem: problem, ProblemMark: m}
}

func tokenFault(src, problem string, s *textScan) *MarkedError {
	if s.tokenStart < 0 || s.tokenProblem != problem {
		return nil
	}
	return &MarkedError{
		Context: s.tokenContext, ContextMark: markAt(src, s.tokenStart),
		Problem: problem, ProblemMark: markAt(src, s.tokenAt),
	}
}

func tabFault(src, problem string, s *textScan) *MarkedError {
	if s.badStart < 0 || s.lastPlain < 0 {
		return nil
	}
	return &MarkedError{
		Context: "while scanning a plain scalar", ContextMark: markAt(src, s.lastPlain),
		Problem: problem, ProblemMark: markAt(src, s.badStart),
	}
}

func simpleKeyFault(src, problem string, s *textScan) *MarkedError {
	if s.staleKey < 0 {
		return nil
	}
	at := streamEndMark(src)
	if s.staleAt < len(src) {
		at = markAt(src, s.staleAt)
	}
	return &MarkedError{
		Context: "while scanning a simple key", ContextMark: markAt(src, s.staleKey),
		Problem: problem, ProblemMark: at,
	}
}

func documentStartFault(src, problem string, _ *textScan) *MarkedError {
	off := afterDocumentEnd(src)
	if off < 0 {
		return nil
	}
	return &MarkedError{Problem: problem, ProblemMark: markAt(src, off)}
}

func incompatibleFault(src, problem string, _ *textScan) *MarkedError {
	i := strings.Index(src, "%YAML")
	if i < 0 || lineStartOffset(src, i) != i {
		return nil
	}
	return &MarkedError{Problem: problem, ProblemMark: markAt(src, i)}
}

func secondValueFault(src, problem string, s *textScan) *MarkedError {
	if s.secondValue < 0 {
		return nil
	}
	return &MarkedError{Problem: problem, ProblemMark: markAt(src, s.secondValue)}
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
	sc := textScanner{
		src: src,
		s: textScan{
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
		},
		lineKey:    -1,
		tokenStart: true,
		lineStart:  true,
	}
	for sc.i < len(src) {
		if sc.step() {
			return sc.s
		}
	}
	sc.s.atEnd = true
	return sc.s
}

// textScanner is scanText's place in src and what it has seen so far.
type textScanner struct {
	src                string
	keyCols            []int // columns of the block mappings' keys still open
	s                  textScan
	i                  int  // offset of the next character
	keyIndent          int  // column of the key whose value is being read
	keyStart           int  // offset of the last token begun
	lineKey            int  // a scalar starting this line at a key's column, or -1
	tokenStart         bool // at a place a token may begin
	lineStart          bool // nothing but indentation so far on this line
	valueSeen          bool // a block ':' has been seen on this line
	inPlainValue       bool
	inFlowPlain        bool // inside a plain scalar in a flow collection
	flowPlainMultiline bool // ... which has crossed a line break
	flowEntryDone      bool // a flow entry ended; ',' or a close must follow
	lineColon          bool // the line has a ':' indicator
}

// step scans the character at sc.i and whatever token it begins, moving
// sc.i past them. It reports whether it found a fault, which ends the scan.
func (sc *textScanner) step() bool {
	r, w := utf8.DecodeRuneInString(sc.src[sc.i:])
	switch {
	case isBreakRune(r):
		sc.lineBreak(w)
		return false
	case isBlank(r):
		return sc.blank(r, w)
	case r == '#' && sc.tokenStart:
		sc.skipComment()
		return false
	}
	return sc.token(r, w)
}

func (sc *textScanner) lineBreak(w int) {
	// A line whose only token is a scalar at a key's column is a
	// simple key that never found its ':'.
	if sc.lineKey >= 0 && !sc.lineColon && sc.s.staleKey < 0 {
		sc.s.staleKey, sc.s.staleAt = sc.lineKey, nextTokenOffset(sc.src, sc.i+w)
	}
	sc.lineKey, sc.lineColon = -1, false
	// A plain value continues onto a line indented past the line
	// its key is on; anything else ends it.
	cont := sc.inPlainValue && len(sc.s.flow) == 0 && lineIndent(sc.src, sc.i+w) > sc.keyIndent
	sc.tokenStart, sc.lineStart = true, true
	if !cont {
		sc.valueSeen, sc.inPlainValue = false, false
	}
	if sc.inFlowPlain {
		sc.flowPlainMultiline = true
	}
	sc.i += w
	if cont {
		// Skip the indentation, so the continuation is not a token.
		for sc.i < len(sc.src) && sc.src[sc.i] == ' ' {
			sc.i++
		}
		sc.tokenStart, sc.lineStart = false, false
	}
}

func (sc *textScanner) blank(r rune, w int) bool {
	if r == '\t' && sc.lineStart && len(sc.s.flow) == 0 {
		// A tab in block indentation, followed by content.
		j := sc.i
		for j < len(sc.src) && isBlank(rune(sc.src[j])) {
			j++
		}
		if j < len(sc.src) && !isBreakRune(rune(sc.src[j])) && sc.src[j] != '#' {
			sc.s.badStart = sc.i
			return true
		}
	}
	sc.tokenStart = true
	sc.i += w
	return false
}

func (sc *textScanner) skipComment() {
	for sc.i < len(sc.src) {
		r, w := utf8.DecodeRuneInString(sc.src[sc.i:])
		if isBreakRune(r) {
			return
		}
		sc.i += w
	}
}

// token is a character that is neither blank, a break nor a comment's.
func (sc *textScanner) token(r rune, w int) bool {
	wasLineStart := sc.lineStart
	sc.lineStart = false
	if len(sc.s.flow) > 0 && sc.flowEntryDone && !strings.ContainsRune(",:]}", r) {
		sc.s.flowJunk = sc.i
		return true
	}
	if wasLineStart && len(sc.s.flow) == 0 {
		sc.lineKeyAt(r)
	}
	if !sc.tokenStart && (len(sc.s.flow) == 0 || !strings.ContainsRune(flowIndicators, r)) && r != ':' {
		sc.i += w
		return false
	}
	return sc.indicator(r, w)
}

// lineKeyAt closes the block mappings whose keys are right of the first
// token on a line, r at sc.i, and notes that token when it is a scalar at
// the column of one still open.
func (sc *textScanner) lineKeyAt(r rune) {
	col := sc.i - lineStartOffset(sc.src, sc.i)
	for len(sc.keyCols) > 0 && sc.keyCols[len(sc.keyCols)-1] > col {
		sc.keyCols = sc.keyCols[:len(sc.keyCols)-1]
	}
	if (r == '\'' || r == '"' || !strings.ContainsRune("-?:[{&*!|>%#", r)) &&
		len(sc.keyCols) > 0 && sc.keyCols[len(sc.keyCols)-1] == col {
		sc.lineKey = sc.i
	}
}

// indicator scans what r, at a place a token may begin or in a flow
// collection, begins.
func (sc *textScanner) indicator(r rune, w int) bool {
	switch r {
	case '\'', '"':
		return sc.quoted()
	case '[', '{':
		sc.s.flow = append(sc.s.flow, sc.i)
		sc.tokenStart, sc.inFlowPlain, sc.flowEntryDone = true, false, false
	case ']', '}':
		return sc.closeFlow(r, w)
	case ',':
		sc.tokenStart, sc.inFlowPlain, sc.flowEntryDone = true, false, false
	case ':':
		return sc.colon(w)
	case '@', '`':
		sc.s.badStart = sc.i
		return true
	case '&', '*':
		return sc.anchor(r)
	case '|', '>':
		return sc.blockScalar(w)
	case '%':
		return sc.directive(w)
	case '!':
		return sc.tag(w)
	case '-', '?':
		if endsIndicator(sc.nextRune()) {
			sc.tokenStart = true
		} else {
			sc.plain(r)
		}
	default:
		sc.plain(r)
	}
	sc.i += w
	return false
}

// nextRune is the character after the one-byte one at sc.i, or 0 at the
// end of src.
func (sc *textScanner) nextRune() rune {
	if sc.i+1 >= len(sc.src) {
		return 0
	}
	next, _ := utf8.DecodeRuneInString(sc.src[sc.i+1:])
	return next
}

// endsIndicator reports whether next, from nextRune, makes the character
// before it an indicator rather than part of a plain scalar.
func endsIndicator(next rune) bool {
	return next == 0 || isBlank(next) || isBreakRune(next)
}

func (sc *textScanner) quoted() bool {
	sc.keyStart = sc.i
	j, closed := scanQuoted(sc.src, sc.i, &sc.s)
	if sc.s.badEscape >= 0 {
		return true
	}
	if !closed {
		sc.s.unclosedQuote = sc.i
		return true
	}
	sc.i = j
	sc.tokenStart = false
	sc.inPlainValue = false
	sc.flowEntryDone = len(sc.s.flow) > 0
	return false
}

func (sc *textScanner) closeFlow(r rune, w int) bool {
	if n := len(sc.s.flow); n > 0 {
		if (sc.src[sc.s.flow[n-1]] == '[') != (r == ']') {
			// The wrong close: the open collection still wants
			// its own.
			sc.s.flowJunk = sc.i
			return true
		}
		sc.s.flow = sc.s.flow[:n-1]
	}
	sc.tokenStart, sc.inFlowPlain = false, false
	sc.flowEntryDone = len(sc.s.flow) > 0
	sc.i += w
	return false
}

func (sc *textScanner) colon(w int) bool {
	next := sc.nextRune()
	if !endsIndicator(next) && (len(sc.s.flow) == 0 || !strings.ContainsRune(flowIndicators, next)) {
		// Part of a plain scalar.
		sc.i += w
		return false
	}
	sc.lineColon = true
	sc.flowEntryDone = false
	if len(sc.s.flow) == 0 {
		if sc.valueSeen && sc.inPlainValue {
			sc.s.secondValue = sc.i
			return true
		}
		sc.blockKey()
	} else if sc.inFlowPlain && sc.flowPlainMultiline && sc.s.flowColon < 0 {
		// A key in a flow collection must be on one line.
		sc.s.flowColon = sc.i
		return true
	}
	sc.inFlowPlain, sc.flowPlainMultiline = false, false
	sc.inPlainValue = false
	sc.tokenStart = true
	sc.i += w
	return false
}

// blockKey notes the key a block ':' ends: its value is being read, and its
// column is a mapping's, closing those at or right of it.
func (sc *textScanner) blockKey() {
	sc.valueSeen = true
	sc.keyIndent = sc.keyStart - lineStartOffset(sc.src, sc.keyStart)
	for len(sc.keyCols) > 0 && sc.keyCols[len(sc.keyCols)-1] >= sc.keyIndent {
		sc.keyCols = sc.keyCols[:len(sc.keyCols)-1]
	}
	sc.keyCols = append(sc.keyCols, sc.keyIndent)
}

// tokenFault records a fault, found at offset at, in the token beginning
// at sc.i.
func (sc *textScanner) tokenFault(context, problem string, at int) {
	sc.s.tokenContext, sc.s.tokenProblem = context, problem
	sc.s.tokenStart, sc.s.tokenAt = sc.i, at
}

func (sc *textScanner) anchor(r rune) bool {
	j := sc.i + 1
	for j < len(sc.src) && isAnchorChar(sc.src[j]) {
		j++
	}
	if j == sc.i+1 || (j < len(sc.src) && !strings.ContainsRune(" \t\n?:,]}%@`", rune(sc.src[j]))) {
		context := "while scanning an anchor"
		if r == '*' {
			context = "while scanning an alias"
		}
		sc.tokenFault(context, "did not find expected alphabetic or numeric character", j)
		return true
	}
	sc.i = j
	sc.tokenStart = false
	return false
}

// blockScalar checks a block scalar's header and skips its content. In a
// flow collection '|' and '>' begin nothing.
func (sc *textScanner) blockScalar(w int) bool {
	if len(sc.s.flow) > 0 {
		sc.i += w
		return false
	}
	j, ok := sc.blockScalarHeader()
	if !ok {
		return true
	}
	sc.i = sc.skipBlockScalar(j)
	sc.valueSeen, sc.inPlainValue = false, false
	return false
}

// blockScalarHeader reads the indicators and comment after the '|' or '>'
// at sc.i, returning the offset of the end of their line; ok is false,
// with the fault recorded, for a header libyaml rejects.
func (sc *textScanner) blockScalarHeader() (int, bool) {
	src := sc.src
	j := sc.i + 1
	for k := 0; k < 2 && j < len(src) && (src[j] == '+' || src[j] == '-' || (src[j] >= '0' && src[j] <= '9')); k++ {
		if src[j] == '0' {
			sc.tokenFault("while scanning a block scalar", "found an indentation indicator equal to 0", j)
			return 0, false
		}
		j++
	}
	for j < len(src) && isBlank(rune(src[j])) {
		j++
	}
	if j < len(src) && src[j] != '#' && src[j] != '\n' {
		sc.tokenFault("while scanning a block scalar", "did not find expected comment or line break", j)
		return 0, false
	}
	for j < len(src) && src[j] != '\n' {
		j++
	}
	return j, true
}

// skipBlockScalar is the offset after the content of the block scalar whose
// header's line ends at j: the lines indented past the first token of the
// header's line.
func (sc *textScanner) skipBlockScalar(j int) int {
	src := sc.src
	parent := lineIndent(src, lineStartOffset(src, sc.i))
	if sc.valueSeen {
		parent = sc.keyStart - lineStartOffset(src, sc.keyStart)
	}
	for j < len(src) {
		next := j + 1
		end := strings.IndexByte(src[next:], '\n')
		line := src[next:]
		if end >= 0 {
			line = src[next : next+end]
		}
		if strings.TrimSpace(line) != "" && lineIndent(src, next) <= parent {
			return j
		}
		if end < 0 {
			return len(src)
		}
		j = next + end
	}
	return j
}

func (sc *textScanner) directive(w int) bool {
	i := sc.i
	if lineStartOffset(sc.src, i) == i {
		j := i + 1
		for j < len(sc.src) && isAnchorChar(sc.src[j]) {
			j++
		}
		if name := sc.src[i+1 : j]; name != "YAML" && name != "TAG" && j > i+1 {
			sc.tokenFault("while scanning a directive", "found unknown directive name", j)
			return true
		}
	}
	sc.i += w
	return false
}

func (sc *textScanner) tag(w int) bool {
	i := sc.i
	if i+1 < len(sc.src) && sc.src[i+1] == '<' {
		j := i + 2
		for j < len(sc.src) && isURIChar(sc.src[j]) {
			j++
		}
		if j >= len(sc.src) || sc.src[j] != '>' {
			sc.tokenFault("while scanning a tag", "did not find the expected '>'", j)
			return true
		}
	}
	sc.i += w
	return false
}

// plain is a character of a plain scalar, maybe its first.
func (sc *textScanner) plain(r rune) {
	if sc.tokenStart && r != '!' && r != '|' && r != '>' {
		sc.s.lastPlain = sc.i
		if len(sc.s.flow) > 0 && !sc.inFlowPlain {
			sc.inFlowPlain, sc.flowPlainMultiline = true, false
		}
		sc.keyStart = sc.i
		if sc.valueSeen && len(sc.s.flow) == 0 {
			sc.inPlainValue = true
		}
	}
	sc.tokenStart = false
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
			next, ok := scanEscape(src, i, j, s)
			if !ok {
				return j, false
			}
			j = next
			continue
		case q == '"' && c == '"':
			return j + 1, true
		}
		j++
	}
	return j, false
}

// hexEscapeWidth is how many hex digits each numeric escape takes.
var hexEscapeWidth = map[rune]int{'x': 2, 'u': 4, 'U': 8}

// scanEscape reads the escape at j in the double-quoted scalar opening at
// quote, returning the offset after it; ok is false, with the fault
// recorded in s, for an escape libyaml rejects.
func scanEscape(src string, quote, j int, s *textScan) (int, bool) {
	if j+1 >= len(src) {
		return j + 2, true
	}
	n, _ := utf8.DecodeRuneInString(src[j+1:])
	if !strings.ContainsRune(validEscapes, n) && !isBreakRune(n) && n != '\r' {
		s.badEscape, s.badEscapeQuote, s.badEscapeProblem = j, quote, "found unknown escape character"
		return j, false
	}
	width := hexEscapeWidth[n]
	for k := j + 2; k < j+2+width; k++ {
		if k >= len(src) || !isHex(src[k]) {
			// libyaml marks where the digits begin.
			s.badEscape, s.badEscapeQuote, s.badEscapeProblem = j+2, quote, "did not find expected hexdecimal number"
			return j, false
		}
	}
	return j + 2 + width, true
}

// blockCollectionError places libyaml's "did not find expected key" (in a
// block mapping) or "did not find expected '-' indicator" (in a block
// sequence): a token where the collection needs its next entry. The problem
// line is the first whose addition makes the document fail that way; the
// collection is the innermost block one of that kind open at that point
// that the token is not left of.
func blockCollectionError(src, problem string, kind yaml.Kind) *MarkedError {
	lines := strings.SplitAfter(src, "\n")
	p := failingLine(lines, problem)
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

	inner := innermostBlock(strings.Join(lines[:p], ""), kind, col)
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

// failingLine is the index of the first of lines whose addition makes the
// document fail, or -1 when none does or the failure is not problem.
func failingLine(lines []string, problem string) int {
	for k := 1; k <= len(lines); k++ {
		err := yaml.NewDecoder(strings.NewReader(strings.Join(lines[:k], ""))).Decode(new(yaml.Node))
		if err == nil || errors.Is(err, io.EOF) {
			continue
		}
		if !strings.HasSuffix(err.Error(), problem) {
			return -1
		}
		return k - 1
	}
	return -1
}

// innermostBlock is the innermost block collection of kind open at the end
// of doc whose column is not right of col, or nil.
func innermostBlock(doc string, kind yaml.Kind, col int) *yaml.Node {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(doc), &root); err != nil || len(root.Content) == 0 {
		return nil
	}
	var inner *yaml.Node
	for n := root.Content[0]; n != nil; {
		if n.Kind == kind && n.Style&yaml.FlowStyle == 0 && n.Column-1 <= col {
			inner = n
		}
		if len(n.Content) == 0 {
			break
		}
		n = n.Content[len(n.Content)-1]
	}
	return inner
}

// afterClosedValue is the offset in a line of the first token that follows
// a quoted scalar or flow collection on it, or -1.
func afterClosedValue(line string) int {
	i := closedValueStart(line)
	if i < 0 {
		return -1
	}
	j, closed := closedValueEnd(line, i)
	if !closed {
		return -1
	}
	for j < len(line) && isBlank(rune(line[j])) {
		j++
	}
	if j < len(line) && line[j] != '#' && line[j] != ':' {
		return j
	}
	return -1
}

// closedValueStart is the offset of the first token in line that opens a
// quoted scalar or flow collection, or -1.
func closedValueStart(line string) int {
	for i := 0; i < len(line); i++ {
		if c := line[i]; c != '\'' && c != '"' && c != '[' && c != '{' {
			continue
		}
		if i > 0 && !strings.ContainsRune(" \t", rune(line[i-1])) {
			continue
		}
		return i
	}
	return -1
}

// closedValueEnd is the offset after the quoted scalar or flow collection
// opening at i in line, or closed false for a quote the line leaves open.
func closedValueEnd(line string, i int) (int, bool) {
	if c := line[i]; c == '\'' || c == '"' {
		var s textScan
		return scanQuoted(line, i, &s)
	}
	depth := 0
	for end := i; end < len(line); end++ {
		switch line[end] {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		}
		if depth == 0 {
			return end + 1, true
		}
	}
	return len(line), true
}
