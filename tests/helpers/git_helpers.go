package helpers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	languagepkg "github.com/blairham/go-pre-commit/pkg/language"
)

// LanguageTestSuite provides common test patterns for language implementations
type LanguageTestSuite struct {
	NewLanguageFunc func() languagepkg.Setup
	Name            string
	ExecutableName  string
	VersionFlag     string
	ExpectedEnvPath string
	TestVersions    []string
}

// RunBasicLanguageTests runs the standard set of tests for a language
func (suite *LanguageTestSuite) RunBasicLanguageTests(t *testing.T) {
	t.Helper()
	tempDir := suite.createTempDir(t)
	defer suite.cleanupTempDir(t, tempDir)

	lang := suite.NewLanguageFunc()

	suite.runNewLanguageTests(t, lang)
	suite.runSetupEnvironmentTests(t, lang, tempDir)
	suite.runHealthCheckTests(t, lang, tempDir)
	suite.runEnvironmentPathTests(t, tempDir)
}

// createTempDir creates a temporary directory for testing
func (suite *LanguageTestSuite) createTempDir(t *testing.T) string {
	t.Helper()
	tempDir, err := os.MkdirTemp("", strings.ToLower(suite.Name)+"-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return tempDir
}

// cleanupTempDir removes the temporary directory
func (suite *LanguageTestSuite) cleanupTempDir(t *testing.T, tempDir string) {
	t.Helper()
	if err := os.RemoveAll(tempDir); err != nil {
		t.Logf("⚠️  Warning: failed to remove temp dir: %v", err)
	}
}

// runNewLanguageTests tests basic language instantiation
func (suite *LanguageTestSuite) runNewLanguageTests(t *testing.T, lang languagepkg.Setup) {
	t.Helper()
	t.Run("NewLanguage", func(t *testing.T) {
		if lang == nil {
			t.Fatalf("NewLanguage returned nil")
		}
		if lang.GetExecutableName() != suite.ExecutableName {
			t.Errorf(
				"Expected executable name '%s', got %s",
				suite.ExecutableName,
				lang.GetExecutableName(),
			)
		}
	})
}

// runSetupEnvironmentTests tests environment setup for different versions
func (suite *LanguageTestSuite) runSetupEnvironmentTests(
	t *testing.T,
	lang languagepkg.Setup,
	tempDir string,
) {
	t.Helper()
	t.Run("SetupEnvironment", func(t *testing.T) {
		for _, version := range suite.TestVersions {
			versionName := version
			if versionName == "" {
				versionName = languagepkg.VersionDefault
			}

			t.Run("version_"+versionName, func(t *testing.T) {
				repoPath := suite.createTestRepo(t, tempDir, versionName)
				suite.testEnvironmentSetup(t, lang, tempDir, version, repoPath)
			})
		}
	})
}

// createTestRepo creates a test repository directory
func (suite *LanguageTestSuite) createTestRepo(t *testing.T, tempDir, versionName string) string {
	t.Helper()
	repoPath := filepath.Join(tempDir, "test_repo_"+versionName)
	if err := os.MkdirAll(repoPath, 0o750); err != nil {
		t.Fatalf("Failed to create test repo directory: %v", err)
	}
	return repoPath
}

// testEnvironmentSetup tests setting up the environment for a specific version
func (suite *LanguageTestSuite) testEnvironmentSetup(
	t *testing.T,
	lang languagepkg.Setup,
	tempDir, version, repoPath string,
) {
	t.Helper()
	envPath, err := lang.SetupEnvironmentWithRepo(tempDir, version, repoPath, "", nil)
	if err != nil {
		t.Logf(
			"Setup failed for %s version %s: %v (this might be expected)",
			suite.Name,
			version,
			err,
		)
	} else {
		t.Logf("✓ Successfully set up %s version %s at %s", suite.Name, version, envPath)
		suite.verifyEnvironmentDir(t, envPath)
	}
}

// verifyEnvironmentDir checks that the environment directory was created
func (suite *LanguageTestSuite) verifyEnvironmentDir(t *testing.T, envPath string) {
	t.Helper()
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Errorf("Environment directory was not created: %s", envPath)
	}
}

// runHealthCheckTests tests environment health checking
func (suite *LanguageTestSuite) runHealthCheckTests(
	t *testing.T,
	lang languagepkg.Setup,
	tempDir string,
) {
	t.Helper()
	t.Run("CheckEnvironmentHealth", func(t *testing.T) {
		envDirName := languagepkg.GetRepositoryEnvironmentName(suite.Name, "")
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

// runEnvironmentPathTests tests environment path generation if expected path is configured
func (suite *LanguageTestSuite) runEnvironmentPathTests(t *testing.T, tempDir string) {
	t.Helper()
	if suite.ExpectedEnvPath != "" {
		t.Run("RepositoryEnvironmentPath", func(t *testing.T) {
			testVersion := suite.getLastTestVersion()
			envDirName := languagepkg.GetRepositoryEnvironmentName(suite.Name, testVersion)
			if envDirName != "" {
				envPath := filepath.Join(tempDir, envDirName)
				expectedSuffix := suite.ExpectedEnvPath
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

// getLastTestVersion returns the last test version or empty string
func (suite *LanguageTestSuite) getLastTestVersion() string {
	if len(suite.TestVersions) > 0 {
		return suite.TestVersions[len(suite.TestVersions)-1]
	}
	return ""
}
