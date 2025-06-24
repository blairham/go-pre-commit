package environment

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnvironmentStateManager(t *testing.T) {
	sm := NewEnvironmentStateManager()
	assert.NotNil(t, sm)
	assert.NotNil(t, sm.initializedEnvs)
	assert.NotNil(t, sm.installingEnvs)
	assert.NotNil(t, sm.versionCache)
	assert.Equal(t, 0, len(sm.initializedEnvs))
	assert.Equal(t, 0, len(sm.installingEnvs))
	assert.Equal(t, 0, len(sm.versionCache))
}

func TestStateManager_EnvironmentInitialization(t *testing.T) {
	sm := NewEnvironmentStateManager()
	envKey := "python:3.9:/path/to/repo"

	// Initially not initialized
	assert.False(t, sm.IsEnvironmentInitialized(envKey))

	// Mark as initialized
	sm.MarkEnvironmentInitialized(envKey)
	assert.True(t, sm.IsEnvironmentInitialized(envKey))

	// Check stats
	stats := sm.GetEnvironmentStats()
	assert.Equal(t, 1, stats["initialized_count"])
	assert.Equal(t, 0, stats["installing_count"])
}

func TestStateManager_EnvironmentInstalling(t *testing.T) {
	sm := NewEnvironmentStateManager()
	envKey := "node:16:/path/to/repo"

	// Initially not installing
	assert.False(t, sm.IsEnvironmentInstalling(envKey))

	// Mark as installing
	err := sm.MarkEnvironmentInstalling(envKey)
	assert.NoError(t, err)
	assert.True(t, sm.IsEnvironmentInstalling(envKey))

	// Try to mark as installing again (should error)
	err = sm.MarkEnvironmentInstalling(envKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is already being installed")

	// Clear installing state
	sm.ClearEnvironmentInstalling(envKey)
	assert.False(t, sm.IsEnvironmentInstalling(envKey))

	// Should be able to mark as installing again
	err = sm.MarkEnvironmentInstalling(envKey)
	assert.NoError(t, err)
	assert.True(t, sm.IsEnvironmentInstalling(envKey))
}

func TestStateManager_MarkInitializedClearsInstalling(t *testing.T) {
	sm := NewEnvironmentStateManager()
	envKey := "ruby:3.0:/path/to/repo"

	// Mark as installing
	err := sm.MarkEnvironmentInstalling(envKey)
	assert.NoError(t, err)
	assert.True(t, sm.IsEnvironmentInstalling(envKey))

	// Mark as initialized should clear installing state
	sm.MarkEnvironmentInitialized(envKey)
	assert.True(t, sm.IsEnvironmentInitialized(envKey))
	assert.False(t, sm.IsEnvironmentInstalling(envKey))
}

func TestStateManager_VersionCache(t *testing.T) {
	sm := NewEnvironmentStateManager()
	versionKey := "python:latest"
	version := "3.9.7"

	// Initially not cached
	cachedVersion, exists := sm.GetCachedVersion(versionKey)
	assert.False(t, exists)
	assert.Empty(t, cachedVersion)

	// Set cached version
	sm.SetCachedVersion(versionKey, version)
	cachedVersion, exists = sm.GetCachedVersion(versionKey)
	assert.True(t, exists)
	assert.Equal(t, version, cachedVersion)

	// Update cached version
	newVersion := "3.10.1"
	sm.SetCachedVersion(versionKey, newVersion)
	cachedVersion, exists = sm.GetCachedVersion(versionKey)
	assert.True(t, exists)
	assert.Equal(t, newVersion, cachedVersion)
}

func TestStateManager_GetStatistics(t *testing.T) {
	sm := NewEnvironmentStateManager()

	// Initial stats
	stats := sm.GetStatistics()
	assert.Equal(t, 0, stats["initialized_count"])
	assert.Equal(t, 0, stats["installing_count"])
	assert.Equal(t, 0, stats["cached_versions"])

	// Add some state
	sm.MarkEnvironmentInitialized("env1")
	sm.MarkEnvironmentInitialized("env2")
	_ = sm.MarkEnvironmentInstalling("env3")
	sm.SetCachedVersion("ver1", "1.0.0")
	sm.SetCachedVersion("ver2", "2.0.0")
	sm.SetCachedVersion("ver3", "3.0.0")

	// Check updated stats
	stats = sm.GetStatistics()
	assert.Equal(t, 2, stats["initialized_count"])
	assert.Equal(t, 1, stats["installing_count"])
	assert.Equal(t, 3, stats["cached_versions"])

	// Verify GetEnvironmentStats returns same data
	envStats := sm.GetEnvironmentStats()
	assert.Equal(t, stats, envStats)
}

func TestStateManager_Reset(t *testing.T) {
	sm := NewEnvironmentStateManager()

	// Add some state
	sm.MarkEnvironmentInitialized("env1")
	_ = sm.MarkEnvironmentInstalling("env2")
	sm.SetCachedVersion("ver1", "1.0.0")

	// Verify state exists
	stats := sm.GetEnvironmentStats()
	assert.Equal(t, 1, stats["initialized_count"])
	assert.Equal(t, 1, stats["installing_count"])
	assert.Equal(t, 1, stats["cached_versions"])

	// Reset
	sm.Reset()

	// Verify state is cleared
	stats = sm.GetEnvironmentStats()
	assert.Equal(t, 0, stats["initialized_count"])
	assert.Equal(t, 0, stats["installing_count"])
	assert.Equal(t, 0, stats["cached_versions"])

	// Verify individual checks
	assert.False(t, sm.IsEnvironmentInitialized("env1"))
	assert.False(t, sm.IsEnvironmentInstalling("env2"))
	_, exists := sm.GetCachedVersion("ver1")
	assert.False(t, exists)
}

func TestStateManager_WaitForEnvironment(t *testing.T) {
	sm := NewEnvironmentStateManager()
	envKey := "python:3.9:/path/to/repo"

	// Test successful wait - environment becomes initialized
	t.Run("success - environment becomes initialized", func(t *testing.T) {
		sm.Reset()

		// Start as installing
		err := sm.MarkEnvironmentInstalling(envKey)
		require.NoError(t, err)

		// Simulate installation completion in another goroutine
		go func() {
			sm.MarkEnvironmentInitialized(envKey)
		}()

		// Wait should succeed
		err = sm.WaitForEnvironment(envKey, 10)
		assert.NoError(t, err)
	})

	// Test failure - environment not installing, not initialized
	t.Run("failure - environment not initialized", func(t *testing.T) {
		sm.Reset()

		err := sm.WaitForEnvironment(envKey, 5)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to initialize")
	})

	// Test timeout - environment stays installing
	t.Run("timeout - environment stays installing", func(t *testing.T) {
		sm.Reset()

		err := sm.MarkEnvironmentInstalling(envKey)
		require.NoError(t, err)

		err = sm.WaitForEnvironment(envKey, 3)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout waiting")
	})
}

func TestStateManager_ConcurrentAccess(t *testing.T) {
	sm := NewEnvironmentStateManager()
	numGoroutines := 100
	envKeyPrefix := "concurrent-env-"

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3) // 3 operations per goroutine

	// Test concurrent initialization
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			envKey := envKeyPrefix + string(rune(id))
			sm.MarkEnvironmentInitialized(envKey)
		}(i)
	}

	// Test concurrent installing
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			envKey := envKeyPrefix + "installing-" + string(rune(id))
			_ = sm.MarkEnvironmentInstalling(envKey)
		}(i)
	}

	// Test concurrent version caching
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			versionKey := "version-" + string(rune(id))
			version := "1.0." + string(rune(id))
			sm.SetCachedVersion(versionKey, version)
		}(i)
	}

	wg.Wait()

	// Verify state is consistent
	stats := sm.GetEnvironmentStats()
	assert.Equal(t, numGoroutines, stats["initialized_count"])
	assert.Equal(t, numGoroutines, stats["installing_count"])
	assert.Equal(t, numGoroutines, stats["cached_versions"])
}

