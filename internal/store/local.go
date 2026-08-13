package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blairham/go-pre-commit/v4/internal/fsutil"
	"github.com/blairham/go-pre-commit/v4/internal/output"
)

// localRepoVersion mirrors Python pre-commit's LOCAL_REPO_VERSION: the "rev"
// recorded for the synthetic repo that backs `repo: local` hooks. Bump it when
// the placeholder layout below changes so existing caches are rebuilt.
const localRepoVersion = "1"

// localMarkerFile is the placeholder file used to recognize an intact local
// repo directory, the way a `.git` directory identifies a real clone.
const localMarkerFile = "setup.py"

// localResources are the placeholder package manifests written into a local
// repo directory. `repo: local` hooks have no repository to install, but the
// language backends still install *something* from the prefix (`pip install .`,
// `gem build`, `cargo install --path .`, …) before layering
// additional_dependencies on top — so each backend needs a valid, empty package
// to point at. These match Python pre-commit's empty_template_* resources.
var localResources = map[string]string{
	"setup.py": `from setuptools import setup


setup(name='pre-commit-placeholder-package', version='0.0.0', py_modules=[])
`,
	"main.go": `package main

func main() {}
`,
	"go.mod": "module pre-commit-placeholder-empty-module\n",
	"Cargo.toml": `[package]
name = "__fake_crate"
version = "0.0.0"

[[bin]]
name = "__fake_cmd"
path = "main.rs"
`,
	"main.rs": "fn main() {}\n",
	"pre_commit_placeholder_package.gemspec": `Gem::Specification.new do |s|
    s.name = 'pre_commit_placeholder_package'
    s.version = '0.0.0'
    s.summary = 'placeholder gem for pre-commit hooks'
    s.authors = ['Anthony Sottile']
end
`,
	"environment.yml": `channels:
  - conda-forge
  - defaults
dependencies:
  # This cannot be empty as otherwise no environment will be created.
  # We're using openssl here as it is available on all system and will
  # most likely be always installed anyways.
  # See https://github.com/conda/conda/issues/9487
  - openssl
`,
	"Makefile.PL": `use ExtUtils::MakeMaker;

WriteMakefile(
    NAME => "PreCommitPlaceholder",
    VERSION => "0.0.1",
);
`,
	"pubspec.yaml": `name: pre_commit_empty_pubspec
environment:
  sdk: '>=2.12.0'
executables: {}
`,
	"pre-commit-package-dev-1.rockspec": `package = "pre-commit-package"
version = "dev-1"

source = {
   url = "git+ssh://git@github.com/pre-commit/pre-commit.git"
}
description = {}
dependencies = {}
build = {
    type = "builtin",
    modules = {},
}
`,
}

// LocalRepoName returns the name a local repo is recorded under, which encodes
// the dependencies installed into it — `local` on its own, or
// `local:dep1,dep2`. Different additional_dependencies therefore get different
// environments instead of sharing (and clobbering) one.
func LocalRepoName(deps []string) string {
	if len(deps) == 0 {
		return "local"
	}
	return "local:" + strings.Join(deps, ",")
}

// LocalRepoKey returns the "repo@rev" key GC uses to keep a local repo alive.
func LocalRepoKey(deps []string) string {
	return LocalRepoName(deps) + "@" + localRepoVersion
}

// isLocalRepo reports whether path holds an intact local repo directory.
func isLocalRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, localMarkerFile))
	return err == nil
}

// MakeLocal returns the directory that backs `repo: local` hooks declaring the
// given additional_dependencies, creating it on first use. It is the local
// equivalent of Clone: hooks get a prefix, so the language backend has
// somewhere to build its environment and install the dependencies into.
func (s *Store) MakeLocal(deps []string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := LocalRepoName(deps)

	if path, err := s.lookupWith(name, localRepoVersion, isLocalRepo); err == nil {
		return path, nil
	}

	if err := s.Init(); err != nil {
		return "", err
	}

	unlock, err := s.acquireLock()
	if err != nil {
		return "", fmt.Errorf("failed to acquire store lock: %w", err)
	}
	defer unlock()

	// Double-check after acquiring the lock (another process may have won).
	if path, err := s.lookupWith(name, localRepoVersion, isLocalRepo); err == nil {
		return path, nil
	}

	output.Info("Initializing environment for %s.", name)

	dest := filepath.Join(s.dir, cloneDirName(name, localRepoVersion))
	// The path is derived from the name, so a partially-written directory from
	// an interrupted run sits exactly where this one wants to go. Clear it.
	if err := fsutil.RemoveAll(dest); err != nil {
		return "", fmt.Errorf(
			"failed to clear stale cache directory %s (remove it manually to recover): %w",
			dest, err)
	}

	if err := writeLocalResources(dest); err != nil {
		_ = fsutil.RemoveAll(dest)
		return "", err
	}

	if err := s.save(name, localRepoVersion, dest); err != nil {
		return "", err
	}

	return dest, nil
}

func writeLocalResources(dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("failed to create local repo directory %s: %w", dest, err)
	}
	for name, contents := range localResources {
		path := filepath.Join(dest, name)
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}
	return nil
}
