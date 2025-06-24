package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// RustLanguageTest implements LanguageTestRunner for Rust
type RustLanguageTest struct {
	*BaseLanguageTest
}

// NewRustLanguageTest creates a new Rust language test
func NewRustLanguageTest(testDir string) *RustLanguageTest {
	return &RustLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangRust, testDir),
	}
}

// GetLanguageName returns the language name
func (rt *RustLanguageTest) GetLanguageName() string {
	return LangRust
}

// SetupRepositoryFiles creates Rust-specific repository files
func (rt *RustLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: fmt
    name: Rust fmt
    description: Format Rust code
    entry: cargo fmt
    language: rust
    files: \.rs$
    args: ['--', '--check']
-   id: clippy
    name: Rust clippy
    description: Lint Rust code
    entry: cargo clippy
    language: rust
    files: \.rs$
    args: ['--', '--deny', 'warnings']
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create Cargo.toml
	cargoFile := filepath.Join(repoPath, "Cargo.toml")
	cargoContent := `[package]
name = "test-rust-hooks"
version = "0.1.0"
edition = "2021"

[dependencies]
`
	if err := os.WriteFile(cargoFile, []byte(cargoContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Cargo.toml: %w", err)
	}

	// Create src directory and main.rs
	srcDir := filepath.Join(repoPath, "src")
	if err := os.MkdirAll(srcDir, 0o750); err != nil {
		return fmt.Errorf("failed to create src directory: %w", err)
	}

	mainFile := filepath.Join(srcDir, "main.rs")
	mainContent := `fn main() {
    println!("Hello, Rust!");
}
`
	if err := os.WriteFile(mainFile, []byte(mainContent), 0o600); err != nil {
		return fmt.Errorf("failed to create main.rs: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Rust language manager
func (rt *RustLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewRustLanguage(), nil
}

// GetAdditionalValidations returns Rust-specific validation tests
func (rt *RustLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "cargo-check",
			Description: "Cargo binary validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				// Basic validation - check if we can get the language manager
				if lang.GetName() != "rust" {
					return fmt.Errorf("expected rust language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}
