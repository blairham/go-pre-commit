package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRepoName(t *testing.T) {
	cases := []struct {
		name string
		deps []string
		want string
	}{
		{"no deps", nil, "local"},
		{"one dep", []string{"pyyaml>=6"}, "local:pyyaml>=6"},
		{"several deps keep order", []string{"b", "a"}, "local:b,a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LocalRepoName(tc.deps); got != tc.want {
				t.Errorf("LocalRepoName(%v) = %q, want %q", tc.deps, got, tc.want)
			}
		})
	}
}

func TestLocalRepoKey(t *testing.T) {
	want := "local:pyyaml>=6@" + localRepoVersion
	if got := LocalRepoKey([]string{"pyyaml>=6"}); got != want {
		t.Errorf("LocalRepoKey = %q, want %q", got, want)
	}
}

func TestMakeLocalWritesPlaceholders(t *testing.T) {
	s := New(t.TempDir())

	dir, err := s.MakeLocal([]string{"pyyaml>=6"})
	if err != nil {
		t.Fatalf("MakeLocal: %v", err)
	}

	for name := range localResources {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("expected placeholder %s in %s: %v", name, dir, err)
		}
	}

	repos, err := s.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 recorded repo, got %d", len(repos))
	}
	if repos[0].Repo != "local:pyyaml>=6" || repos[0].Rev != localRepoVersion {
		t.Errorf("recorded %s@%s, want local:pyyaml>=6@%s", repos[0].Repo, repos[0].Rev, localRepoVersion)
	}
	if repos[0].Path != dir {
		t.Errorf("recorded path %s, want %s", repos[0].Path, dir)
	}
}

func TestMakeLocalIsIdempotent(t *testing.T) {
	s := New(t.TempDir())

	first, err := s.MakeLocal([]string{"pyyaml>=6"})
	if err != nil {
		t.Fatalf("MakeLocal: %v", err)
	}
	second, err := s.MakeLocal([]string{"pyyaml>=6"})
	if err != nil {
		t.Fatalf("MakeLocal (again): %v", err)
	}
	if first != second {
		t.Errorf("expected the same directory, got %s and %s", first, second)
	}
}

func TestMakeLocalSeparatesDependencySets(t *testing.T) {
	s := New(t.TempDir())

	withDeps, err := s.MakeLocal([]string{"pyyaml>=6"})
	if err != nil {
		t.Fatalf("MakeLocal: %v", err)
	}
	otherDeps, err := s.MakeLocal([]string{"pyyaml>=6", "six"})
	if err != nil {
		t.Fatalf("MakeLocal: %v", err)
	}
	noDeps, err := s.MakeLocal(nil)
	if err != nil {
		t.Fatalf("MakeLocal: %v", err)
	}

	if withDeps == otherDeps || withDeps == noDeps || otherDeps == noDeps {
		t.Errorf("expected distinct directories, got %s, %s, %s", withDeps, otherDeps, noDeps)
	}
}

// A directory removed behind the store's back must be rebuilt without leaving a
// second database row: the stale row would shadow the new one and force a
// rebuild on every run.
func TestMakeLocalRebuildsRemovedDirectory(t *testing.T) {
	s := New(t.TempDir())

	dir, err := s.MakeLocal([]string{"pyyaml>=6"})
	if err != nil {
		t.Fatalf("MakeLocal: %v", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	again, err := s.MakeLocal([]string{"pyyaml>=6"})
	if err != nil {
		t.Fatalf("MakeLocal (after removal): %v", err)
	}
	if _, err := os.Stat(filepath.Join(again, localMarkerFile)); err != nil {
		t.Errorf("expected the directory to be rebuilt: %v", err)
	}

	repos, err := s.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 recorded repo after rebuild, got %d", len(repos))
	}
	if repos[0].Path != again {
		t.Errorf("recorded path %s, want %s", repos[0].Path, again)
	}
}
