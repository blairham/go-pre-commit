package pyenv

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test constants
const (
	testPatchVersion = "3.12.7"
)

func TestVersionConstants(t *testing.T) {
	t.Run("TestPythonVersionConstants", func(t *testing.T) {
		// Test that all Python version constants are properly defined
		assert.Equal(t, "3.12", Python312)
		assert.Equal(t, "3.11", Python311)
		assert.Equal(t, "3.10", Python310)
		assert.Equal(t, "3.9", Python39)

		// Test that BaseURL is properly defined
		assert.Equal(t, "https://www.python.org/ftp/python", BaseURL)
		assert.True(t, strings.HasPrefix(BaseURL, "https://"))
		assert.Contains(t, BaseURL, "python.org")
	})

	t.Run("TestPythonPatchVersionsMapping", func(t *testing.T) {
		// Test that pythonPatchVersions contains all expected versions
		expectedVersions := []string{"3.12", "3.11", "3.10", "3.9"}

		for _, version := range expectedVersions {
			patchVersion, exists := pythonPatchVersions[version]
			assert.True(t, exists, "pythonPatchVersions should contain version %s", version)
			assert.NotEmpty(t, patchVersion, "patch version for %s should not be empty", version)

			// Verify patch version format (should be major.minor.patch)
			parts := strings.Split(patchVersion, ".")
			assert.Equal(t, 3, len(parts), "patch version %s should have 3 parts", patchVersion)

			// Verify patch version starts with the major.minor version
			assert.True(t, strings.HasPrefix(patchVersion, version),
				"patch version %s should start with %s", patchVersion, version)
		}
	})

	t.Run("TestPythonPatchVersionsValues", func(t *testing.T) {
		// Test specific patch versions to ensure they are reasonable
		testCases := map[string]string{
			"3.12": "3.12.7",
			"3.11": "3.11.9",
			"3.10": "3.10.12",
			"3.9":  "3.9.19",
		}

		for majorMinor, expectedPatch := range testCases {
			actualPatch, exists := pythonPatchVersions[majorMinor]
			assert.True(t, exists, "pythonPatchVersions should contain %s", majorMinor)
			assert.Equal(t, expectedPatch, actualPatch, "patch version for %s should be %s", majorMinor, expectedPatch)
		}
	})
}

func TestGetStableVersions(t *testing.T) {
	manager := NewManager("/tmp/test-pyenv")

	t.Run("TestGetStableVersionsStructure", func(t *testing.T) {
		versions := manager.getStableVersions()

		// Should return a non-empty list
		assert.Greater(t, len(versions), 0, "getStableVersions should return at least one version")

		// Should contain the expected Python versions
		versionStrings := make([]string, len(versions))
		for i, v := range versions {
			versionStrings[i] = v.Version
		}

		assert.Contains(t, versionStrings, Python312)
		assert.Contains(t, versionStrings, Python311)
		assert.Contains(t, versionStrings, Python310)
		assert.Contains(t, versionStrings, Python39)
	})

	t.Run("TestGetStableVersionsOrder", func(t *testing.T) {
		versions := manager.getStableVersions()

		// First version should be the latest (Python312)
		assert.Equal(t, Python312, versions[0].Version, "first version should be the latest")

		// Should be in descending order
		expectedOrder := []string{Python312, Python311, Python310, Python39}
		for i, expectedVersion := range expectedOrder {
			if i < len(versions) {
				assert.Equal(t, expectedVersion, versions[i].Version,
					"version at index %d should be %s", i, expectedVersion)
			}
		}
	})

	t.Run("TestGetStableVersionsDownloads", func(t *testing.T) {
		versions := manager.getStableVersions()

		for _, version := range versions {
			// Each version should have downloads for all supported platforms
			assert.Contains(t, version.Downloads, "darwin-arm64",
				"version %s should have darwin-arm64 download", version.Version)
			assert.Contains(t, version.Downloads, "darwin-amd64",
				"version %s should have darwin-amd64 download", version.Version)
			assert.Contains(t, version.Downloads, "linux-amd64",
				"version %s should have linux-amd64 download", version.Version)
			assert.Contains(t, version.Downloads, "windows-amd64",
				"version %s should have windows-amd64 download", version.Version)

			// Test that downloads are properly structured
			for platform, download := range version.Downloads {
				assert.Equal(t, version.Version, download.Version,
					"download version should match release version for platform %s", platform)
				assert.NotEmpty(t, download.URL, "download URL should not be empty for platform %s", platform)
				assert.NotEmpty(t, download.Filename, "download filename should not be empty for platform %s", platform)
				assert.True(t, download.Available, "download should be marked as available for platform %s", platform)
			}
		}
	})
}

