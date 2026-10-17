// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Golang — install env composition (upstream golang.install_environment)
// ---------------------------------------------------------------------------

func envValue(env []string, key string) (string, bool) {
	val, found := "", false
	for _, kv := range env {
		if k, v, _ := strings.Cut(kv, "="); k == key {
			val, found = v, true // last one wins, as exec does
		}
	}
	return val, found
}

func TestGoInstallEnvMatchesUpstream(t *testing.T) {
	t.Setenv("GOBIN", "/somewhere/else")
	t.Setenv("GOPATH", "/home/gopath")
	t.Setenv("GIT_DIR", "/host/.git")
	t.Setenv("GIT_SSH_COMMAND", "ssh -i key")
	t.Setenv("GOPROXY", "https://proxy.example")
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOMODCACHE", "/home/modcache")
	t.Setenv("GOTOOLCHAIN", "go1.26.4")

	env := goInstallEnv("/prefix/go_env-default")

	if v, _ := envValue(env, "GOPATH"); v != "/prefix/go_env-default" {
		t.Errorf("GOPATH = %q, want the env dir", v)
	}
	if n := slices.IndexFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "GOPATH=") }); n < 0 ||
		slices.ContainsFunc(env[n+1:], func(kv string) bool { return strings.HasPrefix(kv, "GOPATH=") }) {
		t.Errorf("env %v should carry exactly one GOPATH", env)
	}
	if v, _ := envValue(env, "GOBIN"); v != filepath.Join("/prefix/go_env-default", "bin") {
		t.Errorf("GOBIN = %q, want GOPATH/bin, where upstream's install puts the binaries", v)
	}
	if n := slices.IndexFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "GOBIN=") }); n < 0 ||
		slices.ContainsFunc(env[n+1:], func(kv string) bool { return strings.HasPrefix(kv, "GOBIN=") }) {
		t.Errorf("env %v should carry exactly one GOBIN", env)
	}
	if _, ok := envValue(env, "GIT_DIR"); ok {
		t.Error("GIT_DIR should be removed, as upstream's no_git_env does")
	}
	// Everything else reaches `go install` exactly as the caller set it.
	for k, want := range map[string]string{
		"GIT_SSH_COMMAND": "ssh -i key",
		"GOPROXY":         "https://proxy.example",
		"GOFLAGS":         "-mod=mod",
		"GOMODCACHE":      "/home/modcache",
		"GOTOOLCHAIN":     "go1.26.4",
	} {
		if v, _ := envValue(env, k); v != want {
			t.Errorf("%s = %q, want the caller's %q", k, v, want)
		}
	}
}

func TestGoInstallEnvLeavesUnsetToolchainUnset(t *testing.T) {
	// Upstream sets GOTOOLCHAIN only for a Go it downloaded; a system Go
	// install sees the caller's setting, here none at all.
	t.Setenv("GOTOOLCHAIN", "")
	os.Unsetenv("GOTOOLCHAIN")

	if v, ok := envValue(goInstallEnv("/prefix/go_env-default"), "GOTOOLCHAIN"); ok {
		t.Errorf("GOTOOLCHAIN = %q, want it left unset", v)
	}
}

// Run changes only PATH, as upstream's get_env_patch does for a system Go: a
// hook that runs the go tool must see the caller's GOPATH, not the env dir.
func TestGolangRunLeavesGOPATHAlone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	t.Setenv("GOPATH", "/caller/gopath")
	prefix := t.TempDir()

	code, out, err := (&Golang{}).Run(context.Background(), prefix, prefix,
		`sh -c 'echo "$GOPATH|$PATH"'`, nil, nil, "default")
	if err != nil || code != 0 {
		t.Fatalf("Run: code %d, err %v, out %s", code, err, out)
	}
	gopath, path, _ := strings.Cut(strings.TrimSpace(string(out)), "|")
	if gopath != "/caller/gopath" {
		t.Errorf("hook saw GOPATH=%q, want the caller's", gopath)
	}
	bin := filepath.Join(prefix, "go_env-default", "bin")
	if !strings.HasPrefix(path, bin+string(os.PathListSeparator)) {
		t.Errorf("hook PATH %q should start with %s", path, bin)
	}
}

// The install removes GOPATH/pkg afterwards, as upstream does, so the module
// cache does not stay nested inside the hook repo.
func TestGolangInstallRemovesModuleCache(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}
	prefix := t.TempDir()
	t.Setenv("GOFLAGS", "-mod=mod")
	t.Setenv("GOPROXY", "off")
	for name, body := range map[string]string{
		"go.mod":  "module example.com/hook\n\ngo 1.21\n",
		"main.go": "package main\n\nfunc main() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(prefix, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	envDir := filepath.Join(prefix, "go_env-default")
	if err := os.MkdirAll(filepath.Join(envDir, "pkg", "mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&Golang{}).InstallEnvironment(prefix, "default", nil); err != nil {
		t.Fatalf("InstallEnvironment: %v", err)
	}
	if _, err := os.Stat(filepath.Join(envDir, "pkg")); !os.IsNotExist(err) {
		t.Errorf("GOPATH/pkg should be removed after install, stat: %v", err)
	}
	bin := "hook"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if _, err := os.Stat(filepath.Join(envDir, "bin", bin)); err != nil {
		t.Errorf("hook binary not installed into GOPATH/bin: %v", err)
	}
}
