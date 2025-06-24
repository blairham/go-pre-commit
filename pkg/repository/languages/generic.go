package languages

import "github.com/blairham/go-pre-commit/pkg/language"

// GenericLanguage handles basic language environment setup without auto-installation
type GenericLanguage struct {
	*language.Base
}

// NewGenericLanguage creates a new generic language handler
func NewGenericLanguage(name, executableName, versionFlag, installURL string) *GenericLanguage {
	return &GenericLanguage{
		Base: language.NewBase(name, executableName, versionFlag, installURL),
	}
}

// SetupEnvironmentWithRepo sets up a generic language environment in the repository
func (g *GenericLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	return g.GenericSetupEnvironmentWithRepo(cacheDir, version, repoPath, additionalDeps)
}

// InstallDependencies is a no-op for generic languages
func (g *GenericLanguage) InstallDependencies(_ string, _ []string) error {
	// Most generic languages don't have dependency management implemented
	return nil
}

// CheckHealth performs a generic health check for generic languages
// If the executable name is empty, just verify the environment directory exists
func (g *GenericLanguage) CheckHealth(envPath, version string) error {
	if g.ExecutableName == "" {
		// For languages with no specific executable (script, fail, system), just check directory exists
		return g.GenericCheckHealth(envPath, version)
	}
	// For languages with executables, use the base health check
	return g.Base.CheckHealth(envPath, version)
}

// Language-specific constructors
// Note: Individual language constructors have been moved to separate files