func TestCreatePythonReleaseExtensive(t *testing.T) {
	manager := NewManager("/tmp/test-pyenv")

	t.Run("TestCreatePythonReleaseAllKnownVersions", func(t *testing.T) {
		knownVersions := []string{Python312, Python311, Python310, Python39}

		for _, version := range knownVersions {
			release := manager.createPythonRelease(version)

			assert.Equal(t, version, release.Version, "release version should match input version")
			assert.Equal(t, 4, len(release.Downloads), "should have downloads for 4 platforms")

			// Verify all platforms are present
			expectedPlatforms := []string{"darwin-arm64", "darwin-amd64", "linux-amd64", "windows-amd64"}
			for _, platform := range expectedPlatforms {
				assert.Contains(t, release.Downloads, platform, "should contain platform %s", platform)
			}

			// Verify patch version is used in URLs
			expectedPatchVersion := pythonPatchVersions[version]
			for platform, download := range release.Downloads {
				assert.Contains(t, download.URL, expectedPatchVersion,
					"URL should contain patch version %s for platform %s", expectedPatchVersion, platform)
				assert.Contains(t, download.Filename, expectedPatchVersion,
					"filename should contain patch version %s for platform %s", expectedPatchVersion, platform)
			}
		}
	})

	t.Run("TestCreatePythonReleaseUnknownVersion", func(t *testing.T) {
		unknownVersions := []string{"3.8", "3.13", "4.0", "3.99.99"}

		for _, version := range unknownVersions {
			release := manager.createPythonRelease(version)

			assert.Equal(t, version, release.Version, "release version should match input version")
			assert.Equal(t, 4, len(release.Downloads), "should have downloads for 4 platforms")

			// For unknown versions, should fallback to using the original version
			for platform, download := range release.Downloads {
				assert.Contains(t, download.URL, version,
					"URL should contain original version %s for unknown version on platform %s", version, platform)
				assert.Contains(t, download.Filename, version,
					"filename should contain original version %s for unknown version on platform %s", version, platform)
			}
		}
	})

	t.Run("TestCreatePythonReleaseEdgeCases", func(t *testing.T) {
		edgeCases := []string{"", "invalid", "3", "3.12.7.8"}

		for _, version := range edgeCases {
			release := manager.createPythonRelease(version)

			assert.Equal(t, version, release.Version, "release version should match input version")
			assert.Equal(t, 4, len(release.Downloads), "should have downloads for 4 platforms")

			// Should still create valid downloads structure
			for _, download := range release.Downloads {
				assert.Equal(t, version, download.Version, "download version should match input")
				assert.NotEmpty(t, download.URL, "URL should not be empty")
				assert.NotEmpty(t, download.Filename, "filename should not be empty")
				assert.True(t, download.Available, "should be marked as available")
			}
		}
	})
}

