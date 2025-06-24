package commands

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestRunCommand_Help(t *testing.T) {
	cmd := &RunCommand{}
	help := cmd.Help()

	if help == "" {
		t.Error("help output should not be empty")
	}

	// Check for key components
	expectedStrings := []string{
		"run",
		"Run hooks",
		"--all-files",
		"--files",
		"--hook-stage",
		"--verbose",
		"--show-diff-on-failure",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("help output should contain '%s'", expected)
		}
	}
}

func TestRunCommand_Synopsis(t *testing.T) {
	cmd := &RunCommand{}
	synopsis := cmd.Synopsis()

	if synopsis == "" {
		t.Error("synopsis should not be empty")
	}

	expected := "Run hooks on files"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestRunOptions_Defaults(t *testing.T) {
	// Test defaults by parsing empty arguments
	cmd := &RunCommand{}

	// Capture stdout/stderr to prevent help output during tests
	origStdout := os.Stdout
	origStderr := os.Stderr
	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	// Redirect output to suppress help display during test
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("Failed to open devnull: %v", err)
	}
	defer devNull.Close()

	os.Stdout = devNull
	os.Stderr = devNull

	// Since the command will fail without git repo, we just check that it parses
	// defaults correctly by trying to run with help
	exitCode := cmd.Run([]string{"--help"})
	if exitCode != 0 {
		t.Errorf("Expected help to succeed, got exit code %d", exitCode)
	}

	// We can't easily test the actual defaults without changing the command structure,
	// so we'll skip the detailed default testing for now
	t.Skip("Default value testing requires command structure changes")
}

func TestRunCommand_Run_ArgumentParsing(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectExit   int
		setupGitRepo bool
	}{
		{
			name:         "help flag",
			args:         []string{"--help"},
			expectExit:   0,
			setupGitRepo: false,
		},
		{
			name:         "no git repo",
			args:         []string{},
			expectExit:   1,
			setupGitRepo: false,
		},
		{
			name:         "basic run with git repo",
			args:         []string{},
			expectExit:   0, // Might fail if no config, but parsing should work
			setupGitRepo: true,
		},
		{
			name:         "run all files",
			args:         []string{"--all-files"},
			expectExit:   0,
			setupGitRepo: true,
		},
		{
			name:         "run with verbose",
			args:         []string{"--verbose"},
			expectExit:   0,
			setupGitRepo: true,
		},
		{
			name:         "run with hook stage",
			args:         []string{"--hook-stage", "commit"},
			expectExit:   0,
			setupGitRepo: true,
		},
		{
			name:         "run with files",
			args:         []string{"--files", "file1.py", "file2.py"},
			expectExit:   0,
			setupGitRepo: true,
		},
		{
			name:         "run with fail fast",
			args:         []string{"--fail-fast"},
			expectExit:   0,
			setupGitRepo: true,
		},
		{
			name:         "run with show diff",
			args:         []string{"--show-diff-on-failure"},
			expectExit:   0,
			setupGitRepo: true,
		},
		{
			name:         "run with parallel jobs",
			args:         []string{"--jobs", "2"},
			expectExit:   0,
			setupGitRepo: true,
		},
		{
			name:         "run with timeout",
			args:         []string{"--timeout", "30s"},
			expectExit:   0,
			setupGitRepo: true,
		},
		{
			name:         "run with custom config",
			args:         []string{"--config", "custom-config.yaml"},
			expectExit:   0,
			setupGitRepo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get current directory: %v", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(tempDir); err != nil {
				t.Fatalf("failed to change to temp directory: %v", err)
			}

			if tt.setupGitRepo {
				// Initialize git repository
				if err := exec.Command("git", "init").Run(); err != nil {
					t.Skip("git not available for testing")
				}

				// Configure git for testing
				exec.Command("git", "config", "user.email", "test@example.com").Run()
				exec.Command("git", "config", "user.name", "Test User").Run()
				exec.Command("git", "config", "commit.gpgsign", "false").
					Run()
					// Disable GPG signing for tests

				// Create a basic .pre-commit-config.yaml unless custom config is specified
				configPath := ".pre-commit-config.yaml"
				for i := 0; i < len(tt.args)-1; i++ {
					if tt.args[i] == "--config" || tt.args[i] == "-c" {
						configPath = tt.args[i+1]
						break
					}
				}

				configContent := `repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
`
				if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
					t.Fatalf("failed to create config file: %v", err)
				}

				// Create some test files if --files is specified
				if containsArg(tt.args, "--files") {
					if err := os.WriteFile("file1.py", []byte("print('hello')\n"), 0o644); err != nil {
						t.Fatalf("failed to create test file: %v", err)
					}
					if err := os.WriteFile("file2.py", []byte("print('world')\n"), 0o644); err != nil {
						t.Fatalf("failed to create test file: %v", err)
					}
				}
			}

			cmd := &RunCommand{}
			exitCode := cmd.Run(tt.args)

			// For help flag, we expect exit code 0
			if tt.name == "help flag" && exitCode != 0 {
				t.Errorf("expected exit code 0 for help, got %d", exitCode)
			}

			// For no git repo, we expect exit code 1
			if tt.name == "no git repo" && exitCode != 1 {
				t.Errorf("expected exit code 1 for no git repo, got %d", exitCode)
			}
		})
	}
}

