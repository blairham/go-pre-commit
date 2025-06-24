package languages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/blairham/go-pre-commit/pkg/download/nodeenv"
	"github.com/blairham/go-pre-commit/pkg/git"
	"github.com/blairham/go-pre-commit/pkg/language"
)

const (
	windowsOS = "windows"
)

// NodeLanguage handles Node.js environment setup
type NodeLanguage struct {
	*language.Base
	NodeenvManager       *nodeenv.Manager
	cachedDefaultVersion string
	versionCacheMutex    sync.RWMutex
}

// NewNodeLanguage creates a new Node.js language handler
func NewNodeLanguage() *NodeLanguage {
	return &NodeLanguage{
		Base: language.NewBase(
			"Node",
			"node",
			"--version",
			"https://nodejs.org/",
		),
		NodeenvManager: nil, // Will be initialized with cache directory when needed
	}
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (n *NodeLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return n.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "node")
}

// SetupEnvironmentWithRepoInfo sets up a Node.js environment with repository URL information
func (n *NodeLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return n.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// SetupEnvironmentWithRepo sets up a Node.js environment in the repository directory
func (n *NodeLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Initialize nodeenv manager
	n.initNodeenvManager(cacheDir)

	// Validate inputs and resolve paths
	resolvedRepoPath, err := n.validateAndResolvePaths(repoPath, cacheDir)
	if err != nil {
		return "", err
	}

	// Determine environment version based on system availability
	envVersion, actualVersion := n.determineVersions(version)

	// Create environment path
	envDirName := language.GetRepositoryEnvironmentName("node", envVersion)
	envPath := filepath.Join(resolvedRepoPath, envDirName)

	// Handle Windows long path limitations (following Python pre-commit approach)
	envPath = n.handleWindowsLongPath(envPath)

	// Check if environment exists and is healthy
	if n.isEnvironmentHealthy(envPath) {
		return envPath, nil
	}

	// Setup the environment
	if err := n.setupNewEnvironment(envPath, actualVersion); err != nil {
		return "", err
	}

	// Check if package.json exists - if it does, install dependencies
	packageJSONPath := filepath.Join(resolvedRepoPath, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		// Install dependencies using npm
		if err := n.InstallDependencies(envPath, additionalDeps); err != nil {
			return "", fmt.Errorf("failed to install Node.js dependencies: %w", err)
		}
	} else if len(additionalDeps) > 0 {
		// If no package.json but additional deps specified, warn user
		fmt.Printf("[WARN] Node.js language ignoring additional dependencies "+
			"(no package.json found): %v\n", additionalDeps)
	}

	return envPath, nil
}

// InstallDependencies installs NPM packages and sets up the Node.js environment
func (n *NodeLanguage) InstallDependencies(envPath string, deps []string) error {
	// Check if package.json exists
	packageJSONPath := filepath.Join(filepath.Dir(envPath), "package.json")
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		if len(deps) > 0 {
			fmt.Printf("[WARN] Node.js language ignoring additional dependencies "+
				"(no package.json found): %v\n", deps)
		}
		return nil
	}

	// Set up Node.js environment for package installation
	if err := n.setupNodeEnvironment(envPath); err != nil {
		return fmt.Errorf("failed to setup Node.js environment: %w", err)
	}

	// Run npm install for the package
	if err := n.runNpmInstall(envPath, deps); err != nil {
		return fmt.Errorf("failed to install Node.js dependencies: %w", err)
	}

	return nil
}

