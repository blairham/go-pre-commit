package languages

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blairham/go-pre-commit/pkg/download/pkgmgr"
	"github.com/blairham/go-pre-commit/pkg/language"
)

// RubyLanguage handles Ruby environment setup
type RubyLanguage struct {
	*language.Base
}

// NewRubyLanguage creates a new Ruby language handler
func NewRubyLanguage() *RubyLanguage {
	return &RubyLanguage{
		Base: language.NewBase("ruby", "ruby", "--version", "https://www.ruby-lang.org/"),
	}
}

// GetDefaultVersion returns the default Ruby version
// Following Python pre-commit behavior: returns 'system' if Ruby is installed, otherwise 'default'
func (r *RubyLanguage) GetDefaultVersion() string {
	// Check if system Ruby is available
	if r.IsRuntimeAvailable() {
		return language.VersionSystem
	}
	return language.VersionDefault
}

// PreInitializeEnvironmentWithRepoInfo shows the initialization message and creates the environment directory
func (r *RubyLanguage) PreInitializeEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) error {
	return r.CacheAwarePreInitializeEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "ruby")
}

// SetupEnvironmentWithRepoInfo sets up a Ruby environment with repository URL information
func (r *RubyLanguage) SetupEnvironmentWithRepoInfo(
	cacheDir, version, repoPath, repoURL string,
	additionalDeps []string,
) (string, error) {
	return r.CacheAwareSetupEnvironmentWithRepoInfo(
		cacheDir, version, repoPath, repoURL, additionalDeps, "ruby")
}

// InstallDependencies installs Ruby gems
func (r *RubyLanguage) InstallDependencies(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Create manifest for Ruby package
	manifest := &pkgmgr.Manifest{
		Name:         "precommit_env",
		Version:      "1.0.0",
		Dependencies: deps,
		ManifestType: pkgmgr.Ruby,
	}

	// Create manifest
	if err := r.PackageManager.CreateManifest(envPath, manifest); err != nil {
		return fmt.Errorf("failed to create Ruby package manifest: %w", err)
	}

	// Run package manager command (install gems using bundle install)
	if err := r.runBundleInstall(envPath); err != nil {
		return fmt.Errorf("failed to install Ruby gems: %w", err)
	}

	return nil
}

// CheckEnvironmentHealth checks if the Ruby environment is healthy
func (r *RubyLanguage) CheckEnvironmentHealth(envPath string) bool {
	// Check base health first
	if err := r.CheckHealth(envPath); err != nil {
		return false
	}

	// Check if bundler is available (if Gemfile exists) using package manager utilities
	if r.PackageManager.CheckManifestExists(envPath, pkgmgr.Ruby) {
		// Gemfile exists, check if bundler can resolve dependencies
		gemfilePath := filepath.Join(envPath, "Gemfile")
		cmd := exec.Command("bundle", "check")
		cmd.Dir = envPath
		cmd.Env = append(os.Environ(), "BUNDLE_GEMFILE="+gemfilePath)

		if err := cmd.Run(); err != nil {
			return false
		}
	}

	return true
}

// SetupEnvironmentWithRepo sets up a Ruby environment in the repository directory
// Following Python pre-commit's approach: creates rbenv-style directory with isolated gems
func (r *RubyLanguage) SetupEnvironmentWithRepo(
	cacheDir, _ /* version */, repoPath, repoURL string, // Added repoURL
	additionalDeps []string,
) (string, error) {
	envPath, err := r.determineEnvironmentPath(cacheDir, repoPath)
	if err != nil {
		return "", err
	}

	// Check if environment already exists and has the repository installed
	if r.isRepositoryInstalled(envPath) {
		return envPath, nil
	}

	// Environment exists but might be broken, remove and recreate
	if err := r.removeExistingEnvironment(envPath); err != nil {
		return "", err
	}

	// Show installation progress like Python pre-commit
	r.showInstallationProgress(repoURL)

	// Create environment directory structure
	if err := r.createEnvironmentStructure(envPath); err != nil {
		return "", err
	}

	// Install dependencies from repository
	if err := r.installRepositoryDependencies(envPath, repoPath, additionalDeps); err != nil {
		return "", err
	}

	// Create state files to match Python pre-commit's environment detection
	if err := r.createRubyStateFiles(envPath, additionalDeps); err != nil {
		return "", fmt.Errorf("failed to create state files: %w", err)
	}

	return envPath, nil
}

