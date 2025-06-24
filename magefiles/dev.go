//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

// Dev namespace methods
// Note: Dev and Build types are defined in main.go

// Run builds and runs the application with help
func (Dev) Run() error {
	mg.Deps(Build.Binary)
	return sh.Run("./bin/pre-commit", "--help")
}

// Watch runs the application in watch mode (placeholder for future implementation)
func (Dev) Watch() error {
	fmt.Println("Watch mode not yet implemented...")
	return nil
}
