package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5"
)

// Test constants
const (
	testFileName    = "file1.txt"
	modifiedContent = "modified content"
)

// Shared test fixture to avoid repeated git repository creation
var (
	testRepoOnce sync.Once
	testRepoDir  string
	errTestRepo  error
)

// getSharedTestRepo returns a shared test repository for read-only operations
func getSharedTestRepo(t *testing.T) string {
	t.Helper()
	testRepoOnce.Do(func() {
		testRepoDir, errTestRepo = createTestRepo()
	})
	if errTestRepo != nil {
		t.Fatalf("Failed to create shared test repo: %v", errTestRepo)
	}
	return testRepoDir
}

// createTestRepo creates a test git repository (called once)
func createTestRepo() (string, error) {
	tempDir, err := os.MkdirTemp("", "git-test-shared-*")
	if err != nil {
		return "", err
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		if exec.Command("git", "--version").Run() != nil {
			return "", errors.New("git not available")
		}
		return "", err
	}

	// Configure git user for the test (batch commands)
	commands := [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "commit.gpgsign", "false"},
	}

	for _, cmdArgs := range commands {
		gitCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		gitCmd.Dir = tempDir
		if err := gitCmd.Run(); err != nil {
			return "", err
		}
	}

	// Create test files
	testFiles := map[string]string{
		"file1.txt":     "content1",
		"file2.txt":     "content2",
		"dir/file3.txt": "content3",
	}

	for file, content := range testFiles {
		filePath := filepath.Join(tempDir, file)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			return "", err
		}
	}

	// Add and commit files
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return "", err
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return tempDir, nil
}

func TestFindGitRoot(t *testing.T) {
	t.Parallel() // This test can run in parallel

	// Create a temporary directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir", "deep")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create .git directory in the root
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	tests := []struct {
		name      string
		path      string
		expected  string
		expectErr bool
	}{
		{
			name:      "find root from root directory",
			path:      tempDir,
			expected:  tempDir,
			expectErr: false,
		},
		{
			name:      "find root from subdirectory",
			path:      subDir,
			expected:  tempDir,
			expectErr: false,
		},
		{
			name:      "empty path uses current directory",
			path:      "",
			expected:  "",
			expectErr: false, // Will use current directory, which might be a git repo
		},
		{
			name:      "non-git directory fails",
			path:      t.TempDir(),
			expected:  "",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root, err := FindGitRoot(tt.path)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.expected != "" && root != tt.expected {
					t.Errorf("Expected root %s, got %s", tt.expected, root)
				}
			}
		})
	}
}

func TestFindGitRootWithGitFile(t *testing.T) {
	t.Parallel() // This test can run in parallel

	// Test worktree scenario where .git is a file
	tempDir := t.TempDir()
	gitFile := filepath.Join(tempDir, ".git")

	// Create .git file (like in git worktrees)
	gitContent := "gitdir: /some/other/path/.git/worktrees/branch"
	if err := os.WriteFile(gitFile, []byte(gitContent), 0o644); err != nil {
		t.Fatalf("Failed to create .git file: %v", err)
	}

	root, err := FindGitRoot(tempDir)
	if err != nil {
		t.Errorf("Unexpected error with .git file: %v", err)
	}
	if root != tempDir {
		t.Errorf("Expected root %s, got %s", tempDir, root)
	}
}

func TestIsInRepository(t *testing.T) {
	t.Parallel() // This test can run in parallel

	// Test with non-git directory
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	os.Chdir(tempDir)
	if IsInRepository() {
		t.Error("Expected false for non-git directory")
	}

	// Create .git directory
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	if !IsInRepository() {
		t.Error("Expected true for git directory")
	}
}

// setupTestRepo creates a test git repository with some files
// Use this for tests that need to modify the repository
func setupTestRepo(t *testing.T) string {
	t.Helper()
	tempDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		if exec.Command("git", "--version").Run() != nil {
			t.Skip("Git not available, skipping git integration tests")
		}
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	// Batch configure git user for the test
	commands := [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "commit.gpgsign", "false"},
	}

	for _, cmdArgs := range commands {
		configCmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		configCmd.Dir = tempDir
		if err := configCmd.Run(); err != nil {
			t.Fatalf("Failed to configure git: %v", err)
		}
	}

	// Create some test files
	testFiles := map[string]string{
		"file1.txt":     "content1",
		"file2.txt":     "content2",
		"dir/file3.txt": "content3",
	}

	for file, content := range testFiles {
		filePath := filepath.Join(tempDir, file)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Add and commit files
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit files: %v", err)
	}

	return tempDir
}

