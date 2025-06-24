//go:build mage
// +build mage

package main

import (
	"github.com/magefile/mage/mg"
)

// Default target to run when none is specified
var Default = Build.Binary

// Aliases creates aliases for the nested targets
var Aliases = map[string]interface{}{
	"build":      Build.Binary,
	"test":       Test.Unit,
	"clean":      Clean.All,
	"install":    Build.Install,
	"lint":       Quality.Lint,
	"fmt":        Quality.Format,
	"vet":        Quality.Vet,
	"modernize":  Quality.Modernize,
	"check":      Quality.All,
	"dev":        Dev.Run,
	"release":    Release.Build,
	"snapshot":   Release.Snapshot,
	"deps":       Deps.All,
	"goreleaser": Release.Check,
}

// Build namespace for build-related targets
type Build mg.Namespace

// Test namespace for testing-related targets
type Test mg.Namespace

// Quality namespace for code quality targets
type Quality mg.Namespace

// Clean namespace for cleanup targets
type Clean mg.Namespace

// Dev namespace for development targets
type Dev mg.Namespace

// Release namespace for release targets
type Release mg.Namespace

// Deps namespace for dependency management
type Deps mg.Namespace
