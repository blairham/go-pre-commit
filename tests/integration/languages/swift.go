package languages

import (
	"fmt"
	"os"
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
	hooksContent := `-   id: swift-format
    name: Swift Format
    description: Format Swift code
    entry: swift-format
    language: swift
    files: \.swift$
    args: ['--in-place']
-   id: swiftlint
    name: SwiftLint
    description: Lint Swift code
    entry: swiftlint
    language: swift
    files: \.swift$
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
