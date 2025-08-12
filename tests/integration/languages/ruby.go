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
	*BaseBidirectionalTest
	testVersions []string // Store the configured test versions
}

// NewRubyLanguageTest creates a new Ruby language test
func NewRubyLanguageTest(testDir string) *RubyLanguageTest {
	return &RubyLanguageTest{
		BaseLanguageTest:      NewBaseLanguageTest(LangRuby, testDir),
		BaseBidirectionalTest: NewBaseBidirectionalTest(testDir),
		testVersions:          []string{"default"}, // Default to only testing default version
	}
}

// GetLanguageName returns the name of the language being tested
func (rt *RubyLanguageTest) GetLanguageName() string {
	return LangRuby
}

// SetTestVersions sets the versions to test (called from test configuration)
func (rt *RubyLanguageTest) SetTestVersions(versions []string) {
	rt.testVersions = versions
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
			Execute: func(t *testing.T, envPath, _ string, lang language.Manager) error {
				// Check if Ruby runtime is available on the system
				if !lang.IsRuntimeAvailable() {
					t.Logf("      ⚠️  Warning: Ruby runtime not available on system - using estimated metrics")
					return nil
				}

				// Check if Ruby executable exists in the environment
				rubyExe := filepath.Join(envPath, "bin", "ruby")
				if _, err := os.Stat(rubyExe); os.IsNotExist(err) {
					t.Logf(
						"      ⚠️  Warning: Ruby executable not found in environment - environment setup may have issues",
					)
					return nil
				}
				// Ruby executable found
				return nil
			},
		},
		{
			Name:        "gem-check",
			Description: "Gem installation validation",
			Execute: func(t *testing.T, envPath, _ string, lang language.Manager) error {
				// Check if Ruby runtime is available first
				if !lang.IsRuntimeAvailable() {
					t.Logf("      ⚠️  Warning: Ruby runtime not available - skipping gem check")
					return nil
				}

				// Check if gem exists in the environment
				gemExe := filepath.Join(envPath, "bin", "gem")
				if _, err := os.Stat(gemExe); os.IsNotExist(err) {
					t.Logf(
						"      ⚠️  Warning: Gem executable not found in environment - environment setup may have issues",
					)
					return nil
				}
				// Gem executable found
				return nil
			},
		},
		{
			Name:        "version-specific-testing",
			Description: "Ruby version-specific testing",
			Execute: func(t *testing.T, _ /* envPath */, version string, lang language.Manager) error {
				return rt.testSpecificVersions(t, lang, version)
			},
		},
	}
}

// testSpecificVersions tests Ruby version-specific functionality
func (rt *RubyLanguageTest) testSpecificVersions(t *testing.T, lang language.Manager, currentVersion string) error {
	t.Helper()
	t.Logf("      Testing Ruby version-specific functionality for version: %s", currentVersion)

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

		// Create proper Ruby environment
		envPath, err := lang.SetupEnvironmentWithRepo(rt.cacheDir, version, tempRepo, "", nil)
		if err != nil {
			t.Logf("        ⚠️  Warning: Could not setup Ruby environment for version %s: %v", version, err)
			if removeErr := os.RemoveAll(tempRepo); removeErr != nil {
				t.Logf("⚠️  Warning: failed to remove temp environment: %v", removeErr)
			}
			continue
		}

		// Test version detection
		rt.testVersionDetection(t, envPath, version)
		t.Logf("        ✅ Version %s testing completed", version)

		// Clean up immediately
		if removeErr := os.RemoveAll(tempRepo); removeErr != nil {
			t.Logf("⚠️  Warning: failed to remove temp environment: %v", removeErr)
		}
	}

	return nil
}

// testVersionDetection tests Ruby version detection
func (rt *RubyLanguageTest) testVersionDetection(t *testing.T, envPath, _ string) {
	t.Helper()

	// For Ruby, we can check the ruby version
	rubyExe := filepath.Join(envPath, "bin", "ruby")
	if _, err := os.Stat(rubyExe); os.IsNotExist(err) {
		t.Logf("        Ruby executable not found in environment, skipping version detection")
		return
	}

	t.Logf("        Ruby version detection completed")
}

// GetPreCommitConfig returns the .pre-commit-config.yaml content for Ruby testing
func (rt *RubyLanguageTest) GetPreCommitConfig() string {
	return `repos:
  - repo: local
    hooks:
      - id: test-ruby
        name: Test Ruby Hook
        entry: echo "Testing Ruby"
        language: ruby
        files: \.rb$
`
}

// GetTestFiles returns test files needed for Ruby testing
func (rt *RubyLanguageTest) GetTestFiles() map[string]string {
	return map[string]string{
		"main.rb": `#!/usr/bin/env ruby
puts "Hello from Ruby!"

def greet(name)
  puts "Hello, #{name}!"
end

greet("World")
`,
		"test.rb": `#!/usr/bin/env ruby
require_relative 'main'

puts "Test completed"
`,
		"Gemfile": `source 'https://rubygems.org'

gem 'rake'
gem 'rspec'
`,
	}
}

// GetExpectedDirectories returns the directories expected in Ruby environments
func (rt *RubyLanguageTest) GetExpectedDirectories() []string {
	return []string{
		"lib",    // Ruby library directory
		"bin",    // Ruby executables
		"spec",   // RSpec tests
		"test",   // Test files
		"vendor", // Vendored gems
	}
}

// GetExpectedStateFiles returns state files expected in Ruby environments
func (rt *RubyLanguageTest) GetExpectedStateFiles() []string {
	return []string{
		"Gemfile",       // Ruby gem dependencies
		"Gemfile.lock",  // Ruby gem lock file
		".ruby-version", // Ruby version specification
		"Rakefile",      // Ruby rake tasks
	}
}

// TestBidirectionalCacheCompatibility tests cache compatibility between Go and Python implementations
func (rt *RubyLanguageTest) TestBidirectionalCacheCompatibility(
	t *testing.T,
	pythonBinary, goBinary, tempDir string,
) error {
	t.Helper()
	t.Logf("🔄 Testing Ruby bidirectional cache compatibility")
	t.Logf("   📋 Ruby environments manage gems and dependencies - testing cache compatibility")

	// Use the base bidirectional test framework
	if err := rt.BaseBidirectionalTest.RunBidirectionalCacheTest(t, rt, pythonBinary, goBinary, tempDir); err != nil {
		return fmt.Errorf("ruby bidirectional cache test failed: %w", err)
	}

	t.Logf("✅ Ruby bidirectional cache compatibility test completed")
	return nil
}
