// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package pyyaml reproduces the parts of PyYAML that upstream pre-commit's
// config rewriting depends on: yaml.load with the SafeLoader's YAML 1.1
// resolution, and yaml.dump in the shape upstream's yaml_dump asks for
// (block style, four-space indent, keys in order). Parsing is yaml.v3's;
// what this package adds is PyYAML's reading of the parsed tree and its
// emitter, so a file upstream reformats is reformatted byte for byte.
package pyyaml

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Kind is the Python type a loaded YAML value constructs to.
type Kind int

// The kinds, one per Python type SafeLoader constructs.
const (
	Null Kind = iota
	Bool
	Int
	Float
	Str
	// Timestamp is a date or datetime; it keeps the source text.
	Timestamp
	List
	Map
)

// Value is one loaded YAML value, as PyYAML's SafeLoader constructs it.
type Value struct {
	Int   *big.Int
	Str   string // a Str, or a Timestamp's isoformat text
	Items []*Value
	Pairs []Pair
	Kind  Kind
	Float float64
	Bool  bool
}

// Pair is one entry of a Map, in insertion order.
type Pair struct {
	Key, Value *Value
}

// Parse parses src as a single YAML document, failing where yaml_load would
// fail for a syntax error or a second document. A nil node with no error is
// an empty stream, which yaml_load reads as None.
func Parse(src string) (*yaml.Node, error) {
	dec := yaml.NewDecoder(strings.NewReader(src))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, composeError(src, err)
	}
	var root *yaml.Node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		root = doc.Content[0]
	}
	if err := duplicateAnchor(root); err != nil {
		return nil, err
	}
	var next yaml.Node
	if err := dec.Decode(&next); !errors.Is(err, io.EOF) {
		// _yaml.pyx's get_single_node: the next event is checked before
		// any of the second document is read.
		second := nodeMark(&next)
		if err != nil {
			second = secondDocumentMark(src, root)
		}
		if second == nil || root == nil {
			if err != nil {
				return nil, composeError(src, err)
			}
			return nil, errors.New("expected a single document in the stream")
		}
		return nil, &MarkedError{
			Context: "expected a single document in the stream", ContextMark: nodeMark(root),
			Problem: "but found another document", ProblemMark: second,
		}
	}
	return root, nil
}

// composeError is a yaml.v3 error as the libyaml loader reports it: an
// unknown alias is the composer's, anything else the scanner's or parser's.
func composeError(src string, err error) error {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	if name, ok := strings.CutPrefix(msg, "unknown anchor '"); ok {
		name = strings.TrimSuffix(name, "' referenced")
		if off := aliasOffset(src, name); off >= 0 {
			return &MarkedError{Problem: "found undefined alias", ProblemMark: markAt(src, off)}
		}
		return err
	}
	return syntaxError(src, err)
}

// aliasOffset is the offset of the first "*name" alias token in src, or -1.
func aliasOffset(src, name string) int {
	tok := "*" + name
	for i := 0; ; {
		j := strings.Index(src[i:], tok)
		if j < 0 {
			return -1
		}
		at := i + j
		end := at + len(tok)
		before := at == 0 || strings.ContainsRune(" \t\n[{,:", rune(src[at-1]))
		after := end == len(src) || strings.ContainsRune(" \t\n]},", rune(src[end]))
		if before && after {
			return at
		}
		i = at + 1
	}
}

