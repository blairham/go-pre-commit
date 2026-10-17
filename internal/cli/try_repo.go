// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
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
	Ref             string   `long:"ref"                           description:"Manually select a ref to run against. Otherwise uses HEAD."`
	Rev             string   `long:"rev"                           description:"(DEPRECATED: use --ref) Manually select a rev to run against."`
	AllFiles        bool     `long:"all-files"                     description:"Run on all files in the repo."                                 short:"a"`
	Files           []string `long:"files"                         description:"Specific filenames to run hooks on."`
	Verbose         bool     `long:"verbose"                       description:"Produce hook output regardless of success."                    short:"v"`
	HookStage       string   `long:"hook-stage"                    description:"The stage during which the hook runs."`
	ShowDiffOnFail  bool     `long:"show-diff-on-failure"          description:"When hooks fail, show the diff of changes."`
	FailFast        bool     `long:"fail-fast"                     description:"Stop running hooks after the first failure."`
	FromRef         string   `long:"from-ref"                      description:"Ref to check revision changes."`
	ToRef           string   `long:"to-ref"                        description:"Ref to check revision changes."`
	Source          string   `long:"source"                        description:"(DEPRECATED: use --from-ref) Ref to check revision changes."   short:"s"`
	Origin          string   `long:"origin"                        description:"(DEPRECATED: use --to-ref) Ref to check revision changes."     short:"o"`
	CommitMsgFn     string   `long:"commit-msg-filename"           description:"Filename to check when running during commit-msg."`
	PrepareMsg      string   `long:"prepare-commit-message-source" description:"Source for prepare-commit-msg hook."`
	CommitObjName   string   `long:"commit-object-name"            description:"Commit object name for prepare-commit-msg hook."`
	RemoteURL       string   `long:"remote-url"                    description:"Remote URL for pre-push hook."`
	RemoteName      string   `long:"remote-name"                   description:"Remote name for pre-push hook."`
	RemoteBranch    string   `long:"remote-branch"                 description:"Remote branch for pre-push hook."`
	LocalBranch     string   `long:"local-branch"                  description:"Local branch for pre-push hook."`
	CheckoutType    string   `long:"checkout-type"                 description:"Checkout type for post-checkout hook."`
	IsSquash        string   `long:"is-squash-merge"               description:"Whether the merge is a squash merge."`
	RewriteCmd      string   `long:"rewrite-command"               description:"Rewrite command for post-rewrite hook."`
	PreRebaseUp     string   `long:"pre-rebase-upstream"           description:"Upstream from which the series was forked."`
	PreRebaseBranch string   `long:"pre-rebase-branch"             description:"Branch being rebased."`
	Jobs            int      `long:"jobs"                          description:"Number of jobs to run in parallel."                            short:"j"`
}

