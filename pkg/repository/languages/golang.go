package languages

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/download"
	"github.com/blairham/go-pre-commit/pkg/language"
)

// GoLanguage handles Go environment setup with goenv-like functionality
type GoLanguage struct {
	*language.Base
}

// NewGoLanguage creates a new Go language handler
func NewGoLanguage() *GoLanguage {
	return &GoLanguage{
		Base: language.NewBase(
			"golang",
			"go",
			"version",
			"https://golang.org/",
		),
	}
}

// Architecture mapping for Go downloads (matching Python pre-commit's _ARCH_ALIASES)
var archAliases = map[string]string{
	"x86_64":  "amd64",
	"i386":    "386",
	"aarch64": "arm64",
	"armv8":   "arm64",
	"armv7l":  "armv6l",
}

// getGoArch returns the Go architecture string for downloads
func getGoArch() string {
	arch := strings.ToLower(runtime.GOARCH)
	if alias, ok := archAliases[arch]; ok {
		return alias
	}
	return arch
}

// getGoOS returns the Go OS string for downloads
func getGoOS() string {
	return strings.ToLower(runtime.GOOS)
}

// getGoExtension returns the archive extension based on OS
func getGoExtension() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// inferGoVersion determines the Go version to use
// If version is "default", it fetches the latest stable version from go.dev
func inferGoVersion(version string) (string, error) {
	if version != language.VersionDefault && version != "" {
		return version, nil
	}

	// Fetch latest Go version from go.dev
	resp, err := http.Get("https://go.dev/dl/?mode=json")
	if err != nil {
		return "", fmt.Errorf("failed to fetch Go versions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Go versions response: %w", err)
	}

	var releases []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return "", fmt.Errorf("failed to parse Go versions: %w", err)
	}

	if len(releases) == 0 {
		return "", fmt.Errorf("no Go releases found")
	}

	// Return the first (latest) version, removing the "go" prefix
	return strings.TrimPrefix(releases[0].Version, "go"), nil
}

// getGoDownloadURL constructs the download URL for a specific Go version
func getGoDownloadURL(version string) string {
	return fmt.Sprintf("https://dl.google.com/go/go%s.%s-%s.%s",
		version, getGoOS(), getGoArch(), getGoExtension())
}

// installGo downloads and installs Go to the specified destination
func (g *GoLanguage) installGo(version, destDir string) error {
	url := getGoDownloadURL(version)
	fmt.Printf("Downloading Go %s from %s...\n", version, url)

	// Use the download manager for archive handling
	mgr := download.NewManager()
	if err := mgr.DownloadAndExtract(url, destDir); err != nil {
		return fmt.Errorf("failed to download Go: %w", err)
	}

	// Move the extracted 'go' directory to '.go' (matching Python pre-commit)
	srcDir := filepath.Join(destDir, "go")
	dstDir := filepath.Join(destDir, ".go")

	// Check if extraction created the expected directory
	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("expected 'go' directory not found after extraction: %w", err)
	}

	// Move to .go
	if err := os.Rename(srcDir, dstDir); err != nil {
		return fmt.Errorf("failed to move Go installation: %w", err)
	}

	fmt.Printf("Go %s installed successfully to %s\n", version, dstDir)
	return nil
}

// GetDefaultVersion returns the default Go version
// Following Python pre-commit behavior: returns 'system' if Go is installed, otherwise 'default'
func (g *GoLanguage) GetDefaultVersion() string {
	// Check if system Go is available
	if g.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (g *GoLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	// Use the cache-aware pre-initialization for proper cache tracking
	return g.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "go")
}

// SetupEnvironmentWithRepoInfo sets up a Go environment with repository URL information
func (g *GoLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return g.CacheAwareSetupEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "go")
}

