// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// A --repo the config does not name is an error, reported before anything is
// cloned or rewritten (upstream 4.6.1). local and meta are not updatable, so
// naming them is reported too. Output captured from pre-commit 4.7.0.
func TestAutoupdateRejectsUnknownRepo(t *testing.T) {
	const cfg = `repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
    -   id: trailing-whitespace
-   repo: local
    hooks:
    -   id: x
        name: x
        entry: x
        language: system
`
	// autoupdate changes to the repository root and reports the config
	// relative to it, so the test needs a repository of its own.
	root := tempRepo(t)
	t.Chdir(root)
	path := filepath.Join(root, "cfg.yaml")
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	code := (&AutoupdateCommand{}).Run([]string{
		"-c", path, "--color=never",
		"--repo", "zz",
		"--repo", "https://github.com/pre-commit/pre-commit-hooks",
		"--repo", "local",
		"--repo", "zz",
		"--repo", "a",
	})
	w.Close()
	os.Stdout = orig
	got, _ := io.ReadAll(r)

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	want := "repos (a, local, zz) were not found in cfg.yaml\n"
	if string(got) != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	after, _ := os.ReadFile(path)
	if string(after) != cfg {
		t.Errorf("config was rewritten:\n%s", after)
	}
}
