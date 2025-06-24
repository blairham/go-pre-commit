package languages

import (
	"fmt"
	"strings"
	"testing"
)

func TestFailLanguage(t *testing.T) {
	fail := NewFailLanguage()

	// Use shared helper for comprehensive simple language testing
	testSimpleLanguageInterface(t, fail, "fail")

	// Additional fail-specific tests
	t.Run("NewFailLanguage_TypeChecks", func(t *testing.T) {
		if fail == nil {
			t.Error("NewFailLanguage() returned nil")
			return
		}
		if fail.GenericLanguage == nil {
			t.Error("NewFailLanguage() returned instance with nil SimpleLanguage")
		}
		if fail.Base == nil {
			t.Error("NewFailLanguage() returned instance with nil Base")
		}
	})
}

// Comprehensive tests to improve CheckHealth coverage in SimpleLanguage
func TestFailLanguage_SimpleLanguageCheckHealthCoverage(t *testing.T) {
	t.Run("CheckHealthWithEmptyExecutableName", func(t *testing.T) {
		// Test the fail language which has empty executable name
		fail := NewFailLanguage()
		tempDir := t.TempDir()

		// This should call SimpleCheckHealth since ExecutableName is empty
		err := fail.CheckHealth(tempDir, "1.0")
		if err != nil {
			t.Errorf("CheckHealth() with valid directory should not error: %v", err)
		}

		// Test with non-existent directory
		err = fail.CheckHealth("/non/existent/path", "1.0")
		if err == nil {
			t.Error("CheckHealth() with non-existent directory should return error")
		}
	})

	t.Run("CheckHealthWithNonEmptyExecutableName", func(t *testing.T) {
		// Test a simple language with non-empty executable name to cover the other branch
		simpleWithExec := NewGenericLanguage("test", "bash", "--version", "https://test.com")
		tempDir := t.TempDir()

		// This should call Base.CheckHealth since ExecutableName is not empty
		err := simpleWithExec.CheckHealth(tempDir, "1.0")
		// This might succeed or fail depending on environment, but it exercises the branch
		t.Logf("CheckHealth() with non-empty executable returned: %v", err)
	})

	t.Run("CheckHealthWithEmptyExecutableNameVariousScenarios", func(t *testing.T) {
		fail := NewFailLanguage()

		// Test various scenarios to ensure SimpleCheckHealth is thoroughly tested
		scenarios := []struct {
			name    string
			envPath string
			version string
		}{
			{"ValidPathValidVersion", t.TempDir(), "1.0"},
			{"ValidPathEmptyVersion", t.TempDir(), ""},
			{"EmptyPathValidVersion", "", "1.0"},
			{"EmptyPathEmptyVersion", "", ""},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				err := fail.CheckHealth(scenario.envPath, scenario.version)
				if scenario.envPath == "" {
					// Should fail with empty path
					if err == nil {
						t.Error("CheckHealth() should fail with empty path")
					}
				} else {
					// Should succeed with valid path
					if err != nil {
						t.Errorf("CheckHealth() should succeed with valid path: %v", err)
					}
				}
			})
		}
	})

	t.Run("CompareFailVsOtherSimpleLanguages", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test fail language (empty executable)
		fail := NewFailLanguage()
		errFail := fail.CheckHealth(tempDir, "1.0")

		// Test simple language with executable
		simpleWithExec := NewGenericLanguage("mock", "ls", "--help", "https://example.com")
		errWithExec := simpleWithExec.CheckHealth(tempDir, "1.0")

		// Both should handle the basic case, but through different code paths
		t.Logf("Fail CheckHealth: %v", errFail)
		t.Logf("SimpleWithExec CheckHealth: %v", errWithExec)

		// Test simple language without executable (like fail)
		simpleNoExec := NewGenericLanguage("mock", "", "", "https://example.com")
		errNoExec := simpleNoExec.CheckHealth(tempDir, "1.0")

		// This should behave like fail
		if (errFail == nil) != (errNoExec == nil) {
			t.Error("Languages with empty executable names should behave the same way")
		}
	})
}

