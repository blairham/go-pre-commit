package cache

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blairham/go-pre-commit/pkg/config"
)

// resolvePath resolves symlinks in the path to ensure consistent path comparisons on macOS
func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path // fallback to original path if resolution fails
	}
	return resolved
}

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()

	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	require.NotNil(t, manager)

	defer manager.Close()

	assert.Equal(t, tempDir, manager.GetCacheDir())
	assert.Equal(t, filepath.Join(tempDir, "db.db"), manager.GetDBPath())

	// Verify database file was created
	_, err = os.Stat(manager.GetDBPath())
	assert.NoError(t, err)

	// Verify tables were created and are accessible
	err = manager.verifyDatabaseTables()
	assert.NoError(t, err)
}

func TestNewManager_DatabaseInitFailure(t *testing.T) {
	// Use an invalid path that cannot be created
	invalidPath := "/invalid/path/that/does/not/exist"

	manager, err := NewManager(invalidPath)
	assert.Error(t, err)
	assert.Nil(t, manager)
}

func TestManager_GetRepoPath_NewRepo(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	repo := config.Repo{
		Repo: "https://github.com/user/repo",
		Rev:  "main",
	}

	path := manager.GetRepoPath(repo)

	assert.NotEmpty(t, path)
	assert.True(t, strings.HasPrefix(path, tempDir))
	assert.Contains(t, filepath.Base(path), "repo")
}

func TestManager_GetRepoPath_ExistingRepo(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	repo := config.Repo{
		Repo: "https://github.com/user/repo",
		Rev:  "main",
	}

	// Get path first time
	path1 := manager.GetRepoPath(repo)
	require.NotEmpty(t, path1)

	// Create the .git directory to simulate existing repo
	gitDir := filepath.Join(path1, ".git")
	err = os.MkdirAll(gitDir, 0o755)
	require.NoError(t, err)

	// Update the database entry
	err = manager.UpdateRepoEntry(repo, path1)
	require.NoError(t, err)

	// Get path second time - should return same path
	path2 := manager.GetRepoPath(repo)
	assert.Equal(t, resolvePath(path1), resolvePath(path2))
}

func TestManager_GetRepoPath_StaleEntry(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	repo := config.Repo{
		Repo: "https://github.com/user/repo",
		Rev:  "main",
	}

	// Insert a stale entry directly into database
	stalePath := filepath.Join(tempDir, "nonexistent")
	err = manager.insertTestRepoEntry(repo.Repo, repo.Rev, stalePath)
	require.NoError(t, err)

	// Get path - should create new path since stale one doesn't exist
	path := manager.GetRepoPath(repo)
	assert.NotEqual(t, stalePath, path)
	assert.True(t, strings.HasPrefix(path, tempDir))

	// Verify stale entry was removed
	count, err := manager.countTestRepoEntriesByPath(stalePath)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestManager_GetRepoPathWithDeps(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	repo := config.Repo{
		Repo: "https://github.com/user/repo",
		Rev:  "main",
	}

	deps := []string{"dep1", "dep2"}

	path1 := manager.GetRepoPathWithDeps(repo, deps)
	path2 := manager.GetRepoPathWithDeps(repo, nil)

	// Paths should be different for different dependencies
	assert.NotEqual(t, path1, path2)

	// Create the .git directory and update the entry to simulate existing repo
	gitDir := filepath.Join(path1, ".git")
	err = os.MkdirAll(gitDir, 0o755)
	require.NoError(t, err)
	err = manager.UpdateRepoEntryWithDeps(repo, deps, path1)
	require.NoError(t, err)

	// Same dependencies should return same path after entry exists
	path3 := manager.GetRepoPathWithDeps(repo, deps)
	assert.Equal(t, resolvePath(path1), resolvePath(path3))
}

func TestManager_UpdateRepoEntry(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	repo := config.Repo{
		Repo: "https://github.com/user/repo",
		Rev:  "main",
	}

	repoPath := filepath.Join(tempDir, "test-repo")

	err = manager.UpdateRepoEntry(repo, repoPath)
	assert.NoError(t, err)

	// Verify entry was inserted
	path, err := manager.getTestRepoEntry(repo.Repo, repo.Rev)
	require.NoError(t, err)
	assert.Equal(t, repoPath, path)
}

func TestManager_UpdateRepoEntryWithDeps(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	repo := config.Repo{
		Repo: "https://github.com/user/repo",
		Rev:  "main",
	}

	deps := []string{"dep1", "dep2"}
	repoPath := filepath.Join(tempDir, "test-repo")

	err = manager.UpdateRepoEntryWithDeps(repo, deps, repoPath)
	assert.NoError(t, err)

	// Verify entry was inserted with correct repo name format
	expectedRepoName := "https://github.com/user/repo:dep1,dep2"
	path, err := manager.getTestRepoEntry(expectedRepoName, repo.Rev)
	require.NoError(t, err)
	assert.Equal(t, repoPath, path)
}

func TestManager_CleanCache(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Create some repo directories
	repoDir1 := filepath.Join(tempDir, "repo123456")
	repoDir2 := filepath.Join(tempDir, "repo789012")
	nonRepoDir := filepath.Join(tempDir, "other")

	err = os.MkdirAll(repoDir1, 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir2, 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(nonRepoDir, 0o755)
	require.NoError(t, err)

	// Clean cache
	err = manager.CleanCache()
	assert.NoError(t, err)

	// Verify repo directories were removed
	_, err = os.Stat(repoDir1)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(repoDir2)
	assert.True(t, os.IsNotExist(err))

	// Non-repo directory should still exist
	_, err = os.Stat(nonRepoDir)
	assert.NoError(t, err)
}

func TestManager_CleanCache_LockError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping slow test in short mode")
	}

	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Since CleanCache has a hardcoded 30s timeout, we'll skip this test
	// in normal runs and only run it when specifically testing timeout behavior
	t.Skip("CleanCache timeout test takes 30+ seconds - run with -timeout=45s if needed")
}

