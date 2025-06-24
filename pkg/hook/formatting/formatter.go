// Package formatting handles result formatting and output display
package formatting

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
)

// Color definitions for hook output
var (
	// Status colors
	PassedColor  = color.New(color.BgGreen, color.FgBlack)
	FailedColor  = color.New(color.BgRed, color.FgWhite)
	SkippedColor = color.New(color.BgCyan, color.FgBlack)

	// Detail color (dim light gray)
	DetailColor = color.New(color.Faint, color.FgWhite) // Dimmed light gray
)

// Formatter handles formatting and displaying hook execution results
type Formatter struct {
	colorMode string
	verbose   bool
}

// NewFormatter creates a new result formatter
func NewFormatter(colorMode string, verbose bool) *Formatter {
	return &Formatter{
		colorMode: colorMode,
		verbose:   verbose,
	}
}

// PrintResults prints hook execution results with appropriate formatting
func (f *Formatter) PrintResults(results []execution.Result) {
	var passed, failed, skipped int

	// Configure color based on mode
	shouldUseColor := f.shouldEnableColor()
	color.NoColor = !shouldUseColor

	for _, result := range results {
		// Get hook name, fallback to ID if name not set
		hookName := result.Hook.Name
		if hookName == "" {
			hookName = result.Hook.ID
		}

		// Create dots to fill up to 79 characters total width (like Python pre-commit)
		// This accounts for status text like "Passed" (6 chars) or "Failed" (6 chars)
		totalWidth := 79
		statusWidth := 6 // Length of "Passed" or "Failed"
		dotsLength := max(totalWidth-len(hookName)-statusWidth,
			// Always have at least one dot
			1)
		dots := strings.Repeat(".", dotsLength)

		// Determine result status for switch
		var resultStatus string
		switch {
		case result.Skipped:
			resultStatus = "skipped"
		case result.Success:
			resultStatus = "success"
		default:
			resultStatus = "failed"
		}

		switch resultStatus {
		case "skipped":
			skipped++
			f.printSkippedResult(result, hookName, shouldUseColor)
		case "success":
			passed++
			f.printSuccessResult(result, hookName, dots, shouldUseColor)
		case "failed":
			failed++
			f.printFailureResult(result, hookName, dots, shouldUseColor)
		}
	}

	// Final summary - no extra newline needed, matches Python pre-commit behavior
}

// PrintResultsLegacy provides backward compatibility with the old Result type
// This function will be moved to the hook package for easier migration
func PrintResultsLegacy(results []LegacyResult, verbose bool, colorMode string) {
	formatter := NewFormatter(colorMode, verbose)
	execResults := make([]execution.Result, len(results))

	for i, result := range results {
		execResults[i] = execution.Result{
			Output:   result.Output,
			Error:    result.Error,
			Files:    result.Files,
			Hook:     result.Hook,
			Duration: result.Duration,
			ExitCode: result.ExitCode,
			Success:  result.Success,
			Timeout:  result.Timeout,
			Skipped:  result.Skipped,
		}
	}

	formatter.PrintResults(execResults)
}

// LegacyResult represents the legacy Result type for backward compatibility
type LegacyResult struct {
	Output   string
	Error    string
	Files    []string
	Hook     config.Hook
	Duration time.Duration
	ExitCode int
	Success  bool
	Timeout  bool
	Skipped  bool
}

// printSuccessResult prints a successful hook result
func (f *Formatter) printSuccessResult(
	result execution.Result,
	hookName, dots string,
	shouldUseColor bool,
) {
	if len(result.Files) == 0 && !result.Hook.AlwaysRun {
		return
	}

	if shouldUseColor {
		fmt.Printf("%s%s%s\n", hookName, dots, PassedColor.Sprint("Passed"))
	} else {
		fmt.Printf("%s%sPassed\n", hookName, dots)
	}

	// In non-verbose mode, only show details for failed hooks (matches Python pre-commit)
	if !f.verbose {
		return
	}

	// In verbose mode, show details and output for successful hooks
	f.printHookDetails(result.Hook.ID, result.Duration, shouldUseColor)

	if result.Output != "" {
		// Python pre-commit format: blank line before output, output, blank line after
		fmt.Printf("\n%s\n\n", strings.TrimSpace(result.Output))
	}
}

