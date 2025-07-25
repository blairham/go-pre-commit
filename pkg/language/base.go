// Package language provides base interfaces and implementations for language environments
package language

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/blairham/go-pre-commit/pkg/download"
	"github.com/blairham/go-pre-commit/pkg/download/pkgmgr"
)

// Global tracking for all language environments to ensure consistent behavior
// across all language instances and prevent duplicate installations
var (
	globalInitializedEnvs  = make(map[string]bool)
	globalInstallingEnvs   = make(map[string]bool)
	globalEnvironmentMutex sync.Mutex
	// globalPythonVersionCache removed as unused
)

const (
	// VersionDefault represents the default language version
	VersionDefault = "default"
	// VersionLatest represents the latest available language version
	VersionLatest = "latest"
	// VersionSystem represents using the system-installed language version
	VersionSystem = "system"

	// OSX represents the macOS operating system identifier
	OSX = "osx"
	// Windows represents the Windows operating system identifier
	Windows = "win"
	// WinNT represents the Windows NT operating system identifier
	WinNT = "winnt"
	// Linux represents the Linux operating system identifier
	Linux = "linux"
	// Darwin represents the macOS/Darwin operating system identifier
	Darwin = "darwin"
	// OSWindows represents the Windows operating system string
	OSWindows = "windows"

	// ARM64 represents the ARM 64-bit architecture identifier
	ARM64 = "arm64"
	// AMD64 represents the AMD 64-bit architecture identifier
	AMD64 = "amd64"

	// ExeExt represents the Windows executable file extension
	ExeExt = ".exe"

	// Python represents the Python language identifier for cache normalization
	Python = "python"
)

// Setup defines the interface for setting up language environments
type Setup interface {
	SetupEnvironmentWithRepo(
		cacheDir, version, repoPath, repoURL string,
		additionalDeps []string,
	) (string, error)
	SetupEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL string,
		additionalDeps []string,
	) (string, error)
	PreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL string,
		additionalDeps []string,
	) error
	InstallDependencies(envPath string, deps []string) error
	IsRuntimeAvailable() bool
	GetExecutableName() string
	GetEnvironmentBinPath(envPath string) string
	CheckHealth(envPath, version string) error
	NeedsEnvironmentSetup() bool
	CheckEnvironmentHealth(envPath string) bool
}

// RepositorySetup extends Setup with repository initialization
type RepositorySetup interface {
	Setup
	// SetupEnvironmentWithRepositoryInit handles both repository initialization and environment setup
	SetupEnvironmentWithRepositoryInit(
		cacheDir, version, repoPath, repoURL, repoRef string,
		additionalDeps []string,
		repositoryManager any, // Using interface{} to avoid circular dependency
	) (string, error)
}

// Installer defines the interface for language-specific installation logic
type Installer interface {
	downloadAndInstall(version, envPath string) error
}

// EnvironmentInstaller defines the interface for language-specific environment creation
type EnvironmentInstaller interface {
	CreateLanguageEnvironment(envPath, version string) error
	IsEnvironmentInstalled(envPath, repoPath string) bool
	GetEnvironmentVersion(version string) (string, error)
	GetEnvironmentPath(repoPath, version string) string
}

// Base provides common functionality for language implementations
type Base struct {
	DownloadManager *download.Manager
	PackageManager  *pkgmgr.Manager
	Name            string
	ExecutableName  string
	VersionFlag     string
	InstallURL      string
}

// NewBase creates a new base language instance
func NewBase(name, executableName, versionFlag, installURL string) *Base {
	return &Base{
		Name:            name,
		ExecutableName:  executableName,
		VersionFlag:     versionFlag,
		InstallURL:      installURL,
		DownloadManager: download.NewManager(),
		PackageManager:  pkgmgr.NewManager(),
	}
}

// IsRuntimeAvailable checks if the language runtime is available in the system
func (bl *Base) IsRuntimeAvailable() bool {
	_, err := exec.LookPath(bl.ExecutableName)
	if err == nil {
		return true
	}

	// Special case for Python: if looking for "python" fails, also try "python3"
	if bl.ExecutableName == "python" {
		_, err := exec.LookPath("python3")
		return err == nil
	}

	return false
}

// GetExecutableName returns the executable name for the language
func (bl *Base) GetExecutableName() string {
	return bl.ExecutableName
}

// GetEnvironmentBinPath returns the bin directory path for the environment
func (bl *Base) GetEnvironmentBinPath(envPath string) string {
	return filepath.Join(envPath, "bin")
}

// CheckEnvironmentHealth checks if an existing environment is functional
func (bl *Base) CheckEnvironmentHealth(envPath string) bool {
	binPath := bl.GetEnvironmentBinPath(envPath)
	execPath := filepath.Join(binPath, bl.ExecutableName)
	if _, err := os.Stat(execPath); err != nil {
		return false
	}

	// Test if the environment is functional
	cmd := exec.Command(execPath, bl.VersionFlag)
	return cmd.Run() == nil
}

