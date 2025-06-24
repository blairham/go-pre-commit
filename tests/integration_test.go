package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/blairham/go-pre-commit/tests/integration"
)

// TestAllLanguagesCompatibility runs compatibility tests for all supported languages
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestAllLanguagesCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping all languages compatibility tests in short mode")
	}

	pythonBinaryPath := getPythonPreCommitPath()
	goBinaryPath := getGoPreCommitPath()
	testDataDir := filepath.Join(".", "..", "testdata")
	outputDir := filepath.Join(".", "..", "test-output")

	// Debug output
	t.Logf("DEBUG: Python binary path: %s", pythonBinaryPath)
	t.Logf("DEBUG: Go binary path: %s", goBinaryPath)

	suite := integration.NewSuite(pythonBinaryPath, goBinaryPath, testDataDir, outputDir)
	allTests := suite.GetAllLanguageTests()

	t.Logf("Starting comprehensive language compatibility tests for %d languages", len(allTests))

	// Setup deferred result saving to ensure results are saved even if tests fail
	reportGenerator := integration.NewReportGenerator(suite)
	defer func() {
		// Always save results, even if tests fail
		results := suite.GetResults()
		resultsCount := len(results)
		t.Logf("DEBUG: Saving %d test results (deferred save)", resultsCount)

		if resultsCount > 0 {
			if err := reportGenerator.SaveResults(); err != nil {
				t.Logf("⚠️  Warning: Failed to save test results in deferred save: %v", err)
			} else {
				t.Logf("✅ Successfully saved %d test results", resultsCount)
			}

			if err := reportGenerator.GenerateReport(); err != nil {
				t.Logf("⚠️  Warning: Failed to generate test report in deferred save: %v", err)
			}
		}
	}()

	// Run tests sequentially to ensure results are captured properly
	for _, test := range allTests {
		// capture loop variable
		t.Run(test.Language, func(t *testing.T) {
			// Remove t.Parallel() to ensure results are captured before saving
			executor := integration.NewTestExecutor(suite)
			result := executor.RunLanguageCompatibilityTest(t, test)
			suite.AddResult(result)
		})
	}

	// Save results and generate report
	if err := reportGenerator.SaveResults(); err != nil {
		t.Fatalf("Failed to save test results: %v", err)
	}

	if err := reportGenerator.GenerateReport(); err != nil {
		t.Fatalf("Failed to generate test report: %v", err)
	}

	t.Logf("✅ Language compatibility test suite completed. Results saved to %s", outputDir)
}

// TestCoreLanguages tests only the core programming languages
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestCoreLanguages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping core language tests in short mode")
	}

	coreLanguages := []string{"python", "node", "golang", "rust", "ruby", "conda"}
	runSelectedLanguageTests(t, coreLanguages, "Core Languages")
}

// TestSystemLanguages tests system-level languages
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestSystemLanguages(t *testing.T) {
	systemLanguages := []string{"system", "script", "fail", "pygrep"}
	runSelectedLanguageTests(t, systemLanguages, "System Languages")
}

// TestContainerLanguages tests container-based languages
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestContainerLanguages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container language tests in short mode")
	}

	containerLanguages := []string{"docker", "docker_image"}
	runSelectedLanguageTests(t, containerLanguages, "Container Languages")
}

// TestMobileLanguages tests mobile and modern development languages
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestMobileLanguages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mobile language tests in short mode")
	}

	mobileLanguages := []string{"dart", "swift"}
	runSelectedLanguageTests(t, mobileLanguages, "Mobile Languages")
}

// TestScriptingLanguages tests scripting and data analysis languages
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestScriptingLanguages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping scripting language tests in short mode")
	}

	scriptingLanguages := []string{"lua", "perl", "r"}
	runSelectedLanguageTests(t, scriptingLanguages, "Scripting Languages")
}

