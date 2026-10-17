// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/blairham/go-pre-commit/v4/internal/fatal"
	"github.com/blairham/go-pre-commit/v4/internal/pcre"
	"github.com/blairham/go-pre-commit/v4/internal/pyyaml"
)

// This file ports the parts of cfgv (the schema library upstream validates
// .pre-commit-config.yaml and .pre-commit-hooks.yaml with) and of
// pre_commit/clientlib.py's schemas that can fail. Validation runs over the
// decoded YAML before it is bound to the Go structs, because the error is
// keyed on what the user wrote: its type, whether a key is there at all, and
// the order upstream checks keys in, which decides which of two problems is
// the one reported.
//
// The warnings upstream's schemas emit are schema items too (cfgv_warn.go),
// so they fire in upstream's order, before a later key fails.

// ValidationError is cfgv.ValidationError: a message wrapped in the contexts
// it was raised under, outermost first.
type ValidationError struct {
	ctx   string           // empty for the innermost error
	inner *ValidationError // set when ctx is
	msg   string           // the innermost error's message
}

func newValidationError(format string, args ...any) *ValidationError {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}

// Error is cfgv's ValidationError.__str__: a leading newline, one
// "==> context" line per level, and "=====> message".
func (e *ValidationError) Error() string {
	var b strings.Builder
	b.WriteString("\n")
	err := e
	for err.ctx != "" {
		b.WriteString("==> " + err.ctx + "\n")
		err = err.inner
	}
	b.WriteString("=====> " + err.msg)
	return b.String()
}

// withContext is cfgv.validate_context: a ValidationError raised inside is
// wrapped in one more level; anything else passes through untouched.
func withContext(ctx string, err error) error {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return &ValidationError{ctx: ctx, inner: ve}
	}
	return err
}

// inFile is err in cfgv's "File <path>" context.
func inFile(path string, err *ValidationError) *ValidationError {
	return &ValidationError{ctx: "File " + path, inner: err}
}

// pyError is an exception upstream does not catch while loading, which its
// error handler therefore reports as unexpected (exit 3) under its own type.
type pyError struct {
	typ string
	msg string
}

func (e *pyError) Error() string     { return e.msg }
func (e *pyError) ErrorType() string { return e.typ }

// InvalidConfigError and InvalidManifestError are what cfgv's reraise_as
// turns a ValidationError into, for a config and a manifest respectively.
func invalidConfig(err *ValidationError) *fatal.Error {
	return &fatal.Error{Type: "InvalidConfigError", Msg: err.Error()}
}

func invalidManifest(err *ValidationError) *fatal.Error {
	return &fatal.Error{Type: "InvalidManifestError", Msg: err.Error()}
}

// loadFromFilename is cfgv.load_from_filename up to validation: the file's
// bytes once they have validated, or the error upstream raises. A
// ValidationError comes back as the fatal error wrap makes of it.
func loadFromFilename(
	filename string,
	wrap func(*ValidationError) *fatal.Error,
	validate func(any) error,
) ([]byte, error) {
	reraise := func(err error) error {
		var ve *ValidationError
		if errors.As(err, &ve) {
			return wrap(ve)
		}
		return err
	}

	if info, err := os.Stat(filename); err != nil || !info.Mode().IsRegular() {
		return nil, reraise(newValidationError("%s is not a file", filename))
	}

	ctx := "File " + filename
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err // an OSError: nothing in the load path catches it
	}
	if msg, ok := utf8DecodeError(data); !ok {
		return nil, reraise(withContext(ctx, newValidationError("%s", msg)))
	}

	// Validation sees the values PyYAML builds: YAML 1.1 types (`yes` is a
	// bool), repeated keys collapsed as a dict does, and keys of any type.
	// Python's text-mode open() reads any line ending as "\n", which is
	// what the marks in a YAML error count.
	text := strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\r", "\n")
	doc, err := pyyaml.LoadAny(text)
	if err != nil {
		return nil, reraise(withContext(ctx, newValidationError("%s", err.Error())))
	}

	if err := validate(doc); err != nil {
		return nil, reraise(withContext(ctx, err))
	}
	return data, nil
}