// setupNodeEnvironment sets up the Node.js environment variables and directory structure
func (n *NodeLanguage) setupNodeEnvironment(envPath string) error {
	// Create bin directory if it doesn't exist
	binDir := filepath.Join(envPath, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Set up lib/node_modules directory
	libDir := "lib"
	if runtime.GOOS == windowsOS {
		libDir = "Scripts"
	}
	nodeModulesDir := filepath.Join(envPath, libDir, "node_modules")
	if err := os.MkdirAll(nodeModulesDir, 0o750); err != nil {
		return fmt.Errorf("failed to create node_modules directory: %w", err)
	}

	return nil
}

// runNpmInstall performs the npm install sequence following Python pre-commit's approach
func (n *NodeLanguage) runNpmInstall(envPath string, additionalDeps []string) error {
	repoDir := filepath.Dir(envPath)

	// Check if npm is available
	if !n.isNpmAvailable(envPath) {
		return fmt.Errorf("npm not available in environment. Please ensure Node.js and npm are properly installed")
	}

	// Get the npm executable path
	npmPath := n.getNpmPath(envPath)
	if npmPath == "" || !n.fileExists(npmPath) {
		// Fallback to system npm
		npmPath = "npm"
	}

	// Set up environment variables for npm
	env := n.getNodeEnvVars(envPath)

	// Step 1: Install development and production dependencies locally
	// This follows Python pre-commit's approach: npm install --include=dev --include=prod
	localInstallCmd := []string{
		npmPath, "install",
		"--include=dev", "--include=prod",
		"--ignore-prepublish", "--no-progress", "--no-save",
	}

	if err := n.runCommandInEnv(repoDir, localInstallCmd, env); err != nil {
		return fmt.Errorf("failed to run local npm install: %w", err)
	}

	// Step 2: Create a package tarball
	packCmd := []string{npmPath, "pack"}
	packOutput, err := n.runCommandInEnvWithOutput(repoDir, packCmd, env)
	if err != nil {
		return fmt.Errorf("failed to create package tarball: %w", err)
	}

	packageFile := strings.TrimSpace(packOutput)
	packagePath := filepath.Join(repoDir, packageFile)
	defer func() {
		if err := os.Remove(packagePath); err != nil {
			// Log the error but don't fail the operation
			fmt.Printf("Warning: failed to clean up package file %s: %v\n", packagePath, err)
		}
	}() // Clean up package file after installation

	// Step 3: Install the package globally along with additional dependencies
	installArgs := []string{npmPath, "install", "-g", packagePath}
	installArgs = append(installArgs, additionalDeps...)

	if err := n.runCommandInEnv(repoDir, installArgs, env); err != nil {
		return fmt.Errorf("failed to run global npm install: %w", err)
	}

	// Step 4: Clean up local node_modules (following Python pre-commit approach)
	nodeModulesPath := filepath.Join(repoDir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); err == nil {
		if err := os.RemoveAll(nodeModulesPath); err != nil {
			fmt.Printf("[WARN] Failed to clean up local node_modules: %v\n", err)
		}
	}

	return nil
}

// isNpmAvailable checks if npm is available either in the environment or system
func (n *NodeLanguage) isNpmAvailable(envPath string) bool {
	// First, try to find npm in the Node.js environment (installed via nodeenv)
	npmPath := n.getNpmPath(envPath)
	if npmPath != "" && n.fileExists(npmPath) {
		return true
	}

	// Check if npm is available in the environment with environment variables
	env := n.getNodeEnvVars(envPath)
	cmd := exec.Command("npm", "--version")
	cmd.Env = env
	if err := cmd.Run(); err == nil {
		return true
	}

	// Fallback to system npm
	_, err := exec.LookPath("npm")
	return err == nil
}

// getNpmPath returns the expected path to npm in the Node.js environment
func (n *NodeLanguage) getNpmPath(envPath string) string {
	if runtime.GOOS == windowsOS {
		// On Windows, npm is installed as npm.cmd
		return filepath.Join(envPath, "npm.cmd")
	}
	// On Unix-like systems, npm is in bin/
	return filepath.Join(envPath, "bin", "npm")
}

// getNodeEnvVars returns environment variables needed for Node.js/npm operations
// Implements the same logic as Python pre-commit's get_env_patch function
func (n *NodeLanguage) getNodeEnvVars(envPath string) []string {
	var installPrefix, libDir string

	// Platform-specific environment setup (matching Python pre-commit logic)
	switch runtime.GOOS {
	case windowsOS:
		// On Windows, npm uses Scripts directory
		installPrefix = filepath.Join(envPath, "Scripts")
		libDir = "Scripts"
	default:
		// Unix-like systems (Linux, macOS, etc.)
		installPrefix = envPath
		libDir = "lib"
	}

	// Start with clean environment (filter out problematic git variables)
	env := git.GetCleanEnvironment()

	// Build PATH that includes both the environment bin and existing PATH
	binDir := filepath.Join(envPath, "bin")
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s%c%s", binDir, os.PathListSeparator, currentPath)

	// Add Node.js specific environment variables
	env = n.setEnvVar(env, "NODE_VIRTUAL_ENV", envPath)
	env = n.setEnvVar(env, "NPM_CONFIG_PREFIX", installPrefix)
	env = n.setEnvVar(env, "npm_config_prefix", installPrefix) // Both upper and lowercase needed
	env = n.setEnvVar(env, "NODE_PATH", filepath.Join(envPath, libDir, "node_modules"))
	env = n.setEnvVar(env, "PATH", newPath)

	// Unset user config to avoid conflicts (following Python implementation)
	env = n.unsetEnvVar(env, "NPM_CONFIG_USERCONFIG")
	env = n.unsetEnvVar(env, "npm_config_userconfig")

	return env
}

// setEnvVar sets or updates an environment variable in the environment slice
func (n *NodeLanguage) setEnvVar(env []string, key, value string) []string {
	prefix := key + "="
	newVar := prefix + value

	// Remove existing variable if present
	var result []string
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			if !found {
				result = append(result, newVar)
				found = true
			}
			// Skip the old value
		} else {
			result = append(result, e)
		}
	}

	// Add the variable if it wasn't found
	if !found {
		result = append(result, newVar)
	}

	return result
}

