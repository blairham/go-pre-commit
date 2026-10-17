// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/blairham/go-pre-commit/v4/internal/git"
)

// Node implements the Language interface for Node.js hooks.
type Node struct{}

// Name is the language's name in a hook manifest.
func (n *Node) Name() string { return "node" }

// EnvironmentDir is upstream's ENVIRONMENT_DIR.
func (n *Node) EnvironmentDir() string { return "node_env" }

// GetDefaultVersion is upstream node.get_default_version (4.7.0), cached for
// the process as upstream's lru_cache does.
func (n *Node) GetDefaultVersion() string { return nodeDefaultVersionOnce() }

var nodeDefaultVersionOnce = sync.OnceValue(func() string {
	return nodeDefaultVersion(runtime.GOOS)
})

// nodeDefaultVersion is "system" when the host's node and npm are real
// installs (exeExists: on PATH, not a version manager's shim, not inside the
// home directory), so nodeenv symlinks them instead of downloading a node.
// Otherwise, and always on Windows where nodeenv cannot do `-n system`, it is
// "default": nodeenv's own prebuilt node. A shimmed node is a trampoline into
// whatever version the manager resolves, and its npm is not one the hook was
// installed against; upstream refuses to reuse it.
func nodeDefaultVersion(goos string) string {
	if goos == "windows" {
		return "default"
	}
	if exeExists("node") && exeExists("npm") {
		return "system"
	}
	return "default"
}

// nodeenvArgs is upstream's nodeenv command line: `default` resolves through
// get_default_version, and only a version other than `default` is passed as
// -n, so a `default` env gets nodeenv's own prebuilt node.
func nodeenvArgs(envDir, version, defaultVersion string) []string {
	if version == "default" {
		version = defaultVersion
	}
	args := []string{"--prebuilt", "--clean-src", envDir}
	if version != "default" {
		args = append(args, "-n", version)
	}
	return args
}

// HealthCheck is upstream node.health_check: `node --version`, run inside
// the env (so the env's node, or the host's for a system env, answers),
// must succeed.
func (n *Node) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, n.EnvironmentDir()+"-"+version)
	if code := envCommandExitCode(nodeEnvVars(envDir), "node", "--version"); code != 0 {
		return fmt.Errorf("`node --version` returned %d", code)
	}
	return nil
}

// nodeEnvVars mirrors Python pre-commit's get_env_patch: npm's prefix is
// pointed at the env so `npm install -g` lands the hook's executables in its
// bin dir, which Run then puts on PATH.
func nodeEnvVars(envDir string) []string {
	return nodeEnvVarsFor(runtime.GOOS, envDir)
}

// On Windows a global npm install puts executables directly in the prefix and
// modules in prefix/node_modules, so upstream points the prefix at Scripts
// (the nodeenv bin dir) and NODE_PATH at Scripts/node_modules.
func nodeEnvVarsFor(goos, envDir string) []string {
	binDir := venvBinDirFor(goos, envDir)
	prefix, libDir := envDir, "lib"
	if goos == "windows" {
		prefix, libDir = binDir, "Scripts"
	}
	return []string{
		"NODE_VIRTUAL_ENV=" + envDir,
		"NPM_CONFIG_PREFIX=" + prefix,
		"npm_config_prefix=" + prefix,
		"NODE_PATH=" + filepath.Join(envDir, libDir, "node_modules"),
		PrependPath(binDir),
	}
}

func (n *Node) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	pkgs, err := nodeInstallPackages(prefix, additionalDeps)
	if err != nil {
		return err
	}
	envDir := filepath.Join(prefix, n.EnvironmentDir()+"-"+version)

	// Create the nodeenv ("system" symlinks the host node into the env).
	cmd := exec.Command("nodeenv", nodeenvArgs(envDir, version, nodeDefaultVersion(runtime.GOOS))...)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nodeenv failed: %s: %w", string(out), err)
	}

	// As upstream 4.7.0: npm runs under the host environment with every
	// GIT_* variable that pins a repository stripped, so a hook installed
	// during `git commit -a` or from inside a submodule does not hand the
	// host's GIT_INDEX_FILE / GIT_DIR to the git clone npm makes of the
	// git+file:// package.
	env := append(git.NoGitEnv(), nodeEnvVars(envDir)...)
	// npm is resolved on the env's PATH, as upstream's in_env does, so the
	// npm nodeenv installed with the env's node is the one that runs; the
	// host's is only the fallback (a `-n system` env has no npm of its own).
	npm, err := FindExecutable("npm", venvBinDir(envDir))
	if err != nil {
		return fmt.Errorf("npm not found: %w", err)
	}
	return npmInstall(npm, prefix, env, pkgs)
}

// nodeInstallPackages mirrors upstream's install_environment (4.6.1+): a hook
// repo with .git and package.json is installed straight from git via a
// git+file:// spec, which is how npm runs its prepare/build scripts the way a
// git dependency would; otherwise (a `repo: local` hook) the
// additional_dependencies are the whole environment.
func nodeInstallPackages(prefix string, additionalDeps []string) ([]string, error) {
	var pkgs []string
	if pathExists(filepath.Join(prefix, ".git")) && pathExists(filepath.Join(prefix, "package.json")) {
		pkgs = append(pkgs, "git+file://"+prefix)
	}
	pkgs = append(pkgs, additionalDeps...)
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("`language: node` must have package.json or additional_dependencies")
	}
	return pkgs, nil
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// nodeInstallArgs is upstream's install command, verbatim.
func nodeInstallArgs(pkgs []string) []string {
	return append([]string{"install", "--allow-git=root", "--install-links", "-g"}, pkgs...)
}

// npmInstall installs pkgs into the hook environment, which npm resolves from
// NPM_CONFIG_PREFIX in env. env is the complete environment for npm.
func npmInstall(npm, prefix string, env, pkgs []string) error {
	cmd := exec.Command(npm, nodeInstallArgs(pkgs)...)
	cmd.Dir = prefix
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm install failed: %s: %w", string(out), err)
	}
	return nil
}

// Run implements Language.
func (n *Node) Run(
	ctx context.Context,
	prefix, workDir, entry string,
	args, fileArgs []string,
	version string,
) (int, []byte, error) {
	envDir := filepath.Join(prefix, n.EnvironmentDir()+"-"+version)
	env := nodeEnvVars(envDir)
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
