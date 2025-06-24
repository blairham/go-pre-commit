package pyenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// installMacOS handles Python installation on macOS using PKG files
func (m *Manager) installMacOS(version, downloadPath, installPath string) error {
	// On macOS, we'll extract the PKG and copy the Python framework to our managed location
	// This mimics how pyenv installs Python from official Python.org installers

	// Create a temporary directory for extraction
	tempDir := filepath.Join(m.CacheDir, "temp_"+version)
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			// Cleanup errors are not critical, log and continue
			fmt.Printf("⚠️  Warning: failed to cleanup temp directory: %v\n", err)
		}
	}()

	fmt.Printf("Extracting Python %s installer...\n", version)

	// Extract the PKG using xar (standard on macOS)
	cmd := exec.Command("xar", "-xf", downloadPath, "-C", tempDir)
	if err := cmd.Run(); err != nil {
		// Fall back to pkgutil if xar fails
		cmd = exec.Command("pkgutil", "--expand", downloadPath, tempDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract PKG: %w", err)
		}
	}

	// Find and extract the Python framework payload
	// The structure varies by Python version, so we need to be flexible
	if err := m.extractPythonFramework(tempDir, installPath, version); err != nil {
		return fmt.Errorf("failed to extract Python framework: %w", err)
	}

	// Create symlinks and wrapper scripts for common executables
	if err := m.createMacOSSymlinks(installPath, version); err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	// Fix the dynamic library paths for the extracted Python
	if err := m.fixMacOSLibraryPaths(installPath, version); err != nil {
		return fmt.Errorf("failed to fix library paths: %w", err)
	}

	return nil
}

// extractPythonFramework finds and extracts the Python framework from the PKG
func (m *Manager) extractPythonFramework(tempDir, installPath, version string) error {
	// Look for Python framework payload
	frameworkPaths := []string{
		filepath.Join(tempDir, "Python_Framework.pkg", "Payload"),
		filepath.Join(tempDir, "PythonFramework-"+version+".pkg", "Payload"),
		filepath.Join(tempDir, "PythonFramework.pkg", "Payload"),
	}

	var payloadPath string
	for _, path := range frameworkPaths {
		if _, err := os.Stat(path); err == nil {
			payloadPath = path
			break
		}
	}

	if payloadPath == "" {
		return fmt.Errorf("could not find Python framework payload in PKG")
	}

	// Extract the payload using tar/cpio
	cmd := exec.Command("tar", "-xf", payloadPath, "-C", installPath)
	if err := cmd.Run(); err != nil {
		// Try with cpio if tar fails
		cmd = exec.Command("sh", "-c", fmt.Sprintf("cd %s && cat %s | cpio -i", installPath, payloadPath))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to extract payload: %w", err)
		}
	}

	return nil
}

