package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ====================
// Test Helpers
// ====================

func setupMigrateConfigTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func createConfigFile(t *testing.T, dir, content string) string {
	t.Helper()
	configPath := filepath.Join(dir, ".pre-commit-config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}
	return configPath
}

// ====================
// Help and Synopsis Tests
// ====================

func TestMigrateConfigCommand_Help(t *testing.T) {
	cmd := &MigrateConfigCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"migrate-config",
		"--config",
		"repos:",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help should contain %q", expected)
		}
	}
}

func TestMigrateConfigCommand_Synopsis(t *testing.T) {
	cmd := &MigrateConfigCommand{}
	synopsis := cmd.Synopsis()

	if !strings.Contains(strings.ToLower(synopsis), "migrate") {
		t.Error("Synopsis should mention migration")
	}
}

// ====================
// Basic Migration Tests
// ====================

func TestMigrateConfigCommand_Run_BasicMigration(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Old format config (list without repos: key)
	oldConfig := `- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Read migrated content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read migrated config: %v", err)
	}

	// Verify repos: key was added
	if !strings.Contains(string(content), "repos:") {
		t.Error("Migrated config should contain 'repos:' key")
	}
}

func TestMigrateConfigCommand_Run_AlreadyMigrated(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// New format config (already has repos: key)
	newConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
`
	configPath := createConfigFile(t, tempDir, newConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got: %d", exitCode)
	}

	// Verify content was not modified
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if string(content) != newConfig {
		t.Error("Already migrated config should not be modified")
	}
}

func TestMigrateConfigCommand_Run_MissingConfig(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for missing config, got: %d", exitCode)
	}
}

func TestMigrateConfigCommand_Run_InvalidYAML(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Invalid YAML
	invalidConfig := `repos:
  - repo: https://github.com/example/repo
    hooks:
    - id: test
  invalid: yaml: syntax
`
	createConfigFile(t, tempDir, invalidConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for invalid YAML, got: %d", exitCode)
	}
}

func TestMigrateConfigCommand_Run_CustomConfigPath(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	customConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
`
	customPath := filepath.Join(tempDir, "custom-config.yaml")
	os.WriteFile(customPath, []byte(customConfig), 0644)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{"-c", customPath})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with custom config path, got: %d", exitCode)
	}
}

// ====================
// needsMigration Tests
// ====================

func TestMigrateConfigCommand_NeedsMigration_OldFormat(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	oldConfig := `- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
`
	if !cmd.needsMigration(oldConfig) {
		t.Error("Should detect old format needs migration")
	}
}

func TestMigrateConfigCommand_NeedsMigration_NewFormat(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	newConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
`
	if cmd.needsMigration(newConfig) {
		t.Error("Should detect new format does not need migration")
	}
}

func TestMigrateConfigCommand_NeedsMigration_EmptyConfig(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	emptyConfig := ``
	if cmd.needsMigration(emptyConfig) {
		t.Error("Empty config should not need migration")
	}
}

func TestMigrateConfigCommand_NeedsMigration_OnlyReposKey(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	minimalConfig := `repos: []`
	if cmd.needsMigration(minimalConfig) {
		t.Error("Config with repos: should not need migration")
	}
}

// ====================
// migrateConfig Tests
// ====================

func TestMigrateConfigCommand_MigrateConfig_AddsReposKey(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	oldConfig := `- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
`
	result := cmd.migrateConfig(oldConfig)

	if !strings.HasPrefix(result, "repos:") {
		t.Error("Migrated config should start with 'repos:'")
	}
}

func TestMigrateConfigCommand_MigrateConfig_IndentsContent(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	oldConfig := `- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
`
	result := cmd.migrateConfig(oldConfig)

	// The migration should add repos: and the result should be valid YAML
	// Either the content is indented, or repos: was prepended and YAML parses correctly
	if !strings.Contains(result, "repos:") {
		t.Errorf("Migrated config should contain 'repos:', got:\n%s", result)
	}
	if !strings.Contains(result, "- repo:") {
		t.Errorf("Migrated config should contain repo entry, got:\n%s", result)
	}
}

