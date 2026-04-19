package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/hook"
	"github.com/blairham/go-pre-commit/v4/internal/output"
	"github.com/blairham/go-pre-commit/v4/internal/repository"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// TryRepoCommand implements the "try-repo" command.
type TryRepoCommand struct {
	Meta *Meta
}

type tryRepoFlags struct {
	GlobalFlags
	Ref             string   `long:"ref" description:"Manually select a ref to run against. Otherwise uses HEAD."`
	Rev             string   `long:"rev" description:"(DEPRECATED: use --ref) Manually select a rev to run against."`
	AllFiles        bool     `short:"a" long:"all-files" description:"Run on all files in the repo."`
	Files           []string `long:"files" description:"Specific filenames to run hooks on."`
	Verbose         bool     `short:"v" long:"verbose" description:"Produce hook output regardless of success."`
	HookStage       string   `long:"hook-stage" description:"The stage during which the hook runs."`
	ShowDiffOnFail  bool     `long:"show-diff-on-failure" description:"When hooks fail, show the diff of changes."`
	FailFast        bool     `long:"fail-fast" description:"Stop running hooks after the first failure."`
	FromRef         string   `long:"from-ref" description:"Ref to check revision changes."`
	ToRef           string   `long:"to-ref" description:"Ref to check revision changes."`
	Source          string   `short:"s" long:"source" description:"(DEPRECATED: use --from-ref) Ref to check revision changes."`
	Origin          string   `short:"o" long:"origin" description:"(DEPRECATED: use --to-ref) Ref to check revision changes."`
	CommitMsgFn     string   `long:"commit-msg-filename" description:"Filename to check when running during commit-msg."`
	PrepareMsg      string   `long:"prepare-commit-message-source" description:"Source for prepare-commit-msg hook."`
	CommitObjName   string   `long:"commit-object-name" description:"Commit object name for prepare-commit-msg hook."`
	RemoteURL       string   `long:"remote-url" description:"Remote URL for pre-push hook."`
	RemoteName      string   `long:"remote-name" description:"Remote name for pre-push hook."`
	RemoteBranch    string   `long:"remote-branch" description:"Remote branch for pre-push hook."`
	LocalBranch     string   `long:"local-branch" description:"Local branch for pre-push hook."`
	CheckoutType    string   `long:"checkout-type" description:"Checkout type for post-checkout hook."`
	IsSquash        string   `long:"is-squash-merge" description:"Whether the merge is a squash merge."`
	RewriteCmd      string   `long:"rewrite-command" description:"Rewrite command for post-rewrite hook."`
	PreRebaseUp     string   `long:"pre-rebase-upstream" description:"Upstream from which the series was forked."`
	PreRebaseBranch string   `long:"pre-rebase-branch" description:"Branch being rebased."`
}

func (c *TryRepoCommand) Run(args []string) int {
	var opts tryRepoFlags
	remaining, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if len(remaining) < 1 || len(remaining) > 2 {
		fmt.Fprintf(os.Stderr, "Error: expected 1 or 2 arguments (REPO [hook-id]), got %d\n", len(remaining))
		return 1
	}

	// Handle deprecated flags.
	if opts.Rev != "" && opts.Ref == "" {
		opts.Ref = opts.Rev
	}
	if opts.Source != "" && opts.FromRef == "" {
		opts.FromRef = opts.Source
	}
	if opts.Origin != "" && opts.ToRef == "" {
		opts.ToRef = opts.Origin
	}

	output.SetColorModeFromString(opts.Color)

	repoURL := remaining[0]
	var hookID string
	if len(remaining) > 1 {
		hookID = remaining[1]
	}

	// Determine ref.
	tryRef := opts.Ref
	if tryRef == "" {
		tryRef = "HEAD"
	}

	s := store.New("")

	// Build a temporary config.
	tryConfig := &config.Config{
		Repos: []config.RepoConfig{
			{
				Repo: repoURL,
				Rev:  tryRef,
			},
		},
	}

	// If it's a local path, create a shadow clone with uncommitted changes.
	var hooks []*hook.Hook
	var cleanupDir string
	if isLocalPath(repoURL) {
		var repoDir string
		repoDir, cleanupDir, err = shadowCloneLocal(repoURL, tryRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to prepare local repo: %v\n", err)
			return 1
		}
		if cleanupDir != "" {
			defer os.RemoveAll(cleanupDir)
		}

		manifestPath := filepath.Join(repoDir, config.ManifestFile)
		var manifest []config.ManifestHook
		manifest, err = config.LoadManifest(manifestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load manifest from %s: %v\n", repoDir, err)
			return 1
		}
		for i := range manifest {
			h := hook.FromManifestHook(&manifest[i])
			h.Repo = repoURL
			h.RepoDir = repoDir
			hooks = append(hooks, h)
		}
	} else {
		resolver := repository.NewResolver(s, tryConfig)
		hooks, err = resolver.ResolveAll(context.Background(), tryConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to resolve hooks: %v\n", err)
			return 1
		}
	}

	// Determine files.
	var filenames []string
	if opts.AllFiles {
		filenames, err = git.GetAllFiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get all files: %v\n", err)
			return 1
		}
	} else if len(opts.Files) > 0 {
		filenames = opts.Files
	} else if opts.FromRef != "" && opts.ToRef != "" {
		filenames, err = git.GetChangedFiles(opts.FromRef, opts.ToRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get changed files: %v\n", err)
			return 1
		}
	} else {
		filenames, err = git.GetStagedFiles()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to get staged files: %v\n", err)
			return 1
		}
	}

	stage := config.Stage(opts.HookStage)
	if stage == "" {
		stage = config.HookTypePreCommit
	}

	// Build a minimal config for the runner.
	runCfg := config.DefaultConfig()
	if opts.FailFast {
		runCfg.FailFast = true
	}

	root, _ := git.GetRoot()
	runner := hook.NewRunner(runCfg, hooks, root)
	result := runner.Run(context.Background(), hook.RunOptions{
		HookID:                     hookID,
		HookStage:                  stage,
		Files:                      filenames,
		AllFiles:                   opts.AllFiles,
		Verbose:                    opts.Verbose,
		ShowDiff:                   opts.ShowDiffOnFail,
		Color:                      opts.Color,
		FromRef:                    opts.FromRef,
		ToRef:                      opts.ToRef,
		CommitMsgFilename:          opts.CommitMsgFn,
		PrepareCommitMessageSource: opts.PrepareMsg,
		CommitObjectName:           opts.CommitObjName,
		RemoteName:                 opts.RemoteName,
		RemoteURL:                  opts.RemoteURL,
		RemoteBranch:               opts.RemoteBranch,
		LocalBranch:                opts.LocalBranch,
		CheckoutType:               opts.CheckoutType,
		IsSquashMerge:              opts.IsSquash,
		RewriteCommand:             opts.RewriteCmd,
		PreRebaseUpstream:          opts.PreRebaseUp,
		PreRebaseBranch:            opts.PreRebaseBranch,
	})

	hasFailures := result.Failed > 0 || result.Errors > 0

	if opts.ShowDiffOnFail && hasFailures {
		hook.ShowDiffOnFailure(opts.AllFiles)
	}

	if hasFailures {
		return 1
	}

	return 0
}

