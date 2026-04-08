package languages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ParseEntry – mirrors pre_commit.lang_base.hook_cmd / shlex.split behaviour
// ---------------------------------------------------------------------------

func TestParseEntrySimple(t *testing.T) {
	got := ParseEntry("mycommand")
	want := []string{"mycommand"}
	assertSliceEqual(t, got, want)
}

func TestParseEntryMultipleWords(t *testing.T) {
	got := ParseEntry("cmd arg1 arg2")
	want := []string{"cmd", "arg1", "arg2"}
	assertSliceEqual(t, got, want)
}

func TestParseEntryDoubleQuotes(t *testing.T) {
	// shlex.split('cmd "hello world"') == ['cmd', 'hello world']
	got := ParseEntry(`cmd "hello world"`)
	want := []string{"cmd", "hello world"}
	assertSliceEqual(t, got, want)
}

func TestParseEntrySingleQuotes(t *testing.T) {
	// shlex.split("cmd 'hello world'") == ['cmd', 'hello world']
	got := ParseEntry("cmd 'hello world'")
	want := []string{"cmd", "hello world"}
	assertSliceEqual(t, got, want)
}

func TestParseEntryEmpty(t *testing.T) {
	got := ParseEntry("")
	if len(got) != 0 {
		t.Errorf("ParseEntry(%q) = %v, want []", "", got)
	}
}

func TestParseEntryLeadingTrailingSpaces(t *testing.T) {
	got := ParseEntry("  cmd  arg  ")
	want := []string{"cmd", "arg"}
	assertSliceEqual(t, got, want)
}

func TestParseEntryTabSeparated(t *testing.T) {
	got := ParseEntry("cmd\targ")
	want := []string{"cmd", "arg"}
	assertSliceEqual(t, got, want)
}

func TestParseEntryQuotedEmpty(t *testing.T) {
	// shlex.split('cmd ""') == ['cmd', '']
	got := ParseEntry(`cmd ""`)
	want := []string{"cmd", ""}
	assertSliceEqual(t, got, want)
}

// ---------------------------------------------------------------------------
// PrependPath – mirrors get_env_patch PATH manipulation
// ---------------------------------------------------------------------------

func TestPrependPath(t *testing.T) {
	dir := "/my/custom/bin"
	result := PrependPath(dir)

	if !strings.HasPrefix(result, "PATH=") {
		t.Errorf("PrependPath(%q) = %q, want to start with PATH=", dir, result)
	}
	if !strings.Contains(result, dir) {
		t.Errorf("PrependPath(%q) = %q, want to contain the dir", dir, result)
	}
	// The custom dir should come before the existing PATH entries.
	idx := strings.Index(result, dir)
	existingPath := os.Getenv("PATH")
	if existingPath != "" {
		idxExisting := strings.Index(result, existingPath)
		if idx > idxExisting {
			t.Errorf("PrependPath: custom dir %q should appear before existing PATH in %q", dir, result)
		}
	}
}

func TestPrependPathFormat(t *testing.T) {
	// Verify exact format: PATH=<dir><separator><original>
	dir := "/bin/test"
	result := PrependPath(dir)
	sep := string(os.PathListSeparator)
	wantPrefix := "PATH=" + dir + sep
	if !strings.HasPrefix(result, wantPrefix) {
		t.Errorf("PrependPath(%q) = %q, want prefix %q", dir, result, wantPrefix)
	}
}

// ---------------------------------------------------------------------------
// FindExecutable – mirrors parse_shebang.find_executable
// ---------------------------------------------------------------------------

func TestFindExecutableInGivenPaths(t *testing.T) {
	// Create a temp dir with a fake executable.
	dir := t.TempDir()
	exe := filepath.Join(dir, "myfakeexe")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	found, err := FindExecutable("myfakeexe", dir)
	if err != nil {
		t.Fatalf("FindExecutable: %v", err)
	}
	if found != exe {
		t.Errorf("FindExecutable = %q, want %q", found, exe)
	}
}

