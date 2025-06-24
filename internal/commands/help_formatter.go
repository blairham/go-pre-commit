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

	// Command description
	if h.Description != "" {
		result.WriteString(fmt.Sprintf("%s\n\n", h.Description))
	}

	// Examples section
	if len(h.Examples) > 0 {
		result.WriteString("Examples:\n")
		for _, example := range h.Examples {
			if example.Description != "" {
				result.WriteString(
					fmt.Sprintf("  %s  # %s\n", example.Command, example.Description),
				)
			} else {
				result.WriteString(fmt.Sprintf("  %s\n", example.Command))
			}
		}
		result.WriteString("\n")
	}

	// Notes section
	if len(h.Notes) > 0 {
		result.WriteString("Notes:\n")
		for _, note := range h.Notes {
			result.WriteString(fmt.Sprintf("  • %s\n", note))
		}
		result.WriteString("\n")
	}

	// Auto-generated options help
	var helpBuf strings.Builder
	parser.WriteHelp(&helpBuf)
	result.WriteString(helpBuf.String())

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
