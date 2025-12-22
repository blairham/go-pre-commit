package commands

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// ====================
// Help and Synopsis Tests
// ====================

func TestSampleConfigCommand_Help(t *testing.T) {
	cmd := &SampleConfigCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"sample-config",
		"pre-commit-config.yaml",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help should contain %q", expected)
		}
	}
}

func TestSampleConfigCommand_Synopsis(t *testing.T) {
	cmd := &SampleConfigCommand{}
	synopsis := cmd.Synopsis()

	if !strings.Contains(strings.ToLower(synopsis), "sample") {
		t.Error("Synopsis should mention sample")
	}
}

// ====================
// Python Parity Tests
// ====================

func TestSampleConfigCommand_OutputMatchesPython(t *testing.T) {
	// Python's exact output
	expectedOutput := `# See https://pre-commit.com for more information
# See https://pre-commit.com/hooks.html for more hooks
repos:
-   repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v3.2.0
    hooks:
    -   id: trailing-whitespace
    -   id: end-of-file-fixer
    -   id: check-yaml
    -   id: check-added-large-files
`

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &SampleConfigCommand{}
	exitCode := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	if output != expectedOutput {
		t.Errorf("Output does not match Python's sample-config.\nExpected:\n%s\nGot:\n%s", expectedOutput, output)
	}
}

func TestSampleConfigCommand_AlwaysReturnsZero(t *testing.T) {
	// Capture stdout to prevent test output noise
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := &SampleConfigCommand{}
	exitCode := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout
	io.Copy(io.Discard, r)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}
}

func TestSampleConfigCommand_HasCommentHeader(t *testing.T) {
	if !strings.Contains(SAMPLE_CONFIG, "# See https://pre-commit.com") {
		t.Error("Sample config should contain comment header with pre-commit.com URL")
	}
	if !strings.Contains(SAMPLE_CONFIG, "# See https://pre-commit.com/hooks.html") {
		t.Error("Sample config should contain comment header with hooks URL")
	}
}

func TestSampleConfigCommand_Uses4SpaceIndent(t *testing.T) {
	// Python uses 4-space indentation (actually "-   " which is dash + 3 spaces)
	if !strings.Contains(SAMPLE_CONFIG, "-   repo:") {
		t.Error("Sample config should use 4-space list indentation (dash + 3 spaces)")
	}
	if !strings.Contains(SAMPLE_CONFIG, "    rev:") {
		t.Error("Sample config should use 4-space indentation for rev")
	}
}

func TestSampleConfigCommand_ContainsExpectedHooks(t *testing.T) {
	expectedHooks := []string{
		"trailing-whitespace",
		"end-of-file-fixer",
		"check-yaml",
		"check-added-large-files",
	}

	for _, hook := range expectedHooks {
		if !strings.Contains(SAMPLE_CONFIG, hook) {
			t.Errorf("Sample config should contain hook: %s", hook)
		}
	}
}

func TestSampleConfigCommand_NoExtraFlags(t *testing.T) {
	// Ensure we don't have --force or other flags that Python doesn't have
	cmd := &SampleConfigCommand{}
	help := cmd.Help()

	if strings.Contains(help, "--force") {
		t.Error("Help should not contain --force flag (Python doesn't have it)")
	}
}
