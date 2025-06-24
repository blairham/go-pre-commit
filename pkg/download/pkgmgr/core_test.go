package pkgmgr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/constants"
)

const testEnvPath = constants.TmpEnvPath

func TestPythonPackageManager(t *testing.T) {
	tests := []struct {
		name       string
		executable string
	}{
		{
			name:       "python3 executable",
			executable: "python3",
		},
		{
			name:       "python executable",
			executable: "python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ppm := NewPythonPackageManager(tt.executable)
			assert.NotNil(t, ppm)
			assert.Equal(t, tt.executable, ppm.executable)
			assert.Equal(t, tt.executable, ppm.GetExecutableName())
		})
	}
}

func TestPythonPackageManager_GetExecutablePath(t *testing.T) {
	ppm := NewPythonPackageManager("python3")
	envPath := testEnvPath

	expected := filepath.Join(envPath, "bin")
	assert.Equal(t, expected, ppm.GetExecutablePath(envPath))
}

func TestPythonPackageManager_CreateManifest(t *testing.T) {
	tests := []struct {
		name            string
		expectedContent string
		packages        []string
	}{
		{
			name:            "empty packages",
			packages:        []string{},
			expectedContent: "",
		},
		{
			name:            "single package",
			packages:        []string{"requests"},
			expectedContent: "requests\n",
		},
		{
			name:            "multiple packages",
			packages:        []string{"requests", "flask", "django==3.2"},
			expectedContent: "requests\nflask\ndjango==3.2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			ppm := NewPythonPackageManager("python3")

			err := ppm.CreateManifest(tempDir, tt.packages)
			assert.NoError(t, err)

			if len(tt.packages) == 0 {
				// No file should be created for empty packages
				_, statErr := os.Stat(filepath.Join(tempDir, "requirements.txt"))
				assert.True(t, os.IsNotExist(statErr))
				return
			}

			// Check the requirements.txt file
			requirementsPath := filepath.Join(tempDir, "requirements.txt")
			content, err := os.ReadFile(requirementsPath)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, string(content))

			// Check file permissions
			stat, err := os.Stat(requirementsPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
		})
	}
}