// determineEnvironmentPath determines the correct path for the Ruby environment
func (r *RubyLanguage) determineEnvironmentPath(cacheDir, repoPath string) (string, error) {
	version := language.VersionDefault // Use the constant
	envDirName := language.GetRepositoryEnvironmentName("ruby", version)

	// Prevent creating environment directory in CWD if repoPath is empty
	if repoPath == "" {
		if cacheDir == "" {
			return "", fmt.Errorf("both repoPath and cacheDir are empty, cannot create Ruby environment")
		}
		// Use cache directory when repoPath is empty
		return filepath.Join(cacheDir, "ruby-"+envDirName), nil
	}
	return filepath.Join(repoPath, envDirName), nil
}

// removeExistingEnvironment removes an existing environment if it exists
func (r *RubyLanguage) removeExistingEnvironment(envPath string) error {
	if _, err := os.Stat(envPath); err == nil {
		if err := os.RemoveAll(envPath); err != nil {
			return fmt.Errorf("failed to remove broken environment: %w", err)
		}
	}
	return nil
}

// showInstallationProgress displays installation progress messages
func (r *RubyLanguage) showInstallationProgress(repoURL string) {
	fmt.Printf("[INFO] Installing environment for %s.\n", repoURL)
	fmt.Printf("[INFO] Once installed this environment will be reused.\n")
	fmt.Printf("[INFO] This may take a few minutes...\n")
}

// createEnvironmentStructure creates the basic environment directory structure
func (r *RubyLanguage) createEnvironmentStructure(envPath string) error {
	// Create environment directory and install state files (DRY)
	if err := r.SetupEnvironmentDirectory(envPath, nil); err != nil {
		return err
	}

	// Create gems subdirectory (this is where GEM_HOME will point)
	gemsDir := filepath.Join(envPath, "gems")
	if err := os.MkdirAll(gemsDir, 0o750); err != nil {
		return fmt.Errorf("failed to create gems directory: %w", err)
	}

	// Create gems/bin subdirectory (this is where gem executables go)
	gemsBinDir := filepath.Join(gemsDir, "bin")
	if err := os.MkdirAll(gemsBinDir, 0o750); err != nil {
		return fmt.Errorf("failed to create gems/bin directory: %w", err)
	}

	return nil
}

// installRepositoryDependencies installs dependencies based on what's present in the repository
func (r *RubyLanguage) installRepositoryDependencies(envPath, repoPath string, additionalDeps []string) error {
	// 1. If there's a Gemfile, use bundle install (matching Python pre-commit behavior)
	gemfilePath := filepath.Join(repoPath, "Gemfile")
	if _, err := os.Stat(gemfilePath); err == nil {
		if err := r.installGemsUsingBundle(envPath, repoPath); err != nil {
			return fmt.Errorf("failed to install gems using bundle: %w", err)
		}
	}

	// 2. If there's a .gemspec, build and install the gem
	gemspecPath := filepath.Join(repoPath, "*.gemspec")
	if gemspecs, globErr := filepath.Glob(gemspecPath); globErr == nil && len(gemspecs) > 0 {
		if err := r.buildAndInstallGem(envPath, repoPath); err != nil {
			return fmt.Errorf("failed to build and install gem: %w", err)
		}
	}

	// Install additional dependencies using gem install (like Python pre-commit)
	if len(additionalDeps) > 0 {
		if err := r.installGemsDirectly(envPath, additionalDeps); err != nil {
			return fmt.Errorf("failed to install Ruby gems: %w", err)
		}
	}

	return nil
}