// Helper function to run git commands
func runGitCmd(t *testing.T, repoDir string, args ...string) error {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	return cmd.Run()
}

func TestNewRepository(t *testing.T) {
	t.Parallel() // This test can run in parallel

	// Test with non-git directory
	tempDir := t.TempDir()
	_, err := NewRepository(tempDir)
	if err == nil {
		t.Error("Expected error for non-git directory")
	}

	// Test with git repository using shared test repo
	repoDir := getSharedTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Errorf("Unexpected error with git repository: %v", err)
	}
	if repo == nil {
		t.Error("Expected non-nil repository")
		return
	}
	if repo.Root != repoDir {
		t.Errorf("Expected root %s, got %s", repoDir, repo.Root)
	}
}

func TestRepository_GetAllFiles(t *testing.T) {
	t.Parallel() // This test can run in parallel

	repoDir := getSharedTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	files, err := repo.GetAllFiles()
	if err != nil {
		t.Errorf("Unexpected error getting all files: %v", err)
	}

	expectedFiles := []string{"file1.txt", "file2.txt", "dir/file3.txt"}
	if len(files) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(files))
	}

	fileSet := make(map[string]bool)
	for _, file := range files {
		fileSet[file] = true
	}

	for _, expected := range expectedFiles {
		if !fileSet[expected] {
			t.Errorf("Expected file %s not found in results", expected)
		}
	}
}

func TestRepository_GetCurrentBranch(t *testing.T) {
	t.Parallel() // This test can run in parallel

	repoDir := getSharedTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	branch, err := repo.GetCurrentBranch()
	if err != nil {
		t.Errorf("Unexpected error getting current branch: %v", err)
	}

	// Git might use "main" or "master" as default branch
	if branch != "main" && branch != "master" {
		t.Errorf("Expected 'main' or 'master', got %s", branch)
	}
}

// Test for detached HEAD scenario
func TestRepository_GetCurrentBranch_DetachedHEAD(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Get the HEAD commit hash to checkout to it directly (detached HEAD)
	gitRepo, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("Failed to open git repo: %v", err)
	}

	head, err := gitRepo.Head()
	if err != nil {
		t.Fatalf("Failed to get HEAD: %v", err)
	}

	// Checkout to the commit hash directly (creates detached HEAD)
	worktree, err := gitRepo.Worktree()
	if err != nil {
		t.Fatalf("Failed to get worktree: %v", err)
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: head.Hash(),
	})
	if err != nil {
		t.Fatalf("Failed to checkout to detached HEAD: %v", err)
	}

	// Now GetCurrentBranch should return an error
	_, err = repo.GetCurrentBranch()
	if err == nil {
		t.Error("Expected error for detached HEAD state")
	}
	if !strings.Contains(err.Error(), "HEAD is not pointing to a branch") {
		t.Errorf("Expected error about detached HEAD, got: %v", err)
	}
}

func TestRepository_GetModifiedFiles(t *testing.T) {
	testFileModificationHelper(t, func(repo *Repository) ([]string, error) {
		return repo.GetModifiedFiles()
	}, "modified")
}

