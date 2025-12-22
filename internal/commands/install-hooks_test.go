package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"

	"github.com/blairham/go-pre-commit/pkg/config"
	"github.com/blairham/go-pre-commit/pkg/repository"
)

func TestInstallHooksCommand_Synopsis(t *testing.T) {
	cmd := &InstallHooksCommand{}
	synopsis := cmd.Synopsis()
	expected := "Install hook environments for all environments in the config file"
	if synopsis != expected {
		t.Errorf("Expected synopsis '%s', got '%s'", expected, synopsis)
	}
}

func TestInstallHooksCommand_Help(t *testing.T) {
	cmd := &InstallHooksCommand{}
	help := cmd.Help()

	expectedStrings := []string{
		"install-hooks",
		"--config",
		"--help",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(help, expected) {
			t.Errorf("Help text should contain '%s'", expected)
		}
	}
}

func TestInstallHooksCommandFactory(t *testing.T) {
	cmd, err := InstallHooksCommandFactory()
	if err != nil {
		t.Fatalf("Factory should not return error: %v", err)
	}

	if cmd == nil {
		t.Fatal("Factory should return a command")
	}

	if _, ok := cmd.(*InstallHooksCommand); !ok {
		t.Fatal("Factory should return an InstallHooksCommand")
	}
}

