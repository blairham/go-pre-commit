//go:build mage
// +build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/sh"
)

// Version displays the current version
func Version() error {
	fmt.Println("Version: 0.1.0") // Simple static version for now
	return nil
}

// Commit displays the current git commit
func Commit() error {
	commit, err := sh.Output("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	fmt.Printf("Commit: %s\n", commit)
	return nil
}
