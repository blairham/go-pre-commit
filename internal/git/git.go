// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package git provides helper functions for interacting with git repositories.
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/output"
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
		if strings.HasPrefix(k, "GIT_") && !allowed[k] && !strings.HasPrefix(k, "GIT_CONFIG_KEY_") &&
			!strings.HasPrefix(k, "GIT_CONFIG_VALUE_") {
			continue
		}
		env = append(env, e)
	}
	return env
}

// hostIndexFile is the GIT_INDEX_FILE git exported when it invoked this hook,
// made absolute, or "" when there was none. ScrubProcessEnv records it before
// clearing the variable.
var hostIndexFile string

// ScrubProcessEnv clears the git-managed environment variables that git exports
// when it invokes a hook (GIT_DIR, GIT_INDEX_FILE, GIT_WORK_TREE). They point at
// the *host* repo. If any child git process inherits them — a clone, a checkout,
// a goroutine-spawned command, or anything that forgets NoGitEnv() — it operates
// on the host repo instead of its own and corrupts the host index/object store
// (e.g. a hook-repo clone whose checkout lands entries in the host's index,
// producing "fatal: unable to read <object>").
//
// NoGitEnv() scrubs these per command, but call this once at hook entry as a
// belt-and-suspenders backstop so the leak is impossible regardless of the exec
// path. This only affects this hook process and its children; the parent
// `git commit` retains its own copy and completes normally.
//
// GIT_INDEX_FILE is recorded first (see HostIndexEnv), because it is not
// always the default index: `git commit -a` and `git commit <paths>` hold the
// default index's lock for the whole hook and point GIT_INDEX_FILE at the index
// the commit will record. Commands that read or stash the host's staged state
// must use that index — against the default one they see the wrong staged
// files, and anything that writes it (write-tree, checkout) dies on the held
// index.lock.
func ScrubProcessEnv() {
	if v := os.Getenv("GIT_INDEX_FILE"); v != "" {
		// git runs hooks from the top of the working tree and may export a
		// path relative to it; pin it before anything changes directory.
		if abs, err := filepath.Abs(v); err == nil {
			v = abs
		}
		hostIndexFile = v
	}
	for _, k := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE"} {
		_ = os.Unsetenv(k)
	}
}

// SetHostIndexFile sets the index HostIndexEnv points at; "" means the
// repository's default index. ScrubProcessEnv sets it from the hook's
// environment; tests use this to set or reset it.
func SetHostIndexFile(path string) { hostIndexFile = path }

// HostIndexEnv is NoGitEnv plus the GIT_INDEX_FILE git handed the hook, if
// any. Use it for commands that read or stash the host repo's staged state —
// listing staged files, diffing the working tree, write-tree, checkout — and
// never for commands aimed at another repository (hook-repo clones), which is
// what NoGitEnv protects. Upstream pre-commit draws the same line: it strips
// GIT_INDEX_FILE only for commands that touch other repos.
func HostIndexEnv() []string {
	env := NoGitEnv()
	if hostIndexFile != "" {
		env = append(env, "GIT_INDEX_FILE="+hostIndexFile)
	}
	return env
}

// hostOutput runs a git command against the host repo's index (HostIndexEnv)
// in dir ("" for the current directory) and returns its untrimmed stdout.
func hostOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = HostIndexEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s failed: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

