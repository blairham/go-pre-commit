// Package environment provides improved environment management functionality
package environment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/interfaces"
	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// languageManagerAdapter adapts an interfaces.LanguageManager to language.Manager
type languageManagerAdapter struct {
	interfaces.LanguageManager
}

// Ensure the adapter implements the Manager interface
var _ language.Manager = (*languageManagerAdapter)(nil)

// SetupEnvironment adapts the old interface to the new one
func (a *languageManagerAdapter) SetupEnvironment(cacheDir, version string, additionalDeps []string) (string, error) {
	return a.SetupEnvironmentWithRepoInfo(cacheDir, version, "", "", additionalDeps)
}

// Manager provides centralized environment management
type Manager struct {
	stateManager interfaces.StateManager
	languageMap  map[string]language.Manager
	cacheDir     string
	mutex        sync.RWMutex
}

// NewManager creates a new environment manager
func NewManager(cacheDir string) *Manager {
	return &Manager{
		cacheDir:     cacheDir,
		stateManager: NewEnvironmentStateManager(),
		languageMap:  make(map[string]language.Manager),
	}
}

// SetupEnvironment sets up a language environment
func (m *Manager) SetupEnvironment(
	lang, version string,
	additionalDeps []string,
	repoPath string,
) (string, error) {
	// Get or create language manager
	langMgr, err := m.getOrCreateLanguageManager(lang)
	if err != nil {
		return "", fmt.Errorf("failed to get language manager for %s: %w", lang, err)
	}

	// Check if runtime is available before attempting setup
	if !langMgr.IsRuntimeAvailable() {
		return "", fmt.Errorf("runtime not available for language %s", lang)
	}

	// Setup environment (languages can handle runtime installation during setup)
	var envPath string
	if repoPath != "" {
		// For repository-specific setups, use the repository path as the cache directory
		// so environments are created within the repository structure
		envPath, err = langMgr.SetupEnvironmentWithRepo(repoPath, version, repoPath, "", additionalDeps)
	} else {
		envPath, err = langMgr.SetupEnvironment(m.cacheDir, version, additionalDeps)
	}

	if err != nil {
		return "", fmt.Errorf("failed to setup %s environment: %w", lang, err)
	}

	return envPath, nil
}

// GetEnvironmentBinPath returns the bin path for a language environment
func (m *Manager) GetEnvironmentBinPath(lang, envPath string) (string, error) {
	langMgr, err := m.getOrCreateLanguageManager(lang)
	if err != nil {
		return "", fmt.Errorf("failed to get language manager for %s: %w", lang, err)
	}

	return langMgr.GetEnvironmentBinPath(envPath), nil
}

// IsRuntimeAvailable checks if a language runtime is available
func (m *Manager) IsRuntimeAvailable(lang string) bool {
	langMgr, err := m.getOrCreateLanguageManager(lang)
	if err != nil {
		return false
	}

	return langMgr.IsRuntimeAvailable()
}

// getOrCreateLanguageManager gets or creates a language manager for the given language
func (m *Manager) getOrCreateLanguageManager(lang string) (language.Manager, error) {
	m.mutex.RLock()
	if langMgr, exists := m.languageMap[lang]; exists {
		m.mutex.RUnlock()
		return langMgr, nil
	}
	m.mutex.RUnlock()

	// Create new language manager
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Double-check after acquiring write lock
	if langMgr, exists := m.languageMap[lang]; exists {
		return langMgr, nil
	}

	// Get language registry
	registry := languages.NewLanguageRegistry()
	interfaceLangMgr, exists := registry.GetLanguage(lang)
	if !exists {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}

	// Type assert to interfaces.LanguageManager
	interfacesMgr, ok := interfaceLangMgr.(interfaces.LanguageManager)
	if !ok {
		return nil, fmt.Errorf("language %s does not implement LanguageManager interface", lang)
	}

	// Create adapter to convert from interfaces.LanguageManager to language.LanguageManager
	langMgr := &languageManagerAdapter{interfacesMgr}
	m.languageMap[lang] = langMgr
	return langMgr, nil
}

// GetCacheDir returns the cache directory
func (m *Manager) GetCacheDir() string {
	return m.cacheDir
}

// Close closes the environment manager and cleans up resources
func (m *Manager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Clear the language map
	m.languageMap = make(map[string]language.Manager)

	return nil
}

