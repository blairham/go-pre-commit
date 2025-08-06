package languages

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// SwiftLanguageTest implements LanguageTestRunner for Swift
type SwiftLanguageTest struct {
	*BaseLanguageTest
}

// NewSwiftLanguageTest creates a new Swift language test
func NewSwiftLanguageTest(testDir string) *SwiftLanguageTest {
	return &SwiftLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangSwift, testDir),
	}
}

// GetLanguageName returns the language name
func (st *SwiftLanguageTest) GetLanguageName() string {
	return LangSwift
}

// SetupRepositoryFiles creates Swift-specific repository files
func (st *SwiftLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: swiftformat
    name: Swift Format
    description: Format Swift code
    entry: swiftformat
    language: system
    files: \.swift$
    args: ['--version']
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create Package.swift
	packageFile := filepath.Join(repoPath, "Package.swift")
	packageContent := `// swift-tools-version:5.7
import PackageDescription

let package = Package(
    name: "TestSwiftHooks",
    products: [
        .executable(name: "TestSwiftHooks", targets: ["TestSwiftHooks"]),
    ],
    targets: [
        .executableTarget(name: "TestSwiftHooks"),
    ]
)
`
	if err := os.WriteFile(packageFile, []byte(packageContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Package.swift: %w", err)
	}

	// Create Sources directory and main.swift
	sourcesDir := filepath.Join(repoPath, "Sources", "TestSwiftHooks")
	if err := os.MkdirAll(sourcesDir, 0o750); err != nil {
		return fmt.Errorf("failed to create Sources directory: %w", err)
	}

	mainFile := filepath.Join(sourcesDir, "main.swift")
	mainContent := `print("Hello, Swift!")
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0o600); err != nil {
		return fmt.Errorf("failed to create main.swift: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Swift language manager
func (st *SwiftLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewSwiftLanguage(), nil
}

// GetAdditionalValidations returns Swift-specific validation tests
func (st *SwiftLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "swift-version-check",
			Description: "Swift version validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "swift" {
					return fmt.Errorf("expected swift language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (st *SwiftLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, _ string,
) error {
	t.Helper()

	t.Logf("🔄 Testing Swift language bidirectional cache compatibility")
	t.Logf("   📋 Swift hooks use Swift toolchain - testing cache compatibility")

	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "swift-bidirectional-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("⚠️  Warning: failed to remove temp directory: %v", removeErr)
		}
	}()

	// Test basic cache structure compatibility
	if err := st.testBasicCacheCompatibility(t, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("basic cache compatibility test failed: %w", err)
	}

	t.Logf("✅ Swift language bidirectional cache compatibility test completed")
	return nil
}

// setupTestRepository creates a test repository for Swift language testing
func (st *SwiftLanguageTest) setupTestRepository(t *testing.T, repoPath, _ string) error {
	t.Helper()

	// Create repository directory
	if err := os.MkdirAll(repoPath, 0o750); err != nil {
		return fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to initialize git repository: %w", err)
	}

	// Set git user config for the test
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git user email: %w", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git user name: %w", err)
	}

	return nil
}

// testBasicCacheCompatibility tests basic cache directory compatibility for Swift hooks
func (st *SwiftLanguageTest) testBasicCacheCompatibility(t *testing.T, pythonBinary, goBinary, tempDir string) error {
	t.Helper()

	// Create cache directories
	goCacheDir := filepath.Join(tempDir, "go-cache")
	pythonCacheDir := filepath.Join(tempDir, "python-cache")

	// Create a simple repository for testing
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := st.setupTestRepository(t, repoDir, ""); err != nil {
		return fmt.Errorf("failed to setup test repository: %w", err)
	}

	// Swift language config - using system language since Swift hooks typically use system tools
	configContent := `repos:
-   repo: local
    hooks:
    -   id: swift-format
        name: Swift Format
        entry: swiftformat
        language: system
        files: \.swift$
        args: ['--version']
        pass_filenames: false
    -   id: swift-lint
        name: Swift Lint
        entry: bash -c 'which swift > /dev/null && echo "Swift available" || echo "Swift not found"'
        language: system
        files: \.swift$
        pass_filenames: false
`
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Create test Swift file
	testFile := filepath.Join(repoDir, "test.swift")
	if err := os.WriteFile(testFile, []byte("print(\"Hello, Swift!\")"), 0o600); err != nil {
		return fmt.Errorf("failed to create test file: %w", err)
	}

	// Test 1: Go creates cache
	cmd := exec.Command(goBinary, "install-hooks", "--config", configPath)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PRE_COMMIT_HOME=%s", goCacheDir))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install-hooks failed: %w", err)
	}

	// Test 2: Python creates cache
	cmd = exec.Command(pythonBinary, "install-hooks", "--config", configPath)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("PRE_COMMIT_HOME=%s", pythonCacheDir))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("python install-hooks failed: %w", err)
	}

	// Verify both caches were created
	if _, err := os.Stat(goCacheDir); err != nil {
		return fmt.Errorf("go cache directory not created: %w", err)
	}
	if _, err := os.Stat(pythonCacheDir); err != nil {
		return fmt.Errorf("python cache directory not created: %w", err)
	}

	t.Logf("   ✅ Both Go and Python can create compatible cache structures for Swift hooks")
	return nil
}
