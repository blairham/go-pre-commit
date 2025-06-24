package environment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/constants"
	"github.com/blairham/go-pre-commit/pkg/language"
)

const testPythonEnvPath = constants.TmpPythonEnvPath

// Mock implementations for testing

// mockLanguageManager implements both interfaces.LanguageManager and language.Manager
type mockLanguageManager struct {
	setupError         error
	installError       error
	checkHealthError   error
	preInitError       error
	setupWithRepoError error
	callCounts         map[string]int
	name               string
	envPath            string
	binPath            string
	mu                 sync.Mutex
	runtimeAvailable   bool
	healthCheck        bool
}

func newMockLanguageManager(name string) *mockLanguageManager {
	return &mockLanguageManager{
		name:             name,
		runtimeAvailable: true,
		envPath:          filepath.Join("/tmp", "env", name),
		binPath:          filepath.Join("/tmp", "env", name, "bin"),
		healthCheck:      true,
		callCounts:       make(map[string]int),
	}
}

func (m *mockLanguageManager) incrementCallCount(method string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCounts[method]++
}

func (m *mockLanguageManager) getCallCount(method string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCounts[method]
}

// language.Manager interface methods
func (m *mockLanguageManager) GetName() string {
	m.incrementCallCount("GetName")
	return m.name
}

func (m *mockLanguageManager) SetupEnvironment(
	_ /* cacheDir */, _ /* version */ string, _ /* additionalDeps */ []string,
) (string, error) {
	m.incrementCallCount("SetupEnvironment")
	if m.setupError != nil {
		return "", m.setupError
	}
	return m.envPath, nil
}

func (m *mockLanguageManager) SetupEnvironmentWithRepo(
	_ /* cacheDir */, _ /* version */, _ /* repoPath */, _ /* repoURL */ string,
	_ /* additionalDeps */ []string,
) (string, error) {
	m.incrementCallCount("SetupEnvironmentWithRepo")
	if m.setupWithRepoError != nil {
		return "", m.setupWithRepoError
	}
	return m.envPath, nil
}

func (m *mockLanguageManager) GetEnvironmentBinPath(_ /* envPath */ string) string {
	m.incrementCallCount("GetEnvironmentBinPath")
	return m.binPath
}

func (m *mockLanguageManager) IsRuntimeAvailable() bool {
	m.incrementCallCount("IsRuntimeAvailable")
	return m.runtimeAvailable
}

func (m *mockLanguageManager) CheckEnvironmentHealth(_ /* envPath */ string) bool {
	m.incrementCallCount("CheckEnvironmentHealth")
	return m.healthCheck
}

func (m *mockLanguageManager) SetupEnvironmentWithRepoInfo(
	_ /* cacheDir */, _ /* version */, _ /* repoPath */, _ /* repoURL */ string,
	_ /* additionalDeps */ []string,
) (string, error) {
	m.incrementCallCount("SetupEnvironmentWithRepoInfo")
	if m.setupWithRepoError != nil {
		return "", m.setupWithRepoError
	}
	return m.envPath, nil
}

func (m *mockLanguageManager) PreInitializeEnvironmentWithRepoInfo(
	_ /* cacheDir */, _ /* version */, _ /* repoPath */, _ /* repoURL */ string,
	_ /* additionalDeps */ []string,
) error {
	m.incrementCallCount("PreInitializeEnvironmentWithRepoInfo")
	return m.preInitError
}

func (m *mockLanguageManager) InstallDependencies(_ /* envPath */ string, _ /* deps */ []string) error {
	m.incrementCallCount("InstallDependencies")
	return m.installError
}

func (m *mockLanguageManager) GetExecutableName() string {
	m.incrementCallCount("GetExecutableName")
	return m.name
}

func (m *mockLanguageManager) CheckHealth(_ /* envPath */, _ /* version */ string) error {
	m.incrementCallCount("CheckHealth")
	return m.checkHealthError
}

func (m *mockLanguageManager) NeedsEnvironmentSetup() bool {
	m.incrementCallCount("NeedsEnvironmentSetup")
	return true
}