func (c *TryRepoCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit try-repo [options] REPO [hook-id]

  Try the hooks in a repository, useful for developing new hooks.
  If REPO is a local path, the hooks are run from the local directory
  (including uncommitted changes via a shadow clone).

Options:

      --ref=REF                  Manually select a ref to run against (default: HEAD).
  -a, --all-files                Run on all files in the repo.
      --files=FILE               Specific filenames to run hooks on.
  -v, --verbose                  Produce hook output regardless of success.
      --hook-stage=STAGE         The stage during which the hook runs.
      --show-diff-on-failure     When hooks fail, show the diff of changes.
      --fail-fast                Stop running hooks after the first failure.
      --from-ref=REF             Ref to check revision changes.
      --to-ref=REF               Ref to check revision changes.
  -c, --config=FILE              Path to alternate config file.
      --color=MODE               Whether to use color (auto, always, never).
`)
}

func (c *TryRepoCommand) Synopsis() string {
	return "Try the hooks in a repository"
}

func isLocalPath(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// shadowCloneLocal creates a shadow clone of a local repository so that
// uncommitted changes (staged and unstaged) are included when testing hooks.
// This matches Python pre-commit's try-repo behavior for local repos.
// Returns (repoDir, cleanupDir, error). cleanupDir should be removed when done.
func shadowCloneLocal(localPath, ref string) (string, string, error) {
	// Resolve to absolute path.
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return "", "", err
	}

	// Check if it's a git repo.
	if !git.IsInsideWorkTreeInDir(absPath) {
		// Not a git repo, just use directly.
		return absPath, "", nil
	}

	// Check for any staged or unstaged changes.
	hasStagedChanges, _ := git.HasStagedChanges(absPath)
	hasUnstagedChanges, _ := git.HasUnstagedChanges(absPath)

	if !hasStagedChanges && !hasUnstagedChanges {
		// No local changes, use directly.
		return absPath, "", nil
	}

	// Create a shadow clone to capture uncommitted changes.
	tmpDir, err := os.MkdirTemp("", "pre-commit-try-repo-*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Clone the local repo.
	if err := git.Clone(absPath, tmpDir, "--no-checkout"); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("failed to clone local repo: %w", err)
	}

	// Checkout the target ref.
	if err := git.Checkout(tmpDir, ref); err != nil {
		os.RemoveAll(tmpDir)
		return "", "", fmt.Errorf("failed to checkout %s: %w", ref, err)
	}

	// Apply staged changes.
	if hasStagedChanges {
		diffCmd := exec.Command("git", "diff", "--staged", "--binary")
		diffCmd.Dir = absPath
		patch, err := diffCmd.Output()
		if err == nil && len(patch) > 0 {
			applyCmd := exec.Command("git", "apply", "--allow-empty")
			applyCmd.Dir = tmpDir
			applyCmd.Stdin = bytes.NewReader(patch)
			_ = applyCmd.Run()
		}
	}

	// Apply unstaged changes.
	if hasUnstagedChanges {
		diffCmd := exec.Command("git", "diff", "--binary")
		diffCmd.Dir = absPath
		patch, err := diffCmd.Output()
		if err == nil && len(patch) > 0 {
			applyCmd := exec.Command("git", "apply", "--allow-empty")
			applyCmd.Dir = tmpDir
			applyCmd.Stdin = bytes.NewReader(patch)
			_ = applyCmd.Run()
		}
	}

	// Also copy untracked files.
	untrackedCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard", "-z")
	untrackedCmd.Dir = absPath
	untrackedOut, err := untrackedCmd.Output()
	if err == nil && len(untrackedOut) > 0 {
		for _, f := range splitNullTerminated(string(untrackedOut)) {
			src := filepath.Join(absPath, f)
			dst := filepath.Join(tmpDir, f)
			os.MkdirAll(filepath.Dir(dst), 0o755)
			data, err := os.ReadFile(src)
			if err == nil {
				os.WriteFile(dst, data, 0o644)
			}
		}
	}

	return tmpDir, tmpDir, nil
}

func splitNullTerminated(s string) []string {
	var result []string
	for _, part := range splitByNull(s) {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitByNull(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == 0 {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
