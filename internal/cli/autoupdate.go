// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
	"github.com/blairham/go-pre-commit/v4/internal/git"
	"github.com/blairham/go-pre-commit/v4/internal/output"
	"github.com/blairham/go-pre-commit/v4/internal/pyyaml"
)

// AutoupdateCommand implements the "autoupdate" command.
type AutoupdateCommand struct {
	Meta *Meta
}

type autoupdateFlags struct {
	GlobalFlags
	BleedingEdge bool     `long:"bleeding-edge" description:"Update to the bleeding edge of the default branch instead of the latest tagged version."`
	Freeze       bool     `long:"freeze"        description:"Store the current commit SHA alongside the tag as rev."`
	Repo         []string `long:"repo"          description:"Only update this repository. May be specified multiple times."`
	Jobs         int      `long:"jobs"          description:"Number of threads to use."                                                               short:"j" default:"1"`
	DryRun       bool     `long:"dry-run"       description:"Show what would be updated without writing changes."`
}

func (c *AutoupdateCommand) Run(args []string) int {
	var opts autoupdateFlags
	if _, code, ok := parseArgs("autoupdate", &opts, args, args); !ok {
		return code
	}
	var err error

	output.SetColorModeFromString(opts.Color)

	if err = adjustArgsAndChdir(rootArgs{Config: &opts.Config}); err != nil {
		return fatalError(err)
	}

	// Upstream's autoupdate migrates the config before it reads it. Its
	// "has been migrated" is a print(), so into a pipe it comes out last.
	stdout := newPyStdout()
	defer stdout.Flush()
	if code := migrateConfigExit(opts.Config, true, stdout.Print); code != 0 {
		return code
	}

	cfg, err := config.LoadConfig(opts.Config)
	if err != nil {
		return reportError(err)
	}

	// Upstream (4.6.1) refuses a --repo the config does not name before
	// touching anything, so a typo is an error rather than a silent no-op.
	if missing := missingRepos(cfg, opts.Repo); len(missing) > 0 {
		fmt.Printf("repos (%s) were not found in %s\n", strings.Join(missing, ", "), opts.Config)
		return 1
	}

	// Upstream's config_repos: every repo autoupdate can move, in order;
	// revInfos is indexed alongside it.
	var configRepos []config.RepoConfig
	for _, r := range cfg.Repos {
		if !r.IsLocal() && !r.IsMeta() {
			configRepos = append(configRepos, r)
		}
	}
	var todo []int
	for i, r := range configRepos {
		if len(opts.Repo) == 0 || slices.Contains(opts.Repo, r.Repo) {
			todo = append(todo, i)
		}
	}

	// jobs: 0 is the number of cpus, at most one per repo, at least one.
	jobs := opts.Jobs
	if jobs == 0 {
		jobs = runtime.NumCPU()
	}
	if n := len(opts.Repo); n > 0 {
		jobs = min(jobs, n)
	} else {
		jobs = min(jobs, len(configRepos))
	}
	jobs = max(jobs, 1)

	tagsOnly := !opts.BleedingEdge
	results := make(chan updateResult)
	go func() {
		var wg sync.WaitGroup
		sem := make(chan struct{}, jobs)
		for _, i := range todo {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				results <- updateOne(i, configRepos[i], tagsOnly, opts.Freeze)
			}()
		}
		wg.Wait()
		close(results)
	}()

	// As upstream's as_completed loop: each repo is reported as it finishes.
	// An error other than "cannot be updated" ends the run unwritten, after
	// the updates already started have finished.
	revInfos := make([]*revInfo, len(configRepos))
	changed := false
	retv := 0
	var unexpected error
	for res := range results {
		if unexpected != nil {
			continue
		}
		var cannot cannotUpdateError
		switch {
		case errors.As(res.err, &cannot):
			fmt.Println(cannot)
			retv = 1
			continue
		case res.err != nil:
			unexpected = res.err
			continue
		}
		old, upd := configRepos[res.idx], res.info
		var msg string
		if upd.rev != old.Rev {
			changed = true
			newS := upd.rev
			if upd.frozen != "" {
				newS = upd.frozen + " (frozen)"
			}
			msg = fmt.Sprintf("updating %s -> %s", old.Rev, newS)
			revInfos[res.idx] = &upd
		} else {
			msg = "already up to date!"
		}
		fmt.Printf("[%s] %s\n", old.Repo, msg)
	}
	if unexpected != nil {
		return unexpectedError(unexpected)
	}

	if changed && !opts.DryRun {
		if err := writeNewConfig(opts.Config, revInfos); err != nil {
			return unexpectedError(err)
		}
	}

	return retv
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
      --dry-run         Show what would be updated without writing changes.
  -c, --config=FILE     Path to alternate config file.
      --color=MODE      Whether to use color (auto, always, never).
