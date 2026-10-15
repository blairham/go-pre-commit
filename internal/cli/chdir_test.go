// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/git"
)

// tempRepo makes a git repository with a sub/ directory and alt.yaml and
// hooks/ at its root, and returns its physical path -- the form
// os.Getwd-after-chdir and Python's os.getcwd both report.
func tempRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	for _, d := range []string{"sub", "hooks"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "alt.yaml"), []byte("repos: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// Expectations are pre-commit 4.7.0's, measured by running it from the same
// directories with the same arguments.
func TestAdjustArgsAndChdir(t *testing.T) {
	cases := []struct {
		name       string
		from       string
		config     string
		wantConfig string
		msg        string
		wantMsg    string
		repo       string
		wantRepo   string
		files      []string
		wantFiles  []string
	}{
		{
			name:       "default config from sub",
			from:       "sub",
			config:     ".pre-commit-config.yaml",
			wantConfig: ".pre-commit-config.yaml",
		},
		{name: "dot-relative config at root", from: ".", config: "./alt.yaml", wantConfig: "alt.yaml"},
		{name: "parent-relative config from sub", from: "sub", config: "../alt.yaml", wantConfig: "alt.yaml"},
		{name: "absolute config from sub", from: "sub", config: "ROOT/alt.yaml", wantConfig: "alt.yaml"},
		// A config that does not exist from here is never made absolute, so
		// it is looked for relative to the root instead.
		{name: "missing config from sub", from: "sub", config: "nope.yaml", wantConfig: "nope.yaml"},
		{name: "missing parent config from sub", from: "sub", config: "../nope.yaml", wantConfig: "../nope.yaml"},
		{
			name: "files and commit message from sub", from: "sub", config: "../alt.yaml", wantConfig: "alt.yaml",
			files: []string{"f.txt", "../g.txt", "ROOT/h.txt"}, wantFiles: []string{"sub/f.txt", "g.txt", "h.txt"},
			msg: "MSG", wantMsg: "sub/MSG",
		},
		{
			name:       "local try-repo path from sub",
			from:       "sub",
			config:     "x",
			wantConfig: "x",
			repo:       "../hooks",
			wantRepo:   "hooks",
		},
		{
			name:       "remote try-repo url",
			from:       "sub",
			config:     "x",
			wantConfig: "x",
			repo:       "https://example.com/r",
			wantRepo:   "https://example.com/r",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tempRepo(t)
			sub := func(s string) string { return strings.ReplaceAll(s, "ROOT", root) }
			t.Chdir(filepath.Join(root, tc.from))

			config, msg, repo := sub(tc.config), tc.msg, sub(tc.repo)
			var files []string
			for _, f := range tc.files {
				files = append(files, sub(f))
			}
			a := rootArgs{Config: &config, Files: files, CommitMsgFilename: &msg}
			if tc.repo != "" {
				a.Repo = &repo
			}
			if err := adjustArgsAndChdir(a); err != nil {
				t.Fatal(err)
			}

			if config != tc.wantConfig {
				t.Errorf("config = %q, want %q", config, tc.wantConfig)
			}
			for i := range files {
				if files[i] != tc.wantFiles[i] {
					t.Errorf("files[%d] = %q, want %q", i, files[i], tc.wantFiles[i])
				}
			}
			if msg != tc.wantMsg {
				t.Errorf("commit message = %q, want %q", msg, tc.wantMsg)
			}
			if repo != tc.wantRepo {
				t.Errorf("repo = %q, want %q", repo, tc.wantRepo)
			}
			if wd, _ := physicalWd(); wd != root {
				t.Errorf("cwd = %q, want the root %q", wd, root)
			}
		})
	}
}

// A path the user gives absolute through a symlinked directory is left
// unresolved, while the root is physical, so upstream prints a path that
// climbs out and back in. Measured: `-c <link>/alt.yaml` from <link>/sub
// prints `../<link>/alt.yaml`.
func TestAdjustArgsAndChdir_SymlinkedAbsoluteConfig(t *testing.T) {
	root := tempRepo(t)
	link := filepath.Join(filepath.Dir(root), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skip("cannot symlink:", err)
	}
	t.Chdir(filepath.Join(link, "sub"))

	config := filepath.Join(link, "alt.yaml")
	if err := adjustArgsAndChdir(rootArgs{Config: &config}); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("..", "link", "alt.yaml"); config != want {
		t.Errorf("config = %q, want %q", config, want)
	}
}

func TestAdjustArgsAndChdir_Errors(t *testing.T) {
	t.Run("outside a repository", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
		t.Chdir(dir)
		config := "x"
		if err := adjustArgsAndChdir(rootArgs{Config: &config}); !errors.Is(err, git.ErrNoRepository) {
			t.Errorf("err = %v, want %v", err, git.ErrNoRepository)
		}
	})
	t.Run("inside .git", func(t *testing.T) {
		root := tempRepo(t)
		t.Chdir(filepath.Join(root, ".git"))
		config := "x"
		if err := adjustArgsAndChdir(rootArgs{Config: &config}); !errors.Is(err, git.ErrInsideGitDir) {
			t.Errorf("err = %v, want %v", err, git.ErrInsideGitDir)
		}
	})
}

func TestWriteFatalError(t *testing.T) {
	dir := t.TempDir()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	writeFatalError(dir, git.ErrInsideGitDir.Error())
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)

	logPath := filepath.Join(dir, "pre-commit.log")
	want := "An error has occurred: FatalError: git toplevel unexpectedly empty! make sure you are " +
		"not inside the `.git` directory of your repository.\n" +
		"Check the log at " + logPath + "\n"
	if string(out) != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "### error information") ||
		!strings.Contains(string(log), "FatalError: git toplevel") {
		t.Errorf("log does not record the error:\n%s", log)
	}
}

// The case #109 was filed for: from a subdirectory, the config at the root
// is found, and a `../`-relative --config is reported root-relative.
func TestAutoupdateFromSubdirectory(t *testing.T) {
	root := tempRepo(t)
	const cfg = "repos:\n-   repo: local\n    hooks:\n    -   id: x\n        name: x\n        entry: x\n        language: system\n"
	for _, name := range []string{".pre-commit-config.yaml", "alt.yaml"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(cfg), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		want string
		args []string
	}{
		{want: "repos (q) were not found in .pre-commit-config.yaml\n"},
		{args: []string{"-c", "../alt.yaml"}, want: "repos (q) were not found in alt.yaml\n"},
	} {
		t.Chdir(filepath.Join(root, "sub"))
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		orig := os.Stdout
		os.Stdout = w
		code := (&AutoupdateCommand{}).Run(append([]string{"--color=never", "--repo", "q"}, tc.args...))
		w.Close()
		os.Stdout = orig
		got, _ := io.ReadAll(r)

		if code != 1 || string(got) != tc.want {
			t.Errorf("autoupdate %v from sub/: exit %d, stdout %q; want exit 1, %q", tc.args, code, got, tc.want)
		}
	}
}
