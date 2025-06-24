package nodeenv

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// NodeJSRelease represents a Node.js release from the official API
type NodeJSRelease struct {
	LTS     any      `json:"lts"`
	Version string   `json:"version"`
	Date    string   `json:"date"`
	V8      string   `json:"v8"`
	NPM     string   `json:"npm"`
	Modules string   `json:"modules"`
	OpenSSL string   `json:"openssl"`
	Files   []string `json:"files"`
}

// VersionInfo provides enhanced version information
type VersionInfo struct {
	Date       time.Time
	Version    string
	LTSName    string
	V8Version  string
	NPMVersion string
	IsLTS      bool
}

const (
	// NodeJSReleasesURL is the official Node.js releases API endpoint
	NodeJSReleasesURL = "https://nodejs.org/dist/index.json"

	// RequestTimeout for HTTP requests
	RequestTimeout = 10 * time.Second
)

// FetchAvailableVersions fetches available Node.js versions from the official API
func (m *Manager) FetchAvailableVersions() ([]VersionInfo, error) {
	return m.FetchAvailableVersionsWithContext(context.Background())
}

// FetchAvailableVersionsWithContext fetches available Node.js versions with context
func (m *Manager) FetchAvailableVersionsWithContext(ctx context.Context) ([]VersionInfo, error) {
	client := &http.Client{
		Timeout: RequestTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", NodeJSReleasesURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Node.js releases: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			// Log the error but don't override the main error
			_ = closeErr // Explicitly ignore close error
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch Node.js releases: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var releases []NodeJSRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("failed to parse Node.js releases: %w", err)
	}

	return m.convertToVersionInfo(releases), nil
}

// convertToVersionInfo converts API releases to VersionInfo structs
func (m *Manager) convertToVersionInfo(releases []NodeJSRelease) []VersionInfo {
	versions := make([]VersionInfo, 0, len(releases))

	for _, release := range releases {
		// Parse date
		date, err := time.Parse("2006-01-02", release.Date)
		if err != nil {
			// If date parsing fails, use zero time but continue
			date = time.Time{}
		}

		// Determine LTS status
		isLTS := false
		ltsName := ""
		if release.LTS != nil && release.LTS != false {
			isLTS = true
			if ltsStr, ok := release.LTS.(string); ok {
				ltsName = ltsStr
			}
		}

		version := VersionInfo{
			Version:    strings.TrimPrefix(release.Version, "v"),
			LTSName:    ltsName,
			IsLTS:      isLTS,
			Date:       date,
			V8Version:  release.V8,
			NPMVersion: release.NPM,
		}

		versions = append(versions, version)
	}

	// Sort by version (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i].Version, versions[j].Version) > 0
	})

	return versions
}

// GetLTSVersions returns only LTS versions
func (m *Manager) GetLTSVersions() ([]VersionInfo, error) {
	versions, err := m.FetchAvailableVersions()
	if err != nil {
		return nil, err
	}

	var ltsVersions []VersionInfo
	for _, version := range versions {
		if version.IsLTS {
			ltsVersions = append(ltsVersions, version)
		}
	}

	return ltsVersions, nil
}

// GetLatestVersion returns the latest Node.js version
func (m *Manager) GetLatestVersion() (VersionInfo, error) {
	versions, err := m.FetchAvailableVersions()
	if err != nil {
		return VersionInfo{}, err
	}

	if len(versions) == 0 {
		return VersionInfo{}, fmt.Errorf("no versions available")
	}

	return versions[0], nil
}

// GetLatestLTSVersion returns the latest LTS version
func (m *Manager) GetLatestLTSVersion() (VersionInfo, error) {
	ltsVersions, err := m.GetLTSVersions()
	if err != nil {
		return VersionInfo{}, err
	}

	if len(ltsVersions) == 0 {
		return VersionInfo{}, fmt.Errorf("no LTS versions available")
	}

	return ltsVersions[0], nil
}

// FindVersion finds a version that matches the given specification
func (m *Manager) FindVersion(spec string) (VersionInfo, error) {
	versions, err := m.FetchAvailableVersions()
	if err != nil {
		return VersionInfo{}, err
	}

	// Handle special cases
	switch spec {
	case "latest":
		return m.GetLatestVersion()
	case "lts":
		return m.GetLatestLTSVersion()
	}

	// Handle LTS codenames (e.g., "hydrogen", "gallium")
	lowerSpec := strings.ToLower(spec)
	for _, version := range versions {
		if version.IsLTS && strings.EqualFold(version.LTSName, lowerSpec) {
			return version, nil
		}
	}

	// Handle version prefixes (e.g., "18", "18.19")
	spec = strings.TrimPrefix(spec, "v")
	for _, version := range versions {
		if strings.HasPrefix(version.Version, spec) {
			return version, nil
		}
	}

	// Exact match
	for _, version := range versions {
		if version.Version == spec {
			return version, nil
		}
	}

	return VersionInfo{}, fmt.Errorf("version %q not found", spec)
}

// compareVersions compares two semantic version strings
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if v1 == v2
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Ensure both have at least 3 parts (major.minor.patch)
	for len(parts1) < 3 {
		parts1 = append(parts1, "0")
	}
	for len(parts2) < 3 {
		parts2 = append(parts2, "0")
	}

	for i := range 3 {
		num1 := parseVersionNumber(parts1[i])
		num2 := parseVersionNumber(parts2[i])

		if num1 > num2 {
			return 1
		}
		if num1 < num2 {
			return -1
		}
	}

	return 0
}

// parseVersionNumber extracts the numeric part from a version component
func parseVersionNumber(s string) int {
	// Handle pre-release versions by taking only the numeric part
	for i, char := range s {
		if char < '0' || char > '9' {
			s = s[:i]
			break
		}
	}

	if s == "" {
		return 0
	}

	var num int
	for _, char := range s {
		if char >= '0' && char <= '9' {
			num = num*10 + int(char-'0')
		}
	}

	return num
}

// GetVersionSummary returns a summary of Node.js versions (for display)
func (m *Manager) GetVersionSummary() (string, error) {
	versions, err := m.FetchAvailableVersions()
	if err != nil {
		return "", err
	}

	if len(versions) == 0 {
		return "No Node.js versions available", nil
	}

	latest := versions[0]
	ltsVersions, err := m.GetLTSVersions()
	if err != nil {
		return "", err
	}

	summary := fmt.Sprintf("Latest Node.js version: %s\n", latest.Version)

	if len(ltsVersions) > 0 {
		latestLTS := ltsVersions[0]
		summary += fmt.Sprintf("Latest LTS version: %s (%s)\n", latestLTS.Version, latestLTS.LTSName)
	}

	summary += fmt.Sprintf("Total available versions: %d\n", len(versions))
	summary += fmt.Sprintf("LTS versions available: %d", len(ltsVersions))

	return summary, nil
}

// IsVersionAvailable checks if a version is available for download
func (m *Manager) IsVersionAvailable(version string) (bool, error) {
	_, err := m.FindVersion(version)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
