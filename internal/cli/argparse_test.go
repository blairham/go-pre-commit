// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

// The expected text is Python 3.13 argparse's, as pre-commit 4.7.0 prints
// it (#175).
func TestArgparseUsage(t *testing.T) {
	type usageCase struct {
		cmd   string
		want  string
		width int
	}
	usageRow := func(cmd string, width int, want string) usageCase {
		return usageCase{cmd: cmd, width: width, want: want}
	}
	for _, tc := range []usageCase{
		usageRow("install", 78, "usage: pre-commit install [-h] [--color {auto,always,never}] [-c CONFIG] [-f]\n"+
			"                          [--install-hooks]\n"+
			"                          [-t {commit-msg,post-checkout,post-commit,post-merge,post-rewrite,pre-commit,pre-merge-commit,pre-push,pre-rebase,prepare-commit-msg}]\n"+
			"                          [--allow-missing-config]\n"),
		usageRow("install", 58, "usage: pre-commit install [-h]\n"+
			"                          [--color {auto,always,never}]\n"+
			"                          [-c CONFIG] [-f]\n"+
			"                          [--install-hooks]\n"+
			"                          [-t {commit-msg,post-checkout,post-commit,post-merge,post-rewrite,pre-commit,pre-merge-commit,pre-push,pre-rebase,prepare-commit-msg}]\n"+
			"                          [--allow-missing-config]\n"),
		usageRow("hook-impl", 78, "usage: pre-commit hook-impl [-h] [--color {auto,always,never}] [-c CONFIG]\n"+
			"                            --hook-type HOOK_TYPE [--hook-dir HOOK_DIR]\n"+
			"                            [--skip-on-missing-config]\n"+
			"                            ...\n"),
		usageRow("autoupdate", 78, "usage: pre-commit autoupdate [-h] [--color {auto,always,never}] [-c CONFIG]\n"+
			"                             [--bleeding-edge] [--freeze] [--repo REPO]\n"+
			"                             [-j JOBS]\n"),
		usageRow("clean", 78, "usage: pre-commit clean [-h] [--color {auto,always,never}]\n"),
	} {
		spec := commandSpecs[tc.cmd]
		if got := spec.usage(tc.width); got != tc.want {
			t.Errorf("%s at %d:\n%s\nwant:\n%s", tc.cmd, tc.width, got, tc.want)
		}
	}

	// The mutually exclusive group breaks inside its brackets, at " |".
	got := commandSpecs["try-repo"].usage(78)
	if !strings.Contains(got, " [--ref REF] [--verbose] [--all-files |\n"+
		"                           --files [FILES ...]] [--show-diff-on-failure]\n") {
		t.Errorf("try-repo usage:\n%s", got)
	}
	// A prog longer than 3/4 of the width goes on its own line.
	if got := commandSpecs["try-repo"].usage(28); !strings.HasPrefix(got, "usage: pre-commit try-repo\n       [-h]\n") {
		t.Errorf("try-repo at 28:\n%s", got)
	}
	if got := topLevelSpec.usage(
		78,
	); !strings.HasPrefix(
		got,
		"usage: pre-commit [-h] [-V]\n                  {autoupdate,clean,",
	) {
		t.Errorf("top level:\n%s", got)
	}
}

func captureStderrCode(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	code := fn()
	w.Close()
	os.Stderr = orig
	out, _ := io.ReadAll(r)
	return string(out), code
}