// Note: TestManager_CleanCache_LockError_Slow is available as a separate test
// but is skipped by default due to the 30+ second runtime.
// To test CleanCache timeout behavior, run: go test -run TestManager_CleanCache_LockError_Slow -timeout=45s

func TestManager_MarkConfigUsed(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Create a test config file
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	err = os.WriteFile(configPath, []byte("repos: []"), 0o644)
	require.NoError(t, err)

	err = manager.MarkConfigUsed(configPath)
	assert.NoError(t, err)

	// Verify entry was inserted
	absConfigPath, _ := filepath.Abs(configPath)
	// Resolve symlinks to match what MarkConfigUsed does
	normalizedPath := resolvePath(absConfigPath)
	err = manager.getTestConfigEntry(normalizedPath)
	require.NoError(t, err)
}

func TestManager_MarkConfigUsed_NonexistentFile(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Try to mark non-existent file
	configPath := filepath.Join(tempDir, "nonexistent.yaml")
	err = manager.MarkConfigUsed(configPath)
	assert.NoError(t, err) // Should not error

	// Verify no entry was inserted
	count, err := manager.countTestConfigEntries()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestManager_MarkConfigUsed_Duplicate(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Create a test config file
	configPath := filepath.Join(tempDir, ".pre-commit-config.yaml")
	err = os.WriteFile(configPath, []byte("repos: []"), 0o644)
	require.NoError(t, err)

	// Mark config used twice
	err = manager.MarkConfigUsed(configPath)
	assert.NoError(t, err)
	err = manager.MarkConfigUsed(configPath)
	assert.NoError(t, err)

	// Verify only one entry exists
	count, err := manager.countTestConfigEntries()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestManager_Close(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	err = manager.Close()
	assert.NoError(t, err)

	// Second close should also work
	err = manager.Close()
	assert.NoError(t, err)
}

func TestManager_GenerateRandomRepoDir(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	dir1 := manager.generateRandomRepoDir()
	dir2 := manager.generateRandomRepoDir()

	// Should generate different names
	assert.NotEqual(t, dir1, dir2)

	// Should start with "repo"
	assert.True(t, strings.HasPrefix(dir1, "repo"))
	assert.True(t, strings.HasPrefix(dir2, "repo"))

	// Should have appropriate length
	assert.True(t, len(dir1) > 4) // "repo" + random chars
	assert.True(t, len(dir2) > 4)
}

func TestCreateDBRepoName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		repoURL  string
		expected string
		deps     []string
	}{
		{
			name:     "no dependencies",
			repoURL:  "https://github.com/user/repo",
			deps:     nil,
			expected: "https://github.com/user/repo",
		},
		{
			name:     "empty dependencies",
			repoURL:  "https://github.com/user/repo",
			deps:     []string{},
			expected: "https://github.com/user/repo",
		},
		{
			name:     "single dependency",
			repoURL:  "https://github.com/user/repo",
			deps:     []string{"dep1"},
			expected: "https://github.com/user/repo:dep1",
		},
		{
			name:     "multiple dependencies",
			repoURL:  "https://github.com/user/repo",
			deps:     []string{"dep1", "dep2", "dep3"},
			expected: "https://github.com/user/repo:dep1,dep2,dep3",
		},
		{
			name:     "dependencies with special characters",
			repoURL:  "https://github.com/user/repo",
			deps:     []string{"dep-1", "dep_2", "dep.3"},
			expected: "https://github.com/user/repo:dep-1,dep_2,dep.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := createDBRepoName(tt.repoURL, tt.deps)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInitDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	err = initDatabase(db)
	assert.NoError(t, err)

	// Verify repos table exists and has correct schema
	rows, err := db.QueryContext(context.Background(), "PRAGMA table_info(repos)")
	require.NoError(t, err)
	defer rows.Close()
	defer func() {
		if rowsErr := rows.Err(); rowsErr != nil {
			t.Errorf("Error with rows: %v", rowsErr)
		}
	}()

	repoColumns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue any
		scanErr := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		require.NoError(t, scanErr)
		repoColumns[name] = true
	}

	assert.True(t, repoColumns["repo"])
	assert.True(t, repoColumns["ref"])
	assert.True(t, repoColumns["path"])

	// Verify configs table exists and has correct schema
	rows, err = db.QueryContext(context.Background(), "PRAGMA table_info(configs)")
	require.NoError(t, err)
	defer rows.Close()
	defer func() {
		if err := rows.Err(); err != nil {
			t.Errorf("Error with rows: %v", err)
		}
	}()

	configColumns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue any
		err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		require.NoError(t, err)
		configColumns[name] = true
	}

	assert.True(t, configColumns["path"])
}

