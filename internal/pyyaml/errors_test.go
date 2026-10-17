// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package pyyaml

import "testing"

// TestLoadErrorsMatchPyYAML compares Load's error with str() of what
// upstream's yaml_load raised for the same text under pre-commit 4.7.0 with
// PyYAML 6.0.3's libyaml loader on Python 3.13, the parity suite's
// reference. The inputs are configs broken the ways people break them.
func TestLoadErrorsMatchPyYAML(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			name: "e186/anchor_nothing",
			in:   "repos: &\n",
			want: "while scanning an anchor\n  in \"<unicode string>\", line 1, column 8\ndid not find expected alphabetic or numeric character\n  in \"<unicode string>\", line 1, column 9",
		},
		{name: "e186/bad_date", in: "repos: []\nx: 2020-13-01\n", want: "month must be in 1..12"},
		{
			name: "e186/bad_escape",
			in:   "repos: \"\\q\"\n",
			want: "while parsing a quoted scalar\n  in \"<unicode string>\", line 1, column 8\nfound unknown escape character\n  in \"<unicode string>\", line 1, column 9",
		},
		{
			name: "e186/bad_indent",
			in:   "repos:\n  - a\n - b\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 1\ndid not find expected key\n  in \"<unicode string>\", line 3, column 2",
		},
		{
			name: "e186/block_scalar_bad",
			in:   "repos: |x\n  a\n",
			want: "while scanning a block scalar\n  in \"<unicode string>\", line 1, column 8\ndid not find expected comment or line break\n  in \"<unicode string>\", line 1, column 9",
		},
		{
			name: "e186/block_seq_in_map",
			in:   "repos: []\n- a\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 1\ndid not find expected key\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186/colon_in_plain",
			in:   "repos: a: b\n",
			want: "mapping values are not allowed in this context\n  in \"<unicode string>\", line 1, column 9",
		},
		{
			name: "e186/directive",
			in:   "%FOO\n---\nrepos: []\n",
			want: "while scanning a directive\n  in \"<unicode string>\", line 1, column 1\nfound unknown directive name\n  in \"<unicode string>\", line 1, column 5",
		},
		{
			name: "e186/dquote_unclosed",
			in:   "repos: \"abc\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 1, column 8\nfound unexpected end of stream\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186/dup_anchor",
			in:   "a: &x 1\nb: &x 2\nrepos: []\n",
			want: "found duplicate anchor; first occurrence\n  in \"<unicode string>\", line 1, column 4\nsecond occurrence\n  in \"<unicode string>\", line 2, column 4",
		},
		{
			name: "e186/flow_unclosed",
			in:   "repos: [\n",
			want: "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186/m2",
			in:   "# c\n--- !!map\nrepos: []\n--- &a\nx\n",
			want: "expected a single document in the stream\n  in \"<unicode string>\", line 2, column 5\nbut found another document\n  in \"<unicode string>\", line 4, column 1",
		},
		{
			name: "e186/map_unclosed",
			in:   "repos: {a: 1\n",
			want: "while parsing a flow mapping\n  in \"<unicode string>\", line 1, column 8\ndid not find expected ',' or '}'\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186/mapping_values_not_allowed",
			in:   "a: b: c\n",
			want: "mapping values are not allowed in this context\n  in \"<unicode string>\", line 1, column 5",
		},
		{
			name: "e186/multi_doc",
			in:   "repos: []\n---\nrepos: []\n",
			want: "expected a single document in the stream\n  in \"<unicode string>\", line 1, column 1\nbut found another document\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186/nested_flow",
			in:   "repos: [a, [b, c]\n",
			want: "while parsing a flow sequence\n  in \"<unicode string>\", line 1, column 8\ndid not find expected ',' or ']'\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186/quote_unclosed",
			in:   "repos: 'abc\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 1, column 8\nfound unexpected end of stream\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186/tab",
			in:   "repos:\n\t- a\n",
			want: "while scanning for the next token\nfound character that cannot start any token\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186/tag_bad",
			in:   "repos: !<x\n",
			want: "while scanning a tag\n  in \"<unicode string>\", line 1, column 8\ndid not find the expected '>'\n  in \"<unicode string>\", line 1, column 11",
		},
		{
			name: "e186/undef_alias",
			in:   "repos: *x\n",
			want: "found undefined alias\n  in \"<unicode string>\", line 1, column 8",
		},
		{
			name: "e186/unexpected_end_quote",
			in:   "a: 'b'c\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 1\ndid not find expected key\n  in \"<unicode string>\", line 1, column 7",
		},
		{
			name: "e186/unhashable",
			in:   "? [a]\n: b\nrepos: []\n",
			want: "while constructing a mapping\n  in \"<unicode string>\", line 1, column 1\nfound unhashable key\n  in \"<unicode string>\", line 1, column 3",
		},
		{
			name: "e186/unicode_col",
			in:   "é: [\n",
			want: "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186b/alias_bad",
			in:   "a: *\n",
			want: "while scanning an alias\n  in \"<unicode string>\", line 1, column 4\ndid not find expected alphabetic or numeric character\n  in \"<unicode string>\", line 1, column 5",
		},
		{
			name: "e186b/anchor_bad2",
			in:   "a: &a!b c\n",
			want: "while scanning an anchor\n  in \"<unicode string>\", line 1, column 4\ndid not find expected alphabetic or numeric character\n  in \"<unicode string>\", line 1, column 6",
		},
		{
			name: "e186b/block_indent0",
			in:   "a: |0\n  x\n",
			want: "while scanning a block scalar\n  in \"<unicode string>\", line 1, column 4\nfound an indentation indicator equal to 0\n  in \"<unicode string>\", line 1, column 5",
		},
		{
			name: "e186b/comment_quote",
			in:   "# it's\nrepos: [\n",
			want: "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186b/crlf_flow",
			in:   "repos: [\n",
			want: "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186b/deep_flow",
			in:   "repos:\n- repo: local\n  hooks: [{id: x, name: 'a]b'\n",
			want: "while parsing a flow mapping\n  in \"<unicode string>\", line 3, column 11\ndid not find expected ',' or '}'\n  in \"<unicode string>\", line 4, column 1",
		},
		{
			name: "e186b/directive_yaml_bad",
			in:   "%YAML 2.0\n---\na: 1\n",
			want: "found incompatible YAML document\n  in \"<unicode string>\", line 1, column 1",
		},
		{
			name: "e186b/esc_x",
			in:   "a: \"\\xZZ\"\n",
			want: "while parsing a quoted scalar\n  in \"<unicode string>\", line 1, column 4\ndid not find expected hexdecimal number\n  in \"<unicode string>\", line 1, column 7",
		},
		{
			name: "e186b/flow_comment",
			in:   "repos: [ # c\n  a,\n",
			want: "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186b/key_quote_unclosed",
			in:   "'a: b\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 1, column 1\nfound unexpected end of stream\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186b/map_in_seq_bad",
			in:   "- a: 1\n  b: 2\n c: 3\n",
			want: "while parsing a block collection\n  in \"<unicode string>\", line 1, column 1\ndid not find expected '-' indicator\n  in \"<unicode string>\", line 3, column 2",
		},
		{
			name: "e186b/nested_indent",
			in:   "repos:\n- repo: local\n  hooks:\n   - id: x\n  - id: y\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 2, column 3\ndid not find expected key\n  in \"<unicode string>\", line 5, column 3",
		},
		{
			name: "e186b/nested_indent2",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: y\n   entry: z\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 2, column 3\ndid not find expected key\n  in \"<unicode string>\", line 6, column 4",
		},
		{
			name: "e186b/no_trailing_newline_flow",
			in:   "repos: [",
			want: "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186b/no_trailing_newline_quote",
			in:   "repos: 'x",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 1, column 8\nfound unexpected end of stream\n  in \"<unicode string>\", line 1, column 10",
		},
		{
			name: "e186b/quote_brackets",
			in:   "a: '[b'\nrepos: [\n",
			want: "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186b/quote_later",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: \"x\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 4, column 9\nfound unexpected end of stream\n  in \"<unicode string>\", line 5, column 1",
		},
		{
			name: "e186b/seq_at_map_level_deep",
			in:   "a:\n  b: 1\n  - c\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 2, column 3\ndid not find expected key\n  in \"<unicode string>\", line 3, column 3",
		},
		{
			name: "e186b/tab_mid",
			in:   "repos:\n  - a\n\t- b\n",
			want: "while scanning a plain scalar\n  in \"<unicode string>\", line 2, column 5\nfound a tab character that violates indentation\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186b/two_values",
			in:   "- a: b: c\n",
			want: "mapping values are not allowed in this context\n  in \"<unicode string>\", line 1, column 7",
		},
		{
			name: "e186b/unclosed_single_multiline",
			in:   "a: 'b\n  c\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 1, column 4\nfound unexpected end of stream\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186b/unicode_quote",
			in:   "é: 'x\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 1, column 4\nfound unexpected end of stream\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186b/value_after_flow",
			in:   "a: [b] c\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 1\ndid not find expected key\n  in \"<unicode string>\", line 1, column 8",
		},
		{
			name: "e186b/value_after_quote_later",
			in:   "x: 1\na: \"b\" c\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 1\ndid not find expected key\n  in \"<unicode string>\", line 2, column 8",
		},
		{
			name: "e186c/c1",
			in:   "repos:\n  - repo: local\n    hooks:\n      - id: x\n     - id: y\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 2, column 5\ndid not find expected key\n  in \"<unicode string>\", line 5, column 6",
		},
		{
			name: "e186c/c10",
			in:   "repos: *missing\n",
			want: "found undefined alias\n  in \"<unicode string>\", line 1, column 8",
		},
		{
			name: "e186c/c11",
			in:   "repos: []\n...\nfoo\n",
			want: "did not find expected <document start>\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186c/c12",
			in:   "repos: []\nci: {autofix: true\n",
			want: "while parsing a flow mapping\n  in \"<unicode string>\", line 2, column 5\ndid not find expected ',' or '}'\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186c/c13",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n  entry: y\n   name: z\n",
			want: "mapping values are not allowed in this context\n  in \"<unicode string>\", line 6, column 8",
		},
		{
			name: "e186c/c14",
			in:   "- id: a\n  name: b\n  - c\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 3\ndid not find expected key\n  in \"<unicode string>\", line 3, column 3",
		},
		{
			name: "e186c/c15",
			in:   "repos: [a,\n  b\n",
			want: "while parsing a flow sequence\n  in \"<unicode string>\", line 1, column 8\ndid not find expected ',' or ']'\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186c/c16",
			in:   "x: 'it''s\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 1, column 4\nfound unexpected end of stream\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186c/c17",
			in:   "x: \"\\u12G4\"\n",
			want: "while parsing a quoted scalar\n  in \"<unicode string>\", line 1, column 4\ndid not find expected hexdecimal number\n  in \"<unicode string>\", line 1, column 7",
		},
		{
			name: "e186c/c18",
			in:   "repos: []\nkey: value: other\n",
			want: "mapping values are not allowed in this context\n  in \"<unicode string>\", line 2, column 11",
		},
		{
			name: "e186c/c19",
			in:   "repos: []\n  extra: 1\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 1\ndid not find expected key\n  in \"<unicode string>\", line 2, column 3",
		},
		{
			name: "e186c/c2",
			in:   "default_stages: [commit\nrepos: []\n",
			want: "while parsing a flow sequence\n  in \"<unicode string>\", line 1, column 17\ndid not find expected ',' or ']'\n  in \"<unicode string>\", line 2, column 6",
		},
		{
			name: "e186c/c20",
			in:   "? a\n: b\n- c\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 1\ndid not find expected key\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186c/c3",
			in:   "repos:\n- repo: \"https://x\n  rev: v1\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 2, column 9\nfound unexpected end of stream\n  in \"<unicode string>\", line 4, column 1",
		},
		{
			name: "e186c/c4",
			in:   "repos: []\nfiles: \"^(a|b\\.py$\"\nexclude: '\n",
			want: "while parsing a quoted scalar\n  in \"<unicode string>\", line 2, column 8\nfound unknown escape character\n  in \"<unicode string>\", line 2, column 14",
		},
		{
			name: "e186c/c5",
			in:   "repos:\n- repo: local\n  hooks:\n  - {id: x, name: y, entry: z, language: system\n",
			want: "while parsing a flow mapping\n  in \"<unicode string>\", line 4, column 5\ndid not find expected ',' or '}'\n  in \"<unicode string>\", line 5, column 1",
		},
		{
			name: "e186c/c6",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    args: [--a, \"b\"]x\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 4, column 5\ndid not find expected key\n  in \"<unicode string>\", line 5, column 21",
		},
		{
			name: "e186c/c7",
			in:   "repos:\n\t- repo: local\n",
			want: "while scanning for the next token\nfound character that cannot start any token\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186c/c8",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: \"a\\qb\"\n",
			want: "while parsing a quoted scalar\n  in \"<unicode string>\", line 5, column 11\nfound unknown escape character\n  in \"<unicode string>\", line 5, column 13",
		},
		{
			name: "e186c/c9",
			in:   "a: 1\nb: &x\nc: &x 2\nrepos: []\n",
			want: "found duplicate anchor; first occurrence\n  in \"<unicode string>\", line 2, column 4\nsecond occurrence\n  in \"<unicode string>\", line 3, column 4",
		},
		{
			name: "e186d/d1",
			in:   "repos:\n- repo: https://github.com/a/b\n  rev: v1\n  hooks:\n  - id: x\n    args: ['--foo\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 6, column 12\nfound unexpected end of stream\n  in \"<unicode string>\", line 7, column 1",
		},
		{
			name: "e186d/d10",
			in:   "repos:\n- repo: local\n  hooks: &h\n  - id: x\n- repo: local\n  hooks: *h\n- repo: local\n  hooks: *hh\n",
			want: "found undefined alias\n  in \"<unicode string>\", line 8, column 10",
		},
		{
			name: "e186d/d11",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    entry: \"a\\tb\\\n  c\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 5, column 12\nfound unexpected end of stream\n  in \"<unicode string>\", line 7, column 1",
		},
		{
			name: "e186d/d12",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: |\n      a\n    b\n",
			want: "while scanning a simple key\n  in \"<unicode string>\", line 7, column 5\ncould not find expected ':'\n  in \"<unicode string>\", line 8, column 1",
		},
		{
			name: "e186d/d13",
			in:   "repos: [{repo: local, hooks: [{id: x, name: y}]},\n",
			want: "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 2, column 1",
		},
		{
			name: "e186d/d14",
			in:   "default_language_version:\n  python: python3\n python: 3\nrepos: []\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 1, column 1\ndid not find expected key\n  in \"<unicode string>\", line 3, column 2",
		},
		{
			name: "e186d/d15",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    language: system\n    entry: echo\n    name: x\n    verbose: true false: 1\n",
			want: "mapping values are not allowed in this context\n  in \"<unicode string>\", line 8, column 24",
		},
		{
			name: "e186d/d2",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: x\n    entry: x\n    language: system\n  - id: y\n   name: y\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 2, column 3\ndid not find expected key\n  in \"<unicode string>\", line 9, column 4",
		},
		{
			name: "e186d/d3",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    files: ^(a|b)$\n    exclude: >\n      foo\n     bar\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 4, column 5\ndid not find expected key\n  in \"<unicode string>\", line 8, column 6",
		},
		{
			name: "e186d/d4",
			in:   "minimum_pre_commit_version: '2.0'\nrepos:\n  - repo: local\n    hooks:\n    - id: x\n  - repo: meta\n   hooks: []\n",
			want: "while parsing a block collection\n  in \"<unicode string>\", line 3, column 3\ndid not find expected '-' indicator\n  in \"<unicode string>\", line 7, column 4",
		},
		{
			name: "e186d/d5",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    additional_dependencies: [a, b\n  - id: y\n",
			want: "while parsing a flow sequence\n  in \"<unicode string>\", line 5, column 30\ndid not find expected ',' or ']'\n  in \"<unicode string>\", line 6, column 7",
		},
		{
			name: "e186d/d6",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: x # comment: here\n    entry: a: b\n",
			want: "mapping values are not allowed in this context\n  in \"<unicode string>\", line 6, column 13",
		},
		{
			name: "e186d/d7",
			in:   "repos:\n- {repo: local, hooks: [{id: x}]\n- repo: meta\n",
			want: "while parsing a flow mapping\n  in \"<unicode string>\", line 2, column 3\ndid not find expected ',' or '}'\n  in \"<unicode string>\", line 3, column 1",
		},
		{
			name: "e186d/d8",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    stages: [pre-commit, \"pre-push]\n",
			want: "while scanning a quoted scalar\n  in \"<unicode string>\", line 5, column 26\nfound unexpected end of stream\n  in \"<unicode string>\", line 6, column 1",
		},
		{
			name: "e186d/d9",
			in:   "# header\n\nrepos:\n- repo: local\n\thooks: []\n",
			want: "while scanning a plain scalar\n  in \"<unicode string>\", line 4, column 9\nfound a tab character that violates indentation\n  in \"<unicode string>\", line 5, column 1",
		},
		{
			name: "e186e/e1",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: 'a' 'b'\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 4, column 5\ndid not find expected key\n  in \"<unicode string>\", line 5, column 15",
		},
		{
			name: "e186e/e10",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n\tname: y\n",
			want: "while scanning a plain scalar\n  in \"<unicode string>\", line 4, column 9\nfound a tab character that violates indentation\n  in \"<unicode string>\", line 5, column 1",
		},
		{
			name: "e186e/e11",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: x\n    entry: x\n    language: system\n    types: [file, python\n    stages: [pre-commit]\n",
			want: "while parsing a flow sequence\n  in \"<unicode string>\", line 8, column 12\ndid not find expected ',' or ']'\n  in \"<unicode string>\", line 9, column 11",
		},
		{
			name: "e186e/e12",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    name: \"x\"y\n",
			want: "while parsing a block mapping\n  in \"<unicode string>\", line 4, column 5\ndid not find expected key\n  in \"<unicode string>\", line 5, column 14",
		},
		{
			name: "e186e/e2",
			in:   "repos:\n  - repo: local\n    hooks:\n    - id: x\n   - id: y\n",
			want: "while parsing a block collection\n  in \"<unicode string>\", line 2, column 3\ndid not find expected '-' indicator\n  in \"<unicode string>\", line 5, column 4",
		},
		{
			name: "e186e/e3",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    args: [\"a\" \"b\"]\n",
			want: "while parsing a flow sequence\n  in \"<unicode string>\", line 5, column 11\ndid not find expected ',' or ']'\n  in \"<unicode string>\", line 5, column 16",
		},
		{
			name: "e186e/e4",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    entry: echo\n    files: \"\\.py$\n",
			want: "while parsing a quoted scalar\n  in \"<unicode string>\", line 6, column 12\nfound unknown escape character\n  in \"<unicode string>\", line 6, column 13",
		},
		{
			name: "e186e/e6",
			in:   "repos:\n- repo: local\n  hooks:\n    - id: x\n      name: x\n      language: system\n    entry: x\n",
			want: "while parsing a block collection\n  in \"<unicode string>\", line 4, column 5\ndid not find expected '-' indicator\n  in \"<unicode string>\", line 7, column 5",
		},
		{
			name: "e186e/e7",
			in:   "repos:\n- repo: local\n  hooks: [\n    {id: x},\n    {id: y\n  ]\n",
			want: "while parsing a flow mapping\n  in \"<unicode string>\", line 5, column 5\ndid not find expected ',' or '}'\n  in \"<unicode string>\", line 6, column 3",
		},
		{
			name: "e186e/e8",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: x\n    exclude: |\n      a\n  oops\n",
			want: "while scanning a simple key\n  in \"<unicode string>\", line 7, column 3\ncould not find expected ':'\n  in \"<unicode string>\", line 8, column 1",
		},
		{
			name: "e186e/e9",
			in:   "exclude: '^vendor/'\nrepos:\n- repo: local\n  hooks:\n  - &h\n    id: x\n  - *h\n  - &h\n    id: y\n",
			want: "found duplicate anchor; first occurrence\n  in \"<unicode string>\", line 5, column 5\nsecond occurrence\n  in \"<unicode string>\", line 8, column 5",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(tc.in)
			if err == nil {
				t.Fatalf("Load(%q) succeeded; upstream raised %q", tc.in, tc.want)
			}
			if err.Error() != tc.want {
				t.Errorf("Load(%q)\n got: %q\nwant: %q", tc.in, err.Error(), tc.want)
			}
		})
	}
}

func TestMarkedErrorHidesARedundantContextMark(t *testing.T) {
	m := &Mark{Line: 1}
	e := &MarkedError{
		Context:     "while parsing a flow node",
		ContextMark: m,
		Problem:     "did not find expected node content",
		ProblemMark: &Mark{Line: 1},
	}
	want := "while parsing a flow node\ndid not find expected node content\n  in \"<unicode string>\", line 2, column 1"
	if e.Error() != want {
		t.Errorf("got %q, want %q", e.Error(), want)
	}
}