// unsetEnvVar removes an environment variable from the environment slice
func (n *NodeLanguage) unsetEnvVar(env []string, key string) []string {
	var result []string
	prefix := key + "="
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// runCommandInEnv runs a command with the specified environment variables
func (n *NodeLanguage) runCommandInEnv(dir string, cmdArgs, env []string) error {
	_, err := n.runCommandInEnvWithOutput(dir, cmdArgs, env)
	return err
}

// runCommandInEnvWithOutput runs a command with environment variables and returns output
func (n *NodeLanguage) runCommandInEnvWithOutput(dir string, cmdArgs, env []string) (string, error) {
	if len(cmdArgs) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	cmd.Dir = dir
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command '%s' failed: %w", strings.Join(cmdArgs, " "), err)
	}

	return string(output), nil
}

// SetupEnvironmentWithRepositoryInit handles Node.js environment setup assuming repository is already initialized
//
//nolint:golint,revive // function name is part of interface contract
func (n *NodeLanguage) SetupEnvironmentWithRepositoryInit(
	cacheDir, version, repoPath string,
	additionalDeps []string,
	_ any,
) (string, error) {
	// Repository should already be cloned by PreInitializeHookEnvironments
	// Just set up the Node.js-specific environment
	return n.SetupEnvironmentWithRepo(cacheDir, version, repoPath, "", additionalDeps)
}

// initNodeenvManager initializes the NodeenvManager if it's not already set
func (n *NodeLanguage) initNodeenvManager(cacheDir string) {
	if n.NodeenvManager == nil && cacheDir != "" {
		nodeCacheDir := filepath.Join(cacheDir, "node")
		n.NodeenvManager = nodeenv.NewManager(nodeCacheDir)
	}
}

// installNodeVersionIfNeeded installs a Node.js version if it's not already installed
func (n *NodeLanguage) installNodeVersionIfNeeded(version string) error {
	if n.NodeenvManager == nil {
		return fmt.Errorf("nodeenv manager not initialized")
	}

	if !n.NodeenvManager.IsVersionInstalled(version) {
		if err := n.NodeenvManager.InstallVersion(context.TODO(), version); err != nil {
			return fmt.Errorf("failed to install Node.js %s: %w", version, err)
		}
	}

	return nil
}

// IsRuntimeAvailable checks if Node.js is available in the system
//
//nolint:golint,revive // function name is part of interface contract
func (n *NodeLanguage) IsRuntimeAvailable() bool {
	// Check system Node.js first
	if n.isSystemNodeAvailable() {
		return true
	}

	// If nodeenv manager has any installed versions, consider Node.js available
	if n.NodeenvManager != nil {
		versions, err := n.NodeenvManager.GetInstalledVersions()
		if err == nil && len(versions) > 0 {
			return true
		}
	}

	return false
}

