// Package git provides Git repository operations for pre-commit hooks.
package git

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Repository represents a git repository
type Repository struct {
	repo *git.Repository
	Root string
}

// NewRepository creates a new Repository instance
func NewRepository(path string) (*Repository, error) {
	root, err := FindGitRoot(path)
	if err != nil {
		return nil, err
	}

	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	return &Repository{
		Root: root,
		repo: repo,
	}, nil
}

// FindGitRoot finds the root of the git repository
func FindGitRoot(path string) (string, error) {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	for {
		gitDir := filepath.Join(path, ".git")
		if info, err := os.Stat(gitDir); err == nil {
			if info.IsDir() {
				return path, nil
			}
			// Handle git worktrees (where .git is a file)
			// #nosec G304 -- reading git metadata
			if content, err := os.ReadFile(gitDir); err == nil {
				line := strings.TrimSpace(string(content))
				if strings.HasPrefix(line, "gitdir: ") {
					return path, nil
				}
			}
		}

		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("not in a git repository")
		}
		path = parent
	}
}

// GetStagedFiles returns the list of staged files
func (r *Repository) GetStagedFiles() ([]string, error) {
	if r.repo == nil {
		return nil, errors.New("repository is not initialized")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	var files []string
	for file, fileStatus := range status {
		// Check if file is staged (added, copied, modified in staging area)
		if fileStatus.Staging == git.Added ||
			fileStatus.Staging == git.Modified ||
			fileStatus.Staging == git.Copied {
			files = append(files, file)
		}
	}

	return files, nil
}

// GetAllFiles returns all files in the repository
// This matches Python pre-commit's behavior using 'git ls-files'
func (r *Repository) GetAllFiles() ([]string, error) {
	if r.repo == nil {
		return nil, errors.New("repository is not initialized")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get all files from the git index (staged + committed files)
	// This works even when there's no HEAD commit
	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	fileSet := make(map[string]bool)

	// Add all files from the status (includes staged files)
	for file := range status {
		fileSet[file] = true
	}

	// Try to add files from HEAD commit if available
	r.addHeadFilesToSet(fileSet)

	// Convert set to slice and return
	return r.convertFileSetToSlice(fileSet), nil
}

// addHeadFilesToSet adds files from the HEAD commit to the given file set
// This is a best-effort operation that gracefully handles missing HEAD/commits
func (r *Repository) addHeadFilesToSet(fileSet map[string]bool) {
	head, err := r.repo.Head()
	if err != nil {
		// HEAD doesn't exist (empty repo) - this is normal, not an error
		return
	}

	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		// Can't get commit - continue gracefully
		return
	}

	tree, err := commit.Tree()
	if err != nil {
		// Can't get tree - continue gracefully
		return
	}

	// Add all files from the tree (best-effort, ignore errors)
	//nolint:errcheck // Intentionally ignoring errors - this is best-effort file collection
	tree.Files().ForEach(func(f *object.File) error {
		fileSet[f.Name] = true
		return nil
	})
}

// convertFileSetToSlice converts a file set to a slice
func (r *Repository) convertFileSetToSlice(fileSet map[string]bool) []string {
	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}

	return files
}

// GetChangedFiles returns files changed between two git references
func (r *Repository) GetChangedFiles(fromRef, toRef string) ([]string, error) {
	fromHash, err := r.resolveReference(fromRef)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve reference %s: %w", fromRef, err)
	}

	toHash, err := r.resolveReference(toRef)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve reference %s: %w", toRef, err)
	}

	fromCommit, err := r.repo.CommitObject(fromHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit %s: %w", fromRef, err)
	}

	toCommit, err := r.repo.CommitObject(toHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit %s: %w", toRef, err)
	}

	fromTree, err := fromCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree for %s: %w", fromRef, err)
	}

	toTree, err := toCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree for %s: %w", toRef, err)
	}

	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff between %s and %s: %w", fromRef, toRef, err)
	}

	var files []string
	for _, change := range changes {
		// Include Added, Copied, Modified files (ACM filter equivalent)
		if change.To.Name != "" {
			files = append(files, change.To.Name)
		}
	}

	return files, nil
}

