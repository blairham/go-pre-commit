package interfaces //nolint:revive // interfaces is an appropriate name for this package containing interface definitions

import (
	"context"
	"errors"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// MockCacheManager implements CacheManager interface for testing
type MockCacheManager struct {
	updateError   error
	cleanError    error
	markUsedError error
	repoPath      string
	closed        bool
}

func NewMockCacheManager() *MockCacheManager {
	return &MockCacheManager{
		repoPath: "/mock/repo/path",
	}
}

func (m *MockCacheManager) GetRepoPath(_ config.Repo) string {
	return m.repoPath
}

func (m *MockCacheManager) GetRepoPathWithDeps(_ config.Repo, _ []string) string {
	return m.repoPath + "-with-deps"
}

func (m *MockCacheManager) UpdateRepoEntry(_ config.Repo, _ string) error {
	return m.updateError
}

func (m *MockCacheManager) UpdateRepoEntryWithDeps(_ config.Repo, _ []string, _ string) error {
	return m.updateError
}

func (m *MockCacheManager) CleanCache() error {
	return m.cleanError
}

func (m *MockCacheManager) MarkConfigUsed(_ string) error {
	return m.markUsedError
}

func (m *MockCacheManager) Close() error {
	m.closed = true
	return nil
}

// MockRepositoryManager implements RepositoryManager interface for testing
type MockRepositoryManager struct {
	cloneError  error
	setupError  error
	initError   error
	updateError error
	envVars     map[string]string
	repoPath    string
	isLocal     bool
	isMeta      bool
}

func NewMockRepositoryManager() *MockRepositoryManager {
	return &MockRepositoryManager{
		repoPath: "/mock/repo/path",
		envVars:  map[string]string{"TEST_VAR": "test_value"},
	}
}

func (m *MockRepositoryManager) CloneOrUpdateRepo(_ context.Context, _ config.Repo) (string, error) {
	if m.cloneError != nil {
		return "", m.cloneError
	}
	return m.repoPath, nil
}

func (m *MockRepositoryManager) CloneOrUpdateRepoWithDeps(
	_ context.Context,
	_ config.Repo,
	_ []string,
) (string, error) {
	if m.cloneError != nil {
		return "", m.cloneError
	}
	return m.repoPath + "-with-deps", nil
}

func (m *MockRepositoryManager) SetupHookEnvironment(
	_ config.Hook,
	_ config.Repo,
	_ string,
) (map[string]string, error) {
	if m.setupError != nil {
		return nil, m.setupError
	}
	return m.envVars, nil
}

func (m *MockRepositoryManager) IsLocalRepo(_ config.Repo) bool {
	return m.isLocal
}

func (m *MockRepositoryManager) IsMetaRepo(_ config.Repo) bool {
	return m.isMeta
}

func (m *MockRepositoryManager) InitializeRepositoryCommon(_, _, _ string) error {
	return m.initError
}

func (m *MockRepositoryManager) UpdateDatabaseEntry(_, _, _ string) error {
	return m.updateError
}

// MockDownloadManager implements DownloadManager interface for testing
type MockDownloadManager struct {
	downloadError error
	extractError  error
	installError  error
	makeExecError error
	os            string
	arch          string
}

func NewMockDownloadManager() *MockDownloadManager {
	return &MockDownloadManager{
		os:   "linux",
		arch: "amd64",
	}
}

func (m *MockDownloadManager) DownloadFile(_, _ string) error {
	return m.downloadError
}

func (m *MockDownloadManager) ExtractTarGz(_, _ string) error {
	return m.extractError
}

func (m *MockDownloadManager) ExtractZip(_, _ string) error {
	return m.extractError
}

func (m *MockDownloadManager) InstallBinary(_, _, _ string) error {
	return m.installError
}

func (m *MockDownloadManager) MakeBinaryExecutable(_ string) error {
	return m.makeExecError
}

func (m *MockDownloadManager) GetNormalizedOS() string {
	return m.os
}

func (m *MockDownloadManager) GetNormalizedArch() string {
	return m.arch
}

// MockPackageManager implements PackageManager interface for testing
type MockPackageManager struct {
	createError    error
	runError       error
	manifestExists bool
}

func NewMockPackageManager() *MockPackageManager {
	return &MockPackageManager{
		manifestExists: true,
	}
}

func (m *MockPackageManager) CreateManifest(_ string, _ any) error {
	return m.createError
}

func (m *MockPackageManager) RunInstallCommand(_ string, _ any) error {
	return m.runError
}

func (m *MockPackageManager) CheckManifestExists(_ string, _ any) bool {
	return m.manifestExists
}

// MockStateManager implements StateManager interface for testing
type MockStateManager struct {
	statistics     map[string]any
	initialized    map[string]bool
	installing     map[string]bool
	markInstallErr error
}

func NewMockStateManager() *MockStateManager {
	return &MockStateManager{
		statistics:  map[string]any{"test": "value"},
		initialized: make(map[string]bool),
		installing:  make(map[string]bool),
	}
}

func (m *MockStateManager) GetStatistics() map[string]any {
	return m.statistics
}

func (m *MockStateManager) IsEnvironmentInitialized(envKey string) bool {
	return m.initialized[envKey]
}

func (m *MockStateManager) IsEnvironmentInstalling(envKey string) bool {
	return m.installing[envKey]
}

func (m *MockStateManager) MarkEnvironmentInstalling(envKey string) error {
	if m.markInstallErr != nil {
		return m.markInstallErr
	}
	m.installing[envKey] = true
	return nil
}

func (m *MockStateManager) ClearEnvironmentInstalling(envKey string) {
	delete(m.installing, envKey)
}

func (m *MockStateManager) MarkEnvironmentInitialized(envKey string) {
	m.initialized[envKey] = true
	delete(m.installing, envKey)
}

func (m *MockStateManager) GetEnvironmentStats() map[string]any {
	return map[string]any{
		"initialized": len(m.initialized),
		"installing":  len(m.installing),
	}
}

func (m *MockStateManager) Reset() {
	m.initialized = make(map[string]bool)
	m.installing = make(map[string]bool)
}

// Test CacheManager interface
func TestCacheManagerInterface(t *testing.T) {
	mock := NewMockCacheManager()
	repo := config.Repo{Repo: "test-repo"}

	t.Run("GetRepoPath", func(t *testing.T) {
		path := mock.GetRepoPath(repo)
		if path != "/mock/repo/path" {
			t.Errorf("Expected '/mock/repo/path', got %s", path)
		}
	})

	t.Run("GetRepoPathWithDeps", func(t *testing.T) {
		deps := []string{"dep1", "dep2"}
		path := mock.GetRepoPathWithDeps(repo, deps)
		expected := "/mock/repo/path-with-deps"
		if path != expected {
			t.Errorf("Expected '%s', got %s", expected, path)
		}
	})

	t.Run("UpdateRepoEntry", func(t *testing.T) {
		err := mock.UpdateRepoEntry(repo, "/test/path")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		mock.updateError = errors.New("update failed")
		err = mock.UpdateRepoEntry(repo, "/test/path")
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("Close", func(t *testing.T) {
		err := mock.Close()
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !mock.closed {
			t.Error("Expected mock to be marked as closed")
		}
	})
}

// Test RepositoryManager interface
func TestRepositoryManagerInterface(t *testing.T) {
	mock := NewMockRepositoryManager()
	repo := config.Repo{Repo: "test-repo"}
	hook := config.Hook{ID: "test-hook"}

	t.Run("CloneOrUpdateRepo", func(t *testing.T) {
		path, err := mock.CloneOrUpdateRepo(context.Background(), repo)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if path != "/mock/repo/path" {
			t.Errorf("Expected '/mock/repo/path', got %s", path)
		}

		mock.cloneError = errors.New("clone failed")
		_, err = mock.CloneOrUpdateRepo(context.Background(), repo)
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("SetupHookEnvironment", func(t *testing.T) {
		env, err := mock.SetupHookEnvironment(hook, repo, "/test/path")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if env["TEST_VAR"] != "test_value" {
			t.Errorf("Expected 'test_value', got %s", env["TEST_VAR"])
		}
	})

	t.Run("IsLocalRepo", func(t *testing.T) {
		isLocal := mock.IsLocalRepo(repo)
		if isLocal {
			t.Error("Expected false, got true")
		}

		mock.isLocal = true
		isLocal = mock.IsLocalRepo(repo)
		if !isLocal {
			t.Error("Expected true, got false")
		}
	})
}

// Test DownloadManager interface
func TestDownloadManagerInterface(t *testing.T) {
	mock := NewMockDownloadManager()

	t.Run("DownloadFile", func(t *testing.T) {
		err := mock.DownloadFile("http://test.com/file", "/dest/file")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		mock.downloadError = errors.New("download failed")
		err = mock.DownloadFile("http://test.com/file", "/dest/file")
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("GetNormalizedOS", func(t *testing.T) {
		os := mock.GetNormalizedOS()
		if os != "linux" {
			t.Errorf("Expected 'linux', got %s", os)
		}
	})

	t.Run("GetNormalizedArch", func(t *testing.T) {
		arch := mock.GetNormalizedArch()
		if arch != "amd64" {
			t.Errorf("Expected 'amd64', got %s", arch)
		}
	})
}

// Test PackageManager interface
func TestPackageManagerInterface(t *testing.T) {
	mock := NewMockPackageManager()

	t.Run("CreateManifest", func(t *testing.T) {
		err := mock.CreateManifest("/test/path", map[string]string{"test": "manifest"})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		mock.createError = errors.New("create failed")
		err = mock.CreateManifest("/test/path", map[string]string{"test": "manifest"})
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("CheckManifestExists", func(t *testing.T) {
		exists := mock.CheckManifestExists("/test/path", "package.json")
		if !exists {
			t.Error("Expected true, got false")
		}

		mock.manifestExists = false
		exists = mock.CheckManifestExists("/test/path", "package.json")
		if exists {
			t.Error("Expected false, got true")
		}
	})
}

// Test StateManager interface
func TestStateManagerInterface(t *testing.T) {
	mock := NewMockStateManager()

	t.Run("GetStatistics", func(t *testing.T) {
		stats := mock.GetStatistics()
		if stats["test"] != "value" {
			t.Errorf("Expected 'value', got %v", stats["test"])
		}
	})

	t.Run("Environment State Management", func(t *testing.T) {
		envKey := "test-env"

		// Initially not initialized or installing
		if mock.IsEnvironmentInitialized(envKey) {
			t.Error("Expected environment to not be initialized")
		}
		if mock.IsEnvironmentInstalling(envKey) {
			t.Error("Expected environment to not be installing")
		}

		// Mark as installing
		err := mock.MarkEnvironmentInstalling(envKey)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if !mock.IsEnvironmentInstalling(envKey) {
			t.Error("Expected environment to be installing")
		}

		// Mark as initialized
		mock.MarkEnvironmentInitialized(envKey)
		if !mock.IsEnvironmentInitialized(envKey) {
			t.Error("Expected environment to be initialized")
		}
		if mock.IsEnvironmentInstalling(envKey) {
			t.Error("Expected environment to not be installing after initialization")
		}

		// Reset
		mock.Reset()
		if mock.IsEnvironmentInitialized(envKey) {
			t.Error("Expected environment to not be initialized after reset")
		}
	})

	t.Run("GetEnvironmentStats", func(t *testing.T) {
		stats := mock.GetEnvironmentStats()
		if stats["initialized"] != 0 {
			t.Errorf("Expected 0 initialized, got %v", stats["initialized"])
		}
		if stats["installing"] != 0 {
			t.Errorf("Expected 0 installing, got %v", stats["installing"])
		}
	})
}
