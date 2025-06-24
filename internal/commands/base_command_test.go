package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jessevdk/go-flags"
)

func TestBaseCommand_ParseArgsWithHelp(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
		expectNil   bool // for help case
	}{
		{
			name:        "normal args",
			args:        []string{"arg1", "arg2"},
			expectError: false,
			expectNil:   false,
		},
		{
			name:        "help flag",
			args:        []string{"--help"},
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "short help flag",
			args:        []string{"-h"},
			expectError: false,
			expectNil:   true,
		},
		{
			name:        "invalid flag",
			args:        []string{"--invalid-flag"},
			expectError: true,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := &BaseCommand{
				Name:        "test",
				Description: "Test command",
			}

			// Use CommonOptions as a simple test struct
			var opts CommonOptions

			remaining, err := bc.ParseArgsWithHelp(&opts, tt.args)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectNil && remaining != nil {
				t.Errorf("expected nil remaining args for help case")
			}
		})
	}
}

func TestBaseCommand_GenerateHelp(t *testing.T) {
	bc := &BaseCommand{
		Name:        "test-command",
		Description: "A test command for validation",
		Examples: []Example{
			{Command: "test-command --flag", Description: "Test with flag"},
		},
		Notes: []string{
			"This is a test note",
		},
	}

	var opts CommonOptions
	parser := flags.NewParser(&opts, flags.Default)

	help := bc.GenerateHelp(parser)

	if help == "" {
		t.Error("expected non-empty help output")
	}

	// Check that key components are included
	if !contains(help, "test-command") {
		t.Error("help should contain command name")
	}
	if !contains(help, "A test command for validation") {
		t.Error("help should contain description")
	}
}

func TestBaseCommand_ConfigFileExists(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, ConfigFileName)

	// Test non-existent file
	bc := &BaseCommand{}
	err := bc.ConfigFileExists(configFile)
	if err == nil {
		t.Error("expected error for non-existent config file")
	}

	// Create the file
	if writeErr := os.WriteFile(configFile, []byte("repos: []"), 0o644); writeErr != nil {
		t.Fatalf("failed to create test config file: %v", writeErr)
	}

	// Test existing file
	err = bc.ConfigFileExists(configFile)
	if err != nil {
		t.Errorf("unexpected error for existing config file: %v", err)
	}
}

func TestHookTypeOptions_GetDefaultHookTypes(t *testing.T) {
	tests := []struct {
		name        string
		hookTypes   []string
		defaultType string
		expected    []string
	}{
		{
			name:        "no hook types specified",
			hookTypes:   nil,
			defaultType: "pre-commit",
			expected:    []string{"pre-commit"},
		},
		{
			name:        "empty hook types",
			hookTypes:   []string{},
			defaultType: "pre-commit",
			expected:    []string{"pre-commit"},
		},
		{
			name:        "hook types specified",
			hookTypes:   []string{"pre-push", "pre-commit"},
			defaultType: "pre-commit",
			expected:    []string{"pre-push", "pre-commit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hto := &HookTypeOptions{
				HookTypes: tt.hookTypes,
			}

			result := hto.GetDefaultHookTypes(tt.defaultType)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d hook types, got %d", len(tt.expected), len(result))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("expected hook type %s at index %d, got %s", expected, i, result[i])
				}
			}
		})
	}
}

func TestHookTypeOptions_ValidateHookTypes(t *testing.T) {
	tests := []struct {
		name        string
		hookTypes   []string
		expectError bool
	}{
		{
			name:        "valid hook types",
			hookTypes:   []string{"pre-commit", "pre-push"},
			expectError: false,
		},
		{
			name:        "single valid hook type",
			hookTypes:   []string{"commit-msg"},
			expectError: false,
		},
		{
			name:        "invalid hook type",
			hookTypes:   []string{"invalid-hook"},
			expectError: true,
		},
		{
			name:        "mix of valid and invalid",
			hookTypes:   []string{"pre-commit", "invalid-hook"},
			expectError: true,
		},
		{
			name:        "empty hook types",
			hookTypes:   []string{},
			expectError: false,
		},
		{
			name: "all valid hook types",
			hookTypes: []string{
				"pre-commit", "pre-merge-commit", "pre-push", "prepare-commit-msg",
				"commit-msg", "post-checkout", "post-commit", "post-merge",
				"post-rewrite", "pre-rebase", "pre-auto-gc",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hto := &HookTypeOptions{
				HookTypes: tt.hookTypes,
			}

			err := hto.ValidateHookTypes()

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGitRepositoryCommand_RequireGitRepository(t *testing.T) {
	// This test requires a git repository, so we'll create a temporary one
	tempDir := t.TempDir()

	// Save current directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Test outside git repository
	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Fatalf("failed to change to temp directory: %v", chdirErr)
	}

	grc := &GitRepositoryCommand{}
	_, err = grc.RequireGitRepository()
	if err == nil {
		t.Error("expected error when not in git repository")
	}

	// Initialize git repository
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Note: This test would require actual git commands to work properly
	// In a real test environment, you might want to mock the git.NewRepository function
	// or use a test helper that sets up a proper git repository
}

func TestCommonOptions_Defaults(t *testing.T) {
	var opts CommonOptions
	parser := flags.NewParser(&opts, flags.Default)

	// Parse empty args to get defaults
	_, err := parser.ParseArgs([]string{})
	if err != nil {
		t.Fatalf("failed to parse empty args: %v", err)
	}

	// Check default values
	if opts.Color != "auto" {
		t.Errorf("expected default color 'auto', got '%s'", opts.Color)
	}

	if opts.Config != ConfigFileName {
		t.Errorf("expected default config '%s', got '%s'", ConfigFileName, opts.Config)
	}

	if opts.Help {
		t.Error("help should default to false")
	}

	if opts.Verbose {
		t.Error("verbose should default to false")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsInner(s, substr))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