func TestPlatformSpecificVersionCreation(t *testing.T) {
	manager := NewManager("/tmp/test-pyenv")

	t.Run("TestCreateDarwinVersionDetailed", func(t *testing.T) {
		testCases := []struct {
			version      string
			patchVersion string
			expectedURL  string
			expectedFile string
		}{
			{
				version:      "3.12",
				patchVersion: "3.12.7",
				expectedURL:  "https://www.python.org/ftp/python/3.12.7/python-3.12.7-macos11.pkg",
				expectedFile: "python-3.12.7-macos11.pkg",
			},
			{
				version:      "3.11",
				patchVersion: "3.11.9",
				expectedURL:  "https://www.python.org/ftp/python/3.11.9/python-3.11.9-macos11.pkg",
				expectedFile: "python-3.11.9-macos11.pkg",
			},
		}

		for _, tc := range testCases {
			version := manager.createDarwinVersion(tc.version, tc.patchVersion)

			assert.Equal(t, tc.version, version.Version)
			assert.Equal(t, tc.expectedURL, version.URL)
			assert.Equal(t, tc.expectedFile, version.Filename)
			assert.True(t, version.Available)
			assert.True(t, version.IsPrebuilt, "Darwin versions should be prebuilt")
		}
	})

	t.Run("TestCreateLinuxVersionDetailed", func(t *testing.T) {
		testCases := []struct {
			version      string
			patchVersion string
			expectedURL  string
			expectedFile string
		}{
			{
				version:      "3.12",
				patchVersion: "3.12.7",
				expectedURL:  "https://www.python.org/ftp/python/3.12.7/Python-3.12.7.tgz",
				expectedFile: "Python-3.12.7.tgz",
			},
			{
				version:      "3.11",
				patchVersion: "3.11.9",
				expectedURL:  "https://www.python.org/ftp/python/3.11.9/Python-3.11.9.tgz",
				expectedFile: "Python-3.11.9.tgz",
			},
		}

		for _, tc := range testCases {
			version := manager.createLinuxVersion(tc.version, tc.patchVersion)

			assert.Equal(t, tc.version, version.Version)
			assert.Equal(t, tc.expectedURL, version.URL)
			assert.Equal(t, tc.expectedFile, version.Filename)
			assert.True(t, version.Available)
			assert.False(t, version.IsPrebuilt, "Linux versions should not be prebuilt (need compilation)")
		}
	})

	t.Run("TestCreateWindowsVersionDetailed", func(t *testing.T) {
		testCases := []struct {
			version      string
			patchVersion string
			expectedURL  string
			expectedFile string
		}{
			{
				version:      "3.12",
				patchVersion: "3.12.7",
				expectedURL:  "https://www.python.org/ftp/python/3.12.7/python-3.12.7-amd64.exe",
				expectedFile: "python-3.12.7-amd64.exe",
			},
			{
				version:      "3.11",
				patchVersion: "3.11.9",
				expectedURL:  "https://www.python.org/ftp/python/3.11.9/python-3.11.9-amd64.exe",
				expectedFile: "python-3.11.9-amd64.exe",
			},
		}

		for _, tc := range testCases {
			version := manager.createWindowsVersion(tc.version, tc.patchVersion)

			assert.Equal(t, tc.version, version.Version)
			assert.Equal(t, tc.expectedURL, version.URL)
			assert.Equal(t, tc.expectedFile, version.Filename)
			assert.True(t, version.Available)
			assert.True(t, version.IsPrebuilt, "Windows versions should be prebuilt")
		}
	})
}

func TestVersionURLStructure(t *testing.T) {
	manager := NewManager("/tmp/test-pyenv")

	t.Run("TestURLFormatConsistency", func(t *testing.T) {
		version := Python312
		patchVersion := testPatchVersion

		darwinVer := manager.createDarwinVersion(version, patchVersion)
		linuxVer := manager.createLinuxVersion(version, patchVersion)
		windowsVer := manager.createWindowsVersion(version, patchVersion)

		// All URLs should start with BaseURL
		assert.True(t, strings.HasPrefix(darwinVer.URL, BaseURL))
		assert.True(t, strings.HasPrefix(linuxVer.URL, BaseURL))
		assert.True(t, strings.HasPrefix(windowsVer.URL, BaseURL))

		// All URLs should contain the patch version
		assert.Contains(t, darwinVer.URL, patchVersion)
		assert.Contains(t, linuxVer.URL, patchVersion)
		assert.Contains(t, windowsVer.URL, patchVersion)

		// URLs should follow the expected pattern
		assert.Regexp(t, `^https://www\.python\.org/ftp/python/\d+\.\d+\.\d+/.*$`, darwinVer.URL)
		assert.Regexp(t, `^https://www\.python\.org/ftp/python/\d+\.\d+\.\d+/.*$`, linuxVer.URL)
		assert.Regexp(t, `^https://www\.python\.org/ftp/python/\d+\.\d+\.\d+/.*$`, windowsVer.URL)
	})

	t.Run("TestFilenameConsistency", func(t *testing.T) {
		version := Python312
		patchVersion := testPatchVersion

		darwinVer := manager.createDarwinVersion(version, patchVersion)
		linuxVer := manager.createLinuxVersion(version, patchVersion)
		windowsVer := manager.createWindowsVersion(version, patchVersion)

		// All filenames should contain the patch version
		assert.Contains(t, darwinVer.Filename, patchVersion)
		assert.Contains(t, linuxVer.Filename, patchVersion)
		assert.Contains(t, windowsVer.Filename, patchVersion)

		// Check expected file extensions
		assert.True(t, strings.HasSuffix(darwinVer.Filename, ".pkg"))
		assert.True(t, strings.HasSuffix(linuxVer.Filename, ".tgz"))
		assert.True(t, strings.HasSuffix(windowsVer.Filename, ".exe"))

		// Check platform-specific naming conventions
		assert.Contains(t, darwinVer.Filename, "macos")
		assert.Contains(t, windowsVer.Filename, "amd64")
		assert.True(t, strings.HasPrefix(linuxVer.Filename, "Python-"))
	})
}

