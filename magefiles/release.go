//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/aserto-dev/mage-loot/deps"
	"github.com/magefile/mage/mg"
)

// Release namespace methods
// Note: Release type is defined in main.go

// Build creates a snapshot build using GoReleaser (no release)
func (Release) Build() error {
	fmt.Println("Building snapshot with GoReleaser...")
	goreleaser := deps.BinDep("goreleaser")
	if err := goreleaser("build", "--snapshot", "--clean"); err != nil {
		return err
	}

	// Create symlink to the current platform binary
	fmt.Println("Creating symlink for current platform...")
	return Build{}.Symlink()
}

// Snapshot creates a snapshot release using GoReleaser (no Git tag required)
func (Release) Snapshot() error {
	fmt.Println("Creating snapshot release with GoReleaser...")
	goreleaser := deps.BinDep("goreleaser")
	if err := goreleaser("release", "--snapshot", "--clean"); err != nil {
		return err
	}

	// Create symlink to the current platform binary
	fmt.Println("Creating symlink for current platform...")
	return Build{}.Symlink()
}

// All builds release binaries for all platforms using GoReleaser
func (Release) All() error {
	fmt.Println("Creating release with GoReleaser...")
	goreleaser := deps.BinDep("goreleaser")
	return goreleaser("release", "--clean")
}

// Check validates the GoReleaser configuration
func (Release) Check() error {
	fmt.Println("Checking GoReleaser configuration...")
	goreleaser := deps.BinDep("goreleaser")
	return goreleaser("check")
}

// Archive creates compressed archives of release binaries using GoReleaser
func (Release) Archive() error {
	mg.Deps(Release.Snapshot)
	fmt.Println("Archives created by GoReleaser snapshot...")
	return nil
}
