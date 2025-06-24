//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/aserto-dev/mage-loot/deps"
	"github.com/magefile/mage/sh"
)

// Deps namespace methods
// Note: Deps type is defined in main.go

// All ensures all dependencies are installed
func (Deps) All() {
	fmt.Println("Installing all dependencies...")
	deps.GetAllDeps()
}

// Update updates all dependencies
func (Deps) Update() error {
	fmt.Println("Updating dependencies...")
	return sh.Run("go", "get", "-u", "./...")
}

// Tidy runs go mod tidy
func (Deps) Tidy() error {
	fmt.Println("Tidying dependencies...")
	return sh.Run("go", "mod", "tidy")
}

// Install installs a specific dependency
func (Deps) Install(name string) error {
	fmt.Printf("Installing dependency: %s\n", name)
	dep := deps.BinDep(name)
	return dep()
}