// installGemsDirectly installs gems directly using gem install with isolated GEM_HOME
// This matches Python pre-commit's approach of installing gems into an isolated directory
func (r *RubyLanguage) installGemsDirectly(envPath string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Skip actual gem installation during tests for speed
	gemsDir := filepath.Join(envPath, "gems")
	gemsBinDir := filepath.Join(gemsDir, "bin")

	// Prepare gem install command with isolation flags (matching Python pre-commit)
	args := []string{
		"install",
		"--no-document",
		"--no-format-executable",
		"--no-user-install",
		"--install-dir", gemsDir,
		"--bindir", gemsBinDir,
	}
	args = append(args, deps...)

	cmd := exec.Command("gem", args...)
	cmd.Dir = envPath

	// Set environment variables for isolation (matching Python pre-commit)
	env := append(os.Environ(),
		"GEM_HOME="+gemsDir,
		"GEM_PATH=",
		"BUNDLE_IGNORE_CONFIG=1",
	)
	cmd.Env = env

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install gems %v: %w\nOutput: %s", deps, err, output)
	}

	return nil
}

// installGemsUsingBundle installs gems using bundle install for repositories with Gemfiles
// This uses bundle but installs to our isolated gems directory
func (r *RubyLanguage) installGemsUsingBundle(envPath, repoPath string) error {
	gemsDir := filepath.Join(envPath, "gems")
	gemfilePath := filepath.Join(repoPath, "Gemfile")

	// Use bundle install but redirect to our isolated gems directory
	cmd := exec.Command("bundle", "install", "--path", gemsDir)
	cmd.Dir = repoPath

	// Set environment variables for isolation
	env := append(os.Environ(),
		"BUNDLE_GEMFILE="+gemfilePath,
		"GEM_HOME="+gemsDir,
		"GEM_PATH=",
		"BUNDLE_IGNORE_CONFIG=1",
	)
	cmd.Env = env

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install gems using bundle: %w\nOutput: %s", err, output)
	}

	return nil
}

// buildAndInstallGem builds and installs a gem from a .gemspec file in the repository
// This matches Python pre-commit's behavior when a gem is present in the repository
func (r *RubyLanguage) buildAndInstallGem(envPath, repoPath string) error {
	gemsDir := filepath.Join(envPath, "gems")
	gemsBinDir := filepath.Join(gemsDir, "bin")

	// First, build the gem from .gemspec files
	gemspecPath := filepath.Join(repoPath, "*.gemspec")
	gemspecs, err := filepath.Glob(gemspecPath)
	if err != nil || len(gemspecs) == 0 {
		return fmt.Errorf("no .gemspec files found in repository")
	}

	// Change to repository directory for gem build
	buildCmd := exec.Command("gem", "build")
	buildCmd.Args = append(buildCmd.Args, gemspecs...)
	buildCmd.Dir = repoPath

	if output, buildErr := buildCmd.CombinedOutput(); buildErr != nil {
		return fmt.Errorf("failed to build gem: %w\nOutput: %s", buildErr, output)
	}

	// Find the built .gem files
	gemPath := filepath.Join(repoPath, "*.gem")
	gems, err := filepath.Glob(gemPath)
	if err != nil || len(gems) == 0 {
		return fmt.Errorf("no .gem files found after build")
	}

	// Install the built gem(s) with the same isolation flags
	args := []string{
		"install",
		"--no-document",
		"--no-format-executable",
		"--no-user-install",
		"--install-dir", gemsDir,
		"--bindir", gemsBinDir,
	}
	args = append(args, gems...)

	installCmd := exec.Command("gem", args...)
	installCmd.Dir = repoPath

	// Set environment variables for isolation
	env := append(os.Environ(),
		"GEM_HOME="+gemsDir,
		"GEM_PATH=",
		"BUNDLE_IGNORE_CONFIG=1",
	)
	installCmd.Env = env

	if output, err := installCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to install built gem: %w\nOutput: %s", err, output)
	}

	return nil
}