// utf8DecodeError is the message Python's UnicodeDecodeError carries for the
// first byte that is not valid UTF-8, or ok when there is none.
func utf8DecodeError(data []byte) (string, bool) {
	for i := 0; i < len(data); {
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size <= 1 {
			reason := "invalid start byte"
			if data[i] >= 0xc2 && data[i] <= 0xf4 {
				reason = "invalid continuation byte"
				if i+utf8.UTFMax > len(data) && !slices.ContainsFunc(data[i+1:], func(b byte) bool { return b&0xc0 != 0x80 }) {
					return fmt.Sprintf("'utf-8' codec can't decode bytes in position %d-%d: unexpected end of data",
						i, len(data)-1), false
				}
			}
			return fmt.Sprintf("'utf-8' codec can't decode byte 0x%02x in position %d: %s", data[i], i, reason), false
		}
		i += size
	}
	return "", true
}

// --- checks: cfgv's check_* functions ---

type checkFn func(v any) error

func checkAny(any) error { return nil }

// checkType is cfgv.check_type: "Expected <typename> got <type>".
func checkType(want, typename string) checkFn {
	return func(v any) error {
		if pyTypeName(v) != want {
			return newValidationError("Expected %s got %s", typename, pyTypeName(v))
		}
		return nil
	}
}

var (
	checkString = checkType("str", "string")
	checkBool   = checkType("bool", "bool")
	checkDict   = checkType("dict", "dict")
)

func checkRegex(v any) error {
	s, ok := v.(string)
	if !ok {
		return checkString(v)
	}
	if _, err := pcre.Compile(s); err != nil {
		return newValidationError("%s is not a valid python regex", PyRepr(v))
	}
	return nil
}

func checkAnd(fns ...checkFn) checkFn {
	return func(v any) error {
		for _, fn := range fns {
			if err := fn(v); err != nil {
				return err
			}
		}
		return nil
	}
}

var checkStringRegex = checkAnd(checkString, checkRegex)

func checkArray(inner checkFn) checkFn {
	return func(v any) error {
		list, ok := v.([]any)
		if !ok {
			return newValidationError("Expected array but got %s", PyRepr(pyTypeName(v)))
		}
		for i, val := range list {
			if err := inner(val); err != nil {
				return withContext(fmt.Sprintf("At index %d", i), err)
			}
		}
		return nil
	}
}

// checkOneOf is cfgv.check_one_of; possible is shown sorted.
func checkOneOf(possible []string) checkFn {
	sorted := slices.Clone(possible)
	slices.Sort(sorted)
	return func(v any) error {
		if s, ok := v.(string); ok && slices.Contains(possible, s) {
			return nil
		}
		return newValidationError("Expected one of %s but got: %s", strings.Join(sorted, ", "), PyRepr(v))
	}
}

// checkTypeTag is clientlib.check_type_tag, short of identify's tag list:
// a tag that is not a string can never be one, and is reported as upstream
// does; a string is accepted.
func checkTypeTag(v any) error {
	if _, ok := v.(string); ok {
		return nil
	}
	return newValidationError("Type tag %s is not recognized.  Try upgrading identify and pre-commit?", PyRepr(v))
}

// checkMinVersion is clientlib.check_min_version. parse_version is int() of
// each dotted part, so a part that is not a number is a ValueError, which
// nothing catches.
func checkMinVersion(v any) error {
	s, ok := v.(string)
	if !ok {
		return checkString(v)
	}
	var want []int
	for p := range strings.SplitSeq(s, ".") {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return &pyError{typ: "ValueError", msg: "invalid literal for int() with base 10: " + PyRepr(p)}
		}
		want = append(want, n)
	}
	if slices.Compare(want, splitVersionParts(Version)) > 0 {
		return newValidationError(
			"pre-commit version %s is required but version %s is installed.  "+
				"Perhaps run `pip install --upgrade pre-commit`.", s, Version,
		)
	}
	return nil
}

// --- schema items: cfgv's Required, Optional, Conditional, ... ---

type item func(m map[string]any, raw any) error

func missing(key string) error { return newValidationError("Missing required key: %s", key) }

func optional(key string, fn checkFn) item {
	return func(m map[string]any, _ any) error {
		v, ok := m[key]
		if !ok {
			return nil
		}
		return withContext("At key: "+key, fn(v))
	}
}

func required(key string, fn checkFn) item {
	opt := optional(key, fn)
	return func(m map[string]any, _ any) error {
		if _, ok := m[key]; !ok {
			return missing(key)
		}
		return opt(m, nil)
	}
}

