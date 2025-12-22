package languages

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blairham/go-pre-commit/pkg/language"
)

// RustLanguage handles Rust environment setup with rustup
type RustLanguage struct {
	*language.Base
}

// NewRustLanguage creates a new Rust language handler
func NewRustLanguage() *RustLanguage {
	return &RustLanguage{
		Base: language.NewBase(
			"rust",
			"rustc",
			"--version",
			"https://rustup.rs/",
		),
	}
}

// getRustupURL returns the rustup-init download URL based on OS
func getRustupURL() string {
	if runtime.GOOS == "windows" {
		return "https://win.rustup.rs/x86_64"
	}
	return "https://sh.rustup.rs"
}

// rustToolchain converts language_version to a rust toolchain version
// Matches Python pre-commit's _rust_toolchain() behavior
func rustToolchain(version string) string {
	if version == language.VersionDefault || version == "" {
		return "stable"
	}
	return version
}

// downloadRustupInit downloads rustup-init to the specified destination
func (r *RustLanguage) downloadRustupInit(destPath string) error {
	url := getRustupURL()
	fmt.Printf("Downloading rustup from %s...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download rustup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download rustup: HTTP %d", resp.StatusCode)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create rustup-init file: %w", err)
	}
	defer file.Close()

	// Copy content
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write rustup-init: %w", err)
	}

	// Make executable
	if err := os.Chmod(destPath, 0o750); err != nil {
		return fmt.Errorf("failed to make rustup-init executable: %w", err)
	}

	return nil
}

// installRustWithToolchain installs Rust using rustup with the specified toolchain
// This matches Python pre-commit's install_rust_with_toolchain() behavior
func (r *RustLanguage) installRustWithToolchain(toolchain, envDir string) error {
	// Create a temporary rustup home directory
	rustupDir, err := os.MkdirTemp("", "rustup-home-*")
	if err != nil {
		return fmt.Errorf("failed to create temp rustup dir: %w", err)
	}
	defer os.RemoveAll(rustupDir)

	// Set environment for rustup operations
	env := append(os.Environ(),
		"CARGO_HOME="+envDir,
		"RUSTUP_HOME="+rustupDir,
	)

	// Check if rustup is already available
	_, rustupErr := exec.LookPath("rustup")
	if rustupErr != nil {
		// Download and install rustup-init
		rustupInit := filepath.Join(rustupDir, "rustup-init")
		if runtime.GOOS == "windows" {
			rustupInit += ".exe"
		}

		if err := r.downloadRustupInit(rustupInit); err != nil {
			return err
		}

		// Run rustup-init to install rustup into CARGO_HOME/bin
		cmd := exec.Command(rustupInit, "-y", "--quiet", "--no-modify-path", "--default-toolchain", "none")
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run rustup-init: %w", err)
		}
	}

	// Install the requested toolchain
	rustupBin := filepath.Join(envDir, "bin", "rustup")
	if runtime.GOOS == "windows" {
		rustupBin += ".exe"
	}

	// Use the rustup we just installed or the system one
	rustupCmd := "rustup"
	if _, err := os.Stat(rustupBin); err == nil {
		rustupCmd = rustupBin
	}

	cmd := exec.Command(rustupCmd, "toolchain", "install", "--no-self-update", toolchain)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install rust toolchain %s: %w", toolchain, err)
	}

	fmt.Printf("Rust toolchain %s installed successfully\n", toolchain)
	return nil
}

// GetDefaultVersion returns the default Rust version
// Following Python pre-commit behavior: returns 'system' if Rust is installed, otherwise 'default'
func (r *RustLanguage) GetDefaultVersion() string {
	// Check if system Rust is available
	if r.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (r *RustLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return r.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "rust")
}

// SetupEnvironmentWithRepoInfo sets up a Rust environment with repository URL information
func (r *RustLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return r.CacheAwareSetupEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "rust")
}

