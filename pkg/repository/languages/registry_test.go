package languages

import (
	"slices"
	"sort"
	"testing"
)

func TestLanguageRegistry(t *testing.T) {
	t.Run("NewLanguageRegistry", func(t *testing.T) {
		registry := NewLanguageRegistry()
		if registry == nil {
			t.Error("NewLanguageRegistry() returned nil")
			return
		}
		if registry.languages == nil {
			t.Error("NewLanguageRegistry() returned registry with nil languages map")
		}
	})

	t.Run("GetLanguage", func(t *testing.T) {
		registry := NewLanguageRegistry()

		// Test getting a supported language
		lang, exists := registry.GetLanguage("python")
		if !exists {
			t.Error("GetLanguage('python') should return true for exists")
		}
		if lang == nil {
			t.Error("GetLanguage('python') should return non-nil language")
		}

		// Test getting python3 (should be same instance as python)
		lang3, exists3 := registry.GetLanguage("python3")
		if !exists3 {
			t.Error("GetLanguage('python3') should return true for exists")
		}
		if lang3 == nil {
			t.Error("GetLanguage('python3') should return non-nil language")
		}

		// python and python3 should be the same instance
		if lang != lang3 {
			t.Error("GetLanguage('python') and GetLanguage('python3') should return the same instance")
		}

		// Test getting an unsupported language
		lang, exists = registry.GetLanguage("nonexistent")
		if exists {
			t.Error("GetLanguage('nonexistent') should return false for exists")
		}
		if lang != nil {
			t.Error("GetLanguage('nonexistent') should return nil language")
		}
	})

	t.Run("IsLanguageSupported", func(t *testing.T) {
		registry := NewLanguageRegistry()

		// Test supported languages
		supportedLanguages := []string{
			"python", "python3", "node", "golang", "rust", "ruby",
			"dart", "swift", "lua", "perl", "r", "haskell", "dotnet",
			"julia", "docker", "docker_image", "system", "script",
			"fail", "pygrep", "conda", "coursier",
		}

		for _, lang := range supportedLanguages {
			if !registry.IsLanguageSupported(lang) {
				t.Errorf("IsLanguageSupported('%s') should return true", lang)
			}
		}

		// Test unsupported languages
		unsupportedLanguages := []string{
			"nonexistent", "php", "java", "c++", "c", "assembly",
		}

		for _, lang := range unsupportedLanguages {
			if registry.IsLanguageSupported(lang) {
				t.Errorf("IsLanguageSupported('%s') should return false", lang)
			}
		}
	})

	t.Run("GetSupportedLanguages", func(t *testing.T) {
		registry := NewLanguageRegistry()

		languages := registry.GetSupportedLanguages()
		if languages == nil {
			t.Error("GetSupportedLanguages() returned nil")
			return
		}

		// Should have all expected languages
		expectedLanguages := []string{
			"python", "python3", "node", "golang", "rust", "ruby",
			"dart", "swift", "lua", "perl", "r", "haskell", "dotnet",
			"julia", "docker", "docker_image", "system", "script",
			"fail", "pygrep", "conda", "coursier",
		}

		if len(languages) != len(expectedLanguages) {
			t.Errorf(
				"GetSupportedLanguages() returned %d languages, expected %d",
				len(languages),
				len(expectedLanguages),
			)
		}

		// Convert to map for easier lookup
		languageMap := make(map[string]bool)
		for _, lang := range languages {
			languageMap[lang] = true
		}

		// Check all expected languages are present
		for _, expected := range expectedLanguages {
			if !languageMap[expected] {
				t.Errorf("GetSupportedLanguages() missing expected language: %s", expected)
			}
		}

		// Check no extra languages
		for _, actual := range languages {
			found := slices.Contains(expectedLanguages, actual)
			if !found {
				t.Errorf("GetSupportedLanguages() contains unexpected language: %s", actual)
			}
		}
	})

	t.Run("GetSupportedLanguages_Deterministic", func(t *testing.T) {
		registry := NewLanguageRegistry()

		// Test that GetSupportedLanguages returns consistent results
		languages1 := registry.GetSupportedLanguages()
		languages2 := registry.GetSupportedLanguages()

		// Sort both slices for comparison
		sort.Strings(languages1)
		sort.Strings(languages2)

		if len(languages1) != len(languages2) {
			t.Error("GetSupportedLanguages() returned different lengths on subsequent calls")
			return
		}

		for i, lang := range languages1 {
			if languages2[i] != lang {
				t.Errorf("GetSupportedLanguages() returned different results on subsequent calls")
				break
			}
		}
	})

	t.Run("SharedPythonInstance", func(t *testing.T) {
		registry := NewLanguageRegistry()

		// Verify that python and python3 are the same instance
		python, _ := registry.GetLanguage("python")
		python3, _ := registry.GetLanguage("python3")

		if python != python3 {
			t.Error("python and python3 should be the same instance to ensure identical cache behavior")
		}
	})

	t.Run("AllLanguagesNonNil", func(t *testing.T) {
		registry := NewLanguageRegistry()

		// Test that all registered languages return non-nil instances
		for _, lang := range registry.GetSupportedLanguages() {
			instance, exists := registry.GetLanguage(lang)
			if !exists {
				t.Errorf("Language '%s' should exist but doesn't", lang)
			}
			if instance == nil {
				t.Errorf("Language '%s' instance should not be nil", lang)
			}
		}
	})
}
