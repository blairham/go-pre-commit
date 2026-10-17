// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
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
	// corrupt the host index/object store. See git.ScrubProcessEnv. The
	// legacy hook still gets the environment git gave us, as upstream's does.
	hookEnv := os.Environ()
	git.ScrubProcessEnv()

	var opts hookImplFlags
	remaining, code, ok := parseArgs("hook-impl", &opts, args, args)
	if !ok {
		return code
	}
	var err error

	output.SetColorModeFromString(opts.Color)

	if err = adjustArgsAndChdir(rootArgs{Config: &opts.Config}); err != nil {
		return fatalError(err)
	}

	// Upstream's _run_legacy: refuse to recurse into a pre-commit script
	// saved as the legacy hook, read pre-push's refs up front (both the
	// legacy hook and the ref translation below need them), then run the
	// executable legacy hook, if any, with what we were given on stdin.
	if os.Getenv("PRE_COMMIT_RUNNING_LEGACY") != "" {
		fmt.Fprintf(os.Stderr, "bug: pre-commit's script is installed in migration mode\n"+
			"run `pre-commit install -f --hook-type %s` to fix this\n\n"+
			"Please report this bug at https://github.com/pre-commit/pre-commit/issues\n", opts.HookType)
		return 1
	}
	stdin := []byte{}
	if opts.HookType == "pre-push" {
		var err error
		if stdin, err = readPrePushStdin(); err != nil {
			return reportError(err)
		}
	}
	retv, err := runLegacyHook(opts.HookType, opts.HookDir, remaining, stdin, hookEnv)
	if err != nil {
		return unexpectedError(err)
	}

	if exit, stop := validateHookConfig(retv, opts.Config, opts.SkipOnMissingConfig); stop {
		return exit
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
			return reportError(err)
		}
		if pushArgs == nil {
			// Nothing being pushed needs checking (only deletions, or
			// commits the remote already has): upstream runs no hooks.
			return retv
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
	return retv | runCmd.Run(runArgs)
}

// validateHookConfig is upstream's _validate_config: with no config file, a
// hook installed with --skip-on-missing-config (or run under
// PRE_COMMIT_ALLOW_NO_CONFIG) says it is skipping and exits with the legacy
// hook's status; otherwise it explains how to proceed and fails.
func validateHookConfig(retv int, cfg string, skipOnMissingConfig bool) (exit int, stop bool) {
	if info, err := os.Stat(cfg); err == nil && info.Mode().IsRegular() {
		return 0, false
	}
	if skipOnMissingConfig || os.Getenv("PRE_COMMIT_ALLOW_NO_CONFIG") != "" {
		fmt.Printf("`%s` config file not found. Skipping `pre-commit`.\n", cfg)
		return retv, true
	}
	fmt.Printf("No %s file was found\n"+
		"- To temporarily silence this, run `PRE_COMMIT_ALLOW_NO_CONFIG=1 git ...`\n"+
		"- To permanently silence this, install pre-commit with the --allow-missing-config option\n"+
		"- To uninstall pre-commit run `pre-commit uninstall`\n", cfg)
	return 1, true
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

// runLegacyHook is the rest of upstream's _run_legacy: with no hook dir
// (git 2.54+ config-based hooks) or no executable <type>.legacy in it there
// is nothing to run; otherwise the legacy hook runs with stdin as its input,
// env plus PRE_COMMIT_RUNNING_LEGACY=1 as its environment, and its exit
// status becomes retv. err is a hook that could not be started at all.
func runLegacyHook(hookType, hookDir string, args []string, stdin []byte, env []string) (retv int, err error) {
	if hookDir == "" {
		return 0, nil
	}
	legacyPath := filepath.Join(hookDir, hookType+".legacy")
	if !fsutil.Executable(legacyPath) {
		return 0, nil
	}

	cmd := exec.Command(legacyPath, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(slices.Clone(env), "PRE_COMMIT_RUNNING_LEGACY=1")
	err = cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return 0, err
}