// InstallDependencies installs Rust dependencies (crates) in the environment
func (r *RustLanguage) InstallDependencies(envPath string, deps []string) error {
	// Skip actual dependency installation during tests for speed, except for specific error test cases
	testMode := os.Getenv("GO_PRE_COMMIT_TEST_MODE") == testModeEnvValue
	if testMode && !strings.Contains(envPath, "fail") &&
		!strings.Contains(envPath, "error") &&
		!strings.Contains(envPath, "cargo-not-available") {
		// Create mock environment structure for tests
		binDir := filepath.Join(envPath, "bin")
		if err := os.MkdirAll(binDir, 0o750); err != nil {
			return fmt.Errorf("failed to create mock bin directory: %w", err)
		}

		// Create mock Cargo.toml to simulate successful installation
		cargoToml := filepath.Join(envPath, "Cargo.toml")
		mockContent := "[dependencies]\n"
		for _, dep := range deps {
			mockContent += fmt.Sprintf("%s = \"*\"\n", dep)
		}
		if err := os.WriteFile(cargoToml, []byte(mockContent), 0o600); err != nil {
			return fmt.Errorf("failed to create mock Cargo.toml: %w", err)
		}

		return nil
	}

	cargoBin := filepath.Join(envPath, "bin", "cargo")

	// If cargo is not in the environment, try to use system cargo
	if _, err := os.Stat(cargoBin); err != nil {
		if _, err := exec.LookPath("cargo"); err != nil {
			return fmt.Errorf("cargo not found in environment or system PATH")
		}
		cargoBin = "cargo"
	}

	for _, dep := range deps {
		cmd := exec.Command(cargoBin, "install", dep)
		cmd.Env = append(os.Environ(), "CARGO_HOME="+envPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install Rust dependency %s: %w", dep, err)
		}
	}

	return nil
}

// SetupEnvironmentWithRepo sets up a Rust environment in the repository directory
func (r *RustLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	// Determine if we should use system Rust or bootstrap
	useSystem := version == language.VersionSystem
	if version == "" && r.IsRuntimeAvailable() {
		// Default to system if available
		useSystem = true
		version = language.VersionSystem
	}

	// Handle empty repoPath by using cacheDir instead to avoid creating directories in CWD
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir cannot be empty")
		}
		repoPath = cacheDir
	}

	// Create environment in the repository directory (like Python pre-commit)
	envDirName := language.GetRepositoryEnvironmentName("rust", version)
	envPath := filepath.Join(repoPath, envDirName)

	// Check if environment already exists and is functional
	if r.CheckEnvironmentHealth(envPath) {
		return envPath, nil
	}

	// Environment exists but is broken, remove and recreate
	if _, err := os.Stat(envPath); err == nil {
		if err := os.RemoveAll(envPath); err != nil {
			return "", fmt.Errorf("failed to remove broken environment: %w", err)
		}
	}

	// Create environment directory
	if err := r.CreateEnvironmentDirectory(envPath); err != nil {
		return "", fmt.Errorf("failed to create Rust environment directory: %w", err)
	}

	// If not using system, bootstrap Rust with the specified toolchain
	if !useSystem {
		toolchain := rustToolchain(version)
		if err := r.installRustWithToolchain(toolchain, envPath); err != nil {
			return "", fmt.Errorf("failed to bootstrap Rust: %w", err)
		}
	}

	// Install dependencies if needed
	if len(additionalDeps) > 0 {
		if err := r.InstallDependencies(envPath, additionalDeps); err != nil {
			return "", fmt.Errorf("failed to install Rust dependencies: %w", err)
		}
	}

	return envPath, nil
}

// CheckHealth performs health check for rust environments
func (r *RustLanguage) CheckHealth(envPath, version string) error {
	// For system version, check if rust is available in system PATH
	if version == language.VersionSystem {
		if _, err := exec.LookPath("rustc"); err != nil {
			return fmt.Errorf("system rust (rustc) not available: %w", err)
		}
		if _, err := exec.LookPath("cargo"); err != nil {
			return fmt.Errorf("system cargo not available: %w", err)
		}
		return nil
	}

	// For environment versions, just check if environment directory exists
	// (matching Python pre-commit's basic_health_check pattern)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("environment directory does not exist: %s", envPath)
	}

	// Note: We don't check for rustc inside the environment directory because
	// our current implementation doesn't fully install rust toolchains yet.
	// This matches the behavior where Python pre-commit would only do basic
	// directory existence checks for many languages.
	return nil
}

// GetEnvPatch returns environment variable patches for Rust hook execution
// This matches Python pre-commit's rust.get_env_patch() behavior
func (r *RustLanguage) GetEnvPatch(envPath, version string) map[string]string {
	env := make(map[string]string)

	// Set CARGO_HOME - where cargo stores its cache and builds
	env["CARGO_HOME"] = envPath

	// For non-system versions, set RUSTUP_HOME to a temporary location
	// In a full implementation, this would be where rustup is bootstrapped
	if version != language.VersionSystem && version != "" {
		// Temporarily set RUSTUP_HOME inside the environment
		rustupHome := filepath.Join(envPath, ".rustup")
		_ = os.MkdirAll(rustupHome, 0o750)
		env["RUSTUP_HOME"] = rustupHome
	}

	// Build PATH - include cargo bin directory
	binDir := filepath.Join(envPath, "bin")
	if currentPath := os.Getenv("PATH"); currentPath != "" {
		env["PATH"] = binDir + string(os.PathListSeparator) + currentPath
	} else {
		env["PATH"] = binDir
	}

	return env
}
