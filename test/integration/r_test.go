//go:build integration

// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// rHookScript prints its arguments and fails on any file containing BAD.
const rHookScript = `args <- commandArgs(trailingOnly = TRUE)
cat("args:", args, "\n")
bad <- Filter(function(f) !startsWith(f, "--") && any(grepl("BAD", readLines(f, warn = FALSE))), args)
for (f in bad) cat("bad file:", f, "\n")
quit(status = if (length(bad) > 0) 1 else 0)
`

// renvTemplate is upstream's empty renv project (the same files make_local
// writes for a local hook), which a hook repo ships as renv.lock and
// renv/activate.R.
func renvTemplate(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	for name, src := range map[string]string{
		"renv.lock":       "renv.lock",
		"renv/activate.R": "activate.R",
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", "internal", "store", "resources", src))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(data)
	}
	return files
}

// TestRHooks runs a remote and a `repo: local` r hook through both tools:
// upstream's r.py builds a renv project from the repo's lockfile and runs
// `Rscript --no-save --no-restore --no-site-file --no-environ <script>`,
// the script resolved against the hook repo only for a remote hook.
func TestRHooks(t *testing.T) {
	pyBin := pythonPreCommit(t)
	if _, err := exec.LookPath("Rscript"); err != nil {
		if os.Getenv("PARITY_REQUIRE") != "" {
			t.Fatal("PARITY_REQUIRE is set but R is not installed")
		}
		t.Skip("R is not installed")
	}
	// RENV_PROJECT is set to prove both unset it (upstream's get_env_patch).
	t.Setenv("RENV_PROJECT", "/nonexistent")

	runBoth := func(t *testing.T, cfg string, files map[string]string) (pyOut, goOut string, pyExit, goExit int) {
		t.Helper()
		var repos [2]string
		for i := range repos {
			repos[i] = initTestRepo(t, cfg, "fine\n")
			for name, content := range files {
				path := filepath.Join(repos[i], name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			runCmd(t, repos[i], "git", "add", "-A")
		}
		t.Setenv("PRE_COMMIT_HOME", t.TempDir())
		pyOut, pyExit = runCmd(t, repos[0], pyBin, "run", "--all-files", "--color=never")
		t.Setenv("PRE_COMMIT_HOME", t.TempDir())
		goOut, goExit = runCmd(t, repos[1], goBinary, "run", "--all-files", "--color=never")
		return pyOut, goOut, pyExit, goExit
	}
	argsLine := func(out string) string {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "args: ") {
				return line
			}
		}
		return ""
	}
	check := func(t *testing.T, name, cfg string, files map[string]string, wantArgs string) {
		t.Helper()
		files["bad.txt"] = "BAD\n"
		pyOut, goOut, pyExit, goExit := runBoth(t, cfg, files)
		addExitResult("run", "r: "+name+", rejects bad input", pyExit, goExit,
			pyExit != 0 && goExit != 0 &&
				strings.Contains(pyOut, wantArgs) && strings.Contains(goOut, wantArgs) &&
				strings.Contains(goOut, "bad file: bad.txt"),
			fmt.Sprintf("py:\n%s\ngo:\n%s", pyOut, goOut))
		compareHookStatuses(t, "run", pyOut, goOut, 1)
		addOutputResult("run", "r: "+name+", script sees the same argv", argsLine(pyOut) == argsLine(goOut),
			fmt.Sprintf("py=%q go=%q", argsLine(pyOut), argsLine(goOut)))

		delete(files, "bad.txt")
		pyOut, goOut, pyExit, goExit = runBoth(t, cfg, files)
		addExitResult("run", "r: "+name+", passes good input", pyExit, goExit,
			pyExit == 0 && goExit == 0, fmt.Sprintf("py:\n%s\ngo:\n%s", pyOut, goOut))
	}

	t.Run("remote hook", func(t *testing.T) {
		dir := t.TempDir()
		for _, args := range [][]string{
			{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}, {"config", "commit.gpgsign", "false"},
		} {
			runCmd(t, dir, "git", args...)
		}
		files := renvTemplate(t)
		files["hook.R"] = rHookScript
		files[".pre-commit-hooks.yaml"] = "-   id: no-bad-r\n    name: no BAD (r)\n    entry: Rscript hook.R\n" +
			"    language: r\n    args: [--tag=remote]\n    files: \\.txt$\n"
		for name, content := range files {
			path := filepath.Join(dir, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		runCmd(t, dir, "git", "add", "-A")
		if out, code := runCmd(t, dir, "git", "commit", "-qm", "hook"); code != 0 {
			t.Fatalf("committing the hook repo failed:\n%s", out)
		}
		rev, _ := runCmd(t, dir, "git", "rev-parse", "HEAD")
		cfg := fmt.Sprintf(
			"repos:\n-   repo: %s\n    rev: %s\n    hooks:\n    -   id: no-bad-r\n",
			dir,
			strings.TrimSpace(rev),
		)
		check(t, "remote hook", cfg, map[string]string{}, "args: --tag=remote ")
	})

	t.Run("local hook", func(t *testing.T) {
		cfg := "repos:\n-   repo: local\n    hooks:\n    -   id: no-bad-r\n        name: no BAD (r)\n" +
			"        entry: Rscript tools/hook.R\n        language: r\n        args: [--tag=local]\n        files: \\.txt$\n"
		check(t, "local hook", cfg, map[string]string{"tools/hook.R": rHookScript}, "args: --tag=local ")
	})
}