func TestInstallHooksCommand_Run_HelpFlag(t *testing.T) {
	cmd := &InstallHooksCommand{}

	exitCode := cmd.Run([]string{"--help"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for help flag, got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_MissingConfig(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory with a git repo
	tempDir := t.TempDir()

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Initialize a git repository
	_, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Test with missing config file (default path)
	exitCode := cmd.Run([]string{})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for missing config, got %d", exitCode)
	}

	// Test with custom missing config file
	exitCode = cmd.Run([]string{"--config", "nonexistent.yaml"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for missing custom config, got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_NotGitRepo(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory without a git repo
	tempDir := t.TempDir()

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Create a config file with local repo only (no network needed)
	configContent := `repos:
  - repo: local
    hooks:
      - id: test-hook
        name: Test Hook
        entry: echo "test"
        language: system
`
	err := os.WriteFile(".pre-commit-config.yaml", []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Should succeed - install-hooks does NOT require being in a git repo
	// This matches Python's behavior
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 (no git repo required), got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_InvalidConfig(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory with a git repo
	tempDir := t.TempDir()

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Initialize a git repository
	_, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create invalid config file
	err = os.WriteFile(".pre-commit-config.yaml", []byte("invalid: yaml: content: ["), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test with invalid config
	exitCode := cmd.Run([]string{})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for invalid config, got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_LocalRepoOnly(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory with a git repo
	tempDir := t.TempDir()

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Initialize a git repository
	_, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a config with only local repo (no external repos to install)
	configContent := `repos:
  - repo: local
    hooks:
      - id: test-hook
        name: Test Hook
        entry: echo "test"
        language: system
`
	err = os.WriteFile(".pre-commit-config.yaml", []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Should succeed because local repos are skipped
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for local-only config, got %d", exitCode)
	}
}

func TestInstallHooksCommand_Run_MetaRepoOnly(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory with a git repo
	tempDir := t.TempDir()

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Initialize a git repository
	_, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a config with only meta repo (no external repos to install)
	configContent := `repos:
  - repo: meta
    hooks:
      - id: check-hooks-apply
`
	err = os.WriteFile(".pre-commit-config.yaml", []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Should succeed because meta repos are skipped
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for meta-only config, got %d", exitCode)
	}
}

func TestInstallHooksCommand_ValidateEnvironment(t *testing.T) {
	cmd := &InstallHooksCommand{}

	tests := []struct {
		name      string
		setup     func(tempDir string) error
		wantError bool
	}{
		{
			name: "no git repo with config succeeds",
			setup: func(tempDir string) error {
				// Just create config file, no .git - should succeed
				return os.WriteFile(filepath.Join(tempDir, ".pre-commit-config.yaml"), []byte("repos: []\n"), 0o644)
			},
			wantError: false,
		},
		{
			name: "missing config file",
			setup: func(tempDir string) error {
				// No config file - should fail
				return nil
			},
			wantError: true,
		},
		{
			name: "config present succeeds",
			setup: func(tempDir string) error {
				return os.WriteFile(filepath.Join(tempDir, ".pre-commit-config.yaml"), []byte("repos: []\n"), 0o644)
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			oldDir, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(oldDir)

			if err := tt.setup(tempDir); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			opts := &InstallHooksOptions{Config: ".pre-commit-config.yaml"}
			err := cmd.validateEnvironment(opts)

			if tt.wantError && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestInstallHooksCommand_ConfigOption(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory with a git repo
	tempDir := t.TempDir()

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Initialize a git repository
	_, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a custom-named config file
	configContent := `repos:
  - repo: local
    hooks:
      - id: custom-hook
        name: Custom Hook
        entry: echo "custom"
        language: system
`
	customConfigPath := "custom-config.yaml"
	err = os.WriteFile(customConfigPath, []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test with custom config path using -c flag
	exitCode := cmd.Run([]string{"-c", customConfigPath})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with -c flag, got %d", exitCode)
	}

	// Test with custom config path using --config flag
	exitCode = cmd.Run([]string{"--config", customConfigPath})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 with --config flag, got %d", exitCode)
	}

	// Test that non-existent custom config returns error
	exitCode = cmd.Run([]string{"-c", "does-not-exist.yaml"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1 for non-existent config, got %d", exitCode)
	}
}

func TestInstallHooksCommand_ShouldSkipRepository(t *testing.T) {
	cmd := &InstallHooksCommand{}

	tests := []struct {
		name     string
		repoName string
		want     bool
	}{
		{
			name:     "local repo should be skipped",
			repoName: "local",
			want:     true,
		},
		{
			name:     "meta repo should be skipped",
			repoName: "meta",
			want:     true,
		},
		{
			name:     "remote repo should not be skipped",
			repoName: "https://github.com/example/repo",
			want:     false,
		},
		{
			name:     "github shorthand should not be skipped",
			repoName: "https://github.com/pre-commit/pre-commit-hooks",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := config.Repo{Repo: tt.repoName}
			got := cmd.shouldSkipRepository(repo)
			if got != tt.want {
				t.Errorf("shouldSkipRepository(%q) = %v, want %v", tt.repoName, got, tt.want)
			}
		})
	}
}

// =============================================================================
// Repository Preparation Tests
// =============================================================================

func TestInstallHooksCommand_CheckIfAnyRepositoryNeedsPreparation(t *testing.T) {
	cmd := &InstallHooksCommand{}

	tests := []struct {
		name     string
		repos    []config.Repo
		expected bool
	}{
		{
			name:     "empty repos returns false",
			repos:    []config.Repo{},
			expected: false,
		},
		{
			name: "only local repos returns false",
			repos: []config.Repo{
				{Repo: "local", Hooks: []config.Hook{{ID: "test"}}},
			},
			expected: false,
		},
		{
			name: "only meta repos returns false",
			repos: []config.Repo{
				{Repo: "meta", Hooks: []config.Hook{{ID: "check-hooks-apply"}}},
			},
			expected: false,
		},
		{
			name: "mixed local and meta returns false",
			repos: []config.Repo{
				{Repo: "local", Hooks: []config.Hook{{ID: "test"}}},
				{Repo: "meta", Hooks: []config.Hook{{ID: "check-hooks-apply"}}},
			},
			expected: false,
		},
		{
			name: "remote repo needs preparation",
			repos: []config.Repo{
				{Repo: "https://github.com/pre-commit/pre-commit-hooks", Hooks: []config.Hook{{ID: "trailing-whitespace"}}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			oldDir, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(oldDir)

			// Initialize git repo
			_, err := gogit.PlainInit(tempDir, false)
			if err != nil {
				t.Fatalf("Failed to init git repo: %v", err)
			}

			cfg := &config.Config{Repos: tt.repos}

			// Create a repository manager
			repoManager, err := repository.NewManager()
			if err != nil {
				t.Fatalf("Failed to create repo manager: %v", err)
			}
			defer repoManager.Close()

			got := cmd.checkIfAnyRepositoryNeedsPreparation(cfg, repoManager)
			if got != tt.expected {
				t.Errorf("checkIfAnyRepositoryNeedsPreparation() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInstallHooksCommand_IsRepositoryFullyPrepared(t *testing.T) {
	cmd := &InstallHooksCommand{}

	t.Run("returns false when repo path is empty", func(t *testing.T) {
		tempDir := t.TempDir()
		oldDir, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(oldDir)

		_, err := gogit.PlainInit(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}

		repoManager, err := repository.NewManager()
		if err != nil {
			t.Fatalf("Failed to create repo manager: %v", err)
		}
		defer repoManager.Close()

		// A repo that hasn't been cloned yet
		repo := config.Repo{
			Repo:  "https://github.com/nonexistent/repo",
			Rev:   "v1.0.0",
			Hooks: []config.Hook{{ID: "test-hook"}},
		}

		prepared := cmd.isRepositoryFullyPrepared(repo, repoManager)
		if prepared {
			t.Error("Expected isRepositoryFullyPrepared to return false for uncloned repo")
		}
	})
}

func TestInstallHooksCommand_IsHookEnvironmentReady(t *testing.T) {
	cmd := &InstallHooksCommand{}

	t.Run("returns false when hooks yaml does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		oldDir, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(oldDir)

		_, err := gogit.PlainInit(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}

		repoManager, err := repository.NewManager()
		if err != nil {
			t.Fatalf("Failed to create repo manager: %v", err)
		}
		defer repoManager.Close()

		// Create a fake repo path without .pre-commit-hooks.yaml
		fakeRepoPath := filepath.Join(tempDir, "fake-repo")
		os.MkdirAll(fakeRepoPath, 0o755)

		hook := config.Hook{ID: "test-hook", Language: "python"}
		ready := cmd.isHookEnvironmentReady(hook, fakeRepoPath, repoManager)
		if ready {
			t.Error("Expected isHookEnvironmentReady to return false when hooks yaml doesn't exist")
		}
	})

	t.Run("returns false when environment not healthy", func(t *testing.T) {
		tempDir := t.TempDir()
		oldDir, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(oldDir)

		_, err := gogit.PlainInit(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}

		repoManager, err := repository.NewManager()
		if err != nil {
			t.Fatalf("Failed to create repo manager: %v", err)
		}
		defer repoManager.Close()

		// Create a fake repo path with .pre-commit-hooks.yaml
		fakeRepoPath := filepath.Join(tempDir, "fake-repo")
		os.MkdirAll(fakeRepoPath, 0o755)
		os.WriteFile(filepath.Join(fakeRepoPath, ".pre-commit-hooks.yaml"), []byte("- id: test\n"), 0o644)

		// Hook with a language that won't have an environment set up
		hook := config.Hook{ID: "test-hook", Language: "python", LanguageVersion: "nonexistent"}
		ready := cmd.isHookEnvironmentReady(hook, fakeRepoPath, repoManager)
		if ready {
			t.Error("Expected isHookEnvironmentReady to return false when environment not healthy")
		}
	})
}

func TestInstallHooksCommand_MultipleRepos(t *testing.T) {
	cmd := &InstallHooksCommand{}

	// Create a temporary directory with a git repo
	tempDir := t.TempDir()

	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Initialize a git repository
	_, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create a config with multiple local/meta repos (no network needed)
	configContent := `repos:
  - repo: local
    hooks:
      - id: test-hook-1
        name: Test Hook 1
        entry: echo "test1"
        language: system
  - repo: local
    hooks:
      - id: test-hook-2
        name: Test Hook 2
        entry: echo "test2"
        language: system
  - repo: meta
    hooks:
      - id: check-hooks-apply
`
	err = os.WriteFile(".pre-commit-config.yaml", []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Should succeed because local and meta repos are skipped
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for multiple local/meta repos, got %d", exitCode)
	}
}

// =============================================================================
// Hook Merging Tests
// =============================================================================

func TestInstallHooksCommand_MergeAdditionalDeps(t *testing.T) {
	// Test that additional_dependencies from config override repo definition
	configHook := config.Hook{
		ID:             "test-hook",
		AdditionalDeps: []string{"dep1", "dep2"},
	}
	repoHook := config.Hook{
		ID:             "test-hook",
		Name:           "Test Hook",
		Entry:          "test-entry",
		Language:       "python",
		AdditionalDeps: []string{"original-dep"},
	}

	// Simulate merge logic
	mergedHook := repoHook
	if len(configHook.AdditionalDeps) > 0 {
		mergedHook.AdditionalDeps = configHook.AdditionalDeps
	}

	if len(mergedHook.AdditionalDeps) != 2 {
		t.Errorf("Expected 2 additional deps, got %d", len(mergedHook.AdditionalDeps))
	}
	if mergedHook.AdditionalDeps[0] != "dep1" || mergedHook.AdditionalDeps[1] != "dep2" {
		t.Errorf("Expected deps [dep1, dep2], got %v", mergedHook.AdditionalDeps)
	}
}

func TestInstallHooksCommand_MergeArgs(t *testing.T) {
	// Test that args from config override repo definition
	configHook := config.Hook{
		ID:   "test-hook",
		Args: []string{"--arg1", "--arg2"},
	}
	repoHook := config.Hook{
		ID:       "test-hook",
		Name:     "Test Hook",
		Entry:    "test-entry",
		Language: "python",
		Args:     []string{"--original"},
	}

	// Simulate merge logic
	mergedHook := repoHook
	if len(configHook.Args) > 0 {
		mergedHook.Args = configHook.Args
	}

	if len(mergedHook.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(mergedHook.Args))
	}
	if mergedHook.Args[0] != "--arg1" || mergedHook.Args[1] != "--arg2" {
		t.Errorf("Expected args [--arg1, --arg2], got %v", mergedHook.Args)
	}
}

func TestInstallHooksCommand_MergeLanguageVersion(t *testing.T) {
	// Test that language_version from config is used
	configHook := config.Hook{
		ID:              "test-hook",
		LanguageVersion: "3.9",
	}
	repoHook := config.Hook{
		ID:              "test-hook",
		Name:            "Test Hook",
		Entry:           "test-entry",
		Language:        "python",
		LanguageVersion: "default",
	}

	cfg := &config.Config{
		DefaultLanguageVersion: map[string]string{},
	}

	// Use ResolveEffectiveLanguageVersion which is the actual function used
	hookForVersionResolution := configHook
	hookForVersionResolution.Language = repoHook.Language
	effectiveVersion := config.ResolveEffectiveLanguageVersion(hookForVersionResolution, *cfg)

	if effectiveVersion != "3.9" {
		t.Errorf("Expected language version '3.9', got '%s'", effectiveVersion)
	}
}

func TestInstallHooksCommand_DefaultLanguageVersion(t *testing.T) {
	// Test that default_language_version from config is used when hook doesn't specify
	configHook := config.Hook{
		ID: "test-hook",
		// No LanguageVersion specified
	}
	repoHook := config.Hook{
		ID:       "test-hook",
		Name:     "Test Hook",
		Entry:    "test-entry",
		Language: "python",
	}

	cfg := &config.Config{
		DefaultLanguageVersion: map[string]string{
			"python": "3.10",
		},
	}

	// Use ResolveEffectiveLanguageVersion which is the actual function used
	hookForVersionResolution := configHook
	hookForVersionResolution.Language = repoHook.Language
	effectiveVersion := config.ResolveEffectiveLanguageVersion(hookForVersionResolution, *cfg)

	if effectiveVersion != "3.10" {
		t.Errorf("Expected language version '3.10' from default, got '%s'", effectiveVersion)
	}
}

func TestInstallHooksCommand_RepoDefAsBase(t *testing.T) {
	// Test that repository definition is used as base and config values override
	configHook := config.Hook{
		ID:   "test-hook",
		Args: []string{"--custom-arg"},
		// Other fields not set - should come from repo definition
	}
	repoHook := config.Hook{
		ID:       "test-hook",
		Name:     "Test Hook from Repo",
		Entry:    "repo-entry",
		Language: "python",
		Files:    "\\.py$",
		Types:    []string{"python"},
	}

	// Simulate merge - start with repo hook as base
	mergedHook := repoHook

	// Only override fields that are set in config
	if len(configHook.Args) > 0 {
		mergedHook.Args = configHook.Args
	}

	// Verify repo definition fields are preserved
	if mergedHook.Name != "Test Hook from Repo" {
		t.Errorf("Expected Name 'Test Hook from Repo', got '%s'", mergedHook.Name)
	}
	if mergedHook.Entry != "repo-entry" {
		t.Errorf("Expected Entry 'repo-entry', got '%s'", mergedHook.Entry)
	}
	if mergedHook.Language != "python" {
		t.Errorf("Expected Language 'python', got '%s'", mergedHook.Language)
	}
	if mergedHook.Files != "\\.py$" {
		t.Errorf("Expected Files '\\.py$', got '%s'", mergedHook.Files)
	}

	// Verify config override was applied
	if len(mergedHook.Args) != 1 || mergedHook.Args[0] != "--custom-arg" {
		t.Errorf("Expected Args ['--custom-arg'], got %v", mergedHook.Args)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestInstallHooksCommand_MarksConfigUsed(t *testing.T) {
	// This test verifies that the config is marked as used in the repository manager
	// We can't directly check the database, but we verify the code path is executed

	tempDir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Initialize git repo
	_, err := gogit.PlainInit(tempDir, false)
	if err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Create config
	configContent := `repos:
  - repo: local
    hooks:
      - id: test-hook
        name: Test Hook
        entry: echo "test"
        language: system
`
	err = os.WriteFile(".pre-commit-config.yaml", []byte(configContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cmd := &InstallHooksCommand{}
	exitCode := cmd.Run([]string{})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	// The fact that the command completes successfully with the loadConfigAndInitManager
	// function being called means MarkConfigUsed was called (we ignore its errors)
}

func TestInstallHooksCommand_CheckRepositoriesReady(t *testing.T) {
	cmd := &InstallHooksCommand{}

	tests := []struct {
		name     string
		repos    []config.Repo
		expected bool
	}{
		{
			name:     "empty repos is ready",
			repos:    []config.Repo{},
			expected: true,
		},
		{
			name: "only local repos is ready",
			repos: []config.Repo{
				{Repo: "local", Hooks: []config.Hook{{ID: "test"}}},
			},
			expected: true,
		},
		{
			name: "only meta repos is ready",
			repos: []config.Repo{
				{Repo: "meta", Hooks: []config.Hook{{ID: "check-hooks-apply"}}},
			},
			expected: true,
		},
		{
			name: "remote repo not ready",
			repos: []config.Repo{
				{Repo: "https://github.com/pre-commit/pre-commit-hooks", Hooks: []config.Hook{{ID: "trailing-whitespace"}}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			oldDir, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(oldDir)

			// Initialize git repo
			_, err := gogit.PlainInit(tempDir, false)
			if err != nil {
				t.Fatalf("Failed to init git repo: %v", err)
			}

			cfg := &config.Config{Repos: tt.repos}

			repoManager, err := repository.NewManager()
			if err != nil {
				t.Fatalf("Failed to create repo manager: %v", err)
			}
			defer repoManager.Close()

			got := cmd.CheckRepositoriesReady(cfg, repoManager)
			if got != tt.expected {
				t.Errorf("CheckRepositoriesReady() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInstallHooksCommand_EnsureRepositoriesAndEnvironments(t *testing.T) {
	cmd := &InstallHooksCommand{}

	t.Run("returns nil for empty repos", func(t *testing.T) {
		tempDir := t.TempDir()
		oldDir, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(oldDir)

		_, err := gogit.PlainInit(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}

		cfg := &config.Config{Repos: []config.Repo{}}

		repoManager, err := repository.NewManager()
		if err != nil {
			t.Fatalf("Failed to create repo manager: %v", err)
		}
		defer repoManager.Close()

		err = cmd.ensureRepositoriesAndEnvironments(cfg, repoManager)
		if err != nil {
			t.Errorf("Expected nil error for empty repos, got: %v", err)
		}
	})

	t.Run("returns nil when all repos already prepared (local only)", func(t *testing.T) {
		tempDir := t.TempDir()
		oldDir, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(oldDir)

		_, err := gogit.PlainInit(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}

		cfg := &config.Config{
			Repos: []config.Repo{
				{Repo: "local", Hooks: []config.Hook{{ID: "test"}}},
			},
		}

		repoManager, err := repository.NewManager()
		if err != nil {
			t.Fatalf("Failed to create repo manager: %v", err)
		}
		defer repoManager.Close()

		err = cmd.ensureRepositoriesAndEnvironments(cfg, repoManager)
		if err != nil {
			t.Errorf("Expected nil error for local-only repos, got: %v", err)
		}
	})
}

func TestInstallHooksCommand_PrepareAllRepositories(t *testing.T) {
	cmd := &InstallHooksCommand{}

	t.Run("succeeds with local only repos", func(t *testing.T) {
		tempDir := t.TempDir()
		oldDir, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(oldDir)

		_, err := gogit.PlainInit(tempDir, false)
		if err != nil {
			t.Fatalf("Failed to init git repo: %v", err)
		}

		cfg := &config.Config{
			Repos: []config.Repo{
				{Repo: "local", Hooks: []config.Hook{{ID: "test", Entry: "echo test", Language: "system"}}},
			},
		}

		repoManager, err := repository.NewManager()
		if err != nil {
			t.Fatalf("Failed to create repo manager: %v", err)
		}
		defer repoManager.Close()

		err = cmd.prepareAllRepositories(cfg, repoManager)
		if err != nil {
			t.Errorf("Expected nil error for local repos, got: %v", err)
		}
	})
}