// splitNul splits -z output into its non-empty entries.
func splitNul(out string) []string {
	var result []string
	for _, f := range strings.Split(out, "\x00") {
		if f != "" {
			result = append(result, f)
		}
	}
	return result
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
	cmd.Env = NoGitEnv()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// GetRoot returns the root directory of the current git repository.
func GetRoot() (string, error) {
	return CmdOutput("rev-parse", "--show-toplevel")
}

// FatalError is a failure upstream raises as its FatalError, worded as it
// words it. Those messages end in punctuation, which errors.New strings
// conventionally do not.
type FatalError string

func (e FatalError) Error() string { return string(e) }

// The two failures upstream's git.get_root reports.
const (
	ErrNoRepository FatalError = "git failed. Is it installed, and are you in a Git repository directory?"
	ErrInsideGitDir FatalError = "git toplevel unexpectedly empty! make sure you are not " +
		"inside the `.git` directory of your repository."
)

// Toplevel ports upstream's git.get_root, which pre-commit's main uses to
// find the directory it changes into before running any command that needs
// a repository. It is relative to cwd, the physical working directory, and
// deliberately not `--show-toplevel`: upstream notes that on Windows that
// form exposes the volume behind a drive mapped with SUBST, and asks for
// `--show-cdup` instead, which also means it must reject being run from
// inside the .git directory, where the answer is empty.
//
// It runs git with the caller's environment as upstream's cmd_output does,
// not NoGitEnv: a GIT_DIR or GIT_WORK_TREE the user exported is meant to
// decide which repository this is.
func Toplevel(cwd string) (string, error) {
	run := func(args ...string) (string, error) {
		var stdout bytes.Buffer
		cmd := exec.Command("git", args...)
		cmd.Dir = cwd
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			return "", err
		}
		return strings.TrimSpace(stdout.String()), nil
	}
	cdup, err := run("rev-parse", "--show-cdup")
	if err != nil {
		return "", ErrNoRepository
	}
	insideGitDir, err := run("rev-parse", "--is-inside-git-dir")
	if err != nil {
		return "", ErrNoRepository
	}
	if insideGitDir != "false" {
		return "", ErrInsideGitDir
	}
	return filepath.Join(cwd, cdup), nil
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

// HeadRev returns the commit a remote's HEAD points at, as upstream's
// git.head_rev does with `git ls-remote --exit-code <remote> HEAD`.
func HeadRev(remote string) (string, error) {
	out, err := OutputCaptured("", "ls-remote", "--exit-code", remote, "HEAD")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", fmt.Errorf("git ls-remote %s HEAD returned nothing", remote)
	}
	return fields[0], nil
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
	out, err := hostOutput("", "diff", "--staged", "--name-only", "--diff-filter=ACMRT", "--no-ext-diff", "-z")
	if err != nil {
		return nil, err
	}
	return splitNul(out), nil
}

// GetAllFiles returns all tracked files in the repository.
//
// --deduplicate (git >= 2.31) lists a path once even when an unresolved merge
// conflict leaves it in the index at several stages, as upstream 4.6.1 does.
func GetAllFiles() ([]string, error) {
	out, err := hostOutput("", "ls-files", "-z", "--deduplicate")
	if err != nil {
		return nil, err
	}
	return splitNul(out), nil
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

// NoFSMonitor is upstream's git.NO_FS_MONITOR: config that keeps git's
// builtin fsmonitor daemon from being started for a throwaway repository.
var NoFSMonitor = []string{"-c", "core.useBuiltinFSMonitor=false"}

// CalledProcessError is a failed command whose output was captured rather
// than shown, the way upstream's util.CalledProcessError carries it. Its
// message is upstream's byte for byte, so git's own explanation of a failure
// reaches the user only in the error.
type CalledProcessError struct {
	Cmd        []string
	Stdout     []byte
	Stderr     []byte
	ReturnCode int
}

func (e *CalledProcessError) Error() string {
	indentOrNone := func(part []byte) string {
		if len(part) == 0 {
			return " (none)"
		}
		return strings.TrimRight("\n    "+strings.ReplaceAll(string(part), "\n", "\n    "), " \t\r\n\v\f")
	}
	quoted := make([]string, len(e.Cmd))
	for i, a := range e.Cmd {
		quoted[i] = pyRepr(a)
	}
	cmd := "(" + strings.Join(quoted, ", ")
	if len(quoted) == 1 {
		cmd += ","
	}
	cmd += ")"
	return fmt.Sprintf("command: %s\nreturn code: %d\nstdout:%s\nstderr:%s",
		cmd, e.ReturnCode, indentOrNone(e.Stdout), indentOrNone(e.Stderr))
}

// pyRepr renders s the way Python's repr renders a str, which is how
// upstream prints each argument of a failed command.
func pyRepr(s string) string { return output.PyRepr(s) }

// RunCaptured runs git in dir under NoGitEnv with its output captured, as
// upstream's cmd_output_b does. A non-zero exit is a *CalledProcessError
// holding what git printed; on success the output is discarded.
func RunCaptured(dir string, args ...string) error {
	_, err := runCaptured(dir, NoGitEnv(), args)
	return err
}

// RunCapturedEnv is RunCaptured with env in place of NoGitEnv, for the
// commands upstream runs with a GIT_INDEX_FILE of their own.
func RunCapturedEnv(dir string, env []string, args ...string) error {
	_, err := runCaptured(dir, env, args)
	return err
}

// OutputCaptured is upstream's cmd_output: RunCaptured that returns git's
// stdout, untrimmed, on success.
func OutputCaptured(dir string, args ...string) (string, error) {
	out, err := runCaptured(dir, NoGitEnv(), args)
	return string(out), err
}

func runCaptured(dir string, env, args []string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	// Upstream reports the resolved executable, not the bare name.
	return nil, &CalledProcessError{
		Cmd:        append([]string{cmd.Path}, args...),
		ReturnCode: exitErr.ExitCode(),
		Stdout:     stdout.Bytes(),
		Stderr:     stderr.Bytes(),
	}
}

// HasDiff is upstream's git.has_diff: whether `git diff --quiet` with args
// reports a difference (exit 1) in repo. Any other failure is no difference.
func HasDiff(repo string, args ...string) bool {
	cmdArgs := append([]string{"diff", "--quiet", "--no-ext-diff"}, args...)
	err := RunCaptured(repo, cmdArgs...)
	var cpe *CalledProcessError
	return errors.As(err, &cpe) && cpe.ReturnCode == 1
}

// Path is upstream's git.git_path: `git rev-parse --git-path name` in
// repo, joined onto repo.
func Path(repo, name string) (string, error) {
	out, err := OutputCaptured(repo, "rev-parse", "--git-path", name)
	if err != nil {
		return "", err
	}
	return filepath.Join(repo, strings.TrimSpace(out)), nil
}

// StagedFilesIn is upstream's git.get_staged_files(cwd=dir): every staged
// path except deletions.
func StagedFilesIn(dir string) ([]string, error) {
	out, err := OutputCaptured(dir, "diff", "--staged", "--name-only", "--no-ext-diff", "-z", "--diff-filter=ACMRTUXB")
	if err != nil {
		return nil, err
	}
	return splitNul(out), nil
}

// Commit is upstream's git.commit: a commit of the index in repo as
// pre-commit's own identity, unsigned and without running hooks.
func Commit(repo string) error {
	const name, email = "pre-commit", "asottile+pre-commit@umich.edu"
	env := append(NoGitEnv(),
		"GIT_AUTHOR_NAME="+name, "GIT_COMMITTER_NAME="+name,
		"GIT_AUTHOR_EMAIL="+email, "GIT_COMMITTER_EMAIL="+email,
	)
	return RunCapturedEnv(repo, env, "commit", "--no-edit", "--no-gpg-sign", "-n", "-minit")
}

// BestCandidateTag is upstream's git.get_best_candidate_tag: of the tags
// pointing at rev, the first that looks like a version (has a '.'), else rev.
func BestCandidateTag(rev, gitRepo string) (string, error) {
	args := append(append([]string{}, NoFSMonitor...), "tag", "--points-at", rev)
	out, err := OutputCaptured(gitRepo, args...)
	if err != nil {
		return "", err
	}
	for _, tag := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.Contains(tag, ".") {
			return tag, nil
		}
	}
	return rev, nil
}

