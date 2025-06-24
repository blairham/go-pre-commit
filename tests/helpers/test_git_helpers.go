package helpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGitRepo provides helper functions for creating test git repositories
type TestGitRepo struct {
	t    *testing.T
	Path string
}

// NewTestGitRepo creates a new test git repository in a temporary directory
func NewTestGitRepo(t *testing.T) *TestGitRepo {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Initialize git repository
	gitCmd := exec.Command("git", "init")
	gitCmd.Dir = tempDir
	if err := gitCmd.Run(); err != nil {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			t.Logf("Warning: failed to remove temp dir: %v", removeErr)
		}
		t.Skipf("Git not available for testing: %v", err)
	}

	// Configure git for testing
	testCommands := [][]string{
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "commit.gpgsign", "false"}, // Disable GPG signing for tests
	}

	for _, cmdArgs := range testCommands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			if removeErr := os.RemoveAll(tempDir); removeErr != nil {
				t.Logf("Warning: failed to remove temp dir: %v", removeErr)
			}
			t.Fatalf("Failed to configure git: %v", err)
		}
	}

	return &TestGitRepo{
		Path: tempDir,
		t:    t,
	}
}

// CreateInitialCommit creates an initial commit with a test file
func (r *TestGitRepo) CreateInitialCommit() {
	r.t.Helper()

	// Create a test file
	testFile := filepath.Join(r.Path, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0o600); err != nil {
		r.t.Fatalf("Failed to create test file: %v", err)
	}

	// Add and commit the file
	addCmd := exec.Command("git", "add", "test.txt")
	addCmd.Dir = r.Path
	if err := addCmd.Run(); err != nil {
		r.t.Fatalf("Failed to add test file: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "Initial commit")
	commitCmd.Dir = r.Path
	if err := commitCmd.Run(); err != nil {
		r.t.Fatalf("Failed to create initial commit: %v", err)
	}
}

// WriteFile creates a file in the repository
func (r *TestGitRepo) WriteFile(filename string, content []byte) {
	r.t.Helper()

	filePath := filepath.Join(r.Path, filename)
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		r.t.Fatalf("Failed to write file %s: %v", filename, err)
	}
}

// Cleanup removes the test repository
func (r *TestGitRepo) Cleanup() {
	if err := os.RemoveAll(r.Path); err != nil {
		r.t.Logf("Warning: failed to remove temp dir: %v", err)
	}
}

// ChangeToRepo changes the working directory to the repository
func (r *TestGitRepo) ChangeToRepo() func() {
	r.t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		r.t.Fatalf("Failed to get current directory: %v", err)
	}

	if err := os.Chdir(r.Path); err != nil {
		r.t.Fatalf("Failed to change to repo directory: %v", err)
	}

	return func() {
		if err := os.Chdir(oldDir); err != nil {
			r.t.Logf("Warning: failed to change back to original directory: %v", err)
		}
	}
}