// TestAcademicLanguages tests functional and academic programming languages
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestAcademicLanguages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping academic language tests in short mode")
	}

	academicLanguages := []string{"haskell", "julia"}
	runSelectedLanguageTests(t, academicLanguages, "Academic Languages")
}

// TestEnterpriseLanguages tests enterprise and JVM languages
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestEnterpriseLanguages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping enterprise language tests in short mode")
	}

	enterpriseLanguages := []string{"dotnet", "coursier"}
	runSelectedLanguageTests(t, enterpriseLanguages, "Enterprise Languages")
}

// TestLanguagesByCategory runs tests for all languages grouped by category
func TestLanguagesByCategory(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping language category tests in short mode")
	}

	categories := map[string][]string{
		"Core":       {"python", "node", "golang", "rust", "ruby", "conda"},
		"Mobile":     {"dart", "swift"},
		"Scripting":  {"lua", "perl", "r"},
		"Academic":   {"haskell", "julia"},
		"Enterprise": {"dotnet", "coursier"},
		"Container":  {"docker", "docker_image"},
		"System":     {"system", "script", "fail", "pygrep"},
	}

	for category, languages := range categories {
		// capture loop variable
		t.Run(category, func(t *testing.T) {
			t.Parallel()
			runSelectedLanguageTests(t, languages, category+" Languages")
		})
	}
}

// TestSingleLanguage tests a specific language (controlled by environment variable)
//
//nolint:tparallel // Integration tests with shared filesystem state should not run in parallel
func TestSingleLanguage(t *testing.T) {
	language := os.Getenv("TEST_LANGUAGE")
	if language == "" {
		t.Skip("TEST_LANGUAGE environment variable not set")
	}

	runSelectedLanguageTests(t, []string{language}, fmt.Sprintf("Single Language: %s", language))
}

// TestListAllLanguages lists all configured languages and their properties
func TestListAllLanguages(t *testing.T) {
	suite := integration.NewSuite("", "", ".", ".")
	allTests := suite.GetAllLanguageTests()

	t.Logf("📋 All Configured Languages (%d total):", len(allTests))
	separatorLine := "=" + "========================================================================================"
	t.Logf("%s", separatorLine)

	categories := map[string][]integration.LanguageCompatibilityTest{
		"Core Programming Languages": {},
		"Mobile & Modern Languages":  {},
		"Scripting Languages":        {},
		"Functional & Academic":      {},
		"Enterprise & JVM":           {},
		"Container & Environment":    {},
		"System & Utility":           {},
	}

	// Categorize languages
	for _, test := range allTests {
		switch test.Language {
		case "python", "node", "golang", "rust", "ruby", "conda":
			categories["Core Programming Languages"] = append(categories["Core Programming Languages"], test)
		case "dart", "swift":
			categories["Mobile & Modern Languages"] = append(categories["Mobile & Modern Languages"], test)
		case "lua", "perl", "r":
			categories["Scripting Languages"] = append(categories["Scripting Languages"], test)
		case "haskell", "julia":
			categories["Functional & Academic"] = append(categories["Functional & Academic"], test)
		case "dotnet", "coursier":
			categories["Enterprise & JVM"] = append(categories["Enterprise & JVM"], test)
		case "docker", "docker_image":
			categories["Container & Environment"] = append(categories["Container & Environment"], test)
		case "system", "script", "fail", "pygrep":
			categories["System & Utility"] = append(categories["System & Utility"], test)
		default:
			categories["System & Utility"] = append(categories["System & Utility"], test)
		}
	}

	// Display categorized languages
	for category, tests := range categories {
		if len(tests) > 0 {
			t.Logf("\n📂 %s (%d):", category, len(tests))
			for _, test := range tests {
				status := "❌"
				if isRuntimeAvailable(test.Language) {
					status = "✅"
				}
				cacheStatus := "🔄"
				if test.CacheTestEnabled {
					cacheStatus = "💾"
				}
				t.Logf("   %s %s %s - %s", status, cacheStatus, test.Language, test.TestRepository)
			}
		}
	}

	t.Logf("\n📊 Summary:")
	t.Logf("-----------")
	t.Logf("Total Languages: %d", len(allTests))

	available := 0
	for _, test := range allTests {
		if isRuntimeAvailable(test.Language) {
			available++
		}
	}
	t.Logf("Available Runtimes: %d", available)
	t.Logf("Missing Runtimes: %d", len(allTests)-available)

	cacheEnabled := 0
	for _, test := range allTests {
		if test.CacheTestEnabled {
			cacheEnabled++
		}
	}
	t.Logf("Cache Test Enabled: %d", cacheEnabled)
	t.Logf("Bidirectional Cache: %d", len(allTests))
}

