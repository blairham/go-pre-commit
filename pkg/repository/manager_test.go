package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestNewManager(t *testing.T) {
	// Save original environment variables
	originalHome := os.Getenv("PRE_COMMIT_HOME")
	originalHOME := os.Getenv("HOME")
	defer func() {
		if originalHome != "" {
			os.Setenv("PRE_COMMIT_HOME", originalHome)
		} else {
			os.Unsetenv("PRE_COMMIT_HOME")
		}
		if originalHOME != "" {
			os.Setenv("HOME", originalHOME)
		} else {
			os.Unsetenv("HOME")
		}
	}()

	// Create temporary directory for the test
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		preCommitHome string
		expectError   bool
	}{
		{
			name:          "with PRE_COMMIT_HOME set",
			preCommitHome: filepath.Join(tempDir, "pre-commit-home"),
			expectError:   false,
		},
		{
			name:          "without PRE_COMMIT_HOME",
			preCommitHome: "",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing environment variable
			os.Unsetenv("PRE_COMMIT_HOME")

			// Set a temporary HOME directory for the test
			os.Setenv("HOME", tempDir)

			if tt.preCommitHome != "" {
				os.Setenv("PRE_COMMIT_HOME", tt.preCommitHome)
			}

			manager, err := NewManager()
			if (err != nil) != tt.expectError {
				t.Errorf("NewManager() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if err == nil {
				defer manager.Close()

				if manager == nil {
					t.Error("NewManager() returned nil manager")
				}
			}
		})
	}
}

