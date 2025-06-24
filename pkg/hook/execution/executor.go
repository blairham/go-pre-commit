package execution

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// Executor handles the execution of individual hooks
type Executor struct {
	ctx *Context
}

// NewExecutor creates a new hook executor
func NewExecutor(ctx *Context) *Executor {
	return &Executor{ctx: ctx}
}

// ExecuteWithTimeout executes a command with timeout handling
func (e *Executor) ExecuteWithTimeout(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	if e.ctx.Timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, e.ctx.Timeout)
		defer cancel()

		// Create a copy of the command with the timeout context
		cmdWithTimeout := exec.CommandContext(timeoutCtx, cmd.Path, cmd.Args[1:]...)
		cmdWithTimeout.Dir = cmd.Dir
		cmdWithTimeout.Env = cmd.Env
		cmdWithTimeout.Stdin = cmd.Stdin
		cmdWithTimeout.Stdout = cmd.Stdout
		cmdWithTimeout.Stderr = cmd.Stderr

		return cmdWithTimeout.CombinedOutput()
	}

	return cmd.CombinedOutput()
}

// ProcessExecutionResult processes the result of command execution
func (e *Executor) ProcessExecutionResult(
	result *Result,
	output []byte,
	execErr error,
	originalHook config.Hook,
	start time.Time,
) {
	result.Output = string(output)
	result.Duration = time.Since(start)

	if execErr != nil {
		result.Error = execErr.Error()
		e.processExitCode(result, execErr)

		// Check for timeout
		if e.handleTimeoutError(result, execErr) {
			result.Timeout = true
		} else {
			e.handleExecutionError(result, execErr)
		}
	}

	e.determineHookSuccess(result, originalHook, execErr)
}

// processExitCode extracts the exit code from an execution error
func (e *Executor) processExitCode(result *Result, execErr error) {
	var exitError *exec.ExitError
	if errors.As(execErr, &exitError) {
		result.ExitCode = exitError.ExitCode()
	} else {
		result.ExitCode = 1
	}
}

// handleTimeoutError checks if the error is a timeout error
func (e *Executor) handleTimeoutError(result *Result, execErr error) bool {
	if errors.Is(execErr, context.DeadlineExceeded) {
		result.Error = fmt.Sprintf("Hook timed out after %v", e.ctx.Timeout)
		return true
	}
	return false
}

// handleExecutionError processes execution errors
func (e *Executor) handleExecutionError(result *Result, execErr error) {
	if e.isExecutableNotFoundError(execErr) {
		result.Error = fmt.Sprintf("Executable not found: %s", execErr.Error())
		return
	}

	var exitError *exec.ExitError
	if !errors.As(execErr, &exitError) {
		// Other execution error
		result.Error = fmt.Sprintf("Execution error: %s", execErr.Error())
		return
	}

	// For hooks that produce meaningful output (like linters), don't override
	// the error with a generic message if we have useful output
	if strings.TrimSpace(result.Output) != "" {
		// Clear the generic error - the output contains the real information
		result.Error = ""
		return
	}

	// No useful output, so the generic error message is helpful
	result.Error = fmt.Sprintf("Command failed with exit code %d", exitError.ExitCode())
}

// isExecutableNotFoundError checks if the error is an executable not found error
func (e *Executor) isExecutableNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return os.IsNotExist(err) || errors.Is(err, exec.ErrNotFound)
}

// determineHookSuccess determines if a hook execution was successful
func (e *Executor) determineHookSuccess(result *Result, hook config.Hook, execErr error) {
	if execErr == nil {
		result.Success = true
		return
	}

	// Handle timeout
	if result.Timeout {
		result.Success = false
		return
	}

	// Handle executable not found
	if e.isExecutableNotFoundError(execErr) {
		result.Success = false
		return
	}

	// Check if this is a formatter that modified files
	if e.isFormatterWithModifications(result, hook, execErr) {
		// For formatters that modify files, exit code 1 with modifications is a "failure"
		// in the sense that files were changed and the commit should be blocked
		result.Success = false
		e.addFilesModifiedMessage(result)
		return
	}

	// For most cases, any execution error means failure
	// This includes ExitError (non-zero exit codes)
	result.Success = false
}

// isFormatterWithModifications checks if this is a formatter that modified files
func (e *Executor) isFormatterWithModifications(result *Result, hook config.Hook, execErr error) bool {
	// Only applies to formatters with non-zero exit codes
	var exitError *exec.ExitError
	if !errors.As(execErr, &exitError) || exitError.ExitCode() != 1 {
		return false
	}

	// Check if this is a known formatter hook
	if !e.isFormatterHook(hook) {
		return false
	}

	// Check if the output indicates files were modified
	return e.outputIndicatesModification(result.Output)
}

// isFormatterHook checks if this hook is a known formatter
func (e *Executor) isFormatterHook(hook config.Hook) bool {
	// Common formatter hook IDs
	formatterHooks := map[string]bool{
		"black":         true,
		"autopep8":      true,
		"yapf":          true,
		"isort":         true,
		"prettier":      true,
		"eslint":        true, // ESLint can also fix files
		"rustfmt":       true,
		"gofmt":         true,
		"clang-format":  true,
		"terraform_fmt": true,
	}

	// Check hook ID
	if formatterHooks[hook.ID] {
		return true
	}

	// Check if hook entry suggests it's a formatter
	entry := strings.ToLower(hook.Entry)
	return strings.Contains(entry, "black") ||
		strings.Contains(entry, "autopep8") ||
		strings.Contains(entry, "yapf") ||
		strings.Contains(entry, "isort") ||
		strings.Contains(entry, "prettier") ||
		strings.Contains(entry, "rustfmt") ||
		strings.Contains(entry, "gofmt") ||
		strings.Contains(entry, "clang-format")
}

// outputIndicatesModification checks if the output indicates files were modified
func (e *Executor) outputIndicatesModification(output string) bool {
	lowerOutput := strings.ToLower(output)
	return strings.Contains(lowerOutput, "reformatted") ||
		strings.Contains(lowerOutput, "fixed") ||
		strings.Contains(lowerOutput, "file reformatted") ||
		strings.Contains(lowerOutput, "files reformatted") ||
		strings.Contains(lowerOutput, "would reformat") ||
		strings.Contains(lowerOutput, "formatting")
}

// addFilesModifiedMessage adds the "files were modified" message to the output
func (e *Executor) addFilesModifiedMessage(result *Result) {
	// Add the standard message that Python pre-commit shows
	modifiedMessage := "- files were modified by this hook"

	// If there's existing output, add it after the hook details
	if result.Output != "" {
		// Insert the message at the beginning, similar to how Python pre-commit does it
		lines := strings.Split(result.Output, "\n")

		// Find a good place to insert the message (after hook id, duration, etc.)
		insertIndex := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				insertIndex = i
				break
			}
		}

		// Insert the message
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:insertIndex]...)
		newLines = append(newLines, modifiedMessage)
		newLines = append(newLines, lines[insertIndex:]...)

		result.Output = strings.Join(newLines, "\n")
	} else {
		result.Output = modifiedMessage
	}
}
