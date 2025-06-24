// Package helpers provides test utilities and helper functions for the go-pre-commit test suite
package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	languagepkg "github.com/blairham/go-pre-commit/pkg/language"
)

// LanguageTestConfig holds the configuration for testing a language
type LanguageTestConfig struct {
	Language       any      // The language instance to test
	Name           string   // Expected language name
	ExecutableName string   // Expected executable name
	VersionFlag    string   // Expected version flag
	EnvPathSuffix  string   // Expected suffix for environment path (optional)
	TestVersions   []string // Versions to test for environment setup
}

// LanguageForTest defines the interface that test languages must implement
type LanguageForTest interface {
	SetupEnvironment(cacheDir, version string, additionalDeps []string) (string, error)
	CheckEnvironmentHealth(envPath string) bool
	GetName() string
	GetExecutableName() string
	GetVersionFlag() string
}

// RunLanguageTests runs the standard set of tests for a language
func RunLanguageTests(t *testing.T, config LanguageTestConfig) {
	t.Helper()
	tempDir := createTempTestDir(t, config.Name)
	defer cleanupTempTestDir(t, tempDir)

	lang := assertLanguageInterface(t, config)

	runBasicLanguageTests(t, config, lang)
	runEnvironmentSetupTests(t, config, lang, tempDir)
	runHealthCheckTests(t, config, lang, tempDir)
	runEnvironmentPathTests(t, config, tempDir)
}

// createTempTestDir creates a temporary directory for testing
func createTempTestDir(t *testing.T, name string) string {
	t.Helper()
	tempDir, err := os.MkdirTemp("", strings.ToLower(name)+"-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return tempDir
}

// cleanupTempTestDir removes the temporary test directory
func cleanupTempTestDir(t *testing.T, tempDir string) {
	t.Helper()
	if err := os.RemoveAll(tempDir); err != nil {
		t.Logf("⚠️  Warning: failed to remove temp directory %s: %v", tempDir, err)
	}
}

// assertLanguageInterface validates that the language implements the test interface
func assertLanguageInterface(t *testing.T, config LanguageTestConfig) LanguageForTest {
	t.Helper()
	lang, ok := config.Language.(LanguageForTest)
	if !ok {
		t.Fatalf("Language does not implement LanguageForTest interface")
	}
	return lang
}

// runBasicLanguageTests tests basic language properties
func runBasicLanguageTests(t *testing.T, config LanguageTestConfig, lang LanguageForTest) {
	t.Helper()
	t.Run("New"+config.Name+"Language", func(t *testing.T) {
		if lang == nil {
			t.Fatalf("New%sLanguage returned nil", config.Name)
		}
		if lang.GetName() != config.Name {
			t.Errorf("Expected name '%s', got %s", config.Name, lang.GetName())
		}
		if lang.GetExecutableName() != config.ExecutableName {
			t.Errorf(
				"Expected executable name '%s', got %s",
				config.ExecutableName,
				lang.GetExecutableName(),
			)
		}
		if lang.GetVersionFlag() != config.VersionFlag {
			t.Errorf(
				"Expected version flag '%s', got %s",
				config.VersionFlag,
				lang.GetVersionFlag(),
			)
		}
	})
}

// runEnvironmentSetupTests tests environment setup for different versions
func runEnvironmentSetupTests(
	t *testing.T,
	config LanguageTestConfig,
	lang LanguageForTest,
	tempDir string,
) {
	t.Helper()
	t.Run("SetupEnvironment", func(t *testing.T) {
		for _, version := range config.TestVersions {
			versionName := version
			if versionName == "" {
				versionName = languagepkg.VersionDefault
			}

			t.Run("version_"+versionName, func(t *testing.T) {
				envPath, err := lang.SetupEnvironment(tempDir, version, nil)
				if err != nil {
					t.Logf(
						"Setup failed for %s version %s: %v (this might be expected)",
						config.Name,
						version,
						err,
					)
				} else {
					t.Logf("✓ Successfully set up %s version %s at %s", config.Name, version, envPath)
					verifyEnvironmentDirectory(t, envPath)
				}
			})
		}
	})
}

// verifyEnvironmentDirectory checks that the environment directory was created
func verifyEnvironmentDirectory(t *testing.T, envPath string) {
	t.Helper()
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Errorf("Environment directory was not created: %s", envPath)
	}
}

// runHealthCheckTests tests environment health checking
func runHealthCheckTests(
	t *testing.T,
	config LanguageTestConfig,
	lang LanguageForTest,
	tempDir string,
) {
	t.Helper()
	t.Run("CheckEnvironmentHealth", func(t *testing.T) {
		envDirName := languagepkg.GetRepositoryEnvironmentName(config.Name, "")
		if envDirName == "" {
			envDirName = "test_env"
		}
		envPath := filepath.Join(tempDir, envDirName)
		healthy := lang.CheckEnvironmentHealth(envPath)
		if healthy {
			t.Error("Expected CheckEnvironmentHealth to return false for non-existent environment")
		}
	})
}

// runEnvironmentPathTests tests environment path generation if suffix is provided
func runEnvironmentPathTests(t *testing.T, config LanguageTestConfig, tempDir string) {
	t.Helper()
	if config.EnvPathSuffix != "" {
		t.Run("RepositoryEnvironmentPath", func(t *testing.T) {
			testVersion := getTestVersion(config)
			envDirName := languagepkg.GetRepositoryEnvironmentName(config.Name, testVersion)
			if envDirName != "" {
				envPath := filepath.Join(tempDir, envDirName)
				expectedSuffix := config.EnvPathSuffix
				if !strings.HasSuffix(envPath, expectedSuffix) {
					t.Errorf(
						"Expected environment path to end with %s, got %s",
						expectedSuffix,
						envPath,
					)
				}
			}
		})
	}
}

// getTestVersion gets a test version from the config
func getTestVersion(config LanguageTestConfig) string {
	testVersion := "test-version"
	if len(config.TestVersions) > 0 && config.TestVersions[len(config.TestVersions)-1] != "" {
		testVersion = config.TestVersions[len(config.TestVersions)-1]
	}
	return testVersion
}
