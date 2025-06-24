// Package environment provides improved environment state management
package environment

import (
	"fmt"
	"sync"
	"time"

	"github.com/blairham/go-pre-commit/pkg/interfaces"
)

// Ensure StateManager implements interfaces.StateManager interface
var _ interfaces.StateManager = (*StateManager)(nil)

// StateManager manages environment initialization state without global variables
type StateManager struct {
	initializedEnvs map[string]bool
	installingEnvs  map[string]bool
	versionCache    map[string]string
	mutex           sync.RWMutex
}

// NewEnvironmentStateManager creates a new environment state manager
func NewEnvironmentStateManager() *StateManager {
	return &StateManager{
		initializedEnvs: make(map[string]bool),
		installingEnvs:  make(map[string]bool),
		versionCache:    make(map[string]string),
	}
}

// IsEnvironmentInitialized checks if an environment is already initialized
func (esm *StateManager) IsEnvironmentInitialized(envKey string) bool {
	esm.mutex.RLock()
	defer esm.mutex.RUnlock()
	return esm.initializedEnvs[envKey]
}

// MarkEnvironmentInitialized marks an environment as initialized
func (esm *StateManager) MarkEnvironmentInitialized(envKey string) {
	esm.mutex.Lock()
	defer esm.mutex.Unlock()
	esm.initializedEnvs[envKey] = true
	delete(esm.installingEnvs, envKey) // Remove from installing state
}

// IsEnvironmentInstalling checks if an environment is currently being installed
func (esm *StateManager) IsEnvironmentInstalling(envKey string) bool {
	esm.mutex.RLock()
	defer esm.mutex.RUnlock()
	return esm.installingEnvs[envKey]
}

// MarkEnvironmentInstalling marks an environment as currently installing
func (esm *StateManager) MarkEnvironmentInstalling(envKey string) error {
	esm.mutex.Lock()
	defer esm.mutex.Unlock()

	if esm.installingEnvs[envKey] {
		return fmt.Errorf("environment %s is already being installed", envKey)
	}

	esm.installingEnvs[envKey] = true
	return nil
}

// ClearEnvironmentInstalling removes the installing state for an environment
func (esm *StateManager) ClearEnvironmentInstalling(envKey string) {
	esm.mutex.Lock()
	defer esm.mutex.Unlock()
	delete(esm.installingEnvs, envKey)
}

// GetCachedVersion retrieves a cached version string
func (esm *StateManager) GetCachedVersion(versionKey string) (string, bool) {
	esm.mutex.RLock()
	defer esm.mutex.RUnlock()
	version, exists := esm.versionCache[versionKey]
	return version, exists
}

// SetCachedVersion stores a version string in cache
func (esm *StateManager) SetCachedVersion(versionKey, version string) {
	esm.mutex.Lock()
	defer esm.mutex.Unlock()
	esm.versionCache[versionKey] = version
}

// GetStatistics returns statistics for the StateManager interface
func (esm *StateManager) GetStatistics() map[string]any {
	return esm.GetEnvironmentStats()
}

// GetEnvironmentStats returns current state statistics
func (esm *StateManager) GetEnvironmentStats() map[string]any {
	esm.mutex.RLock()
	defer esm.mutex.RUnlock()

	return map[string]any{
		"initialized_count": len(esm.initializedEnvs),
		"installing_count":  len(esm.installingEnvs),
		"cached_versions":   len(esm.versionCache),
	}
}

// Reset clears all state (useful for testing)
func (esm *StateManager) Reset() {
	esm.mutex.Lock()
	defer esm.mutex.Unlock()

	esm.initializedEnvs = make(map[string]bool)
	esm.installingEnvs = make(map[string]bool)
	esm.versionCache = make(map[string]string)
}

// Stats provides statistics about environment state
type Stats struct {
	InitializedCount int `json:"initialized_count"`
	InstallingCount  int `json:"installing_count"`
	CachedVersions   int `json:"cached_versions"`
}

// WaitForEnvironment waits for an environment to finish installing
func (esm *StateManager) WaitForEnvironment(envKey string, maxAttempts int) error {
	for attempt := range maxAttempts {
		if !esm.IsEnvironmentInstalling(envKey) {
			if esm.IsEnvironmentInitialized(envKey) {
				return nil // Successfully initialized
			}
			return fmt.Errorf("environment %s failed to initialize", envKey)
		}

		// Sleep briefly and check again
		// In a real implementation, you might use channels or other synchronization
		// For now, this provides the interface for proper coordination
		if attempt < maxAttempts-1 { // Don't sleep on the last attempt
			time.Sleep(10 * time.Millisecond)
		}
	}

	return fmt.Errorf("timeout waiting for environment %s to finish installing", envKey)
}

// Context provides context for environment operations
type Context struct {
	StateManager *StateManager
	CacheDir     string
	WorkingDir   string
}

// NewEnvironmentContext creates a new environment context
func NewEnvironmentContext(cacheDir, workingDir string) *Context {
	return &Context{
		StateManager: NewEnvironmentStateManager(),
		CacheDir:     cacheDir,
		WorkingDir:   workingDir,
	}
}

// CreateEnvironmentKey creates a unique key for environment tracking
func (ec *Context) CreateEnvironmentKey(
	language, version, repoURL string,
) string {
	return fmt.Sprintf("%s:%s:%s", language, version, repoURL)
}

// GlobalEnvironmentManager provides a singleton for backward compatibility
// This should eventually be replaced with dependency injection
var (
	globalEnvironmentManager *StateManager
	globalManagerOnce        sync.Once
)

// GetGlobalEnvironmentManager returns the global environment manager
// Deprecated: Use dependency injection instead
func GetGlobalEnvironmentManager() *StateManager {
	globalManagerOnce.Do(func() {
		globalEnvironmentManager = NewEnvironmentStateManager()
	})
	return globalEnvironmentManager
}