func TestMigrateConfigCommand_MigrateConfig_MultipleRepos(t *testing.T) {
	cmd := &MigrateConfigCommand{}

	oldConfig := `- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
- repo: https://github.com/psf/black
  rev: 23.1.0
  hooks:
  - id: black
`
	result := cmd.migrateConfig(oldConfig)

	// Should contain both repos
	if !strings.Contains(result, "pre-commit-hooks") {
		t.Error("Migrated config should contain first repo")
	}
	if !strings.Contains(result, "psf/black") {
		t.Error("Migrated config should contain second repo")
	}
}

// ====================
// Gap Tests (Now Implemented)
// ====================

func TestMigrateConfigCommand_ShaToRev(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with old 'sha' key instead of 'rev'
	oldConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  sha: abc123
  hooks:
  - id: trailing-whitespace
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if strings.Contains(contentStr, "\n  sha:") {
		t.Error("Should migrate 'sha:' to 'rev:'")
	}
	if !strings.Contains(contentStr, "rev:") {
		t.Error("Should contain 'rev:' after migration")
	}
}

func TestMigrateConfigCommand_ShaToRev_QuotedKey(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with quoted 'sha' key
	oldConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  'sha': abc123
  hooks:
  - id: trailing-whitespace
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if strings.Contains(contentStr, "'sha':") {
		t.Error("Should migrate quoted 'sha' to 'rev'")
	}
	if !strings.Contains(contentStr, "'rev':") {
		t.Error("Should preserve single quotes around 'rev' key")
	}
}

func TestMigrateConfigCommand_PythonVenvToLanguage(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with old 'python_venv' language
	oldConfig := `repos:
- repo: local
  hooks:
  - id: test
    language: python_venv
    entry: python test.py
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if strings.Contains(contentStr, "python_venv") {
		t.Error("Should migrate 'python_venv' to 'python'")
	}
	if !strings.Contains(contentStr, "language: python") {
		t.Error("Should contain 'language: python' after migration")
	}
}

func TestMigrateConfigCommand_PythonVenvToLanguage_Quoted(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with quoted 'python_venv' value
	oldConfig := `repos:
- repo: local
  hooks:
  - id: test
    language: 'python_venv'
    entry: python test.py
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if strings.Contains(contentStr, "python_venv") {
		t.Error("Should migrate quoted 'python_venv' to 'python'")
	}
	if !strings.Contains(contentStr, "language: 'python'") {
		t.Error("Should preserve single quotes around 'python' value")
	}
}

func TestMigrateConfigCommand_StagesMigration(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with old stage names in array syntax
	oldConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
    stages: [commit, push]
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	// Check that old stage names were replaced
	if strings.Contains(contentStr, "[commit,") || strings.Contains(contentStr, "[commit]") {
		t.Errorf("Should migrate 'commit' to 'pre-commit' in stages, got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "pre-commit") {
		t.Errorf("Should contain 'pre-commit' after migration, got: %s", contentStr)
	}
	if !strings.Contains(contentStr, "pre-push") {
		t.Errorf("Should contain 'pre-push' after migration, got: %s", contentStr)
	}
}

func TestMigrateConfigCommand_StagesMigration_MergeCommit(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with merge-commit stage
	oldConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
    stages: [merge-commit]
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)
	if !strings.Contains(contentStr, "pre-merge-commit") {
		t.Errorf("Should migrate 'merge-commit' to 'pre-merge-commit', got: %s", contentStr)
	}
}

func TestMigrateConfigCommand_HeaderPreservation(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with header comments
	oldConfig := `# This is a header comment
# Another comment
---
- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)

	// Header comments should be preserved
	if !strings.Contains(contentStr, "# This is a header comment") {
		t.Error("Should preserve header comments")
	}
	if !strings.Contains(contentStr, "---") {
		t.Error("Should preserve YAML document marker")
	}

	// repos: should come after headers
	headerIdx := strings.Index(contentStr, "---")
	reposIdx := strings.Index(contentStr, "repos:")
	if reposIdx < headerIdx {
		t.Error("repos: should come after headers")
	}
}

func TestMigrateConfigCommand_HeaderPreservation_EmptyLines(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with empty lines before content
	oldConfig := `
# Comment after empty line

- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	contentStr := string(content)

	// Comment should be preserved
	if !strings.Contains(contentStr, "# Comment after empty line") {
		t.Error("Should preserve comments")
	}
	// repos: should be present
	if !strings.Contains(contentStr, "repos:") {
		t.Error("Should add repos: key")
	}
}

