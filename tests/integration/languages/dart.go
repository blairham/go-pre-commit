package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// DartLanguageTest implements LanguageTestRunner for Dart
type DartLanguageTest struct {
	*BaseLanguageTest
}

// NewDartLanguageTest creates a new Dart language test
func NewDartLanguageTest(testDir string) *DartLanguageTest {
	return &DartLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangDart, testDir),
	}
}

// GetLanguageName returns the language name
func (dt *DartLanguageTest) GetLanguageName() string {
	return LangDart
}

// SetupRepositoryFiles creates Dart-specific repository files
func (dt *DartLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: dart-format
    name: Dart Format
    description: Format Dart code
    entry: dart format
    language: dart
    files: \.dart$
    args: ['--set-exit-if-changed']
-   id: dart-analyze
    name: Dart Analyze
    description: Analyze Dart code
    entry: dart analyze
    language: dart
    files: \.dart$
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create pubspec.yaml
	pubspecFile := filepath.Join(repoPath, "pubspec.yaml")
	pubspecContent := `name: test_dart_hooks
description: Test Dart hooks for pre-commit
version: 1.0.0

environment:
  sdk: '>=2.17.0 <4.0.0'

dev_dependencies:
  lints: ^2.0.0
`
	if err := os.WriteFile(pubspecFile, []byte(pubspecContent), 0o600); err != nil {
		return fmt.Errorf("failed to create pubspec.yaml: %w", err)
	}

	// Create lib directory and main.dart
	libDir := filepath.Join(repoPath, "lib")
	if err := os.MkdirAll(libDir, 0o750); err != nil {
		return fmt.Errorf("failed to create lib directory: %w", err)
	}

	mainFile := filepath.Join(libDir, "main.dart")
	mainContent := `void main() {
  print('Hello, Dart!');
}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0o600); err != nil {
		return fmt.Errorf("failed to create main.dart: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Dart language manager
func (dt *DartLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewDartLanguage(), nil
}

// GetAdditionalValidations returns Dart-specific validation tests
func (dt *DartLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "dart-sdk-check",
			Description: "Dart SDK validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "dart" {
					return fmt.Errorf("expected dart language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}
