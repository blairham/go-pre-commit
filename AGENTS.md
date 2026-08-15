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
make lint        # go tool golangci-lint run ./...
make fmt         # go tool gofumpt -w .
make vet         # go vet ./...
make tidy        # go mod tidy
make check       # fmt + vet + test
make clean       # remove build/, coverage artifacts
```

**`make check` does not run `lint`** — it is `fmt vet test`. Run `make check` *and* `make lint` before opening a PR.

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
  CI runs them on every push to `main`, and on PRs only when labeled `test-languages`
- Tests must never touch real user state — redirect the home directory and hook cache via `t.TempDir()` + `t.Setenv`

## CI/CD

`.github/workflows/ci.yml` — `GO_VERSION: "1.26"` (minor line; setup-go resolves the latest patch):

| Job | What it does |
|---|---|
| `test` | `make test` |
| `lint` | `golangci-lint-action@v9`, `--timeout=10m` |
| `pre-commit` | Dogfoods the repo's own `action.yml` via `uses: ./` |
| `build` | `make build`, gated on `test` + `lint` |
| `language-integration` | Parity tests with real Python pre-commit, Python 3.13 + Node 22 |

**The repo-root `action.yml` is a public composite action.** It downloads the release binary and runs hooks — a drop-in for `pre-commit/action` with no Python setup. `aws-sso-config`, `aws-config-management`, and `ghorg` consume it in their own CI, so a breaking change to its inputs (`version`, `extra_args`, `cache`, `install-only`) breaks those repos. The `pre-commit` CI job dogfoods it against this repo.

Releases: push a `v*` tag → GoReleaser (`.goreleaser.yaml`) builds, signs, and notarizes, and updates `blairham/homebrew-tap`.

## Toolchain

- `go.mod`'s `go` directive is authoritative and must match `.tool-versions`' `golang` pin **exactly** — enforced by the `check-go-version-sync` hook from [blairham/pre-commit-hooks](https://github.com/blairham/pre-commit-hooks), pinned by `rev` in `.pre-commit-config.yaml`
- golangci-lint and gofumpt are pinned in `go.mod`'s `tool` block — invoke as `go tool <name>`, never a separately installed binary
- Keep the `golangci-lint` pre-commit `rev`, the `go.mod` tool pin, and the CI action version in lockstep
- goreleaser is pinned in `.tool-versions`, not `go.mod`

## Key Dependencies

| Package | Purpose |
|---|---|
| `github.com/mitchellh/cli` | Command dispatch |
| `github.com/jessevdk/go-flags` | Flag parsing (struct tags) |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `github.com/dlclark/regexp2` | PCRE-compatible regex (Python `re` parity) |
| `gopkg.in/yaml.v3` | Config parsing |
