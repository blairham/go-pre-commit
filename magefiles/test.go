//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/aserto-dev/mage-loot/deps"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Test namespace methods
// Note: Test type is defined in main.go

// cleanCacheBeforeTest is deprecated since tests now use isolated cache directories
// This prevents interfering with the user's actual pre-commit cache
func cleanCacheBeforeTest() error {
	// Only clean test output directory, not the pre-commit cache
	if err := os.RemoveAll("test-output"); err != nil {
		fmt.Printf("Warning: failed to clean test output: %v\n", err)
	}
	return nil
}

// ensureTestBinarySymlink ensures the symlink from tests/bin/pre-commit to the actual binary exists
func ensureTestBinarySymlink() error {
	symlinkPath := "tests/bin/pre-commit"
	targetPath := filepath.Join("..", "bin", "pre-commit")

	// Check if symlink already exists and is valid
	if link, err := os.Readlink(symlinkPath); err == nil {
		if !filepath.IsAbs(link) && link == targetPath {
			// Valid relative symlink exists, check if target exists
			if _, err := os.Stat(filepath.Join("tests", targetPath)); err == nil {
				return nil
			}
		}
		// Invalid or broken symlink exists, remove it
		fmt.Printf("Removing invalid symlink: %s -> %s\n", symlinkPath, link)
		os.Remove(symlinkPath)
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll("tests/bin", 0o755); err != nil {
		return fmt.Errorf("failed to create tests/bin directory: %w", err)
	}

	// Get the current working directory to build absolute path
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create absolute path symlink for reliability
	absTargetPath := filepath.Join(wd, "bin", "pre-commit")
	fmt.Printf("Creating symlink: %s -> %s\n", symlinkPath, absTargetPath)

	if err := os.Symlink(absTargetPath, symlinkPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// Unit runs unit tests using gotestsum with parallel execution
func (Test) Unit() error {
	fmt.Println("Running unit tests with parallel execution...")
	return deps.GoDep(
		"gotestsum",
	)(
		"--format",
		"pkgname",
		"--",
		"-p", "4", // Run up to 4 packages in parallel
		"-parallel", "8", // Run up to 8 tests in parallel within each package
		"./pkg/...",
		"./internal/...",
		"./cmd/...",
	)
}

// UnitFast runs unit tests with -short flag and parallel execution to skip slow integration tests
func (Test) UnitFast() error {
	fmt.Println("Running unit tests (fast mode - skipping slow integration tests)...")
	return deps.GoDep(
		"gotestsum",
	)(
		"--format",
		"pkgname",
		"--",
		"-short",
		"-p", "4", // Run up to 4 packages in parallel
		"-parallel", "8", // Run up to 8 tests in parallel within each package
		"./pkg/...",
		"./internal/...",
		"./cmd/...",
	)
}

// UnitParallel runs unit tests with maximum parallel execution (for CI/powerful machines)
func (Test) UnitParallel() error {
	fmt.Println("Running unit tests with maximum parallel execution...")
	return deps.GoDep(
		"gotestsum",
	)(
		"--format",
		"pkgname",
		"--",
		"-p", "8", // Run up to 8 packages in parallel
		"-parallel", "16", // Run up to 16 tests in parallel within each package
		"./pkg/...",
		"./internal/...",
		"./cmd/...",
	)
}

// UnitSingle runs unit tests with no parallelism (for debugging)
func (Test) UnitSingle() error {
	fmt.Println("Running unit tests sequentially (no parallelism)...")
	return deps.GoDep(
		"gotestsum",
	)(
		"--format",
		"pkgname",
		"--",
		"-p", "1", // Run 1 package at a time
		"-parallel", "1", // Run 1 test at a time
		"./pkg/...",
		"./internal/...",
		"./cmd/...",
	)
}

// Coverage runs tests with coverage and parallel execution
func (Test) Coverage() error {
	fmt.Println("Running tests with coverage...")
	return sh.RunV(
		"go",
		"test",
		"-coverprofile=coverage.out",
		"-p", "4", // Run up to 4 packages in parallel
		"-parallel", "8", // Run up to 8 tests in parallel within each package
		"./pkg/...",
		"./internal/...",
		"./cmd/...",
	)
}

// CoverageHTML generates HTML coverage report
func (Test) CoverageHTML() error {
	mg.Deps(Test.Coverage)
	fmt.Println("Generating HTML coverage report...")
	return sh.RunV("go", "tool", "cover", "-html=coverage.out", "-o", "coverage.html")
}

// Benchmark runs benchmark tests
func (Test) Benchmark() error {
	fmt.Println("Running benchmark tests...")
	return sh.RunV("go", "test", "-bench=.", "./internal/formatter")
}

// Languages runs comprehensive language implementation tests
func (Test) Languages() error {
	mg.Deps(Build.Binary) // Ensure we have a binary to test
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Println("Running comprehensive language implementation tests...")
	return sh.RunV("./scripts/test-language-implementations.sh")
}

// LanguagesSingle runs tests for a specific language implementation
func (Test) LanguagesSingle(language string) error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Printf("Running tests for %s language implementation...\n", language)
	return sh.RunV("./scripts/test-language-implementations.sh", language)
}

// LanguagesReport generates a comprehensive language testing report
func (Test) LanguagesReport() error {
	mg.Deps(Test.Languages)
	fmt.Println("Language testing report generated in test-output/ directory")
	return nil
}

// LanguagesCore runs tests for core programming languages only (Python, Node, Go, Rust, Ruby)
func (Test) LanguagesCore() error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	if err := ensureTestBinarySymlink(); err != nil {
		return fmt.Errorf("failed to ensure test binary symlink: %w", err)
	}
	fmt.Println("Running tests for core programming languages...")
	return sh.RunV("go", "test", "./tests", "-run", "TestCoreLanguages", "-v", "-timeout", "30m")
}

// LanguagesSystem runs tests for system-level languages (system, script, fail, pygrep)
func (Test) LanguagesSystem() error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Println("Running tests for system-level languages...")
	return sh.RunWithV(
		map[string]string{"GO_PRECOMMIT_BINARY": "./bin/pre-commit"},
		"go", "test", "./tests", "-run", "TestSystemLanguages", "-v", "-timeout", "10m",
	)
}

