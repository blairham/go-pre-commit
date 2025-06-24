// Package commands provides improved language command building with reduced duplication
package commands

import (
	"os/exec"
	"strings"
)

// CommandTemplate represents a template for building language commands
type CommandTemplate struct {
	CustomBuilder    func(entry string, args []string) *exec.Cmd
	Executable       string
	ScriptExtensions []string
	UseEntryAsScript bool
}

// SimpleLanguageCommandBuilder builds commands for languages that follow simple patterns
type SimpleLanguageCommandBuilder struct {
	templates map[string]*CommandTemplate
}

// NewSimpleLanguageCommandBuilder creates a new builder with common language templates
func NewSimpleLanguageCommandBuilder() *SimpleLanguageCommandBuilder {
	builder := &SimpleLanguageCommandBuilder{
		templates: make(map[string]*CommandTemplate),
	}

	// Register common language patterns
	builder.registerCommonLanguages()
	return builder
}

// registerCommonLanguages sets up templates for common scripting languages
func (slcb *SimpleLanguageCommandBuilder) registerCommonLanguages() {
	// Scripting languages that run source files directly
	scriptingLanguages := map[string]string{
		"ruby":    "ruby",
		"perl":    "perl",
		"lua":     "lua",
		"swift":   "swift",
		"r":       "Rscript",
		"haskell": "runhaskell",
	}

	for lang, executable := range scriptingLanguages {
		slcb.templates[lang] = &CommandTemplate{
			Executable:       executable,
			UseEntryAsScript: true,
			ScriptExtensions: []string{},
		}
	}

	// Special cases with custom logic
	slcb.templates["go"] = &CommandTemplate{
		Executable:       "go",
		UseEntryAsScript: false,
		ScriptExtensions: []string{".go"},
		CustomBuilder:    slcb.buildGoCommand,
	}

	slcb.templates["rust"] = &CommandTemplate{
		Executable:       "rustc",
		UseEntryAsScript: false,
		ScriptExtensions: []string{".rs"},
		CustomBuilder:    slcb.buildRustCommand,
	}
}

// BuildLanguageCommand builds a command using the registered templates
func (slcb *SimpleLanguageCommandBuilder) BuildLanguageCommand(
	language, entry string, args []string,
) *exec.Cmd {
	template, exists := slcb.templates[language]
	if !exists {
		// Fallback: treat as direct executable
		return exec.Command(entry, args...)
	}

	// Use custom builder if available
	if template.CustomBuilder != nil {
		return template.CustomBuilder(entry, args)
	}

	// Use template-based building
	if template.UseEntryAsScript {
		cmdArgs := append([]string{entry}, args...)
		return exec.Command(template.Executable, cmdArgs...)
	}

	// Check if entry should be treated as a script
	for _, ext := range template.ScriptExtensions {
		if strings.HasSuffix(entry, ext) {
			cmdArgs := append([]string{"run", entry}, args...)
			return exec.Command(template.Executable, cmdArgs...)
		}
	}

	// Default: use entry as executable name
	return exec.Command(entry, args...)
}

// buildGoCommand handles Go-specific command building logic
func (slcb *SimpleLanguageCommandBuilder) buildGoCommand(entry string, args []string) *exec.Cmd {
	if strings.HasPrefix(entry, "go ") {
		// Handle "go run", "go build", etc.
		parts := strings.Fields(entry)
		goArgs := parts[1:] // Skip "go"
		goArgs = append(goArgs, args...)
		return exec.Command("go", goArgs...)
	}

	// Go script file - use "go run"
	if strings.HasSuffix(entry, ".go") {
		cmdArgs := append([]string{"run", entry}, args...)
		return exec.Command("go", cmdArgs...)
	}

	return exec.Command(entry, args...)
}

// buildRustCommand handles Rust-specific command building logic
func (slcb *SimpleLanguageCommandBuilder) buildRustCommand(entry string, args []string) *exec.Cmd {
	if strings.HasSuffix(entry, ".rs") {
		// Rust source file - compile and run
		cmdArgs := append([]string{entry}, args...)
		return exec.Command("rustc", cmdArgs...)
	}
	return exec.Command(entry, args...)
}

// RegisterLanguage allows registering custom language templates
func (slcb *SimpleLanguageCommandBuilder) RegisterLanguage(
	language string, template *CommandTemplate,
) {
	slcb.templates[language] = template
}

// GetSupportedLanguages returns a list of supported language identifiers
func (slcb *SimpleLanguageCommandBuilder) GetSupportedLanguages() []string {
	languages := make([]string, 0, len(slcb.templates))
	for lang := range slcb.templates {
		languages = append(languages, lang)
	}
	return languages
}
