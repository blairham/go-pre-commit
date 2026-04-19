package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	flags "github.com/jessevdk/go-flags"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/output"
)

// AutoupdateCommand implements the "autoupdate" command.
type AutoupdateCommand struct {
	Meta *Meta
}

type autoupdateFlags struct {
	GlobalFlags
	BleedingEdge bool     `long:"bleeding-edge" description:"Update to the bleeding edge of the default branch instead of the latest tagged version."`
	Freeze       bool     `long:"freeze" description:"Store the current commit SHA alongside the tag as rev."`
	Repo         []string `long:"repo" description:"Only update this repository. May be specified multiple times."`
	Jobs         int      `short:"j" long:"jobs" default:"1" description:"Number of threads to use."`
}

func (c *AutoupdateCommand) Run(args []string) int {
	var opts autoupdateFlags
	_, err := flags.ParseArgs(&opts, args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	output.SetColorModeFromString(opts.Color)

	cfg, err := config.LoadConfig(opts.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load config: %v\n", err)
		return 1
	}

	// Read the raw config file for rewriting.
	rawBytes, err := os.ReadFile(opts.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read config: %v\n", err)
		return 1
	}

	raw := string(rawBytes)
	changed := false

	// Determine concurrency.
	concurrency := opts.Jobs
	if concurrency < 1 {
		concurrency = 1
	}

	// Build list of update-able repos.
	type updateTask struct {
		repoCfg config.RepoConfig
	}
	var tasks []updateTask
	for _, repoCfg := range cfg.Repos {
		if repoCfg.IsLocal() || repoCfg.IsMeta() {
			continue
		}
		if len(opts.Repo) > 0 {
			found := false
			for _, r := range opts.Repo {
				if r == repoCfg.Repo {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		tasks = append(tasks, updateTask{repoCfg: repoCfg})
	}

	// Process updates (parallel if jobs > 1).
	results := make([]updateResult, len(tasks))

	if concurrency > 1 && len(tasks) > 1 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, concurrency)

		for i, task := range tasks {
			wg.Add(1)
			go func(idx int, t updateTask) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				res := processUpdate(t.repoCfg, opts.BleedingEdge, opts.Freeze)

				results[idx] = res
			}(i, task)
		}
		wg.Wait()
	} else {
		for i, task := range tasks {
			results[i] = processUpdate(task.repoCfg, opts.BleedingEdge, opts.Freeze)
		}
	}

	// Apply results.
	for _, res := range results {
		if res.err != nil {
			fmt.Printf("Updating %s ... failed\n", res.repo)
			output.Warn("Failed to update %s: %v", res.repo, res.err)
			continue
		}

		if res.newRev == res.oldRev {
			fmt.Printf("Updating %s ... already up to date.\n", res.repo)
			continue
		}

		fmt.Printf("Updating %s ... updating %s -> %s.\n", res.repo, res.oldRev, res.newRev)

		// Use regex to replace rev, handling various quoting styles.
		raw = replaceRev(raw, res.oldRev, res.newRev, res.commitHash, opts.Freeze)
		changed = true
	}

	if changed {
		if err := os.WriteFile(opts.Config, []byte(raw), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to write config: %v\n", err)
			return 1
		}
	}

	return 0
}

func (c *AutoupdateCommand) Help() string {
	return strings.TrimSpace(`
Usage: pre-commit autoupdate [options]

  Auto-update pre-commit config to the latest repos' versions.

Options:

      --bleeding-edge   Update to the bleeding edge of the default branch.
      --freeze          Store the current commit SHA alongside the tag as rev.
      --repo=REPO       Only update this repository (may be repeated).
  -j, --jobs=N          Number of threads to use (default: 1).
  -c, --config=FILE     Path to alternate config file.
      --color=MODE      Whether to use color (auto, always, never).
`)
}

func (c *AutoupdateCommand) Synopsis() string {
	return "Auto-update pre-commit config to the latest repos' versions"
}

type updateResult struct {
	repo       string
	oldRev     string
	newRev     string
	commitHash string
	err        error
}

func processUpdate(repoCfg config.RepoConfig, bleeding, freeze bool) updateResult {
	res := updateResult{
		repo:   repoCfg.Repo,
		oldRev: repoCfg.Rev,
	}

	tmpDir, err := os.MkdirTemp("", "pre-commit-autoupdate-*")
	if err != nil {
		res.err = fmt.Errorf("failed to create temp dir: %w", err)
		return res
	}
	defer os.RemoveAll(tmpDir)

	// Use blobless clone for faster autoupdate (we only need tags/refs, not file content).
	if err := git.Clone(repoCfg.Repo, tmpDir, "--filter=blob:none"); err != nil {
		// Fall back to regular clone if filter is not supported.
		os.RemoveAll(tmpDir)
		tmpDir2, _ := os.MkdirTemp("", "pre-commit-autoupdate-*")
		tmpDir = tmpDir2
		if err := git.Clone(repoCfg.Repo, tmpDir); err != nil {
			res.err = fmt.Errorf("failed to clone: %w", err)
			return res
		}
	}

	if bleeding {
		res.newRev, err = getHEAD(tmpDir)
	} else {
		res.newRev, err = getLatestTag(tmpDir)
	}
	if err != nil {
		res.err = fmt.Errorf("failed to get latest version: %w", err)
		return res
	}

	// For freeze mode, also resolve the commit hash.
	if freeze && res.newRev != "" {
		commitHash, err := resolveToCommit(tmpDir, res.newRev)
		if err == nil && commitHash != res.newRev {
			res.commitHash = commitHash
		}
	}

	return res
}

// replaceRev replaces the rev value in the raw YAML, handling quoting.
// For freeze mode: uses the commit SHA as rev and adds the tag as a "# frozen: TAG" comment.
// When not in freeze mode, strips any existing "# frozen:" comments.
func replaceRev(raw, oldRev, newRev, commitHash string, freeze bool) string {
	// Match rev: with optional quoting of the value.
	// Handles: rev: v1.0, rev: 'v1.0', rev: "v1.0"
	// Also matches optional trailing comments (including # frozen: ... comments).
	pattern := fmt.Sprintf(
		`(rev:\s*['"]?)%s(['"]?)(\s*(?:#.*)?)$`,
		regexp.QuoteMeta(oldRev),
	)
	re := regexp.MustCompile("(?m)" + pattern)

	if freeze && commitHash != "" {
		// Python behavior: rev becomes the commit SHA, tag goes in "# frozen: TAG" comment.
		return re.ReplaceAllString(raw,
			fmt.Sprintf("${1}%s${2}  # frozen: %s", commitHash, newRev),
		)
	}

	// When not freezing, strip any existing "# frozen:" comments.
	return re.ReplaceAllString(raw,
		fmt.Sprintf("${1}%s${2}", newRev),
	)
}

func getLatestTag(repoDir string) (string, error) {
	// First try git describe, which finds the most recent tag reachable from HEAD.
	tag, err := git.GetLatestTag(repoDir)
	if err == nil && tag != "" {
		return tag, nil
	}
	// Fall back to listing all tags sorted by version and picking the last.
	tags, err := git.ListTags(repoDir)
	if err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}
	return tags[len(tags)-1], nil
}

func getHEAD(repoDir string) (string, error) {
	return git.GetHeadSHA(repoDir)
}

func resolveToCommit(repoDir string, ref string) (string, error) {
	return git.CmdOutputInDir(repoDir, "rev-parse", ref+"^{}")
}
