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
// sorted one by chance, and few enough that one batch holds them all here.
const fileOrderCount = 24

// fileOrderRepo is a repo of f00.txt..f23.txt plus the config, all staged.
func fileOrderRepo(t *testing.T, cfg string) string {
	t.Helper()
	dir := initTestRepo(t, cfg, "")
	for i := range fileOrderCount {
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