// CreateEnvironmentDirectory creates the environment directory
func (bl *Base) CreateEnvironmentDirectory(envPath string) error {
	return os.MkdirAll(envPath, 0o750)
}

// PrintNotFoundMessage prints a message when a language runtime is not found
func (bl *Base) PrintNotFoundMessage() {
	fmt.Printf("[WARNING] %s runtime not found. Please install %s to use %s hooks.\n",
		bl.Name, bl.Name, bl.Name)
	if bl.InstallURL != "" {
		fmt.Printf("Installation instructions: %s\n", bl.InstallURL)
	}
}

// CheckHealth checks the health of a language environment
func (bl *Base) CheckHealth(envPath, _ string) error {
	binPath := bl.GetEnvironmentBinPath(envPath)
	execPath := filepath.Join(binPath, bl.ExecutableName)

	// Adjust for Windows
	if runtime.GOOS == "windows" && !strings.HasSuffix(execPath, ExeExt) {
		execPath += ExeExt
	}

	if _, err := os.Stat(execPath); err != nil {
		return fmt.Errorf("language runtime not found at %s: %w", execPath, err)
	}

	// Test if the environment is functional
	cmd := exec.Command(execPath, bl.VersionFlag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("language runtime health check failed: %w", err)
	}

	return nil
}

// GetName returns the language name
func (bl *Base) GetName() string {
	return bl.Name
}

// GetVersionFlag returns the version flag for the language
func (bl *Base) GetVersionFlag() string {
	return bl.VersionFlag
}

// SetupEnvironment sets up a language environment
func (bl *Base) SetupEnvironment(
	cacheDir, version string,
	additionalDeps []string,
) (string, error) {
	return bl.SetupEnvironmentWithRepo(cacheDir, version, "", "", additionalDeps)
}

// SetupEnvironmentWithRepo sets up a language environment with repository information
func (bl *Base) SetupEnvironmentWithRepo(
	cacheDir, version, _, _ string, // repoPath and repoURL are unused in base implementation
	_ []string, // additionalDeps is unused in base implementation
) (string, error) {
	// This is a basic implementation that can be overridden by specific languages
	envPath := filepath.Join(cacheDir, fmt.Sprintf("%s-%s", bl.Name, version))

	if err := bl.CreateEnvironmentDirectory(envPath); err != nil {
		return "", fmt.Errorf("failed to create environment directory: %w", err)
	}

	return envPath, nil
}

// SetupEnvironmentWithRepoInfo sets up environment with repository information
func (bl *Base) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	// Delegate to the base implementation for now
	return bl.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// PreInitializeEnvironmentWithRepoInfo performs pre-initialization for environment setup
