//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/aserto-dev/mage-loot/common"
	"github.com/aserto-dev/mage-loot/deps"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Quality namespace methods
// Note: Quality and Test types are defined in main.go

// Lint runs golangci-lint using mage-loot
func (Quality) Lint() error {
	fmt.Println("Running linter...")
	return common.Lint()
}

// Format formats the code using gci, golines, gofumpt, and fieldalignment
func (Quality) Format() error {
	fmt.Println("Formatting imports with gci...")
	if err := deps.GoDep("gci")("write", "--skip-generated", "-s", "standard", "-s", "default", "-s", "prefix(github.com/blairham/go-pre-commit)", "."); err != nil {
		return fmt.Errorf("gci failed: %w", err)
	}

	fmt.Println("Formatting line length with golines...")
	if err := deps.GoDep("golines")("-w", "-m", "120", "."); err != nil {
		return fmt.Errorf("golines failed: %w", err)
	}

	fmt.Println("Formatting code with gofumpt...")
	if err := deps.GoDep("gofumpt")("-l", "-w", "."); err != nil {
		return fmt.Errorf("gofumpt failed: %w", err)
	}

	fmt.Println("Fixing struct field alignment...")
	if err := sh.Run("go", "run", "golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest", "-fix", "./..."); err != nil {
		return fmt.Errorf("fieldalignment failed: %w", err)
	}

	return nil
}

// Vet runs go vet
func (Quality) Vet() error {
	fmt.Println("Running go vet...")
	return sh.Run("go", "vet", "./...")
}

// Modernize runs the Go modernize tool to update code to modern Go patterns
func (Quality) Modernize() error {
	fmt.Println("Running Go modernize tool...")
	return sh.Run(
		"go",
		"run",
		"golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest",
		"-fix",
		"-test",
		"./...",
	)
}

// All runs all quality checks
func (Quality) All() {
	mg.Deps(Quality.Format, Quality.Vet, Quality.Lint, Quality.Modernize, Test.Unit)
}
