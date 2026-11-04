// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"strings"

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
	AllFiles        bool     `short:"a" long:"all-files"                     description:"Run on all files in the repo."`
	Files           []string `          long:"files"                         description:"Specific filenames to run hooks on."`
	ShowDiffOnFail  bool     `          long:"show-diff-on-failure"          description:"When hooks fail, show the diff of changes."`
	HookStage       string   `          long:"hook-stage"                    description:"The stage during which the hook is fired."`
	FromRef         string   `          long:"from-ref"                      description:"Ref to check revision changes."`
	ToRef           string   `          long:"to-ref"                        description:"Ref to check revision changes."`
	Source          string   `short:"s" long:"source"                        description:"(DEPRECATED: use --from-ref) Ref to check revision changes."`
	Origin          string   `short:"o" long:"origin"                        description:"(DEPRECATED: use --to-ref) Ref to check revision changes."`
	CommitMsgFn     string   `          long:"commit-msg-filename"           description:"Filename to check when running during commit-msg."`
	PrepareMsg      string   `          long:"prepare-commit-message-source" description:"Source for prepare-commit-msg hook."`
	CommitObjName   string   `          long:"commit-object-name"            description:"Commit object name for prepare-commit-msg hook."`
	RemoteURL       string   `          long:"remote-url"                    description:"Remote URL for pre-push hook."`
	RemoteName      string   `          long:"remote-name"                   description:"Remote name for pre-push hook."`
	RemoteBranch    string   `          long:"remote-branch"                 description:"Remote branch for pre-push hook."`
	LocalBranch     string   `          long:"local-branch"                  description:"Local branch for pre-push hook."`
	CheckoutType    string   `          long:"checkout-type"                 description:"Checkout type for post-checkout hook."`
	IsSquash        string   `          long:"is-squash-merge"               description:"Whether the merge is a squash merge."`
	RewriteCmd      string   `          long:"rewrite-command"               description:"Rewrite command for post-rewrite hook."`
	PreRebaseUp     string   `          long:"pre-rebase-upstream"           description:"Upstream from which the series was forked."`
	PreRebaseBranch string   `          long:"pre-rebase-branch"             description:"Branch being rebased."`
	Verbose         bool     `short:"v" long:"verbose"                       description:"Produce hook output regardless of success."`
	FailFast        bool     `          long:"fail-fast"                     description:"Stop running hooks after the first failure."`
	NoInstall       bool     `          long:"no-install"                    description:"Skip automatic installation of hook environments."`
	Jobs            int      `short:"j" long:"jobs"                          description:"Number of jobs to run in parallel."`
}

func (c *RunCommand) Run(args []string) int {
	// Backstop in case `run` is reached directly from a git-invoked hook (not via
	// hook-impl): drop the host repo's GIT_DIR/GIT_INDEX_FILE/GIT_WORK_TREE so no
	// child git process can corrupt the host repo. See git.ScrubProcessEnv.
	git.ScrubProcessEnv()

	var opts runFlags

	remaining, code, ok := parseArgs("run", &opts, args, expandFilesFlag(args))
	if !ok {
		return code
	}
	var err error

	// Handle deprecated flags.
	useDeprecated(&opts.FromRef, opts.Source)
	useDeprecated(&opts.ToRef, opts.Origin)

	output.SetColorModeFromString(opts.Color)

	if err = adjustArgsAndChdir(rootArgs{
		Config:            &opts.Config,
		Files:             opts.Files,
		CommitMsgFilename: &opts.CommitMsgFn,
	}); err != nil {
		return fatalError(err)
	}

	// Load config.
	cfg, err := config.LoadConfig(opts.Config)
	if err != nil {
		return reportError(err)
	}

	// Propagate fail_fast from CLI or config.
	if opts.FailFast {
		cfg.FailFast = true
	}

	// Get repository root.
	root, err := git.GetRoot()
	if err != nil {
		return reportError(err)
	}

	// Set PRE_COMMIT=1.
	os.Setenv("PRE_COMMIT", "1")
	defer os.Unsetenv("PRE_COMMIT")

	// Initialize the store.
	s := store.New("")

	// Resolve hooks.
	resolver := repository.NewResolver(s, cfg)
	hooks, err := resolver.ResolveAll(c.Meta.ctx(), cfg)
	if err != nil {
		return reportError(err)
	}

	// Determine files.
	noStash := os.Getenv("PRE_COMMIT_NO_STASH") != ""
	filenames, err := hookFiles(opts.AllFiles, opts.Files, opts.FromRef, opts.ToRef)
	if err != nil {
		return reportError(err)
	}

	// Determine stage.
	stage := hookStage(opts.HookStage)

	// Filter to a single hook if specified.
	var hookID string
	if len(remaining) > 0 {
		hookID = remaining[0]
	}

	// Determine if we need to stash.
	if restore := stashUnstaged(opts.needsStash(noStash), root, s.Dir()); restore != nil {
		// Deferred, as upstream's staged_files_only context manager
		// is, so an error or an interrupt from here on still gives the
		// user their unstaged changes back.
		defer restore()
	}

	// Install environments (unless --no-install).
	if !opts.NoInstall {
		if err := hook.InstallEnvironments(c.Meta.ctx(), hooks, s.ExclusiveLock); err != nil {
			return reportError(err)
		}
	}

	// Run hooks.
	runner := hook.NewRunner(cfg, hooks, root)
	result := runner.Run(c.Meta.ctx(), opts.runOptions(hookID, stage, filenames))

	// Before the stash is restored, as upstream does, so the diff shown is
	// what the hooks changed and not the user's unstaged work.
	return runExit(result, opts.ShowDiffOnFail, opts.AllFiles)
}