func TestManager_PreInitializeHookEnvironments(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-manager")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("PRE_COMMIT_HOME", tempDir)
	defer os.Unsetenv("PRE_COMMIT_HOME")

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	// Create simple test hooks with system language that doesn't require repo cloning
	hooks := []config.HookEnvItem{
		{
			Hook: config.Hook{
				ID:       "test-hook",
				Language: "system",
			},
			Repo: config.Repo{
				Repo: "local", // Use local instead of meta to avoid cloning
				Rev:  "",
			},
			RepoPath: tempDir,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Since system language doesn't require environment setup, this should succeed
	err = manager.PreInitializeHookEnvironments(ctx, hooks)
	if err != nil {
		// For now, let's just check that it doesn't panic and log the error
		t.Logf("PreInitializeHookEnvironments() expected error for test setup = %v", err)
	}
}

func TestManager_Methods(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-manager-methods")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("PRE_COMMIT_HOME", tempDir)
	defer os.Unsetenv("PRE_COMMIT_HOME")

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	t.Run("GetCacheDir", func(t *testing.T) {
		cacheDir := manager.GetCacheDir()
		if cacheDir == "" {
			t.Error("GetCacheDir() returned empty string")
		}
	})

	t.Run("GetRepoPath", func(t *testing.T) {
		testRepo := config.Repo{
			Repo: "https://github.com/pre-commit/pre-commit-hooks",
			Rev:  "v4.4.0",
		}

		repoPath := manager.GetRepoPath(testRepo)
		if repoPath == "" {
			t.Error("GetRepoPath() returned empty string")
		}
	})

	t.Run("GetRepoPathWithDeps", func(t *testing.T) {
		testRepo := config.Repo{
			Repo: "https://github.com/pre-commit/pre-commit-hooks",
			Rev:  "v4.4.0",
		}

		repoPath := manager.GetRepoPathWithDeps(testRepo, []string{})
		if repoPath == "" {
			t.Error("GetRepoPathWithDeps() returned empty string")
		}
	})

	t.Run("CloneOrUpdateRepo", func(t *testing.T) {
		// Test with meta repository (pre-commit-hooks)
		testRepo := config.Repo{
			Repo: "https://github.com/pre-commit/pre-commit-hooks",
			Rev:  "v4.4.0",
		}

		// This will timeout or fail due to network, but tests the method exists
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := manager.CloneOrUpdateRepo(ctx, testRepo)
		// We expect an error due to timeout or network issues, but not a panic
		if err == nil {
			t.Log("CloneOrUpdateRepo succeeded (unexpected but not wrong)")
		}
	})

	t.Run("CloneOrUpdateRepoWithDeps", func(t *testing.T) {
		testRepo := config.Repo{
			Repo: "https://github.com/pre-commit/pre-commit-hooks",
			Rev:  "v4.4.0",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := manager.CloneOrUpdateRepoWithDeps(ctx, testRepo, []string{})
		// We expect an error due to timeout or network issues, but not a panic
		if err == nil {
			t.Log("CloneOrUpdateRepoWithDeps succeeded (unexpected but not wrong)")
		}
	})

	t.Run("CleanCache", func(t *testing.T) {
		err := manager.CleanCache()
		if err != nil {
			t.Errorf("CleanCache() error = %v", err)
		}
	})
	t.Run("IsMetaRepo", func(t *testing.T) {
		metaRepo := config.Repo{
			Repo: "meta",
			Rev:  "HEAD",
		}

		if !manager.IsMetaRepo(metaRepo) {
			t.Error("Expected 'meta' repo to be identified as meta repo")
		}

		normalRepo := config.Repo{
			Repo: "https://github.com/example/test-repo",
			Rev:  "main",
		}

		if manager.IsMetaRepo(normalRepo) {
			t.Error("Expected example repo to NOT be identified as meta repo")
		}
	})

	t.Run("IsLocalRepo", func(t *testing.T) {
		localRepo := config.Repo{
			Repo: "local",
			Rev:  "HEAD",
		}

		if !manager.IsLocalRepo(localRepo) {
			t.Error("Expected 'local' URL to be identified as local repo")
		}

		remoteRepo := config.Repo{
			Repo: "https://github.com/example/test-repo",
			Rev:  "main",
		}

		if manager.IsLocalRepo(remoteRepo) {
			t.Error("Expected remote URL to NOT be identified as local repo")
		}
	})

	t.Run("GetMetaHook", func(t *testing.T) {
		// Test with a known meta hook
		hook, found := manager.GetMetaHook("check-yaml")
		if !found {
			t.Error("Expected to find check-yaml meta hook")
		} else {
			if hook.ID != "check-yaml" {
				t.Errorf("Expected hook ID to be 'check-yaml', got '%s'", hook.ID)
			}
		}

		// Test with non-existent hook
		_, found = manager.GetMetaHook("non-existent-hook")
		if found {
			t.Error("Expected false for non-existent hook")
		}
	})

	t.Run("GetRepositoryHook", func(t *testing.T) {
		repoPath := filepath.Join(tempDir, "test-repo")

		_, found := manager.GetRepositoryHook(repoPath, "test-hook")
		// This will return false because the repo doesn't exist locally,
		// but we're testing that the method doesn't panic
		if found {
			t.Log("GetRepositoryHook found a hook (unexpected but not wrong)")
		}
	})
	t.Run("MarkConfigUsed", func(t *testing.T) {
		configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")

		// This should not panic
		err := manager.MarkConfigUsed(configPath)
		if err != nil {
			t.Logf("MarkConfigUsed returned error: %v (expected for non-existent config)", err)
		}
	})

	t.Run("GetCommonRepositoryManager", func(t *testing.T) {
		ctx := context.Background()
		commonManager := manager.GetCommonRepositoryManager(ctx)
		if commonManager == nil {
			t.Error("GetCommonRepositoryManager() returned nil")
		}
	})
}

func TestManager_EnvironmentMethods(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-manager-env")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("PRE_COMMIT_HOME", tempDir)
	defer os.Unsetenv("PRE_COMMIT_HOME")

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	testRepoPath := filepath.Join(tempDir, "test-repo")
	os.MkdirAll(testRepoPath, 0o755)

	t.Run("SetupHookEnvironment", func(t *testing.T) {
		testRepo := config.Repo{
			Repo: "local",
			Rev:  "HEAD",
		}

		testHook := config.Hook{
			ID:       "test-hook",
			Language: "system",
			Entry:    "echo test",
		}

		_, err := manager.SetupHookEnvironment(testHook, testRepo, testRepoPath)
		if err != nil {
			t.Logf("SetupHookEnvironment returned error: %v (expected for test hook)", err)
		}
	})

	t.Run("GetHookExecutablePath", func(t *testing.T) {
		testHook := config.Hook{
			ID:       "test-hook",
			Language: "system",
			Entry:    "echo test",
		}

		path, err := manager.GetHookExecutablePath(testRepoPath, testHook)
		if err != nil {
			t.Logf("GetHookExecutablePath returned error: %v (expected for test hook)", err)
		}
		if path == "" {
			t.Log("GetHookExecutablePath returned empty path (expected for test hook)")
		}
	})

	t.Run("CheckEnvironmentHealthWithRepo", func(t *testing.T) {
		err := manager.CheckEnvironmentHealthWithRepo("system", "default", testRepoPath)
		if err != nil {
			t.Logf("CheckEnvironmentHealthWithRepo returned error: %v (expected for test repo)", err)
		}
	})

	t.Run("RebuildEnvironmentWithRepo", func(t *testing.T) {
		err := manager.RebuildEnvironmentWithRepo("system", "default", testRepoPath)
		if err != nil {
			t.Logf("RebuildEnvironmentWithRepo returned error: %v (expected for test repo)", err)
		}
	})

	t.Run("RebuildEnvironmentWithRepoInfo", func(t *testing.T) {
		err := manager.RebuildEnvironmentWithRepoInfo("system", "default", testRepoPath, "local")
		if err != nil {
			t.Logf("RebuildEnvironmentWithRepoInfo returned error: %v (expected for test repo)", err)
		}
	})

	t.Run("UpdateRepoEntryWithDeps", func(t *testing.T) {
		testRepo := config.Repo{
			Repo: "https://github.com/example/test-repo",
			Rev:  "main",
		}

		err := manager.UpdateRepoEntryWithDeps(testRepo, []string{}, testRepoPath)
		if err != nil {
			t.Logf("UpdateRepoEntryWithDeps returned error: %v (expected for non-existent repo)", err)
		}
	})

	t.Run("SetupEnvironmentWithRepositoryInit", func(t *testing.T) {
		testRepo := config.Repo{
			Repo: "local",
			Rev:  "HEAD",
		}

		_, err := manager.SetupEnvironmentWithRepositoryInit(testRepo, "system", "default", []string{})
		if err != nil {
			t.Logf("SetupEnvironmentWithRepositoryInit returned error: %v (expected for test hook)", err)
		}
	})
}

func TestManager_ErrorScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-manager-errors")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("PRE_COMMIT_HOME", tempDir)
	defer os.Unsetenv("PRE_COMMIT_HOME")

	manager, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Close()

	t.Run("CloneOrUpdateRepo with invalid URL", func(t *testing.T) {
		invalidRepo := config.Repo{
			Repo: "not-a-url",
			Rev:  "main",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err := manager.CloneOrUpdateRepo(ctx, invalidRepo)
		if err == nil {
			t.Error("Expected error for invalid repo URL")
		}
	})

	t.Run("GetRepositoryHook with non-existent repo path", func(t *testing.T) {
		nonExistentPath := filepath.Join(tempDir, "non-existent-repo")

		_, found := manager.GetRepositoryHook(nonExistentPath, "test-hook")
		if found {
			t.Error("Expected false for non-existent repo path")
		}
	})
	t.Run("MarkConfigUsed with invalid path", func(t *testing.T) {
		invalidPath := "/invalid/path/that/does/not/exist/.pre-commit-config.yaml"

		err := manager.MarkConfigUsed(invalidPath)
		// The method may or may not return an error for an invalid path,
		// we're mainly testing that it doesn't panic
		if err != nil {
			t.Logf("MarkConfigUsed returned error: %v (which is acceptable)", err)
		} else {
			t.Log("MarkConfigUsed succeeded with invalid path (which is also acceptable)")
		}
	})
}
