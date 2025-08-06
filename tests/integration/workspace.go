package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	localRepo   = "local"
	haskellLang = "haskell"
	failLang    = "fail"
)

// WorkspaceManager handles creation and management of test workspaces
type WorkspaceManager struct {
	suite         *Suite
	fileGenerator *FileGenerator
}

// NewWorkspaceManager creates a new workspace manager
func NewWorkspaceManager(suite *Suite) *WorkspaceManager {
	return &WorkspaceManager{
		suite:         suite,
		fileGenerator: NewFileGenerator(),
	}
}

// CreateTestWorkspace creates a test workspace for the language test
func (wm *WorkspaceManager) CreateTestWorkspace(
	t *testing.T,
	test LanguageCompatibilityTest,
) string {
	t.Helper()
	testDir, err := os.MkdirTemp("", fmt.Sprintf("precommit-test-%s-", test.Language))
	if err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	// Create test repository structure
	repoDir := filepath.Join(testDir, "test-repo")
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		t.Fatalf("Failed to create test repo directory: %v", err)
	}

	// Create isolated cache directory for this test to prevent race conditions
	cacheDir := filepath.Join(testDir, "cache")
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		t.Fatalf("Failed to create test cache directory: %v", err)
	}

	// Initialize git repository
	if err := wm.runGitCommand(repoDir, "init"); err != nil {
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Configure git for testing (disable GPG signing and set user info)
	if err := wm.runGitCommand(repoDir, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("Failed to set git user name: %v", err)
	}
	if err := wm.runGitCommand(repoDir, "config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("Failed to set git user email: %v", err)
	}
	if err := wm.runGitCommand(repoDir, "config", "commit.gpgsign", "false"); err != nil {
		t.Fatalf("Failed to disable GPG signing: %v", err)
	}

	// Create test pre-commit config
	configPath := filepath.Join(repoDir, ".pre-commit-config.yaml")
	configContent := wm.generatePreCommitConfig(test)
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("Failed to write pre-commit config: %v", err)
	}

	// Create test files based on language
	wm.createTestFiles(t, repoDir, test)

	return testDir
}

// generatePreCommitConfig generates a pre-commit configuration for the test
func (wm *WorkspaceManager) generatePreCommitConfig(test LanguageCompatibilityTest) string {
	if test.TestRepository == localRepo && test.Language == LangGolang {
		// Generate a local Go hook configuration that uses language: golang
		return fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: %s
        name: Simple Go Test Hook
        entry: go version
        language: golang
        files: \.go$
        pass_filenames: false
`, test.HookID)
	}

	if test.TestRepository == localRepo && test.Language == ScriptLanguage {
		// Generate a local script hook configuration that uses language: script
		return fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: %s
        name: Simple Shell Script Hook
        entry: ./test-script.sh
        language: script
        files: \.txt$
        pass_filenames: false
`, test.HookID)
	}

	if test.TestRepository == localRepo && test.Language == haskellLang {
		// Generate a local Haskell hook configuration that uses language: haskell
		return fmt.Sprintf(`repos:
  - repo: local
    hooks:
      - id: %s
        name: Haskell Formatter Hook
        entry: hindent
        language: haskell
        files: \.hs$
        pass_filenames: true
        additional_dependencies: ['base']
`, test.HookID)
	}

	return fmt.Sprintf(`repos:
  - repo: %s
    rev: %s
    hooks:
      - id: %s
`, test.TestRepository, test.TestCommit, test.HookID)
}

// createTestFiles creates test files appropriate for the language
//
//nolint:gocyclo,cyclop // Switch statement for multiple languages - acceptable for test file generation
func (wm *WorkspaceManager) createTestFiles(
	t *testing.T,
	repoDir string,
	test LanguageCompatibilityTest,
) {
	t.Helper()

	// Create basic test files that most hooks can work with
	yamlFile := filepath.Join(repoDir, "test.yaml")
	yamlContent := `---
name: test
version: 1.0.0
description: Test YAML file
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}

	jsonFile := filepath.Join(repoDir, "test.json")
	jsonContent := `{
  "name": "test",
  "version": "1.0.0",
  "description": "Test JSON file"
}
`
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0o600); err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	// Create language-specific test files
	switch test.Language {
	case LangPython, "conda":
		wm.fileGenerator.CreatePythonFiles(t, repoDir)
	case LangNode:
		wm.fileGenerator.CreateNodeFiles(t, repoDir)
	case LangGolang:
		wm.fileGenerator.CreateGoFiles(t, repoDir)
	case LangRuby:
		wm.fileGenerator.CreateRubyFiles(t, repoDir)
	case LangRust:
		wm.fileGenerator.CreateRustFiles(t, repoDir)
	case "dart":
		wm.fileGenerator.CreateDartFiles(t, repoDir)
	case "swift":
		wm.fileGenerator.CreateSwiftFiles(t, repoDir)
	case "lua":
		wm.fileGenerator.CreateLuaFiles(t, repoDir)
	case "perl":
		wm.fileGenerator.CreatePerlFiles(t, repoDir)
	case "r":
		wm.fileGenerator.CreateRFiles(t, repoDir)
	case haskellLang:
		wm.fileGenerator.CreateHaskellFiles(t, repoDir)
	case "dotnet":
		wm.fileGenerator.CreateDotNetFiles(t, repoDir)
	case ScriptLanguage:
		wm.fileGenerator.CreateScriptFiles(t, repoDir)
	default:
		// Create generic test file
		txtFile := filepath.Join(repoDir, "test.txt")
		content := "This is a test file for language: " + test.Language
		if err := os.WriteFile(txtFile, []byte(content), 0o600); err != nil {
			t.Fatalf("Failed to create test text file: %v", err)
		}
	}

	// Create a README for all languages
	wm.fileGenerator.CreateReadme(t, repoDir, test)
}

// runGitCommand executes a git command in the specified directory
func (wm *WorkspaceManager) runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

// CleanupTestWorkspace removes the test workspace
func (wm *WorkspaceManager) CleanupTestWorkspace(t *testing.T, testDir string) {
	t.Helper()
	if err := os.RemoveAll(testDir); err != nil {
		// Only log as warning if the directory still exists and we can't remove it
		if _, statErr := os.Stat(testDir); statErr == nil {
			// Use logCleanupWarning function (to be defined in a shared utilities package)
			t.Logf("⚠️  Cleanup warning: failed to remove test workspace %s: %v", testDir, err)
		} else {
			// Directory doesn't exist anymore - cleanup succeeded or was already done
			t.Logf("🧹 Debug: test workspace cleanup note for %s: %v (likely already cleaned)", testDir, err)
		}
	}
}
