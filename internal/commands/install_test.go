package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Test helper to set up test environment
func setupInstallTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// Test helper to initialize a git repo
func initGitRepoForInstall(t *testing.T, dir string) {
	t.Helper()

	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create initial commit so HEAD exists
	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	// Create a file
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to create readme: %v", err)
	}

	// Add and commit
	if _, err := w.Add("README.md"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "Test", Email: "test@test.com"},
	})
	if err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
}

// createPreCommitConfig creates a minimal pre-commit config file
func createPreCommitConfigForInstall(t *testing.T, dir string) string {
	t.Helper()
	configPath := filepath.Join(dir, ".pre-commit-config.yaml")
	content := `repos:
  - repo: local
    hooks:
      - id: test
        name: test
        entry: echo test
        language: system
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	return configPath
}

// ====================
// Synopsis Tests
// ====================

func TestInstallCommand_Synopsis(t *testing.T) {
	cmd := &InstallCommand{}
	synopsis := cmd.Synopsis()

	if synopsis == "" {
		t.Error("Synopsis should not be empty")
	}

	// Should mention "install"
	lower := strings.ToLower(synopsis)
	if !strings.Contains(lower, "install") {
		t.Error("Synopsis should mention 'install'")
	}
}

// ====================
// Help Tests
// ====================

func TestInstallCommand_Help(t *testing.T) {
	cmd := &InstallCommand{}
	help := cmd.Help()

	if help == "" {
		t.Error("Help should not be empty")
	}

	// Should contain option descriptions
	expectedOptions := []string{
		"--config",
		"--hook-type",
		"--overwrite",
		"--install-hooks",
		"--allow-missing-config",
	}

	for _, opt := range expectedOptions {
		if !strings.Contains(help, opt) {
			t.Errorf("Help should contain option: %s", opt)
		}
	}
}

func TestInstallCommand_Help_ContainsUsage(t *testing.T) {
	cmd := &InstallCommand{}
	help := cmd.Help()

	// Should contain usage line
	if !strings.Contains(help, "usage: pre-commit install") {
		t.Error("Help should contain usage line")
	}

	// Should contain options section
	if !strings.Contains(help, "options:") {
		t.Error("Help should contain options section")
	}
}

func TestInstallCommand_Help_ContainsHookTypes(t *testing.T) {
	cmd := &InstallCommand{}
	help := cmd.Help()

	// Should list available hook types
	hookTypes := []string{
		"pre-commit",
		"pre-push",
		"commit-msg",
	}

	for _, ht := range hookTypes {
		if !strings.Contains(help, ht) {
			t.Errorf("Help should list hook type: %s", ht)
		}
	}
}

// ====================
// Factory Tests
// ====================

func TestInstallCommandFactory(t *testing.T) {
	cmd, err := InstallCommandFactory()

	if err != nil {
		t.Errorf("Factory should not return error, got: %v", err)
	}

	if cmd == nil {
		t.Error("Factory should return a command")
	}

	_, ok := cmd.(*InstallCommand)
	if !ok {
		t.Error("Factory should return *InstallCommand")
	}
}

// ====================
// Run Tests - Basic
// ====================

func TestInstallCommand_Run_HelpFlag(t *testing.T) {
	cmd := &InstallCommand{}

	exitCode := cmd.Run([]string{"--help"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for help, got: %d", exitCode)
	}
}

func TestInstallCommand_Run_HelpFlagShort(t *testing.T) {
	cmd := &InstallCommand{}

	exitCode := cmd.Run([]string{"-h"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for -h, got: %d", exitCode)
	}
}

func TestInstallCommand_Run_NotGitRepo(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for non-git repo, got: %d", exitCode)
	}
}

func TestInstallCommand_Run_MissingConfig(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	// Should fail because config doesn't exist
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for missing config, got: %d", exitCode)
	}
}

func TestInstallCommand_Run_AllowMissingConfig(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{"--allow-missing-config"})

	// Should succeed with --allow-missing-config
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with --allow-missing-config, got: %d", exitCode)
	}

	// Verify hook was created
	hookPath := filepath.Join(tempDir, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("Pre-commit hook should be created")
	}
}

// ====================
// Run Tests - Hook Installation
// ====================

func TestInstallCommand_Run_InstallsPreCommitByDefault(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify pre-commit hook was created
	hookPath := filepath.Join(tempDir, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("Pre-commit hook should be created by default")
	}
}

func TestInstallCommand_Run_SingleHookType(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{"--hook-type", "pre-push"})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify pre-push hook was created
	hookPath := filepath.Join(tempDir, ".git", "hooks", "pre-push")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("Pre-push hook should be created")
	}
}

func TestInstallCommand_Run_MultipleHookTypes(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{"-t", "pre-commit", "-t", "pre-push", "-t", "commit-msg"})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify all hooks were created
	expectedHooks := []string{"pre-commit", "pre-push", "commit-msg"}
	for _, hook := range expectedHooks {
		hookPath := filepath.Join(tempDir, ".git", "hooks", hook)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Errorf("Hook %s should be created", hook)
		}
	}
}

func TestInstallCommand_Run_Overwrite(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	// Create existing hook
	hooksDir := filepath.Join(tempDir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)
	existingHook := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(existingHook, []byte("#!/bin/sh\necho existing"), 0755)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{"--overwrite"})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with --overwrite, got: %d", exitCode)
	}

	// Verify hook was overwritten
	content, _ := os.ReadFile(existingHook)
	if strings.Contains(string(content), "echo existing") {
		t.Error("Hook should have been overwritten")
	}
}

func TestInstallCommand_Run_SkipExistingWithoutOverwrite(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	// Create existing hook that is one of ours (has our marker)
	hooksDir := filepath.Join(tempDir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)
	existingHook := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(existingHook, []byte("#!/bin/sh\n# Generated by go-pre-commit\nexec pre-commit run"), 0755)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	// Should succeed because it's our hook and can be replaced
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 when our hook exists, got: %d", exitCode)
	}
}

// ====================
// Run Tests - Invalid Inputs
// ====================

func TestInstallCommand_Run_InvalidHookType(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{"--hook-type", "invalid-hook"})

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for invalid hook type, got: %d", exitCode)
	}
}

func TestInstallCommand_Run_CustomConfig(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)

	// Create custom config with different name
	customConfig := filepath.Join(tempDir, "custom-config.yaml")
	content := `repos:
  - repo: local
    hooks:
      - id: test
        name: test
        entry: echo test
        language: system
`
	os.WriteFile(customConfig, []byte(content), 0644)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{"--config", "custom-config.yaml"})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with custom config, got: %d", exitCode)
	}
}

// ====================
// ValidateHookTypes Tests
// ====================

func TestInstallCommand_ValidateHookTypes_AllValid(t *testing.T) {
	cmd := &InstallCommand{}

	validTypes := []string{
		"pre-commit",
		"pre-merge-commit",
		"pre-push",
		"prepare-commit-msg",
		"commit-msg",
		"post-checkout",
		"post-commit",
		"post-merge",
		"post-rewrite",
		"pre-rebase",
		"pre-auto-gc",
	}

	for _, hookType := range validTypes {
		t.Run(hookType, func(t *testing.T) {
			if !cmd.validateHookTypes([]string{hookType}) {
				t.Errorf("Hook type '%s' should be valid", hookType)
			}
		})
	}
}

func TestInstallCommand_ValidateHookTypes_Invalid(t *testing.T) {
	cmd := &InstallCommand{}

	invalidTypes := []string{
		"invalid",
		"pre-commit-invalid",
		"POST-COMMIT",
		"",
	}

	for _, hookType := range invalidTypes {
		t.Run(hookType, func(t *testing.T) {
			if hookType == "" {
				t.Skip("Empty string validation skipped")
			}
			if cmd.validateHookTypes([]string{hookType}) {
				t.Errorf("Hook type '%s' should be invalid", hookType)
			}
		})
	}
}

func TestInstallCommand_ValidateHookTypes_MultipleWithOneInvalid(t *testing.T) {
	cmd := &InstallCommand{}

	// Mix of valid and invalid
	hookTypes := []string{"pre-commit", "invalid-hook", "pre-push"}

	if cmd.validateHookTypes(hookTypes) {
		t.Error("Should reject when any hook type is invalid")
	}
}

// ====================
// GenerateHookScript Tests (hook-impl format - Python parity)
// ====================

func TestInstallCommand_GenerateScript_UsesHookImpl(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-commit", ".pre-commit-config.yaml")

	// Verify hook-impl format (Python parity)
	if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
		t.Error("Script should start with bash shebang (Python parity)")
	}

	if !strings.Contains(script, HookIdentifier) {
		t.Error("Script should contain hook identifier header")
	}

	if !strings.Contains(script, "# ID: "+CurrentHash) {
		t.Error("Script should contain current hash ID")
	}

	if !strings.Contains(script, "ARGS=(hook-impl") {
		t.Error("Script should use hook-impl command (Python parity)")
	}

	if !strings.Contains(script, "--hook-type=pre-commit") {
		t.Error("Script should specify hook type")
	}

	if !strings.Contains(script, "--config=.pre-commit-config.yaml") {
		t.Error("Script should include config path")
	}
}

func TestInstallCommand_GenerateScript_PreCommit(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-commit", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=pre-commit") {
		t.Error("Script should specify hook type")
	}
}

func TestInstallCommand_GenerateScript_PrePush(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-push", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=pre-push") {
		t.Error("Script should specify pre-push hook type")
	}
}

func TestInstallCommand_GenerateScript_CommitMsg(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("commit-msg", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=commit-msg") {
		t.Error("Script should specify commit-msg hook type")
	}
}

func TestInstallCommand_GenerateScript_PrepareCommitMsg(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("prepare-commit-msg", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=prepare-commit-msg") {
		t.Error("Script should specify prepare-commit-msg hook type")
	}
}

func TestInstallCommand_GenerateScript_PostCheckout(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("post-checkout", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=post-checkout") {
		t.Error("Script should specify post-checkout hook type")
	}
}

func TestInstallCommand_GenerateScript_PostMerge(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("post-merge", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=post-merge") {
		t.Error("Script should specify post-merge hook type")
	}
}

func TestInstallCommand_GenerateScript_PostRewrite(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("post-rewrite", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=post-rewrite") {
		t.Error("Script should specify post-rewrite hook type")
	}
}

func TestInstallCommand_GenerateScript_PreRebase(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-rebase", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=pre-rebase") {
		t.Error("Script should specify pre-rebase hook type")
	}
}

func TestInstallCommand_GenerateScript_PostCommit(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("post-commit", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=post-commit") {
		t.Error("Script should specify post-commit hook type")
	}
}

func TestInstallCommand_GenerateScript_PreMergeCommit(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-merge-commit", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=pre-merge-commit") {
		t.Error("Script should specify pre-merge-commit hook type")
	}
}

func TestInstallCommand_GenerateScript_PreAutoGC(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-auto-gc", ".pre-commit-config.yaml")

	if !strings.Contains(script, "--hook-type=pre-auto-gc") {
		t.Error("Script should specify pre-auto-gc hook type")
	}
}

func TestInstallCommand_GenerateScript_AllContainExec(t *testing.T) {
	cmd := &InstallCommand{}

	hookTypes := []string{
		"pre-commit",
		"pre-merge-commit",
		"pre-push",
		"prepare-commit-msg",
		"commit-msg",
		"post-checkout",
		"post-commit",
		"post-merge",
		"post-rewrite",
		"pre-rebase",
		"pre-auto-gc",
	}

	for _, hookType := range hookTypes {
		t.Run(hookType, func(t *testing.T) {
			script := cmd.generateHookScript(hookType, ".pre-commit-config.yaml")
			if !strings.Contains(script, "exec pre-commit") {
				t.Errorf("Script for %s should use exec", hookType)
			}
		})
	}
}

func TestInstallCommand_GenerateScript_TemplateFormat(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-commit", ".pre-commit-config.yaml")

	// Test for Python-like template structure
	tests := []struct {
		name     string
		expected string
	}{
		{"bash_shebang", "#!/usr/bin/env bash"},
		{"identifier_header", HookIdentifier},
		{"hash_identifier", "# ID: " + CurrentHash},
		{"templated_section_start", "# start templated"},
		{"templated_section_end", "# end templated"},
		{"args_array", "ARGS=(hook-impl"},
		{"here_variable", "HERE=\"$(cd \"$(dirname \"$0\")\" && pwd)\""},
		{"hook_dir_arg", "--hook-dir \"$HERE\""},
		{"args_expansion", "${ARGS[@]}"},
		{"command_check", "command -v pre-commit"},
		{"exec_pre_commit", "exec pre-commit"},
		{"error_message", "`pre-commit` not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(script, tt.expected) {
				t.Errorf("Script should contain %q for %s", tt.expected, tt.name)
			}
		})
	}
}

// TestInstallCommand_GenerateScript_PythonTemplateParity verifies the generated script
// matches the structure of Python's hook-tmpl template file
func TestInstallCommand_GenerateScript_PythonTemplateParity(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-commit", ".pre-commit-config.yaml")

	// Python's hook-tmpl has this exact structure:
	// 1. Shebang line
	// 2. Identifier comment (# File generated by pre-commit: ...)
	// 3. Hash ID comment (# ID: <hash>)
	// 4. Templated section with ARGS
	// 5. HERE variable definition
	// 6. ARGS extension with hook-dir
	// 7. Command check and exec

	lines := strings.Split(script, "\n")

	// Line 1: Shebang
	if lines[0] != "#!/usr/bin/env bash" {
		t.Errorf("Line 1 should be bash shebang, got: %s", lines[0])
	}

	// Line 2: Identifier
	if lines[1] != HookIdentifier {
		t.Errorf("Line 2 should be hook identifier, got: %s", lines[1])
	}

	// Line 3: Hash ID
	expectedHashLine := "# ID: " + CurrentHash
	if lines[2] != expectedHashLine {
		t.Errorf("Line 3 should be hash ID (%s), got: %s", expectedHashLine, lines[2])
	}

	// Line 4: Start templated marker
	if lines[3] != "# start templated" {
		t.Errorf("Line 4 should be start templated marker, got: %s", lines[3])
	}

	// Line 5: ARGS array (contains hook-impl and options)
	if !strings.HasPrefix(lines[4], "ARGS=(hook-impl") {
		t.Errorf("Line 5 should start with ARGS=(hook-impl, got: %s", lines[4])
	}

	// Line 6: End templated marker
	if lines[5] != "# end templated" {
		t.Errorf("Line 6 should be end templated marker, got: %s", lines[5])
	}

	// Verify overall structure: templated section should be between markers
	startIdx := strings.Index(script, "# start templated")
	endIdx := strings.Index(script, "# end templated")
	if startIdx >= endIdx {
		t.Error("Templated section markers should be in correct order")
	}

	// Verify HERE is after templated section
	hereIdx := strings.Index(script, "HERE=")
	if hereIdx <= endIdx {
		t.Error("HERE variable should be after templated section")
	}

	// Verify ARGS extension is after HERE
	argsExtIdx := strings.Index(script, "ARGS+=(")
	if argsExtIdx <= hereIdx {
		t.Error("ARGS extension should be after HERE definition")
	}
}

func TestInstallCommand_GenerateScript_ArgsInTemplatedSection(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("commit-msg", "/path/to/config.yaml")

	// Verify the args are within the templated section
	startIdx := strings.Index(script, "# start templated")
	endIdx := strings.Index(script, "# end templated")

	if startIdx == -1 || endIdx == -1 {
		t.Fatal("Script should have templated section markers")
	}

	templatedSection := script[startIdx:endIdx]

	// Check that ARGS array is in the templated section
	if !strings.Contains(templatedSection, "ARGS=(hook-impl") {
		t.Error("ARGS should be in the templated section")
	}

	if !strings.Contains(templatedSection, "--config=/path/to/config.yaml") {
		t.Error("Config path should be in the templated section")
	}

	if !strings.Contains(templatedSection, "--hook-type=commit-msg") {
		t.Error("Hook type should be in the templated section")
	}
}

func TestInstallCommand_GenerateScript_HookDirPassedToHookImpl(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-commit", ".pre-commit-config.yaml")

	// Verify --hook-dir is added to ARGS after the templated section
	// This mirrors Python which uses HERE to pass the hooks directory
	if !strings.Contains(script, "ARGS+=(--hook-dir") {
		t.Error("Script should add --hook-dir to ARGS")
	}

	if !strings.Contains(script, "--hook-dir \"$HERE\"") {
		t.Error("Script should pass $HERE as the hook directory")
	}
}

func TestInstallCommand_GenerateScript_DifferentHookTypes(t *testing.T) {
	cmd := &InstallCommand{}

	hookTypes := []string{
		"pre-commit",
		"pre-push",
		"commit-msg",
		"prepare-commit-msg",
		"post-checkout",
	}

	for _, hookType := range hookTypes {
		t.Run(hookType, func(t *testing.T) {
			script := cmd.generateHookScript(hookType, ".pre-commit-config.yaml")

			expected := fmt.Sprintf("--hook-type=%s", hookType)
			if !strings.Contains(script, expected) {
				t.Errorf("Script should contain %q for hook type %s", expected, hookType)
			}
		})
	}
}

func TestInstallCommand_GenerateScript_CustomConfigPath(t *testing.T) {
	cmd := &InstallCommand{}
	script := cmd.generateHookScript("pre-commit", "/custom/path/config.yaml")

	if !strings.Contains(script, "--config=/custom/path/config.yaml") {
		t.Error("Script should include custom config path")
	}
}

// ====================
// GetHookTypes Tests
// ====================

func TestInstallCommand_GetHookTypes_Default(t *testing.T) {
	cmd := &InstallCommand{}
	opts := &InstallOptions{}

	hookTypes := cmd.getHookTypes(opts)

	if len(hookTypes) != 1 {
		t.Errorf("Expected 1 hook type by default, got %d", len(hookTypes))
	}

	if hookTypes[0] != "pre-commit" {
		t.Errorf("Expected 'pre-commit' by default, got '%s'", hookTypes[0])
	}
}

func TestInstallCommand_GetHookTypes_Specified(t *testing.T) {
	cmd := &InstallCommand{}
	opts := &InstallOptions{
		HookTypes: []string{"pre-push", "commit-msg"},
	}

	hookTypes := cmd.getHookTypes(opts)

	if len(hookTypes) != 2 {
		t.Errorf("Expected 2 hook types, got %d", len(hookTypes))
	}
}

// ====================
// ValidateConfig Tests
// ====================

func TestInstallCommand_ValidateConfig_ConfigExists(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	opts := &InstallOptions{
		Config: ".pre-commit-config.yaml",
	}

	if !cmd.validateConfig(opts) {
		t.Error("Should pass validation when config exists")
	}
}

func TestInstallCommand_ValidateConfig_ConfigMissing(t *testing.T) {
	tempDir := setupInstallTestDir(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	opts := &InstallOptions{
		Config: ".pre-commit-config.yaml",
	}

	if cmd.validateConfig(opts) {
		t.Error("Should fail validation when config missing")
	}
}

func TestInstallCommand_ValidateConfig_AllowMissing(t *testing.T) {
	tempDir := setupInstallTestDir(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	opts := &InstallOptions{
		Config:             ".pre-commit-config.yaml",
		AllowMissingConfig: true,
	}

	if !cmd.validateConfig(opts) {
		t.Error("Should pass validation with --allow-missing-config")
	}
}

// ====================
// Integration Tests
// ====================

func TestInstallCommand_Integration_FullFlow(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify hook file
	hookPath := filepath.Join(tempDir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if os.IsNotExist(err) {
		t.Fatal("Hook file should exist")
	}

	// Verify executable
	if info.Mode()&0111 == 0 {
		t.Error("Hook should be executable")
	}

	// Verify content uses hook-impl (Python parity)
	content, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(content), "hook-impl") {
		t.Error("Hook should use hook-impl command (Python parity)")
	}
	if !strings.Contains(string(content), HookIdentifier) {
		t.Error("Hook should contain hook identifier")
	}
}

func TestInstallCommand_Integration_AllHookTypes(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	allHookTypes := []string{
		"pre-commit",
		"pre-merge-commit",
		"pre-push",
		"prepare-commit-msg",
		"commit-msg",
		"post-checkout",
		"post-commit",
		"post-merge",
		"post-rewrite",
		"pre-rebase",
		"pre-auto-gc",
	}

	// Build args with all hook types
	args := []string{"--overwrite"}
	for _, ht := range allHookTypes {
		args = append(args, "-t", ht)
	}

	cmd := &InstallCommand{}
	exitCode := cmd.Run(args)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify all hooks created
	for _, ht := range allHookTypes {
		hookPath := filepath.Join(tempDir, ".git", "hooks", ht)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Errorf("Hook %s should be created", ht)
		}
	}
}

func TestInstallCommand_Integration_HookIsExecutable(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	hookPath := filepath.Join(tempDir, ".git", "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("Failed to stat hook: %v", err)
	}

	// Check executable bits
	mode := info.Mode()
	if mode&0100 == 0 {
		t.Error("Hook should be executable by owner")
	}
}

// ====================
// ParseArguments Tests
// ====================

func TestInstallCommand_ParseArguments_DefaultValues(t *testing.T) {
	cmd := &InstallCommand{}
	opts, err := cmd.parseArguments([]string{})

	if err != nil {
		t.Fatalf("Unexpected parse error: %v", err)
	}

	if opts.Config != ".pre-commit-config.yaml" {
		t.Errorf("Expected default config '.pre-commit-config.yaml', got '%s'", opts.Config)
	}

	if opts.Color != "auto" {
		t.Errorf("Expected default color 'auto', got '%s'", opts.Color)
	}

	if opts.Overwrite {
		t.Error("Overwrite should be false by default")
	}

	if opts.InstallHooks {
		t.Error("InstallHooks should be false by default")
	}

	if opts.AllowMissingConfig {
		t.Error("AllowMissingConfig should be false by default")
	}
}

func TestInstallCommand_ParseArguments_ShortFlags(t *testing.T) {
	cmd := &InstallCommand{}
	opts, err := cmd.parseArguments([]string{"-c", "myconfig.yaml", "-f", "-t", "pre-push"})

	if err != nil {
		t.Fatalf("Unexpected parse error: %v", err)
	}

	if opts.Config != "myconfig.yaml" {
		t.Errorf("Expected config 'myconfig.yaml', got '%s'", opts.Config)
	}

	if !opts.Overwrite {
		t.Error("Overwrite should be true with -f flag")
	}

	if len(opts.HookTypes) == 0 || opts.HookTypes[0] != "pre-push" {
		t.Error("HookTypes should contain 'pre-push'")
	}
}

func TestInstallCommand_ParseArguments_LongFlags(t *testing.T) {
	cmd := &InstallCommand{}
	opts, err := cmd.parseArguments([]string{
		"--config", "custom.yaml",
		"--overwrite",
		"--hook-type", "commit-msg",
		"--install-hooks",
		"--allow-missing-config",
	})

	if err != nil {
		t.Fatalf("Unexpected parse error: %v", err)
	}

	if opts.Config != "custom.yaml" {
		t.Errorf("Expected config 'custom.yaml', got '%s'", opts.Config)
	}

	if !opts.Overwrite {
		t.Error("Overwrite should be true")
	}

	if !opts.InstallHooks {
		t.Error("InstallHooks should be true")
	}

	if !opts.AllowMissingConfig {
		t.Error("AllowMissingConfig should be true")
	}
}

func TestInstallCommand_ParseArguments_MultipleHookTypes(t *testing.T) {
	cmd := &InstallCommand{}
	opts, err := cmd.parseArguments([]string{
		"-t", "pre-commit",
		"-t", "pre-push",
		"-t", "commit-msg",
	})

	if err != nil {
		t.Fatalf("Unexpected parse error: %v", err)
	}

	if len(opts.HookTypes) != 3 {
		t.Errorf("Expected 3 hook types, got %d", len(opts.HookTypes))
	}

	expected := []string{"pre-commit", "pre-push", "commit-msg"}
	for i, ht := range expected {
		if opts.HookTypes[i] != ht {
			t.Errorf("Expected hook type '%s' at index %d, got '%s'", ht, i, opts.HookTypes[i])
		}
	}
}

// ====================
// core.hooksPath Tests
// ====================

func TestInstallCommand_Run_CoreHooksPathSet(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	// Set core.hooksPath in git config
	repo, _ := git.PlainOpen(tempDir)
	cfg, _ := repo.Config()
	cfg.Raw.Section("core").SetOption("hooksPath", "/custom/hooks")
	repo.SetConfig(cfg)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	// Should fail because core.hooksPath is set
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 when core.hooksPath is set, got: %d", exitCode)
	}
}

func TestInstallCommand_Run_CoreHooksPathNotSet(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	// Should succeed because core.hooksPath is not set
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}
}

// ====================
// default_install_hook_types Tests
// ====================

func TestInstallCommand_Run_DefaultInstallHookTypesFromConfig(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)

	// Create config with default_install_hook_types
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	content := `default_install_hook_types:
  - pre-commit
  - pre-push
repos:
  - repo: local
    hooks:
      - id: test
        name: test
        entry: echo test
        language: system
`
	os.WriteFile(configPath, []byte(content), 0644)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify both hooks were created from config defaults
	for _, hook := range []string{"pre-commit", "pre-push"} {
		hookPath := filepath.Join(tempDir, ".git", "hooks", hook)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Errorf("Hook %s should be created from default_install_hook_types", hook)
		}
	}
}

func TestInstallCommand_Run_CLIOverridesDefaultHookTypes(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)

	// Create config with default_install_hook_types
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	content := `default_install_hook_types:
  - pre-commit
  - pre-push
repos:
  - repo: local
    hooks:
      - id: test
        name: test
        entry: echo test
        language: system
`
	os.WriteFile(configPath, []byte(content), 0644)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	// Explicitly specify only commit-msg
	exitCode := cmd.Run([]string{"-t", "commit-msg"})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify only commit-msg was created (CLI overrides config)
	commitMsgPath := filepath.Join(tempDir, ".git", "hooks", "commit-msg")
	if _, err := os.Stat(commitMsgPath); os.IsNotExist(err) {
		t.Error("commit-msg hook should be created")
	}

	// Verify pre-push was NOT created
	prePushPath := filepath.Join(tempDir, ".git", "hooks", "pre-push")
	if _, err := os.Stat(prePushPath); !os.IsNotExist(err) {
		t.Error("pre-push hook should NOT be created when CLI specifies hook types")
	}
}

// ====================
// Legacy Hook Tests
// ====================

func TestInstallCommand_Run_LegacyHookBackup(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	// Create existing non-pre-commit hook
	hooksDir := filepath.Join(tempDir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)
	existingHook := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(existingHook, []byte("#!/bin/sh\necho my-custom-hook"), 0755)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify legacy backup was created
	legacyPath := filepath.Join(hooksDir, "pre-commit.legacy")
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		t.Error("Legacy hook backup should be created")
	}

	// Verify legacy content is preserved
	legacyContent, _ := os.ReadFile(legacyPath)
	if !strings.Contains(string(legacyContent), "my-custom-hook") {
		t.Error("Legacy hook should contain original content")
	}

	// Verify new hook was installed (check for HookIdentifier used in hook-impl template)
	newContent, _ := os.ReadFile(existingHook)
	if !strings.Contains(string(newContent), HookIdentifier) {
		t.Error("New hook should be installed with hook identifier")
	}
}

func TestInstallCommand_Run_NoLegacyForOurHook(t *testing.T) {
	tempDir := setupInstallTestDir(t)
	initGitRepoForInstall(t, tempDir)
	createPreCommitConfigForInstall(t, tempDir)

	// Create existing hook that IS ours
	hooksDir := filepath.Join(tempDir, ".git", "hooks")
	os.MkdirAll(hooksDir, 0755)
	existingHook := filepath.Join(hooksDir, "pre-commit")
	os.WriteFile(existingHook, []byte("#!/bin/sh\n# Generated by go-pre-commit\nexec pre-commit run"), 0755)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify NO legacy backup was created (it's our hook)
	legacyPath := filepath.Join(hooksDir, "pre-commit.legacy")
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("Legacy backup should NOT be created for our own hook")
	}
}

// ====================
// GetHookTypes Tests (Extended)
// ====================

func TestInstallCommand_GetHookTypes_UsesConfigDefault(t *testing.T) {
	tempDir := setupInstallTestDir(t)

	// Create config with default_install_hook_types
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	content := `default_install_hook_types:
  - pre-push
  - commit-msg
repos: []
`
	os.WriteFile(configPath, []byte(content), 0644)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	opts := &InstallOptions{
		Config:    ".pre-commit-config.yaml",
		HookTypes: []string{}, // No CLI hook types
	}

	hookTypes := cmd.getHookTypes(opts)

	if len(hookTypes) != 2 {
		t.Errorf("Expected 2 hook types from config, got %d", len(hookTypes))
	}

	if hookTypes[0] != "pre-push" || hookTypes[1] != "commit-msg" {
		t.Errorf("Expected [pre-push, commit-msg], got %v", hookTypes)
	}
}

func TestInstallCommand_GetHookTypes_FallbackToPreCommit(t *testing.T) {
	tempDir := setupInstallTestDir(t)

	// Create config WITHOUT default_install_hook_types
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	content := `repos:
  - repo: local
    hooks:
      - id: test
        name: test
        entry: echo test
        language: system
`
	os.WriteFile(configPath, []byte(content), 0644)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	opts := &InstallOptions{
		Config:    ".pre-commit-config.yaml",
		HookTypes: []string{}, // No CLI hook types
	}

	hookTypes := cmd.getHookTypes(opts)

	if len(hookTypes) != 1 || hookTypes[0] != "pre-commit" {
		t.Errorf("Expected [pre-commit] fallback, got %v", hookTypes)
	}
}

func TestInstallCommand_GetHookTypes_CLIOverridesConfig(t *testing.T) {
	tempDir := setupInstallTestDir(t)

	// Create config with default_install_hook_types
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	content := `default_install_hook_types:
  - pre-push
repos: []
`
	os.WriteFile(configPath, []byte(content), 0644)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	opts := &InstallOptions{
		Config:    ".pre-commit-config.yaml",
		HookTypes: []string{"commit-msg"}, // CLI specifies different hook type
	}

	hookTypes := cmd.getHookTypes(opts)

	if len(hookTypes) != 1 || hookTypes[0] != "commit-msg" {
		t.Errorf("Expected [commit-msg] from CLI, got %v", hookTypes)
	}
}

func TestInstallCommand_GetHookTypes_MissingConfigFallback(t *testing.T) {
	tempDir := setupInstallTestDir(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &InstallCommand{}
	opts := &InstallOptions{
		Config:    ".pre-commit-config.yaml", // Does not exist
		HookTypes: []string{},
	}

	hookTypes := cmd.getHookTypes(opts)

	// Should fallback to pre-commit when config doesn't exist
	if len(hookTypes) != 1 || hookTypes[0] != "pre-commit" {
		t.Errorf("Expected [pre-commit] fallback when config missing, got %v", hookTypes)
	}
}