// validateAndResolvePaths validates and resolves the repository path
func (n *NodeLanguage) validateAndResolvePaths(repoPath, cacheDir string) (string, error) {
	// Handle empty repoPath by using cacheDir instead to avoid creating directories in CWD
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir cannot be empty")
		}
		repoPath = cacheDir
	}

	return repoPath, nil
}

// determineVersions determines the environment version name and actual version to use
func (n *NodeLanguage) determineVersions(version string) (envVersion, actualVersion string) {
	// Normalize input version using cached default version logic
	if version == "" {
		version = n.GetDefaultVersion()
	}

	// If a specific version is requested that we don't support, fall back to default
	if version != language.VersionDefault && version != language.VersionSystem {
		// Validate that it's a reasonable version format
		// For now, only support system/default versions, normalize others to default
		_ = !strings.HasPrefix(version, "v") && version != "latest" // Non-standard version format detected
		version = n.GetDefaultVersion()
	}

	// Check if Node.js is available on the system
	systemAvailable := n.isSystemNodeAvailable()

	if systemAvailable && version == language.VersionSystem {
		// Use system Node.js
		return language.VersionSystem, language.VersionSystem
	}

	if systemAvailable && version == language.VersionDefault {
		// System Node.js is available, use default for environment naming
		return language.VersionDefault, language.VersionDefault
	}

	// Node.js not available on system or specific version requested
	// Need to install via nodeenv if available
	if n.NodeenvManager != nil {
		// Get the actual version we'll install
		actualVersion = version
		if version == language.VersionDefault {
			// Try to get latest LTS version
			if latestLTS, err := n.NodeenvManager.GetLatestLTSVersion(); err == nil {
				actualVersion = latestLTS.Version
			} else {
				// Fall back to known LTS version
				actualVersion = "20.11.0"
			}
		}
		// Use actual version for environment naming since system Node.js not available
		return actualVersion, actualVersion
	}

	// No nodeenv manager and no system Node.js - this will fail
	return language.VersionDefault, language.VersionDefault
}

// isSystemNodeAvailable checks if Node.js is available on the system (not via nodeenv)
func (n *NodeLanguage) isSystemNodeAvailable() bool {
	_, err := exec.LookPath("node")
	return err == nil
}

// isEnvironmentHealthy checks if an environment exists and is functional
func (n *NodeLanguage) isEnvironmentHealthy(envPath string) bool {
	// Check if environment already exists
	if _, err := os.Stat(envPath); err == nil {
		// Environment exists, verify it's functional
		if err := n.CheckHealth(envPath, ""); err == nil {
			return true
		}
		// Environment exists but is broken, remove and recreate
		if err := os.RemoveAll(envPath); err != nil {
			fmt.Printf("Warning: failed to remove broken environment: %v\n", err)
		}
	}
	return false
}

// setupNewEnvironment creates a new Node.js environment
func (n *NodeLanguage) setupNewEnvironment(envPath, version string) error {
	// For nodeenv-based installations, try to install the specific version
	// But only if we have a cache directory (nodeenv manager is initialized)
	if n.NodeenvManager != nil && version != language.VersionSystem {
		return n.setupNodeenvEnvironment(envPath, version)
	}

	// Fall back to simple environment setup (just check system Node.js)
	return n.setupSystemEnvironment(envPath)
}

// setupNodeenvEnvironment sets up an environment using nodeenv manager
func (n *NodeLanguage) setupNodeenvEnvironment(envPath, version string) error {
	// Version should already be resolved to actual version
	resolvedVersion := version
	if version == language.VersionDefault {
		// This shouldn't happen with new logic, but handle gracefully
		if latestLTS, err := n.NodeenvManager.GetLatestLTSVersion(); err == nil {
			resolvedVersion = latestLTS.Version
		} else {
			resolvedVersion = "20.11.0"
		}
	}

	// Install the Node.js version if needed
	if err := n.installNodeVersionIfNeeded(resolvedVersion); err != nil {
		// If nodeenv installation fails, fall back to system environment
		if strings.Contains(err.Error(), "failed to create cache directory") ||
			strings.Contains(err.Error(), "failed to create versions directory") {
			return n.setupSystemEnvironment(envPath)
		}
		return fmt.Errorf("failed to install Node.js version: %w", err)
	}

	// Create the environment using nodeenv
	if err := n.NodeenvManager.CreateEnvironment(envPath, resolvedVersion); err != nil {
		return fmt.Errorf("failed to create Node.js environment: %w", err)
	}

	return nil
}

