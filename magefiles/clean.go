//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/sh"
)

// Clean namespace methods
// Note: Clean type is defined in main.go

// All removes all build artifacts
func (Clean) All() error {
	fmt.Println("Cleaning all build artifacts...")
	return os.RemoveAll("bin")
}

// Coverage removes coverage files
func (Clean) Coverage() error {
	fmt.Println("Cleaning coverage files...")
	os.Remove("coverage.out")
	os.Remove("coverage.html")
	return nil
}

// Deps removes dependency cache
func (Clean) Deps() error {
	fmt.Println("Cleaning dependency cache...")
	return sh.Run("go", "clean", "-modcache")
}

// Cache removes pre-commit cache and environments
func (Clean) Cache() error {
	fmt.Println("Cleaning pre-commit cache...")
	if _, err := os.Stat("bin/pre-commit"); err != nil {
		fmt.Println("Binary not found, skipping cache clean")
		return nil
	}
	return sh.RunV("./bin/pre-commit", "clean")
}

// TestOutput removes test output directory
func (Clean) TestOutput() error {
	fmt.Println("Cleaning test output directory...")
	return os.RemoveAll("test-output")
}
