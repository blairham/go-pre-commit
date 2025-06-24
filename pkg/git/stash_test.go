package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Test constants for stash_test.go
const (
	originalContent  = "content1"
	stagedContent    = "staged content"
	unstagedContent  = "unstaged content"
	newFileName      = "new_staged.txt"
	patchFileName    = "test-patch"
	fakePatchContent = "fake patch content"
)

// ...existing code...

func TestRepository_HasUnstagedChanges(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Initially no unstaged changes
	hasChanges, err := repo.HasUnstagedChanges()
	if err != nil {
		t.Errorf("Unexpected error checking unstaged changes: %v", err)
	}
	if hasChanges {
		t.Error("Expected no unstaged changes")
	}

	// Modify a file
	modifiedFile := filepath.Join(repoDir, testFileName)
	if writeErr := os.WriteFile(modifiedFile, []byte(modifiedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	// Should now have unstaged changes
	hasChanges, err = repo.HasUnstagedChanges()
	if err != nil {
		t.Errorf("Unexpected error checking unstaged changes: %v", err)
	}
	if !hasChanges {
		t.Error("Expected unstaged changes after modification")
	}
}

func TestRepository_GetUnstagedChangesFiles(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Initially no changes
	files, err := repo.GetUnstagedChangesFiles()
	if err != nil {
		t.Errorf("Unexpected error getting unstaged files: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 unstaged files, got %d", len(files))
	}

	// Modify multiple files
	modifiedFiles := []string{"file1.txt", "file2.txt"}
	for _, file := range modifiedFiles {
		filePath := filepath.Join(repoDir, file)
		if writeErr := os.WriteFile(filePath, []byte("modified content"), 0o644); writeErr != nil {
			t.Fatalf("Failed to modify file %s: %v", file, writeErr)
		}
	}

	// Should return modified files
	files, err = repo.GetUnstagedChangesFiles()
	if err != nil {
		t.Errorf("Unexpected error getting unstaged files: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 unstaged files, got %d", len(files))
	}

	// Check that all modified files are present
	fileSet := make(map[string]bool)
	for _, file := range files {
		fileSet[file] = true
	}
	for _, expected := range modifiedFiles {
		if !fileSet[expected] {
			t.Errorf("Expected file %s not found in unstaged files", expected)
		}
	}
}

func TestRepository_StashUnstagedChanges(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	cacheDir := t.TempDir()

	// Test with no unstaged changes
	_, err = repo.StashUnstagedChanges(cacheDir)
	if !errors.Is(err, ErrNoUnstagedChanges) {
		t.Errorf("Expected ErrNoUnstagedChanges, got %v", err)
	}

	// Modify a file
	modifiedFile := filepath.Join(repoDir, testFileName)
	if writeErr := os.WriteFile(modifiedFile, []byte(modifiedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	// Stash changes
	stash, err := repo.StashUnstagedChanges(cacheDir)
	if err != nil {
		t.Fatalf("Failed to stash changes: %v", err)
	}
	if stash == nil {
		t.Fatal("Expected non-nil stash info")
	}

	// Check stash info
	if len(stash.Files) != 1 {
		t.Errorf("Expected 1 file in stash, got %d", len(stash.Files))
	}
	if len(stash.Files) > 0 && stash.Files[0] != testFileName {
		t.Errorf("Expected 'file1.txt' in stash, got %s", stash.Files[0])
	}

	// Check that patch file exists
	if _, statErr := os.Stat(stash.PatchFile); statErr != nil {
		t.Errorf("Patch file should exist: %v", statErr)
	}

	// Check that working directory is restored to HEAD state
	currentContent, err := os.ReadFile(modifiedFile)
	if err != nil {
		t.Fatalf("Failed to read file after stash: %v", err)
	}
	if string(currentContent) != originalContent {
		t.Errorf("Expected file to be restored to original content, got %s", string(currentContent))
	}

	// Clean up
	repo.CleanupStash(stash)
}

func TestRepository_CanApplyStash(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with nil stash
	canApply, err := repo.CanApplyStash(nil)
	if err != nil {
		t.Errorf("Unexpected error with nil stash: %v", err)
	}
	if !canApply {
		t.Error("Expected to be able to apply nil stash")
	}

	cacheDir := t.TempDir()

	// Modify and stash a file
	modifiedFile := filepath.Join(repoDir, testFileName)
	if writeErr := os.WriteFile(modifiedFile, []byte(modifiedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	stash, err := repo.StashUnstagedChanges(cacheDir)
	if err != nil {
		t.Fatalf("Failed to stash changes: %v", err)
	}
	defer repo.CleanupStash(stash)

	// Should be able to apply stash
	canApply, err = repo.CanApplyStash(stash)
	if err != nil {
		t.Errorf("Unexpected error checking if stash can be applied: %v", err)
	}
	if !canApply {
		t.Error("Expected to be able to apply stash")
	}
}

func TestRepository_RestoreFromStash(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with nil stash
	err = repo.RestoreFromStash(nil)
	if err != nil {
		t.Errorf("Unexpected error restoring nil stash: %v", err)
	}

	cacheDir := t.TempDir()

	// Modify and stash a file
	modifiedFile := filepath.Join(repoDir, testFileName)
	if writeErr := os.WriteFile(modifiedFile, []byte(modifiedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	stash, err := repo.StashUnstagedChanges(cacheDir)
	if err != nil {
		t.Fatalf("Failed to stash changes: %v", err)
	}

	// Restore from stash
	err = repo.RestoreFromStash(stash)
	if err != nil {
		t.Errorf("Failed to restore from stash: %v", err)
	}

	// Check that file is restored to modified content
	currentContent, err := os.ReadFile(modifiedFile)
	if err != nil {
		t.Fatalf("Failed to read file after restore: %v", err)
	}
	if string(currentContent) != modifiedContent {
		t.Errorf("Expected file to be restored to modified content, got %s", string(currentContent))
	}

	// Check that patch file is cleaned up
	if _, err := os.Stat(stash.PatchFile); err == nil {
		t.Error("Patch file should be cleaned up after restore")
	}
}

func TestRepository_ResetToStaged(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Modify a file and stage it
	modifiedFile := filepath.Join(repoDir, testFileName)
	if writeErr := os.WriteFile(modifiedFile, []byte(stagedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	cmd := exec.Command("git", "add", "file1.txt")
	cmd.Dir = repoDir
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("Failed to stage file: %v", runErr)
	}

	// Modify the file again (unstaged changes)
	if writeErr := os.WriteFile(modifiedFile, []byte(unstagedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file again: %v", writeErr)
	}

	// Reset to staged content
	err = repo.ResetToStaged()
	if err != nil {
		t.Errorf("Failed to reset to staged: %v", err)
	}

	// Check that file contains staged content
	currentContent, err := os.ReadFile(modifiedFile)
	if err != nil {
		t.Fatalf("Failed to read file after reset: %v", err)
	}
	if string(currentContent) != stagedContent {
		t.Errorf("Expected file to contain staged content, got %s", string(currentContent))
	}
}

func TestRepository_CleanupStash(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with nil stash
	repo.CleanupStash(nil) // Should not panic

	cacheDir := t.TempDir()

	// Create a patch file
	patchFile := filepath.Join(cacheDir, patchFileName)
	if err := os.WriteFile(patchFile, []byte(fakePatchContent), 0o644); err != nil {
		t.Fatalf("Failed to create patch file: %v", err)
	}

	stash := &StashInfo{
		PatchFile: patchFile,
		Files:     []string{"test.txt"},
	}

	// Cleanup stash
	repo.CleanupStash(stash)

	// Check that patch file is removed
	if _, err := os.Stat(patchFile); err == nil {
		t.Error("Patch file should be removed after cleanup")
	}
}

func TestRepository_CreatePatchFile(t *testing.T) {
	t.Parallel() // This test can run in parallel

	repo := &Repository{}
	cacheDir := t.TempDir()

	// Test creating patch file
	patchFile, err := repo.createPatchFile(cacheDir)
	if err != nil {
		t.Errorf("Failed to create patch file: %v", err)
	}

	// Check that cache directory exists
	if _, err := os.Stat(cacheDir); err != nil {
		t.Errorf("Cache directory should exist: %v", err)
	}

	// Check that patch file path is within cache directory
	if !strings.HasPrefix(patchFile, cacheDir) {
		t.Errorf("Patch file should be in cache directory")
	}

	// Check that patch file name contains expected elements
	baseName := filepath.Base(patchFile)
	if !strings.HasPrefix(baseName, "patch") {
		t.Errorf("Patch file should start with 'patch', got %s", baseName)
	}
}

func TestRepository_CheckoutFileFromHEAD(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Modify a file
	modifiedFile := filepath.Join(repoDir, testFileName)
	if writeErr := os.WriteFile(modifiedFile, []byte(modifiedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	// Checkout file from HEAD
	err = repo.checkoutFileFromHEAD("file1.txt")
	if err != nil {
		t.Errorf("Failed to checkout file from HEAD: %v", err)
	}

	// Check that file is restored to original content
	currentContent, err := os.ReadFile(modifiedFile)
	if err != nil {
		t.Fatalf("Failed to read file after checkout: %v", err)
	}
	if string(currentContent) != originalContent {
		t.Errorf("Expected file to be restored to original content, got %s", string(currentContent))
	}
}

func TestRepository_WriteFileFromStaged(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Stage a new file
	newFile := filepath.Join(repoDir, newFileName)
	stagedContent := "staged content"
	if writeErr := os.WriteFile(newFile, []byte(stagedContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to create new file: %v", writeErr)
	}

	cmd := exec.Command("git", "add", "new_staged.txt")
	cmd.Dir = repoDir
	if runErr := cmd.Run(); runErr != nil {
		t.Fatalf("Failed to stage file: %v", runErr)
	}

	// Modify the working copy
	workingContent := "working content"
	if writeErr := os.WriteFile(newFile, []byte(workingContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify working file: %v", writeErr)
	}

	// Write staged version to working directory
	err = repo.writeFileFromStaged("new_staged.txt")
	if err != nil {
		t.Errorf("Failed to write file from staged: %v", err)
	}

	// Check that file contains staged content
	currentContent, err := os.ReadFile(newFile)
	if err != nil {
		t.Fatalf("Failed to read file after write from staged: %v", err)
	}
	if string(currentContent) != stagedContent {
		t.Errorf("Expected file to contain staged content, got %s", string(currentContent))
	}
}

func TestStashError(t *testing.T) {
	t.Parallel() // This test can run in parallel

	// Test ErrNoUnstagedChanges
	if ErrNoUnstagedChanges == nil {
		t.Error("ErrNoUnstagedChanges should not be nil")
	}
	if ErrNoUnstagedChanges.Error() != "no unstaged changes to stash" {
		t.Errorf("Expected 'no unstaged changes to stash', got %s", ErrNoUnstagedChanges.Error())
	}
}

func TestStashInfo(t *testing.T) {
	t.Parallel() // This test can run in parallel

	// Test StashInfo struct
	stash := &StashInfo{
		PatchFile: "/path/to/patch",
		Files:     []string{"file1.txt", "file2.txt"},
	}

	if stash.PatchFile != "/path/to/patch" {
		t.Errorf("Expected patch file '/path/to/patch', got %s", stash.PatchFile)
	}
	if len(stash.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(stash.Files))
	}
}

// Integration test for full stash workflow
func TestRepository_StashWorkflow(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	cacheDir := t.TempDir()

	// Modify multiple files
	files := map[string]string{
		"file1.txt": "modified content 1",
		"file2.txt": "modified content 2",
	}

	for file, content := range files {
		filePath := filepath.Join(repoDir, file)
		if writeErr := os.WriteFile(filePath, []byte(content), 0o644); writeErr != nil {
			t.Fatalf("Failed to modify file %s: %v", file, writeErr)
		}
	}

	// Check that we have unstaged changes
	hasChanges, err := repo.HasUnstagedChanges()
	if err != nil {
		t.Fatalf("Failed to check unstaged changes: %v", err)
	}
	if !hasChanges {
		t.Fatal("Expected unstaged changes")
	}

	// Stash the changes
	stash, err := repo.StashUnstagedChanges(cacheDir)
	if err != nil {
		t.Fatalf("Failed to stash changes: %v", err)
	}
	defer repo.CleanupStash(stash)

	// Verify no unstaged changes after stash
	hasChanges, err = repo.HasUnstagedChanges()
	if err != nil {
		t.Fatalf("Failed to check unstaged changes after stash: %v", err)
	}
	if hasChanges {
		t.Error("Expected no unstaged changes after stash")
	}

	// Check if we can apply the stash
	canApply, err := repo.CanApplyStash(stash)
	if err != nil {
		t.Fatalf("Failed to check if stash can be applied: %v", err)
	}
	if !canApply {
		t.Error("Expected to be able to apply stash")
	}

	// Restore from stash
	err = repo.RestoreFromStash(stash)
	if err != nil {
		t.Fatalf("Failed to restore from stash: %v", err)
	}

	// Verify changes are restored
	for file, expectedContent := range files {
		filePath := filepath.Join(repoDir, file)
		currentContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file %s after restore: %v", file, err)
		}
		if string(currentContent) != expectedContent {
			t.Errorf("File %s: expected %s, got %s", file, expectedContent, string(currentContent))
		}
	}
}

// Additional tests for improved coverage

func TestRepository_WriteFileFromStaged_ErrorCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with non-existent staged file
	err = repo.writeFileFromStaged("nonexistent.txt")
	if err == nil {
		t.Error("Expected error for non-existent staged file")
	}

	// Test writing to a path that can't be written (subdirectory that doesn't exist)
	if removeErr := os.RemoveAll(filepath.Join(repoDir, "dir")); removeErr != nil {
		t.Fatalf("Failed to remove directory: %v", removeErr)
	}

	// Create and stage a file in a subdirectory
	subFile := filepath.Join(repoDir, "newdir", "subfile.txt")
	if mkdirErr := os.MkdirAll(filepath.Dir(subFile), 0o755); mkdirErr != nil {
		t.Fatalf("Failed to create subdirectory: %v", mkdirErr)
	}
	if writeErr := os.WriteFile(subFile, []byte("sub content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create sub file: %v", writeErr)
	}

	cmd := exec.Command("git", "add", "newdir/subfile.txt")
	cmd.Dir = repoDir
	if addErr := cmd.Run(); addErr != nil {
		t.Fatalf("Failed to stage sub file: %v", addErr)
	}

	// Remove the directory after staging
	if removeErr := os.RemoveAll(filepath.Join(repoDir, "newdir")); removeErr != nil {
		t.Fatalf("Failed to remove subdirectory: %v", removeErr)
	}

	// Now try to write from staged - should fail because parent directory doesn't exist
	err = repo.writeFileFromStaged("newdir/subfile.txt")
	if err == nil {
		t.Error("Expected error when parent directory doesn't exist")
	}
}

func TestRepository_StashUnstagedChanges_FailureRecovery(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Create a file and modify it
	testFile := filepath.Join(repoDir, "test_recovery.txt")
	if writeErr := os.WriteFile(testFile, []byte("original"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create test file: %v", writeErr)
	}

	cmd := exec.Command("git", "add", "test_recovery.txt")
	cmd.Dir = repoDir
	if addErr := cmd.Run(); addErr != nil {
		t.Fatalf("Failed to stage file: %v", addErr)
	}

	cmd = exec.Command("git", "commit", "-m", "Add test file")
	cmd.Dir = repoDir
	if commitErr := cmd.Run(); commitErr != nil {
		t.Fatalf("Failed to commit file: %v", commitErr)
	}

	// Modify the file
	if writeErr := os.WriteFile(testFile, []byte("modified"), 0o644); writeErr != nil {
		t.Fatalf("Failed to modify file: %v", writeErr)
	}

	// Create a cache directory but make it read-only to cause write failure
	cacheDir := t.TempDir()
	if chmodErr := os.Chmod(cacheDir, 0o444); chmodErr != nil {
		t.Fatalf("Failed to make cache dir read-only: %v", chmodErr)
	}

	// Restore permissions for cleanup
	defer func() {
		os.Chmod(cacheDir, 0o755)
	}()

	// This should fail due to inability to create patch file
	_, err = repo.StashUnstagedChanges(cacheDir)
	if err == nil {
		t.Error("Expected error when cache directory is read-only")
	}
}

func TestRepository_CanApplyStash_EdgeCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with stash that has invalid patch file
	invalidStash := &StashInfo{
		PatchFile: "/nonexistent/patch/file",
		Files:     []string{"test.txt"},
	}

	canApply, err := repo.CanApplyStash(invalidStash)
	if err != nil {
		t.Errorf("Unexpected error with invalid patch file: %v", err)
	}
	if canApply {
		t.Error("Expected to not be able to apply stash with invalid patch file")
	}

	// Create a conflicting patch
	cacheDir := t.TempDir()
	conflictingPatch := filepath.Join(cacheDir, "conflict.patch")

	// Create a patch that would conflict (modify a file that doesn't exist)
	conflictContent := `diff --git a/conflict.txt b/conflict.txt
index 1234567..abcdefg 100644
--- a/conflict.txt
+++ b/conflict.txt
@@ -1 +1 @@
-original line
+modified line
`
	if writeErr := os.WriteFile(conflictingPatch, []byte(conflictContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to create conflicting patch: %v", writeErr)
	}

	conflictStash := &StashInfo{
		PatchFile: conflictingPatch,
		Files:     []string{"conflict.txt"},
	}

	canApply, err = repo.CanApplyStash(conflictStash)
	if err != nil {
		t.Errorf("Unexpected error checking conflicting stash: %v", err)
	}
	if canApply {
		t.Error("Expected to not be able to apply conflicting stash")
	}
}

func TestRepository_RestoreFromStash_EdgeCases(t *testing.T) {
	repoDir := setupTestRepo(t)
	repo, err := NewRepository(repoDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	// Test with stash that has invalid patch file
	invalidStash := &StashInfo{
		PatchFile: "/nonexistent/patch/file",
		Files:     []string{"test.txt"},
	}

	err = repo.RestoreFromStash(invalidStash)
	if err == nil {
		t.Error("Expected error restoring from stash with invalid patch file")
	}

	// Create a valid patch file but with invalid content
	cacheDir := t.TempDir()
	badPatch := filepath.Join(cacheDir, "bad.patch")
	if writeErr := os.WriteFile(badPatch, []byte("invalid patch content"), 0o644); writeErr != nil {
		t.Fatalf("Failed to create bad patch: %v", writeErr)
	}

	badStash := &StashInfo{
		PatchFile: badPatch,
		Files:     []string{"test.txt"},
	}

	err = repo.RestoreFromStash(badStash)
	if err == nil {
		t.Error("Expected error restoring from stash with invalid patch content")
	}
}

func TestRepository_CreatePatchFile_EdgeCases(t *testing.T) {
	t.Parallel() // This test can run in parallel

	repo := &Repository{}

	// Test with invalid cache directory path
	_, err := repo.createPatchFile("/invalid/path/that/cannot/be/created")
	if err == nil {
		t.Error("Expected error creating patch file in invalid directory")
	}

	// Test with existing cache directory
	cacheDir := t.TempDir()
	patchFile1, err := repo.createPatchFile(cacheDir)
	if err != nil {
		t.Errorf("Unexpected error creating first patch file: %v", err)
	}

	// Create another patch file - should have different name
	patchFile2, err := repo.createPatchFile(cacheDir)
	if err != nil {
		t.Errorf("Unexpected error creating second patch file: %v", err)
	}

	if patchFile1 == patchFile2 {
		t.Error("Expected different patch file names")
	}

	// Both should be in the cache directory
	if !strings.HasPrefix(patchFile1, cacheDir) {
		t.Error("First patch file should be in cache directory")
	}
	if !strings.HasPrefix(patchFile2, cacheDir) {
		t.Error("Second patch file should be in cache directory")
	}
}
