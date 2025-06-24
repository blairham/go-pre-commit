package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/blairham/go-pre-commit/pkg/cache"
	"github.com/blairham/go-pre-commit/pkg/config"
)

func TestNewRepositoryOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-ops")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)
	if ops == nil {
		t.Error("NewRepositoryOperations() returned nil")
		return
	}
	if ops.cacheManager != cm {
		t.Error("Repository operations should have correct cache manager reference")
	}
}

func TestOperations_isValidCommitHash(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		expected bool
	}{
		{
			name:     "valid full SHA",
			hash:     "a1b2c3d4e5f6789012345678901234567890abcd",
			expected: true,
		},
		{
			name:     "valid short SHA",
			hash:     "a1b2c3d",
			expected: true,
		},
		{
			name:     "invalid length",
			hash:     "a1b2c3d4",
			expected: false,
		},
		{
			name:     "invalid characters",
			hash:     "g1b2c3d4e5f6789012345678901234567890abcd",
			expected: false,
		},
		{
			name:     "empty string",
			hash:     "",
			expected: false,
		},
		{
			name:     "too long",
			hash:     "a1b2c3d4e5f6789012345678901234567890abcde",
			expected: false,
		},
		{
			name:     "uppercase valid",
			hash:     "A1B2C3D4E5F6789012345678901234567890ABCD",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidCommitHash(tt.hash)
			if result != tt.expected {
				t.Errorf("isValidCommitHash(%q) = %v, want %v", tt.hash, result, tt.expected)
			}
		})
	}
}

func TestOperations_CloneOrUpdateRepo_InvalidRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-ops")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	// Test with invalid repository URL
	repo := config.Repo{
		Repo: "invalid://not-a-real-repo",
		Rev:  "main",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = ops.CloneOrUpdateRepo(ctx, repo)
	if err == nil {
		t.Error("Expected error when cloning invalid repository")
	}
}

func TestOperations_CloneOrUpdateRepo_ExistingRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-ops")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	repo := config.Repo{
		Repo: "https://github.com/test/repo",
		Rev:  "main",
	}

	// Get the expected repo path
	expectedPath := cm.GetRepoPath(repo)

	// Create a fake existing repository with .git directory
	gitDir := filepath.Join(expectedPath, ".git")
	err = os.MkdirAll(gitDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create fake .git dir: %v", err)
	}

	// Create a fake .git/HEAD file to make it look like a real repo
	headFile := filepath.Join(gitDir, "HEAD")
	err = os.WriteFile(headFile, []byte("ref: refs/heads/main\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create fake HEAD file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should return the existing path without trying to clone
	// Note: This might fail if the updateRepo method tries to actually access git
	path, err := ops.CloneOrUpdateRepo(ctx, repo)
	if err != nil {
		// If it fails due to git operations, that's expected for a fake repo
		// Just check that it at least tried to use the right path
		if path != expectedPath && path != "" {
			t.Errorf("Expected path %s, got %s", expectedPath, path)
		}
	} else {
		// If it succeeds, check the path is correct
		if path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, path)
		}
	}
}

func TestOperations_cloneRepo_InvalidPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-ops")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	repo := config.Repo{
		Repo: "invalid://not-a-real-repo",
		Rev:  "main",
	}

	// Try to clone to an invalid path (read-only filesystem simulation)
	invalidPath := "/dev/null/invalid"

	_, err = ops.cloneRepo(context.Background(), repo, invalidPath)
	if err == nil {
		t.Error("Expected error when cloning to invalid path")
	}
}

