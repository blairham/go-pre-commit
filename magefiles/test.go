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
		fmt.Printf("⚠️  Warning: failed to clean test output: %v\n", err)
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
