package commands

import (
	"fmt"
	"strings"

	"github.com/jessevdk/go-flags"
)

// HelpFormatter provides standardized help formatting for all commands
type HelpFormatter struct {
	Command     string
	Description string
	Usage       string
	Examples    []Example
	Notes       []string
}

// Example represents a command example
type Example struct {
	Command     string
	Description string
}

// FormatHelp generates standardized help text for a command
func (h *HelpFormatter) FormatHelp(parser *flags.Parser) string {
	var result strings.Builder

	// Usage line in argparse format (lowercase)
	result.WriteString(fmt.Sprintf("usage: pre-commit %s %s\n\n", h.Command, parser.Usage))

	// Add positional arguments section if present (before options)
	if len(h.Notes) > 0 {
		for _, note := range h.Notes {
			result.WriteString(note)
			result.WriteString("\n")
		}
		result.WriteString("\n")
	}

	// Options section header (lowercase to match argparse)
	result.WriteString("options:\n")

	// Get the auto-generated options
	var helpBuf strings.Builder
	parser.WriteHelp(&helpBuf)
	helpText := helpBuf.String()

	// Extract and reformat options to match Python's argparse style
	lines := strings.Split(helpText, "\n")
	inOptions := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Detect when we enter the options section
		if strings.Contains(line, "Application Options:") {
			inOptions = true
			continue
		}

		// Skip help-related lines
		if strings.Contains(line, "Help Options:") {
			break
		}

	// Process option lines with normalized indentation
	if inOptions && strings.TrimSpace(line) != "" {
		// Replace [auto|always|never] with {auto,always,never} to match Python
		line = strings.ReplaceAll(line, "[auto|always|never]", "{auto,always,never}")

		// Replace placeholders with proper quotes
		line = strings.ReplaceAll(line, "BTICK_", "`")
		line = strings.ReplaceAll(line, "_BTICK", "`")
		line = strings.ReplaceAll(line, "DQUOTE_", "\"")
		line = strings.ReplaceAll(line, "_DQUOTE", "\"")

		// Normalize to 2-space base indent while preserving internal spacing
		trimmed := strings.TrimLeft(line, " ")
		// Count the original indent to calculate relative spacing
		originalIndent := len(line) - len(trimmed)

		// If this line had more than base indent, preserve extra spacing for continuation
		if originalIndent > 2 {
			// This is likely a continuation line, keep the extra indent
			result.WriteString("  ")
			result.WriteString(strings.Repeat(" ", originalIndent-2))
			result.WriteString(trimmed)
		} else {
			// Normal option line, use 2-space indent
			result.WriteString("  ")
			result.WriteString(trimmed)
		}
			result.WriteString("\n")
		}
	}

	return result.String()
}

// StandardSynopsis provides a consistent synopsis format
func StandardSynopsis(description string) string {
	return description
}

// CommonExamples provides common examples that many commands use
var CommonExamples = struct {
	Verbose  Example
	Config   Example
	DryRun   Example
	Help     Example
	AllFiles Example
}{
	Verbose:  Example{Command: "--verbose", Description: "Show detailed output"},
	Config:   Example{Command: "--config custom.yaml", Description: "Use custom config file"},
	DryRun:   Example{Command: "--dry-run", Description: "Show what would be done"},
	Help:     Example{Command: "--help", Description: "Show help message"},
	AllFiles: Example{Command: "--all-files", Description: "Run on all files"},
}
