package pyenv

import "fmt"

// Python version constants - mapping major.minor to latest patch
const (
	Python312 = "3.12"
	Python311 = "3.11"
	Python310 = "3.10"
	Python39  = "3.9"
	BaseURL   = "https://www.python.org/ftp/python"
)

// Python patch versions - latest patch for each major.minor
var pythonPatchVersions = map[string]string{
	"3.12": "3.12.7",
	"3.11": "3.11.9",
	"3.10": "3.10.12",
	"3.9":  "3.9.19",
}

// getStableVersions returns a curated list of stable Python versions
func (m *Manager) getStableVersions() []PythonRelease {
	return []PythonRelease{
		m.createPythonRelease(Python312), // Latest version first
		m.createPythonRelease(Python311),
		m.createPythonRelease(Python310),
		m.createPythonRelease(Python39),
	}
}

// createPythonRelease creates a PythonRelease for a given version
func (m *Manager) createPythonRelease(version string) PythonRelease {
	// Get the latest patch version for this major.minor
	patchVersion, exists := pythonPatchVersions[version]
	if !exists {
		patchVersion = version // fallback to original version if not found
	}

	return PythonRelease{
		Version: version, // Use the major.minor version as the key
		Downloads: map[string]PythonVersion{
			"darwin-arm64":  m.createDarwinVersion(version, patchVersion),
			"darwin-amd64":  m.createDarwinVersion(version, patchVersion),
			"linux-amd64":   m.createLinuxVersion(version, patchVersion),
			"windows-amd64": m.createWindowsVersion(version, patchVersion),
		},
	}
}

// createDarwinVersion creates a macOS Python version
func (m *Manager) createDarwinVersion(version, patchVersion string) PythonVersion {
	return PythonVersion{
		Version:    version, // Use major.minor as the version identifier
		URL:        fmt.Sprintf("%s/%s/python-%s-macos11.pkg", BaseURL, patchVersion, patchVersion),
		Filename:   fmt.Sprintf("python-%s-macos11.pkg", patchVersion),
		Available:  true,
		IsPrebuilt: true,
	}
}

// createLinuxVersion creates a Linux Python version
func (m *Manager) createLinuxVersion(version, patchVersion string) PythonVersion {
	return PythonVersion{
		Version:    version, // Use major.minor as the version identifier
		URL:        fmt.Sprintf("%s/%s/Python-%s.tgz", BaseURL, patchVersion, patchVersion),
		Filename:   fmt.Sprintf("Python-%s.tgz", patchVersion),
		Available:  true,
		IsPrebuilt: false,
	}
}

// createWindowsVersion creates a Windows Python version
func (m *Manager) createWindowsVersion(version, patchVersion string) PythonVersion {
	return PythonVersion{
		Version:    version, // Use major.minor as the version identifier
		URL:        fmt.Sprintf("%s/%s/python-%s-amd64.exe", BaseURL, patchVersion, patchVersion),
		Filename:   fmt.Sprintf("python-%s-amd64.exe", patchVersion),
		Available:  true,
		IsPrebuilt: true,
	}
}