func TestRepository_CheckFileModifications(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with no modifications
	modified, err := repo.CheckFileModifications([]string{"file1.txt"})
	if err != nil {
		t.Errorf("Unexpected error checking modifications: %v", err)
	}
	if modified {
		t.Error("Expected no modifications")
	}

	// Modify a file
	modifiedFile := filepath.Join(repoDir, "file1.txt")
	if writeErr := os.WriteFile(modifiedFile, []byte("modified content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	// Test with modifications
	modified, err = repo.CheckFileModifications([]string{"file1.txt"})
	if err != nil {
		t.Errorf("Unexpected error checking modifications: %v", err)
	}
	if !modified {
		t.Error("Expected modifications detected")
	}

	// Test with empty file list
	modified, err = repo.CheckFileModifications([]string{})
	if err != nil {
		t.Errorf("Unexpected error with empty file list: %v", err)
	}
	if modified {
		t.Error("Expected no modifications for empty list")
	}
}

func TestRepository_GetDiffOutput(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with no changes
	diff, err := repo.GetDiffOutput([]string{"file1.txt"})
	if err != nil {
		t.Errorf("Unexpected error getting diff: %v", err)
	}
	if diff != "No differences detected" {
		t.Errorf("Expected 'No differences detected', got %s", diff)
	}

	// Modify a file
	modifiedFile := filepath.Join(repoDir, "file1.txt")
	if writeErr := os.WriteFile(modifiedFile, []byte("modified content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	// Test with changes
	diff, err = repo.GetDiffOutput([]string{"file1.txt"})
	if err != nil {
		t.Errorf("Unexpected error getting diff: %v", err)
	}
	if !strings.Contains(diff, "file1.txt") {
		t.Error("Expected diff to contain filename")
	}
	if !strings.Contains(diff, "modified content") {
		t.Error("Expected diff to contain new content")
	}

	// Test with empty file list
	diff, err = repo.GetDiffOutput([]string{})
	if err != nil {
		t.Errorf("Unexpected error with empty file list: %v", err)
	}
	if diff != "" {
		t.Errorf("Expected empty diff for empty file list, got %s", diff)
	}
}

func TestRepository_InstallUninstallHook(t *testing.T) {
	repoDir := setupTestRepo(t)
	t.Logf("Test repo created at: %s", repoDir)

	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	t.Logf("Repository root: %s", repo.Root)

	hookName := "pre-commit"
	hookScript := "#!/bin/bash\necho 'test hook'"

	// Check what's in the hooks directory initially
	hooksDir := filepath.Join(repo.Root, ".git", "hooks")
	t.Logf("Hooks directory: %s", hooksDir)
	if entries, readErr := os.ReadDir(hooksDir); readErr == nil {
		t.Logf("Existing hooks:")
		for _, entry := range entries {
			t.Logf("  - %s", entry.Name())
			// Check if it's the pre-commit hook and see its content
			if entry.Name() == "pre-commit" {
				hookPath := filepath.Join(hooksDir, entry.Name())
				if content, contentErr := os.ReadFile(hookPath); contentErr == nil {
					contentStr := string(content)
					if len(contentStr) > 100 {
						contentStr = contentStr[:100]
					}
					t.Logf("    Content: %s", contentStr)
				}
			}
		}
	} else {
		t.Logf("Failed to read hooks directory: %v", readErr)
	}

	// Initially hook should not exist (or should be a sample hook)
	// Remove any existing sample hooks first
	sampleHookPath := filepath.Join(repo.Root, ".git", "hooks", hookName)
	if _, statErr := os.Stat(sampleHookPath); statErr == nil {
		t.Logf("Removing existing hook at %s", sampleHookPath)
		os.Remove(sampleHookPath)
	}

	if repo.HasHook(hookName) {
		t.Error("Hook should not exist initially")
	}

	// Install hook
	if installErr := repo.InstallHook(hookName, hookScript); installErr != nil {
		t.Errorf("Failed to install hook: %v", installErr)
	}

	// Hook should now exist
	if !repo.HasHook(hookName) {
		t.Error("Hook should exist after installation")
	}

	// Check hook content
	hookPath := filepath.Join(repoDir, ".git", "hooks", hookName)
	content, err := os.ReadFile(hookPath)
	if err != nil {
		t.Errorf("Failed to read hook file: %v", err)
	}
	if string(content) != hookScript {
		t.Errorf("Hook content mismatch. Expected %s, got %s", hookScript, string(content))
	}

	// Check hook is executable
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Errorf("Failed to stat hook file: %v", err)
	}
	if info.Mode()&0o100 == 0 {
		t.Error("Hook file should be executable")
	}

	// Uninstall hook
	if err := repo.UninstallHook(hookName); err != nil {
		t.Errorf("Failed to uninstall hook: %v", err)
	}

	// Hook should no longer exist
	if repo.HasHook(hookName) {
		t.Error("Hook should not exist after uninstallation")
	}
}

func TestRepository_GetRemoteURL(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with non-existent remote
	_, err = repo.GetRemoteURL("origin")
	if err == nil {
		t.Error("Expected error for non-existent remote")
	}

	// Add a remote
	remoteURL := "https://github.com/user/repo.git"
	cmd := exec.Command("git", "remote", "add", "origin", remoteURL)
	cmd.Dir = repoDir
	if cmdErr := cmd.Run(); cmdErr != nil {
		t.Fatalf("Failed to add remote: %v", cmdErr)
	}

	// Test getting remote URL
	url, err := repo.GetRemoteURL("origin")
	if err != nil {
		t.Errorf("Unexpected error getting remote URL: %v", err)
	}
	if url != remoteURL {
		t.Errorf("Expected URL %s, got %s", remoteURL, url)
	}
}

func TestRepository_GetStagedFiles(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Initially no staged files
	files, err := repo.GetStagedFiles()
	if err != nil {
		t.Errorf("Unexpected error getting staged files: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 staged files, got %d", len(files))
	}

	// Create and stage a new file
	newFile := filepath.Join(repoDir, "new_file.txt")
	if writeErr := os.WriteFile(newFile, []byte("new content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create new file: %v", writeErr)
	}
	cmd := exec.Command("git", "add", "new_file.txt")
	cmd.Dir = repoDir
	if cmdErr := cmd.Run(); cmdErr != nil {
		t.Fatalf("Failed to stage file: %v", cmdErr)
	}

	files, err = repo.GetStagedFiles()
	if err != nil {
		t.Errorf("Unexpected error getting staged files: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 staged file, got %d", len(files))
	}
	if len(files) > 0 && files[0] != "new_file.txt" {
		t.Errorf("Expected 'new_file.txt', got %s", files[0])
	}
}

func TestRepository_GetUnstagedFiles(t *testing.T) {
	testFileModificationHelper(t, func(repo *Repository) ([]string, error) {
		return repo.GetUnstagedFiles()
	}, "unstaged")
}

func TestRepository_GetCommitFiles(t *testing.T) {
	t.Parallel() // This test can run in parallel

	repoDir := getSharedTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test getting files from HEAD commit
	files, err := repo.GetCommitFiles("HEAD")
	if err != nil {
		t.Errorf("Unexpected error getting commit files: %v", err)
	}

	expectedFiles := []string{"file1.txt", "file2.txt", "dir/file3.txt"}
	if len(files) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(files))
	}

	// Test with invalid commit
	_, err = repo.GetCommitFiles("invalid-commit")
	if err == nil {
		t.Error("Expected error for invalid commit")
	}
}

func TestRepository_GetCommitFiles_ExtendedCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with HEAD~1 (should fail since we only have one commit)
	_, err = repo.GetCommitFiles("HEAD~1")
	if err == nil {
		t.Error("Expected error for non-existent commit")
	}

	// Test with invalid commit reference
	_, err = repo.GetCommitFiles("invalid-commit-hash")
	if err == nil {
		t.Error("Expected error for invalid commit reference")
	}

	// Create another commit and test
	newFile := filepath.Join(repoDir, "newfile.txt")
	if writeErr := os.WriteFile(newFile, []byte("new content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create new file: %v", writeErr)
	}

	if addErr := runGitCmd(t, repoDir, "add", "newfile.txt"); addErr != nil {
		t.Fatalf("Failed to add new file: %v", addErr)
	}
	if commitErr := runGitCmd(t, repoDir, "commit", "-m", "Add new file"); commitErr != nil {
		t.Fatalf("Failed to commit new file: %v", commitErr)
	}

	// Test getting files from the new commit
	files, err := repo.GetCommitFiles("HEAD")
	if err != nil {
		t.Errorf("Unexpected error getting commit files: %v", err)
	}

	// Should contain the new file
	found := slices.Contains(files, "newfile.txt")
	if !found {
		t.Error("Expected new file to be in commit files")
	}
}

