package commands

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitTemplatedirCommand_Synopsis(t *testing.T) {
	cmd := &InitTemplatedirCommand{}
	synopsis := cmd.Synopsis()

	if synopsis == "" {
		t.Error("Synopsis should not be empty")
	}

	// Should mention template directory or git init
	if !strings.Contains(strings.ToLower(synopsis), "template") &&
		!strings.Contains(strings.ToLower(synopsis), "init") {
		t.Errorf("Synopsis should mention 'template' or 'init', got: %s", synopsis)
	}
}

func TestInitTemplatedirCommand_Help(t *testing.T) {
	cmd := &InitTemplatedirCommand{}
	help := cmd.Help()

	if help == "" {
		t.Error("Help should not be empty")
	}

	// Check for expected content
	expectedContent := []string{
		"--config",
		"--hook-type",
		"--no-allow-missing-config", // Python uses --no-allow-missing-config (defaults to allow)
		"DIRECTORY",
		"template",
	}

	for _, expected := range expectedContent {
		if !strings.Contains(help, expected) {
			t.Errorf("Help should contain %q", expected)
		}
	}
}

func TestInitTemplatedirCommandFactory(t *testing.T) {
	cmd, err := InitTemplatedirCommandFactory()

	if err != nil {
		t.Errorf("Factory should not return error, got: %v", err)
	}

	if cmd == nil {
		t.Error("Factory should return a command")
	}

	if _, ok := cmd.(*InitTemplatedirCommand); !ok {
		t.Error("Factory should return *InitTemplatedirCommand")
	}
}

func TestInitTemplatedirCommand_Run_NoDirectory(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := cmd.Run([]string{})

	w.Close()
	os.Stdout = oldStdout

	output := make([]byte, 2048)
	n, _ := r.Read(output)
	outputStr := string(output[:n])

	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}

	if !strings.Contains(outputStr, "directory") || !strings.Contains(outputStr, "required") {
		t.Errorf("Output should mention directory is required, got: %s", outputStr)
	}
}

func TestInitTemplatedirCommand_Run_ValidDirectory(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Create a temp directory
	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create a config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	configContent := `repos:
- repo: local
  hooks:
  - id: test
    name: test
    entry: echo test
    language: system
`
	os.WriteFile(configFile, []byte(configContent), 0o644)

	exitCode := cmd.Run([]string{
		templateDir,
		"--config", configFile,
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Check that hooks directory was created
	hooksDir := filepath.Join(templateDir, "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		t.Error("Hooks directory should be created")
	}
}

func TestInitTemplatedirCommand_CreatesHooksDir(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	cmd.Run([]string{templateDir, "--config", configFile})

	hooksDir := filepath.Join(templateDir, "hooks")
	info, err := os.Stat(hooksDir)
	if err != nil {
		t.Fatalf("Hooks directory should exist: %v", err)
	}

	if !info.IsDir() {
		t.Error("hooks should be a directory")
	}
}

func TestInitTemplatedirCommand_CreatesHookScript(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	cmd.Run([]string{templateDir, "--config", configFile})

	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("pre-commit hook script should be created")
	}
}

func TestInitTemplatedirCommand_HookIsExecutable(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	cmd.Run([]string{templateDir, "--config", configFile})

	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("Hook should exist: %v", err)
	}

	// Check executable bits
	mode := info.Mode()
	if mode&0o100 == 0 {
		t.Error("Hook should be executable by owner")
	}
}

func TestInitTemplatedirCommand_HookScriptContent(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	cmd.Run([]string{templateDir, "--config", configFile})

	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("Failed to read hook: %v", err)
	}

	contentStr := string(content)

	// Check for expected content (bash shebang like Python's template)
	if !strings.HasPrefix(contentStr, "#!/usr/bin/env bash") {
		t.Error("Hook should start with bash shebang")
	}

	if !strings.Contains(contentStr, "hook-impl") {
		t.Error("Hook should use hook-impl")
	}

	if !strings.Contains(contentStr, "--hook-type=pre-commit") {
		t.Error("Hook should specify hook type")
	}

	if !strings.Contains(contentStr, "--config=") {
		t.Error("Hook should specify config file")
	}

	// Template dir hooks should always skip on missing config (parity with Python)
	if !strings.Contains(contentStr, "--skip-on-missing-config") {
		t.Error("Hook should include --skip-on-missing-config for template directories")
	}
}

