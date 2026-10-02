// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/config"
)

// Upstream prints its generated config with yaml_dump (four-space indent)
// between rules of 79 '='. Captured from pre-commit 4.6.2.
func TestPrintTryConfigMatchesUpstream(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	printTryConfig("https://github.com/pre-commit/pre-commit-hooks", "1f20705e3fbacaaa3af09982f83fd4cfbf355c51",
		[]config.HookConfig{{ID: "check-yaml"}})
	w.Close()
	os.Stdout = orig
	got, _ := io.ReadAll(r)

	rule := strings.Repeat("=", 79)
	want := rule + "\nUsing config:\n" + rule + `
repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: 1f20705e3fbacaaa3af09982f83fd4cfbf355c51
    hooks:
    -   id: check-yaml
` + rule + "\n"
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