`)
}

func (c *AutoupdateCommand) Synopsis() string {
	return "Auto-update pre-commit config to the latest repos' versions"
}

// missingRepos returns the requested repos that are not among cfg's
// updatable (non-local, non-meta) repos, sorted and de-duplicated as
// upstream's sorted(set(repos) - config_repos).
func missingRepos(cfg *config.Config, requested []string) []string {
	known := make(map[string]bool, len(cfg.Repos))
	for _, r := range cfg.Repos {
		if r.IsLocal() || r.IsMeta() {
			continue
		}
		known[r.Repo] = true
	}
	var missing []string
	for _, r := range requested {
		if !known[r] {
			missing = append(missing, r)
		}
	}
	slices.Sort(missing)
	return slices.Compact(missing)
}

// revInfo is upstream's RevInfo after update: the rev to write, and the tag
// it was frozen from when --freeze replaced it with a sha.
type revInfo struct {
	hookIDs map[string]bool
	rev     string
	frozen  string
}

// cannotUpdateError is upstream's RepositoryCannotBeUpdatedError: printed as
// is, and the run carries on with exit status 1.
type cannotUpdateError string

func (e cannotUpdateError) Error() string { return string(e) }

type updateResult struct {
	idx  int
	info revInfo
	err  error
}

// updateOne is upstream's _update_one: the new rev, refused if it drops any
// hook the config uses.
func updateOne(idx int, repoCfg config.RepoConfig, tagsOnly, freeze bool) updateResult {
	info, err := updateRevInfo(repoCfg.Repo, tagsOnly, freeze)
	if err != nil {
		return updateResult{idx: idx, err: err}
	}
	var missing []string
	for _, h := range repoCfg.Hooks {
		if !info.hookIDs[h.ID] {
			missing = append(missing, h.ID)
		}
	}
	slices.Sort(missing)
	missing = slices.Compact(missing)
	if len(missing) > 0 {
		return updateResult{idx: idx, err: cannotUpdateError(fmt.Sprintf(
			"[%s] Cannot update because the update target is missing these hooks: %s",
			repoCfg.Repo, strings.Join(missing, ", "),
		))}
	}
	return updateResult{idx: idx, info: info}
}

// updateRevInfo is upstream's RevInfo.update: a blobless fetch of the
// remote's HEAD and tags into an empty repository, the newest tag reachable
// from it (or, bleeding edge, a tag exactly on it), else its sha. git's output
// is captured throughout, as upstream's cmd_output does.
func updateRevInfo(repo string, tagsOnly, freeze bool) (info revInfo, err error) {
	tmp, err := os.MkdirTemp("", "tmp")
	if err != nil {
		return revInfo{}, err
	}
	// Upstream's TemporaryDirectory: a cleanup that fails is an error too.
	defer func() {
		if rmErr := fsutil.RemoveAll(tmp); rmErr != nil && err == nil {
			err = rmErr
		}
	}()
	return fetchRevInfo(tmp, repo, tagsOnly, freeze)
}

func fetchRevInfo(tmp, repo string, tagsOnly, freeze bool) (revInfo, error) {
	g := func(args ...string) []string {
		return append(append(append([]string{}, git.NoFSMonitor...), "-C", tmp), args...)
	}
	tagOpt := "--exact"
	if tagsOnly {
		tagOpt = "--abbrev=0"
	}

	err := git.InitRepo(tmp, repo)
	if err == nil {
		err = git.RunCaptured("", g("config", "extensions.partialClone", "true")...)
	}
	if err == nil {
		err = git.RunCaptured("", g("fetch", "origin", "HEAD", "--quiet", "--filter=blob:none", "--tags")...)
	}
	if err != nil {
		return revInfo{}, err
	}

	var rev string
	out, err := git.OutputCaptured("", g("describe", "FETCH_HEAD", "--tags", tagOpt)...)
	var cpe *git.CalledProcessError
	switch {
	case errors.As(err, &cpe):
		if out, err = git.OutputCaptured("", g("rev-parse", "FETCH_HEAD")...); err != nil {
			return revInfo{}, err
		}
		rev = strings.TrimSpace(out)
	case err != nil:
		return revInfo{}, err
	default:
		rev = strings.TrimSpace(out)
		if tagsOnly {
			if rev, err = git.BestCandidateTag(rev, tmp); err != nil {
				return revInfo{}, err
			}
		}
	}

	info := revInfo{rev: rev}
	if freeze {
		if out, err = git.OutputCaptured("", g("rev-parse", rev)...); err != nil {
			return revInfo{}, err
		}
		if exact := strings.TrimSpace(out); exact != rev {
			info.rev, info.frozen = exact, rev
		}
	}

	// Upstream checks the manifest out only if it exists at the rev (its
	// windows workaround), swallowing a failed git command; a missing
	// manifest is then reported by the load below.
	err = git.RunCaptured("", g("show", info.rev+":"+config.ManifestFile)...)
	if err == nil {
		err = git.RunCaptured("", g("checkout", info.rev, "--", config.ManifestFile)...)
	}
	if err != nil && !errors.As(err, &cpe) {
		return revInfo{}, err
	}
	manifest, err := config.LoadManifest(filepath.Join(tmp, config.ManifestFile))
	if err != nil {
		return revInfo{}, cannotUpdateError(fmt.Sprintf("[%s] %v", repo, err))
	}
	info.hookIDs = make(map[string]bool, len(manifest))
	for _, h := range manifest {
		info.hookIDs[h.ID] = true
	}
	return info, nil
}

// revLineRe is upstream's REV_LINE_RE.
var revLineRe = regexp.MustCompile(`^(\s+)rev:(\s*)(['"]?)([^\s#]+)(.*)(\r?\n)$`)

// writeNewConfig is upstream's _write_new_config: each rev line, matched to
// its repo by position, rewritten in its own quoting style. A "# frozen:"
// comment is replaced or dropped; any other comment is kept.
func writeNewConfig(path string, infos []*revInfo) error {
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, ok := rewriteRevLines(string(original), infos)
	if !ok {
		// Upstream reformats a config whose rev lines it cannot find (flow
		// style, say) and tries once more.
		reformatted, err := reformatYAML(original)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, reformatted, 0o644); err != nil {
			return err
		}
		if updated, ok = rewriteRevLines(string(reformatted), infos); !ok {
			return errors.New("could not find rev lines")
		}
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

// rewriteRevLines applies infos to the rev lines of original, or reports
// false when the number of rev lines is not the number of repos.
func rewriteRevLines(original string, infos []*revInfo) (string, bool) {
	lines := strings.SplitAfter(original, "\n")
	var idxs []int
	for i, line := range lines {
		if revLineRe.MatchString(line) {
			idxs = append(idxs, i)
		}
	}
	if len(idxs) != len(infos) {
		return "", false
	}
	for n, idx := range idxs {
		info := infos[n]
		if info == nil {
			continue
		}
		m := revLineRe.FindStringSubmatch(lines[idx])
		var comment string
		switch {
		case info.frozen != "":
			comment = "  # frozen: " + info.frozen
		case strings.HasPrefix(strings.TrimSpace(m[5]), "# frozen:"):
			comment = ""
		default:
			comment = m[5]
		}
		lines[idx] = m[1] + "rev:" + m[2] + yamlScalar(info.rev, m[3]) + comment + m[6]
	}
	return strings.Join(lines, ""), true
}

// yamlScalar is upstream's yaml_dump({'rev': s}, default_style=style)
// cut after the key: s in the quote style the old rev used, where PyYAML can.
func yamlScalar(s, style string) string {
	var ds byte
	if style != "" {
		ds = style[0]
	}
	v := &pyyaml.Value{Kind: pyyaml.Map, Pairs: []pyyaml.Pair{{
		Key:   &pyyaml.Value{Kind: pyyaml.Str, Str: "rev"},
		Value: &pyyaml.Value{Kind: pyyaml.Str, Str: s},
	}}}
	_, after, _ := strings.Cut(pyyaml.DumpStyle(v, ds), ":")
	return strings.TrimSpace(after)
}

// reformatYAML is upstream's yaml_dump(yaml_load(original)): the config
// re-dumped by PyYAML in block style with four-space indent.
func reformatYAML(original []byte) ([]byte, error) {
	v, err := pyyaml.Load(universalNewlines(string(original)))
	if err != nil {
		return nil, err
	}
	return []byte(pyyaml.Dump(v)), nil
}
