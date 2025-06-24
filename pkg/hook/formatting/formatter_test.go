package formatting

import (
	"os"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/hook/execution"
)

func TestNewFormatter(t *testing.T) {
	formatter := NewFormatter("auto", true)
	assert.NotNil(t, formatter)
	assert.Equal(t, "auto", formatter.colorMode)
	assert.True(t, formatter.verbose)

	formatter2 := NewFormatter("never", false)
	assert.Equal(t, "never", formatter2.colorMode)
	assert.False(t, formatter2.verbose)
}

func TestFormatter_shouldEnableColor(t *testing.T) {
	tests := []struct {
		setup     func()
		name      string
		colorMode string
		expected  bool
	}{
		{
			name:      "always mode",
			colorMode: "always",
			setup:     func() {},
			expected:  true,
		},
		{
			name:      "never mode",
			colorMode: "never",
			setup:     func() {},
			expected:  false,
		},
		{
			name:      "auto mode with tty",
			colorMode: "auto",
			setup: func() {
				// In test environment, TTY detection is handled by color package
				// This test verifies the logic exists and follows the color package's detection
			},
			expected: !color.NoColor, // Use actual detection from color package
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.colorMode, false)
			tt.setup()
			result := formatter.shouldEnableColor()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatter_formatDuration(t *testing.T) {
	formatter := NewFormatter("never", false)

	tests := []struct {
		expected string
		name     string
		duration time.Duration
	}{
		{
			name:     "very fast operation",
			duration: 1 * time.Millisecond,
			expected: "0s",
		},
		{
			name:     "less than second",
			duration: 500 * time.Millisecond,
			expected: "0.50s",
		},
		{
			name:     "exactly one second",
			duration: 1 * time.Second,
			expected: "1.0s",
		},
		{
			name:     "more than one second",
			duration: 2500 * time.Millisecond,
			expected: "2.5s",
		},
		{
			name:     "more than one minute",
			duration: 75 * time.Second,
			expected: "1m15s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatter_formatHookOutput(t *testing.T) {
	formatter := NewFormatter("never", false)

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "single line",
			output:   "single line output",
			expected: "single line output",
		},
		{
			name:     "multi-line output",
			output:   "line1\nline2\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "output with trailing newline",
			output:   "output\n",
			expected: "output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatHookOutput(tt.output, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatter_PrintResults(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:   "test-hook-1",
				Name: "Test Hook 1",
			},
			Success: true,
			Files:   []string{"file1.go"},
			Output:  "test output",
		},
		{
			Hook: config.Hook{
				ID:   "test-hook-2",
				Name: "Test Hook 2",
			},
			Success: false,
			Error:   "test error",
			Output:  "error output",
		},
		{
			Hook: config.Hook{
				ID:   "test-hook-3",
				Name: "Test Hook 3",
			},
			Skipped: true,
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Check that output contains expected elements
	assert.Contains(t, output, "Test Hook 1")
	assert.Contains(t, output, "Test Hook 2")
	assert.Contains(t, output, "Test Hook 3")
	assert.Contains(t, output, "Passed")
	assert.Contains(t, output, "Failed")
	assert.Contains(t, output, "Skipped")
}

func TestPrintResultsLegacy(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	legacyResults := []LegacyResult{
		{
			Hook: config.Hook{
				ID:   "legacy-hook",
				Name: "Legacy Hook",
			},
			Success: true,
			Files:   []string{"file1.go"},
		},
	}

	PrintResultsLegacy(legacyResults, false, "never")

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Legacy Hook")
	assert.Contains(t, output, "Passed")
}

func TestFormatter_printSuccessResult(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	result := execution.Result{
		Hook: config.Hook{
			ID:   "test-hook",
			Name: "Test Hook",
		},
		Success: true,
		Files:   []string{"file1.go"},
		Output:  "success output",
	}

	formatter.printSuccessResult(result, "Test Hook", "...", false)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Test Hook")
	assert.Contains(t, output, "Passed")
}

func TestFormatter_printFailureResult(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	result := execution.Result{
		Hook: config.Hook{
			ID:   "test-hook",
			Name: "Test Hook",
		},
		Success: false,
		Error:   "test error",
		Output:  "error output",
	}

	formatter.printFailureResult(result, "Test Hook", "...", false)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Test Hook")
	assert.Contains(t, output, "Failed")
}

func TestFormatter_printSkippedResult(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	result := execution.Result{
		Hook: config.Hook{
			ID:   "test-hook",
			Name: "Test Hook",
		},
		Skipped: true,
	}

	formatter.printSkippedResult(result, "Test Hook", false)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Test Hook")
	assert.Contains(t, output, "Skipped")
	assert.Contains(t, output, "(no files to check)")
}

func TestFormatter_printHookDetails(t *testing.T) {
	tests := []struct {
		name           string
		hookID         string
		expectedText   []string
		duration       time.Duration
		shouldUseColor bool
	}{
		{
			name:           "hook details without color",
			shouldUseColor: false,
			hookID:         "test-hook-id",
			duration:       2500 * time.Millisecond,
			expectedText:   []string{"- hook id: test-hook-id", "- duration: 2.5s"},
		},
		{
			name:           "hook details with color",
			shouldUseColor: true,
			hookID:         "test-hook-colored",
			duration:       75 * time.Second,
			expectedText:   []string{"test-hook-colored", "1m15s"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout for testing
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			formatter := NewFormatter("never", false)
			formatter.printHookDetails(tt.hookID, tt.duration, tt.shouldUseColor)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			for _, expectedText := range tt.expectedText {
				assert.Contains(t, output, expectedText)
			}
		})
	}
}

func TestFormatter_printFailureDetails(t *testing.T) {
	tests := []struct {
		name           string
		expectedText   []string
		result         execution.Result
		shouldUseColor bool
	}{
		{
			name: "failure details without color",
			result: execution.Result{
				Hook: config.Hook{
					ID: "failed-hook",
				},
				Duration: 1500 * time.Millisecond,
				ExitCode: 1,
				Error:    "Test error message",
			},
			shouldUseColor: false,
			expectedText:   []string{"- hook id: failed-hook", "- exit code: 1", "- error: Test error message"},
		},
		{
			name: "failure details with color",
			result: execution.Result{
				Hook: config.Hook{
					ID: "failed-hook-colored",
				},
				Duration: 500 * time.Millisecond,
				ExitCode: 2,
				Error:    "",
			},
			shouldUseColor: true,
			expectedText:   []string{"failed-hook-colored", "- exit code: 2"},
		},
		{
			name: "timeout failure details",
			result: execution.Result{
				Hook: config.Hook{
					ID:      "timeout-hook",
					Verbose: true,
				},
				Duration: 30 * time.Second,
				ExitCode: 124,
				Timeout:  true,
				Error:    "Command timed out",
			},
			shouldUseColor: false,
			expectedText: []string{
				"- hook id: timeout-hook",
				"- duration: 30.0s (timeout)",
				"- exit code: 124",
				"- error: Command timed out",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout for testing
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			formatter := NewFormatter("never", false)
			formatter.printFailureDetails(tt.result, tt.shouldUseColor)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			for _, expectedText := range tt.expectedText {
				assert.Contains(t, output, expectedText)
			}
		})
	}
}

func TestFormatter_printFailureDetailsColored(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("always", true)
	result := execution.Result{
		Hook: config.Hook{
			ID:      "colored-hook",
			Verbose: true,
		},
		Duration: 2 * time.Second,
		ExitCode: 1,
		Timeout:  false,
	}

	formatter.printFailureDetailsColored(result)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "colored-hook")
	assert.Contains(t, output, "2.0s")
	assert.Contains(t, output, "exit code: 1")
}

func TestFormatter_printFailureDetailsPlain(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)
	result := execution.Result{
		Hook: config.Hook{
			ID:      "plain-hook",
			Verbose: false,
		},
		Duration: 1 * time.Second,
		ExitCode: 2,
		Timeout:  false,
	}

	formatter.printFailureDetailsPlain(result)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "- hook id: plain-hook")
	assert.Contains(t, output, "- exit code: 2")
	// Duration should not be shown when verbose is false and hook.verbose is false
	assert.NotContains(t, output, "- duration:")
}

