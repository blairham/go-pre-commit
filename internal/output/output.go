// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package output provides colored terminal output and hook result formatting.
package output

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/width"
)

// Styles for terminal output.
var (
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	subtleStyle = lipgloss.NewStyle().Faint(true)
)

// ColorMode controls when colors are used.
type ColorMode int

const (
	ColorAuto ColorMode = iota
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
	ResultPassed HookResult = iota
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

// The hook report mirrors upstream's commands/run.py line for line: the
// status line's width and postfix, the per-hook block under it, and stdout as
// the stream for all of it. Comparing only the Passed/Failed word let every one
// of those drift (#86); the parity suite now compares the report byte for byte.

const (
	noFilesPostfix = "(no files to check)"
	skippedMsg     = "Skipped"
)

// ReportCols is upstream's _compute_cols: wide enough for the widest line that
// can appear, "<longest name>...(no files to check)Skipped", and never under 80.
func ReportCols(names []string) int {
	nameLen := 0
	for _, n := range names {
		nameLen = max(nameLen, displayWidth(n))
	}
	return max(nameLen+3+len(noFilesPostfix)+1+len(skippedMsg), 80)
}

// displayWidth is upstream's _len_cjk: East Asian wide and fullwidth runes
// count two columns, everything else one.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			n += 2
		default:
			n++
		}
	}
	return n
}

// dots pads a status line so it ends one column short of cols, as upstream's
// _full_msg and _start_msg do.
func dots(cols int, start, postfix string, endLen int) string {
	return strings.Repeat(".", max(cols-displayWidth(start)-len(postfix)-endLen-1, 0))
}

// PrintHookStart prints a hook's name and dots before it runs, so a slow hook
// shows what is running; PrintHookStatus finishes the line.
func PrintHookStart(name string, cols int) {
	fmt.Print(name + dots(cols, name, "", len("Passed")))
}

// PrintHookStatus finishes the line PrintHookStart began.
func PrintHookStatus(passed bool) {
	if passed {
		fmt.Println(coloredResult(ResultPassed))
	} else {
		fmt.Println(coloredResult(ResultFailed))
	}
}

// PrintHookSkipped prints a skipped hook's whole line: "(no files to check)"
// before Skipped when no file matched, nothing when SKIP named it.
func PrintHookSkipped(name string, cols int, noFiles bool) {
	postfix := ""
	if noFiles {
		postfix = noFilesPostfix
	}
	fmt.Println(name + dots(cols, name, postfix, len(skippedMsg)) + postfix + coloredResult(ResultSkipped))
}

// PrintHookHeader prints a complete status line for the paths upstream has no
// equivalent of (a hook this tool refuses before running it).
func PrintHookHeader(name string, cols int, result HookResult) {
	fmt.Println(name + dots(cols, name, "", len(result.String())) + coloredResult(result))
}

// HookDetails is what upstream prints under a hook's status line.
type HookDetails struct {
	ID            string
	Verbose       bool           // --verbose, or the hook's own verbose: true
	Duration      *time.Duration // nil when the hook did not run
	ExitCode      int
	FilesModified bool
	Output        []byte
	LogFile       string // hook's log_file: also receives the output
}

// PrintHookDetails prints the block under a status line, when upstream would:
// in verbose mode, or when the hook failed.
func PrintHookDetails(d HookDetails) {
	if !d.Verbose && d.ExitCode == 0 && !d.FilesModified {
		return
	}
	fmt.Println(render(subtleStyle, "- hook id: "+d.ID))
	if d.Verbose && d.Duration != nil {
		fmt.Println(render(subtleStyle, "- duration: "+formatDuration(*d.Duration)+"s"))
	}
	if d.ExitCode != 0 {
		fmt.Println(render(subtleStyle, fmt.Sprintf("- exit code: %d", d.ExitCode)))
	}
	if d.FilesModified {
		fmt.Println(render(subtleStyle, "- files were modified by this hook"))
	}
	out := bytes.TrimSpace(d.Output)
	if len(out) == 0 {
		return
	}
	fmt.Println()
	os.Stdout.Write(append(out, '\n'))
	fmt.Println()
	if d.LogFile != "" {
		// Upstream appends exactly the lines it printed, and only these.
		if f, err := os.OpenFile(d.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			_, _ = f.Write(append(out, '\n'))
			_ = f.Close()
		}
	}
}

// formatDuration renders seconds as upstream does, `round(t, 2) or 0` printed
// by Python: "0" for nothing measurable, otherwise a float repr such as "0.01",
// "1.5" or "2.0".
func formatDuration(d time.Duration) string {
	secs := math.Round(d.Seconds()*100) / 100
	if secs == 0 {
		return "0"
	}
	s := strconv.FormatFloat(secs, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// Info prints an informational message.
func Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("[INFO] %s\n", msg)
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