// runExit is the exit status of a run that came to result, once the diff
// the hooks made is shown, if asked for, after a failure.
func runExit(result hook.RunResult, showDiffOnFail, allFiles bool) int {
	hasFailures := result.Failed > 0 || result.Errors > 0

	if showDiffOnFail && hasFailures {
		hook.ShowDiffOnFailure(allFiles)
	}

	if hasFailures {
		return 1
	}

	return 0
}

// useDeprecated takes a deprecated flag's value, old, as its replacement's
// when that is not given.
func useDeprecated(flag *string, old string) {
	if old != "" && *flag == "" {
		*flag = old
	}
}

// runOptions is the hook.RunOptions for running hookID, or every hook,
// at stage over files.
func (o *runFlags) runOptions(hookID string, stage config.Stage, files []string) hook.RunOptions {
	return hook.RunOptions{
		HookID:                     hookID,
		HookStage:                  stage,
		Files:                      files,
		AllFiles:                   o.AllFiles,
		Verbose:                    o.Verbose,
		ShowDiff:                   o.ShowDiffOnFail,
		Color:                      o.Color,
		Jobs:                       o.Jobs,
		FromRef:                    o.FromRef,
		ToRef:                      o.ToRef,
		CommitMsgFilename:          o.CommitMsgFn,
		PrepareCommitMessageSource: o.PrepareMsg,
		CommitObjectName:           o.CommitObjName,
		RemoteName:                 o.RemoteName,
		RemoteURL:                  o.RemoteURL,
		RemoteBranch:               o.RemoteBranch,
		LocalBranch:                o.LocalBranch,
		CheckoutType:               o.CheckoutType,
		IsSquashMerge:              o.IsSquash,
		RewriteCommand:             o.RewriteCmd,
		PreRebaseUpstream:          o.PreRebaseUp,
		PreRebaseBranch:            o.PreRebaseBranch,
	}
}

// needsStash reports whether the run checks the staged files, so stashes
// the unstaged changes first, unless noStash.
func (o *runFlags) needsStash(noStash bool) bool {
	return !o.AllFiles && len(o.Files) == 0 && o.FromRef == "" && o.ToRef == "" && !noStash
}

// hookFiles is the files a run checks: all of them, the ones given, those
// changed between two refs, or the staged ones.
func hookFiles(allFiles bool, files []string, fromRef, toRef string) ([]string, error) {
	switch {
	case allFiles:
		return git.GetAllFiles()
	case len(files) > 0:
		return files, nil
	case fromRef != "" && toRef != "":
		return git.GetChangedFiles(fromRef, toRef)
	}
	return git.GetStagedFiles()
}

// hookStage is the stage a run's hooks run at: the one given, or
// pre-commit.
func hookStage(given string) config.Stage {
	stage := config.TransformStage(config.Stage(given))
	if stage == "" {
		stage = config.HookTypePreCommit
	}
	return stage
}

// stashUnstaged stashes the unstaged changes, if needsStash and there are
// any, and returns the function that restores them, or nil when nothing
// was stashed.
func stashUnstaged(needsStash bool, root, storeDir string) func() {
	if !needsStash {
		return nil
	}
	// StashUnstaged decides whether there is anything to stash itself: it
	// must judge against the index git handed the hook, which a plain
	// `git diff` here would not (see git.HostIndexEnv).
	stashMgr := staged.NewManager(root, storeDir)
	stashed, err := stashMgr.StashUnstaged()
	if err != nil {
		output.Warn("Failed to stash unstaged changes: %v", err)
	}
	if !stashed {
		return nil
	}
	return func() {
		if err := stashMgr.Restore(); err != nil {
			output.Warn("Failed to restore unstaged changes: %v", err)
		}
	}
}

func (c *RunCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit run [options] [hook-id]

  Run hooks. If hook-id is given, only that hook is run, otherwise all hooks
  are run. If no files are specified, all staged files are used.

Options:

  -a, --all-files              Run on all files in the repo.
      --files [FILES ...]      Specific filenames to run hooks on.
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
