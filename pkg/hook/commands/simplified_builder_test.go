package commands

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestNewSimpleLanguageCommandBuilder(t *testing.T) {
	builder := NewSimpleLanguageCommandBuilder()

	if builder == nil {
		t.Fatal("Expected non-nil builder")
	}

	if builder.templates == nil {
		t.Fatal("Expected templates map to be initialized")
	}

	// Check that common languages are registered
	expectedLanguages := []string{"ruby", "perl", "lua", "swift", "r", "haskell", "go", "rust"}
	for _, lang := range expectedLanguages {
		if _, exists := builder.templates[lang]; !exists {
			t.Errorf("Expected language %s to be registered", lang)
		}
	}
}

func TestSimpleLanguageCommandBuilder_RegisterCommonLanguages(t *testing.T) {
	builder := &SimpleLanguageCommandBuilder{
		templates: make(map[string]*CommandTemplate),
	}

	builder.registerCommonLanguages()

	// Test scripting languages
	scriptingLanguages := map[string]string{
		"ruby":    "ruby",
		"perl":    "perl",
		"lua":     "lua",
		"swift":   "swift",
		"r":       "Rscript",
		"haskell": "runhaskell",
	}

	for lang, expectedExec := range scriptingLanguages {
		template, exists := builder.templates[lang]
		if !exists {
			t.Errorf("Expected language %s to be registered", lang)
			continue
		}

		if template.Executable != expectedExec {
			t.Errorf("Expected executable for %s to be %s, got %s", lang, expectedExec, template.Executable)
		}

		if !template.UseEntryAsScript {
			t.Errorf("Expected UseEntryAsScript to be true for %s", lang)
		}
	}

	// Test special cases with custom builders
	goTemplate, exists := builder.templates["go"]
	if !exists {
		t.Error("Expected go language to be registered")
	} else {
		if goTemplate.Executable != "go" {
			t.Errorf("Expected go executable to be 'go', got %s", goTemplate.Executable)
		}
		if goTemplate.UseEntryAsScript {
			t.Error("Expected UseEntryAsScript to be false for go")
		}
		if goTemplate.CustomBuilder == nil {
			t.Error("Expected custom builder for go")
		}
	}

	rustTemplate, exists := builder.templates["rust"]
	if !exists {
		t.Error("Expected rust language to be registered")
	} else {
		if rustTemplate.Executable != "rustc" {
			t.Errorf("Expected rust executable to be 'rustc', got %s", rustTemplate.Executable)
		}
		if rustTemplate.UseEntryAsScript {
			t.Error("Expected UseEntryAsScript to be false for rust")
		}
		if rustTemplate.CustomBuilder == nil {
			t.Error("Expected custom builder for rust")
		}
	}
}

func TestSimpleLanguageCommandBuilder_BuildLanguageCommand_ScriptingLanguages(t *testing.T) {
	builder := NewSimpleLanguageCommandBuilder()

	tests := []struct {
		language     string
		entry        string
		args         []string
		expectedExec string
		expectedArgs []string
	}{
		{
			language:     "ruby",
			entry:        "script.rb",
			args:         []string{"--verbose"},
			expectedExec: "ruby",
			expectedArgs: []string{"ruby", "script.rb", "--verbose"},
		},
		{
			language:     "perl",
			entry:        "script.pl",
			args:         []string{"-w"},
			expectedExec: "perl",
			expectedArgs: []string{"perl", "script.pl", "-w"},
		},
		{
			language:     "lua",
			entry:        "script.lua",
			args:         []string{},
			expectedExec: "lua",
			expectedArgs: []string{"lua", "script.lua"},
		},
		{
			language:     "swift",
			entry:        "script.swift",
			args:         []string{"arg1"},
			expectedExec: "swift",
			expectedArgs: []string{"swift", "script.swift", "arg1"},
		},
		{
			language:     "r",
			entry:        "script.R",
			args:         []string{"--slave"},
			expectedExec: "Rscript",
			expectedArgs: []string{"Rscript", "script.R", "--slave"},
		},
		{
			language:     "haskell",
			entry:        "script.hs",
			args:         []string{},
			expectedExec: "runhaskell",
			expectedArgs: []string{"runhaskell", "script.hs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			cmd := builder.BuildLanguageCommand(tt.language, tt.entry, tt.args)

			if cmd == nil {
				t.Fatal("Expected non-nil command")
			}

			if filepath.Base(cmd.Path) != tt.expectedExec {
				t.Errorf("Expected command path %s, got %s", tt.expectedExec, filepath.Base(cmd.Path))
			}

			if !reflect.DeepEqual(cmd.Args, tt.expectedArgs) {
				t.Errorf("Expected args %v, got %v", tt.expectedArgs, cmd.Args)
			}
		})
	}
}

