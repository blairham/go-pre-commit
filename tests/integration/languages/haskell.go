package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// HaskellLanguageTest implements LanguageTestRunner and BidirectionalTestRunner for Haskell
type HaskellLanguageTest struct {
	*BaseLanguageTest
	*BaseBidirectionalTest
}

// NewHaskellLanguageTest creates a new Haskell language test
func NewHaskellLanguageTest(testDir string) *HaskellLanguageTest {
	return &HaskellLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangHaskell, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(LangHaskell),
	}
}

// GetLanguageName returns the language name
func (ht *HaskellLanguageTest) GetLanguageName() string {
	return LangHaskell
}

// SetupRepositoryFiles creates Haskell-specific repository files
func (ht *HaskellLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: hindent
    name: Hindent
    description: Format Haskell code using hindent
    entry: echo
    language: haskell
    files: \.hs$
    additional_dependencies: ['base']
-   id: hlint
    name: HLint
    description: Lint Haskell code using hlint
    entry: echo
    language: haskell
    files: \.hs$
    additional_dependencies: ['base']
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create .cabal file (for cabal projects - required by Python pre-commit)
	cabalFile := filepath.Join(repoPath, "test-haskell-hooks.cabal")
	cabalContent := `cabal-version: 2.0
name: test-haskell-hooks
version: 0.1.0.0
synopsis: Test Haskell hooks for pre-commit
description: Test Haskell hooks for pre-commit validation
author: Test
maintainer: test@example.com

executable test-haskell-hooks
  main-is: Main.hs
  hs-source-dirs: app
  build-depends: base >= 4.7 && < 5
  default-language: Haskell2010
`
	if err := os.WriteFile(cabalFile, []byte(cabalContent), 0o600); err != nil {
		return fmt.Errorf("failed to create .cabal file: %w", err)
	}

	// Create package.yaml (for stack projects)
	packageFile := filepath.Join(repoPath, "package.yaml")
	packageContent := `name: test-haskell-hooks
version: 0.1.0.0
synopsis: Test Haskell hooks for pre-commit
description: Test Haskell hooks for pre-commit validation

dependencies:
- base >= 4.7 && < 5

executables:
  test-haskell-hooks:
    main: Main.hs
    source-dirs: app
`
	if err := os.WriteFile(packageFile, []byte(packageContent), 0o600); err != nil {
		return fmt.Errorf("failed to create package.yaml: %w", err)
	}

	// Create app directory and Main.hs
	appDir := filepath.Join(repoPath, "app")
	if err := os.MkdirAll(appDir, 0o750); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	mainFile := filepath.Join(appDir, "Main.hs")
	mainContent := `module Main where

main :: IO ()
main = putStrLn "Hello, Haskell!"
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Main.hs: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Haskell language manager
func (ht *HaskellLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewHaskellLanguage(), nil
}

// GetAdditionalValidations returns Haskell-specific validation steps
func (ht *HaskellLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "ghc-version-check",
			Description: "Verify GHC version",
			Execute: func(_ *testing.T, _, _ string, _ language.Manager) error {
				// Simple GHC version check
				return nil // Placeholder - GHC validation would go here
			},
		},
	}
}

// GetPreCommitConfig returns the .pre-commit-config.yaml content for Haskell
func (ht *HaskellLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-haskell
        name: Test Haskell Hook
        entry: echo "Testing Haskell"
        language: haskell
        files: \.hs$
        additional_dependencies: ['base']
`
}

// GetTestFiles returns test files needed for Haskell testing
func (ht *HaskellLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"test.hs": "main = putStrLn \"Hello\"",
	}
}

// GetExpectedDirectories returns directories expected in Haskell environments
func (ht *HaskellLanguageTest) GetExpectedDirectories() []string {
	return []string{"bin", "lib", "include"}
}

// GetExpectedStateFiles returns state files expected in Haskell environments
func (ht *HaskellLanguageTest) GetExpectedStateFiles() []string {
	return []string{".install_state_v1", ".install_state_v2"}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (ht *HaskellLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, _ string,
) error {
	t.Helper()

	t.Logf("🔄 Testing Haskell bidirectional cache compatibility")
	t.Logf("   📋 Haskell environments create structured directories with state files")
	t.Logf("   📋 Testing cache compatibility with new installation method")

	// Create a temporary directory for this test
	tempDir, err := os.MkdirTemp("", "haskell-bidirectional-test-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("🧹 Cleanup: failed to remove temp directory: %v", removeErr)
		}
	}()

	// Use the base bidirectional test implementation
	if err := ht.BaseBidirectionalTest.RunBidirectionalCacheTest(t, ht, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ Haskell bidirectional cache compatibility test completed")
	return nil
}
