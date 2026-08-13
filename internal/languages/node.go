package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Node implements the Language interface for Node.js hooks.
type Node struct{}

func (n *Node) Name() string              { return "node" }
func (n *Node) EnvironmentDir() string    { return "node_env" }
func (n *Node) GetDefaultVersion() string { return "default" }

func (n *Node) HealthCheck(prefix, version string) error {
	envDir := filepath.Join(prefix, n.EnvironmentDir()+"-"+version)
	nodePath := filepath.Join(envDir, "bin", "node")
	cmd := exec.Command(nodePath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("node environment unhealthy: %w", err)
	}
	return nil
}

// nodeEnvVars mirrors Python pre-commit's get_env_patch: npm's prefix is
// pointed at the env so `npm install -g` lands the hook's executables in
// envDir/bin, which Run then puts on PATH.
func nodeEnvVars(envDir string) []string {
	return []string{
		"NODE_VIRTUAL_ENV=" + envDir,
		"NPM_CONFIG_PREFIX=" + envDir,
		"npm_config_prefix=" + envDir,
		"NODE_PATH=" + filepath.Join(envDir, "lib", "node_modules"),
		PrependPath(filepath.Join(envDir, "bin")),
	}
}

func (n *Node) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, n.EnvironmentDir()+"-"+version)

	nodeVersion := version
	if nodeVersion == "default" {
		nodeVersion = "system"
	}

	// Create the nodeenv ("system" symlinks the host node into the env).
	cmd := exec.Command("nodeenv", "--prebuilt", "--clean-src", envDir, "-n", nodeVersion)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nodeenv failed: %s: %w", string(out), err)
	}

	env := nodeEnvVars(envDir)

	// A `repo: local` hook has no package to install — its prefix is a
	// placeholder directory with no package.json — so there is nothing to pack
	// and the additional_dependencies are the whole environment.
	if _, err := os.Stat(filepath.Join(prefix, "package.json")); err != nil {
		if len(additionalDeps) == 0 {
			return fmt.Errorf("`language: node` must have package.json or additional_dependencies")
		}
		return npmInstallGlobal(prefix, env, additionalDeps)
	}

	// Install the hook repo's own dependencies locally, then pack it and
	// install the package globally into the env alongside additional deps —
	// the same local-install → pack → global-install dance as Python
	// pre-commit, which is what creates the bin entry points in envDir/bin.
	cmd = exec.Command("npm", "install")
	cmd.Dir = prefix
	cmd.Env = append(cmd.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm install failed: %s: %w", string(out), err)
	}

	cmd = exec.Command("npm", "pack")
	cmd.Dir = prefix
	cmd.Env = append(cmd.Environ(), env...)
	packOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("npm pack failed: %s: %w", string(packOut), err)
	}
	lines := strings.Split(strings.TrimSpace(string(packOut)), "\n")
	pkg := filepath.Join(prefix, strings.TrimSpace(lines[len(lines)-1]))
	defer os.Remove(pkg)

	return npmInstallGlobal(prefix, env, append([]string{pkg}, additionalDeps...))
}

// npmInstallGlobal installs packages into the hook environment, which npm
// resolves from NPM_CONFIG_PREFIX in env.
func npmInstallGlobal(prefix string, env, pkgs []string) error {
	cmd := exec.Command("npm", append([]string{"install", "-g"}, pkgs...)...)
	cmd.Dir = prefix
	cmd.Env = append(cmd.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm install -g failed: %s: %w", string(out), err)
	}
	return nil
}

func (n *Node) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, n.EnvironmentDir()+"-"+version)
	env := nodeEnvVars(envDir)
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