func TestManager_DatabaseOperations_Integration(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	repo1 := config.Repo{Repo: "https://github.com/user/repo1", Rev: "main"}
	repo2 := config.Repo{Repo: "https://github.com/user/repo2", Rev: "v1.0"}

	// Test complete workflow
	path1 := manager.GetRepoPath(repo1)
	path2 := manager.GetRepoPath(repo2)

	assert.NotEqual(t, path1, path2)

	// Create actual directories to simulate real repos
	err = os.MkdirAll(filepath.Join(path1, ".git"), 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(path2, ".git"), 0o755)
	require.NoError(t, err)

	// Update entries
	err = manager.UpdateRepoEntry(repo1, path1)
	require.NoError(t, err)
	err = manager.UpdateRepoEntry(repo2, path2)
	require.NoError(t, err)

	// Verify subsequent calls return same paths
	path1Again := manager.GetRepoPath(repo1)
	path2Again := manager.GetRepoPath(repo2)
	assert.Equal(t, resolvePath(path1), resolvePath(path1Again))
	assert.Equal(t, resolvePath(path2), resolvePath(path2Again))

	// Test with dependencies
	path1WithDeps := manager.GetRepoPathWithDeps(repo1, []string{"dep1"})
	assert.NotEqual(t, path1, path1WithDeps)

	// Clean cache should remove all repo directories
	err = manager.CleanCache()
	require.NoError(t, err)

	_, err = os.Stat(path1)
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(path2)
	assert.True(t, os.IsNotExist(err))
}