func TestStateManager_Stats(t *testing.T) {
	// Test the Stats struct
	stats := Stats{
		InitializedCount: 5,
		InstallingCount:  3,
		CachedVersions:   10,
	}

	assert.Equal(t, 5, stats.InitializedCount)
	assert.Equal(t, 3, stats.InstallingCount)
	assert.Equal(t, 10, stats.CachedVersions)
}

func TestNewEnvironmentContext(t *testing.T) {
	cacheDir := "/tmp/cache"
	workingDir := "/tmp/work"

	ctx := NewEnvironmentContext(cacheDir, workingDir)
	assert.NotNil(t, ctx)
	assert.Equal(t, cacheDir, ctx.CacheDir)
	assert.Equal(t, workingDir, ctx.WorkingDir)
	assert.NotNil(t, ctx.StateManager)
}

func TestContext_CreateEnvironmentKey(t *testing.T) {
	ctx := NewEnvironmentContext("/tmp/cache", "/tmp/work")

	tests := []struct {
		language string
		version  string
		repoURL  string
		expected string
	}{
		{
			language: "python",
			version:  "3.9",
			repoURL:  "https://github.com/user/repo",
			expected: "python:3.9:https://github.com/user/repo",
		},
		{
			language: "node",
			version:  "16.0.0",
			repoURL:  "/local/path",
			expected: "node:16.0.0:/local/path",
		},
		{
			language: "ruby",
			version:  "",
			repoURL:  "",
			expected: "ruby::",
		},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			key := ctx.CreateEnvironmentKey(tt.language, tt.version, tt.repoURL)
			assert.Equal(t, tt.expected, key)
		})
	}
}

