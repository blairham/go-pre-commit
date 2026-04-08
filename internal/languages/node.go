package languages

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
)

// Node implements the Language interface for Node.js hooks.
type Node struct{}

func (n *Node) Name() string           { return "node" }
func (n *Node) EnvironmentDir() string  { return "node_env" }
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

func (n *Node) InstallEnvironment(prefix, version string, additionalDeps []string) error {
	envDir := filepath.Join(prefix, n.EnvironmentDir()+"-"+version)

	nodeVersion := version
	if nodeVersion == "default" {
		nodeVersion = "system"
	}

	// Create nodeenv.
	cmd := exec.Command("nodeenv", "--prebuilt", "-p", envDir)
	if nodeVersion != "system" {
		cmd = exec.Command("nodeenv", "--prebuilt", "--node="+nodeVersion, "-p", envDir)
	}
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("nodeenv failed: %s: %w", string(out), err)
	}

	// Install the hook package.
	npm := filepath.Join(envDir, "bin", "npm")
	installArgs := []string{"install", "--dev"}
	installArgs = append(installArgs, additionalDeps...)
	cmd = exec.Command(npm, installArgs...)
	cmd.Dir = prefix
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("npm install failed: %s: %w", string(out), err)
	}

	return nil
}

func (n *Node) Run(ctx context.Context, prefix, workDir, entry string, args, fileArgs []string, version string) (int, []byte, error) {
	envDir := filepath.Join(prefix, n.EnvironmentDir()+"-"+version)
	binDir := filepath.Join(envDir, "bin")
	env := []string{PrependPath(binDir)}
	return RunHookCommand(ctx, workDir, entry, args, fileArgs, env)
}
