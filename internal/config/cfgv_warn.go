// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/output"
)

// The warnings upstream's schemas emit while they validate. Each is a schema
// item, so it fires at the point in the check order upstream's does: before
// a later key fails validation, and interleaved with the other warnings as
// upstream's are.

// warnAdditionalKeys is cfgv.WarnAdditionalKeys:
// sorted(set(dct) - set(keys)), handed to callback, which joins them.
func warnAdditionalKeys(keys []string, callback func(m map[string]any, extra string)) item {
	return func(m map[string]any, raw any) error {
		extra, err := pySortedExtraKeys(raw, keys)
		if err != nil || len(extra) == 0 {
			return err
		}
		joined, err := pyJoinStrs(extra)
		if err != nil {
			return err
		}
		callback(m, joined)
		return nil
	}
}

// warnUnknownKeysRoot is clientlib.warn_unknown_keys_root.
func warnUnknownKeysRoot(_ map[string]any, extra string) {
	output.Warn("Unexpected key(s) present at root: %s", extra)
}

// warnUnknownKeysRepo is clientlib.warn_unknown_keys_repo.
func warnUnknownKeysRepo(m map[string]any, extra string) {
	output.Warn("Unexpected key(s) present on %s: %s", pyStr(m["repo"]), extra)
}

// deprecatedStagesWarning is clientlib.DeprecatedStagesWarning (and, with
// what "top-level `default_stages`", DeprecatedDefaultStagesWarning).
func deprecatedStagesWarning(key string, what func(m map[string]any) string) item {
	return func(m map[string]any, _ any) error {
		v, ok := m[key]
		if !ok {
			return nil
		}
		list, ok := v.([]any)
		if !ok {
			return checkArray(checkAny)(v)
		}
		var legacy []string
		for _, s := range list {
			if str, ok := s.(string); ok {
				if _, ok := legacyStages[Stage(str)]; ok {
					legacy = append(legacy, str)
				}
			}
		}
		if len(legacy) > 0 {
			output.Warn(
				"%s uses deprecated stage names (%s) which will be removed in a "+
					"future version.  "+
					"run: `pre-commit migrate-config` to automatically fix this.",
				what(m), strings.Join(legacy, ", "),
			)
		}
		return nil
	}
}

func hookWhat(m map[string]any) string { return "hook id `" + pyStr(m["id"]) + "`" }

func topWhat(map[string]any) string { return "top-level `default_stages`" }

// warnMutableRevItem is the warning half of clientlib.WarnMutableRev; the
// check half is the conditionalRequired item before it.
func warnMutableRevItem(m map[string]any, _ any) error {
	if rev, ok := m["rev"].(string); ok {
		WarnMutableRev(pyStr(m["repo"]), rev)
	}
	return nil
}

var fwdSlashRegexes = []string{`[\\/]`, `[\/]`, `[/\\]`}

// sensibleRegexAtHook is the warning half of
// clientlib.OptionalSensibleRegexAtHook.
func sensibleRegexAtHook(key string) item {
	return func(m map[string]any, _ any) error {
		v, ok := m[key].(string)
		if !ok {
			return nil // absent: dct.get(key, '') holds nothing to warn about
		}
		id := PyRepr(m["id"])
		if strings.Contains(v, "/*") {
			output.Warn("The %s field in hook %s is a regex, not a glob -- "+
				"matching '/*' probably isn't what you want here", PyRepr(key), id)
		}
		for _, re := range fwdSlashRegexes {
			if strings.Contains(v, re) {
				output.Warn("pre-commit normalizes slashes in the %s field in hook %s "+
					"to forward slashes, so you can use / instead of %s", PyRepr(key), id, re)
			}
		}
		return nil
	}
}

// sensibleRegexAtTop is the warning half of
// clientlib.OptionalSensibleRegexAtTop.
func sensibleRegexAtTop(key string) item {
	return func(m map[string]any, _ any) error {
		v, ok := m[key].(string)
		if !ok {
			return nil // absent: dct.get(key, '') holds nothing to warn about
		}
		if strings.Contains(v, "/*") {
			output.Warn("The top-level %s field is a regex, not a glob -- "+
				"matching '/*' probably isn't what you want here", PyRepr(key))
		}
		for _, re := range fwdSlashRegexes {
			if strings.Contains(v, re) {
				output.Warn("pre-commit normalizes the slashes in the top-level %s "+
					"field to forward slashes, so you can use / instead of %s", PyRepr(key), re)
			}
		}
		return nil
	}
}

