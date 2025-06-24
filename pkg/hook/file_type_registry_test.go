package hook

import (
	"testing"
)

func TestFileTypeRegistry_NewFileTypeRegistry(t *testing.T) {
	registry := NewFileTypeRegistry()

	if registry == nil {
		t.Fatal("registry should not be nil")
	}

	if registry.typeMap == nil {
		t.Fatal("typeMap should not be nil")
	}

	// Check that some basic types are registered
	expectedTypes := []string{
		"python", "javascript", "typescript", "go", "java", "rust",
		"yaml", "json", "markdown", "html", "css",
	}

	for _, typ := range expectedTypes {
		if _, exists := registry.typeMap[typ]; !exists {
			t.Errorf("expected type '%s' to be registered", typ)
		}
	}
}

func TestFileTypeRegistry_MatchesType(t *testing.T) {
	registry := NewFileTypeRegistry()

	tests := []struct {
		file     string
		fileType string
		expected bool
	}{
		// Python files
		{"script.py", "python", true},
		{"module.pyx", "python", true},
		{"interface.pyi", "python", true},
		{"script.js", "python", false},

		// JavaScript files
		{"app.js", "javascript", true},
		{"component.jsx", "javascript", true},
		{"module.mjs", "javascript", true},
		{"script.py", "javascript", false},

		// TypeScript files
		{"component.ts", "typescript", true},
		{"component.tsx", "typescript", true},
		{"script.js", "typescript", false},

		// Go files
		{"main.go", "go", true},
		{"script.py", "go", false},

		// YAML files
		{"config.yaml", "yaml", true},
		{"config.yml", "yaml", true},
		{"config.json", "yaml", false},

		// JSON files
		{"package.json", "json", true},
		{"config.yaml", "json", false},

		// Markdown files
		{"README.md", "markdown", true},
		{"doc.markdown", "markdown", true},
		{"notes.mdown", "markdown", true},
		{"file.mkd", "markdown", true},
		{"script.py", "markdown", false},

		// HTML files
		{"index.html", "html", true},
		{"page.htm", "html", true},
		{"template.xhtml", "html", true},
		{"script.py", "html", false},

		// CSS files
		{"style.css", "css", true},
		{"style.scss", "css", true},
		{"style.sass", "css", true},
		{"style.less", "css", true},
		{"script.js", "css", false},

		// Shell files
		{"script.sh", "shell", true},
		{"script.bash", "shell", true},
		{"script.zsh", "shell", true},
		{"script.fish", "shell", true},
		{"script.py", "shell", false},

		// Ruby files
		{"app.rb", "ruby", true},
		{"script.py", "ruby", false},

		// Java files
		{"Main.java", "java", true},
		{"script.py", "java", false},

		// Rust files
		{"main.rs", "rust", true},
		{"script.py", "rust", false},

		// C files
		{"main.c", "c", true},
		{"header.h", "c", true},
		{"main.cpp", "c", false},

		// C++ files
		{"main.cpp", "cpp", true},
		{"main.cc", "cpp", true},
		{"main.cxx", "cpp", true},
		{"header.hpp", "cpp", true},
		{"header.hxx", "cpp", true},
		{"main.c", "cpp", false},

		// Case insensitive matching
		{"FILE.PY", "python", true},
		{"FILE.JS", "javascript", true},
		{"CONFIG.YAML", "yaml", true},
	}

	for _, tt := range tests {
		t.Run(tt.file+"_"+tt.fileType, func(t *testing.T) {
			result := registry.MatchesType(tt.file, tt.fileType)
			if result != tt.expected {
				t.Errorf("MatchesType(%s, %s) = %v, expected %v",
					tt.file, tt.fileType, result, tt.expected)
			}
		})
	}
}

func TestFileTypeRegistry_SpecialCases(t *testing.T) {
	registry := NewFileTypeRegistry()

	tests := []struct {
		file     string
		fileType string
		expected bool
	}{
		// Dockerfile special cases
		{"Dockerfile", "dockerfile", true},
		{"dockerfile", "dockerfile", true},
		{"Dockerfile.prod", "dockerfile", true},
		{"dockerfile.dev", "dockerfile", true},
		{"docker-compose.yml", "dockerfile", false},

		// Ruby special cases
		{"Gemfile", "ruby", true},
		{"gemfile", "ruby", true},
		{"Rakefile", "ruby", true},
		{"rakefile", "ruby", true},
		{"app.rb", "ruby", true},

		// Helm special cases
		{"Chart.yaml", "helm", true},
		{"chart.yaml", "helm", true},
		{"templates/deployment.yaml", "helm", true},
		{"other.yaml", "helm", false},

		// Docker Compose special cases
		{"docker-compose.yml", "docker-compose", true},
		{"docker-compose.yaml", "docker-compose", true},
		{"docker-compose.dev.yml", "docker-compose", true},
		{"compose.yml", "docker-compose", true},
		{"other.yml", "docker-compose", false},

		// Vagrant special cases
		{"Vagrantfile", "vagrant", true},
		{"vagrantfile", "vagrant", true},
		{"other.rb", "vagrant", false},

		// Template special cases
		{"template.html", "django", false}, // Not in templates/
		{"templates/base.html", "django", true},
		{"app/templates/index.html", "flask", true},
		{"template.j2", "jinja", true},
		{"template.jinja", "jinja", true},
		{"template.jinja2", "jinja", true},
		{"template.hbs", "handlebars", true},
		{"template.handlebars", "handlebars", true},
		{"template.mustache", "mustache", true},
		{"template.liquid", "liquid", true},
		{"template.tpl", "smarty", true},
		{"template.njk", "nunjucks", true},

		// Framework special cases
		{"component.vue", "vue", true},
		{"component.svelte", "svelte", true},
		{"component.jsx", "react", true},
		{"component.tsx", "react", true},
		{"service.component.ts", "angular", true},
		{"app.service.ts", "angular", true},
		{"main.module.ts", "angular", true},
		{"regular.ts", "angular", false},
	}

	for _, tt := range tests {
		t.Run(tt.file+"_"+tt.fileType, func(t *testing.T) {
			result := registry.MatchesType(tt.file, tt.fileType)
			if result != tt.expected {
				t.Errorf("MatchesType(%s, %s) = %v, expected %v",
					tt.file, tt.fileType, result, tt.expected)
			}
		})
	}
}