// SetupEnvironmentWithRepo sets up environment with repository context (legacy compatibility)
func (m *Manager) SetupEnvironmentWithRepo(
	lang, version, repoPath string,
	additionalDeps []string,
) (string, error) {
	return m.SetupEnvironment(lang, version, additionalDeps, repoPath)
}

// PreInitializeEnvironment performs pre-initialization for environment setup
func (m *Manager) PreInitializeEnvironment(
	lang, version, repoPath string,
	additionalDeps []string,
) error {
	langMgr, err := m.getOrCreateLanguageManager(lang)
	if err != nil {
		return fmt.Errorf("failed to get language manager for %s: %w", lang, err)
	}

	return langMgr.PreInitializeEnvironmentWithRepoInfo(m.cacheDir, version, repoPath, "", additionalDeps)
}

// InstallDependencies installs dependencies in an environment
func (m *Manager) InstallDependencies(
	lang, envPath string,
	deps []string,
) error {
	langMgr, err := m.getOrCreateLanguageManager(lang)
	if err != nil {
		return fmt.Errorf("failed to get language manager for %s: %w", lang, err)
	}

	return langMgr.InstallDependencies(envPath, deps)
}

// CheckEnvironmentHealth checks if an environment is functional
func (m *Manager) CheckEnvironmentHealth(lang, envPath string) bool {
	langMgr, err := m.getOrCreateLanguageManager(lang)
	if err != nil {
		return false
	}

	return langMgr.CheckEnvironmentHealth(envPath)
}

// SetupHookEnvironment sets up the environment for a hook
func (m *Manager) SetupHookEnvironment(hook config.Hook, _ config.Repo, repoPath string) (map[string]string, error) {
	// Extract language info
	hookLanguage := hook.Language
	version := hook.LanguageVersion
	if version == "" {
		version = "default"
	}

	// Set up the environment and get the environment path
	envPath, err := m.SetupEnvironment(hookLanguage, version, hook.AdditionalDeps, repoPath)
	if err != nil {
		return nil, err
	}

	// Get the language manager to set up language-specific environment variables
	langMgr, err := m.getOrCreateLanguageManager(hookLanguage)
	if err != nil {
		return nil, fmt.Errorf("failed to get language manager for %s: %w", hookLanguage, err)
	}

	// Start with basic environment variables
	env := make(map[string]string)

	// Add environment path information
	if envPath != "" {
		env["PRE_COMMIT_ENV_PATH"] = envPath
		m.addLanguageSpecificEnvVars(env, hookLanguage, envPath, langMgr)
	}

	return env, nil
}

// addLanguageSpecificEnvVars adds language-specific environment variables
func (m *Manager) addLanguageSpecificEnvVars(
	env map[string]string,
	hookLanguage, envPath string,
	langMgr language.Manager,
) {
	switch hookLanguage {
	case "python", "python3":
		m.addPythonEnvVars(env, envPath, langMgr)
	case "node":
		m.addNodeEnvVars(env, envPath, langMgr)
	case "ruby":
		m.addRubyEnvVars(env, envPath)
	case "golang":
		m.addGoEnvVars(env, envPath)
	case "rust":
		m.addRustEnvVars(env, envPath)
	case "coursier":
		m.addCoursierEnvVars(env, envPath)
	}
}

// addPythonEnvVars adds Python-specific environment variables
func (m *Manager) addPythonEnvVars(env map[string]string, envPath string, langMgr language.Manager) {
	env["VIRTUAL_ENV"] = envPath
	m.updatePathEnv(env, langMgr.GetEnvironmentBinPath(envPath))
}

// addNodeEnvVars adds Node.js-specific environment variables
func (m *Manager) addNodeEnvVars(env map[string]string, envPath string, langMgr language.Manager) {
	env["NODE_VIRTUAL_ENV"] = envPath
	env["NPM_CONFIG_PREFIX"] = envPath
	env["npm_config_prefix"] = envPath
	m.updatePathEnv(env, langMgr.GetEnvironmentBinPath(envPath))
}

