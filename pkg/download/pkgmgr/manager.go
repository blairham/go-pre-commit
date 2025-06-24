// Package pkgmgr provides utilities for working with various package managers
// like Swift Package Manager, Dart pub, Ruby gems, etc.
package pkgmgr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Type represents different package manager types
type Type int

const (
	// Swift represents Swift package manager
	Swift Type = iota
	// Dart represents Dart package manager
	Dart
	// Ruby represents Ruby package manager
	Ruby
	// Node represents Node package manager
	Node
	// Python represents Python package manager
	Python
)

// Manifest represents a package manifest configuration
type Manifest struct {
	Name            string
	Version         string
	Dependencies    []string
	AdditionalFiles []File
	ManifestType    Type
}

// File represents additional files that need to be created with the manifest
type File struct {
	Path    string
	Content string
	Mode    os.FileMode
}

// Manager provides package manager functionality
type Manager struct{}

// NewManager creates a new package manager instance
func NewManager() *Manager {
	return &Manager{}
}

// CreateManifest creates a package manifest file based on the type and dependencies
func (pm *Manager) CreateManifest(envPath string, manifest *Manifest) error {
	// Ensure the environment directory exists
	if err := os.MkdirAll(envPath, 0o750); err != nil {
		return fmt.Errorf("failed to create environment directory %s: %w", envPath, err)
	}

	var manifestPath string
	var manifestContent string

	switch manifest.ManifestType {
	case Swift:
		manifestPath = filepath.Join(envPath, "Package.swift")
		manifestContent = pm.generateSwiftPackage(manifest)
	case Dart:
		manifestPath = filepath.Join(envPath, "pubspec.yaml")
		manifestContent = pm.generateDartPubspec(manifest)
	case Ruby:
		manifestPath = filepath.Join(envPath, "Gemfile")
		manifestContent = pm.generateRubyGemfile(manifest)
	default:
		return fmt.Errorf("unsupported package manager type: %d", manifest.ManifestType)
	}

	// Write the manifest file
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o600); err != nil {
		return fmt.Errorf("failed to create manifest file %s: %w", manifestPath, err)
	}

	// Create additional files if specified
	for _, file := range manifest.AdditionalFiles {
		fullPath := filepath.Join(envPath, file.Path)

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", fullPath, err)
		}

		if err := os.WriteFile(fullPath, []byte(file.Content), file.Mode); err != nil {
			return fmt.Errorf("failed to create additional file %s: %w", fullPath, err)
		}
	}

	return nil
}

// generateSwiftPackage generates Package.swift content
func (pm *Manager) generateSwiftPackage(manifest *Manifest) string {
	content := `// swift-tools-version:5.5
import PackageDescription

let package = Package(
    name: "` + manifest.Name + `",
    platforms: [
        .macOS(.v10_15),
        .iOS(.v13),
        .tvOS(.v13),
        .watchOS(.v6)
    ],
    dependencies: [
`

	for _, dep := range manifest.Dependencies {
		// Parse dependency specification (url@version or url)
		parts := strings.Split(dep, "@")
		if len(parts) == 2 {
			content += fmt.Sprintf("        .package(url: %q, from: %q),\n", parts[0], parts[1])
		} else {
			content += fmt.Sprintf("        .package(url: %q, branch: \"main\"),\n", dep)
		}
	}

	content += `    ],
    targets: [
        .target(
            name: "` + manifest.Name + `",
            dependencies: [])
    ]
)
`
	return content
}

// generateDartPubspec generates pubspec.yaml content
func (pm *Manager) generateDartPubspec(manifest *Manifest) string {
	content := `name: ` + manifest.Name + `
version: ` + manifest.Version + `

environment:
  sdk: '>=2.17.0 <4.0.0'

dependencies:
`

	for _, dep := range manifest.Dependencies {
		// Parse dependency specification (name:version or just name)
		parts := strings.Split(dep, ":")
		if len(parts) == 2 {
			content += fmt.Sprintf("  %s: %s\n", parts[0], parts[1])
		} else {
			content += fmt.Sprintf("  %s: any\n", dep)
		}
	}

	return content
}

// generateRubyGemfile generates Gemfile content
func (pm *Manager) generateRubyGemfile(manifest *Manifest) string {
	content := "source 'https://rubygems.org'\n\n"
	for _, dep := range manifest.Dependencies {
		content += fmt.Sprintf("gem '%s'\n", dep)
	}
	return content
}

// RunInstallCommand runs the appropriate package manager install command
func (pm *Manager) RunInstallCommand(envPath string, manifestType Type) error {
	var cmd *exec.Cmd

	switch manifestType {
	case Swift:
		cmd = exec.Command("swift", "package", "resolve")
	case Dart:
		cmd = exec.Command("dart", "pub", "get")
	case Ruby:
		gemfilePath := filepath.Join(envPath, "Gemfile")
		cmd = exec.Command("bundle", "install", "--path", "vendor/bundle")
		cmd.Env = append(os.Environ(), "BUNDLE_GEMFILE="+gemfilePath)
	default:
		return fmt.Errorf("unsupported package manager type for command execution: %d", manifestType)
	}

	cmd.Dir = envPath

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run package manager command: %w\nOutput: %s", err, output)
	}

	return nil
}

// CheckManifestExists checks if a manifest file exists and dependencies are resolved
func (pm *Manager) CheckManifestExists(envPath string, manifestType Type) bool {
	var manifestPath string
	var resolvedPath string

	switch manifestType {
	case Swift:
		manifestPath = filepath.Join(envPath, "Package.swift")
		resolvedPath = filepath.Join(envPath, "Package.resolved")
	case Dart:
		manifestPath = filepath.Join(envPath, "pubspec.yaml")
		resolvedPath = filepath.Join(envPath, ".dart_tool", "package_config.json")
	case Ruby:
		manifestPath = filepath.Join(envPath, "Gemfile")
		resolvedPath = filepath.Join(envPath, "Gemfile.lock")
	default:
		return false
	}

	// Check if manifest exists
	if _, err := os.Stat(manifestPath); err != nil {
		return false
	}

	// Check if dependencies are resolved
	if _, err := os.Stat(resolvedPath); err != nil {
		return false
	}

	return true
}