func (bl *Base) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, _, repoPath, _ string, // version and repoURL are unused in base implementation
	_ []string, // additionalDeps is unused in base implementation
) error {
	// Basic pre-initialization - just ensure directories exist
	if cacheDir != "" {
		if err := os.MkdirAll(cacheDir, 0o750); err != nil {
			return fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	if repoPath != "" {
		if err := os.MkdirAll(repoPath, 0o750); err != nil {
			return fmt.Errorf("failed to create repo directory: %w", err)
		}
	}

	return nil
}

// InstallDependencies installs additional dependencies (base implementation)
func (bl *Base) InstallDependencies(_ string, deps []string) error { // envPath is unused in base implementation
	if len(deps) > 0 {
		fmt.Printf("[INFO] %s: Installing additional dependencies: %v\n", bl.Name, deps)
		// This is a base implementation - specific languages should override
	}
	return nil
}

// NeedsEnvironmentSetup returns whether the language needs environment setup
func (bl *Base) NeedsEnvironmentSetup() bool {
	// Most languages need environment setup
	return true
}

// ParseRepoURL parses repository URL from directory name
func ParseRepoURL(dirName string) string {
	// Handle common repository URL patterns
	dirName = strings.TrimPrefix(dirName, "file://")
	dirName = strings.TrimPrefix(dirName, "git://")
	dirName = strings.TrimPrefix(dirName, "https://")
	dirName = strings.TrimPrefix(dirName, "http://")
	dirName = strings.TrimSuffix(dirName, ".git")

	// Convert common patterns
	parts := strings.Split(dirName, "/")
	if len(parts) >= 2 {
		// Handle github.com/user/repo pattern
		if strings.Contains(dirName, "github.com") {
			for i, part := range parts {
				if part == "github.com" && i+2 < len(parts) {
					return fmt.Sprintf("https://github.com/%s/%s", parts[i+1], parts[i+2])
				}
			}
		}
	}

	return dirName
}

// GetGlobalInitializedEnvs returns the global environment tracking map (for testing)
func GetGlobalInitializedEnvs() map[string]bool {
	globalEnvironmentMutex.Lock()
	defer globalEnvironmentMutex.Unlock()

	result := make(map[string]bool)
	maps.Copy(result, globalInitializedEnvs)
	return result
}

// ClearGlobalEnvironmentState clears global environment state (for testing)
func ClearGlobalEnvironmentState() {
	globalEnvironmentMutex.Lock()
	defer globalEnvironmentMutex.Unlock()

	globalInitializedEnvs = make(map[string]bool)
	globalInstallingEnvs = make(map[string]bool)
	// Python version cache was unused and removed
}

// MarkEnvironmentInitialized marks an environment as initialized
func MarkEnvironmentInitialized(envKey string) {
	globalEnvironmentMutex.Lock()
	defer globalEnvironmentMutex.Unlock()

	globalInitializedEnvs[envKey] = true
	delete(globalInstallingEnvs, envKey)
}

// IsEnvironmentInitialized checks if an environment is already initialized
func IsEnvironmentInitialized(envKey string) bool {
	globalEnvironmentMutex.Lock()
	defer globalEnvironmentMutex.Unlock()

	return globalInitializedEnvs[envKey]
}

// MarkEnvironmentInstalling marks an environment as currently installing
func MarkEnvironmentInstalling(envKey string) bool {
	globalEnvironmentMutex.Lock()
	defer globalEnvironmentMutex.Unlock()

	if globalInstallingEnvs[envKey] {
		return false // Already installing
	}

	globalInstallingEnvs[envKey] = true
	return true // Successfully marked as installing
}

// IsEnvironmentInstalling checks if an environment is currently installing
func IsEnvironmentInstalling(envKey string) bool {
	globalEnvironmentMutex.Lock()
	defer globalEnvironmentMutex.Unlock()

	return globalInstallingEnvs[envKey]
}

// GenericSetupEnvironmentWithRepo provides a common implementation for generic languages
func (bl *Base) GenericSetupEnvironmentWithRepo(
	_, version, repoPath string,
	_ []string,
) (string, error) {
	// Use generic naming for environment directory
	envDirName := fmt.Sprintf("%s-%s", strings.ToLower(bl.Name), version)
	if version == VersionSystem || bl.Name == VersionSystem || bl.Name == "script" || bl.Name == "fail" {
		// Languages like system, script, fail don't need separate environments
		return repoPath, nil
	}

	envPath := filepath.Join(repoPath, envDirName)
	if err := os.MkdirAll(envPath, 0o750); err != nil {
		return "", fmt.Errorf("failed to create environment directory: %w", err)
	}
	return envPath, nil
}

// GenericInstallDependencies does nothing for generic languages (no dependencies to install)
func (bl *Base) GenericInstallDependencies(_ string, deps []string) error {
	if len(deps) > 0 {
		fmt.Printf("[WARN] %s language ignoring additional dependencies: %v\n", bl.Name, deps)
	}
	return nil
}

// GenericIsRuntimeAvailable always returns true for generic languages
func (bl *Base) GenericIsRuntimeAvailable() bool {
	return true
}

// GenericCheckHealth always passes for generic languages
func (bl *Base) GenericCheckHealth(envPath, _ string) error {
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("%s environment directory does not exist: %s", bl.Name, envPath)
	}
	return nil
}

// CacheAwarePreInitializeEnvironmentWithRepoInfo provides cache-aware pre-initialization
func (bl *Base) CacheAwarePreInitializeEnvironmentWithRepoInfo(
	_, version, repoPath, repoURL string,
	_ []string, // additionalDeps is unused
	_ string, // languageName is unused
) error {
	// Simplified version - delegate to PreInitializeEnvironmentWithRepoInfo
	return bl.PreInitializeEnvironmentWithRepoInfo("", version, repoPath, repoURL, nil)
}

// CacheAwareSetupEnvironmentWithRepoInfo provides cache-aware environment setup for languages
func (bl *Base) CacheAwareSetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
	_ string, // languageName is unused
) (string, error) {
	// Simplified version - delegate to SetupEnvironmentWithRepo
	return bl.SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL, additionalDeps)
}

// GetRepositoryEnvironmentName returns the standardized environment directory name for a given language and version
func GetRepositoryEnvironmentName(language, version string) string {
	// Normalize language name to lowercase for case-insensitive matching
	lowerLang := strings.ToLower(language)

	// If version is empty, default to "default"
	if version == "" {
		version = VersionDefault
	}

	// Handle language aliases
	switch lowerLang {
	case "nodejs":
		lowerLang = "node_"
	case "golang":
		lowerLang = "go"
	case Python, "python3":
		lowerLang = "py_"
	case ".net":
		lowerLang = "dotnet"
	}

	// Languages that don't need separate environments
	switch lowerLang {
	case "system", "script", "fail", "pygrep":
		return ""
	}

	// For other languages, return standardized name following Python pre-commit pattern
	return fmt.Sprintf("%senv-%s", lowerLang, version)
}

// CreateNormalizedEnvironmentKey creates a normalized key for environment tracking
func CreateNormalizedEnvironmentKey(language, repoURL, envPath string) string {
	return fmt.Sprintf("%s-%s-%s", strings.ToLower(language), repoURL, envPath)
}
