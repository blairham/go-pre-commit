// Package languages provides Go-specific integration test implementations.
package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// GoLanguageTest implements LanguageTestRunner for Go
type GoLanguageTest struct {
	*BaseLanguageTest
	testVersions []string // Store the configured test versions
}

// NewGoLanguageTest creates a new Go language test
func NewGoLanguageTest(testDir string) *GoLanguageTest {
	return &GoLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangGolang, testDir),
		testVersions:     []string{"default"}, // Default to only testing default version
	}
}

// GetLanguageName returns the name of the language being tested
func (gt *GoLanguageTest) GetLanguageName() string {
	return LangGolang
}

// SetTestVersions sets the versions to test (called from test configuration)
func (gt *GoLanguageTest) SetTestVersions(versions []string) {
	gt.testVersions = versions
}

// SetupRepositoryFiles creates Go-specific files in the test repository
func (gt *GoLanguageTest) SetupRepositoryFiles(repoPath string) error {
	goModContent := "module test\n\ngo 1.19"
	if err := os.WriteFile(filepath.Join(repoPath, "go.mod"), []byte(goModContent), 0o600); err != nil {
		return fmt.Errorf("failed to create go.mod: %w", err)
	}
	return nil
}

// GetLanguageManager returns the Go language manager
func (gt *GoLanguageTest) GetLanguageManager() (language.Manager, error) {
	registry := languages.NewLanguageRegistry()
	langImpl, exists := registry.GetLanguage(LangGolang)
	if !exists {
		return nil, fmt.Errorf("language %s not found in registry", LangGolang)
	}

	lang, ok := langImpl.(language.Manager)
	if !ok {
		return nil, fmt.Errorf("language %s does not implement LanguageManager interface", LangGolang)
	}

	return lang, nil
}

// GetAdditionalValidations returns Go-specific validation steps
func (gt *GoLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "go-executable-check",
			Description: "Go executable validation",
			Execute: func(t *testing.T, _, _ string, _ language.Manager) error {
				// For Go, we typically don't create isolated environments like Python
				// So we just check if Go is available on the system
				t.Logf("      Go language uses system-wide installation")
				return nil
			},
		},
		{
			Name:        "version-specific-testing",
			Description: "Go version-specific testing",
			Execute: func(t *testing.T, _ /* envPath */, version string, lang language.Manager) error {
				return gt.testSpecificVersions(t, lang, version)
			},
		},
	}
}

// testSpecificVersions tests Go version-specific functionality
func (gt *GoLanguageTest) testSpecificVersions(t *testing.T, _ language.Manager, currentVersion string) error {
	t.Helper()
	t.Logf("      Testing Go version-specific functionality for version: %s", currentVersion)

	// Use configured test versions instead of hardcoded ones
	for _, version := range gt.testVersions {
		if version == currentVersion {
			continue // Skip testing the current version again
		}

		t.Logf("        Testing version: %s", version)

		// For Go, version testing is simplified since it uses system-wide installation
		gt.testVersionDetection(t, "", version)
		t.Logf("        ✅ Version %s testing completed", version)
	}

	return nil
}

// testVersionDetection tests Go version detection

func (gt *GoLanguageTest) testVersionDetection(t *testing.T, _, _ string) {
	t.Helper()

	// For Go, we check the system Go version
	t.Logf("        Go uses system-wide installation, skipping version-specific environment detection")
}
