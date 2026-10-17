// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fileOrderCount is enough files that a shuffled order cannot pass for the
// sorted one by chance.
const fileOrderCount = 24

// fileOrderRepo is a repo of f00.txt..f23.txt plus the config, all staged.
func fileOrderRepo(t *testing.T, cfg string) string {
	t.Helper()
	return numberedFilesRepo(t, cfg, fileOrderCount)
}

// numberedFilesRepo is a repo of n files f00.txt, f01.txt, ... plus the
// config, all staged.
func numberedFilesRepo(t *testing.T, cfg string, n int) string {
	t.Helper()
	dir := initTestRepo(t, cfg, "")
	for i := range n {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%02d.txt", i)), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runCmd(t, dir, "git", "add", ".")
	return dir
}

// receivedFiles returns the f*.txt names the hook printed, in output order.
// Each name starts a line of its own (pygrep follows it with ":1:x"), so
// upstream's parallel xargs partitions, whose outputs it concatenates in
// partition order, read the same as one batch.
func receivedFiles(out string) []string {
	var files []string
	for _, line := range strings.Split(out, "\n") {
		line, _, _ = strings.Cut(strings.TrimSpace(line), ":")
		if strings.HasPrefix(line, "f") && strings.HasSuffix(line, ".txt") {
			files = append(files, line)
		}
	}
	return files
}

// TestHookFileOrder compares the exact order a hook receives its files in
// (#120). Upstream's run_xargs shuffles them with a fixed seed unless the
// hook is require_serial; pygrep calls xargs directly and is never shuffled.
func TestHookFileOrder(t *testing.T) {
	pyBin := pythonPreCommit(t)

	cases := []struct {
		name     string
		hook     string
		shuffled bool // whether upstream's order must differ from sorted
	}{
		{
			name: "system hook gets the seeded shuffle",
			hook: `entry: sh -c 'for f; do echo "$f"; done' --
    language: system`,
			shuffled: true,
		},
		{
			name: "require_serial hook keeps the given order",
			hook: `entry: sh -c 'for f; do echo "$f"; done' --
    language: system
    require_serial: true`,
		},
		{
			name: "pygrep hook keeps the given order",
			hook: `entry: x
    language: pygrep`,
		},
		{
			name: "fail hook keeps the given order",
			hook: `entry: nope
    language: fail`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PRE_COMMIT_HOME", t.TempDir())
			cfg := "repos:\n- repo: local\n  hooks:\n  - id: order\n    name: order\n    " +
				tc.hook + "\n    files: '^f'\n    verbose: true\n"
			pyOut, _ := runCmd(t, fileOrderRepo(t, cfg), pyBin, "run", "--all-files", "--color=never")
			goOut, _ := runCmd(t, fileOrderRepo(t, cfg), goBinary, "run", "--all-files", "--color=never")
			pyFiles, goFiles := receivedFiles(pyOut), receivedFiles(goOut)

			// The input must exercise the question: every file arrives, and
			// upstream's order is (or is not) the sorted one.
			sorted := slices.Sorted(slices.Values(pyFiles))
			exercised := len(pyFiles) == fileOrderCount && slices.Equal(pyFiles, sorted) != tc.shuffled

			addOutputResult("run", "file order: "+tc.name,
				exercised && slices.Equal(pyFiles, goFiles),
				fmt.Sprintf("exercised=%v\npy=%q\ngo=%q\npy output:\n%s\ngo output:\n%s",
					exercised, pyFiles, goFiles, pyOut, goOut))
		})
	}
}

// partitionFileCount is enough files that upstream's xargs splits them into
// several partitions on any runner with two or more CPUs, and into exactly
// two under TRAVIS.
const partitionFileCount = 40

// invocations returns the "invoked" lines a hook printed, one per time
// it was run, each carrying the files that run received.
func invocations(out string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "invoked ") {
			lines = append(lines, line)
		}
	}
	return lines
}

// TestHookPartitions compares how many times a hook runs and which files
// each run receives (#132). Upstream's xargs splits the files into
// partitions of at most max(4, ceil(files/jobs)), jobs being the CPU count,
// 2 under TRAVIS, and 1 under PRE_COMMIT_NO_CONCURRENCY or for a
// require_serial hook, then concatenates the partitions' output in order.
func TestHookPartitions(t *testing.T) {
	pyBin := pythonPreCommit(t)

	cases := []struct {
		name  string
		hook  string
		env   map[string]string
		multi bool // whether upstream must run the hook more than once
	}{
		{name: "default concurrency partitions", multi: true},
		{name: "TRAVIS makes two partitions", env: map[string]string{"TRAVIS": "1"}, multi: true},
		{name: "PRE_COMMIT_NO_CONCURRENCY runs once", env: map[string]string{"PRE_COMMIT_NO_CONCURRENCY": "1"}},
		{name: "require_serial runs once", hook: "\n    require_serial: true"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PRE_COMMIT_HOME", t.TempDir())
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			cfg := "repos:\n- repo: local\n  hooks:\n  - id: part\n    name: part\n" +
				"    entry: sh -c 'echo \"invoked $*\"' --\n    language: system" + tc.hook +
				"\n    files: '^f'\n    verbose: true\n"
			pyOut, _ := runCmd(t, numberedFilesRepo(t, cfg, partitionFileCount), pyBin, "run", "--all-files", "--color=never")
			goOut, _ := runCmd(t, numberedFilesRepo(t, cfg, partitionFileCount), goBinary, "run", "--all-files", "--color=never")
			pyRuns, goRuns := invocations(pyOut), invocations(goOut)

			// The input must exercise the question: upstream ran the hook
			// at least once, and more than once exactly when it should.
			exercised := len(pyRuns) > 0 && (len(pyRuns) > 1) == tc.multi
			if tc.env["TRAVIS"] != "" {
				exercised = len(pyRuns) == 2
			}

			addOutputResult("run", "xargs partitions: "+tc.name,
				exercised && slices.Equal(pyRuns, goRuns),
				fmt.Sprintf("exercised=%v\npy=%q\ngo=%q\npy output:\n%s\ngo output:\n%s",
					exercised, pyRuns, goRuns, pyOut, goOut))
		})
	}
}