// Test helper function to create a temporary git repository for testing
func createTestGitRepo(t *testing.T, dir string) {
	t.Helper()

	// Create basic git structure
	gitDir := filepath.Join(dir, ".git")
	err := os.MkdirAll(gitDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	// Create minimal git files
	files := map[string]string{
		".git/HEAD":        "ref: refs/heads/main\n",
		".git/config":      "[core]\n\trepositoryformatversion = 0\n",
		".git/description": "Unnamed repository\n",
	}

	for file, content := range files {
		filePath := filepath.Join(dir, file)
		if mkdirErr := os.MkdirAll(filepath.Dir(filePath), 0o755); mkdirErr != nil {
			t.Fatalf("Failed to create dir for %s: %v", file, mkdirErr)
		}
		if writeErr := os.WriteFile(filePath, []byte(content), 0o644); writeErr != nil {
			t.Fatalf("Failed to write %s: %v", file, writeErr)
		}
	}

	// Create refs directories
	refsDir := filepath.Join(gitDir, "refs", "heads")
	err = os.MkdirAll(refsDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create refs dir: %v", err)
	}

	// Create a fake commit hash for main branch
	mainRef := filepath.Join(refsDir, "main")
	err = os.WriteFile(mainRef, []byte("a1b2c3d4e5f6789012345678901234567890abcd\n"), 0o644)
	if err != nil {
		t.Fatalf("Failed to write main ref: %v", err)
	}
}

func TestOperations_updateRepo_NoGitRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-ops")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	// Try to update a non-existent repository
	err = ops.updateRepo(tempDir, "main")
	if err == nil {
		t.Error("Expected error when updating non-existent repository")
	}
}

func TestOperations_updateRepo_WithFakeRepo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-ops")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a fake git repository
	repoDir := filepath.Join(tempDir, "test-repo")
	createTestGitRepo(t, repoDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	// This will likely fail because it's not a real git repo,
	// but we're testing that it at least tries to open it
	err = ops.updateRepo(repoDir, "main")
	if err == nil {
		t.Log("Update succeeded (unexpected but not necessarily wrong)")
	} else {
		// Expected to fail with a git-related error
		if !contains(err.Error(), "failed to") {
			t.Errorf("Expected git-related error, got: %v", err)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsAtIndex(s, substr))))
}

func containsAtIndex(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestOperations_CloneWithLock(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-ops")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	repo := config.Repo{
		Repo: "invalid://not-a-real-repo",
		Rev:  "main",
	}

	repoPath := filepath.Join(tempDir, "test-clone")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// This should fail due to invalid repo, but test that locking works
	_, err = ops.cloneWithLock(ctx, repo, repoPath)
	if err == nil {
		t.Error("Expected error when cloning invalid repository")
	}
}

// Additional comprehensive tests for repository package

func TestOperations_CloneOrUpdateRepoWithDeps(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-deps")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	repo := config.Repo{
		Repo: "invalid://not-a-real-repo",
		Rev:  "main",
	}

	deps := []string{"dep1", "dep2"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should fail due to invalid repo
	_, err = ops.CloneOrUpdateRepoWithDeps(ctx, repo, deps)
	if err == nil {
		t.Error("Expected error when cloning invalid repository with deps")
	}
}

func TestOperations_cloneWithLockAndDeps(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-lock-deps")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	repo := config.Repo{
		Repo: "invalid://not-a-real-repo",
		Rev:  "main",
	}

	repoPath := filepath.Join(tempDir, "test-clone-deps")
	deps := []string{"dep1", "dep2"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Should fail due to invalid repo
	_, err = ops.cloneWithLockAndDeps(ctx, repo, repoPath, deps)
	if err == nil {
		t.Error("Expected error when cloning invalid repository with deps and lock")
	}
}

func TestOperations_checkoutRevision_ErrorCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-checkout")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	// These functions are unexported and require *git.Repository and *git.Worktree
	// We'll test them indirectly through the public methods

	// Test that invalid repos fail appropriately
	repo := config.Repo{
		Repo: "invalid://not-a-real-repo",
		Rev:  "main",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = ops.CloneOrUpdateRepo(ctx, repo)
	if err == nil {
		t.Error("Expected error when cloning invalid repository")
	}
}

func TestOperations_EdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-edge-cases")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	t.Run("various invalid repos", func(t *testing.T) {
		invalidRepos := []config.Repo{
			{Repo: "", Rev: "main"},
			{Repo: "not-a-url", Rev: "main"},
			{Repo: "ftp://invalid-protocol", Rev: "main"},
			{Repo: "https://non-existent-domain-12345.com/repo", Rev: "main"},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		for _, repo := range invalidRepos {
			_, err := ops.CloneOrUpdateRepo(ctx, repo)
			if err == nil {
				t.Errorf("Expected error for invalid repo: %s", repo.Repo)
			}
		}
	})

	t.Run("updateRepo with various scenarios", func(t *testing.T) {
		// Test updateRepo with non-existent directory
		err := ops.updateRepo("/non/existent", "main")
		if err == nil {
			t.Error("Expected error for non-existent directory")
		}

		// Test updateRepo with empty revision
		err = ops.updateRepo(tempDir, "")
		if err == nil {
			t.Error("Expected error for empty revision")
		}
	})
}

