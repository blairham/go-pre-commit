package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookInstaller_Install(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	opts := &HookInstallOptions{
		Config:              configFile,
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           true,
		SkipOnMissingConfig: true,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify hook was created
	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("Hook script should be created")
	}
}

func TestHookInstaller_Install_MultipleHookTypes(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	opts := &HookInstallOptions{
		Config:              configFile,
		HookTypes:           []string{"pre-commit", "pre-push", "commit-msg"},
		GitDir:              templateDir,
		Overwrite:           true,
		SkipOnMissingConfig: false,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify all hooks were created
	for _, hookType := range opts.HookTypes {
		hookPath := filepath.Join(templateDir, "hooks", hookType)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Errorf("Hook script %s should be created", hookType)
		}
	}
}

func TestHookInstaller_Install_MissingConfig(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	opts := &HookInstallOptions{
		Config:              "/nonexistent/config.yaml",
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           true,
		SkipOnMissingConfig: false,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err == nil {
		t.Error("Install should fail with missing config")
	}

	if !strings.Contains(err.Error(), "config file not found") {
		t.Errorf("Error should mention config file not found, got: %v", err)
	}
}

func TestHookInstaller_Install_AllowMissingConfig(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	opts := &HookInstallOptions{
		Config:              "/nonexistent/config.yaml",
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           true,
		SkipOnMissingConfig: false,
		AllowMissingConfig:  true, // Allow missing config
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install should succeed with AllowMissingConfig=true: %v", err)
	}

	// Verify hook was created
	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("Hook script should be created")
	}
}

func TestHookInstaller_Install_NoOverwrite(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Pre-create the hook with custom content (NOT our script)
	hooksDir := filepath.Join(templateDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	legacyPath := hookPath + ".legacy"
	originalContent := "#!/bin/sh\necho 'original'\n"
	os.WriteFile(hookPath, []byte(originalContent), 0o755)

	opts := &HookInstallOptions{
		Config:              configFile,
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           false, // Don't overwrite - but legacy should still be created
		SkipOnMissingConfig: false,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// With legacy hook handling:
	// - Original hook (not our script) should be moved to .legacy
	// - New hook should be installed
	// - Migration mode message shown (Overwrite=false)

	// Verify legacy file was created with original content
	legacyContent, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("Legacy file should exist: %v", err)
	}
	if string(legacyContent) != originalContent {
		t.Error("Legacy file should contain original hook content")
	}

	// Verify new hook was installed
	newContent, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(newContent), "hook-impl") {
		t.Error("New pre-commit hook should be installed")
	}
}

