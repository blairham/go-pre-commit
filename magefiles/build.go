//go:build mage
// +build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/magefile/mage/sh"
)

// Build namespace methods
// Note: Build type is defined in main.go

// Binary builds the main binary
func (Build) Binary() error {
	fmt.Println("Building go-pre-commit...")
	return sh.Run("go", "build", "-o", "bin/pre-commit", "./cmd/pre-commit")
}

// Install installs the binary to $GOPATH/bin
func (Build) Install() error {
	fmt.Println("Installing go-pre-commit...")
	return sh.Run("go", "install", "./cmd/pre-commit")
}

// Debug builds with debug flags
func (Build) Debug() error {
	fmt.Println("Building go-pre-commit with debug flags...")
	return sh.Run(
		"go",
		"build",
		"-gcflags",
		"all=-N -l",
		"-o",
		"bin/pre-commit-debug",
		"./cmd/pre-commit",
	)
}

// Symlink creates a symlink from the GoReleaser build output to bin/pre-commit
func (Build) Symlink() error {
	currentOS := runtime.GOOS
	currentArch := runtime.GOARCH

	// Map Go arch names to GoReleaser arch names
	arch := currentArch
	if currentArch == "amd64" {
		arch = "x86_64"
	}

	// Construct the path to the GoReleaser build output
	binaryName := "pre-commit"
	if currentOS == "windows" {
		binaryName = "pre-commit.exe"
	}

	sourcePath := filepath.Join(
		"dist",
		fmt.Sprintf("pre-commit_%s_%s", currentOS, arch),
		binaryName,
	)
	targetPath := filepath.Join("bin", "pre-commit")

	if currentOS == "windows" {
		targetPath = filepath.Join("bin", "pre-commit.exe")
	}

	// Create bin directory if it doesn't exist
	if err := os.MkdirAll("bin", 0o755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Remove existing symlink/file if it exists
	if _, err := os.Lstat(targetPath); err == nil {
		if err := os.Remove(targetPath); err != nil {
			return fmt.Errorf("failed to remove existing file: %w", err)
		}
	}

	// Check if source file exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf(
			"GoReleaser build output not found at %s. Run 'mage release:build' first",
			sourcePath,
		)
	}

	// Create symlink (or copy on Windows)
	if currentOS == "windows" {
		// Windows doesn't support symlinks easily, so copy the file
		fmt.Printf("Copying %s to %s...\n", sourcePath, targetPath)
		return sh.Copy(targetPath, sourcePath)
	} else {
		// Create relative symlink
		relPath, err := filepath.Rel(filepath.Dir(targetPath), sourcePath)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}
		fmt.Printf("Creating symlink %s -> %s...\n", targetPath, relPath)
		return os.Symlink(relPath, targetPath)
	}
}