// GetRubyEnvironmentVariables returns environment variables for running Ruby hooks
// This matches Python pre-commit's environment setup with GEM_HOME, GEM_PATH, etc.
func (r *RubyLanguage) GetRubyEnvironmentVariables(envPath string) []string {
	gemsDir := filepath.Join(envPath, "gems")
	gemsBinDir := filepath.Join(gemsDir, "bin")

	env := []string{
		"GEM_HOME=" + gemsDir,
		"GEM_PATH=",              // Clear GEM_PATH for isolation
		"BUNDLE_IGNORE_CONFIG=1", // Ignore bundler configuration
	}

	// Add gems/bin to PATH
	if currentPath := os.Getenv("PATH"); currentPath != "" {
		env = append(env, "PATH="+gemsBinDir+string(os.PathListSeparator)+currentPath)
	} else {
		env = append(env, "PATH="+gemsBinDir)
	}

	return env
}

// createRubyStateFiles creates state files to track Ruby environment installation
// Similar to Python's .install_state_v1 and .install_state_v2 files
func (r *RubyLanguage) createRubyStateFiles(envPath string, additionalDeps []string) error {
	// Create .ruby_install_state with JSON containing additional dependencies
	state := map[string][]string{
		"additional_dependencies": additionalDeps,
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal Ruby state JSON: %w", err)
	}

	// Write .ruby_install_state atomically (like Python pre-commit does)
	stateFile := filepath.Join(envPath, ".ruby_install_state")
	stagingFile := stateFile + "staging"

	if err := os.WriteFile(stagingFile, stateJSON, 0o600); err != nil {
		return fmt.Errorf("failed to write staging Ruby state file: %w", err)
	}

	if err := os.Rename(stagingFile, stateFile); err != nil {
		return fmt.Errorf("failed to move Ruby state file into place: %w", err)
	}

	return nil
}

// CheckHealth checks if the Ruby environment is healthy
// Ruby uses system Ruby with isolated gem directories, so we don't check for
// a ruby executable in the environment directory like other languages
func (r *RubyLanguage) CheckHealth(envPath string) error {
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return fmt.Errorf("ruby environment directory does not exist: %s", envPath)
	}

	// Check if system Ruby is available
	if !r.IsRuntimeAvailable() {
		return fmt.Errorf("system ruby not available for ruby environment")
	}

	// Check if gems directory exists (this is where the isolated gems are)
	gemsDir := filepath.Join(envPath, "gems")
	if _, err := os.Stat(gemsDir); err != nil {
		return fmt.Errorf("ruby gems directory does not exist: %s", gemsDir)
	}

	return nil
}

// isRepositoryInstalled checks if a repository is already installed in the Ruby environment
func (r *RubyLanguage) isRepositoryInstalled(envPath string) bool {
	// First check if state file exists (matching Python pre-commit's logic)
	stateFile := filepath.Join(envPath, ".ruby_install_state")
	if _, err := os.Stat(stateFile); err == nil {
		// State file exists, environment is installed
		return true
	}

	// Check if gems directory exists and has content
	gemsDir := filepath.Join(envPath, "gems")
	if _, err := os.Stat(gemsDir); err != nil {
		return false
	}

	// Check if any gems are installed
	entries, err := os.ReadDir(gemsDir)
	if err != nil {
		return false
	}

	// If gems directory has content (more than just bin directory), environment is set up
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != "bin" {
			return true // Found a gem directory
		}
	}

	return false
}

// runBundleInstall runs bundle install to install Ruby gems from the Gemfile
func (r *RubyLanguage) runBundleInstall(envPath string) error {
	gemfilePath := filepath.Join(envPath, "Gemfile")
	cmd := exec.Command("bundle", "install", "--path", "gems")
	cmd.Dir = envPath
	cmd.Env = append(os.Environ(), "BUNDLE_GEMFILE="+gemfilePath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bundle install failed: %w", err)
	}

	return nil
}
