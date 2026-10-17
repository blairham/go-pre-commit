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

// Upstream gives every language but node, python and r the
// basic_health_check, which never asks for a rebuild: with nothing on PATH
// at all, an installed env still counts as healthy, and a missing toolchain
// shows when the hook runs instead.
func TestBasicHealthChecks(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	prefix := t.TempDir()
	for _, name := range []string{
		"conda", "coursier", "dart", "docker", "docker_image", "dotnet", "fail",
		"golang", "haskell", "julia", "lua", "perl", "pygrep", "ruby", "rust",
		"swift", "system", "script",
	} {
		lang, err := Get(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, version := range []string{"default", "system", "1.2.3"} {
			if err := lang.HealthCheck(prefix, version); err != nil {
				t.Errorf("%s HealthCheck(%s) = %v, want healthy", name, version, err)
			}
		}
	}
}

// node.health_check runs `node --version` inside the env.
func TestNodeHealthCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake node is a sh script")
	}
	t.Setenv("PATH", t.TempDir())
	prefix := t.TempDir()
	bin := filepath.Join(prefix, "node_env-system", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	n := &Node{}
	if err := n.HealthCheck(prefix, "system"); err == nil || err.Error() != "`node --version` returned 1" {
		t.Errorf("no node at all: %v", err)
	}
	write := func(code string) {
		if err := os.WriteFile(filepath.Join(bin, "node"), []byte("#!/bin/sh\nexit "+code+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write("3")
	if err := n.HealthCheck(prefix, "system"); err == nil || err.Error() != "`node --version` returned 3" {
		t.Errorf("failing node: %v", err)
	}
	write("0")
	if err := n.HealthCheck(prefix, "system"); err != nil {
		t.Errorf("working node: %v", err)
	}
}

// python.health_check: pyvenv.cfg and the versions it records.
func TestPythonHealthCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake pythons are sh scripts")
	}
	fakePython := func(t *testing.T, path, version string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("#!/bin/sh\necho "+version+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		name, cfg, envPy, basePy, want string
	}{
		{name: "no cfg", want: "pyvenv.cfg does not exist (old virtualenv?)"},
		{name: "no version", cfg: "home = /usr/bin\n", want: "created virtualenv's pyvenv.cfg is missing `version_info`"},
		{name: "healthy", cfg: "version_info = 3.12.1.final.0\n", envPy: "3.12.1.final.0"},
		{
			name: "env python changed", cfg: "version_info = 3.12.1.final.0\n", envPy: "3.12.2.final.0",
			want: "virtualenv python version did not match created version:\n" +
				"- actual version: 3.12.2.final.0\n- expected version: 3.12.1.final.0\n",
		},
		{name: "base healthy", cfg: "version_info = 3.12.1.final.0\nbase-executable = BASE\n", envPy: "3.12.1.final.0", basePy: "3.12.1.final.0"},
		{
			name: "base changed", cfg: "version_info = 3.12.1.final.0\nbase-executable = BASE\n", envPy: "3.12.1.final.0", basePy: "3.13.0.final.0",
			want: "base executable python version does not match created version:\n" +
				"- base-executable version: 3.13.0.final.0\n- expected version: 3.12.1.final.0\n",
		},
		{name: "stdlib venv", cfg: "home = /usr/bin\nversion = 3.12.1\n", envPy: "3.12.1.final.0"},
		{
			name: "stdlib venv changed", cfg: "version = 3.12.1\n", envPy: "3.12.2.final.0",
			want: "virtualenv python version did not match created version:\n" +
				"- actual version: 3.12.2\n- expected version: 3.12.1\n",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			prefix := t.TempDir()
			env := filepath.Join(prefix, "py_env-python3.12")
			base := filepath.Join(t.TempDir(), "python3.12")
			if c.cfg != "" {
				if err := os.MkdirAll(env, 0o755); err != nil {
					t.Fatal(err)
				}
				cfg := []byte(strings.ReplaceAll(c.cfg, "BASE", base))
				if err := os.WriteFile(filepath.Join(env, "pyvenv.cfg"), cfg, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if c.envPy != "" {
				fakePython(t, filepath.Join(env, "bin", "python"), c.envPy)
			}
			if c.basePy != "" {
				fakePython(t, base, c.basePy)
			}
			err := (&Python{}).HealthCheck(prefix, "python3.12")
			got := ""
			if err != nil {
				got = err.Error()
			}
			if got != c.want {
				t.Errorf("HealthCheck = %q, want %q", got, c.want)
			}
		})
	}
}