// createMacOSSymlinks creates symlinks for Python executables on macOS
func (m *Manager) createMacOSSymlinks(installPath, version string) error {
	binDir := filepath.Join(installPath, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	versionParts := strings.Split(version, ".")
	if len(versionParts) < 2 {
		return fmt.Errorf("invalid version format: %s", version)
	}
	majorMinor := versionParts[0] + "." + versionParts[1]

	// Find the actual Python executable in the extracted framework structure
	// The PKG extraction creates a direct framework structure in installPath
	possiblePythonPaths := []string{
		// Direct framework structure (what we actually have)
		filepath.Join(installPath, "Versions", majorMinor, "bin", "python"+majorMinor),
		// Alternative with Python.framework wrapper
		filepath.Join(installPath, "Python.framework", "Versions", majorMinor, "bin", "python"+majorMinor),
		// Library/Frameworks structure
		filepath.Join(
			installPath,
			"Library",
			"Frameworks",
			"Python.framework",
			"Versions",
			majorMinor,
			"bin",
			"python"+majorMinor,
		),
	}

	var pythonExe string

	for _, path := range possiblePythonPaths {
		if _, err := os.Stat(path); err == nil {
			pythonExe = path
			break
		}
	}

	if pythonExe == "" {
		return fmt.Errorf("python executable not found in any expected location under %s", installPath)
	}

	// Create symlinks
	pipExe := filepath.Join(filepath.Dir(pythonExe), "pip"+majorMinor)
	// Check if pip exists at the expected location
	if _, err := os.Stat(pipExe); err != nil {
		// Try alternative pip location
		pipExe = filepath.Join(filepath.Dir(pythonExe), "pip3")
		if _, err := os.Stat(pipExe); err != nil {
			// If pip doesn't exist, just create python symlinks
			pipExe = ""
		}
	}

	// Create symlinks and wrapper scripts
	symlinks := map[string]string{
		"python":  pythonExe,
		"python3": pythonExe,
	}

	// Add pip symlinks if pip exists
	if pipExe != "" {
		symlinks["pip"] = pipExe
		symlinks["pip3"] = pipExe
	}

	for linkName, target := range symlinks {
		linkPath := filepath.Join(binDir, linkName)
		if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("⚠️  Warning: failed to remove existing symlink %s: %v\n", linkPath, err)
		}

		// Create regular symlinks - we'll handle environment in the language manager
		if err := os.Symlink(target, linkPath); err != nil {
			fmt.Printf("⚠️  Warning: failed to create symlink %s -> %s: %v\n", linkPath, target, err)
		}
	}

	return nil
}

// installLinuxPrebuilt handles prebuilt Python installation on Linux
func (m *Manager) installLinuxPrebuilt(_, _, _ string) error {
	// For Linux, we typically don't have prebuilt binaries from python.org
	// This method would be used if we had portable Python builds
	return fmt.Errorf("prebuilt Python binaries not available for Linux")
}

// installLinuxFromSource handles Python installation from source on Linux
func (m *Manager) installLinuxFromSource(version, downloadPath, installPath string) error {
	// This mimics pyenv's python-build process
	fmt.Printf("Building Python %s from source...\n", version)

	// Create a temporary directory for building
	tempDir := filepath.Join(m.CacheDir, "build_"+version)
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Printf("⚠️  Warning: failed to clean up temporary directory: %v\n", err)
		}
	}()

	// Extract the source archive
	fmt.Println("Extracting source code...")
	cmd := exec.Command("tar", "-xzf", downloadPath, "-C", tempDir, "--strip-components=1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract Python source: %w", err)
	}

	// Check for required build dependencies
	if err := m.checkLinuxBuildDeps(); err != nil {
		fmt.Printf("⚠️  Warning: Some build dependencies may be missing: %v\n", err)
	}

	// Configure the build with optimizations (like pyenv)
	fmt.Println("Configuring build...")
	configureArgs := []string{
		"./configure",
		"--prefix=" + installPath,
		"--enable-optimizations",
		"--enable-shared",
		"--with-ensurepip=install",
		"--enable-loadable-sqlite-extensions",
	}

	// Add additional configure options based on system
	if m.hasOpenSSL() {
		configureArgs = append(configureArgs, "--with-openssl=/usr/local/ssl")
	}

	cmd = exec.Command(configureArgs[0], configureArgs[1:]...)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(),
		"LDFLAGS=-Wl,-rpath,"+installPath+"/lib",
		"CPPFLAGS=-I"+installPath+"/include",
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure Python build: %w", err)
	}

	// Build Python with multiple cores
	fmt.Printf("Building Python (using %d cores)...\n", runtime.NumCPU())
	cmd = exec.Command("make", "-j", fmt.Sprintf("%d", runtime.NumCPU()))
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Python: %w", err)
	}

	// Install Python
	if os.Getenv("DEBUG") != "" || os.Getenv("VERBOSE") != "" {
		fmt.Println("Installing Python...")
	}
	cmd = exec.Command("make", "install")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Python: %w", err)
	}

	// Verify the installation
	pythonExe := filepath.Join(installPath, "bin", "python3")
	if _, err := os.Stat(pythonExe); err != nil {
		return fmt.Errorf("python executable not found after installation: %w", err)
	}

	return nil
}

