// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/fatal"
)

// The expected messages are Python pre-commit 4.7.0's, measured with
// `pre-commit validate-config` on each document (#126).
func TestLoadConfigValidationErrors(t *testing.T) {
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	for _, tc := range []struct {
		name, doc, want string
	}{
		{"empty", "", "=====> Expected a Config map but got a NoneType"},
		{"list", "- a\n", "=====> Expected a Config map but got a list"},
		{"no repos", "foo: 1\n", "==> At Config()\n=====> Missing required key: repos"},
		{"repos not a list", "repos: 5\n", "==> At Config()\n==> At key: repos\n=====> Expected array but got 'int'"},
		{"repo not a map", "repos: [5]\n", "==> At Config()\n==> At key: repos\n=====> Expected a Repository map but got a int"},
		{"no repo key", "repos:\n- hooks: []\n", "==> At Config()\n==> At key: repos\n==> At Repository(repo=MISSING)\n=====> Missing required key: repo"},
		{
			"remote without rev", "repos:\n- repo: https://x/y\n  hooks: [{id: a}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='https://x/y')\n=====> Missing required key: rev",
		},
		{
			// hooks is checked before rev.
			"remote without hooks or rev", "repos:\n- repo: https://x/y\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='https://x/y')\n=====> Missing required key: hooks",
		},
		{
			"rev on local", "repos:\n- repo: local\n  rev: v1\n  hooks: [{id: a, name: a, entry: b, language: system}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='local')\n" +
				"=====> Expected rev to be absent when repo is any of ('local', 'meta'), found rev: 'v1'",
		},
		{
			"hooks not a list", "repos:\n- repo: local\n  hooks: 5\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='local')\n==> At key: hooks\n=====> Expected array but got 'int'",
		},
		{
			"local hook without name", "repos:\n- repo: local\n  hooks: [{id: a, entry: b, language: system}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='local')\n==> At key: hooks\n==> At Hook(id='a')\n" +
				"=====> Missing required key: name",
		},
		{
			"hook without id", "repos:\n- repo: local\n  hooks: [{name: a, entry: b, language: system}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='local')\n==> At key: hooks\n==> At Hook(id=MISSING)\n" +
				"=====> Missing required key: id",
		},
		{
			"unknown language", "repos:\n- repo: local\n  hooks: [{id: a, name: a, entry: b, language: cobol}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='local')\n==> At key: hooks\n==> At Hook(id='a')\n" +
				"==> At key: language\n=====> Expected one of conda, coursier, dart, docker, docker_image, dotnet, fail, " +
				"golang, haskell, julia, lua, node, perl, pygrep, python, r, ruby, rust, swift, unsupported, " +
				"unsupported_script but got: 'cobol'",
		},
		{
			"hook regex", "repos:\n- repo: local\n  hooks: [{id: a, name: a, entry: b, language: system, files: '('}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='local')\n==> At key: hooks\n==> At Hook(id='a')\n" +
				"==> At key: files\n=====> '(' is not a valid python regex",
		},
		{
			"args not strings", "repos:\n- repo: local\n  hooks: [{id: a, name: a, entry: b, language: system, args: [1]}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='local')\n==> At key: hooks\n==> At Hook(id='a')\n" +
				"==> At key: args\n==> At index 0\n=====> Expected string got int",
		},
		{
			"type tag not a string", "repos:\n- repo: local\n  hooks: [{id: a, name: a, entry: b, language: system, types: [5]}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='local')\n==> At key: hooks\n==> At Hook(id='a')\n" +
				"==> At key: types\n==> At index 0\n=====> Type tag 5 is not recognized.  Try upgrading identify and pre-commit?",
		},
		{
			"unknown meta hook", "repos:\n- repo: meta\n  hooks: [{id: nope}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='meta')\n==> At key: hooks\n==> At Hook(id='nope')\n" +
				"==> At key: id\n=====> Expected one of check-hooks-apply, check-useless-excludes, identity but got: 'nope'",
		},
		{
			"meta entry", "repos:\n- repo: meta\n  hooks: [{id: identity, entry: x}]\n",
			"==> At Config()\n==> At key: repos\n==> At Repository(repo='meta')\n==> At key: hooks\n==> At Hook(id='identity')\n" +
				"=====> 'entry' cannot be overridden",
		},
		{"top regex", "files: '('\nrepos: []\n", "==> At Config()\n==> At key: files\n=====> '(' is not a valid python regex"},
		{
			"unknown stage", "default_stages: [nope]\nrepos: []\n",
			"==> At Config()\n==> At key: default_stages\n==> At index 0\n=====> Expected one of commit-msg, manual, " +
				"post-checkout, post-commit, post-merge, post-rewrite, pre-commit, pre-merge-commit, pre-push, pre-rebase, " +
				"prepare-commit-msg but got: 'nope'",
		},
		{
			"unknown default_language_version key", "default_language_version: {cobol: x}\nrepos: []\n",
			"==> At Config()\n==> At key: default_language_version\n==> At DefaultLanguageVersion()\n" +
				"=====> Additional keys found: cobol.  Only these keys are allowed: conda, coursier, dart, docker, " +
				"docker_image, dotnet, fail, golang, haskell, julia, lua, node, perl, pygrep, python, r, ruby, rust, " +
				"swift, unsupported, unsupported_script",
		},
		{"fail_fast not bool", "fail_fast: 1\nrepos: []\n", "==> At Config()\n==> At key: fail_fast\n=====> Expected bool got int"},
		{"ci not a map", "ci: []\nrepos: []\n", "==> At Config()\n==> At key: ci\n=====> Expected dict got list"},
		{
			"minimum version", "minimum_pre_commit_version: '99.0'\nrepos: []\n",
			"==> At Config()\n==> At key: minimum_pre_commit_version\n=====> pre-commit version 99.0 is required but " +
				"version " + Version + " is installed.  Perhaps run `pip install --upgrade pre-commit`.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "c.yaml")
			if err := os.WriteFile(path, []byte(tc.doc), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadConfig(path)
			var fe *fatal.Error
			if !errors.As(err, &fe) || fe.Type != "InvalidConfigError" {
				t.Fatalf("err = %#v, want an InvalidConfigError", err)
			}
			want := "\n==> File " + path + "\n" + tc.want
			if fe.Msg != want {
				t.Errorf("message:\n%s\nwant:\n%s", fe.Msg, want)
			}
		})
	}
}