// CheckHealth verifies that the Node.js environment is working correctly
func (n *NodeLanguage) CheckHealth(envPath, _ string) error {
	// Set up environment variables
	env := n.getNodeEnvVars(envPath)

	// Run node --version to verify Node.js is working
	cmd := exec.Command("node", "--version")
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() != 0 {
			return fmt.Errorf("`node --version` returned %d", cmd.ProcessState.ExitCode())
		}
		return fmt.Errorf("node runtime not available: %w", err)
	}

	return nil
}

// setupSystemEnvironment sets up an environment using system Node.js
func (n *NodeLanguage) setupSystemEnvironment(envPath string) error {
	if !n.IsRuntimeAvailable() {
		return fmt.Errorf("node.js runtime not found. Please install Node.js to use Node.js hooks.\n"+
			"Installation instructions: %s", n.InstallURL)
	}

	// Create environment directory
	if err := n.CreateEnvironmentDirectory(envPath); err != nil {
		return fmt.Errorf("failed to create Node.js environment directory: %w", err)
	}

	// Set up the Node.js environment structure
	if err := n.setupNodeEnvironment(envPath); err != nil {
		return fmt.Errorf("failed to setup Node.js environment: %w", err)
	}

	return nil
}

// GetExecutablePath returns the path to a Node.js executable in the environment
func (n *NodeLanguage) GetExecutablePath(envPath, executableName string) string {
	// Check if it's a global npm package (installed via npm install -g)
	binDir := filepath.Join(envPath, "bin")
	execPath := filepath.Join(binDir, executableName)

	// On Windows, npm creates .cmd wrapper files
	if runtime.GOOS == windowsOS {
		if cmdPath := execPath + ".cmd"; n.fileExists(cmdPath) {
			return cmdPath
		}
		if exePath := execPath + ".exe"; n.fileExists(exePath) {
			return exePath
		}
	}

	// Check for the executable as-is
	if n.fileExists(execPath) {
		return execPath
	}

	// Fallback to system PATH (for system Node.js installations)
	return executableName
}

// fileExists checks if a file exists
func (n *NodeLanguage) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetDefaultVersion returns the default Node.js version with caching
// Implements the same logic as Python pre-commit with platform-specific handling
func (n *NodeLanguage) GetDefaultVersion() string {
	n.versionCacheMutex.RLock()
	if n.cachedDefaultVersion != "" {
		defer n.versionCacheMutex.RUnlock()
		return n.cachedDefaultVersion
	}
	n.versionCacheMutex.RUnlock()

	n.versionCacheMutex.Lock()
	defer n.versionCacheMutex.Unlock()

	// Double-check after acquiring write lock
	if n.cachedDefaultVersion != "" {
		return n.cachedDefaultVersion
	}

	// nodeenv does not yet support `-n system` on windows
	if runtime.GOOS == windowsOS {
		n.cachedDefaultVersion = language.VersionDefault
		return n.cachedDefaultVersion
	}

	// if node is already installed, we can save a bunch of setup time by
	// using the installed version
	if n.isNodeAndNpmAvailable() {
		n.cachedDefaultVersion = language.VersionSystem
		return n.cachedDefaultVersion
	}

	n.cachedDefaultVersion = language.VersionDefault
	return n.cachedDefaultVersion
}

// isNodeAndNpmAvailable checks if both node and npm are available on the system
func (n *NodeLanguage) isNodeAndNpmAvailable() bool {
	// Check for node
	if _, err := exec.LookPath("node"); err != nil {
		return false
	}

	// Check for npm
	if _, err := exec.LookPath("npm"); err != nil {
		return false
	}

	return true
}

// handleWindowsLongPath adds Windows long path prefix if needed
// This handles paths longer than 260 characters on Windows
// Matches the Python pre-commit implementation
func (n *NodeLanguage) handleWindowsLongPath(path string) string {
	if runtime.GOOS == windowsOS {
		// Add long path prefix for Windows to handle paths > 260 characters
		// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx
		return `\\?\` + filepath.ToSlash(filepath.Clean(path))
	}
	return path
}