func TestSimpleLanguageCommandBuilder_BuildLanguageCommand_UnknownLanguage(t *testing.T) {
	builder := NewSimpleLanguageCommandBuilder()

	entry := "custom-tool"
	args := []string{"--flag", "value"}

	cmd := builder.BuildLanguageCommand("unknown", entry, args)

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"custom-tool", "--flag", "value"}
	if !reflect.DeepEqual(cmd.Args, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, cmd.Args)
	}
}

func TestSimpleLanguageCommandBuilder_BuildGoCommand(t *testing.T) {
	builder := NewSimpleLanguageCommandBuilder()

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedExec string
		expectedArgs []string
	}{
		{
			name:         "go command with subcommand",
			entry:        "go run",
			args:         []string{"main.go"},
			expectedExec: "go",
			expectedArgs: []string{"go", "run", "main.go"},
		},
		{
			name:         "go build command",
			entry:        "go build",
			args:         []string{"-o", "output"},
			expectedExec: "go",
			expectedArgs: []string{"go", "build", "-o", "output"},
		},
		{
			name:         "go script file",
			entry:        "script.go",
			args:         []string{"arg1"},
			expectedExec: "go",
			expectedArgs: []string{"go", "run", "script.go", "arg1"},
		},
		{
			name:         "go executable",
			entry:        "gofmt",
			args:         []string{"-w", "file.go"},
			expectedExec: "gofmt",
			expectedArgs: []string{"gofmt", "-w", "file.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.BuildLanguageCommand("go", tt.entry, tt.args)

			if cmd == nil {
				t.Fatal("Expected non-nil command")
			}

			if filepath.Base(cmd.Path) != tt.expectedExec {
				t.Errorf("Expected command path %s, got %s", tt.expectedExec, filepath.Base(cmd.Path))
			}

			if !reflect.DeepEqual(cmd.Args, tt.expectedArgs) {
				t.Errorf("Expected args %v, got %v", tt.expectedArgs, cmd.Args)
			}
		})
	}
}

func TestSimpleLanguageCommandBuilder_BuildRustCommand(t *testing.T) {
	builder := NewSimpleLanguageCommandBuilder()

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedExec string
		expectedArgs []string
	}{
		{
			name:         "rust source file",
			entry:        "main.rs",
			args:         []string{"-O"},
			expectedExec: "rustc",
			expectedArgs: []string{"rustc", "main.rs", "-O"},
		},
		{
			name:         "rust executable",
			entry:        "rustfmt",
			args:         []string{"--check"},
			expectedExec: "rustfmt",
			expectedArgs: []string{"rustfmt", "--check"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.BuildLanguageCommand("rust", tt.entry, tt.args)

			if cmd == nil {
				t.Fatal("Expected non-nil command")
			}

			if filepath.Base(cmd.Path) != tt.expectedExec {
				t.Errorf("Expected command path %s, got %s", tt.expectedExec, filepath.Base(cmd.Path))
			}

			if !reflect.DeepEqual(cmd.Args, tt.expectedArgs) {
				t.Errorf("Expected args %v, got %v", tt.expectedArgs, cmd.Args)
			}
		})
	}
}

func TestSimpleLanguageCommandBuilder_RegisterLanguage(t *testing.T) {
	builder := NewSimpleLanguageCommandBuilder()

	customTemplate := &CommandTemplate{
		Executable:       "custom-exec",
		UseEntryAsScript: true,
		ScriptExtensions: []string{".custom"},
	}

	builder.RegisterLanguage("custom", customTemplate)

	template, exists := builder.templates["custom"]
	if !exists {
		t.Error("Expected custom language to be registered")
	}

	if template != customTemplate {
		t.Error("Expected registered template to match provided template")
	}

	// Test using the custom language
	cmd := builder.BuildLanguageCommand("custom", "script.custom", []string{"arg1"})
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"custom-exec", "script.custom", "arg1"}
	if !reflect.DeepEqual(cmd.Args, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, cmd.Args)
	}
}