func TestHookInstaller_Install_Overwrite(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Pre-create the hook with custom content (NOT our script)
	hooksDir := filepath.Join(templateDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	legacyPath := hookPath + ".legacy"
	originalContent := "#!/bin/sh\necho 'original'\n"
	os.WriteFile(hookPath, []byte(originalContent), 0o755)

	opts := &HookInstallOptions{
		Config:              configFile,
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           true, // Overwrite - should delete legacy file
		SkipOnMissingConfig: false,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify new hook was installed
	content, _ := os.ReadFile(hookPath)
	if string(content) == originalContent {
		t.Error("Hook should be overwritten when Overwrite=true")
	}

	if !strings.Contains(string(content), "hook-impl") {
		t.Error("New hook should contain hook-impl")
	}

	// Verify legacy file was deleted (Overwrite=true)
	if _, err := os.Stat(legacyPath); err == nil {
		t.Error("Legacy file should be deleted when Overwrite=true")
	}
}

func TestHookInstaller_GenerateHookScript_WithSkipOnMissingConfig(t *testing.T) {
	installer := NewHookInstaller()

	opts := &HookInstallOptions{
		Config:              ".pre-commit-config.yaml",
		SkipOnMissingConfig: true,
	}

	script := installer.generateHookScript("pre-commit", opts)

	if !strings.Contains(script, "--skip-on-missing-config") {
		t.Error("Script should contain --skip-on-missing-config when SkipOnMissingConfig=true")
	}

	if !strings.Contains(script, "--config=.pre-commit-config.yaml") {
		t.Error("Script should contain config path")
	}

	if !strings.Contains(script, "--hook-type=pre-commit") {
		t.Error("Script should contain hook type")
	}
}

func TestHookInstaller_GenerateHookScript_WithoutSkipOnMissingConfig(t *testing.T) {
	installer := NewHookInstaller()

	opts := &HookInstallOptions{
		Config:              ".pre-commit-config.yaml",
		SkipOnMissingConfig: false,
	}

	script := installer.generateHookScript("pre-commit", opts)

	if strings.Contains(script, "--skip-on-missing-config") {
		t.Error("Script should not contain --skip-on-missing-config when SkipOnMissingConfig=false")
	}
}

// Template-based script tests (Python parity)
func TestHookInstaller_GenerateHookScript_TemplateFormat(t *testing.T) {
	installer := NewHookInstaller()

	opts := &HookInstallOptions{
		Config:              ".pre-commit-config.yaml",
		SkipOnMissingConfig: false,
	}

	script := installer.generateHookScript("pre-commit", opts)

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

func TestHookInstaller_GenerateHookScript_ArgsInTemplatedSection(t *testing.T) {
	installer := NewHookInstaller()

	opts := &HookInstallOptions{
		Config:              "/path/to/config.yaml",
		HookTypes:           []string{"commit-msg"},
		SkipOnMissingConfig: true,
	}

	script := installer.generateHookScript("commit-msg", opts)

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

	if !strings.Contains(templatedSection, "--skip-on-missing-config") {
		t.Error("Skip-on-missing-config should be in the templated section when enabled")
	}
}

func TestHookInstaller_GenerateHookScript_HookDirPassedToHookImpl(t *testing.T) {
	installer := NewHookInstaller()

	opts := &HookInstallOptions{
		Config:              ".pre-commit-config.yaml",
		SkipOnMissingConfig: false,
	}

	script := installer.generateHookScript("pre-commit", opts)

	// Verify --hook-dir is added to ARGS after the templated section
	// This mirrors Python which uses HERE to pass the hooks directory
	if !strings.Contains(script, "ARGS+=(--hook-dir") {
		t.Error("Script should add --hook-dir to ARGS")
	}

	if !strings.Contains(script, "--hook-dir \"$HERE\"") {
		t.Error("Script should pass $HERE as the hook directory")
	}
}

func TestHookInstaller_GenerateHookScript_DifferentHookTypes(t *testing.T) {
	installer := NewHookInstaller()

	hookTypes := []string{
		"pre-commit",
		"pre-push",
		"commit-msg",
		"prepare-commit-msg",
		"post-checkout",
	}

	for _, hookType := range hookTypes {
		t.Run(hookType, func(t *testing.T) {
			opts := &HookInstallOptions{
				Config:              ".pre-commit-config.yaml",
				SkipOnMissingConfig: false,
			}

			script := installer.generateHookScript(hookType, opts)

			expected := fmt.Sprintf("--hook-type=%s", hookType)
			if !strings.Contains(script, expected) {
				t.Errorf("Script should contain %q for hook type %s", expected, hookType)
			}
		})
	}
}

func TestHookInstaller_GetHooksDir(t *testing.T) {
	installer := NewHookInstaller()

	tests := []struct {
		name     string
		gitDir   string
		expected string
	}{
		{
			name:     "empty_gitDir_defaults_to_git_hooks",
			gitDir:   "",
			expected: filepath.Join(".git", "hooks"),
		},
		{
			name:     "template_directory",
			gitDir:   "/path/to/template",
			expected: filepath.Join("/path/to/template", "hooks"),
		},
		{
			name:     "relative_path",
			gitDir:   "my-template",
			expected: filepath.Join("my-template", "hooks"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := installer.getHooksDir(tt.gitDir)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestValidHookTypes(t *testing.T) {
	hookTypes := ValidHookTypes()

	expectedTypes := []string{
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

	if len(hookTypes) != len(expectedTypes) {
		t.Errorf("Expected %d hook types, got %d", len(expectedTypes), len(hookTypes))
	}

	for _, expected := range expectedTypes {
		found := false
		for _, actual := range hookTypes {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected hook type %s not found", expected)
		}
	}
}

func TestIsValidHookType(t *testing.T) {
	tests := []struct {
		hookType string
		expected bool
	}{
		{"pre-commit", true},
		{"pre-push", true},
		{"commit-msg", true},
		{"invalid", false},
		{"", false},
		{"pre-comit", false}, // typo
	}

	for _, tt := range tests {
		t.Run(tt.hookType, func(t *testing.T) {
			result := IsValidHookType(tt.hookType)
			if result != tt.expected {
				t.Errorf("IsValidHookType(%q) = %v, expected %v", tt.hookType, result, tt.expected)
			}
		})
	}
}

func TestHookInstaller_HookIsExecutable(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	opts := &HookInstallOptions{
		Config:              configFile,
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           true,
		SkipOnMissingConfig: false,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("Failed to stat hook: %v", err)
	}

	// Check executable bit
	mode := info.Mode()
	if mode&0o100 == 0 {
		t.Error("Hook should be executable by owner")
	}
}

// Tests for legacy hook handling (Python parity)

func TestHookInstaller_LegacyHook_MovesExistingHook(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Pre-create an existing hook (NOT our script)
	hooksDir := filepath.Join(templateDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	legacyPath := hookPath + ".legacy"
	existingContent := "#!/bin/sh\necho 'user hook'\nexit 0\n"
	os.WriteFile(hookPath, []byte(existingContent), 0o755)

	opts := &HookInstallOptions{
		Config:              configFile,
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           false,
		SkipOnMissingConfig: false,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify existing hook was moved to .legacy
	legacyContent, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("Legacy hook should exist: %v", err)
	}
	if string(legacyContent) != existingContent {
		t.Error("Legacy hook should contain original content")
	}

	// Verify new hook was installed
	newContent, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(newContent), HookIdentifier) {
		t.Error("New hook should be installed with our identifier")
	}
}

func TestHookInstaller_LegacyHook_DoesNotMoveOurScript(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Pre-create an existing hook that IS our script
	hooksDir := filepath.Join(templateDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	legacyPath := hookPath + ".legacy"
	// This contains our identifier so it's recognized as our script
	ourScript := "#!/usr/bin/env bash\n# File generated by pre-commit: https://pre-commit.com\n# ID: go-pre-commit-v1\necho 'old version'\n"
	os.WriteFile(hookPath, []byte(ourScript), 0o755)

	opts := &HookInstallOptions{
		Config:              configFile,
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           false,
		SkipOnMissingConfig: false,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify NO legacy file was created (it's our script)
	if _, err := os.Stat(legacyPath); err == nil {
		t.Error("Legacy file should NOT be created for our own script")
	}

	// Verify hook was updated (overwritten since it's ours)
	newContent, _ := os.ReadFile(hookPath)
	if !strings.Contains(string(newContent), "hook-impl") {
		t.Error("Our script should be updated to new version")
	}
}

func TestHookInstaller_LegacyHook_OverwriteDeletesLegacy(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	// Create config file
	configFile := filepath.Join(tempDir, ".pre-commit-config.yaml")
	os.WriteFile(configFile, []byte("repos: []\n"), 0o644)

	// Pre-create hooks directory with both hook and legacy
	hooksDir := filepath.Join(templateDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)
	hookPath := filepath.Join(hooksDir, "pre-commit")
	legacyPath := hookPath + ".legacy"

	// Create existing hook
	os.WriteFile(hookPath, []byte("#!/bin/sh\necho 'current'\n"), 0o755)
	// Create existing legacy file
	os.WriteFile(legacyPath, []byte("#!/bin/sh\necho 'legacy'\n"), 0o755)

	opts := &HookInstallOptions{
		Config:              configFile,
		HookTypes:           []string{"pre-commit"},
		GitDir:              templateDir,
		Overwrite:           true, // Should delete legacy
		SkipOnMissingConfig: false,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify legacy file was deleted
	if _, err := os.Stat(legacyPath); err == nil {
		t.Error("Legacy file should be deleted when Overwrite=true")
	}
}

func TestHookInstaller_IsOurScript(t *testing.T) {
	installer := NewHookInstaller()
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "current_hash",
			content:  "#!/usr/bin/env bash\n# File generated by pre-commit: https://pre-commit.com\n# ID: 138fd403232d2ddd5efb44317e38bf03\nexec pre-commit hook-impl\n",
			expected: true,
		},
		{
			name:     "prior_hash_1",
			content:  "#!/usr/bin/env bash\n# File generated by pre-commit: https://pre-commit.com\n# ID: 4d9958c90bc262f47553e2c073f14cfe\nexec pre-commit hook-impl\n",
			expected: true,
		},
		{
			name:     "prior_hash_2",
			content:  "#!/usr/bin/env bash\n# File generated by pre-commit: https://pre-commit.com\n# ID: d8ee923c46731b42cd95cc869add4062\nexec pre-commit hook-impl\n",
			expected: true,
		},
		{
			name:     "prior_hash_3",
			content:  "#!/usr/bin/env bash\n# File generated by pre-commit: https://pre-commit.com\n# ID: 49fd668cb42069aa1b6048464be5d395\nexec pre-commit hook-impl\n",
			expected: true,
		},
		{
			name:     "prior_hash_4",
			content:  "#!/usr/bin/env bash\n# File generated by pre-commit: https://pre-commit.com\n# ID: 79f09a650522a87b0da915d0d983b2de\nexec pre-commit hook-impl\n",
			expected: true,
		},
		{
			name:     "prior_hash_5",
			content:  "#!/usr/bin/env bash\n# File generated by pre-commit: https://pre-commit.com\n# ID: e358c9dae00eac5d06b38dfdb1e33a8c\nexec pre-commit hook-impl\n",
			expected: true,
		},
		{
			name:     "go_pre_commit_v1",
			content:  "#!/usr/bin/env bash\n# File generated by pre-commit: https://pre-commit.com\n# ID: go-pre-commit-v1\nexec pre-commit hook-impl\n",
			expected: true,
		},
		{
			name:     "user_script",
			content:  "#!/bin/sh\necho 'user hook'\n",
			expected: false,
		},
		{
			name:     "husky_script",
			content:  "#!/bin/sh\n. \"$(dirname \"$0\")/_/husky.sh\"\nnpx lint-staged\n",
			expected: false,
		},
		{
			name:     "empty_file",
			content:  "",
			expected: false,
		},
		{
			name:     "hash_in_middle",
			content:  "#!/bin/bash\nsome stuff\n# ID: 138fd403232d2ddd5efb44317e38bf03\nmore stuff\n",
			expected: true,
		},
		{
			name:     "wrong_hash",
			content:  "#!/bin/bash\n# ID: 00000000000000000000000000000000\nstuff\n",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hookPath := filepath.Join(tempDir, tt.name)
			os.WriteFile(hookPath, []byte(tt.content), 0o755)

			result := installer.isOurScript(hookPath)
			if result != tt.expected {
				t.Errorf("isOurScript() = %v, expected %v for %q", result, tt.expected, tt.name)
			}
		})
	}
}

func TestHookInstaller_IsOurScript_NonexistentFile(t *testing.T) {
	installer := NewHookInstaller()

	result := installer.isOurScript("/nonexistent/path/hook")
	if result {
		t.Error("isOurScript should return false for nonexistent file")
	}
}

// Tests for getHookTypes - matching Python's _hook_types() behavior

func TestHookInstaller_GetHookTypes_ExplicitHookTypes(t *testing.T) {
	installer := NewHookInstaller()

	// When hook types are explicitly provided, use them (regardless of config)
	provided := []string{"pre-push", "commit-msg"}
	result := installer.getHookTypes("/nonexistent/config.yaml", provided)

	if len(result) != 2 {
		t.Fatalf("Expected 2 hook types, got %d", len(result))
	}
	if result[0] != "pre-push" || result[1] != "commit-msg" {
		t.Errorf("Expected [pre-push, commit-msg], got %v", result)
	}
}

func TestHookInstaller_GetHookTypes_DefaultFromConfig(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")

	// Create config with default_install_hook_types
	configContent := `repos: []
default_install_hook_types:
  - pre-commit
  - pre-push
  - commit-msg
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// When no hook types provided, should use config's default_install_hook_types
	result := installer.getHookTypes(configPath, nil)

	if len(result) != 3 {
		t.Fatalf("Expected 3 hook types from config, got %d: %v", len(result), result)
	}
	if result[0] != "pre-commit" || result[1] != "pre-push" || result[2] != "commit-msg" {
		t.Errorf("Expected [pre-commit, pre-push, commit-msg], got %v", result)
	}
}

func TestHookInstaller_GetHookTypes_FallbackToPreCommit_NoConfig(t *testing.T) {
	installer := NewHookInstaller()

	// When config doesn't exist, should fallback to ["pre-commit"]
	result := installer.getHookTypes("/nonexistent/config.yaml", nil)

	if len(result) != 1 {
		t.Fatalf("Expected 1 hook type, got %d: %v", len(result), result)
	}
	if result[0] != "pre-commit" {
		t.Errorf("Expected [pre-commit], got %v", result)
	}
}

func TestHookInstaller_GetHookTypes_FallbackToPreCommit_EmptyDefaultInConfig(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")

	// Create config without default_install_hook_types (empty/not set)
	configContent := `repos: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// When config exists but default_install_hook_types is empty, fallback to ["pre-commit"]
	result := installer.getHookTypes(configPath, nil)

	if len(result) != 1 {
		t.Fatalf("Expected 1 hook type, got %d: %v", len(result), result)
	}
	if result[0] != "pre-commit" {
		t.Errorf("Expected [pre-commit], got %v", result)
	}
}

func TestHookInstaller_GetHookTypes_FallbackToPreCommit_InvalidConfig(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")

	// Create invalid config (not valid YAML or missing required fields)
	configContent := `not: valid: yaml: [
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// When config is invalid, should fallback to ["pre-commit"]
	result := installer.getHookTypes(configPath, nil)

	if len(result) != 1 {
		t.Fatalf("Expected 1 hook type, got %d: %v", len(result), result)
	}
	if result[0] != "pre-commit" {
		t.Errorf("Expected [pre-commit], got %v", result)
	}
}

func TestHookInstaller_GetHookTypes_EmptyProvidedUsesConfig(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")

	// Create config with default_install_hook_types
	configContent := `repos: []
default_install_hook_types:
  - pre-push
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	// Empty slice (not nil) should still use config
	result := installer.getHookTypes(configPath, []string{})

	if len(result) != 1 {
		t.Fatalf("Expected 1 hook type from config, got %d: %v", len(result), result)
	}
	if result[0] != "pre-push" {
		t.Errorf("Expected [pre-push], got %v", result)
	}
}

func TestHookInstaller_Install_DefaultHookTypesFromConfig(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")

	// Create config with default_install_hook_types
	configContent := `repos: []
default_install_hook_types:
  - pre-commit
  - pre-push
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	opts := &HookInstallOptions{
		Config:              configPath,
		HookTypes:           nil, // Not provided - should use config defaults
		GitDir:              templateDir,
		Overwrite:           true,
		SkipOnMissingConfig: true,
		AllowMissingConfig:  false,
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify both hooks were created (from config's default_install_hook_types)
	for _, hookType := range []string{"pre-commit", "pre-push"} {
		hookPath := filepath.Join(templateDir, "hooks", hookType)
		if _, err := os.Stat(hookPath); os.IsNotExist(err) {
			t.Errorf("Hook script %s should be created from config default", hookType)
		}
	}
}

func TestHookInstaller_Install_FallsBackToPreCommit(t *testing.T) {
	installer := NewHookInstaller()

	tempDir := t.TempDir()
	templateDir := filepath.Join(tempDir, "git-template")

	opts := &HookInstallOptions{
		Config:              "/nonexistent/.pre-commit-config.yaml",
		HookTypes:           nil, // Not provided
		GitDir:              templateDir,
		Overwrite:           true,
		SkipOnMissingConfig: true,
		AllowMissingConfig:  true, // Allow missing config
	}

	err := installer.Install(opts)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	// Verify pre-commit hook was created (fallback default)
	hookPath := filepath.Join(templateDir, "hooks", "pre-commit")
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		t.Error("pre-commit hook script should be created as fallback default")
	}

	// Verify no other hooks were created
	hooksDir := filepath.Join(templateDir, "hooks")
	entries, _ := os.ReadDir(hooksDir)
	if len(entries) != 1 {
		t.Errorf("Expected only 1 hook (pre-commit), got %d", len(entries))
	}
}