func TestNewManager(t *testing.T) {
	cacheDir := "/tmp/test-cache"
	manager := NewManager(cacheDir)

	if manager.cacheDir != cacheDir {
		t.Errorf("Expected cache dir %s, got %s", cacheDir, manager.cacheDir)
	}

	if manager.languageMap == nil {
		t.Error("Language map should be initialized")
	}

	if manager.stateManager == nil {
		t.Error("State manager should be initialized")
	}

	if len(manager.languageMap) != 0 {
		t.Error("Language map should be empty initially")
	}
}

func TestManager_GetCacheDir(t *testing.T) {
	cacheDir := "/tmp/test-cache"
	manager := NewManager(cacheDir)

	if got := manager.GetCacheDir(); got != cacheDir {
		t.Errorf("Expected cache dir %s, got %s", cacheDir, got)
	}
}

func TestManager_Close(t *testing.T) {
	manager := NewManager("/tmp/test")

	// Add some languages to the map
	manager.languageMap["python"] = newMockLanguageManager("python")
	manager.languageMap["node"] = newMockLanguageManager("node")

	if len(manager.languageMap) != 2 {
		t.Error("Expected 2 languages in map before close")
	}

	err := manager.Close()
	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}

	if len(manager.languageMap) != 0 {
		t.Error("Language map should be empty after close")
	}
}

func TestManager_SetupEnvironment(t *testing.T) {
	tests := []struct {
		mockSetup      func(*mockLanguageManager)
		name           string
		lang           string
		version        string
		repoPath       string
		expectPath     string
		additionalDeps []string
		expectError    bool
	}{
		{
			name:        "successful setup without repo",
			lang:        "python",
			version:     "3.9",
			expectError: false,
			expectPath:  testPythonEnvPath,
			mockSetup:   func(_ *mockLanguageManager) {},
		},
		{
			name:        "successful setup with repo",
			lang:        "python",
			version:     "3.9",
			repoPath:    "/tmp/repo",
			expectError: false,
			expectPath:  testPythonEnvPath,
			mockSetup:   func(_ *mockLanguageManager) {},
		},
		{
			name:        "runtime not available",
			lang:        "python",
			version:     "3.9",
			expectError: true,
			mockSetup: func(m *mockLanguageManager) {
				m.runtimeAvailable = false
			},
		},
		{
			name:        "setup error without repo",
			lang:        "python",
			version:     "3.9",
			expectError: true,
			mockSetup: func(m *mockLanguageManager) {
				m.setupError = errors.New("setup failed")
			},
		},
		{
			name:        "setup error with repo",
			lang:        "python",
			version:     "3.9",
			repoPath:    "/tmp/repo",
			expectError: true,
			mockSetup: func(m *mockLanguageManager) {
				m.setupWithRepoError = errors.New("setup with repo failed")
			},
		},
		{
			name:           "with additional dependencies",
			lang:           "python",
			version:        "3.9",
			additionalDeps: []string{"requests", "numpy"},
			expectError:    false,
			expectPath:     testPythonEnvPath,
			mockSetup:      func(_ *mockLanguageManager) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager("/tmp/cache")
			mockLang := newMockLanguageManager(tt.lang)
			tt.mockSetup(mockLang)

			// Inject the mock directly into the manager
			manager.languageMap[tt.lang] = mockLang

			path, err := manager.SetupEnvironment(tt.lang, tt.version, tt.additionalDeps, tt.repoPath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if path != tt.expectPath {
				t.Errorf("Expected path %s, got %s", tt.expectPath, path)
			}

			// Verify correct method was called
			if tt.repoPath != "" {
				if mockLang.getCallCount("SetupEnvironmentWithRepo") != 1 {
					t.Error("Expected SetupEnvironmentWithRepo to be called once")
				}
			} else {
				if mockLang.getCallCount("SetupEnvironment") != 1 {
					t.Error("Expected SetupEnvironment to be called once")
				}
			}
		})
	}
}