func TestPythonPackageManager_InstallPackages(t *testing.T) {
	tempDir := t.TempDir()
	ppm := NewPythonPackageManager("python3")

	// Test empty packages (should not error)
	err := ppm.InstallPackages(tempDir, []string{})
	assert.NoError(t, err)

	// Test with non-existent pip (should error)
	err = ppm.InstallPackages(tempDir, []string{"requests"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pip not found in environment")
}

func TestPythonPackageManager_InstallPackages_WithMockPip(t *testing.T) {
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	err := os.MkdirAll(binDir, 0o755)
	require.NoError(t, err)

	// Create a mock pip script that succeeds
	pipPath := filepath.Join(binDir, "pip")
	pipScript := `#!/bin/bash
echo "Successfully installed packages: $@"
exit 0`
	err = os.WriteFile(pipPath, []byte(pipScript), 0o755)
	require.NoError(t, err)

	ppm := NewPythonPackageManager("python3")
	err = ppm.InstallPackages(tempDir, []string{"requests", "flask"})
	assert.NoError(t, err)
}

func TestPythonPackageManager_InstallPackages_PipFallback(t *testing.T) {
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	err := os.MkdirAll(binDir, 0o755)
	require.NoError(t, err)

	// Create only pip3 (not pip)
	pip3Path := filepath.Join(binDir, "pip3")
	pipScript := `#!/bin/bash
echo "Successfully installed packages: $@"
exit 0`
	err = os.WriteFile(pip3Path, []byte(pipScript), 0o755)
	require.NoError(t, err)

	ppm := NewPythonPackageManager("python3")
	err = ppm.InstallPackages(tempDir, []string{"requests"})
	assert.NoError(t, err)
}

func TestNodePackageManager(t *testing.T) {
	tests := []struct {
		name           string
		packageManager string
		expected       string
	}{
		{
			name:           "default to npm",
			packageManager: "",
			expected:       "npm",
		},
		{
			name:           "explicit npm",
			packageManager: "npm",
			expected:       "npm",
		},
		{
			name:           "yarn",
			packageManager: "yarn",
			expected:       "yarn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			npm := NewNodePackageManager(tt.packageManager)
			assert.NotNil(t, npm)
			assert.Equal(t, tt.expected, npm.packageManager)
			assert.Equal(t, tt.expected, npm.GetExecutableName())
		})
	}
}

func TestNodePackageManager_GetExecutablePath(t *testing.T) {
	npm := NewNodePackageManager("npm")
	envPath := testEnvPath

	expected := filepath.Join(envPath, "node_modules", ".bin")
	assert.Equal(t, expected, npm.GetExecutablePath(envPath))
}

func TestNodePackageManager_CreateManifest(t *testing.T) {
	tests := []struct {
		name     string
		packages []string
	}{
		{
			name:     "empty packages",
			packages: []string{},
		},
		{
			name:     "packages with versions",
			packages: []string{"express@4.18.0", "lodash@4.17.21"},
		},
		{
			name:     "packages without versions",
			packages: []string{"react", "vue"},
		},
		{
			name:     "mixed packages",
			packages: []string{"express@4.18.0", "react", "lodash@4.17.21"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			npm := NewNodePackageManager("npm")

			err := npm.CreateManifest(tempDir, tt.packages)
			assert.NoError(t, err)

			// Check the package.json file
			packageJSONPath := filepath.Join(tempDir, "package.json")
			content, err := os.ReadFile(packageJSONPath)
			require.NoError(t, err)

			contentStr := string(content)
			assert.Contains(t, contentStr, `"name": "pre-commit-env"`)
			assert.Contains(t, contentStr, `"version": "1.0.0"`)

			// Check dependencies section for non-empty packages
			if len(tt.packages) > 0 {
				assert.Contains(t, contentStr, `"dependencies"`)
				for _, pkg := range tt.packages {
					parts := strings.Split(pkg, "@")
					packageName := parts[0]
					assert.Contains(t, contentStr, packageName)
				}
			}

			// Check file permissions
			stat, err := os.Stat(packageJSONPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
		})
	}
}

func TestNodePackageManager_InstallPackages(t *testing.T) {
	tempDir := t.TempDir()
	npm := NewNodePackageManager("npm")

	// Test empty packages (should not error)
	err := npm.InstallPackages(tempDir, []string{})
	assert.NoError(t, err)

	// Test with non-existent package manager (should error)
	npmNonExistent := NewNodePackageManager("non-existent-npm")
	err = npmNonExistent.InstallPackages(tempDir, []string{"express"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "install failed")
}

func TestRubyPackageManager(t *testing.T) {
	rpm := NewRubyPackageManager()
	assert.NotNil(t, rpm)
	assert.Equal(t, "gem", rpm.GetExecutableName())
}

func TestRubyPackageManager_GetExecutablePath(t *testing.T) {
	rpm := NewRubyPackageManager()
	envPath := testEnvPath

	expected := filepath.Join(envPath, "bin")
	assert.Equal(t, expected, rpm.GetExecutablePath(envPath))
}

func TestRubyPackageManager_CreateManifest(t *testing.T) {
	tests := []struct {
		name            string
		expectedContent string
		packages        []string
	}{
		{
			name:            "empty packages",
			packages:        []string{},
			expectedContent: "",
		},
		{
			name:            "single package",
			packages:        []string{"rails"},
			expectedContent: "source 'https://rubygems.org'\n\ngem 'rails'\n",
		},
		{
			name:            "multiple packages",
			packages:        []string{"rails", "rspec", "rubocop"},
			expectedContent: "source 'https://rubygems.org'\n\ngem 'rails'\ngem 'rspec'\ngem 'rubocop'\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			rpm := NewRubyPackageManager()

			err := rpm.CreateManifest(tempDir, tt.packages)
			assert.NoError(t, err)

			if len(tt.packages) == 0 {
				// No file should be created for empty packages
				_, statErr := os.Stat(filepath.Join(tempDir, "Gemfile"))
				assert.True(t, os.IsNotExist(statErr))
				return
			}

			// Check the Gemfile
			gemfilePath := filepath.Join(tempDir, "Gemfile")
			content, err := os.ReadFile(gemfilePath)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedContent, string(content))

			// Check file permissions
			stat, err := os.Stat(gemfilePath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
		})
	}
}

func TestRubyPackageManager_InstallPackages(t *testing.T) {
	tempDir := t.TempDir()
	rpm := NewRubyPackageManager()

	// Test empty packages (should not error)
	err := rpm.InstallPackages(tempDir, []string{})
	assert.NoError(t, err)

	// Test with non-existent gem (should error)
	err = rpm.InstallPackages(tempDir, []string{"rails"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "gem install failed")
}

func TestPackageManagerFactory(t *testing.T) {
	factory := NewPackageManagerFactory()
	assert.NotNil(t, factory)

	// Test supported languages
	supportedLangs := factory.GetSupportedLanguages()
	expectedLangs := []string{"python", "python3", "node", "nodejs", "ruby"}
	assert.ElementsMatch(t, expectedLangs, supportedLangs)
}

func TestPackageManagerFactory_CreatePackageManager(t *testing.T) {
	factory := NewPackageManagerFactory()

	tests := []struct {
		name        string
		language    string
		expectedPM  string
		expectError bool
	}{
		{
			name:        "python",
			language:    "python",
			expectError: false,
			expectedPM:  "*pkgmgr.PythonPackageManager",
		},
		{
			name:        "python3",
			language:    "python3",
			expectError: false,
			expectedPM:  "*pkgmgr.PythonPackageManager",
		},
		{
			name:        "node",
			language:    "node",
			expectError: false,
			expectedPM:  "*pkgmgr.NodePackageManager",
		},
		{
			name:        "nodejs",
			language:    "nodejs",
			expectError: false,
			expectedPM:  "*pkgmgr.NodePackageManager",
		},
		{
			name:        "ruby",
			language:    "ruby",
			expectError: false,
			expectedPM:  "*pkgmgr.RubyPackageManager",
		},
		{
			name:        "unsupported language",
			language:    "rust",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm, err := factory.CreatePackageManager(tt.language)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, pm)
				assert.Contains(t, err.Error(), "unsupported language")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pm)

				// Verify the correct type was created
				switch tt.language {
				case "python", "python3":
					_, ok := pm.(*PythonPackageManager)
					assert.True(t, ok)
				case "node", "nodejs":
					_, ok := pm.(*NodePackageManager)
					assert.True(t, ok)
				case "ruby":
					_, ok := pm.(*RubyPackageManager)
					assert.True(t, ok)
				}
			}
		})
	}
}

