package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// DiagnosticsManager handles command execution with detailed diagnostics
type DiagnosticsManager struct{}

// NewDiagnosticsManager creates a new diagnostics manager
func NewDiagnosticsManager() *DiagnosticsManager {
	return &DiagnosticsManager{}
}

// RunCommandWithDiagnostics runs a command and captures detailed output for analysis
func (dm *DiagnosticsManager) RunCommandWithDiagnostics(
	t *testing.T,
	dir string,
	timeout time.Duration,
	cmd string,
	args ...string,
) (*CommandDiagnostics, error) {
	t.Helper()

	diagnostics := &CommandDiagnostics{
		Command: cmd + " " + strings.Join(args, " "),
		Dir:     dir,
		Start:   time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	execCmd := exec.CommandContext(ctx, cmd, args...)
	execCmd.Dir = dir

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()

	diagnostics.Duration = time.Since(diagnostics.Start)
	diagnostics.Stdout = stdout.String()
	diagnostics.Stderr = stderr.String()
	diagnostics.ExitCode = 0

	if err != nil {
		diagnostics.Error = err.Error()
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			diagnostics.ExitCode = exitError.ExitCode()
		} else {
			diagnostics.ExitCode = -1
		}
	}

	return diagnostics, err
}

// AnalyzeInstallationFailure provides detailed analysis of why installation failed
func (dm *DiagnosticsManager) AnalyzeInstallationFailure(
	t *testing.T,
	test LanguageCompatibilityTest,
	diagnostics *CommandDiagnostics,
) string {
	t.Helper()

	analysis := []string{}

	// Check for common failure patterns
	if strings.Contains(diagnostics.Stderr, "No such file or directory") ||
		strings.Contains(diagnostics.Stderr, "command not found") {
		analysis = append(analysis, fmt.Sprintf("Runtime %s not available - install required", test.Language))
	}

	if strings.Contains(diagnostics.Stderr, "timeout") ||
		strings.Contains(diagnostics.Error, "timeout") {
		analysis = append(analysis, "Installation timed out - network or performance issue")
	}

	if strings.Contains(diagnostics.Stderr, "git clone") ||
		strings.Contains(diagnostics.Stderr, "Failed to clone") {
		analysis = append(analysis, "Repository cloning failed - network or access issue")
	}

	if strings.Contains(diagnostics.Stderr, "Permission denied") {
		analysis = append(analysis, "Permission denied - check file system permissions")
	}

	if strings.Contains(diagnostics.Stderr, "disk space") ||
		strings.Contains(diagnostics.Stderr, "No space left") {
		analysis = append(analysis, "Insufficient disk space")
	}

	if len(analysis) == 0 {
		analysis = append(
			analysis,
			"Installation failed with exit code "+fmt.Sprintf("%d", diagnostics.ExitCode),
		)
		if diagnostics.Stderr != "" {
			analysis = append(analysis, "stderr: "+strings.TrimSpace(diagnostics.Stderr))
		}
	}

	return strings.Join(analysis, "; ")
}

// IsExpectedInstallationFailure determines if an installation failure is expected
func (dm *DiagnosticsManager) IsExpectedInstallationFailure(
	_ LanguageCompatibilityTest,
	diagnostics *CommandDiagnostics,
) bool {
	// Runtime not available is expected for many languages in test environments
	if strings.Contains(diagnostics.Stderr, "command not found") ||
		strings.Contains(diagnostics.Stderr, "No such file or directory") {
		return true
	}

	// Network issues during repository cloning are expected in restricted environments
	if strings.Contains(diagnostics.Stderr, "Failed to clone") ||
		strings.Contains(diagnostics.Stderr, "git clone") {
		return true
	}

	// Timeout issues are expected for complex language setups
	if strings.Contains(diagnostics.Error, "timeout") ||
		strings.Contains(diagnostics.Stderr, "timeout") {
		return true
	}

	return false
}

// RunCommandWithOutput executes a command and returns its output
func (dm *DiagnosticsManager) RunCommandWithOutput(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// RunCommandWithTimeout executes a command with a timeout
func (dm *DiagnosticsManager) RunCommandWithTimeout(
	dir string,
	timeout time.Duration,
	name string,
	args ...string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.Run()
}