func TestParseArgsErrors(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	hookTypes := "'commit-msg', 'post-checkout', 'post-commit', 'post-merge', 'post-rewrite', 'pre-commit', " +
		"'pre-merge-commit', 'pre-push', 'pre-rebase', 'prepare-commit-msg'"
	type errCase struct {
		cmd     string
		prog    string
		message string
		args    []string
	}
	errRow := func(cmd string, args []string, prog, message string) errCase {
		return errCase{cmd: cmd, args: args, prog: prog, message: message}
	}
	for _, tc := range []errCase{
		errRow(
			"install",
			[]string{"-t", "bogus"},
			"pre-commit install",
			"argument -t/--hook-type: invalid choice: 'bogus' (choose from "+hookTypes+")",
		),
		errRow(
			"install",
			[]string{"-t", "pre-commit", "--hook-type=nope"},
			"pre-commit install",
			"argument -t/--hook-type: invalid choice: 'nope' (choose from "+hookTypes+")",
		),
		errRow("run", []string{"--bogus"}, "pre-commit", "unrecognized arguments: --bogus"),
		errRow("run", []string{"--bogus", "-x", "a", "b"}, "pre-commit", "unrecognized arguments: --bogus -x b"),
		errRow("run", []string{"a", "--", "b"}, "pre-commit", "unrecognized arguments: b"),
		errRow("run", []string{"-c"}, "pre-commit run", "argument -c/--config: expected one argument"),
		errRow("run", []string{"-c", "--verbose"}, "pre-commit run", "argument -c/--config: expected one argument"),
		errRow("run", []string{"--verbose=x"}, "pre-commit run", "argument --verbose/-v: ignored explicit argument 'x'"),
		errRow(
			"run",
			[]string{"--all-files", "--files", "a"},
			"pre-commit run",
			"argument --files: not allowed with argument --all-files/-a",
		),
		errRow(
			"run",
			[]string{"--files", "a", "-a"},
			"pre-commit run",
			"argument --all-files/-a: not allowed with argument --files",
		),
		errRow("run", []string{"--color", "bogus"}, "pre-commit run", "argument --color: invalid use_color value: 'bogus'"),
		errRow(
			"run",
			[]string{"--hook-stage", "nope"},
			"pre-commit run",
			"argument --hook-stage: invalid choice: 'nope' (choose from "+hookTypes+", 'manual')",
		),
		errRow("autoupdate", []string{"-j", "x"}, "pre-commit autoupdate", "argument -j/--jobs: invalid int value: 'x'"),
		errRow("init-templatedir", nil, "pre-commit init-templatedir", "the following arguments are required: directory"),
		errRow("try-repo", nil, "pre-commit try-repo", "the following arguments are required: repo"),
		errRow("try-repo", []string{"a", "b", "c"}, "pre-commit", "unrecognized arguments: c"),
		errRow("hook-impl", nil, "pre-commit hook-impl", "the following arguments are required: --hook-type"),
		errRow("gc", []string{"extra"}, "pre-commit", "unrecognized arguments: extra"),
		errRow("clean", []string{"-c", "x"}, "pre-commit", "unrecognized arguments: -c x"),
	} {
		var opts any
		switch tc.cmd {
		case "run":
			opts = &runFlags{}
		case "install":
			opts = &installFlags{}
		case "autoupdate":
			opts = &autoupdateFlags{}
		case "init-templatedir":
			opts = &initTemplateDirFlags{}
		case "try-repo":
			opts = &tryRepoFlags{}
		case "hook-impl":
			opts = &hookImplFlags{}
		default:
			opts = &ColorFlags{}
		}
		out, code := captureStderrCode(t, func() int {
			_, code, ok := parseArgs(tc.cmd, opts, tc.args, expandFilesFlag(tc.args))
			if ok {
				return 0
			}
			return code
		})
		spec := topLevelSpec
		if tc.prog != "pre-commit" {
			spec = commandSpecs[tc.cmd]
		}
		want := spec.usage(78) + tc.prog + ": error: " + tc.message + "\n"
		if code != 2 || out != want {
			t.Errorf("%s %q: code %d\n%s\nwant:\n%s", tc.cmd, tc.args, code, out, want)
		}
	}
}

func TestParseArgsAccepts(t *testing.T) {
	var opts runFlags
	args := []string{"hook", "-a", "--hook-stage", "commit"}
	pos, _, ok := parseArgs("run", &opts, args, args)
	if !ok || len(pos) != 1 || pos[0] != "hook" || !opts.AllFiles || opts.HookStage != "commit" {
		t.Errorf("pos = %q ok = %v opts = %+v", pos, ok, opts)
	}
	var hi hookImplFlags
	args = []string{"--hook-type=pre-push", "--hook-dir", "d", "--", "origin", "--weird"}
	pos, _, ok = parseArgs("hook-impl", &hi, args, args)
	if !ok || strings.Join(pos, " ") != "origin --weird" || hi.HookType != "pre-push" {
		t.Errorf("hook-impl pos = %q ok = %v", pos, ok)
	}
}
