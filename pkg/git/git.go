// Package git provides Git repository operations for pre-commit hooks.
package git

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
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

// HasCoreHooksPath checks if core.hooksPath is set in the git config
func (r *Repository) HasCoreHooksPath() bool {
	cfg, err := r.repo.Config()
	if err != nil {
		return false
	}

	// Check the raw config for core.hooksPath
	coreSection := cfg.Raw.Section("core")
	if coreSection == nil {
		return false
	}

	hooksPath := coreSection.Option("hooksPath")
	return hooksPath != ""
}

// GetHooksPath returns the hooks directory path, considering core.hooksPath
func (r *Repository) GetHooksPath() string {
	cfg, err := r.repo.Config()
	if err != nil {
		return filepath.Join(r.Root, ".git", "hooks")
	}

	coreSection := cfg.Raw.Section("core")
	if coreSection != nil {
		if hooksPath := coreSection.Option("hooksPath"); hooksPath != "" {
			if filepath.IsAbs(hooksPath) {
				return hooksPath
			}
			return filepath.Join(r.Root, hooksPath)
		}
	}

	return filepath.Join(r.Root, ".git", "hooks")
}

// HookIdentifier is used to identify hook files we install (matches Python)
// This mirrors Python's approach of using a unique header to identify scripts
const HookIdentifier = "# File generated by pre-commit: https://pre-commit.com"

// CurrentHash is the current hash used to identify our hook scripts
// This matches Python's CURRENT_HASH value for compatibility
const CurrentHash = "138fd403232d2ddd5efb44317e38bf03"

// PriorHashes are previous hash values used in older versions
// This matches Python's PRIOR_HASHES for backwards compatibility
var PriorHashes = []string{
	"4d9958c90bc262f47553e2c073f14cfe",
	"d8ee923c46731b42cd95cc869add4062",
	"49fd668cb42069aa1b6048464be5d395",
	"79f09a650522a87b0da915d0d983b2de",
	"e358c9dae00eac5d06b38dfdb1e33a8c",
	"go-pre-commit-v1",           // Our previous identifier
	"# Generated by go-pre-commit", // Legacy marker from earlier versions
}

// IsOurHook checks if the hook at the given path was generated by go-pre-commit
// This mirrors Python's is_our_script() function which checks for CURRENT_HASH or PRIOR_HASHES
func (r *Repository) IsOurHook(hookName string) bool {
	hookPath := filepath.Join(r.Root, ".git", "hooks", hookName)
	// #nosec G304 -- reading hook script for identification
	content, err := os.ReadFile(hookPath)
	if err != nil {
		return false
	}

	contentStr := string(content)

	// Check for the hook identifier header
	if strings.Contains(contentStr, HookIdentifier) {
		return true
	}

	// Check for current hash
	if strings.Contains(contentStr, CurrentHash) {
		return true
	}

	// Check for any prior hashes (backwards compatibility)
	for _, priorHash := range PriorHashes {
		if strings.Contains(contentStr, priorHash) {
			return true
		}
	}

	return false
}

// InstallHook installs a git hook, backing up existing non-pre-commit hooks to .legacy
// If overwrite is true, existing .legacy files are removed
func (r *Repository) InstallHook(hookName, script string, overwrite bool) error {
	hooksDir := filepath.Join(r.Root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o750); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	hookPath := filepath.Join(hooksDir, hookName)
	legacyPath := hookPath + ".legacy"

	// If overwrite and legacy exists, remove it
	if overwrite {
		os.Remove(legacyPath) // Ignore error if doesn't exist
	}

	// If hook exists and is not ours, move it to .legacy
	if r.HasHook(hookName) && !r.IsOurHook(hookName) {
		if err := os.Rename(hookPath, legacyPath); err != nil {
			return fmt.Errorf("failed to backup existing hook to %s: %w", legacyPath, err)
		}
	}

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

// GetBestCandidateTag gets the best tag candidate from multiple tags pointing at the same commit.
// When multiple tags exist on a SHA, this prefers tags that look like version numbers (contain a dot).
// This matches the behavior of Python's pre-commit implementation.
func GetBestCandidateTag(rev, repoURL string) (string, error) {
	// Create a remote to list references
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})

	// List remote references
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return rev, fmt.Errorf("failed to list remote refs: %w", err)
	}

	var tagsAtRev []string
	revHash := plumbing.NewHash(rev)

	for _, ref := range refs {
		// Only process tag references
		if !ref.Name().IsTag() {
			continue
		}

		commitHash := ref.Hash()
		tagName := ref.Name().Short()

		// Check if this tag points to our revision (support partial hash matching)
		if commitHash.String() == rev ||
			strings.HasPrefix(commitHash.String(), rev) ||
			strings.HasPrefix(rev, commitHash.String()) ||
			commitHash == revHash {
			tagsAtRev = append(tagsAtRev, tagName)
		}
	}

	// If we found tags, prefer ones that contain a dot (version tags)
	for _, tag := range tagsAtRev {
		if strings.Contains(tag, ".") {
			return tag, nil
		}
	}

	// If we found tags but none with dots, return the first one
	if len(tagsAtRev) > 0 {
		return tagsAtRev[0], nil
	}

	// No tags found, return the original revision
	return rev, nil
}

// GetRemoteTags fetches all tags from a remote repository and returns them sorted by version.
func GetRemoteTags(repoURL string) (map[string]string, error) {
	// Create a remote to list references
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})

	// List remote references
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	tags := make(map[string]string)
	for _, ref := range refs {
		if ref.Name().IsTag() {
			tagName := ref.Name().Short()
			commitHash := ref.Hash().String()
			tags[tagName] = commitHash
		}
	}

	return tags, nil
}