func TestVersionDataIntegrity(t *testing.T) {
	manager := NewManager("/tmp/test-pyenv")

	t.Run("TestPythonVersionStructureIntegrity", func(t *testing.T) {
		version := manager.createDarwinVersion(Python312, testPatchVersion)

		// Verify all required fields are set
		assert.NotEmpty(t, version.Version, "Version should not be empty")
		assert.NotEmpty(t, version.URL, "URL should not be empty")
		assert.NotEmpty(t, version.Filename, "Filename should not be empty")
		assert.True(t, version.Available, "Available should be true")

		// Verify the version structure is consistent
		assert.Equal(t, Python312, version.Version)
	})

	t.Run("TestPythonReleaseStructureIntegrity", func(t *testing.T) {
		release := manager.createPythonRelease(Python312)

		// Verify all required fields are set
		assert.NotEmpty(t, release.Version, "Version should not be empty")
		assert.NotEmpty(t, release.Downloads, "Downloads should not be empty")
		assert.Equal(t, 4, len(release.Downloads), "Should have 4 platform downloads")

		// Verify each download is properly structured
		for platform, download := range release.Downloads {
			assert.NotEmpty(t, platform, "Platform key should not be empty")
			assert.NotEmpty(t, download.Version, "Download version should not be empty")
			assert.NotEmpty(t, download.URL, "Download URL should not be empty")
			assert.NotEmpty(t, download.Filename, "Download filename should not be empty")
			assert.True(t, download.Available, "Download should be available")
		}
	})

	t.Run("TestVersionConstantsIntegrity", func(t *testing.T) {
		// Verify constants are properly defined and consistent
		constants := []string{Python312, Python311, Python310, Python39}

		for _, constant := range constants {
			assert.NotEmpty(t, constant, "Version constant should not be empty")
			assert.Regexp(t, `^\d+\.\d+$`, constant, "Version constant should match major.minor format")

			// Verify it exists in the patch versions map
			_, exists := pythonPatchVersions[constant]
			assert.True(t, exists, "Version constant %s should exist in pythonPatchVersions", constant)
		}

		// Verify BaseURL is well-formed
		assert.NotEmpty(t, BaseURL, "BaseURL should not be empty")
		assert.True(t, strings.HasPrefix(BaseURL, "https://"), "BaseURL should use HTTPS")
		assert.Contains(t, BaseURL, "python.org", "BaseURL should point to python.org")
	})
}

func BenchmarkVersionCreation(b *testing.B) {
	manager := NewManager("/tmp/test-pyenv")

	b.Run("BenchmarkGetStableVersions", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			versions := manager.getStableVersions()
			_ = versions // prevent optimization
		}
	})

	b.Run("BenchmarkCreatePythonRelease", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			release := manager.createPythonRelease(Python312)
			_ = release // prevent optimization
		}
	})

	b.Run("BenchmarkCreatePlatformVersions", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			darwin := manager.createDarwinVersion(Python312, testPatchVersion)
			linux := manager.createLinuxVersion(Python312, testPatchVersion)
			windows := manager.createWindowsVersion(Python312, testPatchVersion)
			_, _, _ = darwin, linux, windows // prevent optimization
		}
	})
}
