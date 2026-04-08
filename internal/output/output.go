// Package output provides colored terminal output and hook result formatting.
package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles for terminal output.
var (
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

// ColorMode controls when colors are used.
type ColorMode int

const (
	ColorAuto   ColorMode = iota
	ColorAlways
	ColorNever
)

var currentColorMode = ColorAuto

// SetColorMode sets the global color mode.
func SetColorMode(mode ColorMode) {
	currentColorMode = mode
}

// SetColorModeFromString parses a color mode string.
func SetColorModeFromString(s string) {
	switch strings.ToLower(s) {
	case "always":
		currentColorMode = ColorAlways
	case "never":
		currentColorMode = ColorNever
	default:
		currentColorMode = ColorAuto
	}
}

// UseColor returns whether color output is enabled.
func UseColor() bool {
	switch currentColorMode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	default:
		// Auto: check if stdout is a terminal and TERM is not "dumb".
		if os.Getenv("TERM") == "dumb" {
			return false
		}
		if os.Getenv("PRE_COMMIT_COLOR") != "" {
			return SetColorFromEnv()
		}
		// Check if stdout is a terminal.
		fi, err := os.Stdout.Stat()
		if err != nil {
			return false
		}
		return fi.Mode()&os.ModeCharDevice != 0
	}
}

// SetColorFromEnv reads PRE_COMMIT_COLOR env var.
func SetColorFromEnv() bool {
	v := os.Getenv("PRE_COMMIT_COLOR")
	switch strings.ToLower(v) {
	case "always", "1", "true":
		return true
	case "never", "0", "false":
		return false
	default:
		return true
	}
}

func render(style lipgloss.Style, text string) string {
	if !UseColor() {
		return text
	}
	return style.Render(text)
}

// HookResult represents the outcome of running a hook.
type HookResult int

const (
	ResultPassed  HookResult = iota
	ResultFailed
	ResultSkipped
	ResultError
)

// String returns the string representation of a HookResult.
func (r HookResult) String() string {
	switch r {
	case ResultPassed:
		return "Passed"
	case ResultFailed:
		return "Failed"
	case ResultSkipped:
		return "Skipped"
	case ResultError:
		return "Error"
	default:
		return "Unknown"
	}
}

func coloredResult(result HookResult) string {
	switch result {
	case ResultPassed:
		return render(greenStyle, "Passed")
	case ResultFailed:
		return render(redStyle, "Failed")
	case ResultSkipped:
		return render(yellowStyle, "Skipped")
	case ResultError:
		return render(redStyle, "Error")
	default:
		return "Unknown"
	}
}

// PrintHookHeader prints a hook execution header line.
// Format: "Hook Name...................................................Result"
func PrintHookHeader(name string, result HookResult) {
	totalWidth := TerminalWidth()
	nameLen := len(name)
	resultStr := result.String()
	resultLen := len(resultStr)
	dotsLen := totalWidth - nameLen - resultLen
	if dotsLen < 1 {
		dotsLen = 1
	}
	dots := strings.Repeat(".", dotsLen)
	fmt.Fprintf(os.Stderr, "%s%s%s\n", name, dots, coloredResult(result))
}

// PrintHookOutput prints hook output with optional indentation.
func PrintHookOutput(output []byte, hookID string, exitCode int, verbose bool) {
	if len(output) == 0 && !verbose {
		return
	}

	if exitCode != 0 || verbose {
		fmt.Fprintf(os.Stderr, "- hook id: %s\n", hookID)
		if exitCode != 0 {
			fmt.Fprintf(os.Stderr, "- exit code: %d\n", exitCode)
		}
	}

	if len(output) > 0 {
		outStr := string(output)
		// Check if files were modified.
		if strings.Contains(outStr, "Files were modified by this hook") {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, render(yellowStyle, "Files were modified by this hook. Additional output:"))
			fmt.Fprintln(os.Stderr)
		}
		fmt.Fprint(os.Stderr, outStr)
		if !strings.HasSuffix(outStr, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}
}

// TerminalWidth returns the terminal width, defaulting to 80.
func TerminalWidth() int {
	// Try to get terminal width from environment.
	if cols := os.Getenv("COLUMNS"); cols != "" {
		n := 0
		for _, c := range cols {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			}
		}
		if n > 0 {
			return n
		}
	}
	// Default.
	return 80
}

// Info prints an informational message.
func Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", render(cyanStyle, "INFO"), msg)
}

// Warn prints a warning message.
func Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[%s] %s\n", render(yellowStyle, "WARNING"), msg)
}

// Error prints an error message.
func Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s\n", render(redStyle, "ERROR"), msg)
}

// PrintSeparator prints a separator line.
func PrintSeparator() {
	fmt.Println(strings.Repeat("=", 79))
}