// Helper function to run tests for selected languages
func runSelectedLanguageTests(t *testing.T, languageNames []string, category string) {
	t.Helper()

	pythonBinaryPath := getPythonPreCommitPath()
	goBinaryPath := getGoPreCommitPath()
	testDataDir := filepath.Join(".", "..", "testdata")
	outputDir := filepath.Join(".", "..", "test-output")

	// Debug output
	t.Logf("DEBUG: Python binary path: %s", pythonBinaryPath)
	t.Logf("DEBUG: Go binary path: %s", goBinaryPath)

	suite := integration.NewSuite(pythonBinaryPath, goBinaryPath, testDataDir, outputDir)
	allTests := suite.GetAllLanguageTests()

	// Filter tests for selected languages
	var selectedTests []integration.LanguageCompatibilityTest
	for _, test := range allTests {
		if slices.Contains(languageNames, test.Language) {
			selectedTests = append(selectedTests, test)
		}
	}

	if len(selectedTests) == 0 {
		t.Skipf("No tests found for %s languages: %v", category, languageNames)
	}

	t.Logf("Starting %s compatibility tests for %d languages", category, len(selectedTests))

	// Setup deferred result saving to ensure results are saved even if tests fail
	reportGenerator := integration.NewReportGenerator(suite)
	defer func() {
		// Always save results, even if tests fail
		results := suite.GetResults()
		resultsCount := len(results)
		t.Logf("DEBUG: Saving %d test results (deferred save)", resultsCount)

		if resultsCount > 0 {
			if err := reportGenerator.SaveResults(); err != nil {
				t.Logf("⚠️  Warning: Failed to save test results in deferred save: %v", err)
			} else {
				t.Logf("✅ Successfully saved %d test results", resultsCount)
			}
		}
	}()

	// Run tests sequentially to ensure results are captured properly
	for _, test := range selectedTests {
		// capture loop variable
		t.Run(test.Language, func(t *testing.T) {
			// Remove t.Parallel() to ensure results are captured before saving
			executor := integration.NewTestExecutor(suite)
			result := executor.RunLanguageCompatibilityTest(t, test)
			suite.AddResult(result)
		})
	}

	// Debug: Check how many results we have
	results := suite.GetResults()
	resultsCount := len(results)
	t.Logf("DEBUG: Found %d test results before saving", resultsCount)

	// Save results and generate report for this category
	if err := reportGenerator.SaveResults(); err != nil {
		t.Fatalf("Failed to save test results: %v", err)
	}

	categoryOutputDir := filepath.Join(
		outputDir,
		"categories",
		category,
	)
	categorySuite := integration.NewSuite(
		pythonBinaryPath,
		goBinaryPath,
		testDataDir,
		categoryOutputDir,
	)
	categorySuite.SetResults(suite.GetResults())
	categoryReportGenerator := integration.NewReportGenerator(categorySuite)
	if err := categoryReportGenerator.GenerateReport(); err != nil {
		t.Logf("⚠️  Warning: Failed to generate category report: %v", err)
	}

	t.Logf("✅ %s compatibility test suite completed", category)
}