func TestFindExecutableFallsBackToPath(t *testing.T) {
	// "sh" should always be on PATH.
	found, err := FindExecutable("sh")
	if err != nil {
		t.Fatalf("FindExecutable(sh): %v", err)
	}
	if found == "" {
		t.Error("FindExecutable(sh) returned empty string")
	}
}

func TestFindExecutableMissingReturnsError(t *testing.T) {
	_, err := FindExecutable("__this_does_not_exist__")
	if err == nil {
		t.Error("FindExecutable for missing binary = nil, want error")
	}
}

func TestFindExecutablePreferFirstPath(t *testing.T) {
	// Two dirs each with a binary of the same name – first dir should win.
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	for _, d := range []string{dir1, dir2} {
		exe := filepath.Join(d, "myexe")
		if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	found, err := FindExecutable("myexe", dir1, dir2)
	if err != nil {
		t.Fatal(err)
	}
	if found != filepath.Join(dir1, "myexe") {
		t.Errorf("FindExecutable = %q, want path in dir1 %q", found, dir1)
	}
}

// ---------------------------------------------------------------------------
// RunCommand – basic execution semantics
// ---------------------------------------------------------------------------

func TestRunCommandSuccess(t *testing.T) {
	code, out, err := RunCommand(context.Background(), t.TempDir(), "echo", "hello")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("output %q does not contain 'hello'", out)
	}
}

func TestRunCommandNonZeroExit(t *testing.T) {
	code, _, err := RunCommand(context.Background(), t.TempDir(), "sh", "-c", "exit 7")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if code != 7 {
		t.Errorf("exit code = %d, want 7", code)
	}
}

func TestRunCommandCapturesCombinedOutput(t *testing.T) {
	code, out, err := RunCommand(context.Background(), t.TempDir(), "sh", "-c", "echo stdout; echo stderr >&2")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(string(out), "stdout") || !strings.Contains(string(out), "stderr") {
		t.Errorf("output %q should contain both stdout and stderr", out)
	}
}

func TestRunCommandMissingBinary(t *testing.T) {
	_, _, err := RunCommand(context.Background(), t.TempDir(), "__no_such_binary__")
	if err == nil {
		t.Error("RunCommand with missing binary = nil, want error")
	}
}

// ---------------------------------------------------------------------------
// RunHookCommand – entry splitting + execution
// ---------------------------------------------------------------------------

func TestRunHookCommandSplitsEntry(t *testing.T) {
	// "echo hello" should be split into ["echo", "hello"] and then args appended.
	code, out, err := RunHookCommand(context.Background(), t.TempDir(), "echo hello", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunHookCommand: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(string(out), "hello") {
		t.Errorf("output %q does not contain 'hello'", out)
	}
}

func TestRunHookCommandAppendsArgs(t *testing.T) {
	code, out, err := RunHookCommand(context.Background(), t.TempDir(), "echo", []string{"arg1"}, []string{"arg2"}, nil)
	if err != nil {
		t.Fatalf("RunHookCommand: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(string(out), "arg1") || !strings.Contains(string(out), "arg2") {
		t.Errorf("output %q should contain arg1 and arg2", out)
	}
}

func TestRunHookCommandEmptyEntryReturnsError(t *testing.T) {
	_, _, err := RunHookCommand(context.Background(), t.TempDir(), "", nil, nil, nil)
	if err == nil {
		t.Error("RunHookCommand with empty entry = nil, want error")
	}
}

func TestRunHookCommandSetsEnv(t *testing.T) {
	env := []string{"MY_TEST_VAR=parity_check"}
	code, out, err := RunHookCommand(context.Background(), t.TempDir(), "sh", []string{"-c", "echo $MY_TEST_VAR"}, nil, env)
	if err != nil {
		t.Fatalf("RunHookCommand: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if !strings.Contains(string(out), "parity_check") {
		t.Errorf("output %q does not contain env value 'parity_check'", out)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func assertSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("len = %d, want %d: got %v, want %v", len(got), len(want), got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
