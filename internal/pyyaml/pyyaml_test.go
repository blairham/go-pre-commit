// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package pyyaml

import "testing"

// TestDumpMatchesPyYAML compares Dump(Load(in)) with what upstream's
// yaml_dump(yaml_load(in)) wrote for the same input under pre-commit 4.7.0
// (PyYAML 6.0.3); want is that output verbatim.
func TestDumpMatchesPyYAML(t *testing.T) {
	cases := []struct {
		name, in, want string
		err            bool
	}{
		{
			name: "anchmap",
			in:   "a: &m {x: [1, 2]}\nb: *m\nc: *m\nd: &s [a]\ne: [*s, *m]\n",
			want: "a: &id001\n    x:\n    - 1\n    - 2\nb: *id001\nc: *id001\nd: &id002\n- a\ne:\n- *id002\n- *id001\n",
			err:  false,
		},
		{
			name: "block",
			in:   "a: |\n  line1\n  line2\nb: >\n  folded text\n  here\n\nc: |-\n  no newline\nd: \"tab\\tx\"\ne: \"bell\\a\"\nf: \"nel\\N\"\ng: \"\\U0001F600\"\nh: \"\\u4e2d\\u6587\"\n",
			want: "a: 'line1\n\n    line2\n\n    '\nb: 'folded text here\n\n    '\nc: no newline\nd: \"tab\\tx\"\ne: \"bell\\a\"\nf: \"nel\\N\"\ng: \"\\U0001F600\"\nh: \"\\u4E2D\\u6587\"\n",
			err:  false,
		},
		{name: "comment_only", in: "# just a comment\n", want: "null\n...\n", err: false},
		{
			name: "deep",
			in:   "repos:\n- repo: x\n  hooks:\n  - id: y\n    args: [--a, {k: [1, {z: []}]}]\n    stages: [pre-commit]\n",
			want: "repos:\n-   repo: x\n    hooks:\n    -   id: y\n        args:\n        - --a\n        -   k:\n            - 1\n            -   z: []\n        stages:\n        - pre-commit\n",
			err:  false,
		},
		{
			name: "dt",
			in:   "a: 2001-12-14t21:59:43.10-05:00\nb: 2001-12-14 21:59:43.10\nc: 2002-12-14\nd: 2002-1-4 1:02:03Z\ne: 2002-01-04 01:02:03.0000001 +5\nf: 2002-01-04T01:02:03 -00:30\n",
			want: "a: 2001-12-14 21:59:43.100000-05:00\nb: 2001-12-14 21:59:43.100000\nc: 2002-12-14\nd: 2002-01-04 01:02:03+00:00\ne: 2002-01-04 01:02:03+05:00\nf: 2002-01-04 01:02:03-00:30\n",
			err:  false,
		},
		{name: "empty", in: "", want: "null\n...\n", err: false},
		{name: "emptyvals", in: "a:\nb: []\nc: {}\nd: ''\n", want: "a: null\nb: []\nc: {}\nd: ''\n", err: false},
		{
			name: "flow1",
			in:   "repos: [{repo: https://github.com/pre-commit/pre-commit-hooks, rev: v4.0.0, hooks: [{id: trailing-whitespace}, {id: end-of-file-fixer, args: ['--fix=lf', \"x y\"]}]}]\n",
			want: "repos:\n-   repo: https://github.com/pre-commit/pre-commit-hooks\n    rev: v4.0.0\n    hooks:\n    -   id: trailing-whitespace\n    -   id: end-of-file-fixer\n        args:\n        - --fix=lf\n        - x y\n",
			err:  false,
		},
		{name: "keys", in: "1: a\ntrue: b\n? [a]\n: c\n'': d\nnull: e\n", want: "", err: true},
		{
			name: "long",
			in:   "args: ['word0 word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12 word13 word14 word15 word16 word17 word18 word19 word20 word21 word22 word23 word24 word25 word26 word27 word28 word29 word30 word31 word32 word33 word34 word35 word36 word37 word38 word39']\nb: \"w0 w1 w2 w3 w4 w5 w6 w7 w8 w9 w10 w11 w12 w13 w14 w15 w16 w17 w18 w19 w20 w21 w22 w23 w24 w25 w26 w27 w28 w29 w30 w31 w32 w33 w34 w35 w36 w37 w38 w39\\n\"\nc: \"é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é é \"\nd: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx y'\n",
			want: "args:\n- word0 word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12\n    word13 word14 word15 word16 word17 word18 word19 word20 word21 word22 word23 word24\n    word25 word26 word27 word28 word29 word30 word31 word32 word33 word34 word35 word36\n    word37 word38 word39\nb: 'w0 w1 w2 w3 w4 w5 w6 w7 w8 w9 w10 w11 w12 w13 w14 w15 w16 w17 w18 w19 w20 w21\n    w22 w23 w24 w25 w26 w27 w28 w29 w30 w31 w32 w33 w34 w35 w36 w37 w38 w39\n\n    '\nc: \"\\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9\\\n    \\ \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9\\\n    \\ \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9 \\xE9\\\n    \\ \\xE9 \\xE9 \"\nd: xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n    y\n",
			err:  false,
		},
		{
			name: "longkey",
			in:   "kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk: v\n",
			want: "? kkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkkk\n: v\n",
			err:  false,
		},
		{
			name: "merge",
			in:   "base: &b {x: 1, y: 2}\nother:\n  <<: *b\n  y: 3\nlist: &l [1, 2]\nref: *l\n",
			want: "base:\n    x: 1\n    y: 2\nother:\n    x: 1\n    y: 3\nlist: &id001\n- 1\n- 2\nref: *id001\n",
			err:  false,
		},
		{name: "multiline_key", in: "\"a\\nb\": c\n", want: "? 'a\n\n    b'\n: c\n", err: false},
		{
			name: "nested",
			in:   "repos:\n- repo: local\n  hooks:\n  - id: a\n    name: a\n    entry: echo\n    language: system\n    files: ^(a|b)\\.py$\n    exclude: '(?x)^(\n      foo|\n      bar\n    )$'\n    additional_dependencies: [a, b]\n- repo: meta\n  hooks: [{id: check-useless-excludes}]\n",
			want: "repos:\n-   repo: local\n    hooks:\n    -   id: a\n        name: a\n        entry: echo\n        language: system\n        files: ^(a|b)\\.py$\n        exclude: (?x)^( foo| bar )$\n        additional_dependencies:\n        - a\n        - b\n-   repo: meta\n    hooks:\n    -   id: check-useless-excludes\n",
			err:  false,
		},
		{
			name: "numbers",
			in:   "a: 1_000\nb: +12\nc: -0x1A\nd: 0b101\ne: 1.\nf: .5\ng: 6.8523015e+5\nh: 190:20:30.15\ni: 123456789012345678901234567890\nj: 1e5\nk: 12e03\nl: 0.0001\nm: 0.00001\n",
			want: "a: 1000\nb: 12\nc: -26\nd: 5\ne: 1.0\nf: 0.5\ng: 685230.15\nh: 685230.15\ni: 123456789012345678901234567890\nj: 1e5\nk: 12e03\nl: 0.0001\nm: 1.0e-05\n",
			err:  false,
		},
		{
			name: "quote",
			in:   "a: \"café\"\nb: \"tab\\there\"\nc: \"- x\"\nd: \"x # y\"\ne: \"x: y\"\nf: \"#x\"\ng: \" lead\"\nh: \"trail \"\ni: \"multi\\nline\"\nj: \"it's\"\nk: \"@foo\"\nl: \"%x\"\nm: \"---x\"\nn: \"a,b\"\no: \"?x\"\np: \"x?\"\n",
			want: "a: \"caf\\xE9\"\nb: \"tab\\there\"\nc: '- x'\nd: 'x # y'\ne: 'x: y'\nf: '#x'\ng: ' lead'\nh: 'trail '\ni: 'multi\n\n    line'\nj: it's\nk: '@foo'\nl: '%x'\nm: '---x'\nn: a,b\no: ?x\np: x?\n",
			err:  false,
		},
		{name: "root_list", in: "- a\n- b\n", want: "- a\n- b\n", err: false},
		{name: "scalar_root", in: "hello\n", want: "hello\n...\n", err: false},
		{
			name: "spaces",
			in:   "a: \"  \"\nb: \" \"\nc: \"a\\n\\nb\"\nd: \"a \\nb\"\ne: \"a\\n b\"\nf: \"\\n\"\n",
			want: "a: '  '\nb: ' '\nc: 'a\n\n\n    b'\nd: \"a \\nb\"\ne: \"a\\n b\"\nf: '\n\n    '\n",
			err:  false,
		},
		{
			name: "strs",
			in:   "a: 'yes'\nb: 'No'\nc: '~'\nd: 'null'\ne: '='\nf: '<<'\ng: '0x10'\nh: '12:30'\ni: 'a  b'\nj: 'x\ty'\nk: '-'\nl: '- '\nm: ':'\nn: 'a:b'\no: 'a :b'\np: '[x]'\nq: 'x]'\nr: '...'\ns: 'a#b'\nt: 'a #b'\n",
			want: "a: 'yes'\nb: 'No'\nc: '~'\nd: 'null'\ne: '='\nf: '<<'\ng: '0x10'\nh: '12:30'\ni: a  b\nj: \"x\\ty\"\nk: '-'\nl: '- '\nm: ':'\nn: a:b\no: a :b\np: '[x]'\nq: x]\nr: '...'\ns: a#b\nt: 'a #b'\n",
			err:  false,
		},
		{
			name: "types",
			in:   "a: yes\nb: 'yes'\nc: 1.0\nd: '1.0'\ne: 0x1F\nf: 1:20\ng: 2020-01-01\nh: ~\ni: ''\nj: 1e17\nk: 1.5e-7\nl: .inf\nm: -0\nn: 0o17\no: 017\np: 3.0\nq: 100000000000000000.0\nr: \"null\"\ns: [] \nt: {}\nu: x: y\n",
			want: "",
			err:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := Load(tc.in)
			if tc.err {
				if err == nil {
					t.Fatalf("Load(%q) succeeded; PyYAML refuses it", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load(%q): %v", tc.in, err)
			}
			if got := Dump(v); got != tc.want {
				t.Errorf("Dump mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	for in, want := range map[string]string{
		"":           "null",
		"~":          "null",
		"yes":        "bool",
		"Off":        "bool",
		"y":          "str",
		"1.0":        "float",
		"1e3":        "str",
		"1:20":       "int",
		"0x1F":       "int",
		"017":        "int",
		"2020-01-01": "timestamp",
		"<<":         "merge",
		"=":          "value",
		"v1.0.0":     "str",
	} {
		if got := Resolve(in); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDumpStyle(t *testing.T) {
	rev := func(s string) *Value {
		return &Value{Kind: Map, Pairs: []Pair{{&Value{Kind: Str, Str: "rev"}, &Value{Kind: Str, Str: s}}}}
	}
	for _, tc := range []struct {
		s, want string
		style   byte
	}{
		{s: "v1.0", style: 0, want: "rev: v1.0\n"},
		{s: "1.0", style: 0, want: "rev: '1.0'\n"},
		{s: "v1.0", style: '\'', want: "'rev': 'v1.0'\n"},
		{s: "v1.0", style: '"', want: "\"rev\": \"v1.0\"\n"},
		{s: "café", style: '\'', want: "'rev': \"caf\\xE9\"\n"},
	} {
		if got := DumpStyle(rev(tc.s), tc.style); got != tc.want {
			t.Errorf("DumpStyle(%q, %q) = %q, want %q", tc.s, tc.style, got, tc.want)
		}
	}
}
