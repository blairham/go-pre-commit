package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// Helper function to create an ExitError with exit code 1
func createExitErrorWithCode1() *exec.ExitError {
	// Run a command that exits with code 1 to get a real ExitError
	cmd := exec.Command("false")
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr
	}
	// Fallback: this should work on most systems
	return &exec.ExitError{ProcessState: &os.ProcessState{}}
}

func TestNewExecutor(t *testing.T) {
	ctx := &Context{
		Timeout: 30 * time.Second,
	}

	executor := NewExecutor(ctx)
	assert.NotNil(t, executor)
	assert.Equal(t, ctx, executor.ctx)
}

func TestExecutor_ExecuteWithTimeout(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		timeout     time.Duration
		expectError bool
	}{
		{
			name:        "no timeout",
			command:     "echo",
			timeout:     0,
			expectError: false,
		},
		{
			name:        "with timeout - success",
			command:     "echo",
			timeout:     5 * time.Second,
			expectError: false,
		},
		{
			name:        "with timeout - timeout exceeded",
			command:     "sleep",
			timeout:     100 * time.Millisecond,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Timeout: tt.timeout,
			}
			executor := NewExecutor(ctx)

			var cmd *exec.Cmd
			switch tt.command {
			case "echo":
				cmd = exec.Command("echo", "test")
			case "sleep":
				cmd = exec.Command("sleep", "1")
			}

			output, err := executor.ExecuteWithTimeout(context.Background(), cmd)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.command == "echo" {
					assert.Contains(t, string(output), "test")
				}
			}
		})
	}
}

func TestExecutor_ProcessExecutionResult(t *testing.T) {
	tests := []struct {
		name         string
		output       []byte
		execErr      error
		hook         config.Hook
		expectResult Result
	}{
		{
			name:    "successful execution",
			output:  []byte("success output"),
			execErr: nil,
			hook: config.Hook{
				ID:   "test-hook",
				Name: "Test Hook",
			},
			expectResult: Result{
				Output:  "success output",
				Error:   "",
				Success: true,
			},
		},
		{
			name:    "failed execution with exit code",
			output:  []byte("failure output"),
			execErr: &exec.ExitError{},
			hook: config.Hook{
				ID:   "test-hook",
				Name: "Test Hook",
			},
			expectResult: Result{
				Output:   "failure output",
				Success:  false,
				ExitCode: -1, // ExitError returns -1 for ProcessState
				Error:    "", // Should be empty since we have useful output
			},
		},
		{
			name:    "failed execution with no output",
			output:  []byte(""),
			execErr: &exec.ExitError{},
			hook: config.Hook{
				ID:   "test-hook",
				Name: "Test Hook",
			},
			expectResult: Result{
				Output:   "",
				Success:  false,
				ExitCode: -1,                                 // ExitError returns -1 for ProcessState
				Error:    "Command failed with exit code -1", // Should show generic error since no useful output
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{
				Timeout: 30 * time.Second,
			}
			executor := NewExecutor(ctx)

			result := &Result{}
			start := time.Now()

			executor.ProcessExecutionResult(result, tt.output, tt.execErr, tt.hook, start)

			assert.Equal(t, tt.expectResult.Output, result.Output)
			assert.Equal(t, tt.expectResult.Success, result.Success)
			assert.Equal(t, tt.expectResult.Error, result.Error)
			assert.GreaterOrEqual(t, result.Duration, time.Duration(0))
		})
	}
}

func TestExecutor_processExitCode(t *testing.T) {
	ctx := &Context{}
	executor := NewExecutor(ctx)

	tests := []struct {
		err          error
		name         string
		expectedCode int
	}{
		{
			name:         "exit error",
			err:          &exec.ExitError{},
			expectedCode: -1, // ExitError returns -1 for ProcessState
		},
		{
			name:         "other error",
			err:          exec.ErrNotFound,
			expectedCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{}
			executor.processExitCode(result, tt.err)
			assert.Equal(t, tt.expectedCode, result.ExitCode)
		})
	}
}

