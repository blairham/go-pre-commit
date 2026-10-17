// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitTemplateDirOutput compares what init-templatedir prints (#163):
// upstream warns on stdout unless git's init.templateDir resolves to the
// target directory.
func TestInitTemplateDirOutput(t *testing.T) {
	pyBin := pythonPreCommit(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	for _, tc := range []struct {
		name, configured string
		extra            []string
	}{
		{"init.templateDir unset", "", nil},
		{"init.templateDir relative to the target", "../tmpl", nil},
		{"init.templateDir via ~", "~/tmpl", nil},
		{"init.templateDir elsewhere", "/elsewhere", nil},
		{"init.templateDir elsewhere, --no-allow-missing-config", "/elsewhere", []string{"--no-allow-missing-config"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var outs, errs [2]string
			var codes [2]int
			for i, bin := range []string{pyBin, goBinary} {
				base, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				work := filepath.Join(base, "w")
				if err := os.Mkdir(work, 0o755); err != nil {
					t.Fatal(err)
				}
				gitconfig := filepath.Join(base, "gitconfig")
				content := ""
				if tc.configured != "" {
					content = "[init]\n\ttemplateDir = " + tc.configured + "\n"
				}
				if err := os.WriteFile(gitconfig, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
				t.Setenv("HOME", base)
				args := append(append([]string{"init-templatedir"}, tc.extra...), "../tmpl")
				out, errOut, code := runSplit(t, work, bin, args...)
				outs[i] = strings.ReplaceAll(out, base, "<BASE>")
				errs[i] = strings.ReplaceAll(errOut, base, "<BASE>")
				codes[i] = code
			}
			addExitResult("init-templatedir", tc.name, codes[0], codes[1], codes[0] == codes[1], "")
			addOutputResult("init-templatedir", tc.name+": stdout", outs[0] == outs[1] && outs[0] != "",
				fmt.Sprintf("py=%q go=%q", outs[0], outs[1]))
			addOutputResult("init-templatedir", tc.name+": stderr", errs[0] == errs[1],
				fmt.Sprintf("py=%q go=%q", errs[0], errs[1]))
		})
	}
}