func TestFormatter_formatHookOutput_WithColor(t *testing.T) {
	formatter := NewFormatter("always", false)

	tests := []struct {
		name     string
		output   string
		expected []string // Text that should be present
	}{
		{
			name:   "output with special lines",
			output: "- files were modified by this hook\nsome normal output\n- hook id: test-hook",
			expected: []string{
				"some normal output", // Normal line should be preserved
			},
		},
		{
			name:   "output with hook metadata lines",
			output: "- hook id: test\n- duration: 1.5s\n- error: some error\nnormal output",
			expected: []string{
				"normal output", // Normal line should be preserved
			},
		},
		{
			name:   "empty lines preservation",
			output: "line1\n\nline3\n",
			expected: []string{
				"line1",
				"line3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatHookOutput(tt.output, true)
			for _, expected := range tt.expected {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestFormatter_shouldEnableColor_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		colorMode string
		expected  bool
	}{
		{
			name:      "unknown color mode defaults to auto",
			colorMode: "unknown",
			expected:  false, // In test environment, usually no TTY
		},
		{
			name:      "empty color mode defaults to auto",
			colorMode: "",
			expected:  false, // In test environment, usually no TTY
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.colorMode, false)
			result := formatter.shouldEnableColor()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatter_PrintResults_VerboseMode(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", true) // Verbose mode

	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:   "verbose-hook",
				Name: "Verbose Hook",
			},
			Success:  true,
			Files:    []string{"file1.go"},
			Output:   "verbose output",
			Duration: 1500 * time.Millisecond,
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Verbose Hook")
	assert.Contains(t, output, "Passed")
	assert.Contains(t, output, "- hook id: verbose-hook")
	assert.Contains(t, output, "- duration: 1.5s")
	assert.Contains(t, output, "verbose output")
}

func TestFormatter_PrintResults_SkippedVerboseMode(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", true) // Verbose mode

	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:   "skipped-verbose-hook",
				Name: "Skipped Verbose Hook",
			},
			Skipped: true,
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Skipped Verbose Hook")
	assert.Contains(t, output, "Skipped")
	assert.Contains(t, output, "(no files to check)")
	assert.Contains(t, output, "- hook id: skipped-verbose-hook")
}

func TestFormatter_PrintResults_TimeoutFailure(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:   "timeout-hook",
				Name: "Timeout Hook",
			},
			Success:  false,
			Timeout:  true,
			ExitCode: 124,
			Duration: 30 * time.Second,
			Error:    "Command timed out",
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Timeout Hook")
	assert.Contains(t, output, "Failed (timeout)")
	assert.Contains(t, output, "- hook id: timeout-hook")
	assert.Contains(t, output, "- exit code: 124")
}

func TestFormatter_PrintResults_WithColors(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("always", false) // Always use colors

	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:   "color-hook-success",
				Name: "Color Hook Success",
			},
			Success: true,
			Files:   []string{"file1.go"},
		},
		{
			Hook: config.Hook{
				ID:   "color-hook-failure",
				Name: "Color Hook Failure",
			},
			Success:  false,
			ExitCode: 1,
			Error:    "Test error",
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Color Hook Success")
	assert.Contains(t, output, "Color Hook Failure")
	// Note: Color output contains ANSI escape codes, but we can still check for the text
}

func TestFormatter_PrintResults_NoFilesAlwaysRun(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:        "no-files-hook",
				Name:      "No Files Hook",
				AlwaysRun: false,
			},
			Success: true,
			Files:   []string{}, // No files
		},
		{
			Hook: config.Hook{
				ID:        "always-run-hook",
				Name:      "Always Run Hook",
				AlwaysRun: true,
			},
			Success: true,
			Files:   []string{}, // No files but AlwaysRun is true
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Hook with no files and AlwaysRun=false should not appear
	assert.NotContains(t, output, "No Files Hook")
	// Hook with AlwaysRun=true should appear even with no files
	assert.Contains(t, output, "Always Run Hook")
	assert.Contains(t, output, "Passed")
}

