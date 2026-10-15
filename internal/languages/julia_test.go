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

// ---------------------------------------------------------------------------
// Julia — JULIA_DEPOT_PATH (upstream 4.6.1, #104)
// ---------------------------------------------------------------------------

func TestJuliaEnvSetsDepotInsideHookEnv(t *testing.T) {
	envDir := filepath.Join("prefix", "julia_env-default")
	want := "JULIA_DEPOT_PATH=" + filepath.Join(envDir, "depot") + string(os.PathListSeparator)

	env := juliaEnv(envDir)
	if len(env) != 1 || env[0] != want {
		t.Errorf("juliaEnv(%q) = %v, want [%s]", envDir, env, want)
	}
}

// fakeJulia puts a `julia` on PATH that appends its JULIA_DEPOT_PATH to log.
func fakeJulia(t *testing.T) (log string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake julia is a shell script")
	}
	bin := t.TempDir()
	log = filepath.Join(t.TempDir(), "depot.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$JULIA_DEPOT_PATH\" >> " + log + "\n"
	if err := os.WriteFile(filepath.Join(bin, "julia"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("JULIA_DEPOT_PATH", "/shared/depot")
	return log
}

func readDepotLog(t *testing.T, log string) string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func TestJuliaInstallAndRunUseHookDepot(t *testing.T) {
	log := fakeJulia(t)
	prefix := t.TempDir()
	j := &Julia{}
	want := filepath.Join(prefix, "julia_env-default", "depot") + string(os.PathListSeparator)

	if err := j.InstallEnvironment(prefix, "default", nil); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	if got := readDepotLog(t, log); got != want {
		t.Errorf("install JULIA_DEPOT_PATH = %q, want %q", got, want)
	}

	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	code, out, err := j.Run(context.Background(), prefix, prefix, "julia hook.jl", nil, nil, "default")
	if err != nil || code != 0 {
		t.Fatalf("Run: code=%d err=%v out=%s", code, err, out)
	}
	if got := readDepotLog(t, log); got != want {
		t.Errorf("run JULIA_DEPOT_PATH = %q, want %q", got, want)
	}
}