// GetUnstagedFiles returns the list of unstaged files
func (r *Repository) GetUnstagedFiles() ([]string, error) {
	if r.repo == nil {
		return nil, errors.New("repository is not initialized")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	var files []string
	for file, fileStatus := range status {
		// Check if file is modified in working tree (added, copied, modified)
		if fileStatus.Worktree == git.Modified ||
			fileStatus.Worktree == git.Untracked {
			files = append(files, file)
		}
	}

	return files, nil
}

// GetCommitFiles returns files in a specific commit
func (r *Repository) GetCommitFiles(commitRef string) ([]string, error) {
	hash, err := r.resolveReference(commitRef)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve commit %s: %w", commitRef, err)
	}

	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit %s: %w", commitRef, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree for commit %s: %w", commitRef, err)
	}

	// If this is the first commit, return all files
	if len(commit.ParentHashes) == 0 {
		var files []string
		err = tree.Files().ForEach(func(f *object.File) error {
			files = append(files, f.Name)
			return nil
		})
		return files, err
	}

	// Compare with first parent to get changes in this commit
	parentCommit, err := r.repo.CommitObject(commit.ParentHashes[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get parent commit: %w", err)
	}

	parentTree, err := parentCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get parent tree: %w", err)
	}

	changes, err := parentTree.Diff(tree)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff for commit %s: %w", commitRef, err)
	}

	var files []string
	for _, change := range changes {
		if change.To.Name != "" {
			files = append(files, change.To.Name)
		}
	}

	return files, nil
}

// GetPushFiles returns files being pushed between local and remote branches
func (r *Repository) GetPushFiles(localBranch, remoteBranch string) ([]string, error) {
	// Try to get diff between remote and local branches
	localHash, err := r.resolveReference(localBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve local branch %s: %w", localBranch, err)
	}

	remoteHash, err := r.resolveReference(remoteBranch)
	if err != nil {
		// If remote branch doesn't exist, return all files in the local branch
		return r.GetAllFiles()
	}

	localCommit, err := r.repo.CommitObject(localHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get local commit: %w", err)
	}

	remoteCommit, err := r.repo.CommitObject(remoteHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote commit: %w", err)
	}

	localTree, err := localCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get local tree: %w", err)
	}

	remoteTree, err := remoteCommit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get remote tree: %w", err)
	}

	changes, err := remoteTree.Diff(localTree)
	if err != nil {
		return nil, fmt.Errorf("failed to get diff between branches: %w", err)
	}

	var files []string
	for _, change := range changes {
		if change.To.Name != "" {
			files = append(files, change.To.Name)
		}
	}

	return files, nil
}

// GetCurrentBranch returns the current branch name
func (r *Repository) GetCurrentBranch() (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	if !head.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is not pointing to a branch")
	}

	return head.Name().Short(), nil
}

// GetRemoteURL returns the URL of the specified remote
func (r *Repository) GetRemoteURL(remoteName string) (string, error) {
	remote, err := r.repo.Remote(remoteName)
	if err != nil {
		return "", fmt.Errorf("failed to get remote %s: %w", remoteName, err)
	}

	config := remote.Config()
	if len(config.URLs) == 0 {
		return "", fmt.Errorf("no URLs configured for remote %s", remoteName)
	}

	return config.URLs[0], nil
}

// IsInRepository checks if we're in a git repository
func IsInRepository() bool {
	_, err := FindGitRoot("")
	return err == nil
}

