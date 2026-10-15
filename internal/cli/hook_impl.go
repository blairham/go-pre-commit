// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/output"
)

// HookImplCommand is the hidden hook-impl command.
// This is the command that the installed git hook scripts call back into.
// It translates git hook arguments into pre-commit run arguments.
type HookImplCommand struct {
	Meta *Meta
}

type hookImplFlags struct {
	Config              string `long:"config"                 default:".pre-commit-config.yaml" description:"Path to config file."`
	HookType            string `long:"hook-type"                                                description:"The hook type being run."        required:"true"`
	HookDir             string `long:"hook-dir"                                                 description:"The hook directory."`
	SkipOnMissingConfig bool   `long:"skip-on-missing-config"                                   description:"Skip if config file is missing."`
	Color               string `long:"color"                  default:"auto"                    description:"Whether to use color in output."`
}

func (c *HookImplCommand) Run(args []string) int {
	// Invoked by git from within a commit/checkout/etc., so GIT_DIR,
	// GIT_INDEX_FILE and GIT_WORK_TREE point at the host repo. Clear them up front
	// so no child git process (e.g. a hook-repo clone) can inherit them and
	// corrupt the host index/object store. See git.ScrubProcessEnv.
	git.ScrubProcessEnv()

	var opts hookImplFlags
	remaining, err := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash).ParseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	output.SetColorModeFromString(opts.Color)

	if err = adjustArgsAndChdir(rootArgs{Config: &opts.Config}); err != nil {
		return fatalError(err)
	}

	// Check if config exists when --skip-on-missing-config is set.
	if opts.SkipOnMissingConfig {
		if _, err := os.Stat(opts.Config); os.IsNotExist(err) {
			return 0
		}
	}

	// Check PRE_COMMIT_ALLOW_NO_CONFIG.
	if os.Getenv("PRE_COMMIT_ALLOW_NO_CONFIG") != "" {
		if _, err := os.Stat(opts.Config); os.IsNotExist(err) {
			return 0
		}
	}

	// pre-push refs arrive on stdin, and both the legacy hook and the ref
	// translation below need them, so read it once up front (upstream's
	// _run_legacy does the same). Handing the legacy hook os.Stdin itself
	// would let it consume the refs before we see them.
	var stdin []byte
	if opts.HookType == "pre-push" {
		var err error
		if stdin, err = readPrePushStdin(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}

	// Run legacy hook first (e.g., .pre-commit.legacy).
	if err := runLegacyHook(opts.HookType, opts.HookDir, remaining, stdin); err != nil {
		output.Warn("Legacy hook failed: %v", err)
	}

	// Build args for the run command based on hook type.
	runArgs := []string{}

	// Add the config.
	if opts.Config != "" {
		runArgs = append(runArgs, "--config", opts.Config)
	}

	// Add stage.
	runArgs = append(runArgs, "--hook-stage", opts.HookType)

	// Map hook-type-specific arguments.
	switch opts.HookType {
	case "pre-commit", "pre-merge-commit":
		// No additional args.

	case "pre-push":
		// Args: <remote-name> <remote-url>
		if len(remaining) != 2 {
			fmt.Fprintf(os.Stderr, "hook-impl for pre-push expected 2 arguments but got %d: %q\n", len(remaining), remaining)
			return 1
		}
		pushArgs, err := prePushArgs("", remaining[0], remaining[1], stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		if pushArgs == nil {
			// Nothing being pushed needs checking (only deletions, or
			// commits the remote already has): upstream runs no hooks.
			return 0
		}
		runArgs = append(runArgs, pushArgs...)

	case "commit-msg":
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--commit-msg-filename", remaining[0])
		}

	case "prepare-commit-msg":
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--commit-msg-filename", remaining[0])
		}
		if len(remaining) >= 2 {
			runArgs = append(runArgs, "--prepare-commit-message-source", remaining[1])
		}
		if len(remaining) >= 3 {
			runArgs = append(runArgs, "--commit-object-name", remaining[2])
		}

	case "post-checkout":
		if len(remaining) >= 3 {
			runArgs = append(runArgs, "--from-ref", remaining[0])
			runArgs = append(runArgs, "--to-ref", remaining[1])
			runArgs = append(runArgs, "--checkout-type", remaining[2])
		}
		runArgs = append(runArgs, "--all-files")

	case "post-commit":
		runArgs = append(runArgs, "--all-files")

	case "post-merge":
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--is-squash-merge", remaining[0])
		}
		runArgs = append(runArgs, "--all-files")

	case "post-rewrite":
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--rewrite-command", remaining[0])
		}
		runArgs = append(runArgs, "--all-files")

	case "pre-rebase":
		// Args: <upstream> [<branch>]
		if len(remaining) >= 1 {
			runArgs = append(runArgs, "--pre-rebase-upstream", remaining[0])
		}
		if len(remaining) >= 2 {
			runArgs = append(runArgs, "--pre-rebase-branch", remaining[1])
		}
	}

	// Execute the run command directly.
	runCmd := &RunCommand{Meta: c.Meta}
	return runCmd.Run(runArgs)
}