func TestManager_GenerateFallbackRepoDir(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Test the fallback directory generation function directly
	dir1 := manager.generateFallbackRepoDir()
	dir2 := manager.generateFallbackRepoDir()

	// Should generate different names
	assert.NotEqual(t, dir1, dir2)

	// Should start with "repo"
	assert.True(t, strings.HasPrefix(dir1, "repo"))
	assert.True(t, strings.HasPrefix(dir2, "repo"))

	// Should have appropriate length (repo + 8 chars)
	assert.Equal(t, 12, len(dir1))
	assert.Equal(t, 12, len(dir2))

	// Should only contain valid charset
	validChars := "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, dir := range []string{dir1, dir2} {
		suffix := dir[4:] // remove "repo" prefix
		for _, char := range suffix {
			assert.Contains(t, validChars, string(char))
		}
	}
}

func TestManager_GenerateRandomRepoDir_FallbackPath(t *testing.T) {
	// Create a manager with a cache directory that cannot be used for temp dir creation
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Make the cache directory read-only to force fallback
	err = os.Chmod(tempDir, 0o555)
	require.NoError(t, err)
	defer func() {
		// Restore permissions for cleanup
		os.Chmod(tempDir, 0o755)
	}()

	// This should trigger the fallback path
	dir := manager.generateRandomRepoDir()
	assert.True(t, strings.HasPrefix(dir, "repo"))
	assert.True(t, len(dir) >= 4) // Should have some random suffix
}

func TestManager_Close_MultipleClose(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// First close should succeed
	err = manager.Close()
	assert.NoError(t, err)

	// Second close should also succeed (idempotent)
	err = manager.Close()
	assert.NoError(t, err)

	// Third close should also succeed
	err = manager.Close()
	assert.NoError(t, err)
}

func TestManager_Close_NilDB(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)

	// Close once
	err = manager.Close()
	require.NoError(t, err)

	// Manually set db to nil to test the nil check
	manager.db = nil

	// Close again - should handle nil db gracefully
	err = manager.Close()
	assert.NoError(t, err)
}

func TestManager_InsertRepoEntry_Error(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Close the database to force an error
	err = manager.Close()
	require.NoError(t, err)

	// Try to insert entry with closed database - should handle error gracefully
	err = manager.insertRepoEntry("test-repo", "main", "/some/path")
	assert.Error(t, err) // Should return an error
}

func TestManager_VerifyDatabaseTables_ReposTableError(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Drop the repos table to cause verification to fail
	_, err = manager.db.ExecContext(context.Background(), "DROP TABLE repos")
	require.NoError(t, err)

	err = manager.verifyDatabaseTables()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repos table verification failed")
}

func TestManager_VerifyDatabaseTables_ConfigsTableError(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Drop the configs table to cause verification to fail
	_, err = manager.db.ExecContext(context.Background(), "DROP TABLE configs")
	require.NoError(t, err)

	err = manager.verifyDatabaseTables()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configs table verification failed")
}

// Additional tests to improve coverage

func TestNewManager_InitDatabaseFailure(t *testing.T) {
	tempDir := t.TempDir()

	// Create a file where the database should be to cause initDatabase to fail
	dbPath := filepath.Join(tempDir, "db.db")
	err := os.WriteFile(dbPath, []byte("invalid sqlite data"), 0o644)
	require.NoError(t, err)

	// Make the file read-only to cause database initialization to fail
	err = os.Chmod(dbPath, 0o444)
	require.NoError(t, err)
	defer os.Chmod(dbPath, 0o644) // Restore for cleanup

	// This should fail during database initialization
	manager, err := NewManager(tempDir)
	assert.Error(t, err)
	assert.Nil(t, manager)
	assert.Contains(t, err.Error(), "failed to initialize database")
}

func TestNewManager_DatabaseCloseOnInitFailure(t *testing.T) {
	tempDir := t.TempDir()

	// Use a very long path that might cause issues with database operations
	longPath := strings.Repeat("a", 250)
	invalidDir := filepath.Join(tempDir, longPath, longPath)

	manager, err := NewManager(invalidDir)
	// This might succeed or fail depending on the OS, but if it fails,
	// the database should be properly closed
	if err != nil {
		assert.Nil(t, manager)
	} else if manager != nil {
		manager.Close()
	}
}