// LanguagesContainer runs tests for container-based languages (docker, docker_image)
func (Test) LanguagesContainer() error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Println("Running tests for container-based languages...")
	return sh.RunWithV(
		map[string]string{"GO_PRECOMMIT_BINARY": "./bin/pre-commit"},
		"go", "test", "./tests", "-run", "TestContainerLanguages", "-v", "-timeout", "15m",
	)
}

// LanguagesMobile runs tests for mobile and modern development languages (dart, swift)
func (Test) LanguagesMobile() error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Println("Running tests for mobile and modern development languages...")
	return sh.RunWithV(
		map[string]string{"GO_PRECOMMIT_BINARY": "./bin/pre-commit"},
		"go", "test", "./tests", "-run", "TestMobileLanguages", "-v", "-timeout", "15m",
	)
}

// LanguagesScripting runs tests for scripting and data analysis languages (lua, perl, r)
func (Test) LanguagesScripting() error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Println("Running tests for scripting and data analysis languages...")
	return sh.RunWithV(
		map[string]string{"GO_PRECOMMIT_BINARY": "./bin/pre-commit"},
		"go", "test", "./tests", "-run", "TestScriptingLanguages", "-v", "-timeout", "20m",
	)
}

// LanguagesAcademic runs tests for functional and academic programming languages (haskell, julia)
func (Test) LanguagesAcademic() error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Println("Running tests for functional and academic programming languages...")
	return sh.RunWithV(
		map[string]string{"GO_PRECOMMIT_BINARY": "./bin/pre-commit"},
		"go", "test", "./tests", "-run", "TestAcademicLanguages", "-v", "-timeout", "25m",
	)
}

