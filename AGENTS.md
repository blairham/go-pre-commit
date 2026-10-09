# AGENTS.md — go-pre-commit

Cross-tool source of truth for this repo. The sibling `CLAUDE.md` imports it; the tree-level `~/Developer/github.com/blairham/AGENTS.md` applies underneath and this file wins on conflict.

## Project Overview

go-pre-commit is a Go reimplementation of [pre-commit](https://github.com/pre-commit/pre-commit) — a framework for managing multi-language git hooks. It is a **drop-in replacement**: same CLI surface, same `.pre-commit-config.yaml`, same cache layout, no Python required.

- **Module:** `github.com/blairham/go-pre-commit/v4` (note the `/v4` suffix — every internal import carries it)
- **Binary name:** `pre-commit`, not `go-pre-commit`
- **CLI framework:** `mitchellh/cli` for dispatch, `jessevdk/go-flags` for flag parsing
- **Entry point:** `main.go` → `internal/cli.Run()`

**Versioning tracks upstream parity, not semver of this codebase.** `v4.6.0` means feature parity with Python pre-commit 4.6.0. When adding features, diff against the [upstream CHANGELOG](https://github.com/pre-commit/pre-commit) rather than inventing behavior — divergence from Python pre-commit is a bug, not a feature.

## Quick Reference

```bash
make build       # Build to build/pre-commit
make install     # go install with version ldflags
make test        # go test -v -race ./...
make test-cover  # tests + HTML coverage report
make fmt         # go tool gofumpt -w .
make vet         # go vet ./...
make tidy        # go mod tidy
make check       # vet + test + build
make clean       # remove build/, coverage artifacts
```

**There is no `lint` target, and agents never run golangci-lint by hand.** It runs as a pre-commit hook on every commit and in CI's `Pre-commit` job; fix what the hook reports and commit again.

## Project Structure

```
main.go                  # Entry point — delegates to internal/cli.Run()
action.yml               # Composite GitHub Action (repo root) — see CI/CD below

internal/
  cli/                   # Command definitions; each command implements cli.Command
  config/                # .pre-commit-config.yaml parsing; holds Version (ldflags target)
  fsutil/                # Filesystem helpers
  git/                   # Git operations — staging, refs, hooks dir
  hook/                  # Hook execution engine and runner
  identify/              # File type identification by extension, filename, shebang
  languages/             # Language backends — python, node, golang, ruby, rust, docker, …
  output/                # Terminal output formatting (lipgloss styles)
  pcre/                  # PCRE regex support via dlclark/regexp2
  repository/            # Hook repository resolution and caching
  staged/                # Stash management for staged files
  store/                 # On-disk cache for cloned hook repos
  xargs/                 # Parallel execution with batching

test/integration/        # Parity tests against real Python pre-commit (build tag: integration)
```

## Code Conventions

- Commands implement `mitchellh/cli.Command`: `Run(args []string) int`, `Help() string`, `Synopsis() string`
- Each command embeds `*Meta` for shared state, and has a flags struct embedding `GlobalFlags`
- Flag parsing uses `jessevdk/go-flags` struct tags
- Error output goes to stderr; return `1` for failure, `0` for success
- **Mirror the Python CLI exactly** — flag names, output wording, and exit codes are part of the contract
- Formatting is `gofumpt` + `goimports` with `github.com/blairham/go-pre-commit` as the local prefix, applied by the `golangci-lint-fmt` pre-commit hook (see `.golangci.yml` for the enabled formatters)

## Testing

- `make test` runs the unit suite with the race detector — this is the gate that matters day to day
- **Parity tests** live in `test/integration/` behind the `integration` build tag and need real Python pre-commit installed:
  ```bash
  go test -v -tags=integration -timeout=600s ./test/integration/
  ```
  CI runs them on every push to `main` and on every pull request that changes code
- Tests must never touch real user state — redirect the home directory and hook cache via `t.TempDir()` + `t.Setenv`

## CI/CD

`.github/workflows/ci.yml` is the one workflow that gates a merge; Go comes from `go.mod` (`go-version-file`). Its first job calls the shared `go-ci.yml` in [blairham/.github](https://github.com/blairham/.github), pinned by the SHA of that repo's latest `vX.Y.Z` tag; the rest are this repo's own, `needs: changes` (a `go-changes.yml` call at the same pin, so they start without waiting for all of go-ci), with steps gated on `needs.changes.outputs.code`. Every action, in the workflows and in `action.yml`, is pinned to a commit SHA with a `# vX.Y.Z` comment — Dependabot moves them.

| Job | What it does |
|---|---|
| `CI / Pre-commit` | The hooks over the change's diff, then the `golangci-lint-new` manual hook (issues the change introduces). Runs the go-pre-commit release pinned in blairham/.github, so this repo never needs a release of itself to pass CI |
| `CI / Detect changed files` | Skips the code jobs for prose-only PRs (inside the jobs, never a `paths:` filter) |
| `CI / Build and test (ubuntu-latest)` | `make test` |
| `CI / Fuzz` | Every Fuzz target, on pushes to main and weekly |
| `Parity with Python pre-commit` | Differential suite against real Python pre-commit 4.6.2 |
| `Build` | `make build` |
| `Action (ubuntu/macos/windows)` | Runs `action.yml` from the checkout (`uses: ./`): installs the *released* binary and runs a pygrep and a python hook |
| `Hooks from source (windows-latest)` | Builds this checkout on Windows and runs a python, node, golang, ruby and rust hook, each against input it must reject |

The synced config files (`.golangci.yml`, `.pre-commit-config.yaml`, `.editorconfig`, `.yamllint.yml`, `.gitleaks.toml`, `dependabot.yml`, `CODEOWNERS`, `codeql.yml`, `scorecard.yml`) are rendered by blairham/.github's `make sync`; change them there (or as an approved override in its `overrides/go-pre-commit.yml`), not here.

`codeql.yml` (security-extended) and `scorecard.yml` (OpenSSF Scorecard) run on pushes to `main` and on a schedule; CodeQL also runs on PRs.

**The repo-root `action.yml` is a public composite action.** It downloads the release binary and runs hooks — a drop-in for `pre-commit/action` with no Python setup. `aws-sso-config`, `aws-config-management`, and `ghorg` consume it in their own CI, so a breaking change to its inputs (`version`, `extra_args`, `cache`, `install-only`) breaks those repos. The `pre-commit` CI job dogfoods it against this repo.

Releases: move `CHANGELOG.md`'s `[Unreleased]` under the version, push a `v*` tag → `release.yml` calls blairham/.github's `go-release.yml`, which runs the tests, then GoReleaser (`.goreleaser.yaml`) builds, signs, and notarizes, updates `blairham/homebrew-tap`, attests provenance, and publishes the tag's CHANGELOG section as the notes; then the moving `v4` alias tag follows.

## Toolchain

- `go.mod`'s `go` directive is authoritative and must match `.tool-versions`' `golang` pin **exactly** — enforced by the `check-go-version-sync` hook from [blairham/pre-commit-hooks](https://github.com/blairham/pre-commit-hooks), pinned by `rev` in `.pre-commit-config.yaml`
- golangci-lint and gofumpt are pinned in `go.mod`'s `tool` block — invoke as `go tool <name>`, never a separately installed binary
- Keep the `golangci-lint` pre-commit `rev` and the `go.mod` tool pin in lockstep
- goreleaser is pinned in `.tool-versions`, not `go.mod`

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/mitchellh/cli` | Command dispatch |
| `github.com/jessevdk/go-flags` | Flag parsing (struct tags) |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `github.com/dlclark/regexp2` | PCRE-compatible regex (Python `re` parity) |
| `gopkg.in/yaml.v3` | Config parsing |