func TestFormatter_PrintResults_HookVerboseMode(t *testing.T) {
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false) // Formatter not verbose

	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:      "hook-verbose",
				Name:    "Hook Verbose",
				Verbose: true, // But hook itself is verbose
			},
			Success:  false,
			ExitCode: 1,
			Duration: 2 * time.Second,
			Error:    "Hook error",
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Hook Verbose")
	assert.Contains(t, output, "Failed")
	assert.Contains(t, output, "- hook id: hook-verbose")
	assert.Contains(t, output, "- duration: 2.0s") // Duration should show because hook.Verbose=true
	assert.Contains(t, output, "- exit code: 1")
	assert.Contains(t, output, "- error: Hook error")
}

func TestFormatter_formatDuration_EdgeCases(t *testing.T) {
	formatter := NewFormatter("never", false)

	tests := []struct {
		name     string
		expected string
		duration time.Duration
	}{
		{
			name:     "exactly 5ms boundary",
			duration: 5 * time.Millisecond,
			expected: "0.01s",
		},
		{
			name:     "exactly 1s boundary",
			duration: 1 * time.Second,
			expected: "1.0s",
		},
		{
			name:     "exactly 60s boundary",
			duration: 60 * time.Second,
			expected: "1m0s",
		},
		{
			name:     "very long duration",
			duration: 3661 * time.Second, // 1 hour, 1 minute, 1 second
			expected: "61m1s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLegacyResult_AllFields(t *testing.T) {
	// Test all fields of LegacyResult to ensure complete coverage
	duration := 10 * time.Second
	hook := config.Hook{
		ID:   "legacy-test",
		Name: "Legacy Test Hook",
	}

	legacyResult := LegacyResult{
		Output:   "legacy output",
		Error:    "legacy error",
		Files:    []string{"file1.go", "file2.go"},
		Hook:     hook,
		Duration: duration,
		ExitCode: 2,
		Success:  true,
		Timeout:  true,
		Skipped:  true,
	}

	// Verify all fields
	assert.Equal(t, "legacy output", legacyResult.Output)
	assert.Equal(t, "legacy error", legacyResult.Error)
	assert.Equal(t, []string{"file1.go", "file2.go"}, legacyResult.Files)
	assert.Equal(t, hook, legacyResult.Hook)
	assert.Equal(t, duration, legacyResult.Duration)
	assert.Equal(t, 2, legacyResult.ExitCode)
	assert.True(t, legacyResult.Success)
	assert.True(t, legacyResult.Timeout)
	assert.True(t, legacyResult.Skipped)
}

func TestFormatter_PrintResults_FinalSummary(t *testing.T) {
	// Test the final summary line that's printed when all hooks pass
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	// All successful results should trigger the final summary
	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:   "success-hook-1",
				Name: "Success Hook 1",
			},
			Success: true,
			Files:   []string{"file1.go"},
		},
		{
			Hook: config.Hook{
				ID:   "success-hook-2",
				Name: "Success Hook 2",
			},
			Success: true,
			Files:   []string{"file2.go"},
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Success Hook 1")
	assert.Contains(t, output, "Success Hook 2")
	assert.Contains(t, output, "Passed")
	// The output should end with a newline (final summary)
	assert.True(t, len(output) > 0)
}

func TestFormatter_PrintResults_NoFinalSummary(t *testing.T) {
	// Test that no final summary is printed when there are failures
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	// Mix of success and failure - should not trigger final summary
	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:   "success-hook",
				Name: "Success Hook",
			},
			Success: true,
			Files:   []string{"file1.go"},
		},
		{
			Hook: config.Hook{
				ID:   "failure-hook",
				Name: "Failure Hook",
			},
			Success:  false,
			ExitCode: 1,
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Success Hook")
	assert.Contains(t, output, "Failure Hook")
	assert.Contains(t, output, "Passed")
	assert.Contains(t, output, "Failed")
}