// GetRemoteHEAD gets the HEAD commit hash from a remote repository.
func GetRemoteHEAD(repoURL string) (string, error) {
	// Create a remote to list references
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})

	// List remote references
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list remote refs: %w", err)
	}

	for _, ref := range refs {
		if ref.Name().String() == "HEAD" {
			return ref.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("HEAD not found in remote repository")
}

// GetLatestVersionTag gets the latest semantic version tag from a remote repository.
func GetLatestVersionTag(repoURL string) (string, string, error) {
	tags, err := GetRemoteTags(repoURL)
	if err != nil {
		return "", "", err
	}

	// Regex to match version tags (e.g., v1.2.3, 1.2.3)
	versionRegex := regexp.MustCompile(`^v?\d+\.\d+\.\d+`)

	var versionTags []string
	tagToHash := make(map[string]string)

	for tag, hash := range tags {
		if versionRegex.MatchString(tag) {
			versionTags = append(versionTags, tag)
			tagToHash[tag] = hash
		}
	}

	if len(versionTags) == 0 {
		return "", "", fmt.Errorf("no version tags found")
	}

	// Sort tags in reverse order (latest first)
	// This is a simple lexicographic sort, which works for most semantic versions
	sort.Slice(versionTags, func(i, j int) bool {
		return versionTags[i] > versionTags[j]
	})

	latestTag := versionTags[0]
	return latestTag, tagToHash[latestTag], nil
}

// GetCommitForRef gets the commit hash for a specific ref (tag or branch) from a remote repository.
func GetCommitForRef(repoURL, ref string) (string, error) {
	// Create a remote to list references
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})

	// List remote references
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list remote refs: %w", err)
	}

	// Try to find the ref as a tag first
	tagRefName := plumbing.NewTagReferenceName(ref)
	for _, r := range refs {
		if r.Name() == tagRefName {
			return r.Hash().String(), nil
		}
	}

	// Try as a branch
	branchRefName := plumbing.NewBranchReferenceName(ref)
	for _, r := range refs {
		if r.Name() == branchRefName {
			return r.Hash().String(), nil
		}
	}

	return "", fmt.Errorf("ref %s not found in remote repository", ref)
}

// RevExists checks if a git revision exists in the repository
func (r *Repository) RevExists(rev string) bool {
	if r.repo == nil {
		return false
	}
	_, err := r.resolveReference(rev)
	return err == nil
}

// FindAncestors finds ancestors of a commit that aren't in the specified remote
func (r *Repository) FindAncestors(localSHA, remoteName string) ([]string, error) {
	if r.repo == nil {
		return nil, errors.New("repository is not initialized")
	}

	// Resolve the local commit
	localHash, err := r.resolveReference(localSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve local SHA: %w", err)
	}

	// Get all remote refs for the specified remote
	remote, err := r.repo.Remote(remoteName)
	if err != nil {
		return nil, fmt.Errorf("failed to get remote %s: %w", remoteName, err)
	}

	remoteRefs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}

	// Build a set of all commits reachable from remote refs
	remoteCommits := make(map[plumbing.Hash]bool)
	for _, ref := range remoteRefs {
		if ref.Type() == plumbing.HashReference || ref.Name().IsBranch() {
			hash := ref.Hash()
			// Walk commits from this remote ref
			iter, err := r.repo.Log(&git.LogOptions{From: hash})
			if err != nil {
				continue
			}
			_ = iter.ForEach(func(c *object.Commit) error {
				remoteCommits[c.Hash] = true
				return nil
			})
		}
	}

	// Walk from local commit to find commits not in remote
	var ancestors []string
	iter, err := r.repo.Log(&git.LogOptions{From: localHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	err = iter.ForEach(func(c *object.Commit) error {
		if !remoteCommits[c.Hash] {
			ancestors = append(ancestors, c.Hash.String())
		} else {
			// Stop when we reach a commit that exists in remote
			return io.EOF
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	// Reverse to get topological order (oldest first)
	for i, j := 0, len(ancestors)-1; i < j; i, j = i+1, j-1 {
		ancestors[i], ancestors[j] = ancestors[j], ancestors[i]
	}

	return ancestors, nil
}

// GetRootCommits returns a set of root commits (commits with no parents) reachable from localSHA
func (r *Repository) GetRootCommits(localSHA string) (map[string]bool, error) {
	if r.repo == nil {
		return nil, errors.New("repository is not initialized")
	}

	localHash, err := r.resolveReference(localSHA)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve SHA: %w", err)
	}

	roots := make(map[string]bool)
	iter, err := r.repo.Log(&git.LogOptions{From: localHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}

	err = iter.ForEach(func(c *object.Commit) error {
		if c.NumParents() == 0 {
			roots[c.Hash.String()] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	return roots, nil
}

// GetParentCommit gets the parent hash of a commit
func (r *Repository) GetParentCommit(commitSHA string) (string, error) {
	if r.repo == nil {
		return "", errors.New("repository is not initialized")
	}

	hash, err := r.resolveReference(commitSHA)
	if err != nil {
		return "", fmt.Errorf("failed to resolve commit: %w", err)
	}

	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return "", fmt.Errorf("failed to get commit object: %w", err)
	}

	if commit.NumParents() == 0 {
		return "", fmt.Errorf("commit has no parent")
	}

	parent, err := commit.Parent(0)
	if err != nil {
		return "", fmt.Errorf("failed to get parent: %w", err)
	}

	return parent.Hash.String(), nil
}