// Helper functions to get binary paths and check runtime availability

// getPythonPreCommitPath returns the path to the Python pre-commit binary
func getPythonPreCommitPath() string {
	// Check environment variable first
	if path := os.Getenv("PYTHON_PRECOMMIT_BINARY"); path != "" {
		return path
	}

	// Try to find pre-commit in PATH first (most reliable method)
	if path, err := exec.LookPath("pre-commit"); err == nil {
		return path
	}

	// Check common installation locations as fallback
	commonPaths := []string{
		"/usr/local/bin/pre-commit",
		"/usr/bin/pre-commit",
		filepath.Join(os.Getenv("HOME"), ".local/bin/pre-commit"),
		"/opt/homebrew/bin/pre-commit",
		filepath.Join(os.Getenv("HOME"), ".asdf/shims/pre-commit"),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Return empty string if not found (tests will handle gracefully)
	return ""
}

// getGoPreCommitPath returns the path to the Go pre-commit binary
func getGoPreCommitPath() string {
	// Check environment variable first (highest priority)
	if path := os.Getenv("GO_PRECOMMIT_BINARY"); path != "" {
		return path
	}

	// Get the current working directory to build absolute path
	wd, err := os.Getwd()
	if err != nil {
		return "./bin/pre-commit" // fallback
	}

	// Build path relative to project root
	// If we're in any subdirectory, find the project root
	for dir := wd; dir != "/" && dir != "." && dir != ""; {
		binPath := filepath.Join(dir, "bin", "pre-commit")
		if _, err := os.Stat(binPath); err == nil {
			// Always return absolute path
			if absPath, err := filepath.Abs(binPath); err == nil {
				return absPath
			}
			return binPath
		}
		dir = filepath.Dir(dir)
		// Stop if we've reached the root or no change
		if dir == "/" || dir == "." {
			break
		}
	}

	// Try relative paths from current working directory
	candidates := []string{
		"./bin/pre-commit",     // current directory
		"../bin/pre-commit",    // parent directory (if running from tests/)
		"../../bin/pre-commit", // grandparent directory
	}

	for _, candidate := range candidates {
		if absPath, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath
			}
		}
	}

	// Fallback: try relative to current directory
	return "./bin/pre-commit"
}

// isRuntimeAvailable checks if a language runtime is available
func isRuntimeAvailable(language string) bool {
	switch language {
	case "system", "script", "fail", "pygrep":
		return true // These don't require external runtimes
	case "python":
		_, err := exec.LookPath("python3")
		if err != nil {
			_, err = exec.LookPath("python")
		}
		return err == nil
	case "node":
		_, err := exec.LookPath("node")
		return err == nil
	case "golang":
		_, err := exec.LookPath("go")
		return err == nil
	case "rust":
		_, err := exec.LookPath("cargo")
		return err == nil
	case "ruby":
		_, err := exec.LookPath("ruby")
		return err == nil
	case "conda":
		_, err := exec.LookPath("conda")
		return err == nil
	case "dart":
		_, err := exec.LookPath("dart")
		return err == nil
	case "swift":
		_, err := exec.LookPath("swift")
		return err == nil
	case "lua":
		_, err := exec.LookPath("lua")
		return err == nil
	case "perl":
		_, err := exec.LookPath("perl")
		return err == nil
	case "r":
		_, err := exec.LookPath("Rscript")
		return err == nil
	case "haskell":
		_, err := exec.LookPath("ghc")
		return err == nil
	case "julia":
		_, err := exec.LookPath("julia")
		return err == nil
	case "dotnet":
		_, err := exec.LookPath("dotnet")
		return err == nil
	case "coursier":
		_, err := exec.LookPath("cs")
		if err != nil {
			_, err = exec.LookPath("coursier")
		}
		return err == nil
	case "docker", "docker_image":
		_, err := exec.LookPath("docker")
		return err == nil
	default:
		return false
	}
}
