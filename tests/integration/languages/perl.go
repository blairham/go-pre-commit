package languages

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/language"
	"github.com/blairham/go-pre-commit/pkg/repository/languages"
)

// PerlLanguageTest implements LanguageTestRunner for Perl
type PerlLanguageTest struct {
	*BaseLanguageTest
}

// NewPerlLanguageTest creates a new Perl language test
func NewPerlLanguageTest(testDir string) *PerlLanguageTest {
	return &PerlLanguageTest{
		BaseLanguageTest: NewBaseLanguageTest(LangPerl, testDir),
	}
}

// GetLanguageName returns the language name
func (pt *PerlLanguageTest) GetLanguageName() string {
	return LangPerl
}

// SetupRepositoryFiles creates Perl-specific repository files
func (pt *PerlLanguageTest) SetupRepositoryFiles(repoPath string) error {
	// Create .pre-commit-hooks.yaml
	hooksFile := filepath.Join(repoPath, ".pre-commit-hooks.yaml")
	hooksContent := `-   id: perl-critic
    name: Perl Critic
    description: Lint Perl code with Perl::Critic
    entry: perlcritic
    language: perl
    files: \.pl$
-   id: perl-tidy
    name: Perl Tidy
    description: Format Perl code with Perl::Tidy
    entry: perltidy
    language: perl
    files: \.pl$
    args: ['-b']
`
	if err := os.WriteFile(hooksFile, []byte(hooksContent), 0o600); err != nil {
		return fmt.Errorf("failed to create hooks file: %w", err)
	}

	// Create a simple Perl script
	perlFile := filepath.Join(repoPath, "test.pl")
	perlContent := `#!/usr/bin/perl
use strict;
use warnings;

print "Hello, Perl!\n";
`
	//nolint:gosec // Script files need executable permissions
	//nolint:gosec // Script files need executable permissions
	//nolint:gosec // Script files need executable permissions
	if err := os.WriteFile(perlFile, []byte(perlContent), 0o755); err != nil {
		return fmt.Errorf("failed to create test.pl: %w", err)
	}

	return nil
}

// GetLanguageManager returns the Perl language manager
func (pt *PerlLanguageTest) GetLanguageManager() (language.Manager, error) {
	return languages.NewPerlLanguage(), nil
}

// GetAdditionalValidations returns Perl-specific validation tests
func (pt *PerlLanguageTest) GetAdditionalValidations() []ValidationStep {
	return []ValidationStep{
		{
			Name:        "perl-version-check",
			Description: "Perl version validation",
			Execute: func(_ *testing.T, _, _ string, lang language.Manager) error {
				if lang.GetName() != "perl" {
					return fmt.Errorf("expected perl language, got %s", lang.GetName())
				}
				return nil
			},
		},
	}
}
