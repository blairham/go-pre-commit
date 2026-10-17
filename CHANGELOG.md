# Changelog

All notable changes to go-pre-commit are recorded here. The release workflow
publishes a tag's section as that GitHub release's notes, and fails a release
whose section is missing.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Version numbers track upstream parity rather than semver of this codebase:
`v4.7.x` targets feature parity with Python pre-commit 4.7 (`v4.6.x` targeted 4.6). Sections up to and
including 4.6.16 were reconstructed from the GitHub release notes when this
file was introduced; the full notes are on the
[releases page](https://github.com/blairham/go-pre-commit/releases).

## [Unreleased]

### Fixed

- `validate-config` and `validate-manifest` print nothing for a valid file,
  as upstream does, instead of `Config file(s) are valid.` /
  `Manifest file(s) are valid.`. A failure is printed to stdout in cfgv's
  shape (`==> File <name>` then `=====> <message>`, or
  `=====> <name> is not a file` for a missing path) rather than to stderr as
  `<name>: <error>`, and with no filenames nothing is validated and the exit
  is 0, as upstream's `nargs='*'` does, instead of falling back to the default
  file. (#145)
- `install` and `uninstall` are ported from upstream's
  `install_uninstall.py`. The hook path comes from git's common dir after
  changing to the root, so `install` prints
  `pre-commit installed at .git/hooks/pre-commit` in a plain checkout (and
  an absolute path in a linked worktree) rather than an absolute path
  everywhere. A foreign hook moves to `.legacy` silently, and an existing
  legacy hook is announced as upstream does (`Running in migration mode with
  existing hooks at ...`); `-f` removes it. `uninstall` prints
  `<type> uninstalled` and `Restored previous hooks to <path>`, says nothing
  about a hook that is missing or not ours, and without `-t` takes the
  config's `default_install_hook_types`, as `install` does. With
  `core.hooksPath` set, `install` now refuses with upstream's
  ``[ERROR] Cowardly refusing to install hooks with `core.hooksPath` set.``
  and exit 1, rather than warning and writing into that directory; the
  `config file ... not found` warning, which upstream does not print, is
  gone. (#130)
- Health checks are upstream's, and are used where upstream uses them. An
  installed environment is reused only while its language's health check
  passes, and a fresh one that fails it is an error ("BUG: expected
  environment for … to be healthy immediately after install"); before, the
  checks were never consulted. Every language upstream gives the basic check
  (conda, coursier, dart, docker, dotnet, golang, haskell, julia, lua, perl,
  ruby, rust, swift, …) is now always healthy, rather than failing when its
  tool is not on PATH; node runs `node --version` inside the env; python
  compares the versions `pyvenv.cfg` records with the env's and the base
  interpreter's, so an upgraded host Python gets its envs rebuilt. (#140)
- `run <hook-id>` selects and reports as upstream does. A miss prints
  ``No hook with id `<id>` in stage `<stage>` `` on stdout, where this port
  printed `[ERROR] No hook with id "<id>" found` on stderr. A `repo: local`
  hook is now found by its `alias`; a hook with no `stages` is in every
  stage, `manual` included (upstream's `default_stages` defaults to all of
  them); and `--hook-stage` takes the legacy names `commit`, `merge-commit`
  and `push`, as upstream's `transform_stage` does. (#129)
- `language_version: default` is resolved where upstream resolves it, when
  each hook is built: a version still `default` after the manifest and the
  config's hook are merged takes the config's `default_language_version`, and
  one still `default` after that takes the language's own default. The env
  directory is named after the resolved version, as upstream's is
  (`node_env-system`, `golangenv-system`, `py_env-python3.13`, …), and a
  manifest's explicit `language_version: default` now takes the config's
  `default_language_version` instead of ignoring it. python's default is
  upstream's (the versioned interpreter for the Python a pip-installed
  pre-commit would run under, then its major version), and rust's is
  `system` when `cargo --version` succeeds. The env directories upstream
  names differently now carry upstream's names: `golangenv`, `conda`,
  `coursier`, `dartenv`, `dotnetenv`, `renv`. A `language_version` on a
  language that installs no environment is refused in upstream's words, as
  `additional_dependencies` already was. Existing environments under the old
  names are not reused: each is reinstalled once under its new name. (#138)
- `language: fail` lists the files it failed on, as upstream's `fail.py`
  does: the entry, a blank line, then one file per line. It printed only the
  entry. (#128)
- golang and ruby choose their default version as upstream does. A
  `language_version: default` golang hook uses the host's Go only when
  `go` is a real install (on PATH, not a version manager's shim, not inside
  the home directory); otherwise the install downloads the newest Go from
  go.dev into the env's `.go` and builds with it under `GOTOOLCHAIN=local`,
  as an explicit `language_version` also now does. A ruby hook uses the
  host's ruby only when both `ruby` and `gem` are real installs; otherwise
  the install unpacks upstream's vendored rbenv into the env, and an explicit
  version is downloaded (or built) through ruby-download/ruby-build. Ruby's
  gem build and install now pass upstream's flags (every gemspec;
  `--no-format-executable --no-user-install --install-dir --bindir`), and
  hooks run with `GEM_PATH` unset and `BUNDLE_IGNORE_CONFIG=1`. The Windows
  rules of `exe_exists` (drive letters and UNC shares, `commonpath` across
  drives, a home at a drive root) now match ntpath. (#137)
- A config with `repos: []` is valid, as it is upstream, where `repos` is
  `cfgv.Required` over an array: `run`, `validate-config`, `autoupdate` and
  `install-hooks` accept it instead of failing with `'repos' is required`. A
  missing (or null) `repos` still fails. (#131)
- The deprecated stage names `commit`, `merge-commit` and `push` are warned
  about as upstream does: for a hook's `stages` and for `default_stages` in
  the config (with a `pre-commit migrate-config` hint), and once a hook repo
  is cloned whose manifest still uses them (with a
  `pre-commit autoupdate --repo` hint). A hook repo's manifest that uses the
  old names now has them migrated as the config's were, so those hooks run
  at the right stage instead of never matching. (#136)
- The mutable-`rev` warning is upstream's. It fired only for a `rev` of
  exactly `master`, `main`, `develop` or `HEAD`, on stderr, in this port's own
  words; it now fires for any `rev` with no `.` that is not all hex (so `v1`
  and `stable` warn, as do path repos), with upstream's wording, as a
  `[WARNING]` line on stdout. (#135)
- `gc` no longer deletes repos that another checkout's config still uses.
  Every command upstream runs inside a repository now records its config in
  the store, as upstream's `store.mark_config_used` does (the path
  realpath'd, a missing config skipped, an unwritable store left alone), so
  `gc` sees every config that uses the cache rather than only the current
  one. `gc` also forgets tracked configs that no longer load, as upstream's
  `_gc` does. (#133)
- A hook repo whose hooks declare different `additional_dependencies` (such
  as blairham/pre-commit-hooks' golang hooks and its `go-vulncheck`) no longer
  reinstalls its environments on every run. All of a repo's hooks shared one
  clone, so both dependency sets were installed into one environment
  directory, concurrently, each overwriting the other's install state; the
  next run saw a mismatch and reinstalled both, and the two `go install`s
  sharing one module cache failed intermittently with `unknown revision`. As
  upstream's `store.clone(repo, rev, deps)` does, each dependency set now gets
  a clone of its own, recorded as `repo:dep1,dep2` (which `gc` keeps while a
  config uses it). Existing caches are reinstalled once. Installs also take
  the store's exclusive lock and re-check what still needs installing once
  they hold it, as upstream's `install_hook_envs` does, so two commits in
  sibling worktrees no longer build the same environment at once. (#141)
- The golang backend builds `go install`'s environment as upstream does: the
  caller's environment without git's variables, with GOPATH at the hook env.
  It no longer forces `GOTOOLCHAIN=local`, and a hook runs with only PATH
  changed rather than with GOPATH pointed at its env, so a hook that runs the
  go tool uses the caller's module cache. The env's module cache is removed
  after install, as upstream does. GOBIN is set to the env's `bin` rather
  than removed, which is the directory upstream installs into, so that a go
  shim exporting its own GOBIN (asdf's does) cannot send the binaries
  elsewhere. (#141)
- `language: pygrep` prints what upstream's `pygrep.py` prints, in all four
  modes. A matching line is `file:N:line` with no space after the line
  number and without its line ending. `--multiline` reports the first match
  at the line it starts on, printed as that whole line plus the rest of the
  match, and its `^`/`$` are per line (upstream compiles with `MULTILINE |
  DOTALL`; this port had only `DOTALL`). `--negate` was inverted: it now
  fails, naming the file, when the pattern is nowhere in a file, rather than
  when it is found. `--ignore-case` and argparse's abbreviations of the long
  options are accepted. (#127)
- `autoupdate` and `try-repo` run upstream 4.7.0's git commands with git's
  output captured, so `Cloning into ...`, `--filter is ignored` and similar
  no longer reach the terminal. `autoupdate` fetches each repo's `HEAD` and
  tags blobless into an empty repository, as upstream's `RevInfo.update`
  does; it prints upstream's `[repo] updating X -> Y` and `already up to
  date!` lines, picks the tag that looks like a version when a moving tag
  shares its commit, takes a tag on `HEAD` under `--bleeding-edge`, refuses
  an update that drops a hook the config uses (exit 1), and rewrites each
  `rev:` line by position in its own quoting, keeping comments other than
  `# frozen:`. `try-repo` follows upstream's `_repo_ref`: a local repository
  is cloned at its `HEAD` like a remote one, and only when it has
  uncommitted changes to tracked files is a shadow repository committed with
  them (untracked files are no longer copied in). A git command that fails
  is now upstream's `An unexpected error has occurred: CalledProcessError:
  ...` with exit 3. (#134)
- Hooks run in upstream's xargs partitions instead of one invocation. The
  files are split into runs of at most `max(4, ceil(files / jobs))`
  arguments, each also short enough for the platform's command-length limit
  (ARG_MAX less the environment, clamped to 4096..131072 bytes; 30720 UTF-16
  units on Windows, about 7 KB for a `.bat`/`.cmd`), and run on
  `min(partitions, jobs)` workers with their output joined in partition
  order and the exit code of largest magnitude. `jobs` is the CPU count, 2
  under `TRAVIS`, and 1 under `PRE_COMMIT_NO_CONCURRENCY` or for a
  `require_serial` hook; `run --jobs` still overrides it. A hook that prints
  once per invocation now prints as often as upstream's does, and a large
  file list no longer risks exceeding the OS argument limit. (#132)
- Hooks receive their files in upstream's order. Upstream shuffles each
  hook's file list with a fixed-seed `random.Random` before splitting it into
  xargs batches, unless the hook is `require_serial`; this port never
  shuffled, so order-sensitive hooks saw a different sequence. A
  byte-compatible port of CPython's MT19937 seeding, `getrandbits` and
  `shuffle` now reproduces it, skipping `require_serial`, `fail` and `pygrep`
  hooks as upstream does. (#120)
- Every command upstream runs inside a repository (`autoupdate`, `hook-impl`,
  `install`, `install-hooks`, `migrate-config`, `run`, `try-repo`,
  `uninstall`) now changes to the repository root first and rewrites
  `--config` relative to it, as upstream's `_adjust_args_and_chdir` does;
  `run` and `try-repo` do the same for `--files` and
  `--commit-msg-filename`, and `try-repo` for a local repository path. Run
  from a subdirectory, these commands now find the root's config instead of
  failing to load it, and a printed config path is root-relative
  (`-c ../alt.yaml` reports `alt.yaml`). Outside a repository, or inside
  `.git`, they fail with upstream's `An error has occurred: FatalError: ...`
  and a log in the store. (#109)
- `language: julia` is ported from upstream 4.7.0. Hooks now run their
  script: the entry's first word was dropped, so julia saw the hook's own
  arguments as options and failed. The script resolves against the hook repo
  for a remote hook and is left as written for `repo: local`. The env sets
  `JULIA_LOAD_PATH` and unsets `JULIA_PROJECT`, as upstream's does. Install
  copies the hook's `(Julia)Project.toml` and `(Julia)Manifest.toml` (or starts
  from an empty `Project.toml`) and instantiates it with upstream's script,
  where it used to `Pkg.develop` the hook repo and fail on any repo without a
  project file. The environment directory is upstream's `juliaenv-<version>`;
  a julia hook installed by an earlier release is reinstalled once, and its
  old `julia_env-*` directory stays in the cache until the repo is gc'd. CI now
  installs and runs a real julia hook, remote and local, against Python
  pre-commit. (#118)

- The first install of a remote hook repo no longer prints git's own output.
  The store clones the way upstream's `Store.clone` does: `git init`, then a
  shallow fetch of exactly the pinned `rev` (so a commit sha is fetched at
  depth 1 instead of failing `git clone --branch` and falling back to a full
  clone), submodules at depth 1, and a fetch of all tags only when that
  fails. git's output is captured and shown only in the error when a clone
  fails, as upstream's `CalledProcessError` reports it, and
  `[INFO] Initializing environment for <repo>.` is printed as upstream prints
  it. A parity row compares the first-install output byte for byte. (#117)

- `language: node`'s default version follows upstream: `system` only when
  `node` and `npm` are both real installs on `PATH` (not in a version
  manager's `shims` directory, and not under the home directory unless that
  is `/`), otherwise `default`, which nodeenv installs with no `-n`, i.e. its
  own prebuilt node. Every non-Windows install used to symlink the host's
  node, so on a machine whose node and npm are asdf shims the hook install
  failed with `EALLOWGIT` where Python pre-commit passes. Ports upstream's
  `lang_base.exe_exists`. (#119)

## [4.7.0] - 2026-10-15

### Changed

- The parity target moves to Python pre-commit 4.7.0: the differential suite
  pins and measures against it, and `v4.7.x` means parity with upstream 4.7.
  4.7.0's one change (node hooks install without the host's `GIT_*`
  variables) shipped in 4.6.17 with #102; its `git commit -a` parity row now
  grades both sides instead of only this one. (#107)

## [4.6.17] - 2026-10-15

### Changed

- CI and releases run on the shared reusable workflows in
  [blairham/.github](https://github.com/blairham/.github), and the lint, hook
  and editor configuration is synced from its baseline. Release signatures and
  provenance now carry the shared workflow's identity; SECURITY.md has the new
  verify commands. Release notes come from this file. (#96)
- The composite action is tested from the checkout on Linux, macOS and
  Windows. (#96)

### Fixed

- `autoupdate --repo` with a repo the config does not name prints
  `repos (...) were not found in <config>` and exits 1 before updating
  anything, as upstream 4.6.1 does, instead of silently ignoring it. (#105)
- `run --all-files` passes a file with an unresolved merge conflict to hooks
  once rather than once per index stage, via `git ls-files --deduplicate`
  (upstream 4.6.1, pre-commit/pre-commit#3727). This needs git 2.31 or
  later. (#106)
- The pre-push hook now decides what to check the way upstream's
  `_pre_push_ns` does. Pushing a new branch checks the commits being pushed
  rather than the staged files; deleting a branch runs no hooks; a remote tip
  that was never fetched falls back to checking the commits the remote lacks
  instead of failing the push; a push that includes the root commit runs on
  all files; and with several refs on stdin the first one with something to
  check wins. A legacy pre-push hook still sees git's ref lines. (#103)
- `julia`: hooks install and run with `JULIA_DEPOT_PATH` inside the hook
  env, as upstream 4.6.1 does, so packages and precompile caches no longer
  land in the shared depot. (#104)
- `language: node` hooks install the way upstream 4.7.0 does:
  `npm install --allow-git=root --install-links -g git+file://<repo>` plus
  `additional_dependencies`, replacing the local install → `npm pack` →
  global install sequence that npm 12 breaks, and fixing hooks with a
  `scripts.build` under npm 11. npm runs without the host's repository-pinning
  `GIT_*` variables, so a hook installed during `git commit -a` or inside a
  submodule works, and the env's own npm is used ahead of the host's. (#102)
- Releases no longer pass `target_commitish`, which made creating a release
  fail with a 403 when its range touched `.github/workflows`. (#95)

## [4.6.16] - 2026-10-03

### Added

- `run`: hooks run in a pty when color is on, as upstream's do. (#93)

## [4.6.15] - 2026-10-03

### Fixed

- `run`: the hook report matches upstream byte for byte. (#87)
- `run`: upstream's colors, and its `--show-diff-on-failure`. (#92)

### Changed

- Release provenance is attested with GitHub's SHA-pinnable action. (#89)

## [4.6.14] - 2026-10-03

### Changed

- Build provenance on releases; hash-pinned CI installs; x/crypto 0.56.0.
  (#84)

## [4.6.13] - 2026-10-03

### Fixed

- `--version` and `--help` go to stdout, and a bare `pre-commit` runs. (#82)

## [4.6.12] - 2026-10-02

### Fixed

- `try-repo`: a remote repository runs its hooks, as upstream does. (#75)

### Changed

- Every release's notes name the upstream version it targets. (#76)

## [4.6.11] - 2026-10-02

### Fixed

- Stash against the commit's index, and detect rewrites by diff. (#73)

## [4.6.10] - 2026-10-02

### Fixed

- Action: resolve `latest` to one tag before downloading. (#70)
- python, node and golang hooks work on Windows. (#68)

## [4.6.9] - 2026-10-02

### Fixed

- `install` writes hooks to the common git dir, as upstream does. (#65)
- `--version` reports the module version when no ldflag set it. (#66)

## [4.6.8] - 2026-10-02

### Security

- `checksums.txt` is signed with keyless cosign; actions are pinned by SHA
  and the release workflow is least-privilege; CodeQL and OpenSSF Scorecard
  run. (#59, #62)

## [4.6.7] - 2026-09-08

### Fixed

- `run --files` takes several paths, as upstream does. (#50)

## [4.6.6] - 2026-08-15

### Fixed

- `install` wipes a stateless leftover environment before installing over
  it. (#40)

## [4.6.5] - 2026-08-14

### Fixed

- Environments are built for `repo: local` hooks. (#38)

## [4.6.4] - 2026-08-13

### Changed

- Homebrew distribution is a formula instead of a cask. (#36)

## [4.6.3] - 2026-08-13

### Fixed

- `identify` no longer tags directories, symlinks and `pyproject.toml`
  wrongly. (#35)

## [4.6.2] - 2026-08-06

### Changed

- Hardened macOS Gatekeeper handling in the Homebrew cask.

## [4.6.1] - 2026-08-05

### Fixed

- Action: retry transport errors when downloading the release archive. (#33)

## [4.6.0] - 2026-07-05

### Added

- Behavioral parity with Python pre-commit 4.6.0.
- A composite GitHub Action for running hooks in CI, with an `install-only`
  input. (#31)

### Fixed

- golang hooks respect a caller-set `GOTOOLCHAIN` when installing
  environments. (#32)
- node hook installs, manifest `additional_dependencies`, and dev version
  parsing.

## [4.5.4] - 2026-06-20

### Fixed

- Leaked git environment variables no longer corrupt the host repository
  during hooks. (#30)

### Changed

- macOS binaries are signed and notarized.

## [4.5.3] - 2026-05-14

### Added

- `autoupdate --dry-run`.
- Repeated `-t/--hook-type` for `uninstall` and `init-templatedir`.
- `try-repo --jobs`.

### Fixed

- Hook concurrency is capped by file count, as in Python.
- The version is read from Go build info under `go install`.

## [4.5.2] - 2026-04-20

### Changed

- The entry point moved from `cmd/pre-commit` to the module root. (#24)

## [4.5.1] - 2026-04-19

### Added

- First release of the rewrite: Python parity for `run` and `uninstall`, the
  `/v4` module path, and a GoReleaser release workflow.

[Unreleased]: https://github.com/blairham/go-pre-commit/compare/v4.7.0...HEAD
[4.7.0]: https://github.com/blairham/go-pre-commit/compare/v4.6.17...v4.7.0
[4.6.17]: https://github.com/blairham/go-pre-commit/compare/v4.6.16...v4.6.17
[4.6.16]: https://github.com/blairham/go-pre-commit/compare/v4.6.15...v4.6.16
[4.6.15]: https://github.com/blairham/go-pre-commit/compare/v4.6.14...v4.6.15
[4.6.14]: https://github.com/blairham/go-pre-commit/compare/v4.6.13...v4.6.14
[4.6.13]: https://github.com/blairham/go-pre-commit/compare/v4.6.12...v4.6.13
[4.6.12]: https://github.com/blairham/go-pre-commit/compare/v4.6.11...v4.6.12
[4.6.11]: https://github.com/blairham/go-pre-commit/compare/v4.6.10...v4.6.11
[4.6.10]: https://github.com/blairham/go-pre-commit/compare/v4.6.9...v4.6.10
[4.6.9]: https://github.com/blairham/go-pre-commit/compare/v4.6.8...v4.6.9
[4.6.8]: https://github.com/blairham/go-pre-commit/compare/v4.6.7...v4.6.8
[4.6.7]: https://github.com/blairham/go-pre-commit/compare/v4.6.6...v4.6.7
[4.6.6]: https://github.com/blairham/go-pre-commit/compare/v4.6.5...v4.6.6
[4.6.5]: https://github.com/blairham/go-pre-commit/compare/v4.6.4...v4.6.5
[4.6.4]: https://github.com/blairham/go-pre-commit/compare/v4.6.3...v4.6.4
[4.6.3]: https://github.com/blairham/go-pre-commit/compare/v4.6.2...v4.6.3
[4.6.2]: https://github.com/blairham/go-pre-commit/compare/v4.6.1...v4.6.2
[4.6.1]: https://github.com/blairham/go-pre-commit/compare/v4.6.0...v4.6.1
[4.6.0]: https://github.com/blairham/go-pre-commit/compare/v4.5.4...v4.6.0
[4.5.4]: https://github.com/blairham/go-pre-commit/compare/v4.5.3...v4.5.4
[4.5.3]: https://github.com/blairham/go-pre-commit/compare/v4.5.2...v4.5.3
[4.5.2]: https://github.com/blairham/go-pre-commit/compare/v4.5.1...v4.5.2
[4.5.1]: https://github.com/blairham/go-pre-commit/releases/tag/v4.5.1
