// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzLoadConfig feeds arbitrary bytes through the same read, decode, validate
// and default path a user's .pre-commit-config.yaml takes. The config is input
// this tool does not control, so the property is that a bad one is an error,
// never a panic.
func FuzzLoadConfig(f *testing.F) {
	for _, seed := range []string{
		"",
		"repos: []\n",
		"repos:\n-   repo: local\n    hooks:\n    -   id: x\n        name: x\n        entry: x\n        language: system\n",
		"repos:\n- repo: https://github.com/pre-commit/pre-commit-hooks\n  rev: main\n  hooks:\n  - id: trailing-whitespace\n",
		"default_stages: [commit, push]\ndefault_language_version: {python: python3}\nrepos: []\n",
		"minimum_pre_commit_version: '99.0.0'\nrepos: []\n",
		"repos: [\n",
	} {
		f.Add([]byte(seed))
	}
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(dir, ".pre-commit-config.yaml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = LoadConfig(path)
	})
}

// FuzzLoadManifest does the same for .pre-commit-hooks.yaml, which comes from
// whatever repository a config points at.
func FuzzLoadManifest(f *testing.F) {
	for _, seed := range []string{
		"",
		"-   id: x\n    name: x\n    entry: x\n    language: system\n",
		"- id: x\n",
		"{}\n",
		"- [\n",
	} {
		f.Add([]byte(seed))
	}
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(dir, ".pre-commit-hooks.yaml")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = LoadManifest(path)
	})
}

// FuzzCheckMinimumVersion checks that the comparison behind
// minimum_pre_commit_version is an ordering: every version satisfies itself,
// and of any two versions at least one satisfies the other. A comparison that
// is neither would refuse a config on both sides of a version.
func FuzzCheckMinimumVersion(f *testing.F) {
	for _, seed := range [][2]string{
		{"4.6.0", "4.6.0"},
		{"4.6.0", "4.6"},
		{"v4.6.0-1-gabc1234-dirty", "4.6.1"},
		{"1.10.0", "1.9.9"},
		{"", "0"},
		{"x.y", "1..2"},
	} {
		f.Add(seed[0], seed[1])
	}
	orig := Version
	f.Cleanup(func() { Version = orig })
	f.Fuzz(func(t *testing.T, a, b string) {
		Version = a
		if !CheckMinimumVersion(a) {
			t.Fatalf("version %q does not satisfy itself", a)
		}
		aMeetsB := CheckMinimumVersion(b)
		Version = b
		bMeetsA := CheckMinimumVersion(a)
		if !aMeetsB && !bMeetsA {
			t.Fatalf("neither %q nor %q satisfies the other", a, b)
		}
	})
}