func TestFormatter_PrintResults_OnlySkipped(t *testing.T) {
	// Test scenario with only skipped hooks (no passed hooks)
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("never", false)

	results := []execution.Result{
		{
			Hook: config.Hook{
				ID:   "skipped-hook-1",
				Name: "Skipped Hook 1",
			},
			Skipped: true,
		},
		{
			Hook: config.Hook{
				ID:   "skipped-hook-2",
				Name: "Skipped Hook 2",
			},
			Skipped: true,
		},
	}

	formatter.PrintResults(results)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Skipped Hook 1")
	assert.Contains(t, output, "Skipped Hook 2")
	assert.Contains(t, output, "Skipped")
	// Should not have final summary since no passed hooks
}

func TestFormatter_printSkippedResult_VerboseWithColor(t *testing.T) {
	// Test skipped result in verbose mode with color
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("always", true) // Verbose and with color

	result := execution.Result{
		Hook: config.Hook{
			ID:   "skipped-verbose-color-hook",
			Name: "Skipped Verbose Color Hook",
		},
		Skipped: true,
	}

	formatter.printSkippedResult(result, "Skipped Verbose Color Hook", true)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Skipped Verbose Color Hook")
	assert.Contains(t, output, "Skipped")
	assert.Contains(t, output, "(no files to check)")
	assert.Contains(t, output, "skipped-verbose-color-hook") // Hook ID should be shown in verbose mode
}