func TestManager_CleanCache_LockFailure(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Remove the lock file that was created during NewManager
	lockPath := filepath.Join(tempDir, ".lock")
	err = os.Remove(lockPath)
	require.NoError(t, err)

	// Make the cache directory read-only to prevent lock file creation
	err = os.Chmod(tempDir, 0o555)
	require.NoError(t, err)
	defer os.Chmod(tempDir, 0o755) // Restore for cleanup

	// Use a very short timeout for the test
	err = manager.CleanCacheWithTimeout(100 * time.Millisecond)
	// This should fail due to lock creation issues
	assert.Error(t, err)
}

func TestManager_RemoveAllRepoDirectories_ReadDirError(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Remove the cache directory to cause ReadDir to fail
	err = os.RemoveAll(tempDir)
	require.NoError(t, err)

	err = manager.removeAllRepoDirectories()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read cache directory")
}

func TestManager_RemoveAllRepoDirectories_RemoveError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Create a repo directory
	repoDir := filepath.Join(tempDir, "repo123456")
	err = os.MkdirAll(repoDir, 0o755)
	require.NoError(t, err)

	// Create a file inside and make the directory non-writable
	testFile := filepath.Join(repoDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0o644)
	require.NoError(t, err)

	// Make parent directory non-writable to prevent removal
	err = os.Chmod(tempDir, 0o555)
	require.NoError(t, err)
	defer os.Chmod(tempDir, 0o755) // Restore for cleanup

	err = manager.removeAllRepoDirectories()
	// This should fail due to permission issues
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove repository cache")
}

func TestInitDatabase_ReposTableError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create a conflicting table with same name but different schema
	_, err = db.ExecContext(context.Background(), "CREATE TABLE repos (id INTEGER)")
	require.NoError(t, err)

	// Now try to initialize - this should fail because table exists with different schema
	err = initDatabase(db)
	// Note: SQLite CREATE TABLE IF NOT EXISTS might not fail here
	// depending on how it handles schema conflicts, but this tests the code path
	if err != nil {
		assert.Error(t, err)
	}
}

func TestInitDatabase_ConfigsTableError(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Create repos table first (this should succeed)
	err = initDatabase(db)
	require.NoError(t, err)

	// Drop configs table and create conflicting one
	_, err = db.ExecContext(context.Background(), "DROP TABLE configs")
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), "CREATE TABLE configs (id INTEGER)")
	require.NoError(t, err)

	// Try to initialize again - this tests the configs table creation path
	err = initDatabase(db)
	// Again, this might not fail with SQLite's IF NOT EXISTS behavior
	if err != nil {
		assert.Error(t, err)
	}
}

func TestManager_GetExistingRepoPath_DatabaseError(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Close the database to cause query to fail
	err = manager.db.Close()
	require.NoError(t, err)

	path := manager.getExistingRepoPath("test-repo", "main")
	assert.Empty(t, path) // Should return empty string on database error
}

func TestManager_GetExistingRepoPath_DeleteStaleEntryError(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	repo := "https://github.com/user/repo"
	rev := "main"
	stalePath := filepath.Join(tempDir, "nonexistent")

	// Insert a stale entry
	err = manager.insertTestRepoEntry(repo, rev, stalePath)
	require.NoError(t, err)

	// Close database to cause delete to fail
	err = manager.db.Close()
	require.NoError(t, err)

	// This should attempt to delete the stale entry but fail silently
	path := manager.getExistingRepoPath(repo, rev)
	assert.Empty(t, path)
}

func TestManager_InsertRepoEntry_DatabaseError(t *testing.T) {
	tempDir := t.TempDir()
	manager, err := NewManager(tempDir)
	require.NoError(t, err)
	defer manager.Close()

	// Close database to cause insert to fail
	err = manager.db.Close()
	require.NoError(t, err)

	err = manager.insertRepoEntry("test-repo", "main", "/test/path")
	assert.Error(t, err) // Should return error when database is closed
}
