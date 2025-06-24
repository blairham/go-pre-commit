package integration

import (
	"fmt"
	"testing"

	"github.com/blairham/go-pre-commit/tests/integration/languages"
)

// createLanguageTestRunner creates a language test runner based on the language type
//
//nolint:gocyclo,cyclop // This switch statement is inherently complex due to the large number of supported languages
func createLanguageTestRunner(language, testDir string) languages.LanguageTestRunner {
	switch language {
	case languages.LangPython:
		return languages.NewPythonLanguageTest(testDir)
	case languages.LangNode:
		return languages.NewNodeLanguageTest(testDir)
	case languages.LangGolang:
		return languages.NewGoLanguageTest(testDir)
	case languages.LangRuby:
		return languages.NewRubyLanguageTest(testDir)
	case languages.LangRust:
		return languages.NewRustLanguageTest(testDir)
	case languages.LangDart:
		return languages.NewDartLanguageTest(testDir)
	case languages.LangSwift:
		return languages.NewSwiftLanguageTest(testDir)
	case languages.LangLua:
		return languages.NewLuaLanguageTest(testDir)
	case languages.LangPerl:
		return languages.NewPerlLanguageTest(testDir)
	case languages.LangR:
		return languages.NewRLanguageTest(testDir)
	case languages.LangHaskell:
		return languages.NewHaskellLanguageTest(testDir)
	case languages.LangJulia:
		return languages.NewJuliaLanguageTest(testDir)
	case languages.LangDotnet:
		return languages.NewDotnetLanguageTest(testDir)
	case languages.LangCoursier:
		return languages.NewCoursierLanguageTest(testDir)
	case languages.LangDocker:
		return languages.NewDockerLanguageTest(testDir)
	case languages.LangDockerImage:
		return languages.NewDockerImageLanguageTest(testDir)
	case languages.LangConda:
		return languages.NewCondaLanguageTest(testDir)
	case languages.LangSystem:
		return languages.NewSystemLanguageTest(testDir)
	case languages.LangScript:
		return languages.NewScriptLanguageTest(testDir)
	case languages.LangFail:
		return languages.NewFailLanguageTest(testDir)
	case languages.LangPygrep:
		return languages.NewPygrepLanguageTest(testDir)
	default:
		return languages.NewGenericLanguageTest(language, testDir)
	}
}

// LanguageTestFactory creates the appropriate language test runner for a given language
func LanguageTestFactory(language, testDir string) (languages.LanguageTestRunner, error) {
	runner := createLanguageTestRunner(language, testDir)
	return runner, nil
}

// testRepositoryAndEnvironmentSetup verifies that repository cloning and environment creation work correctly
// This function now uses the new modular language test architecture
func (te *TestExecutor) testRepositoryAndEnvironmentSetup(
	t *testing.T,
	test LanguageCompatibilityTest,
	testDir string,
	_ *TestResults,
) error {
	t.Helper()

	// Create the appropriate language test runner
	runner, err := LanguageTestFactory(test.Language, testDir)
	if err != nil {
		return fmt.Errorf("failed to create language test runner: %w", err)
	}

	// Convert to languages.LanguageCompatibilityTest for the languages package
	langTest := languages.LanguageCompatibilityTest{
		Language:               test.Language,
		TestRepository:         test.TestRepository,
		TestVersions:           test.TestVersions,
		AdditionalDependencies: test.AdditionalDependencies,
		TestTimeout:            test.TestTimeout,
	}

	// Run the test using the language-specific runner
	baseTest := languages.NewBaseLanguageTest(test.Language, testDir)
	return baseTest.RunRepositoryAndEnvironmentSetup(t, langTest, runner)
}
