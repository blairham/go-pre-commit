// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// pythonHookScript is the script pre-commit 4.7.0 installs, byte for byte.
const pythonHookScript = "#!/usr/bin/env bash\n" +
	"# File generated by pre-commit: https://pre-commit.com\n" +
	"# ID: 138fd403232d2ddd5efb44317e38bf03\n\n" +
	"# start templated\n" +
	"INSTALL_PYTHON=/venv/bin/python3\n" +
	"ARGS=(hook-impl --config=.pre-commit-config.yaml --hook-type=pre-commit)\n" +
	"# end templated\n\n" +
	"HERE=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\n" +
	"ARGS+=(--hook-dir \"$HERE\" -- \"$@\")\n\n" +
	"if [ -x \"$INSTALL_PYTHON\" ]; then\n" +
	"    exec \"$INSTALL_PYTHON\" -mpre_commit \"${ARGS[@]}\"\n" +
	"elif command -v pre-commit > /dev/null; then\n" +
	"    exec pre-commit \"${ARGS[@]}\"\n" +
	"else\n" +
	"    echo '`pre-commit` not found.  Did you forget to activate your virtualenv?' 1>&2\n" +
	"    exit 1\n" +
	"fi\n"

// The script is upstream's hook-tmpl with the hash and the templated ARGS,
// and differs only in running pre-commit from PATH rather than a pinned
// interpreter. --allow-missing-config is what adds --skip-on-missing-config.
func TestInstallWritesUpstreamHookScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("upstream writes a /bin/sh line first on Windows")
	}
	root := tempRepo(t)
	t.Chdir(root)

	want := "#!/usr/bin/env bash\n" +
		"# File generated by pre-commit: https://pre-commit.com\n" +
		"# ID: 138fd403232d2ddd5efb44317e38bf03\n\n" +
		"# start templated\n" +
		"ARGS=(hook-impl --config=.pre-commit-config.yaml --hook-type=pre-commit --skip-on-missing-config)\n" +
		"# end templated\n\n" +
		"HERE=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\n" +
		"ARGS+=(--hook-dir \"$HERE\" -- \"$@\")\n\n" +
		"if command -v pre-commit > /dev/null; then\n" +
		"    exec pre-commit \"${ARGS[@]}\"\n" +
		"else\n" +
		"    echo '`pre-commit` not found.  Did you forget to install it?' 1>&2\n" +
		"    exit 1\n" +
		"fi\n"
	if _, code := install(t, "--allow-missing-config"); code != 0 {
		t.Fatalf("install exited %d", code)
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".git/hooks/pre-commit")); string(got) != want {
		t.Errorf("hook script:\n%s\nwant:\n%s", got, want)
	}

	// Without the flag the hook fails on a missing config, as upstream's.
	if _, code := install(t); code != 0 {
		t.Fatalf("install exited %d", code)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".git/hooks/pre-commit"))
	if !strings.Contains(string(got), "ARGS=(hook-impl --config=.pre-commit-config.yaml --hook-type=pre-commit)\n") {
		t.Errorf("install without --allow-missing-config:\n%s", got)
	}

	// The ARGS line is shlex.join'd.
	if _, code := install(t, "-c", "it's my.yaml", "-t", "pre-push"); code != 0 {
		t.Fatalf("install exited %d", code)
	}
	got, _ = os.ReadFile(filepath.Join(root, ".git/hooks/pre-push"))
	if !strings.Contains(string(got), `ARGS=(hook-impl '--config=it'"'"'s my.yaml' --hook-type=pre-push)`+"\n") {
		t.Errorf("quoted config path:\n%s", got)
	}
}