// InstallHook installs a git hook
func (r *Repository) InstallHook(hookName, script string) error {
	hooksDir := filepath.Join(r.Root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hookPath := filepath.Join(hooksDir, hookName)
	if err := os.WriteFile(hookPath, []byte(script), 0o600); err != nil {
		return fmt.Errorf("failed to write hook file: %w", err)
	}

	// Make the hook script executable
	// #nosec G302 - Hook scripts need to be executable
	if err := os.Chmod(hookPath, 0o700); err != nil {
		return fmt.Errorf("failed to make hook executable: %w", err)
	}

	return nil
}

// UninstallHook removes a git hook
func (r *Repository) UninstallHook(hookName string) error {
	hookPath := filepath.Join(r.Root, ".git", "hooks", hookName)
	if err := os.Remove(hookPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove hook: %w", err)
	}
	return nil
}

// HasHook checks if a hook is installed
func (r *Repository) HasHook(hookName string) bool {
	hookPath := filepath.Join(r.Root, ".git", "hooks", hookName)
	_, err := os.Stat(hookPath)
	return err == nil
}

// GetModifiedFiles returns the list of modified files in the working directory
func (r *Repository) GetModifiedFiles() ([]string, error) {
	if r.repo == nil {
		return nil, errors.New("repository is not initialized")
	}

	w, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := w.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	var files []string
	for file, st := range status {
		// Check if the file is modified, staged, or deleted
		if st.Worktree != git.Unmodified || st.Staging != git.Unmodified {
			files = append(files, file)
		}
	}

	return files, nil
}

// CheckFileModifications checks if any of the specified files have been modified
// by comparing the working directory against the git index
func (r *Repository) CheckFileModifications(files []string) (bool, error) {
	if len(files) == 0 {
		return false, nil
	}

	if r.repo == nil {
		return false, errors.New("repository is not initialized")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get status: %w", err)
	}

	// Convert files slice to a map for faster lookup
	fileSet := make(map[string]bool, len(files))
	for _, file := range files {
		fileSet[file] = true
	}

	// Check if any of the specified files have modifications
	for file, fileStatus := range status {
		if fileSet[file] {
			// Check if the file has been modified (staging area != working directory)
			if fileStatus.Worktree != git.Unmodified {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetDiffOutput gets git diff output for the given files
func (r *Repository) GetDiffOutput(files []string) (string, error) {
	if len(files) == 0 {
		return "", nil
	}

	if r.repo == nil {
		return "", errors.New("repository is not initialized")
	}

	worktree, err := r.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get the HEAD commit
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	headCommit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD commit: %w", err)
	}

	// Get the HEAD tree
	headTree, err := headCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD tree: %w", err)
	}

	var diffOutput strings.Builder
	hasContent := false

	// For each file, get the diff
	for _, file := range files {
		fileDiff, err := r.getFileDiff(headTree, worktree, file)
		if err != nil {
			return "", fmt.Errorf("failed to get diff for file %s: %w", file, err)
		}
		if fileDiff != "" {
			diffOutput.WriteString(fileDiff)
			if !strings.HasSuffix(fileDiff, "\n") {
				diffOutput.WriteString("\n")
			}
			hasContent = true
		}
	}

	if !hasContent {
		return "No differences detected", nil
	}

	return strings.TrimSpace(diffOutput.String()), nil
}

// getFileDiff gets the diff for a single file between HEAD and working directory
func (r *Repository) getFileDiff(
	headTree *object.Tree,
	worktree *git.Worktree,
	file string,
) (string, error) {
	// Get the file from HEAD
	headFile, err := headTree.File(file)
	var headContent string
	if err != nil {
		// File doesn't exist in HEAD (new file)
		headContent = ""
	} else {
		headContent, err = headFile.Contents()
		if err != nil {
			return "", fmt.Errorf("failed to get HEAD content for %s: %w", file, err)
		}
	}

	// Get the current working directory content
	workingFile, err := worktree.Filesystem.Open(file)
	if err != nil {
		// File doesn't exist in working directory (deleted file)
		if headContent != "" {
			return fmt.Sprintf("--- a/%s\n+++ /dev/null\n@@ -1,%d +0,0 @@\n-%s",
				file, len(strings.Split(headContent, "\n")),
				strings.ReplaceAll(headContent, "\n", "\n-")), nil
		}
		return "", nil
	}
	defer func() {
		_ = workingFile.Close() //nolint:errcheck // Best effort close, error not critical
	}()

	// Read the file content
	buf := make([]byte, 64*1024) // Read in 64KB chunks
	var workingContent strings.Builder
	for {
		n, err := workingFile.Read(buf)
		if n > 0 {
			workingContent.Write(buf[:n])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("failed to read working file %s: %w", file, err)
		}
	}
	workingContentStr := workingContent.String()

	// If contents are the same, no diff
	if headContent == workingContentStr {
		return "", nil
	}

	// Generate a simple diff output
	return r.generateSimpleDiff(file, headContent, workingContentStr), nil
}

// generateSimpleDiff creates a simple unified diff format
func (r *Repository) generateSimpleDiff(file, oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- a/%s\n", file))
	diff.WriteString(fmt.Sprintf("+++ b/%s\n", file))

	// Simple line-by-line comparison
	maxLines := max(len(newLines), len(oldLines))

	if maxLines > 0 {
		diff.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines)))

		// Show differences
		for i := range maxLines {
			var oldLine, newLine string
			if i < len(oldLines) {
				oldLine = oldLines[i]
			}
			if i < len(newLines) {
				newLine = newLines[i]
			}

			if i < len(oldLines) && (i >= len(newLines) || oldLine != newLine) {
				diff.WriteString(fmt.Sprintf("-%s\n", oldLine))
			}
			if i < len(newLines) && (i >= len(oldLines) || oldLine != newLine) {
				diff.WriteString(fmt.Sprintf("+%s\n", newLine))
			}
		}
	}

	return diff.String()
}