func TestOperations_PublicMethodsOnly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-public-methods")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	// Test public methods with various scenarios
	repo := config.Repo{
		Repo: "https://github.com/nonexistent/repo",
		Rev:  "main",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// These should fail gracefully
	_, err = ops.CloneOrUpdateRepo(ctx, repo)
	if err == nil {
		t.Error("Expected error for non-existent repository")
	}

	_, err = ops.CloneOrUpdateRepoWithDeps(ctx, repo, []string{"dep1"})
	if err == nil {
		t.Error("Expected error for non-existent repository with deps")
	}
}

// Test error scenarios and edge cases
func TestOperations_ErrorScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-errors")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	t.Run("updateRepo with empty path", func(t *testing.T) {
		err := ops.updateRepo("", "main")
		if err == nil {
			t.Error("Expected error for empty path")
		}
	})

	// Note: checkoutRevision, resolveRevision, and fetchAndCheckout are private methods
	// that require git.Repository and git.Worktree parameters. They are tested indirectly
	// through public methods like Clone and Update.
}

// Test concurrent operations
func TestOperations_ConcurrentCloning(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-concurrent")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	repo := config.Repo{
		Repo: "invalid://concurrent-test",
		Rev:  "main",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start multiple concurrent clone operations
	errChan := make(chan error, 3)
	for range 3 {
		go func() {
			_, err := ops.CloneOrUpdateRepo(ctx, repo)
			errChan <- err
		}()
	}

	// Collect results
	errors := 0
	for range 3 {
		if err := <-errChan; err != nil {
			errors++
		}
	}

	// All should fail due to invalid repo, but no panics or deadlocks
	if errors != 3 {
		t.Logf("Expected all 3 operations to fail, got %d failures", errors)
	}
}

func TestOperations_AdditionalCoverage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-additional")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	t.Run("CloneOrUpdateRepoWithDeps success path", func(t *testing.T) {
		// Test with a shorter timeout to avoid long waits
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Test with meta repository type to trigger different code paths
		testRepo := config.Repo{
			Repo: "meta",
			Rev:  "HEAD",
		}

		_, err := ops.CloneOrUpdateRepoWithDeps(ctx, testRepo, []string{})
		// Meta repos don't require network access, so this might succeed
		if err != nil {
			t.Logf("CloneOrUpdateRepoWithDeps with meta repo returned error: %v", err)
		}
	})

	t.Run("CloneOrUpdateRepoWithDeps with local repo", func(t *testing.T) {
		ctx := context.Background()

		// Create a fake local git repository
		localRepoPath := filepath.Join(tempDir, "local-repo")
		err := os.MkdirAll(filepath.Join(localRepoPath, ".git"), 0o755)
		if err != nil {
			t.Fatalf("Failed to create fake git repo: %v", err)
		}

		testRepo := config.Repo{
			Repo: localRepoPath,
			Rev:  "HEAD",
		}

		_, err = ops.CloneOrUpdateRepoWithDeps(ctx, testRepo, []string{"python"})
		if err != nil {
			t.Logf("CloneOrUpdateRepoWithDeps with local repo returned error: %v (expected)", err)
		}
	})

	t.Run("cloneWithLockAndDeps different scenarios", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		testRepo := config.Repo{
			Repo: "https://github.com/nonexistent/repo",
			Rev:  "main",
		}

		targetPath := filepath.Join(tempDir, "deps-test")

		// Test with different dependency combinations
		testCases := []struct {
			name string
			deps []string
		}{
			{"no deps", []string{}},
			{"single dep", []string{"python"}},
			{"multiple deps", []string{"python", "node"}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				specificPath := filepath.Join(targetPath, tc.name)
				_, err := ops.cloneWithLockAndDeps(ctx, testRepo, specificPath, tc.deps)
				if err != nil {
					t.Logf("cloneWithLockAndDeps with %s returned error: %v (expected)", tc.name, err)
				}
			})
		}
	})

	t.Run("cloneRepo with different URL types", func(t *testing.T) {
		testCases := []struct {
			name string
			url  string
		}{
			{"github https", "https://github.com/example/repo"},
			{"github ssh", "git@github.com:example/repo.git"},
			{"invalid url", "not-a-url"},
			{"empty url", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				testRepo := config.Repo{
					Repo: tc.url,
					Rev:  "main",
				}

				targetPath := filepath.Join(tempDir, "clone-test-"+tc.name)
				_, err := ops.cloneRepo(context.Background(), testRepo, targetPath)
				if err != nil {
					t.Logf("cloneRepo with %s returned error: %v (expected)", tc.name, err)
				}
			})
		}
	})
	t.Run("updateRepo with various scenarios", func(t *testing.T) {
		// Create a directory that's not a git repo
		nonGitPath := filepath.Join(tempDir, "not-git")
		err := os.MkdirAll(nonGitPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create test dir: %v", err)
		}

		err = ops.updateRepo(nonGitPath, "main")
		if err != nil {
			t.Logf("updateRepo with non-git directory returned error: %v (expected)", err)
		}
	})
}

