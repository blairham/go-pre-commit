// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadConfigText(t *testing.T, text string) (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	var err error
	out := captureStdout(t, func() { _, err = LoadConfig(path) })
	return out, err
}

// Upstream warns about keys its schemas do not know, at the point in the
// check order cfgv's WarnAdditionalKeys sits, and joins them as strings.
// Outputs captured from pre-commit 4.7.0 (#187).
func TestUnknownKeyWarnings(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"root", "repos: []\nfoo: 1\nbar: 2\n", "[WARNING] Unexpected key(s) present at root: bar, foo\n"},
		{"repo", "repos:\n- repo: local\n  hooks: []\n  zz: 1\n", "[WARNING] Unexpected key(s) present on local: zz\n"},
		{
			"regex not glob, slashes", "repos:\n- repo: local\n  hooks:\n  - id: a\n    name: a\n    entry: e\n" +
				"    language: system\n    files: \"src/*\"\n    exclude: \"a[\\\\\\\\/]b\"\n",
			"[WARNING] The 'files' field in hook 'a' is a regex, not a glob -- matching '/*' probably isn't what you want here\n" +
				"[WARNING] pre-commit normalizes slashes in the 'exclude' field in hook 'a' to forward slashes, so you can use / instead of [\\\\/]\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := loadConfigText(t, tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if out != tc.want {
				t.Errorf("stdout = %q, want %q", out, tc.want)
			}
		})
	}
}

// The warnings fire as the schema reaches them, so they come out in
// upstream's order and before a later key fails validation.
func TestWarningsInSchemaOrder(t *testing.T) {
	in := "repos:\n- repo: https://x\n  rev: main\n  hooks:\n  - id: a\n    stages: [commit]\n  zz: 1\n" +
		"default_stages: [push]\nfoo: 1\nfiles: \"*.py\"\n"
	out, err := loadConfigText(t, in)
	if err == nil {
		t.Fatal("an invalid files regex loaded")
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		lines = append(lines, l[:min(len(l), 40)])
	}
	want := []string{
		"[WARNING] hook id `a` uses deprecated st",
		"[WARNING] The 'rev' field of repo 'https",
		"[WARNING] Unexpected key(s) present on h",
		"[WARNING] top-level `default_stages` use",
	}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Errorf("warnings:\n%s\nwant prefixes:\n%s", out, strings.Join(want, "\n"))
	}
}

// Keys that are not strings make upstream's sorted() or ", ".join() raise
// TypeError, which its error handler reports as unexpected. Messages
// captured from pre-commit 4.7.0.
func TestNonStringKeysRaiseTypeError(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"int", "repos: []\n1: a\n", "sequence item 0: expected str instance, int found"},
		{"float", "repos: []\n1.5: a\n", "sequence item 0: expected str instance, float found"},
		{"YAML 1.1 bool", "repos: []\non: 1\n", "sequence item 0: expected str instance, bool found"},
		{"None and bool", "repos: []\nnull: a\ntrue: b\n", "'<' not supported between instances of 'bool' and 'NoneType'"},
		{"int and str", "repos: []\n1: a\nfoo: 2\n", "'<' not supported between instances of 'str' and 'int'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadConfigText(t, tc.in)
			var pe *pyError
			if !errors.As(err, &pe) || pe.ErrorType() != "TypeError" || pe.Error() != tc.want {
				t.Errorf("err = %v, want TypeError %q", err, tc.want)
			}
		})
	}
}

// NoAdditionalKeys prints the extra keys with str(), so a non-string key
// that sorts is named, not a crash.
func TestNoAdditionalKeysNonString(t *testing.T) {
	_, err := loadConfigText(t, "repos: []\ndefault_language_version:\n  python: x\n  1: y\n")
	if err == nil ||
		!strings.Contains(err.Error(), "=====> Additional keys found: 1.  Only these keys are allowed: conda,") {
		t.Errorf("err = %v", err)
	}
}
