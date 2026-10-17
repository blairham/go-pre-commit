// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// runStdout runs fn with stdout captured and color off.
func runStdout(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	t.Setenv("PRE_COMMIT_COLOR", "never")
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := fn()
	w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

func install(t *testing.T, args ...string) (string, int) {
	t.Helper()
	return runStdout(t, func() int { return (&InstallCommand{Meta: &Meta{}}).Run(args) })
}

func uninstall(t *testing.T, args ...string) (string, int) {
	t.Helper()
	return runStdout(t, func() int { return (&UninstallCommand{Meta: &Meta{}}).Run(args) })
}

func wantOut(t *testing.T, what, got string, gotCode int, want string, wantCode int) {
	t.Helper()
	if got != want || gotCode != wantCode {
		t.Errorf("%s: got (%d) %q, want (%d) %q", what, gotCode, got, wantCode, want)
	}
}

// Every expectation below is pre-commit 4.7.0's output for the same steps.
func TestInstallUninstallMessages(t *testing.T) {
	root := tempRepo(t)
	t.Chdir(filepath.Join(root, "sub"))

	out, code := install(t)
	wantOut(t, "install from sub", out, code, "pre-commit installed at .git/hooks/pre-commit\n", 0)

	out, code = install(t, "-t", "pre-push")
	wantOut(t, "install -t", out, code, "pre-commit installed at .git/hooks/pre-push\n", 0)

	// A foreign hook moves to .legacy silently; the legacy is announced on
	// this and every later install.
	legacy := "#!/bin/sh\necho legacy\n"
	if err := os.WriteFile(filepath.Join(root, ".git/hooks/commit-msg"), []byte(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	migration := "Running in migration mode with existing hooks at .git/hooks/commit-msg.legacy\n" +
		"Use -f to use only pre-commit.\npre-commit installed at .git/hooks/commit-msg\n"
	for range 2 {
		out, code = install(t, "-t", "commit-msg")
		wantOut(t, "install over a foreign hook", out, code, migration, 0)
	}

	out, code = uninstall(t, "-t", "commit-msg")
	wantOut(t, "uninstall with a legacy hook", out, code,
		"commit-msg uninstalled\nRestored previous hooks to .git/hooks/commit-msg\n", 0)
	if got, _ := os.ReadFile(filepath.Join(root, ".git/hooks/commit-msg")); string(got) != legacy {
		t.Errorf("legacy hook not restored: %q", got)
	}

	// Not ours, then missing: both silent.
	out, code = uninstall(t, "-t", "commit-msg")
	wantOut(t, "uninstall a foreign hook", out, code, "", 0)
	out, code = uninstall(t, "-t", "post-merge")
	wantOut(t, "uninstall a missing hook", out, code, "", 0)

	// -f drops the legacy hook rather than announcing it.
	out, code = install(t, "-t", "commit-msg", "-f")
	wantOut(t, "install -f", out, code, "pre-commit installed at .git/hooks/commit-msg\n", 0)
	if _, err := os.Stat(filepath.Join(root, ".git/hooks/commit-msg.legacy")); !os.IsNotExist(err) {
		t.Errorf("install -f left the legacy hook: %v", err)
	}

	out, code = uninstall(t)
	wantOut(t, "uninstall", out, code, "pre-commit uninstalled\n", 0)
}

// uninstall without -t takes the config's default_install_hook_types, as
// install does.
func TestUninstallDefaultHookTypes(t *testing.T) {
	root := tempRepo(t)
	t.Chdir(root)
	cfg := "default_install_hook_types: [pre-commit, pre-push]\nrepos: []\n"
	if err := os.WriteFile(filepath.Join(root, ".pre-commit-config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := install(t)
	wantOut(t, "install", out, code,
		"pre-commit installed at .git/hooks/pre-commit\npre-commit installed at .git/hooks/pre-push\n", 0)
	out, code = uninstall(t)
	wantOut(t, "uninstall", out, code, "pre-commit uninstalled\npre-push uninstalled\n", 0)
}

func TestInstallRefusesCoreHooksPath(t *testing.T) {
	root := tempRepo(t)
	t.Chdir(root)
	if out, err := exec.Command("git", "config", "core.hooksPath", "hooks").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}
	out, code := install(t)
	wantOut(t, "install", out, code, "[ERROR] Cowardly refusing to install hooks with `core.hooksPath` set.\n"+
		"hint: `git config --unset-all core.hooksPath`\n", 1)
	if entries, _ := os.ReadDir(filepath.Join(root, "hooks")); len(entries) != 0 {
		t.Errorf("install wrote into core.hooksPath: %v", entries)
	}
}

// In a linked worktree the common dir is absolute, and so is the path.
func TestInstallLinkedWorktree(t *testing.T) {
	root := tempRepo(t)
	for _, args := range [][]string{
		{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "i"},
		{"worktree", "add", "-q", filepath.Join(root, "wt")},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(filepath.Join(root, "wt"))
	out, code := install(t, "-t", "pre-merge-commit")
	wantOut(t, "install in a worktree", out, code,
		"pre-commit installed at "+filepath.Join(root, ".git/hooks/pre-merge-commit")+"\n", 0)
	out, code = uninstall(t, "-t", "pre-merge-commit")
	wantOut(t, "uninstall in a worktree", out, code, "pre-merge-commit uninstalled\n", 0)
}
