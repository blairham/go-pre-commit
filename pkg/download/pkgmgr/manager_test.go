package pkgmgr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
}

func TestType_Constants(t *testing.T) {
	// Test that the constants are defined correctly
	assert.Equal(t, Type(0), Swift)
	assert.Equal(t, Type(1), Dart)
	assert.Equal(t, Type(2), Ruby)
	assert.Equal(t, Type(3), Node)
	assert.Equal(t, Type(4), Python)
}

func TestManager_CreateManifest_Swift(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager()

	manifest := &Manifest{
		Name:    "TestPackage",
		Version: "1.0.0",
		Dependencies: []string{
			"https://github.com/apple/swift-argument-parser@1.0.0",
			"https://github.com/vapor/vapor",
		},
		ManifestType: Swift,
	}

	err := manager.CreateManifest(tempDir, manifest)
	assert.NoError(t, err)

	// Check Package.swift was created
	packagePath := filepath.Join(tempDir, "Package.swift")
	content, err := os.ReadFile(packagePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "// swift-tools-version:5.5")
	assert.Contains(t, contentStr, `name: "TestPackage"`)
	assert.Contains(t, contentStr, ".package(url: \"https://github.com/apple/swift-argument-parser\", from: \"1.0.0\")")
	assert.Contains(t, contentStr, ".package(url: \"https://github.com/vapor/vapor\", branch: \"main\")")

	// Check file permissions
	stat, err := os.Stat(packagePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
}

func TestManager_CreateManifest_Dart(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager()

	manifest := &Manifest{
		Name:         "test_package",
		Version:      "1.0.0",
		Dependencies: []string{"http:^0.13.0", "json_annotation", "build_runner:^2.1.0"},
		ManifestType: Dart,
	}

	err := manager.CreateManifest(tempDir, manifest)
	assert.NoError(t, err)

	// Check pubspec.yaml was created
	pubspecPath := filepath.Join(tempDir, "pubspec.yaml")
	content, err := os.ReadFile(pubspecPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "name: test_package")
	assert.Contains(t, contentStr, "version: 1.0.0")
	assert.Contains(t, contentStr, "sdk: '>=2.17.0 <4.0.0'")
	assert.Contains(t, contentStr, "http: ^0.13.0")
	assert.Contains(t, contentStr, "json_annotation: any")
	assert.Contains(t, contentStr, "build_runner: ^2.1.0")

	// Check file permissions
	stat, err := os.Stat(pubspecPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
}

func TestManager_CreateManifest_Ruby(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager()

	manifest := &Manifest{
		Name:         "test_gem",
		Version:      "1.0.0",
		Dependencies: []string{"rails", "rspec", "rubocop"},
		ManifestType: Ruby,
	}

	err := manager.CreateManifest(tempDir, manifest)
	assert.NoError(t, err)

	// Check Gemfile was created
	gemfilePath := filepath.Join(tempDir, "Gemfile")
	content, err := os.ReadFile(gemfilePath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "source 'https://rubygems.org'")
	assert.Contains(t, contentStr, "gem 'rails'")
	assert.Contains(t, contentStr, "gem 'rspec'")
	assert.Contains(t, contentStr, "gem 'rubocop'")

	// Check file permissions
	stat, err := os.Stat(gemfilePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
}

func TestManager_CreateManifest_WithAdditionalFiles(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager()

	additionalFiles := []File{
		{
			Path:    "src/main.swift",
			Content: "import Foundation\nprint(\"Hello, World!\")",
			Mode:    0o644,
		},
		{
			Path:    "README.md",
			Content: "# Test Package\nThis is a test package.",
			Mode:    0o644,
		},
		{
			Path:    "scripts/build.sh",
			Content: "#!/bin/bash\necho \"Building...\"",
			Mode:    0o755,
		},
	}

	manifest := &Manifest{
		Name:            "TestPackage",
		Version:         "1.0.0",
		Dependencies:    []string{"https://github.com/vapor/vapor"},
		AdditionalFiles: additionalFiles,
		ManifestType:    Swift,
	}

	err := manager.CreateManifest(tempDir, manifest)
	assert.NoError(t, err)

	// Check that all additional files were created
	for _, file := range additionalFiles {
		fullPath := filepath.Join(tempDir, file.Path)

		// Check file exists
		fileStat, statErr := os.Stat(fullPath)
		require.NoError(t, statErr)

		// Check file permissions
		assert.Equal(t, file.Mode, fileStat.Mode().Perm())

		// Check file content
		content, readErr := os.ReadFile(fullPath)
		require.NoError(t, readErr)
		assert.Equal(t, file.Content, string(content))
	}

	// Check main manifest was also created
	packagePath := filepath.Join(tempDir, "Package.swift")
	_, err = os.Stat(packagePath)
	assert.NoError(t, err)
}

func TestManager_CreateManifest_UnsupportedType(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager()

	manifest := &Manifest{
		Name:         "test",
		Version:      "1.0.0",
		Dependencies: []string{"some-dep"},
		ManifestType: Type(999), // Invalid type
	}

	err := manager.CreateManifest(tempDir, manifest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported package manager type: 999")
}

func TestManager_CreateManifest_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, "non", "existent", "path")
	manager := NewManager()

	manifest := &Manifest{
		Name:         "TestPackage",
		Version:      "1.0.0",
		Dependencies: []string{},
		ManifestType: Swift,
	}

	err := manager.CreateManifest(envPath, manifest)
	assert.NoError(t, err)

	// Check that the directory was created
	stat, err := os.Stat(envPath)
	require.NoError(t, err)
	assert.True(t, stat.IsDir())

	// Check that Package.swift was created
	packagePath := filepath.Join(envPath, "Package.swift")
	_, err = os.Stat(packagePath)
	assert.NoError(t, err)
}

func TestManager_generateSwiftPackage(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name          string
		manifest      *Manifest
		expectedParts []string
	}{
		{
			name: "basic package",
			manifest: &Manifest{
				Name:         "MyPackage",
				Dependencies: []string{},
			},
			expectedParts: []string{
				"// swift-tools-version:5.5",
				"import PackageDescription",
				`name: "MyPackage"`,
				".macOS(.v10_15)",
			},
		},
		{
			name: "package with dependencies",
			manifest: &Manifest{
				Name: "MyPackage",
				Dependencies: []string{
					"https://github.com/vapor/vapor@4.0.0",
					"https://github.com/apple/swift-argument-parser",
				},
			},
			expectedParts: []string{
				".package(url: \"https://github.com/vapor/vapor\", from: \"4.0.0\")",
				".package(url: \"https://github.com/apple/swift-argument-parser\", branch: \"main\")",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := manager.generateSwiftPackage(tt.manifest)

			for _, expectedPart := range tt.expectedParts {
				assert.Contains(t, content, expectedPart)
			}
		})
	}
}

func TestManager_generateDartPubspec(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name          string
		manifest      *Manifest
		expectedParts []string
	}{
		{
			name: "basic pubspec",
			manifest: &Manifest{
				Name:         "my_package",
				Version:      "1.0.0",
				Dependencies: []string{},
			},
			expectedParts: []string{
				"name: my_package",
				"version: 1.0.0",
				"sdk: '>=2.17.0 <4.0.0'",
			},
		},
		{
			name: "pubspec with dependencies",
			manifest: &Manifest{
				Name:    "my_package",
				Version: "1.0.0",
				Dependencies: []string{
					"http:^0.13.0",
					"json_annotation",
					"build_runner:^2.1.0",
				},
			},
			expectedParts: []string{
				"http: ^0.13.0",
				"json_annotation: any",
				"build_runner: ^2.1.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := manager.generateDartPubspec(tt.manifest)

			for _, expectedPart := range tt.expectedParts {
				assert.Contains(t, content, expectedPart)
			}
		})
	}
}