func TestExecutor_handleTimeoutError(t *testing.T) {
	ctx := &Context{
		Timeout: 1 * time.Second,
	}
	executor := NewExecutor(ctx)

	tests := []struct {
		err            error
		name           string
		expectedResult bool
	}{
		{
			name:           "timeout error",
			err:            context.DeadlineExceeded,
			expectedResult: true,
		},
		{
			name:           "other error",
			err:            exec.ErrNotFound,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{}
			isTimeout := executor.handleTimeoutError(result, tt.err)
			assert.Equal(t, tt.expectedResult, isTimeout)

			if tt.expectedResult {
				assert.Contains(t, result.Error, "timed out")
			}
		})
	}
}

func TestExecutor_handleExecutionError(t *testing.T) {
	ctx := &Context{}
	executor := NewExecutor(ctx)

	tests := []struct {
		name          string
		err           error
		output        string
		expectedError string
	}{
		{
			name:          "executable not found",
			err:           exec.ErrNotFound,
			output:        "",
			expectedError: "Executable not found",
		},
		{
			name:          "exit error with no output",
			err:           &exec.ExitError{},
			output:        "",
			expectedError: "Command failed with exit code",
		},
		{
			name:          "exit error with output",
			err:           &exec.ExitError{},
			output:        "some useful output from linter",
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{Output: tt.output}
			executor.handleExecutionError(result, tt.err)
			if tt.expectedError == "" {
				assert.Empty(t, result.Error)
			} else {
				assert.Contains(t, result.Error, tt.expectedError)
			}
		})
	}
}