func TestSimpleLanguageCommandBuilder_GetSupportedLanguages(t *testing.T) {
	builder := NewSimpleLanguageCommandBuilder()

	languages := builder.GetSupportedLanguages()

	if len(languages) == 0 {
		t.Error("Expected at least some supported languages")
	}

	// Check that all expected languages are present
	expectedLanguages := []string{"ruby", "perl", "lua", "swift", "r", "haskell", "go", "rust"}
	languageSet := make(map[string]bool)
	for _, lang := range languages {
		languageSet[lang] = true
	}

	for _, expected := range expectedLanguages {
		if !languageSet[expected] {
			t.Errorf("Expected language %s to be in supported languages", expected)
		}
	}

	// Test with custom language
	builder.RegisterLanguage("kotlin", &CommandTemplate{
		Executable:       "kotlin",
		UseEntryAsScript: true,
	})

	updatedLanguages := builder.GetSupportedLanguages()
	if len(updatedLanguages) != len(languages)+1 {
		t.Errorf("Expected %d languages after adding custom, got %d", len(languages)+1, len(updatedLanguages))
	}

	// Check that kotlin is now included
	found := slices.Contains(updatedLanguages, "kotlin")
	if !found {
		t.Error("Expected kotlin to be in updated supported languages")
	}
}

func TestCommandTemplate_WithScriptExtensions(t *testing.T) {
	builder := &SimpleLanguageCommandBuilder{
		templates: make(map[string]*CommandTemplate),
	}

	// Register a language with script extensions
	template := &CommandTemplate{
		Executable:       "custom-exec",
		UseEntryAsScript: false,
		ScriptExtensions: []string{".script", ".custom"},
	}

	builder.RegisterLanguage("custom", template)

	tests := []struct {
		name         string
		entry        string
		args         []string
		expectedExec string
		expectedArgs []string
	}{
		{
			name:         "script extension match",
			entry:        "test.script",
			args:         []string{"arg1"},
			expectedExec: "custom-exec",
			expectedArgs: []string{"custom-exec", "run", "test.script", "arg1"},
		},
		{
			name:         "custom extension match",
			entry:        "test.custom",
			args:         []string{},
			expectedExec: "custom-exec",
			expectedArgs: []string{"custom-exec", "run", "test.custom"},
		},
		{
			name:         "no extension match",
			entry:        "test.other",
			args:         []string{"arg1"},
			expectedExec: "test.other",
			expectedArgs: []string{"test.other", "arg1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := builder.BuildLanguageCommand("custom", tt.entry, tt.args)

			if cmd == nil {
				t.Fatal("Expected non-nil command")
			}

			if filepath.Base(cmd.Path) != tt.expectedExec {
				t.Errorf("Expected command path %s, got %s", tt.expectedExec, filepath.Base(cmd.Path))
			}

			if !reflect.DeepEqual(cmd.Args, tt.expectedArgs) {
				t.Errorf("Expected args %v, got %v", tt.expectedArgs, cmd.Args)
			}
		})
	}
}

func TestSimpleLanguageCommandBuilder_EdgeCases(t *testing.T) {
	builder := NewSimpleLanguageCommandBuilder()

	// Test with empty args
	cmd := builder.BuildLanguageCommand("ruby", "script.rb", []string{})
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"ruby", "script.rb"}
	if !reflect.DeepEqual(cmd.Args, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, cmd.Args)
	}

	// Test with nil args
	cmd = builder.BuildLanguageCommand("ruby", "script.rb", nil)
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs = []string{"ruby", "script.rb"}
	if !reflect.DeepEqual(cmd.Args, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, cmd.Args)
	}

	// Test go command with complex entry
	cmd = builder.BuildLanguageCommand("go", "go test -v ./...", []string{})
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs = []string{"go", "test", "-v", "./..."}
	if !reflect.DeepEqual(cmd.Args, expectedArgs) {
		t.Errorf("Expected args %v, got %v", expectedArgs, cmd.Args)
	}
}