// Additional edge case tests for comprehensive coverage
func TestFailLanguage_EdgeCasesAndErrorPaths(t *testing.T) {
	fail := NewFailLanguage()

	t.Run("PathEdgeCases", func(t *testing.T) {
		// Test various edge case paths
		testPaths := []string{
			"/path/with spaces/fail",
			"/path-with-dashes",
			"/path_with_underscores",
			"/path.with.dots",
			"relative/path",
			".",
			"/",
		}

		for _, path := range testPaths {
			t.Run("Path_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
				// Test CheckHealth with various paths
				err := fail.CheckHealth(path, "1.0")
				t.Logf("CheckHealth for path %q returned: %v", path, err)

				// Test CheckEnvironmentHealth
				result := fail.CheckEnvironmentHealth(path)
				t.Logf("CheckEnvironmentHealth for path %q returned: %v", path, result)
			})
		}
	})

	t.Run("VersionEdgeCases", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test various version formats
		versions := []string{
			"1.0.0",
			"latest",
			"stable",
			"beta",
			"v1.0",
			"1.0-alpha",
			"1.0.0-beta.1",
			"main",
			"master",
			"develop",
		}

		for _, version := range versions {
			t.Run("Version_"+version, func(t *testing.T) {
				err := fail.CheckHealth(tempDir, version)
				if err != nil {
					t.Errorf("CheckHealth with version %q should not error: %v", version, err)
				}
			})
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test concurrent calls to various methods
		done := make(chan bool, 4)

		go func() {
			err := fail.CheckHealth(tempDir, "1.0")
			t.Logf("Concurrent CheckHealth returned: %v", err)
			done <- true
		}()

		go func() {
			result := fail.CheckEnvironmentHealth(tempDir)
			t.Logf("Concurrent CheckEnvironmentHealth returned: %v", result)
			done <- true
		}()

		go func() {
			err := fail.InstallDependencies(tempDir, []string{"dep1"})
			t.Logf("Concurrent InstallDependencies returned: %v", err)
			done <- true
		}()

		go func() {
			envPath, err := fail.SetupEnvironmentWithRepo("", "1.0", tempDir, "https://example.com", []string{})
			t.Logf("Concurrent SetupEnvironmentWithRepo returned: %s, %v", envPath, err)
			done <- true
		}()

		// Wait for all goroutines
		for range 4 {
			<-done
		}
	})

	t.Run("MethodChaining", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test method calls in sequence to ensure state consistency
		envPath, err := fail.SetupEnvironmentWithRepo(
			"",
			"1.0",
			tempDir,
			"https://example.com",
			[]string{"dep1", "dep2"},
		)
		if err != nil {
			t.Errorf("SetupEnvironmentWithRepo failed: %v", err)
		}

		err = fail.InstallDependencies(envPath, []string{"dep3", "dep4"})
		if err != nil {
			t.Errorf("InstallDependencies failed: %v", err)
		}

		err = fail.CheckHealth(envPath, "1.0")
		if err != nil {
			t.Errorf("CheckHealth failed: %v", err)
		}

		result := fail.CheckEnvironmentHealth(envPath)
		t.Logf("Final CheckEnvironmentHealth result: %v", result)
	})

	t.Run("LargeInputs", func(t *testing.T) {
		tempDir := t.TempDir()

		// Test with large number of dependencies
		largeDeps := make([]string, 100)
		for i := range 100 {
			largeDeps[i] = fmt.Sprintf("dep%d", i)
		}

		err := fail.InstallDependencies(tempDir, largeDeps)
		if err != nil {
			t.Errorf("InstallDependencies with large input failed: %v", err)
		}

		// Test with very long paths
		longPath := tempDir + "/" + strings.Repeat("very-long-directory-name", 10)
		err = fail.CheckHealth(longPath, "1.0")
		if err == nil {
			t.Log("CheckHealth with very long path succeeded")
		} else {
			t.Logf("CheckHealth with very long path failed as expected: %v", err)
		}
	})
}
