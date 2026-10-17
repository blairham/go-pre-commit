// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package pyyaml

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Dump is upstream's yaml_dump(v): yaml.dump with the SafeDumper,
// default_flow_style=False, indent=4, sort_keys=False, and PyYAML's other
// defaults (width 80, allow_unicode off, "\n" line breaks).
func Dump(v *Value) string {
	return DumpStyle(v, 0)
}

// DumpStyle is Dump with yaml.dump's default_style: 0 for none, or '\”
// or '"' to quote every scalar in that style where PyYAML can.
func DumpStyle(v *Value, defaultStyle byte) string {
	e := &emitter{indent: -1, whitespace: true, indention: true, bestIndent: 4, bestWidth: 80, defaultStyle: defaultStyle}
	if v == nil {
		v = &Value{Kind: Null}
	}
	e.anchors = map[*Value]string{}
	e.emitted = map[*Value]bool{}
	e.anchorNodes(v, map[*Value]bool{})
	e.expectNode(v, true, false, false, false)
	// expect_document_end: an implicit document ends with write_indent.
	e.writeIndent()
	if e.openEnded {
		e.writeIndicator("...", true, false, false)
		e.writeIndent()
	}
	return e.out.String()
}

// emitter is the part of PyYAML's Emitter a block-style SafeDumper dump
// reaches, with its column and whitespace bookkeeping kept exactly.
type emitter struct {
	emitted          map[*Value]bool
	anchors          map[*Value]string
	out              strings.Builder
	indents          []int
	column           int
	bestIndent       int
	bestWidth        int
	flowLevel        int
	indent           int
	whitespace       bool
	indention        bool
	openEnded        bool
	defaultStyle     byte
	rootContext      bool
	sequenceContext  bool
	mappingContext   bool
	simpleKeyContext bool
}

// scalar is a represented scalar: its text, and whether the plain form
// would resolve back to the same type (the serializer's implicit[0]).
type scalar struct {
	text     string
	implicit bool
}

func represent(v *Value) scalar {
	switch v.Kind {
	case Null:
		return scalar{"null", true}
	case Bool:
		if v.Bool {
			return scalar{"true", true}
		}
		return scalar{"false", true}
	case Int:
		return scalar{v.Int.String(), true}
	case Float:
		return scalar{reprFloat(v.Float), true}
	case Timestamp:
		return scalar{v.Str, true}
	}
	return scalar{v.Str, Resolve(v.Str) == "str"}
}

// reprFloat is SafeRepresenter.represent_float over Python's repr.
func reprFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return ".nan"
	case math.IsInf(f, 1):
		return ".inf"
	case math.IsInf(f, -1):
		return "-.inf"
	}
	value := strings.ToLower(pyRepr(f))
	if !strings.Contains(value, ".") && strings.Contains(value, "e") {
		value = strings.Replace(value, "e", ".0e", 1)
	}
	return value
}

