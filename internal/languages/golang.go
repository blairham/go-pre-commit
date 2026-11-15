// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
	"github.com/blairham/go-pre-commit/v4/internal/git"
)

// Golang implements the Language interface for Go hooks.
type Golang struct{}

// Name is the language's name in a hook manifest.
func (g *Golang) Name() string { return "golang" }

// EnvironmentDir is the env directory's name before its version.
func (g *Golang) EnvironmentDir() string { return "golangenv" }

// GetDefaultVersion is upstream golang.get_default_version (4.7.0), cached
// for the process as upstream's lru_cache does: "system" when the host's go
// is a real install (exeExists: on PATH, not a version manager's shim, not
// inside the home directory), otherwise "default", for which the install
// downloads the latest Go.
func (g *Golang) GetDefaultVersion() string { return golangDefaultVersionOnce() }

var golangDefaultVersionOnce = sync.OnceValue(golangDefaultVersion)

func golangDefaultVersion() string {
	if exeExists("go") {
		return VersionSystem
	}
	return VersionDefault
}

// effectiveVersion is the version the install and run act on. A
// `default` is resolved through GetDefaultVersion, as upstream resolves it
// before the hook reaches the backend; the env directory keeps the name it
// was given.
func (g *Golang) effectiveVersion(version string) string {
	if version == VersionDefault {
		return g.GetDefaultVersion()
	}
	return version
}

// HealthCheck is upstream's lang_base.basic_health_check: an installed
// environment is always healthy. A missing toolchain shows when the hook runs.
func (g *Golang) HealthCheck(_, _ string) error { return nil }

// goInstallEnv is the environment upstream's golang.install_environment
// (4.7.0) gives `go install` for a system Go: the caller's environment
// without git's variables (no_git_env) and GOPATH pointed at the hook env.
// Nothing else is touched — GOPROXY, GOFLAGS, GOMODCACHE and GOTOOLCHAIN
// reach `go install` exactly as the caller set them, so it resolves modules
// the way the same command run by hand does.
//
// Upstream removes GOBIN so the binaries land in GOPATH/bin. This sets GOBIN
// to GOPATH/bin instead, which is the same directory, because removing it is
// not enough wherever something else supplies one: asdf's go shim exports
// GOBIN into the go command when it is unset, and `go env -w GOBIN` lives in
// a file the environment cannot remove. Either one sends the binaries out of
// the env, and the hook's entry is then not found.
//
// For a Go the install downloaded (any version but "system"), upstream also
// sets GOTOOLCHAIN=local, so a hook repo's go.mod cannot switch toolchains,
// GOROOT to the download, and puts the download's bin first on PATH.
func goInstallEnv(envDir, version string) []string {
	set := []string{"GOPATH=" + envDir, "GOBIN=" + filepath.Join(envDir, "bin")}
	if version != VersionSystem {
		goroot := filepath.Join(envDir, ".go")
		set = append(set,
			"GOTOOLCHAIN=local",
			"GOROOT="+goroot,
			PrependPath(filepath.Join(goroot, "bin")),
		)
	}
	// Each key appears once, so whatever reads the list — exec, or
	// lookPathInEnv resolving go on PATH — sees the value set here.
	keys := make([]string, 0, len(set))
	for _, kv := range set {
		k, _, _ := strings.Cut(kv, "=")
		keys = append(keys, k)
	}
	return append(environWithout(git.NoGitEnv(), keys), set...)
}

// goEnvPatch is upstream's golang.get_env_patch: a system Go changes only
// PATH; a downloaded one also pins GOROOT and GOTOOLCHAIN=local and puts its
// own bin on PATH after the hook's.
func goEnvPatch(envDir, version string) []string {
	if version == VersionSystem {
		return []string{PrependPath(filepath.Join(envDir, "bin"))}
	}
	goroot := filepath.Join(envDir, ".go")
	return []string{
		"GOROOT=" + goroot,
		"GOTOOLCHAIN=local",
		"PATH=" + strings.Join([]string{
			filepath.Join(envDir, "bin"), filepath.Join(goroot, "bin"), os.Getenv("PATH"),
		}, string(os.PathListSeparator)),
	}
}

// Where the Go download comes from; variables so tests can serve their own.
var (
	goVersionsURL = "https://go.dev/dl/?mode=json"
	goDownloadURL = "https://dl.google.com/go/"
)

// goArch is upstream's _ARCH: platform.machine() lowered and aliased to Go's
// names, which runtime.GOARCH already is, save 32-bit ARM, whose downloads
// are armv6l.
func goArch(goarch string) string {
	if goarch == "arm" {
		return "armv6l"
	}
	return goarch
}

