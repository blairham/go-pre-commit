// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/blairham/go-pre-commit/v4/internal/config"
	"github.com/blairham/go-pre-commit/v4/internal/store"
)

// gc keeps every repo a tracked config still uses, not only the current
// config's, and forgets the tracked configs that are gone -- upstream's _gc.
func TestGCKeepsReposOfOtherTrackedConfigs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PRE_COMMIT_HOME", home)

	writeConfig := func(dir, repo string) string {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, ".pre-commit-config.yaml")
		body := "repos:\n- repo: " + repo + "\n  rev: v1\n  hooks:\n  - id: h\n"
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	base := t.TempDir()
	cfgA := writeConfig(filepath.Join(base, "a"), "https://example.com/used-by-a")
	cfgB := writeConfig(filepath.Join(base, "b"), "https://example.com/used-by-b")
	cfgGone := writeConfig(filepath.Join(base, "gone"), "https://example.com/used-by-gone")

	type entry = store.RepoEntry
	var repos []entry
	for _, name := range []string{"used-by-a", "used-by-b", "used-by-gone", "unused"} {
		dir := filepath.Join(home, "repo"+name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		repos = append(repos, entry{Repo: "https://example.com/" + name, Rev: "v1", Path: dir})
	}
	data, err := json.Marshal(map[string]any{"repos": repos})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(home, "db.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	s := store.New(home)
	for _, c := range []string{cfgA, cfgGone} {
		if err = s.MarkConfigUsed(c); err != nil {
			t.Fatal(err)
		}
	}
	if err = os.Remove(cfgGone); err != nil {
		t.Fatal(err)
	}

	// Run from b, whose config gc reads as the current one.
	t.Chdir(filepath.Dir(cfgB))
	if code := (&GCCommand{Meta: &Meta{}}).Run(nil); code != 0 {
		t.Fatalf("gc exited %d", code)
	}

	left, err := s.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range left {
		names = append(names, r.Repo)
	}
	want := []string{"https://example.com/used-by-a", "https://example.com/used-by-b"}
	if !slices.Equal(names, want) {
		t.Errorf("repos kept = %q, want %q", names, want)
	}

	tracked, err := s.GetTrackedConfigs()
	if err != nil {
		t.Fatal(err)
	}
	realA, err := filepath.EvalSymlinks(cfgA)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tracked, []string{realA}) {
		t.Errorf("tracked configs = %q, want only %q", tracked, realA)
	}
}

// gc keeps the per-dependency clones a config's hooks run from, as upstream's
// _mark_used_repos does: the hook's own additional_dependencies, or else the
// manifest's.
func TestMarkDepsClonesUsed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	origin := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = origin
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	manifest := "- {id: a, name: a, entry: a, language: golang, additional_dependencies: [m@v1]}\n" +
		"- {id: b, name: b, entry: b, language: golang}\n"
	if err := os.WriteFile(filepath.Join(origin, config.ManifestFile), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-q", "-m", "init")
	sha := git("rev-parse", "HEAD")

	s := store.New(t.TempDir())
	if _, err := s.Clone(origin, sha); err != nil {
		t.Fatal(err)
	}

	used := map[string]bool{}
	markDepsClonesUsed(s, config.RepoConfig{Repo: origin, Rev: sha, Hooks: []config.HookConfig{
		{ID: "a"},
		{ID: "b", AdditionalDependencies: []string{"x", "y"}},
		{ID: "missing", AdditionalDependencies: []string{"z"}},
	}}, used)

	for _, key := range []string{origin + ":m@v1@" + sha, origin + ":x,y@" + sha} {
		if !used[key] {
			t.Errorf("%s should be marked used; got %v", key, used)
		}
	}
	if used[origin+":z@"+sha] {
		t.Error("a hook missing from the manifest marks nothing, as upstream")
	}
}