func TestManager_generateRubyGemfile(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name          string
		manifest      *Manifest
		expectedParts []string
	}{
		{
			name: "basic gemfile",
			manifest: &Manifest{
				Dependencies: []string{},
			},
			expectedParts: []string{
				"source 'https://rubygems.org'",
			},
		},
		{
			name: "gemfile with dependencies",
			manifest: &Manifest{
				Dependencies: []string{"rails", "rspec", "rubocop"},
			},
			expectedParts: []string{
				"source 'https://rubygems.org'",
				"gem 'rails'",
				"gem 'rspec'",
				"gem 'rubocop'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := manager.generateRubyGemfile(tt.manifest)

			for _, expectedPart := range tt.expectedParts {
				assert.Contains(t, content, expectedPart)
			}
		})
	}
}

func TestManager_RunInstallCommand(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager()

	tests := []struct {
		name         string
		errorMsg     string
		manifestType Type
		expectError  bool
	}{
		{
			name:         "swift install command",
			manifestType: Swift,
			expectError:  true, // Will fail because swift command doesn't exist
			errorMsg:     "failed to run package manager command",
		},
		{
			name:         "dart install command",
			manifestType: Dart,
			expectError:  true, // Will fail because dart command doesn't exist
			errorMsg:     "failed to run package manager command",
		},
		{
			name:         "ruby install command",
			manifestType: Ruby,
			expectError:  true, // Will fail because bundle command doesn't exist
			errorMsg:     "failed to run package manager command",
		},
		{
			name:         "unsupported type",
			manifestType: Type(999),
			expectError:  true,
			errorMsg:     "unsupported package manager type for command execution: 999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.RunInstallCommand(tempDir, tt.manifestType)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_CheckManifestExists(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager()

	tests := []struct {
		setupFunc    func(string) error
		name         string
		manifestType Type
		expected     bool
	}{
		{
			name:         "swift - no files",
			manifestType: Swift,
			setupFunc:    func(string) error { return nil },
			expected:     false,
		},
		{
			name:         "swift - only manifest",
			manifestType: Swift,
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "Package.swift"), []byte("// test"), 0o644)
			},
			expected: false, // Missing Package.resolved
		},
		{
			name:         "swift - manifest and resolved",
			manifestType: Swift,
			setupFunc: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "Package.swift"), []byte("// test"), 0o644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "Package.resolved"), []byte("{}"), 0o644)
			},
			expected: true,
		},
		{
			name:         "dart - no files",
			manifestType: Dart,
			setupFunc:    func(string) error { return nil },
			expected:     false,
		},
		{
			name:         "dart - only manifest",
			manifestType: Dart,
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: test"), 0o644)
			},
			expected: false, // Missing .dart_tool/package_config.json
		},
		{
			name:         "dart - manifest and resolved",
			manifestType: Dart,
			setupFunc: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: test"), 0o644); err != nil {
					return err
				}
				dartToolDir := filepath.Join(dir, ".dart_tool")
				if err := os.MkdirAll(dartToolDir, 0o755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dartToolDir, "package_config.json"), []byte("{}"), 0o644)
			},
			expected: true,
		},
		{
			name:         "ruby - no files",
			manifestType: Ruby,
			setupFunc:    func(string) error { return nil },
			expected:     false,
		},
		{
			name:         "ruby - only manifest",
			manifestType: Ruby,
			setupFunc: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'"), 0o644)
			},
			expected: false, // Missing Gemfile.lock
		},
		{
			name:         "ruby - manifest and lock",
			manifestType: Ruby,
			setupFunc: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'"), 0o644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "Gemfile.lock"), []byte("DEPENDENCIES"), 0o644)
			},
			expected: true,
		},
		{
			name:         "unsupported type",
			manifestType: Type(999),
			setupFunc:    func(string) error { return nil },
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh subdirectory for this test
			testDir := filepath.Join(tempDir, tt.name)
			err := os.MkdirAll(testDir, 0o755)
			require.NoError(t, err)

			// Setup files
			err = tt.setupFunc(testDir)
			require.NoError(t, err)

			// Test
			result := manager.CheckManifestExists(testDir, tt.manifestType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManifest_Struct(t *testing.T) {
	// Test that the Manifest struct can be created and fields accessed
	manifest := &Manifest{
		Name:         "TestPackage",
		Version:      "1.0.0",
		Dependencies: []string{"dep1", "dep2"},
		AdditionalFiles: []File{
			{
				Path:    "test.txt",
				Content: "test content",
				Mode:    0o644,
			},
		},
		ManifestType: Swift,
	}

	assert.Equal(t, "TestPackage", manifest.Name)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.Len(t, manifest.Dependencies, 2)
	assert.Len(t, manifest.AdditionalFiles, 1)
	assert.Equal(t, Swift, manifest.ManifestType)
}

func TestFile_Struct(t *testing.T) {
	// Test that the File struct can be created and fields accessed
	file := File{
		Path:    "src/main.go",
		Content: "package main\n\nfunc main() {}",
		Mode:    0o644,
	}

	assert.Equal(t, "src/main.go", file.Path)
	assert.Contains(t, file.Content, "package main")
	assert.Equal(t, os.FileMode(0o644), file.Mode)
}

func TestManager_CreateManifest_FileWriteError(t *testing.T) {
	// Test error handling when file cannot be written
	invalidDir := "/root/cannot_write_here"
	manager := NewManager()

	manifest := &Manifest{
		Name:         "TestPackage",
		Version:      "1.0.0",
		Dependencies: []string{},
		ManifestType: Swift,
	}

	err := manager.CreateManifest(invalidDir, manifest)
	assert.Error(t, err)
	// The exact error will depend on the system, but it should be a directory creation error
}

func TestManager_CreateManifest_AdditionalFileError(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager()

	// Create an additional file with an invalid path (containing directory that can't be created)
	additionalFiles := []File{
		{
			Path:    "\x00invalid/path", // Null character makes this invalid on most systems
			Content: "test",
			Mode:    0o644,
		},
	}

	manifest := &Manifest{
		Name:            "TestPackage",
		Version:         "1.0.0",
		Dependencies:    []string{},
		AdditionalFiles: additionalFiles,
		ManifestType:    Swift,
	}

	err := manager.CreateManifest(tempDir, manifest)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create directory for")
}

func TestPackageManager_ParseDependencies(t *testing.T) {
	manager := NewManager()

	// Test Swift dependency parsing
	manifest := &Manifest{
		Name: "Test",
		Dependencies: []string{
			"https://github.com/vapor/vapor@4.0.0",
			"https://github.com/apple/swift-argument-parser",
			"https://github.com/realm/realm-swift@10.0.0",
		},
	}

	content := manager.generateSwiftPackage(manifest)

	// Check that versioned dependencies use "from"
	assert.Contains(t, content, ".package(url: \"https://github.com/vapor/vapor\", from: \"4.0.0\")")
	assert.Contains(t, content, ".package(url: \"https://github.com/realm/realm-swift\", from: \"10.0.0\")")

	// Check that unversioned dependencies use "branch: main"
	assert.Contains(t, content, ".package(url: \"https://github.com/apple/swift-argument-parser\", branch: \"main\")")

	// Test Dart dependency parsing
	dartContent := manager.generateDartPubspec(&Manifest{
		Name:    "test",
		Version: "1.0.0",
		Dependencies: []string{
			"http:^0.13.0",
			"json_annotation",
			"build_runner:^2.1.0",
		},
	})

	assert.Contains(t, dartContent, "http: ^0.13.0")
	assert.Contains(t, dartContent, "json_annotation: any")
	assert.Contains(t, dartContent, "build_runner: ^2.1.0")
}