// inferGoVersion is upstream's _infer_go_version: an explicit version is
// used as is; "default" is the newest release go.dev lists.
func inferGoVersion(version string) (_ string, err error) {
	if version != VersionDefault {
		return version, nil
	}
	resp, err := httpGet(goVersionsURL)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", goVersionsURL, resp.Status)
	}
	var releases []struct {
		Version string `json:"version"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&releases); decodeErr != nil {
		return "", decodeErr
	}
	if len(releases) == 0 {
		return "", fmt.Errorf("GET %s: no releases listed", goVersionsURL)
	}
	return strings.TrimPrefix(releases[0].Version, "go"), nil
}

// installGo is upstream's _install_go: download the distribution for this
// OS and architecture and unpack it as <env>/.go.
func installGo(version, envDir string) (err error) {
	version, err = inferGoVersion(version)
	if err != nil {
		return err
	}
	ext := "tar.gz"
	if runtime.GOOS == goosWindows {
		ext = "zip"
	}
	arch := goArch(runtime.GOARCH)
	url := fmt.Sprintf("%sgo%s.%s-%s.%s", goDownloadURL, version, runtime.GOOS, arch, ext)
	resp, err := httpGet(url)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf( //nolint:staticcheck // upstream's wording, capital and all
			"Could not find a version matching your system requirements (os=%s; arch=%s)",
			runtime.GOOS, arch,
		)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	if mkErr := os.MkdirAll(envDir, 0o755); mkErr != nil { // #nosec G301 -- as upstream's os.makedirs
		return mkErr
	}
	if ext == "zip" {
		err = extractZipStream(resp.Body, envDir)
	} else {
		err = extractTarGz(resp.Body, envDir)
	}
	if err != nil {
		return err
	}
	return os.Rename(filepath.Join(envDir, "go"), filepath.Join(envDir, ".go"))
}

// extractZipStream spools a zip download to a temporary file, which the zip
// format needs, and unpacks it into dest.
func extractZipStream(r io.Reader, dest string) (err error) {
	tmp, err := os.CreateTemp("", "pre-commit-go-*.zip")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.Remove(tmp.Name())) }()
	_, copyErr := io.Copy(tmp, r)
	if spoolErr := errors.Join(copyErr, tmp.Close()); spoolErr != nil {
		return spoolErr
	}
	return extractZip(tmp.Name(), dest)
}

func (g *Golang) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)
	version = g.effectiveVersion(version)

	if version != VersionSystem {
		if err := installGo(version, envDir); err != nil {
			return err
		}
	}

	env := goInstallEnv(envDir, version)

	// The go is the one on the install env's PATH — a downloaded Go's, when
	// there is one — as upstream's setup_cmd resolves it.
	goBin, err := lookPathInEnv("go", env)
	if err != nil {
		goBin = "go"
	}

	if err := removeLegacyGoEnvs(prefix); err != nil {
		return err
	}

	// Install the hook package.
	cmd := command(goBin, "install", "./...")
	cmd.Dir = prefix
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go install failed: %s: %w", string(out), err)
	}

	// Install additional dependencies.
	for _, dep := range additionalDeps {
		cmd := command(goBin, "install", dep)
		cmd.Dir = prefix
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go install %s failed: %s: %w", dep, string(out), err)
		}
	}

	// As upstream: the module cache under GOPATH/pkg is not needed once the
	// binaries are built, so it is removed to save disk space.
	return fsutil.RemoveAll(filepath.Join(envDir, "pkg"))
}

// legacyGoEnvDir is the env directory's name before 4.7.1 took upstream's.
const legacyGoEnvDir = "go_env"

// removeLegacyGoEnvs removes the env dirs this port's releases before 4.7.1
// left in a hook's clone, which no release since reads: go_env-<version>, and
// the bare go_env that held their install state through 4.6.4. They are not
// inert. Those releases also kept the module cache in GOPATH/pkg, and `go
// install ./...` runs in the clone, so its pattern walks into that cache and
// refuses any module there with no go.mod of its own ("directory
// go_env-default/pkg/mod/... outside main module or its selected
// dependencies"). The first install after an upgrade failed that way until
// the dirs were deleted by hand. No upstream language names its env go_env,
// so a store shared with Python pre-commit loses nothing.
//
// Only directories with those names are removed, and only in the clone.
// Another version's golangenv is left alone even when a crashed install left
// its pkg behind, as upstream leaves it: one clone's versions install in
// parallel here, so that pkg may be a cache another install is filling. The
// env being installed was already cleared by hook.InstallEnvironments, as by
// upstream's _hook_install. A module cache is read-only, hence
// fsutil.RemoveAll.
func removeLegacyGoEnvs(prefix string) error {
	entries, err := os.ReadDir(prefix)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || (name != legacyGoEnvDir && !strings.HasPrefix(name, legacyGoEnvDir+"-")) {
			continue
		}
		path := filepath.Join(prefix, name)
		if err := fsutil.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s, an env from an older release: %w", path, err)
		}
	}
	return nil
}

// Run implements Language.
func (g *Golang) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	// upstream's get_env_patch. GOPATH is left as the caller has it, so a
	// hook that runs the go tool (govulncheck, say) uses the caller's module
	// cache rather than refilling the env's.
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)
	env := goEnvPatch(envDir, g.effectiveVersion(version))
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