func TestLoadConfigAcceptsEmptyRepos(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("repos: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err != nil {
		t.Errorf("repos: [] is a valid config upstream: %v", err)
	}
}

// A minimum version that is not dotted integers is int()'s ValueError,
// which nothing in the load path catches: not an InvalidConfigError.
func TestLoadConfigUnparsableMinimumVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(path, []byte("minimum_pre_commit_version: 'x'\nrepos: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	var pe *pyError
	if !errors.As(err, &pe) || pe.ErrorType() != "ValueError" ||
		pe.Error() != "invalid literal for int() with base 10: 'x'" {
		t.Errorf("err = %#v, want ValueError", err)
	}
}

func TestLoadManifestValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name, doc, want string
	}{
		{"not a list", "5\n", "=====> Expected array but got 'int'"},
		{
			"a map is a pre-commit 5 manifest", "x: 1\n",
			"=====> \n=====> pre-commit version 5 is required but version " + Version +
				" is installed.  Perhaps run `pip install --upgrade pre-commit`.",
		},
		{"missing name", "- id: a\n", "==> At Hook(id='a')\n=====> Missing required key: name"},
		{
			"system is translated", "- {id: a, name: a, entry: a, language: system, always_run: x}\n",
			"==> At Hook(id='a')\n==> At key: always_run\n=====> Expected bool got str",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "m.yaml")
			if err := os.WriteFile(path, []byte(tc.doc), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadManifest(path)
			var fe *fatal.Error
			if !errors.As(err, &fe) || fe.Type != "InvalidManifestError" {
				t.Fatalf("err = %#v, want an InvalidManifestError", err)
			}
			if want := "\n==> File " + path + "\n" + tc.want; fe.Msg != want {
				t.Errorf("message:\n%s\nwant:\n%s", fe.Msg, want)
			}
		})
	}
}

func TestPyRepr(t *testing.T) {
	for _, tc := range []struct {
		v    any
		want string
	}{
		{nil, "None"},
		{true, "True"},
		{5, "5"},
		{1.0, "1.0"},
		{1.5, "1.5"},
		{"a", "'a'"},
		{"it's", `"it's"`},
		{`a'b"c`, `'a\'b"c'`},
		{"a\\b\n", `'a\\b\n'`},
		{[]any{"a", 1}, "['a', 1]"},
		{map[string]any{"b": 1, "a": "x"}, "{'a': 'x', 'b': 1}"},
	} {
		if got := PyRepr(tc.v); got != tc.want {
			t.Errorf("PyRepr(%#v) = %s, want %s", tc.v, got, tc.want)
		}
	}
}

func TestUTF8DecodeError(t *testing.T) {
	for _, tc := range []struct {
		want string
		data []byte
	}{
		{data: []byte("ok\n"), want: ""},
		{data: []byte("a\xffb"), want: "'utf-8' codec can't decode byte 0xff in position 1: invalid start byte"},
		{data: []byte("a\xc3(b"), want: "'utf-8' codec can't decode byte 0xc3 in position 1: invalid continuation byte"},
		{data: []byte("ab\xe2\x82"), want: "'utf-8' codec can't decode bytes in position 2-3: unexpected end of data"},
	} {
		got, ok := utf8DecodeError(tc.data)
		if ok != (tc.want == "") || got != tc.want {
			t.Errorf("utf8DecodeError(%q) = %q, %v; want %q", tc.data, got, ok, tc.want)
		}
	}
}