func TestRepository_NilRepository(t *testing.T) {
	t.Parallel() // This test can run in parallel

	repo := &Repository{repo: nil}

	// Test methods that should return errors with nil repo
	_, err := repo.GetStagedFiles()
	if err == nil {
		t.Error("Expected error for nil repository in GetStagedFiles")
	}

	_, err = repo.GetUnstagedFiles()
	if err == nil {
		t.Error("Expected error for nil repository in GetUnstagedFiles")
	}

	_, err = repo.GetModifiedFiles()
	if err == nil {
		t.Error("Expected error for nil repository in GetModifiedFiles")
	}

	_, err = repo.CheckFileModifications([]string{"test.txt"})
	if err == nil {
		t.Error("Expected error for nil repository in CheckFileModifications")
	}

	_, err = repo.GetDiffOutput([]string{"test.txt"})
	if err == nil {
		t.Error("Expected error for nil repository in GetDiffOutput")
	}
}

func TestRepository_GenerateSimpleDiff(t *testing.T) {
	t.Parallel() // This test can run in parallel

	repo := &Repository{}

	tests := []struct {
		name          string
		file          string
		oldContent    string
		newContent    string
		shouldContain []string
	}{
		{
			name:          "simple modification",
			file:          "test.txt",
			oldContent:    "line1\nline2",
			newContent:    "line1\nmodified line2",
			shouldContain: []string{"test.txt", "-line2", "+modified line2"},
		},
		{
			name:          "file deletion",
			file:          "deleted.txt",
			oldContent:    "content",
			newContent:    "",
			shouldContain: []string{"deleted.txt", "-content"},
		},
		{
			name:          "file addition",
			file:          "new.txt",
			oldContent:    "",
			newContent:    "new content",
			shouldContain: []string{"new.txt", "+new content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			diff := repo.generateSimpleDiff(tt.file, tt.oldContent, tt.newContent)

			for _, expected := range tt.shouldContain {
				if !strings.Contains(diff, expected) {
					t.Errorf(
						"Expected diff to contain %q, but it didn't. Diff:\n%s",
						expected,
						diff,
					)
				}
			}
		})
	}
}