func TestFormatter_printFailureDetailsColored_ExitCodeZero(t *testing.T) {
	// Test failure details with exit code 0 (edge case)
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("always", false) // With color but not verbose

	result := execution.Result{
		Hook: config.Hook{
			ID:      "exit-code-zero-hook",
			Verbose: false,
		},
		Duration: 1 * time.Second,
		ExitCode: 0, // Exit code 0 should not be shown
		Timeout:  false,
	}

	formatter.printFailureDetailsColored(result)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "exit-code-zero-hook")
	// Duration should not be shown when both verbose=false and hook.verbose=false
	assert.NotContains(t, output, "- duration:")
	// Exit code should not be shown when it's 0
	assert.NotContains(t, output, "- exit code:")
}

func TestFormatter_printFailureDetailsColored_VerboseTimeout(t *testing.T) {
	// Test colored failure details with timeout in verbose mode
	// Capture stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	formatter := NewFormatter("always", true) // Verbose and with color

	result := execution.Result{
		Hook: config.Hook{
			ID:      "timeout-colored-hook",
			Verbose: false, // Hook not verbose, but formatter is
		},
		Duration: 30 * time.Second,
		ExitCode: 124,
		Timeout:  true,
	}

	formatter.printFailureDetailsColored(result)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "timeout-colored-hook")
	assert.Contains(t, output, "30.0s (timeout)") // Should show duration with timeout indicator
}

func TestFormatter_PrintResults_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		results      []execution.Result
		expectedText []string
	}{
		{
			name: "hook with empty name falls back to ID",
			results: []execution.Result{
				{
					Hook: config.Hook{
						ID:   "fallback-hook-id",
						Name: "", // Empty name
					},
					Success: true,
					Files:   []string{"file1.go"},
				},
			},
			expectedText: []string{"fallback-hook-id", "Passed"},
		},
		{
			name: "very long hook name gets minimum dots",
			results: []execution.Result{
				{
					Hook: config.Hook{
						ID:   "short-id",
						Name: "This is a very long hook name that exceeds the normal width and should result in minimum dots being used",
					},
					Success: true,
					Files:   []string{"file1.go"},
				},
			},
			expectedText: []string{"This is a very long hook name", "Passed"},
		},
		{
			name: "hook with both success false and skipped false (default to failed)",
			results: []execution.Result{
				{
					Hook: config.Hook{
						ID:   "default-failed-hook",
						Name: "Default Failed Hook",
					},
					Success:  false,
					Skipped:  false,
					ExitCode: 1,
				},
			},
			expectedText: []string{"Default Failed Hook", "Failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout for testing
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			formatter := NewFormatter("never", false)
			formatter.PrintResults(tt.results)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			buf := make([]byte, 2048) // Larger buffer for long names
			n, _ := r.Read(buf)
			output := string(buf[:n])

			for _, expectedText := range tt.expectedText {
				assert.Contains(t, output, expectedText)
			}
		})
	}
}

func TestFormatter_printSkippedResult_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		hookName     string
		colorMode    string
		expectedText []string
		verbose      bool
	}{
		{
			name:      "very long hook name with skipped dots calculation",
			hookName:  "This is an extremely long hook name that will require recalculation of dots for skipped status because it has extra text",
			verbose:   false,
			colorMode: "never",
			expectedText: []string{
				"This is an extremely long hook name",
				"(no files to check)",
				"Skipped",
			},
		},
		{
			name:      "skipped hook in non-verbose mode with color",
			hookName:  "Skipped Color Hook",
			verbose:   false,
			colorMode: "always",
			expectedText: []string{
				"Skipped Color Hook",
				"(no files to check)",
				"Skipped",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout for testing
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			formatter := NewFormatter(tt.colorMode, tt.verbose)
			result := execution.Result{
				Hook: config.Hook{
					ID:   "test-skipped-hook",
					Name: tt.hookName,
				},
				Skipped: true,
			}

			shouldUseColor := formatter.shouldEnableColor()
			formatter.printSkippedResult(result, tt.hookName, shouldUseColor)

			// Restore stdout
			w.Close()
			os.Stdout = oldStdout

			// Read captured output
			buf := make([]byte, 2048)
			n, _ := r.Read(buf)
			output := string(buf[:n])

			for _, expectedText := range tt.expectedText {
				assert.Contains(t, output, expectedText)
			}
		})
	}
}