// condition is cfgv's condition_value: a plain value, or NotIn.
type condition struct {
	equals string
	notIn  []string
}

func (c condition) matches(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	if c.notIn != nil {
		s, isStr := v.(string)
		return !isStr || !slices.Contains(c.notIn, s)
	}
	s, isStr := v.(string)
	return isStr && s == c.equals
}

func (c condition) describeOpposite() string {
	if c.notIn != nil {
		quoted := make([]string, len(c.notIn))
		for i, s := range c.notIn {
			quoted[i] = PyRepr(s)
		}
		return "is any of (" + strings.Join(quoted, ", ") + ")"
	}
	return "is not " + PyRepr(c.equals)
}

// conditionalRequired is cfgv.Conditional / ConditionalRecurse: key is
// required while condKey matches cond, and with ensureAbsent must be absent
// while it is present and does not.
func conditionalRequired(key string, fn checkFn, condKey string, cond condition, ensureAbsent bool) item {
	req := required(key, fn)
	return func(m map[string]any, _ any) error {
		if cond.matches(m, condKey) {
			return req(m, nil)
		}
		_, hasCond := m[condKey]
		if v, has := m[key]; hasCond && ensureAbsent && has {
			return newValidationError("Expected %s to be absent when %s %s, found %s: %s",
				key, condKey, cond.describeOpposite(), key, PyRepr(v))
		}
		return nil
	}
}

func notAllowed(key string) item {
	return func(m map[string]any, _ any) error {
		if _, ok := m[key]; ok {
			return newValidationError("%s cannot be overridden", PyRepr(key))
		}
		return nil
	}
}

func noAdditionalKeys(keys []string) item {
	return func(_ map[string]any, raw any) error {
		extra, err := pySortedExtraKeys(raw, keys)
		if err != nil || len(extra) == 0 {
			return err
		}
		strs := make([]string, len(extra))
		for i, k := range extra {
			strs[i] = pyStr(k)
		}
		return newValidationError("Additional keys found: %s.  Only these keys are allowed: %s",
			strings.Join(strs, ", "), strings.Join(keys, ", "))
	}
}

// stagesMigration is clientlib.StagesMigration's check: an array, whose
// entries after the legacy-name translation are all stages.
func stagesMigration(key string) item {
	return optional(key, func(v any) error {
		list, ok := v.([]any)
		if !ok {
			return checkArray(checkAny)(v)
		}
		translated := make([]any, len(list))
		for i, s := range list {
			translated[i] = s
			if str, ok := s.(string); ok {
				if to, ok := legacyStages[Stage(str)]; ok {
					translated[i] = string(to)
				}
			}
		}
		return checkArray(checkOneOf(stageNames()))(translated)
	})
}

// languageMigration is clientlib.LanguageMigration (and, required,
// LanguageMigrationRequired): system and script are checked as their
// current names.
func languageMigration(key string, isRequired bool) item {
	check := checkOneOf(LanguageNames)
	opt := optional(key, func(v any) error {
		if s, ok := v.(string); ok {
			switch s {
			case "system":
				v = "unsupported"
			case "script":
				v = "unsupported_script"
			}
		}
		return check(v)
	})
	return func(m map[string]any, _ any) error {
		if _, ok := m[key]; !ok && isRequired {
			return missing(key)
		}
		return opt(m, nil)
	}
}

// mapSchema is cfgv.Map.
type mapSchema struct {
	name  string
	idKey string
	items []item
}

func (s mapSchema) check(v any) error {
	m, ok := asStringMap(v)
	if !ok {
		return newValidationError("Expected a %s map but got a %s", s.name, pyTypeName(v))
	}
	ctx := "At " + s.name + "()"
	if s.idKey != "" {
		id := "MISSING"
		if idv, ok := m[s.idKey]; ok {
			id = PyRepr(idv)
		}
		ctx = fmt.Sprintf("At %s(%s=%s)", s.name, s.idKey, id)
	}
	for _, it := range s.items {
		if err := it(m, v); err != nil {
			return withContext(ctx, err)
		}
	}
	return nil
}

// arrayOf is cfgv.Array: an array, each entry checked against the map with
// no per-index context.
func arrayOf(of mapSchema) checkFn {
	return func(v any) error {
		list, ok := v.([]any)
		if !ok {
			return checkArray(checkAny)(v)
		}
		for _, val := range list {
			if err := of.check(val); err != nil {
				return err
			}
		}
		return nil
	}
}