// checkLinuxBuildDeps checks for required build dependencies on Linux
func (m *Manager) checkLinuxBuildDeps() error {
	requiredCmds := []string{"gcc", "make", "pkg-config"}
	var missing []string

	for _, cmd := range requiredCmds {
		if _, err := exec.LookPath(cmd); err != nil {
			missing = append(missing, cmd)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing build dependencies: %s", strings.Join(missing, ", "))
	}

	return nil
}

// hasOpenSSL checks if OpenSSL development headers are available
func (m *Manager) hasOpenSSL() bool {
	// Check for common OpenSSL header locations
	locations := []string{
		"/usr/include/openssl/ssl.h",
		"/usr/local/include/openssl/ssl.h",
		"/usr/local/ssl/include/openssl/ssl.h",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return true
		}
	}

	return false
}

// installWindows handles Python installation on Windows using EXE installers
func (m *Manager) installWindows(_, downloadPath, installPath string) error {
	// Windows Python installers can be run with /quiet and /targetdir options
	cmd := exec.Command(downloadPath, "/quiet", "/targetdir="+installPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Python on Windows: %w", err)
	}

	return nil
}

// findExecutable searches for an executable in PATH
func (m *Manager) findExecutable(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("executable %s not found in PATH: %w", name, err)
	}
	return path, nil
}

// VerifyInstallation verifies that a Python installation is working
func (m *Manager) VerifyInstallation(version string) error {
	pythonPath := m.GetPythonExecutable(version)

	// Check if the executable exists
	if _, err := os.Stat(pythonPath); err != nil {
		return fmt.Errorf("python executable not found at %s: %w", pythonPath, err)
	}

	// Try to run Python with --version
	cmd := exec.Command(pythonPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to run Python --version: %w", err)
	}

	// Verify the version output
	versionOutput := strings.TrimSpace(string(output))
	if !strings.Contains(versionOutput, version) {
		return fmt.Errorf("python version mismatch: expected %s, got %s", version, versionOutput)
	}

	return nil
}

// GetPythonVersion returns the version of a Python executable
func (m *Manager) GetPythonVersion(pythonPath string) (string, error) {
	cmd := exec.Command(pythonPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get Python version: %w", err)
	}

	versionOutput := strings.TrimSpace(string(output))
	// Parse "Python 3.12.5" -> "3.12.5"
	if after, ok := strings.CutPrefix(versionOutput, "Python "); ok {
		return after, nil
	}

	return versionOutput, nil
}

// UninstallVersion removes a Python version
func (m *Manager) UninstallVersion(version string) error {
	installPath := m.GetVersionPath(version)

	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return fmt.Errorf("python version %s is not installed", version)
	}

	return os.RemoveAll(installPath)
}

// ListVersions returns both installed and available versions
func (m *Manager) ListVersions() (installed, available []string, err error) {
	installed, err = m.GetInstalledVersions()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get installed versions: %w", err)
	}

	availableReleases, err := m.GetAvailableVersions()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get available versions: %w", err)
	}

	for _, release := range availableReleases {
		available = append(available, release.Version)
	}

	return installed, available, nil
}

// fixMacOSLibraryPaths fixes the hardcoded library paths in the extracted Python executable
func (m *Manager) fixMacOSLibraryPaths(installPath, version string) error {
	majorMinor, err := m.extractMajorMinorVersion(version)
	if err != nil {
		return err
	}

	pythonExe, pythonLib, err := m.locatePythonComponents(installPath, majorMinor)
	if err != nil {
		return err
	}

	if err := m.validateMacOSTools(); err != nil {
		return err
	}

	systemFrameworkPath := "/Library/Frameworks/Python.framework/Versions/" + majorMinor + "/Python"

	fmt.Printf("Fixing library paths for Python %s...\n", version)

	if err := m.fixMainPythonExecutable(pythonExe, pythonLib, systemFrameworkPath); err != nil {
		return err
	}

	if err := m.fixBinDirectoryExecutables(installPath, majorMinor, pythonLib, systemFrameworkPath); err != nil {
		return err
	}

	fmt.Printf("Successfully fixed library paths for Python %s\n", version)
	return nil
}