// Each tool takes the other's script as its own: install replaces Python's
// without keeping it as a legacy hook, and uninstall removes it.
func TestInstallRecognizesUpstreamScript(t *testing.T) {
	root := tempRepo(t)
	t.Chdir(root)
	hook := filepath.Join(root, ".git/hooks/pre-commit")
	if err := os.MkdirAll(filepath.Dir(hook), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hook, []byte(pythonHookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	out, code := install(t)
	wantOut(t, "install over upstream's script", out, code, "pre-commit installed at .git/hooks/pre-commit\n", 0)
	if _, err := os.Stat(hook + ".legacy"); !os.IsNotExist(err) {
		t.Errorf("upstream's script was kept as a legacy hook: %v", err)
	}

	if err := os.WriteFile(hook, []byte(pythonHookScript), 0o755); err != nil {
		t.Fatal(err)
	}
	out, code = uninstall(t)
	wantOut(t, "uninstall upstream's script", out, code, "pre-commit uninstalled\n", 0)
}

// init-templatedir skips on a missing config unless told not to, as
// upstream's skip_on_missing_config=True default does.
func TestInitTemplateDirSkipOnMissingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"tmpl"}, true},
		{[]string{"--no-allow-missing-config", "tmpl"}, false},
	} {
		_, code := runStdout(t, func() int { return (&InitTemplateDirCommand{Meta: &Meta{}}).Run(tc.args) })
		if code != 0 {
			t.Fatalf("%v exited %d", tc.args, code)
		}
		got, _ := os.ReadFile(filepath.Join(dir, "tmpl/hooks/pre-commit"))
		if strings.Contains(string(got), "--skip-on-missing-config") != tc.want ||
			!strings.Contains(string(got), currentHash) {
			t.Errorf("%v: script:\n%s", tc.args, got)
		}
	}
}

// init-templatedir's warnings, measured against pre-commit 4.7.0: none when
// init.templateDir resolves to the target (through a relative path, ~ or a
// symlink), two on stdout otherwise, naming the target's realpath. It goes
// through install, so default_install_hook_types applies and a legacy hook
// is dropped.
func TestInitTemplateDirWarnings(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(base, "w")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(base, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	gitconfig := filepath.Join(base, "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("HOME", base)
	t.Chdir(work)

	tmpl := filepath.Join(base, "tmpl")
	warn := "[WARNING] `init.templateDir` not set to the target directory\n" +
		"[WARNING] maybe `git config --global init.templateDir " + tmpl + "`?\n"
	installed := "pre-commit installed at ../tmpl/hooks/pre-commit\n"
	run := func(args ...string) (string, int) {
		return runStdout(t, func() int { return (&InitTemplateDirCommand{Meta: &Meta{}}).Run(args) })
	}
	for _, tc := range []struct {
		name, configured string
		want             string
	}{
		{"unset", "", installed + warn},
		{"relative", "../tmpl", installed},
		{"tilde", "~/tmpl", installed},
		{"through a symlink", filepath.Join(base, "link", "tmpl"), installed},
		{"elsewhere", "/elsewhere", installed + warn},
	} {
		content := ""
		if tc.configured != "" {
			content = "[init]\n\ttemplateDir = " + tc.configured + "\n"
		}
		if err := os.WriteFile(gitconfig, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		out, code := run("../tmpl")
		wantOut(t, tc.name, out, code, tc.want, 0)
	}

	// A legacy hook is dropped (overwrite=True), and the config's
	// default_install_hook_types decides the types.
	if err := os.WriteFile(filepath.Join(tmpl, "hooks", "pre-commit.legacy"), []byte("old\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpl, "hooks", "pre-commit"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, code := run("../tmpl")
	wantOut(t, "over a foreign hook", out, code, installed+warn, 0)
	if _, err := os.Stat(filepath.Join(tmpl, "hooks", "pre-commit.legacy")); !os.IsNotExist(err) {
		t.Errorf("legacy hook kept: %v", err)
	}
	cfg := "default_install_hook_types: [pre-push, commit-msg]\nrepos: []\n"
	if err := os.WriteFile(".pre-commit-config.yaml", []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = run("../tmpl")
	wantOut(t, "default_install_hook_types", out, code,
		"pre-commit installed at ../tmpl/hooks/pre-push\npre-commit installed at ../tmpl/hooks/commit-msg\n"+warn, 0)

	// --no-allow-missing-config prints nothing on stderr either.
	if err := os.Remove(".pre-commit-config.yaml"); err != nil {
		t.Fatal(err)
	}
	stderr := captureStderr(t, func() { out, code = run("--no-allow-missing-config", "../tmpl") })
	wantOut(t, "--no-allow-missing-config", out, code, installed+warn, 0)
	if stderr != "" {
		t.Errorf("--no-allow-missing-config: stderr %q", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	b, _ := io.ReadAll(r)
	r.Close()
	return string(b)
}