// printFailureResult prints a failed hook result
func (f *Formatter) printFailureResult(
	result execution.Result,
	hookName, dots string,
	shouldUseColor bool,
) {
	statusText := "Failed"
	if result.Timeout {
		statusText = "Failed (timeout)"
	}

	if shouldUseColor {
		fmt.Printf("%s%s%s\n", hookName, dots, FailedColor.Sprint(statusText))
	} else {
		fmt.Printf("%s%s%s\n", hookName, dots, statusText)
	}

	// Always show failure details
	f.printFailureDetails(result, shouldUseColor)

	if result.Output != "" {
		fmt.Printf("\n%s\n\n", f.formatHookOutput(result.Output, shouldUseColor))
	}
}

// printSkippedResult prints a skipped hook result
func (f *Formatter) printSkippedResult(
	result execution.Result,
	hookName string,
	shouldUseColor bool,
) {
	// For skipped hooks, Python shows "(no files to check)Skipped" with only "Skipped" colored
	// We need to recalculate dots to account for the extra text
	prefixText := "(no files to check)"
	skippedText := "Skipped"
	fullText := prefixText + skippedText
	totalWidth := 79
	dotsLength := max(totalWidth-len(hookName)-len(fullText),
		// Always have at least one dot
		1)
	skippedDots := strings.Repeat(".", dotsLength)

	// Print with only "Skipped" colored (matching Python pre-commit)
	if shouldUseColor {
		fmt.Printf(
			"%s%s%s%s\n",
			hookName,
			skippedDots,
			prefixText,
			SkippedColor.Sprint(skippedText),
		)
	} else {
		fmt.Printf("%s%s%s%s\n", hookName, skippedDots, prefixText, skippedText)
	}

	if !f.verbose {
		return
	}

	// In verbose mode, always show hook id for skipped hooks (like Python)
	if shouldUseColor {
		fmt.Printf("%s\n", DetailColor.Sprintf("- hook id: %s", result.Hook.ID))
	} else {
		fmt.Printf("- hook id: %s\n", result.Hook.ID)
	}
}

// printHookDetails prints hook metadata with optional color
// Used for success cases where verbose OR hook.verbose is true
func (f *Formatter) printHookDetails(hookID string, duration time.Duration, shouldUseColor bool) {
	formattedDuration := f.formatDuration(duration)
	if shouldUseColor {
		fmt.Printf("%s\n", DetailColor.Sprintf("- hook id: %s", hookID))
		fmt.Printf("%s\n", DetailColor.Sprintf("- duration: %s", formattedDuration))
	} else {
		fmt.Printf("- hook id: %s\n", hookID)
		fmt.Printf("- duration: %s\n", formattedDuration)
	}
}

// printFailureDetails prints failure-specific details with optional color
func (f *Formatter) printFailureDetails(result execution.Result, shouldUseColor bool) {
	if shouldUseColor {
		f.printFailureDetailsColored(result)
	} else {
		f.printFailureDetailsPlain(result)
	}

	if result.Error != "" {
		if shouldUseColor {
			fmt.Printf("%s\n", DetailColor.Sprintf("- error: %s", result.Error))
		} else {
			fmt.Printf("- error: %s\n", result.Error)
		}
	}
}

