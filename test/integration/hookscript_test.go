// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestHookScript grades the script install and init-templatedir write (#164):
// a commit in a repo with no config, through a hook installed each way, and
// each tool taking the other's script as its own by upstream's hash rule.
func TestHookScript(t *testing.T) {
	pyBin := pythonPreCommit(t)
	// This port's script runs `pre-commit` from PATH; Python's runs its own
	// interpreter.
	t.Setenv("PATH", filepath.Dir(goBinary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())
	t.Setenv("GIT_TEMPLATE_DIR", t.TempDir())
	t.Setenv("PRE_COMMIT_ALLOW_NO_CONFIG", "")

	commit := func(repo string) (string, int) {
		return runCmd(t, repo, "git", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "x")
	}

	// Through install: with --allow-missing-config the hook skips, without
	// it the hook fails and says how to proceed.
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"install --allow-missing-config", []string{"install", "--allow-missing-config"}},
		{"install", []string{"install"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var outs [2]string
			var codes [2]int
			for i, bin := range []string{pyBin, goBinary} {
				repo := initTestRepo(t, "", "x\n")
				if out, code := runCmd(t, repo, bin, tc.args...); code != 0 {
					t.Fatalf("%s %v: %s", bin, tc.args, out)
				}
				outs[i], codes[i] = commit(repo)
			}
			addExitResult("install", "commit with no config after "+tc.name, codes[0], codes[1], codes[0] == codes[1], "")
			addOutputResult("install", "commit with no config after "+tc.name+": output",
				outs[0] == outs[1] && outs[0] != "", fmt.Sprintf("py=%q go=%q", outs[0], outs[1]))
		})
	}

	// Through init-templatedir, which skips on a missing config by default.
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"init-templatedir", nil},
		{"init-templatedir --no-allow-missing-config", []string{"--no-allow-missing-config"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var outs [2]string
			var codes [2]int
			for i, bin := range []string{pyBin, goBinary} {
				base := t.TempDir()
				tmpl := filepath.Join(base, "tmpl")
				args := append([]string{"init-templatedir"}, tc.extra...)
				if out, code := runCmd(t, base, bin, append(args, tmpl)...); code != 0 {
					t.Fatalf("%s %v: %s", bin, args, out)
				}
				repo := filepath.Join(base, "repo")
				if out, code := runCmd(t, base, "git", "init", "-q", "--template="+tmpl, repo); code != 0 {
					t.Fatalf("git init: %s", out)
				}
				for _, args := range [][]string{{"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
					runCmd(t, repo, "git", args...)
				}
				if err := os.WriteFile(filepath.Join(repo, "x"), []byte("x\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				runCmd(t, repo, "git", "add", "x")
				outs[i], codes[i] = commit(repo)
			}
			addExitResult(
				"init-templatedir",
				"commit with no config after "+tc.name,
				codes[0],
				codes[1],
				codes[0] == codes[1],
				"",
			)
			addOutputResult("init-templatedir", "commit with no config after "+tc.name+": output",
				outs[0] == outs[1] && outs[0] != "", fmt.Sprintf("py=%q go=%q", outs[0], outs[1]))
		})
	}

	// Each tool installs over, and uninstalls, the other's script as its own:
	// no .legacy is kept, and uninstall removes it.
	for _, tc := range []struct{ name, first, second string }{
		{"python over go", goBinary, pyBin},
		{"go over python", pyBin, goBinary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := initTestRepo(t, "repos: []\n", "x\n")
			hook := filepath.Join(repo, ".git", "hooks", "pre-commit")
			runCmd(t, repo, tc.first, "install")
			out, code := runCmd(t, repo, tc.second, "install")
			noLegacy := !fileExists(hook + ".legacy")
			addFSResult("install", "recognizes the other tool's script: "+tc.name,
				code == 0 && noLegacy && fileExists(hook), fmt.Sprintf("exit=%d legacy=%v out=%q", code, !noLegacy, out))

			runCmd(t, repo, tc.first, "install")
			out, code = runCmd(t, repo, tc.second, "uninstall")
			addFSResult("uninstall", "removes the other tool's script: "+tc.name,
				code == 0 && !fileExists(hook) && out == "pre-commit uninstalled\n",
				fmt.Sprintf("exit=%d present=%v out=%q", code, fileExists(hook), out))
		})
	}
}
