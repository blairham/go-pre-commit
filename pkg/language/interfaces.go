// Package language provides base interfaces and implementations for language environments
package language

// Core defines the core interface for basic language operations
type Core interface {
	// GetName returns the name of the language
	GetName() string

	// GetExecutableName returns the executable name for the language
	GetExecutableName() string

	// IsRuntimeAvailable checks if the language runtime is available
	IsRuntimeAvailable() bool

	// NeedsEnvironmentSetup returns whether the language needs environment setup
	NeedsEnvironmentSetup() bool
}

// EnvironmentManager defines the interface for environment management operations
type EnvironmentManager interface {
	// SetupEnvironment sets up a language environment
	SetupEnvironment(cacheDir, version string, additionalDeps []string) (string, error)

	// SetupEnvironmentWithRepo sets up environment with repository context
	SetupEnvironmentWithRepo(cacheDir, version, repoPath, repoURL string, additionalDeps []string) (string, error)

	// SetupEnvironmentWithRepoInfo sets up environment with repository URL information
	SetupEnvironmentWithRepoInfo(cacheDir, version, repoPath, repoURL string, additionalDeps []string) (string, error)

	// PreInitializeEnvironmentWithRepoInfo performs pre-initialization for environment setup
	PreInitializeEnvironmentWithRepoInfo(cacheDir, version, repoPath, repoURL string, additionalDeps []string) error

	// GetEnvironmentBinPath returns the bin path for the environment
	GetEnvironmentBinPath(envPath string) string
}

// HealthChecker defines the interface for environment health operations
type HealthChecker interface {
	// CheckEnvironmentHealth checks if an existing environment is functional
	CheckEnvironmentHealth(envPath string) bool

	// CheckHealth verifies the environment is healthy
	CheckHealth(envPath, version string) error
}

// DependencyManager defines the interface for dependency management
type DependencyManager interface {
	// InstallDependencies installs dependencies in the environment
	InstallDependencies(envPath string, deps []string) error
}

// Manager defines the complete interface for language-specific operations
// It embeds all the smaller interfaces to maintain backward compatibility
type Manager interface {
	Core
	EnvironmentManager
	HealthChecker
	DependencyManager
}