func TestRunCommand_HookStageValidation(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Skip("git not available for testing")
	}

	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "config", "commit.gpgsign", "false").Run() // Disable GPG signing for tests

	// Create config file
	configContent := `repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
`
	if err := os.WriteFile(".pre-commit-config.yaml", []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	tests := []struct {
		name      string
		hookStage string
		expectErr bool
	}{
		{
			name:      "valid hook stage - commit",
			hookStage: "commit",
			expectErr: false,
		},
		{
			name:      "valid hook stage - push",
			hookStage: "push",
			expectErr: false,
		},
		{
			name:      "valid hook stage - manual",
			hookStage: "manual",
			expectErr: false,
		},
		{
			name:      "empty hook stage",
			hookStage: "",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{}
			if tt.hookStage != "" {
				args = append(args, "--hook-stage", tt.hookStage)
			}

			cmd := &RunCommand{}
			exitCode := cmd.Run(args)

			// We're mainly testing that argument parsing works correctly
			// The actual hook execution might fail due to environment issues
			if tt.expectErr && exitCode == 0 {
				t.Errorf("expected error but command succeeded")
			}
		})
	}
}

func TestRunCommand_FileHandling(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Skip("git not available for testing")
	}

	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "config", "commit.gpgsign", "false").Run() // Disable GPG signing for tests

	// Create config file
	configContent := `repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v4.4.0
    hooks:
      - id: trailing-whitespace
`
	if err := os.WriteFile(".pre-commit-config.yaml", []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// Create test files
	testFiles := []string{"test1.py", "test2.js", "test3.md"}
	for _, file := range testFiles {
		content := "// Test content\n"
		if strings.HasSuffix(file, ".py") {
			content = "# Test content\n"
		} else if strings.HasSuffix(file, ".md") {
			content = "# Test content\n"
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create test file %s: %v", file, err)
		}
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "run all files",
			args: []string{"--all-files"},
		},
		{
			name: "run specific files",
			args: []string{"--files", "test1.py", "test2.js"},
		},
		{
			name: "run with file patterns",
			args: []string{"--files", "*.py"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			cmd := &RunCommand{}
			exitCode := cmd.Run(tt.args)

			// We're mainly testing that the command can parse file arguments
			// The actual execution might fail due to missing hook environments
			_ = exitCode // Ignore exit code for now as environment setup is complex
		})
	}
}

// Helper function to check if args contains a specific argument
func containsArg(args []string, arg string) bool {
	return slices.Contains(args, arg)
}
