package commands

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHazmatCommand_Help(t *testing.T) {
	cmd := &HazmatCommand{}
	help := cmd.Help()

	expectedPhrases := []string{
		"hazmat",
		"cd",
		"ignore-exit-code",
		"n1",
		"Composable tools",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(help, phrase) {
			t.Errorf("Help text should contain %q", phrase)
		}
	}
}

func TestHazmatCommand_Synopsis(t *testing.T) {
	cmd := &HazmatCommand{}
	synopsis := cmd.Synopsis()

	if !strings.Contains(synopsis, "Composable") {
		t.Error("Synopsis should mention composable tools")
	}
}

func TestHazmatCommand_RunNoArgs(t *testing.T) {
	cmd := &HazmatCommand{}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for no args (shows help), got %d", exitCode)
	}

	if !strings.Contains(buf.String(), "hazmat") {
		t.Error("Output should contain help text")
	}
}

func TestHazmatCommand_RunInvalidSubcommand(t *testing.T) {
	cmd := &HazmatCommand{}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := cmd.Run([]string{"invalid"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for invalid subcommand, got %d", exitCode)
	}

	if !strings.Contains(buf.String(), "invalid choice") {
		t.Errorf("Output should mention invalid choice, got: %s", buf.String())
	}
}

func TestCmdFilenames(t *testing.T) {
	tests := []struct {
		name          string
		input         []string
		wantCmd       []string
		wantFilenames []string
		wantErr       bool
	}{
		{
			name:          "basic split",
			input:         []string{"echo", "hello", "--", "file1.txt", "file2.txt"},
			wantCmd:       []string{"echo", "hello"},
			wantFilenames: []string{"file1.txt", "file2.txt"},
			wantErr:       false,
		},
		{
			name:          "no filenames",
			input:         []string{"echo", "--"},
			wantCmd:       []string{"echo"},
			wantFilenames: []string{},
			wantErr:       false,
		},
		{
			name:          "no separator",
			input:         []string{"echo", "hello"},
			wantCmd:       nil,
			wantFilenames: nil,
			wantErr:       true,
		},
		{
			name:          "multiple separators uses last",
			input:         []string{"echo", "--", "middle", "--", "file.txt"},
			wantCmd:       []string{"echo", "--", "middle"},
			wantFilenames: []string{"file.txt"},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotFilenames, err := cmdFilenames(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("cmdFilenames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(gotCmd) != len(tt.wantCmd) {
					t.Errorf("cmdFilenames() cmd = %v, want %v", gotCmd, tt.wantCmd)
				}
				for i := range gotCmd {
					if gotCmd[i] != tt.wantCmd[i] {
						t.Errorf("cmdFilenames() cmd[%d] = %v, want %v", i, gotCmd[i], tt.wantCmd[i])
					}
				}

				if len(gotFilenames) != len(tt.wantFilenames) {
					t.Errorf("cmdFilenames() filenames = %v, want %v", gotFilenames, tt.wantFilenames)
				}
				for i := range gotFilenames {
					if gotFilenames[i] != tt.wantFilenames[i] {
						t.Errorf("cmdFilenames() filenames[%d] = %v, want %v", i, gotFilenames[i], tt.wantFilenames[i])
					}
				}
			}
		})
	}
}

func TestHazmatIgnoreExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cmd := &HazmatCommand{}

	// Test that a failing command returns 0
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"cmd", "/c", "exit", "1"}
	} else {
		args = []string{"sh", "-c", "exit 1"}
	}

	exitCode := cmd.runIgnoreExitCode(args)
	if exitCode != 0 {
		t.Errorf("ignore-exit-code should return 0, got %d", exitCode)
	}
}