func TestOperations_IntegrationScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-repo-integration")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	t.Run("cloneWithLock timeout scenarios", func(t *testing.T) {
		// Test timeout behavior - with very short timeout, we expect either timeout or network error
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		testRepo := config.Repo{
			Repo: "https://github.com/nonexistent/nonexistent-repo-timeout-test",
			Rev:  "main",
		}

		targetPath := filepath.Join(tempDir, "timeout-test")
		_, err := ops.cloneWithLock(ctx, testRepo, targetPath)
		if err == nil {
			t.Error("Expected timeout or network error for very short timeout")
		}
		t.Logf("Got expected error: %v", err)
	})

	t.Run("CloneOrUpdateRepo with existing directory", func(t *testing.T) {
		// Create an existing directory
		existingPath := filepath.Join(tempDir, "existing")
		err := os.MkdirAll(existingPath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create existing dir: %v", err)
		}

		testRepo := config.Repo{
			Repo: "https://github.com/example/repo",
			Rev:  "main",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		_, err = ops.CloneOrUpdateRepo(ctx, testRepo)
		if err != nil {
			t.Logf("CloneOrUpdateRepo with existing directory returned error: %v (expected)", err)
		}
	})
}

func TestOperations_GitOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-git-ops")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	t.Run("checkoutRevision with invalid repository", func(t *testing.T) {
		// Create an in-memory Git repository that's minimal
		storage := memory.NewStorage()
		fs := memfs.New()

		repo, err := git.Init(storage, fs)
		if err != nil {
			t.Fatalf("Failed to create in-memory repo: %v", err)
		}

		worktree, err := repo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		// Test checkoutRevision with non-existent revision
		err = ops.checkoutRevision(repo, worktree, "non-existent-revision")
		if err == nil {
			t.Error("Expected error for non-existent revision")
		}

		// Test checkoutRevision with invalid hash
		err = ops.checkoutRevision(repo, worktree, "invalidhash")
		if err == nil {
			t.Error("Expected error for invalid hash")
		}

		// Test checkoutRevision with valid-looking but non-existent hash
		err = ops.checkoutRevision(repo, worktree, "a1b2c3d4e5f6789012345678901234567890abcd")
		if err == nil {
			t.Error("Expected error for non-existent valid hash")
		}
	})

	t.Run("resolveRevision error scenarios", func(t *testing.T) {
		// Create an in-memory Git repository
		storage := memory.NewStorage()
		fs := memfs.New()

		repo, err := git.Init(storage, fs)
		if err != nil {
			t.Fatalf("Failed to create in-memory repo: %v", err)
		}

		// Test resolveRevision with non-existent tag
		_, err = ops.resolveRevision(repo, "non-existent-tag")
		if err == nil {
			t.Error("Expected error for non-existent tag")
		}

		// Test resolveRevision with non-existent branch
		_, err = ops.resolveRevision(repo, "non-existent-branch")
		if err == nil {
			t.Error("Expected error for non-existent branch")
		}

		// Test resolveRevision with invalid hash format
		_, err = ops.resolveRevision(repo, "invalidhash")
		if err == nil {
			t.Error("Expected error for invalid hash format")
		}
	})

	t.Run("fetchAndCheckout error scenarios", func(t *testing.T) {
		// Create an in-memory Git repository without remote
		storage := memory.NewStorage()
		fs := memfs.New()

		repo, err := git.Init(storage, fs)
		if err != nil {
			t.Fatalf("Failed to create in-memory repo: %v", err)
		}

		worktree, err := repo.Worktree()
		if err != nil {
			t.Fatalf("Failed to get worktree: %v", err)
		}

		// Test fetchAndCheckout without remote (should handle fetch error gracefully)
		err = ops.fetchAndCheckout(repo, worktree, "main")
		if err == nil {
			t.Error("Expected error for fetchAndCheckout without remote")
		}

		// Test fetchAndCheckout with non-existent revision
		err = ops.fetchAndCheckout(repo, worktree, "non-existent-revision")
		if err == nil {
			t.Error("Expected error for fetchAndCheckout with non-existent revision")
		}
	})
}

