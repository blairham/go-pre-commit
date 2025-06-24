// Package languages provides Rust-specific integration test implementations.
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

// RustLanguageTest implements LanguageTestRunner for Rust
type RustLanguageTest struct {
	*BaseLanguageTest
	testVersions []string // Store the configured test versions
}

// NewRustLanguageTest creates a new Rust language test
func NewRustLanguageTest(testDir string) *RustLanguageTest {
	return &RustLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangRust, testDir),
		testVersions:     []string{"default"}, // Default to only testing default version
	}
}

// GetLanguageName returns the name of the language being tested
func (rt *RustLanguageTest) GetLanguageName() string {
	return LangRust
}

// SetTestVersions sets the versions to test (called from test configuration)
func (rt *RustLanguageTest) SetTestVersions(versions []string) {
	rt.testVersions = versions
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
	registry := languages.NewLanguageRegistry()
	langImpl, exists := registry.GetLanguage(LangRust)
	if !exists {
		return nil, fmt.Errorf("language %s not found in registry", LangRust)
	}

	lang, ok := langImpl.(language.Manager)
	if !ok {
		return nil, fmt.Errorf("language %s does not implement LanguageManager interface", LangRust)
	}

	return lang, nil
}

// GetAdditionalValidations returns Rust-specific validation steps
func (rt *RustLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "rust-executable-check",
			Description: "Rust executable validation",
			Execute: func(t *testing.T, _, _ string, lang language.Manager) error {
				// Check if we can get the language manager
				if lang.GetName() != "rust" {
					return fmt.Errorf("expected rust language, got %s", lang.GetName())
				}

				// Check if Rust runtime is available
				if !lang.IsRuntimeAvailable() {
					t.Logf("      ⚠️  Warning: Rust runtime not available on system - using estimated metrics")
					return nil
				}

				// Check if cargo is available
				if _, err := exec.LookPath("cargo"); err != nil {
					t.Logf("      ⚠️  Warning: cargo not found in PATH - Rust hooks may not work")
					return err
				}

				t.Logf("      ✅ Rust and cargo are available")
				return nil
			},
		},
		{
			Name:        "version-specific-testing",
			Description: "Rust version-specific testing",
			Execute: func(t *testing.T, _ /* envPath */, version string, lang language.Manager) error {
				return rt.testSpecificVersions(t, lang, version)
			},
		},
	}
}

// testSpecificVersions tests Rust version-specific functionality
func (rt *RustLanguageTest) testSpecificVersions(t *testing.T, lang language.Manager, currentVersion string) error {
	t.Helper()
	t.Logf("      Testing Rust version-specific functionality for version: %s", currentVersion)

	// Use configured test versions instead of hardcoded ones
	for _, version := range rt.testVersions {
		if version == currentVersion {
			continue // Skip testing the current version again
		}

		t.Logf("        Testing version: %s", version)

		// Create temporary test environment for this version
		tempRepo, err := rt.CreateMockRepository(t, version, rt)
		if err != nil {
			t.Logf("        ⚠️  Warning: Could not create test repository for version %s: %v", version, err)
			continue
		}

		// Create proper Rust environment
		envPath, err := lang.SetupEnvironmentWithRepo(rt.cacheDir, version, tempRepo, "", nil)
		if err != nil {
			t.Logf("        ⚠️  Warning: Could not setup Rust environment for version %s: %v", version, err)
			if removeErr := os.RemoveAll(tempRepo); removeErr != nil {
				t.Logf("⚠️  Warning: failed to remove temp environment: %v", removeErr)
			}
			continue
		}

		// Test version detection
		if err := rt.testVersionDetection(t, envPath, version); err != nil {
			t.Logf("        ⚠️  Warning: Version %s detection failed: %v", version, err)
		} else {
			t.Logf("        ✅ Version %s testing completed", version)
		}

		// Clean up immediately
		if removeErr := os.RemoveAll(tempRepo); removeErr != nil {
			t.Logf("⚠️  Warning: failed to remove temp environment: %v", removeErr)
		}
	}

	return nil
}

// testVersionDetection tests Rust version detection
func (rt *RustLanguageTest) testVersionDetection(t *testing.T, envPath, _ string) error {
	t.Helper()

	// For Rust, we can check the cargo version
	cargoExe := filepath.Join(envPath, "bin", "cargo")
	if _, err := os.Stat(cargoExe); os.IsNotExist(err) {
		t.Logf("        Cargo executable not found in environment, skipping version detection")
		return nil
	}

	cmd := exec.Command(cargoExe, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get cargo version: %w", err)
	}

	t.Logf("        Cargo version: %s", string(output))
	return nil
}
