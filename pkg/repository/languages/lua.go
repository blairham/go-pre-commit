package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// LuaLanguage handles Lua environment setup
type LuaLanguage struct {
	*language.Base
}

// NewLuaLanguage creates a new Lua language handler
func NewLuaLanguage() *LuaLanguage {
	return &LuaLanguage{
		Base: language.NewBase("lua", "lua", "-v", "https://www.lua.org/"),
	}
}

// GetDefaultVersion returns the default Lua version
// Following Python pre-commit behavior: returns 'system' if Lua is installed, otherwise 'default'
func (l *LuaLanguage) GetDefaultVersion() string {
	// Check if system Lua is available
	if l.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (l *LuaLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return l.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "lua")
}

// SetupEnvironmentWithRepoInfo sets up a Lua environment with repository URL information
func (l *LuaLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return l.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// CheckHealth verifies that Lua is working in the environment
func (l *LuaLanguage) CheckHealth(envPath string) error {
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("lua environment directory does not exist: %s", envPath)
	}

	// For Lua, we use the system runtime, so check if it's available
	if !l.IsRuntimeAvailable() {
		return fmt.Errorf("lua runtime not found in system PATH")
	}

	return nil
}

// InstallDependencies installs Lua rocks using luarocks
//
//nolint:gocognit,gocyclo,cyclop,nestif // Complex dependency installation logic
func (l *LuaLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Check if luarocks is available
	if _, err := exec.LookPath("luarocks"); err != nil {
		fmt.Printf("⚠️  Warning: luarocks not found, cannot install Lua dependencies: %s\n", strings.Join(deps, " "))
		return fmt.Errorf("luarocks not found: %w", err)
	}

	// Create a local rocks tree
	rocksPath := filepath.Join(envPath, "lua_modules")
	if err := os.MkdirAll(rocksPath, 0o750); err != nil {
		return fmt.Errorf("failed to create lua_modules directory: %w", err)
	}

	// Install each dependency
	for _, dep := range deps {
		// Parse dependency specification (name==version or just name)
		parts := strings.Split(dep, "==")
		var rock, version string
		if len(parts) == 2 {
			rock = parts[0]
			version = parts[1]
		} else {
			rock = dep
		}

		var cmd *exec.Cmd
		if version != "" {
			cmd = exec.Command("luarocks", "install", "--tree", rocksPath, rock, version)
		} else {
			cmd = exec.Command("luarocks", "install", "--tree", rocksPath, rock)
		}

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to install Lua rock %s: %w\nOutput: %s", rock, err, output)
		}
	}

	return nil
}

// CheckEnvironmentHealth checks if the Lua environment is healthy
func (l *LuaLanguage) CheckEnvironmentHealth(envPath string) bool {
	// First check if the environment directory exists
	if _, err := os.Stat(envPath); err != nil {
		return false
	}

	// Try the base health check (looks for lua in environment bin directory)
	if err := l.CheckHealth(envPath); err != nil {
		// Environment lua not found, check if system lua is available as fallback
		if !l.IsRuntimeAvailable() {
			return false
		}
		// System lua is available, environment is considered healthy for execution
	}
	// Found lua in environment or system fallback available, continue with full check

	// Check if lua_modules exists (if dependencies were installed)
	rocksPath := filepath.Join(envPath, "lua_modules")
	if _, err := os.Stat(rocksPath); err == nil {
		// lua_modules exists, check if it has proper structure
		libPath := filepath.Join(rocksPath, "lib", "lua")
		if _, err := os.Stat(libPath); err != nil {
			return false
		}
	}

	return true
}

// SetupEnvironmentWithRepo sets up a Lua environment within a repository context
func (l *LuaLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Use repository-aware environment naming following pre-commit conventions
	envDirName := language.GetRepositoryEnvironmentName(l.Name, version)
	if envDirName == "" {
		// For languages that don't need separate environments
		return repoPath, nil
	}

	// Prevent creating environment directory in CWD if repoPath is empty
	var envPath string
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir are empty, cannot create Lua environment")
		}
		// Use cache directory when repoPath is empty
		envPath = filepath.Join(cacheDir, "lua-"+envDirName)
	} else {
		// Create environment in the repository directory (like Python pre-commit)
		envPath = filepath.Join(repoPath, envDirName)
	}

	// Check if environment already exists and is functional
	if l.CheckEnvironmentHealth(envPath) {
		return envPath, nil
	}

	// Environment exists but is broken, remove and recreate
	if _, err := os.Stat(envPath); err == nil {
		if err := os.RemoveAll(envPath); err != nil {
			return "", fmt.Errorf("failed to remove broken environment: %w", err)
		}
	}

	// Create environment directory
	if err := os.MkdirAll(envPath, 0o750); err != nil {
		return "", fmt.Errorf("failed to create Lua environment directory: %w", err)
	}

	// Install additional dependencies if specified
	if len(additionalDeps) > 0 {
		if err := l.InstallDependencies(envPath, additionalDeps); err != nil {
			return "", fmt.Errorf("failed to install Lua dependencies: %w", err)
		}
	}

	return envPath, nil
}
