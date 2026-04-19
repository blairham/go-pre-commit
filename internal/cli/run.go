package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/hook"
	"github.com/blairham/go-pre-commit/v4/internal/output"
	"github.com/blairham/go-pre-commit/v4/internal/repository"
	"github.com/blairham/go-pre-commit/v4/internal/staged"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// RunCommand implements the "run" command.
type RunCommand struct {
	Meta *Meta
}

type runFlags struct {
	GlobalFlags
	AllFiles        bool     `short:"a" long:"all-files" description:"Run on all files in the repo."`
	Files           []string `long:"files" description:"Specific filenames to run hooks on."`
	ShowDiffOnFail  bool     `long:"show-diff-on-failure" description:"When hooks fail, show the diff of changes."`
	HookStage       string   `long:"hook-stage" description:"The stage during which the hook is fired."`
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
	Verbose         bool     `short:"v" long:"verbose" description:"Produce hook output regardless of success."`
	FailFast        bool     `long:"fail-fast" description:"Stop running hooks after the first failure."`
	NoInstall       bool     `long:"no-install" description:"Skip automatic installation of hook environments."`
	Jobs            int      `short:"j" long:"jobs" description:"Number of jobs to run in parallel."`
}

func (c *RunCommand) Run(args []string) int {
	var opts runFlags
	opts.Jobs = runtime.NumCPU()

	p := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	remaining, err := p.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Handle deprecated flags.
	if opts.Source != "" && opts.FromRef == "" {
		opts.FromRef = opts.Source
	}
	if opts.Origin != "" && opts.ToRef == "" {
		opts.ToRef = opts.Origin
	}

	// At most one positional arg (hook-id).
	if len(remaining) > 1 {
		fmt.Fprintf(os.Stderr, "Error: expected at most 1 argument, got %d\n", len(remaining))
		return 1
	}

	output.SetColorModeFromString(opts.Color)

	// --files and --all-files are mutually exclusive.
	if opts.AllFiles && len(opts.Files) > 0 {
		fmt.Fprintf(os.Stderr, "Error: --all-files and --files are mutually exclusive\n")
		return 1
	}

	// Load config.
	cfg, err := config.LoadConfig(opts.Config)
	if err != nil {
		if os.Getenv("PRE_COMMIT_ALLOW_NO_CONFIG") != "" {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: failed to load config: %v\n", err)
		return 1
	}

	// Propagate fail_fast from CLI or config.
	if opts.FailFast {
		cfg.FailFast = true
	}

	// Get repository root.
	root, err := git.GetRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get git root: %v\n", err)
		return 1
	}

	// Set PRE_COMMIT=1.
	os.Setenv("PRE_COMMIT", "1")
	defer os.Unsetenv("PRE_COMMIT")

	// Initialize the store.
	s := store.New("")

	// Resolve hooks.
	resolver := repository.NewResolver(s, cfg)
	hooks, err := resolver.ResolveAll(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to resolve hooks: %v\n", err)
		return 1
	}

	// Determine files.
	var filenames []string
	noStash := os.Getenv("PRE_COMMIT_NO_STASH") != ""
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

	// Determine stage.
	stage := config.Stage(opts.HookStage)
	if stage == "" {
		stage = config.HookTypePreCommit
	}

	// Filter to a single hook if specified.
	var hookID string
	if len(remaining) > 0 {
		hookID = remaining[0]
	}

	// Determine if we need to stash.
	needsStash := !opts.AllFiles && len(opts.Files) == 0 && opts.FromRef == "" && opts.ToRef == "" && !noStash
	var stashMgr *staged.Manager
	if needsStash {
		hasUnstaged, _ := git.HasUnstagedChanges(root)
		if hasUnstaged {
			stashMgr = staged.NewManager(root)
			stashed, err := stashMgr.StashUnstaged()
			if !stashed || err != nil {
				output.Warn("Failed to stash unstaged changes: %v", err)
				stashMgr = nil
			}
		}
	}

	// Install environments (unless --no-install).
	if !opts.NoInstall {
		if err := hook.InstallEnvironments(context.Background(), hooks); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to install environments: %v\n", err)
			return 1
		}
	}

	// Run hooks.
	runner := hook.NewRunner(cfg, hooks, root)
	result := runner.Run(context.Background(), hook.RunOptions{
		HookID:                     hookID,
		HookStage:                  stage,
		Files:                      filenames,
		AllFiles:                   opts.AllFiles,
		Verbose:                    opts.Verbose,
		ShowDiff:                   opts.ShowDiffOnFail,
		Color:                      opts.Color,
		Jobs:                       opts.Jobs,
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

	// Restore stash.
	if stashMgr != nil {
		if err := stashMgr.Restore(); err != nil {
			output.Warn("Failed to restore unstaged changes: %v", err)
		}
	}

	hasFailures := result.Failed > 0 || result.Errors > 0

	// Show diff on failure if requested.
	if opts.ShowDiffOnFail && hasFailures {
		hook.ShowDiffOnFailure(opts.AllFiles)
	}

	if hasFailures {
		return 1
	}

	return 0
}

func (c *RunCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit run [options] [hook-id]

  Run hooks. If hook-id is given, only that hook is run, otherwise all hooks
  are run. If no files are specified, all staged files are used.

Options:

  -a, --all-files              Run on all files in the repo.
      --files=FILE             Specific filenames to run hooks on.
      --show-diff-on-failure   When hooks fail, show the diff of changes.
      --hook-stage=STAGE       The stage during which the hook is fired.
      --from-ref=REF           Ref to check revision changes.
      --to-ref=REF             Ref to check revision changes.
  -v, --verbose                Produce hook output regardless of success.
      --fail-fast              Stop running hooks after the first failure.
      --no-install             Skip automatic installation of hook environments.
  -j, --jobs=N                 Number of jobs to run in parallel.
  -c, --config=FILE            Path to alternate config file.
      --color=MODE             Whether to use color (auto, always, never).
`)
}

func (c *RunCommand) Synopsis() string {
	return "Run hooks"
}