// resolveReference resolves a git reference (branch, tag, commit hash) to a hash
func (r *Repository) resolveReference(ref string) (plumbing.Hash, error) {
	// Try to resolve as a branch or tag first
	if resolvedRef, err := r.repo.ResolveRevision(plumbing.Revision(ref)); err == nil {
		return *resolvedRef, nil
	}

	// Try to parse as a direct hash
	if hash := plumbing.NewHash(ref); !hash.IsZero() {
		return hash, nil
	}

	return plumbing.ZeroHash, fmt.Errorf("unable to resolve reference: %s", ref)
}

// HasUnmergedFiles checks if there are unmerged files in the repository
func (r *Repository) HasUnmergedFiles() bool {
	worktree, err := r.repo.Worktree()
	if err != nil {
		return false
	}

	status, err := worktree.Status()
	if err != nil {
		return false
	}

	// Check if any files have unmerged status
	// In go-git, unmerged files show with specific staging codes
	for _, fileStatus := range status {
		// Check for actual merge conflict markers
		// Unmerged files have specific staging codes like UpdatedButUnmerged
		if fileStatus.Staging == git.UpdatedButUnmerged ||
			fileStatus.Worktree == git.UpdatedButUnmerged {
			return true
		}
	}
	return false
}

// HasUnstagedChangesForFile checks if a specific file has unstaged changes
func (r *Repository) HasUnstagedChangesForFile(filePath string) bool {
	worktree, err := r.repo.Worktree()
	if err != nil {
		return false
	}

	status, err := worktree.Status()
	if err != nil {
		return false
	}

	// Check if the specific file has unstaged changes
	if fileStatus, exists := status[filePath]; exists {
		return fileStatus.Worktree == git.Modified
	}

	return false
}

// GetStagedFileContent gets the staged content of a file
func (r *Repository) GetStagedFileContent(filePath string) ([]byte, error) {
	// Get the index (staged content)
	worktree, err := r.repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get the file from the index
	file, err := worktree.Filesystem.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open staged file: %w", err)
	}
	defer func() { _ = file.Close() }() //nolint:errcheck // intentionally ignore cleanup error

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read staged file content: %w", err)
	}

	return content, nil
}