func TestGetGlobalEnvironmentManager(t *testing.T) {
	// Get the global manager
	manager1 := GetGlobalEnvironmentManager()
	assert.NotNil(t, manager1)

	// Get it again - should be the same instance
	manager2 := GetGlobalEnvironmentManager()
	assert.Same(t, manager1, manager2)

	// Test that it works
	envKey := "global-test"
	assert.False(t, manager1.IsEnvironmentInitialized(envKey))
	manager1.MarkEnvironmentInitialized(envKey)
	assert.True(t, manager1.IsEnvironmentInitialized(envKey))
	assert.True(t, manager2.IsEnvironmentInitialized(envKey))
}

func TestStateManager_InterfaceCompliance(t *testing.T) {
	// Test that StateManager implements interfaces.StateManager
	var sm any = NewEnvironmentStateManager()

	// This should compile if the interface is properly implemented
	_, ok := sm.(interface {
		IsEnvironmentInitialized(envKey string) bool
		MarkEnvironmentInitialized(envKey string)
		IsEnvironmentInstalling(envKey string) bool
		MarkEnvironmentInstalling(envKey string) error
		ClearEnvironmentInstalling(envKey string)
		GetCachedVersion(versionKey string) (string, bool)
		SetCachedVersion(versionKey, version string)
		GetStatistics() map[string]any
		Reset()
	})

	assert.True(t, ok, "StateManager should implement the expected interface")
}

func TestStateManager_EdgeCases(t *testing.T) {
	sm := NewEnvironmentStateManager()

	// Test empty keys
	t.Run("empty environment key", func(t *testing.T) {
		assert.False(t, sm.IsEnvironmentInitialized(""))
		sm.MarkEnvironmentInitialized("")
		assert.True(t, sm.IsEnvironmentInitialized(""))
	})

	// Test empty version key
	t.Run("empty version key", func(t *testing.T) {
		sm.SetCachedVersion("", "some-version")
		version, exists := sm.GetCachedVersion("")
		assert.True(t, exists)
		assert.Equal(t, "some-version", version)
	})

	// Test overwriting version
	t.Run("overwrite cached version", func(t *testing.T) {
		key := "overwrite-test"
		sm.SetCachedVersion(key, "v1.0.0")
		sm.SetCachedVersion(key, "v2.0.0")

		version, exists := sm.GetCachedVersion(key)
		assert.True(t, exists)
		assert.Equal(t, "v2.0.0", version)
	})

	// Test installing state edge cases
	t.Run("clear non-existent installing state", func(t *testing.T) {
		// Should not panic
		sm.ClearEnvironmentInstalling("non-existent")
		assert.False(t, sm.IsEnvironmentInstalling("non-existent"))
	})
}