func (c *HookImplCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit hook-impl [options] [-- args...]

  Implementation of git hooks (internal use only).

Options:

      --config=FILE              Path to config file.
      --hook-type=TYPE           The hook type being run.
      --hook-dir=DIR             The hook directory.
      --skip-on-missing-config   Skip if config file is missing.
      --color=MODE               Whether to use color (auto, always, never).
`)
}

func (c *HookImplCommand) Synopsis() string {
	return "Implementation of git hooks (internal use only)"
}

// readPrePushStdin reads the ref lines git feeds a pre-push hook on stdin.
func readPrePushStdin() ([]byte, error) {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return nil, nil //nolint:nilerr // no piped input: nothing to read
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading pre-push refs from stdin: %w", err)
	}
	return data, nil
}

// z40 is the all-zeros object name git uses for a ref that does not exist on
// one side of a push.
const z40 = "0000000000000000000000000000000000000000"

// prePushArgs is a port of upstream's _pre_push_ns (hook_impl.py). For each
// `<local ref> <local sha> <remote ref> <remote sha>` line git passes on
// stdin it decides which commits are being pushed:
//
//   - a deletion (local sha all zeros) is skipped;
//   - when the remote sha is a commit we have, check remote_sha..local_sha;
//   - otherwise (a new branch, or a remote tip we never fetched) walk the
//     commits not on any ref of <remote>; none means nothing new is pushed and
//     the line is skipped, a root commit among them means the whole tree is
//     new (--all-files), else check from the first new commit's parent.
//
// The first line that yields something wins. A nil result with a nil error
// means nothing needs checking, and upstream then runs no hooks at all.
// dir is the repository to run git in ("" for the current directory).
func prePushArgs(dir, remoteName, remoteURL string, stdin []byte) ([]string, error) {
	for _, line := range strings.Split(string(stdin), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		// Upstream splits from the right (rsplit(maxsplit=3)), so only the
		// local ref may contain whitespace.
		n := len(parts)
		localBranch := strings.Join(parts[:n-3], " ")
		localSHA, remoteBranch, remoteSHA := parts[n-3], parts[n-2], parts[n-1]

		common := []string{
			"--remote-name", remoteName, "--remote-url", remoteURL,
			"--remote-branch", remoteBranch, "--local-branch", localBranch,
		}

		if localSHA == z40 {
			continue
		}
		if remoteSHA != z40 && revExists(dir, remoteSHA) {
			return append([]string{"--from-ref", remoteSHA, "--to-ref", localSHA}, common...), nil
		}

		// Ancestors not found in the remote.
		ancestors, err := gitOutput(dir, "rev-list", localSHA, "--topo-order", "--reverse",
			"--not", "--remotes="+remoteName)
		if err != nil {
			return nil, err
		}
		if ancestors == "" {
			continue
		}
		firstAncestor := strings.SplitN(ancestors, "\n", 2)[0]
		roots, err := gitOutput(dir, "rev-list", "--max-parents=0", localSHA)
		if err != nil {
			return nil, err
		}
		if slices.Contains(strings.Split(roots, "\n"), firstAncestor) {
			// Pushing the whole tree, root commit included.
			return append([]string{"--all-files"}, common...), nil
		}
		source, err := gitOutput(dir, "rev-parse", firstAncestor+"^")
		if err != nil {
			return nil, err
		}
		return append([]string{"--from-ref", source, "--to-ref", localSHA}, common...), nil
	}
	return nil, nil
}

// revExists reports whether rev names a commit in the local object store,
// as upstream's _rev_exists does with `git cat-file -e <rev>^{commit}`
// (pre-commit 4.6.1, pre-commit/pre-commit#3726). Like upstream, git's
// stderr is not captured.
func revExists(dir, rev string) bool {
	cmd := exec.Command("git", "cat-file", "-e", rev+"^{commit}")
	cmd.Dir = dir
	cmd.Env = git.NoGitEnv()
	cmd.Stderr = os.Stderr
	return cmd.Run() == nil
}

// gitOutput runs git in dir and returns its trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	if dir == "" {
		return git.CmdOutput(args...)
	}
	return git.CmdOutputInDir(dir, args...)
}

// runLegacyHook runs the legacy hook script if it exists.
// stdin is what the hook itself was given on stdin when it had to be read up
// front (pre-push); otherwise the legacy hook inherits ours.
func runLegacyHook(hookType, hookDir string, args []string, stdin []byte) error {
	legacyPath := filepath.Join(legacyHookDir(hookDir), hookType+".legacy")
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return nil // No legacy hook.
	}

	cmd := exec.Command(legacyPath, args...)
	cmd.Stdin = os.Stdin
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// legacyHookDir resolves where a saved .legacy hook lives: the --hook-dir git
// invoked us from when available (handles core.hooksPath and template dirs),
// otherwise $GIT_DIR/hooks.
func legacyHookDir(hookDir string) string {
	if hookDir != "" {
		return hookDir
	}
	gitDir := os.Getenv("GIT_DIR")
	if gitDir == "" {
		gitDir = ".git"
	}
	return filepath.Join(gitDir, "hooks")
}