// Run is upstream's try_repo: it resolves the repository and ref, prints the
// config it generated, and runs that config's hooks.
func (c *TryRepoCommand) Run(args []string) (code int) {
	var opts tryRepoFlags
	remaining, code, ok := parseArgs("try-repo", &opts, args, expandFilesFlag(args))
	if !ok {
		return code
	}
	var err error

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

	if err = adjustArgsAndChdir(rootArgs{
		Config:            &opts.Config,
		Files:             opts.Files,
		CommitMsgFilename: &opts.CommitMsgFn,
		Repo:              &remaining[0],
	}); err != nil {
		return fatalError(err)
	}

	repoURL := remaining[0]
	var hookID string
	if len(remaining) > 1 {
		hookID = remaining[1]
	}

	// As upstream: everything, the shadow repository included, lives in one
	// throwaway directory that is also the store, rather than ~/.cache.
	tempdir, err := os.MkdirTemp("", "tmp")
	if err != nil {
		return unexpectedError(err)
	}
	// Upstream's TemporaryDirectory: a cleanup that fails is an error too.
	defer func() {
		if rmErr := fsutil.RemoveAll(tempdir); rmErr != nil && code == 0 {
			code = unexpectedError(rmErr)
		}
	}()

	repo, ref, err := tryRepoRef(tempdir, repoURL, opts.Ref)
	if err != nil {
		return unexpectedError(err)
	}

	s := store.New(tempdir)
	var hookCfgs []config.HookConfig
	if hookID != "" {
		hookCfgs = []config.HookConfig{{ID: hookID}}
	} else {
		repoPath, cloneErr := s.Clone(repo, ref)
		if cloneErr != nil {
			return unexpectedError(cloneErr)
		}
		manifest, loadErr := config.LoadManifest(filepath.Join(repoPath, config.ManifestFile))
		if loadErr != nil {
			return reportError(loadErr)
		}
		for _, m := range manifest {
			hookCfgs = append(hookCfgs, config.HookConfig{ID: m.ID})
		}
		sort.Slice(hookCfgs, func(i, j int) bool { return hookCfgs[i].ID < hookCfgs[j].ID })
	}
	tryConfig := &config.Config{
		Repos: []config.RepoConfig{{Repo: repo, Rev: ref, Hooks: hookCfgs}},
	}
	printTryConfig(repo, ref, hookCfgs)

	resolver := repository.NewResolver(s, tryConfig)
	hooks, err := resolver.ResolveAll(c.Meta.ctx(), tryConfig)
	if err != nil {
		return reportError(err)
	}

	// Determine files.
	var filenames []string
	if opts.AllFiles {
		filenames, err = git.GetAllFiles()
		if err != nil {
			return reportError(err)
		}
	} else if len(opts.Files) > 0 {
		filenames = opts.Files
	} else if opts.FromRef != "" && opts.ToRef != "" {
		filenames, err = git.GetChangedFiles(opts.FromRef, opts.ToRef)
		if err != nil {
			return reportError(err)
		}
	} else {
		filenames, err = git.GetStagedFiles()
		if err != nil {
			return reportError(err)
		}
	}

	stage := config.TransformStage(config.Stage(opts.HookStage))
	if stage == "" {
		stage = config.HookTypePreCommit
	}

	// Build a minimal config for the runner.
	runCfg := config.DefaultConfig()
	if opts.FailFast {
		runCfg.FailFast = true
	}

	// Upstream's try_repo hands off to run, which installs each hook's
	// environment first; without this every python, node or golang hook
	// errors with its executable not found.
	if err := hook.InstallEnvironments(c.Meta.ctx(), hooks, s.ExclusiveLock); err != nil {
		return reportError(err)
	}

	root, _ := git.GetRoot()
	runner := hook.NewRunner(runCfg, hooks, root)
	result := runner.Run(c.Meta.ctx(), hook.RunOptions{
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

	hasFailures := result.Failed > 0 || result.Errors > 0

	if opts.ShowDiffOnFail && hasFailures {
		hook.ShowDiffOnFailure(opts.AllFiles)
	}

	if hasFailures {
		return 1
	}

	return 0
}

// printTryConfig writes the generated config the way upstream does: a rule of
// 79 '=' around "Using config:" and its yaml_dump (four-space indent).
func printTryConfig(repo, rev string, hooks []config.HookConfig) {
	rule := strings.Repeat("=", 79)
	var b strings.Builder
	fmt.Fprintf(&b, "%s\nUsing config:\n%s\n", rule, rule)
	fmt.Fprintf(&b, "repos:\n-   repo: %s\n    rev: %s\n    hooks:\n", repo, rev)
	for _, h := range hooks {
		fmt.Fprintf(&b, "    -   id: %s\n", h.ID)
	}
	fmt.Fprintf(&b, "%s\n", rule)
	fmt.Print(b.String())
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
      --files [FILES ...]        Specific filenames to run hooks on.
  -v, --verbose                  Produce hook output regardless of success.
      --hook-stage=STAGE         The stage during which the hook runs.
      --show-diff-on-failure     When hooks fail, show the diff of changes.
      --fail-fast                Stop running hooks after the first failure.
      --from-ref=REF             Ref to check revision changes.
      --to-ref=REF               Ref to check revision changes.
  -j, --jobs=N                   Number of jobs to run in parallel.
  -c, --config=FILE              Path to alternate config file.
      --color=MODE               Whether to use color (auto, always, never).
`)
}

func (c *TryRepoCommand) Synopsis() string {
	return "Try the hooks in a repository"
}

// tryRepoRef is upstream's try_repo._repo_ref. An explicit ref is used as
// given. Otherwise the ref is the repository's HEAD, and a local repository
// with uncommitted changes to tracked files is copied into a shadow repository
// under tmpdir with those changes committed, so the hooks run as edited.
// Untracked files are not included, as upstream does not include them.
func tryRepoRef(tmpdir, repo, ref string) (string, string, error) {
	if ref != "" {
		return repo, ref, nil
	}
	ref, err := git.HeadRev(repo)
	if err != nil {
		return "", "", err
	}
	if _, statErr := os.Stat(repo); statErr != nil || !git.HasDiff(repo, "HEAD") {
		return repo, ref, nil
	}

	output.Warn("Creating temporary repo with uncommitted changes...")

	shadow := filepath.Join(tmpdir, "shadow-repo")
	err = git.RunCaptured("", "clone", repo, shadow)
	if err == nil {
		err = git.RunCaptured(shadow, "checkout", ref, "-b", "_pc_tmp")
	}
	if err != nil {
		return "", "", err
	}

	idx, err := git.Path(shadow, "index")
	if err != nil {
		return "", "", err
	}
	objs, err := git.Path(shadow, "objects")
	if err != nil {
		return "", "", err
	}
	env := append(git.NoGitEnv(), "GIT_INDEX_FILE="+idx, "GIT_OBJECT_DIRECTORY="+objs)

	staged, err := git.StagedFilesIn(repo)
	if err != nil {
		return "", "", err
	}
	// Upstream's xargs, whose exit status it does not check.
	var cpe *git.CalledProcessError
	for _, batch := range argBatches(staged) {
		err = git.RunCapturedEnv(repo, env, append([]string{"add", "--"}, batch...)...)
		if err != nil && !errors.As(err, &cpe) {
			return "", "", err
		}
	}
	err = git.RunCapturedEnv(repo, env, "add", "-u")
	if err == nil {
		err = git.Commit(shadow)
	}
	if err != nil {
		return "", "", err
	}

	rev, err := git.HeadRev(shadow)
	if err != nil {
		return "", "", err
	}
	return shadow, rev, nil
}

// argBatches splits files into command lines that stay well under the
// platform's argument limit, as upstream's xargs partitions them.
func argBatches(files []string) [][]string {
	const maxLen = 32 * 1024
	var batches [][]string
	var cur []string
	size := 0
	for _, f := range files {
		if len(cur) > 0 && size+len(f)+1 > maxLen {
			batches = append(batches, cur)
			cur, size = nil, 0
		}
		cur = append(cur, f)
		size += len(f) + 1
	}
	if len(cur) > 0 {
		batches = append(batches, cur)
	}
	return batches
}