// --- clientlib's schemas ---

// LanguageNames is upstream's all_languages.language_names: every value
// `language` may take (after system and script are translated).
var LanguageNames = []string{
	"conda", "coursier", "dart", "docker", "docker_image", "dotnet", "fail",
	"golang", "haskell", "julia", "lua", "node", "perl", "pygrep", "python",
	"r", "ruby", "rust", "swift", "unsupported", "unsupported_script",
}

func hookTypeNames() []string {
	names := make([]string, 0, len(AllHookTypes()))
	for _, h := range AllHookTypes() {
		names = append(names, string(h))
	}
	return names
}

func stageNames() []string {
	names := make([]string, 0, len(AllStages()))
	for _, s := range AllStages() {
		names = append(names, string(s))
	}
	return names
}

// manifestHookItems is MANIFEST_HOOK_DICT's items, in upstream's order.
// The items CONFIG_HOOK_DICT and META_HOOK_DICT reuse as optional are
// marked by key so they can be rebuilt that way.
type keyedItem struct {
	fn  checkFn
	it  item
	key string
}

func manifestHookItems() []keyedItem {
	opt := func(key string, fn checkFn) keyedItem { return keyedItem{key: key, fn: fn, it: optional(key, fn)} }
	req := func(key string, fn checkFn) keyedItem { return keyedItem{key: key, fn: fn, it: required(key, fn)} }
	return []keyedItem{
		opt("minimum_pre_commit_version", checkAnd(checkString, checkMinVersion)),
		req("id", checkString),
		req("name", checkString),
		req("entry", checkString),
		{key: "language", fn: checkOneOf(LanguageNames), it: languageMigration("language", true)},
		opt("alias", checkString),
		opt("files", checkStringRegex),
		opt("exclude", checkStringRegex),
		opt("types", checkArray(checkTypeTag)),
		opt("types_or", checkArray(checkTypeTag)),
		opt("exclude_types", checkArray(checkTypeTag)),
		opt("additional_dependencies", checkArray(checkString)),
		opt("args", checkArray(checkString)),
		opt("always_run", checkBool),
		opt("fail_fast", checkBool),
		opt("pass_filenames", checkBool),
		opt("description", checkString),
		opt("language_version", checkString),
		opt("log_file", checkString),
		opt("require_serial", checkBool),
		{key: "stages", it: stagesMigration("stages")},
		opt("verbose", checkBool),
	}
}

func manifestHookSchema() mapSchema {
	s := mapSchema{name: "Hook", idKey: "id"}
	for _, k := range manifestHookItems() {
		s.items = append(s.items, k.it)
	}
	return s
}

// commonHookWarnings is clientlib._COMMON_HOOK_WARNINGS, which ends every
// hook schema in a config (not a manifest's).
func commonHookWarnings() []item {
	return []item{
		sensibleRegexAtHook("files"),
		sensibleRegexAtHook("exclude"),
		deprecatedStagesWarning("stages", hookWhat),
	}
}

// localHookSchema is LOCAL_HOOK_DICT: the manifest's hook, with the
// common warnings.
func localHookSchema() mapSchema {
	s := manifestHookSchema()
	s.items = append(s.items, commonHookWarnings()...)
	return s
}

func configHookSchema() mapSchema {
	s := mapSchema{name: "Hook", idKey: "id", items: []item{required("id", checkString)}}
	for _, k := range manifestHookItems() {
		if k.key == "id" || k.key == "stages" || k.key == "language" {
			continue
		}
		s.items = append(s.items, optional(k.key, k.fn))
	}
	s.items = append(s.items, stagesMigration("stages"), languageMigration("language", false))
	s.items = append(s.items, commonHookWarnings()...)
	return s
}

var metaHookIDs = []string{"check-hooks-apply", "check-useless-excludes", "identity"}

func metaHookSchema() mapSchema {
	s := mapSchema{name: "Hook", idKey: "id", items: []item{
		required("id", checkString),
		required("id", checkOneOf(metaHookIDs)),
		optional("language", checkOneOf([]string{"unsupported"})),
		notAllowed("entry"),
	}}
	for _, k := range manifestHookItems() {
		switch k.key {
		case "name", "language", "entry":
			s.items = append(s.items, optional(k.key, k.fn))
		default:
			s.items = append(s.items, k.it)
		}
	}
	s.items = append(s.items, commonHookWarnings()...)
	return s
}