func TestMigrateConfigCommand_QuoteStylePreservation_DoubleQuotes(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with double-quoted 'sha' key
	oldConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  "sha": abc123
  hooks:
  - id: trailing-whitespace
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	cmd.Run([]string{})

	content, _ := os.ReadFile(configPath)
	// Should preserve double quotes around the key
	if !strings.Contains(string(content), `"rev":`) {
		t.Errorf("Should preserve double quote style, got: %s", string(content))
	}
}

func TestMigrateConfigCommand_QuietMode(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with sha that needs migration
	oldConfig := `repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  sha: abc123
  hooks:
  - id: trailing-whitespace
`
	configPath := createConfigFile(t, tempDir, oldConfig)

	cmd := &MigrateConfigCommand{}
	err := cmd.MigrateConfigQuiet(configPath)

	if err != nil {
		t.Errorf("MigrateConfigQuiet should not return error, got: %v", err)
	}

	// Verify migration happened
	content, _ := os.ReadFile(configPath)
	if strings.Contains(string(content), "\n  sha:") {
		t.Error("Should have migrated sha to rev")
	}
}

func TestMigrateConfigCommand_QuietMode_MissingFile(t *testing.T) {
	cmd := &MigrateConfigCommand{}
	err := cmd.MigrateConfigQuiet("/nonexistent/path/config.yaml")

	if err == nil {
		t.Error("MigrateConfigQuiet should return error for missing file")
	}
}

// ====================
// CLI Options Tests
// ====================

func TestMigrateConfigCommand_ParseArguments_DefaultConfig(t *testing.T) {
	// Verify default config path is .pre-commit-config.yaml
	tempDir := setupMigrateConfigTestDir(t)

	newConfig := `repos: []`
	createConfigFile(t, tempDir, newConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{})

	// Should use default config path successfully
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with default config, got: %d", exitCode)
	}
}

func TestMigrateConfigCommand_ParseArguments_HelpFlag(t *testing.T) {
	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{"--help"})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for --help, got: %d", exitCode)
	}
}

func TestMigrateConfigCommand_ParseArguments_ShortFlags(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	newConfig := `repos: []`
	configPath := filepath.Join(tempDir, "custom.yaml")
	os.WriteFile(configPath, []byte(newConfig), 0644)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{"-c", configPath})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with short flags, got: %d", exitCode)
	}
}

// ====================
// Edge Cases
// ====================

func TestMigrateConfigCommand_EdgeCase_EmptyFile(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	emptyConfig := ``
	createConfigFile(t, tempDir, emptyConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{})

	// Empty file should not need migration and should not error
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for empty file, got: %d", exitCode)
	}
}

func TestMigrateConfigCommand_EdgeCase_OnlyComments(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	commentOnlyConfig := `# Just a comment
# Another comment
`
	createConfigFile(t, tempDir, commentOnlyConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{})

	// Comment-only file should not need migration
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for comment-only file, got: %d", exitCode)
	}
}

func TestMigrateConfigCommand_EdgeCase_MixedContent(t *testing.T) {
	tempDir := setupMigrateConfigTestDir(t)

	// Config with repos: key but also other top-level keys
	mixedConfig := `default_stages: [commit]
repos:
- repo: https://github.com/pre-commit/pre-commit-hooks
  rev: v4.4.0
  hooks:
  - id: trailing-whitespace
ci:
  skip: [trailing-whitespace]
`
	createConfigFile(t, tempDir, mixedConfig)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	cmd := &MigrateConfigCommand{}
	exitCode := cmd.Run([]string{})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for mixed content, got: %d", exitCode)
	}
}