func TestManager_GetEnvironmentBinPath(t *testing.T) {
	manager := NewManager("/tmp/cache")
	mockLang := newMockLanguageManager("python")
	manager.languageMap["python"] = mockLang

	binPath, err := manager.GetEnvironmentBinPath("python", testPythonEnvPath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedPath := "/tmp/env/python/bin"
	if binPath != expectedPath {
		t.Errorf("Expected bin path %s, got %s", expectedPath, binPath)
	}

	if mockLang.getCallCount("GetEnvironmentBinPath") != 1 {
		t.Error("Expected GetEnvironmentBinPath to be called once")
	}
}

func TestManager_IsRuntimeAvailable(t *testing.T) {
	tests := []struct {
		name             string
		lang             string
		runtimeAvailable bool
		expectAvailable  bool
	}{
		{
			name:             "runtime available",
			lang:             "python",
			runtimeAvailable: true,
			expectAvailable:  true,
		},
		{
			name:             "runtime not available",
			lang:             "python",
			runtimeAvailable: false,
			expectAvailable:  false,
		},
		{
			name:            "unsupported language",
			lang:            "unsupported",
			expectAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager("/tmp/cache")

			if tt.lang != "unsupported" {
				mockLang := newMockLanguageManager(tt.lang)
				mockLang.runtimeAvailable = tt.runtimeAvailable
				manager.languageMap[tt.lang] = mockLang
			}

			available := manager.IsRuntimeAvailable(tt.lang)
			if available != tt.expectAvailable {
				t.Errorf("Expected runtime available %v, got %v", tt.expectAvailable, available)
			}
		})
	}
}

func TestManager_InstallDependencies(t *testing.T) {
	tests := []struct {
		installError error
		name         string
		lang         string
		envPath      string
		deps         []string
		expectError  bool
	}{
		{
			name:        "successful install",
			lang:        "python",
			envPath:     "/tmp/env",
			deps:        []string{"requests", "numpy"},
			expectError: false,
		},
		{
			name:         "install error",
			lang:         "python",
			envPath:      "/tmp/env",
			deps:         []string{"requests"},
			installError: errors.New("install failed"),
			expectError:  true,
		},
		{
			name:        "empty dependencies",
			lang:        "python",
			envPath:     "/tmp/env",
			deps:        []string{},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager("/tmp/cache")
			mockLang := newMockLanguageManager(tt.lang)
			mockLang.installError = tt.installError
			manager.languageMap[tt.lang] = mockLang

			err := manager.InstallDependencies(tt.lang, tt.envPath, tt.deps)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if mockLang.getCallCount("InstallDependencies") != 1 {
				t.Error("Expected InstallDependencies to be called once")
			}
		})
	}
}

func TestManager_CheckEnvironmentHealth(t *testing.T) {
	tests := []struct {
		name        string
		lang        string
		envPath     string
		healthCheck bool
		expectError bool
		expectGood  bool
	}{
		{
			name:        "healthy environment",
			lang:        "python",
			envPath:     "/tmp/env",
			healthCheck: true,
			expectGood:  true,
		},
		{
			name:        "unhealthy environment",
			lang:        "python",
			envPath:     "/tmp/env",
			healthCheck: false,
			expectGood:  false,
		},
		{
			name:       "unsupported language",
			lang:       "unsupported",
			envPath:    "/tmp/env",
			expectGood: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager("/tmp/cache")

			if tt.lang != "unsupported" {
				mockLang := newMockLanguageManager(tt.lang)
				mockLang.healthCheck = tt.healthCheck
				manager.languageMap[tt.lang] = mockLang
			}

			isHealthy := manager.CheckEnvironmentHealth(tt.lang, tt.envPath)
			if isHealthy != tt.expectGood {
				t.Errorf("Expected health check %v, got %v", tt.expectGood, isHealthy)
			}
		})
	}
}

func TestManager_SetupHookEnvironment(t *testing.T) {
	hook := config.Hook{
		Language:        "python",
		LanguageVersion: "3.9",
		AdditionalDeps:  []string{"requests"},
	}
	repo := config.Repo{
		Repo: "https://github.com/example/repo",
		Rev:  "main",
	}
	repoPath := "/tmp/repo"

	manager := NewManager("/tmp/cache")
	mockLang := newMockLanguageManager("python")
	manager.languageMap["python"] = mockLang

	env, err := manager.SetupHookEnvironment(hook, repo, repoPath)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedEnv := map[string]string{
		"PRE_COMMIT_ENV_PATH": testPythonEnvPath,
		"PRE_COMMIT_LANGUAGE": "python",
		"PRE_COMMIT_VERSION":  "3.9",
	}

	for key, expectedValue := range expectedEnv {
		if actualValue, exists := env[key]; !exists || actualValue != expectedValue {
			t.Errorf("Expected env[%s] = %s, got %s (exists: %v)", key, expectedValue, actualValue, exists)
		}
	}
}

