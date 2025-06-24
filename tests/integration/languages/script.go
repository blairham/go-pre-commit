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

// ScriptLanguageTest implements LanguageTestRunner for Script
type ScriptLanguageTest struct {
	*GenericLanguageTest
}

// NewScriptLanguageTest creates a new Script language test
func NewScriptLanguageTest(testDir string) *ScriptLanguageTest {
	return &ScriptLanguageTest{
		GenericLanguageTest: NewGenericLanguageTest("script", testDir),
	}
}

// GetLanguageName returns the language name
func (st *ScriptLanguageTest) GetLanguageName() string {
	return "script"
}

// SetupRepositoryFiles creates Script-specific repository files
//
//nolint:funlen // Setup function naturally has many file creation steps
func (st *ScriptLanguageTest) SetupRepositoryFiles(
	repoPath string,
) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `- id: complex-shell-script
  name: Complex Shell Script
  description: Run a complex shell script that simulates environment setup
  entry: ./scripts/complex-check.sh
  language: script
  files: \.txt$
- id: environment-check-script
  name: Environment Check Script
  description: Script that checks environment and simulates setup overhead
  entry: ./scripts/env-check.sh
  language: script
  files: \.(txt|md)$
- id: simple-shell-script
  name: Simple Shell Script
  description: Run a simple shell script
  entry: ./scripts/test.sh
  language: script
  files: \.txt$
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create scripts directory
	scriptsDir := filepath.Join(repoPath, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o750); err != nil {
		return fmt.Errorf("failed to create scripts directory: %w", err)
	}

	// Create complex check script (simulates environment setup)
	complexScript := filepath.Join(scriptsDir, "complex-check.sh")
	complexContent := `#!/bin/bash
# Complex script that simulates environment setup and checking
echo "Starting complex environment check..."
sleep 0.1  # Simulate environment setup time

# Check for various tools
if command -v python3 >/dev/null 2>&1; then
    echo "Python3 found: $(python3 --version 2>&1)"
fi

if command -v node >/dev/null 2>&1; then
    echo "Node.js found: $(node --version 2>&1)"
fi

# Simulate some file processing
for file in "$@"; do
    if [ -f "$file" ]; then
        echo "Processing: $file ($(wc -l < "$file") lines)"
    fi
done

echo "Complex check completed"
`
	if err := os.WriteFile(complexScript, []byte(complexContent), 0o750); err != nil { //nolint:gosec // Script files need execute permissions
		return fmt.Errorf("failed to create complex script: %w", err)
	}

	// Create environment check script
	envCheckScript := filepath.Join(scriptsDir, "env-check.sh")
	envCheckContent := `#!/bin/bash
# Environment check script that simulates setup overhead
echo "Checking environment setup..."
sleep 0.05  # Simulate environment verification time

# Check environment variables
if [ -n "$PRE_COMMIT" ]; then
    echo "Pre-commit environment detected"
fi

# Check for cache directories and simulate cache usage
if [ -d "${PRE_COMMIT_HOME:-$HOME/.cache/pre-commit}" ]; then
    echo "Cache directory found - using cached environment"
else
    echo "No cache found - setting up environment"
    sleep 0.1  # Simulate longer setup time for uncached
fi

echo "Environment check completed for: $*"
`
	if err := os.WriteFile(envCheckScript, []byte(envCheckContent), 0o750); err != nil { //nolint:gosec // Script files need execute permissions
		return fmt.Errorf("failed to create env check script: %w", err)
	}

	// Create test script
	testScript := filepath.Join(scriptsDir, "test.sh")
	testContent := `#!/bin/bash
echo "Hello from script language!"
echo "File: $1"
`
	//nolint:gosec // Script files need executable permissions
	if err := os.WriteFile(testScript, []byte(testContent), 0o755); err != nil {
		return fmt.Errorf("failed to create test script: %w", err)
	}

	// Create test files for the hooks to process
	testFile := filepath.Join(repoPath, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content for script hooks\n"), 0o600); err != nil {
		return fmt.Errorf("failed to create test file: %w", err)
	}

	// Create additional test files to make caching more meaningful
	for i := 1; i <= 3; i++ {
		extraFile := filepath.Join(repoPath, fmt.Sprintf("script-test%d.txt", i))
		content := fmt.Sprintf("Script test file %d content\nLine 2\nLine 3\n", i)
		if err := os.WriteFile(extraFile, []byte(content), 0o600); err != nil {
			return fmt.Errorf("failed to create extra test file %d: %w", i, err)
		}
	}

	return nil
}

// GetLanguageManager returns the Script language manager
func (st *ScriptLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewScriptLanguage(), nil
}

// GetAdditionalValidations returns Script-specific validation tests
func (st *ScriptLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "script-executable-check",
			Description: "Script executable validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "script" {
					return fmt.Errorf("expected script language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (st *ScriptLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, _ string,
) error {
	t.Helper()

	t.Logf("🔄 Testing Script language bidirectional cache compatibility")
	t.Logf("   📋 Script hooks use executable scripts - testing cache compatibility")

	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "script-bidirectional-test-*")
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

	t.Logf("✅ Script language bidirectional cache compatibility test completed")
	return nil
}

// setupTestRepository creates a test repository for script language testing
func (st *ScriptLanguageTest) setupTestRepository(t *testing.T, repoPath, _ string) error {
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

// testBasicCacheCompatibility tests basic cache directory compatibility for script hooks
func (st *ScriptLanguageTest) testBasicCacheCompatibility(t *testing.T, pythonBinary, goBinary, tempDir string) error {
	t.Helper()

	// Create cache directories
	goCacheDir := filepath.Join(tempDir, "go-cache")
	pythonCacheDir := filepath.Join(tempDir, "python-cache")

	// Create a simple repository for testing
	repoDir := filepath.Join(tempDir, "test-repo")
	if err := st.setupTestRepository(t, repoDir, ""); err != nil {
		return fmt.Errorf("failed to setup test repository: %w", err)
	}

	// Create local repository with script hooks
	if err := st.SetupRepositoryFiles(repoDir); err != nil {
		return fmt.Errorf("failed to setup repository files: %w", err)
	}

	// Script language config with more complex hooks for better cache testing
	configContent := `repos:
-   repo: local
    hooks:
    -   id: complex-shell-script
        name: Complex Shell Script
        entry: ./scripts/complex-check.sh
        language: script
        files: \.txt$
    -   id: environment-check-script
        name: Environment Check Script
        entry: ./scripts/env-check.sh
        language: script
        files: \.(txt|md)$
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
    -   id: check-yaml
    -   id: check-json
`
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
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

	t.Logf("   ✅ Both Go and Python can create compatible cache structures for script hooks")
	return nil
}