// pyRepr is Python's float repr: the shortest round-tripping digits, in
// fixed notation for exponents from -4 to 15 and scientific otherwise.
func pyRepr(f float64) string {
	sci := strconv.FormatFloat(f, 'e', -1, 64) // d.ddde±XX
	mant, expS, _ := strings.Cut(sci, "e")
	exp, err := strconv.Atoi(expS)
	if err != nil {
		return sci
	}
	if exp < -4 || exp >= 16 {
		sign := "+"
		if exp < 0 {
			sign = "-"
			exp = -exp
		}
		return fmt.Sprintf("%se%s%02d", mant, sign, exp)
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

func (e *emitter) increaseIndent(flow, indentless bool) {
	e.indents = append(e.indents, e.indent)
	switch {
	case e.indent < 0:
		if flow {
			e.indent = e.bestIndent
		} else {
			e.indent = 0
		}
	case !indentless:
		e.indent += e.bestIndent
	}
}

func (e *emitter) popIndent() {
	e.indent = e.indents[len(e.indents)-1]
	e.indents = e.indents[:len(e.indents)-1]
}

// anchorNodes is Serializer.anchor_node: a collection met a second time
// gets the next "id%03d" anchor.
func (e *emitter) anchorNodes(v *Value, seen map[*Value]bool) {
	if v.Kind != List && v.Kind != Map {
		return
	}
	if seen[v] {
		if _, ok := e.anchors[v]; !ok {
			e.anchors[v] = fmt.Sprintf("id%03d", len(e.anchors)+1)
		}
		return
	}
	seen[v] = true
	for _, item := range v.Items {
		e.anchorNodes(item, seen)
	}
	for _, p := range v.Pairs {
		e.anchorNodes(p.Key, seen)
		e.anchorNodes(p.Value, seen)
	}
}

func (e *emitter) expectNode(v *Value, root, sequence, mapping, simpleKey bool) {
	e.rootContext, e.sequenceContext, e.mappingContext, e.simpleKeyContext = root, sequence, mapping, simpleKey
	if anchor, ok := e.anchors[v]; ok {
		if e.emitted[v] {
			e.writeIndicator("*"+anchor, true, false, false)
			return
		}
		e.emitted[v] = true
		e.writeIndicator("&"+anchor, true, false, false)
	}
	switch v.Kind {
	case List:
		if e.flowLevel > 0 || len(v.Items) == 0 {
			e.expectFlowSequence(v)
		} else {
			e.expectBlockSequence(v)
		}
	case Map:
		if e.flowLevel > 0 || len(v.Pairs) == 0 {
			e.expectFlowMapping(v)
		} else {
			e.expectBlockMapping(v)
		}
	default:
		e.increaseIndent(true, false)
		e.processScalar(represent(v))
		e.popIndent()
	}
}

// expectFlowSequence and expectFlowMapping are reached only for an empty
// collection: default_flow_style=False puts everything else in block style.
func (e *emitter) expectFlowSequence(v *Value) {
	e.writeIndicator("[", true, true, false)
	e.flowLevel++
	e.increaseIndent(true, false)
	for i, item := range v.Items {
		if i > 0 {
			e.writeIndicator(",", false, false, false)
		}
		if e.column > e.bestWidth {
			e.writeIndent()
		}
		e.expectNode(item, false, true, false, false)
	}
	e.popIndent()
	e.flowLevel--
	e.writeIndicator("]", false, false, false)
}

func (e *emitter) expectFlowMapping(v *Value) {
	e.writeIndicator("{", true, true, false)
	e.flowLevel++
	e.increaseIndent(true, false)
	for i, p := range v.Pairs {
		if i > 0 {
			e.writeIndicator(",", false, false, false)
		}
		if e.column > e.bestWidth {
			e.writeIndent()
		}
		if e.checkSimpleKey(p.Key) {
			e.expectNode(p.Key, false, false, true, true)
			e.writeIndicator(":", false, false, false)
		} else {
			e.writeIndicator("?", true, false, false)
			e.expectNode(p.Key, false, false, true, false)
			if e.column > e.bestWidth {
				e.writeIndent()
			}
			e.writeIndicator(":", true, false, false)
		}
		e.expectNode(p.Value, false, false, true, false)
	}
	e.popIndent()
	e.flowLevel--
	e.writeIndicator("}", false, false, false)
}

func (e *emitter) expectBlockSequence(v *Value) {
	indentless := e.mappingContext && !e.indention
	e.increaseIndent(false, indentless)
	for _, item := range v.Items {
		e.writeIndent()
		e.writeIndicator("-", true, false, true)
		e.expectNode(item, false, true, false, false)
	}
	e.popIndent()
}

func (e *emitter) expectBlockMapping(v *Value) {
	e.increaseIndent(false, false)
	for _, p := range v.Pairs {
		e.writeIndent()
		if e.checkSimpleKey(p.Key) {
			e.expectNode(p.Key, false, false, true, true)
			e.writeIndicator(":", false, false, false)
		} else {
			e.writeIndicator("?", true, false, true)
			e.expectNode(p.Key, false, false, true, false)
			e.writeIndent()
			e.writeIndicator(":", true, false, true)
		}
		e.expectNode(p.Value, false, false, true, false)
	}
	e.popIndent()
}

func (e *emitter) checkSimpleKey(k *Value) bool {
	switch k.Kind {
	case List:
		return len(k.Items) == 0
	case Map:
		return len(k.Pairs) == 0
	}
	a := analyze(represent(k).text)
	return len([]rune(a.scalar)) < 128 && !a.empty && !a.multiline
}

func (e *emitter) chooseStyle(s scalar, a analysis) byte {
	if e.defaultStyle == '"' {
		return '"'
	}
	if e.defaultStyle == 0 && s.implicit &&
		(!e.simpleKeyContext || (!a.empty && !a.multiline)) &&
		((e.flowLevel > 0 && a.allowFlowPlain) || (e.flowLevel == 0 && a.allowBlockPlain)) {
		return 0
	}
	if a.allowSingleQuoted && (!e.simpleKeyContext || !a.multiline) {
		return '\''
	}
	return '"'
}

func (e *emitter) processScalar(s scalar) {
	a := analyze(s.text)
	split := !e.simpleKeyContext
	text := []rune(a.scalar)
	switch e.chooseStyle(s, a) {
	case '"':
		e.writeDoubleQuoted(text, split)
	case '\'':
		e.writeSingleQuoted(text, split)
	default:
		e.writePlain(text, split)
	}
}

type analysis struct {
	scalar                                                         string
	empty, multiline                                               bool
	allowFlowPlain, allowBlockPlain, allowSingleQuoted, allowBlock bool
}

func isSpaceOrBreakOrNul(r rune) bool {
	return strings.ContainsRune("\x00 \t\r\n\u0085  ", r)
}

func isBreak(r rune) bool {
	return r == '\n' || r == '\u0085' || r == ' ' || r == ' '
}

// analyze is Emitter.analyze_scalar with allow_unicode off.
func analyze(str string) analysis {
	if str == "" {
		return analysis{scalar: str, empty: true, allowBlockPlain: true, allowSingleQuoted: true}
	}
	s := []rune(str)
	var blockIndicators, flowIndicators, lineBreaks, specialCharacters bool
	var leadingSpace, leadingBreak, trailingSpace, trailingBreak, breakSpace, spaceBreak bool

	if strings.HasPrefix(str, "---") || strings.HasPrefix(str, "...") {
		blockIndicators, flowIndicators = true, true
	}
	precededByWhitespace := true
	followedByWhitespace := len(s) == 1 || isSpaceOrBreakOrNul(s[1])
	previousSpace, previousBreak := false, false

	for i, ch := range s {
		if i == 0 {
			if strings.ContainsRune("#,[]{}&*!|>'\"%@`", ch) {
				flowIndicators, blockIndicators = true, true
			}
			if ch == '?' || ch == ':' {
				flowIndicators = true
				if followedByWhitespace {
					blockIndicators = true
				}
			}
			if ch == '-' && followedByWhitespace {
				flowIndicators, blockIndicators = true, true
			}
		} else {
			if strings.ContainsRune(",?[]{}", ch) {
				flowIndicators = true
			}
			if ch == ':' {
				flowIndicators = true
				if followedByWhitespace {
					blockIndicators = true
				}
			}
			if ch == '#' && precededByWhitespace {
				flowIndicators, blockIndicators = true, true
			}
		}
		if isBreak(ch) {
			lineBreaks = true
		}
		if ch != '\n' && (ch < 0x20 || ch > 0x7e) {
			// Unicode characters are special too, since allow_unicode is off.
			specialCharacters = true
		}
		switch {
		case ch == ' ':
			if i == 0 {
				leadingSpace = true
			}
			if i == len(s)-1 {
				trailingSpace = true
			}
			if previousBreak {
				breakSpace = true
			}
			previousSpace, previousBreak = true, false
		case isBreak(ch):
			if i == 0 {
				leadingBreak = true
			}
			if i == len(s)-1 {
				trailingBreak = true
			}
			if previousSpace {
				spaceBreak = true
			}
			previousSpace, previousBreak = false, true
		default:
			previousSpace, previousBreak = false, false
		}
		precededByWhitespace = isSpaceOrBreakOrNul(ch)
		followedByWhitespace = i+2 >= len(s) || isSpaceOrBreakOrNul(s[i+2])
	}

	a := analysis{
		scalar: str, multiline: lineBreaks,
		allowFlowPlain: true, allowBlockPlain: true, allowSingleQuoted: true, allowBlock: true,
	}
	if leadingSpace || leadingBreak || trailingSpace || trailingBreak {
		a.allowFlowPlain, a.allowBlockPlain = false, false
	}
	if trailingSpace {
		a.allowBlock = false
	}
	if breakSpace {
		a.allowFlowPlain, a.allowBlockPlain, a.allowSingleQuoted = false, false, false
	}
	if spaceBreak || specialCharacters {
		a.allowFlowPlain, a.allowBlockPlain, a.allowSingleQuoted, a.allowBlock = false, false, false, false
	}
	if lineBreaks {
		a.allowFlowPlain, a.allowBlockPlain = false, false
	}
	if flowIndicators {
		a.allowFlowPlain = false
	}
	if blockIndicators {
		a.allowBlockPlain = false
	}
	return a
}

func (e *emitter) write(data string) {
	e.column += len([]rune(data))
	e.out.WriteString(data)
}

func (e *emitter) writeIndicator(indicator string, needWhitespace, whitespace, indention bool) {
	data := indicator
	if !e.whitespace && needWhitespace {
		data = " " + indicator
	}
	e.whitespace = whitespace
	e.indention = e.indention && indention
	e.openEnded = false
	e.write(data)
}

func (e *emitter) writeIndent() {
	indent := max(e.indent, 0)
	if !e.indention || e.column > indent || (e.column == indent && !e.whitespace) {
		e.writeLineBreak("")
	}
	if e.column < indent {
		e.whitespace = true
		e.write(strings.Repeat(" ", indent-e.column))
	}
}

func (e *emitter) writeLineBreak(data string) {
	if data == "" {
		data = "\n"
	}
	e.whitespace = true
	e.indention = true
	e.column = 0
	e.out.WriteString(data)
}

func (e *emitter) writeBreaks(text []rune) {
	if text[0] == '\n' {
		e.writeLineBreak("")
	}
	for _, br := range text {
		if br == '\n' {
			e.writeLineBreak("")
		} else {
			e.writeLineBreak(string(br))
		}
	}
	e.writeIndent()
}

func (e *emitter) writePlain(text []rune, split bool) {
	if e.rootContext {
		e.openEnded = true
	}
	if len(text) == 0 {
		return
	}
	if !e.whitespace {
		e.write(" ")
	}
	e.whitespace, e.indention = false, false
	spaces, breaks := false, false
	start := 0
	for end := 0; end <= len(text); end++ {
		var ch rune = -1
		if end < len(text) {
			ch = text[end]
		}
		switch {
		case spaces:
			if ch != ' ' {
				if start+1 == end && e.column > e.bestWidth && split {
					e.writeIndent()
					e.whitespace, e.indention = false, false
				} else {
					e.write(string(text[start:end]))
				}
				start = end
			}
		case breaks:
			if !isBreak(ch) {
				e.writeBreaks(text[start:end])
				e.whitespace, e.indention = false, false
				start = end
			}
		default:
			if ch == -1 || ch == ' ' || isBreak(ch) {
				e.write(string(text[start:end]))
				start = end
			}
		}
		if ch != -1 {
			spaces, breaks = ch == ' ', isBreak(ch)
		}
	}
}

func (e *emitter) writeSingleQuoted(text []rune, split bool) {
	e.writeIndicator("'", true, false, false)
	spaces, breaks := false, false
	start := 0
	for end := 0; end <= len(text); end++ {
		var ch rune = -1
		if end < len(text) {
			ch = text[end]
		}
		switch {
		case spaces:
			if ch != ' ' {
				if start+1 == end && e.column > e.bestWidth && split && start != 0 && end != len(text) {
					e.writeIndent()
				} else {
					e.write(string(text[start:end]))
				}
				start = end
			}
		case breaks:
			if !isBreak(ch) {
				e.writeBreaks(text[start:end])
				start = end
			}
		default:
			if ch == -1 || ch == ' ' || isBreak(ch) || ch == '\'' {
				if start < end {
					e.write(string(text[start:end]))
					start = end
				}
			}
		}
		if ch == '\'' {
			e.write("''")
			start = end + 1
		}
		if ch != -1 {
			spaces, breaks = ch == ' ', isBreak(ch)
		}
	}
	e.writeIndicator("'", false, false, false)
}

var escapeReplacements = map[rune]string{
	0: "0", '\a': "a", '\b': "b", '\t': "t", '\n': "n", '\v': "v", '\f': "f",
	'\r': "r", 0x1b: "e", '"': "\"", '\\': "\\", 0x85: "N", 0xa0: "_",
	0x2028: "L", 0x2029: "P",
}

func (e *emitter) writeDoubleQuoted(text []rune, split bool) {
	e.writeIndicator("\"", true, false, false)
	start := 0
	for end := 0; end <= len(text); end++ {
		var ch rune = -1
		if end < len(text) {
			ch = text[end]
		}
		if ch == -1 || ch == '"' || ch == '\\' || ch == 0x85 || ch == 0x2028 || ch == 0x2029 || ch == 0xfeff ||
			ch < 0x20 || ch > 0x7e {
			if start < end {
				e.write(string(text[start:end]))
				start = end
			}
			if ch != -1 {
				var data string
				switch r, ok := escapeReplacements[ch]; {
				case ok:
					data = "\\" + r
				case ch <= 0xff:
					data = fmt.Sprintf("\\x%02X", ch)
				case ch <= 0xffff:
					data = fmt.Sprintf("\\u%04X", ch)
				default:
					data = fmt.Sprintf("\\U%08X", ch)
				}
				e.write(data)
				start = end + 1
			}
		}
		if end > 0 && end < len(text)-1 && (ch == ' ' || start >= end) &&
			e.column+(end-start) > e.bestWidth && split {
			data := "\\"
			if start < end {
				data = string(text[start:end]) + data
			}
			if start < end {
				start = end
			}
			e.write(data)
			e.writeIndent()
			e.whitespace, e.indention = false, false
			if text[start] == ' ' {
				e.write("\\")
			}
		}
	}
	e.writeIndicator("\"", false, false, false)
}
