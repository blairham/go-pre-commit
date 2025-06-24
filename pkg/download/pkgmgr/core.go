// Package pkgmgr provides unified package manager abstractions
package pkgmgr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PackageManager defines a unified interface for different package managers
type PackageManager interface {
	// InstallPackages installs packages in the given environment
	InstallPackages(envPath string, packages []string) error

	// CreateManifest creates a manifest file for the environment
	CreateManifest(envPath string, packages []string) error

	// GetExecutablePath returns the path to executables in this environment
	GetExecutablePath(envPath string) string

	// GetExecutableName returns the name of the package manager executable
	GetExecutableName() string

	// IsAvailable checks if the package manager is available on the system
	IsAvailable() bool
}

// PythonPackageManager handles Python pip/pip3 package management
type PythonPackageManager struct {
	executable string
}

// NewPythonPackageManager creates a new Python package manager
func NewPythonPackageManager(pythonExecutable string) *PythonPackageManager {
	return &PythonPackageManager{
		executable: pythonExecutable,
	}
}

// InstallPackages installs the specified packages using pip
func (ppm *PythonPackageManager) InstallPackages(envPath string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	pipPath := filepath.Join(envPath, "bin", "pip")
	if _, err := os.Stat(pipPath); os.IsNotExist(err) {
		// Try pip3
		pipPath = filepath.Join(envPath, "bin", "pip3")
		if _, err := os.Stat(pipPath); os.IsNotExist(err) {
			return fmt.Errorf("pip not found in environment %s", envPath)
		}
	}

	args := append([]string{"install"}, packages...)
	cmd := exec.Command(pipPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pip install failed: %w\nOutput: %s", err, output)
	}

	return nil
}

// CreateManifest creates a requirements.txt file with the specified packages
func (ppm *PythonPackageManager) CreateManifest(envPath string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	requirementsPath := filepath.Join(envPath, "requirements.txt")
	content := strings.Join(packages, "\n") + "\n"

	return os.WriteFile(
		requirementsPath,
		[]byte(content),
		0o600, //nolint:gosec // requirements file needs secure permissions
	)
}

// GetExecutablePath returns the path to the Python executable
func (ppm *PythonPackageManager) GetExecutablePath(envPath string) string {
	return filepath.Join(envPath, "bin")
}

// GetExecutableName returns the name of the Python executable
func (ppm *PythonPackageManager) GetExecutableName() string {
	return ppm.executable
}

// IsAvailable checks if Python is available on the system
func (ppm *PythonPackageManager) IsAvailable() bool {
	_, err := exec.LookPath(ppm.executable)
	return err == nil
}

// NodePackageManager handles Node.js npm/yarn package management
type NodePackageManager struct {
	packageManager string // "npm" or "yarn"
}

// NewNodePackageManager creates a new Node package manager
func NewNodePackageManager(packageManager string) *NodePackageManager {
	if packageManager == "" {
		packageManager = "npm"
	}
	return &NodePackageManager{
		packageManager: packageManager,
	}
}

// InstallPackages installs the specified packages using npm/yarn
func (npm *NodePackageManager) InstallPackages(envPath string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	args := []string{"install"}
	if npm.packageManager == "npm" {
		args = append(args, "--prefix", envPath)
	}
	args = append(args, packages...)

	cmd := exec.Command(npm.packageManager, args...)
	if npm.packageManager == "yarn" {
		cmd.Dir = envPath
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s install failed: %w\nOutput: %s", npm.packageManager, err, output)
	}

	return nil
}

// CreateManifest creates a package.json file with the specified packages
func (npm *NodePackageManager) CreateManifest(envPath string, packages []string) error {
	packageJSON := map[string]any{
		"name":         "pre-commit-env",
		"version":      "1.0.0",
		"description":  "Pre-commit environment",
		"dependencies": make(map[string]string),
	}

	// Parse packages into dependencies
	deps, ok := packageJSON["dependencies"].(map[string]string)
	if !ok {
		return fmt.Errorf("invalid package.json dependencies format")
	}
	for _, pkg := range packages {
		// Simple parsing: package@version or just package
		parts := strings.Split(pkg, "@")
		if len(parts) == 2 {
			deps[parts[0]] = parts[1]
		} else {
			deps[pkg] = "latest"
		}
	}

	// Write package.json (simplified)
	content := `{
  "name": "pre-commit-env",
  "version": "1.0.0",
  "description": "Pre-commit environment"
}`

	if len(packages) > 0 {
		// In a real implementation, use proper JSON marshaling
		content = strings.Replace(content, "}", `,
  "dependencies": {`, 1)

		for pkg, version := range deps {
			content += fmt.Sprintf("    %q: %q,", pkg, version)
		}
		content = strings.TrimSuffix(content, ",") + "\n  }\n}"
	}

	return os.WriteFile(
		filepath.Join(envPath, "package.json"),
		[]byte(content),
		0o600, //nolint:gosec // package.json needs secure permissions
	)
}