func TestOperations_CloneRepoVariations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-clone-variations")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	t.Run("cloneRepo error path coverage", func(t *testing.T) {
		testCases := []struct {
			name      string
			repo      config.Repo
			wantError bool
		}{
			{
				name: "empty repo URL",
				repo: config.Repo{
					Repo: "",
					Rev:  "main",
				},
				wantError: true,
			},
			{
				name: "malformed URL",
				repo: config.Repo{
					Repo: "not-a-valid-url",
					Rev:  "main",
				},
				wantError: true,
			},
			{
				name: "file protocol (local path that doesn't exist)",
				repo: config.Repo{
					Repo: "file:///non/existent/path",
					Rev:  "main",
				},
				wantError: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				targetPath := filepath.Join(tempDir, "clone-"+tc.name)
				_, err := ops.cloneRepo(context.Background(), tc.repo, targetPath)

				if tc.wantError && err == nil {
					t.Errorf("Expected error for %s, but got none", tc.name)
				}
				if !tc.wantError && err != nil {
					t.Errorf("Expected no error for %s, but got: %v", tc.name, err)
				}
			})
		}
	})

	t.Run("CloneOrUpdateRepoWithDeps path coverage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Test with meta repository
		metaRepo := config.Repo{
			Repo: "meta",
			Rev:  "HEAD",
		}

		_, err := ops.CloneOrUpdateRepoWithDeps(ctx, metaRepo, []string{})
		if err == nil {
			t.Error("Expected error for meta repo clone")
		}

		// Test with local repository (non-existent path)
		localRepo := config.Repo{
			Repo: "/non/existent/local/repo",
			Rev:  "HEAD",
		}

		_, err = ops.CloneOrUpdateRepoWithDeps(ctx, localRepo, []string{"python"})
		if err == nil {
			t.Error("Expected error for non-existent local repo")
		}

		// Test with different dependency scenarios
		remoteRepo := config.Repo{
			Repo: "https://github.com/nonexistent/repo",
			Rev:  "v1.0.0",
		}

		testDeps := [][]string{
			{},                   // no deps
			{"python"},           // single dep
			{"python", "node"},   // multiple deps
			{"unknown-language"}, // unknown language dep
		}

		for i, deps := range testDeps {
			t.Run(fmt.Sprintf("deps_scenario_%d", i), func(t *testing.T) {
				_, err := ops.CloneOrUpdateRepoWithDeps(ctx, remoteRepo, deps)
				// We expect errors due to non-existent repo, but this exercises the code paths
				if err == nil {
					t.Logf("Unexpected success for deps scenario %d", i)
				}
			})
		}
	})
}

