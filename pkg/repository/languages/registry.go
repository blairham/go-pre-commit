package languages

// LanguageRegistry holds all language implementations
type LanguageRegistry struct {
	languages map[string]any
}

// NewLanguageRegistry creates a new language registry with all supported languages
func NewLanguageRegistry() *LanguageRegistry {
	registry := &LanguageRegistry{
		languages: make(map[string]any),
	}

	// Primary programming languages (alphabetical order)
	registry.languages["conda"] = NewCondaLanguage()
	registry.languages["coursier"] = NewCoursierLanguage()
	registry.languages["dart"] = NewDartLanguage()
	registry.languages["dotnet"] = NewDotnetLanguage()
	registry.languages["golang"] = NewGoLanguage()
	registry.languages["haskell"] = NewHaskellLanguage()
	registry.languages["julia"] = NewJuliaLanguage()
	registry.languages["lua"] = NewLuaLanguage()
	registry.languages["node"] = NewNodeLanguage()
	registry.languages["perl"] = NewPerlLanguage()

	// Python instances (shared instance to ensure identical cache behavior)
	sharedPython := NewPythonLanguage()
	registry.languages["python"] = sharedPython
	registry.languages["python3"] = sharedPython

	registry.languages["r"] = NewRLanguage()
	registry.languages["ruby"] = NewRubyLanguage()
	registry.languages["rust"] = NewRustLanguage()
	registry.languages["swift"] = NewSwiftLanguage()

	// Container technologies
	registry.languages["docker"] = NewDockerLanguage()
	registry.languages["docker_image"] = NewDockerImageLanguage()

	// System and utility languages
	registry.languages["fail"] = NewFailLanguage()
	registry.languages["pygrep"] = NewPygrepLanguage()
	registry.languages["script"] = NewScriptLanguage()
	registry.languages["system"] = NewSystemLanguage()

	return registry
}

// GetLanguage returns the language setup for a given language name
func (lr *LanguageRegistry) GetLanguage(language string) (any, bool) {
	lang, exists := lr.languages[language]
	return lang, exists
}

// IsLanguageSupported checks if a language is supported
func (lr *LanguageRegistry) IsLanguageSupported(language string) bool {
	_, exists := lr.languages[language]
	return exists
}

// GetSupportedLanguages returns a list of all supported languages
func (lr *LanguageRegistry) GetSupportedLanguages() []string {
	languages := make([]string, 0, len(lr.languages))
	for lang := range lr.languages {
		languages = append(languages, lang)
	}
	return languages
}

// GetPrimaryLanguages returns languages excluding aliases and utility languages
func (lr *LanguageRegistry) GetPrimaryLanguages() []string {
	primary := []string{
		"conda", "coursier", "dart", "dotnet", "golang", "haskell", "julia",
		"lua", "node", "perl", "python", "r", "ruby", "rust", "swift",
	}

	var supported []string
	for _, lang := range primary {
		if lr.IsLanguageSupported(lang) {
			supported = append(supported, lang)
		}
	}
	return supported
}

// GetContainerLanguages returns container-related languages
func (lr *LanguageRegistry) GetContainerLanguages() []string {
	container := []string{"docker", "docker_image"}

	var supported []string
	for _, lang := range container {
		if lr.IsLanguageSupported(lang) {
			supported = append(supported, lang)
		}
	}
	return supported
}

// GetUtilityLanguages returns utility and system languages
func (lr *LanguageRegistry) GetUtilityLanguages() []string {
	utility := []string{"fail", "pygrep", "script", "system"}

	var supported []string
	for _, lang := range utility {
		if lr.IsLanguageSupported(lang) {
			supported = append(supported, lang)
		}
	}
	return supported
}