func TestManager_PreInitializeEnvironment(t *testing.T) {
	tests := []struct {
		preInitError   error
		name           string
		lang           string
		version        string
		repoPath       string
		additionalDeps []string
		expectError    bool
	}{
		{
			name:        "successful pre-init",
			lang:        "python",
			version:     "3.9",
			repoPath:    "/tmp/repo",
			expectError: false,
		},
		{
			name:         "pre-init error",
			lang:         "python",
			version:      "3.9",
			repoPath:     "/tmp/repo",
			preInitError: errors.New("pre-init failed"),
			expectError:  true,
		},
		{
			name:           "with additional deps",
			lang:           "python",
			version:        "3.9",
			repoPath:       "/tmp/repo",
			additionalDeps: []string{"requests", "numpy"},
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager("/tmp/cache")
			mockLang := newMockLanguageManager(tt.lang)
			mockLang.preInitError = tt.preInitError
			manager.languageMap[tt.lang] = mockLang

			err := manager.PreInitializeEnvironment(tt.lang, tt.version, tt.repoPath, tt.additionalDeps)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			if mockLang.getCallCount("PreInitializeEnvironmentWithRepoInfo") != 1 {
				t.Error("Expected PreInitializeEnvironmentWithRepoInfo to be called once")
			}
		})
	}
}

