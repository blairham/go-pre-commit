// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
	"github.com/blairham/go-pre-commit/v4/internal/git"
)

// Golang implements the Language interface for Go hooks.
type Golang struct{}

func (g *Golang) Name() string              { return "golang" }
func (g *Golang) EnvironmentDir() string    { return "go_env" }
func (g *Golang) GetDefaultVersion() string { return "default" }

func (g *Golang) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil || len(entries) == 0 {
		return fmt.Errorf("golang environment unhealthy: no binaries in %s", binDir)
	}
	return nil
}

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
func goInstallEnv(envDir string) []string {
	base := git.NoGitEnv()
	env := make([]string, 0, len(base)+2)
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if k == "GOBIN" || k == "GOPATH" {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GOPATH="+envDir, "GOBIN="+filepath.Join(envDir, "bin"))
}

func (g *Golang) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)

	env := goInstallEnv(envDir)

	// Install the hook package.
	cmd := exec.Command("go", "install", "./...")
	cmd.Dir = prefix
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go install failed: %s: %w", string(out), err)
	}

	// Install additional dependencies.
	for _, dep := range additionalDeps {
		cmd := exec.Command("go", "install", dep)
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

// Run implements Language.
func (g *Golang) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	// upstream's get_env_patch for a system Go: only PATH changes. GOPATH is
	// left as the caller has it, so a hook that runs the go tool (govulncheck,
	// say) uses the caller's module cache rather than refilling the env's.
	envDir := filepath.Join(prefix, g.EnvironmentDir()+"-"+version)
	env := []string{PrependPath(filepath.Join(envDir, "bin"))}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