// LanguagesEnterprise runs tests for enterprise and JVM languages (dotnet, coursier)
func (Test) LanguagesEnterprise() error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Println("Running tests for enterprise and JVM languages...")
	return sh.RunWithV(
		map[string]string{"GO_PRECOMMIT_BINARY": "./bin/pre-commit"},
		"go", "test", "./tests", "-run", "TestEnterpriseLanguages", "-v", "-timeout", "20m",
	)
}

// LanguagesByCategory runs tests for all languages grouped by category
func (Test) LanguagesByCategory() error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Println("Running tests for all languages grouped by category...")
	return sh.RunWithV(
		map[string]string{"GO_PRECOMMIT_BINARY": "./bin/pre-commit"},
		"go", "test", "./tests", "-run", "TestLanguagesByCategory", "-v", "-timeout", "60m",
	)
}

// LanguagesSingleGo runs integration tests for a specific language using Go tests
func (Test) LanguagesSingleGo(language string) error {
	mg.Deps(Build.Binary)
	if err := cleanCacheBeforeTest(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}
	fmt.Printf("Running Go integration tests for %s language...\n", language)
	return sh.RunWithV(
		map[string]string{
			"GO_PRECOMMIT_BINARY": "./bin/pre-commit",
			"TEST_LANGUAGE":       language,
		},
		"go", "test", "./tests", "-run", "TestSingleLanguage", "-v", "-timeout", "15m",
	)
}

// LanguagesList shows all configured languages and their properties
func (Test) LanguagesList() error {
	fmt.Println("Listing all configured languages...")
	return sh.RunV("go", "test", "./tests", "-run", "TestListAllLanguages", "-v")
}

// GetCPUCores returns the number of available CPU cores
func GetCPUCores() int {
	return runtime.NumCPU()
}

// PrintCPUCores prints the number of available CPU cores
func PrintCPUCores() {
	numCores := GetCPUCores()
	fmt.Printf("Number of available CPU cores: %d\n", numCores)
}

// ParallelismConfig holds the parallelism configuration for tests
var ParallelismConfig = struct {
	Packages int // Number of packages to test in parallel
	Tests    int // Number of tests to run in parallel within each package
}{
	Packages: 4, // Default: Run up to 4 packages in parallel
	Tests:    8, // Default: Run up to 8 tests in parallel within each package
}

// init function to set parallelism based on CPU cores
func init() {
	numCores := GetCPUCores()
	if numCores > 4 {
		ParallelismConfig.Packages = numCores / 2 // Use half of the CPU cores for package parallelism
		ParallelismConfig.Tests = numCores * 2    // Use double the CPU cores for test parallelism
	}
	fmt.Printf(
		"Parallelism configured: %d packages, %d tests per package\n",
		ParallelismConfig.Packages,
		ParallelismConfig.Tests,
	)
}

// UnitAuto automatically adjusts parallelism based on available CPU cores
func (Test) UnitAuto() error {
	cpuCount := runtime.NumCPU()
	// Use reasonable defaults: half CPU count for packages, full CPU count for tests
	packageParallel := cpuCount / 2
	if packageParallel < 1 {
		packageParallel = 1
	}
	testParallel := cpuCount

	fmt.Printf("Running unit tests with auto-detected parallelism (CPUs: %d, packages: %d, tests: %d)...\n",
		cpuCount, packageParallel, testParallel)

	return deps.GoDep(
		"gotestsum",
	)(
		"--format",
		"pkgname",
		"--",
		"-p", strconv.Itoa(packageParallel),
		"-parallel", strconv.Itoa(testParallel),
		"./pkg/...",
		"./internal/...",
		"./cmd/...",
	)
}
