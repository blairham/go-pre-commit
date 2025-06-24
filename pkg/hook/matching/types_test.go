package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatcher_initializeTypeMatchers(t *testing.T) {
	matcher := NewMatcher()

	// Test that all expected type matchers are initialized
	expectedTypes := []string{
		"text",
		"python",
		"javascript",
		"typescript",
		"go",
		"java",
		"c",
		"cpp",
		"rust",
		"ruby",
		"php",
		"swift",
		"kotlin",
		"scala",
		"csharp",
		"perl",
		"lua",
		"r",
		"haskell",
		"clojure",
		"erlang",
		"elixir",
		"dart",
		"julia",
		"html",
		"css",
		"xml",
		"yaml",
		"json",
		"markdown",
		"sql",
		"shell",
		"powershell",
	}

	for _, expectedType := range expectedTypes {
		_, exists := matcher.typeMatchers[expectedType]
		assert.True(t, exists, "Type matcher for '%s' should exist", expectedType)
	}
}

func TestTypeMatcher_python(t *testing.T) {
	matcher := NewMatcher()
	pythonMatcher := matcher.typeMatchers["python"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "python file .py",
			file:     "main.py",
			expected: true,
		},
		{
			name:     "python interface file .pyi",
			file:     "types.pyi",
			expected: true,
		},
		{
			name:     "cython file .pyx",
			file:     "module.pyx",
			expected: true,
		},
		{
			name:     "non-python file",
			file:     "main.go",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := pythonMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_javascript(t *testing.T) {
	matcher := NewMatcher()
	jsMatcher := matcher.typeMatchers["javascript"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "javascript file .js",
			file:     "main.js",
			expected: true,
		},
		{
			name:     "jsx file .jsx",
			file:     "component.jsx",
			expected: true,
		},
		{
			name:     "module js file .mjs",
			file:     "module.mjs",
			expected: true,
		},
		{
			name:     "non-javascript file",
			file:     "main.py",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := jsMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_typescript(t *testing.T) {
	matcher := NewMatcher()
	tsMatcher := matcher.typeMatchers["typescript"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "typescript file .ts",
			file:     "main.ts",
			expected: true,
		},
		{
			name:     "tsx file .tsx",
			file:     "component.tsx",
			expected: true,
		},
		{
			name:     "non-typescript file",
			file:     "main.js",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := tsMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_go(t *testing.T) {
	matcher := NewMatcher()
	goMatcher := matcher.typeMatchers["go"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "go file .go",
			file:     "main.go",
			expected: true,
		},
		{
			name:     "go test file .go",
			file:     "main_test.go",
			expected: true,
		},
		{
			name:     "non-go file",
			file:     "main.py",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := goMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_yaml(t *testing.T) {
	matcher := NewMatcher()
	yamlMatcher := matcher.typeMatchers["yaml"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "yaml file .yaml",
			file:     "config.yaml",
			expected: true,
		},
		{
			name:     "yml file .yml",
			file:     "config.yml",
			expected: true,
		},
		{
			name:     "non-yaml file",
			file:     "config.json",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := yamlMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_json(t *testing.T) {
	matcher := NewMatcher()
	jsonMatcher := matcher.typeMatchers["json"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "json file .json",
			file:     "package.json",
			expected: true,
		},
		{
			name:     "json with comments .jsonc",
			file:     "tsconfig.jsonc",
			expected: true,
		},
		{
			name:     "non-json file",
			file:     "config.yaml",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := jsonMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_shell(t *testing.T) {
	matcher := NewMatcher()
	shellMatcher := matcher.typeMatchers["shell"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "shell script .sh",
			file:     "setup.sh",
			expected: true,
		},
		{
			name:     "bash script .bash",
			file:     "script.bash",
			expected: true,
		},
		{
			name:     "zsh script .zsh",
			file:     "config.zsh",
			expected: true,
		},
		{
			name:     "fish script .fish",
			file:     "config.fish",
			expected: true,
		},
		{
			name:     "non-shell file",
			file:     "main.py",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := shellMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_markdown(t *testing.T) {
	matcher := NewMatcher()
	mdMatcher := matcher.typeMatchers["markdown"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "markdown file .md",
			file:     "README.md",
			expected: true,
		},
		{
			name:     "markdown file .markdown",
			file:     "doc.markdown",
			expected: true,
		},
		{
			name:     "markdown file .mdown",
			file:     "notes.mdown",
			expected: true,
		},
		{
			name:     "markdown file .mkd",
			file:     "guide.mkd",
			expected: true,
		},
		{
			name:     "non-markdown file",
			file:     "README.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := mdMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasExt(t *testing.T) {
	matcher := NewMatcher()

	// Test the hasExt helper function
	pythonMatcher := matcher.hasExt(".py", ".pyi", ".pyx")

	tests := []struct {
		name     string
		ext      string
		expected bool
	}{
		{
			name:     "matches .py",
			ext:      ".py",
			expected: true,
		},
		{
			name:     "matches .pyi",
			ext:      ".pyi",
			expected: true,
		},
		{
			name:     "matches .pyx",
			ext:      ".pyx",
			expected: true,
		},
		{
			name:     "does not match .go",
			ext:      ".go",
			expected: false,
		},
		{
			name:     "case insensitive match handled by caller",
			ext:      ".py", // The caller converts to lowercase before calling
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pythonMatcher(tt.ext, "", "")
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper functions for testing
func getFileExt(file string) string {
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '.' {
			return file[i:]
		}
		if file[i] == '/' {
			break
		}
	}
	return ""
}

func getFileName(file string) string {
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' {
			return file[i+1:]
		}
	}
	return file
}

func TestMatcher_hasExtOrFileName(t *testing.T) {
	matcher := NewMatcher()

	// Test dockerfile matcher specifically since it uses hasExtOrFileName
	dockerfileMatcher := matcher.typeMatchers["dockerfile"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "matches Dockerfile",
			file:     "Dockerfile",
			expected: true,
		},
		{
			name:     "matches dockerfile lowercase",
			file:     "dockerfile",
			expected: true,
		},
		{
			name:     "matches Dockerfile in subdirectory",
			file:     "docker/Dockerfile",
			expected: true,
		},
		{
			name:     "does not match other files",
			file:     "not-a-dockerfile.txt",
			expected: false,
		},
		{
			name:     "does not match partial match",
			file:     "MyDockerfile",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := dockerfileMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_hasExtOrFileName_WithExtensions(t *testing.T) {
	matcher := NewMatcher()

	// Create a custom matcher with both extensions and names to test both branches
	testMatcher := matcher.hasExtOrFileName([]string{".test", ".example"}, []string{"TestFile", "ExampleFile"})

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "matches by extension .test",
			file:     "some.test",
			expected: true,
		},
		{
			name:     "matches by extension .example",
			file:     "file.example",
			expected: true,
		},
		{
			name:     "matches by filename TestFile",
			file:     "TestFile",
			expected: true,
		},
		{
			name:     "matches by filename ExampleFile",
			file:     "path/ExampleFile",
			expected: true,
		},
		{
			name:     "does not match wrong extension",
			file:     "file.wrong",
			expected: false,
		},
		{
			name:     "does not match wrong filename",
			file:     "WrongFile",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := testMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_makefile(t *testing.T) {
	matcher := NewMatcher()
	makefileMatcher := matcher.typeMatchers["makefile"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "matches Makefile",
			file:     "Makefile",
			expected: true,
		},
		{
			name:     "matches makefile lowercase",
			file:     "makefile",
			expected: true,
		},
		{
			name:     "matches GNUmakefile",
			file:     "GNUmakefile",
			expected: true,
		},
		{
			name:     "matches Makefile in subdirectory",
			file:     "build/Makefile",
			expected: true,
		},
		{
			name:     "does not match other files",
			file:     "not-a-makefile.txt",
			expected: false,
		},
		{
			name:     "does not match partial match",
			file:     "MyMakefile",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := makefileMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_react(t *testing.T) {
	matcher := NewMatcher()
	reactMatcher := matcher.typeMatchers["react"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "matches jsx file",
			file:     "component.jsx",
			expected: true,
		},
		{
			name:     "matches tsx file",
			file:     "component.tsx",
			expected: true,
		},
		{
			name:     "matches file with react in path",
			file:     "src/react/component.js",
			expected: true,
		},
		{
			name:     "does not match regular js",
			file:     "regular.js",
			expected: false,
		},
		{
			name:     "does not match unrelated files",
			file:     "unrelated.py",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := reactMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_angular(t *testing.T) {
	matcher := NewMatcher()
	angularMatcher := matcher.typeMatchers["angular"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "matches component file",
			file:     "app.component.ts",
			expected: true,
		},
		{
			name:     "matches service file",
			file:     "data.service.ts",
			expected: true,
		},
		{
			name:     "matches module file",
			file:     "app.module.ts",
			expected: true,
		},
		{
			name:     "does not match regular ts",
			file:     "regular.ts",
			expected: false,
		},
		{
			name:     "does not match non-ts files",
			file:     "component.js",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := angularMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMatcher_isTextFile(t *testing.T) {
	matcher := NewMatcher()
	textMatcher := matcher.typeMatchers["text"]

	tests := []struct {
		name     string
		file     string
		expected bool
	}{
		{
			name:     "matches txt file",
			file:     "readme.txt",
			expected: true,
		},
		{
			name:     "matches log file",
			file:     "error.log",
			expected: true,
		},
		{
			name:     "matches config file",
			file:     "app.cfg",
			expected: true,
		},
		{
			name:     "matches conf file",
			file:     "nginx.conf",
			expected: true,
		},
		{
			name:     "matches ini file",
			file:     "settings.ini",
			expected: true,
		},
		{
			name:     "matches properties file",
			file:     "app.properties",
			expected: true,
		},
		{
			name:     "matches python as programming language",
			file:     "main.py",
			expected: true,
		},
		{
			name:     "matches javascript as programming language",
			file:     "app.js",
			expected: true,
		},
		{
			name:     "matches go as programming language",
			file:     "main.go",
			expected: true,
		},
		{
			name:     "matches markdown as programming language",
			file:     "README.md",
			expected: true,
		},
		{
			name:     "does not match binary-like files",
			file:     "image.png",
			expected: false,
		},
		{
			name:     "does not match unknown extensions",
			file:     "file.unknown",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := textMatcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_initializeTypeMatchers_AdditionalTypes(t *testing.T) {
	matcher := NewMatcher()

	// Test some additional types that might not be covered by other tests
	additionalTypes := []string{
		"dockerfile",
		"makefile",
		"react",
		"angular",
		"vue",
		"svelte",
		"powershell",
		"java",
		"c",
		"cpp",
		"kotlin",
		"scala",
		"csharp",
		"clojure",
		"erlang",
		"elixir",
		"julia",
		"html",
		"css",
		"xml",
		"sql",
	}

	for _, typeName := range additionalTypes {
		t.Run("type matcher exists for "+typeName, func(t *testing.T) {
			_, exists := matcher.typeMatchers[typeName]
			assert.True(t, exists, "Type matcher for '%s' should exist", typeName)
		})
	}
}

func TestTypeMatcher_additionalLanguages(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		typeName string
		file     string
		expected bool
	}{
		// Test additional language matchers that might not be fully tested
		{"java", "Main.java", true},
		{"c", "main.c", true},
		{"c", "header.h", true},
		{"cpp", "main.cpp", true},
		{"cpp", "header.hpp", true},
		{"rust", "main.rs", true},
		{"php", "index.php", true},
		{"kotlin", "Main.kt", true},
		{"scala", "Main.scala", true},
		{"csharp", "Program.cs", true},
		{"clojure", "core.clj", true},
		{"erlang", "main.erl", true},
		{"elixir", "main.ex", true},
		{"julia", "script.jl", true},
		{"html", "index.html", true},
		{"css", "style.css", true},
		{"xml", "config.xml", true},
		{"sql", "schema.sql", true},
		{"powershell", "script.ps1", true},
		{"vue", "component.vue", true},
		{"svelte", "component.svelte", true},
	}

	for _, tt := range tests {
		t.Run(tt.typeName+"_matches_"+tt.file, func(t *testing.T) {
			matcher, exists := matcher.typeMatchers[tt.typeName]
			assert.True(t, exists, "Type matcher should exist for %s", tt.typeName)

			ext := getFileExt(tt.file)
			fileName := getFileName(tt.file)
			result := matcher(ext, fileName, tt.file)
			assert.Equal(t, tt.expected, result)
		})
	}
}