func configSchema() mapSchema {
	local := condition{equals: "local"}
	meta := condition{equals: "meta"}
	remote := condition{notIn: []string{"local", "meta"}}
	repo := mapSchema{name: "Repository", idKey: "repo", items: []item{
		required("repo", checkString),
		conditionalRequired("hooks", arrayOf(configHookSchema()), "repo", remote, false),
		conditionalRequired("hooks", arrayOf(localHookSchema()), "repo", local, false),
		conditionalRequired("hooks", arrayOf(metaHookSchema()), "repo", meta, false),
		conditionalRequired("rev", checkString, "repo", remote, true),
		warnMutableRevItem,
		warnAdditionalKeys([]string{"repo", "rev", "hooks"}, warnUnknownKeysRepo),
	}}

	langVersion := mapSchema{name: "DefaultLanguageVersion", items: []item{noAdditionalKeys(LanguageNames)}}
	for _, l := range LanguageNames {
		langVersion.items = append(langVersion.items, optional(l, checkString))
	}

	return mapSchema{name: "Config", items: []item{
		optional("minimum_pre_commit_version", checkAnd(checkString, checkMinVersion)),
		required("repos", arrayOf(repo)),
		optional("default_install_hook_types", checkArray(checkOneOf(hookTypeNames()))),
		optional("default_language_version", langVersion.check),
		stagesMigration("default_stages"),
		deprecatedStagesWarning("default_stages", topWhat),
		optional("files", checkStringRegex),
		optional("exclude", checkStringRegex),
		optional("fail_fast", checkBool),
		warnAdditionalKeys([]string{
			"repos", "default_install_hook_types", "default_language_version",
			"default_stages", "files", "exclude", "fail_fast",
			"minimum_pre_commit_version", "ci",
		}, warnUnknownKeysRoot),
		sensibleRegexAtTop("files"),
		sensibleRegexAtTop("exclude"),
		optional("ci", checkDict),
	}}
}

// validateManifest is MANIFEST_SCHEMA behind clientlib's forward-compat
// loader: a manifest that is a map is a format from pre-commit 5, and is
// reported as check_min_version('5') would, wrapped as the load strategy's
// own failure.
func validateManifest(doc any) error {
	if _, ok := asStringMap(doc); ok {
		inner := checkMinVersion("5")
		var ve *ValidationError
		if errors.As(inner, &ve) {
			return newValidationError("%s", ve.Error())
		}
		return inner
	}
	return arrayOf(manifestHookSchema())(doc)
}

// --- Python's view of decoded YAML ---

func asStringMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out, true
	}
	return nil, false
}

// pyTypeName is type(v).__name__ for what PyYAML would have built.
func pyTypeName(v any) string {
	if ts, ok := v.(pyyaml.PyTimestamp); ok {
		if ts.Date {
			return "date"
		}
		return "datetime"
	}
	switch v.(type) {
	case nil:
		return "NoneType"
	case bool:
		return "bool"
	case int, int64, uint64, *big.Int:
		return "int"
	case float64:
		return "float"
	case string:
		return "str"
	case []byte:
		return "bytes"
	case []any:
		return "list"
	case map[string]any, map[any]any:
		return "dict"
	case time.Time:
		return "datetime"
	}
	return fmt.Sprintf("%T", v)
}

// PyRepr is Python's repr() for the values YAML decodes to.
func PyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return pyStrRepr(x)
	case float64:
		switch {
		case math.IsInf(x, 1):
			return "inf"
		case math.IsInf(x, -1):
			return "-inf"
		case math.IsNaN(x):
			return "nan"
		}
		s := strconv.FormatFloat(x, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eEn") {
			s += ".0"
		}
		return s
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = PyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any, map[any]any:
		m, _ := asStringMap(x)
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = pyStrRepr(k) + ": " + PyRepr(m[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return fmt.Sprint(v)
}

func pyStrRepr(s string) string {
	quote := byte('\'')
	if strings.Contains(s, "'") && !strings.Contains(s, `"`) {
		quote = '"'
	}
	var b strings.Builder
	b.WriteByte(quote)
	for _, r := range s {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == rune(quote):
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte(quote)
	return b.String()
}
