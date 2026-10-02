// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package gittest keeps tests out of the developer's git configuration.
//
// Every git process a test starts — directly, or through the code under test
// — would otherwise read ~/.gitconfig and the system config. A maintainer's
// tag.gpgsign=true made `git tag v1.0.0` demand a message and fail three
// tests that pass on a bare CI runner; an insteadOf rewrite, a hooksPath or
// a signing key would leak in the same way. Tests must not depend on, or
// touch, the real user's state.
package gittest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Main points git at an empty global config and away from the system one for
// the whole test binary, runs the tests, and returns their exit code. Use it
// from TestMain:
//
//	func TestMain(m *testing.M) { os.Exit(gittest.Main(m)) }
func Main(m *testing.M) int {
	dir, err := os.MkdirTemp("", "gittest-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "gittest:", err)
		return 1
	}
	defer os.RemoveAll(dir)

	global := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(global, nil, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "gittest:", err)
		return 1
	}
	os.Setenv("GIT_CONFIG_GLOBAL", global)
	os.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	return m.Run()
}
