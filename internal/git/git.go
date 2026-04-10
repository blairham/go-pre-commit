// Package git provides helper functions for interacting with git repositories.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NoGitEnv returns the current environment with GIT_* variables removed
// (except for a safe allowlist). This matches the Python pre-commit's
// no_git_env() which prevents many subtle bugs:
//   - GIT_DIR: causes git clone to clone wrong thing
//   - GIT_INDEX_FILE: causes 'error invalid object' during commit
//   - GIT_WORK_TREE: exported by some git versions during hook execution
//
// See https://github.com/pre-commit/pre-commit/issues/300
func NoGitEnv() []string {
	allowed := map[string]bool{
		"GIT_EXEC_PATH":             true,
		"GIT_SSH":                   true,
		"GIT_SSH_COMMAND":           true,
		"GIT_SSL_CAINFO":            true,
		"GIT_SSL_NO_VERIFY":         true,
		"GIT_CONFIG_COUNT":          true,
		"GIT_HTTP_PROXY_AUTHMETHOD": true,
		"GIT_ALLOW_PROTOCOL":        true,
		"GIT_ASKPASS":               true,
	}
	var env []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if strings.HasPrefix(k, "GIT_") && !allowed[k] && !strings.HasPrefix(k, "GIT_CONFIG_KEY_") && !strings.HasPrefix(k, "GIT_CONFIG_VALUE_") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// CmdOutput runs a git command and returns its stdout.
func CmdOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Env = NoGitEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CmdOutputInDir runs a git command in a specific directory.
func CmdOutputInDir(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = NoGitEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s (in %s) failed: %w\nstderr: %s", strings.Join(args, " "), dir, err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RunInDir runs a git command in a directory, returning combined output.
func RunInDir(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetRoot returns the root directory of the current git repository.
func GetRoot() (string, error) {
	return CmdOutput("rev-parse", "--show-toplevel")
}

// GetRootInDir returns the root directory of a git repository from a given directory.
func GetRootInDir(dir string) (string, error) {
	return CmdOutputInDir(dir, "rev-parse", "--show-toplevel")
}

// GetGitDir returns the .git directory path.
func GetGitDir(root string) (string, error) {
	out, err := CmdOutputInDir(root, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) {
		return out, nil
	}
	return filepath.Join(root, out), nil
}

// GetGitCommonDir returns the git common directory (for worktrees).
func GetGitCommonDir(root string) (string, error) {
	out, err := CmdOutputInDir(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(out) {
		return out, nil
	}
	return filepath.Join(root, out), nil
}

// GetStagedFiles returns a list of staged file paths.
func GetStagedFiles() ([]string, error) {
	out, err := CmdOutput("diff", "--staged", "--name-only", "--diff-filter=ACMRT", "--no-ext-diff", "-z")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	// Split by null byte (from -z flag).
	files := strings.Split(out, "\x00")
	// Remove empty entries.
	var result []string
	for _, f := range files {
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// GetAllFiles returns all tracked files in the repository.
func GetAllFiles() ([]string, error) {
	out, err := CmdOutput("ls-files", "-z")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	files := strings.Split(out, "\x00")
	var result []string
	for _, f := range files {
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// GetChangedFiles returns files changed between two refs.
func GetChangedFiles(fromRef, toRef string) ([]string, error) {
	out, err := CmdOutput("diff", "--name-only", "--diff-filter=ACMRT", "--no-ext-diff", "-z", fromRef+"..."+toRef)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	files := strings.Split(out, "\x00")
	var result []string
	for _, f := range files {
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// GetDefaultBranch returns the default branch name.
func GetDefaultBranch(remote string) (string, error) {
	out, err := CmdOutput("symbolic-ref", fmt.Sprintf("refs/remotes/%s/HEAD", remote))
	if err != nil {
		return "", err
	}
	// Strip refs/remotes/origin/ prefix.
	prefix := fmt.Sprintf("refs/remotes/%s/", remote)
	return strings.TrimPrefix(out, prefix), nil
}

// Clone clones a repository.
func Clone(url, dest string, args ...string) error {
	cmdArgs := []string{"clone"}
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, url, dest)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ShallowClone clones a repository with depth 1.
func ShallowClone(url, dest, ref string) error {
	return Clone(url, dest, "--depth", "1", "--branch", ref, "--single-branch", "-c", "protocol.version=2")
}

// Fetch runs git fetch in a directory.
func Fetch(dir string, args ...string) error {
	cmdArgs := append([]string{"fetch"}, args...)
	return RunInDir(dir, cmdArgs...)
}

// FetchTags fetches tags from origin.
func FetchTags(dir string) error {
	return Fetch(dir, "origin", "--tags")
}

// Checkout checks out a ref in a directory.
func Checkout(dir, ref string) error {
	return RunInDir(dir, "checkout", ref)
}

// GetHeadSHA returns the HEAD SHA.
func GetHeadSHA(dir string) (string, error) {
	return CmdOutputInDir(dir, "rev-parse", "HEAD")
}

// GetLatestTag returns the latest tag.
func GetLatestTag(dir string) (string, error) {
	out, err := CmdOutputInDir(dir, "describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", err
	}
	return out, nil
}

// ListTags returns all tags sorted by version.
func ListTags(dir string) ([]string, error) {
	out, err := CmdOutputInDir(dir, "tag", "--sort", "version:refname")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

// GetTagSHA returns the SHA for a given tag.
func GetTagSHA(dir, tag string) (string, error) {
	return CmdOutputInDir(dir, "rev-parse", tag)
}

// Diff runs git diff and returns the output.
func Diff(args ...string) (string, error) {
	cmdArgs := append([]string{"diff"}, args...)
	return CmdOutput(cmdArgs...)
}

// DiffInDir runs git diff in a directory.
func DiffInDir(dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"diff"}, args...)
	return CmdOutputInDir(dir, cmdArgs...)
}

// Init initializes a new git repository.
func Init(dir string) error {
	return RunInDir(dir, "init")
}

// IsInsideWorkTree returns true if the current directory is inside a git work tree.
func IsInsideWorkTree() bool {
	out, err := CmdOutput("rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// IsInsideWorkTreeInDir checks if a directory is inside a git work tree.
func IsInsideWorkTreeInDir(dir string) bool {
	out, err := CmdOutputInDir(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// StashPush stashes changes.
func StashPush(dir string, args ...string) error {
	cmdArgs := append([]string{"stash", "push"}, args...)
	return RunInDir(dir, cmdArgs...)
}

// StashPop pops the stash.
func StashPop(dir string) error {
	return RunInDir(dir, "stash", "pop")
}

// CheckoutIndex checks out the index to a temp directory.
func CheckoutIndex(dir, dest string) error {
	cmd := exec.Command("git", "checkout-index", "-a", "--prefix="+dest+"/")
	cmd.Dir = dir
	return cmd.Run()
}

// HasUnstagedChanges checks if there are unstaged changes.
func HasUnstagedChanges(dir string) (bool, error) {
	out, err := CmdOutputInDir(dir, "diff", "--name-only")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// HasStagedChanges checks if there are staged changes.
func HasStagedChanges(dir string) (bool, error) {
	out, err := CmdOutputInDir(dir, "diff", "--staged", "--name-only")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// GetHooksDir returns the git hooks directory path.
func GetHooksDir(root ...string) (string, error) {
	var rootDir string
	if len(root) > 0 && root[0] != "" {
		rootDir = root[0]
	} else {
		var err error
		rootDir, err = GetRoot()
		if err != nil {
			return "", err
		}
	}

	// First try core.hooksPath config.
	out, err := CmdOutputInDir(rootDir, "config", "--get", "core.hooksPath")
	if err == nil && out != "" {
		if filepath.IsAbs(out) {
			return out, nil
		}
		return filepath.Join(rootDir, out), nil
	}

	// Fall back to .git/hooks.
	gitDir, err := GetGitDir(rootDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(gitDir, "hooks"), nil
}

// IntentToAddFiles returns files that were added with --intent-to-add.
func IntentToAddFiles() ([]string, error) {
	out, err := CmdOutput("diff", "--no-ext-diff", "--diff-filter=A", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var result []string
	for _, f := range strings.Split(out, "\x00") {
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// GetConflictedFiles returns files with merge conflicts.
func GetConflictedFiles() ([]string, error) {
	out, err := CmdOutput("diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var result []string
	for _, f := range strings.Split(out, "\x00") {
		if f != "" {
			result = append(result, f)
		}
	}
	return result, nil
}

// IsInMergeConflict checks if the repository is in a merge conflict state.
func IsInMergeConflict() bool {
	files, err := GetConflictedFiles()
	if err != nil {
		return false
	}
	return len(files) > 0
}

// HasCoreHookPathsSet checks if core.hooksPath is configured.
func HasCoreHookPathsSet() bool {
	out, err := CmdOutput("config", "--get", "core.hooksPath")
	return err == nil && strings.TrimSpace(out) != ""
}

// GetBestCandidateTag returns the tag that best describes the current HEAD.
func GetBestCandidateTag(dir string) (string, error) {
	out, err := CmdOutputInDir(dir, "describe", "--tags", "--exact-match", "HEAD")
	if err != nil {
		// No exact tag, try closest.
		out, err = CmdOutputInDir(dir, "describe", "--tags", "--abbrev=0", "HEAD")
		if err != nil {
			return "", err
		}
	}
	return out, nil
}

// WriteTree writes the current index as a tree object.
func WriteTree(dir string) (string, error) {
	return CmdOutputInDir(dir, "write-tree")
}

// DiffIndex shows changes between a tree-ish and the working tree.
func DiffIndex(dir, treeish string) (string, error) {
	return CmdOutputInDir(dir, "diff-index", "-p", treeish, "--")
}

// CheckoutIndexToDir checks out index contents to a directory.
func CheckoutIndexToDir(dir, dest string) error {
	cmd := exec.Command("git", "checkout-index", "--all", "--force", "--prefix="+dest+"/")
	cmd.Dir = dir
	cmd.Env = NoGitEnv()
	cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_WORK_TREE=%s", dest))
	return cmd.Run()
}

// ReadTree reads a tree object into the index.
func ReadTree(dir, treeish string) error {
	return RunInDir(dir, "read-tree", treeish)
}