func TestManager_SetupEnvironmentWithRepo(t *testing.T) {
	manager := NewManager("/tmp/cache")
	mockLang := newMockLanguageManager("python")
	manager.languageMap["python"] = mockLang

	path, err := manager.SetupEnvironmentWithRepo("python", "3.9", "/tmp/repo", []string{"requests"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedPath := testPythonEnvPath
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}

	// This should call SetupEnvironment with repoPath parameter
	if mockLang.getCallCount("SetupEnvironmentWithRepo") != 1 {
		t.Error("Expected SetupEnvironmentWithRepo to be called once")
	}
}

func TestManager_CheckEnvironmentHealthWithRepo(t *testing.T) {
	tests := []struct {
		setupError  error
		name        string
		lang        string
		version     string
		repoPath    string
		healthCheck bool
		expectError bool
	}{
		{
			name:        "healthy environment",
			lang:        "python",
			version:     "3.9",
			repoPath:    "/tmp/repo",
			healthCheck: true,
			expectError: false,
		},
		{
			name:        "unhealthy environment",
			lang:        "python",
			version:     "3.9",
			repoPath:    "/tmp/repo",
			healthCheck: false,
			expectError: true,
		},
		{
			name:        "setup error",
			lang:        "python",
			version:     "3.9",
			repoPath:    "/tmp/repo",
			setupError:  errors.New("setup failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager("/tmp/cache")
			mockLang := newMockLanguageManager(tt.lang)
			mockLang.setupWithRepoError = tt.setupError
			mockLang.healthCheck = tt.healthCheck
			manager.languageMap[tt.lang] = mockLang

			err := manager.CheckEnvironmentHealthWithRepo(tt.lang, tt.version, tt.repoPath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestManager_RebuildEnvironmentWithRepo(t *testing.T) {
	tests := []struct {
		setupError  error
		name        string
		lang        string
		version     string
		repoPath    string
		expectError bool
	}{
		{
			name:        "successful rebuild",
			lang:        "python",
			version:     "3.9",
			repoPath:    "/tmp/repo",
			expectError: false,
		},
		{
			name:        "rebuild error",
			lang:        "python",
			version:     "3.9",
			repoPath:    "/tmp/repo",
			setupError:  errors.New("rebuild failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager("/tmp/cache")
			mockLang := newMockLanguageManager(tt.lang)
			mockLang.setupWithRepoError = tt.setupError
			manager.languageMap[tt.lang] = mockLang

			err := manager.RebuildEnvironmentWithRepo(tt.lang, tt.version, tt.repoPath)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestManager_RebuildEnvironmentWithRepoInfo(t *testing.T) {
	manager := NewManager("/tmp/cache")
	mockLang := newMockLanguageManager("python")
	manager.languageMap["python"] = mockLang

	err := manager.RebuildEnvironmentWithRepoInfo("python", "3.9", "/tmp/repo", "https://github.com/example/repo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should call SetupEnvironment
	if mockLang.getCallCount("SetupEnvironmentWithRepo") != 1 {
		t.Error("Expected SetupEnvironmentWithRepo to be called once")
	}
}

func TestManager_PreInitializeHookEnvironments(t *testing.T) {
	hooks := []config.HookEnvItem{
		{
			RepoPath: "/tmp/repo1",
			Repo:     config.Repo{Repo: "https://github.com/example/repo1"},
			Hook: config.Hook{
				Language:        "python",
				LanguageVersion: "3.9",
				AdditionalDeps:  []string{"requests"},
			},
		},
		{
			RepoPath: "/tmp/repo2",
			Repo:     config.Repo{Repo: "https://github.com/example/repo2"},
			Hook: config.Hook{
				Language:        "node",
				LanguageVersion: "16",
				AdditionalDeps:  []string{"express"},
			},
		},
	}

	manager := NewManager("/tmp/cache")
	mockPython := newMockLanguageManager("python")
	mockNode := newMockLanguageManager("node")
	manager.languageMap["python"] = mockPython
	manager.languageMap["node"] = mockNode

	err := manager.PreInitializeHookEnvironments(context.Background(), hooks, nil)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if mockPython.getCallCount("PreInitializeEnvironmentWithRepoInfo") != 1 {
		t.Error("Expected PreInitializeEnvironmentWithRepoInfo to be called once for python")
	}

	if mockNode.getCallCount("PreInitializeEnvironmentWithRepoInfo") != 1 {
		t.Error("Expected PreInitializeEnvironmentWithRepoInfo to be called once for node")
	}
}

func TestManager_PreInitializeHookEnvironments_Error(t *testing.T) {
	hooks := []config.HookEnvItem{
		{
			RepoPath: "/tmp/repo1",
			Hook: config.Hook{
				Language:        "python",
				LanguageVersion: "3.9",
			},
		},
	}

	manager := NewManager("/tmp/cache")
	mockPython := newMockLanguageManager("python")
	mockPython.preInitError = errors.New("pre-init failed")
	manager.languageMap["python"] = mockPython

	err := manager.PreInitializeHookEnvironments(context.Background(), hooks, nil)
	if err == nil {
		t.Error("Expected error but got none")
	}

	if !strings.Contains(fmt.Sprintf("%v", err), "failed to pre-initialize python environment") {
		t.Errorf("Expected error to mention python pre-initialization, got: %v", err)
	}
}

func TestManager_SetupEnvironmentWithRepositoryInit(t *testing.T) {
	repo := config.Repo{Repo: "https://github.com/example/repo"}
	manager := NewManager("/tmp/cache")
	mockLang := newMockLanguageManager("python")
	manager.languageMap["python"] = mockLang

	path, err := manager.SetupEnvironmentWithRepositoryInit(repo, "python", "3.9", []string{"requests"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedPath := testPythonEnvPath
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}
}

func TestManager_GetCommonRepositoryManager(t *testing.T) {
	manager := NewManager("/tmp/cache")
	repoOps := "mock-repo-ops"

	result := manager.GetCommonRepositoryManager(context.Background(), repoOps)
	if result != repoOps {
		t.Errorf("Expected repository ops to be returned as-is, got %v", result)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	manager := NewManager("/tmp/cache")
	mockLang := newMockLanguageManager("python")

	// Test concurrent access to getOrCreateLanguageManager
	var wg sync.WaitGroup
	const numGoroutines = 10

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Inject the mock directly for the first access
			manager.mutex.Lock()
			if _, exists := manager.languageMap["python"]; !exists {
				manager.languageMap["python"] = mockLang
			}
			manager.mutex.Unlock()

			// Test concurrent calls
			_, err := manager.SetupEnvironment("python", "3.9", nil, "")
			if err != nil {
				t.Errorf("Unexpected error in concurrent access: %v", err)
			}
		}()
	}

	wg.Wait()

	// Verify the language manager was created and used
	if mockLang.getCallCount("SetupEnvironment") != numGoroutines {
		t.Errorf(
			"Expected %d calls to SetupEnvironment, got %d",
			numGoroutines,
			mockLang.getCallCount("SetupEnvironment"),
		)
	}
}

func TestManager_UnsupportedLanguage(t *testing.T) {
	manager := NewManager("/tmp/cache")

	// Test with unsupported language (no mock injected)
	_, err := manager.SetupEnvironment("unsupported", "1.0", nil, "")
	if err == nil {
		t.Error("Expected error for unsupported language")
	}

	// Test other methods with unsupported language
	available := manager.IsRuntimeAvailable("unsupported")
	if available {
		t.Error("Expected runtime not available for unsupported language")
	}

	_, err = manager.GetEnvironmentBinPath("unsupported", "/tmp/env")
	if err == nil {
		t.Error("Expected error for unsupported language")
	}

	healthy := manager.CheckEnvironmentHealth("unsupported", "/tmp/env")
	if healthy {
		t.Error("Expected unhealthy environment for unsupported language")
	}
}

// Test the language manager adapter
func TestLanguageManagerAdapter(t *testing.T) {
	mockMgr := newMockLanguageManager("python")
	adapter := &languageManagerAdapter{mockMgr}

	// Test that it implements the language.Manager interface
	var _ language.Manager = adapter

	// Test the adaptation method
	path, err := adapter.SetupEnvironment("/tmp/cache", "3.9", []string{"requests"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedPath := testPythonEnvPath
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}

	// Verify it called the adapted method
	if mockMgr.getCallCount("SetupEnvironmentWithRepoInfo") != 1 {
		t.Error("Expected SetupEnvironmentWithRepoInfo to be called once")
	}
}

// Benchmark tests for performance
func BenchmarkManager_SetupEnvironment(b *testing.B) {
	manager := NewManager("/tmp/cache")
	mockLang := newMockLanguageManager("python")
	manager.languageMap["python"] = mockLang

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.SetupEnvironment("python", "3.9", nil, "")
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkManager_ConcurrentSetup(b *testing.B) {
	manager := NewManager("/tmp/cache")
	mockLang := newMockLanguageManager("python")
	manager.languageMap["python"] = mockLang

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := manager.SetupEnvironment("python", "3.9", nil, "")
			if err != nil {
				b.Fatalf("Unexpected error: %v", err)
			}
		}
	})
}

// Integration-style tests with temporary directories
func TestManager_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "env-manager-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewManager(tempDir)
	mockLang := newMockLanguageManager("python")

	// Set a realistic environment path within the temp directory
	mockLang.envPath = filepath.Join(tempDir, "env", "python")
	mockLang.binPath = filepath.Join(tempDir, "env", "python", "bin")
	manager.languageMap["python"] = mockLang

	// Test full workflow
	path, err := manager.SetupEnvironment("python", "3.9", []string{"requests"}, "")
	if err != nil {
		t.Errorf("Setup failed: %v", err)
	}

	binPath, err := manager.GetEnvironmentBinPath("python", path)
	if err != nil {
		t.Errorf("Get bin path failed: %v", err)
	}

	if !manager.IsRuntimeAvailable("python") {
		t.Error("Expected runtime to be available")
	}

	if !manager.CheckEnvironmentHealth("python", path) {
		t.Error("Expected environment to be healthy")
	}

	err = manager.InstallDependencies("python", path, []string{"numpy"})
	if err != nil {
		t.Errorf("Install dependencies failed: %v", err)
	}

	// Verify expected paths
	expectedBinPath := filepath.Join(tempDir, "env", "python", "bin")
	if binPath != expectedBinPath {
		t.Errorf("Expected bin path %s, got %s", expectedBinPath, binPath)
	}

	// Test cleanup
	err = manager.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
