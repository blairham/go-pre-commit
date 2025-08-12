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
	*BaseBidirectionalTest
}

// NewPerlLanguageTest creates a new Perl language test
func NewPerlLanguageTest(testDir string) *PerlLanguageTest {
	return &PerlLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangPerl, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(testDir),
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
	hooksContent := `-   id: perl-syntax-check
    name: Perl Syntax Check
    description: Check Perl syntax
    entry: perl
    language: perl
    files: \.pl$
    args: ['-c']
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

// GetPreCommitConfig returns the .pre-commit-config.yaml content for Perl testing
func (pt *PerlLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-perl
        name: Test Perl Hook
        entry: echo "Testing Perl"
        language: perl
        files: \.pl$
`
}

// GetTestFiles returns test files needed for Perl testing
func (pt *PerlLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"main.pl": `#!/usr/bin/perl
use strict;
use warnings;

print "Hello from Perl!\n";

sub greet {
    my $name = shift;
    print "Hello, $name!\n";
}

greet("World");
`,
		"test.pl": `#!/usr/bin/perl
use strict;
use warnings;

require "main.pl";

print "Test completed\n";
`,
	}
}

// GetExpectedDirectories returns the directories expected in Perl environments
func (pt *PerlLanguageTest) GetExpectedDirectories() []string {
	return []string{
		"lib",   // Perl library directory
		"blib",  // Perl build library
		"local", // Local Perl modules
		"perl5", // Perl5 modules
	}
}

// GetExpectedStateFiles returns state files expected in Perl environments
func (pt *PerlLanguageTest) GetExpectedStateFiles() []string {
	return []string{
		"Makefile.PL", // Perl Makefile
		"Build.PL",    // Module::Build script
		"META.yml",    // CPAN metadata
		"cpanfile",    // Perl dependencies
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (pt *PerlLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, tempDir string,
) error {
	t.Helper()
	t.Logf("🔄 Testing Perl bidirectional cache compatibility")
	t.Logf("   📋 Perl environments manage modules and libraries - testing cache compatibility")

	// Use the base bidirectional test framework
	if err := pt.BaseBidirectionalTest.RunBidirectionalCacheTest(t, pt, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("perl bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ Perl bidirectional cache compatibility test completed")
	return nil
}