// SetupEnvironmentWithRepo sets up a Go environment for a specific repository
func (g *GoLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return g.setupEnvironmentWithRepoInternal(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// setupEnvironmentWithRepoInternal contains the actual environment setup logic
func (g *GoLanguage) setupEnvironmentWithRepoInternal(
	cacheDir, version, repoPath, _ string,
	additionalDeps []string,
) (string, error) {
	// Determine if we should use system Go or bootstrap
	useSystem := version == language.VersionSystem
	if version == "" && g.IsRuntimeAvailable() {
		// Default to system if available
		useSystem = true
		version = language.VersionSystem
	}

	// Determine the Go version for environment naming
	envVersion := version
	if envVersion == language.VersionDefault || envVersion == "" {
		// Infer the version for environment naming
		var err error
		envVersion, err = inferGoVersion(version)
		if err != nil {
			return "", fmt.Errorf("failed to determine Go version: %w", err)
		}
	}

	// Create environment path
	envDirName := language.GetRepositoryEnvironmentName("go", envVersion)
	envPath := filepath.Join(cacheDir, envDirName)

	// Check if environment already exists and is functional
	if g.IsEnvironmentInstalled(envPath, repoPath) {
		return envPath, nil
	}

	// Environment exists but might be broken, remove and recreate
	if _, err := os.Stat(envPath); err == nil {
		if err := os.RemoveAll(envPath); err != nil {
			return "", fmt.Errorf("failed to remove broken Go environment: %w", err)
		}
	}

	// Create environment directory
	if err := g.CreateEnvironmentDirectory(envPath); err != nil {
		return "", fmt.Errorf("failed to create Go environment directory: %w", err)
	}

	if useSystem {
		// System Go: create symlinks to system Go binaries
		if !g.IsRuntimeAvailable() {
			return "", fmt.Errorf("go runtime not found. Please install Go to use Go hooks.\n"+
				"Installation instructions: %s", g.InstallURL)
		}
		if err := g.setupSystemGoSymlinks(envPath); err != nil {
			return "", fmt.Errorf("failed to setup Go symlinks: %w", err)
		}
	} else {
		// Bootstrap Go: download and install the specified version
		goVersion, err := inferGoVersion(version)
		if err != nil {
			return "", fmt.Errorf("failed to determine Go version: %w", err)
		}
		if err := g.installGo(goVersion, envPath); err != nil {
			return "", fmt.Errorf("failed to bootstrap Go: %w", err)
		}
	}

	// Log warning if additional dependencies are specified (not supported without package management)
	if len(additionalDeps) > 0 {
		fmt.Printf("⚠️  Warning: Go language ignoring additional dependencies "+
			"(only uses pre-installed Go runtime): %v\n", additionalDeps)
	}

	return envPath, nil
}

// InstallDependencies does nothing for Go (only uses pre-installed runtime)
func (g *GoLanguage) InstallDependencies(_ string, deps []string) error {
	// Go language uses pre-installed runtime only
	if len(deps) > 0 {
		fmt.Printf(
			"⚠️  Warning: Go language ignoring additional dependencies (only uses pre-installed Go runtime): %v\n",
			deps,
		)
	}
	return nil
}

// isRepositoryInstalled checks if the repository is properly set up in the environment
func (g *GoLanguage) isRepositoryInstalled(envPath, _ string) bool {
	// For simplified implementation, just check if environment directory exists
	_, err := os.Stat(envPath)
	return err == nil
}

// SetupEnvironmentWithRepositoryInit handles Go environment setup assuming repository is already initialized
//
//nolint:revive // function name is part of interface contract
func (g *GoLanguage) SetupEnvironmentWithRepositoryInit(
	cacheDir, version, repoPath string,
	additionalDeps []string,
	repoURLAny any,
) (string, error) {
	// Convert repoURLAny to string if it's not nil
	repoURL := ""
	if repoURLAny != nil {
		if url, ok := repoURLAny.(string); ok {
			repoURL = url
		}
	}

	return g.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// IsEnvironmentInstalled checks if the Go environment is properly installed and functional
func (g *GoLanguage) IsEnvironmentInstalled(envPath, repoPath string) bool {
	return g.isRepositoryInstalled(envPath, repoPath)
}

// CacheAwareSetupEnvironmentWithRepoInfo provides cache-aware environment setup for Go
//
//nolint:revive // function name is part of interface contract
func (g *GoLanguage) CacheAwareSetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
	_ string, // language name parameter (unused)
) (string, error) {
	return g.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// setupSystemGoSymlinks creates symlinks to system Go binaries in the environment
func (g *GoLanguage) setupSystemGoSymlinks(envPath string) error {
	binDir := filepath.Join(envPath, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Find system Go executable
	goExecPath, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("system Go executable not found: %w", err)
	}

	// Create symlink for go executable
	goSymlink := filepath.Join(binDir, "go")
	if err := os.Symlink(goExecPath, goSymlink); err != nil {
		return fmt.Errorf("failed to create go symlink: %w", err)
	}

	// Find and symlink gofmt if available
	if gofmtPath, err := exec.LookPath("gofmt"); err == nil {
		gofmtSymlink := filepath.Join(binDir, "gofmt")
		if err := os.Symlink(gofmtPath, gofmtSymlink); err != nil {
			// Non-fatal error - gofmt is optional
			fmt.Printf("⚠️  Warning: Failed to create gofmt symlink: %v\n", err)
		}
	}

	fmt.Printf("Info: Created symlinks to system Go: %s -> %s\n", goSymlink, goExecPath)
	return nil
}

// CheckHealth checks the health of a Go environment
func (g *GoLanguage) CheckHealth(envPath, _ string) error {
	binPath := filepath.Join(envPath, "bin")
	goExecPath := filepath.Join(binPath, "go")

	// Check if go executable exists in environment
	if _, err := os.Stat(goExecPath); err != nil {
		// If symlink doesn't exist, try to create it
		if err := g.setupSystemGoSymlinks(envPath); err != nil {
			return fmt.Errorf("failed to setup Go symlinks during health check: %w", err)
		}
	}

	// Test if Go is functional
	cmd := exec.Command(goExecPath, "version")
	// Set up proper environment for Go
	cmd.Env = append(os.Environ(), g.getGoEnvironment(envPath)...)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go runtime health check failed: %w", err)
	}

	return nil
}

// getGoEnvironment returns environment variables needed for Go execution
func (g *GoLanguage) getGoEnvironment(envPath string) []string {
	var env []string

	// Set GOCACHE to prevent cache errors
	goCacheDir := filepath.Join(envPath, "gocache")
	if err := os.MkdirAll(goCacheDir, 0o750); err == nil {
		env = append(env, fmt.Sprintf("GOCACHE=%s", goCacheDir))
	}

	// Set GOPATH if needed (optional for Go modules)
	goPath := filepath.Join(envPath, "gopath")
	if err := os.MkdirAll(goPath, 0o750); err == nil {
		env = append(env, fmt.Sprintf("GOPATH=%s", goPath))
	}

	return env
}

// GetEnvPatch returns environment variable patches for Go hook execution
// This matches Python pre-commit's golang.get_env_patch() behavior
func (g *GoLanguage) GetEnvPatch(envPath, version string) map[string]string {
	env := make(map[string]string)

	// Set GOCACHE to prevent cache errors
	goCacheDir := filepath.Join(envPath, "gocache")
	_ = os.MkdirAll(goCacheDir, 0o750)
	env["GOCACHE"] = goCacheDir

	// Set GOPATH
	goPath := filepath.Join(envPath, "gopath")
	_ = os.MkdirAll(goPath, 0o750)
	env["GOPATH"] = goPath

	// For non-system versions, set GOROOT and GOTOOLCHAIN
	if version != language.VersionSystem && version != "" {
		goRoot := filepath.Join(envPath, ".go")
		if _, err := os.Stat(goRoot); err == nil {
			env["GOROOT"] = goRoot
			env["GOTOOLCHAIN"] = "local"
		}
	}

	// Build PATH - include env/bin and potentially env/.go/bin
	binDir := filepath.Join(envPath, "bin")
	pathParts := []string{binDir}

	// Add .go/bin if GOROOT is set
	if goRoot, ok := env["GOROOT"]; ok {
		pathParts = append(pathParts, filepath.Join(goRoot, "bin"))
	}

	if currentPath := os.Getenv("PATH"); currentPath != "" {
		pathParts = append(pathParts, currentPath)
	}
	env["PATH"] = strings.Join(pathParts, string(os.PathListSeparator))

	return env
}
