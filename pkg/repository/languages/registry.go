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

	// Register all language implementations
	// Create a single shared Python instance to ensure identical cache behavior
	sharedPython := NewPythonLanguage()
	registry.languages["python"] = sharedPython
	registry.languages["python3"] = sharedPython
	registry.languages["node"] = NewNodeLanguage()
	registry.languages["golang"] = NewGoLanguage()
	registry.languages["rust"] = NewRustLanguage()

	// Individual language implementations
	registry.languages["ruby"] = NewRubyLanguage()
	registry.languages["dart"] = NewDartLanguage()
	registry.languages["swift"] = NewSwiftLanguage()
	registry.languages["lua"] = NewLuaLanguage()
	registry.languages["perl"] = NewPerlLanguage()
	registry.languages["r"] = NewRLanguage()
	registry.languages["haskell"] = NewHaskellLanguage()
	registry.languages["dotnet"] = NewDotnetLanguage()
	registry.languages["julia"] = NewJuliaLanguage()

	// Container and system languages
	registry.languages["docker"] = NewDockerLanguage()
	registry.languages["docker_image"] = NewDockerImageLanguage()
	registry.languages["system"] = NewSystemLanguage()
	registry.languages["script"] = NewScriptLanguage()
	registry.languages["fail"] = NewFailLanguage()
	registry.languages["pygrep"] = NewPygrepLanguage()

	// conda is a separate language implementation, not a Python variant
	registry.languages["conda"] = NewCondaLanguage()
	registry.languages["coursier"] = NewCoursierLanguage()

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
