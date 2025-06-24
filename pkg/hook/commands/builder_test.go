package commands

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// Test constants
const (
	testEcho = "echo"
)

func TestNewBuilder(t *testing.T) {
	repoRoot := "/test/repo"
	builder := NewBuilder(repoRoot)

	if builder == nil {
		t.Fatal("Expected non-nil builder")
	}

	if builder.repoRoot != repoRoot {
		t.Errorf("Expected repoRoot %s, got %s", repoRoot, builder.repoRoot)
	}
}

func TestBuilder_shouldPassFilenames(t *testing.T) {
	tests := []struct {
		name     string
		hook     config.Hook
		expected bool
	}{
		{
			name: "hook with pass_filenames explicitly true",
			hook: config.Hook{
				ID:            "test",
				PassFilenames: boolPtr(true),
			},
			expected: true,
		},
		{
			name: "hook with pass_filenames explicitly false",
			hook: config.Hook{
				ID:            "test",
				PassFilenames: boolPtr(false),
			},
			expected: false,
		},
		{
			name: "hook with docker language defaults to false",
			hook: config.Hook{
				ID:       "test",
				Language: "docker",
			},
			expected: false,
		},
		{
			name: "hook with docker_image language defaults to false",
			hook: config.Hook{
				ID:       "test",
				Language: "docker_image",
			},
			expected: false,
		},
		{
			name: "hook with system language defaults to true",
			hook: config.Hook{
				ID:       "test",
				Language: "system",
			},
			expected: true,
		},
		{
			name: "hook with python language defaults to true",
			hook: config.Hook{
				ID:       "test",
				Language: "python",
			},
			expected: true,
		},
		{
			name: "hook with no language defaults to true",
			hook: config.Hook{
				ID: "test",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldPassFilenames(tt.hook)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuilder_BuildCommand_SystemLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "system",
		Entry:    testEcho,
		Args:     []string{"hello"},
	}

	files := []string{"file1.txt", "file2.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	if cmd.Path != testEcho && filepath.Base(cmd.Path) != testEcho {
		t.Errorf("Expected command to be echo, got %s", cmd.Path)
	}

	expectedArgs := []string{testEcho, "hello", "file1.txt", "file2.txt"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}

	for i, arg := range expectedArgs {
		if i < len(cmd.Args) && cmd.Args[i] != arg {
			t.Errorf("Expected arg %d to be %s, got %s", i, arg, cmd.Args[i])
		}
	}
}

func TestBuilder_BuildCommand_FailLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "fail",
		Entry:    "This hook always fails",
	}

	files := []string{"file1.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Fail language should use sh or cmd
	expectedCmd := "sh"
	if runtime.GOOS == "windows" {
		expectedCmd = "cmd"
	}

	if filepath.Base(cmd.Path) != expectedCmd {
		t.Errorf("Expected command to be %s, got %s", expectedCmd, filepath.Base(cmd.Path))
	}
}

func TestBuilder_BuildCommand_ScriptLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "script",
		Entry:    "test.sh",
		Args:     []string{"--verbose"},
	}

	files := []string{"file1.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Script should execute the entry directly
	if !filepath.IsAbs(cmd.Path) {
		// On some systems, it might resolve to a relative path
		if cmd.Args[0] != "test.sh" {
			t.Errorf("Expected script entry in args, got %s", cmd.Args[0])
		}
	}
}

func TestBuilder_BuildCommand_PythonLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "python",
		Entry:    "script.py",
		Args:     []string{"--check"},
	}

	files := []string{"file1.py"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Should contain python in the path
	cmdName := filepath.Base(cmd.Path)
	if cmdName != "python" && cmdName != "python3" && cmdName != "python.exe" &&
		cmdName != "python3.exe" {
		t.Errorf("Expected python command, got %s", cmdName)
	}
}

func TestBuilder_BuildCommand_NodeLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "node",
		Entry:    "script.js",
		Args:     []string{"--fix"},
	}

	files := []string{"file1.js"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Should directly execute the script/entry (not via 'node')
	cmdName := filepath.Base(cmd.Path)
	if cmdName != "script.js" {
		t.Errorf("Expected script.js command, got %s", cmdName)
	}
}

func TestBuilder_BuildCommand_DockerLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "docker",
		Entry:    "my-docker-image",
		Args:     []string{"--arg1"},
	}

	files := []string{"file1.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Should use docker command
	cmdName := filepath.Base(cmd.Path)
	if cmdName != LanguageDocker && cmdName != "docker.exe" {
		t.Errorf("Expected docker command, got %s", cmdName)
	}

	// Docker should not pass filenames by default
	hasFilename := slices.Contains(cmd.Args, "file1.txt")
	if hasFilename {
		t.Error("Docker language should not pass filenames by default")
	}
}

func TestBuilder_BuildCommand_DockerImageLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "docker_image",
		Entry:    "alpine:latest",
	}

	files := []string{"file1.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Should use docker command
	cmdName := filepath.Base(cmd.Path)
	if cmdName != "docker" && cmdName != "docker.exe" {
		t.Errorf("Expected docker command, got %s", cmdName)
	}
}

func TestBuilder_BuildCommand_GoLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "golang",
		Entry:    "go fmt",
		Args:     []string{"./..."},
	}

	files := []string{"file1.go"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Should use go command
	cmdName := filepath.Base(cmd.Path)
	if cmdName != "go" && cmdName != "go.exe" {
		t.Errorf("Expected go command, got %s", cmdName)
	}
}