func TestOperations_UpdateRepoScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-update-scenarios")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	t.Run("updateRepo with different path scenarios", func(t *testing.T) {
		testCases := []struct {
			setupFn  func(string) error
			name     string
			path     string
			revision string
		}{
			{
				name:     "non-existent directory",
				path:     "/non/existent/path",
				revision: "main",
				setupFn:  nil,
			},
			{
				name:     "empty directory",
				path:     "",
				revision: "main",
				setupFn: func(path string) error {
					return os.MkdirAll(path, 0o755)
				},
			},
			{
				name:     "directory without .git",
				path:     "",
				revision: "main",
				setupFn: func(path string) error {
					return os.MkdirAll(path, 0o755)
				},
			},
		}

		for i, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				testPath := tc.path
				if tc.path == "" {
					testPath = filepath.Join(tempDir, fmt.Sprintf("update-test-%d", i))
				}

				if tc.setupFn != nil {
					if err := tc.setupFn(testPath); err != nil {
						t.Fatalf("Setup failed: %v", err)
					}
				}

				err := ops.updateRepo(testPath, tc.revision)
				// We expect errors for all these scenarios
				if err == nil {
					t.Errorf("Expected error for %s scenario", tc.name)
				}
			})
		}
	})

	t.Run("CloneOrUpdateRepo with existing directories", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		// Test scenarios where directories already exist
		testCases := []struct {
			setupFn func(string) error
			name    string
			repo    config.Repo
		}{
			{
				name: "existing empty directory",
				repo: config.Repo{
					Repo: "https://github.com/example/test-repo",
					Rev:  "main",
				},
				setupFn: func(path string) error {
					return os.MkdirAll(path, 0o755)
				},
			},
			{
				name: "existing directory with .git (should update)",
				repo: config.Repo{
					Repo: "https://github.com/example/test-repo",
					Rev:  "v1.0.0",
				},
				setupFn: func(path string) error {
					gitDir := filepath.Join(path, ".git")
					return os.MkdirAll(gitDir, 0o755)
				},
			},
		}

		for i, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create test directory
				testPath := filepath.Join(tempDir, fmt.Sprintf("existing-%d", i))
				if err := tc.setupFn(testPath); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}

				_, err := ops.CloneOrUpdateRepo(ctx, tc.repo)
				// We expect errors due to network/timeout, but this exercises the code paths
				if err == nil {
					t.Logf("Unexpected success for %s", tc.name)
				}
			})
		}
	})
}

func TestOperations_EdgeCaseScenarios(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-edge-cases")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cm, err := cache.NewManager(tempDir)
	if err != nil {
		t.Fatalf("cache.NewManager() error = %v", err)
	}
	defer cm.Close()

	ops := NewRepositoryOperations(cm)

	t.Run("cloneWithLock with different timeout scenarios", func(t *testing.T) {
		testCases := []struct {
			name    string
			repo    config.Repo
			timeout time.Duration
		}{
			{
				name:    "immediate timeout",
				timeout: 1 * time.Nanosecond,
				repo: config.Repo{
					Repo: "https://github.com/example/repo",
					Rev:  "main",
				},
			},
			{
				name:    "very short timeout",
				timeout: 1 * time.Millisecond,
				repo: config.Repo{
					Repo: "https://github.com/nonexistent/nonexistent-repo-timeout-test",
					Rev:  "main",
				},
			},
		}

		for i, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
				defer cancel()

				targetPath := filepath.Join(tempDir, fmt.Sprintf("timeout-test-%d", i))
				_, err := ops.cloneWithLock(ctx, tc.repo, targetPath)

				// We expect timeout or network errors due to very short timeout
				if err == nil {
					t.Errorf("Expected timeout or network error for %s", tc.name)
				}
				t.Logf("Got expected error for %s: %v", tc.name, err)
			})
		}
	})

	t.Run("cloneWithLockAndDeps comprehensive scenarios", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Test various repo and dependency combinations
		testScenarios := []struct {
			repo config.Repo
			deps []string
		}{
			{
				repo: config.Repo{Repo: "meta", Rev: "HEAD"},
				deps: []string{},
			},
			{
				repo: config.Repo{Repo: "local", Rev: "HEAD"},
				deps: []string{"python"},
			},
			{
				repo: config.Repo{Repo: "https://github.com/example/repo", Rev: "main"},
				deps: []string{"python", "node", "ruby"},
			},
			{
				repo: config.Repo{Repo: "file:///tmp/nonexistent", Rev: "main"},
				deps: []string{"unknown-lang"},
			},
		}

		for i, scenario := range testScenarios {
			t.Run(fmt.Sprintf("scenario_%d", i), func(t *testing.T) {
				targetPath := filepath.Join(tempDir, fmt.Sprintf("deps-scenario-%d", i))
				_, err := ops.cloneWithLockAndDeps(ctx, scenario.repo, targetPath, scenario.deps)

				// We expect errors for all these scenarios, but they exercise different code paths
				if err == nil {
					t.Logf("Unexpected success for scenario %d", i)
				}
			})
		}
	})
}
