package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHazmatCdCommand_RunsInSubdir(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a marker file in the subdir.
	marker := filepath.Join(subdir, "marker.txt")
	if err := os.WriteFile(marker, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := &HazmatCdCommand{Meta: &Meta{}}
	// Run "test -f marker.txt" in the subdir — should succeed.
	code := cmd.Run([]string{subdir, "test", "-f", "marker.txt"})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestHazmatCdCommand_FailsWithNoArgs(t *testing.T) {
	cmd := &HazmatCdCommand{Meta: &Meta{}}
	code := cmd.Run(nil)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
}

func TestHazmatCdCommand_OneArg(t *testing.T) {
	cmd := &HazmatCdCommand{Meta: &Meta{}}
	code := cmd.Run([]string{"/tmp"})
	if code != 1 {
		t.Errorf("expected exit code 1 with only one arg, got %d", code)
	}
}

func TestHazmatCdCommand_PropagatesExitCode(t *testing.T) {
	cmd := &HazmatCdCommand{Meta: &Meta{}}
	code := cmd.Run([]string{"/tmp", "false"})
	if code == 0 {
		t.Error("expected non-zero exit code from 'false' command")
	}
}

func TestHazmatIgnoreExitCodeCommand_AlwaysReturnsZero(t *testing.T) {
	cmd := &HazmatIgnoreExitCodeCommand{Meta: &Meta{}}
	code := cmd.Run([]string{"false"})
	if code != 0 {
		t.Errorf("expected exit code 0 (ignored), got %d", code)
	}
}

func TestHazmatIgnoreExitCodeCommand_Success(t *testing.T) {
	cmd := &HazmatIgnoreExitCodeCommand{Meta: &Meta{}}
	code := cmd.Run([]string{"true"})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestHazmatIgnoreExitCodeCommand_NoArgs(t *testing.T) {
	cmd := &HazmatIgnoreExitCodeCommand{Meta: &Meta{}}
	code := cmd.Run(nil)
	if code != 1 {
		t.Errorf("expected exit code 1 for no args, got %d", code)
	}
}

func TestHazmatN1Command_RunsPerFile(t *testing.T) {
	dir := t.TempDir()
	// Create test files.
	for _, name := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cmd := &HazmatN1Command{Meta: &Meta{}}
	// test -f -- <files> should succeed for each file.
	code := cmd.Run([]string{
		"test", "-f",
		"--",
		filepath.Join(dir, "a.txt"),
		filepath.Join(dir, "b.txt"),
	})
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

func TestHazmatN1Command_PropagatesFailure(t *testing.T) {
	cmd := &HazmatN1Command{Meta: &Meta{}}
	code := cmd.Run([]string{
		"test", "-f",
		"--",
		"/nonexistent/file1.txt",
		"/nonexistent/file2.txt",
	})
	if code == 0 {
		t.Error("expected non-zero exit code for missing files")
	}
}

func TestHazmatN1Command_NoSeparator(t *testing.T) {
	cmd := &HazmatN1Command{Meta: &Meta{}}
	code := cmd.Run([]string{"echo", "hello"})
	if code != 1 {
		t.Errorf("expected exit code 1 for missing separator, got %d", code)
	}
}

func TestHazmatN1Command_NoCmdBeforeSeparator(t *testing.T) {
	cmd := &HazmatN1Command{Meta: &Meta{}}
	code := cmd.Run([]string{"--", "file.txt"})
	if code != 1 {
		t.Errorf("expected exit code 1 for no command, got %d", code)
	}
}
