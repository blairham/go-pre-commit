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

const chdirCfg = `repos:
-   repo: local
    hooks:
    -   id: echo
        name: echo
        entry: echo
        language: system
`

// chdirRepo is a repository with the config and alt.yaml at its root, and
// sub/f.txt beneath it, at its physical path so an absolute --config given
// to it is not also a symlink case.
func chdirRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(initTestRepo(t, chdirCfg, ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "alt.yaml"), []byte(chdirCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// Upstream's main changes to the repository root before every command that
// needs one, and rewrites --config (and run's --files) relative to it (#109).
// So a command run from a subdirectory finds the root's config, and any
// message that prints the config path prints it root-relative.
func TestRootRelativeArgs(t *testing.T) {
	pyBin := pythonPreCommit(t)
	t.Setenv("PRE_COMMIT_HOME", t.TempDir())

	// autoupdate --repo q names the config path in its error, which makes
	// the rewritten --config visible.
	for _, tc := range []struct {
		name, from string
		config     func(root string) []string
	}{
		{"default config from sub/", "sub", func(string) []string { return nil }},
		{"../alt.yaml from sub/", "sub", func(string) []string { return []string{"-c", "../alt.yaml"} }},
		{"./alt.yaml from the root", ".", func(string) []string { return []string{"-c", "./alt.yaml"} }},
		{"absolute config from sub/", "sub", func(root string) []string {
			return []string{"-c", filepath.Join(root, "alt.yaml")}
		}},
	} {
		t.Run("autoupdate "+tc.name, func(t *testing.T) {
			pyRoot, goRoot := chdirRepo(t), chdirRepo(t)
			args := []string{"autoupdate", "--color=never", "--repo", "q"}
			pyOut, pyErr, pyExit := runSplit(t, filepath.Join(pyRoot, tc.from), pyBin, append(args, tc.config(pyRoot)...)...)
			goOut, goErr, goExit := runSplit(t, filepath.Join(goRoot, tc.from), goBinary, append(args, tc.config(goRoot)...)...)
			addExitResult("autoupdate", "root-relative config: "+tc.name, pyExit, goExit, pyExit == goExit, "")
			addOutputResult("autoupdate", "root-relative config: "+tc.name,
				pyOut == goOut && pyErr == goErr,
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		})
	}

	t.Run("run --files from sub/", func(t *testing.T) {
		pyRoot, goRoot := chdirRepo(t), chdirRepo(t)
		args := []string{"run", "echo", "--verbose", "--color=never", "-c", "../alt.yaml", "--files", "f.txt"}
		pyOut, pyExit := runCmd(t, filepath.Join(pyRoot, "sub"), pyBin, args...)
		goOut, goExit := runCmd(t, filepath.Join(goRoot, "sub"), goBinary, args...)
		addExitResult("run", "--files from sub/", pyExit, goExit, pyExit == goExit && goExit == 0, "")
		// The hook echoes the filenames it was given, which upstream makes
		// root-relative.
		pyOK, goOK := strings.Contains(pyOut, "\nsub/f.txt\n"), strings.Contains(goOut, "\nsub/f.txt\n")
		addOutputResult("run", "--files from sub/ reach the hook root-relative", pyOK && goOK,
			fmt.Sprintf("py=%q go=%q", pyOut, goOut))
	})

	t.Run("install -c ../alt.yaml from sub/", func(t *testing.T) {
		pyRoot, goRoot := chdirRepo(t), chdirRepo(t)
		args := []string{"install", "-c", "../alt.yaml"}
		_, pyExit := runCmd(t, filepath.Join(pyRoot, "sub"), pyBin, args...)
		_, goExit := runCmd(t, filepath.Join(goRoot, "sub"), goBinary, args...)
		addExitResult("install", "from sub/", pyExit, goExit, pyExit == goExit && goExit == 0, "")
		// The hook script records the config it was installed with.
		pyArgs, goArgs := argsLine(readFile(filepath.Join(pyRoot, ".git", "hooks", "pre-commit"))),
			argsLine(readFile(filepath.Join(goRoot, ".git", "hooks", "pre-commit")))
		addFSResult("install", "hook records the root-relative config",
			pyArgs == goArgs && pyArgs != "", fmt.Sprintf("py=%q go=%q", pyArgs, goArgs))
	})

	// Outside a repository, and inside .git, upstream's get_root fails.
	for _, tc := range []struct{ name, from string }{
		{"inside .git", ".git"},
		{"outside a repository", ""},
	} {
		t.Run("run "+tc.name, func(t *testing.T) {
			pyDir, goDir := t.TempDir(), t.TempDir()
			if tc.from != "" {
				pyDir, goDir = filepath.Join(chdirRepo(t), tc.from), filepath.Join(chdirRepo(t), tc.from)
			} else {
				t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(pyDir)+string(os.PathListSeparator)+filepath.Dir(goDir))
			}
			pyOut, pyErr, pyExit := runSplit(t, pyDir, pyBin, "run")
			goOut, goErr, goExit := runSplit(t, goDir, goBinary, "run")
			addExitResult("run", tc.name, pyExit, goExit, pyExit == goExit && goExit == 1, "")
			// The second line points at the log, in the shared store.
			pyLine, _, _ := strings.Cut(pyOut, "\n")
			goLine, _, _ := strings.Cut(goOut, "\n")
			addOutputResult("run", tc.name+" reports upstream's FatalError",
				pyLine == goLine && pyErr == goErr && strings.HasPrefix(goLine, "An error has occurred: FatalError: "),
				fmt.Sprintf("py=%q/%q go=%q/%q", pyOut, pyErr, goOut, goErr))
		})
	}
}

func argsLine(script string) string {
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, "ARGS=") {
			return line
		}
	}
	return ""
}
