package language

import (
	"testing"
)

// MockCore implements Core interface for testing
type MockCore struct {
	name           string
	executableName string
	runtimeAvail   bool
	needsSetup     bool
}

func NewMockCore(name, executable string) *MockCore {
	return &MockCore{
		name:           name,
		executableName: executable,
		runtimeAvail:   true,
		needsSetup:     true,
	}
}

func (m *MockCore) GetName() string {
	return m.name
}

func (m *MockCore) GetExecutableName() string {
	return m.executableName
}

func (m *MockCore) IsRuntimeAvailable() bool {
	return m.runtimeAvail
}

func (m *MockCore) NeedsEnvironmentSetup() bool {
	return m.needsSetup
}

// MockEnvironmentManager implements EnvironmentManager interface for testing
type MockEnvironmentManager struct {
	envPath    string
	setupErr   error
	preInitErr error
	binPath    string
}

func NewMockEnvironmentManager() *MockEnvironmentManager {
	return &MockEnvironmentManager{
		envPath: "/mock/env/path",
		binPath: "/mock/env/path/bin",
	}
}

func (m *MockEnvironmentManager) SetupEnvironment(_, _ string, _ []string) (string, error) {
	if m.setupErr != nil {
		return "", m.setupErr
	}
	return m.envPath, nil
}

func (m *MockEnvironmentManager) SetupEnvironmentWithRepo(_, _, _, _ string, _ []string) (string, error) {
	if m.setupErr != nil {
		return "", m.setupErr
	}
	return m.envPath, nil
}

func (m *MockEnvironmentManager) SetupEnvironmentWithRepoInfo(_, _, _, _ string, _ []string) (string, error) {
	if m.setupErr != nil {
		return "", m.setupErr
	}
	return m.envPath, nil
}

func (m *MockEnvironmentManager) PreInitializeEnvironmentWithRepoInfo(_, _, _, _ string, _ []string) error {
	return m.preInitErr
}

func (m *MockEnvironmentManager) GetEnvironmentBinPath(_ string) string {
	return m.binPath
}

// MockHealthChecker implements HealthChecker interface for testing
type MockHealthChecker struct {
	healthErr error
	healthy   bool
}

func NewMockHealthChecker() *MockHealthChecker {
	return &MockHealthChecker{
		healthy: true,
	}
}

func (m *MockHealthChecker) CheckEnvironmentHealth(_ string) bool {
	return m.healthy
}

func (m *MockHealthChecker) CheckHealth(_, _ string) error {
	return m.healthErr
}

// MockDependencyManager implements DependencyManager interface for testing
type MockDependencyManager struct {
	installErr error
}

func NewMockDependencyManager() *MockDependencyManager {
	return &MockDependencyManager{}
}

func (m *MockDependencyManager) InstallDependencies(_ string, _ []string) error {
	return m.installErr
}

// MockManager implements Manager interface by embedding all smaller interfaces
type MockManager struct {
	*MockCore
	*MockEnvironmentManager
	*MockHealthChecker
	*MockDependencyManager
}

func NewMockManager(name, executable string) *MockManager {
	return &MockManager{
		MockCore:               NewMockCore(name, executable),
		MockEnvironmentManager: NewMockEnvironmentManager(),
		MockHealthChecker:      NewMockHealthChecker(),
		MockDependencyManager:  NewMockDependencyManager(),
	}
}

// Test Core interface
func TestCoreInterface(t *testing.T) {
	mock := NewMockCore("TestLang", "testlang")

	t.Run("GetName", func(t *testing.T) {
		name := mock.GetName()
		if name != "TestLang" {
			t.Errorf("Expected 'TestLang', got %s", name)
		}
	})

	t.Run("GetExecutableName", func(t *testing.T) {
		executable := mock.GetExecutableName()
		if executable != "testlang" {
			t.Errorf("Expected 'testlang', got %s", executable)
		}
	})

	t.Run("IsRuntimeAvailable", func(t *testing.T) {
		available := mock.IsRuntimeAvailable()
		if !available {
			t.Error("Expected runtime to be available")
		}

		mock.runtimeAvail = false
		available = mock.IsRuntimeAvailable()
		if available {
			t.Error("Expected runtime to not be available")
		}
	})

	t.Run("NeedsEnvironmentSetup", func(t *testing.T) {
		needs := mock.NeedsEnvironmentSetup()
		if !needs {
			t.Error("Expected environment setup to be needed")
		}

		mock.needsSetup = false
		needs = mock.NeedsEnvironmentSetup()
		if needs {
			t.Error("Expected environment setup to not be needed")
		}
	})
}

// Test EnvironmentManager interface
func TestEnvironmentManagerInterface(t *testing.T) {
	mock := NewMockEnvironmentManager()

	t.Run("SetupEnvironment", func(t *testing.T) {
		envPath, err := mock.SetupEnvironment("/cache", "1.0", []string{"dep1"})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if envPath != "/mock/env/path" {
			t.Errorf("Expected '/mock/env/path', got %s", envPath)
		}
	})

	t.Run("SetupEnvironmentWithRepo", func(t *testing.T) {
		envPath, err := mock.SetupEnvironmentWithRepo("/cache", "1.0", "/repo", "http://repo.com", []string{"dep1"})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if envPath != "/mock/env/path" {
			t.Errorf("Expected '/mock/env/path', got %s", envPath)
		}
	})

	t.Run("GetEnvironmentBinPath", func(t *testing.T) {
		binPath := mock.GetEnvironmentBinPath("/some/env")
		if binPath != "/mock/env/path/bin" {
			t.Errorf("Expected '/mock/env/path/bin', got %s", binPath)
		}
	})

	t.Run("PreInitializeEnvironmentWithRepoInfo", func(t *testing.T) {
		err := mock.PreInitializeEnvironmentWithRepoInfo("/cache", "1.0", "/repo", "http://repo.com", []string{"dep1"})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

// Test HealthChecker interface
func TestHealthCheckerInterface(t *testing.T) {
	mock := NewMockHealthChecker()

	t.Run("CheckEnvironmentHealth", func(t *testing.T) {
		healthy := mock.CheckEnvironmentHealth("/env/path")
		if !healthy {
			t.Error("Expected environment to be healthy")
		}

		mock.healthy = false
		healthy = mock.CheckEnvironmentHealth("/env/path")
		if healthy {
			t.Error("Expected environment to not be healthy")
		}
	})

	t.Run("CheckHealth", func(t *testing.T) {
		err := mock.CheckHealth("/env/path", "1.0")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

// Test DependencyManager interface
func TestDependencyManagerInterface(t *testing.T) {
	mock := NewMockDependencyManager()

	t.Run("InstallDependencies", func(t *testing.T) {
		deps := []string{"dep1", "dep2"}
		err := mock.InstallDependencies("/env/path", deps)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

// Test Manager interface (composite interface)
func TestManagerInterface(t *testing.T) {
	mock := NewMockManager("Python", "python")

	// Test Core methods
	if mock.GetName() != "Python" {
		t.Errorf("Expected 'Python', got %s", mock.GetName())
	}

	// Test EnvironmentManager methods
	envPath, err := mock.SetupEnvironment("/cache", "3.9", nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if envPath == "" {
		t.Error("Expected non-empty environment path")
	}

	// Test HealthChecker methods
	healthy := mock.CheckEnvironmentHealth("/env/path")
	if !healthy {
		t.Error("Expected environment to be healthy")
	}

	// Test DependencyManager methods
	err = mock.InstallDependencies("/env/path", []string{"requests"})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
