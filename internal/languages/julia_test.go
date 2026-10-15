// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ported from upstream 4.7.0's pre_commit/languages/julia.py (#104, #118).

func TestJuliaEnvironmentDirIsUpstreams(t *testing.T) {
	if got := (&Julia{}).EnvironmentDir(); got != "juliaenv" {
		t.Errorf("EnvironmentDir() = %q, want upstream's %q", got, "juliaenv")
	}
}

func TestJuliaEnvMatchesUpstreamEnvPatch(t *testing.T) {
	envDir := filepath.Join("prefix", "juliaenv-default")
	want := []string{
		"JULIA_LOAD_PATH=" + envDir,
		"JULIA_DEPOT_PATH=" + filepath.Join(envDir, "depot") + string(os.PathListSeparator),
	}
	got := juliaEnv(envDir)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("juliaEnv(%q) = %v, want %v", envDir, got, want)
	}
	if len(juliaUnset) != 1 || juliaUnset[0] != "JULIA_PROJECT" {
		t.Errorf("juliaUnset = %v, want [JULIA_PROJECT]", juliaUnset)
	}
}

// fakeJulia puts a `julia` on PATH that logs one line per argument, then its
// JULIA_LOAD_PATH, JULIA_DEPOT_PATH and JULIA_PROJECT (or <unset>). Newlines
// inside an argument are logged as \x1c so the install script stays one line.
func fakeJulia(t *testing.T) (log string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake julia is a shell script")
	}
	bin := t.TempDir()
	log = filepath.Join(t.TempDir(), "julia.log")
	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do printf 'arg=%s\\n' \"$(printf %s \"$a\" | tr '\\n' '\\034')\"; done >> " + log + "\n" +
		"printf 'load=%s\\ndepot=%s\\nproject=%s\\n' \"$JULIA_LOAD_PATH\" \"$JULIA_DEPOT_PATH\" \"${JULIA_PROJECT-<unset>}\" >> " + log + "\n"
	if err := os.WriteFile(filepath.Join(bin, "julia"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("JULIA_DEPOT_PATH", "/shared/depot")
	t.Setenv("JULIA_LOAD_PATH", "/shared/load")
	t.Setenv("JULIA_PROJECT", "/shared/project")
	return log
}

// readJuliaLog returns the logged arguments (dropping the long install
// script) and the logged env lines, then removes the log.
func readJuliaLog(t *testing.T, log string) (args, env []string) {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		switch {
		case strings.HasPrefix(line, "arg="):
			args = append(args, strings.TrimPrefix(line, "arg="))
		case strings.HasPrefix(line, "load="), strings.HasPrefix(line, "depot="), strings.HasPrefix(line, "project="):
			env = append(env, line)
		}
	}
	return args, env
}

func wantJuliaEnv(envDir string) []string {
	return []string{
		"load=" + envDir,
		"depot=" + filepath.Join(envDir, "depot") + string(os.PathListSeparator),
		"project=<unset>",
	}
}

func TestJuliaRunPassesScriptAndPatchesEnv(t *testing.T) {
	log := fakeJulia(t)
	prefix := t.TempDir()
	work := t.TempDir()
	envDir := filepath.Join(prefix, "juliaenv-default")
	j := &Julia{}

	for _, tc := range []struct {
		name   string
		script string
		local  bool
	}{
		{name: "remote hook script is relative to the hook repo", script: filepath.Join(prefix, "bin", "hook.jl")},
		{name: "local hook script is left as written", local: true, script: filepath.Join("bin", "hook.jl")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithLocalHook(context.Background(), tc.local)
			code, out, err := j.Run(
				ctx,
				prefix,
				work,
				"bin/hook.jl --arg1",
				[]string{"--arg2"},
				[]string{"a.jl", "b.jl"},
				"default",
			)
			if err != nil || code != 0 {
				t.Fatalf("Run: code=%d err=%v out=%s", code, err, out)
			}
			args, env := readJuliaLog(t, log)
			want := []string{"--startup-file=no", tc.script, "--arg1", "--arg2", "a.jl", "b.jl"}
			if strings.Join(args, "\n") != strings.Join(want, "\n") {
				t.Errorf("julia args = %q, want %q", args, want)
			}
			if strings.Join(env, "\n") != strings.Join(wantJuliaEnv(envDir), "\n") {
				t.Errorf("julia env = %q, want %q", env, wantJuliaEnv(envDir))
			}
		})
	}
}

func TestJuliaRunWithoutLocalMarkIsRemote(t *testing.T) {
	log := fakeJulia(t)
	prefix := t.TempDir()
	if _, _, err := (&Julia{}).Run(context.Background(), prefix, t.TempDir(), "hook.jl", nil, nil, "default"); err != nil {
		t.Fatal(err)
	}
	args, _ := readJuliaLog(t, log)
	if len(args) < 2 || args[1] != filepath.Join(prefix, "hook.jl") {
		t.Errorf("julia args = %q, want the script joined to the prefix", args)
	}
}

func TestJuliaInstallEnvironment(t *testing.T) {
	log := fakeJulia(t)

	t.Run("copies the project and manifest", func(t *testing.T) {
		prefix := t.TempDir()
		envDir := filepath.Join(prefix, "juliaenv-default")
		files := map[string]string{
			"JuliaProject.toml": "name = \"A\"\n",
			"Project.toml":      "name = \"ignored\"\n",
			"Manifest.toml":     "# manifest\n",
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(prefix, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := (&Julia{}).InstallEnvironment(prefix, "default", []string{"Example", "JSON@0.21"}); err != nil {
			t.Fatalf("InstallEnvironment: %v", err)
		}
		for name, want := range map[string]string{"JuliaProject.toml": files["JuliaProject.toml"], "Manifest.toml": files["Manifest.toml"]} {
			got, err := os.ReadFile(filepath.Join(envDir, name))
			if err != nil || string(got) != want {
				t.Errorf("%s in env = %q (%v), want %q", name, got, err, want)
			}
		}
		if _, err := os.Stat(filepath.Join(envDir, "Project.toml")); err == nil {
			t.Error("Project.toml copied although JuliaProject.toml was found first")
		}

		args, env := readJuliaLog(t, log)
		wantArgs := []string{
			"--startup-file=no", "-e", strings.ReplaceAll(juliaInstallScript, "\n", "\x1c"),
			"--", envDir, "Example", "JSON@0.21",
		}
		if strings.Join(args, "\n") != strings.Join(wantArgs, "\n") {
			t.Errorf("install args = %q, want %q", args, wantArgs)
		}
		if strings.Join(env, "\n") != strings.Join(wantJuliaEnv(envDir), "\n") {
			t.Errorf("install env = %q, want %q", env, wantJuliaEnv(envDir))
		}
	})

	t.Run("writes an empty Project.toml when the hook has none", func(t *testing.T) {
		prefix := t.TempDir()
		if err := (&Julia{}).InstallEnvironment(prefix, "default", nil); err != nil {
			t.Fatalf("InstallEnvironment: %v", err)
		}
		info, err := os.Stat(filepath.Join(prefix, "juliaenv-default", "Project.toml"))
		if err != nil || info.Size() != 0 {
			t.Errorf("empty Project.toml: info=%v err=%v", info, err)
		}
		args, _ := readJuliaLog(t, log)
		if len(args) != 5 {
			t.Errorf("install args without deps = %q, want 5", args)
		}
	})
}
