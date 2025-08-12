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

// InstallDependencies is a no-op for generic language
func (g *GenericLanguage) InstallDependencies(_ string, _ []string) error {
	// No dependencies to install for generic language
	return nil
}

// CheckHealth performs a generic health check for generic languages
// If the executable name is empty, just verify the environment directory exists
func (g *GenericLanguage) CheckHealth(envPath string) error {
	if g.ExecutableName == "" {
		// For languages with no specific executable (script, fail, system), just check directory exists
		return g.GenericCheckHealth(envPath)
	}
	// For languages with executables, use the base health check
	return g.Base.CheckHealth(envPath)
}

// Language-specific constructors
// Note: Individual language constructors have been moved to separate files