func TestExecutor_isExecutableNotFoundError(t *testing.T) {
	ctx := &Context{}
	executor := NewExecutor(ctx)

	tests := []struct {
		err      error
		name     string
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "not found error",
			err:      exec.ErrNotFound,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.isExecutableNotFoundError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutor_determineHookSuccess(t *testing.T) {
	ctx := &Context{}
	executor := NewExecutor(ctx)

	tests := []struct {
		name            string
		execErr         error
		hook            config.Hook
		result          Result
		expectedSuccess bool
	}{
		{
			name:            "no error",
			execErr:         nil,
			hook:            config.Hook{},
			result:          Result{},
			expectedSuccess: true,
		},
		{
			name:    "timeout error",
			execErr: context.DeadlineExceeded,
			hook:    config.Hook{},
			result: Result{
				Timeout: true,
			},
			expectedSuccess: false,
		},
		{
			name:    "executable not found",
			execErr: exec.ErrNotFound,
			hook:    config.Hook{},
			result: Result{
				ExitCode: 1,
			},
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result
			executor.determineHookSuccess(&result, tt.hook, tt.execErr)
			assert.Equal(t, tt.expectedSuccess, result.Success)
		})
	}
}

// Additional tests for improved coverage

func TestExecutor_isFormatterHook(t *testing.T) {
	ctx := &Context{Timeout: 30 * time.Second}
	executor := NewExecutor(ctx)

	tests := []struct {
		name     string
		hook     config.Hook
		expected bool
	}{
		{
			name: "black formatter by ID",
			hook: config.Hook{
				ID:    "black",
				Entry: "python -m black",
			},
			expected: true,
		},
		{
			name: "autopep8 formatter by ID",
			hook: config.Hook{
				ID:    "autopep8",
				Entry: "autopep8",
			},
			expected: true,
		},
		{
			name: "prettier formatter by ID",
			hook: config.Hook{
				ID:    "prettier",
				Entry: "prettier",
			},
			expected: true,
		},
		{
			name: "gofmt formatter by ID",
			hook: config.Hook{
				ID:    "gofmt",
				Entry: "gofmt",
			},
			expected: true,
		},
		{
			name: "rustfmt formatter by ID",
			hook: config.Hook{
				ID:    "rustfmt",
				Entry: "rustfmt",
			},
			expected: true,
		},
		{
			name: "formatter detected by entry - black",
			hook: config.Hook{
				ID:    "custom-black-hook",
				Entry: "python -m black --check",
			},
			expected: true,
		},
		{
			name: "formatter detected by entry - prettier",
			hook: config.Hook{
				ID:    "format-js",
				Entry: "npx prettier --write",
			},
			expected: true,
		},
		{
			name: "formatter detected by entry - gofmt",
			hook: config.Hook{
				ID:    "go-format",
				Entry: "gofmt -w",
			},
			expected: true,
		},
		{
			name: "non-formatter hook",
			hook: config.Hook{
				ID:    "flake8",
				Entry: "flake8",
			},
			expected: false,
		},
		{
			name: "linter hook",
			hook: config.Hook{
				ID:    "pylint",
				Entry: "pylint",
			},
			expected: false,
		},
		{
			name: "test hook",
			hook: config.Hook{
				ID:    "pytest",
				Entry: "pytest",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.isFormatterHook(tt.hook)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutor_outputIndicatesModification(t *testing.T) {
	ctx := &Context{Timeout: 30 * time.Second}
	executor := NewExecutor(ctx)

	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "reformatted files",
			output:   "reformatted main.py",
			expected: true,
		},
		{
			name:     "files reformatted",
			output:   "All done! ✨ 🍰 ✨\n2 files reformatted, 3 files left unchanged.",
			expected: true,
		},
		{
			name:     "would reformat",
			output:   "would reformat main.py",
			expected: true,
		},
		{
			name:     "fixed files",
			output:   "Fixed ./src/main.py",
			expected: true,
		},
		{
			name:     "formatting output",
			output:   "Formatting complete",
			expected: true,
		},
		{
			name:     "case insensitive - REFORMATTED",
			output:   "REFORMATTED main.py",
			expected: true,
		},
		{
			name:     "case insensitive - FIXED",
			output:   "FIXED ./src/main.py",
			expected: true,
		},
		{
			name:     "no modification indicators",
			output:   "All files are well formatted",
			expected: false,
		},
		{
			name:     "linter output",
			output:   "main.py:10:5: E302 expected 2 blank lines, found 1",
			expected: false,
		},
		{
			name:     "test results",
			output:   "All tests passed",
			expected: false,
		},
		{
			name:     "empty output",
			output:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.outputIndicatesModification(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutor_addFilesModifiedMessage(t *testing.T) {
	ctx := &Context{Timeout: 30 * time.Second}
	executor := NewExecutor(ctx)

	tests := []struct {
		name           string
		initialOutput  string
		expectedOutput string
	}{
		{
			name:           "empty output",
			initialOutput:  "",
			expectedOutput: "- files were modified by this hook",
		},
		{
			name:           "output with newlines",
			initialOutput:  "Hook execution details\n\nSome output here",
			expectedOutput: "Hook execution details\n- files were modified by this hook\n\nSome output here",
		},
		{
			name:           "single line output",
			initialOutput:  "Formatting complete",
			expectedOutput: "- files were modified by this hook\nFormatting complete",
		},
		{
			name:           "multi-line output with blank line",
			initialOutput:  "black check failed\n\nFiles reformatted: main.py",
			expectedOutput: "black check failed\n- files were modified by this hook\n\nFiles reformatted: main.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &Result{Output: tt.initialOutput}
			executor.addFilesModifiedMessage(result)
			assert.Equal(t, tt.expectedOutput, result.Output)
		})
	}
}

func TestExecutor_isFormatterWithModifications(t *testing.T) {
	ctx := &Context{Timeout: 30 * time.Second}
	executor := NewExecutor(ctx)

	tests := []struct {
		execErr  error
		result   *Result
		name     string
		hook     config.Hook
		expected bool
	}{
		{
			name: "formatter with exit code 1 and modification output",
			result: &Result{
				Output: "reformatted main.py",
			},
			hook: config.Hook{
				ID:    "black",
				Entry: "black",
			},
			execErr:  createExitErrorWithCode1(),
			expected: true,
		},
		{
			name: "formatter with exit code 1 but no modification output",
			result: &Result{
				Output: "All files are well formatted",
			},
			hook: config.Hook{
				ID:    "black",
				Entry: "black",
			},
			execErr:  createExitErrorWithCode1(),
			expected: false,
		},
		{
			name: "non-formatter with exit code 1",
			result: &Result{
				Output: "reformatted main.py",
			},
			hook: config.Hook{
				ID:    "flake8",
				Entry: "flake8",
			},
			execErr:  createExitErrorWithCode1(),
			expected: false,
		},
		{
			name: "formatter with no error",
			result: &Result{
				Output: "reformatted main.py",
			},
			hook: config.Hook{
				ID:    "black",
				Entry: "black",
			},
			execErr:  nil,
			expected: false,
		},
		{
			name: "formatter with different exit code",
			result: &Result{
				Output: "reformatted main.py",
			},
			hook: config.Hook{
				ID:    "black",
				Entry: "black",
			},
			execErr:  fmt.Errorf("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.isFormatterWithModifications(tt.result, tt.hook, tt.execErr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecutor_handleExecutionError_EdgeCases(t *testing.T) {
	ctx := &Context{Timeout: 30 * time.Second}
	executor := NewExecutor(ctx)

	tests := []struct {
		name          string
		result        *Result
		err           error
		expectedError string
	}{
		{
			name: "exit error with output",
			result: &Result{
				Output: "Some linting errors found",
			},
			err:           createExitErrorWithCode1(),
			expectedError: "", // Should clear error when there's useful output
		},
		{
			name: "exit error without output",
			result: &Result{
				Output: "",
			},
			err:           createExitErrorWithCode1(),
			expectedError: "Command failed with exit code 1",
		},
		{
			name: "executable not found error",
			result: &Result{
				Output: "",
			},
			err:           os.ErrNotExist,
			expectedError: "Executable not found: file does not exist",
		},
		{
			name: "generic execution error",
			result: &Result{
				Output: "",
			},
			err:           fmt.Errorf("generic error"),
			expectedError: "Execution error: generic error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor.handleExecutionError(tt.result, tt.err)
			assert.Equal(t, tt.expectedError, tt.result.Error)
		})
	}
}

func TestExecutor_determineHookSuccess_EdgeCases(t *testing.T) {
	ctx := &Context{Timeout: 30 * time.Second}
	executor := NewExecutor(ctx)

	tests := []struct {
		execErr    error
		result     *Result
		name       string
		hook       config.Hook
		expectPass bool
	}{
		{
			name: "successful execution",
			result: &Result{
				ExitCode: 0,
				Output:   "All checks passed",
			},
			hook: config.Hook{
				ID:    "test-hook",
				Entry: "test-command",
			},
			execErr:    nil,
			expectPass: true,
		},
		{
			name: "execution error - general failure",
			result: &Result{
				ExitCode: 1,
				Output:   "Test failure",
			},
			hook: config.Hook{
				ID:    "test-hook",
				Entry: "test-command",
			},
			execErr:    &exec.ExitError{},
			expectPass: false,
		},
		{
			name: "timeout error",
			result: &Result{
				Timeout:  true,
				ExitCode: 124,
				Output:   "Command timed out",
			},
			hook: config.Hook{
				ID:    "slow-hook",
				Entry: "slow-command",
			},
			execErr:    context.DeadlineExceeded,
			expectPass: false,
		},
		{
			name: "formatter with modifications - black",
			result: &Result{
				ExitCode: 1,
				Output:   "reformatted main.py",
			},
			hook: config.Hook{
				ID:    "black",
				Entry: "black",
			},
			execErr:    &exec.ExitError{},
			expectPass: false, // Formatters with modifications should "fail" to block commit
		},
		{
			name: "formatter without modifications - black",
			result: &Result{
				ExitCode: 0,
				Output:   "would reformat 0 files",
			},
			hook: config.Hook{
				ID:    "black",
				Entry: "black",
			},
			execErr:    nil,
			expectPass: true,
		},
		{
			name: "non-formatter with exit code 1",
			result: &Result{
				ExitCode: 1,
				Output:   "Linting errors found",
			},
			hook: config.Hook{
				ID:    "pylint",
				Entry: "pylint",
			},
			execErr:    &exec.ExitError{},
			expectPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the result to avoid side effects
			result := &Result{
				ExitCode: tt.result.ExitCode,
				Output:   tt.result.Output,
				Timeout:  tt.result.Timeout,
			}

			executor.determineHookSuccess(result, tt.hook, tt.execErr)
			assert.Equal(t, tt.expectPass, result.Success)
		})
	}
}