// GetExecutablePath returns the path to the Node.js executable
func (npm *NodePackageManager) GetExecutablePath(envPath string) string {
	return filepath.Join(envPath, "node_modules", ".bin")
}

// GetExecutableName returns the name of the Node.js executable
func (npm *NodePackageManager) GetExecutableName() string {
	return npm.packageManager
}

// IsAvailable checks if Node.js is available on the system
func (npm *NodePackageManager) IsAvailable() bool {
	_, err := exec.LookPath(npm.packageManager)
	return err == nil
}

// RubyPackageManager handles Ruby gem package management
type RubyPackageManager struct{}

// NewRubyPackageManager creates a new Ruby package manager
func NewRubyPackageManager() *RubyPackageManager {
	return &RubyPackageManager{}
}

// InstallPackages installs the specified packages using gem
func (rpm *RubyPackageManager) InstallPackages(envPath string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	// Use bundler for Ruby package management
	for _, pkg := range packages {
		args := []string{"install", pkg, "--install-dir", envPath}
		cmd := exec.Command("gem", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("gem install failed for %s: %w\nOutput: %s", pkg, err, output)
		}
	}

	return nil
}

// CreateManifest creates a Gemfile with the specified packages
func (rpm *RubyPackageManager) CreateManifest(envPath string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	gemfile := "source 'https://rubygems.org'\n\n"
	for _, pkg := range packages {
		gemfile += fmt.Sprintf("gem '%s'\n", pkg)
	}

	return os.WriteFile(
		filepath.Join(envPath, "Gemfile"),
		[]byte(gemfile),
		0o600, //nolint:gosec // Gemfile needs secure permissions
	)
}

// GetExecutablePath returns the path to the Ruby executable
func (rpm *RubyPackageManager) GetExecutablePath(envPath string) string {
	return filepath.Join(envPath, "bin")
}

// GetExecutableName returns the name of the Ruby executable
func (rpm *RubyPackageManager) GetExecutableName() string {
	return "gem"
}

// IsAvailable checks if Ruby is available on the system
func (rpm *RubyPackageManager) IsAvailable() bool {
	_, err := exec.LookPath("gem")
	return err == nil
}

// PackageManagerFactory creates appropriate package managers for languages
type PackageManagerFactory struct{}

// NewPackageManagerFactory creates a new package manager factory
func NewPackageManagerFactory() *PackageManagerFactory {
	return &PackageManagerFactory{}
}

// CreatePackageManager creates a package manager for the specified language
func (pmf *PackageManagerFactory) CreatePackageManager(language string) (PackageManager, error) {
	switch language {
	case "python", "python3":
		return NewPythonPackageManager("python3"), nil
	case "node", "nodejs":
		return NewNodePackageManager("npm"), nil
	case "ruby":
		return NewRubyPackageManager(), nil
	default:
		return nil, fmt.Errorf("unsupported language for package management: %s", language)
	}
}

// GetSupportedLanguages returns languages that support package management
func (pmf *PackageManagerFactory) GetSupportedLanguages() []string {
	return []string{"python", "python3", "node", "nodejs", "ruby"}
}

// UnifiedPackageInstaller provides a high-level interface for package installation
type UnifiedPackageInstaller struct {
	factory *PackageManagerFactory
}

// NewUnifiedPackageInstaller creates a new unified package installer
func NewUnifiedPackageInstaller() *UnifiedPackageInstaller {
	return &UnifiedPackageInstaller{
		factory: NewPackageManagerFactory(),
	}
}

// InstallPackagesForLanguage installs packages for a specific language environment
func (upi *UnifiedPackageInstaller) InstallPackagesForLanguage(
	language, envPath string, packages []string,
) error {
	if len(packages) == 0 {
		return nil
	}

	pm, err := upi.factory.CreatePackageManager(language)
	if err != nil {
		return fmt.Errorf("failed to create package manager for %s: %w", language, err)
	}

	if !pm.IsAvailable() {
		return fmt.Errorf("package manager for %s is not available", language)
	}

	// Create manifest first
	if err := pm.CreateManifest(envPath, packages); err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// Install packages
	if err := pm.InstallPackages(envPath, packages); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}

	return nil
}