func TestUnifiedPackageInstaller(t *testing.T) {
	upi := NewUnifiedPackageInstaller()
	assert.NotNil(t, upi)
	assert.NotNil(t, upi.factory)
}

func TestUnifiedPackageInstaller_InstallPackagesForLanguage(t *testing.T) {
	tempDir := t.TempDir()
	upi := NewUnifiedPackageInstaller()

	tests := []struct {
		name        string
		language    string
		errorMsg    string
		packages    []string
		expectError bool
	}{
		{
			name:        "empty packages",
			language:    "python",
			packages:    []string{},
			expectError: false,
		},
		{
			name:        "unsupported language",
			language:    "rust",
			packages:    []string{"tokio"},
			expectError: true,
			errorMsg:    "failed to create package manager",
		},
		{
			name:        "python packages - availability dependent",
			language:    "python",
			packages:    []string{"requests"},
			expectError: true, // Will either fail due to missing pip or unavailable python
			errorMsg:    "",   // Don't check specific message since it depends on system
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := upi.InstallPackagesForLanguage(tt.language, tempDir, tt.packages)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUnifiedPackageInstaller_InstallPackagesForLanguage_ManifestCreation(t *testing.T) {
	tempDir := t.TempDir()
	upi := NewUnifiedPackageInstaller()

	// Test manifest creation for Python (even though install will fail)
	err := upi.InstallPackagesForLanguage("python", tempDir, []string{"requests"})
	assert.Error(t, err) // Will fail on install due to missing pip

	// But manifest should be created
	requirementsPath := filepath.Join(tempDir, "requirements.txt")
	_, err = os.Stat(requirementsPath)
	assert.NoError(t, err)

	content, err := os.ReadFile(requirementsPath)
	require.NoError(t, err)
	assert.Equal(t, "requests\n", string(content))
}

func TestPackageManagerInterface(_ *testing.T) {
	// Test that all package managers implement the interface correctly
	var _ PackageManager = &PythonPackageManager{}
	var _ PackageManager = &NodePackageManager{}
	var _ PackageManager = &RubyPackageManager{}
}

func TestPackageManager_IsAvailable(t *testing.T) {
	// These tests check the actual system availability, so we can't guarantee results
	// but we can test that the methods don't panic

	ppm := NewPythonPackageManager("python3")
	available := ppm.IsAvailable()
	// Just ensure it returns a boolean without panicking
	assert.IsType(t, false, available)

	npm := NewNodePackageManager("npm")
	available = npm.IsAvailable()
	assert.IsType(t, false, available)

	rpm := NewRubyPackageManager()
	available = rpm.IsAvailable()
	assert.IsType(t, false, available)
}
