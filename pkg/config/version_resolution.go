package config

// ResolveEffectiveLanguageVersion determines the effective language version for a hook
// considering both the hook's specific language_version and the default_language_version config.
// This implements the same logic as Python pre-commit for multi-version support.
func ResolveEffectiveLanguageVersion(hook Hook, config Config) string {
	// If the hook specifies a language_version, use it (takes precedence)
	if hook.LanguageVersion != "" {
		return hook.LanguageVersion
	}

	// Otherwise, check if there's a default_language_version for this language
	if config.DefaultLanguageVersion != nil {
		if defaultVersion, exists := config.DefaultLanguageVersion[hook.Language]; exists {
			return defaultVersion
		}
	}

	// No specific version found, return empty string (will use language's default)
	return ""
}
