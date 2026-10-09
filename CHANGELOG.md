# Changelog

All notable changes to go-pre-commit are recorded here. The release workflow
publishes a tag's section as that GitHub release's notes, and fails a release
whose section is missing.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Version numbers track upstream parity rather than semver of this codebase:
`v4.6.x` targets feature parity with Python pre-commit 4.6. Sections up to and
including 4.6.16 were reconstructed from the GitHub release notes when this
file was introduced; the full notes are on the
[releases page](https://github.com/blairham/go-pre-commit/releases).

## [Unreleased]

### Changed

- CI and releases run on the shared reusable workflows in
  [blairham/.github](https://github.com/blairham/.github), and the lint, hook
  and editor configuration is synced from its baseline. Release signatures and
  provenance now carry the shared workflow's identity; SECURITY.md has the new
  verify commands. Release notes come from this file. (#96)
- The composite action is tested from the checkout on Linux, macOS and
  Windows. (#96)

### Fixed

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

[Unreleased]: https://github.com/blairham/go-pre-commit/compare/v4.6.16...HEAD
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
