// Package pcre provides PCRE-compatible regex matching using regexp2.
// Python's pre-commit uses Python's re module (PCRE), which supports features
// like lookahead, lookbehind, and backreferences that Go's stdlib regexp (RE2)
// does not. This package wraps regexp2 to provide compatible behavior.
package pcre

import (
	"time"

	"github.com/dlclark/regexp2"
)

// DefaultTimeout is the maximum time a regex match can take before being killed.
// This prevents catastrophic backtracking from hanging the process.
const DefaultTimeout = 5 * time.Second

// Compile compiles a PCRE-compatible regex pattern.
func Compile(pattern string) (*regexp2.Regexp, error) {
	re, err := regexp2.Compile(pattern, regexp2.RE2)
	if err != nil {
		// Fall back to full PCRE mode if RE2 mode fails (e.g. backreferences).
		re, err = regexp2.Compile(pattern, regexp2.None)
		if err != nil {
			return nil, err
		}
	}
	re.MatchTimeout = DefaultTimeout
	return re, nil
}

// MustCompile compiles a PCRE-compatible regex pattern and panics on error.
func MustCompile(pattern string) *regexp2.Regexp {
	re, err := Compile(pattern)
	if err != nil {
		panic("pcre: Compile(" + pattern + "): " + err.Error())
	}
	return re
}

// MatchString reports whether the string s contains any match of the pattern.
func MatchString(pattern, s string) (bool, error) {
	re, err := Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(s)
}

// Match reports whether the byte slice b contains any match of the compiled regex.
func Match(re *regexp2.Regexp, s string) bool {
	m, _ := re.MatchString(s)
	return m
}

// FindString returns the first match of the compiled regex in the string.
func FindString(re *regexp2.Regexp, s string) string {
	m, err := re.FindStringMatch(s)
	if err != nil || m == nil {
		return ""
	}
	return m.String()
}
