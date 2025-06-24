package matching

import (
	"slices"
	"strings"
)

// initializeTypeMatchers creates a map of type matchers
func (m *Matcher) initializeTypeMatchers() map[string]TypeMatcher {
	matchers := make(map[string]TypeMatcher)

	// Basic file types
	matchers["text"] = m.isTextFile

	// Programming languages
	matchers["python"] = m.hasExt(".py", ".pyi", ".pyx")
	matchers["javascript"] = m.hasExt(".js", ".jsx", ".mjs")
	matchers["typescript"] = m.hasExt(".ts", ".tsx")
	matchers["go"] = m.hasExt(".go")
	matchers["java"] = m.hasExt(".java")
	matchers["c"] = m.hasExt(".c", ".h")
	matchers["cpp"] = m.hasExt(".cpp", ".cxx", ".cc", ".hpp", ".hxx", ".hh")
	matchers["rust"] = m.hasExt(".rs")
	matchers["ruby"] = m.hasExt(".rb", ".rbw")
	matchers["php"] = m.hasExt(".php", ".phtml")
	matchers["swift"] = m.hasExt(".swift")
	matchers["kotlin"] = m.hasExt(".kt", ".kts")
	matchers["scala"] = m.hasExt(".scala", ".sc")
	matchers["csharp"] = m.hasExt(".cs")
	matchers["perl"] = m.hasExt(".pl", ".pm")
	matchers["lua"] = m.hasExt(".lua")
	matchers["r"] = m.hasExt(".r", ".R")
	matchers["haskell"] = m.hasExt(".hs", ".lhs")
	matchers["clojure"] = m.hasExt(".clj", ".cljs", ".cljc")
	matchers["erlang"] = m.hasExt(".erl", ".hrl")
	matchers["elixir"] = m.hasExt(".ex", ".exs")
	matchers["dart"] = m.hasExt(".dart")
	matchers["julia"] = m.hasExt(".jl")

	// Markup and data languages
	matchers["html"] = m.hasExt(".html", ".htm", ".xhtml")
	matchers["css"] = m.hasExt(".css", ".scss", ".sass", ".less")
	matchers["xml"] = m.hasExt(".xml", ".xsd", ".xsl")
	matchers["yaml"] = m.hasExt(".yaml", ".yml")
	matchers["json"] = m.hasExt(".json", ".jsonc")
	matchers["markdown"] = m.hasExt(".md", ".markdown", ".mdown", ".mkd")
	matchers["sql"] = m.hasExt(".sql")

	// Shell and system
	matchers["shell"] = m.hasExt(".sh", ".bash", ".zsh", ".fish")
	matchers["powershell"] = m.hasExt(".ps1", ".psm1", ".psd1")
	matchers["dockerfile"] = m.hasExtOrFileName([]string{}, []string{"Dockerfile", "dockerfile"})
	matchers["makefile"] = m.hasExtOrFileName(
		[]string{},
		[]string{"Makefile", "makefile", "GNUmakefile"},
	)

	// Framework-specific
	matchers["vue"] = m.hasExt(".vue")
	matchers["svelte"] = m.hasExt(".svelte")
	matchers["react"] = func(ext, _ /* fileName */, file string) bool {
		return ext == ".jsx" || ext == ".tsx" || strings.Contains(file, "react")
	}
	matchers["angular"] = func(ext, _ /* fileName */, file string) bool {
		return ext == ".ts" && (strings.Contains(file, ".component.") ||
			strings.Contains(file, ".service.") || strings.Contains(file, ".module."))
	}

	return matchers
}

// hasExt creates a matcher that checks for specific file extensions
func (m *Matcher) hasExt(extensions ...string) TypeMatcher {
	return func(ext, _ /* fileName */, _ /* file */ string) bool {
		return slices.Contains(extensions, ext)
	}
}

// hasExtOrFileName creates a matcher that checks extensions or specific filenames
func (m *Matcher) hasExtOrFileName(extensions, names []string) TypeMatcher {
	return func(ext, fileName, _ /* file */ string) bool {
		// Check extensions
		if slices.Contains(extensions, ext) {
			return true
		}

		// Check specific filenames
		return slices.Contains(names, fileName)
	}
}

// isTextFile determines if a file is likely a text file
func (m *Matcher) isTextFile(ext, fileName, file string) bool {
	textExtensions := []string{
		".txt", ".md", ".rst", ".log", ".cfg", ".conf", ".ini", ".properties",
	}

	if slices.Contains(textExtensions, ext) {
		return true
	}

	// Programming language files are also text files
	programmingTypes := []string{
		"python", "javascript", "typescript", "go", "java", "c", "cpp",
		"rust", "ruby", "php", "swift", "kotlin", "scala", "csharp",
		"html", "css", "xml", "yaml", "json", "markdown", "sql", "shell",
	}

	for _, progType := range programmingTypes {
		if matcher, exists := m.typeMatchers[progType]; exists {
			if matcher(ext, fileName, file) {
				return true
			}
		}
	}

	return false
}