// addRubyEnvVars adds Ruby-specific environment variables
func (m *Manager) addRubyEnvVars(env map[string]string, envPath string) {
	gemsDir := filepath.Join(envPath, "gems")
	env["GEM_HOME"] = gemsDir
	env["GEM_PATH"] = "" // Clear for isolation
	env["BUNDLE_IGNORE_CONFIG"] = "1"
	gemsBinDir := filepath.Join(gemsDir, "bin")
	m.updatePathEnv(env, gemsBinDir)
}

// addGoEnvVars adds Go-specific environment variables
func (m *Manager) addGoEnvVars(env map[string]string, envPath string) {
	env["GOCACHE"] = filepath.Join(envPath, "gocache")
	env["GOPATH"] = filepath.Join(envPath, "gopath")
}

// addRustEnvVars adds Rust-specific environment variables
func (m *Manager) addRustEnvVars(env map[string]string, envPath string) {
	env["CARGO_HOME"] = filepath.Join(envPath, "cargo")
}

// addCoursierEnvVars adds Coursier-specific environment variables
func (m *Manager) addCoursierEnvVars(env map[string]string, envPath string) {
	env["COURSIER_CACHE"] = filepath.Join(envPath, ".cs-cache")
	m.updatePathEnv(env, envPath)
}

// updatePathEnv updates the PATH environment variable
func (m *Manager) updatePathEnv(env map[string]string, binPath string) {
	if binPath != "" {
		if currentPath, exists := env["PATH"]; exists {
			env["PATH"] = binPath + string(os.PathListSeparator) + currentPath
		} else {
			env["PATH"] = binPath + string(os.PathListSeparator) + os.Getenv("PATH")
		}
	}
}

// CheckEnvironmentHealthWithRepo checks if a language environment is healthy within a repository context
func (m *Manager) CheckEnvironmentHealthWithRepo(lang, version, repoPath string) error {
	envPath, err := m.SetupEnvironment(lang, version, nil, repoPath)
	if err != nil {
		return err
	}

	if !m.CheckEnvironmentHealth(lang, envPath) {
		return fmt.Errorf("environment health check failed for %s", lang)
	}

	return nil
}

// RebuildEnvironmentWithRepo rebuilds a language environment within a repository context
func (m *Manager) RebuildEnvironmentWithRepo(lang, version, repoPath string) error {
	// For the modular manager, rebuilding is just setting up again
	_, err := m.SetupEnvironment(lang, version, nil, repoPath)
	return err
}

// RebuildEnvironmentWithRepoInfo rebuilds a language environment within a repository context with repo URL
func (m *Manager) RebuildEnvironmentWithRepoInfo(
	lang, version, repoPath, _ string, // repoURL is unused
) error {
	// For the modular manager, rebuilding is just setting up again
	_, err := m.SetupEnvironment(lang, version, nil, repoPath)
	return err
}

// PreInitializeHookEnvironments performs the pre-initialization phase for all hook environments
func (m *Manager) PreInitializeHookEnvironments(
	_ context.Context, // ctx is unused
	hooks []config.HookEnvItem,
	_ any, // repositoryOps is unused
) error {
	for _, hook := range hooks {
		// Extract fields from HookEnvItem
		lang := hook.Hook.Language
		version := hook.Hook.LanguageVersion
		repoPath := hook.RepoPath
		additionalDeps := hook.Hook.AdditionalDeps

		err := m.PreInitializeEnvironment(lang, version, repoPath, additionalDeps)
		if err != nil {
			return fmt.Errorf("failed to pre-initialize %s environment: %w", lang, err)
		}
	}
	return nil
}

// SetupEnvironmentWithRepositoryInit sets up an environment assuming the repository is already initialized
func (m *Manager) SetupEnvironmentWithRepositoryInit(
	_ config.Repo, lang, version string, additionalDeps []string, // repo is unused
) (string, error) {
	// For repository-based setup, we need to use a repo path
	// This would typically come from a repository manager
	return m.SetupEnvironment(lang, version, additionalDeps, "")
}

// GetCommonRepositoryManager returns a repository manager interface that languages can use
// for repository initialization. This is a compatibility method.
func (m *Manager) GetCommonRepositoryManager(
	_ context.Context, // ctx is unused
	repositoryOps any, // Use interface{} to avoid import cycles
) any {
	// This is a compatibility method that provides repository management functionality
	// In a full implementation, this would return a proper repository manager
	return repositoryOps
}
