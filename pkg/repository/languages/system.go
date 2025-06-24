package languages

// SystemLanguage handles system-level commands that don't require environment setup
type SystemLanguage struct {
	*GenericLanguage
}

// NewSystemLanguage creates a new system language handler
func NewSystemLanguage() *SystemLanguage {
	return &SystemLanguage{
		GenericLanguage: NewGenericLanguage("system", "", "", ""),
	}
}

// IsRuntimeAvailable always returns true for system language
func (s *SystemLanguage) IsRuntimeAvailable() bool {
	return true
}

// SetupEnvironmentWithRepo for system language creates a minimal environment directory for consistency
func (s *SystemLanguage) SetupEnvironmentWithRepo(
	cacheDir, version, repoPath, _ string, // repoURL is unused
	additionalDeps []string,
) (string, error) {
	return s.GenericSetupEnvironmentWithRepo(cacheDir, version, repoPath, additionalDeps)
}