// extractMajorMinorVersion extracts major.minor version from full version string
func (m *Manager) extractMajorMinorVersion(version string) (string, error) {
	versionParts := strings.Split(version, ".")
	if len(versionParts) < 2 {
		return "", fmt.Errorf("invalid version format: %s", version)
	}
	return versionParts[0] + "." + versionParts[1], nil
}

// locatePythonComponents finds the Python executable and library
func (m *Manager) locatePythonComponents(installPath, majorMinor string) (string, string, error) {
	// Find the Python executable that needs fixing
	pythonExe := filepath.Join(installPath, "Versions", majorMinor, "bin", "python"+majorMinor)
	if _, err := os.Stat(pythonExe); err != nil {
		return "", "", fmt.Errorf("python executable not found at %s", pythonExe)
	}

	// Find the actual Python library in our extracted structure
	pythonLib := filepath.Join(installPath, "Versions", majorMinor, "Python")
	if _, err := os.Stat(pythonLib); err != nil {
		// Try alternative location
		pythonLib = filepath.Join(installPath, "Python")
		if _, err := os.Stat(pythonLib); err != nil {
			return "", "", fmt.Errorf("python library not found at expected locations")
		}
	}

	return pythonExe, pythonLib, nil
}

// validateMacOSTools checks if required macOS tools are available
func (m *Manager) validateMacOSTools() error {
	if _, err := exec.LookPath("install_name_tool"); err != nil {
		return fmt.Errorf("install_name_tool not available: %w", err)
	}
	return nil
}

// fixMainPythonExecutable fixes the main Python executable library paths
func (m *Manager) fixMainPythonExecutable(pythonExe, pythonLib, systemFrameworkPath string) error {
	// Check current library dependencies
	cmd := exec.Command("otool", "-L", pythonExe)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check library dependencies: %w", err)
	}

	outputStr := string(output)
	if strings.Contains(outputStr, systemFrameworkPath) {
		cmd = exec.Command("install_name_tool", "-change", systemFrameworkPath, pythonLib, pythonExe)
		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("failed to fix Python framework path: %w", runErr)
		}
		fmt.Printf("Fixed Python framework path: %s -> %s\n", systemFrameworkPath, pythonLib)
	}
	return nil
}

// fixBinDirectoryExecutables fixes library paths for all executables in bin directory
func (m *Manager) fixBinDirectoryExecutables(installPath, majorMinor, pythonLib, systemFrameworkPath string) error {
	binDir := filepath.Join(installPath, "Versions", majorMinor, "bin")
	files, err := os.ReadDir(binDir)
	if err != nil {
		return fmt.Errorf("failed to read bin directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if err := m.fixExecutableLibraryPath(binDir, file, pythonLib, systemFrameworkPath); err != nil {
			// Log warning but continue with other files for non-critical errors
			fmt.Printf("⚠️  Warning: failed to process %s: %v\n", file.Name(), err)
		}
	}
	return nil
}

// fixExecutableLibraryPath fixes library path for a single executable file
func (m *Manager) fixExecutableLibraryPath(
	binDir string,
	file os.DirEntry,
	pythonLib, systemFrameworkPath string,
) error {
	filePath := filepath.Join(binDir, file.Name())

	// Only process executable files that might link to Python
	info, err := file.Info()
	if err != nil {
		return err // Return the actual error
	}
	if info.Mode()&0o111 == 0 {
		return nil // Skip non-executable files (this is intentional)
	}

	// Check if this executable links to the system Python framework
	cmd := exec.Command("otool", "-L", filePath)
	output, err := cmd.Output()
	if err != nil {
		return err // Return the actual error instead of ignoring it
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, systemFrameworkPath) {
		return nil // Skip files that don't need fixing
	}

	cmd = exec.Command("install_name_tool", "-change", systemFrameworkPath, pythonLib, filePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install_name_tool failed: %w", err)
	}

	fmt.Printf("Fixed library path for %s\n", file.Name())
	return nil
}