func TestHazmatN1(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp directory with test files
	tmpDir, err := os.MkdirTemp("", "hazmat-n1-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	for _, name := range []string{"file1.txt", "file2.txt", "file3.txt"} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	cmd := &HazmatCommand{}

	// Test n1 with echo - should run once per file
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"cmd", "/c", "echo", "--",
			filepath.Join(tmpDir, "file1.txt"),
			filepath.Join(tmpDir, "file2.txt"),
		}
	} else {
		args = []string{"echo", "--",
			filepath.Join(tmpDir, "file1.txt"),
			filepath.Join(tmpDir, "file2.txt"),
		}
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := cmd.runN1(args)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	if exitCode != 0 {
		t.Errorf("n1 should return 0, got %d", exitCode)
	}

	// Output should contain both filenames (run separately)
	output := buf.String()
	if !strings.Contains(output, "file1.txt") || !strings.Contains(output, "file2.txt") {
		t.Errorf("n1 should output both filenames, got: %s", output)
	}
}

func TestHazmatCD(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "hazmat-cd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdir
	subdir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create test file in subdir
	testFile := filepath.Join(subdir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := &HazmatCommand{}

	// Change to parent dir to run test
	oldDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldDir)

	// Test cd subcommand
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"subdir", "cmd", "/c", "type", "test.txt", "--", "subdir/test.txt"}
	} else {
		args = []string{"subdir", "cat", "--", "subdir/test.txt"}
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := cmd.runCD(args)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	if exitCode != 0 {
		t.Errorf("cd should return 0, got %d. Output: %s", exitCode, buf.String())
	}
}

func TestHazmatCD_MissingPrefix(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cmd := &HazmatCommand{}

	// Test with file that doesn't have the subdir prefix
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"subdir", "cmd", "/c", "echo", "--", "other/file.txt"}
	} else {
		args = []string{"subdir", "echo", "--", "other/file.txt"}
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	exitCode := cmd.runCD(args)

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)

	if exitCode != 1 {
		t.Errorf("cd with wrong prefix should return 1, got %d", exitCode)
	}

	if !strings.Contains(buf.String(), "unexpected file without prefix") {
		t.Errorf("Should mention unexpected file, got: %s", buf.String())
	}
}

func TestIsHazmatEntry(t *testing.T) {
	tests := []struct {
		entry    string
		expected bool
	}{
		{"pre-commit hazmat cd subdir", true},
		{"pre-commit hazmat n1", true},
		{"pre-commit hazmat ignore-exit-code", true},
		{"pre-commit run", false},
		{"echo hello", false},
		{"", false},
		{"pre-commit", false},
	}

	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			result := IsHazmatEntry(tt.entry)
			if result != tt.expected {
				t.Errorf("IsHazmatEntry(%q) = %v, want %v", tt.entry, result, tt.expected)
			}
		})
	}
}

func TestValidateHazmatEntry(t *testing.T) {
	tests := []struct {
		entry   string
		wantErr bool
	}{
		{"pre-commit hazmat cd subdir", false},
		{"pre-commit hazmat n1", false},
		{"pre-commit hazmat ignore-exit-code", false},
		{"pre-commit hazmat invalid", true},
		{"pre-commit run", false},
		{"echo hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			err := ValidateHazmatEntry(tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHazmatEntry(%q) error = %v, wantErr %v", tt.entry, err, tt.wantErr)
			}
		})
	}
}

func TestTransformHazmatEntry(t *testing.T) {
	tests := []struct {
		entry      string
		executable string
		expected   string
	}{
		{
			"pre-commit hazmat n1 -- echo",
			"/usr/local/bin/pre-commit",
			"/usr/local/bin/pre-commit hazmat n1 -- echo",
		},
		{
			"echo hello",
			"/usr/local/bin/pre-commit",
			"echo hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			result := TransformHazmatEntry(tt.entry, tt.executable)
			if result != tt.expected {
				t.Errorf("TransformHazmatEntry(%q, %q) = %q, want %q",
					tt.entry, tt.executable, result, tt.expected)
			}
		})
	}
}

func TestHazmatCommandFactory(t *testing.T) {
	cmd, err := HazmatCommandFactory()
	if err != nil {
		t.Fatalf("HazmatCommandFactory() error = %v", err)
	}

	if cmd == nil {
		t.Fatal("HazmatCommandFactory() returned nil")
	}

	if _, ok := cmd.(*HazmatCommand); !ok {
		t.Error("HazmatCommandFactory() should return *HazmatCommand")
	}
}

// Ensure exec package is used
var _ = exec.Command
