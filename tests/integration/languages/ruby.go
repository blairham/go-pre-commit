// Package languages provides Ruby-specific integration test implementations.
package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// RubyLanguageTest implements LanguageTestRunner for Ruby
type RubyLanguageTest struct {
	*BaseLanguageTest
}

// NewRubyLanguageTest creates a new Ruby language test
func NewRubyLanguageTest(testDir string) *RubyLanguageTest {
	return &RubyLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangRuby, testDir),
	}
}

// SetupRepositoryFiles creates Ruby-specific files in the test repository
func (rt *RubyLanguageTest) SetupRepositoryFiles(repoPath string) error {
	gemfileContent := "source 'https://rubygems.org'\ngem 'rake'"
	if err := os.WriteFile(filepath.Join(repoPath, "Gemfile"), []byte(gemfileContent), 0o600); err != nil {
		return fmt.Errorf("failed to create Gemfile: %w", err)
	}
	return nil
}

// GetLanguageManager returns the Ruby language manager
func (rt *RubyLanguageTest) GetLanguageManager() (language.Manager, error) {
	registry := languages.NewLanguageRegistry()
	langImpl, exists := registry.GetLanguage(LangRuby)
	if !exists {
		return nil, fmt.Errorf("language %s not found in registry", LangRuby)
	}

	lang, ok := langImpl.(language.Manager)
	if !ok {
		return nil, fmt.Errorf("language %s does not implement LanguageManager interface", LangRuby)
	}

	return lang, nil
}

// GetAdditionalValidations returns Ruby-specific validation steps
func (rt *RubyLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "ruby-executable-check",
			Description: "Ruby executable validation",
			Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if Ruby executable exists in the environment
				rubyExe := filepath.Join(envPath, "bin", "ruby")
				if _, err := os.Stat(rubyExe); os.IsNotExist(err) {
					return fmt.Errorf("ruby executable not found in environment")
				}
				t.Logf("      Found Ruby executable: %s", rubyExe)
				return nil
			},
		},
		{
			Name:        "gem-check",
			Description: "Gem installation validation",
			Execute: func(t *testing.T, envPath, _ string, _ language.Manager) error {
				// Check if gem exists in the environment
				gemExe := filepath.Join(envPath, "bin", "gem")
				if _, err := os.Stat(gemExe); os.IsNotExist(err) {
					return fmt.Errorf("gem executable not found in environment")
				}
				t.Logf("      Found gem executable: %s", gemExe)
				return nil
			},
		},
	}
}

// GetLanguageName returns the name of the Ruby language
func (rt *RubyLanguageTest) GetLanguageName() string {
	return LangRuby
}