// --- Python's view of mapping keys ---

// pyStr is Python's str() of a decoded YAML value.
func pyStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return PyRepr(v)
}

func pyTypeOfKey(v any) string {
	if _, ok := v.(int); ok {
		return "int"
	}
	return pyTypeName(v)
}

// pySortedExtraKeys is sorted(set(dct) - set(keys)) over raw's keys, as the
// Python values PyYAML would have built. Keys of types Python cannot order
// against each other raise its TypeError.
func pySortedExtraKeys(raw any, keys []string) ([]any, error) {
	var extra []any
	switch m := raw.(type) {
	case map[string]any:
		for k := range m {
			if !slices.Contains(keys, k) {
				extra = append(extra, k)
			}
		}
	case map[any]any:
		for k := range m {
			if s, ok := k.(string); ok && slices.Contains(keys, s) {
				continue
			}
			extra = append(extra, k)
		}
	}
	return pySorted(setOrder(extra))
}

// setOrder approximates the order a CPython set iterates these keys in:
// the non-string keys by their hash's slot in a small table, then the
// strings, whose hashes are randomized per process. Only which two keys are
// compared first depends on it, and so only the wording of a TypeError.
func setOrder(keys []any) []any {
	out := slices.Clone(keys)
	sort.SliceStable(out, func(i, j int) bool {
		hi, iNum := pyHash(out[i])
		hj, jNum := pyHash(out[j])
		switch {
		case iNum && jNum:
			if hi&7 != hj&7 {
				return hi&7 < hj&7
			}
			return hi < hj
		case iNum != jNum:
			return iNum
		}
		return pyStr(out[i]) < pyStr(out[j])
	})
	return out
}

// pyHash is CPython's hash for the keys whose hash is not randomized.
func pyHash(v any) (uint64, bool) {
	switch x := v.(type) {
	case nil:
		return 0xFCA86420, true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case int:
		return uint64(x), true
	case float64:
		if x == math.Trunc(x) && !math.IsInf(x, 0) {
			return uint64(int64(x)), true
		}
		return math.Float64bits(x), true
	}
	return 0, false
}

// pyLess is Python's a < b over keys, or the TypeError it raises.
func pyLess(a, b any) (bool, error) {
	an, aNum := pyNumber(a)
	bn, bNum := pyNumber(b)
	if aNum && bNum {
		return an < bn, nil
	}
	as, aStr := a.(string)
	bs, bStr := b.(string)
	if aStr && bStr {
		return as < bs, nil
	}
	return false, &pyError{typ: "TypeError", msg: fmt.Sprintf("'<' not supported between instances of %s and %s",
		PyRepr(pyTypeOfKey(a)), PyRepr(pyTypeOfKey(b)))}
}

func pyNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case int:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

// pySorted is sorted() for the handful of keys a mapping has: CPython's
// binary insertion sort, comparing in its order so the same pair raises.
func pySorted(keys []any) ([]any, error) {
	a := slices.Clone(keys)
	for i := 1; i < len(a); i++ {
		pivot := a[i]
		lo, hi := 0, i
		for lo < hi {
			mid := (lo + hi) / 2
			less, err := pyLess(pivot, a[mid])
			if err != nil {
				return nil, err
			}
			if less {
				hi = mid
			} else {
				lo = mid + 1
			}
		}
		copy(a[lo+1:i+1], a[lo:i])
		a[lo] = pivot
	}
	return a, nil
}

// pyJoinStrs is ", ".join(keys), which refuses anything but a str.
func pyJoinStrs(keys []any) (string, error) {
	parts := make([]string, len(keys))
	for i, k := range keys {
		s, ok := k.(string)
		if !ok {
			return "", &pyError{typ: "TypeError", msg: "sequence item " + strconv.Itoa(i) +
				": expected str instance, " + pyTypeOfKey(k) + " found"}
		}
		parts[i] = s
	}
	return strings.Join(parts, ", "), nil
}
