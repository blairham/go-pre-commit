// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package gittest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(Main(m)) }

// Under Main, git sees no global or system configuration at all, whatever
// the developer running the tests has configured. Repository-local config is
// out of scope; it is asked from a directory outside any repository.
func TestNoUserConfigVisible(t *testing.T) {
	cmd := exec.Command("git", "config", "--list", "--show-scope")
	cmd.Dir = t.TempDir()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config --list: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "global\t") || strings.HasPrefix(line, "system\t") {
			t.Errorf("tests can see git configuration: %s", line)
		}
	}
}
