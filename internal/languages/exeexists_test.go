// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeTools writes executable sh scripts named names into dir.
func fakeTools(t *testing.T, dir string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// nodeLayouts are the four places upstream's exe_exists distinguishes, each
// with HOME and PATH set to an isolated tree.
func nodeLayouts(t *testing.T) map[string]func(t *testing.T) {
	t.Helper()
	return map[string]func(t *testing.T){
		// A real install outside the home directory.
		"system": func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home", "u"))
			fakeTools(t, filepath.Join(root, "usr", "bin"), "node", "npm")
			t.Setenv("PATH", filepath.Join(root, "usr", "bin"))
		},
		// asdf: ~/.asdf/shims, but a shims dir anywhere is refused.
		"shim": func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home", "u"))
			fakeTools(t, filepath.Join(root, "opt", "asdf", "shims"), "node", "npm")
			t.Setenv("PATH", filepath.Join(root, "opt", "asdf", "shims"))
		},
		// nvm and friends: a real binary, but under the home directory.
		"in-home": func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home", "u")
			t.Setenv("HOME", home+"/") // expanduser strips the slash
			fakeTools(t, filepath.Join(home, ".nvm", "bin"), "node", "npm")
			t.Setenv("PATH", filepath.Join(home, ".nvm", "bin"))
		},
		// HOME=/ (a container or service user): everything is "in" home, so
		// the home check is skipped.
		"home-is-root": func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", "/")
			fakeTools(t, filepath.Join(root, "usr", "bin"), "node", "npm")
			t.Setenv("PATH", filepath.Join(root, "usr", "bin"))
		},
		// node without npm is not a usable system install.
		"no-npm": func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home", "u"))
			fakeTools(t, filepath.Join(root, "usr", "bin"), "node")
			t.Setenv("PATH", filepath.Join(root, "usr", "bin"))
		},
	}
}

var nodeLayoutWant = map[string]string{
	"system":       "system",
	"shim":         "default",
	"in-home":      "default",
	"home-is-root": "system",
	"no-npm":       "default",
}

func TestNodeDefaultVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tools are sh scripts")
	}
	for name, setup := range nodeLayouts(t) {
		t.Run(name, func(t *testing.T) {
			setup(t)
			if got := nodeDefaultVersion(runtime.GOOS); got != nodeLayoutWant[name] {
				t.Errorf("nodeDefaultVersion = %q, want %q", got, nodeLayoutWant[name])
			}
			// nodeenv cannot do `-n system` on Windows, so upstream never
			// picks it there, whatever is on PATH.
			if got := nodeDefaultVersion("windows"); got != "default" {
				t.Errorf("nodeDefaultVersion(windows) = %q, want default", got)
			}
		})
	}
}

// The install is where the default version shows: `system` symlinks the
// host's node (-n system), `default` passes no -n so nodeenv downloads one.
// Before #119 every non-Windows default install got -n system, which on an
// asdf machine handed npm the shim's node and failed with EALLOWGIT.
func TestNodeInstallEnvironmentDefaultVersion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake tools are sh scripts")
	}
	for name, setup := range nodeLayouts(t) {
		t.Run(name, func(t *testing.T) {
			setup(t)
			// The fake nodeenv (and, for no-npm, the npm the install itself
			// needs) live inside HOME and after the layout on PATH, so
			// exeExists never counts them as a system install.
			tools := filepath.Join(os.Getenv("HOME"), "tools")
			if os.Getenv("HOME") == "/" {
				tools = t.TempDir() // the layout already has node and npm
			}
			fakeTools(t, tools, "npm")
			record := filepath.Join(t.TempDir(), "nodeenv.args")
			if err := os.WriteFile(
				filepath.Join(tools, "nodeenv"),
				[]byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+record+"\n"),
				0o755,
			); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+tools)

			prefix := mkNodePrefix(t, false, false)
			if err := (&Node{}).InstallEnvironment(prefix, "default", []string{"x@1"}); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSpace(string(raw)), "\n")
			want := []string{"--prebuilt", "--clean-src", filepath.Join(prefix, "node_env-default")}
			if nodeLayoutWant[name] == "system" {
				want = append(want, "-n", "system")
			}
			assertSliceEqual(t, args, want)
		})
	}
}

func TestExeExists(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix paths")
	}
	root := t.TempDir()
	fakeTools(t, filepath.Join(root, "bin"), "tool")
	fakeTools(t, filepath.Join(root, "shims"), "shimmed")
	fakeTools(t, filepath.Join(root, "u", "bin"), "mine")
	fakeTools(t, filepath.Join(root, "uu", "bin"), "sibling")
	noexec := filepath.Join(root, "bin", "noexec")
	if err := os.WriteFile(noexec, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "bin", "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := strings.Join([]string{
		filepath.Join(root, "bin"), filepath.Join(root, "shims"),
		filepath.Join(root, "u", "bin"), filepath.Join(root, "uu", "bin"),
	}, string(os.PathListSeparator))
	t.Setenv("PATH", path)
	t.Setenv("HOME", filepath.Join(root, "u"))

	cases := map[string]bool{
		"tool":    true,
		"shimmed": false,
		"mine":    false,
		// commonpath works on components: <root>/uu is not inside <root>/u.
		"sibling": true,
		"noexec":  false,
		"adir":    false,
		"missing": false,
	}
	for exe, want := range cases {
		if got := exeExists(exe); got != want {
			t.Errorf("exeExists(%q) = %v, want %v", exe, got, want)
		}
	}
}

func TestPyCommonPathAndDirname(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix paths")
	}
	for _, c := range []struct {
		a, b, want string
		ok         bool
	}{
		{"/a/b/c", "/a/b", "/a/b", true},
		{"/a/bc", "/a/b", "/a", true},
		{"/x", "/", "/", true},
		{"rel/x", "/a", "", false},
		{"//a/./b//c", "/a/b", "/a/b", true},
	} {
		got, ok := pyCommonPath(c.a, c.b)
		if got != c.want || ok != c.ok {
			t.Errorf("pyCommonPath(%q, %q) = %q, %v; want %q, %v", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
	for in, want := range map[string]string{"/": "/", "/a": "/", "/a/b/": "/a/b", "a": "", "//": "//"} {
		if got := pyDirname(in); got != want {
			t.Errorf("pyDirname(%q) = %q, want %q", in, got, want)
		}
	}
}