func TestInitTemplatedirCommand_MultipleHookTypes(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	exitCode := cmd.Run([]string{
		templateDir,
		"--config", configFile,
		"--hook-type", "pre-commit",
		"--hook-type", "pre-push",
		"--hook-type", "commit-msg",
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Check all hooks were created
	hookTypes := []string{"pre-commit", "pre-push", "commit-msg"}
	for _, hookType := range hookTypes {
		hookPath := filepath.Join(templateDir, "hooks", hookType)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Errorf("Hook %s should be created", hookType)
		}
	}
}

func TestInitTemplatedirCommand_DefaultHookTypesFromConfig(t *testing.T) {
	// Test that init-templatedir uses default_install_hook_types from config
	// when no --hook-type is specified (matching Python's _hook_types() behavior)
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file with default_install_hook_types
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	configContent := `repos: []
default_install_hook_types:
  - pre-commit
  - pre-push
  - commit-msg
`
	os.WriteFile(configFile, []byte(configContent), 0o644)

	// Run WITHOUT --hook-type flags - should use config defaults
	exitCode := cmd.Run([]string{
		templateDir,
		"--config", configFile,
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Check all hooks from config's default_install_hook_types were created
	expectedHooks := []string{"pre-commit", "pre-push", "commit-msg"}
	for _, hookType := range expectedHooks {
		hookPath := filepath.Join(templateDir, "hooks", hookType)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Errorf("Hook %s should be created from config's default_install_hook_types", hookType)
		}
	}
}

func TestInitTemplatedirCommand_FallbackToPreCommitDefault(t *testing.T) {
	// Test that init-templatedir falls back to pre-commit when:
	// - No --hook-type specified
	// - Config doesn't have default_install_hook_types
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file WITHOUT default_install_hook_types
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	configContent := `repos: []
`
	os.WriteFile(configFile, []byte(configContent), 0o644)

	// Run WITHOUT --hook-type flags
	exitCode := cmd.Run([]string{
		templateDir,
		"--config", configFile,
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Should have pre-commit hook (fallback default)
	preCommitHook := filepath.Join(templateDir, "hooks", "pre-commit")
	if _, err := os.Stat(preCommitHook); os.IsNotExist(err) {
		t.Error("pre-commit hook should be created as fallback default")
	}

	// Should NOT have other hooks
	prePushHook := filepath.Join(templateDir, "hooks", "pre-push")
	if _, err := os.Stat(prePushHook); !os.IsNotExist(err) {
		t.Error("pre-push hook should NOT be created (not in defaults)")
	}
}

func TestInitTemplatedirCommand_ExplicitHookTypeOverridesConfigDefault(t *testing.T) {
	// Test that --hook-type flags override config's default_install_hook_types
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file with default_install_hook_types
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	configContent := `repos: []
default_install_hook_types:
  - pre-commit
  - pre-push
  - commit-msg
`
	os.WriteFile(configFile, []byte(configContent), 0o644)

	// Run WITH explicit --hook-type - should override config defaults
	exitCode := cmd.Run([]string{
		templateDir,
		"--config", configFile,
		"--hook-type", "pre-push", // Only pre-push, not all 3 from config
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Should have pre-push hook (explicitly requested)
	prePushHook := filepath.Join(templateDir, "hooks", "pre-push")
	if _, err := os.Stat(prePushHook); os.IsNotExist(err) {
		t.Error("pre-push hook should be created (explicitly requested)")
	}

	// Should NOT have pre-commit hook (config default overridden)
	preCommitHook := filepath.Join(templateDir, "hooks", "pre-commit")
	if _, err := os.Stat(preCommitHook); !os.IsNotExist(err) {
		t.Error("pre-commit hook should NOT be created (config default overridden)")
	}
}

func TestInitTemplatedirCommand_ConfigValidation(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Try with non-existent config AND --no-allow-missing-config
	// By default, missing config is allowed (like Python)
	exitCode := cmd.Run([]string{
		templateDir,
		"--config", "/nonexistent/config.yaml",
		"--no-allow-missing-config",
	})

	w.Close()
	os.Stdout = oldStdout

	output := make([]byte, 2048)
	n, _ := r.Read(output)
	outputStr := string(output[:n])

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for missing config with --no-allow-missing-config, got %d", exitCode)
	}

	if !strings.Contains(outputStr, "config") && !strings.Contains(outputStr, "not found") {
		t.Errorf("Output should mention config not found, got: %s", outputStr)
	}
}

func TestInitTemplatedirCommand_AllowMissingConfigByDefault(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// By default, missing config should be allowed (like Python's behavior)
	exitCode := cmd.Run([]string{
		templateDir,
		"--config", "/nonexistent/config.yaml",
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 (missing config allowed by default), got %d", exitCode)
	}

	// Check that hooks directory was still created
	hooksDir := filepath.Join(templateDir, "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		t.Error("Hooks directory should be created even with missing config")
	}
}

func TestInitTemplatedirCommand_HelpFlag(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := cmd.Run([]string{"--help"})

	w.Close()
	os.Stdout = oldStdout

	output := make([]byte, 4096)
	n, _ := r.Read(output)
	outputStr := string(output[:n])

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for --help, got %d", exitCode)
	}

	// Help output should contain usage info
	if !strings.Contains(outputStr, "DIRECTORY") || !strings.Contains(outputStr, "config") {
		t.Errorf("Help output should contain usage info, got: %s", outputStr)
	}
}

func TestInitTemplatedirCommand_ParseArguments(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tests := []struct {
		name                     string
		args                     []string
		wantDir                  string
		wantConfig               string
		wantHookTypes            []string
		wantNoAllowMissingConfig bool
		wantError                bool
	}{
		{
			name:          "basic directory",
			args:          []string{"/path/to/template"},
			wantDir:       "/path/to/template",
			wantConfig:    ".pre-commit-config.yaml",
			wantHookTypes: []string{"pre-commit"},
		},
		{
			name:       "with config",
			args:       []string{"/path/to/template", "--config", "custom.yaml"},
			wantDir:    "/path/to/template",
			wantConfig: "custom.yaml",
		},
		{
			name:          "with hook types",
			args:          []string{"/path/to/template", "-t", "pre-push"},
			wantDir:       "/path/to/template",
			wantHookTypes: []string{"pre-push"},
		},
		{
			name:                    "with no-allow-missing-config",
			args:                    []string{"/path/to/template", "--no-allow-missing-config"},
			wantDir:                 "/path/to/template",
			wantNoAllowMissingConfig: true,
		},
		{
			name:      "no directory",
			args:      []string{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, dir, rc := cmd.parseAndValidateArgs(tt.args)

			if tt.wantError {
				if rc == -1 {
					t.Error("Expected error but got success")
				}
				return
			}

			if rc != -1 {
				t.Errorf("Expected success but got error code %d", rc)
				return
			}

			if dir != tt.wantDir {
				t.Errorf("Directory = %q, want %q", dir, tt.wantDir)
			}

			if tt.wantConfig != "" && opts.Config != tt.wantConfig {
				t.Errorf("Config = %q, want %q", opts.Config, tt.wantConfig)
			}

			if tt.wantNoAllowMissingConfig && !opts.NoAllowMissingConfig {
				t.Error("NoAllowMissingConfig should be true")
			}
		})
	}
}

func TestInitTemplatedirCommand_ColorOption(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tests := []struct {
		name      string
		args      []string
		wantColor string
		wantError bool
	}{
		{
			name:      "default color (auto)",
			args:      []string{"/path/to/template"},
			wantColor: "auto",
		},
		{
			name:      "color auto",
			args:      []string{"/path/to/template", "--color", "auto"},
			wantColor: "auto",
		},
		{
			name:      "color always",
			args:      []string{"/path/to/template", "--color", "always"},
			wantColor: "always",
		},
		{
			name:      "color never",
			args:      []string{"/path/to/template", "--color", "never"},
			wantColor: "never",
		},
		{
			name:      "invalid color option",
			args:      []string{"/path/to/template", "--color", "invalid"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, _, rc := cmd.parseAndValidateArgs(tt.args)

			if tt.wantError {
				if rc == -1 {
					t.Error("Expected error for invalid color option but got success")
				}
				return
			}

			if rc != -1 {
				t.Errorf("Expected success but got error code %d", rc)
				return
			}

			if opts.Color != tt.wantColor {
				t.Errorf("Color = %q, want %q", opts.Color, tt.wantColor)
			}
		})
	}
}

func TestInitTemplatedirCommand_HelpShowsColorOption(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	helpText := cmd.Help()

	// Help should mention the --color option
	if !strings.Contains(helpText, "--color") {
		t.Error("Help text should contain --color option")
	}

	// Help should show the valid color choices
	if !strings.Contains(helpText, "auto") {
		t.Error("Help text should mention 'auto' color option")
	}
}

func TestInitTemplatedirCommand_HookScriptUsesConfigPath(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file with custom path
	configFile := filepath.Join(tempDir, "custom-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	cmd.Run([]string{
		templateDir,
		"--config", configFile,
	})

	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("Failed to read hook: %v", err)
	}

	if !strings.Contains(string(content), configFile) {
		t.Errorf("Hook script should reference config file %s", configFile)
	}
}

func TestInitTemplatedirCommand_CreatesNestedDirectory(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	// Create a deeply nested path
	templateDir := filepath.Join(tempDir, "path", "to", "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	exitCode := cmd.Run([]string{
		templateDir,
		"--config", configFile,
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// Check the nested structure was created
	hooksDir := filepath.Join(templateDir, "hooks")
	if _, err := os.Stat(hooksDir); os.IsNotExist(err) {
		t.Error("Nested hooks directory should be created")
	}
}

func TestInitTemplatedirCommand_ShortFlags(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, "config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Use short flags
	exitCode := cmd.Run([]string{
		templateDir,
		"-c", configFile,
		"-t", "pre-commit",
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with short flags, got %d", exitCode)
	}
}

func TestInitTemplatedirCommand_ExistingDirectory(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Pre-create the template directory
	os.MkdirAll(filepath.Join(templateDir, "hooks"), 0o755)

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Should succeed even if directory exists
	exitCode := cmd.Run([]string{
		templateDir,
		"--config", configFile,
	})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for existing directory, got %d", exitCode)
	}
}

func TestInitTemplatedirCommand_HookScriptForDifferentTypes(t *testing.T) {
	hookTypes := []string{
		"pre-commit",
		"pre-push",
		"commit-msg",
		"prepare-commit-msg",
		"post-checkout",
		"post-commit",
		"post-merge",
		"post-rewrite",
		"pre-rebase",
	}

	for _, hookType := range hookTypes {
		t.Run(hookType, func(t *testing.T) {
			cmd := &InitTemplatedirCommand{}

			tempDir := t.TempDir()
			templateDir := filepath.Join(tempDir, "git-template")

			// Create config file
			configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
			os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

			exitCode := cmd.Run([]string{
				templateDir,
				"--config", configFile,
				"--hook-type", hookType,
			})

			if exitCode != 0 {
				t.Errorf("Expected exit code 0 for %s, got %d", hookType, exitCode)
			}

			hookPath := filepath.Join(templateDir, "hooks", hookType)
			content, err := os.ReadFile(hookPath)
			if err != nil {
				t.Fatalf("Failed to read hook: %v", err)
			}

			if !strings.Contains(string(content), "--hook-type="+hookType) {
				t.Errorf("Hook script should contain --hook-type=%s", hookType)
			}
		})
	}
}

func TestInitTemplatedirCommand_GitConfigWarning(t *testing.T) {
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Capture stdout to check for warning
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	exitCode := cmd.Run([]string{templateDir, "--config", configFile})

	w.Close()
	os.Stdout = oldStdout

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should print a warning since the git config is likely not set to the temp dir
	if !strings.Contains(output, "[WARNING]") {
		// This is expected when init.templateDir is not set or differs
		// We just verify the command runs successfully
		t.Log("No warning printed (init.templateDir may already be configured)")
	}

	if strings.Contains(output, "[WARNING]") {
		// Verify the warning includes the template directory
		if !strings.Contains(output, "init.templateDir") {
			t.Error("Warning should mention init.templateDir")
		}
	}
}

func TestInitTemplatedirCommand_CheckGitTemplateDir(t *testing.T) {
	// Test the checkGitTemplateDir method directly
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create the directory so we can get its absolute path
	os.MkdirAll(templateDir, 0o755)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd.checkGitTemplateDir(templateDir)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should typically warn since the temp dir won't match init.templateDir
	// Unless the user has init.templateDir configured to this exact path
	t.Logf("checkGitTemplateDir output: %q", output)
}

func TestInitTemplatedirCommand_CheckGitTemplateDir_Behaviors(t *testing.T) {
	// Test various scenarios for git config check behavior
	// This matches Python's check: cmd_output('git', 'config', 'init.templateDir')
	tests := []struct {
		name            string
		setupFunc       func(t *testing.T, templateDir string) func()
		expectWarning   bool
		warningContains string
	}{
		{
			name: "warns_when_not_configured",
			setupFunc: func(t *testing.T, templateDir string) func() {
				// Save current config
				oldConfig, _ := exec.Command("git", "config", "init.templateDir").Output()
				// Unset the config
				exec.Command("git", "config", "--global", "--unset", "init.templateDir").Run()
				return func() {
					if len(oldConfig) > 0 {
						exec.Command("git", "config", "--global", "init.templateDir", strings.TrimSpace(string(oldConfig))).Run()
					}
				}
			},
			expectWarning:   true,
			warningContains: "[WARNING] `init.templateDir` not set to the target directory",
		},
		{
			name: "warns_when_configured_to_different_path",
			setupFunc: func(t *testing.T, templateDir string) func() {
				oldConfig, _ := exec.Command("git", "config", "init.templateDir").Output()
				// Set to a different path
				exec.Command("git", "config", "--global", "init.templateDir", "/tmp/some-other-template").Run()
				return func() {
					if len(oldConfig) > 0 {
						exec.Command("git", "config", "--global", "init.templateDir", strings.TrimSpace(string(oldConfig))).Run()
					} else {
						exec.Command("git", "config", "--global", "--unset", "init.templateDir").Run()
					}
				}
			},
			expectWarning:   true,
			warningContains: "[WARNING] `init.templateDir` not set to the target directory",
		},
		{
			name: "no_warning_when_configured_correctly",
			setupFunc: func(t *testing.T, templateDir string) func() {
				oldConfig, _ := exec.Command("git", "config", "init.templateDir").Output()
				// Get absolute path
				absPath, _ := filepath.Abs(templateDir)
				// Set to the template dir
				exec.Command("git", "config", "--global", "init.templateDir", absPath).Run()
				return func() {
					if len(oldConfig) > 0 {
						exec.Command("git", "config", "--global", "init.templateDir", strings.TrimSpace(string(oldConfig))).Run()
					} else {
						exec.Command("git", "config", "--global", "--unset", "init.templateDir").Run()
					}
				}
			},
			expectWarning:   false,
			warningContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &InitTemplatedirCommand{}
			tempDir := t.TempDir()
			templateDir := filepath.Join(tempDir, "git-template")
			os.MkdirAll(templateDir, 0o755)

			// Setup and get cleanup function
			cleanup := tt.setupFunc(t, templateDir)
			defer cleanup()

			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			cmd.checkGitTemplateDir(templateDir)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			if tt.expectWarning {
				if !strings.Contains(output, tt.warningContains) {
					t.Errorf("Expected warning containing %q, got: %q", tt.warningContains, output)
				}
			} else {
				if strings.Contains(output, "[WARNING]") {
					t.Errorf("Expected no warning, got: %q", output)
				}
			}
		})
	}
}

// Tests for delegation to HookInstaller (Python parity: init_templatedir delegates to install())

func TestInitTemplatedirCommand_DelegatesToHookInstaller(t *testing.T) {
	// Verify that init-templatedir uses HookInstaller under the hood
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	exitCode := cmd.Run([]string{templateDir, "--config", configFile})

	if exitCode != 0 {
		t.Fatalf("Expected exit code 0, got %d", exitCode)
	}

	// The installer field should be set after Run()
	if cmd.installer == nil {
		t.Error("Command should have installer set after Run()")
	}
}

func TestInitTemplatedirCommand_UsesOverwriteTrue(t *testing.T) {
	// Python's init_templatedir uses overwrite=True
	// Verify that we overwrite existing hooks
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Pre-create the hook with different content
	hooksDir := filepath.Join(templateDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(hookPath, []byte("#!/bin/sh\necho 'original'\n"), 0o755)

	// Run init-templatedir - should overwrite
	exitCode := cmd.Run([]string{templateDir, "--config", configFile})

	if exitCode != 0 {
		t.Fatalf("Expected exit code 0, got %d", exitCode)
	}

	// Verify the hook was overwritten (should contain hook-impl now)
	content, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(content), "hook-impl") {
		t.Error("init-templatedir should overwrite existing hooks (overwrite=True like Python)")
	}
}

func TestInitTemplatedirCommand_UsesSkipOnMissingConfigTrue(t *testing.T) {
	// Python's init_templatedir uses skip_on_missing_config=True
	// Verify the generated hook script includes --skip-on-missing-config
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	cmd.Run([]string{templateDir, "--config", configFile})

	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("Failed to read hook: %v", err)
	}

	if !strings.Contains(string(content), "--skip-on-missing-config") {
		t.Error("init-templatedir should use --skip-on-missing-config in generated hooks (like Python)")
	}
}

func TestInitTemplatedirCommand_DelegatesWithCorrectGitDir(t *testing.T) {
	// Verify that init-templatedir passes the template directory as GitDir
	// (Python passes git_dir=directory to install())
	cmd := &InitTemplatedirCommand{}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "my-custom-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	cmd.Run([]string{templateDir, "--config", configFile})

	// Verify hooks were created in the template directory (not .git/hooks)
	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("Hook should be created in template directory, not .git/hooks")
	}

	// Verify .git/hooks doesn't exist (we're not in a git repo context)
	gitHookPath := filepath.Join(tempDir, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(gitHookPath); err == nil {
		t.Error("Hook should NOT be created in .git/hooks for init-templatedir")
	}
}

func TestInitTemplatedirCommand_CustomInstaller(t *testing.T) {
	// Test that we can inject a custom installer (useful for testing)
	customInstaller := NewHookInstaller()
	cmd := &InitTemplatedirCommand{
		installer: customInstaller,
	}

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	exitCode := cmd.Run([]string{templateDir, "--config", configFile})

	if exitCode != 0 {
		t.Fatalf("Expected exit code 0, got %d", exitCode)
	}

	// Verify the injected installer was used
	if cmd.installer != customInstaller {
		t.Error("Custom installer should be used when provided")
	}
}