// InitRepo ports upstream's git.init_repo: an empty repository at path, with
// no template (so the user's template hooks cannot recurse into it), whose
// origin is remote. A remote that is a local directory is made absolute, since
// the commands that follow run from inside path.
func InitRepo(path, remote string) error {
	if info, err := os.Stat(remote); err == nil && info.IsDir() {
		if abs, err := filepath.Abs(remote); err == nil {
			remote = abs
		}
	}
	initArgs := append(append([]string{}, NoFSMonitor...), "init", "--template=", path)
	if err := RunCaptured("", initArgs...); err != nil {
		return err
	}
	remoteArgs := append(append([]string{}, NoFSMonitor...), "remote", "add", "origin", remote)
	return RunCaptured(path, remoteArgs...)
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
	cmd.Env = NoGitEnv()
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

	// Fall back to the common dir's hooks. In a linked worktree --git-dir is
	// .git/worktrees/<name>, which git never reads hooks from; upstream joins
	// get_git_common_dir() for the same reason.
	commonDir, err := GetGitCommonDir(rootDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "hooks"), nil
}

// IntentToAddFiles returns files in the host index that were added with
// --intent-to-add.
func IntentToAddFiles(dir string) ([]string, error) {
	out, err := hostOutput(dir, "diff", "--no-ext-diff", "--ignore-submodules", "--diff-filter=A", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	return splitNul(out), nil
}

// GetConflictedFiles returns files with merge conflicts.
func GetConflictedFiles() ([]string, error) {
	out, err := hostOutput("", "diff", "--name-only", "--diff-filter=U", "-z")
	if err != nil {
		return nil, err
	}
	return splitNul(out), nil
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

// WriteTree writes the host index as a tree object.
func WriteTree(dir string) (string, error) {
	out, err := hostOutput(dir, "write-tree")
	return strings.TrimSpace(out), err
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

// ReadTree reads a tree object into the host index.
func ReadTree(dir, treeish string) error {
	_, err := hostOutput(dir, "read-tree", treeish)
	return err
}