// duplicateAnchor is _yaml.pyx's check that an anchor names one node.
func duplicateAnchor(root *yaml.Node) error {
	seen := map[string]*yaml.Node{}
	var walk func(n *yaml.Node) error
	walk = func(n *yaml.Node) error {
		if n == nil || n.Kind == yaml.AliasNode {
			return nil
		}
		if n.Anchor != "" {
			if first, ok := seen[n.Anchor]; ok {
				return &MarkedError{
					Context: "found duplicate anchor; first occurrence", ContextMark: nodeMark(first),
					Problem: "second occurrence", ProblemMark: nodeMark(n),
				}
			}
			seen[n.Anchor] = n
		}
		for _, c := range n.Content {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}

// secondDocumentMark finds the "---" that starts a second document when
// that document does not parse, so its own position is unknown.
func secondDocumentMark(src string, root *yaml.Node) *Mark {
	if root == nil {
		return nil
	}
	off := 0
	for line := 0; off < len(src); line++ {
		end := strings.IndexByte(src[off:], '\n')
		text := src[off:]
		if end >= 0 {
			text = src[off : off+end]
		}
		if line >= root.Line && (text == "---" || strings.HasPrefix(text, "--- ") || strings.HasPrefix(text, "---\t")) {
			return &Mark{Line: line}
		}
		if end < 0 {
			break
		}
		off += end + 1
	}
	return nil
}

// Load is yaml_load: Parse, then construct the value. A nil Value is None.
func Load(src string) (*Value, error) {
	n, err := Parse(src)
	if err != nil || n == nil {
		return nil, err
	}
	return construct(n)
}

func construct(n *yaml.Node) (*Value, error) {
	return (&constructor{built: map[*yaml.Node]*Value{}}).construct(n)
}

// constructor builds each node once, so every alias of a collection is the
// same Value, as PyYAML's constructed_objects makes it the same object.
type constructor struct {
	built map[*yaml.Node]*Value
}

func (c *constructor) construct(n *yaml.Node) (*Value, error) {
	for n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if v, ok := c.built[n]; ok {
		return v, nil
	}
	v, err := c.build(n)
	if err == nil {
		c.built[n] = v
	}
	return v, err
}

func (c *constructor) build(n *yaml.Node) (*Value, error) {
	switch n.Kind {
	case yaml.ScalarNode:
		return constructScalar(n)
	case yaml.SequenceNode:
		v := &Value{Kind: List}
		for _, item := range n.Content {
			cv, err := c.construct(item)
			if err != nil {
				return nil, err
			}
			v.Items = append(v.Items, cv)
		}
		return v, nil
	case yaml.MappingNode:
		v := &Value{Kind: Map}
		pairs, err := flatten(n)
		if err != nil {
			return nil, err
		}
		for _, p := range pairs {
			k, err := c.construct(p[0])
			if err != nil {
				return nil, err
			}
			if k.Kind == List || k.Kind == Map {
				return nil, &MarkedError{
					Context: "while constructing a mapping", ContextMark: nodeMark(n),
					Problem: "found unhashable key", ProblemMark: nodeMark(p[0]),
				}
			}
			val, err := c.construct(p[1])
			if err != nil {
				return nil, err
			}
			v.set(k, val)
		}
		return v, nil
	}
	return &Value{Kind: Null}, nil
}

// set assigns as a Python dict does: a repeated key keeps its first
// position and takes the last value.
func (v *Value) set(k, val *Value) {
	for i := range v.Pairs {
		if sameKey(v.Pairs[i].Key, k) {
			v.Pairs[i].Value = val
			return
		}
	}
	v.Pairs = append(v.Pairs, Pair{k, val})
}

func sameKey(a, b *Value) bool {
	// Python's bool, int and float compare (and hash) by value:
	// True == 1 == 1.0 is one dict key.
	if an, ok := number(a); ok {
		bn, ok := number(b)
		return ok && an.Cmp(bn) == 0
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case Null:
		return true
	case Bool:
		return a.Bool == b.Bool
	case Int:
		return a.Int.Cmp(b.Int) == 0
	case Float:
		return a.Float == b.Float
	case Str, Timestamp:
		return a.Str == b.Str
	}
	return false
}

// number is v's numeric value when v is a bool, int or finite float.
func number(v *Value) (*big.Float, bool) {
	switch v.Kind {
	case Bool:
		if v.Bool {
			return big.NewFloat(1), true
		}
		return big.NewFloat(0), true
	case Int:
		return new(big.Float).SetInt(v.Int), true
	case Float:
		if math.IsNaN(v.Float) {
			return nil, false
		}
		if math.IsInf(v.Float, 0) {
			return new(big.Float).SetInf(v.Float < 0), true
		}
		return big.NewFloat(v.Float), true
	}
	return nil, false
}

// flatten is SafeConstructor.flatten_mapping: merge keys' mappings come
// first, then the mapping's own pairs.
func flatten(n *yaml.Node) ([][2]*yaml.Node, error) {
	var merge, own [][2]*yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if !isMergeKey(k) {
			own = append(own, [2]*yaml.Node{k, v})
			continue
		}
		for v.Kind == yaml.AliasNode {
			v = v.Alias
		}
		switch v.Kind {
		case yaml.MappingNode:
			sub, err := flatten(v)
			if err != nil {
				return nil, err
			}
			merge = append(merge, sub...)
		case yaml.SequenceNode:
			var subs [][][2]*yaml.Node
			for _, s := range v.Content {
				for s.Kind == yaml.AliasNode {
					s = s.Alias
				}
				if s.Kind != yaml.MappingNode {
					return nil, &MarkedError{
						Context: "while constructing a mapping", ContextMark: nodeMark(n),
						Problem:     "expected a mapping for merging, but found " + nodeID(s),
						ProblemMark: nodeMark(s),
					}
				}
				sub, err := flatten(s)
				if err != nil {
					return nil, err
				}
				subs = append(subs, sub)
			}
			for i := len(subs) - 1; i >= 0; i-- {
				merge = append(merge, subs[i]...)
			}
		default:
			return nil, &MarkedError{
				Context: "while constructing a mapping", ContextMark: nodeMark(n),
				Problem:     "expected a mapping or list of mappings for merging, but found " + nodeID(v),
				ProblemMark: nodeMark(v),
			}
		}
	}
	return append(merge, own...), nil
}

// nodeID is PyYAML's Node.id: the kind of node, as error messages name it.
func nodeID(n *yaml.Node) string {
	switch n.Kind {
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	}
	return "scalar"
}

func isMergeKey(k *yaml.Node) bool {
	if k.Kind != yaml.ScalarNode {
		return false
	}
	if k.Style&yaml.TaggedStyle != 0 {
		return k.Tag == "!!merge"
	}
	return k.Style&(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0 &&
		k.Value == "<<"
}

func constructScalar(n *yaml.Node) (*Value, error) {
	tag := ""
	switch {
	case n.Style&yaml.TaggedStyle != 0:
		tag = strings.TrimPrefix(n.Tag, "!!")
	case n.Style&(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0:
		tag = "str"
	default:
		tag = Resolve(n.Value)
	}
	if tag == "timestamp" {
		iso, ok, err := isoTimestamp(n.Value)
		if err != nil {
			return nil, err
		}
		if ok {
			return &Value{Kind: Timestamp, Str: iso}, nil
		}
	}
	return scalarOf(tag, n.Value), nil
}

func scalarOf(tag, s string) *Value {
	switch tag {
	case "null":
		return &Value{Kind: Null}
	case "bool":
		switch strings.ToLower(s) {
		case "yes", "true", "on":
			return &Value{Kind: Bool, Bool: true}
		case "no", "false", "off":
			return &Value{Kind: Bool, Bool: false}
		}
	case "int":
		if i, ok := parseInt(s); ok {
			return &Value{Kind: Int, Int: i}
		}
	case "float":
		if f, ok := parseFloat(s); ok {
			return &Value{Kind: Float, Float: f}
		}
	case "timestamp":
		if iso, ok, err := isoTimestamp(s); ok && err == nil {
			return &Value{Kind: Timestamp, Str: iso}
		}
	}
	return &Value{Kind: Str, Str: s}
}

var (
	boolRe  = regexp.MustCompile(`^(?:yes|Yes|YES|no|No|NO|true|True|TRUE|false|False|FALSE|on|On|ON|off|Off|OFF)$`)
	floatRe = regexp.MustCompile(`^(?:[-+]?(?:[0-9][0-9_]*)\.[0-9_]*(?:[eE][-+][0-9]+)?` +
		`|\.[0-9][0-9_]*(?:[eE][-+][0-9]+)?` +
		`|[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+\.[0-9_]*` +
		`|[-+]?\.(?:inf|Inf|INF)` +
		`|\.(?:nan|NaN|NAN))$`)
	intRe = regexp.MustCompile(`^(?:[-+]?0b[0-1_]+` +
		`|[-+]?0[0-7_]+` +
		`|[-+]?(?:0|[1-9][0-9_]*)` +
		`|[-+]?0x[0-9a-fA-F_]+` +
		`|[-+]?[1-9][0-9_]*(?::[0-5]?[0-9])+)$`)
	mergeRe     = regexp.MustCompile(`^(?:<<)$`)
	nullRe      = regexp.MustCompile(`^(?:~|null|Null|NULL|)$`)
	timestampRe = regexp.MustCompile(`^(?:[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]` +
		`|[0-9][0-9][0-9][0-9]-[0-9][0-9]?-[0-9][0-9]?` +
		`(?:[Tt]|[ \t]+)[0-9][0-9]?` +
		`:[0-9][0-9]:[0-9][0-9](?:\.[0-9]*)?` +
		`(?:[ \t]*(?:Z|[-+][0-9][0-9]?(?::[0-9][0-9])?))?)$`)
	valueRe = regexp.MustCompile(`^=$`)
)

type resolver struct {
	tag    string
	re     *regexp.Regexp
	starts string
}

// resolvers are PyYAML's implicit resolvers in registration order; each
// applies only to a scalar starting with one of its characters.
var resolvers = []resolver{
	{"bool", boolRe, "yYnNtTfFoO"},
	{"float", floatRe, "-+0123456789."},
	{"int", intRe, "-+0123456789"},
	{"merge", mergeRe, "<"},
	{"null", nullRe, "~nN"},
	{"timestamp", timestampRe, "0123456789"},
	{"value", valueRe, "="},
}

// Resolve is the tag PyYAML's Resolver gives s as a plain scalar.
func Resolve(s string) string {
	if s == "" {
		return "null"
	}
	first, _ := utf8.DecodeRuneInString(s)
	for _, r := range resolvers {
		if strings.ContainsRune(r.starts, first) && r.re.MatchString(s) {
			return r.tag
		}
	}
	return "str"
}

// parseInt is SafeConstructor.construct_yaml_int.
func parseInt(s string) (*big.Int, bool) {
	s = strings.ReplaceAll(s, "_", "")
	sign := 1
	if s != "" && s[0] == '-' {
		sign = -1
	}
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	if s == "" {
		return nil, false
	}
	v := new(big.Int)
	var ok bool
	switch {
	case s == "0":
		return v, true
	case strings.HasPrefix(s, "0b"):
		_, ok = v.SetString(s[2:], 2)
	case strings.HasPrefix(s, "0x"):
		_, ok = v.SetString(s[2:], 16)
	case s[0] == '0':
		_, ok = v.SetString(s, 8)
	case strings.Contains(s, ":"):
		parts := strings.Split(s, ":")
		ok = true
		for _, p := range parts {
			d, err := strconv.ParseInt(p, 10, 64)
			if err != nil {
				return nil, false
			}
			v.Mul(v, big.NewInt(60))
			v.Add(v, big.NewInt(d))
		}
	default:
		_, ok = v.SetString(s, 10)
	}
	if !ok {
		return nil, false
	}
	if sign < 0 {
		v.Neg(v)
	}
	return v, true
}

// parseFloat is SafeConstructor.construct_yaml_float.
func parseFloat(s string) (float64, bool) {
	s = strings.ToLower(strings.ReplaceAll(s, "_", ""))
	sign := 1.0
	if s != "" && s[0] == '-' {
		sign = -1
	}
	if s != "" && (s[0] == '+' || s[0] == '-') {
		s = s[1:]
	}
	switch {
	case s == ".inf":
		return sign * math.Inf(1), true
	case s == ".nan":
		return math.NaN(), true
	case strings.Contains(s, ":"):
		v := 0.0
		for _, p := range strings.Split(s, ":") {
			d, err := strconv.ParseFloat(p, 64)
			if err != nil {
				return 0, false
			}
			v = v*60 + d
		}
		return sign * v, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return sign * f, true
}

var timestampParts = regexp.MustCompile(`^([0-9]{4})-([0-9][0-9]?)-([0-9][0-9]?)` +
	`(?:(?:[Tt]|[ \t]+)([0-9][0-9]?):([0-9][0-9]):([0-9][0-9])(?:\.([0-9]*))?` +
	`(?:[ \t]*(Z|([-+])([0-9][0-9]?)(?::([0-9][0-9]))?))?)?$`)

// isoTimestamp is construct_yaml_timestamp followed by the representer's
// isoformat: a date as YYYY-MM-DD, a datetime as isoformat(' '). A value
// the datetime constructors refuse is their ValueError, worded as Python
// 3.13's (the parity suite's reference).
func isoTimestamp(s string) (string, bool, error) {
	m := timestampParts.FindStringSubmatch(s)
	if m == nil {
		return "", false, nil
	}
	// Every group atoi reads matched digits only, so it cannot fail.
	atoi := func(x string) int {
		n, err := strconv.Atoi(x)
		if err != nil {
			return 0
		}
		return n
	}
	year, month, day := atoi(m[1]), atoi(m[2]), atoi(m[3])

	// datetime.timezone(delta) is built before the datetime itself.
	var tz string
	switch {
	case m[9] != "":
		offset := atoi(m[10])*60 + atoi(m[11])
		if m[9] == "-" {
			offset = -offset
		}
		if offset <= -24*60 || offset >= 24*60 {
			return "", false, valueError(fmt.Sprintf("offset must be a timedelta strictly between "+
				"-timedelta(hours=24) and timedelta(hours=24), not %s.", timedeltaRepr(offset*60)))
		}
		sign := "+"
		if offset < 0 {
			sign, offset = "-", -offset
		}
		tz = fmt.Sprintf("%s%02d:%02d", sign, offset/60, offset%60)
	case m[8] == "Z":
		tz = "+00:00"
	}

	if err := checkDate(year, month, day); err != nil {
		return "", false, err
	}
	out := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	if m[4] == "" {
		return out, true, nil
	}
	hour, minute, second := atoi(m[4]), atoi(m[5]), atoi(m[6])
	switch {
	case hour > 23:
		return "", false, errors.New("hour must be in 0..23")
	case minute > 59:
		return "", false, errors.New("minute must be in 0..59")
	case second > 59:
		return "", false, errors.New("second must be in 0..59")
	}
	out += fmt.Sprintf(" %02d:%02d:%02d", hour, minute, second)
	if frac := m[7]; frac != "" {
		if len(frac) > 6 {
			frac = frac[:6]
		}
		frac += strings.Repeat("0", 6-len(frac))
		if atoi(frac) != 0 {
			out += "." + frac
		}
	}
	return out + tz, true, nil
}

// valueError is a Python ValueError's message, kept verbatim (CPython's
// end with a full stop where Go's would not).
type valueError string

func (e valueError) Error() string { return string(e) }

// checkDate is CPython's check_date_args.
func checkDate(year, month, day int) error {
	switch {
	case year < 1 || year > 9999:
		return fmt.Errorf("year %d is out of range", year)
	case month < 1 || month > 12:
		return errors.New("month must be in 1..12")
	}
	last := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day < 1 || day > last {
		return errors.New("day is out of range for month")
	}
	return nil
}

// timedeltaRepr is repr(datetime.timedelta(seconds=secs)).
func timedeltaRepr(secs int) string {
	days := secs / 86400
	rem := secs % 86400
	if rem < 0 {
		days--
		rem += 86400
	}
	var parts []string
	if days != 0 {
		parts = append(parts, fmt.Sprintf("days=%d", days))
	}
	if rem != 0 {
		parts = append(parts, fmt.Sprintf("seconds=%d", rem))
	}
	if len(parts) == 0 {
		return "datetime.timedelta(0)"
	}
	return "datetime.timedelta(" + strings.Join(parts, ", ") + ")"
}

// PyTimestamp is a timestamp as LoadAny returns it: Date for a
// datetime.date, otherwise a datetime.datetime; Text is its isoformat.
type PyTimestamp struct {
	Text string
	Date bool
}

// LoadAny is yaml_load as Go values with Python's types: nil, bool, int (or
// *big.Int past int64), float64, string, PyTimestamp, []any, and
// map[string]any -- or map[any]any when a key is not a string.
func LoadAny(src string) (any, error) {
	v, err := Load(src)
	if err != nil || v == nil {
		return nil, err
	}
	return v.ToAny(), nil
}

// ToAny is v as LoadAny returns it.
func (v *Value) ToAny() any {
	switch v.Kind {
	case Bool:
		return v.Bool
	case Int:
		if v.Int.IsInt64() {
			return int(v.Int.Int64())
		}
		return v.Int
	case Float:
		return v.Float
	case Str:
		return v.Str
	case Timestamp:
		return PyTimestamp{Text: v.Str, Date: !strings.Contains(v.Str, " ")}
	case List:
		out := make([]any, len(v.Items))
		for i, item := range v.Items {
			out[i] = item.ToAny()
		}
		return out
	case Map:
		allStr := true
		for _, p := range v.Pairs {
			if p.Key.Kind != Str {
				allStr = false
				break
			}
		}
		if allStr {
			out := make(map[string]any, len(v.Pairs))
			for _, p := range v.Pairs {
				out[p.Key.Str] = p.Value.ToAny()
			}
			return out
		}
		out := make(map[any]any, len(v.Pairs))
		for _, p := range v.Pairs {
			k := p.Key.ToAny()
			if b, ok := k.(*big.Int); ok {
				k = b.String() // a *big.Int is not a usable map key
			}
			out[k] = p.Value.ToAny()
		}
		return out
	}
	return nil
}