func TestBuilder_BuildCommand_RustLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "rust",
		Entry:    "cargo fmt",
		Args:     []string{"--check"},
	}

	files := []string{"file1.rs"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Should use cargo fmt command (not just cargo)
	expectedCmd := "cargo fmt"
	actualCmd := cmd.Args[0] // The actual command being run
	if actualCmd != expectedCmd {
		t.Errorf("Expected %s command, got %s", expectedCmd, actualCmd)
	}
}

func TestBuilder_BuildCommand_RubyLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "ruby",
		Entry:    "rubocop",
		Args:     []string{"--auto-correct"},
	}

	files := []string{"file1.rb"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Should use ruby command
	cmdName := filepath.Base(cmd.Path)
	if cmdName != "ruby" && cmdName != "ruby.exe" {
		t.Errorf("Expected ruby command, got %s", cmdName)
	}
}

func TestBuilder_BuildCommand_DefaultLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:    "test-hook",
		Entry: "echo",
		Args:  []string{"test"},
	}

	files := []string{"file1.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Default language should be system
	if cmd.Path != "echo" && filepath.Base(cmd.Path) != "echo" {
		t.Errorf("Expected echo command for default language, got %s", cmd.Path)
	}
}

func TestBuilder_BuildCommand_PassFilenamesDisabled(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:            "test-hook",
		Language:      "system",
		Entry:         "echo",
		Args:          []string{"hello"},
		PassFilenames: boolPtr(false),
	}

	files := []string{"file1.txt", "file2.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	// Should not include filenames when pass_filenames is false
	expectedArgs := []string{"echo", "hello"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}

	for i, arg := range expectedArgs {
		if i < len(cmd.Args) && cmd.Args[i] != arg {
			t.Errorf("Expected arg %d to be %s, got %s", i, arg, cmd.Args[i])
		}
	}
}

func TestBuilder_BuildCommand_NoFiles(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "system",
		Entry:    "echo",
		Args:     []string{"hello"},
	}

	files := []string{} // No files
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"echo", "hello"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

// Additional language tests for improved coverage

func TestBuilder_BuildCommand_PerlLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "perl",
		Entry:    "test.pl",
		Args:     []string{"-w"},
	}

	files := []string{"file1.txt", "file2.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"perl", "test.pl", "-w", "file1.txt", "file2.txt"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}

	for i, arg := range expectedArgs {
		if i < len(cmd.Args) && cmd.Args[i] != arg {
			t.Errorf("Expected arg %d to be %s, got %s", i, arg, cmd.Args[i])
		}
	}
}

func TestBuilder_BuildCommand_LuaLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "lua",
		Entry:    "test.lua",
		Args:     []string{"--verbose"},
	}

	files := []string{"file1.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"lua", "test.lua", "--verbose", "file1.txt"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_SwiftLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "swift",
		Entry:    "test.swift",
		Args:     []string{},
	}

	files := []string{"file1.swift"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"swift", "test.swift", "file1.swift"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_RLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "r",
		Entry:    "test.R",
		Args:     []string{"--slave"},
	}

	files := []string{"data.csv"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"Rscript", "test.R", "--slave", "data.csv"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_HaskellLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "haskell",
		Entry:    "test.hs",
		Args:     []string{},
	}

	files := []string{"file1.hs"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"runhaskell", "test.hs", "file1.hs"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_CondaLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "conda",
		Entry:    "flake8",
		Args:     []string{"--max-line-length=88"},
	}

	files := []string{"test.py"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"flake8", "--max-line-length=88", "test.py"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_CoursierLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "coursier",
		Entry:    "scalafmt",
		Args:     []string{"--test"},
	}

	files := []string{"Test.scala"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"scalafmt", "--test", "Test.scala"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_DartLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "dart",
		Entry:    "dartfmt",
		Args:     []string{"-w"},
	}

	files := []string{"lib/main.dart"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"dartfmt", "-w", "lib/main.dart"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_DotnetLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "dotnet",
		Entry:    "dotnet-format",
		Args:     []string{"--check"},
	}

	files := []string{"Program.cs"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"dotnet-format", "--check", "Program.cs"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_JuliaLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "julia",
		Entry:    "format.jl",
		Args:     []string{"--check"},
	}

	files := []string{"src/main.jl"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"julia", "format.jl", "--check", "src/main.jl"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_PygrepLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "pygrep",
		Entry:    "pygrep-hooks-django",
		Args:     []string{},
	}

	files := []string{"views.py"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"pygrep-hooks-django", "views.py"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_GenericLanguage(t *testing.T) {
	builder := NewBuilder("/test/repo")

	hook := config.Hook{
		ID:       "test-hook",
		Language: "unknown-language",
		Entry:    "custom-linter",
		Args:     []string{"--strict"},
	}

	files := []string{"file1.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"custom-linter", "--strict", "file1.txt"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}
}

func TestBuilder_BuildCommand_SystemLanguage_EdgeCases(t *testing.T) {
	builder := NewBuilder("/test/repo")

	// Test with empty entry
	hook := config.Hook{
		ID:       "test-hook",
		Language: "system",
		Entry:    "",
		Args:     []string{},
	}

	files := []string{"file1.txt"}
	repo := config.Repo{Repo: "local"}
	env := map[string]string{}

	_, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err == nil {
		t.Error("Expected error for empty entry")
	}

	// Test with complex shell command
	hook.Entry = "echo hello world"
	cmd, err := builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	expectedArgs := []string{"echo", "hello", "world", "file1.txt"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}

	// Test with entry that's only spaces
	hook.Entry = "   "
	_, err = builder.BuildCommand(hook, files, "/test/repo", repo, env)
	if err == nil {
		t.Error("Expected error for whitespace-only entry")
	}
}

// Helper function to create a bool pointer
func boolPtr(b bool) *bool {
	return &b
}