// testFileModificationHelper is a helper function for testing file modification scenarios
func testFileModificationHelper(
	t *testing.T,
	getFiles func(*Repository) ([]string, error),
	testName string,
) {
	t.Helper()

	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Initially no modified/unstaged files
	files, err := getFiles(repo)
	if err != nil {
		t.Errorf("Unexpected error getting %s files: %v", testName, err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 %s files, got %d", testName, len(files))
	}

	// Modify a file
	modifiedFile := filepath.Join(repoDir, testFileName)
	if writeErr := os.WriteFile(modifiedFile, []byte(modifiedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	files, err = getFiles(repo)
	if err != nil {
		t.Errorf("Unexpected error getting %s files: %v", testName, err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 %s file, got %d", testName, len(files))
	}
	if len(files) > 0 && files[0] != testFileName {
		t.Errorf("Expected '%s', got %s", testFileName, files[0])
	}
}

func TestRepository_GetChangedFiles(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Create and commit initial file
	file1 := filepath.Join(repoDir, "file1.txt")
	if writeErr := os.WriteFile(file1, []byte("initial content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create file1: %v", writeErr)
	}

	// Add and commit
	cmd := exec.Command("git", "add", "file1.txt")
	cmd.Dir = repoDir
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("Failed to add file1: %v", runErr)
	}
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoDir
	if commitErr := cmd.Run(); commitErr != nil {
		t.Fatalf("Failed to commit: %v", commitErr)
	}

	// Get the initial commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get HEAD hash: %v", err)
	}
	initialCommit := strings.TrimSpace(string(output))

	// Modify and commit another file
	file2 := filepath.Join(repoDir, "file2.txt")
	if writeErr := os.WriteFile(file2, []byte("second file"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create file2: %v", writeErr)
	}

	cmd = exec.Command("git", "add", "file2.txt")
	cmd.Dir = repoDir
	if addErr := cmd.Run(); addErr != nil {
		t.Fatalf("Failed to add file2: %v", addErr)
	}
	cmd = exec.Command("git", "commit", "-m", "Add second file")
	cmd.Dir = repoDir
	if commitErr := cmd.Run(); commitErr != nil {
		t.Fatalf("Failed to commit: %v", commitErr)
	}

	// Test getting changed files between commits
	files, err := repo.GetChangedFiles(initialCommit, "HEAD")
	if err != nil {
		t.Errorf("Unexpected error getting changed files: %v", err)
	}

	expected := []string{"file2.txt"}
	if len(files) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(files))
	}

	for _, expectedFile := range expected {
		found := slices.Contains(files, expectedFile)
		if !found {
			t.Errorf("Expected file %s not found in changed files", expectedFile)
		}
	}
}

func TestRepository_GetChangedFiles_InvalidRef(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with invalid reference
	_, err = repo.GetChangedFiles("invalid-ref", "HEAD")
	if err == nil {
		t.Error("Expected error with invalid reference")
	}

	// Test with missing HEAD (empty repo)
	_, err = repo.GetChangedFiles("HEAD", "HEAD~1")
	if err == nil {
		t.Error("Expected error with non-existent commits")
	}
}

func TestRepository_GetPushFiles(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Create and commit initial file
	file1 := filepath.Join(repoDir, "file1.txt")
	if writeErr := os.WriteFile(file1, []byte("initial content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create file1: %v", writeErr)
	}

	if addErr := runGitCmd(t, repoDir, "add", "file1.txt"); addErr != nil {
		t.Fatalf("Failed to add file1: %v", addErr)
	}
	if commitErr := runGitCmd(t, repoDir, "commit", "-m", "Initial commit"); commitErr != nil {
		t.Fatalf("Failed to commit: %v", commitErr)
	}

	// Create a "remote" branch by creating another commit and resetting
	file2 := filepath.Join(repoDir, "file2.txt")
	if writeErr := os.WriteFile(file2, []byte("remote content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create file2: %v", writeErr)
	}

	if addErr := runGitCmd(t, repoDir, "add", "file2.txt"); addErr != nil {
		t.Fatalf("Failed to add file2: %v", addErr)
	}
	if commitErr := runGitCmd(t, repoDir, "commit", "-m", "Remote commit"); commitErr != nil {
		t.Fatalf("Failed to commit: %v", commitErr)
	}

	// Create a remote branch reference
	if branchErr := runGitCmd(t, repoDir, "branch", "origin/main"); branchErr != nil {
		t.Fatalf("Failed to create branch: %v", branchErr)
	}

	// Reset to previous commit to simulate local being behind
	if resetErr := runGitCmd(t, repoDir, "reset", "--hard", "HEAD~1"); resetErr != nil {
		t.Fatalf("Failed to reset: %v", resetErr)
	}

	// Now create new local changes
	file3 := filepath.Join(repoDir, "file3.txt")
	if writeErr := os.WriteFile(file3, []byte("local content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create file3: %v", writeErr)
	}

	if addErr := runGitCmd(t, repoDir, "add", "file3.txt"); addErr != nil {
		t.Fatalf("Failed to add file3: %v", addErr)
	}
	if commitErr := runGitCmd(t, repoDir, "commit", "-m", "Local commit"); commitErr != nil {
		t.Fatalf("Failed to commit: %v", commitErr)
	}

	// Test getting push files
	files, err := repo.GetPushFiles("HEAD", "origin/main")
	if err != nil {
		t.Errorf("Unexpected error getting push files: %v", err)
	}

	// Should contain the file that differs between local and remote
	if len(files) == 0 {
		t.Error("Expected some files to be different between branches")
	}
}

func TestRepository_GetPushFiles_NoRemote(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Create and commit a file
	file1 := filepath.Join(repoDir, "file1.txt")
	if writeErr := os.WriteFile(file1, []byte("content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}

	if addErr := runGitCmd(t, repoDir, "add", "file1.txt"); addErr != nil {
		t.Fatalf("Failed to add file: %v", addErr)
	}
	if commitErr := runGitCmd(t, repoDir, "commit", "-m", "Commit"); commitErr != nil {
		t.Fatalf("Failed to commit: %v", commitErr)
	}

	// Test with non-existent remote - should return all files
	files, err := repo.GetPushFiles("HEAD", "origin/nonexistent")
	if err != nil {
		t.Errorf("Unexpected error with non-existent remote: %v", err)
	}

	// Should return all files since remote doesn't exist
	if len(files) == 0 {
		t.Error("Expected files to be returned when remote doesn't exist")
	}
}

func TestRepository_HasUnmergedFiles(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Initially no unmerged files
	hasUnmerged := repo.HasUnmergedFiles()
	if hasUnmerged {
		t.Error("Expected no unmerged files initially")
	}

	// Create a conflict scenario would be complex, so we test the basic case
	// The function should handle cases where worktree/status operations fail gracefully
}

func TestRepository_HasUnstagedChangesForFile(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Create and commit a file
	fileName := "test_file.txt"
	filePath := filepath.Join(repoDir, fileName)
	if err := os.WriteFile(filePath, []byte("original content"), 0o644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	if err := runGitCmd(t, repoDir, "add", fileName); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := runGitCmd(t, repoDir, "commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Initially no unstaged changes for the file
	hasChanges := repo.HasUnstagedChangesForFile(fileName)
	if hasChanges {
		t.Error("Expected no unstaged changes initially")
	}

	// Modify the file
	if err := os.WriteFile(filePath, []byte("modified content"), 0o644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}

	// Should now have unstaged changes
	hasChanges = repo.HasUnstagedChangesForFile(fileName)
	if !hasChanges {
		t.Error("Expected unstaged changes after modification")
	}

	// Test with non-existent file
	hasChanges = repo.HasUnstagedChangesForFile("nonexistent.txt")
	if hasChanges {
		t.Error("Expected no changes for non-existent file")
	}
}

func TestRepository_GetStagedFileContent(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Create a file and stage it
	fileName := "staged_file.txt"
	filePath := filepath.Join(repoDir, fileName)
	originalContent := "staged content"
	if writeErr := os.WriteFile(filePath, []byte(originalContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to create file: %v", writeErr)
	}

	if addErr := runGitCmd(t, repoDir, "add", fileName); addErr != nil {
		t.Fatalf("Failed to stage file: %v", addErr)
	}

	// Get staged content
	content, err := repo.GetStagedFileContent(fileName)
	if err != nil {
		t.Errorf("Unexpected error getting staged content: %v", err)
	}

	if string(content) != originalContent {
		t.Errorf("Expected staged content %q, got %q", originalContent, string(content))
	}

	// Test with non-existent file
	_, err = repo.GetStagedFileContent("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestRepository_InstallHook_EdgeCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test installing hook with empty script
	err = repo.InstallHook("pre-push", "")
	if err != nil {
		t.Errorf("Unexpected error installing hook with empty script: %v", err)
	}

	// Verify hook was installed
	if !repo.HasHook("pre-push") {
		t.Error("Expected hook to be installed")
	}

	// Install hook with newlines and special characters
	complexScript := "#!/bin/bash\necho 'Complex script with special chars: $@'\nexit 0"
	err = repo.InstallHook("pre-receive", complexScript)
	if err != nil {
		t.Errorf("Unexpected error installing complex hook: %v", err)
	}

	// Verify complex hook was installed
	if !repo.HasHook("pre-receive") {
		t.Error("Expected complex hook to be installed")
	}

	// Install to non-existent hooks directory by creating invalid repo structure
	invalidRepo := &Repository{
		Root: "/non/existent/path",
		repo: repo.repo, // Keep valid repo object but invalid path
	}
	err = invalidRepo.InstallHook("test-hook", "#!/bin/bash")
	if err == nil {
		t.Error("Expected error installing hook to non-existent directory")
	}
}

func TestRepository_UninstallHook_EdgeCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Uninstalling non-existent hook
	err = repo.UninstallHook("non-existent-hook")
	if err != nil {
		t.Errorf("Unexpected error uninstalling non-existent hook: %v", err)
	}

	// Install a hook first
	err = repo.InstallHook("test-hook", "#!/bin/bash\necho 'test'")
	if err != nil {
		t.Fatalf("Failed to install test hook: %v", err)
	}

	// Verify it exists
	if !repo.HasHook("test-hook") {
		t.Fatal("Expected test hook to be installed")
	}

	// Uninstall it
	err = repo.UninstallHook("test-hook")
	if err != nil {
		t.Errorf("Unexpected error uninstalling hook: %v", err)
	}

	// Verify it's gone
	if repo.HasHook("test-hook") {
		t.Error("Expected test hook to be uninstalled")
	}

	// Test uninstalling from non-existent hooks directory
	// Note: UninstallHook gracefully handles non-existent files/directories
	invalidRepo := &Repository{
		Root: "/non/existent/path",
		repo: repo.repo,
	}
	err = invalidRepo.UninstallHook("test-hook")
	if err != nil {
		t.Errorf("Expected UninstallHook to handle non-existent paths gracefully, got: %v", err)
	}
}

func TestRepository_StashUnstagedChanges_ErrorCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Stashing when there are no unstaged changes
	_, err = repo.StashUnstagedChanges(t.TempDir())
	if err == nil || !errors.Is(err, ErrNoUnstagedChanges) {
		t.Error("Expected ErrNoUnstagedChanges when no unstaged changes exist")
	}

	// Create unstaged changes
	testFile := filepath.Join(repoDir, "file1.txt")
	if writeErr := os.WriteFile(testFile, []byte("modified content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	// Stashing with invalid cache directory
	_, err = repo.StashUnstagedChanges("/invalid/path/that/does/not/exist")
	if err == nil {
		t.Error("Expected error with invalid cache directory")
	}

	// Successful stashing
	cacheDir := t.TempDir()
	stash, err := repo.StashUnstagedChanges(cacheDir)
	if err != nil {
		t.Errorf("Unexpected error stashing changes: %v", err)
	}
	if stash == nil {
		t.Error("Expected non-nil stash info")
		return
	}
	if len(stash.Files) == 0 {
		t.Error("Expected stash to contain files")
	}
}

func TestRepository_HasUnmergedFiles_EdgeCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Normal repository (should have no unmerged files)
	hasUnmerged := repo.HasUnmergedFiles()
	if hasUnmerged {
		t.Error("Expected no unmerged files in normal repository")
	}

	// Create a scenario that simulates merge conflicts
	// Modify a file and stage it
	testFile := filepath.Join(repoDir, "file1.txt")
	if err := os.WriteFile(testFile, []byte("staged content"), 0o644); err != nil {
		t.Fatalf("Failed to modify file: %v", err)
	}
	if err := runGitCmd(t, repoDir, "add", "file1.txt"); err != nil {
		t.Fatalf("Failed to stage file: %v", err)
	}

	// Then modify it again without staging
	if err := os.WriteFile(testFile, []byte("unstaged content"), 0o644); err != nil {
		t.Fatalf("Failed to modify file again: %v", err)
	}

	// This should create a file that's both staged and has working tree changes
	// The function checks for this condition as a proxy for unmerged files
	_ = repo.HasUnmergedFiles()
	// Note: This may or may not detect unmerged files depending on go-git's behavior
	// The test is mainly to ensure the function doesn't crash
}

func TestRepository_GetStagedFileContent_ErrorCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test that unstaged files can still be read (since the function reads from worktree)
	testFile := filepath.Join(repoDir, "unstaged.txt")
	expectedContent := "unstaged content"
	if writeErr := os.WriteFile(testFile, []byte(expectedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to create unstaged file: %v", writeErr)
	}

	content, err := repo.GetStagedFileContent("unstaged.txt")
	if err != nil {
		t.Errorf("Unexpected error for unstaged file: %v", err)
	}
	if string(content) != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, string(content))
	}

	// Test error case: file that doesn't exist at all
	_, err = repo.GetStagedFileContent("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}

	// Test error case: file that has been deleted from worktree
	if removeErr := os.Remove(filepath.Join(repoDir, "file1.txt")); removeErr != nil {
		t.Fatalf("Failed to delete file: %v", removeErr)
	}

	_, err = repo.GetStagedFileContent("file1.txt")
	if err == nil {
		t.Error("Expected error for deleted file")
	}
}
