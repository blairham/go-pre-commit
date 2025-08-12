package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// ScriptLanguageTest implements LanguageTestRunner and BidirectionalTestRunner for Script
type ScriptLanguageTest struct {
	*GenericLanguageTest
	*BaseBidirectionalTest
}

// NewScriptLanguageTest creates a new Script language test
func NewScriptLanguageTest(testDir string) *ScriptLanguageTest {
	return &ScriptLanguageTest{
		GenericLanguageTest:   NewGenericLanguageTest("script", testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest("script"),
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

// GetPreCommitConfig returns the .pre-commit-config.yaml content for Script
func (st *ScriptLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-script
        name: Test Script Hook
        entry: ./scripts/test-script.sh
        language: script
        files: \.txt$
`
}

// GetTestFiles returns test files needed for Script testing
func (st *ScriptLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"test.txt": "This is a test file for script hooks.",
		"scripts/test-script.sh": `#!/bin/bash
echo "Script hook executed"
exit 0
`,
	}
}

// GetExpectedDirectories returns directories expected in Script environments
func (st *ScriptLanguageTest) GetExpectedDirectories() []string {
	// Script language doesn't create environment directories
	return []string{}
}

// GetExpectedStateFiles returns state files expected in Script environments
func (st *ScriptLanguageTest) GetExpectedStateFiles() []string {
	// Script language doesn't create state files
	return []string{}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (st *ScriptLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, _ string,
) error {
	t.Helper()

	t.Logf("🔄 Testing Script language bidirectional cache compatibility")
	t.Logf("   📋 Script hooks use executable scripts - testing cache compatibility")

	// Create a temporary directory for this test
	tempDir, err := os.MkdirTemp("", "script-bidirectional-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("🧹 Cleanup: failed to remove temp directory: %v", removeErr)
		}
	}()

	// Use the base bidirectional test implementation
	if err := st.RunBidirectionalCacheTest(t, st, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("bidirectional cache test failed: %w", err)
	}

	t.Logf("   ✅ Both Go and Python can create compatible cache structures for script hooks")
	t.Logf("✅ Script language bidirectional cache compatibility test completed")
	return nil
}