// printFailureDetailsColored prints failure details with color
func (f *Formatter) printFailureDetailsColored(result execution.Result) {
	formattedDuration := f.formatDuration(result.Duration)
	fmt.Printf("%s\n", DetailColor.Sprintf("- hook id: %s", result.Hook.ID))

	// Duration: show when (verbose OR hook.verbose)
	if f.verbose || result.Hook.Verbose {
		if result.Timeout {
			fmt.Printf("%s\n", DetailColor.Sprintf("- duration: %s (timeout)", formattedDuration))
		} else {
			fmt.Printf("%s\n", DetailColor.Sprintf("- duration: %s", formattedDuration))
		}
	}

	// Exit code: show when retcode != 0 (for failures, this is always true)
	if result.ExitCode != 0 {
		fmt.Printf("%s\n", DetailColor.Sprintf("- exit code: %d", result.ExitCode))
	}
}

// printFailureDetailsPlain prints failure details without color
func (f *Formatter) printFailureDetailsPlain(result execution.Result) {
	formattedDuration := f.formatDuration(result.Duration)
	fmt.Printf("- hook id: %s\n", result.Hook.ID)

	// Duration: show when (verbose OR hook.verbose)
	if f.verbose || result.Hook.Verbose {
		if result.Timeout {
			fmt.Printf("- duration: %s (timeout)\n", formattedDuration)
		} else {
			fmt.Printf("- duration: %s\n", formattedDuration)
		}
	}

	// Exit code: show when retcode != 0 (for failures, this is always true)
	if result.ExitCode != 0 {
		fmt.Printf("- exit code: %d\n", result.ExitCode)
	}
}

// formatHookOutput formats hook output with proper colors for special lines
func (f *Formatter) formatHookOutput(output string, shouldUseColor bool) string {
	// Don't completely strip - just trim trailing whitespace/newlines to avoid double spacing
	output = strings.TrimRight(output, "\n\r\t ")

	if !shouldUseColor {
		return output
	}

	lines := strings.Split(output, "\n")
	var formattedLines []string

	for _, line := range lines {
		// Preserve empty lines and whitespace formatting
		if line == "" {
			formattedLines = append(formattedLines, "")
			continue
		}

		trimmedLine := strings.TrimSpace(line)

		// Check if this line should be in dark gray (muted color)
		if strings.HasPrefix(trimmedLine, "- files were modified by this hook") ||
			strings.HasPrefix(trimmedLine, "- hook id:") ||
			strings.HasPrefix(trimmedLine, "- duration:") ||
			strings.HasPrefix(trimmedLine, "- error:") {
			// Apply dark gray color using fatih/color only to these special lines
			formattedLines = append(formattedLines, DetailColor.Sprint(trimmedLine))
		} else {
			// Keep original formatting for hook output (preserves black's colors/bold and indentation)
			formattedLines = append(formattedLines, line)
		}
	}

	return strings.Join(formattedLines, "\n")
}

// shouldEnableColor determines if color output should be used based on the color mode setting
func (f *Formatter) shouldEnableColor() bool {
	switch f.colorMode {
	case "always":
		return true
	case "never":
		return false
	case "auto":
		// Check if stdout is a terminal
		return !color.NoColor // fatih/color package auto-detects terminals
	default:
		// Check if stdout is a terminal
		return !color.NoColor // fatih/color package auto-detects terminals
	}
}

// formatDuration formats duration to match Python pre-commit's style
// Python rounds very fast operations to "0s" and shows longer ones with appropriate precision
func (f *Formatter) formatDuration(duration time.Duration) string {
	// Convert to seconds for comparison
	seconds := duration.Seconds()

	switch {
	case seconds < 0.005: // Less than 5ms shows as 0s like Python
		return "0s"
	case seconds < 1.0: // Less than 1s shows as milliseconds with 2 decimal places
		return fmt.Sprintf("%.2fs", seconds)
	case seconds < 60.0: // Less than 1 minute shows as seconds with 1 decimal place
		return fmt.Sprintf("%.1fs", seconds)
	default: // 1 minute or more shows minutes and seconds
		minutes := int(seconds) / 60
		remainingSeconds := int(seconds) % 60
		return fmt.Sprintf("%dm%ds", minutes, remainingSeconds)
	}
}
