// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	env := goInstallEnv("/prefix/golangenv-default", "system")

	if v, _ := envValue(env, "GOPATH"); v != "/prefix/golangenv-default" {
		t.Errorf("GOPATH = %q, want the env dir", v)
	}
	if n := slices.IndexFunc(env, func(kv string) bool { return strings.HasPrefix(kv, "GOPATH=") }); n < 0 ||
		slices.ContainsFunc(env[n+1:], func(kv string) bool { return strings.HasPrefix(kv, "GOPATH=") }) {
		t.Errorf("env %v should carry exactly one GOPATH", env)
	}
	if v, _ := envValue(env, "GOBIN"); v != filepath.Join("/prefix/golangenv-default", "bin") {
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

	if v, ok := envValue(goInstallEnv("/prefix/golangenv-default", "system"), "GOTOOLCHAIN"); ok {
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
		`sh -c 'echo "$GOPATH|$PATH"'`, nil, nil, "system")
	if err != nil || code != 0 {
		t.Fatalf("Run: code %d, err %v, out %s", code, err, out)
	}
	gopath, path, _ := strings.Cut(strings.TrimSpace(string(out)), "|")
	if gopath != "/caller/gopath" {
		t.Errorf("hook saw GOPATH=%q, want the caller's", gopath)
	}
	bin := filepath.Join(prefix, "golangenv-system", "bin")
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
	envDir := filepath.Join(prefix, "golangenv-system")
	if err := os.MkdirAll(filepath.Join(envDir, "pkg", "mod"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (&Golang{}).InstallEnvironment(prefix, "system", nil); err != nil {
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

// goDistServer serves a go.dev release list naming version and a Go
// distribution whose go/bin/go is a script that records its environment and
// "installs" by writing $GOBIN/hook.
func goDistServer(t *testing.T, version string) (record string) {
	t.Helper()
	record = filepath.Join(t.TempDir(), "go.env")
	goScript := "#!/bin/sh\n" +
		"echo \"$0|$GOROOT|$GOTOOLCHAIN|$GOPATH|$*\" >> " + record + "\n" +
		"mkdir -p \"$GOBIN\" && : > \"$GOBIN/hook\" && chmod +x \"$GOBIN/hook\"\n"
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, d := range []string{"go/", "go/bin/"} {
		if err := tw.WriteHeader(&tar.Header{Name: d, Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.WriteHeader(&tar.Header{
		Name: "go/bin/go", Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(goScript)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(goScript)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archive := fmt.Sprintf("/go%s.%s-%s.tar.gz", version, runtime.GOOS, goArch(runtime.GOARCH))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dl/":
			fmt.Fprintf(w, `[{"version": "go%s"}, {"version": "go1.0.0"}]`, version)
		case archive:
			_, _ = w.Write(buf.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	oldList, oldDL := goVersionsURL, goDownloadURL
	goVersionsURL, goDownloadURL = srv.URL+"/dl/?mode=json", srv.URL+"/"
	t.Cleanup(func() { goVersionsURL, goDownloadURL = oldList, oldDL })
	return record
}

// Without a system Go, `default` downloads the newest Go into <env>/.go and
// installs with it, under GOTOOLCHAIN=local and GOROOT set, as upstream's
// install_environment does; an explicit version downloads that version.
func TestGolangInstallDownloadsGo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake go is a sh script")
	}
	for _, c := range []struct{ version, serve string }{
		{"default", "1.99.1"},
		{"1.98.0", "1.98.0"},
	} {
		t.Run(c.version, func(t *testing.T) {
			withDefaultVersion(t, &golangDefaultVersionOnce, "default")
			record := goDistServer(t, c.serve)
			prefix := t.TempDir()
			if err := (&Golang{}).InstallEnvironment(prefix, c.version, []string{"example.com/dep@v1"}); err != nil {
				t.Fatalf("InstallEnvironment: %v", err)
			}
			envDir := filepath.Join(prefix, "golangenv-"+c.version)
			goroot := filepath.Join(envDir, ".go")
			raw, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			want := []string{
				filepath.Join(goroot, "bin", "go") + "|" + goroot + "|local|" + envDir + "|install ./...",
				filepath.Join(goroot, "bin", "go") + "|" + goroot + "|local|" + envDir + "|install example.com/dep@v1",
			}
			assertSliceEqual(t, lines, want)
			if _, err := os.Stat(filepath.Join(envDir, "go")); !os.IsNotExist(err) {
				t.Errorf("the download should be moved to .go, not left at go (%v)", err)
			}
		})
	}
}

func TestGolangInstallUnknownPlatform(t *testing.T) {
	goDistServer(t, "1.99.1")
	err := (&Golang{}).InstallEnvironment(t.TempDir(), "1.2.3", nil)
	want := fmt.Sprintf(
		"Could not find a version matching your system requirements (os=%s; arch=%s)",
		runtime.GOOS, goArch(runtime.GOARCH),
	)
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

// upstream's get_env_patch: a system Go changes only PATH; any other puts
// the env's bin and then the downloaded Go's on PATH, with GOROOT and
// GOTOOLCHAIN=local.
func TestGoEnvPatch(t *testing.T) {
	t.Setenv("PATH", "/usr/bin")
	sep := string(os.PathListSeparator)
	env := "/p/golangenv-x"
	assertSliceEqual(t, goEnvPatch(env, "system"), []string{
		"PATH=" + filepath.Join(env, "bin") + sep + "/usr/bin",
	})
	for _, v := range []string{"default", "1.22.0"} {
		assertSliceEqual(t, goEnvPatch(env, v), []string{
			"GOROOT=" + filepath.Join(env, ".go"),
			"GOTOOLCHAIN=local",
			"PATH=" + filepath.Join(env, "bin") + sep + filepath.Join(env, ".go", "bin") + sep + "/usr/bin",
		})
	}
}

func TestGoInstallEnvDownloadedGo(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "auto")
	env := goInstallEnv("/p/golangenv-default", "default")
	if v, _ := envValue(env, "GOTOOLCHAIN"); v != "local" {
		t.Errorf("GOTOOLCHAIN = %q, want local for a downloaded Go", v)
	}
	if v, _ := envValue(env, "GOROOT"); v != filepath.Join("/p/golangenv-default", ".go") {
		t.Errorf("GOROOT = %q, want the download", v)
	}
	if v, _ := envValue(env, "PATH"); !strings.HasPrefix(v, filepath.Join("/p/golangenv-default", ".go", "bin")) {
		t.Errorf("PATH = %q, want the download's bin first", v)
	}
}

func TestGoArch(t *testing.T) {
	for in, want := range map[string]string{"amd64": "amd64", "arm64": "arm64", "386": "386", "arm": "armv6l"} {
		if got := goArch(in); got != want {
			t.Errorf("goArch(%q) = %q, want %q", in, got, want)
		}
	}
}

// withDefaultVersion pins a backend's cached default version for one test,
// so the layout of the machine running it does not decide the branch taken.
func withDefaultVersion(t *testing.T, fn *func() string, v string) {
	t.Helper()
	old := *fn
	*fn = func() string { return v }
	t.Cleanup(func() { *fn = old })
}
