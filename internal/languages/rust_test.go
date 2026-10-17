// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// upstream's rust.get_default_version asks cargo itself: a cargo that is on
// PATH but cannot run (rustup with no toolchain) is not a system rust.
func TestRustDefaultVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake cargo is a sh script")
	}
	for script, want := range map[string]string{
		"#!/bin/sh\nexit 0\n": "system",
		"#!/bin/sh\nexit 1\n": "default",
		"":                    "default", // no cargo at all
	} {
		dir := t.TempDir()
		if script != "" {
			if err := os.WriteFile(filepath.Join(dir, "cargo"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", dir)
		if got := rustDefaultVersion(); got != want {
			t.Errorf("cargo %q: rustDefaultVersion = %q, want %q", script, got, want)
		}
	}
}