func TestFileTypeRegistry_TextFiles(t *testing.T) {
	registry := NewFileTypeRegistry()

	tests := []struct {
		file     string
		expected bool
	}{
		// Text files
		{"script.py", true},
		{"readme.txt", true},
		{"config.yaml", true},
		{"main.go", true},

		// Binary files
		{"image.png", false},
		{"photo.jpg", false},
		{"photo.jpeg", false},
		{"icon.gif", false},
		{"document.pdf", false},
		{"archive.zip", false},
		{"archive.tar", false},
		{"archive.gz", false},
		{"program.exe", false},
		{"library.dll", false},
		{"library.so", false},
		{"binary.bin", false},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			result := registry.MatchesType(tt.file, "text")
			if result != tt.expected {
				t.Errorf("MatchesType(%s, text) = %v, expected %v",
					tt.file, result, tt.expected)
			}
		})
	}
}

func TestFileTypeRegistry_MatchesAnyType(t *testing.T) {
	registry := NewFileTypeRegistry()

	tests := []struct {
		file     string
		types    []string
		expected bool
	}{
		{
			file:     "script.py",
			types:    []string{"python", "javascript"},
			expected: true,
		},
		{
			file:     "app.js",
			types:    []string{"python", "javascript"},
			expected: true,
		},
		{
			file:     "style.css",
			types:    []string{"python", "javascript"},
			expected: false,
		},
		{
			file:     "component.tsx",
			types:    []string{"typescript", "react"},
			expected: true,
		},
		{
			file:     "README.md",
			types:    []string{"markdown", "text"},
			expected: true,
		},
		{
			file:     "image.png",
			types:    []string{"python", "javascript", "go"},
			expected: false,
		},
		{
			file:     "empty.file",
			types:    []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			result := registry.MatchesAnyType(tt.file, tt.types)
			if result != tt.expected {
				t.Errorf("MatchesAnyType(%s, %v) = %v, expected %v",
					tt.file, tt.types, result, tt.expected)
			}
		})
	}
}

func TestFileTypeRegistry_MatchesAllTypes(t *testing.T) {
	registry := NewFileTypeRegistry()

	tests := []struct {
		file     string
		types    []string
		expected bool
	}{
		{
			file:     "script.py",
			types:    []string{"python", "text"},
			expected: true,
		},
		{
			file:     "script.py",
			types:    []string{"python", "javascript"},
			expected: false,
		},
		{
			file:     "component.tsx",
			types:    []string{"typescript", "react"},
			expected: true,
		},
		{
			file:     "component.ts",
			types:    []string{"typescript", "react"},
			expected: false,
		},
		{
			file:     "README.md",
			types:    []string{"markdown", "text"},
			expected: true,
		},
		{
			file:     "image.png",
			types:    []string{"python", "javascript"},
			expected: false,
		},
		{
			file:     "script.py",
			types:    []string{},
			expected: true, // Empty list should return true
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			result := registry.MatchesAllTypes(tt.file, tt.types)
			if result != tt.expected {
				t.Errorf("MatchesAllTypes(%s, %v) = %v, expected %v",
					tt.file, tt.types, result, tt.expected)
			}
		})
	}
}

func TestFileTypeRegistry_AddCustomType(t *testing.T) {
	registry := NewFileTypeRegistry()

	// Add a custom type
	registry.AddCustomType("custom", []string{".custom", ".special"})

	// Test the custom type
	tests := []struct {
		file     string
		expected bool
	}{
		{"file.custom", true},
		{"file.special", true},
		{"file.py", false},
		{"file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			result := registry.MatchesType(tt.file, "custom")
			if result != tt.expected {
				t.Errorf("MatchesType(%s, custom) = %v, expected %v",
					tt.file, result, tt.expected)
			}
		})
	}

	// Test that we can override existing types
	registry.AddCustomType("python", []string{".py3"})

	// Now .py should not match python (overridden), but .py3 should
	if registry.MatchesType("script.py", "python") {
		t.Error("expected .py to not match python after override")
	}

	if !registry.MatchesType("script.py3", "python") {
		t.Error("expected .py3 to match python after override")
	}
}

func BenchmarkFileTypeRegistry_MatchesType(b *testing.B) {
	registry := NewFileTypeRegistry()

	files := []string{
		"script.py", "app.js", "component.tsx", "main.go",
		"style.css", "index.html", "config.yaml", "data.json",
		"README.md", "main.rs", "App.java", "main.c",
	}

	types := []string{
		"python", "javascript", "typescript", "go",
		"css", "html", "yaml", "json",
		"markdown", "rust", "java", "c",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		file := files[i%len(files)]
		typ := types[i%len(types)]
		registry.MatchesType(file, typ)
	}
}
